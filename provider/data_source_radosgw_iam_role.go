package provider

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &RoleDataSource{}

func NewIAMRoleDataSource() datasource.DataSource {
	return &RoleDataSource{}
}

// RoleDataSource defines the data source implementation.
type RoleDataSource struct {
	client    *RadosgwClient
	iamClient *IAMClient
}

// RoleDataSourceModel describes the data source data model.
type RoleDataSourceModel struct {
	Name               types.String `tfsdk:"name"`
	Path               types.String `tfsdk:"path"`
	Description        types.String `tfsdk:"description"`
	AssumeRolePolicy   types.String `tfsdk:"assume_role_policy"`
	MaxSessionDuration types.Int64  `tfsdk:"max_session_duration"`
	ARN                types.String `tfsdk:"arn"`
	CreateDate         types.String `tfsdk:"create_date"`
	UniqueID           types.String `tfsdk:"unique_id"`
}

func (d *RoleDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iam_role"
}

func (d *RoleDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves information about an existing IAM role in RadosGW. " +
			"Use this data source to get role properties without having to hard code ARNs.",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the role to look up.",
				Required:            true,
			},
			"path": schema.StringAttribute{
				MarkdownDescription: "The path to the role.",
				Computed:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "The description of the role.",
				Computed:            true,
			},
			"assume_role_policy": schema.StringAttribute{
				MarkdownDescription: "The trust relationship policy document (in JSON format) that grants an entity permission to assume the role.",
				Computed:            true,
			},
			"max_session_duration": schema.Int64Attribute{
				MarkdownDescription: "Maximum session duration (in seconds) for the role.",
				Computed:            true,
			},
			"arn": schema.StringAttribute{
				MarkdownDescription: "Amazon Resource Name (ARN) of the role.",
				Computed:            true,
			},
			"create_date": schema.StringAttribute{
				MarkdownDescription: "Date and time when the role was created, in RFC 3339 format.",
				Computed:            true,
			},
			"unique_id": schema.StringAttribute{
				MarkdownDescription: "Stable and unique string identifying the role.",
				Computed:            true,
			},
		},
	}
}

func (d *RoleDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
	d.iamClient = NewIAMClient(
		client.Admin.Endpoint,
		client.Admin.AccessKey,
		client.Admin.SecretKey,
		client.Admin.HTTPClient,
	)
}

func (d *RoleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config RoleDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	roleName := config.Name.ValueString()

	tflog.Debug(ctx, "Reading RadosGW role data source", map[string]any{
		"name": roleName,
	})

	// Get role info
	params := url.Values{}
	params.Set("Action", "GetRole")
	params.Set("RoleName", roleName)

	body, err := d.iamClient.DoRequest(ctx, params, "iam")
	if err != nil {
		if errors.Is(err, ErrNoSuchEntity) {
			resp.Diagnostics.AddError(
				"Role Not Found",
				fmt.Sprintf("Role with name %q does not exist.", roleName),
			)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading RadosGW Role",
			fmt.Sprintf("Could not read role %s: %s", roleName, err.Error()),
		)
		return
	}

	var response getRoleResponseXML
	if err := xml.Unmarshal(body, &response); err != nil {
		resp.Diagnostics.AddError(
			"Error Parsing Response",
			fmt.Sprintf("Could not parse GetRole response: %s", err.Error()),
		)
		return
	}

	role := response.Result.Role

	// URL decode the assume role policy document
	assumeRolePolicy, err := url.QueryUnescape(role.AssumeRolePolicyDocument)
	if err != nil {
		assumeRolePolicy = role.AssumeRolePolicyDocument
	}

	// Populate the model
	config.Name = types.StringValue(role.RoleName)
	config.Path = types.StringValue(role.Path)
	config.ARN = types.StringValue(role.Arn)
	config.UniqueID = types.StringValue(role.RoleId)
	config.CreateDate = types.StringValue(role.CreateDate)
	config.MaxSessionDuration = types.Int64Value(role.MaxSessionDuration)
	config.AssumeRolePolicy = types.StringValue(assumeRolePolicy)

	if role.Description != "" {
		config.Description = types.StringValue(role.Description)
	} else {
		config.Description = types.StringNull()
	}

	tflog.Trace(ctx, "Read role data source", map[string]any{
		"name": role.RoleName,
		"arn":  role.Arn,
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
