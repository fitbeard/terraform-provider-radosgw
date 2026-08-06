package provider

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/ceph/go-ceph/rgw/admin"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// accountIDRegexp matches the RadosGW account ID format: "RGW" followed by 17 digits.
var accountIDRegexp = regexp.MustCompile(`^RGW[0-9]{17}$`)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &AccountResource{}
	_ resource.ResourceWithImportState = &AccountResource{}
)

func NewIAMAccountResource() resource.Resource {
	return &AccountResource{}
}

// AccountResource defines the resource implementation.
type AccountResource struct {
	client *RadosgwClient
}

// AccountResourceModel describes the resource data model.
type AccountResourceModel struct {
	ID            types.String `tfsdk:"id"`
	AccountID     types.String `tfsdk:"account_id"`
	Name          types.String `tfsdk:"name"`
	Email         types.String `tfsdk:"email"`
	Tenant        types.String `tfsdk:"tenant"`
	MaxUsers      types.Int64  `tfsdk:"max_users"`
	MaxRoles      types.Int64  `tfsdk:"max_roles"`
	MaxGroups     types.Int64  `tfsdk:"max_groups"`
	MaxAccessKeys types.Int64  `tfsdk:"max_access_keys"`
	MaxBuckets    types.Int64  `tfsdk:"max_buckets"`
}

func (r *AccountResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iam_account"
}

