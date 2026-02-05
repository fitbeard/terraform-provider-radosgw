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
