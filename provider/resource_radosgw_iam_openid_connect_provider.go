package provider

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &OIDCProviderResource{}
var _ resource.ResourceWithImportState = &OIDCProviderResource{}

func NewIAMOIDCProviderResource() resource.Resource {
	return &OIDCProviderResource{}
}

// OIDCProviderResource defines the resource implementation.
type OIDCProviderResource struct {
	client    *RadosgwClient
	iamClient *IAMClient
}

// OIDCProviderResourceModel describes the resource data model.
type OIDCProviderResourceModel struct {
	ARN            types.String `tfsdk:"arn"`
	URL            types.String `tfsdk:"url"`
	ClientIDList   types.Set    `tfsdk:"client_id_list"`
	ThumbprintList types.List   `tfsdk:"thumbprint_list"`
	AllowUpdates   types.Bool   `tfsdk:"allow_updates"`
}

// Custom plan modifiers for conditional replacement based on allow_updates

type requiresReplaceIfAllowUpdatesIsFalseModifier struct{}

func requiresReplaceIfAllowUpdatesIsFalse() planmodifier.Set {
	return &requiresReplaceIfAllowUpdatesIsFalseModifier{}
}

func (m *requiresReplaceIfAllowUpdatesIsFalseModifier) Description(ctx context.Context) string {
	return "Requires replacement if allow_updates is false and the value changes."
}

func (m *requiresReplaceIfAllowUpdatesIsFalseModifier) MarkdownDescription(ctx context.Context) string {
	return "Requires replacement if `allow_updates` is false and the value changes."
}

func (m *requiresReplaceIfAllowUpdatesIsFalseModifier) PlanModifySet(ctx context.Context, req planmodifier.SetRequest, resp *planmodifier.SetResponse) {
	var allowUpdates types.Bool
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("allow_updates"), &allowUpdates)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if allowUpdates.IsNull() || !allowUpdates.ValueBool() {
		if !req.PlanValue.Equal(req.StateValue) {
			resp.RequiresReplace = true
		}
	}
}

type requiresReplaceIfAllowUpdatesIsFalseForListModifier struct{}

func requiresReplaceIfAllowUpdatesIsFalseForList() planmodifier.List {
	return &requiresReplaceIfAllowUpdatesIsFalseForListModifier{}
}

func (m *requiresReplaceIfAllowUpdatesIsFalseForListModifier) Description(ctx context.Context) string {
	return "Requires replacement if allow_updates is false and the value changes."
}

func (m *requiresReplaceIfAllowUpdatesIsFalseForListModifier) MarkdownDescription(ctx context.Context) string {
	return "Requires replacement if `allow_updates` is false and the value changes."
}

func (m *requiresReplaceIfAllowUpdatesIsFalseForListModifier) PlanModifyList(ctx context.Context, req planmodifier.ListRequest, resp *planmodifier.ListResponse) {
	var allowUpdates types.Bool
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("allow_updates"), &allowUpdates)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if allowUpdates.IsNull() || !allowUpdates.ValueBool() {
		if !req.PlanValue.Equal(req.StateValue) {
			resp.RequiresReplace = true
		}
	}
}

// XML response structures for RadosGW OIDC API
type createOIDCProviderResponseXML struct {
	XMLName xml.Name                 `xml:"CreateOpenIDConnectProviderResponse"`
	Result  createOIDCProviderResult `xml:"CreateOpenIDConnectProviderResult"`
}

type createOIDCProviderResult struct {
	OpenIDConnectProviderArn string `xml:"OpenIDConnectProviderArn"`
}

type getOIDCProviderResponseXML struct {
	XMLName xml.Name              `xml:"GetOpenIDConnectProviderResponse"`
	Result  getOIDCProviderResult `xml:"GetOpenIDConnectProviderResult"`
}

type getOIDCProviderResult struct {
	URL          string `xml:"Url"`
	ClientIDList struct {
		Members []string `xml:"member"`
	} `xml:"ClientIDList"`
	ThumbprintList struct {
		Members []string `xml:"member"`
	} `xml:"ThumbprintList"`
}

