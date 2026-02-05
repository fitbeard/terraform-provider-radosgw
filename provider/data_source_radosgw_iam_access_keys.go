package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/ceph/go-ceph/rgw/admin"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &AccessKeysDataSource{}

func NewIAMAccessKeysDataSource() datasource.DataSource {
	return &AccessKeysDataSource{}
}

// AccessKeysDataSource defines the data source implementation.
type AccessKeysDataSource struct {
	client *RadosgwClient
}

// AccessKeysDataSourceModel describes the data source data model.
type AccessKeysDataSourceModel struct {
	UserID     types.String `tfsdk:"user_id"`
	KeyType    types.String `tfsdk:"key_type"`
	AccessKeys types.List   `tfsdk:"access_keys"`
	ID         types.String `tfsdk:"id"`
}

// AccessKeyModel represents a single access key in the list.
type AccessKeyModel struct {
	AccessKeyID types.String `tfsdk:"access_key_id"`
	User        types.String `tfsdk:"user"`
	KeyType     types.String `tfsdk:"key_type"`
}

func (d *AccessKeysDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iam_access_keys"
}

func (d *AccessKeysDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves information about IAM access keys associated with the specified RadosGW user. " +
			"Use this data source to get S3 and/or Swift access keys for a user.\n\n" +
			"**Note:** The RadosGW API also returns `active` and `create_date` fields, but these are not yet " +
			"exposed by the go-ceph library. They will be added in a future version when go-ceph supports them.\n\n" +
			"**Note:** Listing multiple S3 keys per user requires Ceph Squid (19.x) or higher. " +
			"Older versions (Reef 18.x) may have issues when multiple keys exist.",

		Attributes: map[string]schema.Attribute{
			"user_id": schema.StringAttribute{
				MarkdownDescription: "The user ID to retrieve access keys for.",
				Required:            true,
			},
			"key_type": schema.StringAttribute{
				MarkdownDescription: "Filter by key type. Valid values: `s3`, `swift`. If not specified, returns both S3 and Swift keys.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("s3", "swift"),
				},
			},
			"access_keys": schema.ListNestedAttribute{
				MarkdownDescription: "List of access keys associated with the user.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"access_key_id": schema.StringAttribute{
							MarkdownDescription: "The access key ID. For S3 keys this is the access key. For Swift keys this is `user_id:subuser`.",
							Computed:            true,
						},
						"user": schema.StringAttribute{
							MarkdownDescription: "The user or subuser ID associated with this key. For S3 keys this is the user ID. For Swift keys this is `user_id:subuser`.",
							Computed:            true,
						},
						"key_type": schema.StringAttribute{
							MarkdownDescription: "The type of key: `s3` or `swift`.",
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

func (d *AccessKeysDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *AccessKeysDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config AccessKeysDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	userID := config.UserID.ValueString()
	keyTypeFilter := config.KeyType.ValueString() // Empty string if not set

	tflog.Debug(ctx, "Reading RadosGW access keys", map[string]any{
		"user_id":  userID,
		"key_type": keyTypeFilter,
	})

	// Get user to retrieve keys
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
			"Error Reading Access Keys",
			fmt.Sprintf("Could not read user %q: %s", userID, err.Error()),
		)
		return
	}

	// Build list of access keys
	accessKeys := make([]AccessKeyModel, 0)

	// Add S3 keys if not filtered to swift only
	if keyTypeFilter == "" || keyTypeFilter == "s3" {
		for _, key := range user.Keys {
			accessKeys = append(accessKeys, AccessKeyModel{
				AccessKeyID: types.StringValue(key.AccessKey),
				User:        types.StringValue(key.User),
				KeyType:     types.StringValue("s3"),
			})
		}
	}

	// Add Swift keys if not filtered to s3 only
	if keyTypeFilter == "" || keyTypeFilter == "swift" {
		for _, key := range user.SwiftKeys {
			accessKeys = append(accessKeys, AccessKeyModel{
				AccessKeyID: types.StringValue(key.User), // Swift key ID is user:subuser
				User:        types.StringValue(key.User),
				KeyType:     types.StringValue("swift"),
			})
		}
	}

	tflog.Debug(ctx, "Found access keys", map[string]any{
		"user_id":     userID,
		"key_type":    keyTypeFilter,
		"s3_count":    len(user.Keys),
		"swift_count": len(user.SwiftKeys),
		"total":       len(accessKeys),
	})

	// Convert to list type
	accessKeysList, diags := types.ListValueFrom(ctx, types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"access_key_id": types.StringType,
			"user":          types.StringType,
			"key_type":      types.StringType,
		},
	}, accessKeys)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	config.AccessKeys = accessKeysList
	config.ID = types.StringValue(userID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
