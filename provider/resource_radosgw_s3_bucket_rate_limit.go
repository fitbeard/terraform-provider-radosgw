package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ceph/go-ceph/rgw/admin"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &S3BucketRateLimitResource{}
var _ resource.ResourceWithImportState = &S3BucketRateLimitResource{}

func NewS3BucketRateLimitResource() resource.Resource {
	return &S3BucketRateLimitResource{}
}

// S3BucketRateLimitResource defines the resource implementation.
type S3BucketRateLimitResource struct {
	client *RadosgwClient
}

// =============================================================================
// Data model
// =============================================================================

// S3BucketRateLimitResourceModel represents a per-bucket rate limit managed via
// the Admin Ops API. Only the fields supported on Ceph Reef/Squid/Tentacle
// (18-20) are exposed; max-list-ops/max-delete-ops are Ceph 21+ and are added
// when that release is supported.
type S3BucketRateLimitResourceModel struct {
	Bucket        types.String `tfsdk:"bucket"`
	Tenant        types.String `tfsdk:"tenant"`
	Enabled       types.Bool   `tfsdk:"enabled"`
	MaxReadOps    types.Int64  `tfsdk:"max_read_ops"`
	MaxWriteOps   types.Int64  `tfsdk:"max_write_ops"`
	MaxReadBytes  types.Int64  `tfsdk:"max_read_bytes"`
	MaxWriteBytes types.Int64  `tfsdk:"max_write_bytes"`
}

// =============================================================================
// Resource interface methods
// =============================================================================

func (r *S3BucketRateLimitResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_s3_bucket_rate_limit"
}

func (r *S3BucketRateLimitResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the rate limit for a single S3 bucket in RadosGW via the Admin Ops API " +
			"(`ratelimit-scope=bucket`). Rate limits throttle the number of operations and the number of bytes " +
			"read from or written to the bucket per minute.\n\n" +
			"~> **Admin capability required.** Unlike the S3-based bucket sub-resources (`radosgw_s3_bucket_acl`, " +
			"`radosgw_s3_bucket_cors_configuration`, …), rate limits are only available through the Admin Ops API " +
			"and require the **`ratelimit`** capability (`ratelimit=write` to set, `ratelimit=read` to read). A user " +
			"without it receives `AccessDenied`. This capability is cluster-wide and independent of the `buckets` cap.\n\n" +
			"~> **Note:** A bucket has a single rate-limit configuration. Declaring multiple " +
			"`radosgw_s3_bucket_rate_limit` resources for the same bucket will cause them to overwrite each other.\n\n" +
			"~> **Ceph version:** `max_list_ops` and `max_delete_ops` were added in Ceph 21 (Ceph v21). They are not " +
			"exposed by this resource yet because the latest stable release is Tentacle (20); they will be added " +
			"once Ceph 21 is supported. On Reef/Squid/Tentacle only `max_read_ops`, `max_write_ops`, `max_read_bytes`, " +
			"and `max_write_bytes` apply.\n\n" +
			"~> **Note:** When this resource is destroyed, the bucket's rate limit is disabled and its limits are reset to `0`.",

		Attributes: map[string]schema.Attribute{
			"bucket": schema.StringAttribute{
				MarkdownDescription: "The name of the bucket to rate-limit.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"tenant": schema.StringAttribute{
				MarkdownDescription: "The tenant that owns the bucket. Leave unset for a bucket in the default " +
					"namespace. When set, the bucket is addressed as `tenant/bucket`.",
				Optional: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the rate limit is enabled for the bucket. Default: `true`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"max_read_ops": schema.Int64Attribute{
				MarkdownDescription: "Maximum read operations per minute. `0` disables this limit (the default). Omitting the attribute resets it to `0`.",
				Optional:            true,
				Computed:            true,
				Validators: []validator.Int64{
					int64validator.AtLeast(0),
				},
				Default: int64default.StaticInt64(0),
			},
			"max_write_ops": schema.Int64Attribute{
				MarkdownDescription: "Maximum write operations per minute. `0` disables this limit (the default). Omitting the attribute resets it to `0`.",
				Optional:            true,
				Computed:            true,
				Validators: []validator.Int64{
					int64validator.AtLeast(0),
				},
				Default: int64default.StaticInt64(0),
			},
			"max_read_bytes": schema.Int64Attribute{
				MarkdownDescription: "Maximum read bytes per minute. `0` disables this limit (the default). Omitting the attribute resets it to `0`.",
				Optional:            true,
				Computed:            true,
				Validators: []validator.Int64{
					int64validator.AtLeast(0),
				},
				Default: int64default.StaticInt64(0),
			},
			"max_write_bytes": schema.Int64Attribute{
				MarkdownDescription: "Maximum write bytes per minute. `0` disables this limit (the default). Omitting the attribute resets it to `0`.",
				Optional:            true,
				Computed:            true,
				Validators: []validator.Int64{
					int64validator.AtLeast(0),
				},
				Default: int64default.StaticInt64(0),
			},
		},
	}
}

