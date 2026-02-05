package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &PolicyDocumentDataSource{}

func NewIAMPolicyDocumentDataSource() datasource.DataSource {
	return &PolicyDocumentDataSource{}
}

// PolicyDocumentDataSource defines the data source implementation.
type PolicyDocumentDataSource struct{}

// PolicyDocumentDataSourceModel describes the data source data model.
type PolicyDocumentDataSourceModel struct {
	Version    types.String           `tfsdk:"version"`
	PolicyID   types.String           `tfsdk:"policy_id"`
	Statements []PolicyStatementModel `tfsdk:"statement"`
	JSON       types.String           `tfsdk:"json"`
}

// PolicyStatementModel describes a policy statement.
type PolicyStatementModel struct {
	Sid           types.String           `tfsdk:"sid"`
	Effect        types.String           `tfsdk:"effect"`
	Actions       types.Set              `tfsdk:"actions"`
	NotActions    types.Set              `tfsdk:"not_actions"`
	Resources     types.Set              `tfsdk:"resources"`
	NotResources  types.Set              `tfsdk:"not_resources"`
	Principals    []PolicyPrincipalModel `tfsdk:"principals"`
	NotPrincipals []PolicyPrincipalModel `tfsdk:"not_principals"`
	Conditions    []PolicyConditionModel `tfsdk:"condition"`
}

// PolicyPrincipalModel describes a principal in a policy statement.
type PolicyPrincipalModel struct {
	Type        types.String `tfsdk:"type"`
	Identifiers types.Set    `tfsdk:"identifiers"`
}

// PolicyConditionModel describes a condition in a policy statement.
type PolicyConditionModel struct {
	Test     types.String `tfsdk:"test"`
	Variable types.String `tfsdk:"variable"`
	Values   types.List   `tfsdk:"values"`
}

func (d *PolicyDocumentDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iam_policy_document"
}

