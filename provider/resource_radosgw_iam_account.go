package provider

import (
	"context"
	"fmt"
	"regexp"

	"github.com/ceph/go-ceph/rgw/admin"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

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
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an IAM account in RadosGW.\n\n" +
			"IAM accounts provide multi-tenant isolation, allowing tenants to independently manage their own users, roles, policies, and buckets through the IAM API. Each account is managed by an account root user.\n\n" +
			"### Key Concepts\n\n" +
			"- **Account**: A top-level container for IAM resources (users, roles, policies, buckets)\n" +
			"- **Account Root User**: A special user created for managing the account via IAM API\n" +
			"- **Account ID**: A unique identifier for the account (needs to start with \"RGW\" followed by 17 numbers)\n\n" +
			"~> **Note:** IAM accounts require Ceph Squid (19.x) or later.\n\n" +
			"~> **Note:** Account-level quota management is not yet supported by the underlying `go-ceph` library, because Quota apis for account is only available mainlien. We provide this in the future once the underlying library support made it available.\n\n" +
			"~> **Note**: This resource requires the `accounts=*` capability on the RadosGW user used by the provider.",
		Attributes: map[string]schema.Attribute{
			"account_id": schema.StringAttribute{
				MarkdownDescription: "The account ID. Must start with 'RGW' prefix. If not provided, RGW will auto-generate one.",
				Optional:            true,
				Computed:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(accountIDRegexp(), "must start with 'RGW' prefix"),
				},
				PlanModifiers: []planmodifier.String{
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
				MarkdownDescription: "The tenant identifier. Cannot be modified after creation.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"max_users": schema.Int64Attribute{
				MarkdownDescription: "The maximum number of users that can be created under this account. Use -1 for unlimited. Default is -1.",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(-1),
			},
			"max_roles": schema.Int64Attribute{
				MarkdownDescription: "The maximum number of roles that can be created under this account. Use -1 for unlimited. Default is -1.",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(-1),
			},
			"max_groups": schema.Int64Attribute{
				MarkdownDescription: "The maximum number of groups that can be created under this account. Use -1 for unlimited. Default is -1.",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(-1),
			},
			"max_access_keys": schema.Int64Attribute{
				MarkdownDescription: "The maximum number of access keys that can be created under this account. Use -1 for unlimited. Default is -1.",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(-1),
			},
			"max_buckets": schema.Int64Attribute{
				MarkdownDescription: "The maximum number of buckets that can be created under this account. Use -1 for unlimited. Default is 1000.",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(1000),
			},
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

	userProvidedID := !data.AccountID.IsNull() && data.AccountID.ValueString() != ""

	tflog.Debug(ctx, "Creating RadosGW account", map[string]any{
		"account_id":    data.AccountID.ValueString(),
		"name":          data.Name.ValueString(),
		"user_provided": userProvidedID,
	})

	account := buildAccountFromModel(ctx, &data)

	var createdAccount admin.Account
	err := retryOnConcurrentModification(ctx, fmt.Sprintf("CreateAccount %s", data.AccountID.ValueString()), func() error {
		var createErr error
		createdAccount, createErr = r.client.Admin.CreateAccount(ctx, account)
		return createErr
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating RadosGW Account",
			fmt.Sprintf("Could not create account: %s", err.Error()),
		)
		return
	}

	populateAccountResourceModel(&data, &createdAccount)

	tflog.Trace(ctx, "Created RadosGW account", map[string]any{
		"account_id": data.AccountID.ValueString(),
	})

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

	populateAccountResourceModel(&data, &account)

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

	account := buildAccountFromModel(ctx, &data)

	var updatedAccount admin.Account
	err := retryOnConcurrentModification(ctx, fmt.Sprintf("ModifyAccount %s", data.AccountID.ValueString()), func() error {
		var modifyErr error
		updatedAccount, modifyErr = r.client.Admin.ModifyAccount(ctx, account)
		return modifyErr
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating RadosGW Account",
			fmt.Sprintf("Could not update account %s: %s", data.AccountID.ValueString(), err.Error()),
		)
		return
	}

	populateAccountResourceModel(&data, &updatedAccount)

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
		resp.Diagnostics.AddError(
			"Error Deleting RadosGW Account",
			fmt.Sprintf("Could not delete account %s: %s", accountID, err.Error()),
		)
		return
	}
}

func (r *AccountResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("account_id"), req.ID)...)
}

func buildAccountFromModel(ctx context.Context, data *AccountResourceModel) admin.Account {
	account := admin.Account{
		Name: data.Name.ValueString(),
	}

	if !data.AccountID.IsNull() && !data.AccountID.IsUnknown() {
		account.ID = data.AccountID.ValueString()
	}

	if !data.Email.IsNull() {
		account.Email = data.Email.ValueString()
	}

	if !data.Tenant.IsNull() && !data.Tenant.IsUnknown() {
		account.Tenant = data.Tenant.ValueString()
	}

	account.MaxUsers = intPtr(data.MaxUsers.ValueInt64())
	account.MaxRoles = intPtr(data.MaxRoles.ValueInt64())
	account.MaxGroups = intPtr(data.MaxGroups.ValueInt64())
	account.MaxAccessKeys = intPtr(data.MaxAccessKeys.ValueInt64())
	account.MaxBuckets = intPtr(data.MaxBuckets.ValueInt64())

	return account
}

func populateAccountResourceModel(data *AccountResourceModel, account *admin.Account) {
	data.AccountID = types.StringValue(account.ID)
	data.Name = types.StringValue(account.Name)
	data.Email = types.StringValue(account.Email)
	data.Tenant = types.StringValue(account.Tenant)

	if account.MaxUsers != nil {
		data.MaxUsers = types.Int64Value(*account.MaxUsers)
	}
	if account.MaxRoles != nil {
		data.MaxRoles = types.Int64Value(*account.MaxRoles)
	}
	if account.MaxGroups != nil {
		data.MaxGroups = types.Int64Value(*account.MaxGroups)
	}
	if account.MaxAccessKeys != nil {
		data.MaxAccessKeys = types.Int64Value(*account.MaxAccessKeys)
	}
	if account.MaxBuckets != nil {
		data.MaxBuckets = types.Int64Value(*account.MaxBuckets)
	}
}

func accountIDRegexp() *regexp.Regexp {
	return regexp.MustCompile("^RGW[0-9]{17}$")
}

func intPtr(v int64) *int64 {
	return &v
}
