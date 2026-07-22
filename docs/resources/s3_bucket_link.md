---
subcategory: "S3 (Simple Storage)"
page_title: "RadosGW: radosgw_s3_bucket_link"
description: |-
  Manages bucket ownership in RadosGW by linking a bucket to a specified user.
  This resource links an existing bucket to a user, unlinking it from any previous owner. It is primarily useful for:
  Transferring bucket ownership between usersMoving buckets from one tenant to anotherRenaming buckets during the link operation
  On destruction, the bucket can optionally be linked to a different user (via unlink_to_uid), or simply unlinked from the current user.
  ~> Note: The bucket must already exist. This resource does not create buckets, only manages ownership. For same-namespace links (the target user is in the same namespace as the bucket — i.e. no tenant, or the same tenant), the owner attribute on radosgw_s3_bucket is read-only, so the two resources can be used together without conflict. This is not true when linking to a tenant user — see the tenant note below.
  ~> Important: When transferring bucket ownership, the radosgw_s3_bucket_acl and radosgw_s3_bucket_policy resources can only be managed by the bucket owner. If you transfer ownership to a different user, you will need separate provider credentials (aliases) to manage those resources.
  ~> Tenant users: RadosGW scopes buckets by tenant. Linking a bucket to a tenant user (a uid of the form tenant$user, e.g. when OpenStack Keystone rgw_keystone_implicit_tenants is enabled) moves the bucket into that tenant's namespace (it becomes tenant/bucket). The provider handles the addressing automatically — you still reference the plain bucket name here. However, because the bucket physically moves namespaces, do not also manage that same bucket's lifecycle with radosgw_s3_bucket by its plain name (it will report drift and try to recreate it). Create such buckets out-of-band (or in the tenant) and manage only their ownership with this resource.
---

# radosgw_s3_bucket_link

Manages bucket ownership in RadosGW by linking a bucket to a specified user.

This resource links an existing bucket to a user, unlinking it from any previous owner. It is primarily useful for:
- Transferring bucket ownership between users
- Moving buckets from one tenant to another
- Renaming buckets during the link operation

On destruction, the bucket can optionally be linked to a different user (via `unlink_to_uid`), or simply unlinked from the current user.

~> **Note:** The bucket must already exist. This resource does not create buckets, only manages ownership. For **same-namespace** links (the target user is in the same namespace as the bucket — i.e. no tenant, or the same tenant), the `owner` attribute on `radosgw_s3_bucket` is read-only, so the two resources can be used together without conflict. This is **not** true when linking to a *tenant* user — see the tenant note below.

~> **Important:** When transferring bucket ownership, the `radosgw_s3_bucket_acl` and `radosgw_s3_bucket_policy` resources can only be managed by the bucket owner. If you transfer ownership to a different user, you will need separate provider credentials (aliases) to manage those resources.

~> **Tenant users:** RadosGW scopes buckets by tenant. Linking a bucket to a tenant user (a `uid` of the form `tenant$user`, e.g. when OpenStack Keystone `rgw_keystone_implicit_tenants` is enabled) **moves the bucket into that tenant's namespace** (it becomes `tenant/bucket`). The provider handles the addressing automatically — you still reference the plain bucket name here. However, because the bucket physically moves namespaces, do **not** also manage that same bucket's lifecycle with `radosgw_s3_bucket` by its plain name (it will report drift and try to recreate it). Create such buckets out-of-band (or in the tenant) and manage only their ownership with this resource.

## Example Usage

```terraform
# Create a bucket and transfer ownership using bucket_link
resource "radosgw_s3_bucket" "managed" {
  bucket        = "my-managed-bucket"
  force_destroy = true
}

resource "radosgw_s3_bucket_link" "managed" {
  bucket = radosgw_s3_bucket.managed.bucket
  uid    = radosgw_user.new_owner.user_id

  # On destroy, transfer back to original owner
  unlink_to_uid = radosgw_user.original_owner.user_id
}

# Transfer ownership of an existing bucket
resource "radosgw_s3_bucket_link" "transfer" {
  bucket = "existing-bucket"
  uid    = radosgw_user.new_owner.user_id
}

# Transfer bucket with automatic reversion on destroy
resource "radosgw_s3_bucket_link" "temporary" {
  bucket        = "shared-bucket"
  uid           = radosgw_user.temporary_user.user_id
  unlink_to_uid = radosgw_user.original_owner.user_id
}

# Rename a bucket while transferring ownership
resource "radosgw_s3_bucket_link" "rename" {
  bucket          = "old-bucket-name"
  uid             = radosgw_user.new_owner.user_id
  new_bucket_name = "new-bucket-name"
}

# Move bucket between tenants
resource "radosgw_s3_bucket_link" "tenant_move" {
  bucket = "bucket-to-move"
  uid    = "tenant1$user1" # tenant$user format
}

# Reference user resources
resource "radosgw_user" "new_owner" {
  user_id      = "new-owner"
  display_name = "New Bucket Owner"
}

resource "radosgw_user" "original_owner" {
  user_id      = "original-owner"
  display_name = "Original Bucket Owner"
}

resource "radosgw_user" "temporary_user" {
  user_id      = "temporary-user"
  display_name = "Temporary User"
}
```

<!-- schema generated by tfplugindocs -->

## Argument Reference

The following arguments are supported:


* `bucket` - (Required) The name of the bucket to link. The bucket must already exist.
* `uid` - (Required) The user ID to link the bucket to. This user will become the bucket owner.


* `new_bucket_name` - (Optional) Optional new name for the bucket. Use this to rename the bucket during the link operation.
* `unlink_to_uid` - (Optional) The user ID to link the bucket to when this resource is destroyed. If not set, the bucket will be unlinked from the user but remain in the system.




## Attributes Reference

The following attributes are exported:

* `bucket_id` - The unique bucket ID assigned by RadosGW.
* `bucket` - See Argument Reference above.
* `uid` - See Argument Reference above.
* `new_bucket_name` - See Argument Reference above.
* `unlink_to_uid` - See Argument Reference above.
## Import

Import is supported using the following syntax:

```shell
# Import a bucket link by bucket name (owner is auto-detected)
terraform import radosgw_s3_bucket_link.example "my-bucket"

# Import with explicit owner (bucket:uid format)
terraform import radosgw_s3_bucket_link.example "my-bucket:bucket-owner"

# Import a bucket in a tenant
terraform import radosgw_s3_bucket_link.tenant "my-bucket:tenant1\$user1"
```
