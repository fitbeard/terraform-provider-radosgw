# =============================================================================
# S3 Bucket Resource Tests
# =============================================================================
# Purpose: Test radosgw_s3_bucket resource with various configurations
# Resources: 2 buckets (basic, with quota and force_destroy)
# Dependencies: None (standalone)
# =============================================================================

# -----------------------------------------------------------------------------
# Basic bucket - minimal configuration
# -----------------------------------------------------------------------------
resource "radosgw_s3_bucket" "test_basic" {
  bucket = "test-basic-bucket"
}

# -----------------------------------------------------------------------------
# Bucket with force_destroy and quota
# -----------------------------------------------------------------------------
resource "radosgw_s3_bucket" "test_force_destroy" {
  bucket        = "test-force-destroy-bucket"
  force_destroy = true

  bucket_quota = {
    enabled     = true
    max_size    = 10737418240 # 10 GB in bytes
    max_objects = 10000
  }

  # Bucket tags. The declared set is authoritative — tags not listed here are
  # removed from the bucket.
  tags = {
    environment = "test"
    team        = "storage"
  }
}

# =============================================================================
# Outputs
# =============================================================================

output "basic_bucket_id" {
  value = radosgw_s3_bucket.test_basic.id
}

output "basic_bucket_owner" {
  value = radosgw_s3_bucket.test_basic.owner
}

output "basic_bucket_creation_time" {
  value = radosgw_s3_bucket.test_basic.creation_time
}

output "force_destroy_bucket_id" {
  value = radosgw_s3_bucket.test_force_destroy.id
}

output "force_destroy_bucket_quota" {
  value = radosgw_s3_bucket.test_force_destroy.bucket_quota
}

output "force_destroy_bucket_tags" {
  value = radosgw_s3_bucket.test_force_destroy.tags
}
