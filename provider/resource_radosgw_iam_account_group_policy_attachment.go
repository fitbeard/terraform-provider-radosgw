package provider

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ resource.Resource = &IAMAccountGroupPolicyAttachmentResource{}
var _ resource.ResourceWithImportState = &IAMAccountGroupPolicyAttachmentResource{}

func NewIAMAccountGroupPolicyAttachmentResource() resource.Resource {
	return &IAMAccountGroupPolicyAttachmentResource{}
}

// IAMAccountGroupPolicyAttachmentResource attaches a managed policy to an
// account IAM group (IAM AttachGroupPolicy). Following the AWS model, it manages a single
// attachment (not the exclusive set).
type IAMAccountGroupPolicyAttachmentResource struct {
	client    *RadosgwClient
	iamClient *IAMClient
}

type IAMAccountGroupPolicyAttachmentResourceModel struct {
	Group     types.String `tfsdk:"group"`
	PolicyARN types.String `tfsdk:"policy_arn"`
	ID        types.String `tfsdk:"id"`
}

func (r *IAMAccountGroupPolicyAttachmentResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iam_account_group_policy_attachment"
}

func (r *IAMAccountGroupPolicyAttachmentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Attaches a managed policy (e.g. `arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess`) to " +
			"an account IAM group (IAM `AttachGroupPolicy`). Like `aws_iam_group_policy_attachment`, it manages a " +
			"single attachment, so several can target the same group without conflict. Manage it with account-root " +
			"credentials. For inline policies, use `radosgw_iam_account_group_policy`.",

		Attributes: map[string]schema.Attribute{
			"group": schema.StringAttribute{
				MarkdownDescription: "The name of the IAM group to attach the policy to.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"policy_arn": schema.StringAttribute{
				MarkdownDescription: "The ARN of a built-in managed policy to attach. RadosGW does not support " +
					"custom managed policies (there is no `CreatePolicy`), so this must be one of the six predefined " +
					"managed policies: `arn:aws:iam::aws:policy/AmazonS3FullAccess`, `.../AmazonS3ReadOnlyAccess`, " +
					"`.../AmazonSNSFullAccess`, `.../AmazonSNSReadOnlyAccess`, `.../IAMFullAccess`, or " +
					"`.../IAMReadOnlyAccess`. Any other ARN returns `NoSuchEntity`. For custom permissions, use an " +
					"inline policy (`radosgw_iam_account_group_policy`) instead.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "The identifier of the attachment. Format: `group/policy_arn`.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *IAMAccountGroupPolicyAttachmentResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *IAMAccountGroupPolicyAttachmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan IAMAccountGroupPolicyAttachmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := iamAttachPolicy(ctx, r.iamClient, "AttachGroupPolicy", "GroupName", plan.Group.ValueString(), plan.PolicyARN.ValueString()); err != nil {
		resp.Diagnostics.AddError(
			"Error Attaching Group Policy",
			fmt.Sprintf("Could not attach policy %s to group %s: %s", plan.PolicyARN.ValueString(), plan.Group.ValueString(), err.Error()),
		)
		return
	}

	plan.ID = types.StringValue(plan.Group.ValueString() + "/" + plan.PolicyARN.ValueString())
	tflog.Trace(ctx, "Attached group policy", map[string]any{"group": plan.Group.ValueString(), "policy_arn": plan.PolicyARN.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *IAMAccountGroupPolicyAttachmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state IAMAccountGroupPolicyAttachmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	arns, err := iamListGroupAttachedPolicyARNs(ctx, r.iamClient, state.Group.ValueString())
	if err != nil {
		if errors.Is(err, ErrNoSuchEntity) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading Group Policy Attachment",
			fmt.Sprintf("Could not list attached policies for group %s: %s", state.Group.ValueString(), err.Error()),
		)
		return
	}

	if !slices.Contains(arns, state.PolicyARN.ValueString()) {
		tflog.Info(ctx, "Group policy attachment not found, removing from state")
		resp.State.RemoveResource(ctx)
		return
	}

	state.ID = types.StringValue(state.Group.ValueString() + "/" + state.PolicyARN.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *IAMAccountGroupPolicyAttachmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// All attributes are RequiresReplace; nothing to update in place.
	var plan IAMAccountGroupPolicyAttachmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *IAMAccountGroupPolicyAttachmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state IAMAccountGroupPolicyAttachmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := iamDetachPolicy(ctx, r.iamClient, "DetachGroupPolicy", "GroupName", state.Group.ValueString(), state.PolicyARN.ValueString()); err != nil {
		if errors.Is(err, ErrNoSuchEntity) {
			return
		}
		resp.Diagnostics.AddError(
			"Error Detaching Group Policy",
			fmt.Sprintf("Could not detach policy %s from group %s: %s", state.PolicyARN.ValueString(), state.Group.ValueString(), err.Error()),
		)
		return
	}
	tflog.Trace(ctx, "Detached group policy", map[string]any{"group": state.Group.ValueString(), "policy_arn": state.PolicyARN.ValueString()})
}

func (r *IAMAccountGroupPolicyAttachmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid Import ID", "Import ID must be in the format `group/policy_arn`.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("group"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("policy_arn"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
