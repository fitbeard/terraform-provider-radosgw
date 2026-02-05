package provider

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &RolePolicyResource{}
var _ resource.ResourceWithImportState = &RolePolicyResource{}

func NewIAMRolePolicyResource() resource.Resource {
	return &RolePolicyResource{}
}

// RolePolicyResource defines the resource implementation.
type RolePolicyResource struct {
	client    *RadosgwClient
	iamClient *IAMClient
}

// RolePolicyResourceModel describes the resource data model.
type RolePolicyResourceModel struct {
	Role   types.String `tfsdk:"role"`
	Name   types.String `tfsdk:"name"`
	Policy types.String `tfsdk:"policy"`
	ID     types.String `tfsdk:"id"`
}

// XML response structures for RadosGW Role Policy API
type getRolePolicyResponseXML struct {
	XMLName xml.Name            `xml:"GetRolePolicyResponse"`
	Result  getRolePolicyResult `xml:"GetRolePolicyResult"`
}

type getRolePolicyResult struct {
	RoleName       string `xml:"RoleName"`
	PolicyName     string `xml:"PolicyName"`
	PolicyDocument string `xml:"PolicyDocument"`
}

func (r *RolePolicyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iam_role_policy"
}

func (r *RolePolicyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an inline IAM policy for a RadosGW role. Inline policies are embedded " +
			"directly in the role. For managed policies that can be attached to multiple roles, " +
			"consider using a separate policy resource.",

		Attributes: map[string]schema.Attribute{
			"role": schema.StringAttribute{
				MarkdownDescription: "The name of the role to associate the policy with.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 64),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the policy. Must be unique within the role.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 128),
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^[\w+=,.@-]+$`),
						"must contain only alphanumeric characters, plus (+), equals (=), comma (,), period (.), at (@), underscore (_), and hyphen (-)",
					),
				},
			},
			"policy": schema.StringAttribute{
				MarkdownDescription: "The policy document (in JSON format). Use `jsonencode()` or the " +
					"`radosgw_iam_policy_document` data source to generate this.",
				Required: true,
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier for this policy. Format: `role:policy_name`.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *RolePolicyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *RolePolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan RolePolicyResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate and normalize the policy JSON
	normalizedPolicy, err := normalizeJSONPolicy(plan.Policy.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid Policy",
			fmt.Sprintf("The policy is not valid JSON: %s", err.Error()),
		)
		return
	}

	params := url.Values{}
	params.Set("Action", "PutRolePolicy")
	params.Set("RoleName", plan.Role.ValueString())
	params.Set("PolicyName", plan.Name.ValueString())
	params.Set("PolicyDocument", normalizedPolicy)

	_, err = r.iamClient.DoRequest(ctx, params, "iam")
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Role Policy",
			fmt.Sprintf("Could not create policy %s for role %s: %s", plan.Name.ValueString(), plan.Role.ValueString(), err.Error()),
		)
		return
	}

	// Set computed fields
	plan.ID = types.StringValue(fmt.Sprintf("%s:%s", plan.Role.ValueString(), plan.Name.ValueString()))
	plan.Policy = types.StringValue(normalizedPolicy)

	tflog.Trace(ctx, "Created role policy", map[string]interface{}{
		"role":   plan.Role.ValueString(),
		"policy": plan.Name.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *RolePolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state RolePolicyResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := url.Values{}
	params.Set("Action", "GetRolePolicy")
	params.Set("RoleName", state.Role.ValueString())
	params.Set("PolicyName", state.Name.ValueString())

	body, err := r.iamClient.DoRequest(ctx, params, "iam")
	if err != nil {
		if errors.Is(err, ErrNoSuchEntity) {
			tflog.Info(ctx, "Role policy not found, removing from state", map[string]interface{}{
				"role":   state.Role.ValueString(),
				"policy": state.Name.ValueString(),
			})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading Role Policy",
			fmt.Sprintf("Could not read policy %s for role %s: %s", state.Name.ValueString(), state.Role.ValueString(), err.Error()),
		)
		return
	}

	var response getRolePolicyResponseXML
	if err := xml.Unmarshal(body, &response); err != nil {
		resp.Diagnostics.AddError(
			"Error Parsing Response",
			fmt.Sprintf("Could not parse GetRolePolicy response: %s", err.Error()),
		)
		return
	}

	// URL decode the policy if it's URL-encoded
	policyDoc := response.Result.PolicyDocument
	decodedPolicy, err := url.QueryUnescape(policyDoc)
	if err != nil {
		decodedPolicy = policyDoc
	}

	// Normalize the policy for comparison
	normalizedPolicy, err := normalizeJSONPolicy(decodedPolicy)
	if err == nil {
		state.Policy = types.StringValue(normalizedPolicy)
	} else {
		state.Policy = types.StringValue(decodedPolicy)
	}

	state.ID = types.StringValue(fmt.Sprintf("%s:%s", state.Role.ValueString(), state.Name.ValueString()))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *RolePolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan RolePolicyResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate and normalize the policy JSON
	normalizedPolicy, err := normalizeJSONPolicy(plan.Policy.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid Policy",
			fmt.Sprintf("The policy is not valid JSON: %s", err.Error()),
		)
		return
	}

	// PutRolePolicy is idempotent - it creates or updates
	params := url.Values{}
	params.Set("Action", "PutRolePolicy")
	params.Set("RoleName", plan.Role.ValueString())
	params.Set("PolicyName", plan.Name.ValueString())
	params.Set("PolicyDocument", normalizedPolicy)

	_, err = r.iamClient.DoRequest(ctx, params, "iam")
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Role Policy",
			fmt.Sprintf("Could not update policy %s for role %s: %s", plan.Name.ValueString(), plan.Role.ValueString(), err.Error()),
		)
		return
	}

	plan.Policy = types.StringValue(normalizedPolicy)
	plan.ID = types.StringValue(fmt.Sprintf("%s:%s", plan.Role.ValueString(), plan.Name.ValueString()))

	tflog.Debug(ctx, "Updated role policy", map[string]interface{}{
		"role":   plan.Role.ValueString(),
		"policy": plan.Name.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *RolePolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state RolePolicyResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := url.Values{}
	params.Set("Action", "DeleteRolePolicy")
	params.Set("RoleName", state.Role.ValueString())
	params.Set("PolicyName", state.Name.ValueString())

	_, err := r.iamClient.DoRequest(ctx, params, "iam")
	if err != nil {
		if errors.Is(err, ErrNoSuchEntity) {
			tflog.Info(ctx, "Role policy already deleted", map[string]interface{}{
				"role":   state.Role.ValueString(),
				"policy": state.Name.ValueString(),
			})
			return
		}
		resp.Diagnostics.AddError(
			"Error Deleting Role Policy",
			fmt.Sprintf("Could not delete policy %s for role %s: %s", state.Name.ValueString(), state.Role.ValueString(), err.Error()),
		)
		return
	}

	tflog.Trace(ctx, "Deleted role policy", map[string]interface{}{
		"role":   state.Role.ValueString(),
		"policy": state.Name.ValueString(),
	})
}

func (r *RolePolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import format: "role_name:policy_name"
	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Import ID must be in the format 'role_name:policy_name'. Example: 'my-role:my-policy'",
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("role"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