func (r *OIDCProviderResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iam_openid_connect_provider"
}

func (r *OIDCProviderResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an OpenID Connect (OIDC) identity provider in RadosGW. " +
			"This resource allows you to establish trust between RadosGW and an external OpenID Connect provider " +
			"for federated authentication.\n\n" +
			"~> **Note:** RadosGW normalizes OIDC provider URLs by stripping the protocol (`http://`, `https://`) " +
			"when constructing ARNs. This means `http://example.com` and `https://example.com` would result in the " +
			"**same** ARN `arn:aws:iam:::oidc-provider/example.com`. You cannot create separate providers for the " +
			"same domain with different protocols. Changing the protocol in the URL triggers resource replacement.",

		Attributes: map[string]schema.Attribute{
			"arn": schema.StringAttribute{
				MarkdownDescription: "Amazon Resource Name (ARN) of the OIDC provider. " +
					"Format: `arn:aws:iam:::oidc-provider/<url>` (URL portion excludes protocol).",
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"url": schema.StringAttribute{
				MarkdownDescription: "URL of the identity provider. This value corresponds to the `iss` claim in OIDC tokens. " +
					"Must include the protocol (`http://` or `https://`). The full URL is stored and used when RadosGW " +
					"contacts the OIDC provider, but the protocol is stripped when constructing the ARN.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"client_id_list": schema.SetAttribute{
				MarkdownDescription: "List of client IDs (also known as audiences) that are allowed to authenticate with this provider. " +
					"These values correspond to the `aud` claim in OIDC tokens.",
				Required:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.Set{
					requiresReplaceIfAllowUpdatesIsFalse(),
				},
			},
			"thumbprint_list": schema.ListAttribute{
				MarkdownDescription: "List of certificate thumbprints for the OpenID Connect provider's IDP certificate(s). " +
					"Each thumbprint is a hex-encoded SHA-1 hash (40 characters). A maximum of 5 thumbprints are allowed.",
				Required:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.List{
					requiresReplaceIfAllowUpdatesIsFalseForList(),
				},
				Validators: []validator.List{
					listvalidator.SizeBetween(1, 5),
					listvalidator.ValueStringsAre(stringvalidator.LengthBetween(40, 40)),
				},
			},
			"allow_updates": schema.BoolAttribute{
				MarkdownDescription: "Whether to allow in-place updates for `client_id_list` and `thumbprint_list`. " +
					"When `true` (default), changes will be applied in-place. " +
					"When `false`, changes to these attributes will destroy and recreate the provider. " +
					"**Note:** In-place updates require Ceph Tentacle (20.x). On older versions, set to `false`.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(true),
			},
		},
	}
}

func (r *OIDCProviderResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	// Create IAM client using the same credentials and endpoint
	r.iamClient = NewIAMClient(
		client.Admin.Endpoint,
		client.Admin.AccessKey,
		client.Admin.SecretKey,
		client.Admin.HTTPClient,
	)
}

