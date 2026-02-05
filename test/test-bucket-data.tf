# =============================================================================
# S3 Bucket Data Source Tests
# =============================================================================
# Purpose: Test radosgw_s3_bucket data source for bucket information retrieval
# Resources: 1 data source
# Dependencies: test-bucket.tf (test_force_destroy bucket)
# =============================================================================

data "radosgw_s3_bucket" "test" {
  bucket = radosgw_s3_bucket.test_force_destroy.bucket

  depends_on = [radosgw_s3_bucket.test_force_destroy]
}

# =============================================================================
# Outputs - demonstrates all available attributes
# =============================================================================

output "bucket_id" {
  value = data.radosgw_s3_bucket.test.id
}

output "bucket_owner" {
  value = data.radosgw_s3_bucket.test.owner
}

output "bucket_creation_time" {
  value = data.radosgw_s3_bucket.test.creation_time
}

output "bucket_versioning" {
  value = data.radosgw_s3_bucket.test.versioning
}

output "bucket_placement" {
  value = data.radosgw_s3_bucket.test.explicit_placement
}

output "bucket_quota_info" {
  value = data.radosgw_s3_bucket.test.bucket_quota
}

output "bucket_zonegroup" {
  value = data.radosgw_s3_bucket.test.zonegroup
}
