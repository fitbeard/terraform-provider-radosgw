# Disable ACLs on a bucket (the bucket owner owns every object).
# Requires Ceph 21 (Umbrella).
resource "radosgw_s3_bucket" "example" {
  bucket = "my-ownership-bucket"
}

resource "radosgw_s3_bucket_ownership_controls" "example" {
  bucket = radosgw_s3_bucket.example.bucket

  rule {
    object_ownership = "BucketOwnerEnforced"
  }
}
