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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &IAMAccountUserResource{}
var _ resource.ResourceWithImportState = &IAMAccountUserResource{}

func NewIAMAccountUserResource() resource.Resource {
	return &IAMAccountUserResource{}
}

// IAMAccountUserResource manages a user within a RadosGW account through the
// IAM API (as opposed to radosgw_iam_user, which uses the Admin Ops API).
type IAMAccountUserResource struct {
	client    *RadosgwClient
	iamClient *IAMClient
}

type IAMAccountUserResourceModel struct {
	Name         types.String `tfsdk:"name"`
	Path         types.String `tfsdk:"path"`
	ForceDestroy types.Bool   `tfsdk:"force_destroy"`
	ARN          types.String `tfsdk:"arn"`
	UniqueID     types.String `tfsdk:"unique_id"`
	CreateDate   types.String `tfsdk:"create_date"`
}

// =============================================================================
// IAM response types
// =============================================================================

type iamUserXML struct {
	Path       string `xml:"Path"`
	UserName   string `xml:"UserName"`
	UserId     string `xml:"UserId"`
	Arn        string `xml:"Arn"`
	CreateDate string `xml:"CreateDate"`
}

type createUserResponseXML struct {
	XMLName xml.Name `xml:"CreateUserResponse"`
	Result  struct {
		User iamUserXML `xml:"User"`
	} `xml:"CreateUserResult"`
}

type getUserResponseXML struct {
	XMLName xml.Name `xml:"GetUserResponse"`
	Result  struct {
		User iamUserXML `xml:"User"`
	} `xml:"GetUserResult"`
}

// =============================================================================
// Resource interface methods
// =============================================================================

func (r *IAMAccountUserResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iam_account_user"
}

