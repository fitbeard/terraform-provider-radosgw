# =============================================================================
# Access Keys Data Source Tests
# =============================================================================
# Purpose: Test radosgw_iam_access_keys data source with filtering
# Resources: 3 data sources (all keys, S3 only, Swift only)
# Dependencies: test-keys.tf (access keys and subuser)
# =============================================================================

# Get all access keys (S3 and Swift) for the test user
data "radosgw_iam_access_keys" "all" {
  user_id = radosgw_iam_user.test.user_id

  depends_on = [
    radosgw_iam_access_key.test_s3_auto,
    radosgw_iam_access_key.test_s3_custom,
    radosgw_iam_access_key.test_swift,
  ]
}

# Get only S3 access keys using key_type filter
data "radosgw_iam_access_keys" "s3_only" {
  user_id  = radosgw_iam_user.test.user_id
  key_type = "s3"

  depends_on = [
    radosgw_iam_access_key.test_s3_auto,
    radosgw_iam_access_key.test_s3_custom,
  ]
}

# Get only Swift access keys using key_type filter
data "radosgw_iam_access_keys" "swift_only" {
  user_id  = radosgw_iam_user.test.user_id
  key_type = "swift"

  depends_on = [
    radosgw_iam_access_key.test_swift,
  ]
}

# =============================================================================
# Outputs
# =============================================================================

output "all_access_keys_count" {
  description = "Total number of access keys (S3 + Swift)"
  value       = length(data.radosgw_iam_access_keys.all.access_keys)
}

output "s3_access_keys_count" {
  description = "Number of S3 access keys"
  value       = length(data.radosgw_iam_access_keys.s3_only.access_keys)
}

output "swift_access_keys_count" {
  description = "Number of Swift access keys"
  value       = length(data.radosgw_iam_access_keys.swift_only.access_keys)
}

output "s3_access_key_ids" {
  description = "List of S3 access key IDs"
  value       = [for key in data.radosgw_iam_access_keys.s3_only.access_keys : key.access_key_id]
}