func (r *S3BucketRateLimitResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *S3BucketRateLimitResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan S3BucketRateLimitResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucketID := rateLimitBucketID(plan.Tenant.ValueString(), plan.Bucket.ValueString())

	spec := buildRateLimitSpec(bucketID, plan)
	if err := r.client.Admin.SetIndividualBucketRateLimit(ctx, spec); err != nil {
		resp.Diagnostics.AddError(
			"Error Setting Bucket Rate Limit",
			rateLimitErrorDetail("set rate limit on", bucketID, err),
		)
		return
	}

	tflog.Trace(ctx, "Created S3 bucket rate limit", map[string]any{"bucket": bucketID})

	if diags := r.refreshRateLimit(ctx, bucketID, &plan); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *S3BucketRateLimitResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state S3BucketRateLimitResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucketID := rateLimitBucketID(state.Tenant.ValueString(), state.Bucket.ValueString())

	spec, err := r.client.Admin.GetIndividualBucketRateLimit(ctx, bucketID)
	if err != nil {
		if errors.Is(err, admin.ErrNoSuchBucket) {
			tflog.Info(ctx, "Bucket not found while reading rate limit, removing from state", map[string]any{"bucket": bucketID})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading Bucket Rate Limit",
			rateLimitErrorDetail("read rate limit for", bucketID, err),
		)
		return
	}

	flattenRateLimit(spec, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *S3BucketRateLimitResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan S3BucketRateLimitResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucketID := rateLimitBucketID(plan.Tenant.ValueString(), plan.Bucket.ValueString())

	spec := buildRateLimitSpec(bucketID, plan)
	if err := r.client.Admin.SetIndividualBucketRateLimit(ctx, spec); err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Bucket Rate Limit",
			rateLimitErrorDetail("update rate limit on", bucketID, err),
		)
		return
	}

	tflog.Debug(ctx, "Updated S3 bucket rate limit", map[string]any{"bucket": bucketID})

	if diags := r.refreshRateLimit(ctx, bucketID, &plan); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *S3BucketRateLimitResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state S3BucketRateLimitResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucketID := rateLimitBucketID(state.Tenant.ValueString(), state.Bucket.ValueString())

	// Reset the bucket to an unthrottled state: disabled with all limits at 0.
	disabled := false
	zero := int64(0)
	spec := admin.RateLimitSpec{
		Bucket:        bucketID,
		Enabled:       &disabled,
		MaxReadOps:    &zero,
		MaxWriteOps:   &zero,
		MaxReadBytes:  &zero,
		MaxWriteBytes: &zero,
	}
	if err := r.client.Admin.SetIndividualBucketRateLimit(ctx, spec); err != nil {
		// If the bucket is already gone there is nothing to reset.
		if errors.Is(err, admin.ErrNoSuchBucket) {
			return
		}
		resp.Diagnostics.AddError(
			"Error Deleting Bucket Rate Limit",
			rateLimitErrorDetail("reset rate limit on", bucketID, err),
		)
		return
	}

	tflog.Trace(ctx, "Deleted S3 bucket rate limit", map[string]any{"bucket": bucketID})
}

func (r *S3BucketRateLimitResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Accept either "bucket" or "tenant/bucket".
	id := req.ID
	if tenant, bucket, ok := strings.Cut(id, "/"); ok {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("tenant"), tenant)...)
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("bucket"), bucket)...)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("bucket"), id)...)
}

