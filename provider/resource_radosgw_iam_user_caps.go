package provider

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/ceph/go-ceph/rgw/admin"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// uniqueCapTypesValidator validates that all caps have unique type values
type uniqueCapTypesValidator struct{}

func (v uniqueCapTypesValidator) Description(ctx context.Context) string {
	return "validates that all capabilities have unique type values"
}

func (v uniqueCapTypesValidator) MarkdownDescription(ctx context.Context) string {
	return "validates that all capabilities have unique type values"
}

func (v uniqueCapTypesValidator) ValidateSet(ctx context.Context, req validator.SetRequest, resp *validator.SetResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	var caps []CapModel
	diags := req.ConfigValue.ElementsAs(ctx, &caps, false)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	seen := make(map[string]bool)
	for _, cap := range caps {
		if cap.Type.IsUnknown() {
			continue
		}
		capType := cap.Type.ValueString()
		if seen[capType] {
			resp.Diagnostics.AddAttributeError(
				req.Path,
				"Duplicate Capability Type",
				fmt.Sprintf("Capability type %q is specified more than once. Each capability type can only appear once. Use perm = \"*\" for full access instead of specifying both read and write.", capType),
			)
			return
		}
		seen[capType] = true
	}
}

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &UserCapsResource{}
var _ resource.ResourceWithImportState = &UserCapsResource{}

func NewIAMUserCapsResource() resource.Resource {
	return &UserCapsResource{}
}

// UserCapsResource defines the resource implementation.
type UserCapsResource struct {
	client *RadosgwClient
}

// CapModel represents a single capability
type CapModel struct {
	Type types.String `tfsdk:"type"`
	Perm types.String `tfsdk:"perm"`
}

// UserCapsResourceModel describes the resource data model.
type UserCapsResourceModel struct {
	UserID types.String `tfsdk:"user_id"`
	Caps   types.Set    `tfsdk:"caps"`
}

// Valid capability types in RadosGW
var validCapTypes = []string{
	"users", "buckets", "metadata", "usage", "zone", "info",
	"accounts", "ratelimit", "roles", "user-policy", "amz-cache",
	"oidc-provider", "bilog", "mdlog", "datalog",
}

// Valid permissions
var validPerms = []string{"*", "read", "write"}

// capsNormalizePlanModifier normalizes caps during planning
type capsNormalizePlanModifier struct{}

func (m capsNormalizePlanModifier) Description(ctx context.Context) string {
	return "Normalizes capabilities by merging read+write to * for the same type"
}

func (m capsNormalizePlanModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m capsNormalizePlanModifier) PlanModifySet(ctx context.Context, req planmodifier.SetRequest, resp *planmodifier.SetResponse) {
	// If the value is unknown or null, don't normalize
	if req.PlanValue.IsUnknown() || req.PlanValue.IsNull() {
		return
	}

	// Normalize the planned caps
	normalizedCaps, err := normalizeCaps(ctx, req.PlanValue)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Normalizing Capabilities",
			fmt.Sprintf("Could not normalize capabilities during planning: %s", err.Error()),
		)
		return
	}

	resp.PlanValue = normalizedCaps
}

func (r *UserCapsResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iam_user_caps"
}

func (r *UserCapsResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Manages RadosGW user capabilities. Capabilities control administrative permissions for a user.

Capabilities are defined as a set of type/permission pairs. Each capability type can only appear once in the set.
Use ` + "`perm = \"*\"`" + ` for full access instead of specifying separate read and write entries.

The provider automatically:
- Sorts capabilities alphabetically by type to prevent unnecessary changes
- Validates that no duplicate capability types are specified

On deletion, all specified capabilities are removed from the user.`,

		Attributes: map[string]schema.Attribute{
			"user_id": schema.StringAttribute{
				MarkdownDescription: "The user ID to manage capabilities for.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"caps": schema.SetNestedAttribute{
				MarkdownDescription: "Set of capabilities to grant to the user. Each capability has a type and permission level.",
				Required:            true,
				Validators: []validator.Set{
					uniqueCapTypesValidator{},
				},
				PlanModifiers: []planmodifier.Set{
					capsNormalizePlanModifier{},
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"type": schema.StringAttribute{
							MarkdownDescription: "The capability type. Valid values: `users`, `buckets`, `metadata`, `usage`, `zone`, `info`, `accounts`, `ratelimit`, `roles`, `user-policy`, `amz-cache`, `oidc-provider`, `bilog`, `mdlog`, `datalog`.",
							Required:            true,
							Validators: []validator.String{
								stringvalidator.OneOf(validCapTypes...),
							},
						},
						"perm": schema.StringAttribute{
							MarkdownDescription: "The permission level. Valid values: `*` (full access), `read`, `write`. Note: Ceph converts `read` + `write` to `*` internally.",
							Required:            true,
							Validators: []validator.String{
								stringvalidator.OneOf(validPerms...),
							},
						},
					},
				},
			},
		},
	}
}

