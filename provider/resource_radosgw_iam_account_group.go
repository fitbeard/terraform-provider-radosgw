package provider

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ resource.Resource = &IAMAccountGroupResource{}
var _ resource.ResourceWithImportState = &IAMAccountGroupResource{}

func NewIAMAccountGroupResource() resource.Resource {
	return &IAMAccountGroupResource{}
}

// IAMAccountGroupResource manages an IAM group within a RadosGW account (IAM
// CreateGroup).
type IAMAccountGroupResource struct {
	client    *RadosgwClient
	iamClient *IAMClient
}

type IAMAccountGroupResourceModel struct {
	Name     types.String `tfsdk:"name"`
	Path     types.String `tfsdk:"path"`
	ARN      types.String `tfsdk:"arn"`
	UniqueID types.String `tfsdk:"unique_id"`
}

// iamGroupXML is the shared IAM Group API object.
type iamGroupXML struct {
	Path      string `xml:"Path"`
	GroupName string `xml:"GroupName"`
	GroupId   string `xml:"GroupId"`
	Arn       string `xml:"Arn"`
}

type createGroupResponseXML struct {
	XMLName xml.Name `xml:"CreateGroupResponse"`
	Result  struct {
		Group iamGroupXML `xml:"Group"`
	} `xml:"CreateGroupResult"`
}

type getGroupResponseXML struct {
	XMLName xml.Name `xml:"GetGroupResponse"`
	Result  struct {
		Group iamGroupXML `xml:"Group"`
	} `xml:"GetGroupResult"`
}

func (r *IAMAccountGroupResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iam_account_group"
}

func (r *IAMAccountGroupResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an IAM group within a RadosGW account (IAM `CreateGroup`). Groups collect " +
			"users so policies can be granted to all members at once (see `radosgw_iam_account_group_policy`, " +
			"`radosgw_iam_account_group_policy_attachment`, and `radosgw_iam_account_group_membership`). Manage it with " +
			"account-root credentials — no admin capability required.",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the group. Can be renamed in place.",
				Required:            true,
			},
			"path": schema.StringAttribute{
				MarkdownDescription: "The path for the group (an IAM path, which must begin and end with `/`). Defaults to `/`.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("/"),
			},
			"arn": schema.StringAttribute{
				MarkdownDescription: "The Amazon Resource Name (ARN) that identifies the group.",
				Computed:            true,
			},
			"unique_id": schema.StringAttribute{
				MarkdownDescription: "The stable unique identifier assigned to the group by RadosGW.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *IAMAccountGroupResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *IAMAccountGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan IAMAccountGroupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := url.Values{}
	params.Set("Action", "CreateGroup")
	params.Set("GroupName", plan.Name.ValueString())
	if !plan.Path.IsNull() && !plan.Path.IsUnknown() {
		params.Set("Path", plan.Path.ValueString())
	}

	body, err := r.iamClient.DoRequest(ctx, params, "iam")
	if err != nil {
		resp.Diagnostics.AddError("Error Creating IAM Group", fmt.Sprintf("Could not create group %s: %s", plan.Name.ValueString(), err.Error()))
		return
	}

	var response createGroupResponseXML
	if err := xml.Unmarshal(body, &response); err != nil {
		resp.Diagnostics.AddError("Error Parsing Response", fmt.Sprintf("Could not parse CreateGroup response: %s", err.Error()))
		return
	}

	applyIAMGroupToModel(response.Result.Group, &plan)
	tflog.Trace(ctx, "Created IAM group", map[string]any{"name": plan.Name.ValueString(), "arn": plan.ARN.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *IAMAccountGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state IAMAccountGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	group, err := r.getGroup(ctx, state.Name.ValueString())
	if err != nil {
		if errors.Is(err, ErrNoSuchEntity) {
			tflog.Info(ctx, "IAM group not found, removing from state")
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error Reading IAM Group", fmt.Sprintf("Could not read group %s: %s", state.Name.ValueString(), err.Error()))
		return
	}

	applyIAMGroupToModel(group, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *IAMAccountGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state IAMAccountGroupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := url.Values{}
	params.Set("Action", "UpdateGroup")
	params.Set("GroupName", state.Name.ValueString())
	changed := false
	if plan.Name.ValueString() != state.Name.ValueString() {
		params.Set("NewGroupName", plan.Name.ValueString())
		changed = true
	}
	if plan.Path.ValueString() != state.Path.ValueString() {
		params.Set("NewPath", plan.Path.ValueString())
		changed = true
	}
	if changed {
		if _, err := r.iamClient.DoRequest(ctx, params, "iam"); err != nil {
			resp.Diagnostics.AddError("Error Updating IAM Group", fmt.Sprintf("Could not update group %s: %s", state.Name.ValueString(), err.Error()))
			return
		}
	}

	group, err := r.getGroup(ctx, plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error Reading IAM Group After Update", fmt.Sprintf("Could not read group %s: %s", plan.Name.ValueString(), err.Error()))
		return
	}
	applyIAMGroupToModel(group, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *IAMAccountGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state IAMAccountGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := url.Values{}
	params.Set("Action", "DeleteGroup")
	params.Set("GroupName", state.Name.ValueString())

	_, err := r.iamClient.DoRequest(ctx, params, "iam")
	if err != nil && !errors.Is(err, ErrNoSuchEntity) {
		resp.Diagnostics.AddError(
			"Error Deleting IAM Group",
			fmt.Sprintf("Could not delete group %s: %s. Remove its members and policies first.", state.Name.ValueString(), err.Error()),
		)
		return
	}
	tflog.Trace(ctx, "Deleted IAM group", map[string]any{"name": state.Name.ValueString()})
}

func (r *IAMAccountGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}

func (r *IAMAccountGroupResource) getGroup(ctx context.Context, name string) (iamGroupXML, error) {
	return iamGetGroup(ctx, r.iamClient, name)
}

func applyIAMGroupToModel(group iamGroupXML, model *IAMAccountGroupResourceModel) {
	model.Name = types.StringValue(group.GroupName)
	model.Path = types.StringValue(group.Path)
	model.ARN = types.StringValue(group.Arn)
	model.UniqueID = types.StringValue(group.GroupId)
}
