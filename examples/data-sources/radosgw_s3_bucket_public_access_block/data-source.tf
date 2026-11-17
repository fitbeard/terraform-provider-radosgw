# Look up the public access block configured for an existing bucket
data "radosgw_s3_bucket_public_access_block" "example" {
  bucket = "my-private-bucket"
}

output "bucket_public_access_block" {
  description = "Public access block settings for the bucket"
  value = {
    block_public_acls       = data.radosgw_s3_bucket_public_access_block.example.block_public_acls
    block_public_policy     = data.radosgw_s3_bucket_public_access_block.example.block_public_policy
    ignore_public_acls      = data.radosgw_s3_bucket_public_access_block.example.ignore_public_acls
    restrict_public_buckets = data.radosgw_s3_bucket_public_access_block.example.restrict_public_buckets
  }
}
