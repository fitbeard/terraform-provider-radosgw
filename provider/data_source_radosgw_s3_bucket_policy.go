package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &BucketPolicyDataSource{}

func NewS3BucketPolicyDataSource() datasource.DataSource {
	return &BucketPolicyDataSource{}
}

// BucketPolicyDataSource retrieves the policy attached to an S3 bucket.
type BucketPolicyDataSource struct {
	client *RadosgwClient
}

// BucketPolicyDataSourceModel describes the data source data model.
type BucketPolicyDataSourceModel struct {
	Bucket types.String `tfsdk:"bucket"`
	Policy types.String `tfsdk:"policy"`
	ID     types.String `tfsdk:"id"`
}

func (d *BucketPolicyDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_s3_bucket_policy"
}

func (d *BucketPolicyDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves the IAM policy document attached to an S3 bucket in RadosGW.",

		Attributes: map[string]schema.Attribute{
			"bucket": schema.StringAttribute{
				MarkdownDescription: "The name of the bucket to retrieve the policy for.",
				Required:            true,
			},
			"policy": schema.StringAttribute{
				MarkdownDescription: "The IAM bucket policy document in JSON format.",
				Computed:            true,
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "The bucket name (same as `bucket`).",
				Computed:            true,
			},
		},
	}
}

func (d *BucketPolicyDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*RadosgwClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *RadosgwClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *BucketPolicyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config BucketPolicyDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucket := config.Bucket.ValueString()

	tflog.Debug(ctx, "Reading S3 bucket policy", map[string]any{
		"bucket": bucket,
	})

	// Get the bucket policy using S3 API
	output, err := d.client.S3.GetBucketPolicy(ctx, &s3.GetBucketPolicyInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		var apiErr smithy.APIError
		if ok := errors.As(err, &apiErr); ok {
			if apiErr.ErrorCode() == "NoSuchBucketPolicy" {
				resp.Diagnostics.AddError(
					"Bucket Policy Not Found",
					fmt.Sprintf("No policy is attached to bucket %q.", bucket),
				)
				return
			}
			if apiErr.ErrorCode() == "NoSuchBucket" {
				resp.Diagnostics.AddError(
					"Bucket Not Found",
					fmt.Sprintf("Bucket %q does not exist.", bucket),
				)
				return
			}
		}
		resp.Diagnostics.AddError(
			"Error Reading Bucket Policy",
			fmt.Sprintf("Could not read bucket policy for bucket %q: %s", bucket, err.Error()),
		)
		return
	}

	if output.Policy == nil || *output.Policy == "" {
		resp.Diagnostics.AddError(
			"Bucket Policy Not Found",
			fmt.Sprintf("No policy is attached to bucket %q.", bucket),
		)
		return
	}

	tflog.Debug(ctx, "Found bucket policy", map[string]any{
		"bucket": bucket,
	})

	// Normalize the policy JSON for consistent output
	normalizedPolicy, err := normalizeJSONString(*output.Policy)
	if err != nil {
		// If we can't normalize, use the raw policy
		normalizedPolicy = *output.Policy
	}

	config.Policy = types.StringValue(normalizedPolicy)
	config.ID = types.StringValue(bucket)

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
