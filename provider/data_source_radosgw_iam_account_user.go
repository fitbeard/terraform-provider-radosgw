package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &IAMAccountUserDataSource{}

func NewIAMAccountUserDataSource() datasource.DataSource {
	return &IAMAccountUserDataSource{}
}

type IAMAccountUserDataSource struct {
	iamClient *IAMClient
}

type IAMAccountUserDataSourceModel struct {
	Name       types.String `tfsdk:"name"`
	Path       types.String `tfsdk:"path"`
	ARN        types.String `tfsdk:"arn"`
	UniqueID   types.String `tfsdk:"unique_id"`
	CreateDate types.String `tfsdk:"create_date"`
}

func (d *IAMAccountUserDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iam_account_user"
}

func (d *IAMAccountUserDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up an IAM user within a RadosGW account (via the IAM `GetUser` API). " +
			"Use with account (or `iam:GetUser`-granted) credentials. See `radosgw_iam_account_user`.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the IAM user to look up.",
				Required:            true,
			},
			"path": schema.StringAttribute{
				MarkdownDescription: "The user's IAM path.",
				Computed:            true,
			},
			"arn": schema.StringAttribute{
				MarkdownDescription: "The user's ARN.",
				Computed:            true,
			},
			"unique_id": schema.StringAttribute{
				MarkdownDescription: "The user's stable unique identifier.",
				Computed:            true,
			},
			"create_date": schema.StringAttribute{
				MarkdownDescription: "The date and time the user was created, in RFC 3339 format.",
				Computed:            true,
			},
		},
	}
}

func (d *IAMAccountUserDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *IAMAccountUserDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config IAMAccountUserDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	user, err := iamGetUser(ctx, d.iamClient, config.Name.ValueString())
	if err != nil {
		if errors.Is(err, ErrNoSuchEntity) {
			resp.Diagnostics.AddError(
				"IAM Account User Not Found",
				fmt.Sprintf("No IAM user named %q was found in the account.", config.Name.ValueString()),
			)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading IAM Account User",
			fmt.Sprintf("Could not read IAM user %s: %s", config.Name.ValueString(), err.Error()),
		)
		return
	}

	config.Name = types.StringValue(user.UserName)
	config.Path = types.StringValue(user.Path)
	config.ARN = types.StringValue(user.Arn)
	config.UniqueID = types.StringValue(user.UserId)
	config.CreateDate = types.StringValue(user.CreateDate)

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
