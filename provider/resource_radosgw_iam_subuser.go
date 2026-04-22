package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ceph/go-ceph/rgw/admin"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// accessToAPI converts user-friendly access format to API format
// Uses go-ceph SubuserAccess constants for API parameters
// read-write -> readwrite (SubuserAccessReadWrite)
// full-control -> full (SubuserAccessFull)
func accessToAPI(access string) string {
	switch access {
	case string(admin.SubuserAccessReplyReadWrite): // "read-write"
		return string(admin.SubuserAccessReadWrite) // "readwrite"
	case string(admin.SubuserAccessReplyFull): // "full-control"
		return string(admin.SubuserAccessFull) // "full"
	default:
		return access // read, write, or empty
	}
}

// accessFromAPI converts API response format to user-friendly format
// Ceph returns SubuserAccessReply* constants: "read-write", "full-control", etc.
// We keep those as-is since they're already user-friendly
func accessFromAPI(access string) string {
	// Ceph already returns SubuserAccessReply* format (read-write, full-control)
	return access
}

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &SubuserResource{}
	_ resource.ResourceWithImportState = &SubuserResource{}
)

func NewIAMSubuserResource() resource.Resource {
	return &SubuserResource{}
}

// SubuserResource defines the resource implementation.
type SubuserResource struct {
	client *RadosgwClient
}

// SubuserResourceModel describes the resource data model.
type SubuserResourceModel struct {
	UserID    types.String `tfsdk:"user_id"`
	AccountID types.String `tfsdk:"account_id"`
	Subuser   types.String `tfsdk:"subuser"`
	Access    types.String `tfsdk:"access"`
	SecretKey types.String `tfsdk:"secret_key"`
	FullID    types.String `tfsdk:"id"`
}

func (r *SubuserResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iam_subuser"
}

