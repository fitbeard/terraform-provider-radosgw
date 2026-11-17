# Lock down a bucket: block and ignore all public access.
resource "radosgw_s3_bucket" "example" {
  bucket = "my-private-bucket"
}

resource "radosgw_s3_bucket_public_access_block" "example" {
  bucket = radosgw_s3_bucket.example.bucket

  block_public_acls   = true
  block_public_policy = true
  ignore_public_acls  = true

  # restrict_public_buckets requires Ceph 20 (Tentacle) or higher.
  restrict_public_buckets = true
}
