# Look up the rate limit configured for an existing bucket
data "radosgw_s3_bucket_rate_limit" "example" {
  bucket = "my-rate-limited-bucket"
}

output "bucket_rate_limit" {
  description = "Rate limit settings for the bucket"
  value = {
    enabled         = data.radosgw_s3_bucket_rate_limit.example.enabled
    max_read_ops    = data.radosgw_s3_bucket_rate_limit.example.max_read_ops
    max_write_ops   = data.radosgw_s3_bucket_rate_limit.example.max_write_ops
    max_read_bytes  = data.radosgw_s3_bucket_rate_limit.example.max_read_bytes
    max_write_bytes = data.radosgw_s3_bucket_rate_limit.example.max_write_bytes
  }
}
