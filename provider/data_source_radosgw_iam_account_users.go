package provider

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &IAMAccountUsersDataSource{}

func NewIAMAccountUsersDataSource() datasource.DataSource {
	return &IAMAccountUsersDataSource{}
}

type IAMAccountUsersDataSource struct {
	iamClient *IAMClient
}

type IAMAccountUsersDataSourceModel struct {
	PathPrefix types.String `tfsdk:"path_prefix"`
	NameRegex  types.String `tfsdk:"name_regex"`
	Names      types.Set    `tfsdk:"names"`
	ARNs       types.Set    `tfsdk:"arns"`
	ID         types.String `tfsdk:"id"`
}

func (d *IAMAccountUsersDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iam_account_users"
}

func (d *IAMAccountUsersDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the IAM users within a RadosGW account (via the IAM `ListUsers` API), " +
			"optionally filtered by path prefix and/or a name regular expression. " +
			"Use with account (or `iam:ListUsers`-granted) credentials.",
		Attributes: map[string]schema.Attribute{
			"path_prefix": schema.StringAttribute{
				MarkdownDescription: "Only return users whose path starts with this prefix (e.g. `/engineering/`).",
				Optional:            true,
			},
			"name_regex": schema.StringAttribute{
				MarkdownDescription: "Only return users whose name matches this regular expression (applied client-side).",
				Optional:            true,
			},
			"names": schema.SetAttribute{
				MarkdownDescription: "Set of matching user names.",
				Computed:            true,
				ElementType:         types.StringType,
			},
			"arns": schema.SetAttribute{
				MarkdownDescription: "Set of matching user ARNs.",
				Computed:            true,
				ElementType:         types.StringType,
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "Data source identifier.",
				Computed:            true,
			},
		},
	}
}

func (d *IAMAccountUsersDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*RadosgwClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *RadosgwClient, got: %T.", req.ProviderData),
		)
		return
	}
	d.iamClient = newAccountIAMClient(client)
}

func (d *IAMAccountUsersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config IAMAccountUsersDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var nameRe *regexp.Regexp
	if !config.NameRegex.IsNull() && config.NameRegex.ValueString() != "" {
		re, err := regexp.Compile(config.NameRegex.ValueString())
		if err != nil {
			resp.Diagnostics.AddError(
				"Invalid name_regex",
				fmt.Sprintf("Could not compile name_regex %q: %s", config.NameRegex.ValueString(), err.Error()),
			)
			return
		}
		nameRe = re
	}

	users, err := iamListUsers(ctx, d.iamClient, config.PathPrefix.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Listing IAM Account Users",
			fmt.Sprintf("Could not list IAM users: %s", err.Error()),
		)
		return
	}

	names := make([]string, 0, len(users))
	arns := make([]string, 0, len(users))
	for _, u := range users {
		if nameRe != nil && !nameRe.MatchString(u.UserName) {
			continue
		}
		names = append(names, u.UserName)
		arns = append(arns, u.Arn)
	}

	namesSet, diags := types.SetValueFrom(ctx, types.StringType, names)
	resp.Diagnostics.Append(diags...)
	arnsSet, diags := types.SetValueFrom(ctx, types.StringType, arns)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	config.Names = namesSet
	config.ARNs = arnsSet
	config.ID = types.StringValue("radosgw-account-users")

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
