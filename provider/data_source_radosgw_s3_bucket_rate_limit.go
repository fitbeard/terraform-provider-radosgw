package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &S3BucketRateLimitDataSource{}

func NewS3BucketRateLimitDataSource() datasource.DataSource {
	return &S3BucketRateLimitDataSource{}
}

// S3BucketRateLimitDataSource retrieves the rate limit of an S3 bucket via the
// Admin Ops API.
type S3BucketRateLimitDataSource struct {
	client *RadosgwClient
}

// S3BucketRateLimitDataSourceModel describes the data source data model. It
// mirrors the resource model; all rate-limit settings are computed.
type S3BucketRateLimitDataSourceModel struct {
	// Input
	Bucket types.String `tfsdk:"bucket"`
	Tenant types.String `tfsdk:"tenant"`

	// Computed
	Enabled       types.Bool  `tfsdk:"enabled"`
	MaxReadOps    types.Int64 `tfsdk:"max_read_ops"`
	MaxWriteOps   types.Int64 `tfsdk:"max_write_ops"`
	MaxReadBytes  types.Int64 `tfsdk:"max_read_bytes"`
	MaxWriteBytes types.Int64 `tfsdk:"max_write_bytes"`
}

func (d *S3BucketRateLimitDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_s3_bucket_rate_limit"
}

func (d *S3BucketRateLimitDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves the rate limit configured for an S3 bucket in RadosGW via the Admin Ops API " +
			"(`ratelimit-scope=bucket`).\n\n" +
			"~> **Admin capability required.** Reading a bucket rate limit requires the Admin Ops `ratelimit` " +
			"capability (`ratelimit=read`). A user without it receives `AccessDenied`.\n\n" +
			"~> **Ceph version:** `max_list_ops` and `max_delete_ops` were added in Ceph 21 and are not exposed yet " +
			"(the latest stable release is Tentacle, 20).",

		Attributes: map[string]schema.Attribute{
			"bucket": schema.StringAttribute{
				MarkdownDescription: "The name of the bucket to look up.",
				Required:            true,
			},
			"tenant": schema.StringAttribute{
				MarkdownDescription: "The tenant that owns the bucket. Leave unset for a bucket in the default " +
					"namespace. When set, the bucket is addressed as `tenant/bucket`.",
				Optional: true,
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the rate limit is enabled for the bucket.",
				Computed:            true,
			},
			"max_read_ops": schema.Int64Attribute{
				MarkdownDescription: "Maximum read operations per minute. `0` means the limit is disabled.",
				Computed:            true,
			},
			"max_write_ops": schema.Int64Attribute{
				MarkdownDescription: "Maximum write operations per minute. `0` means the limit is disabled.",
				Computed:            true,
			},
			"max_read_bytes": schema.Int64Attribute{
				MarkdownDescription: "Maximum read bytes per minute. `0` means the limit is disabled.",
				Computed:            true,
			},
			"max_write_bytes": schema.Int64Attribute{
				MarkdownDescription: "Maximum write bytes per minute. `0` means the limit is disabled.",
				Computed:            true,
			},
		},
	}
}

func (d *S3BucketRateLimitDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *S3BucketRateLimitDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config S3BucketRateLimitDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucketID := rateLimitBucketID(config.Tenant.ValueString(), config.Bucket.ValueString())

	tflog.Debug(ctx, "Reading RadosGW bucket rate limit", map[string]any{"bucket": bucketID})

	spec, err := d.client.Admin.GetIndividualBucketRateLimit(ctx, bucketID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Bucket Rate Limit",
			rateLimitErrorDetail("read rate limit for", bucketID, err),
		)
		return
	}

	if spec.Enabled != nil {
		config.Enabled = types.BoolValue(*spec.Enabled)
	} else {
		config.Enabled = types.BoolValue(false)
	}
	config.MaxReadOps = int64FromPtr(spec.MaxReadOps)
	config.MaxWriteOps = int64FromPtr(spec.MaxWriteOps)
	config.MaxReadBytes = int64FromPtr(spec.MaxReadBytes)
	config.MaxWriteBytes = int64FromPtr(spec.MaxWriteBytes)

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
