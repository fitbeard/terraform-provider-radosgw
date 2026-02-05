# Import a user quota (total quota across all user's buckets)
# Format: user_id:type (type is "user" or "bucket")
terraform import radosgw_iam_quota.user_quota example-user:user

# Import a bucket quota (per-bucket quota for all user's buckets)
terraform import radosgw_iam_quota.bucket_quota example-user:bucket
