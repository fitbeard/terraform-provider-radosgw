# =============================================================================
# Bucket Link Resource Tests
# =============================================================================
# Purpose: Test radosgw_s3_bucket_link resource for transferring bucket ownership
# Resources: 2 users, 1 bucket, 1 bucket link
# Dependencies: None (standalone)
# Notes: On destroy, ownership is transferred back to admin to allow cleanup
# =============================================================================

# User to receive bucket ownership
resource "radosgw_iam_user" "bucket_owner1" {
  user_id      = "bucket-owner-1"
  display_name = "Bucket Owner 1"
}

# Alternative user for testing ownership transfer
resource "radosgw_iam_user" "bucket_owner2" {
  user_id      = "bucket-owner-2"
  display_name = "Bucket Owner 2"
}

# Bucket initially created by admin user (provider credentials)
resource "radosgw_s3_bucket" "test_bucket_link" {
  bucket        = "test-bucket-link"
  force_destroy = true
}

# Transfer bucket ownership from admin to bucket_owner2
# On destroy, transfer back to admin so bucket can be deleted
resource "radosgw_s3_bucket_link" "test_link" {
  bucket        = radosgw_s3_bucket.test_bucket_link.bucket
  uid           = radosgw_iam_user.bucket_owner2.user_id
  unlink_to_uid = "admin" # Transfer back to admin on destroy for cleanup

  depends_on = [radosgw_s3_bucket.test_bucket_link]
}

# =============================================================================
# Outputs
# =============================================================================

output "bucket_link_owner1_id" {
  value = radosgw_iam_user.bucket_owner1.user_id
}

output "bucket_link_owner2_id" {
  value = radosgw_iam_user.bucket_owner2.user_id
}

output "bucket_link_bucket_id" {
  value = radosgw_s3_bucket_link.test_link.bucket_id
}

output "bucket_link_current_owner" {
  value = radosgw_s3_bucket_link.test_link.uid
}

# -----------------------------------------------------------------------------
# Tenant user link
# -----------------------------------------------------------------------------
# Linking a bucket to a *tenant* user (a uid of the form tenant$user, e.g. from
# OpenStack Keystone implicit tenants) previously failed with "NoSuchKey"
# because RadosGW scopes buckets by tenant. The provider now addresses the
# bucket in the correct namespace automatically.
#
# IMPORTANT: linking to a tenant user MOVES the bucket into the tenant namespace
# (it becomes tenant/bucket). After the move, a radosgw_s3_bucket resource that
# tracks the same bucket by its plain name can no longer find it, drops it from
# state, and on the next apply RE-CREATES an empty duplicate in the default
# namespace. So do NOT co-manage a tenant-linked bucket with radosgw_s3_bucket.
# This mirrors the real-world scenario, where the bucket already exists (created
# by Keystone/federation or another tool) and only its ownership is managed here.
#
# To keep this example self-contained, the source bucket is created OUT OF BAND
# (via the AWS CLI in a terraform_data provisioner, NOT radosgw_s3_bucket) so it
# lands in the default namespace before being linked, and there is no drift on
# re-apply. It is deleted again on destroy, after the link returns it to admin.

resource "radosgw_iam_user" "tenant_bucket_owner" {
  user_id      = "tenant-bucket-owner"
  display_name = "TenantBucketOwner"
  tenant       = "tftenant1"
}

# Out-of-band source bucket (admin, default namespace). Requires the AWS CLI.
resource "terraform_data" "tenant_source_bucket" {
  input = "test-bucket-link-tenant"

  provisioner "local-exec" {
    command = "AWS_ACCESS_KEY_ID=admin AWS_SECRET_ACCESS_KEY=secretkey aws --endpoint-url http://127.0.0.1:7480 --region default s3api create-bucket --bucket ${self.input}"
  }

  # Delete the bucket on destroy (the link returns it to admin/default first).
  provisioner "local-exec" {
    when       = destroy
    command    = "AWS_ACCESS_KEY_ID=admin AWS_SECRET_ACCESS_KEY=secretkey aws --endpoint-url http://127.0.0.1:7480 --region default s3 rb s3://${self.input} --force"
    on_failure = continue
  }
}

resource "radosgw_s3_bucket_link" "test_link_tenant" {
  bucket        = terraform_data.tenant_source_bucket.input
  uid           = radosgw_iam_user.tenant_bucket_owner.user_id
  unlink_to_uid = "admin" # Return to admin (default namespace) on destroy

  depends_on = [radosgw_iam_user.tenant_bucket_owner]
}

output "bucket_link_tenant_owner" {
  value = radosgw_s3_bucket_link.test_link_tenant.uid
}

output "bucket_link_tenant_bucket_id" {
  value = radosgw_s3_bucket_link.test_link_tenant.bucket_id
}
