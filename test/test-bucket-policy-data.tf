# =============================================================================
# Bucket Policy Data Source Tests
# =============================================================================
# Purpose: Test radosgw_s3_bucket_policy data source
# Resources: 1 data source
# Dependencies: test-bucket-policy.tf (policy_test bucket and bucket policy)
# =============================================================================

data "radosgw_s3_bucket_policy" "test" {
  bucket = radosgw_s3_bucket.policy_test.bucket

  depends_on = [radosgw_s3_bucket_policy.test]
}

# =============================================================================
# Outputs
# =============================================================================

output "data_bucket_policy" {
  description = "Retrieved bucket policy JSON"
  value       = data.radosgw_s3_bucket_policy.test.policy
}

output "data_bucket_policy_id" {
  value = data.radosgw_s3_bucket_policy.test.id
}
