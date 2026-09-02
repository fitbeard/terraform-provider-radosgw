package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/ceph/go-ceph/rgw/admin"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &BucketResource{}
var _ resource.ResourceWithImportState = &BucketResource{}
var _ resource.ResourceWithModifyPlan = &BucketResource{}

func NewS3BucketResource() resource.Resource {
	return &BucketResource{}
}

// BucketResource defines the resource implementation.
type BucketResource struct {
	client *RadosgwClient
}

// BucketResourceModel describes the resource data model.
type BucketResourceModel struct {
	// User-configurable attributes
	Bucket            types.String `tfsdk:"bucket"`
	ForceDestroy      types.Bool   `tfsdk:"force_destroy"`
	ObjectLockEnabled types.Bool   `tfsdk:"object_lock_enabled"`
	Owner             types.String `tfsdk:"owner"`
	Tenant            types.String `tfsdk:"tenant"`
	Versioning        types.String `tfsdk:"versioning"`
	Acl               types.String `tfsdk:"acl"`
	BucketQuota       types.Object `tfsdk:"bucket_quota"`

	// Computed attributes from Admin API
	ID                types.String `tfsdk:"id"`
	CreationTime      types.String `tfsdk:"creation_time"`
	PlacementRule     types.String `tfsdk:"placement_rule"`
	Zonegroup         types.String `tfsdk:"zonegroup"`
	NumShards         types.Int64  `tfsdk:"num_shards"`
	Marker            types.String `tfsdk:"marker"`
	IndexType         types.String `tfsdk:"index_type"`
	ExplicitPlacement types.Object `tfsdk:"explicit_placement"`
}

// explicitPlacementAttrTypes returns the attribute types for explicit_placement.
func explicitPlacementAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"data_pool":       types.StringType,
		"data_extra_pool": types.StringType,
		"index_pool":      types.StringType,
	}
}

// bucketQuotaAttrTypes returns the attribute types for bucket_quota.
func bucketQuotaAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"enabled":     types.BoolType,
		"max_size":    types.Int64Type,
		"max_objects": types.Int64Type,
	}
}

func (r *BucketResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_s3_bucket"
}

