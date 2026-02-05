package provider

import (
	"context"
	"fmt"

	"github.com/ceph/go-ceph/rgw/admin"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &BucketDataSource{}

func NewS3BucketDataSource() datasource.DataSource {
	return &BucketDataSource{}
}

// BucketDataSource retrieves information about an S3 bucket in RadosGW.
type BucketDataSource struct {
	client *RadosgwClient
}

// BucketDataSourceModel describes the data source data model.
type BucketDataSourceModel struct {
	// Input
	Bucket types.String `tfsdk:"bucket"`

	// Computed attributes from Admin API
	ID                types.String `tfsdk:"id"`
	Owner             types.String `tfsdk:"owner"`
	Tenant            types.String `tfsdk:"tenant"`
	Versioning        types.String `tfsdk:"versioning"`
	ObjectLockEnabled types.Bool   `tfsdk:"object_lock_enabled"`
	CreationTime      types.String `tfsdk:"creation_time"`
	PlacementRule     types.String `tfsdk:"placement_rule"`
	Zonegroup         types.String `tfsdk:"zonegroup"`
	NumShards         types.Int64  `tfsdk:"num_shards"`
	Marker            types.String `tfsdk:"marker"`
	IndexType         types.String `tfsdk:"index_type"`
	ExplicitPlacement types.Object `tfsdk:"explicit_placement"`
	BucketQuota       types.Object `tfsdk:"bucket_quota"`
}

func (d *BucketDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_s3_bucket"
}

func (d *BucketDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves information about an existing S3 bucket in RadosGW using the Admin API.",

		Attributes: map[string]schema.Attribute{
			"bucket": schema.StringAttribute{
				MarkdownDescription: "The name of the bucket to look up.",
				Required:            true,
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the bucket assigned by RadosGW.",
				Computed:            true,
			},
			"owner": schema.StringAttribute{
				MarkdownDescription: "The user ID of the bucket owner.",
				Computed:            true,
			},
			"tenant": schema.StringAttribute{
				MarkdownDescription: "The tenant the bucket belongs to.",
				Computed:            true,
			},
			"versioning": schema.StringAttribute{
				MarkdownDescription: "The versioning state of the bucket: `off`, `enabled`, or `suspended`.",
				Computed:            true,
			},
			"object_lock_enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether S3 Object Lock is enabled for the bucket.",
				Computed:            true,
			},
			"creation_time": schema.StringAttribute{
				MarkdownDescription: "The creation time of the bucket in RFC3339 format.",
				Computed:            true,
			},
			"placement_rule": schema.StringAttribute{
				MarkdownDescription: "The placement rule for the bucket, determining which pools store the bucket's data.",
				Computed:            true,
			},
			"zonegroup": schema.StringAttribute{
				MarkdownDescription: "The zonegroup ID where the bucket is located.",
				Computed:            true,
			},
			"num_shards": schema.Int64Attribute{
				MarkdownDescription: "The number of shards for the bucket index.",
				Computed:            true,
			},
			"marker": schema.StringAttribute{
				MarkdownDescription: "The internal bucket marker used by RadosGW.",
				Computed:            true,
			},
			"index_type": schema.StringAttribute{
				MarkdownDescription: "The type of bucket index (e.g., 'Normal').",
				Computed:            true,
			},
			"explicit_placement": schema.SingleNestedAttribute{
				MarkdownDescription: "Explicit placement configuration showing the RADOS pools used for the bucket.",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"data_pool": schema.StringAttribute{
						MarkdownDescription: "The RADOS pool for storing object data.",
						Computed:            true,
					},
					"data_extra_pool": schema.StringAttribute{
						MarkdownDescription: "The RADOS pool for storing extra object data (e.g., multipart uploads).",
						Computed:            true,
					},
					"index_pool": schema.StringAttribute{
						MarkdownDescription: "The RADOS pool for storing the bucket index.",
						Computed:            true,
					},
				},
			},
			"bucket_quota": schema.SingleNestedAttribute{
				MarkdownDescription: "Quota settings for this specific bucket.",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"enabled": schema.BoolAttribute{
						MarkdownDescription: "Whether the bucket quota is enabled.",
						Computed:            true,
					},
					"max_size": schema.Int64Attribute{
						MarkdownDescription: "Maximum size in bytes. `-1` means unlimited.",
						Computed:            true,
					},
					"max_objects": schema.Int64Attribute{
						MarkdownDescription: "Maximum number of objects. `-1` means unlimited.",
						Computed:            true,
					},
				},
			},
		},
	}
}

