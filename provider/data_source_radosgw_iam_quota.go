package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/ceph/go-ceph/rgw/admin"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &QuotaDataSource{}

func NewIAMQuotaDataSource() datasource.DataSource {
	return &QuotaDataSource{}
}

// QuotaDataSource retrieves user-level quota information from RadosGW.
// This data source reads quotas that apply to a user, NOT individual buckets.
// There are two types of user-level quotas:
//   - "user" quota: Total storage/objects limit for the user across ALL their buckets
//   - "bucket" quota: Per-bucket limit that applies to EACH bucket owned by the user
type QuotaDataSource struct {
	client *RadosgwClient
}

// QuotaDataSourceModel describes the data source data model.
type QuotaDataSourceModel struct {
	UserID     types.String `tfsdk:"user_id"`
	Type       types.String `tfsdk:"type"`
	Enabled    types.Bool   `tfsdk:"enabled"`
	MaxSize    types.Int64  `tfsdk:"max_size"`
	MaxObjects types.Int64  `tfsdk:"max_objects"`
	ID         types.String `tfsdk:"id"`
}

func (d *QuotaDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iam_quota"
}

func (d *QuotaDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Retrieves user-level quota information from RadosGW.

**Important:** This data source reads quotas at the **user level**, not individual bucket quotas. There are two types:

- **User quota** (` + "`type = \"user\"`" + `): The total storage limit across ALL buckets owned by the user.

- **Bucket quota** (` + "`type = \"bucket\"`" + `): The per-bucket limit that applies to EACH bucket owned by the user.`,

		Attributes: map[string]schema.Attribute{
			"user_id": schema.StringAttribute{
				MarkdownDescription: "The user ID to retrieve quota for.",
				Required:            true,
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "The quota type to retrieve:\n" +
					"  - `user`: Total quota across all user's buckets combined\n" +
					"  - `bucket`: Per-bucket quota applied to each bucket the user owns",
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf("user", "bucket"),
				},
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the quota is enabled.",
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
			"id": schema.StringAttribute{
				MarkdownDescription: "The data source identifier in format `user_id:type`.",
				Computed:            true,
			},
		},
	}
}

func (d *QuotaDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *QuotaDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config QuotaDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	userID := config.UserID.ValueString()
	quotaType := config.Type.ValueString()

	tflog.Debug(ctx, "Reading RadosGW user quota", map[string]any{
		"user_id": userID,
		"type":    quotaType,
	})

	// Prepare request to get user's quota
	reqQuotaSpec := admin.QuotaSpec{
		UID: userID,
	}

	// Get user-level quota based on type
	var err error
	var quotaSpec admin.QuotaSpec
	if quotaType == "user" {
		quotaSpec, err = d.client.Admin.GetUserQuota(ctx, reqQuotaSpec)
	} else {
		quotaSpec, err = d.client.Admin.GetBucketQuota(ctx, reqQuotaSpec)
	}

	if err != nil {
		if errors.Is(err, admin.ErrNoSuchUser) {
			resp.Diagnostics.AddError(
				"User Not Found",
				fmt.Sprintf("User %q does not exist.", userID),
			)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading User Quota",
			fmt.Sprintf("Could not read %s quota for user %q: %s", quotaType, userID, err.Error()),
		)
		return
	}

	tflog.Debug(ctx, "Found user quota", map[string]any{
		"user_id":     userID,
		"type":        quotaType,
		"enabled":     quotaSpec.Enabled,
		"max_size":    quotaSpec.MaxSize,
		"max_objects": quotaSpec.MaxObjects,
	})

	// Set values from response
	if quotaSpec.Enabled != nil {
		config.Enabled = types.BoolValue(*quotaSpec.Enabled)
	} else {
		config.Enabled = types.BoolValue(false)
	}

	if quotaSpec.MaxSize != nil {
		config.MaxSize = types.Int64Value(*quotaSpec.MaxSize)
	} else {
		config.MaxSize = types.Int64Value(-1)
	}

	if quotaSpec.MaxObjects != nil {
		config.MaxObjects = types.Int64Value(*quotaSpec.MaxObjects)
	} else {
		config.MaxObjects = types.Int64Value(-1)
	}

	config.ID = types.StringValue(fmt.Sprintf("%s:%s", userID, quotaType))

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