func (r *BucketResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an S3 bucket in Ceph RadosGW. This resource creates buckets via the S3 API and enriches them with metadata from the Admin API when available.\n\n" +
			"~> **Users without admin capabilities** (e.g. authenticated via OpenStack Keystone federation, or an account root user): bucket create/read/update/delete work over the S3 API alone, and the S3-derivable attributes (`owner`, `creation_time`, `versioning`) are still populated. The Admin-API-only attributes — `id`, `marker`, `num_shards`, `index_type`, `placement_rule`, `zonegroup`, `explicit_placement`, and `bucket_quota` — are `null` because they require the `buckets` capability / admin-ops access. `force_destroy` also falls back to emptying the bucket over S3.\n\n" +
			"~> **Account users:** a non-root account user has no permissions by default and can only create buckets once the account root grants it an IAM policy (or role/group) allowing the S3 action; otherwise `CreateBucket` fails with `AccessDenied`. An account root user has full access to its account's resources without a policy.",

		Attributes: map[string]schema.Attribute{
			// User-configurable attributes
			"bucket": schema.StringAttribute{
				MarkdownDescription: "The name of the bucket. Must be unique within the RadosGW cluster. Bucket names must be between 3 and 63 characters, start with a lowercase letter or number, and contain only lowercase letters, numbers, and hyphens.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"force_destroy": schema.BoolAttribute{
				MarkdownDescription: "Whether to delete all objects in the bucket when destroying the resource. Uses the Admin API with purge-objects option. Default is false.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"object_lock_enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether S3 Object Lock is enabled for the bucket. Can only be set at creation time and cannot be modified afterwards.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"owner": schema.StringAttribute{
				MarkdownDescription: "The user ID of the bucket owner. This is a read-only attribute reflecting the current owner. The bucket is owned by the user whose credentials are used in the provider. To transfer ownership, use the `radosgw_s3_bucket_link` resource.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"tenant": schema.StringAttribute{
				MarkdownDescription: "The tenant the bucket belongs to. Can only be set at creation time. When set, the bucket is created with the tenant prefix.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"versioning": schema.StringAttribute{
				MarkdownDescription: "The versioning state of the bucket. Valid values: 'off', 'enabled', 'suspended'. Default is 'off'. " +
					"**Note:** versioning is one-way — once a bucket has been set to 'enabled', RadosGW (following the " +
					"S3 specification) only allows switching between 'enabled' and 'suspended'; it can never be turned " +
					"back 'off'. Use 'suspended' to stop creating new object versions.",
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("off"),
				Validators: []validator.String{
					stringvalidator.OneOf("off", "enabled", "suspended"),
				},
			},
			"acl": schema.StringAttribute{
				MarkdownDescription: "The canned ACL of the bucket. This is a read-only attribute. To manage bucket ACLs, use the `radosgw_s3_bucket_acl` resource.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"bucket_quota": schema.SingleNestedAttribute{
				MarkdownDescription: "Quota settings for this specific bucket. Managed via the Admin API.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.UseStateForUnknown(),
				},
				Attributes: map[string]schema.Attribute{
					"enabled": schema.BoolAttribute{
						MarkdownDescription: "Whether the bucket quota is enabled.",
						Optional:            true,
						Computed:            true,
					},
					"max_size": schema.Int64Attribute{
						MarkdownDescription: "Maximum size in bytes. -1 means unlimited.",
						Optional:            true,
						Computed:            true,
					},
					"max_objects": schema.Int64Attribute{
						MarkdownDescription: "Maximum number of objects. -1 means unlimited.",
						Optional:            true,
						Computed:            true,
					},
				},
			},

			// Computed attributes from Admin API
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the bucket assigned by RadosGW.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"placement_rule": schema.StringAttribute{
				MarkdownDescription: "The placement rule for the bucket, determining which pools store the bucket's data.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"creation_time": schema.StringAttribute{
				MarkdownDescription: "The creation time of the bucket in RFC3339 format.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"zonegroup": schema.StringAttribute{
				MarkdownDescription: "The zonegroup ID where the bucket is located.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"num_shards": schema.Int64Attribute{
				MarkdownDescription: "The number of shards for the bucket index.",
				Computed:            true,
			},
			"marker": schema.StringAttribute{
				MarkdownDescription: "The internal bucket marker used by RadosGW.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"index_type": schema.StringAttribute{
				MarkdownDescription: "The type of bucket index (e.g., 'Normal').",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"explicit_placement": schema.SingleNestedAttribute{
				MarkdownDescription: "Explicit placement configuration showing the RADOS pools used for the bucket.",
				Computed:            true,
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.UseStateForUnknown(),
				},
				Attributes: map[string]schema.Attribute{
					"data_pool": schema.StringAttribute{
						MarkdownDescription: "The RADOS pool for storing object data.",
						Computed:            true,
					},
					"data_extra_pool": schema.StringAttribute{
						MarkdownDescription: "The RADOS pool for storing extra object data (e.g., multipart uploads).",
						Computed:            true,
					},
					"index_pool": schema.StringAttribute{
						MarkdownDescription: "The RADOS pool for storing the bucket index.",
						Computed:            true,
					},
				},
			},
		},
	}
}

