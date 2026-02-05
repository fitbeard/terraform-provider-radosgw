package provider

import (
	"context"
	"errors"
	"fmt"
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

~> **Note:** The bucket must already exist. This resource does not create buckets, only manages ownership. The ` + "`owner`" + ` attribute on ` + "`radosgw_s3_bucket`" + ` is read-only, so this resource can be used alongside it without conflicts.

~> **Important:** When transferring bucket ownership, the ` + "`radosgw_s3_bucket_acl`" + ` and ` + "`radosgw_s3_bucket_policy`" + ` resources can only be managed by the bucket owner. If you transfer ownership to a different user, you will need separate provider credentials (aliases) to manage those resources.`,

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

	bucketLink := admin.BucketLinkInput{
		Bucket: data.Bucket.ValueString(),
		UID:    data.UID.ValueString(),
	}

	if !data.NewBucketName.IsNull() && data.NewBucketName.ValueString() != "" {
		bucketLink.NewBucketName = data.NewBucketName.ValueString()
	}

	tflog.Debug(ctx, "Linking bucket to user", map[string]any{
		"bucket":          data.Bucket.ValueString(),
		"uid":             data.UID.ValueString(),
		"new_bucket_name": data.NewBucketName.ValueString(),
	})

	// Link bucket with retry logic for ConcurrentModification
	err := retryOnConcurrentModification(ctx, fmt.Sprintf("LinkBucket %s to %s", data.Bucket.ValueString(), data.UID.ValueString()), func() error {
		return r.client.Admin.LinkBucket(ctx, bucketLink)
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Linking Bucket",
			fmt.Sprintf("Could not link bucket %s to user %s: %s", data.Bucket.ValueString(), data.UID.ValueString(), err.Error()),
		)
		return
	}

	// Get bucket info to retrieve the bucket ID
	effectiveBucketName := data.Bucket.ValueString()
	if !data.NewBucketName.IsNull() && data.NewBucketName.ValueString() != "" {
		effectiveBucketName = data.NewBucketName.ValueString()
	}

	bucketInfo, err := r.client.Admin.GetBucketInfo(ctx, admin.Bucket{Bucket: effectiveBucketName})
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

	// Get the effective bucket name (might have been renamed)
	effectiveBucketName := data.Bucket.ValueString()
	if !data.NewBucketName.IsNull() && data.NewBucketName.ValueString() != "" {
		effectiveBucketName = data.NewBucketName.ValueString()
	}

	// Get user's buckets to verify the link still exists
	buckets, err := r.client.Admin.ListUsersBuckets(ctx, data.UID.ValueString())
	if err != nil {
		if errors.Is(err, admin.ErrNoSuchUser) {
			tflog.Info(ctx, "User no longer exists, removing bucket link from state")
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading Bucket Link",
			fmt.Sprintf("Could not list buckets for user %s: %s", data.UID.ValueString(), err.Error()),
		)
		return
	}

	// Check if bucket is in user's bucket list
	found := false
	for _, bucket := range buckets {
		if bucket == effectiveBucketName {
			found = true
			break
		}
	}

	if !found {
		tflog.Info(ctx, "Bucket is no longer linked to user, removing from state", map[string]any{
			"bucket": effectiveBucketName,
			"uid":    data.UID.ValueString(),
		})
		resp.State.RemoveResource(ctx)
		return
	}

	// Get bucket info for bucket_id
	bucketInfo, err := r.client.Admin.GetBucketInfo(ctx, admin.Bucket{Bucket: effectiveBucketName})
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

	var err error
	if !data.UnlinkToUID.IsNull() && data.UnlinkToUID.ValueString() != "" {
		// Link bucket to a different user
		err = retryOnConcurrentModification(ctx, fmt.Sprintf("LinkBucket %s to %s (on destroy)", effectiveBucketName, data.UnlinkToUID.ValueString()), func() error {
			return r.client.Admin.LinkBucket(ctx, admin.BucketLinkInput{
				Bucket: effectiveBucketName,
				UID:    data.UnlinkToUID.ValueString(),
			})
		})
	} else {
		// Unlink bucket from current user
		err = retryOnConcurrentModification(ctx, fmt.Sprintf("UnlinkBucket %s from %s", effectiveBucketName, data.UID.ValueString()), func() error {
			return r.client.Admin.UnlinkBucket(ctx, admin.BucketLinkInput{
				Bucket: effectiveBucketName,
				UID:    data.UID.ValueString(),
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
