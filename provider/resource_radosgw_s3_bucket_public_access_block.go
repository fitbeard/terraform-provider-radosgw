package provider

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &S3BucketPublicAccessBlockResource{}
var _ resource.ResourceWithImportState = &S3BucketPublicAccessBlockResource{}
var _ resource.ResourceWithModifyPlan = &S3BucketPublicAccessBlockResource{}

func NewS3BucketPublicAccessBlockResource() resource.Resource {
	return &S3BucketPublicAccessBlockResource{}
}

// S3BucketPublicAccessBlockResource defines the resource implementation.
type S3BucketPublicAccessBlockResource struct {
	client *RadosgwClient
}

// =============================================================================
// Data model
// =============================================================================

type S3BucketPublicAccessBlockModel struct {
	Bucket                types.String `tfsdk:"bucket"`
	BlockPublicAcls       types.Bool   `tfsdk:"block_public_acls"`
	BlockPublicPolicy     types.Bool   `tfsdk:"block_public_policy"`
	IgnorePublicAcls      types.Bool   `tfsdk:"ignore_public_acls"`
	RestrictPublicBuckets types.Bool   `tfsdk:"restrict_public_buckets"`
	SkipDestroy           types.Bool   `tfsdk:"skip_destroy"`
}

// =============================================================================
// Resource interface methods
// =============================================================================

func (r *S3BucketPublicAccessBlockResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_s3_bucket_public_access_block"
}

func (r *S3BucketPublicAccessBlockResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the S3 Public Access Block configuration for a bucket in RadosGW, via the " +
			"standard S3 `PutPublicAccessBlock` API. This is the RadosGW equivalent of " +
			"`aws_s3_bucket_public_access_block` and lets you block or ignore public access granted via ACLs and " +
			"bucket policies.\n\n" +
			"### Ceph version support\n\n" +
			"Support is **per-setting** and depends on the provider's `ceph_version`:\n\n" +
			"| Setting | Minimum Ceph release |\n" +
			"| --- | --- |\n" +
			"| `block_public_acls` | Squid (19) or earlier |\n" +
			"| `ignore_public_acls` | Squid (19) or earlier |\n" +
			"| `block_public_policy` | Squid (19) or earlier |\n" +
			"| `restrict_public_buckets` | **Tentacle (20)** — added in 20.2.0 |\n\n" +
			"`restrict_public_buckets = true` is rejected at plan time when `ceph_version` is older than Tentacle, " +
			"where RadosGW does not implement the property and would silently fail to enforce it. The other three " +
			"settings work on all supported releases.\n\n" +
			"~> **Verified on Ceph 21.2.0 (Umbrella):** all four settings round-trip and are enforced " +
			"(`block_public_acls` rejects public-ACL `PutObject`; `ignore_public_acls` makes a public object's " +
			"anonymous read return 403; `block_public_policy` rejects a public bucket policy). On **Tentacle before " +
			"20.2.3**, `block_public_acls` had a bug that rejected ordinary (non-public) uploads — upgrade to 20.2.3+ " +
			"before enabling it.\n\n" +
			"~> **Note:** A bucket has a single public-access-block configuration. Declaring multiple " +
			"`radosgw_s3_bucket_public_access_block` resources for the same bucket will cause them to overwrite each " +
			"other.",

		Attributes: map[string]schema.Attribute{
			"bucket": schema.StringAttribute{
				MarkdownDescription: "The name of the bucket.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"block_public_acls": schema.BoolAttribute{
				MarkdownDescription: "Whether RadosGW should block public ACLs for this bucket. When `true`, " +
					"`PutObject`/`PutBucketAcl` requests that specify a public ACL are rejected. Defaults to `false`.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"block_public_policy": schema.BoolAttribute{
				MarkdownDescription: "Whether RadosGW should block public bucket policies for this bucket. When " +
					"`true`, `PutBucketPolicy` requests that grant public access are rejected. Defaults to `false`.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"ignore_public_acls": schema.BoolAttribute{
				MarkdownDescription: "Whether RadosGW should ignore public ACLs for this bucket. When `true`, any " +
					"existing public ACLs on the bucket and its objects are ignored (access is treated as private). " +
					"Defaults to `false`.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"restrict_public_buckets": schema.BoolAttribute{
				MarkdownDescription: "Whether RadosGW should restrict public bucket policies for this bucket. When " +
					"`true`, access to a bucket with a public policy is restricted to authorized users within the " +
					"bucket owner's account. Defaults to `false`. **Requires Ceph 20 (Tentacle) or higher** — this " +
					"property was added in Tentacle 20.2.0 and setting it to `true` is rejected at plan time on older " +
					"releases.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"skip_destroy": schema.BoolAttribute{
				MarkdownDescription: "When `true`, the public-access-block configuration is left in place on the " +
					"bucket when this resource is destroyed (instead of being removed). Defaults to `false`.",
				Optional: true,
			},
		},
	}
}

func (r *S3BucketPublicAccessBlockResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ModifyPlan rejects restrict_public_buckets=true on clusters older than
// Tentacle (20.x). The RestrictPublicBuckets property was added to RadosGW in
// Tentacle 20.2.0; on older releases RadosGW does not implement it, so enabling
// it would give a false sense of a security control that is not enforced. The
// other three settings are supported on all releases, so the resource itself is
// not version-gated.
func (r *S3BucketPublicAccessBlockResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() || r.client == nil {
		return
	}

	var plan S3BucketPublicAccessBlockModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if plan.RestrictPublicBuckets.ValueBool() && !r.client.CephVersion.GreaterThanOrEqual(CephVersion_Tentacle) {
		resp.Diagnostics.AddAttributeError(
			path.Root("restrict_public_buckets"),
			"restrict_public_buckets requires Ceph 20 (Tentacle)",
			fmt.Sprintf("The RestrictPublicBuckets property of the S3 public access block was added in Ceph "+
				"Tentacle (20.2.0), but the provider targets %s. On older releases RadosGW does not implement this "+
				"property and would not enforce it. Set the provider `ceph_version` to `20`/`tentacle` or higher, or "+
				"leave `restrict_public_buckets` unset/false.", r.client.CephVersion),
		)
	}
}

// =============================================================================
// CRUD
// =============================================================================

func (r *S3BucketPublicAccessBlockResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan S3BucketPublicAccessBlockModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if diags := r.putPublicAccessBlock(ctx, plan); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	tflog.Trace(ctx, "Created S3 bucket public access block", map[string]any{"bucket": plan.Bucket.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *S3BucketPublicAccessBlockResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state S3BucketPublicAccessBlockModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucket := state.Bucket.ValueString()

	output, err := r.client.S3.GetPublicAccessBlock(ctx, &s3.GetPublicAccessBlockInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		if isS3NoSuchPublicAccessBlock(err) || isS3BucketNotFound(err) {
			tflog.Info(ctx, "S3 bucket public access block not found, removing from state", map[string]any{"bucket": bucket})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading S3 Bucket Public Access Block",
			fmt.Sprintf("Could not read public access block for bucket %s: %s", bucket, err),
		)
		return
	}

	flattenPublicAccessBlock(output.PublicAccessBlockConfiguration, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *S3BucketPublicAccessBlockResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan S3BucketPublicAccessBlockModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if diags := r.putPublicAccessBlock(ctx, plan); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	tflog.Debug(ctx, "Updated S3 bucket public access block", map[string]any{"bucket": plan.Bucket.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *S3BucketPublicAccessBlockResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state S3BucketPublicAccessBlockModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !state.SkipDestroy.IsNull() && state.SkipDestroy.ValueBool() {
		tflog.Trace(ctx, "skip_destroy set, leaving public access block in place", map[string]any{"bucket": state.Bucket.ValueString()})
		return
	}

	bucket := state.Bucket.ValueString()

	_, err := r.client.S3.DeletePublicAccessBlock(ctx, &s3.DeletePublicAccessBlockInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		if isS3NoSuchPublicAccessBlock(err) || isS3BucketNotFound(err) {
			return
		}
		resp.Diagnostics.AddError(
			"Error Deleting S3 Bucket Public Access Block",
			fmt.Sprintf("Could not delete public access block for bucket %s: %s", bucket, err),
		)
		return
	}

	tflog.Trace(ctx, "Deleted S3 bucket public access block", map[string]any{"bucket": bucket})
}

func (r *S3BucketPublicAccessBlockResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("bucket"), req, resp)
}

// =============================================================================
// Helpers
// =============================================================================

// putPublicAccessBlock builds and applies the configuration for the model.
func (r *S3BucketPublicAccessBlockResource) putPublicAccessBlock(ctx context.Context, plan S3BucketPublicAccessBlockModel) diag.Diagnostics {
	var diags diag.Diagnostics

	_, err := r.client.S3.PutPublicAccessBlock(ctx, &s3.PutPublicAccessBlockInput{
		Bucket: aws.String(plan.Bucket.ValueString()),
		PublicAccessBlockConfiguration: &s3types.PublicAccessBlockConfiguration{
			BlockPublicAcls:       aws.Bool(plan.BlockPublicAcls.ValueBool()),
			BlockPublicPolicy:     aws.Bool(plan.BlockPublicPolicy.ValueBool()),
			IgnorePublicAcls:      aws.Bool(plan.IgnorePublicAcls.ValueBool()),
			RestrictPublicBuckets: aws.Bool(plan.RestrictPublicBuckets.ValueBool()),
		},
	})
	if err != nil {
		diags.AddError(
			"Error Applying S3 Bucket Public Access Block",
			fmt.Sprintf("Could not set public access block for bucket %s: %s", plan.Bucket.ValueString(), err),
		)
	}
	return diags
}

// flattenPublicAccessBlock maps the API response into the model. A nil pointer
// maps to false (matching the schema default), so computed values are never left
// unknown.
func flattenPublicAccessBlock(cfg *s3types.PublicAccessBlockConfiguration, state *S3BucketPublicAccessBlockModel) {
	if cfg == nil {
		state.BlockPublicAcls = types.BoolValue(false)
		state.BlockPublicPolicy = types.BoolValue(false)
		state.IgnorePublicAcls = types.BoolValue(false)
		state.RestrictPublicBuckets = types.BoolValue(false)
		return
	}
	state.BlockPublicAcls = types.BoolValue(aws.ToBool(cfg.BlockPublicAcls))
	state.BlockPublicPolicy = types.BoolValue(aws.ToBool(cfg.BlockPublicPolicy))
	state.IgnorePublicAcls = types.BoolValue(aws.ToBool(cfg.IgnorePublicAcls))
	state.RestrictPublicBuckets = types.BoolValue(aws.ToBool(cfg.RestrictPublicBuckets))
}

// isS3NoSuchPublicAccessBlock reports whether the S3 API responded with
// NoSuchPublicAccessBlockConfiguration — the bucket exists but has no public
// access block configured.
func isS3NoSuchPublicAccessBlock(err error) bool {
	return isS3ErrorCode(err, "NoSuchPublicAccessBlockConfiguration")
}
