# Import a bucket rate limit by bucket name
terraform import radosgw_s3_bucket_rate_limit.example "my-rate-limited-bucket"

# For a bucket in a tenant namespace, use "tenant/bucket"
terraform import radosgw_s3_bucket_rate_limit.example "my-tenant/my-rate-limited-bucket"
