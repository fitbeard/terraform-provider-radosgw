# =============================================================================
# Bucket Link Resource Tests
# =============================================================================
# Purpose: Test radosgw_s3_bucket_link resource for transferring bucket ownership
# Covers three cases: (1) a plain admin bucket linked to another user; (2) a
# default-namespace bucket linked to a tenant user; (3) a bucket that already
# lives in a tenant namespace (created by the tenant user) linked to that user.
# Dependencies: cases (2) and (3) create the source bucket out-of-band via the
# AWS CLI (terraform_data provisioners), so `aws` must be on PATH.
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

# -----------------------------------------------------------------------------
# Tenant user link — source bucket ALREADY in the tenant namespace
# -----------------------------------------------------------------------------
# The section above links a bucket that starts in the DEFAULT namespace. This
# one covers the case when the bucket is created
# BY the tenant/Keystone user over S3, so it already lives in the *tenant*
# namespace (tenant/bucket). Linking it used to fail with "NoSuchKey" because the
# provider only addressed the default namespace. It now falls back to the tenant
# namespace, so a plain bucket name just works — no drift on re-apply.
#
# The bucket is created out-of-band with the AWS CLI authenticated AS the tenant
# user (its auto-generated key), so it lands in the tenant namespace. On destroy
# the link returns it to admin (default namespace) and it is then removed.

resource "radosgw_iam_user" "tenant_owned_owner" {
  user_id      = "opensearch"
  display_name = "OpenSearch"
  tenant       = "sif"
}

resource "radosgw_iam_access_key" "tenant_owned_owner" {
  user_id = radosgw_iam_user.tenant_owned_owner.user_id
  # key pair auto-generated by RadosGW
}

# Source bucket created AS the tenant user -> lands in the "sif" tenant namespace.
resource "terraform_data" "tenant_owned_bucket" {
  input = "issue94-tenant-owned-bucket"

  provisioner "local-exec" {
    command = "AWS_ACCESS_KEY_ID=${radosgw_iam_access_key.tenant_owned_owner.access_key} AWS_SECRET_ACCESS_KEY=${radosgw_iam_access_key.tenant_owned_owner.secret_key} aws --endpoint-url http://127.0.0.1:7480 --region default s3api create-bucket --bucket ${self.input}"
  }

  # After the link returns the bucket to admin (default ns) on destroy, remove it.
  provisioner "local-exec" {
    when       = destroy
    command    = "AWS_ACCESS_KEY_ID=admin AWS_SECRET_ACCESS_KEY=secretkey aws --endpoint-url http://127.0.0.1:7480 --region default s3 rb s3://${self.input} --force"
    on_failure = continue
  }

  depends_on = [radosgw_iam_access_key.tenant_owned_owner]
}

# Link the tenant-namespace bucket to its tenant user, using plain names.
# Previously failed with NoSuchKey; now resolved via the tenant-namespace fallback.
resource "radosgw_s3_bucket_link" "test_link_tenant_owned" {
  bucket        = terraform_data.tenant_owned_bucket.input
  uid           = radosgw_iam_user.tenant_owned_owner.user_id
  unlink_to_uid = "admin" # Return to admin (default namespace) on destroy

  depends_on = [terraform_data.tenant_owned_bucket]
}

output "bucket_link_tenant_owned_owner" {
  value = radosgw_s3_bucket_link.test_link_tenant_owned.uid
}

output "bucket_link_tenant_owned_bucket_id" {
  value = radosgw_s3_bucket_link.test_link_tenant_owned.bucket_id
}