// ModifyPlan validates versioning transitions at plan time. Versioning is
// one-way in S3/RadosGW: once a bucket is "enabled" it can only move between
// "enabled" and "suspended", never back to "off". Without this check the plan
// would show enabled -> off, the apply would silently no-op the change, and the
// mandatory re-read would then contradict the plan — surfacing as the confusing
// generic "Provider produced inconsistent result after apply … this is a bug in
// the provider" error. Catching it here fails the plan with an actionable message
// before any apply.
func (r *BucketResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	// Only relevant for updates: skip create (no prior state) and destroy (no plan).
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}

	var stateVersioning, planVersioning types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("versioning"), &stateVersioning)...)
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("versioning"), &planVersioning)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if planVersioning.ValueString() == "off" &&
		(stateVersioning.ValueString() == "enabled" || stateVersioning.ValueString() == "suspended") {
		resp.Diagnostics.AddAttributeError(
			path.Root("versioning"),
			"Cannot Disable Bucket Versioning",
			fmt.Sprintf("Versioning is currently %q and cannot be turned off. RadosGW (following the S3 "+
				"specification) only allows switching between \"enabled\" and \"suspended\" once versioning has been "+
				"enabled. To stop creating new object versions, set versioning = \"suspended\".",
				stateVersioning.ValueString()),
		)
	}
}