func (r *UserCapsResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// capsToString converts the caps set to the string format expected by go-ceph
// It also normalizes and sorts the capabilities
func capsToString(ctx context.Context, caps types.Set) (string, error) {
	if caps.IsNull() || caps.IsUnknown() {
		return "", nil
	}

	var capModels []CapModel
	diags := caps.ElementsAs(ctx, &capModels, false)
	if diags.HasError() {
		return "", fmt.Errorf("failed to convert caps to models")
	}

	// Build a map to handle duplicates and normalize permissions
	capMap := make(map[string]string)
	for _, cap := range capModels {
		capType := cap.Type.ValueString()
		perm := cap.Perm.ValueString()

		existing, ok := capMap[capType]
		if ok {
			// Merge permissions
			perm = mergePermissions(existing, perm)
		}
		capMap[capType] = perm
	}

	// Sort by type
	types := make([]string, 0, len(capMap))
	for t := range capMap {
		types = append(types, t)
	}
	sort.Strings(types)

	// Build the string
	var parts []string
	for _, t := range types {
		parts = append(parts, fmt.Sprintf("%s=%s", t, capMap[t]))
	}

	return strings.Join(parts, ";"), nil
}

// mergePermissions merges two permissions, converting read+write to *
func mergePermissions(perm1, perm2 string) string {
	if perm1 == "*" || perm2 == "*" {
		return "*"
	}
	if (perm1 == "read" && perm2 == "write") || (perm1 == "write" && perm2 == "read") {
		return "*"
	}
	// If same permission, just return it
	if perm1 == perm2 {
		return perm1
	}
	// Otherwise combine them (this shouldn't happen with validation, but handle it)
	return "*"
}

// stringToCaps converts a caps string from Ceph to a set of CapModel
func stringToCaps(ctx context.Context, capsStr string) (types.Set, error) {
	capAttrTypes := map[string]attr.Type{
		"type": types.StringType,
		"perm": types.StringType,
	}

	if capsStr == "" {
		return types.SetNull(types.ObjectType{AttrTypes: capAttrTypes}), nil
	}

	parts := strings.Split(capsStr, ";")
	var capValues []attr.Value

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}

		capType := strings.TrimSpace(kv[0])
		perm := strings.TrimSpace(kv[1])

		capObj, diags := types.ObjectValue(capAttrTypes, map[string]attr.Value{
			"type": types.StringValue(capType),
			"perm": types.StringValue(perm),
		})
		if diags.HasError() {
			return types.SetNull(types.ObjectType{AttrTypes: capAttrTypes}), fmt.Errorf("failed to create cap object")
		}

		capValues = append(capValues, capObj)
	}

	result, diags := types.SetValue(types.ObjectType{AttrTypes: capAttrTypes}, capValues)
	if diags.HasError() {
		return types.SetNull(types.ObjectType{AttrTypes: capAttrTypes}), fmt.Errorf("failed to create caps set")
	}

	return result, nil
}

// cephCapsToSet converts Ceph admin.UserCapSpec slice to a Terraform set
func cephCapsToSet(ctx context.Context, cephCaps []admin.UserCapSpec) (types.Set, error) {
	capAttrTypes := map[string]attr.Type{
		"type": types.StringType,
		"perm": types.StringType,
	}

	if len(cephCaps) == 0 {
		return types.SetNull(types.ObjectType{AttrTypes: capAttrTypes}), nil
	}

	var capValues []attr.Value
	for _, cap := range cephCaps {
		capObj, diags := types.ObjectValue(capAttrTypes, map[string]attr.Value{
			"type": types.StringValue(cap.Type),
			"perm": types.StringValue(cap.Perm),
		})
		if diags.HasError() {
			return types.SetNull(types.ObjectType{AttrTypes: capAttrTypes}), fmt.Errorf("failed to create cap object")
		}
		capValues = append(capValues, capObj)
	}

	result, diags := types.SetValue(types.ObjectType{AttrTypes: capAttrTypes}, capValues)
	if diags.HasError() {
		return types.SetNull(types.ObjectType{AttrTypes: capAttrTypes}), fmt.Errorf("failed to create caps set")
	}

	return result, nil
}

// normalizeCaps normalizes the caps set by sorting and merging permissions
func normalizeCaps(ctx context.Context, caps types.Set) (types.Set, error) {
	// Convert to string (this normalizes)
	capsStr, err := capsToString(ctx, caps)
	if err != nil {
		return caps, err
	}

	// Convert back to set
	return stringToCaps(ctx, capsStr)
}

