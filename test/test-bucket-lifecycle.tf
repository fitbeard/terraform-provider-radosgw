# =============================================================================
# Bucket Lifecycle Configuration Resource Tests
# =============================================================================
# Purpose: Test radosgw_s3_bucket_lifecycle_configuration resource
# Resources: 1 bucket, 1 lifecycle configuration with multiple rules
# Dependencies: None (standalone)
# =============================================================================

resource "radosgw_s3_bucket" "lifecycle_test" {
  bucket        = "lifecycle-test-bucket"
  force_destroy = true
}

# Lifecycle configuration with multiple rules
resource "radosgw_s3_bucket_lifecycle_configuration" "test" {
  bucket = radosgw_s3_bucket.lifecycle_test.bucket

  # Rule 1: Long-term archive expiration
  rule {
    id     = "archive-expiration"
    status = "Enabled"

    filter {
      prefix = "archive/"
    }

    expiration {
      days = 365
    }
  }

  # Rule 2: Temporary files cleanup
  rule {
    id     = "expire-temp-objects"
    status = "Enabled"

    filter {
      prefix = "temp/"
    }

    expiration {
      days = 30
    }
  }

  # Rule 3: Logs expiration with different prefix
  rule {
    id     = "expire-logs"
    status = "Enabled"

    filter {
      prefix = "logs/"
    }

    expiration {
      days = 90
    }
  }
}

# =============================================================================
# Outputs
# =============================================================================

output "lifecycle_bucket" {
  value = radosgw_s3_bucket_lifecycle_configuration.test.bucket
}

output "lifecycle_rules_count" {
  value = length(radosgw_s3_bucket_lifecycle_configuration.test.rule)
}
