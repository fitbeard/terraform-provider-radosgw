# Per-bucket CORS configuration
resource "radosgw_s3_bucket" "example" {
  bucket = "my-cors-bucket"
}

resource "radosgw_s3_bucket_cors_configuration" "example" {
  bucket = radosgw_s3_bucket.example.bucket

  # Allow a specific web application to read and write objects.
  cors_rule {
    id              = "allow-web-app"
    allowed_headers = ["*"]
    allowed_methods = ["GET", "PUT", "POST"]
    allowed_origins = ["https://app.example.com"]
    expose_headers  = ["ETag", "x-amz-request-id"]
    max_age_seconds = 3600
  }

  # Allow anonymous GETs from any origin (e.g. public assets).
  cors_rule {
    allowed_methods = ["GET"]
    allowed_origins = ["*"]
  }
}
