package provider

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &S3BucketPublicAccessBlockDataSource{}

func NewS3BucketPublicAccessBlockDataSource() datasource.DataSource {
	return &S3BucketPublicAccessBlockDataSource{}
}

// S3BucketPublicAccessBlockDataSource reads a bucket's public access block.
type S3BucketPublicAccessBlockDataSource struct {
	client *RadosgwClient
}

type S3BucketPublicAccessBlockDataSourceModel struct {
	Bucket                types.String `tfsdk:"bucket"`
	BlockPublicAcls       types.Bool   `tfsdk:"block_public_acls"`
	BlockPublicPolicy     types.Bool   `tfsdk:"block_public_policy"`
	IgnorePublicAcls      types.Bool   `tfsdk:"ignore_public_acls"`
	RestrictPublicBuckets types.Bool   `tfsdk:"restrict_public_buckets"`
}

func (d *S3BucketPublicAccessBlockDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_s3_bucket_public_access_block"
}

func (d *S3BucketPublicAccessBlockDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads the S3 Public Access Block configuration of a bucket in RadosGW, via the " +
			"standard S3 `GetPublicAccessBlock` API.\n\n" +
			"~> **Note:** When a bucket has no public-access-block configuration, all four settings are reported as " +
			"`false` — the effective state, in which nothing is blocked. See " +
			"[`radosgw_s3_bucket_public_access_block`](../resources/s3_bucket_public_access_block) for the per-setting " +
			"Ceph version support.",

		Attributes: map[string]schema.Attribute{
			"bucket": schema.StringAttribute{
				MarkdownDescription: "The name of the bucket.",
				Required:            true,
			},
			"block_public_acls": schema.BoolAttribute{
				MarkdownDescription: "Whether RadosGW blocks public ACLs for this bucket.",
				Computed:            true,
			},
			"block_public_policy": schema.BoolAttribute{
				MarkdownDescription: "Whether RadosGW blocks public bucket policies for this bucket.",
				Computed:            true,
			},
			"ignore_public_acls": schema.BoolAttribute{
				MarkdownDescription: "Whether RadosGW ignores public ACLs for this bucket.",
				Computed:            true,
			},
			"restrict_public_buckets": schema.BoolAttribute{
				MarkdownDescription: "Whether RadosGW restricts public bucket policies for this bucket.",
				Computed:            true,
			},
		},
	}
}

func (d *S3BucketPublicAccessBlockDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *S3BucketPublicAccessBlockDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config S3BucketPublicAccessBlockDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucket := config.Bucket.ValueString()

	tflog.Debug(ctx, "Reading S3 bucket public access block", map[string]any{"bucket": bucket})

	output, err := d.client.S3.GetPublicAccessBlock(ctx, &s3.GetPublicAccessBlockInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		if isS3NoSuchPublicAccessBlock(err) {
			// No configuration: report the effective state (nothing blocked).
			config.BlockPublicAcls = types.BoolValue(false)
			config.BlockPublicPolicy = types.BoolValue(false)
			config.IgnorePublicAcls = types.BoolValue(false)
			config.RestrictPublicBuckets = types.BoolValue(false)
			resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
			return
		}
		if isS3BucketNotFound(err) {
			resp.Diagnostics.AddError(
				"Bucket Not Found",
				fmt.Sprintf("Bucket %q does not exist.", bucket),
			)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading S3 Bucket Public Access Block",
			fmt.Sprintf("Could not read public access block for bucket %q: %s", bucket, err),
		)
		return
	}

	cfg := output.PublicAccessBlockConfiguration
	if cfg == nil {
		cfg = &s3types.PublicAccessBlockConfiguration{}
	}
	config.BlockPublicAcls = types.BoolValue(aws.ToBool(cfg.BlockPublicAcls))
	config.BlockPublicPolicy = types.BoolValue(aws.ToBool(cfg.BlockPublicPolicy))
	config.IgnorePublicAcls = types.BoolValue(aws.ToBool(cfg.IgnorePublicAcls))
	config.RestrictPublicBuckets = types.BoolValue(aws.ToBool(cfg.RestrictPublicBuckets))

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
