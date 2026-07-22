package provider

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/ceph/go-ceph/rgw/admin"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &BucketLinkResource{}
var _ resource.ResourceWithImportState = &BucketLinkResource{}

func NewS3BucketLinkResource() resource.Resource {
	return &BucketLinkResource{}
}

// bucketTenant returns the tenant component of a resolved (possibly
// tenant-qualified "tenant$user") user ID, or "" for a non-tenant user.
func bucketTenant(resolvedUID string) string {
	if tenant, _, ok := splitTenantQualifiedUserID(resolvedUID); ok {
		return tenant
	}
	return ""
}

// linkSourceBucket qualifies a bucket name as the source of a link to a tenant
// user. RadosGW scopes buckets by tenant, so moving a default-namespace bucket to
// a tenant user requires addressing it with a leading "/" (empty-tenant)
// qualifier; otherwise RadosGW looks for the bucket inside the tenant namespace
// and returns NoSuchKey. A name that already carries a tenant ("/") is unchanged.
func linkSourceBucket(tenant, bucket string) string {
	if tenant == "" || strings.Contains(bucket, "/") {
		return bucket
	}
	return "/" + bucket
}

// tenantScopedBucket qualifies a bucket name for reads and unlinks against a
// tenant user's namespace ("tenant/bucket"). A name that already carries a
// tenant ("/") is returned unchanged.
func tenantScopedBucket(tenant, bucket string) string {
	if tenant == "" || strings.Contains(bucket, "/") {
		return bucket
	}
	return tenant + "/" + bucket
}

// BucketLinkResource defines the resource implementation.
type BucketLinkResource struct {
	client *RadosgwClient
}

// BucketLinkResourceModel describes the resource data model.
type BucketLinkResourceModel struct {
	Bucket        types.String `tfsdk:"bucket"`
	UID           types.String `tfsdk:"uid"`
	BucketID      types.String `tfsdk:"bucket_id"`
	NewBucketName types.String `tfsdk:"new_bucket_name"`
	UnlinkToUID   types.String `tfsdk:"unlink_to_uid"`
}

func (r *BucketLinkResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_s3_bucket_link"
}

