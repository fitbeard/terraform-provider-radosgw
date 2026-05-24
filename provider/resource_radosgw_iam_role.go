package provider

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"net/url"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &RoleResource{}
var _ resource.ResourceWithImportState = &RoleResource{}

func NewIAMRoleResource() resource.Resource {
	return &RoleResource{}
}

// RoleResource defines the resource implementation.
type RoleResource struct {
	client    *RadosgwClient
	iamClient *IAMClient
}

// RoleResourceModel describes the resource data model.
type RoleResourceModel struct {
	Name               types.String `tfsdk:"name"`
	Path               types.String `tfsdk:"path"`
	Description        types.String `tfsdk:"description"`
	AssumeRolePolicy   types.String `tfsdk:"assume_role_policy"`
	MaxSessionDuration types.Int64  `tfsdk:"max_session_duration"`
	Tags               types.Map    `tfsdk:"tags"`
	ARN                types.String `tfsdk:"arn"`
	CreateDate         types.String `tfsdk:"create_date"`
	UniqueID           types.String `tfsdk:"unique_id"`
}

// XML response structures for RadosGW Role API
type createRoleResponseXML struct {
	XMLName xml.Name         `xml:"CreateRoleResponse"`
	Result  createRoleResult `xml:"CreateRoleResult"`
}

type createRoleResult struct {
	Role roleXML `xml:"Role"`
}

type getRoleResponseXML struct {
	XMLName xml.Name      `xml:"GetRoleResponse"`
	Result  getRoleResult `xml:"GetRoleResult"`
}

type getRoleResult struct {
	Role roleXML `xml:"Role"`
}

type roleXML struct {
	RoleName                 string `xml:"RoleName"`
	RoleId                   string `xml:"RoleId"`
	Path                     string `xml:"Path"`
	Arn                      string `xml:"Arn"`
	CreateDate               string `xml:"CreateDate"`
	MaxSessionDuration       int64  `xml:"MaxSessionDuration"`
	AssumeRolePolicyDocument string `xml:"AssumeRolePolicyDocument"`
	Description              string `xml:"Description"`
}

type tagXML struct {
	Key   string `xml:"Key"`
	Value string `xml:"Value"`
}

type listRoleTagsResponseXML struct {
	XMLName xml.Name           `xml:"ListRoleTagsResponse"`
	Result  listRoleTagsResult `xml:"ListRoleTagsResult"`
}

type listRoleTagsResult struct {
	Tags        roleTagsXML `xml:"Tags"`
	IsTruncated bool        `xml:"IsTruncated"`
	Marker      string      `xml:"Marker"`
}

type roleTagsXML struct {
	Members []tagXML `xml:"member"`
	Keys    []string `xml:"Key>Key"`
	Values  []string `xml:"Value>Value"`
}

func (r *RoleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iam_role"
}

func (r *RoleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an IAM role in RadosGW. Roles define a set of permissions for making " +
			"service requests and can be assumed by trusted entities using STS AssumeRole or AssumeRoleWithWebIdentity.",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the role. Must be unique and can contain up to 64 characters. " +
					"Valid characters: alphanumeric characters, plus (+), equals (=), comma (,), period (.), at (@), underscore (_), and hyphen (-).",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 64),
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^[\w+=,.@-]+$`),
						"must contain only alphanumeric characters, plus (+), equals (=), comma (,), period (.), at (@), underscore (_), and hyphen (-)",
					),
				},
			},
			"path": schema.StringAttribute{
				MarkdownDescription: "The path to the role. Default is `/`. Paths must begin and end with `/`.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("/"),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 512),
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^/.*/$|^/$`),
						"must begin and end with /",
					),
				},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "A description of the role. Maximum 1000 characters. " +
					"~> **Note:** This field is stored in state but may not be returned by the RadosGW API on older Ceph versions (Reef 18.x). " +
					"The provider preserves the configured value in this case.",
				Optional: true,
				Validators: []validator.String{
					stringvalidator.LengthAtMost(1000),
				},
			},
			"assume_role_policy": schema.StringAttribute{
				MarkdownDescription: "The trust relationship policy document (in JSON format) that grants an entity " +
					"permission to assume the role. Use `jsonencode()` or the `radosgw_iam_policy_document` data source to generate this.",
				Required: true,
			},
			"max_session_duration": schema.Int64Attribute{
				MarkdownDescription: "Maximum session duration (in seconds) for the role. Default is 3600 (1 hour). " +
					"Valid values: 3600-43200 (1-12 hours).",
				Optional: true,
				Computed: true,
				Default:  int64default.StaticInt64(3600),
				Validators: []validator.Int64{
					int64validator.Between(3600, 43200),
				},
			},
			"tags": schema.MapAttribute{
				MarkdownDescription: "Map of tags to assign to the role.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"arn": schema.StringAttribute{
				MarkdownDescription: "Amazon Resource Name (ARN) of the role.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"create_date": schema.StringAttribute{
				MarkdownDescription: "Date and time when the role was created.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"unique_id": schema.StringAttribute{
				MarkdownDescription: "Unique identifier for the role.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *RoleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	r.iamClient = NewIAMClient(
		client.Admin.Endpoint,
		client.Admin.AccessKey,
		client.Admin.SecretKey,
		client.Admin.HTTPClient,
	)
}