func (r *AccountResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	// limitInt64 builds the schema for the account's numeric limit attributes.
	// These are Optional+Computed with no hard-coded default: when unset, the
	// value is omitted from the API request so RadosGW applies its own default
	// (1000 for users/roles/groups/buckets, 4 for access keys), and the
	// resulting value is read back into state. UseStateForUnknown keeps that
	// computed value stable across plans so unrelated changes don't re-plan it
	// as unknown.
	//
	// The sign convention is NOT uniform across these limits, so no single
	// sentinel can mean "unlimited" for all of them (see the per-attribute
	// descriptions): for max_buckets a negative value DENIES bucket creation
	// and 0 means unlimited, whereas for the other four a negative value means
	// unlimited and 0 denies creation. Omitting the field sidesteps this
	// entirely.
	limitInt64 := func(desc string) schema.Int64Attribute {
		return schema.Int64Attribute{
			MarkdownDescription: desc,
			Optional:            true,
			Computed:            true,
			PlanModifiers: []planmodifier.Int64{
				int64planmodifier.UseStateForUnknown(),
			},
		}
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a RadosGW account.\n\n" +
			"Accounts provide AWS IAM-style multi-tenancy: an account owns the users, " +
			"roles, groups, and buckets created within it, and quotas apply to the " +
			"account as a whole. Users are associated with an account through the " +
			"`account_id` attribute of `radosgw_iam_user`, and one user may be marked " +
			"as the account root via `account_root`.\n\n" +
			"### Permissions\n\n" +
			"The **account root** user has full access to all of the account's resources " +
			"without any policy. Regular (non-root) account users start with **no " +
			"permissions** — the root must grant them access through IAM identity " +
			"policies, group membership, or assumable roles before they can, for " +
			"example, create buckets. Permissions can be granted with inline policies or " +
			"with AWS **managed policies**, attached by the account root via the IAM API " +
			"(`aws iam attach-user-policy`). RadosGW provides managed policies such as " +
			"`arn:aws:iam::aws:policy/AmazonS3FullAccess` and " +
			"`arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess`.\n\n" +
			"~> **Note:** These are two distinct permission systems. IAM policies (above) " +
			"govern an account's S3/IAM data-plane actions and are account-scoped. RadosGW " +
			"admin capabilities (`radosgw-admin caps add`) are separate, authorize the " +
			"Admin Ops API, and are **cluster-wide**; they can be added to an account user " +
			"but do not grant S3/IAM data-plane access.\n\n" +
			"~> **Note:** Accounts require Ceph Squid (19.x) or later; they are not " +
			"available on Reef (18.x).\n\n" +
			"~> **Note:** A capability-name bug affects the account read/delete admin " +
			"operations on all Squid (19.2.x) releases and on Tentacle before 20.2.2: " +
			"they check a mistyped `account` capability that cannot be granted, so a " +
			"normal `accounts=*` user receives `AccessDenied` (refresh, import, and " +
			"destroy fail). On those versions the provider's RadosGW user must be a " +
			"**system user** (`radosgw-admin user modify --uid=<user> --system`). Ceph " +
			"**20.2.2+** fixes this and works with `accounts=*` alone.\n\n" +
			"~> **Note:** This resource requires the `accounts=*` capability on the " +
			"RadosGW user configured in the provider.\n\n" +
			"~> **Note:** Account and default-bucket quota management is not supported " +
			"by this resource yet, because the underlying `go-ceph` library does not " +
			"expose an account quota setter. Manage account quotas with `radosgw-admin " +
			"quota` in the meantime; support will be added once available upstream.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The account ID. Same value as `account_id`; exposed as the Terraform resource identifier.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"account_id": schema.StringAttribute{
				MarkdownDescription: "The account ID. Must be the string `RGW` followed by 17 digits. If omitted, RadosGW auto-generates one. Cannot be modified after creation.",
				Optional:            true,
				Computed:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(accountIDRegexp, "must be \"RGW\" followed by 17 digits"),
				},
				PlanModifiers: []planmodifier.String{
					// Keep the RadosGW-generated ID stable across plans; without
					// this a computed ID re-plans as unknown and, combined with
					// RequiresReplace, forces spurious replacements on unrelated
					// attribute changes.
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The account name.",
				Required:            true,
			},
			"email": schema.StringAttribute{
				MarkdownDescription: "The email address associated with the account.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"tenant": schema.StringAttribute{
				MarkdownDescription: "The tenant under which the account exists. Cannot be modified after creation.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"max_users":       limitInt64("The maximum number of users the account can own. If unset, RadosGW applies its default of `1000`. A negative value (e.g. `-1`) means unlimited; `0` disables user creation. A positive value caps the count."),
			"max_roles":       limitInt64("The maximum number of roles the account can own. If unset, RadosGW applies its default of `1000`. A negative value (e.g. `-1`) means unlimited; `0` disables role creation. A positive value caps the count."),
			"max_groups":      limitInt64("The maximum number of groups the account can own. If unset, RadosGW applies its default of `1000`. A negative value (e.g. `-1`) means unlimited; `0` disables group creation. A positive value caps the count."),
			"max_access_keys": limitInt64("The maximum number of access keys per account user. If unset, RadosGW applies its default of `4`. A negative value (e.g. `-1`) means unlimited; `0` disables key creation. A positive value caps the count."),
			// NOTE: max_buckets uses the OPPOSITE sign convention from the limits
			// above (see rgw_op.cc check_owner_max_buckets: `remaining < 0` returns
			// -EPERM, `remaining == 0` is unlimited).
			"max_buckets": limitInt64("The maximum number of buckets the account can own. If unset, RadosGW applies its default of `1000`. **Note:** unlike the other limits, `0` means unlimited and a **negative value (e.g. `-1`) DISABLES bucket creation** (account users get `403 AccessDenied`). A positive value caps the count. This account-level limit governs bucket creation by **all** of the account's members (buckets are owned by the account); a member's own `radosgw_iam_user` `max_buckets` is not enforced."),
		},
	}
}