func (r *BucketResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *BucketResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data BucketResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucketName := data.Bucket.ValueString()
	tenant := data.Tenant.ValueString()

	// Build full bucket name with tenant if specified
	fullBucketName := bucketName
	if tenant != "" {
		fullBucketName = tenant + ":" + bucketName
	}

	tflog.Debug(ctx, "Creating bucket", map[string]any{
		"bucket": fullBucketName,
		"tenant": tenant,
	})

	// Create bucket using S3 API
	createInput := &s3.CreateBucketInput{
		Bucket:                     &fullBucketName,
		ObjectLockEnabledForBucket: data.ObjectLockEnabled.ValueBoolPointer(),
	}

	_, err := r.client.S3.CreateBucket(ctx, createInput)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Bucket",
			fmt.Sprintf("Could not create bucket %s: %s", fullBucketName, err.Error()),
		)
		return
	}

	tflog.Trace(ctx, "Created bucket", map[string]any{
		"bucket": fullBucketName,
	})

	// Set versioning if specified (only for enabled or suspended, not for off)
	versioning := data.Versioning.ValueString()
	if versioning == "enabled" || versioning == "suspended" {
		err = r.setBucketVersioning(ctx, fullBucketName, versioning)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Setting Bucket Versioning",
				fmt.Sprintf("Could not set versioning on bucket %s: %s", fullBucketName, err.Error()),
			)
			return
		}
	}

	// Set bucket quota if specified
	if !data.BucketQuota.IsNull() && !data.BucketQuota.IsUnknown() {
		err = r.setBucketQuota(ctx, bucketName, data.BucketQuota)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Setting Bucket Quota",
				fmt.Sprintf("Could not set quota on bucket %s: %s", bucketName, err.Error()),
			)
			return
		}
	}

	// Read bucket info from Admin API to populate computed fields.
	bucketInfo, err := r.client.Admin.GetBucketInfo(ctx, admin.Bucket{Bucket: bucketName})
	if err != nil {
		// The Admin API is unavailable (e.g. a Keystone-federated user without
		// caps) or the bucket lives in a tenant namespace the plain name can't
		// address. Set all admin-only computed fields to null so state is valid,
		// and populate the S3-derivable fields (owner/creation_time/versioning)
		// via the S3 API, which works with the configured user's own credentials.
		tflog.Warn(ctx, "Admin API unavailable after bucket creation; falling back to S3-only metadata", map[string]any{
			"bucket": bucketName,
			"error":  err.Error(),
		})
		data.ID = types.StringValue(bucketName)
		data.Marker = types.StringNull()
		data.NumShards = types.Int64Null()
		data.IndexType = types.StringNull()
		data.PlacementRule = types.StringNull()
		data.Zonegroup = types.StringNull()
		data.ExplicitPlacement = types.ObjectNull(explicitPlacementAttrTypes())
		data.Acl = types.StringNull()
		if data.Tenant.IsUnknown() {
			data.Tenant = types.StringValue(tenant)
		}
		if data.BucketQuota.IsNull() || data.BucketQuota.IsUnknown() {
			data.BucketQuota = types.ObjectNull(bucketQuotaAttrTypes())
		}
		r.populateBucketFromS3(ctx, &data, fullBucketName)
	} else {
		r.populateModelFromBucketInfo(ctx, &data, &bucketInfo)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *BucketResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data BucketResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucketName := data.Bucket.ValueString()
	tenant := data.Tenant.ValueString()
	fullBucketName := bucketName
	if tenant != "" {
		fullBucketName = tenant + ":" + bucketName
	}

	// Preserve user-configured values that aren't returned by the Admin API.
	forceDestroy := data.ForceDestroy

	tflog.Debug(ctx, "Reading bucket", map[string]any{
		"bucket": bucketName,
	})

	// Prefer the Admin API (richest metadata). Backward compatible: users with
	// the buckets cap get the full attribute set exactly as before.
	bucketInfo, err := r.client.Admin.GetBucketInfo(ctx, admin.Bucket{Bucket: bucketName})
	if err == nil {
		r.populateModelFromBucketInfo(ctx, &data, &bucketInfo)
		data.ForceDestroy = forceDestroy
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	// Admin API unavailable — use the S3 API to determine existence (works with
	// the user's own credentials and resolves their tenant namespace).
	exists, headErr := bucketExistsViaS3(ctx, r.client.S3, fullBucketName)
	if headErr != nil {
		// Could not confirm via S3 either: trust an Admin "not found", otherwise
		// surface the original error.
		if isBucketNotFoundError(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading Bucket",
			fmt.Sprintf("Could not read bucket %s: %s", bucketName, err.Error()),
		)
		return
	}
	if !exists {
		tflog.Debug(ctx, "Bucket not found, removing from state", map[string]any{"bucket": bucketName})
		resp.State.RemoveResource(ctx)
		return
	}

	// Bucket exists but Admin metadata is unavailable. Admin-only fields are kept
	// as-is from prior state (already loaded into data); refresh the S3-derivable
	// fields to detect drift.
	tflog.Debug(ctx, "Admin API unavailable; refreshing bucket via S3-only metadata", map[string]any{"bucket": bucketName})
	r.populateBucketFromS3(ctx, &data, fullBucketName)
	data.ForceDestroy = forceDestroy

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *BucketResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data BucketResourceModel
	var state BucketResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucketName := data.Bucket.ValueString()
	tenant := data.Tenant.ValueString()

	fullBucketName := bucketName
	if tenant != "" {
		fullBucketName = tenant + ":" + bucketName
	}

	tflog.Debug(ctx, "Updating bucket", map[string]any{
		"bucket": bucketName,
	})

	// Handle versioning change
	if !data.Versioning.Equal(state.Versioning) {
		versioning := data.Versioning.ValueString()
		if versioning == "enabled" || versioning == "suspended" {
			err := r.setBucketVersioning(ctx, fullBucketName, versioning)
			if err != nil {
				resp.Diagnostics.AddError(
					"Error Setting Bucket Versioning",
					fmt.Sprintf("Could not set versioning on bucket %s: %s", bucketName, err.Error()),
				)
				return
			}
		}
	}

	// Handle quota change
	if !data.BucketQuota.Equal(state.BucketQuota) && !data.BucketQuota.IsNull() {
		err := r.setBucketQuota(ctx, bucketName, data.BucketQuota)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Setting Bucket Quota",
				fmt.Sprintf("Could not set quota on bucket %s: %s", bucketName, err.Error()),
			)
			return
		}
	}

	// Re-read bucket info to get fresh computed values.
	bucketInfo, err := r.client.Admin.GetBucketInfo(ctx, admin.Bucket{Bucket: bucketName})
	if err != nil {
		tflog.Warn(ctx, "Admin API unavailable during update; keeping admin-only fields and refreshing via S3", map[string]any{
			"bucket": bucketName,
			"error":  err.Error(),
		})
		// Admin-only fields cannot be refreshed without caps; preserve their prior
		// state values to avoid drift.
		data.ID = state.ID
		data.Zonegroup = state.Zonegroup
		data.NumShards = state.NumShards
		data.Marker = state.Marker
		data.IndexType = state.IndexType
		data.PlacementRule = state.PlacementRule
		data.ExplicitPlacement = state.ExplicitPlacement
		data.Acl = types.StringNull()
		if data.Tenant.IsUnknown() {
			data.Tenant = state.Tenant
		}
		if data.BucketQuota.IsUnknown() {
			data.BucketQuota = state.BucketQuota
		}
		// Refresh S3-derivable fields (owner/creation_time/versioning).
		r.populateBucketFromS3(ctx, &data, fullBucketName)
	} else {
		r.populateModelFromBucketInfo(ctx, &data, &bucketInfo)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *BucketResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data BucketResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucketName := data.Bucket.ValueString()
	tenant := data.Tenant.ValueString()
	fullBucketName := bucketName
	if tenant != "" {
		fullBucketName = tenant + ":" + bucketName
	}
	forceDestroy := data.ForceDestroy.ValueBool()

	tflog.Debug(ctx, "Deleting bucket", map[string]any{
		"bucket":        bucketName,
		"force_destroy": forceDestroy,
	})

	if forceDestroy {
		// Prefer the Admin API (purge-objects) — backward compatible for users
		// with the buckets cap.
		purge := true
		err := r.client.Admin.RemoveBucket(ctx, admin.Bucket{
			Bucket:      bucketName,
			PurgeObject: &purge,
		})
		if err == nil {
			// deleted via admin
		} else if isBucketNotFoundError(err) {
			tflog.Debug(ctx, "Bucket already deleted", map[string]any{"bucket": bucketName})
			return
		} else {
			// Admin API unavailable (e.g. a Keystone-federated user without caps).
			// Fall back to emptying and deleting the bucket over S3.
			tflog.Warn(ctx, "Admin RemoveBucket failed; falling back to S3 force-destroy", map[string]any{
				"bucket": bucketName,
				"error":  err.Error(),
			})
			if s3Err := r.emptyAndDeleteBucketViaS3(ctx, fullBucketName); s3Err != nil {
				if isS3BucketNotFound(s3Err) {
					return
				}
				resp.Diagnostics.AddError(
					"Error Deleting Bucket",
					fmt.Sprintf("Could not delete bucket %s with force_destroy: admin API error: %s; S3 fallback error: %s", bucketName, err.Error(), s3Err.Error()),
				)
				return
			}
		}
	} else {
		// Use S3 API for standard deletion (bucket must be empty)
		_, err := r.client.S3.DeleteBucket(ctx, &s3.DeleteBucketInput{
			Bucket: &fullBucketName,
		})
		if err != nil {
			var ae smithy.APIError
			if errors.As(err, &ae) {
				if ae.ErrorCode() == "NoSuchBucket" || ae.ErrorCode() == "404" {
					tflog.Debug(ctx, "Bucket already deleted", map[string]any{
						"bucket": bucketName,
					})
					return
				}
				if ae.ErrorCode() == "BucketNotEmpty" {
					resp.Diagnostics.AddError(
						"Bucket Not Empty",
						fmt.Sprintf("Bucket %s is not empty. Set force_destroy = true to delete the bucket and all its contents.", bucketName),
					)
					return
				}
			}
			resp.Diagnostics.AddError(
				"Error Deleting Bucket",
				fmt.Sprintf("Could not delete bucket %s: %s", bucketName, err.Error()),
			)
			return
		}
	}

	tflog.Trace(ctx, "Deleted bucket", map[string]any{
		"bucket": bucketName,
	})
}

