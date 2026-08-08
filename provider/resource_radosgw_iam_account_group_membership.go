package provider

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ resource.Resource = &IAMAccountGroupMembershipResource{}
var _ resource.ResourceWithImportState = &IAMAccountGroupMembershipResource{}

func NewIAMAccountGroupMembershipResource() resource.Resource {
	return &IAMAccountGroupMembershipResource{}
}

// IAMAccountGroupMembershipResource manages the set of users in an IAM group
// (IAM AddUserToGroup/RemoveUserFromGroup). Like aws_iam_group_membership, it
// manages the group's membership EXCLUSIVELY: users added out-of-band are
// removed on the next apply.
type IAMAccountGroupMembershipResource struct {
	client    *RadosgwClient
	iamClient *IAMClient
}

type IAMAccountGroupMembershipResourceModel struct {
	Group types.String `tfsdk:"group"`
	Users types.Set    `tfsdk:"users"`
}

func (r *IAMAccountGroupMembershipResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iam_account_group_membership"
}

func (r *IAMAccountGroupMembershipResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the membership of a RadosGW account IAM group (IAM `AddUserToGroup` / " +
			"`RemoveUserFromGroup`). Manage it with account-root credentials.\n\n" +
			"~> **Exclusive membership:** this resource manages the group's **entire** member list, like " +
			"`aws_iam_group_membership`. Any user added to the group outside Terraform is removed on the next " +
			"apply. To manage a single user's membership non-exclusively, add the user via a separate mechanism.",

		Attributes: map[string]schema.Attribute{
			"group": schema.StringAttribute{
				MarkdownDescription: "The name of the IAM group.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"users": schema.SetAttribute{
				MarkdownDescription: "The set of IAM user names that are members of the group.",
				Required:            true,
				ElementType:         types.StringType,
			},
		},
	}
}

func (r *IAMAccountGroupMembershipResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *IAMAccountGroupMembershipResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan IAMAccountGroupMembershipResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var users []string
	resp.Diagnostics.Append(plan.Users.ElementsAs(ctx, &users, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	for _, u := range users {
		if err := r.addUser(ctx, plan.Group.ValueString(), u); err != nil {
			resp.Diagnostics.AddError(
				"Error Adding User To Group",
				fmt.Sprintf("Could not add user %s to group %s: %s", u, plan.Group.ValueString(), err.Error()),
			)
			return
		}
	}

	tflog.Trace(ctx, "Created group membership", map[string]any{"group": plan.Group.ValueString(), "count": len(users)})
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *IAMAccountGroupMembershipResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state IAMAccountGroupMembershipResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	members, err := iamGetGroupMembers(ctx, r.iamClient, state.Group.ValueString())
	if err != nil {
		if errors.Is(err, ErrNoSuchEntity) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading Group Membership",
			fmt.Sprintf("Could not read members of group %s: %s", state.Group.ValueString(), err.Error()),
		)
		return
	}

	usersSet, diags := types.SetValueFrom(ctx, types.StringType, members)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Users = usersSet
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *IAMAccountGroupMembershipResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state IAMAccountGroupMembershipResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var want, have []string
	resp.Diagnostics.Append(plan.Users.ElementsAs(ctx, &want, false)...)
	resp.Diagnostics.Append(state.Users.ElementsAs(ctx, &have, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	group := plan.Group.ValueString()
	wantSet := toStringSet(want)
	haveSet := toStringSet(have)

	for u := range wantSet {
		if _, ok := haveSet[u]; !ok {
			if err := r.addUser(ctx, group, u); err != nil {
				resp.Diagnostics.AddError("Error Adding User To Group", fmt.Sprintf("Could not add user %s to group %s: %s", u, group, err.Error()))
				return
			}
		}
	}
	for u := range haveSet {
		if _, ok := wantSet[u]; !ok {
			if err := r.removeUser(ctx, group, u); err != nil {
				resp.Diagnostics.AddError("Error Removing User From Group", fmt.Sprintf("Could not remove user %s from group %s: %s", u, group, err.Error()))
				return
			}
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *IAMAccountGroupMembershipResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state IAMAccountGroupMembershipResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var users []string
	resp.Diagnostics.Append(state.Users.ElementsAs(ctx, &users, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	for _, u := range users {
		if err := r.removeUser(ctx, state.Group.ValueString(), u); err != nil && !errors.Is(err, ErrNoSuchEntity) {
			resp.Diagnostics.AddError(
				"Error Removing User From Group",
				fmt.Sprintf("Could not remove user %s from group %s: %s", u, state.Group.ValueString(), err.Error()),
			)
			return
		}
	}
}

func (r *IAMAccountGroupMembershipResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("group"), req, resp)
}

func (r *IAMAccountGroupMembershipResource) addUser(ctx context.Context, group, user string) error {
	params := url.Values{}
	params.Set("Action", "AddUserToGroup")
	params.Set("GroupName", group)
	params.Set("UserName", user)
	_, err := r.iamClient.DoRequest(ctx, params, "iam")
	return err
}

func (r *IAMAccountGroupMembershipResource) removeUser(ctx context.Context, group, user string) error {
	params := url.Values{}
	params.Set("Action", "RemoveUserFromGroup")
	params.Set("GroupName", group)
	params.Set("UserName", user)
	_, err := r.iamClient.DoRequest(ctx, params, "iam")
	return err
}

func toStringSet(items []string) map[string]struct{} {
	m := make(map[string]struct{}, len(items))
	for _, it := range items {
		m[it] = struct{}{}
	}
	return m
}
