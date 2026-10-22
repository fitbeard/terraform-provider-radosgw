# Per-bucket rate limit (Admin Ops API; requires the `ratelimit` capability)
resource "radosgw_s3_bucket" "example" {
  bucket = "my-rate-limited-bucket"
}

resource "radosgw_s3_bucket_rate_limit" "example" {
  bucket = radosgw_s3_bucket.example.bucket

  enabled         = true
  max_read_ops    = 1000    # read operations per minute (0 disables)
  max_write_ops   = 500     # write operations per minute (0 disables)
  max_read_bytes  = 1048576 # read bytes per minute (0 disables)
  max_write_bytes = 524288  # write bytes per minute (0 disables)
}