func (r *RoleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan RoleResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate and normalize the assume role policy JSON
	normalizedPolicy, err := normalizeJSONPolicy(plan.AssumeRolePolicy.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid Assume Role Policy",
			fmt.Sprintf("The assume_role_policy is not valid JSON: %s", err.Error()),
		)
		return
	}

	params := url.Values{}
	params.Set("Action", "CreateRole")
	params.Set("RoleName", plan.Name.ValueString())
	params.Set("Path", plan.Path.ValueString())
	params.Set("AssumeRolePolicyDocument", normalizedPolicy)
	if !plan.MaxSessionDuration.IsNull() {
		params.Set("MaxSessionDuration", fmt.Sprintf("%d", plan.MaxSessionDuration.ValueInt64()))
	}
	if !plan.Description.IsNull() && plan.Description.ValueString() != "" {
		params.Set("Description", plan.Description.ValueString())
	}

	tags, diags := terraformMapToStringMap(ctx, plan.Tags)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	body, err := r.iamClient.DoRequest(ctx, params, "iam")
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Role",
			fmt.Sprintf("Could not create role %s: %s", plan.Name.ValueString(), err.Error()),
		)
		return
	}

	var response createRoleResponseXML
	if err := xml.Unmarshal(body, &response); err != nil {
		resp.Diagnostics.AddError(
			"Error Parsing Response",
			fmt.Sprintf("Could not parse CreateRole response: %s", err.Error()),
		)
		return
	}

	role := response.Result.Role

	plan.ARN = types.StringValue(role.Arn)
	plan.CreateDate = types.StringValue(role.CreateDate)
	plan.UniqueID = types.StringValue(role.RoleId)
	plan.Path = types.StringValue(role.Path)
	plan.MaxSessionDuration = types.Int64Value(role.MaxSessionDuration)
	if role.Description != "" {
		plan.Description = types.StringValue(role.Description)
	}
	// Store the normalized policy to avoid perpetual diffs
	plan.AssumeRolePolicy = types.StringValue(normalizedPolicy)

	if len(tags) > 0 {
		if err := r.tagRole(ctx, plan.Name.ValueString(), tags); err != nil {
			deleteParams := url.Values{}
			deleteParams.Set("Action", "DeleteRole")
			deleteParams.Set("RoleName", plan.Name.ValueString())
			_, deleteErr := r.iamClient.DoRequest(ctx, deleteParams, "iam")
			if deleteErr != nil {
				resp.Diagnostics.AddWarning(
					"Error Cleaning Up Role",
					fmt.Sprintf("Could not delete role %s after tag creation failed: %s", plan.Name.ValueString(), deleteErr.Error()),
				)
			}
			resp.Diagnostics.AddError(
				"Error Tagging Role",
				fmt.Sprintf("Could not tag role %s: %s", plan.Name.ValueString(), err.Error()),
			)
			return
		}
	}

	tflog.Trace(ctx, "Created role", map[string]any{
		"name": plan.Name.ValueString(),
		"arn":  role.Arn,
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *RoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state RoleResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Preserve the current description from state - older Ceph versions (Reef)
	// don't return this field in the API response
	currentDescription := state.Description
	currentTags := state.Tags

	params := url.Values{}
	params.Set("Action", "GetRole")
	params.Set("RoleName", state.Name.ValueString())

	body, err := r.iamClient.DoRequest(ctx, params, "iam")
	if err != nil {
		if errors.Is(err, ErrNoSuchEntity) {
			tflog.Info(ctx, "Role not found, removing from state", map[string]any{
				"name": state.Name.ValueString(),
			})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading Role",
			fmt.Sprintf("Could not read role %s: %s", state.Name.ValueString(), err.Error()),
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

	state.ARN = types.StringValue(role.Arn)
	state.CreateDate = types.StringValue(role.CreateDate)
	state.UniqueID = types.StringValue(role.RoleId)
	state.Path = types.StringValue(role.Path)
	state.MaxSessionDuration = types.Int64Value(role.MaxSessionDuration)

	// Handle description field - older Ceph versions (Reef 18.x) don't return it
	if role.Description != "" {
		state.Description = types.StringValue(role.Description)
	} else if !currentDescription.IsNull() {
		// Preserve state value if API didn't return it (Reef compatibility)
		state.Description = currentDescription
	} else {
		state.Description = types.StringNull()
	}

	// Normalize the policy from the API response to compare with state
	if role.AssumeRolePolicyDocument != "" {
		// URL decode the policy if it's URL-encoded
		decodedPolicy, err := url.QueryUnescape(role.AssumeRolePolicyDocument)
		if err != nil {
			decodedPolicy = role.AssumeRolePolicyDocument
		}
		normalizedPolicy, err := normalizeJSONPolicy(decodedPolicy)
		if err == nil {
			state.AssumeRolePolicy = types.StringValue(normalizedPolicy)
		} else {
			state.AssumeRolePolicy = types.StringValue(decodedPolicy)
		}
	}

	tags, err := listRoleTags(ctx, r.iamClient, state.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Role Tags",
			fmt.Sprintf("Could not read tags for role %s: %s", state.Name.ValueString(), err.Error()),
		)
		return
	}
	tagMap, tagDiags := stringMapToTerraformMap(ctx, tags, currentTags)
	resp.Diagnostics.Append(tagDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Tags = tagMap

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *RoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state RoleResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Update assume role policy if changed
	if !plan.AssumeRolePolicy.Equal(state.AssumeRolePolicy) {
		normalizedPolicy, err := normalizeJSONPolicy(plan.AssumeRolePolicy.ValueString())
		if err != nil {
			resp.Diagnostics.AddError(
				"Invalid Assume Role Policy",
				fmt.Sprintf("The assume_role_policy is not valid JSON: %s", err.Error()),
			)
			return
		}

		params := url.Values{}
		params.Set("Action", "UpdateAssumeRolePolicy")
		params.Set("RoleName", plan.Name.ValueString())
		params.Set("PolicyDocument", normalizedPolicy)

		_, err = r.iamClient.DoRequest(ctx, params, "iam")
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Updating Assume Role Policy",
				fmt.Sprintf("Could not update assume role policy for role %s: %s", plan.Name.ValueString(), err.Error()),
			)
			return
		}

		plan.AssumeRolePolicy = types.StringValue(normalizedPolicy)

		tflog.Debug(ctx, "Updated assume role policy", map[string]any{
			"name": plan.Name.ValueString(),
		})
	}

	// Update max session duration and/or description if changed
	// Note: RadosGW UpdateRole API resets unspecified fields to defaults,
	// so we must always send both MaxSessionDuration and Description
	if !plan.MaxSessionDuration.Equal(state.MaxSessionDuration) || !plan.Description.Equal(state.Description) {
		params := url.Values{}
		params.Set("Action", "UpdateRole")
		params.Set("RoleName", plan.Name.ValueString())
		// Always include MaxSessionDuration to prevent reset to default
		params.Set("MaxSessionDuration", fmt.Sprintf("%d", plan.MaxSessionDuration.ValueInt64()))
		// Always include Description (empty string to clear)
		if !plan.Description.IsNull() {
			params.Set("Description", plan.Description.ValueString())
		} else {
			params.Set("Description", "")
		}

		_, err := r.iamClient.DoRequest(ctx, params, "iam")
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Updating Role",
				fmt.Sprintf("Could not update role %s: %s", plan.Name.ValueString(), err.Error()),
			)
			return
		}

		tflog.Debug(ctx, "Updated role", map[string]any{
			"name":                 plan.Name.ValueString(),
			"max_session_duration": plan.MaxSessionDuration.ValueInt64(),
			"description":          plan.Description.ValueString(),
		})
	}

	// Update tags if changed
	if !plan.Tags.Equal(state.Tags) {
		plannedTags, diags := terraformMapToStringMap(ctx, plan.Tags)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		currentTags, diags := terraformMapToStringMap(ctx, state.Tags)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		if err := r.updateRoleTags(ctx, plan.Name.ValueString(), currentTags, plannedTags); err != nil {
			resp.Diagnostics.AddError(
				"Error Updating Role Tags",
				fmt.Sprintf("Could not update tags for role %s: %s", plan.Name.ValueString(), err.Error()),
			)
			return
		}

		tflog.Debug(ctx, "Updated role tags", map[string]any{
			"name": plan.Name.ValueString(),
		})
	}

	// Preserve computed fields
	plan.ARN = state.ARN
	plan.CreateDate = state.CreateDate
	plan.UniqueID = state.UniqueID

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *RoleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state RoleResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Note: Role can only be deleted when it has no permission policies attached
	// Users should delete role policies first

	params := url.Values{}
	params.Set("Action", "DeleteRole")
	params.Set("RoleName", state.Name.ValueString())

	_, err := r.iamClient.DoRequest(ctx, params, "iam")
	if err != nil {
		if errors.Is(err, ErrNoSuchEntity) {
			tflog.Info(ctx, "Role already deleted", map[string]any{
				"name": state.Name.ValueString(),
			})
			return
		}
		resp.Diagnostics.AddError(
			"Error Deleting Role",
			fmt.Sprintf("Could not delete role %s: %s. Note: Roles cannot be deleted while they have attached policies.", state.Name.ValueString(), err.Error()),
		)
		return
	}

	tflog.Trace(ctx, "Deleted role", map[string]any{
		"name": state.Name.ValueString(),
	})
}

