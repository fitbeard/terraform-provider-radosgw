# =============================================================================
# S3 Bucket Website Configuration Resource Tests
# =============================================================================
# Purpose: Test radosgw_s3_bucket_website_configuration resource
# Resources: 1 bucket + 1 website configuration (index document + routing rule)
# Dependencies: None (standalone)
# Website URL: http://test-static-bucket.s3-website.storage.host:7480 (see devcontainer.json and bootstrap-ceph.sh for DNS and website setup)
# =============================================================================

# -----------------------------------------------------------------------------
# Static website bucket
# -----------------------------------------------------------------------------
resource "radosgw_s3_bucket" "static" {
  bucket = "test-static-bucket"
}

resource "radosgw_s3_bucket_acl" "static" {
  bucket = radosgw_s3_bucket.static.bucket
  acl    = "public-read"
}

# -----------------------------------------------------------------------------
# Website configuration — index document + routing rule
# -----------------------------------------------------------------------------
resource "radosgw_s3_bucket_website_configuration" "static" {
  bucket = radosgw_s3_bucket.static.bucket

  index_document {
    suffix = "index.html"
  }

  routing_rule {
    condition {
      http_error_code_returned_equals = 404
    }
    redirect {
      host_name          = "google.com"
      http_redirect_code = 301
      protocol           = "https"
      replace_key_with   = "/"
    }
  }
}

# =============================================================================
# Outputs
# =============================================================================

output "static_bucket_id" {
  description = "Static website bucket name"
  value       = radosgw_s3_bucket.static.id
}

output "static_website_index" {
  description = "Index document suffix"
  value       = radosgw_s3_bucket_website_configuration.static.index_document[0].suffix
}
