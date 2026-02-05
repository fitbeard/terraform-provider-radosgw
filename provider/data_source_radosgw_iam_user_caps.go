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
var _ datasource.DataSource = &UserCapsDataSource{}

func NewIAMUserCapsDataSource() datasource.DataSource {
	return &UserCapsDataSource{}
}

// UserCapsDataSource defines the data source implementation.
type UserCapsDataSource struct {
	client *RadosgwClient
}

// UserCapsDataSourceModel describes the data source data model.
type UserCapsDataSourceModel struct {
	UserID types.String `tfsdk:"user_id"`
	Caps   types.Set    `tfsdk:"caps"`
	ID     types.String `tfsdk:"id"`
}

func (d *UserCapsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iam_user_caps"
}

func (d *UserCapsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves the capabilities (administrative permissions) for a RadosGW user.\n\n" +
			"Capabilities control what administrative operations a user can perform. " +
			"Each capability has a type (e.g., `users`, `buckets`) and a permission level (`read`, `write`, or `*`).",

		Attributes: map[string]schema.Attribute{
			"user_id": schema.StringAttribute{
				MarkdownDescription: "The user ID to retrieve capabilities for.",
				Required:            true,
			},
			"caps": schema.SetNestedAttribute{
				MarkdownDescription: "Set of capabilities assigned to the user.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"type": schema.StringAttribute{
							MarkdownDescription: "The capability type (e.g., `users`, `buckets`, `metadata`, `usage`, `zone`, `info`, `accounts`, `ratelimit`, `roles`, `user-policy`, `amz-cache`, `oidc-provider`, `bilog`, `mdlog`, `datalog`).",
							Computed:            true,
						},
						"perm": schema.StringAttribute{
							MarkdownDescription: "The permission level: `*` (full access), `read`, or `write`.",
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

func (d *UserCapsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *UserCapsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config UserCapsDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	userID := config.UserID.ValueString()

	tflog.Debug(ctx, "Reading RadosGW user capabilities", map[string]any{
		"user_id": userID,
	})

	// Get user to retrieve capabilities
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
			"Error Reading User Capabilities",
			fmt.Sprintf("Could not read user %q: %s", userID, err.Error()),
		)
		return
	}

	tflog.Debug(ctx, "Found user capabilities", map[string]any{
		"user_id": userID,
		"count":   len(user.Caps),
	})

	// Convert Ceph caps to Terraform set
	capsSet, err := cephCapsToSet(ctx, user.Caps)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Converting Capabilities",
			fmt.Sprintf("Could not convert capabilities from Ceph: %s", err.Error()),
		)
		return
	}

	// If no caps, return empty set instead of null
	if capsSet.IsNull() {
		capAttrTypes := map[string]attr.Type{
			"type": types.StringType,
			"perm": types.StringType,
		}
		capsSet, _ = types.SetValue(types.ObjectType{AttrTypes: capAttrTypes}, []attr.Value{})
	}

	config.Caps = capsSet
	config.ID = types.StringValue(userID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
