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
		MarkdownDescription: "Manages an S3 bucket in Ceph RadosGW. This resource creates buckets via the S3 API and manages bucket configuration through both S3 and Admin APIs.",

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
				MarkdownDescription: "The versioning state of the bucket. Valid values: 'off', 'enabled', 'suspended'. Default is 'off'.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("off"),
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

	// Read bucket info from Admin API to populate computed fields
	bucketInfo, err := r.client.Admin.GetBucketInfo(ctx, admin.Bucket{Bucket: bucketName})
	if err != nil {
		tflog.Warn(ctx, "Could not get bucket info after creation", map[string]any{
			"bucket": bucketName,
			"error":  err.Error(),
		})
		data.ID = types.StringValue(bucketName)
		data.ExplicitPlacement = types.ObjectNull(explicitPlacementAttrTypes())
		data.Acl = types.StringNull()
		data.Owner = types.StringNull()
		if data.BucketQuota.IsNull() || data.BucketQuota.IsUnknown() {
			data.BucketQuota = types.ObjectNull(bucketQuotaAttrTypes())
		}
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

	tflog.Debug(ctx, "Reading bucket", map[string]any{
		"bucket": bucketName,
	})

	// Get bucket info from Admin API
	bucketInfo, err := r.client.Admin.GetBucketInfo(ctx, admin.Bucket{Bucket: bucketName})
	if err != nil {
		if isBucketNotFoundError(err) {
			tflog.Debug(ctx, "Bucket not found, removing from state", map[string]any{
				"bucket": bucketName,
			})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading Bucket",
			fmt.Sprintf("Could not read bucket %s: %s", bucketName, err.Error()),
		)
		return
	}

	// Preserve user-configured values that aren't returned by Admin API
	forceDestroy := data.ForceDestroy

	r.populateModelFromBucketInfo(ctx, &data, &bucketInfo)

	// Restore force_destroy from state (not returned by Admin API)
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

	// Re-read bucket info to get fresh computed values
	bucketInfo, err := r.client.Admin.GetBucketInfo(ctx, admin.Bucket{Bucket: bucketName})
	if err != nil {
		tflog.Warn(ctx, "Could not refresh bucket info during update", map[string]any{
			"bucket": bucketName,
			"error":  err.Error(),
		})
		// Keep most state values but update user-configurable ones
		data.ID = state.ID
		data.CreationTime = state.CreationTime
		data.Zonegroup = state.Zonegroup
		data.NumShards = state.NumShards
		data.Marker = state.Marker
		data.IndexType = state.IndexType
		data.ExplicitPlacement = state.ExplicitPlacement
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
	forceDestroy := data.ForceDestroy.ValueBool()

	tflog.Debug(ctx, "Deleting bucket", map[string]any{
		"bucket":        bucketName,
		"force_destroy": forceDestroy,
	})

	if forceDestroy {
		// Use Admin API to remove bucket with purge-objects option
		purge := true
		err := r.client.Admin.RemoveBucket(ctx, admin.Bucket{
			Bucket:      bucketName,
			PurgeObject: &purge,
		})
		if err != nil {
			if isBucketNotFoundError(err) {
				tflog.Debug(ctx, "Bucket already deleted", map[string]any{
					"bucket": bucketName,
				})
				return
			}
			resp.Diagnostics.AddError(
				"Error Deleting Bucket",
				fmt.Sprintf("Could not delete bucket %s with force_destroy: %s", bucketName, err.Error()),
			)
			return
		}
	} else {
		// Use S3 API for standard deletion (bucket must be empty)
		_, err := r.client.S3.DeleteBucket(ctx, &s3.DeleteBucketInput{
			Bucket: &bucketName,
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
