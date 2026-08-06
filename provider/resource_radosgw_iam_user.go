package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"

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
var (
	_ resource.Resource                   = &UserResource{}
	_ resource.ResourceWithImportState    = &UserResource{}
	_ resource.ResourceWithValidateConfig = &UserResource{}
)

func NewIAMUserResource() resource.Resource {
	return &UserResource{}
}

// typeUseStateUnlessAccountRootChanges keeps the computed `type` value stable
// across plans (like UseStateForUnknown), except when `account_root` is changing.
// Promoting or demoting an account user flips its type between "rgw" and "root",
// so in that case `type` must be recomputed rather than assumed unchanged
// (otherwise the plan promises the old type and apply produces a different one).
type typeUseStateUnlessAccountRootChanges struct{}

func (typeUseStateUnlessAccountRootChanges) Description(context.Context) string {
	return "Use prior state for type unless account_root is changing."
}

func (typeUseStateUnlessAccountRootChanges) MarkdownDescription(context.Context) string {
	return "Use prior state for `type` unless `account_root` is changing."
}

func (typeUseStateUnlessAccountRootChanges) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	// No prior state on create; no plan on destroy.
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}
	// Only fill in an unknown planned value.
	if !req.PlanValue.IsUnknown() {
		return
	}

	var stateRoot, planRoot types.Bool
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("account_root"), &stateRoot)...)
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("account_root"), &planRoot)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If account_root is changing, leave type unknown so it is recomputed.
	// (Equal returns false when either value is unknown, which is the safe path.)
	if !planRoot.Equal(stateRoot) {
		return
	}

	// Otherwise reuse the prior value to avoid a spurious "known after apply".
	resp.PlanValue = req.StateValue
}

// UserResource defines the resource implementation.
type UserResource struct {
	client *RadosgwClient
}

// UserResourceModel describes the resource data model.
type UserResourceModel struct {
	ID                  types.String `tfsdk:"id"`
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
	AccountID           types.String `tfsdk:"account_id"`
	AccountRoot         types.Bool   `tfsdk:"account_root"`
}

func (r *UserResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iam_user"
}

func (r *UserResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a RadosGW user.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The tenant-qualified RGW user ID. For tenant users this is `tenant$user_id`; for non-tenant users this matches `user_id`.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"user_id": schema.StringAttribute{
				MarkdownDescription: "The local user ID. For tenant users, this remains the user name without the tenant prefix; use `id` when another resource needs the tenant-qualified RGW user ID.",
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
				MarkdownDescription: "The tenant to which the user belongs. Cannot be modified after creation. When the user belongs to an account (`account_id`), the tenant must exactly match the account's tenant — RadosGW does not inherit it, so leaving this empty for a tenanted account is rejected.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"max_buckets": schema.Int64Attribute{
				MarkdownDescription: "The maximum number of buckets the user can own. Default is 1000. " +
					"**Not enforced for account members:** when the user belongs to an account (`account_id` is set), " +
					"the buckets it creates are owned by the account, so RadosGW enforces the **account's** " +
					"`max_buckets` (see `radosgw_iam_account`), not this per-user value — it is stored but has no " +
					"effect. The same applies to the user's other quota settings for account members.",
				Optional: true,
				Computed: true,
				Default:  int64default.StaticInt64(1000),
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
				MarkdownDescription: "The user type (e.g., 'rgw', 'ldap', or 'root' for an account root user).",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					typeUseStateUnlessAccountRootChanges{},
				},
			},
			"account_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the account (`radosgw_iam_account`) this user belongs to. When set, the user is created within the account and the account owns its resources. Changing it (including removing it, which returns the user to no account) forces the user to be recreated. Requires Ceph Squid (19.x) or later. Note: RadosGW treats an account user's `display_name` as an IAM entity name, so for account users it must not contain spaces.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"account_root": schema.BoolAttribute{
				MarkdownDescription: "Whether this user is the account root user (able to manage the account's IAM resources). Only valid together with `account_id`. Can be toggled in place without recreating the user. Defaults to `false`, so removing it from the configuration demotes a root user.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
		},
	}
}

