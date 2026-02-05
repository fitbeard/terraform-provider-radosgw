package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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
var _ resource.Resource = &BucketAclResource{}
var _ resource.ResourceWithImportState = &BucketAclResource{}

func NewS3BucketAclResource() resource.Resource {
	return &BucketAclResource{}
}

// BucketAclResource defines the resource implementation.
type BucketAclResource struct {
	client *RadosgwClient
}

// BucketAclResourceModel describes the resource data model.
type BucketAclResourceModel struct {
	ID     types.String `tfsdk:"id"`
	Bucket types.String `tfsdk:"bucket"`
	Acl    types.String `tfsdk:"acl"`
}

func (r *BucketAclResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_s3_bucket_acl"
}

func (r *BucketAclResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Manages the ACL (Access Control List) for an S3 bucket in Ceph RadosGW. This resource allows you to set canned ACLs on buckets and tracks drift when the ACL is changed outside of Terraform.

~> **Important:** This resource can only manage ACLs for buckets owned by the user configured in the provider. The S3 API restricts ACL operations to the bucket owner only - even admin credentials cannot manage ACLs on buckets owned by other users. If you need to manage ACLs on buckets with different owners, you must use separate provider configurations (aliases) with each owner's credentials.

~> **Note:** When destroying this resource, the bucket ACL is reset to ` + "`private`" + `.`,

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The resource identifier (bucket name).",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"bucket": schema.StringAttribute{
				MarkdownDescription: "The name of the bucket to apply the ACL to.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"acl": schema.StringAttribute{
				MarkdownDescription: "The canned ACL to apply to the bucket. Valid values: `private`, `public-read`, `public-read-write`, `authenticated-read`.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("private", "public-read", "public-read-write", "authenticated-read"),
				},
			},
		},
	}
}

func (r *BucketAclResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *BucketAclResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data BucketAclResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucketName := data.Bucket.ValueString()
	acl := data.Acl.ValueString()

	tflog.Debug(ctx, "Setting bucket ACL", map[string]any{
		"bucket": bucketName,
		"acl":    acl,
	})

	err := r.putBucketAcl(ctx, bucketName, acl)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Setting Bucket ACL",
			fmt.Sprintf("Could not set ACL on bucket %s: %s", bucketName, err.Error()),
		)
		return
	}

	// Set the ID (just bucket name for stability)
	data.ID = types.StringValue(bucketName)

	tflog.Trace(ctx, "Set bucket ACL", map[string]any{
		"bucket": bucketName,
		"acl":    acl,
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *BucketAclResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data BucketAclResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucketName := data.Bucket.ValueString()

	tflog.Debug(ctx, "Reading bucket ACL", map[string]any{
		"bucket": bucketName,
	})

	// Get current ACL from S3 API
	currentAcl, err := r.getBucketAcl(ctx, bucketName)
	if err != nil {
		// Check if bucket doesn't exist
		if isBucketNotFoundS3Error(err) {
			tflog.Debug(ctx, "Bucket not found, removing ACL resource from state", map[string]any{
				"bucket": bucketName,
			})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading Bucket ACL",
			fmt.Sprintf("Could not read ACL for bucket %s: %s", bucketName, err.Error()),
		)
		return
	}

	// Update the model with current ACL (drift detection)
	data.Acl = types.StringValue(currentAcl)
	// ID stays as bucket name (stable)

	tflog.Debug(ctx, "Read bucket ACL", map[string]any{
		"bucket": bucketName,
		"acl":    currentAcl,
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *BucketAclResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data BucketAclResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucketName := data.Bucket.ValueString()
	acl := data.Acl.ValueString()

	tflog.Debug(ctx, "Updating bucket ACL", map[string]any{
		"bucket": bucketName,
		"acl":    acl,
	})

	err := r.putBucketAcl(ctx, bucketName, acl)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Bucket ACL",
			fmt.Sprintf("Could not update ACL on bucket %s: %s", bucketName, err.Error()),
		)
		return
	}

	// ID stays as bucket name (stable)
	data.ID = types.StringValue(bucketName)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *BucketAclResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data BucketAclResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucketName := data.Bucket.ValueString()

	tflog.Debug(ctx, "Resetting bucket ACL to private", map[string]any{
		"bucket": bucketName,
	})

	// Reset ACL to private on delete
	err := r.putBucketAcl(ctx, bucketName, "private")
	if err != nil {
		// Ignore errors if bucket doesn't exist
		if !isBucketNotFoundS3Error(err) {
			resp.Diagnostics.AddError(
				"Error Resetting Bucket ACL",
				fmt.Sprintf("Could not reset ACL on bucket %s: %s", bucketName, err.Error()),
			)
			return
		}
	}

	tflog.Trace(ctx, "Reset bucket ACL to private", map[string]any{
		"bucket": bucketName,
	})
}

func (r *BucketAclResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import format: bucket_name
	bucketName := req.ID
	if bucketName == "" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Import ID must be the bucket name.",
		)
		return
	}

	tflog.Debug(ctx, "Importing bucket ACL", map[string]any{
		"bucket": bucketName,
	})

	// Read current ACL
	currentAcl, err := r.getBucketAcl(ctx, bucketName)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Importing Bucket ACL",
			fmt.Sprintf("Could not read ACL for bucket %s: %s", bucketName, err.Error()),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("bucket"), bucketName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("acl"), currentAcl)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), bucketName)...)
}

// putBucketAcl sets a canned ACL on a bucket.
func (r *BucketAclResource) putBucketAcl(ctx context.Context, bucketName, acl string) error {
	var cannedAcl s3types.BucketCannedACL
	switch acl {
	case "private":
		cannedAcl = s3types.BucketCannedACLPrivate
	case "public-read":
		cannedAcl = s3types.BucketCannedACLPublicRead
	case "public-read-write":
		cannedAcl = s3types.BucketCannedACLPublicReadWrite
	case "authenticated-read":
		cannedAcl = s3types.BucketCannedACLAuthenticatedRead
	default:
		return fmt.Errorf("unsupported ACL: %s", acl)
	}

	_, err := r.client.S3.PutBucketAcl(ctx, &s3.PutBucketAclInput{
		Bucket: &bucketName,
		ACL:    cannedAcl,
	})
	return err
}

// getBucketAcl retrieves the current ACL of a bucket and maps it to a canned ACL string.
func (r *BucketAclResource) getBucketAcl(ctx context.Context, bucketName string) (string, error) {
	output, err := r.client.S3.GetBucketAcl(ctx, &s3.GetBucketAclInput{
		Bucket: &bucketName,
	})
	if err != nil {
		return "", err
	}

	return mapGrantsToCannedAcl(output.Owner, output.Grants), nil
}

// mapGrantsToCannedAcl maps S3 ACL grants to a canned ACL string.
// Returns "private" if no matching canned ACL pattern is found.
func mapGrantsToCannedAcl(owner *s3types.Owner, grants []s3types.Grant) string {
	if owner == nil {
		return "private"
	}

	ownerID := ""
	if owner.ID != nil {
		ownerID = *owner.ID
	}

	// Track what grants we have
	hasOwnerFullControl := false
	hasAllUsersRead := false
	hasAllUsersWrite := false
	hasAuthenticatedUsersRead := false

	for _, grant := range grants {
		if grant.Grantee == nil || grant.Permission == "" {
			continue
		}

		switch grant.Grantee.Type {
		case s3types.TypeCanonicalUser:
			// Check if this is the owner with FULL_CONTROL
			if grant.Grantee.ID != nil && *grant.Grantee.ID == ownerID {
				if grant.Permission == s3types.PermissionFullControl {
					hasOwnerFullControl = true
				}
			}
		case s3types.TypeGroup:
			if grant.Grantee.URI == nil {
				continue
			}
			uri := *grant.Grantee.URI
			switch uri {
			case "http://acs.amazonaws.com/groups/global/AllUsers":
				switch grant.Permission {
				case s3types.PermissionRead:
					hasAllUsersRead = true
				case s3types.PermissionWrite:
					hasAllUsersWrite = true
				}
			case "http://acs.amazonaws.com/groups/global/AuthenticatedUsers":
				if grant.Permission == s3types.PermissionRead {
					hasAuthenticatedUsersRead = true
				}
			}
		}
	}

	// Match grant patterns to canned ACLs
	// All canned ACLs require owner to have FULL_CONTROL
	if !hasOwnerFullControl {
		return "private"
	}

	// public-read-write: owner FULL_CONTROL + AllUsers READ + AllUsers WRITE
	if hasAllUsersRead && hasAllUsersWrite && !hasAuthenticatedUsersRead {
		return "public-read-write"
	}

	// public-read: owner FULL_CONTROL + AllUsers READ (no WRITE)
	if hasAllUsersRead && !hasAllUsersWrite && !hasAuthenticatedUsersRead {
		return "public-read"
	}

	// authenticated-read: owner FULL_CONTROL + AuthenticatedUsers READ
	if hasAuthenticatedUsersRead && !hasAllUsersRead && !hasAllUsersWrite {
		return "authenticated-read"
	}

	// private: only owner has FULL_CONTROL
	if !hasAllUsersRead && !hasAllUsersWrite && !hasAuthenticatedUsersRead {
		return "private"
	}

	// Unknown pattern - default to private
	return "private"
}

// isBucketNotFoundS3Error checks if an S3 error indicates the bucket doesn't exist.
func isBucketNotFoundS3Error(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "NoSuchBucket") || strings.Contains(errStr, "404")
}
