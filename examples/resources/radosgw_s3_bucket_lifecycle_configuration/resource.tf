# Basic lifecycle rule - expire objects after 90 days
resource "radosgw_s3_bucket" "example" {
  bucket = "my-lifecycle-bucket"
}

resource "radosgw_s3_bucket_lifecycle_configuration" "expire_old_objects" {
  bucket = radosgw_s3_bucket.example.bucket

  rule {
    id     = "expire-old-objects"
    status = "Enabled"

    expiration {
      days = 90
    }
  }
}

# Lifecycle with prefix filter - only apply to logs/
resource "radosgw_s3_bucket_lifecycle_configuration" "expire_logs" {
  bucket = radosgw_s3_bucket.example.bucket

  rule {
    id     = "expire-logs"
    status = "Enabled"

    filter {
      prefix = "logs/"
    }

    expiration {
      days = 30
    }
  }
}

# Multiple rules - different expiration for different prefixes
resource "radosgw_s3_bucket" "multi_rule" {
  bucket = "multi-rule-bucket"
}

resource "radosgw_s3_bucket_lifecycle_configuration" "multi_rule" {
  bucket = radosgw_s3_bucket.multi_rule.bucket

  rule {
    id     = "expire-temp"
    status = "Enabled"

    filter {
      prefix = "temp/"
    }

    expiration {
      days = 7
    }
  }

  rule {
    id     = "expire-archive"
    status = "Enabled"

    filter {
      prefix = "archive/"
    }

    expiration {
      days = 365
    }
  }
}

# Transition to different storage class
resource "radosgw_s3_bucket" "tiered" {
  bucket = "tiered-storage-bucket"
}

resource "radosgw_s3_bucket_lifecycle_configuration" "tiering" {
  bucket = radosgw_s3_bucket.tiered.bucket

  rule {
    id     = "move-to-cold-storage"
    status = "Enabled"

    transition {
      days          = 30
      storage_class = "COLD"
    }

    expiration {
      days = 365
    }
  }
}

# Versioned bucket - manage noncurrent versions
resource "radosgw_s3_bucket" "versioned" {
  bucket     = "versioned-bucket"
  versioning = "enabled"
}

resource "radosgw_s3_bucket_lifecycle_configuration" "noncurrent_cleanup" {
  bucket = radosgw_s3_bucket.versioned.bucket

  rule {
    id     = "cleanup-old-versions"
    status = "Enabled"

    noncurrent_version_expiration {
      noncurrent_days = 30
    }
  }
}

# Cleanup incomplete multipart uploads
resource "radosgw_s3_bucket_lifecycle_configuration" "abort_multipart" {
  bucket = radosgw_s3_bucket.example.bucket

  rule {
    id     = "abort-incomplete-uploads"
    status = "Enabled"

    abort_incomplete_multipart_upload {
      days_after_initiation = 7
    }
  }
}

# Filter by tag
resource "radosgw_s3_bucket_lifecycle_configuration" "tagged_expiration" {
  bucket = radosgw_s3_bucket.example.bucket

  rule {
    id     = "expire-temporary-tagged"
    status = "Enabled"

    filter {
      tag {
        key   = "Environment"
        value = "Development"
      }
    }

    expiration {
      days = 14
    }
  }
}

# Complex filter with AND condition
resource "radosgw_s3_bucket_lifecycle_configuration" "complex_filter" {
  bucket = radosgw_s3_bucket.example.bucket

  rule {
    id     = "complex-rule"
    status = "Enabled"

    filter {
      and {
        prefix = "data/"
        tags = {
          "Project" = "Analytics"
          "Tier"    = "Archive"
        }
      }
    }

    expiration {
      days = 180
    }
  }
}

# Disabled rule (for temporary suspension)
resource "radosgw_s3_bucket_lifecycle_configuration" "disabled_rule" {
  bucket = radosgw_s3_bucket.example.bucket

  rule {
    id     = "disabled-cleanup"
    status = "Disabled"

    expiration {
      days = 30
    }
  }
}