// =============================================================================
// Helpers
// =============================================================================

// rateLimitBucketID qualifies a bucket name for the Admin Ops rate-limit API.
// RadosGW scopes buckets by tenant, so a tenant bucket is addressed as
// "tenant/bucket". A name that already carries a tenant ("/") is unchanged.
func rateLimitBucketID(tenant, bucket string) string {
	if tenant == "" || strings.Contains(bucket, "/") {
		return bucket
	}
	return tenant + "/" + bucket
}

// buildRateLimitSpec maps the Terraform model to a go-ceph RateLimitSpec, sending
// only the settings the configuration specifies (null/unknown fields are left out
// so RadosGW keeps their current value).
func buildRateLimitSpec(bucketID string, m S3BucketRateLimitResourceModel) admin.RateLimitSpec {
	spec := admin.RateLimitSpec{Bucket: bucketID}
	if !m.Enabled.IsNull() && !m.Enabled.IsUnknown() {
		v := m.Enabled.ValueBool()
		spec.Enabled = &v
	}
	if !m.MaxReadOps.IsNull() && !m.MaxReadOps.IsUnknown() {
		v := m.MaxReadOps.ValueInt64()
		spec.MaxReadOps = &v
	}
	if !m.MaxWriteOps.IsNull() && !m.MaxWriteOps.IsUnknown() {
		v := m.MaxWriteOps.ValueInt64()
		spec.MaxWriteOps = &v
	}
	if !m.MaxReadBytes.IsNull() && !m.MaxReadBytes.IsUnknown() {
		v := m.MaxReadBytes.ValueInt64()
		spec.MaxReadBytes = &v
	}
	if !m.MaxWriteBytes.IsNull() && !m.MaxWriteBytes.IsUnknown() {
		v := m.MaxWriteBytes.ValueInt64()
		spec.MaxWriteBytes = &v
	}
	return spec
}

// flattenRateLimit populates the computed model fields from an Admin API
// response. RadosGW always returns the four core settings; a nil pointer maps to
// the zero value so the computed attributes are never left unknown.
func flattenRateLimit(spec admin.RateLimitSpec, m *S3BucketRateLimitResourceModel) {
	if spec.Enabled != nil {
		m.Enabled = types.BoolValue(*spec.Enabled)
	} else {
		m.Enabled = types.BoolValue(false)
	}
	m.MaxReadOps = int64FromPtr(spec.MaxReadOps)
	m.MaxWriteOps = int64FromPtr(spec.MaxWriteOps)
	m.MaxReadBytes = int64FromPtr(spec.MaxReadBytes)
	m.MaxWriteBytes = int64FromPtr(spec.MaxWriteBytes)
}

// refreshRateLimit reads the bucket's rate limit back from the Admin API and
// populates the computed fields of the model. Returns diagnostics on error.
func (r *S3BucketRateLimitResource) refreshRateLimit(ctx context.Context, bucketID string, m *S3BucketRateLimitResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	spec, err := r.client.Admin.GetIndividualBucketRateLimit(ctx, bucketID)
	if err != nil {
		diags.AddError("Error Reading Bucket Rate Limit", rateLimitErrorDetail("read rate limit for", bucketID, err))
		return diags
	}
	flattenRateLimit(spec, m)
	return diags
}

// int64FromPtr maps an *int64 to a types.Int64, using 0 for a nil pointer.
func int64FromPtr(p *int64) types.Int64 {
	if p == nil {
		return types.Int64Value(0)
	}
	return types.Int64Value(*p)
}

// rateLimitErrorDetail builds a consistent error detail, calling out the most
// likely cause (a missing ratelimit capability) on AccessDenied.
func rateLimitErrorDetail(action, bucketID string, err error) string {
	if errors.Is(err, admin.ErrAccessDenied) {
		return fmt.Sprintf("Could not %s bucket %s: %s. This operation requires the Admin Ops `ratelimit` "+
			"capability (grant it with `radosgw-admin caps add --uid=<user> --caps=\"ratelimit=*\"`).",
			action, bucketID, err)
	}
	return fmt.Sprintf("Could not %s bucket %s: %s", action, bucketID, err)
}
