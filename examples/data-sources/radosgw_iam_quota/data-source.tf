# Get user quota (total limit across all user's buckets)
data "radosgw_iam_quota" "user_quota" {
  user_id = radosgw_iam_user.example.user_id
  type    = "user"
}

# Get bucket quota (per-bucket limit for all user's buckets)
data "radosgw_iam_quota" "bucket_quota" {
  user_id = radosgw_iam_user.example.user_id
  type    = "bucket"
}

# Reference user resource
resource "radosgw_iam_user" "example" {
  user_id      = "example-user"
  display_name = "Example User"
}

# Output user quota details
output "user_quota" {
  description = "User's total quota across all buckets"
  value = {
    enabled     = data.radosgw_iam_quota.user_quota.enabled
    max_size    = data.radosgw_iam_quota.user_quota.max_size
    max_objects = data.radosgw_iam_quota.user_quota.max_objects
  }
}

# Output bucket quota details
output "bucket_quota" {
  description = "Per-bucket quota for user's buckets"
  value = {
    enabled     = data.radosgw_iam_quota.bucket_quota.enabled
    max_size    = data.radosgw_iam_quota.bucket_quota.max_size
    max_objects = data.radosgw_iam_quota.bucket_quota.max_objects
  }
}

# Check if quotas are enabled
output "quotas_enabled" {
  description = "Whether quotas are enabled"
  value = {
    user_quota_enabled   = data.radosgw_iam_quota.user_quota.enabled
    bucket_quota_enabled = data.radosgw_iam_quota.bucket_quota.enabled
  }
}