func (r *BucketResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	bucketName := req.ID

	tflog.Debug(ctx, "Importing bucket", map[string]any{
		"bucket": bucketName,
	})

	// Verify bucket exists using Admin API
	bucketInfo, err := r.client.Admin.GetBucketInfo(ctx, admin.Bucket{Bucket: bucketName})
	if err != nil {
		if isBucketNotFoundError(err) {
			resp.Diagnostics.AddError(
				"Bucket Not Found",
				fmt.Sprintf("Bucket %s does not exist.", bucketName),
			)
			return
		}
		resp.Diagnostics.AddError(
			"Error Importing Bucket",
			fmt.Sprintf("Could not import bucket %s: %s", bucketName, err.Error()),
		)
		return
	}

	// Set attributes for import
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("bucket"), bucketName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("force_destroy"), false)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("object_lock_enabled"), bucketInfo.ObjectLockEnabled)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("tenant"), bucketInfo.Tenant)...)
}

// setBucketVersioning sets the versioning state on a bucket.
func (r *BucketResource) setBucketVersioning(ctx context.Context, bucketName, versioning string) error {
	var status s3types.BucketVersioningStatus
	switch versioning {
	case "enabled":
		status = s3types.BucketVersioningStatusEnabled
	case "suspended":
		status = s3types.BucketVersioningStatusSuspended
	default:
		return nil
	}

	_, err := r.client.S3.PutBucketVersioning(ctx, &s3.PutBucketVersioningInput{
		Bucket: &bucketName,
		VersioningConfiguration: &s3types.VersioningConfiguration{
			Status: status,
		},
	})
	return err
}

