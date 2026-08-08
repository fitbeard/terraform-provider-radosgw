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

var _ resource.Resource = &AccountUserPolicyResource{}
var _ resource.ResourceWithImportState = &AccountUserPolicyResource{}

func NewIAMAccountUserPolicyResource() resource.Resource {
	return &AccountUserPolicyResource{}
}

// AccountUserPolicyResource manages an inline IAM policy for a user within an account
// (IAM API PutUserPolicy).
type AccountUserPolicyResource struct {
	client    *RadosgwClient
	iamClient *IAMClient
}

type AccountUserPolicyResourceModel struct {
	User   types.String `tfsdk:"user"`
	Name   types.String `tfsdk:"name"`
	Policy types.String `tfsdk:"policy"`
	ID     types.String `tfsdk:"id"`
}

type getUserPolicyResponseXML struct {
	XMLName xml.Name `xml:"GetUserPolicyResponse"`
	Result  struct {
		UserName       string `xml:"UserName"`
		PolicyName     string `xml:"PolicyName"`
		PolicyDocument string `xml:"PolicyDocument"`
	} `xml:"GetUserPolicyResult"`
}

func (r *AccountUserPolicyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iam_account_user_policy"
}

func (r *AccountUserPolicyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an **inline** IAM policy for a user within a RadosGW account (IAM " +
			"`PutUserPolicy`). This is how you grant permissions to a `radosgw_iam_account_user`. Manage it with " +
			"account-root credentials (no admin capability required). For AWS-managed policies, use " +
			"`radosgw_iam_account_user_policy_attachment` instead.",

		Attributes: map[string]schema.Attribute{
			"user": schema.StringAttribute{
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
				MarkdownDescription: "The name of the policy. Must be unique within the user.",
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

func (r *AccountUserPolicyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *AccountUserPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan AccountUserPolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	normalizedPolicy, err := normalizeJSONPolicy(plan.Policy.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Policy", fmt.Sprintf("The policy is not valid JSON: %s", err.Error()))
		return
	}

	if err := r.putUserPolicy(ctx, plan.User.ValueString(), plan.Name.ValueString(), normalizedPolicy); err != nil {
		resp.Diagnostics.AddError(
			"Error Creating User Policy",
			fmt.Sprintf("Could not create policy %s for user %s: %s", plan.Name.ValueString(), plan.User.ValueString(), err.Error()),
		)
		return
	}

	plan.Policy = types.StringValue(normalizedPolicy)
	plan.ID = types.StringValue(fmt.Sprintf("%s:%s", plan.User.ValueString(), plan.Name.ValueString()))

	tflog.Trace(ctx, "Created user policy", map[string]any{"user": plan.User.ValueString(), "policy": plan.Name.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AccountUserPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state AccountUserPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := url.Values{}
	params.Set("Action", "GetUserPolicy")
	params.Set("UserName", state.User.ValueString())
	params.Set("PolicyName", state.Name.ValueString())

	body, err := r.iamClient.DoRequest(ctx, params, "iam")
	if err != nil {
		if errors.Is(err, ErrNoSuchEntity) {
			tflog.Info(ctx, "User policy not found, removing from state")
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading User Policy",
			fmt.Sprintf("Could not read policy %s for user %s: %s", state.Name.ValueString(), state.User.ValueString(), err.Error()),
		)
		return
	}

	var response getUserPolicyResponseXML
	if err := xml.Unmarshal(body, &response); err != nil {
		resp.Diagnostics.AddError("Error Parsing Response", fmt.Sprintf("Could not parse GetUserPolicy response: %s", err.Error()))
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
	state.ID = types.StringValue(fmt.Sprintf("%s:%s", state.User.ValueString(), state.Name.ValueString()))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *AccountUserPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan AccountUserPolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	normalizedPolicy, err := normalizeJSONPolicy(plan.Policy.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Policy", fmt.Sprintf("The policy is not valid JSON: %s", err.Error()))
		return
	}

	// PutUserPolicy is idempotent — it creates or updates.
	if err := r.putUserPolicy(ctx, plan.User.ValueString(), plan.Name.ValueString(), normalizedPolicy); err != nil {
		resp.Diagnostics.AddError(
			"Error Updating User Policy",
			fmt.Sprintf("Could not update policy %s for user %s: %s", plan.Name.ValueString(), plan.User.ValueString(), err.Error()),
		)
		return
	}

	plan.Policy = types.StringValue(normalizedPolicy)
	plan.ID = types.StringValue(fmt.Sprintf("%s:%s", plan.User.ValueString(), plan.Name.ValueString()))

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AccountUserPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state AccountUserPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := url.Values{}
	params.Set("Action", "DeleteUserPolicy")
	params.Set("UserName", state.User.ValueString())
	params.Set("PolicyName", state.Name.ValueString())

	_, err := r.iamClient.DoRequest(ctx, params, "iam")
	if err != nil && !errors.Is(err, ErrNoSuchEntity) {
		resp.Diagnostics.AddError(
			"Error Deleting User Policy",
			fmt.Sprintf("Could not delete policy %s for user %s: %s", state.Name.ValueString(), state.User.ValueString(), err.Error()),
		)
		return
	}
	tflog.Trace(ctx, "Deleted user policy", map[string]any{"user": state.User.ValueString(), "policy": state.Name.ValueString()})
}

func (r *AccountUserPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid Import ID", "Import ID must be in the format 'user:policy_name'.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("user"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func (r *AccountUserPolicyResource) putUserPolicy(ctx context.Context, user, name, policy string) error {
	params := url.Values{}
	params.Set("Action", "PutUserPolicy")
	params.Set("UserName", user)
	params.Set("PolicyName", name)
	params.Set("PolicyDocument", policy)
	_, err := r.iamClient.DoRequest(ctx, params, "iam")
	return err
}
