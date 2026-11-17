# =============================================================================
# S3 Bucket Ownership Controls Resource Test
# =============================================================================
# Purpose: Test radosgw_s3_bucket_ownership_controls (S3 Object Ownership)
# Resources: 1 bucket + 1 ownership-controls configuration
# Dependencies: None (standalone)
#
# Requires Ceph 21 (Umbrella). S3 Object Ownership is Umbrella-only, so the
# provider gates this resource on `ceph_version`: on an older release it is
# rejected at plan time. The dev bootstrap runs Umbrella.
#
# Manual verification after `tofu apply`:
#   AWS_ACCESS_KEY_ID=admin AWS_SECRET_ACCESS_KEY=secretkey \
#     aws --endpoint-url http://127.0.0.1:7480 --region default \
#     s3api get-bucket-ownership-controls --bucket test-ownership-bucket
#   -> Rules[0].ObjectOwnership == "BucketOwnerEnforced"
# =============================================================================

resource "radosgw_s3_bucket" "ownership" {
  bucket        = "test-ownership-bucket"
  force_destroy = true
}

resource "radosgw_s3_bucket_ownership_controls" "ownership" {
  bucket = radosgw_s3_bucket.ownership.bucket

  # BucketOwnerEnforced disables ACLs entirely (recommended). Other valid values:
  # BucketOwnerPreferred, ObjectWriter.
  rule {
    object_ownership = "BucketOwnerEnforced"
  }
}

output "ownership_bucket" {
  value = radosgw_s3_bucket.ownership.bucket
}

output "object_ownership" {
  value = radosgw_s3_bucket_ownership_controls.ownership.rule[0].object_ownership
}