// setBucketQuota sets the quota on a bucket via Admin API.
func (r *BucketResource) setBucketQuota(ctx context.Context, bucketName string, quotaObj types.Object) error {
	if quotaObj.IsNull() || quotaObj.IsUnknown() {
		return nil
	}

	var quota BucketQuotaModel
	diags := quotaObj.As(ctx, &quota, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return fmt.Errorf("could not parse bucket quota")
	}

	// Get bucket info to find owner
	bucketInfo, err := r.client.Admin.GetBucketInfo(ctx, admin.Bucket{Bucket: bucketName})
	if err != nil {
		return fmt.Errorf("could not get bucket info: %w", err)
	}

	quotaSpec := admin.QuotaSpec{
		UID:    bucketInfo.Owner,
		Bucket: bucketName,
	}

	if !quota.Enabled.IsNull() && !quota.Enabled.IsUnknown() {
		enabled := quota.Enabled.ValueBool()
		quotaSpec.Enabled = &enabled
	}
	if !quota.MaxSize.IsNull() && !quota.MaxSize.IsUnknown() {
		maxSize := quota.MaxSize.ValueInt64()
		quotaSpec.MaxSize = &maxSize
	}
	if !quota.MaxObjects.IsNull() && !quota.MaxObjects.IsUnknown() {
		maxObjects := quota.MaxObjects.ValueInt64()
		quotaSpec.MaxObjects = &maxObjects
	}

	return r.client.Admin.SetIndividualBucketQuota(ctx, quotaSpec)
}

// BucketQuotaModel represents bucket quota settings.
type BucketQuotaModel struct {
	Enabled    types.Bool  `tfsdk:"enabled"`
	MaxSize    types.Int64 `tfsdk:"max_size"`
	MaxObjects types.Int64 `tfsdk:"max_objects"`
}

