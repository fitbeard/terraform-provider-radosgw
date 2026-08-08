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

var _ resource.Resource = &AccountGroupPolicyResource{}
var _ resource.ResourceWithImportState = &AccountGroupPolicyResource{}

func NewIAMAccountGroupPolicyResource() resource.Resource {
	return &AccountGroupPolicyResource{}
}

// AccountGroupPolicyResource manages an inline IAM policy for a group within an account
// (IAM API PutGroupPolicy).
type AccountGroupPolicyResource struct {
	client    *RadosgwClient
	iamClient *IAMClient
}

type AccountGroupPolicyResourceModel struct {
	Group  types.String `tfsdk:"group"`
	Name   types.String `tfsdk:"name"`
	Policy types.String `tfsdk:"policy"`
	ID     types.String `tfsdk:"id"`
}

type getGroupPolicyResponseXML struct {
	XMLName xml.Name `xml:"GetGroupPolicyResponse"`
	Result  struct {
		GroupName      string `xml:"GroupName"`
		PolicyName     string `xml:"PolicyName"`
		PolicyDocument string `xml:"PolicyDocument"`
	} `xml:"GetGroupPolicyResult"`
}

func (r *AccountGroupPolicyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iam_account_group_policy"
}

func (r *AccountGroupPolicyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an **inline** IAM policy for a group within a RadosGW account (IAM " +
			"`PutGroupPolicy`). This is how you grant permissions to a `radosgw_iam_account_group`. Manage it with " +
			"account-root credentials (no admin capability required). For AWS-managed policies, use " +
			"`radosgw_iam_account_group_policy_attachment` instead.",

		Attributes: map[string]schema.Attribute{
			"group": schema.StringAttribute{
				MarkdownDescription: "The name of the IAM user to attach the inline policy to.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 64),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the policy. Must be unique within the group.",
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
				MarkdownDescription: "The unique identifier for this policy. Format: `user:policy_name`.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *AccountGroupPolicyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	r.iamClient = newAccountIAMClient(client)
}

func (r *AccountGroupPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan AccountGroupPolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	normalizedPolicy, err := normalizeJSONPolicy(plan.Policy.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Policy", fmt.Sprintf("The policy is not valid JSON: %s", err.Error()))
		return
	}

	if err := r.putGroupPolicy(ctx, plan.Group.ValueString(), plan.Name.ValueString(), normalizedPolicy); err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Group Policy",
			fmt.Sprintf("Could not create policy %s for group %s: %s", plan.Name.ValueString(), plan.Group.ValueString(), err.Error()),
		)
		return
	}

	plan.Policy = types.StringValue(normalizedPolicy)
	plan.ID = types.StringValue(fmt.Sprintf("%s:%s", plan.Group.ValueString(), plan.Name.ValueString()))

	tflog.Trace(ctx, "Created group policy", map[string]any{"group": plan.Group.ValueString(), "policy": plan.Name.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AccountGroupPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state AccountGroupPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := url.Values{}
	params.Set("Action", "GetGroupPolicy")
	params.Set("GroupName", state.Group.ValueString())
	params.Set("PolicyName", state.Name.ValueString())

	body, err := r.iamClient.DoRequest(ctx, params, "iam")
	if err != nil {
		if errors.Is(err, ErrNoSuchEntity) {
			tflog.Info(ctx, "Group policy not found, removing from state")
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading Group Policy",
			fmt.Sprintf("Could not read policy %s for group %s: %s", state.Name.ValueString(), state.Group.ValueString(), err.Error()),
		)
		return
	}

	var response getGroupPolicyResponseXML
	if err := xml.Unmarshal(body, &response); err != nil {
		resp.Diagnostics.AddError("Error Parsing Response", fmt.Sprintf("Could not parse GetGroupPolicy response: %s", err.Error()))
		return
	}

	decodedPolicy, err := url.QueryUnescape(response.Result.PolicyDocument)
	if err != nil {
		decodedPolicy = response.Result.PolicyDocument
	}
	if normalized, nerr := normalizeJSONPolicy(decodedPolicy); nerr == nil {
		state.Policy = types.StringValue(normalized)
	} else {
		state.Policy = types.StringValue(decodedPolicy)
	}
	state.ID = types.StringValue(fmt.Sprintf("%s:%s", state.Group.ValueString(), state.Name.ValueString()))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *AccountGroupPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan AccountGroupPolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	normalizedPolicy, err := normalizeJSONPolicy(plan.Policy.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Policy", fmt.Sprintf("The policy is not valid JSON: %s", err.Error()))
		return
	}

	// PutGroupPolicy is idempotent — it creates or updates.
	if err := r.putGroupPolicy(ctx, plan.Group.ValueString(), plan.Name.ValueString(), normalizedPolicy); err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Group Policy",
			fmt.Sprintf("Could not update policy %s for group %s: %s", plan.Name.ValueString(), plan.Group.ValueString(), err.Error()),
		)
		return
	}

	plan.Policy = types.StringValue(normalizedPolicy)
	plan.ID = types.StringValue(fmt.Sprintf("%s:%s", plan.Group.ValueString(), plan.Name.ValueString()))

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AccountGroupPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state AccountGroupPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := url.Values{}
	params.Set("Action", "DeleteGroupPolicy")
	params.Set("GroupName", state.Group.ValueString())
	params.Set("PolicyName", state.Name.ValueString())

	_, err := r.iamClient.DoRequest(ctx, params, "iam")
	if err != nil && !errors.Is(err, ErrNoSuchEntity) {
		resp.Diagnostics.AddError(
			"Error Deleting Group Policy",
			fmt.Sprintf("Could not delete policy %s for group %s: %s", state.Name.ValueString(), state.Group.ValueString(), err.Error()),
		)
		return
	}
	tflog.Trace(ctx, "Deleted user policy", map[string]any{"group": state.Group.ValueString(), "policy": state.Name.ValueString()})
}

func (r *AccountGroupPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid Import ID", "Import ID must be in the format 'user:policy_name'.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("group"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func (r *AccountGroupPolicyResource) putGroupPolicy(ctx context.Context, user, name, policy string) error {
	params := url.Values{}
	params.Set("Action", "PutGroupPolicy")
	params.Set("GroupName", user)
	params.Set("PolicyName", name)
	params.Set("PolicyDocument", policy)
	_, err := r.iamClient.DoRequest(ctx, params, "iam")
	return err
}
