package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ceph/go-ceph/rgw/admin"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &UserResource{}
var _ resource.ResourceWithImportState = &UserResource{}

func NewIAMUserResource() resource.Resource {
	return &UserResource{}
}

// UserResource defines the resource implementation.
type UserResource struct {
	client *RadosgwClient
}

// UserResourceModel describes the resource data model.
type UserResourceModel struct {
	UserID              types.String `tfsdk:"user_id"`
	DisplayName         types.String `tfsdk:"display_name"`
	Email               types.String `tfsdk:"email"`
	Tenant              types.String `tfsdk:"tenant"`
	MaxBuckets          types.Int64  `tfsdk:"max_buckets"`
	Suspended           types.Bool   `tfsdk:"suspended"`
	OpMask              types.String `tfsdk:"op_mask"`
	DefaultPlacement    types.String `tfsdk:"default_placement"`
	DefaultStorageClass types.String `tfsdk:"default_storage_class"`
	Type                types.String `tfsdk:"type"`
}

func (r *UserResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iam_user"
}

func (r *UserResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a RadosGW user.",

		Attributes: map[string]schema.Attribute{
			"user_id": schema.StringAttribute{
				MarkdownDescription: "The user ID.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "The display name of the user.",
				Required:            true,
			},
			"email": schema.StringAttribute{
				MarkdownDescription: "The email address of the user. Note: Once set, this field cannot be cleared, only changed to a different value.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"tenant": schema.StringAttribute{
				MarkdownDescription: "The tenant to which the user belongs. Cannot be modified after creation.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"max_buckets": schema.Int64Attribute{
				MarkdownDescription: "The maximum number of buckets the user can own. Default is 1000.",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(1000),
			},
			"suspended": schema.BoolAttribute{
				MarkdownDescription: "Whether the user is suspended. Default is false.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"op_mask": schema.StringAttribute{
				MarkdownDescription: "The operation mask for the user. Default is 'read, write, delete'.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("read, write, delete"),
			},
			"default_placement": schema.StringAttribute{
				MarkdownDescription: "The default placement for the user's buckets. Note: Once set, this field cannot be cleared, only changed to a different value.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"default_storage_class": schema.StringAttribute{
				MarkdownDescription: "The default storage class for the user's objects.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "The user type (e.g., 'rgw', 'ldap').",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *UserResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *UserResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data UserResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Creating RadosGW user", map[string]any{
		"user_id": data.UserID.ValueString(),
	})

	// Prepare user creation parameters
	maxBuckets := int(data.MaxBuckets.ValueInt64())
	suspended := 0
	if data.Suspended.ValueBool() {
		suspended = 1
	}
	generateKey := false

	userConfig := admin.User{
		ID:               data.UserID.ValueString(),
		DisplayName:      data.DisplayName.ValueString(),
		Email:            data.Email.ValueString(),
		Tenant:           data.Tenant.ValueString(),
		MaxBuckets:       &maxBuckets,
		Suspended:        &suspended,
		OpMask:           data.OpMask.ValueString(),
		DefaultPlacement: data.DefaultPlacement.ValueString(),
		GenerateKey:      &generateKey,
	}

	// Create user with retry logic for ConcurrentModification
	var user admin.User
	err := retryOnConcurrentModification(ctx, fmt.Sprintf("CreateUser %s", data.UserID.ValueString()), func() error {
		var createErr error
		user, createErr = r.client.Admin.CreateUser(ctx, userConfig)
		return createErr
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating RadosGW User",
			"Could not create user, unexpected error: "+err.Error(),
		)
		return
	}

	// Update state with created user data
	data.UserID = types.StringValue(user.ID)
	data.DisplayName = types.StringValue(user.DisplayName)
	data.Email = types.StringValue(user.Email)
	data.Tenant = types.StringValue(user.Tenant)
	data.MaxBuckets = types.Int64Value(int64(*user.MaxBuckets))
	data.Suspended = types.BoolValue(*user.Suspended != 0)
	data.OpMask = types.StringValue(user.OpMask)
	data.DefaultPlacement = types.StringValue(user.DefaultPlacement)
	data.DefaultStorageClass = types.StringValue(user.DefaultStorageClass)
	data.Type = types.StringValue(user.Type)

	tflog.Trace(ctx, "Created RadosGW user")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data UserResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Build the full user ID for API calls
	// For tenant users, the format is "tenant$user_id"
	fullUserID := buildFullUserID(data.UserID.ValueString(), data.Tenant.ValueString())

	tflog.Debug(ctx, "Reading RadosGW user", map[string]any{
		"user_id":      data.UserID.ValueString(),
		"tenant":       data.Tenant.ValueString(),
		"full_user_id": fullUserID,
	})

	// Get user info
	user, err := r.client.Admin.GetUser(ctx, admin.User{ID: fullUserID})
	if err != nil {
		// If user doesn't exist, remove from state
		if errors.Is(err, admin.ErrNoSuchUser) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading RadosGW User",
			fmt.Sprintf("Could not read user %s: %s", data.UserID.ValueString(), err.Error()),
		)
		return
	}

	// Update state
	data.UserID = types.StringValue(user.ID)
	data.DisplayName = types.StringValue(user.DisplayName)
	data.Email = types.StringValue(user.Email)
	data.Tenant = types.StringValue(user.Tenant)
	data.MaxBuckets = types.Int64Value(int64(*user.MaxBuckets))
	data.Suspended = types.BoolValue(*user.Suspended != 0)
	data.OpMask = types.StringValue(user.OpMask)
	data.DefaultPlacement = types.StringValue(user.DefaultPlacement)
	data.DefaultStorageClass = types.StringValue(user.DefaultStorageClass)
	data.Type = types.StringValue(user.Type)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data UserResourceModel
	var state UserResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Build the full user ID for API calls
	fullUserID := buildFullUserID(data.UserID.ValueString(), data.Tenant.ValueString())

	tflog.Debug(ctx, "Updating RadosGW user", map[string]any{
		"user_id":      data.UserID.ValueString(),
		"tenant":       data.Tenant.ValueString(),
		"full_user_id": fullUserID,
	})

	// Prepare user modification parameters
	maxBuckets := int(data.MaxBuckets.ValueInt64())
	suspended := 0
	if data.Suspended.ValueBool() {
		suspended = 1
	}

	userConfig := admin.User{
		ID:          fullUserID,
		DisplayName: data.DisplayName.ValueString(),
		MaxBuckets:  &maxBuckets,
		Suspended:   &suspended,
		OpMask:      data.OpMask.ValueString(),
	}

	// Only set Email if provided (can't be cleared once set)
	if !data.Email.IsNull() {
		userConfig.Email = data.Email.ValueString()
	}

	// Only set DefaultPlacement if provided (can't be cleared once set)
	if !data.DefaultPlacement.IsNull() {
		userConfig.DefaultPlacement = data.DefaultPlacement.ValueString()
	}

	// Modify user with retry logic for ConcurrentModification
	var user admin.User
	err := retryOnConcurrentModification(ctx, fmt.Sprintf("ModifyUser %s", data.UserID.ValueString()), func() error {
		var modifyErr error
		user, modifyErr = r.client.Admin.ModifyUser(ctx, userConfig)
		return modifyErr
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating RadosGW User",
			"Could not update user, unexpected error: "+err.Error(),
		)
		return
	}

	// Update state
	data.UserID = types.StringValue(user.ID)
	data.DisplayName = types.StringValue(user.DisplayName)
	data.Email = types.StringValue(user.Email)
	data.Tenant = types.StringValue(user.Tenant)
	data.MaxBuckets = types.Int64Value(int64(*user.MaxBuckets))
	data.Suspended = types.BoolValue(*user.Suspended != 0)
	data.OpMask = types.StringValue(user.OpMask)
	data.DefaultPlacement = types.StringValue(user.DefaultPlacement)
	data.DefaultStorageClass = types.StringValue(user.DefaultStorageClass)
	data.Type = types.StringValue(user.Type)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data UserResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Build the full user ID for API calls
	fullUserID := buildFullUserID(data.UserID.ValueString(), data.Tenant.ValueString())

	tflog.Debug(ctx, "Deleting RadosGW user", map[string]any{
		"user_id":      data.UserID.ValueString(),
		"tenant":       data.Tenant.ValueString(),
		"full_user_id": fullUserID,
	})

	// Delete user with retry logic for ConcurrentModification
	err := retryOnConcurrentModification(ctx, fmt.Sprintf("RemoveUser %s", fullUserID), func() error {
		return r.client.Admin.RemoveUser(ctx, admin.User{ID: fullUserID})
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting RadosGW User",
			"Could not delete user, unexpected error: "+err.Error(),
		)
		return
	}
}

func (r *UserResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import ID can be either "user_id" or "tenant$user_id"
	importID := req.ID

	var userID, tenant string
	if idx := strings.Index(importID, "$"); idx != -1 {
		// Format: tenant$user_id
		tenant = importID[:idx]
		userID = importID[idx+1:]
	} else {
		// Format: user_id (no tenant)
		userID = importID
		tenant = ""
	}

	tflog.Debug(ctx, "Importing RadosGW user", map[string]any{
		"import_id": importID,
		"user_id":   userID,
		"tenant":    tenant,
	})

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("user_id"), userID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("tenant"), tenant)...)
}

// buildFullUserID constructs the full user ID for API calls.
// For tenant users, the format is "tenant$user_id".
// For non-tenant users, it's just "user_id".
func buildFullUserID(userID, tenant string) string {
	if tenant != "" {
		return tenant + "$" + userID
	}
	return userID
}