// populateModelFromBucketInfo updates the model with data from Admin API bucket info.
func (r *BucketResource) populateModelFromBucketInfo(ctx context.Context, data *BucketResourceModel, info *admin.Bucket) {
	data.ID = types.StringValue(info.ID)
	data.Owner = types.StringValue(info.Owner)
	// ACL is managed by radosgw_s3_bucket_acl resource, set to null
	data.Acl = types.StringNull()
	data.Tenant = types.StringValue(info.Tenant)
	data.PlacementRule = types.StringValue(info.PlacementRule)
	data.Zonegroup = types.StringValue(info.Zonegroup)
	data.Marker = types.StringValue(info.Marker)
	data.IndexType = types.StringValue(info.IndexType)
	data.ObjectLockEnabled = types.BoolValue(info.ObjectLockEnabled)

	// Handle versioning - map API response to our schema values
	if info.Versioning != nil {
		switch *info.Versioning {
		case "Enabled", "enabled":
			data.Versioning = types.StringValue("enabled")
		case "Suspended", "suspended":
			data.Versioning = types.StringValue("suspended")
		default:
			data.Versioning = types.StringValue("off")
		}
	} else {
		data.Versioning = types.StringValue("off")
	}

	// Handle num_shards
	if info.NumShards != nil {
		data.NumShards = types.Int64Value(int64(*info.NumShards))
	} else {
		data.NumShards = types.Int64Null()
	}

	// Handle creation time
	if info.CreationTime != nil {
		data.CreationTime = types.StringValue(info.CreationTime.Format("2006-01-02T15:04:05Z07:00"))
	} else {
		data.CreationTime = types.StringNull()
	}

	// Build explicit_placement object
	placementObj, diags := types.ObjectValue(explicitPlacementAttrTypes(), map[string]attr.Value{
		"data_pool":       types.StringValue(info.ExplicitPlacement.DataPool),
		"data_extra_pool": types.StringValue(info.ExplicitPlacement.DataExtraPool),
		"index_pool":      types.StringValue(info.ExplicitPlacement.IndexPool),
	})
	if diags.HasError() {
		tflog.Warn(ctx, "Could not build explicit_placement object")
		data.ExplicitPlacement = types.ObjectNull(explicitPlacementAttrTypes())
	} else {
		data.ExplicitPlacement = placementObj
	}

	// Build bucket_quota object
	quotaValues := map[string]attr.Value{
		"enabled":     types.BoolNull(),
		"max_size":    types.Int64Null(),
		"max_objects": types.Int64Null(),
	}
	if info.BucketQuota.Enabled != nil {
		quotaValues["enabled"] = types.BoolValue(*info.BucketQuota.Enabled)
	}
	if info.BucketQuota.MaxSize != nil {
		quotaValues["max_size"] = types.Int64Value(*info.BucketQuota.MaxSize)
	}
	if info.BucketQuota.MaxObjects != nil {
		quotaValues["max_objects"] = types.Int64Value(*info.BucketQuota.MaxObjects)
	}

	quotaObj, diags := types.ObjectValue(bucketQuotaAttrTypes(), quotaValues)
	if diags.HasError() {
		tflog.Warn(ctx, "Could not build bucket_quota object")
		data.BucketQuota = types.ObjectNull(bucketQuotaAttrTypes())
	} else {
		data.BucketQuota = quotaObj
	}
}

// isBucketNotFoundError checks if an error indicates the bucket doesn't exist.
func isBucketNotFoundError(err error) bool {
	return errors.Is(err, admin.ErrNoSuchBucket)
}

// bucketExistsViaS3 reports whether the bucket exists using the S3 API, which
// works with the configured user's own credentials (no admin caps required) and
// resolves the user's tenant namespace. Used as the source of truth for
// existence when the Admin API is unavailable.
func bucketExistsViaS3(ctx context.Context, s3c *s3.Client, bucket string) (bool, error) {
	_, err := s3c.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: &bucket})
	if err == nil {
		return true, nil
	}
	var notFound *s3types.NotFound
	if errors.As(err, &notFound) {
		return false, nil
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NotFound", "NoSuchBucket", "404":
			return false, nil
		}
	}
	return false, err
}

// isS3BucketNotFound reports whether an S3 error means the bucket does not exist.
func isS3BucketNotFound(err error) bool {
	var notFound *s3types.NotFound
	if errors.As(err, &notFound) {
		return true
	}
	var noBucket *s3types.NoSuchBucket
	if errors.As(err, &noBucket) {
		return true
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NotFound", "NoSuchBucket", "404":
			return true
		}
	}
	return false
}

