package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/ceph/go-ceph/rgw/admin"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &UserDataSource{}

func NewIAMUserDataSource() datasource.DataSource {
	return &UserDataSource{}
}

// UserDataSource defines the data source implementation.
type UserDataSource struct {
	client *RadosgwClient
}

// UserDataSourceModel describes the data source data model.
type UserDataSourceModel struct {
	UserID              types.String `tfsdk:"user_id"`
	DisplayName         types.String `tfsdk:"display_name"`
	Email               types.String `tfsdk:"email"`
	Tenant              types.String `tfsdk:"tenant"`
	MaxBuckets          types.Int64  `tfsdk:"max_buckets"`
	Suspended           types.Bool   `tfsdk:"suspended"`
	OpMask              types.String `tfsdk:"op_mask"`
	DefaultPlacement    types.String `tfsdk:"default_placement"`
	DefaultStorageClass types.String `tfsdk:"default_storage_class"`
	Type                types.String `tfsdk:"type"`
}

func (d *UserDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iam_user"
}

func (d *UserDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves information about an existing RadosGW user. " +
			"Use this data source to get details about a user without having to hard code user IDs.",

		Attributes: map[string]schema.Attribute{
			"user_id": schema.StringAttribute{
				MarkdownDescription: "The user ID to look up. For users in a tenant, use the format `tenant$user_id`.",
				Required:            true,
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "The display name of the user.",
				Computed:            true,
			},
			"email": schema.StringAttribute{
				MarkdownDescription: "The email address of the user.",
				Computed:            true,
			},
			"tenant": schema.StringAttribute{
				MarkdownDescription: "The tenant to which the user belongs.",
				Computed:            true,
			},
			"max_buckets": schema.Int64Attribute{
				MarkdownDescription: "The maximum number of buckets the user can own.",
				Computed:            true,
			},
			"suspended": schema.BoolAttribute{
				MarkdownDescription: "Whether the user is suspended.",
				Computed:            true,
			},
			"op_mask": schema.StringAttribute{
				MarkdownDescription: "The operation mask for the user (e.g., 'read, write, delete').",
				Computed:            true,
			},
			"default_placement": schema.StringAttribute{
				MarkdownDescription: "The default placement for the user's buckets.",
				Computed:            true,
			},
			"default_storage_class": schema.StringAttribute{
				MarkdownDescription: "The default storage class for the user's objects.",
				Computed:            true,
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "The user type (e.g., 'rgw', 'ldap').",
				Computed:            true,
			},
		},
	}
}

func (d *UserDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *UserDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config UserDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	userID := config.UserID.ValueString()

	tflog.Debug(ctx, "Reading RadosGW user data source", map[string]any{
		"user_id": userID,
	})

	// Get user info
	user, err := d.client.Admin.GetUser(ctx, admin.User{ID: userID})
	if err != nil {
		if errors.Is(err, admin.ErrNoSuchUser) {
			resp.Diagnostics.AddError(
				"User Not Found",
				fmt.Sprintf("User with ID %q does not exist.", userID),
			)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading RadosGW User",
			fmt.Sprintf("Could not read user %s: %s", userID, err.Error()),
		)
		return
	}

	// Populate the model
	config.UserID = types.StringValue(user.ID)
	config.DisplayName = types.StringValue(user.DisplayName)
	config.Email = types.StringValue(user.Email)
	config.Tenant = types.StringValue(user.Tenant)
	config.MaxBuckets = types.Int64Value(int64(*user.MaxBuckets))
	config.Suspended = types.BoolValue(*user.Suspended != 0)
	config.OpMask = types.StringValue(user.OpMask)
	config.DefaultPlacement = types.StringValue(user.DefaultPlacement)
	config.DefaultStorageClass = types.StringValue(user.DefaultStorageClass)
	config.Type = types.StringValue(user.Type)

	tflog.Trace(ctx, "Read user data source", map[string]any{
		"user_id":      user.ID,
		"display_name": user.DisplayName,
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
