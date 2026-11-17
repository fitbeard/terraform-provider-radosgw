# =============================================================================
# S3 Bucket Public Access Block Resource Test
# =============================================================================
# Purpose: Test radosgw_s3_bucket_public_access_block (+ its data source)
# Resources: 1 bucket + 1 public-access-block configuration + 1 data source
# Dependencies: None (standalone)
#
# Support is per-setting (see the resource docs):
#   - block_public_acls / ignore_public_acls / block_public_policy: all releases
#   - restrict_public_buckets: Ceph 20 (Tentacle) or higher (added in 20.2.0);
#     rejected at plan time on older releases. The dev bootstrap runs Umbrella.
#
# Manual verification after `tofu apply`:
#   AWS_ACCESS_KEY_ID=admin AWS_SECRET_ACCESS_KEY=secretkey \
#     aws --endpoint-url http://127.0.0.1:7480 --region default \
#     s3api get-public-access-block --bucket test-public-access-block-bucket
#   -> PublicAccessBlockConfiguration has all four flags == true
# =============================================================================

resource "radosgw_s3_bucket" "pab" {
  bucket        = "test-public-access-block-bucket"
  force_destroy = true
}

resource "radosgw_s3_bucket_public_access_block" "pab" {
  bucket = radosgw_s3_bucket.pab.bucket

  block_public_acls   = true
  block_public_policy = true
  ignore_public_acls  = true

  # Requires Ceph 20 (Tentacle) or higher.
  restrict_public_buckets = true
}

data "radosgw_s3_bucket_public_access_block" "pab" {
  bucket = radosgw_s3_bucket_public_access_block.pab.bucket
}

output "pab_bucket" {
  value = radosgw_s3_bucket.pab.bucket
}

output "pab_settings" {
  value = {
    block_public_acls       = data.radosgw_s3_bucket_public_access_block.pab.block_public_acls
    block_public_policy     = data.radosgw_s3_bucket_public_access_block.pab.block_public_policy
    ignore_public_acls      = data.radosgw_s3_bucket_public_access_block.pab.ignore_public_acls
    restrict_public_buckets = data.radosgw_s3_bucket_public_access_block.pab.restrict_public_buckets
  }
}
