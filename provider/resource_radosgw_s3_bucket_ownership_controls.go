package provider

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &S3BucketOwnershipControlsResource{}
var _ resource.ResourceWithImportState = &S3BucketOwnershipControlsResource{}
var _ resource.ResourceWithModifyPlan = &S3BucketOwnershipControlsResource{}

func NewS3BucketOwnershipControlsResource() resource.Resource {
	return &S3BucketOwnershipControlsResource{}
}

// S3BucketOwnershipControlsResource defines the resource implementation.
type S3BucketOwnershipControlsResource struct {
	client *RadosgwClient
}

// =============================================================================
// Data model
// =============================================================================

type S3BucketOwnershipControlsModel struct {
	Bucket types.String `tfsdk:"bucket"`
	Rule   types.List   `tfsdk:"rule"`
}

type ownershipControlsRuleModel struct {
	ObjectOwnership types.String `tfsdk:"object_ownership"`
}

func ownershipRuleAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"object_ownership": types.StringType,
	}
}

// =============================================================================
// Resource interface methods
// =============================================================================

func (r *S3BucketOwnershipControlsResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_s3_bucket_ownership_controls"
}

func (r *S3BucketOwnershipControlsResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages S3 Object Ownership for a bucket in RadosGW, via the standard S3 " +
			"`PutBucketOwnershipControls` API. This is the RadosGW equivalent of `aws_s3_bucket_ownership_controls` " +
			"and controls whether object ACLs are used — set `BucketOwnerEnforced` to disable ACLs entirely.\n\n" +
			"Object ownership assignment matches AWS:\n\n" +
			"- `ObjectWriter`: the uploader owns the object. Uploading with the `bucket-owner-full-control` canned ACL " +
			"grants the bucket owner `FULL_CONTROL` but the uploader remains the owner.\n" +
			"- `BucketOwnerPreferred`: the uploader owns the object, **unless** it is uploaded with the " +
			"`bucket-owner-full-control` canned ACL, in which case the bucket owner owns it.\n" +
			"- `BucketOwnerEnforced`: the bucket owner owns every object; ACLs are disabled and access is governed only " +
			"by bucket/IAM policy.\n\n" +
			"Changing the mode affects only **future** uploads; existing objects keep their owner (switching to " +
			"`BucketOwnerEnforced` additionally stops object ACLs from being evaluated).\n\n" +
			"~> **Ceph version:** S3 Object Ownership requires **Ceph 21 (Umbrella)**. When the provider's " +
			"`ceph_version` targets an older release, this resource is rejected at plan time.\n\n" +
			"~> **RadosGW differences from AWS** (verified on Ceph 21.2.0):\n" +
			"- **Default (no configuration):** a RadosGW bucket with no ownership controls has ACLs **enabled** " +
			"(`ObjectWriter`-equivalent). AWS defaults new buckets to `BucketOwnerEnforced`. To disable ACLs on " +
			"RadosGW you must set `BucketOwnerEnforced` explicitly with this resource. (The bucket's default ACL is " +
			"still private/owner-only — ACLs being enabled means a public-read ACL *can* be set, not that the bucket " +
			"is public; use `radosgw_s3_bucket_public_access_block` to prevent public ACLs.)\n" +
			"- **Under `BucketOwnerEnforced`:** setting an ACL via `PutObject`/`CopyObject` (`x-amz-acl`) or " +
			"`PutBucketAcl` is rejected with `AccessControlListNotSupported` (same as AWS). However, `GetObjectAcl` " +
			"and `PutObjectAcl` return `403 AccessDenied` on RadosGW (AWS returns the implicit owner-only ACL for the " +
			"read, and `AccessControlListNotSupported` for the write). `GetBucketAcl` still returns the owner ACL.\n\n" +
			"~> **Note:** A bucket has a single ownership-controls configuration. Declaring multiple " +
			"`radosgw_s3_bucket_ownership_controls` resources for the same bucket will cause them to overwrite each " +
			"other. When this resource is destroyed, the ownership controls are removed from the bucket.",

		Attributes: map[string]schema.Attribute{
			"bucket": schema.StringAttribute{
				MarkdownDescription: "The name of the bucket.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},

		Blocks: map[string]schema.Block{
			"rule": schema.ListNestedBlock{
				MarkdownDescription: "The Object Ownership rule. Exactly one rule must be provided.",
				Validators: []validator.List{
					listvalidator.SizeBetween(1, 1),
				},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"object_ownership": schema.StringAttribute{
							MarkdownDescription: "Object ownership mode. Valid values: `BucketOwnerEnforced` " +
								"(ACLs are disabled; the bucket owner owns every object), `BucketOwnerPreferred` " +
								"(objects uploaded with the `bucket-owner-full-control` canned ACL are owned by the " +
								"bucket owner), or `ObjectWriter` (the object writer remains the owner).",
							Required: true,
							Validators: []validator.String{
								stringvalidator.OneOf(
									string(s3types.ObjectOwnershipBucketOwnerEnforced),
									string(s3types.ObjectOwnershipBucketOwnerPreferred),
									string(s3types.ObjectOwnershipObjectWriter),
								),
							},
						},
					},
				},
			},
		},
	}
}

