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
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves information about an existing RadosGW account.\n\n" +
			"~> **Note:** IAM accounts require Ceph Squid (19.x) or later.",

		Attributes: map[string]schema.Attribute{
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
				MarkdownDescription: "The tenant identifier.",
				Computed:            true,
			},
			"max_users": schema.Int64Attribute{
				MarkdownDescription: "The maximum number of users allowed (-1 for unlimited).",
				Computed:            true,
			},
			"max_roles": schema.Int64Attribute{
				MarkdownDescription: "The maximum number of roles allowed (-1 for unlimited).",
				Computed:            true,
			},
			"max_groups": schema.Int64Attribute{
				MarkdownDescription: "The maximum number of groups allowed (-1 for unlimited).",
				Computed:            true,
			},
			"max_access_keys": schema.Int64Attribute{
				MarkdownDescription: "The maximum number of access keys allowed (-1 for unlimited).",
				Computed:            true,
			},
			"max_buckets": schema.Int64Attribute{
				MarkdownDescription: "The maximum number of buckets allowed (-1 for unlimited).",
				Computed:            true,
			},
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

	config.AccountID = types.StringValue(account.ID)
	config.Name = types.StringValue(account.Name)
	config.Email = types.StringValue(account.Email)
	config.Tenant = types.StringValue(account.Tenant)

	if account.MaxUsers != nil {
		config.MaxUsers = types.Int64Value(*account.MaxUsers)
	} else {
		config.MaxUsers = types.Int64Value(-1)
	}

	if account.MaxRoles != nil {
		config.MaxRoles = types.Int64Value(*account.MaxRoles)
	} else {
		config.MaxRoles = types.Int64Value(-1)
	}

	if account.MaxGroups != nil {
		config.MaxGroups = types.Int64Value(*account.MaxGroups)
	} else {
		config.MaxGroups = types.Int64Value(-1)
	}

	if account.MaxAccessKeys != nil {
		config.MaxAccessKeys = types.Int64Value(*account.MaxAccessKeys)
	} else {
		config.MaxAccessKeys = types.Int64Value(-1)
	}

	if account.MaxBuckets != nil {
		config.MaxBuckets = types.Int64Value(*account.MaxBuckets)
	} else {
		config.MaxBuckets = types.Int64Value(-1)
	}

	tflog.Trace(ctx, "Read account data source", map[string]any{
		"account_id":   account.ID,
		"account_name": account.Name,
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