func (r *RoleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}

// normalizeJSONPolicy parses and re-encodes JSON to normalize whitespace and key ordering.
func normalizeJSONPolicy(policy string) (string, error) {
	var parsed any
	if err := json.Unmarshal([]byte(policy), &parsed); err != nil {
		return "", err
	}

	// Re-encode with consistent formatting (no extra whitespace)
	normalized, err := json.Marshal(parsed)
	if err != nil {
		return "", err
	}

	return string(normalized), nil
}

func listRoleTags(ctx context.Context, iamClient *IAMClient, roleName string) (map[string]string, error) {
	params := url.Values{}
	params.Set("Action", "ListRoleTags")
	params.Set("RoleName", roleName)

	tags := make(map[string]string)
	for {
		body, err := iamClient.DoRequest(ctx, params, "iam")
		if err != nil {
			return nil, err
		}

		var response listRoleTagsResponseXML
		if err := xml.Unmarshal(body, &response); err != nil {
			return nil, fmt.Errorf("could not parse ListRoleTags response: %w", err)
		}

		for key, value := range response.Result.Tags.toMap() {
			tags[key] = value
		}

		if !response.Result.IsTruncated {
			break
		}
		params.Set("Marker", response.Result.Marker)
	}

	return tags, nil
}

func (t roleTagsXML) toMap() map[string]string {
	tags := make(map[string]string)

	for _, tag := range t.Members {
		key := tag.Key
		if key == "" {
			continue
		}
		tags[key] = tag.Value
	}

	for i := 0; i < len(t.Keys) && i < len(t.Values); i++ {
		key := t.Keys[i]
		if key == "" {
			continue
		}
		tags[key] = t.Values[i]
	}

	return tags
}