func (r *BucketLinkResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Manages bucket ownership in RadosGW by linking a bucket to a specified user.

This resource links an existing bucket to a user, unlinking it from any previous owner. It is primarily useful for:
- Transferring bucket ownership between users
- Moving buckets from one tenant to another
- Renaming buckets during the link operation

On destruction, the bucket can optionally be linked to a different user (via ` + "`unlink_to_uid`" + `), or simply unlinked from the current user.

~> **Note:** The bucket must already exist. This resource does not create buckets, only manages ownership. For **same-namespace** links (the target user is in the same namespace as the bucket — i.e. no tenant, or the same tenant), the ` + "`owner`" + ` attribute on ` + "`radosgw_s3_bucket`" + ` is read-only, so the two resources can be used together without conflict. This is **not** true when linking to a *tenant* user — see the tenant note below.

~> **Important:** When transferring bucket ownership, the ` + "`radosgw_s3_bucket_acl`" + ` and ` + "`radosgw_s3_bucket_policy`" + ` resources can only be managed by the bucket owner. If you transfer ownership to a different user, you will need separate provider credentials (aliases) to manage those resources.

~> **Tenant users:** RadosGW scopes buckets by tenant. Linking a bucket to a tenant user (a ` + "`uid`" + ` of the form ` + "`tenant$user`" + `, e.g. when OpenStack Keystone ` + "`rgw_keystone_implicit_tenants`" + ` is enabled) **moves the bucket into that tenant's namespace** (it becomes ` + "`tenant/bucket`" + `). The provider handles the addressing automatically — you still reference the plain bucket name here. However, because the bucket physically moves namespaces, do **not** also manage that same bucket's lifecycle with ` + "`radosgw_s3_bucket`" + ` by its plain name (it will report drift and try to recreate it). Create such buckets out-of-band (or in the tenant) and manage only their ownership with this resource.`,

		Attributes: map[string]schema.Attribute{
			"bucket": schema.StringAttribute{
				MarkdownDescription: "The name of the bucket to link. The bucket must already exist.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"uid": schema.StringAttribute{
				MarkdownDescription: "The user ID to link the bucket to. This user will become the bucket owner.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"bucket_id": schema.StringAttribute{
				MarkdownDescription: "The unique bucket ID assigned by RadosGW.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"new_bucket_name": schema.StringAttribute{
				MarkdownDescription: "Optional new name for the bucket. Use this to rename the bucket during the link operation.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"unlink_to_uid": schema.StringAttribute{
				MarkdownDescription: "The user ID to link the bucket to when this resource is destroyed. If not set, the bucket will be unlinked from the user but remain in the system.",
				Optional:            true,
			},
		},
	}
}

func (r *BucketLinkResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *BucketLinkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data BucketLinkResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	uid := data.UID.ValueString()
	resolvedUID, err := resolveUserID(ctx, r.client, uid)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Resolving User",
			fmt.Sprintf("Could not resolve user %s for bucket link: %s", uid, err.Error()),
		)
		return
	}

	// RadosGW scopes buckets by tenant. When linking to a tenant user, the
	// source bucket (in the default namespace) must be addressed as "/bucket";
	// otherwise RadosGW searches the tenant namespace and returns NoSuchKey.
	tenant := bucketTenant(resolvedUID)
	bucketLink := admin.BucketLinkInput{
		Bucket: linkSourceBucket(tenant, data.Bucket.ValueString()),
		UID:    resolvedUID,
	}

	if !data.NewBucketName.IsNull() && data.NewBucketName.ValueString() != "" {
		bucketLink.NewBucketName = data.NewBucketName.ValueString()
	}

	tflog.Debug(ctx, "Linking bucket to user", map[string]any{
		"bucket":          data.Bucket.ValueString(),
		"uid":             uid,
		"resolved_uid":    resolvedUID,
		"tenant":          tenant,
		"link_source":     bucketLink.Bucket,
		"new_bucket_name": data.NewBucketName.ValueString(),
	})

	// Link bucket with retry logic for ConcurrentModification
	err = retryOnConcurrentModification(ctx, fmt.Sprintf("LinkBucket %s to %s", data.Bucket.ValueString(), resolvedUID), func() error {
		return r.client.Admin.LinkBucket(ctx, bucketLink)
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Linking Bucket",
			fmt.Sprintf("Could not link bucket %s to user %s: %s", data.Bucket.ValueString(), uid, err.Error()),
		)
		return
	}

	// Get bucket info to retrieve the bucket ID. After a link to a tenant user
	// the bucket lives in the tenant namespace, so address it as "tenant/bucket".
	effectiveBucketName := data.Bucket.ValueString()
	if !data.NewBucketName.IsNull() && data.NewBucketName.ValueString() != "" {
		effectiveBucketName = data.NewBucketName.ValueString()
	}

	bucketInfo, err := r.client.Admin.GetBucketInfo(ctx, admin.Bucket{Bucket: tenantScopedBucket(tenant, effectiveBucketName)})
	if err != nil {
		tflog.Warn(ctx, "Could not retrieve bucket info after link", map[string]any{
			"bucket": effectiveBucketName,
			"error":  err.Error(),
		})
		data.BucketID = types.StringValue("")
	} else {
		data.BucketID = types.StringValue(bucketInfo.ID)
	}

	tflog.Trace(ctx, "Linked bucket to user")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *BucketLinkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data BucketLinkResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading bucket link", map[string]any{
		"bucket": data.Bucket.ValueString(),
		"uid":    data.UID.ValueString(),
	})

	uid := data.UID.ValueString()
	resolvedUID, err := resolveUserID(ctx, r.client, uid)
	if err != nil {
		if errors.Is(err, admin.ErrNoSuchUser) {
			tflog.Info(ctx, "User no longer exists, removing bucket link from state")
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Resolving User",
			fmt.Sprintf("Could not resolve user %s for bucket link: %s", uid, err.Error()),
		)
		return
	}

	// Get the effective bucket name (might have been renamed)
	effectiveBucketName := data.Bucket.ValueString()
	if !data.NewBucketName.IsNull() && data.NewBucketName.ValueString() != "" {
		effectiveBucketName = data.NewBucketName.ValueString()
	}

	// Get user's buckets to verify the link still exists
	buckets, err := r.client.Admin.ListUsersBuckets(ctx, resolvedUID)
	if err != nil {
		if errors.Is(err, admin.ErrNoSuchUser) {
			tflog.Info(ctx, "User no longer exists, removing bucket link from state")
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading Bucket Link",
			fmt.Sprintf("Could not list buckets for user %s: %s", uid, err.Error()),
		)
		return
	}

	// Check if bucket is in user's bucket list
	if !slices.Contains(buckets, effectiveBucketName) {
		tflog.Info(ctx, "Bucket is no longer linked to user, removing from state", map[string]any{
			"bucket": effectiveBucketName,
			"uid":    data.UID.ValueString(),
		})
		resp.State.RemoveResource(ctx)
		return
	}

	// Get bucket info for bucket_id. For a tenant user the bucket lives in the
	// tenant namespace, so address it as "tenant/bucket".
	bucketInfo, err := r.client.Admin.GetBucketInfo(ctx, admin.Bucket{Bucket: tenantScopedBucket(bucketTenant(resolvedUID), effectiveBucketName)})
	if err != nil {
		if errors.Is(err, admin.ErrNoSuchBucket) {
			tflog.Info(ctx, "Bucket no longer exists, removing from state")
			resp.State.RemoveResource(ctx)
			return
		}
		tflog.Warn(ctx, "Could not retrieve bucket info", map[string]any{
			"bucket": effectiveBucketName,
			"error":  err.Error(),
		})
	} else {
		data.BucketID = types.StringValue(bucketInfo.ID)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *BucketLinkResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data BucketLinkResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Only unlink_to_uid can be updated in place (bucket, uid, new_bucket_name require replace)
	// Just save the updated state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *BucketLinkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data BucketLinkResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Get the effective bucket name
	effectiveBucketName := data.Bucket.ValueString()
	if !data.NewBucketName.IsNull() && data.NewBucketName.ValueString() != "" {
		effectiveBucketName = data.NewBucketName.ValueString()
	}

	tflog.Debug(ctx, "Deleting bucket link", map[string]any{
		"bucket":        effectiveBucketName,
		"uid":           data.UID.ValueString(),
		"unlink_to_uid": data.UnlinkToUID.ValueString(),
	})

	// Resolve the current owner to determine the bucket's tenant namespace.
	resolvedUID, resolveErr := resolveUserID(ctx, r.client, data.UID.ValueString())
	if resolveErr != nil {
		if errors.Is(resolveErr, admin.ErrNoSuchUser) {
			return
		}
		resp.Diagnostics.AddError(
			"Error Resolving User",
			fmt.Sprintf("Could not resolve user %s for bucket unlink: %s", data.UID.ValueString(), resolveErr.Error()),
		)
		return
	}
	currentTenant := bucketTenant(resolvedUID)

	var err error
	if !data.UnlinkToUID.IsNull() && data.UnlinkToUID.ValueString() != "" {
		unlinkToUID := data.UnlinkToUID.ValueString()
		resolvedUnlinkToUID, relinkErr := resolveUserID(ctx, r.client, unlinkToUID)
		if relinkErr != nil {
			if errors.Is(relinkErr, admin.ErrNoSuchUser) {
				return
			}
			resp.Diagnostics.AddError(
				"Error Resolving User",
				fmt.Sprintf("Could not resolve user %s for bucket relink: %s", unlinkToUID, relinkErr.Error()),
			)
			return
		}
		// Relink to a different user. Address the bucket in its current namespace:
		// "tenant/bucket" if currently owned by a tenant user, otherwise the
		// leading-slash form when moving a default-namespace bucket to a tenant.
		source := tenantScopedBucket(currentTenant, effectiveBucketName)
		if currentTenant == "" {
			source = linkSourceBucket(bucketTenant(resolvedUnlinkToUID), effectiveBucketName)
		}
		err = retryOnConcurrentModification(ctx, fmt.Sprintf("LinkBucket %s to %s (on destroy)", source, resolvedUnlinkToUID), func() error {
			return r.client.Admin.LinkBucket(ctx, admin.BucketLinkInput{
				Bucket: source,
				UID:    resolvedUnlinkToUID,
			})
		})
	} else {
		// Unlink from the current user, addressing the bucket in its namespace.
		unlinkBucketName := tenantScopedBucket(currentTenant, effectiveBucketName)
		err = retryOnConcurrentModification(ctx, fmt.Sprintf("UnlinkBucket %s from %s", unlinkBucketName, resolvedUID), func() error {
			return r.client.Admin.UnlinkBucket(ctx, admin.BucketLinkInput{
				Bucket: unlinkBucketName,
				UID:    resolvedUID,
			})
		})
	}

	if err != nil {
		// Ignore errors if bucket no longer exists
		if !errors.Is(err, admin.ErrNoSuchBucket) && !errors.Is(err, admin.ErrNoSuchUser) {
			resp.Diagnostics.AddError(
				"Error Deleting Bucket Link",
				fmt.Sprintf("Could not unlink/relink bucket %s: %s", effectiveBucketName, err.Error()),
			)
			return
		}
	}
}

func (r *BucketLinkResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import format: "bucket:uid" or just "bucket" (uid will be read from bucket info)
	parts := strings.SplitN(req.ID, ":", 2)

	bucket := parts[0]
	var uid string

	if len(parts) == 2 {
		uid = parts[1]
	} else {
		// Get bucket info to find the owner
		bucketInfo, err := r.client.Admin.GetBucketInfo(ctx, admin.Bucket{Bucket: bucket})
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Importing Bucket Link",
				fmt.Sprintf("Could not get bucket info for %s: %s. Try importing with format 'bucket:uid'.", bucket, err.Error()),
			)
			return
		}
		uid = bucketInfo.Owner
	}

	tflog.Debug(ctx, "Importing bucket link", map[string]any{
		"bucket": bucket,
		"uid":    uid,
	})

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("bucket"), bucket)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("uid"), uid)...)
}
