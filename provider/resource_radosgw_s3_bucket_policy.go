package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &BucketPolicyResource{}
var _ resource.ResourceWithImportState = &BucketPolicyResource{}

func NewS3BucketPolicyResource() resource.Resource {
	return &BucketPolicyResource{}
}

// BucketPolicyResource defines the resource implementation.
type BucketPolicyResource struct {
	client *RadosgwClient
}

// BucketPolicyResourceModel describes the resource data model.
type BucketPolicyResourceModel struct {
	Bucket types.String `tfsdk:"bucket"`
	Policy types.String `tfsdk:"policy"`
	ID     types.String `tfsdk:"id"`
}

func (r *BucketPolicyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_s3_bucket_policy"
}

func (r *BucketPolicyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Attaches a policy to an S3 bucket in RadosGW. Only one policy can be attached to a bucket at a time.

Bucket policies provide access control for S3 buckets and their objects. They use the same policy language as IAM policies
but are attached directly to buckets rather than users or roles.

~> **Note:** Bucket policies in RadosGW support a subset of Amazon S3 policy features. See the
[Ceph RadosGW Bucket Policies documentation](https://docs.ceph.com/en/latest/radosgw/bucketpolicy/) for supported actions and conditions.

~> **Important:** Destroying this resource will delete the bucket policy. The bucket itself will remain.`,

		Attributes: map[string]schema.Attribute{
			"bucket": schema.StringAttribute{
				MarkdownDescription: "The name of the bucket to which the policy will be applied.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"policy": schema.StringAttribute{
				MarkdownDescription: "The policy document in JSON format. Use `jsonencode()` or the `radosgw_iam_policy_document` data source to generate this.",
				Required:            true,
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "The bucket name (used as the resource ID).",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *BucketPolicyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*RadosgwClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *RadosgwClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *BucketPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan BucketPolicyResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucket := plan.Bucket.ValueString()
	policy := plan.Policy.ValueString()

	// Normalize the policy JSON
	normalizedPolicy, err := normalizeJSONString(policy)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid Policy JSON",
			fmt.Sprintf("The policy is not valid JSON: %s", err.Error()),
		)
		return
	}

	// Put the bucket policy
	_, err = r.client.S3.PutBucketPolicy(ctx, &s3.PutBucketPolicyInput{
		Bucket: aws.String(bucket),
		Policy: aws.String(normalizedPolicy),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Bucket Policy",
			fmt.Sprintf("Could not create bucket policy for bucket %s: %s", bucket, err.Error()),
		)
		return
	}

	plan.ID = types.StringValue(bucket)
	plan.Policy = types.StringValue(normalizedPolicy)

	tflog.Trace(ctx, "Created bucket policy", map[string]any{
		"bucket": bucket,
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *BucketPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state BucketPolicyResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucket := state.Bucket.ValueString()

	// Get the bucket policy
	output, err := r.client.S3.GetBucketPolicy(ctx, &s3.GetBucketPolicyInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		// Check if policy doesn't exist
		var apiErr smithy.APIError
		if ok := errors.As(err, &apiErr); ok {
			if apiErr.ErrorCode() == "NoSuchBucketPolicy" {
				tflog.Info(ctx, "Bucket policy not found, removing from state", map[string]any{
					"bucket": bucket,
				})
				resp.State.RemoveResource(ctx)
				return
			}
			if apiErr.ErrorCode() == "NoSuchBucket" {
				tflog.Info(ctx, "Bucket not found, removing from state", map[string]any{
					"bucket": bucket,
				})
				resp.State.RemoveResource(ctx)
				return
			}
		}
		resp.Diagnostics.AddError(
			"Error Reading Bucket Policy",
			fmt.Sprintf("Could not read bucket policy for bucket %s: %s", bucket, err.Error()),
		)
		return
	}

	if output.Policy == nil {
		tflog.Info(ctx, "Bucket policy is empty, removing from state", map[string]any{
			"bucket": bucket,
		})
		resp.State.RemoveResource(ctx)
		return
	}

	// Normalize the policy for comparison
	normalizedPolicy, err := normalizeJSONString(*output.Policy)
	if err != nil {
		// If we can't normalize, use the raw policy
		normalizedPolicy = *output.Policy
	}

	state.Policy = types.StringValue(normalizedPolicy)
	state.ID = types.StringValue(bucket)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *BucketPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan BucketPolicyResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucket := plan.Bucket.ValueString()
	policy := plan.Policy.ValueString()

	// Normalize the policy JSON
	normalizedPolicy, err := normalizeJSONString(policy)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid Policy JSON",
			fmt.Sprintf("The policy is not valid JSON: %s", err.Error()),
		)
		return
	}

	// Put the bucket policy (same as create - PutBucketPolicy is idempotent)
	_, err = r.client.S3.PutBucketPolicy(ctx, &s3.PutBucketPolicyInput{
		Bucket: aws.String(bucket),
		Policy: aws.String(normalizedPolicy),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Bucket Policy",
			fmt.Sprintf("Could not update bucket policy for bucket %s: %s", bucket, err.Error()),
		)
		return
	}

	plan.Policy = types.StringValue(normalizedPolicy)
	plan.ID = types.StringValue(bucket)

	tflog.Debug(ctx, "Updated bucket policy", map[string]any{
		"bucket": bucket,
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *BucketPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state BucketPolicyResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucket := state.Bucket.ValueString()

	// Delete the bucket policy
	_, err := r.client.S3.DeleteBucketPolicy(ctx, &s3.DeleteBucketPolicyInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		// Ignore errors if bucket or policy doesn't exist
		var apiErr smithy.APIError
		if ok := errors.As(err, &apiErr); ok {
			if apiErr.ErrorCode() == "NoSuchBucketPolicy" || apiErr.ErrorCode() == "NoSuchBucket" {
				tflog.Info(ctx, "Bucket or policy already deleted", map[string]any{
					"bucket": bucket,
				})
				return
			}
		}
		resp.Diagnostics.AddError(
			"Error Deleting Bucket Policy",
			fmt.Sprintf("Could not delete bucket policy for bucket %s: %s", bucket, err.Error()),
		)
		return
	}

	tflog.Trace(ctx, "Deleted bucket policy", map[string]any{
		"bucket": bucket,
	})
}

func (r *BucketPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import by bucket name
	resource.ImportStatePassthroughID(ctx, path.Root("bucket"), req, resp)
}

// normalizeJSONString parses and re-encodes JSON to normalize whitespace and key ordering.
func normalizeJSONString(jsonStr string) (string, error) {
	var parsed any
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		return "", err
	}

	// Re-encode with consistent formatting (no extra whitespace)
	normalized, err := json.Marshal(parsed)
	if err != nil {
		return "", err
	}

	return string(normalized), nil
}
