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
var _ datasource.DataSource = &AccountDataSource{}

func NewIAMAccountDataSource() datasource.DataSource {
	return &AccountDataSource{}
}

// AccountDataSource defines the data source implementation.
type AccountDataSource struct {
	client *RadosgwClient
}

// AccountDataSourceModel describes the data source data model.
type AccountDataSourceModel struct {
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

func (d *AccountDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iam_account"
}

func (d *AccountDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	computed := func(desc string) schema.Int64Attribute {
		return schema.Int64Attribute{MarkdownDescription: desc, Computed: true}
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves information about an existing RadosGW account.\n\n" +
			"~> **Note:** Accounts require Ceph Squid (19.x) or later. On Ceph 20.2.1 the " +
			"account read admin operation is broken by a capability-name bug; use 20.2.2 or later.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The account ID. Same value as `account_id`.",
				Computed:            true,
			},
			"account_id": schema.StringAttribute{
				MarkdownDescription: "The account ID to look up.",
				Required:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The account name.",
				Computed:            true,
			},
			"email": schema.StringAttribute{
				MarkdownDescription: "The email address associated with the account.",
				Computed:            true,
			},
			"tenant": schema.StringAttribute{
				MarkdownDescription: "The tenant under which the account exists.",
				Computed:            true,
			},
			"max_users":       computed("The maximum number of users the account can own (-1 for unlimited)."),
			"max_roles":       computed("The maximum number of roles the account can own (-1 for unlimited)."),
			"max_groups":      computed("The maximum number of groups the account can own (-1 for unlimited)."),
			"max_access_keys": computed("The maximum number of access keys the account can own (-1 for unlimited)."),
			"max_buckets":     computed("The maximum number of buckets the account can own (-1 for unlimited)."),
		},
	}
}

func (d *AccountDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *AccountDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config AccountDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	accountID := config.AccountID.ValueString()

	tflog.Debug(ctx, "Reading RadosGW account data source", map[string]any{
		"account_id": accountID,
	})

	account, err := d.client.Admin.GetAccount(ctx, accountID)
	if err != nil {
		if isAccountNotFoundError(err) {
			resp.Diagnostics.AddError(
				"Account Not Found",
				fmt.Sprintf("Account with ID %q does not exist.", accountID),
			)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading RadosGW Account",
			fmt.Sprintf("Could not read account %s: %s", accountID, err.Error()),
		)
		return
	}

	config.ID = types.StringValue(account.ID)
	config.AccountID = types.StringValue(account.ID)
	config.Name = types.StringValue(account.Name)
	config.Email = types.StringValue(account.Email)
	config.Tenant = types.StringValue(account.Tenant)
	config.MaxUsers = int64ValueFromPtr(account.MaxUsers)
	config.MaxRoles = int64ValueFromPtr(account.MaxRoles)
	config.MaxGroups = int64ValueFromPtr(account.MaxGroups)
	config.MaxAccessKeys = int64ValueFromPtr(account.MaxAccessKeys)
	config.MaxBuckets = int64ValueFromPtr(account.MaxBuckets)

	tflog.Trace(ctx, "Read RadosGW account data source", map[string]any{
		"account_id": account.ID,
		"name":       account.Name,
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