func (r *SubuserResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a RadosGW subuser. Subusers are additional identities under a parent user used for Swift API access. The full subuser ID has the format `{user_id}:{subuser}`. " +
			"~> **Note:** Ceph automatically generates one Swift secret key when creating a subuser (only one Swift key is allowed per subuser). " +
			"Keys can be managed separately or replaced later using the `radosgw_iam_access_key` resource with `key_type = \"swift\"`.\n\n" +
			"~> **Note:** Creating multiple subusers for a single user requires Ceph Squid (19.x) or higher. " +
			"Older versions (Reef 18.x) may have issues with multiple subuser creation.",

		Attributes: map[string]schema.Attribute{
			"user_id": schema.StringAttribute{
				MarkdownDescription: "The parent user ID.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"account_id": schema.StringAttribute{
				MarkdownDescription: "The account ID to associate the subuser with. When specified, the subuser is created within the account context.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"subuser": schema.StringAttribute{
				MarkdownDescription: "The subuser name (without the parent user prefix). The full subuser ID will be `{user_id}:{subuser}`.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"access": schema.StringAttribute{
				MarkdownDescription: "Access level for the subuser. Valid values: `read`, `write`, `read-write`, `full-control`. Default: `read`.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("read"),
				Validators: []validator.String{
					stringvalidator.OneOf("read", "write", "read-write", "full-control"),
				},
			},
			"secret_key": schema.StringAttribute{
				MarkdownDescription: "The auto-generated Swift secret key. This is the initial key created by Ceph when the subuser is created. " +
					"~> **Note:** For production use, consider managing keys explicitly with the `radosgw_iam_access_key` resource for rotation and lifecycle management. " +
					"This field is computed (read-only) and will not detect or track external key changes.",
				Computed:  true,
				Sensitive: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "The full subuser ID in the format `{user_id}:{subuser}`.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *SubuserResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*RadosgwClient)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *RadosgwClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *SubuserResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SubuserResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	userID := data.UserID.ValueString()
	resolvedUserID, err := resolveUserID(ctx, r.client, userID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Resolving User",
			fmt.Sprintf("Could not resolve parent user %s for subuser creation: %s", userID, err.Error()),
		)
		return
	}
	fullSubuserID := resolvedUserID + ":" + data.Subuser.ValueString()

	tflog.Debug(ctx, "Creating subuser", map[string]any{
		"user_id":          userID,
		"resolved_user_id": resolvedUserID,
		"account_id":       data.AccountID.ValueString(),
		"subuser":          data.Subuser.ValueString(),
		"full_id":          fullSubuserID,
		"access":           data.Access.ValueString(),
	})

	// Create subuser
	// Note: Ceph generates Swift keys by default, regardless of generate-secret parameter
	// We leave the field as nil since setting it to false doesn't prevent key generation
	subuser := admin.SubuserSpec{
		Name:   fullSubuserID,
		Access: admin.SubuserAccess(accessToAPI(data.Access.ValueString())),
	}

	user := admin.User{
		ID:        resolvedUserID,
		AccountID: data.AccountID.ValueString(),
	}

	tflog.Debug(ctx, "CreateSubuser parameters", map[string]any{
		"subuser_spec": fmt.Sprintf("%+v", subuser),
		"user":         fmt.Sprintf("%+v", user),
	})

	err = retryOnConcurrentModification(ctx, fmt.Sprintf("CreateSubuser %s", fullSubuserID), func() error {
		return r.client.Admin.CreateSubuser(ctx, user, subuser)
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Subuser",
			fmt.Sprintf("Could not create subuser %s: %s", fullSubuserID, err.Error()),
		)
		return
	}

	// Fetch the user to get the auto-generated Swift secret key
	// Architecture note: We expose the auto-generated key as a computed attribute for simple use cases.
	// For production deployments with key rotation requirements, users should manage keys explicitly
	// using the separate radosgw_iam_access_key resource. This gives users both simplicity (auto-key) and
	// control (explicit key management) following Terraform best practices (similar to AWS IAM user vs access key).
	userInfo, err := r.client.Admin.GetUser(ctx, admin.User{ID: resolvedUserID})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading User After Subuser Creation",
			fmt.Sprintf("Could not read user %s to retrieve secret key: %s", userID, err.Error()),
		)
		return
	}

	// Find and store the auto-generated Swift secret key
	// Note: Ceph automatically generates exactly one Swift key per subuser upon creation
	for _, key := range userInfo.SwiftKeys {
		if key.User == fullSubuserID {
			data.SecretKey = types.StringValue(key.SecretKey)
			tflog.Debug(ctx, "Retrieved auto-generated Swift secret key", map[string]any{
				"subuser": fullSubuserID,
			})
			break
		}
	}

	// Set computed fields
	data.FullID = types.StringValue(fullSubuserID)
	data.AccountID = types.StringValue(userInfo.AccountID)

	tflog.Trace(ctx, "Created subuser")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SubuserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data SubuserResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	userID := data.UserID.ValueString()
	resolvedUserID, err := resolveUserID(ctx, r.client, userID)
	if err != nil {
		// If user doesn't exist, remove subuser from state
		if errors.Is(err, admin.ErrNoSuchUser) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Resolving User",
			fmt.Sprintf("Could not resolve parent user %s for subuser read: %s", userID, err.Error()),
		)
		return
	}
	fullSubuserID := resolvedUserID + ":" + data.Subuser.ValueString()

	tflog.Debug(ctx, "Reading subuser", map[string]any{
		"user_id":          userID,
		"resolved_user_id": resolvedUserID,
		"full_id":          fullSubuserID,
	})

	// Get the parent user to check subusers
	user, err := r.client.Admin.GetUser(ctx, admin.User{ID: resolvedUserID})
	if err != nil {
		// If user doesn't exist, remove subuser from state
		if errors.Is(err, admin.ErrNoSuchUser) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading User",
			fmt.Sprintf("Could not read user %s: %s", userID, err.Error()),
		)
		return
	}

	// Find the subuser in the user's subusers list
	found := false
	for _, subuser := range user.Subusers {
		if subuser.Name == fullSubuserID {
			found = true
			// Update access from current state
			if subuser.Access != "" {
				data.Access = types.StringValue(accessFromAPI(string(subuser.Access)))
			}
			break
		}
	}

	if !found {
		// Subuser doesn't exist, remove from state
		resp.State.RemoveResource(ctx)
		return
	}

	// Fetch the Swift secret key from the user's keys
	// Note: We only read the key that exists in state. If the key was externally rotated
	// (e.g., via radosgw_iam_access_key resource or manual admin commands), this will detect the change.
	for _, key := range user.SwiftKeys {
		if key.User == fullSubuserID {
			data.SecretKey = types.StringValue(key.SecretKey)
			break
		}
	}

	// Ensure computed fields are set
	data.FullID = types.StringValue(fullSubuserID)
	data.AccountID = types.StringValue(user.AccountID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SubuserResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data SubuserResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Read current state to preserve computed fields (id, secret_key) that don't change during update
	var state SubuserResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	userID := data.UserID.ValueString()
	resolvedUserID, err := resolveUserID(ctx, r.client, userID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Resolving User",
			fmt.Sprintf("Could not resolve parent user %s for subuser update: %s", userID, err.Error()),
		)
		return
	}
	fullSubuserID := resolvedUserID + ":" + data.Subuser.ValueString()

	tflog.Debug(ctx, "Updating subuser", map[string]any{
		"user_id":          userID,
		"resolved_user_id": resolvedUserID,
		"account_id":       data.AccountID.ValueString(),
		"full_id":          fullSubuserID,
		"access":           data.Access.ValueString(),
	})

	// Modify subuser (only access can be updated)
	subuser := admin.SubuserSpec{
		Name:   fullSubuserID,
		Access: admin.SubuserAccess(accessToAPI(data.Access.ValueString())),
	}

	user := admin.User{
		ID:        resolvedUserID,
		AccountID: data.AccountID.ValueString(),
	}

	err = retryOnConcurrentModification(ctx, fmt.Sprintf("ModifySubuser %s", fullSubuserID), func() error {
		return r.client.Admin.ModifySubuser(ctx, user, subuser)
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Subuser",
			fmt.Sprintf("Could not update subuser %s: %s", fullSubuserID, err.Error()),
		)
		return
	}

	// Preserve the key while refreshing the computed full ID from the resolved user.
	data.FullID = types.StringValue(fullSubuserID)
	data.AccountID = state.AccountID
	data.SecretKey = state.SecretKey

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SubuserResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data SubuserResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	userID := data.UserID.ValueString()
	resolvedUserID, err := resolveUserID(ctx, r.client, userID)
	if err != nil {
		if errors.Is(err, admin.ErrNoSuchUser) {
			return
		}
		resp.Diagnostics.AddError(
			"Error Resolving User",
			fmt.Sprintf("Could not resolve parent user %s for subuser deletion: %s", userID, err.Error()),
		)
		return
	}
	fullSubuserID := resolvedUserID + ":" + data.Subuser.ValueString()

	tflog.Debug(ctx, "Deleting subuser", map[string]any{
		"user_id":          userID,
		"resolved_user_id": resolvedUserID,
		"full_id":          fullSubuserID,
	})

	purgeKeys := true
	subuser := admin.SubuserSpec{
		Name:      fullSubuserID,
		PurgeKeys: &purgeKeys, // Purge associated keys
	}

	user := admin.User{
		ID:        resolvedUserID,
		AccountID: data.AccountID.ValueString(),
	}

	err = retryOnConcurrentModification(ctx, fmt.Sprintf("RemoveSubuser %s", fullSubuserID), func() error {
		return r.client.Admin.RemoveSubuser(ctx, user, subuser)
	})
	if err != nil {
		// Ignore error if user or subuser doesn't exist
		if !errors.Is(err, admin.ErrNoSuchUser) && !errors.Is(err, admin.ErrNoSuchSubUser) {
			resp.Diagnostics.AddError(
				"Error Deleting Subuser",
				fmt.Sprintf("Could not delete subuser %s: %s", fullSubuserID, err.Error()),
			)
			return
		}
	}
}

func (r *SubuserResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import format: "user_id:subuser" (the full subuser ID)
	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Import ID must be in the format 'user_id:subuser'. Example: 'myuser:swift'",
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("user_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("subuser"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
