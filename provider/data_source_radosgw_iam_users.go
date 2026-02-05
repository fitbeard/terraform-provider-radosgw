package provider

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &UsersDataSource{}

func NewIAMUsersDataSource() datasource.DataSource {
	return &UsersDataSource{}
}

// UsersDataSource defines the data source implementation.
type UsersDataSource struct {
	client *RadosgwClient
}

// UsersDataSourceModel describes the data source data model.
type UsersDataSourceModel struct {
	NameRegex types.String `tfsdk:"name_regex"`
	UserIDs   types.Set    `tfsdk:"user_ids"`
	ID        types.String `tfsdk:"id"`
}

func (d *UsersDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iam_users"
}

func (d *UsersDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves a list of RadosGW user IDs. " +
			"Use this data source to get all users or filter them by a regex pattern.",

		Attributes: map[string]schema.Attribute{
			"name_regex": schema.StringAttribute{
				MarkdownDescription: "A regex pattern to filter user IDs. Only users whose ID matches the pattern will be returned.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(regexp.MustCompile(`.*`), "must be a valid regex pattern"),
				},
			},
			"user_ids": schema.SetAttribute{
				MarkdownDescription: "Set of user IDs matching the filter criteria. If no filter is specified, all user IDs are returned.",
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

func (d *UsersDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *UsersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config UsersDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading RadosGW users data source")

	// Get all users
	users, err := d.client.Admin.GetUsers(ctx)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading RadosGW Users",
			fmt.Sprintf("Could not list users: %s", err.Error()),
		)
		return
	}

	// Filter by regex if provided
	var filteredUsers []string
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

		for _, userID := range *users {
			if re.MatchString(userID) {
				filteredUsers = append(filteredUsers, userID)
			}
		}

		tflog.Debug(ctx, "Filtered users by regex", map[string]any{
			"pattern":       pattern,
			"total_users":   len(*users),
			"matched_users": len(filteredUsers),
		})
	} else {
		filteredUsers = *users
		tflog.Debug(ctx, "Returning all users", map[string]any{
			"total_users": len(filteredUsers),
		})
	}

	// Convert to set
	userIDSet, diags := types.SetValueFrom(ctx, types.StringType, filteredUsers)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	config.UserIDs = userIDSet
	config.ID = types.StringValue("radosgw-users")

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
