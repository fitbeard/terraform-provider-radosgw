package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/ceph/go-ceph/rgw/admin"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &SubusersDataSource{}

func NewIAMSubusersDataSource() datasource.DataSource {
	return &SubusersDataSource{}
}

// SubusersDataSource defines the data source implementation.
type SubusersDataSource struct {
	client *RadosgwClient
}

// SubusersDataSourceModel describes the data source data model.
type SubusersDataSourceModel struct {
	UserID   types.String `tfsdk:"user_id"`
	Subusers types.List   `tfsdk:"subusers"`
	ID       types.String `tfsdk:"id"`
}

// SubuserModel represents a single subuser in the list.
type SubuserModel struct {
	ID     types.String `tfsdk:"id"`
	Name   types.String `tfsdk:"name"`
	Access types.String `tfsdk:"access"`
}

func (d *SubusersDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iam_subusers"
}

func (d *SubusersDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves information about subusers associated with a RadosGW user.\n\n" +
			"Subusers are additional identities under a parent user, typically used for Swift API access. " +
			"Each subuser has a full ID in the format `{user_id}:{subuser_name}`.\n\n" +
			"~> **Note:** Listing multiple subusers per user requires Ceph Squid (19.x) or higher. " +
			"Older versions (Reef 18.x) may have issues when multiple subusers exist.",

		Attributes: map[string]schema.Attribute{
			"user_id": schema.StringAttribute{
				MarkdownDescription: "The parent user ID to retrieve subusers for.",
				Required:            true,
			},
			"subusers": schema.ListNestedAttribute{
				MarkdownDescription: "List of subusers associated with the user.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "The full subuser ID in the format `{user_id}:{subuser_name}`.",
							Computed:            true,
						},
						"name": schema.StringAttribute{
							MarkdownDescription: "The subuser name (without the parent user prefix).",
							Computed:            true,
						},
						"access": schema.StringAttribute{
							MarkdownDescription: "The access level: `read`, `write`, `read-write`, or `full-control`.",
							Computed:            true,
						},
					},
				},
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "The data source identifier (same as user_id).",
				Computed:            true,
			},
		},
	}
}

func (d *SubusersDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *SubusersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config SubusersDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	userID := config.UserID.ValueString()

	tflog.Debug(ctx, "Reading RadosGW subusers", map[string]any{
		"user_id": userID,
	})

	// Get user to retrieve subusers
	user, err := d.client.Admin.GetUser(ctx, admin.User{ID: userID})
	if err != nil {
		if errors.Is(err, admin.ErrNoSuchUser) {
			resp.Diagnostics.AddError(
				"User Not Found",
				fmt.Sprintf("User %q does not exist.", userID),
			)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading Subusers",
			fmt.Sprintf("Could not read user %q: %s", userID, err.Error()),
		)
		return
	}

	tflog.Debug(ctx, "Found subusers", map[string]any{
		"user_id": userID,
		"count":   len(user.Subusers),
	})

	// Build list of subusers
	subusers := make([]SubuserModel, 0, len(user.Subusers))
	for _, subuser := range user.Subusers {
		// Extract just the subuser name from the full ID (user_id:subuser_name)
		name := subuser.Name
		if idx := len(userID) + 1; len(name) > idx {
			name = subuser.Name[idx:]
		}

		subusers = append(subusers, SubuserModel{
			ID:     types.StringValue(subuser.Name),
			Name:   types.StringValue(name),
			Access: types.StringValue(accessFromAPI(string(subuser.Access))),
		})
	}

	// Convert to list type
	subusersList, diags := types.ListValueFrom(ctx, types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"id":     types.StringType,
			"name":   types.StringType,
			"access": types.StringType,
		},
	}, subusers)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	config.Subusers = subusersList
	config.ID = types.StringValue(userID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
