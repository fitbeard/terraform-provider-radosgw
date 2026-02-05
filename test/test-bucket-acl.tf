# =============================================================================
# Bucket ACL Resource Tests
# =============================================================================
# Purpose: Test radosgw_s3_bucket_acl resource with all supported ACL types
# Resources: 5 buckets, 5 bucket ACLs
# Dependencies: None (standalone)
# =============================================================================

# -----------------------------------------------------------------------------
# 1. Private ACL (default, most restrictive)
# -----------------------------------------------------------------------------
resource "radosgw_s3_bucket" "acl_test_private" {
  bucket        = "test-acl-private"
  force_destroy = true
}

resource "radosgw_s3_bucket_acl" "private" {
  bucket = radosgw_s3_bucket.acl_test_private.bucket
  acl    = "private"
}

# -----------------------------------------------------------------------------
# 2. Public-read ACL (anyone can read, only owner can write)
# -----------------------------------------------------------------------------
resource "radosgw_s3_bucket" "acl_test_public_read" {
  bucket        = "test-acl-public-read"
  force_destroy = true
}

resource "radosgw_s3_bucket_acl" "public_read" {
  bucket = radosgw_s3_bucket.acl_test_public_read.bucket
  acl    = "public-read"
}

# -----------------------------------------------------------------------------
# 3. Public-read-write ACL (anyone can read and write)
# -----------------------------------------------------------------------------
resource "radosgw_s3_bucket" "acl_test_public_rw" {
  bucket        = "test-acl-public-rw"
  force_destroy = true
}

resource "radosgw_s3_bucket_acl" "public_rw" {
  bucket = radosgw_s3_bucket.acl_test_public_rw.bucket
  acl    = "public-read-write"
}

# -----------------------------------------------------------------------------
# 4. Authenticated-read ACL (any authenticated user can read)
# -----------------------------------------------------------------------------
resource "radosgw_s3_bucket" "acl_test_auth_read" {
  bucket        = "test-acl-auth-read"
  force_destroy = true
}

resource "radosgw_s3_bucket_acl" "auth_read" {
  bucket = radosgw_s3_bucket.acl_test_auth_read.bucket
  acl    = "authenticated-read"
}

# -----------------------------------------------------------------------------
# 5. Second authenticated-read bucket (demonstrates multiple instances)
# Can be used for import testing:
#   terraform import radosgw_s3_bucket_acl.auth_read_alt test-acl-auth-read-alt
# -----------------------------------------------------------------------------
resource "radosgw_s3_bucket" "acl_test_auth_read_alt" {
  bucket        = "test-acl-auth-read-alt"
  force_destroy = true
}

resource "radosgw_s3_bucket_acl" "auth_read_alt" {
  bucket = radosgw_s3_bucket.acl_test_auth_read_alt.bucket
  acl    = "authenticated-read"
}

# =============================================================================
# Outputs
# =============================================================================

output "acl_private" {
  value = {
    bucket = radosgw_s3_bucket_acl.private.bucket
    acl    = radosgw_s3_bucket_acl.private.acl
    id     = radosgw_s3_bucket_acl.private.id
  }
}

output "acl_public_read" {
  value = {
    bucket = radosgw_s3_bucket_acl.public_read.bucket
    acl    = radosgw_s3_bucket_acl.public_read.acl
    id     = radosgw_s3_bucket_acl.public_read.id
  }
}

output "acl_public_rw" {
  value = {
    bucket = radosgw_s3_bucket_acl.public_rw.bucket
    acl    = radosgw_s3_bucket_acl.public_rw.acl
    id     = radosgw_s3_bucket_acl.public_rw.id
  }
}

output "acl_auth_read" {
  value = {
    bucket = radosgw_s3_bucket_acl.auth_read.bucket
    acl    = radosgw_s3_bucket_acl.auth_read.acl
    id     = radosgw_s3_bucket_acl.auth_read.id
  }
}

output "acl_auth_read_alt" {
  value = {
    bucket = radosgw_s3_bucket_acl.auth_read_alt.bucket
    acl    = radosgw_s3_bucket_acl.auth_read_alt.acl
    id     = radosgw_s3_bucket_acl.auth_read_alt.id
  }
}
