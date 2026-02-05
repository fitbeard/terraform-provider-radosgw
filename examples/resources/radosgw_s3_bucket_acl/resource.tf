# Manage ACL for an existing bucket
resource "radosgw_s3_bucket" "example" {
  bucket = "my-example-bucket"
}

resource "radosgw_s3_bucket_acl" "example" {
  bucket = radosgw_s3_bucket.example.bucket
  acl    = "private"
}

# Set public-read ACL on a bucket
resource "radosgw_s3_bucket" "public" {
  bucket = "my-public-bucket"
}

resource "radosgw_s3_bucket_acl" "public" {
  bucket = radosgw_s3_bucket.public.bucket
  acl    = "public-read"
}

# Set public-read-write ACL on a bucket
resource "radosgw_s3_bucket" "public_rw" {
  bucket = "my-public-rw-bucket"
}

resource "radosgw_s3_bucket_acl" "public_rw" {
  bucket = radosgw_s3_bucket.public_rw.bucket
  acl    = "public-read-write"
}

# Set authenticated-read ACL on a bucket
resource "radosgw_s3_bucket" "auth_read" {
  bucket = "my-auth-read-bucket"
}

resource "radosgw_s3_bucket_acl" "auth_read" {
  bucket = radosgw_s3_bucket.auth_read.bucket
  acl    = "authenticated-read"
}
