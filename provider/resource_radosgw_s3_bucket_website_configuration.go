package provider

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &S3BucketWebsiteConfigurationResource{}
var _ resource.ResourceWithImportState = &S3BucketWebsiteConfigurationResource{}

func NewS3BucketWebsiteConfigurationResource() resource.Resource {
	return &S3BucketWebsiteConfigurationResource{}
}

// S3BucketWebsiteConfigurationResource defines the resource implementation.
type S3BucketWebsiteConfigurationResource struct {
	client *RadosgwClient
}

// =============================================================================
// Data models
// =============================================================================

type S3BucketWebsiteConfigurationModel struct {
	Bucket              types.String `tfsdk:"bucket"`
	IndexDocument       types.List   `tfsdk:"index_document"`
	ErrorDocument       types.List   `tfsdk:"error_document"`
	RedirectAllRequests types.List   `tfsdk:"redirect_all_requests_to"`
	RoutingRules        types.List   `tfsdk:"routing_rule"`
}

type IndexDocumentModel struct {
	Suffix types.String `tfsdk:"suffix"`
}

type ErrorDocumentModel struct {
	Key types.String `tfsdk:"key"`
}

type RedirectAllRequestsToModel struct {
	HostName types.String `tfsdk:"host_name"`
	Protocol types.String `tfsdk:"protocol"`
}

type RoutingRuleModel struct {
	Condition types.List `tfsdk:"condition"`
	Redirect  types.List `tfsdk:"redirect"`
}

type RoutingRuleConditionModel struct {
	HttpErrorCodeReturnedEquals types.String `tfsdk:"http_error_code_returned_equals"`
	KeyPrefixEquals             types.String `tfsdk:"key_prefix_equals"`
}

type RoutingRuleRedirectModel struct {
	HostName             types.String `tfsdk:"host_name"`
	HttpRedirectCode     types.String `tfsdk:"http_redirect_code"`
	Protocol             types.String `tfsdk:"protocol"`
	ReplaceKeyPrefixWith types.String `tfsdk:"replace_key_prefix_with"`
	ReplaceKeyWith       types.String `tfsdk:"replace_key_with"`
}

// =============================================================================
// Attribute type helpers (for types.List element types)
// =============================================================================

func indexDocumentAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"suffix": types.StringType,
	}
}

func errorDocumentAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"key": types.StringType,
	}
}

func redirectAllRequestsToAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"host_name": types.StringType,
		"protocol":  types.StringType,
	}
}

func routingRuleConditionAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"http_error_code_returned_equals": types.StringType,
		"key_prefix_equals":               types.StringType,
	}
}

func routingRuleRedirectAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"host_name":               types.StringType,
		"http_redirect_code":      types.StringType,
		"protocol":                types.StringType,
		"replace_key_prefix_with": types.StringType,
		"replace_key_with":        types.StringType,
	}
}

func routingRuleAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"condition": types.ListType{ElemType: types.ObjectType{AttrTypes: routingRuleConditionAttrTypes()}},
		"redirect":  types.ListType{ElemType: types.ObjectType{AttrTypes: routingRuleRedirectAttrTypes()}},
	}
}

// =============================================================================
// Resource interface methods
// =============================================================================

func (r *S3BucketWebsiteConfigurationResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_s3_bucket_website_configuration"
}

func (r *S3BucketWebsiteConfigurationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an S3 bucket website configuration in RadosGW. " +
			"This resource configures a bucket to serve static website content.\n\n" +
			"When configured, the bucket content is accessible through the RadosGW " +
			"S3 website endpoint: `http://<bucket>.<rgw-host>:<port>`.\n\n" +
			"~> **Note:** S3 buckets only support a single website configuration. " +
			"Declaring multiple `radosgw_s3_bucket_website_configuration` resources for " +
			"the same bucket will cause conflicts.\n\n" +
			"~> **Note:** When this resource is destroyed, the website configuration is " +
			"removed from the bucket.",

		Attributes: map[string]schema.Attribute{
			"bucket": schema.StringAttribute{
				MarkdownDescription: "The name of the bucket.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},

		Blocks: map[string]schema.Block{
			"index_document": schema.ListNestedBlock{
				MarkdownDescription: "The name of the index document for the website. " +
					"Required if `redirect_all_requests_to` is not specified.",
				Validators: []validator.List{
					listvalidator.SizeAtMost(1),
					listvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("redirect_all_requests_to")),
				},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"suffix": schema.StringAttribute{
							MarkdownDescription: "A suffix that is appended to a request that is for " +
								"a directory on the website endpoint. For example, if the suffix is " +
								"`index.html` and you make a request to `samplebucket/images/`, the data " +
								"that is returned will be for the object with the key name " +
								"`images/index.html`. The suffix must not be empty and must not include " +
								"a slash character.",
							Required: true,
						},
					},
				},
			},

			"error_document": schema.ListNestedBlock{
				MarkdownDescription: "The name of the error document for the website.",
				Validators: []validator.List{
					listvalidator.SizeAtMost(1),
					listvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("redirect_all_requests_to")),
				},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"key": schema.StringAttribute{
							MarkdownDescription: "The object key name to use when a 4XX class error occurs.",
							Required:            true,
						},
					},
				},
			},

			"redirect_all_requests_to": schema.ListNestedBlock{
				MarkdownDescription: "Redirect behavior for every request to this " +
					"bucket's website endpoint. Conflicts with `index_document`, " +
					"`error_document`, and `routing_rule`.",
				Validators: []validator.List{
					listvalidator.SizeAtMost(1),
					listvalidator.ConflictsWith(
						path.MatchRelative().AtParent().AtName("index_document"),
						path.MatchRelative().AtParent().AtName("error_document"),
						path.MatchRelative().AtParent().AtName("routing_rule"),
					),
				},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"host_name": schema.StringAttribute{
							MarkdownDescription: "Name of the host where requests are redirected.",
							Required:            true,
						},
						"protocol": schema.StringAttribute{
							MarkdownDescription: "Protocol to use when redirecting requests. " +
								"The default is the protocol that is used in the original request. " +
								"Valid values: `http`, `https`.",
							Optional: true,
							Validators: []validator.String{
								stringvalidator.OneOf("http", "https"),
							},
						},
					},
				},
			},

			"routing_rule": schema.ListNestedBlock{
				MarkdownDescription: "A list of rules that define when a redirect is applied " +
					"and the redirect behavior. Conflicts with `redirect_all_requests_to`.",
				Validators: []validator.List{
					listvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("redirect_all_requests_to")),
				},
				NestedObject: schema.NestedBlockObject{
					Blocks: map[string]schema.Block{
						"condition": schema.ListNestedBlock{
							MarkdownDescription: "A condition that must be met for the specified " +
								"redirect to apply.",
							Validators: []validator.List{
								listvalidator.SizeAtMost(1),
							},
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"http_error_code_returned_equals": schema.StringAttribute{
										MarkdownDescription: "The HTTP error code when the redirect " +
											"is applied. If specified with `key_prefix_equals`, then " +
											"both conditions must be true for the redirect to be applied.",
										Optional: true,
									},
									"key_prefix_equals": schema.StringAttribute{
										MarkdownDescription: "The object key name prefix when the " +
											"redirect is applied. If specified with " +
											"`http_error_code_returned_equals`, then both conditions " +
											"must be true for the redirect to be applied.",
										Optional: true,
									},
								},
							},
						},

						"redirect": schema.ListNestedBlock{
							MarkdownDescription: "Redirect information. You can redirect requests " +
								"to another host, to another page, or with another protocol.",
							Validators: []validator.List{
								listvalidator.SizeAtMost(1),
								listvalidator.SizeAtLeast(1),
							},
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"host_name": schema.StringAttribute{
										MarkdownDescription: "The host name to use in the redirect request.",
										Optional:            true,
									},
									"http_redirect_code": schema.StringAttribute{
										MarkdownDescription: "The HTTP redirect code to use on the response.",
										Optional:            true,
									},
									"protocol": schema.StringAttribute{
										MarkdownDescription: "Protocol to use when redirecting requests. " +
											"The default is the protocol that is used in the original " +
											"request. Valid values: `http`, `https`.",
										Optional: true,
										Validators: []validator.String{
											stringvalidator.OneOf("http", "https"),
										},
									},
									"replace_key_prefix_with": schema.StringAttribute{
										MarkdownDescription: "The object key prefix to use in the redirect " +
											"request. For example, to redirect requests for all pages with " +
											"prefix `docs/` to `documents/`, set `key_prefix_equals` to " +
											"`docs/` in the condition and `replace_key_prefix_with` to " +
											"`/documents` in the redirect. Cannot be specified with " +
											"`replace_key_with`.",
										Optional: true,
									},
									"replace_key_with": schema.StringAttribute{
										MarkdownDescription: "The specific object key to use in the redirect " +
											"request. For example, redirect request to `error.html`. " +
											"Cannot be specified with `replace_key_prefix_with`.",
										Optional: true,
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

func (r *S3BucketWebsiteConfigurationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
}

// =============================================================================
// CRUD
// =============================================================================

func (r *S3BucketWebsiteConfigurationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan S3BucketWebsiteConfigurationModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucket := plan.Bucket.ValueString()

	websiteConfig, diags := expandWebsiteConfiguration(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	input := &s3.PutBucketWebsiteInput{
		Bucket:               aws.String(bucket),
		WebsiteConfiguration: websiteConfig,
	}

	_, err := r.client.S3.PutBucketWebsite(ctx, input)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating S3 Bucket Website Configuration",
			fmt.Sprintf("Could not set website configuration for bucket %s: %s", bucket, err),
		)
		return
	}

	tflog.Trace(ctx, "Created S3 bucket website configuration", map[string]any{
		"bucket": bucket,
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *S3BucketWebsiteConfigurationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state S3BucketWebsiteConfigurationModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucket := state.Bucket.ValueString()

	output, err := r.client.S3.GetBucketWebsite(ctx, &s3.GetBucketWebsiteInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		if isS3NoSuchWebsiteConfiguration(err) {
			tflog.Info(ctx, "S3 bucket website configuration not found, removing from state", map[string]any{
				"bucket": bucket,
			})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading S3 Bucket Website Configuration",
			fmt.Sprintf("Could not read website configuration for bucket %s: %s", bucket, err),
		)
		return
	}

	// Map response to state
	state.Bucket = types.StringValue(bucket)

	diags := flattenWebsiteConfiguration(ctx, output, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *S3BucketWebsiteConfigurationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan S3BucketWebsiteConfigurationModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucket := plan.Bucket.ValueString()

	websiteConfig, diags := expandWebsiteConfiguration(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	input := &s3.PutBucketWebsiteInput{
		Bucket:               aws.String(bucket),
		WebsiteConfiguration: websiteConfig,
	}

	_, err := r.client.S3.PutBucketWebsite(ctx, input)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating S3 Bucket Website Configuration",
			fmt.Sprintf("Could not update website configuration for bucket %s: %s", bucket, err),
		)
		return
	}

	tflog.Debug(ctx, "Updated S3 bucket website configuration", map[string]any{
		"bucket": bucket,
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *S3BucketWebsiteConfigurationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state S3BucketWebsiteConfigurationModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucket := state.Bucket.ValueString()

	_, err := r.client.S3.DeleteBucketWebsite(ctx, &s3.DeleteBucketWebsiteInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		if isS3NoSuchWebsiteConfiguration(err) {
			return
		}
		resp.Diagnostics.AddError(
			"Error Deleting S3 Bucket Website Configuration",
			fmt.Sprintf("Could not delete website configuration for bucket %s: %s", bucket, err),
		)
		return
	}

	tflog.Trace(ctx, "Deleted S3 bucket website configuration", map[string]any{
		"bucket": bucket,
	})
}

func (r *S3BucketWebsiteConfigurationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("bucket"), req, resp)
}

// =============================================================================
// Helpers
// =============================================================================

// isS3NoSuchWebsiteConfiguration returns true when the S3 API responds with
// NoSuchWebsiteConfiguration — meaning the bucket exists but has no website
// config.
func isS3NoSuchWebsiteConfiguration(err error) bool {
	if err == nil {
		return false
	}
	return isS3ErrorCode(err, "NoSuchWebsiteConfiguration")
}

// isS3ErrorCode checks whether an error from the aws-sdk-go-v2 S3 client
// contains the given error code string.
func isS3ErrorCode(err error, code string) bool {
	if err == nil {
		return false
	}
	// aws-sdk-go-v2 wraps API errors; check the string representation
	// as a simple cross-version compatible approach.
	errStr := err.Error()
	return contains(errStr, code)
}

// contains checks if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// =============================================================================
// Expand: Terraform model → S3 API types
// =============================================================================

func expandWebsiteConfiguration(ctx context.Context, model S3BucketWebsiteConfigurationModel) (*s3types.WebsiteConfiguration, diag.Diagnostics) {
	config := &s3types.WebsiteConfiguration{}
	var allDiags diag.Diagnostics

	// Index Document
	if !model.IndexDocument.IsNull() && len(model.IndexDocument.Elements()) > 0 {
		var docs []IndexDocumentModel
		diags := model.IndexDocument.ElementsAs(ctx, &docs, false)
		allDiags.Append(diags...)
		if len(docs) > 0 {
			config.IndexDocument = &s3types.IndexDocument{
				Suffix: aws.String(docs[0].Suffix.ValueString()),
			}
		}
	}

	// Error Document
	if !model.ErrorDocument.IsNull() && len(model.ErrorDocument.Elements()) > 0 {
		var docs []ErrorDocumentModel
		diags := model.ErrorDocument.ElementsAs(ctx, &docs, false)
		allDiags.Append(diags...)
		if len(docs) > 0 {
			config.ErrorDocument = &s3types.ErrorDocument{
				Key: aws.String(docs[0].Key.ValueString()),
			}
		}
	}

	// Redirect all requests
	if !model.RedirectAllRequests.IsNull() && len(model.RedirectAllRequests.Elements()) > 0 {
		var redirects []RedirectAllRequestsToModel
		diags := model.RedirectAllRequests.ElementsAs(ctx, &redirects, false)
		allDiags.Append(diags...)
		if len(redirects) > 0 {
			redirect := &s3types.RedirectAllRequestsTo{
				HostName: aws.String(redirects[0].HostName.ValueString()),
			}
			if !redirects[0].Protocol.IsNull() && redirects[0].Protocol.ValueString() != "" {
				redirect.Protocol = s3types.Protocol(redirects[0].Protocol.ValueString())
			}
			config.RedirectAllRequestsTo = redirect
		}
	}

	// Routing rules
	if !model.RoutingRules.IsNull() && len(model.RoutingRules.Elements()) > 0 {
		var rules []RoutingRuleModel
		diags := model.RoutingRules.ElementsAs(ctx, &rules, false)
		allDiags.Append(diags...)

		for _, rule := range rules {
			s3Rule := s3types.RoutingRule{}

			// Condition
			if !rule.Condition.IsNull() && len(rule.Condition.Elements()) > 0 {
				var conditions []RoutingRuleConditionModel
				diags := rule.Condition.ElementsAs(ctx, &conditions, false)
				allDiags.Append(diags...)
				if len(conditions) > 0 {
					cond := &s3types.Condition{}
					if !conditions[0].HttpErrorCodeReturnedEquals.IsNull() && conditions[0].HttpErrorCodeReturnedEquals.ValueString() != "" {
						cond.HttpErrorCodeReturnedEquals = aws.String(conditions[0].HttpErrorCodeReturnedEquals.ValueString())
					}
					if !conditions[0].KeyPrefixEquals.IsNull() && conditions[0].KeyPrefixEquals.ValueString() != "" {
						cond.KeyPrefixEquals = aws.String(conditions[0].KeyPrefixEquals.ValueString())
					}
					s3Rule.Condition = cond
				}
			}

			// Redirect
			if !rule.Redirect.IsNull() && len(rule.Redirect.Elements()) > 0 {
				var redirects []RoutingRuleRedirectModel
				diags := rule.Redirect.ElementsAs(ctx, &redirects, false)
				allDiags.Append(diags...)
				if len(redirects) > 0 {
					redir := &s3types.Redirect{}
					if !redirects[0].HostName.IsNull() && redirects[0].HostName.ValueString() != "" {
						redir.HostName = aws.String(redirects[0].HostName.ValueString())
					}
					if !redirects[0].HttpRedirectCode.IsNull() && redirects[0].HttpRedirectCode.ValueString() != "" {
						redir.HttpRedirectCode = aws.String(redirects[0].HttpRedirectCode.ValueString())
					}
					if !redirects[0].Protocol.IsNull() && redirects[0].Protocol.ValueString() != "" {
						redir.Protocol = s3types.Protocol(redirects[0].Protocol.ValueString())
					}
					if !redirects[0].ReplaceKeyPrefixWith.IsNull() && redirects[0].ReplaceKeyPrefixWith.ValueString() != "" {
						redir.ReplaceKeyPrefixWith = aws.String(redirects[0].ReplaceKeyPrefixWith.ValueString())
					}
					if !redirects[0].ReplaceKeyWith.IsNull() && redirects[0].ReplaceKeyWith.ValueString() != "" {
						redir.ReplaceKeyWith = aws.String(redirects[0].ReplaceKeyWith.ValueString())
					}
					s3Rule.Redirect = redir
				}
			}

			config.RoutingRules = append(config.RoutingRules, s3Rule)
		}
	}

	return config, allDiags
}

// =============================================================================
// Flatten: S3 API response → Terraform model
// =============================================================================

func flattenWebsiteConfiguration(ctx context.Context, output *s3.GetBucketWebsiteOutput, state *S3BucketWebsiteConfigurationModel) diag.Diagnostics {
	var allDiags diag.Diagnostics

	// Index Document
	if output.IndexDocument != nil && output.IndexDocument.Suffix != nil {
		indexDoc, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: indexDocumentAttrTypes()}, []IndexDocumentModel{
			{Suffix: types.StringValue(aws.ToString(output.IndexDocument.Suffix))},
		})
		allDiags.Append(diags...)
		state.IndexDocument = indexDoc
	} else {
		state.IndexDocument = types.ListNull(types.ObjectType{AttrTypes: indexDocumentAttrTypes()})
	}

	// Error Document
	if output.ErrorDocument != nil && output.ErrorDocument.Key != nil {
		errorDoc, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: errorDocumentAttrTypes()}, []ErrorDocumentModel{
			{Key: types.StringValue(aws.ToString(output.ErrorDocument.Key))},
		})
		allDiags.Append(diags...)
		state.ErrorDocument = errorDoc
	} else {
		state.ErrorDocument = types.ListNull(types.ObjectType{AttrTypes: errorDocumentAttrTypes()})
	}

	// Redirect All Requests To
	if output.RedirectAllRequestsTo != nil && output.RedirectAllRequestsTo.HostName != nil {
		redirect := RedirectAllRequestsToModel{
			HostName: types.StringValue(aws.ToString(output.RedirectAllRequestsTo.HostName)),
		}
		if output.RedirectAllRequestsTo.Protocol != "" {
			redirect.Protocol = types.StringValue(string(output.RedirectAllRequestsTo.Protocol))
		} else {
			redirect.Protocol = types.StringNull()
		}

		redirectList, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: redirectAllRequestsToAttrTypes()}, []RedirectAllRequestsToModel{redirect})
		allDiags.Append(diags...)
		state.RedirectAllRequests = redirectList
	} else {
		state.RedirectAllRequests = types.ListNull(types.ObjectType{AttrTypes: redirectAllRequestsToAttrTypes()})
	}

	// Routing Rules
	if len(output.RoutingRules) > 0 {
		var ruleModels []RoutingRuleModel

		for _, rule := range output.RoutingRules {
			ruleModel := RoutingRuleModel{}

			// Condition — treat as null when all fields are empty (RadosGW may
			// return an empty Condition struct even when none was configured).
			if rule.Condition != nil &&
				(rule.Condition.HttpErrorCodeReturnedEquals != nil || rule.Condition.KeyPrefixEquals != nil) {
				condModel := RoutingRuleConditionModel{}
				if rule.Condition.HttpErrorCodeReturnedEquals != nil {
					condModel.HttpErrorCodeReturnedEquals = types.StringValue(aws.ToString(rule.Condition.HttpErrorCodeReturnedEquals))
				} else {
					condModel.HttpErrorCodeReturnedEquals = types.StringNull()
				}
				if rule.Condition.KeyPrefixEquals != nil {
					condModel.KeyPrefixEquals = types.StringValue(aws.ToString(rule.Condition.KeyPrefixEquals))
				} else {
					condModel.KeyPrefixEquals = types.StringNull()
				}

				condList, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: routingRuleConditionAttrTypes()}, []RoutingRuleConditionModel{condModel})
				allDiags.Append(diags...)
				ruleModel.Condition = condList
			} else {
				ruleModel.Condition = types.ListNull(types.ObjectType{AttrTypes: routingRuleConditionAttrTypes()})
			}

			// Redirect
			if rule.Redirect != nil {
				redirModel := RoutingRuleRedirectModel{}
				if rule.Redirect.HostName != nil {
					redirModel.HostName = types.StringValue(aws.ToString(rule.Redirect.HostName))
				} else {
					redirModel.HostName = types.StringNull()
				}
				if rule.Redirect.HttpRedirectCode != nil {
					redirModel.HttpRedirectCode = types.StringValue(aws.ToString(rule.Redirect.HttpRedirectCode))
				} else {
					redirModel.HttpRedirectCode = types.StringNull()
				}
				if rule.Redirect.Protocol != "" {
					redirModel.Protocol = types.StringValue(string(rule.Redirect.Protocol))
				} else {
					redirModel.Protocol = types.StringNull()
				}
				if rule.Redirect.ReplaceKeyPrefixWith != nil {
					redirModel.ReplaceKeyPrefixWith = types.StringValue(aws.ToString(rule.Redirect.ReplaceKeyPrefixWith))
				} else {
					redirModel.ReplaceKeyPrefixWith = types.StringNull()
				}
				if rule.Redirect.ReplaceKeyWith != nil {
					redirModel.ReplaceKeyWith = types.StringValue(aws.ToString(rule.Redirect.ReplaceKeyWith))
				} else {
					redirModel.ReplaceKeyWith = types.StringNull()
				}

				redirList, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: routingRuleRedirectAttrTypes()}, []RoutingRuleRedirectModel{redirModel})
				allDiags.Append(diags...)
				ruleModel.Redirect = redirList
			} else {
				ruleModel.Redirect = types.ListNull(types.ObjectType{AttrTypes: routingRuleRedirectAttrTypes()})
			}

			ruleModels = append(ruleModels, ruleModel)
		}

		rulesList, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: routingRuleAttrTypes()}, ruleModels)
		allDiags.Append(diags...)
		state.RoutingRules = rulesList
	} else {
		state.RoutingRules = types.ListNull(types.ObjectType{AttrTypes: routingRuleAttrTypes()})
	}

	return allDiags
}