// ValidateConfig enforces account-specific rules on the user configuration:
//   - account_root is only valid together with account_id;
//   - an account user's display_name is used as its IAM user name by RadosGW and
//     therefore may not contain whitespace.
func (r *UserResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data UserResourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// account_root only makes sense inside an account. A known-true account_root
	// with a null account_id is a configuration error.
	if data.AccountRoot.ValueBool() && data.AccountID.IsNull() {
		resp.Diagnostics.AddAttributeError(
			path.Root("account_root"),
			"Invalid account_root",
			"account_root can only be set to true when account_id is also configured.",
		)
	}

	// When the user is assigned to an account (account_id configured, even if its
	// value is not yet known), RadosGW derives an IAM user name from display_name,
	// which rejects whitespace with "UserName contains invalid characters".
	if !data.AccountID.IsNull() && !data.DisplayName.IsNull() && !data.DisplayName.IsUnknown() {
		if strings.ContainsFunc(data.DisplayName.ValueString(), unicode.IsSpace) {
			resp.Diagnostics.AddAttributeError(
				path.Root("display_name"),
				"Invalid display_name for account user",
				"When account_id is set, display_name is used as the account user's IAM user name and must not contain spaces or other whitespace.",
			)
		}
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

	// Associate the user with an account when requested. account_root is only
	// meaningful alongside account_id (enforced by ValidateConfig).
	if !data.AccountID.IsNull() && !data.AccountID.IsUnknown() && data.AccountID.ValueString() != "" {
		userConfig.AccountID = data.AccountID.ValueString()
		accountRoot := data.AccountRoot.ValueBool()
		userConfig.AccountRoot = &accountRoot

		// An account user's tenant must exactly equal the account's tenant
		// (RadosGW does not inherit it). Look up the account to surface a clear
		// error before creating the user. If the lookup is unavailable (e.g. the
		// Ceph 20.2.1 account-read bug), skip it and let CreateUser return the
		// server-side "User tenant does not match account tenant" error.
		if account, err := r.client.Admin.GetAccount(ctx, userConfig.AccountID); err == nil {
			if account.Tenant != data.Tenant.ValueString() {
				resp.Diagnostics.AddAttributeError(
					path.Root("tenant"),
					"User tenant does not match account tenant",
					fmt.Sprintf("Account %s has tenant %q, but the user's tenant is %q. An account user's tenant must exactly match the account's tenant.",
						userConfig.AccountID, account.Tenant, data.Tenant.ValueString()),
				)
				return
			}
		}
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

	// Update state with created user data. Preserve user_id as configured and
	// expose the RGW tenant-qualified identifier through id.
	populateUserResourceModel(&data, user)

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
	populateUserResourceModel(&data, user)

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

	// Preserve the account association and apply account_root toggles. account_id
	// is immutable (RequiresReplace), but account_root can change in place, so it
	// must be sent to ModifyUser (with account_id for context).
	if !data.AccountID.IsNull() && !data.AccountID.IsUnknown() && data.AccountID.ValueString() != "" {
		userConfig.AccountID = data.AccountID.ValueString()
		accountRoot := data.AccountRoot.ValueBool()
		userConfig.AccountRoot = &accountRoot
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
	populateUserResourceModel(&data, user)

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
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), buildFullUserID(userID, tenant))...)
}

func populateUserResourceModel(data *UserResourceModel, user admin.User) {
	tenant := user.Tenant
	if tenant == "" && !data.Tenant.IsNull() && !data.Tenant.IsUnknown() {
		tenant = data.Tenant.ValueString()
	}
	if tenant == "" {
		if parsedTenant, _, ok := splitTenantQualifiedUserID(user.ID); ok {
			tenant = parsedTenant
		}
	}

	userID := data.UserID.ValueString()
	if userID == "" {
		if _, localUserID, ok := splitTenantQualifiedUserID(user.ID); ok {
			userID = localUserID
		} else {
			userID = user.ID
		}
	}
	if parsedTenant, localUserID, ok := splitTenantQualifiedUserID(userID); ok && tenant == parsedTenant {
		userID = localUserID
	}

	data.ID = types.StringValue(buildFullUserID(userID, tenant))
	data.UserID = types.StringValue(userID)
	data.DisplayName = types.StringValue(user.DisplayName)
	data.Email = types.StringValue(user.Email)
	data.Tenant = types.StringValue(tenant)
	data.MaxBuckets = types.Int64Value(int64(*user.MaxBuckets))
	data.Suspended = types.BoolValue(*user.Suspended != 0)
	data.OpMask = types.StringValue(user.OpMask)
	data.DefaultPlacement = types.StringValue(user.DefaultPlacement)
	data.DefaultStorageClass = types.StringValue(user.DefaultStorageClass)
	data.Type = types.StringValue(user.Type)
	data.AccountID = types.StringValue(user.AccountID)
	// go-ceph's User.AccountRoot has no JSON tag, so it is never populated from
	// API responses. The account root user is reported with type "root".
	data.AccountRoot = types.BoolValue(user.AccountRoot != nil && *user.AccountRoot || user.Type == "root")
}
