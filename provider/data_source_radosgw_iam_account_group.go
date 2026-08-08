package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &IAMAccountGroupDataSource{}

func NewIAMAccountGroupDataSource() datasource.DataSource {
	return &IAMAccountGroupDataSource{}
}

type IAMAccountGroupDataSource struct {
	iamClient *IAMClient
}

type IAMAccountGroupDataSourceModel struct {
	Name     types.String `tfsdk:"name"`
	Path     types.String `tfsdk:"path"`
	ARN      types.String `tfsdk:"arn"`
	UniqueID types.String `tfsdk:"unique_id"`
	Users    types.List   `tfsdk:"users"`
}

func (d *IAMAccountGroupDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iam_account_group"
}

func (d *IAMAccountGroupDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up an IAM group within a RadosGW account (IAM `GetGroup`), including its member " +
			"users. Use with account (or `iam:GetGroup`-granted) credentials.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the IAM group to look up.",
				Required:            true,
			},
			"path": schema.StringAttribute{
				MarkdownDescription: "The group's IAM path.",
				Computed:            true,
			},
			"arn": schema.StringAttribute{
				MarkdownDescription: "The group's ARN.",
				Computed:            true,
			},
			"unique_id": schema.StringAttribute{
				MarkdownDescription: "The group's stable unique identifier.",
				Computed:            true,
			},
			"users": schema.ListAttribute{
				MarkdownDescription: "The names of the users that are members of the group.",
				Computed:            true,
				ElementType:         types.StringType,
			},
		},
	}
}

func (d *IAMAccountGroupDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *IAMAccountGroupDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config IAMAccountGroupDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	group, err := iamGetGroup(ctx, d.iamClient, config.Name.ValueString())
	if err != nil {
		if errors.Is(err, ErrNoSuchEntity) {
			resp.Diagnostics.AddError(
				"IAM Account Group Not Found",
				fmt.Sprintf("No IAM group named %q was found in the account.", config.Name.ValueString()),
			)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading IAM Account Group",
			fmt.Sprintf("Could not read IAM group %s: %s", config.Name.ValueString(), err.Error()),
		)
		return
	}

	members, err := iamGetGroupMembers(ctx, d.iamClient, config.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading IAM Account Group Members",
			fmt.Sprintf("Could not read members of group %s: %s", config.Name.ValueString(), err.Error()),
		)
		return
	}
	usersList, diags := types.ListValueFrom(ctx, types.StringType, members)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	config.Name = types.StringValue(group.GroupName)
	config.Path = types.StringValue(group.Path)
	config.ARN = types.StringValue(group.Arn)
	config.UniqueID = types.StringValue(group.GroupId)
	config.Users = usersList

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
