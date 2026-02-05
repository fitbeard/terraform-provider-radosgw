package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/ceph/go-ceph/rgw/admin"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// keyCreationMutexes serializes key creation per user to prevent race conditions
// when multiple auto-generated S3 keys are created in parallel.
var keyCreationMutexes sync.Map

func getKeyMutex(userID string) *sync.Mutex {
	mutex, _ := keyCreationMutexes.LoadOrStore(userID, &sync.Mutex{})
	return mutex.(*sync.Mutex)
}

var _ resource.Resource = &KeyResource{}
var _ resource.ResourceWithImportState = &KeyResource{}
var _ resource.ResourceWithModifyPlan = &KeyResource{}

func NewIAMAcessKeyResource() resource.Resource {
	return &KeyResource{}
}

type KeyResource struct {
	client *RadosgwClient
}

type KeyResourceModel struct {
	ID        types.String `tfsdk:"id"`
	UserID    types.String `tfsdk:"user_id"`
	SubUser   types.String `tfsdk:"subuser"`
	KeyType   types.String `tfsdk:"key_type"`
	AccessKey types.String `tfsdk:"access_key"`
	SecretKey types.String `tfsdk:"secret_key"`
	Generated types.Bool   `tfsdk:"generated"`
}

func (r *KeyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iam_access_key"
}