func (r *RoleResource) updateRoleTags(ctx context.Context, roleName string, oldTags, newTags map[string]string) error {
	tagsToAdd := make(map[string]string)
	var tagKeysToRemove []string

	for key, newValue := range newTags {
		if oldValue, ok := oldTags[key]; !ok || oldValue != newValue {
			tagsToAdd[key] = newValue
		}
	}

	for key := range oldTags {
		if _, ok := newTags[key]; !ok {
			tagKeysToRemove = append(tagKeysToRemove, key)
		}
	}

	if len(tagKeysToRemove) > 0 {
		if err := r.untagRole(ctx, roleName, tagKeysToRemove); err != nil {
			return err
		}
	}

	if len(tagsToAdd) > 0 {
		if err := r.tagRole(ctx, roleName, tagsToAdd); err != nil {
			return err
		}
	}

	return nil
}

func (r *RoleResource) tagRole(ctx context.Context, roleName string, tags map[string]string) error {
	params := url.Values{}
	params.Set("Action", "TagRole")
	params.Set("RoleName", roleName)
	addTagsToParams(params, tags)

	_, err := r.iamClient.DoRequest(ctx, params, "iam")
	return err
}

func (r *RoleResource) untagRole(ctx context.Context, roleName string, tagKeys []string) error {
	for _, key := range tagKeys {
		params := url.Values{}
		params.Set("Action", "UntagRole")
		params.Set("RoleName", roleName)
		params.Set("TagKeys.member.1", key)
		// RadosGW currently collects this exact key in UntagRole rather than
		// the numbered AWS Query member above.
		params.Set("TagKeys.member.", key)

		if _, err := r.iamClient.DoRequest(ctx, params, "iam"); err != nil {
			return err
		}
	}

	return nil
}

func addTagsToParams(params url.Values, tags map[string]string) {
	i := 1
	for key, value := range tags {
		params.Set(fmt.Sprintf("Tags.member.%d.Key", i), key)
		params.Set(fmt.Sprintf("Tags.member.%d.Value", i), value)
		i++
	}
}

func terraformMapToStringMap(ctx context.Context, value types.Map) (map[string]string, diag.Diagnostics) {
	if value.IsNull() || value.IsUnknown() {
		return map[string]string{}, nil
	}

	tags := make(map[string]string)
	diags := value.ElementsAs(ctx, &tags, false)
	return tags, diags
}

func stringMapToTerraformMap(ctx context.Context, tags map[string]string, prior types.Map) (types.Map, diag.Diagnostics) {
	if len(tags) == 0 && prior.IsNull() {
		return types.MapNull(types.StringType), nil
	}

	value, diags := types.MapValueFrom(ctx, types.StringType, tags)
	return value, diags
}
