package provider

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/url"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &RolesDataSource{}

func NewIAMRolesDataSource() datasource.DataSource {
	return &RolesDataSource{}
}

// RolesDataSource defines the data source implementation.
type RolesDataSource struct {
	client    *RadosgwClient
	iamClient *IAMClient
}

// RolesDataSourceModel describes the data source data model.
type RolesDataSourceModel struct {
	NameRegex  types.String `tfsdk:"name_regex"`
	PathPrefix types.String `tfsdk:"path_prefix"`
	Names      types.Set    `tfsdk:"names"`
	ARNs       types.Set    `tfsdk:"arns"`
	ID         types.String `tfsdk:"id"`
}

// XML response structures for ListRoles
type listRolesResponseXML struct {
	XMLName xml.Name        `xml:"ListRolesResponse"`
	Result  listRolesResult `xml:"ListRolesResult"`
}

type listRolesResult struct {
	Roles       rolesListXML `xml:"Roles"`
	IsTruncated bool         `xml:"IsTruncated"`
	Marker      string       `xml:"Marker"`
}

type rolesListXML struct {
	Members []roleXML `xml:"member"`
}

func (d *RolesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iam_roles"
}

func (d *RolesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves a list of IAM role names and ARNs in RadosGW. " +
			"Use this data source to get all roles or filter them by path prefix or name pattern.",

		Attributes: map[string]schema.Attribute{
			"name_regex": schema.StringAttribute{
				MarkdownDescription: "A regex pattern to filter role names. Only roles whose name matches the pattern will be returned.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(regexp.MustCompile(`.*`), "must be a valid regex pattern"),
				},
			},
			"path_prefix": schema.StringAttribute{
				MarkdownDescription: "Path prefix for filtering the results. For example, `/application_abc/` would return all roles " +
					"whose path starts with `/application_abc/`. Defaults to `/` if not specified.",
				Optional: true,
			},
			"names": schema.SetAttribute{
				MarkdownDescription: "Set of role names matching the filter criteria.",
				Computed:            true,
				ElementType:         types.StringType,
			},
			"arns": schema.SetAttribute{
				MarkdownDescription: "Set of role ARNs matching the filter criteria.",
				Computed:            true,
				ElementType:         types.StringType,
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "The data source identifier.",
				Computed:            true,
			},
		},
	}
}

func (d *RolesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *RolesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config RolesDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading RadosGW roles data source")

	// Build request parameters
	params := url.Values{}
	params.Set("Action", "ListRoles")

	if !config.PathPrefix.IsNull() && config.PathPrefix.ValueString() != "" {
		params.Set("PathPrefix", config.PathPrefix.ValueString())
	}

	// Get all roles (handle pagination if needed)
	var allRoles []roleXML
	for {
		body, err := d.iamClient.DoRequest(ctx, params, "iam")
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Reading RadosGW Roles",
				fmt.Sprintf("Could not list roles: %s", err.Error()),
			)
			return
		}

		var response listRolesResponseXML
		if err := xml.Unmarshal(body, &response); err != nil {
			resp.Diagnostics.AddError(
				"Error Parsing Response",
				fmt.Sprintf("Could not parse ListRoles response: %s", err.Error()),
			)
			return
		}

		allRoles = append(allRoles, response.Result.Roles.Members...)

		if !response.Result.IsTruncated {
			break
		}
		params.Set("Marker", response.Result.Marker)
	}

	// Filter by regex if provided
	var filteredNames []string
	var filteredARNs []string

	if !config.NameRegex.IsNull() && config.NameRegex.ValueString() != "" {
		pattern := config.NameRegex.ValueString()
		re, err := regexp.Compile(pattern)
		if err != nil {
			resp.Diagnostics.AddError(
				"Invalid Regex Pattern",
				fmt.Sprintf("Could not compile regex pattern %q: %s", pattern, err.Error()),
			)
			return
		}

		for _, role := range allRoles {
			if re.MatchString(role.RoleName) {
				filteredNames = append(filteredNames, role.RoleName)
				filteredARNs = append(filteredARNs, role.Arn)
			}
		}

		tflog.Debug(ctx, "Filtered roles by regex", map[string]any{
			"pattern":       pattern,
			"total_roles":   len(allRoles),
			"matched_roles": len(filteredNames),
		})
	} else {
		for _, role := range allRoles {
			filteredNames = append(filteredNames, role.RoleName)
			filteredARNs = append(filteredARNs, role.Arn)
		}
		tflog.Debug(ctx, "Returning all roles", map[string]any{
			"total_roles": len(filteredNames),
		})
	}

	// Convert to sets
	namesSet, diags := types.SetValueFrom(ctx, types.StringType, filteredNames)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	arnsSet, diags := types.SetValueFrom(ctx, types.StringType, filteredARNs)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	config.Names = namesSet
	config.ARNs = arnsSet
	config.ID = types.StringValue("radosgw-roles")

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
