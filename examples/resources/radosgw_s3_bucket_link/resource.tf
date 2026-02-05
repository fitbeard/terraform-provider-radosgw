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
