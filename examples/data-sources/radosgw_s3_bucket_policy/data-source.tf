# Get the policy attached to an existing bucket
data "radosgw_s3_bucket_policy" "example" {
  bucket = "my-bucket"
}

# Get the policy for a bucket managed by Terraform
data "radosgw_s3_bucket_policy" "managed" {
  bucket = radosgw_s3_bucket.example.bucket

  depends_on = [radosgw_s3_bucket_policy.example]
}

# Reference bucket and policy resources
resource "radosgw_s3_bucket" "example" {
  bucket = "example-bucket"
}

resource "radosgw_s3_bucket_policy" "example" {
  bucket = radosgw_s3_bucket.example.bucket

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect    = "Allow"
        Principal = "*"
        Action    = ["s3:GetObject"]
        Resource  = "arn:aws:s3:::${radosgw_s3_bucket.example.bucket}/*"
      }
    ]
  })
}

# Output the policy
output "bucket_policy" {
  description = "The bucket policy document"
  value       = data.radosgw_s3_bucket_policy.managed.policy
}
