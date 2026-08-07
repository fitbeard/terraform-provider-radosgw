package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &IAMAccountAccessKeysDataSource{}

func NewIAMAccountAccessKeysDataSource() datasource.DataSource {
	return &IAMAccountAccessKeysDataSource{}
}

type IAMAccountAccessKeysDataSource struct {
	iamClient *IAMClient
}

type IAMAccountAccessKeysDataSourceModel struct {
	User       types.String `tfsdk:"user"`
	AccessKeys types.List   `tfsdk:"access_keys"`
	ID         types.String `tfsdk:"id"`
}

func accountAccessKeyAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"access_key_id": types.StringType,
		"status":        types.StringType,
		"create_date":   types.StringType,
	}
}

func (d *IAMAccountAccessKeysDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iam_account_access_keys"
}

func (d *IAMAccountAccessKeysDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the access keys of an IAM user within a RadosGW account (via the IAM " +
			"`ListAccessKeys` API). Secrets are never returned. Use with account (or `iam:ListAccessKeys`-granted) " +
			"credentials.",
		Attributes: map[string]schema.Attribute{
			"user": schema.StringAttribute{
				MarkdownDescription: "The IAM user whose access keys to list.",
				Required:            true,
			},
			"access_keys": schema.ListNestedAttribute{
				MarkdownDescription: "The user's access keys.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"access_key_id": schema.StringAttribute{
							MarkdownDescription: "The access key ID.",
							Computed:            true,
						},
						"status": schema.StringAttribute{
							MarkdownDescription: "The status of the access key: `Active` or `Inactive`.",
							Computed:            true,
						},
						"create_date": schema.StringAttribute{
							MarkdownDescription: "The date and time the access key was created, in RFC 3339 format.",
							Computed:            true,
						},
					},
				},
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "Data source identifier (the user name).",
				Computed:            true,
			},
		},
	}
}

func (d *IAMAccountAccessKeysDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *IAMAccountAccessKeysDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config IAMAccountAccessKeysDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	user := config.User.ValueString()
	keys, err := iamListAccessKeys(ctx, d.iamClient, user)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Listing IAM Account Access Keys",
			fmt.Sprintf("Could not list access keys for user %s: %s", user, err.Error()),
		)
		return
	}

	elems := make([]attr.Value, 0, len(keys))
	for _, k := range keys {
		obj, diags := types.ObjectValue(accountAccessKeyAttrTypes(), map[string]attr.Value{
			"access_key_id": types.StringValue(k.AccessKeyId),
			"status":        types.StringValue(strings.ToLower(k.Status)),
			"create_date":   types.StringValue(k.CreateDate),
		})
		resp.Diagnostics.Append(diags...)
		elems = append(elems, obj)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	list, diags := types.ListValue(types.ObjectType{AttrTypes: accountAccessKeyAttrTypes()}, elems)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	config.AccessKeys = list
	config.ID = types.StringValue(user)

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