func (r *IAMAccountUserResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an IAM user **within a RadosGW account**, using the S3/IAM API " +
			"(`CreateUser`) rather than the cluster-wide Admin Ops API.\n\n" +
			"Unlike `radosgw_iam_user` (which requires the cluster-wide `users` admin capability), this resource " +
			"is created through the account's IAM data plane, so it can be managed by an **account root user** — " +
			"or any account member holding an IAM policy that grants `iam:CreateUser` — **without any admin " +
			"capabilities**. Configure the provider (typically an aliased instance) with the account root's " +
			"credentials.\n\n" +
			"~> **`radosgw_iam_user` vs `radosgw_iam_account_user`:** use `radosgw_iam_user` (Admin Ops API) when " +
			"you have admin caps and need full RGW user metadata — display name, email, quotas, `op_mask`, " +
			"suspension, caps, Swift subusers — including creating an account's **root** user. Use this resource " +
			"for least-privilege management of member users inside an account, where only the IAM identity " +
			"(name, path, ARN) is needed. The two manage **different entities**: an IAM account user has no Admin " +
			"Ops metadata record. Grant it S3/IAM permissions with IAM policies, and create its credentials with " +
			"`radosgw_iam_account_access_key`.\n\n" +
			"~> **Note:** RadosGW's IAM user API does not support tags or a permissions boundary, so those " +
			"AWS attributes are intentionally omitted.",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the IAM user. Can be renamed in place.",
				Required:            true,
			},
			"path": schema.StringAttribute{
				MarkdownDescription: "The path for the user (an IAM path, which must begin and end with `/`). Defaults to `/`.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("/"),
			},
			"force_destroy": schema.BoolAttribute{
				MarkdownDescription: "When `true`, destroying this resource first deletes the user's access keys, " +
					"inline policies, and attached (managed) policies so the user can be removed. Without it, " +
					"RadosGW refuses to delete a user that still has access keys or policies. Defaults to `false`.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"arn": schema.StringAttribute{
				MarkdownDescription: "The Amazon Resource Name (ARN) that identifies the user, scoped to the account " +
					"(`arn:aws:iam::<account-id>:user/<path><name>`).",
				// No UseStateForUnknown: the ARN embeds path and name, so it must be
				// recomputed whenever either changes (rename/re-path).
				Computed: true,
			},
			"unique_id": schema.StringAttribute{
				MarkdownDescription: "The stable unique identifier assigned to the user by RadosGW.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"create_date": schema.StringAttribute{
				MarkdownDescription: "The date and time the user was created, in RFC 3339 format.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *IAMAccountUserResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// =============================================================================
// CRUD
// =============================================================================

func (r *IAMAccountUserResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan IAMAccountUserResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := url.Values{}
	params.Set("Action", "CreateUser")
	params.Set("UserName", plan.Name.ValueString())
	if !plan.Path.IsNull() && !plan.Path.IsUnknown() {
		params.Set("Path", plan.Path.ValueString())
	}

	body, err := r.iamClient.DoRequest(ctx, params, "iam")
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating IAM Account User",
			fmt.Sprintf("Could not create IAM user %s: %s", plan.Name.ValueString(), err.Error()),
		)
		return
	}

	var response createUserResponseXML
	if err := xml.Unmarshal(body, &response); err != nil {
		resp.Diagnostics.AddError(
			"Error Parsing Response",
			fmt.Sprintf("Could not parse CreateUser response: %s", err.Error()),
		)
		return
	}

	applyIAMUserToModel(response.Result.User, &plan)

	tflog.Trace(ctx, "Created IAM account user", map[string]any{
		"name": plan.Name.ValueString(),
		"arn":  plan.ARN.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *IAMAccountUserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state IAMAccountUserResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	user, err := r.getIAMUser(ctx, state.Name.ValueString())
	if err != nil {
		if errors.Is(err, ErrNoSuchEntity) {
			tflog.Info(ctx, "IAM account user not found, removing from state", map[string]any{
				"name": state.Name.ValueString(),
			})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading IAM Account User",
			fmt.Sprintf("Could not read IAM user %s: %s", state.Name.ValueString(), err.Error()),
		)
		return
	}

	applyIAMUserToModel(user, &state)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *IAMAccountUserResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state IAMAccountUserResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// UpdateUser renames (NewUserName) and/or re-paths (NewPath) an existing user.
	params := url.Values{}
	params.Set("Action", "UpdateUser")
	params.Set("UserName", state.Name.ValueString())
	changed := false
	if plan.Name.ValueString() != state.Name.ValueString() {
		params.Set("NewUserName", plan.Name.ValueString())
		changed = true
	}
	if plan.Path.ValueString() != state.Path.ValueString() {
		params.Set("NewPath", plan.Path.ValueString())
		changed = true
	}

	if changed {
		if _, err := r.iamClient.DoRequest(ctx, params, "iam"); err != nil {
			resp.Diagnostics.AddError(
				"Error Updating IAM Account User",
				fmt.Sprintf("Could not update IAM user %s: %s", state.Name.ValueString(), err.Error()),
			)
			return
		}
	}

	// Renaming/re-pathing changes the ARN, so read the effective values back.
	user, err := r.getIAMUser(ctx, plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading IAM Account User After Update",
			fmt.Sprintf("Could not read IAM user %s: %s", plan.Name.ValueString(), err.Error()),
		)
		return
	}

	applyIAMUserToModel(user, &plan)

	tflog.Debug(ctx, "Updated IAM account user", map[string]any{"name": plan.Name.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *IAMAccountUserResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state IAMAccountUserResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	userName := state.Name.ValueString()

	if state.ForceDestroy.ValueBool() {
		if err := r.purgeUserDependencies(ctx, userName); err != nil {
			resp.Diagnostics.AddError(
				"Error Purging IAM Account User Dependencies",
				fmt.Sprintf("force_destroy could not remove dependencies of user %s: %s", userName, err.Error()),
			)
			return
		}
	}

	params := url.Values{}
	params.Set("Action", "DeleteUser")
	params.Set("UserName", userName)

	_, err := r.iamClient.DoRequest(ctx, params, "iam")
	if err != nil {
		if errors.Is(err, ErrNoSuchEntity) {
			return
		}
		resp.Diagnostics.AddError(
			"Error Deleting IAM Account User",
			fmt.Sprintf("Could not delete IAM user %s: %s. If the user still has access keys or policies, set "+
				"force_destroy = true.", userName, err.Error()),
		)
		return
	}

	tflog.Trace(ctx, "Deleted IAM account user", map[string]any{"name": userName})
}

func (r *IAMAccountUserResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}

// =============================================================================
// Helpers
// =============================================================================

// getIAMUser fetches a user via the IAM GetUser action.
func (r *IAMAccountUserResource) getIAMUser(ctx context.Context, userName string) (iamUserXML, error) {
	return iamGetUser(ctx, r.iamClient, userName)
}

// purgeUserDependencies removes a user's access keys, inline policies, and
// attached managed policies so the user itself can be deleted (force_destroy).
func (r *IAMAccountUserResource) purgeUserDependencies(ctx context.Context, userName string) error {
	keyIDs, err := iamListAccessKeyIDs(ctx, r.iamClient, userName)
	if err != nil {
		return fmt.Errorf("listing access keys: %w", err)
	}
	for _, id := range keyIDs {
		p := url.Values{}
		p.Set("Action", "DeleteAccessKey")
		p.Set("UserName", userName)
		p.Set("AccessKeyId", id)
		if _, err := r.iamClient.DoRequest(ctx, p, "iam"); err != nil {
			return fmt.Errorf("deleting access key %s: %w", id, err)
		}
	}

	inline, err := iamListUserInlinePolicies(ctx, r.iamClient, userName)
	if err != nil {
		return fmt.Errorf("listing inline policies: %w", err)
	}
	for _, name := range inline {
		p := url.Values{}
		p.Set("Action", "DeleteUserPolicy")
		p.Set("UserName", userName)
		p.Set("PolicyName", name)
		if _, err := r.iamClient.DoRequest(ctx, p, "iam"); err != nil {
			return fmt.Errorf("deleting inline policy %s: %w", name, err)
		}
	}

	attached, err := iamListAttachedUserPolicyARNs(ctx, r.iamClient, userName)
	if err != nil {
		return fmt.Errorf("listing attached policies: %w", err)
	}
	for _, arn := range attached {
		p := url.Values{}
		p.Set("Action", "DetachUserPolicy")
		p.Set("UserName", userName)
		p.Set("PolicyArn", arn)
		if _, err := r.iamClient.DoRequest(ctx, p, "iam"); err != nil {
			return fmt.Errorf("detaching policy %s: %w", arn, err)
		}
	}

	return nil
}

// applyIAMUserToModel copies an IAM user API object into the resource model.
func applyIAMUserToModel(user iamUserXML, model *IAMAccountUserResourceModel) {
	model.Name = types.StringValue(user.UserName)
	model.Path = types.StringValue(user.Path)
	model.ARN = types.StringValue(user.Arn)
	model.UniqueID = types.StringValue(user.UserId)
	model.CreateDate = types.StringValue(user.CreateDate)
}