func (r *S3BucketOwnershipControlsResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ModifyPlan rejects the resource on clusters older than Umbrella (21.x), where
// S3 Object Ownership is unsupported.
func (r *S3BucketOwnershipControlsResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	// Nothing to gate on destroy, before the provider client is configured, or on
	// a supported release.
	if req.Plan.Raw.IsNull() || r.client == nil || r.client.CephVersion.GreaterThanOrEqual(CephVersion_Umbrella) {
		return
	}
	resp.Diagnostics.AddError(
		"radosgw_s3_bucket_ownership_controls requires Ceph 21 (Umbrella)",
		fmt.Sprintf("S3 Object Ownership is only supported on Ceph 21 (Umbrella) or higher, but the provider targets "+
			"%s. Set the provider `ceph_version` to `21`/`umbrella` on an Umbrella cluster to use this resource.",
			r.client.CephVersion),
	)
}

// =============================================================================
// CRUD
// =============================================================================

func (r *S3BucketOwnershipControlsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan S3BucketOwnershipControlsModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if diags := r.putOwnershipControls(ctx, plan); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	tflog.Trace(ctx, "Created S3 bucket ownership controls", map[string]any{"bucket": plan.Bucket.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *S3BucketOwnershipControlsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state S3BucketOwnershipControlsModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucket := state.Bucket.ValueString()

	output, err := r.client.S3.GetBucketOwnershipControls(ctx, &s3.GetBucketOwnershipControlsInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		if isS3OwnershipControlsNotFound(err) || isS3BucketNotFound(err) {
			tflog.Info(ctx, "S3 bucket ownership controls not found, removing from state", map[string]any{"bucket": bucket})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading S3 Bucket Ownership Controls",
			fmt.Sprintf("Could not read ownership controls for bucket %s: %s", bucket, err),
		)
		return
	}

	resp.Diagnostics.Append(flattenOwnershipControls(ctx, output.OwnershipControls, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *S3BucketOwnershipControlsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan S3BucketOwnershipControlsModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if diags := r.putOwnershipControls(ctx, plan); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	tflog.Debug(ctx, "Updated S3 bucket ownership controls", map[string]any{"bucket": plan.Bucket.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *S3BucketOwnershipControlsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state S3BucketOwnershipControlsModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucket := state.Bucket.ValueString()

	_, err := r.client.S3.DeleteBucketOwnershipControls(ctx, &s3.DeleteBucketOwnershipControlsInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		if isS3OwnershipControlsNotFound(err) || isS3BucketNotFound(err) {
			return
		}
		resp.Diagnostics.AddError(
			"Error Deleting S3 Bucket Ownership Controls",
			fmt.Sprintf("Could not delete ownership controls for bucket %s: %s", bucket, err),
		)
		return
	}

	tflog.Trace(ctx, "Deleted S3 bucket ownership controls", map[string]any{"bucket": bucket})
}

func (r *S3BucketOwnershipControlsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("bucket"), req, resp)
}

// =============================================================================
// Helpers
// =============================================================================

// putOwnershipControls builds and applies the ownership controls for the model.
func (r *S3BucketOwnershipControlsResource) putOwnershipControls(ctx context.Context, plan S3BucketOwnershipControlsModel) diag.Diagnostics {
	var diags diag.Diagnostics

	var rules []ownershipControlsRuleModel
	diags.Append(plan.Rule.ElementsAs(ctx, &rules, false)...)
	if diags.HasError() {
		return diags
	}

	apiRules := make([]s3types.OwnershipControlsRule, 0, len(rules))
	for _, rule := range rules {
		apiRules = append(apiRules, s3types.OwnershipControlsRule{
			ObjectOwnership: s3types.ObjectOwnership(rule.ObjectOwnership.ValueString()),
		})
	}

	_, err := r.client.S3.PutBucketOwnershipControls(ctx, &s3.PutBucketOwnershipControlsInput{
		Bucket:            aws.String(plan.Bucket.ValueString()),
		OwnershipControls: &s3types.OwnershipControls{Rules: apiRules},
	})
	if err != nil {
		diags.AddError(
			"Error Applying S3 Bucket Ownership Controls",
			fmt.Sprintf("Could not set ownership controls for bucket %s: %s", plan.Bucket.ValueString(), err),
		)
	}
	return diags
}

// flattenOwnershipControls maps the API response into the model's rule list.
func flattenOwnershipControls(ctx context.Context, controls *s3types.OwnershipControls, state *S3BucketOwnershipControlsModel) diag.Diagnostics {
	var diags diag.Diagnostics

	ruleModels := make([]ownershipControlsRuleModel, 0)
	if controls != nil {
		for _, rule := range controls.Rules {
			ruleModels = append(ruleModels, ownershipControlsRuleModel{
				ObjectOwnership: types.StringValue(string(rule.ObjectOwnership)),
			})
		}
	}

	ruleList, d := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: ownershipRuleAttrTypes()}, ruleModels)
	diags.Append(d...)
	state.Rule = ruleList
	return diags
}

// isS3OwnershipControlsNotFound reports whether the S3 API responded with
// OwnershipControlsNotFoundError — the bucket exists but has no ownership controls.
func isS3OwnershipControlsNotFound(err error) bool {
	return isS3ErrorCode(err, "OwnershipControlsNotFoundError")
}