func (r *KeyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an IAM access key for S3 or Swift access in RadosGW.\n\n" +
			"**S3 keys:** Multiple access keys per user are supported. Keys are identified by `access_key`.\n\n" +
			"**Swift keys:** Only one access key per subuser is supported. Creating a new key replaces the existing one. " +
			"Requires a `subuser` attribute.\n\n" +
			"**Note:** Managing multiple S3 keys per user requires Ceph Squid (19.x) or higher. " +
			"Older versions (Reef 18.x) may have issues with key deletion when multiple keys exist.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The resource identifier. For S3 keys: the `access_key`. For Swift keys: `user_id:subuser`.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"user_id": schema.StringAttribute{
				MarkdownDescription: "The user ID for which to create the key.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"subuser": schema.StringAttribute{
				MarkdownDescription: "The subuser name (without the user prefix). Required for Swift keys, not used for S3 keys.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"key_type": schema.StringAttribute{
				MarkdownDescription: "The type of key. Valid values: `s3` (default), `swift`.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("s3"),
				Validators: []validator.String{
					stringvalidator.OneOf("s3", "swift"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"access_key": schema.StringAttribute{
				MarkdownDescription: "The access key. For S3 keys: if not provided, it will be auto-generated. For Swift keys: this is computed as `user_id:subuser`. Changing this value will force resource replacement.",
				Optional:            true,
				Computed:            true,
				Sensitive:           true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"secret_key": schema.StringAttribute{
				MarkdownDescription: "The secret key. If not provided, it will be auto-generated. Changing this value will update the key in place.",
				Optional:            true,
				Computed:            true,
				Sensitive:           true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"generated": schema.BoolAttribute{
				MarkdownDescription: "Whether the key was auto-generated (true) or user-specified (false). Only applicable for S3 keys.",
				Computed:            true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *KeyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*RadosgwClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *RadosgwClient, got: %T", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *KeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data KeyResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	keyType := data.KeyType.ValueString()

	// Validate Swift key requirements
	if keyType == "swift" {
		if data.SubUser.IsNull() || data.SubUser.ValueString() == "" {
			resp.Diagnostics.AddError(
				"Missing Required Field",
				"subuser is required when key_type is 'swift'",
			)
			return
		}
	}

	tflog.Debug(ctx, "Creating RadosGW key", map[string]any{
		"user_id":  data.UserID.ValueString(),
		"key_type": keyType,
		"subuser":  data.SubUser.ValueString(),
	})

	if keyType == "swift" {
		r.createSwiftKey(ctx, &data, resp)
	} else {
		r.createS3Key(ctx, &data, resp)
	}
}

func (r *KeyResource) createS3Key(ctx context.Context, data *KeyResourceModel, resp *resource.CreateResponse) {
	userMutex := getKeyMutex(data.UserID.ValueString())
	userMutex.Lock()
	defer userMutex.Unlock()

	// Snapshot existing keys to identify newly created auto-generated key
	var existingAccessKeys map[string]bool
	if data.AccessKey.IsNull() || data.AccessKey.ValueString() == "" {
		existingAccessKeys = make(map[string]bool)
		user, err := r.client.Admin.GetUser(ctx, admin.User{ID: data.UserID.ValueString()})
		if err == nil {
			for _, key := range user.Keys {
				existingAccessKeys[key.AccessKey] = true
			}
		}
	}

	generateKey := true
	keySpec := admin.UserKeySpec{
		UID:         data.UserID.ValueString(),
		KeyType:     "s3",
		GenerateKey: &generateKey,
	}

	if !data.AccessKey.IsNull() && data.AccessKey.ValueString() != "" {
		keySpec.AccessKey = data.AccessKey.ValueString()
	}
	if !data.SecretKey.IsNull() && data.SecretKey.ValueString() != "" {
		keySpec.SecretKey = data.SecretKey.ValueString()
	}

	var keys *[]admin.UserKeySpec
	err := retryOnConcurrentModification(ctx, fmt.Sprintf("CreateKey %s", data.UserID.ValueString()), func() error {
		var createErr error
		keys, createErr = r.client.Admin.CreateKey(ctx, keySpec)
		return createErr
	})

	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating S3 Key",
			fmt.Sprintf("Could not create key for user %s: %s", data.UserID.ValueString(), err.Error()),
		)
		return
	}

	if keys == nil || len(*keys) == 0 {
		resp.Diagnostics.AddError(
			"Error Creating S3 Key",
			"No keys returned from CreateKey operation",
		)
		return
	}

	var createdKey *admin.UserKeySpec
	if !data.AccessKey.IsNull() && data.AccessKey.ValueString() != "" {
		for i := range *keys {
			key := &(*keys)[i]
			if key.AccessKey == data.AccessKey.ValueString() {
				createdKey = key
				break
			}
		}
	} else {
		for i := range *keys {
			key := &(*keys)[i]
			if key.User == data.UserID.ValueString() && !existingAccessKeys[key.AccessKey] {
				createdKey = key
				break
			}
		}
	}

	if createdKey == nil {
		resp.Diagnostics.AddError(
			"Error Creating S3 Key",
			"Could not find created key in response",
		)
		return
	}

	wasGenerated := data.AccessKey.IsNull() || data.AccessKey.ValueString() == ""

	data.AccessKey = types.StringValue(createdKey.AccessKey)
	data.SecretKey = types.StringValue(createdKey.SecretKey)
	data.ID = types.StringValue(createdKey.AccessKey)
	data.Generated = types.BoolValue(wasGenerated)

	tflog.Trace(ctx, "Created S3 key")
	resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
}

func (r *KeyResource) createSwiftKey(ctx context.Context, data *KeyResourceModel, resp *resource.CreateResponse) {
	fullSubuserID := fmt.Sprintf("%s:%s", data.UserID.ValueString(), data.SubUser.ValueString())

	tflog.Debug(ctx, "Creating Swift key", map[string]any{
		"user_id": data.UserID.ValueString(),
		"subuser": data.SubUser.ValueString(),
		"full_id": fullSubuserID,
	})

	generateKey := true
	keySpec := admin.UserKeySpec{
		UID:         data.UserID.ValueString(),
		SubUser:     data.SubUser.ValueString(),
		KeyType:     "swift",
		GenerateKey: &generateKey,
	}

	if !data.SecretKey.IsNull() && data.SecretKey.ValueString() != "" {
		keySpec.SecretKey = data.SecretKey.ValueString()
	}

	var keys *[]admin.UserKeySpec
	err := retryOnConcurrentModification(ctx, fmt.Sprintf("CreateKey %s", fullSubuserID), func() error {
		var createErr error
		keys, createErr = r.client.Admin.CreateKey(ctx, keySpec)
		return createErr
	})

	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Swift Key",
			fmt.Sprintf("Could not create Swift key for subuser %s: %s", fullSubuserID, err.Error()),
		)
		return
	}

	if keys == nil || len(*keys) == 0 {
		resp.Diagnostics.AddError(
			"Error Creating Swift Key",
			"No keys returned from CreateKey operation",
		)
		return
	}

	var createdKey *admin.UserKeySpec
	for i := range *keys {
		key := &(*keys)[i]
		if key.User == fullSubuserID {
			createdKey = key
			break
		}
	}

	if createdKey == nil {
		resp.Diagnostics.AddError(
			"Error Creating Swift Key",
			fmt.Sprintf("Could not find created Swift key in response for %s", fullSubuserID),
		)
		return
	}

	data.AccessKey = types.StringValue(createdKey.User)
	data.SecretKey = types.StringValue(createdKey.SecretKey)
	data.ID = types.StringValue(fullSubuserID)
	data.Generated = types.BoolNull()

	tflog.Trace(ctx, "Created Swift key")
	resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
}

func (r *KeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data KeyResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	keyType := data.KeyType.ValueString()

	tflog.Debug(ctx, "Reading RadosGW key", map[string]any{
		"user_id":  data.UserID.ValueString(),
		"key_type": keyType,
	})

	user, err := r.client.Admin.GetUser(ctx, admin.User{ID: data.UserID.ValueString()})
	if err != nil {
		if errors.Is(err, admin.ErrNoSuchUser) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading Key",
			fmt.Sprintf("Could not read user %s: %s", data.UserID.ValueString(), err.Error()),
		)
		return
	}

	if keyType == "swift" {
		fullSubuserID := fmt.Sprintf("%s:%s", data.UserID.ValueString(), data.SubUser.ValueString())
		found := false
		for i := range user.SwiftKeys {
			swiftKey := &user.SwiftKeys[i]
			if swiftKey.User == fullSubuserID {
				found = true
				break
			}
		}

		if !found {
			tflog.Debug(ctx, "Swift key not found, removing from state", map[string]any{
				"user_id": data.UserID.ValueString(),
				"full_id": fullSubuserID,
			})
			resp.State.RemoveResource(ctx)
			return
		}
	} else {
		found := false
		for i := range user.Keys {
			key := &user.Keys[i]
			if key.AccessKey == data.AccessKey.ValueString() {
				found = true
				break
			}
		}

		if !found {
			tflog.Debug(ctx, "S3 key not found, removing from state", map[string]any{
				"user_id":    data.UserID.ValueString(),
				"access_key": data.AccessKey.ValueString(),
			})
			resp.State.RemoveResource(ctx)
			return
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *KeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state KeyResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Only secret_key can be updated in place
	if !plan.SecretKey.Equal(state.SecretKey) {
		keyType := state.KeyType.ValueString()

		tflog.Debug(ctx, "Updating secret_key in place", map[string]any{
			"user_id":  state.UserID.ValueString(),
			"key_type": keyType,
		})

		generateKey := false
		keySpec := admin.UserKeySpec{
			UID:         state.UserID.ValueString(),
			KeyType:     keyType,
			SecretKey:   plan.SecretKey.ValueString(),
			GenerateKey: &generateKey,
		}

		if keyType == "swift" {
			keySpec.SubUser = state.SubUser.ValueString()
		} else {
			keySpec.AccessKey = state.AccessKey.ValueString()
		}

		err := retryOnConcurrentModification(ctx, fmt.Sprintf("UpdateKey %s", state.ID.ValueString()), func() error {
			_, createErr := r.client.Admin.CreateKey(ctx, keySpec)
			return createErr
		})

		if err != nil {
			resp.Diagnostics.AddError(
				"Error Updating Secret Key",
				fmt.Sprintf("Could not update secret key: %s", err.Error()),
			)
			return
		}

		tflog.Trace(ctx, "Updated secret_key in place")
	}

	// Copy plan to state (preserving computed values that didn't change)
	plan.ID = state.ID
	plan.AccessKey = state.AccessKey
	plan.Generated = state.Generated

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *KeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data KeyResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	keyType := data.KeyType.ValueString()

	tflog.Debug(ctx, "Deleting RadosGW key", map[string]any{
		"user_id":  data.UserID.ValueString(),
		"key_type": keyType,
	})

	keySpec := admin.UserKeySpec{
		UID:     data.UserID.ValueString(),
		KeyType: keyType,
	}

	if keyType == "swift" {
		keySpec.SubUser = data.SubUser.ValueString()
	} else {
		keySpec.AccessKey = data.AccessKey.ValueString()
	}

	err := retryOnConcurrentModification(ctx, fmt.Sprintf("RemoveKey %s", data.ID.ValueString()), func() error {
		return r.client.Admin.RemoveKey(ctx, keySpec)
	})

	if err != nil && !errors.Is(err, admin.ErrNoSuchKey) {
		resp.Diagnostics.AddError(
			"Error Deleting Key",
			fmt.Sprintf("Could not delete key: %s", err.Error()),
		)
		return
	}

	tflog.Trace(ctx, "Deleted RadosGW key")
}

func (r *KeyResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}

	var state KeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var config KeyResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	keyType := state.KeyType.ValueString()

	// For S3 keys, handle switching between specified and auto-generated
	if keyType == "s3" {
		if !state.Generated.IsNull() && !state.Generated.ValueBool() && config.AccessKey.IsNull() {
			// Specified → auto-generated
			resp.RequiresReplace = append(resp.RequiresReplace, path.Root("access_key"))
			resp.Plan.SetAttribute(ctx, path.Root("access_key"), types.StringUnknown())
			resp.Plan.SetAttribute(ctx, path.Root("secret_key"), types.StringUnknown())
			resp.Plan.SetAttribute(ctx, path.Root("generated"), types.BoolValue(true))
			resp.Plan.SetAttribute(ctx, path.Root("id"), types.StringUnknown())
		}

		if !state.Generated.IsNull() && state.Generated.ValueBool() && !config.AccessKey.IsNull() {
			// Auto-generated → specified
			resp.RequiresReplace = append(resp.RequiresReplace, path.Root("access_key"))
			resp.Plan.SetAttribute(ctx, path.Root("generated"), types.BoolValue(false))
		}
	}
}

func (r *KeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import format: "s3:user_id:access_key" or "swift:user_id:subuser"
	parts := strings.SplitN(req.ID, ":", 3)
	if len(parts) < 3 {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Import ID must be in format 's3:user_id:access_key' for S3 keys or 'swift:user_id:subuser' for Swift keys",
		)
		return
	}

	keyType := parts[0]
	userID := parts[1]
	keyID := parts[2]

	if keyType != "s3" && keyType != "swift" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Key type must be 's3' or 'swift', got: %s", keyType),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("user_id"), userID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("key_type"), keyType)...)

	if keyType == "swift" {
		fullSubuserID := fmt.Sprintf("%s:%s", userID, keyID)
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("subuser"), keyID)...)
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("access_key"), fullSubuserID)...)
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), fullSubuserID)...)
	} else {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("access_key"), keyID)...)
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), keyID)...)
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("generated"), false)...)
	}
}
