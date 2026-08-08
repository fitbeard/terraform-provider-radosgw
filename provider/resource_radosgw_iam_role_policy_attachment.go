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

var _ resource.Resource = &IAMRolePolicyAttachmentResource{}
var _ resource.ResourceWithImportState = &IAMRolePolicyAttachmentResource{}

func NewIAMRolePolicyAttachmentResource() resource.Resource {
	return &IAMRolePolicyAttachmentResource{}
}

// IAMRolePolicyAttachmentResource attaches a managed policy to an account
// role (IAM AttachRolePolicy). Following the AWS model, it manages a single
// attachment (not the exclusive set).
type IAMRolePolicyAttachmentResource struct {
	client    *RadosgwClient
	iamClient *IAMClient
}

type IAMRolePolicyAttachmentResourceModel struct {
	Role      types.String `tfsdk:"role"`
	PolicyARN types.String `tfsdk:"policy_arn"`
	ID        types.String `tfsdk:"id"`
}

func (r *IAMRolePolicyAttachmentResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iam_role_policy_attachment"
}

func (r *IAMRolePolicyAttachmentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Attaches a managed policy (e.g. `arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess`) to " +
			"a role (IAM `AttachRolePolicy`). Like `aws_iam_role_policy_attachment`, it manages a " +
			"single attachment, so several can target the same role without conflict. Manage it with account-root " +
			"credentials. For inline policies, use `radosgw_iam_role_policy`.",

		Attributes: map[string]schema.Attribute{
			"role": schema.StringAttribute{
				MarkdownDescription: "The name of the role to attach the policy to.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"policy_arn": schema.StringAttribute{
				MarkdownDescription: "The ARN of a built-in managed policy to attach. RadosGW does not support " +
					"custom managed policies (there is no `CreatePolicy`), so this must be one of the six predefined " +
					"managed policies: `arn:aws:iam::aws:policy/AmazonS3FullAccess`, `.../AmazonS3ReadOnlyAccess`, " +
					"`.../AmazonSNSFullAccess`, `.../AmazonSNSReadOnlyAccess`, `.../IAMFullAccess`, or " +
					"`.../IAMReadOnlyAccess`. Any other ARN returns `NoSuchEntity`. For custom permissions, use an " +
					"inline policy (`radosgw_iam_role_policy`) instead.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "The identifier of the attachment. Format: `role/policy_arn`.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *IAMRolePolicyAttachmentResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *IAMRolePolicyAttachmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan IAMRolePolicyAttachmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := iamAttachPolicy(ctx, r.iamClient, "AttachRolePolicy", "RoleName", plan.Role.ValueString(), plan.PolicyARN.ValueString()); err != nil {
		resp.Diagnostics.AddError(
			"Error Attaching Role Policy",
			fmt.Sprintf("Could not attach policy %s to role %s: %s", plan.PolicyARN.ValueString(), plan.Role.ValueString(), err.Error()),
		)
		return
	}

	plan.ID = types.StringValue(plan.Role.ValueString() + "/" + plan.PolicyARN.ValueString())
	tflog.Trace(ctx, "Attached role policy", map[string]any{"role": plan.Role.ValueString(), "policy_arn": plan.PolicyARN.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *IAMRolePolicyAttachmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state IAMRolePolicyAttachmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	arns, err := iamListRoleAttachedPolicyARNs(ctx, r.iamClient, state.Role.ValueString())
	if err != nil {
		if errors.Is(err, ErrNoSuchEntity) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading Role Policy Attachment",
			fmt.Sprintf("Could not list attached policies for role %s: %s", state.Role.ValueString(), err.Error()),
		)
		return
	}

	if !slices.Contains(arns, state.PolicyARN.ValueString()) {
		tflog.Info(ctx, "Role policy attachment not found, removing from state")
		resp.State.RemoveResource(ctx)
		return
	}

	state.ID = types.StringValue(state.Role.ValueString() + "/" + state.PolicyARN.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *IAMRolePolicyAttachmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// All attributes are RequiresReplace; nothing to update in place.
	var plan IAMRolePolicyAttachmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *IAMRolePolicyAttachmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state IAMRolePolicyAttachmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := iamDetachPolicy(ctx, r.iamClient, "DetachRolePolicy", "RoleName", state.Role.ValueString(), state.PolicyARN.ValueString()); err != nil {
		if errors.Is(err, ErrNoSuchEntity) {
			return
		}
		resp.Diagnostics.AddError(
			"Error Detaching Role Policy",
			fmt.Sprintf("Could not detach policy %s from role %s: %s", state.PolicyARN.ValueString(), state.Role.ValueString(), err.Error()),
		)
		return
	}
	tflog.Trace(ctx, "Detached role policy", map[string]any{"role": state.Role.ValueString(), "policy_arn": state.PolicyARN.ValueString()})
}

func (r *IAMRolePolicyAttachmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid Import ID", "Import ID must be in the format 'role/policy_arn'.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("role"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("policy_arn"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