// emptyAndDeleteBucketViaS3 removes all objects (including versions and delete
// markers) and then the bucket, using only the S3 API. This is the force_destroy
// path for users without admin caps (the Admin RemoveBucket purge is unavailable).
func (r *BucketResource) emptyAndDeleteBucketViaS3(ctx context.Context, bucket string) error {
	// ListObjectVersions returns current objects for unversioned buckets too, so
	// this single pass covers both versioned and unversioned buckets.
	pager := s3.NewListObjectVersionsPaginator(r.client.S3, &s3.ListObjectVersionsInput{Bucket: &bucket})
	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return err
		}
		ids := make([]s3types.ObjectIdentifier, 0, len(page.Versions)+len(page.DeleteMarkers))
		for _, v := range page.Versions {
			ids = append(ids, s3types.ObjectIdentifier{Key: v.Key, VersionId: v.VersionId})
		}
		for _, m := range page.DeleteMarkers {
			ids = append(ids, s3types.ObjectIdentifier{Key: m.Key, VersionId: m.VersionId})
		}
		if len(ids) > 0 {
			if _, err := r.client.S3.DeleteObjects(ctx, &s3.DeleteObjectsInput{
				Bucket: &bucket,
				Delete: &s3types.Delete{Objects: ids},
			}); err != nil {
				return err
			}
		}
	}
	_, err := r.client.S3.DeleteBucket(ctx, &s3.DeleteBucketInput{Bucket: &bucket})
	return err
}

// deriveBucketS3Fields fetches the S3-derivable bucket attributes (owner,
// creation_time, versioning) via the S3 API, which works with the configured
// user's own credentials — no admin caps required. Each returned value is null
// if the S3 API could not provide it. fullName is the tenant-qualified name for
// GetBucketAcl/GetBucketVersioning; ListBuckets returns plain names, so plainName
// is matched there. Shared by the resource and the data source.
func deriveBucketS3Fields(ctx context.Context, s3c *s3.Client, plainName, fullName string) (owner, creationTime, versioning types.String) {
	owner, creationTime, versioning = types.StringNull(), types.StringNull(), types.StringNull()

	if acl, err := s3c.GetBucketAcl(ctx, &s3.GetBucketAclInput{Bucket: &fullName}); err == nil &&
		acl.Owner != nil && acl.Owner.ID != nil {
		owner = types.StringValue(*acl.Owner.ID)
	}

	if ver, err := s3c.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{Bucket: &fullName}); err == nil {
		switch ver.Status {
		case s3types.BucketVersioningStatusEnabled:
			versioning = types.StringValue("enabled")
		case s3types.BucketVersioningStatusSuspended:
			versioning = types.StringValue("suspended")
		default:
			versioning = types.StringValue("off")
		}
	}

	if out, err := s3c.ListBuckets(ctx, &s3.ListBucketsInput{}); err == nil {
		for _, b := range out.Buckets {
			if b.Name != nil && *b.Name == plainName && b.CreationDate != nil {
				creationTime = types.StringValue(b.CreationDate.Format("2006-01-02T15:04:05Z07:00"))
				break
			}
		}
	}
	return owner, creationTime, versioning
}

// populateBucketFromS3 fills the S3-derivable computed attributes on the resource
// model when the Admin API is unavailable. Best-effort: a field is only set to a
// safe known value so state never contains an unknown.
func (r *BucketResource) populateBucketFromS3(ctx context.Context, data *BucketResourceModel, fullBucketName string) {
	owner, creationTime, versioning := deriveBucketS3Fields(ctx, r.client.S3, data.Bucket.ValueString(), fullBucketName)

	if !owner.IsNull() {
		data.Owner = owner
	} else if data.Owner.IsUnknown() {
		data.Owner = types.StringNull()
	}

	if !versioning.IsNull() {
		data.Versioning = versioning
	} else if data.Versioning.IsUnknown() {
		data.Versioning = types.StringValue("off")
	}

	if !creationTime.IsNull() {
		data.CreationTime = creationTime
	} else if data.CreationTime.IsUnknown() {
		data.CreationTime = types.StringNull()
	}
}