func (d *BucketDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *BucketDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config BucketDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucketName := config.Bucket.ValueString()

	tflog.Debug(ctx, "Reading RadosGW bucket", map[string]any{
		"bucket": bucketName,
	})

	// Get bucket info from Admin API
	bucketInfo, err := d.client.Admin.GetBucketInfo(ctx, admin.Bucket{Bucket: bucketName})
	if err != nil {
		if isBucketNotFoundError(err) {
			resp.Diagnostics.AddError(
				"Bucket Not Found",
				fmt.Sprintf("Bucket %q does not exist.", bucketName),
			)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading Bucket",
			fmt.Sprintf("Could not read bucket %q: %s", bucketName, err.Error()),
		)
		return
	}

	tflog.Debug(ctx, "Found bucket", map[string]any{
		"bucket": bucketName,
		"id":     bucketInfo.ID,
		"owner":  bucketInfo.Owner,
	})

	// Populate model from bucket info
	d.populateModelFromBucketInfo(ctx, &config, &bucketInfo)

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

// populateModelFromBucketInfo updates the model with data from Admin API bucket info.
func (d *BucketDataSource) populateModelFromBucketInfo(ctx context.Context, data *BucketDataSourceModel, info *admin.Bucket) {
	data.ID = types.StringValue(info.ID)
	data.Owner = types.StringValue(info.Owner)
	data.Tenant = types.StringValue(info.Tenant)
	data.PlacementRule = types.StringValue(info.PlacementRule)
	data.Zonegroup = types.StringValue(info.Zonegroup)
	data.Marker = types.StringValue(info.Marker)
	data.IndexType = types.StringValue(info.IndexType)
	data.ObjectLockEnabled = types.BoolValue(info.ObjectLockEnabled)

	// Handle versioning - map API response to our schema values
	if info.Versioning != nil {
		switch *info.Versioning {
		case "Enabled", "enabled":
			data.Versioning = types.StringValue("enabled")
		case "Suspended", "suspended":
			data.Versioning = types.StringValue("suspended")
		default:
			data.Versioning = types.StringValue("off")
		}
	} else {
		data.Versioning = types.StringValue("off")
	}

	// Handle num_shards
	if info.NumShards != nil {
		data.NumShards = types.Int64Value(int64(*info.NumShards))
	} else {
		data.NumShards = types.Int64Null()
	}

	// Handle creation time
	if info.CreationTime != nil {
		data.CreationTime = types.StringValue(info.CreationTime.Format("2006-01-02T15:04:05Z07:00"))
	} else {
		data.CreationTime = types.StringNull()
	}

	// Build explicit_placement object
	placementObj, diags := types.ObjectValue(explicitPlacementAttrTypes(), map[string]attr.Value{
		"data_pool":       types.StringValue(info.ExplicitPlacement.DataPool),
		"data_extra_pool": types.StringValue(info.ExplicitPlacement.DataExtraPool),
		"index_pool":      types.StringValue(info.ExplicitPlacement.IndexPool),
	})
	if diags.HasError() {
		tflog.Warn(ctx, "Could not build explicit_placement object")
		data.ExplicitPlacement = types.ObjectNull(explicitPlacementAttrTypes())
	} else {
		data.ExplicitPlacement = placementObj
	}

	// Build bucket_quota object
	quotaValues := map[string]attr.Value{
		"enabled":     types.BoolNull(),
		"max_size":    types.Int64Null(),
		"max_objects": types.Int64Null(),
	}
	if info.BucketQuota.Enabled != nil {
		quotaValues["enabled"] = types.BoolValue(*info.BucketQuota.Enabled)
	}
	if info.BucketQuota.MaxSize != nil {
		quotaValues["max_size"] = types.Int64Value(*info.BucketQuota.MaxSize)
	}
	if info.BucketQuota.MaxObjects != nil {
		quotaValues["max_objects"] = types.Int64Value(*info.BucketQuota.MaxObjects)
	}

	quotaObj, diags := types.ObjectValue(bucketQuotaAttrTypes(), quotaValues)
	if diags.HasError() {
		tflog.Warn(ctx, "Could not build bucket_quota object")
		data.BucketQuota = types.ObjectNull(bucketQuotaAttrTypes())
	} else {
		data.BucketQuota = quotaObj
	}
}