func (r *UserCapsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data UserCapsResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Convert caps to string format for API
	capsStr, err := capsToString(ctx, data.Caps)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Converting Capabilities",
			fmt.Sprintf("Could not convert capabilities: %s", err.Error()),
		)
		return
	}

	tflog.Debug(ctx, "Adding user capabilities", map[string]any{
		"user_id": data.UserID.ValueString(),
		"caps":    capsStr,
	})

	// Add capabilities with retry logic for ConcurrentModification
	err = retryOnConcurrentModification(ctx, fmt.Sprintf("AddUserCap %s", data.UserID.ValueString()), func() error {
		_, addErr := r.client.Admin.AddUserCap(ctx, data.UserID.ValueString(), capsStr)
		return addErr
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Adding User Capabilities",
			fmt.Sprintf("Could not add capabilities for user %s: %s", data.UserID.ValueString(), err.Error()),
		)
		return
	}

	// Normalize caps before saving to state
	normalizedCaps, err := normalizeCaps(ctx, data.Caps)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Normalizing Capabilities",
			fmt.Sprintf("Could not normalize capabilities: %s", err.Error()),
		)
		return
	}
	data.Caps = normalizedCaps

	tflog.Trace(ctx, "Added user capabilities")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserCapsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data UserCapsResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading user capabilities", map[string]any{
		"user_id": data.UserID.ValueString(),
	})

	// Get user info to retrieve current capabilities
	user, err := r.client.Admin.GetUser(ctx, admin.User{ID: data.UserID.ValueString()})
	if err != nil {
		// If user doesn't exist, remove caps from state
		if errors.Is(err, admin.ErrNoSuchUser) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading User Capabilities",
			fmt.Sprintf("Could not read user %s: %s", data.UserID.ValueString(), err.Error()),
		)
		return
	}

	// If no caps remain, remove the resource
	if len(user.Caps) == 0 {
		resp.State.RemoveResource(ctx)
		return
	}

	// Convert Ceph caps to Terraform set
	capsSet, err := cephCapsToSet(ctx, user.Caps)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Converting Capabilities",
			fmt.Sprintf("Could not convert capabilities from Ceph: %s", err.Error()),
		)
		return
	}

	data.Caps = capsSet

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserCapsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data UserCapsResourceModel
	var state UserCapsResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Convert old caps to string for removal
	oldCapsStr, err := capsToString(ctx, state.Caps)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Converting Old Capabilities",
			fmt.Sprintf("Could not convert old capabilities: %s", err.Error()),
		)
		return
	}

	// Convert new caps to string for addition
	newCapsStr, err := capsToString(ctx, data.Caps)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Converting New Capabilities",
			fmt.Sprintf("Could not convert new capabilities: %s", err.Error()),
		)
		return
	}

	tflog.Debug(ctx, "Updating user capabilities", map[string]any{
		"user_id":  data.UserID.ValueString(),
		"old_caps": oldCapsStr,
		"new_caps": newCapsStr,
	})

	// Remove old capabilities with retry logic
	if oldCapsStr != "" {
		err = retryOnConcurrentModification(ctx, fmt.Sprintf("RemoveUserCap %s", state.UserID.ValueString()), func() error {
			_, removeErr := r.client.Admin.RemoveUserCap(ctx, state.UserID.ValueString(), oldCapsStr)
			return removeErr
		})
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Removing Old User Capabilities",
				fmt.Sprintf("Could not remove old capabilities for user %s: %s", state.UserID.ValueString(), err.Error()),
			)
			return
		}
	}

	// Add new capabilities with retry logic
	if newCapsStr != "" {
		err = retryOnConcurrentModification(ctx, fmt.Sprintf("AddUserCap %s", data.UserID.ValueString()), func() error {
			_, addErr := r.client.Admin.AddUserCap(ctx, data.UserID.ValueString(), newCapsStr)
			return addErr
		})
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Adding New User Capabilities",
				fmt.Sprintf("Could not add new capabilities for user %s: %s", data.UserID.ValueString(), err.Error()),
			)
			return
		}
	}

	// Normalize caps before saving to state
	normalizedCaps, err := normalizeCaps(ctx, data.Caps)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Normalizing Capabilities",
			fmt.Sprintf("Could not normalize capabilities: %s", err.Error()),
		)
		return
	}
	data.Caps = normalizedCaps

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserCapsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data UserCapsResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Convert caps to string for removal
	capsStr, err := capsToString(ctx, data.Caps)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Converting Capabilities",
			fmt.Sprintf("Could not convert capabilities: %s", err.Error()),
		)
		return
	}

	tflog.Debug(ctx, "Removing user capabilities", map[string]any{
		"user_id": data.UserID.ValueString(),
		"caps":    capsStr,
	})

	// Remove capabilities with retry logic
	err = retryOnConcurrentModification(ctx, fmt.Sprintf("RemoveUserCap %s", data.UserID.ValueString()), func() error {
		_, removeErr := r.client.Admin.RemoveUserCap(ctx, data.UserID.ValueString(), capsStr)
		return removeErr
	})
	if err != nil {
		// Ignore error if user doesn't exist
		if !errors.Is(err, admin.ErrNoSuchUser) {
			resp.Diagnostics.AddError(
				"Error Removing User Capabilities",
				fmt.Sprintf("Could not remove capabilities for user %s: %s", data.UserID.ValueString(), err.Error()),
			)
			return
		}
	}
}

func (r *UserCapsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import by user_id only - we'll read the caps from the API
	resource.ImportStatePassthroughID(ctx, path.Root("user_id"), req, resp)
}