func (d *PolicyDocumentDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Generates an IAM policy document in JSON format for use with resources that " +
			"require policies, such as `radosgw_role`, `radosgw_role_policy`, and `radosgw_s3_bucket_policy`. " +
			"This data source allows you to define policies using HCL instead of writing raw JSON.",

		Attributes: map[string]schema.Attribute{
			"version": schema.StringAttribute{
				MarkdownDescription: "IAM policy document version. Valid values: `2012-10-17` (default), `2008-10-17`.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("2012-10-17", "2008-10-17"),
				},
			},
			"policy_id": schema.StringAttribute{
				MarkdownDescription: "Optional identifier for the policy.",
				Optional:            true,
			},
			"json": schema.StringAttribute{
				MarkdownDescription: "The generated IAM policy document in JSON format.",
				Computed:            true,
			},
		},

		Blocks: map[string]schema.Block{
			"statement": schema.ListNestedBlock{
				MarkdownDescription: "A policy statement. Multiple statements can be specified.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"sid": schema.StringAttribute{
							MarkdownDescription: "Optional statement identifier.",
							Optional:            true,
						},
						"effect": schema.StringAttribute{
							MarkdownDescription: "Whether this statement allows or denies access. Valid values: `Allow`, `Deny`. Default: `Allow`.",
							Optional:            true,
							Validators: []validator.String{
								stringvalidator.OneOf("Allow", "Deny"),
							},
						},
						"actions": schema.SetAttribute{
							MarkdownDescription: "List of actions that this statement applies to (e.g., `s3:GetObject`, `s3:*`).",
							Optional:            true,
							ElementType:         types.StringType,
						},
						"not_actions": schema.SetAttribute{
							MarkdownDescription: "List of actions that this statement does NOT apply to. Use with `Allow` to create an allow-all-except policy.",
							Optional:            true,
							ElementType:         types.StringType,
						},
						"resources": schema.SetAttribute{
							MarkdownDescription: "List of resources that this statement applies to (e.g., `arn:aws:s3:::bucket/*`).",
							Optional:            true,
							ElementType:         types.StringType,
						},
						"not_resources": schema.SetAttribute{
							MarkdownDescription: "List of resources that this statement does NOT apply to.",
							Optional:            true,
							ElementType:         types.StringType,
						},
					},
					Blocks: map[string]schema.Block{
						"principals": schema.ListNestedBlock{
							MarkdownDescription: "Principal (entity) to which this statement applies.",
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"type": schema.StringAttribute{
										MarkdownDescription: "Type of principal. Valid values: `AWS`, `Federated`, `*`. Note: `Service` principals are not supported in RadosGW.",
										Required:            true,
									},
									"identifiers": schema.SetAttribute{
										MarkdownDescription: "List of identifiers for the principal (e.g., ARNs, account IDs, `*`).",
										Required:            true,
										ElementType:         types.StringType,
									},
								},
							},
						},
						"not_principals": schema.ListNestedBlock{
							MarkdownDescription: "Principal to which this statement does NOT apply.",
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"type": schema.StringAttribute{
										MarkdownDescription: "Type of principal. Valid values: `AWS`, `Federated`, `*`. Note: `Service` principals are not supported in RadosGW.",
										Required:            true,
									},
									"identifiers": schema.SetAttribute{
										MarkdownDescription: "List of identifiers for the principal.",
										Required:            true,
										ElementType:         types.StringType,
									},
								},
							},
						},
						"condition": schema.ListNestedBlock{
							MarkdownDescription: "Condition that must be satisfied for this statement to apply.",
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"test": schema.StringAttribute{
										MarkdownDescription: "Condition operator (e.g., `StringEquals`, `StringLike`, `ArnLike`).",
										Required:            true,
									},
									"variable": schema.StringAttribute{
										MarkdownDescription: "Context key to evaluate (e.g., `aws:username`, `s3:prefix`).",
										Required:            true,
									},
									"values": schema.ListAttribute{
										MarkdownDescription: "Values to compare against the context key.",
										Required:            true,
										ElementType:         types.StringType,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (d *PolicyDocumentDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// This data source doesn't need any provider configuration
}

func (d *PolicyDocumentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data PolicyDocumentDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build the policy document
	policy := make(map[string]any)

	// Set version (default to 2012-10-17)
	version := "2012-10-17"
	if !data.Version.IsNull() && data.Version.ValueString() != "" {
		version = data.Version.ValueString()
	}
	policy["Version"] = version

	// Set policy ID if provided
	if !data.PolicyID.IsNull() && data.PolicyID.ValueString() != "" {
		policy["Id"] = data.PolicyID.ValueString()
	}

	// Build statements
	if len(data.Statements) > 0 {
		statements := make([]map[string]any, 0, len(data.Statements))

		for _, stmt := range data.Statements {
			statement := make(map[string]any)

			// Sid
			if !stmt.Sid.IsNull() && stmt.Sid.ValueString() != "" {
				statement["Sid"] = stmt.Sid.ValueString()
			}

			// Effect (default to Allow)
			effect := "Allow"
			if !stmt.Effect.IsNull() && stmt.Effect.ValueString() != "" {
				effect = stmt.Effect.ValueString()
			}
			statement["Effect"] = effect

			// Actions
			if !stmt.Actions.IsNull() {
				var actions []string
				resp.Diagnostics.Append(stmt.Actions.ElementsAs(ctx, &actions, false)...)
				if resp.Diagnostics.HasError() {
					return
				}
				if len(actions) > 0 {
					statement["Action"] = actions
				}
			}

			// NotActions
			if !stmt.NotActions.IsNull() {
				var notActions []string
				resp.Diagnostics.Append(stmt.NotActions.ElementsAs(ctx, &notActions, false)...)
				if resp.Diagnostics.HasError() {
					return
				}
				if len(notActions) > 0 {
					statement["NotAction"] = notActions
				}
			}

			// Resources
			if !stmt.Resources.IsNull() {
				var resources []string
				resp.Diagnostics.Append(stmt.Resources.ElementsAs(ctx, &resources, false)...)
				if resp.Diagnostics.HasError() {
					return
				}
				if len(resources) > 0 {
					statement["Resource"] = resources
				}
			}

			// NotResources
			if !stmt.NotResources.IsNull() {
				var notResources []string
				resp.Diagnostics.Append(stmt.NotResources.ElementsAs(ctx, &notResources, false)...)
				if resp.Diagnostics.HasError() {
					return
				}
				if len(notResources) > 0 {
					statement["NotResource"] = notResources
				}
			}

			// Principals
			if len(stmt.Principals) > 0 {
				principals := d.buildPrincipals(ctx, stmt.Principals, &resp.Diagnostics)
				if resp.Diagnostics.HasError() {
					return
				}
				if principals != nil {
					statement["Principal"] = principals
				}
			}

			// NotPrincipals
			if len(stmt.NotPrincipals) > 0 {
				notPrincipals := d.buildPrincipals(ctx, stmt.NotPrincipals, &resp.Diagnostics)
				if resp.Diagnostics.HasError() {
					return
				}
				if notPrincipals != nil {
					statement["NotPrincipal"] = notPrincipals
				}
			}

			// Conditions
			if len(stmt.Conditions) > 0 {
				conditions := d.buildConditions(ctx, stmt.Conditions, &resp.Diagnostics)
				if resp.Diagnostics.HasError() {
					return
				}
				if conditions != nil {
					statement["Condition"] = conditions
				}
			}

			statements = append(statements, statement)
		}

		policy["Statement"] = statements
	}

	// Generate JSON
	jsonBytes, err := json.Marshal(policy)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Generating Policy JSON",
			fmt.Sprintf("Could not generate policy JSON: %s", err.Error()),
		)
		return
	}

	data.JSON = types.StringValue(string(jsonBytes))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *PolicyDocumentDataSource) buildPrincipals(ctx context.Context, principals []PolicyPrincipalModel, diags *diag.Diagnostics) any {
	// Check for wildcard principal
	for _, p := range principals {
		if p.Type.ValueString() == "*" {
			return "*"
		}
	}

	// Build principal map
	principalMap := make(map[string]any)

	for _, p := range principals {
		var identifiers []string
		diags.Append(p.Identifiers.ElementsAs(ctx, &identifiers, false)...)
		if diags.HasError() {
			return nil
		}

		principalType := p.Type.ValueString()

		// If only one identifier, use string; otherwise use array
		if len(identifiers) == 1 {
			principalMap[principalType] = identifiers[0]
		} else if len(identifiers) > 1 {
			principalMap[principalType] = identifiers
		}
	}

	if len(principalMap) == 0 {
		return nil
	}

	return principalMap
}

func (d *PolicyDocumentDataSource) buildConditions(ctx context.Context, conditions []PolicyConditionModel, diags *diag.Diagnostics) map[string]any {
	if len(conditions) == 0 {
		return nil
	}

	// Group conditions by test (operator)
	conditionMap := make(map[string]map[string]any)

	for _, c := range conditions {
		test := c.Test.ValueString()
		variable := c.Variable.ValueString()

		var values []string
		diags.Append(c.Values.ElementsAs(ctx, &values, false)...)
		if diags.HasError() {
			return nil
		}

		if _, ok := conditionMap[test]; !ok {
			conditionMap[test] = make(map[string]any)
		}

		// If only one value, use string; otherwise use array
		if len(values) == 1 {
			conditionMap[test][variable] = values[0]
		} else if len(values) > 1 {
			conditionMap[test][variable] = values
		}
	}

	// Convert to properly typed map
	result := make(map[string]any)

	// Sort keys for consistent output
	tests := make([]string, 0, len(conditionMap))
	for test := range conditionMap {
		tests = append(tests, test)
	}
	sort.Strings(tests)

	for _, test := range tests {
		result[test] = conditionMap[test]
	}

	return result
}
