package provider

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
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
var _ resource.Resource = &S3BucketCorsConfigurationResource{}
var _ resource.ResourceWithImportState = &S3BucketCorsConfigurationResource{}

func NewS3BucketCorsConfigurationResource() resource.Resource {
	return &S3BucketCorsConfigurationResource{}
}

// S3BucketCorsConfigurationResource defines the resource implementation.
type S3BucketCorsConfigurationResource struct {
	client *RadosgwClient
}

// =============================================================================
// Data models
// =============================================================================

type S3BucketCorsConfigurationModel struct {
	Bucket    types.String `tfsdk:"bucket"`
	CorsRules types.List   `tfsdk:"cors_rule"`
}

type CorsRuleModel struct {
	ID             types.String `tfsdk:"id"`
	AllowedHeaders types.Set    `tfsdk:"allowed_headers"`
	AllowedMethods types.Set    `tfsdk:"allowed_methods"`
	AllowedOrigins types.Set    `tfsdk:"allowed_origins"`
	ExposeHeaders  types.Set    `tfsdk:"expose_headers"`
	MaxAgeSeconds  types.Int64  `tfsdk:"max_age_seconds"`
}

func corsRuleAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"id":              types.StringType,
		"allowed_headers": types.SetType{ElemType: types.StringType},
		"allowed_methods": types.SetType{ElemType: types.StringType},
		"allowed_origins": types.SetType{ElemType: types.StringType},
		"expose_headers":  types.SetType{ElemType: types.StringType},
		"max_age_seconds": types.Int64Type,
	}
}

// =============================================================================
// Resource interface methods
// =============================================================================

func (r *S3BucketCorsConfigurationResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_s3_bucket_cors_configuration"
}

func (r *S3BucketCorsConfigurationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Cross-Origin Resource Sharing (CORS) configuration for an S3 bucket in RadosGW " +
			"using the standard S3 `PutBucketCors` API. CORS lets web applications served from one origin access " +
			"objects in a bucket from a different origin.\n\n" +
			"~> **Note:** A bucket has a single CORS configuration. Declaring multiple " +
			"`radosgw_s3_bucket_cors_configuration` resources for the same bucket will cause them to overwrite each " +
			"other. Provide all rules in one resource via multiple `cors_rule` blocks.\n\n" +
			"~> **Note:** When this resource is destroyed, the CORS configuration is removed from the bucket.\n\n" +
			"### Per-bucket vs. global CORS\n\n" +
			"This resource manages **per-bucket** CORS (the S3 CORS API), which is what almost all use cases need. " +
			"RadosGW also supports an optional **global (gateway-wide) CORS policy** configured on the Ceph cluster, " +
			"not through the S3 API and therefore not managed by this resource. It is set with `ceph config` and " +
			"applies to every bucket served by the gateway:\n\n" +
			"```bash\n" +
			"ceph config set client.rgw rgw_gcors_allow_origins \"https://app.example.com\"\n" +
			"ceph config set client.rgw rgw_gcors_allow_methods \"GET, PUT, POST\"\n" +
			"ceph config set client.rgw rgw_gcors_allow_headers \"*\"\n" +
			"ceph config set client.rgw rgw_gcors_expose_headers \"ETag\"\n" +
			"```\n\n" +
			"These options are **not runtime-updatable** — restart the RGW daemon(s) after changing them. Use the " +
			"global policy for a gateway-wide default; use this resource for per-bucket rules.",

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
			"cors_rule": schema.ListNestedBlock{
				MarkdownDescription: "A set of origins and methods (cross-origin access that you want to allow). " +
					"You can configure up to 100 rules; provide each as a separate `cors_rule` block.",
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "A unique identifier for the rule. The value cannot be longer than 255 characters.",
							Optional:            true,
						},
						"allowed_headers": schema.SetAttribute{
							MarkdownDescription: "Set of headers that are specified in the `Access-Control-Request-Headers` header. " +
								"Use `*` to allow any header. Order is not significant (RadosGW normalizes it).",
							Optional:    true,
							ElementType: types.StringType,
						},
						"allowed_methods": schema.SetAttribute{
							MarkdownDescription: "Set of HTTP methods that you allow the origin to execute. " +
								"Valid values: `GET`, `PUT`, `HEAD`, `POST`, `DELETE`. Order is not significant " +
								"(RadosGW normalizes it).",
							Required:    true,
							ElementType: types.StringType,
							Validators: []validator.Set{
								setvalidator.SizeAtLeast(1),
							},
						},
						"allowed_origins": schema.SetAttribute{
							MarkdownDescription: "Set of origins you want customers to be able to access the bucket from. " +
								"Use `*` to allow any origin. Order is not significant (RadosGW normalizes it).",
							Required:    true,
							ElementType: types.StringType,
							Validators: []validator.Set{
								setvalidator.SizeAtLeast(1),
							},
						},
						"expose_headers": schema.SetAttribute{
							MarkdownDescription: "Set of headers in the response that you want customers to be able to access " +
								"from their applications (for example, from a JavaScript `XMLHttpRequest` object).",
							Optional:    true,
							ElementType: types.StringType,
						},
						"max_age_seconds": schema.Int64Attribute{
							MarkdownDescription: "The time in seconds that your browser is to cache the preflight response for the specified resource.",
							Optional:            true,
						},
					},
				},
			},
		},
	}
}