func (r *OIDCProviderResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan OIDCProviderResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var clientIDs []string
	resp.Diagnostics.Append(plan.ClientIDList.ElementsAs(ctx, &clientIDs, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var thumbprints []string
	resp.Diagnostics.Append(plan.ThumbprintList.ElementsAs(ctx, &thumbprints, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := url.Values{}
	params.Set("Action", "CreateOpenIDConnectProvider")
	params.Set("Url", plan.URL.ValueString())

	for i, clientID := range clientIDs {
		params.Set(fmt.Sprintf("ClientIDList.member.%d", i+1), clientID)
	}

	for i, thumbprint := range thumbprints {
		params.Set(fmt.Sprintf("ThumbprintList.member.%d", i+1), thumbprint)
	}

	body, err := r.iamClient.DoRequest(ctx, params, "iam")
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating OIDC Provider",
			fmt.Sprintf("Could not create OIDC provider: %s", err.Error()),
		)
		return
	}

	var response createOIDCProviderResponseXML
	if err := xml.Unmarshal(body, &response); err != nil {
		resp.Diagnostics.AddError(
			"Error Parsing Response",
			fmt.Sprintf("Could not parse CreateOpenIDConnectProvider response: %s", err.Error()),
		)
		return
	}

	if response.Result.OpenIDConnectProviderArn == "" {
		resp.Diagnostics.AddError(
			"Error Creating OIDC Provider",
			"API did not return an ARN",
		)
		return
	}

	plan.ARN = types.StringValue(response.Result.OpenIDConnectProviderArn)

	tflog.Trace(ctx, "Created OIDC provider", map[string]interface{}{
		"arn": response.Result.OpenIDConnectProviderArn,
		"url": plan.URL.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *OIDCProviderResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state OIDCProviderResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	arn := state.ARN.ValueString()

	params := url.Values{}
	params.Set("Action", "GetOpenIDConnectProvider")
	params.Set("OpenIDConnectProviderArn", arn)

	body, err := r.iamClient.DoRequest(ctx, params, "iam")
	if err != nil {
		if errors.Is(err, ErrNoSuchEntity) {
			tflog.Info(ctx, "OIDC provider not found, removing from state", map[string]interface{}{
				"arn": arn,
			})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading OIDC Provider",
			fmt.Sprintf("Could not read OIDC provider %s: %s", arn, err.Error()),
		)
		return
	}

	var response getOIDCProviderResponseXML
	if err := xml.Unmarshal(body, &response); err != nil {
		resp.Diagnostics.AddError(
			"Error Parsing Response",
			fmt.Sprintf("Could not parse GetOpenIDConnectProvider response: %s", err.Error()),
		)
		return
	}

	state.URL = types.StringValue(response.Result.URL)

	clientIDSet, diags := types.SetValueFrom(ctx, types.StringType, response.Result.ClientIDList.Members)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.ClientIDList = clientIDSet

	thumbprintList, diags := types.ListValueFrom(ctx, types.StringType, response.Result.ThumbprintList.Members)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.ThumbprintList = thumbprintList

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *OIDCProviderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state OIDCProviderResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	arn := state.ARN.ValueString()

	// Handle thumbprint list changes
	if !plan.ThumbprintList.Equal(state.ThumbprintList) {
		var thumbprints []string
		resp.Diagnostics.Append(plan.ThumbprintList.ElementsAs(ctx, &thumbprints, false)...)
		if resp.Diagnostics.HasError() {
			return
		}

		params := url.Values{}
		params.Set("Action", "UpdateOpenIDConnectProviderThumbprint")
		params.Set("OpenIDConnectProviderArn", arn)
		for i, thumbprint := range thumbprints {
			params.Set(fmt.Sprintf("ThumbprintList.member.%d", i+1), thumbprint)
		}

		_, err := r.iamClient.DoRequest(ctx, params, "iam")
		if err != nil {
			var iamErr *IAMError
			if errors.As(err, &iamErr) && iamErr.StatusCode == 405 {
				resp.Diagnostics.AddError(
					"Error Updating OIDC Provider Thumbprint",
					"UpdateOpenIDConnectProviderThumbprint is not supported. This requires Ceph Tentacle (20.x). Set allow_updates=false to force resource replacement instead.",
				)
				return
			}
			resp.Diagnostics.AddError(
				"Error Updating OIDC Provider Thumbprint",
				fmt.Sprintf("Could not update thumbprint for OIDC provider %s: %s", arn, err.Error()),
			)
			return
		}

		tflog.Debug(ctx, "Updated OIDC provider thumbprint", map[string]interface{}{
			"arn": arn,
		})
	}

	// Handle client ID changes
	if !plan.ClientIDList.Equal(state.ClientIDList) {
		var planClientIDs, stateClientIDs []string
		resp.Diagnostics.Append(plan.ClientIDList.ElementsAs(ctx, &planClientIDs, false)...)
		resp.Diagnostics.Append(state.ClientIDList.ElementsAs(ctx, &stateClientIDs, false)...)
		if resp.Diagnostics.HasError() {
			return
		}

		stateMap := make(map[string]bool)
		for _, id := range stateClientIDs {
			stateMap[id] = true
		}

		planMap := make(map[string]bool)
		for _, id := range planClientIDs {
			planMap[id] = true
		}

		// Add new client IDs
		for _, clientID := range planClientIDs {
			if !stateMap[clientID] {
				params := url.Values{}
				params.Set("Action", "AddClientIDToOpenIDConnectProvider")
				params.Set("OpenIDConnectProviderArn", arn)
				params.Set("ClientID", clientID)

				_, err := r.iamClient.DoRequest(ctx, params, "iam")
				if err != nil {
					var iamErr *IAMError
					if errors.As(err, &iamErr) && iamErr.StatusCode == 405 {
						resp.Diagnostics.AddError(
							"Error Adding Client ID",
							"AddClientIDToOpenIDConnectProvider is not supported. This requires Ceph Tentacle (20.x). Set allow_updates=false to force resource replacement instead.",
						)
						return
					}
					resp.Diagnostics.AddError(
						"Error Adding Client ID",
						fmt.Sprintf("Could not add client ID %s to OIDC provider %s: %s", clientID, arn, err.Error()),
					)
					return
				}

				tflog.Debug(ctx, "Added client ID to OIDC provider", map[string]interface{}{
					"arn":       arn,
					"client_id": clientID,
				})
			}
		}

		// Remove old client IDs
		for _, clientID := range stateClientIDs {
			if !planMap[clientID] {
				params := url.Values{}
				params.Set("Action", "RemoveClientIDFromOpenIDConnectProvider")
				params.Set("OpenIDConnectProviderArn", arn)
				params.Set("ClientID", clientID)

				_, err := r.iamClient.DoRequest(ctx, params, "iam")
				if err != nil {
					var iamErr *IAMError
					if errors.As(err, &iamErr) && iamErr.StatusCode == 405 {
						resp.Diagnostics.AddError(
							"Error Removing Client ID",
							"RemoveClientIDFromOpenIDConnectProvider is not supported. This requires Ceph Tentacle (20.x). Set allow_updates=false to force resource replacement instead.",
						)
						return
					}
					resp.Diagnostics.AddError(
						"Error Removing Client ID",
						fmt.Sprintf("Could not remove client ID %s from OIDC provider %s: %s", clientID, arn, err.Error()),
					)
					return
				}

				tflog.Debug(ctx, "Removed client ID from OIDC provider", map[string]interface{}{
					"arn":       arn,
					"client_id": clientID,
				})
			}
		}
	}

	plan.ARN = state.ARN
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *OIDCProviderResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state OIDCProviderResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	arn := state.ARN.ValueString()

	params := url.Values{}
	params.Set("Action", "DeleteOpenIDConnectProvider")
	params.Set("OpenIDConnectProviderArn", arn)

	_, err := r.iamClient.DoRequest(ctx, params, "iam")
	if err != nil {
		if errors.Is(err, ErrNoSuchEntity) {
			tflog.Info(ctx, "OIDC provider already deleted", map[string]interface{}{
				"arn": arn,
			})
			return
		}
		resp.Diagnostics.AddError(
			"Error Deleting OIDC Provider",
			fmt.Sprintf("Could not delete OIDC provider %s: %s", arn, err.Error()),
		)
		return
	}

	tflog.Trace(ctx, "Deleted OIDC provider", map[string]interface{}{
		"arn": arn,
	})
}

func (r *OIDCProviderResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Handle both ARN formats:
	// - Full ARN: arn:aws:iam:::oidc-provider/accounts.google.com
	// - Just the provider URL path: accounts.google.com
	importID := req.ID

	if !strings.HasPrefix(importID, "arn:") {
		// Convert URL path to full ARN format
		importID = fmt.Sprintf("arn:aws:iam:::oidc-provider/%s", importID)
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("arn"), importID)...)
}
