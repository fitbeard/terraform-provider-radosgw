---
subcategory: "S3 (Simple Storage)"
page_title: "RadosGW: radosgw_s3_bucket_ownership_controls"
description: |-
  Manages S3 Object Ownership for a bucket in RadosGW, via the standard S3 PutBucketOwnershipControls API. This is the RadosGW equivalent of aws_s3_bucket_ownership_controls and controls whether object ACLs are used — set BucketOwnerEnforced to disable ACLs entirely.
  Object ownership assignment matches AWS:
  ObjectWriter: the uploader owns the object. Uploading with the bucket-owner-full-control canned ACL grants the bucket owner FULL_CONTROL but the uploader remains the owner.BucketOwnerPreferred: the uploader owns the object, unless it is uploaded with the bucket-owner-full-control canned ACL, in which case the bucket owner owns it.BucketOwnerEnforced: the bucket owner owns every object; ACLs are disabled and access is governed only by bucket/IAM policy.
  Changing the mode affects only future uploads; existing objects keep their owner (switching to BucketOwnerEnforced additionally stops object ACLs from being evaluated).
  ~> Ceph version: S3 Object Ownership requires Ceph 21 (Umbrella). When the provider's ceph_version targets an older release, this resource is rejected at plan time.
  ~> RadosGW differences from AWS (verified on Ceph 21.2.0):
  Default (no configuration): a RadosGW bucket with no ownership controls has ACLs enabled (ObjectWriter-equivalent). AWS defaults new buckets to BucketOwnerEnforced. To disable ACLs on RadosGW you must set BucketOwnerEnforced explicitly with this resource. (The bucket's default ACL is still private/owner-only — ACLs being enabled means a public-read ACL can be set, not that the bucket is public; use radosgw_s3_bucket_public_access_block to prevent public ACLs.)Under BucketOwnerEnforced: setting an ACL via PutObject/CopyObject (x-amz-acl) or PutBucketAcl is rejected with AccessControlListNotSupported (same as AWS). However, GetObjectAcl and PutObjectAcl return 403 AccessDenied on RadosGW (AWS returns the implicit owner-only ACL for the read, and AccessControlListNotSupported for the write). GetBucketAcl still returns the owner ACL.
  ~> Note: A bucket has a single ownership-controls configuration. Declaring multiple radosgw_s3_bucket_ownership_controls resources for the same bucket will cause them to overwrite each other. When this resource is destroyed, the ownership controls are removed from the bucket.
---

# radosgw_s3_bucket_ownership_controls

Manages S3 Object Ownership for a bucket in RadosGW, via the standard S3 `PutBucketOwnershipControls` API. This is the RadosGW equivalent of `aws_s3_bucket_ownership_controls` and controls whether object ACLs are used — set `BucketOwnerEnforced` to disable ACLs entirely.

Object ownership assignment matches AWS:

- `ObjectWriter`: the uploader owns the object. Uploading with the `bucket-owner-full-control` canned ACL grants the bucket owner `FULL_CONTROL` but the uploader remains the owner.
- `BucketOwnerPreferred`: the uploader owns the object, **unless** it is uploaded with the `bucket-owner-full-control` canned ACL, in which case the bucket owner owns it.
- `BucketOwnerEnforced`: the bucket owner owns every object; ACLs are disabled and access is governed only by bucket/IAM policy.

Changing the mode affects only **future** uploads; existing objects keep their owner (switching to `BucketOwnerEnforced` additionally stops object ACLs from being evaluated).

~> **Ceph version:** S3 Object Ownership requires **Ceph 21 (Umbrella)**. When the provider's `ceph_version` targets an older release, this resource is rejected at plan time.

~> **RadosGW differences from AWS** (verified on Ceph 21.2.0):
- **Default (no configuration):** a RadosGW bucket with no ownership controls has ACLs **enabled** (`ObjectWriter`-equivalent). AWS defaults new buckets to `BucketOwnerEnforced`. To disable ACLs on RadosGW you must set `BucketOwnerEnforced` explicitly with this resource. (The bucket's default ACL is still private/owner-only — ACLs being enabled means a public-read ACL *can* be set, not that the bucket is public; use `radosgw_s3_bucket_public_access_block` to prevent public ACLs.)
- **Under `BucketOwnerEnforced`:** setting an ACL via `PutObject`/`CopyObject` (`x-amz-acl`) or `PutBucketAcl` is rejected with `AccessControlListNotSupported` (same as AWS). However, `GetObjectAcl` and `PutObjectAcl` return `403 AccessDenied` on RadosGW (AWS returns the implicit owner-only ACL for the read, and `AccessControlListNotSupported` for the write). `GetBucketAcl` still returns the owner ACL.

~> **Note:** A bucket has a single ownership-controls configuration. Declaring multiple `radosgw_s3_bucket_ownership_controls` resources for the same bucket will cause them to overwrite each other. When this resource is destroyed, the ownership controls are removed from the bucket.

## Example Usage

```terraform
# Disable ACLs on a bucket (the bucket owner owns every object).
# Requires Ceph 21 (Umbrella).
resource "radosgw_s3_bucket" "example" {
  bucket = "my-ownership-bucket"
}

resource "radosgw_s3_bucket_ownership_controls" "example" {
  bucket = radosgw_s3_bucket.example.bucket

  rule {
    object_ownership = "BucketOwnerEnforced"
  }
}
```

<!-- schema generated by tfplugindocs -->

## Argument Reference

The following arguments are supported:


* `bucket` - (Required) The name of the bucket.


* `rule` - (Optional) The Object Ownership rule. Exactly one rule must be provided. (see [below for nested schema](#nestedblock--rule))


## Attributes Reference

The following attributes are exported:

* `bucket` - See Argument Reference above.
* `rule` - See Argument Reference above.

<a id="nestedblock--rule"></a>
### Nested Schema for `rule`

Required:

- `object_ownership` (String) Object ownership mode. Valid values: `BucketOwnerEnforced` (ACLs are disabled; the bucket owner owns every object), `BucketOwnerPreferred` (objects uploaded with the `bucket-owner-full-control` canned ACL are owned by the bucket owner), or `ObjectWriter` (the object writer remains the owner).

## Import

Import is supported using the following syntax:

```shell
# Import bucket ownership controls by bucket name
terraform import radosgw_s3_bucket_ownership_controls.example "my-ownership-bucket"
```
