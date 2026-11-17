package provider

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3control"
	s3controltypes "github.com/aws/aws-sdk-go-v2/service/s3control/types"
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
var _ resource.Resource = &S3AccountPublicAccessBlockResource{}
var _ resource.ResourceWithImportState = &S3AccountPublicAccessBlockResource{}
var _ resource.ResourceWithModifyPlan = &S3AccountPublicAccessBlockResource{}

func NewS3AccountPublicAccessBlockResource() resource.Resource {
	return &S3AccountPublicAccessBlockResource{}
}

// S3AccountPublicAccessBlockResource defines the resource implementation.
type S3AccountPublicAccessBlockResource struct {
	client *RadosgwClient
}

// =============================================================================
// Data model
// =============================================================================

type S3AccountPublicAccessBlockModel struct {
	AccountID             types.String `tfsdk:"account_id"`
	BlockPublicAcls       types.Bool   `tfsdk:"block_public_acls"`
	BlockPublicPolicy     types.Bool   `tfsdk:"block_public_policy"`
	IgnorePublicAcls      types.Bool   `tfsdk:"ignore_public_acls"`
	RestrictPublicBuckets types.Bool   `tfsdk:"restrict_public_buckets"`
	SkipDestroy           types.Bool   `tfsdk:"skip_destroy"`
}

// =============================================================================
// Resource interface methods
// =============================================================================

func (r *S3AccountPublicAccessBlockResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_s3_account_public_access_block"
}

func (r *S3AccountPublicAccessBlockResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the account-wide S3 Public Access Block configuration for a RadosGW account, " +
			"via the S3 Control `PutPublicAccessBlock` API. This is the RadosGW equivalent of " +
			"`aws_s3_account_public_access_block`. The configuration applies to **all buckets owned by the account**; " +
			"when both an account-level and a bucket-level " +
			"([`radosgw_s3_bucket_public_access_block`](s3_bucket_public_access_block)) configuration are present, the " +
			"more restrictive of the two wins.\n\n" +
			"~> **Ceph version:** account-wide Public Access Block is **Ceph 21 (Umbrella) only** (added in 21.1.1; " +
			"not backported to Squid or Tentacle). When the provider's `ceph_version` targets an older release this " +
			"resource is rejected at plan time. Verified on Ceph 21.2.0: all four settings round-trip and are " +
			"enforced (e.g. `block_public_acls` rejects `CreateBucket`/`PutBucketAcl` with a public ACL across the " +
			"account's buckets).\n\n" +
			"~> **Credentials:** account-wide Public Access Block is managed by the account's **own root " +
			"credentials** — the RadosGW admin user cannot set it for an account (it returns `MethodNotAllowed`), which " +
			"matches AWS (an account manages its own Block Public Access). Configure the provider (typically an " +
			"[aliased provider](https://developer.hashicorp.com/terraform/language/providers/configuration#alias-multiple-provider-configurations)) " +
			"with the account root's `access_key`/`secret_key`, and set `account_id` to that same account.\n\n" +
			"~> **Addressing:** the AWS SDK normally addresses the S3 Control API at `<account-id>.<host>`, which needs " +
			"wildcard DNS. The provider instead sends the account id in the `x-amz-account-id` header (RadosGW ignores " +
			"the hostname), so no special DNS is required. The `s3control` API must be enabled on the gateway " +
			"(`rgw_enable_apis` includes `s3control`, which is the default).",

		Attributes: map[string]schema.Attribute{
			"account_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the RadosGW account the configuration applies to (for example " +
					"`RGW12345678901234567`).",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"block_public_acls": schema.BoolAttribute{
				MarkdownDescription: "Whether RadosGW should block public ACLs for buckets in this account. When " +
					"`true`, `CreateBucket`/`PutObject`/`PutBucketAcl` requests that specify a public ACL are rejected. " +
					"Defaults to `false`.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"block_public_policy": schema.BoolAttribute{
				MarkdownDescription: "Whether RadosGW should block public bucket policies for buckets in this " +
					"account. When `true`, `PutBucketPolicy` requests that grant public access are rejected. Defaults " +
					"to `false`.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"ignore_public_acls": schema.BoolAttribute{
				MarkdownDescription: "Whether RadosGW should ignore public ACLs for buckets in this account. When " +
					"`true`, any existing public ACLs on the account's buckets and objects are ignored (access is " +
					"treated as private). Defaults to `false`.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"restrict_public_buckets": schema.BoolAttribute{
				MarkdownDescription: "Whether RadosGW should restrict public bucket policies for buckets in this " +
					"account. When `true`, access to a bucket with a public policy is restricted to authorized users " +
					"within the account. Defaults to `false`.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"skip_destroy": schema.BoolAttribute{
				MarkdownDescription: "When `true`, the public-access-block configuration is left in place on the " +
					"account when this resource is destroyed (instead of being removed). Defaults to `false`.",
				Optional: true,
			},
		},
	}
}

func (r *S3AccountPublicAccessBlockResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ModifyPlan rejects the resource on clusters older than Umbrella (21.x), where
// the account-wide S3 Control PublicAccessBlock API does not exist.
func (r *S3AccountPublicAccessBlockResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() || r.client == nil || r.client.CephVersion.GreaterThanOrEqual(CephVersion_Umbrella) {
		return
	}
	resp.Diagnostics.AddError(
		"radosgw_s3_account_public_access_block requires Ceph 21 (Umbrella)",
		fmt.Sprintf("Account-wide Public Access Block (the S3 Control API) was added in Ceph 21.1.1 (Umbrella) and "+
			"was not backported to Squid or Tentacle, but the provider targets %s. Set the provider `ceph_version` "+
			"to `21`/`umbrella` on an Umbrella cluster to use this resource.", r.client.CephVersion),
	)
}

// =============================================================================
// CRUD
// =============================================================================

func (r *S3AccountPublicAccessBlockResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan S3AccountPublicAccessBlockModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if diags := r.putPublicAccessBlock(ctx, plan); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	tflog.Trace(ctx, "Created S3 account public access block", map[string]any{"account_id": plan.AccountID.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *S3AccountPublicAccessBlockResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state S3AccountPublicAccessBlockModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	accountID := state.AccountID.ValueString()

	output, err := r.client.S3Control.GetPublicAccessBlock(ctx, &s3control.GetPublicAccessBlockInput{
		AccountId: aws.String(accountID),
	})
	if err != nil {
		if isS3NoSuchPublicAccessBlock(err) {
			tflog.Info(ctx, "S3 account public access block not found, removing from state", map[string]any{"account_id": accountID})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading S3 Account Public Access Block",
			fmt.Sprintf("Could not read public access block for account %s: %s", accountID, err),
		)
		return
	}

	flattenAccountPublicAccessBlock(output.PublicAccessBlockConfiguration, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *S3AccountPublicAccessBlockResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan S3AccountPublicAccessBlockModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if diags := r.putPublicAccessBlock(ctx, plan); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	tflog.Debug(ctx, "Updated S3 account public access block", map[string]any{"account_id": plan.AccountID.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *S3AccountPublicAccessBlockResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state S3AccountPublicAccessBlockModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !state.SkipDestroy.IsNull() && state.SkipDestroy.ValueBool() {
		tflog.Trace(ctx, "skip_destroy set, leaving account public access block in place", map[string]any{"account_id": state.AccountID.ValueString()})
		return
	}

	accountID := state.AccountID.ValueString()

	_, err := r.client.S3Control.DeletePublicAccessBlock(ctx, &s3control.DeletePublicAccessBlockInput{
		AccountId: aws.String(accountID),
	})
	if err != nil {
		if isS3NoSuchPublicAccessBlock(err) {
			return
		}
		resp.Diagnostics.AddError(
			"Error Deleting S3 Account Public Access Block",
			fmt.Sprintf("Could not delete public access block for account %s: %s", accountID, err),
		)
		return
	}

	tflog.Trace(ctx, "Deleted S3 account public access block", map[string]any{"account_id": accountID})
}

func (r *S3AccountPublicAccessBlockResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("account_id"), req, resp)
}

// =============================================================================
// Helpers
// =============================================================================

// putPublicAccessBlock builds and applies the configuration for the model.
func (r *S3AccountPublicAccessBlockResource) putPublicAccessBlock(ctx context.Context, plan S3AccountPublicAccessBlockModel) diag.Diagnostics {
	var diags diag.Diagnostics

	_, err := r.client.S3Control.PutPublicAccessBlock(ctx, &s3control.PutPublicAccessBlockInput{
		AccountId: aws.String(plan.AccountID.ValueString()),
		PublicAccessBlockConfiguration: &s3controltypes.PublicAccessBlockConfiguration{
			BlockPublicAcls:       aws.Bool(plan.BlockPublicAcls.ValueBool()),
			BlockPublicPolicy:     aws.Bool(plan.BlockPublicPolicy.ValueBool()),
			IgnorePublicAcls:      aws.Bool(plan.IgnorePublicAcls.ValueBool()),
			RestrictPublicBuckets: aws.Bool(plan.RestrictPublicBuckets.ValueBool()),
		},
	})
	if err != nil {
		diags.AddError(
			"Error Applying S3 Account Public Access Block",
			fmt.Sprintf("Could not set public access block for account %s: %s", plan.AccountID.ValueString(), err),
		)
	}
	return diags
}

// flattenAccountPublicAccessBlock maps the API response into the model. A nil
// pointer maps to false (matching the schema default), so computed values are
// never left unknown.
func flattenAccountPublicAccessBlock(cfg *s3controltypes.PublicAccessBlockConfiguration, state *S3AccountPublicAccessBlockModel) {
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