func (r *AccountResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *AccountResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AccountResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Creating RadosGW account", map[string]any{
		"account_id": data.AccountID.ValueString(),
		"name":       data.Name.ValueString(),
	})

	accountConfig := buildAccountFromModel(&data)

	var account admin.Account
	err := retryOnConcurrentModification(ctx, fmt.Sprintf("CreateAccount %s", data.Name.ValueString()), func() error {
		var createErr error
		account, createErr = r.client.Admin.CreateAccount(ctx, accountConfig)
		return createErr
	})
	if err != nil {
		if errors.Is(err, admin.ErrAccountAlreadyExists) {
			resp.Diagnostics.AddError(
				"RadosGW Account Already Exists",
				fmt.Sprintf("An account with name %q (or account_id %q) already exists. "+
					"Account names and IDs must be unique within the cluster. Choose a different "+
					"name/account_id, or import the existing account with `terraform import`.",
					data.Name.ValueString(), data.AccountID.ValueString()),
			)
			return
		}
		resp.Diagnostics.AddError(
			"Error Creating RadosGW Account",
			"Could not create account, unexpected error: "+err.Error(),
		)
		return
	}

	populateAccountResourceModel(&data, account)

	tflog.Trace(ctx, "Created RadosGW account")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AccountResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AccountResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	accountID := data.AccountID.ValueString()

	tflog.Debug(ctx, "Reading RadosGW account", map[string]any{
		"account_id": accountID,
	})

	account, err := r.client.Admin.GetAccount(ctx, accountID)
	if err != nil {
		if isAccountNotFoundError(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading RadosGW Account",
			fmt.Sprintf("Could not read account %s: %s", accountID, err.Error()),
		)
		return
	}

	populateAccountResourceModel(&data, account)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AccountResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data AccountResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Updating RadosGW account", map[string]any{
		"account_id": data.AccountID.ValueString(),
	})

	accountConfig := buildAccountFromModel(&data)

	var account admin.Account
	err := retryOnConcurrentModification(ctx, fmt.Sprintf("ModifyAccount %s", data.AccountID.ValueString()), func() error {
		var modifyErr error
		account, modifyErr = r.client.Admin.ModifyAccount(ctx, accountConfig)
		return modifyErr
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating RadosGW Account",
			"Could not update account, unexpected error: "+err.Error(),
		)
		return
	}

	populateAccountResourceModel(&data, account)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AccountResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AccountResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	accountID := data.AccountID.ValueString()

	tflog.Debug(ctx, "Deleting RadosGW account", map[string]any{
		"account_id": accountID,
	})

	err := retryOnConcurrentModification(ctx, fmt.Sprintf("DeleteAccount %s", accountID), func() error {
		return r.client.Admin.DeleteAccount(ctx, accountID)
	})
	if err != nil {
		// Treat an already-absent account as a successful delete.
		if isAccountNotFoundError(err) {
			return
		}
		resp.Diagnostics.AddError(
			"Error Deleting RadosGW Account",
			"Could not delete account, unexpected error: "+err.Error(),
		)
		return
	}
}

func (r *AccountResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("account_id"), req.ID)...)
}

// buildAccountFromModel maps the Terraform model onto an admin.Account. Numeric
// limits are only sent when known so that RadosGW can assign its own defaults
// for omitted (unknown) values.
func buildAccountFromModel(data *AccountResourceModel) admin.Account {
	account := admin.Account{
		Name: data.Name.ValueString(),
	}

	if !data.AccountID.IsNull() && !data.AccountID.IsUnknown() {
		account.ID = data.AccountID.ValueString()
	}
	if !data.Email.IsNull() && !data.Email.IsUnknown() {
		account.Email = data.Email.ValueString()
	}
	if !data.Tenant.IsNull() && !data.Tenant.IsUnknown() {
		account.Tenant = data.Tenant.ValueString()
	}

	account.MaxUsers = int64PtrFromModel(data.MaxUsers)
	account.MaxRoles = int64PtrFromModel(data.MaxRoles)
	account.MaxGroups = int64PtrFromModel(data.MaxGroups)
	account.MaxAccessKeys = int64PtrFromModel(data.MaxAccessKeys)
	account.MaxBuckets = int64PtrFromModel(data.MaxBuckets)

	return account
}

// int64PtrFromModel returns a pointer to the attribute value, or nil when the
// value is null or unknown (so it is omitted from the API request).
func int64PtrFromModel(v types.Int64) *int64 {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	val := v.ValueInt64()
	return &val
}

func populateAccountResourceModel(data *AccountResourceModel, account admin.Account) {
	data.ID = types.StringValue(account.ID)
	data.AccountID = types.StringValue(account.ID)
	data.Name = types.StringValue(account.Name)
	data.Email = types.StringValue(account.Email)
	data.Tenant = types.StringValue(account.Tenant)
	data.MaxUsers = int64ValueFromPtr(account.MaxUsers)
	data.MaxRoles = int64ValueFromPtr(account.MaxRoles)
	data.MaxGroups = int64ValueFromPtr(account.MaxGroups)
	data.MaxAccessKeys = int64ValueFromPtr(account.MaxAccessKeys)
	data.MaxBuckets = int64ValueFromPtr(account.MaxBuckets)
}

// int64ValueFromPtr converts an optional API integer into a Terraform value.
// RadosGW always echoes the effective limit (its own default when the field was
// omitted), so a nil pointer is not expected; it maps to null defensively.
func int64ValueFromPtr(v *int64) types.Int64 {
	if v == nil {
		return types.Int64Null()
	}
	return types.Int64Value(*v)
}