func (r *S3BucketCorsConfigurationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *S3BucketCorsConfigurationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan S3BucketCorsConfigurationModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if diags := r.putCors(ctx, plan); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	tflog.Trace(ctx, "Created S3 bucket CORS configuration", map[string]any{"bucket": plan.Bucket.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *S3BucketCorsConfigurationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state S3BucketCorsConfigurationModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucket := state.Bucket.ValueString()

	output, err := r.client.S3.GetBucketCors(ctx, &s3.GetBucketCorsInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		if isS3NoSuchCORSConfiguration(err) {
			tflog.Info(ctx, "S3 bucket CORS configuration not found, removing from state", map[string]any{"bucket": bucket})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading S3 Bucket CORS Configuration",
			fmt.Sprintf("Could not read CORS configuration for bucket %s: %s", bucket, err),
		)
		return
	}

	state.Bucket = types.StringValue(bucket)
	resp.Diagnostics.Append(flattenCorsConfiguration(ctx, output, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *S3BucketCorsConfigurationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan S3BucketCorsConfigurationModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if diags := r.putCors(ctx, plan); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	tflog.Debug(ctx, "Updated S3 bucket CORS configuration", map[string]any{"bucket": plan.Bucket.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *S3BucketCorsConfigurationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state S3BucketCorsConfigurationModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucket := state.Bucket.ValueString()

	_, err := r.client.S3.DeleteBucketCors(ctx, &s3.DeleteBucketCorsInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		if isS3NoSuchCORSConfiguration(err) {
			return
		}
		resp.Diagnostics.AddError(
			"Error Deleting S3 Bucket CORS Configuration",
			fmt.Sprintf("Could not delete CORS configuration for bucket %s: %s", bucket, err),
		)
		return
	}

	tflog.Trace(ctx, "Deleted S3 bucket CORS configuration", map[string]any{"bucket": bucket})
}

func (r *S3BucketCorsConfigurationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("bucket"), req, resp)
}

// =============================================================================
// Helpers
// =============================================================================

// putCors builds and applies the CORS configuration for the given model.
func (r *S3BucketCorsConfigurationResource) putCors(ctx context.Context, plan S3BucketCorsConfigurationModel) diag.Diagnostics {
	bucket := plan.Bucket.ValueString()

	corsConfig, diags := expandCorsConfiguration(ctx, plan)
	if diags.HasError() {
		return diags
	}

	_, err := r.client.S3.PutBucketCors(ctx, &s3.PutBucketCorsInput{
		Bucket:            aws.String(bucket),
		CORSConfiguration: corsConfig,
	})
	if err != nil {
		diags.AddError(
			"Error Applying S3 Bucket CORS Configuration",
			fmt.Sprintf("Could not set CORS configuration for bucket %s: %s", bucket, err),
		)
	}
	return diags
}

// isS3NoSuchCORSConfiguration returns true when the S3 API responds with
// NoSuchCORSConfiguration — the bucket exists but has no CORS configuration.
func isS3NoSuchCORSConfiguration(err error) bool {
	return isS3ErrorCode(err, "NoSuchCORSConfiguration")
}

// =============================================================================
// Expand: Terraform model → S3 API types
// =============================================================================

func expandCorsConfiguration(ctx context.Context, model S3BucketCorsConfigurationModel) (*s3types.CORSConfiguration, diag.Diagnostics) {
	var allDiags diag.Diagnostics

	var rules []CorsRuleModel
	allDiags.Append(model.CorsRules.ElementsAs(ctx, &rules, false)...)
	if allDiags.HasError() {
		return nil, allDiags
	}

	config := &s3types.CORSConfiguration{}
	for _, rule := range rules {
		s3Rule := s3types.CORSRule{}

		if !rule.ID.IsNull() && rule.ID.ValueString() != "" {
			s3Rule.ID = aws.String(rule.ID.ValueString())
		}

		s3Rule.AllowedHeaders = expandStringSet(ctx, rule.AllowedHeaders, &allDiags)
		s3Rule.AllowedMethods = expandStringSet(ctx, rule.AllowedMethods, &allDiags)
		s3Rule.AllowedOrigins = expandStringSet(ctx, rule.AllowedOrigins, &allDiags)
		s3Rule.ExposeHeaders = expandStringSet(ctx, rule.ExposeHeaders, &allDiags)

		if !rule.MaxAgeSeconds.IsNull() {
			s3Rule.MaxAgeSeconds = aws.Int32(int32(rule.MaxAgeSeconds.ValueInt64()))
		}

		config.CORSRules = append(config.CORSRules, s3Rule)
	}

	return config, allDiags
}

// expandStringSet converts a types.Set of strings to a []string, or nil when
// the set is null/unknown.
func expandStringSet(ctx context.Context, set types.Set, allDiags *diag.Diagnostics) []string {
	if set.IsNull() || set.IsUnknown() {
		return nil
	}
	var out []string
	allDiags.Append(set.ElementsAs(ctx, &out, false)...)
	return out
}

// =============================================================================
// Flatten: S3 API response → Terraform model
// =============================================================================

func flattenCorsConfiguration(ctx context.Context, output *s3.GetBucketCorsOutput, state *S3BucketCorsConfigurationModel) diag.Diagnostics {
	var allDiags diag.Diagnostics

	ruleModels := make([]CorsRuleModel, 0, len(output.CORSRules))
	for _, rule := range output.CORSRules {
		m := CorsRuleModel{
			AllowedHeaders: flattenStringSet(ctx, rule.AllowedHeaders, &allDiags),
			AllowedMethods: flattenStringSet(ctx, rule.AllowedMethods, &allDiags),
			AllowedOrigins: flattenStringSet(ctx, rule.AllowedOrigins, &allDiags),
			ExposeHeaders:  flattenStringSet(ctx, rule.ExposeHeaders, &allDiags),
		}
		if rule.ID != nil {
			m.ID = types.StringValue(aws.ToString(rule.ID))
		} else {
			m.ID = types.StringNull()
		}
		if rule.MaxAgeSeconds != nil {
			m.MaxAgeSeconds = types.Int64Value(int64(*rule.MaxAgeSeconds))
		} else {
			m.MaxAgeSeconds = types.Int64Null()
		}
		ruleModels = append(ruleModels, m)
	}

	rulesList, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: corsRuleAttrTypes()}, ruleModels)
	allDiags.Append(diags...)
	state.CorsRules = rulesList

	return allDiags
}

// flattenStringSet converts a []string from the S3 API to a types.Set, mapping
// an empty/absent slice to null (so optional sets that were not configured do
// not show spurious drift). Using a Set makes ordering insignificant, which
// matters because RadosGW normalizes the order of allowed methods/headers/origins.
func flattenStringSet(ctx context.Context, in []string, allDiags *diag.Diagnostics) types.Set {
	if len(in) == 0 {
		return types.SetNull(types.StringType)
	}
	set, diags := types.SetValueFrom(ctx, types.StringType, in)
	allDiags.Append(diags...)
	return set
}
