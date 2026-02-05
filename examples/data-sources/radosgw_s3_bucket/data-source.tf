# Get information about an existing bucket
data "radosgw_s3_bucket" "example" {
  bucket = "my-existing-bucket"
}

# Get information about a bucket created with a resource
data "radosgw_s3_bucket" "managed" {
  bucket = radosgw_s3_bucket.example.bucket

  depends_on = [radosgw_s3_bucket.example]
}

# Reference bucket resource
resource "radosgw_s3_bucket" "example" {
  bucket = "example-bucket"
}

# Output bucket details
output "bucket_info" {
  description = "Bucket information"
  value = {
    id            = data.radosgw_s3_bucket.managed.id
    owner         = data.radosgw_s3_bucket.managed.owner
    creation_time = data.radosgw_s3_bucket.managed.creation_time
    versioning    = data.radosgw_s3_bucket.managed.versioning
  }
}

# Output placement info
output "bucket_placement" {
  description = "Bucket placement configuration"
  value       = data.radosgw_s3_bucket.managed.explicit_placement
}

# Output quota info
output "bucket_quota" {
  description = "Bucket quota settings"
  value       = data.radosgw_s3_bucket.managed.bucket_quota
}
