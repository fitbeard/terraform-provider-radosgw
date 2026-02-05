package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ceph/go-ceph/rgw/admin"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &QuotaResource{}
var _ resource.ResourceWithImportState = &QuotaResource{}

func NewIAMQuotaResource() resource.Resource {
	return &QuotaResource{}
}

// QuotaResource manages user-level quotas in RadosGW.
// This resource configures quotas that apply to a user, NOT individual buckets.
// There are two types of user-level quotas:
//   - "user" quota: Limits the total storage/objects for the user across ALL their buckets
//   - "bucket" quota: Sets a per-bucket limit that applies to EACH bucket owned by the user
//
// Note: This is NOT for managing quotas on individual buckets.
type QuotaResource struct {
	client *RadosgwClient
}

// QuotaResourceModel describes the resource data model for user-level quotas.
type QuotaResourceModel struct {
	UserID     types.String `tfsdk:"user_id"`
	Type       types.String `tfsdk:"type"`
	Enabled    types.Bool   `tfsdk:"enabled"`
	MaxSize    types.Int64  `tfsdk:"max_size"`
	MaxObjects types.Int64  `tfsdk:"max_objects"`
}

func (r *QuotaResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iam_quota"
}

func (r *QuotaResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Manages user-level quotas in RadosGW. This resource configures storage and object limits for a user.

**Important:** This resource manages quotas at the **user level**, not individual bucket quotas. There are two types:

- **User quota** (` + "`type = \"user\"`" + `): Sets the total storage limit across ALL buckets owned by the user. When exceeded, the user cannot store more data in any of their buckets.

- **Bucket quota** (` + "`type = \"bucket\"`" + `): Sets a per-bucket limit that applies to EACH bucket owned by the user. Every bucket the user owns will have this same quota applied.

Upon deletion, the quota is disabled (not removed, as quotas are properties of users).

**Note:** Account-level quotas are not yet supported by the go-ceph library. Only user-level quotas are currently available.`,

		Attributes: map[string]schema.Attribute{
			"user_id": schema.StringAttribute{
				MarkdownDescription: "The user ID to configure quotas for.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "The quota type:\n" +
					"  - `user`: Total quota across all user's buckets combined\n" +
					"  - `bucket`: Per-bucket quota applied to each bucket the user owns",
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf("user", "bucket"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the quota is enabled. Default: `true`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"max_size": schema.Int64Attribute{
				MarkdownDescription: "Maximum size in bytes. Use `-1` for unlimited. Default: `-1`.",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(-1),
			},
			"max_objects": schema.Int64Attribute{
				MarkdownDescription: "Maximum number of objects. Use `-1` for unlimited. Default: `-1`.",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(-1),
			},
		},
	}
}

func (r *QuotaResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *QuotaResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data QuotaResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Build quota spec for the user
	enabled := data.Enabled.ValueBool()
	quota := admin.QuotaSpec{
		UID:       data.UserID.ValueString(),
		QuotaType: data.Type.ValueString(),
		Enabled:   &enabled,
	}

	// Set max_size (default -1 for unlimited)
	if !data.MaxSize.IsNull() {
		maxSize := data.MaxSize.ValueInt64()
		quota.MaxSize = &maxSize
	} else {
		maxSize := int64(-1)
		quota.MaxSize = &maxSize
		data.MaxSize = types.Int64Value(-1)
	}

	// Set max_objects
	if !data.MaxObjects.IsNull() {
		maxObjects := data.MaxObjects.ValueInt64()
		quota.MaxObjects = &maxObjects
	} else {
		maxObjects := int64(-1)
		quota.MaxObjects = &maxObjects
		data.MaxObjects = types.Int64Value(-1)
	}

	// Set user-level quota based on type with retry logic for ConcurrentModification
	// "user" type: Sets total quota for the user across all their buckets
	// "bucket" type: Sets per-bucket quota for all buckets owned by this user
	err := retryOnConcurrentModification(ctx, fmt.Sprintf("SetQuota %s/%s", data.Type.ValueString(), data.UserID.ValueString()), func() error {
		if data.Type.ValueString() == "user" {
			return r.client.Admin.SetUserQuota(ctx, quota)
		}
		return r.client.Admin.SetBucketQuota(ctx, quota)
	})

	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating User Quota",
			fmt.Sprintf("Could not create %s quota for user %s: %s", data.Type.ValueString(), data.UserID.ValueString(), err.Error()),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *QuotaResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data QuotaResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Prepare request to get user's quota
	reqQuotaSpec := admin.QuotaSpec{
		UID: data.UserID.ValueString(),
	}

	// Get user-level quota based on type
	var err error
	var quotaSpec admin.QuotaSpec
	if data.Type.ValueString() == "user" {
		quotaSpec, err = r.client.Admin.GetUserQuota(ctx, reqQuotaSpec)
	} else {
		quotaSpec, err = r.client.Admin.GetBucketQuota(ctx, reqQuotaSpec)
	}

	if err != nil {
		if errors.Is(err, admin.ErrNoSuchUser) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading User Quota",
			fmt.Sprintf("Could not read %s quota for user %s: %s", data.Type.ValueString(), data.UserID.ValueString(), err.Error()),
		)
		return
	}

	// Update state from response
	if quotaSpec.Enabled != nil {
		data.Enabled = types.BoolValue(*quotaSpec.Enabled)
	}

	// Set max_size from response
	if quotaSpec.MaxSize != nil {
		data.MaxSize = types.Int64Value(*quotaSpec.MaxSize)
	} else {
		data.MaxSize = types.Int64Value(-1)
	}

	if quotaSpec.MaxObjects != nil {
		data.MaxObjects = types.Int64Value(*quotaSpec.MaxObjects)
	} else {
		data.MaxObjects = types.Int64Value(-1)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *QuotaResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data QuotaResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Build quota spec for the user
	enabled := data.Enabled.ValueBool()
	quota := admin.QuotaSpec{
		UID:       data.UserID.ValueString(),
		QuotaType: data.Type.ValueString(),
		Enabled:   &enabled,
	}

	// Set max_size (default -1 for unlimited)
	if !data.MaxSize.IsNull() {
		maxSize := data.MaxSize.ValueInt64()
		quota.MaxSize = &maxSize
	} else {
		maxSize := int64(-1)
		quota.MaxSize = &maxSize
	}

	// Set max_objects
	if !data.MaxObjects.IsNull() {
		maxObjects := data.MaxObjects.ValueInt64()
		quota.MaxObjects = &maxObjects
	} else {
		maxObjects := int64(-1)
		quota.MaxObjects = &maxObjects
	}

	// Update user-level quota based on type with retry logic for ConcurrentModification
	err := retryOnConcurrentModification(ctx, fmt.Sprintf("UpdateQuota %s/%s", data.Type.ValueString(), data.UserID.ValueString()), func() error {
		if data.Type.ValueString() == "user" {
			return r.client.Admin.SetUserQuota(ctx, quota)
		}
		return r.client.Admin.SetBucketQuota(ctx, quota)
	})

	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating User Quota",
			fmt.Sprintf("Could not update %s quota for user %s: %s", data.Type.ValueString(), data.UserID.ValueString(), err.Error()),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *QuotaResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data QuotaResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Disable quota on delete (quotas cannot be removed, only disabled)
	// This resets the user's quota to unlimited and disabled state
	enabled := false
	maxSize := int64(-1)
	maxObjects := int64(-1)

	quota := admin.QuotaSpec{
		UID:        data.UserID.ValueString(),
		QuotaType:  data.Type.ValueString(),
		Enabled:    &enabled,
		MaxSize:    &maxSize,
		MaxObjects: &maxObjects,
	}

	// Disable user-level quota based on type with retry logic for ConcurrentModification
	err := retryOnConcurrentModification(ctx, fmt.Sprintf("DeleteQuota %s/%s", data.Type.ValueString(), data.UserID.ValueString()), func() error {
		if data.Type.ValueString() == "user" {
			return r.client.Admin.SetUserQuota(ctx, quota)
		}
		return r.client.Admin.SetBucketQuota(ctx, quota)
	})

	if err != nil && !errors.Is(err, admin.ErrNoSuchUser) {
		resp.Diagnostics.AddError(
			"Error Deleting User Quota",
			fmt.Sprintf("Could not disable %s quota for user %s: %s", data.Type.ValueString(), data.UserID.ValueString(), err.Error()),
		)
		return
	}
}

func (r *QuotaResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import format: "user_id:type" where type is "user" or "bucket"
	// Example: "myuser:user" imports the user's total quota across all buckets
	// Example: "myuser:bucket" imports the user's per-bucket quota setting
	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Import ID must be in the format 'user_id:type'. Example: 'myuser:user' or 'myuser:bucket'",
		)
		return
	}

	userID := parts[0]
	quotaType := parts[1]

	if quotaType != "user" && quotaType != "bucket" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Quota type must be 'user' or 'bucket', got: %s", quotaType),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("user_id"), userID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("type"), quotaType)...)
}
