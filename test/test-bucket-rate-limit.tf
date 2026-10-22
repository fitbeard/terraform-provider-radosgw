# =============================================================================
# S3 Bucket Rate Limit Resource + Data Source Tests
# =============================================================================
# Purpose: Test radosgw_s3_bucket_rate_limit resource and data source
# Resources: 1 bucket + 1 rate limit; 1 rate-limit data source
# Dependencies: None (standalone)
#
# Requires the Admin Ops `ratelimit` capability on the provider user (the dev
# bootstrap grants `ratelimit=*`). On an existing cluster add it with:
#   radosgw-admin caps add --uid=admin --caps="ratelimit=*"
#
# NOTE: max_list_ops / max_delete_ops are Ceph 21+ and are intentionally NOT
# managed here — the latest stable release is Tentacle (20). Only the four core
# limits apply on Reef/Squid/Tentacle.
#
# Manual verification after `tofu apply` — read the limit back over the Admin API:
#   radosgw-admin --conf /tmp/ceph-dev/ceph.conf ratelimit get \
#     --ratelimit-scope=bucket --bucket=test-rate-limit-bucket
#   -> should report enabled=true with the max_* values set below.
#
# Or via this provider's data source:
#   tofu output rate_limit
# =============================================================================

resource "radosgw_s3_bucket" "rate_limit" {
  bucket        = "test-rate-limit-bucket"
  force_destroy = true
}

resource "radosgw_s3_bucket_rate_limit" "rate_limit" {
  bucket = radosgw_s3_bucket.rate_limit.bucket

  enabled         = true
  max_read_ops    = 1000    # read operations per minute (0 disables)
  max_write_ops   = 500     # write operations per minute (0 disables)
  max_read_bytes  = 1048576 # 1 MiB read bytes per minute (0 disables)
  max_write_bytes = 524288  # 512 KiB write bytes per minute (0 disables)
}

# Read the rate limit back through the data source (depends on the resource so
# the apply order is deterministic).
data "radosgw_s3_bucket_rate_limit" "rate_limit" {
  bucket     = radosgw_s3_bucket_rate_limit.rate_limit.bucket
  depends_on = [radosgw_s3_bucket_rate_limit.rate_limit]
}

output "rate_limit_bucket" {
  value = radosgw_s3_bucket.rate_limit.bucket
}

output "rate_limit" {
  description = "Rate limit read back via the data source"
  value = {
    enabled         = data.radosgw_s3_bucket_rate_limit.rate_limit.enabled
    max_read_ops    = data.radosgw_s3_bucket_rate_limit.rate_limit.max_read_ops
    max_write_ops   = data.radosgw_s3_bucket_rate_limit.rate_limit.max_write_ops
    max_read_bytes  = data.radosgw_s3_bucket_rate_limit.rate_limit.max_read_bytes
    max_write_bytes = data.radosgw_s3_bucket_rate_limit.rate_limit.max_write_bytes
  }
}
