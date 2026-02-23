package provider

import (
	"context"
	"encoding/json"
	"encoding/xml"
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

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &SNSTopicPolicyResource{}
var _ resource.ResourceWithImportState = &SNSTopicPolicyResource{}

func NewSNSTopicPolicyResource() resource.Resource {
	return &SNSTopicPolicyResource{}
}

// SNSTopicPolicyResource defines the resource implementation.
type SNSTopicPolicyResource struct {
	client    *RadosgwClient
	iamClient *IAMClient
}

// SNSTopicPolicyResourceModel describes the resource data model.
type SNSTopicPolicyResourceModel struct {
	ARN    types.String `tfsdk:"arn"`
	Policy types.String `tfsdk:"policy"`
	Owner  types.String `tfsdk:"owner"`
}

func (r *SNSTopicPolicyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sns_topic_policy"
}

func (r *SNSTopicPolicyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an access policy for an SNS topic in RadosGW. " +
			"The policy controls who can access the topic in addition to the owner.\n\n" +
			"~> **Note:** This resource manages the policy independently from the `radosgw_sns_topic` resource. " +
			"Using both on the same topic will cause conflicts.\n\n" +
			"~> **Note:** RadosGW's `CreateTopic` API (used for topic updates) replaces all topic " +
			"attributes, which would normally clear the policy. The `radosgw_sns_topic` resource " +
			"handles this by reading and re-including the existing policy in every update call, " +
			"so the policy managed by this resource is preserved through topic changes.\n\n" +
			"~> **Ceph version requirement:** This resource requires **Ceph Squid (19.x) or later**. " +
			"The `SetTopicAttributes` API used to manage topic policies is not available on Ceph Reef (18.x).",

		Attributes: map[string]schema.Attribute{
			"arn": schema.StringAttribute{
				MarkdownDescription: "The ARN of the SNS topic to set the policy on.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"policy": schema.StringAttribute{
				MarkdownDescription: "The fully-formed JSON policy document. " +
					"Use `jsonencode()` or the `radosgw_iam_policy_document` data source to generate this.\n\n" +
					"Supported actions:\n" +
					"  - `sns:GetTopicAttributes` — list or get existing topics\n" +
					"  - `sns:SetTopicAttributes` — set attributes for the existing topic\n" +
					"  - `sns:DeleteTopic` — delete the existing topic\n" +
					"  - `sns:Publish` — create/subscribe notifications on the existing topic",
				Required: true,
			},
			"owner": schema.StringAttribute{
				MarkdownDescription: "The owner of the SNS topic.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *SNSTopicPolicyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *SNSTopicPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan SNSTopicPolicyResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Normalize the policy JSON
	normalizedPolicy, err := normalizeIAMPolicyJSON(plan.Policy.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid Policy",
			fmt.Sprintf("The policy is not valid JSON: %s", err.Error()),
		)
		return
	}

	// Set the policy on the topic using SetTopicAttributes
	if err := r.setTopicPolicy(ctx, plan.ARN.ValueString(), normalizedPolicy); err != nil {
		resp.Diagnostics.AddError(
			"Error Setting SNS Topic Policy",
			fmt.Sprintf("Could not set policy on topic %s: %s", plan.ARN.ValueString(), err.Error()),
		)
		return
	}

	plan.Policy = types.StringValue(normalizedPolicy)

	// Read back topic attributes to get the owner
	owner, err := r.readTopicOwner(ctx, plan.ARN.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading SNS Topic",
			fmt.Sprintf("Policy was set but could not read topic %s: %s", plan.ARN.ValueString(), err.Error()),
		)
		return
	}

	plan.Owner = types.StringValue(owner)

	tflog.Trace(ctx, "Created SNS topic policy", map[string]any{
		"arn": plan.ARN.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *SNSTopicPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state SNSTopicPolicyResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	attrs, err := r.readTopicAttributes(ctx, state.ARN.ValueString())
	if err != nil {
		if isSNSTopicNotFound(err) {
			tflog.Info(ctx, "SNS topic not found, removing policy from state", map[string]any{
				"arn": state.ARN.ValueString(),
			})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading SNS Topic Policy",
			fmt.Sprintf("Could not read topic %s: %s", state.ARN.ValueString(), err.Error()),
		)
		return
	}

	state.Owner = types.StringValue(attrs["User"])

	// Read the policy — if it's semantically equivalent to what we have
	// in state, keep the state value to avoid spurious diffs. RadosGW
	// may normalize the JSON (e.g. collapsing single-element arrays to
	// scalars) which would cause perpetual drift otherwise.
	policy := attrs["Policy"]
	if policy == "" {
		// Topic exists but policy was removed out-of-band
		tflog.Info(ctx, "SNS topic policy is empty, removing from state", map[string]any{
			"arn": state.ARN.ValueString(),
		})
		resp.State.RemoveResource(ctx)
		return
	}

	// Compare the API policy with the current state policy.
	// If they are semantically equivalent, keep the state value unchanged.
	if !state.Policy.IsNull() && !state.Policy.IsUnknown() {
		equivalent, err := arePoliciesEquivalent(state.Policy.ValueString(), policy)
		if err == nil && equivalent {
			// Policies are equivalent — keep state as-is (no drift)
			resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
			return
		}
	}

	// Policies differ or this is an import — normalize and set
	normalizedPolicy, err := normalizeIAMPolicyJSON(policy)
	if err == nil {
		state.Policy = types.StringValue(normalizedPolicy)
	} else {
		state.Policy = types.StringValue(policy)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *SNSTopicPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan SNSTopicPolicyResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Normalize the policy JSON
	normalizedPolicy, err := normalizeIAMPolicyJSON(plan.Policy.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid Policy",
			fmt.Sprintf("The policy is not valid JSON: %s", err.Error()),
		)
		return
	}

	if err := r.setTopicPolicy(ctx, plan.ARN.ValueString(), normalizedPolicy); err != nil {
		resp.Diagnostics.AddError(
			"Error Updating SNS Topic Policy",
			fmt.Sprintf("Could not update policy on topic %s: %s", plan.ARN.ValueString(), err.Error()),
		)
		return
	}

	plan.Policy = types.StringValue(normalizedPolicy)

	tflog.Debug(ctx, "Updated SNS topic policy", map[string]any{
		"arn": plan.ARN.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *SNSTopicPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state SNSTopicPolicyResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Clear the policy by setting it to an empty value
	if err := r.setTopicPolicy(ctx, state.ARN.ValueString(), ""); err != nil {
		// If the topic is already gone, the policy is effectively deleted
		if isSNSTopicNotFound(err) {
			tflog.Info(ctx, "SNS topic already deleted, policy is gone", map[string]any{
				"arn": state.ARN.ValueString(),
			})
			return
		}
		resp.Diagnostics.AddError(
			"Error Removing SNS Topic Policy",
			fmt.Sprintf("Could not remove policy from topic %s: %s", state.ARN.ValueString(), err.Error()),
		)
		return
	}

	tflog.Trace(ctx, "Deleted SNS topic policy", map[string]any{
		"arn": state.ARN.ValueString(),
	})
}

func (r *SNSTopicPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("arn"), req, resp)
}

// =============================================================================
// Helper Functions
// =============================================================================

// setTopicPolicy uses SetTopicAttributes to set or clear the Policy attribute.
func (r *SNSTopicPolicyResource) setTopicPolicy(ctx context.Context, arn, policy string) error {
	params := url.Values{}
	params.Set("Action", "SetTopicAttributes")
	params.Set("TopicArn", arn)
	params.Set("AttributeName", "Policy")
	params.Set("AttributeValue", policy)

	_, err := r.iamClient.DoPostRequest(ctx, params, "sns")
	return err
}

// readTopicAttributes calls GetTopicAttributes and returns all key-value pairs.
func (r *SNSTopicPolicyResource) readTopicAttributes(ctx context.Context, arn string) (map[string]string, error) {
	params := url.Values{}
	params.Set("Action", "GetTopicAttributes")
	params.Set("TopicArn", arn)

	body, err := r.iamClient.DoPostRequest(ctx, params, "sns")
	if err != nil {
		return nil, err
	}

	var response getTopicAttributesResponseXML
	if err := xml.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("could not parse GetTopicAttributes response: %w", err)
	}

	attrs := make(map[string]string)
	for _, entry := range response.Result.Attributes.Entries {
		attrs[entry.Key] = entry.Value
	}

	return attrs, nil
}

// readTopicOwner is a convenience wrapper that returns just the topic owner.
func (r *SNSTopicPolicyResource) readTopicOwner(ctx context.Context, arn string) (string, error) {
	attrs, err := r.readTopicAttributes(ctx, arn)
	if err != nil {
		return "", err
	}
	return attrs["User"], nil
}

// normalizeIAMPolicyJSON parses an IAM-style policy JSON document and
// re-encodes it in a canonical form so that semantically equivalent
// policies produce identical strings.
//
// RadosGW collapses single-element arrays in Principal, Action, and
// Resource to scalar strings when storing the policy, but the
// radosgw_iam_policy_document data source keeps Action and Resource
// as arrays (only Principal is collapsed for single identifiers).
//
// To handle both directions consistently, this function normalizes
// the policy to a canonical form where:
//   - Action, NotAction, Resource, NotResource: always arrays
//   - Principal/NotPrincipal identifiers: scalar for single value,
//     array for multiple (matching radosgw_iam_policy_document output)
func normalizeIAMPolicyJSON(policyJSON string) (string, error) {
	var doc map[string]any
	if err := json.Unmarshal([]byte(policyJSON), &doc); err != nil {
		return "", err
	}

	// Normalize Statement (may be a single object or an array)
	if stmts, ok := doc["Statement"]; ok {
		switch v := stmts.(type) {
		case []any:
			for _, s := range v {
				if stmt, ok := s.(map[string]any); ok {
					normalizePolicyStatement(stmt)
				}
			}
		case map[string]any:
			normalizePolicyStatement(v)
			doc["Statement"] = []any{v}
		}
	}

	normalized, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	return string(normalized), nil
}

// normalizePolicyStatement normalizes a single IAM policy statement.
// Action/Resource fields are always expanded to arrays (matching
// radosgw_iam_policy_document output). Principal identifiers are
// collapsed to scalars when single-valued (also matching the data source).
func normalizePolicyStatement(stmt map[string]any) {
	// Action and Resource: always arrays (data source always outputs arrays)
	for _, key := range []string{"Action", "NotAction", "Resource", "NotResource"} {
		if v, ok := stmt[key]; ok {
			stmt[key] = ensureSlice(v)
		}
	}

	// Principal and NotPrincipal: collapse single-element to scalar
	// (data source does this in buildPrincipals)
	for _, key := range []string{"Principal", "NotPrincipal"} {
		if v, ok := stmt[key]; ok {
			switch p := v.(type) {
			case map[string]any:
				for pk, pv := range p {
					p[pk] = collapseSingleElementSlice(pv)
				}
				// "*" or other scalar left as-is
			}
		}
	}
}

// ensureSlice converts a scalar value to a single-element []any slice.
// If the value is already a slice, it is returned as-is.
func ensureSlice(v any) any {
	if _, ok := v.([]any); ok {
		return v
	}
	return []any{v}
}

// collapseSingleElementSlice converts a single-element []any slice to
// its contained value. Multi-element slices and non-slices are returned as-is.
func collapseSingleElementSlice(v any) any {
	if arr, ok := v.([]any); ok && len(arr) == 1 {
		return arr[0]
	}
	return v
}

// arePoliciesEquivalent compares two IAM policy JSON strings for semantic
// equivalence. It normalizes both to the same canonical form before comparison,
// handling differences like single-element arrays vs scalars in Action,
// Resource, and Principal.
func arePoliciesEquivalent(policy1, policy2 string) (bool, error) {
	normalized1, err := normalizeIAMPolicyJSON(policy1)
	if err != nil {
		return false, err
	}
	normalized2, err := normalizeIAMPolicyJSON(policy2)
	if err != nil {
		return false, err
	}
	return normalized1 == normalized2, nil
}
