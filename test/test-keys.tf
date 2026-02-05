# =============================================================================
# Access Key Resource Tests
# =============================================================================
# Purpose: Test radosgw_iam_access_key resource with various configurations
# Resources: 8 S3 keys, 1 subuser, 1 Swift key
# Dependencies: main.tf (radosgw_iam_user.test)
# =============================================================================

# -----------------------------------------------------------------------------
# S3 Keys - Auto-generated credentials
# -----------------------------------------------------------------------------

# Auto-generated S3 key (both access_key and secret_key generated)
resource "radosgw_iam_access_key" "test_s3_auto" {
  user_id = radosgw_iam_user.test.user_id
}

# Multiple auto-generated keys to test multi-key support
resource "radosgw_iam_access_key" "test_s3_auto2" {
  user_id = radosgw_iam_user.test.user_id
}

resource "radosgw_iam_access_key" "test_s3_auto3" {
  user_id = radosgw_iam_user.test.user_id
}

resource "radosgw_iam_access_key" "test_s3_auto4" {
  user_id = radosgw_iam_user.test.user_id
}

resource "radosgw_iam_access_key" "test_s3_auto5" {
  user_id = radosgw_iam_user.test.user_id
}

resource "radosgw_iam_access_key" "test_s3_auto6" {
  user_id = radosgw_iam_user.test.user_id
}

# -----------------------------------------------------------------------------
# S3 Keys - User-specified credentials
# -----------------------------------------------------------------------------

# S3 key with custom access_key and secret_key
resource "radosgw_iam_access_key" "test_s3_custom" {
  user_id    = radosgw_iam_user.test.user_id
  access_key = "CUSTOM_ACCESS_KEY_001"
  secret_key = "CustomSecretKey123456789012345678901234"
}

# Second custom key with different credentials
resource "radosgw_iam_access_key" "test_s3_custom2" {
  user_id    = radosgw_iam_user.test.user_id
  access_key = "CUSTOM_ACCESS_KEY_002"
  secret_key = "AnotherSecretKey98765432109876543210987"
}

# -----------------------------------------------------------------------------
# Swift Keys - Subuser-based authentication
# -----------------------------------------------------------------------------

# Create a subuser for Swift key testing
resource "radosgw_iam_subuser" "test_swift" {
  user_id = radosgw_iam_user.test.user_id
  subuser = "swiftuser"
  access  = "full-control"
}

# Swift key for the subuser with custom secret
resource "radosgw_iam_access_key" "test_swift" {
  user_id    = radosgw_iam_subuser.test_swift.user_id
  subuser    = radosgw_iam_subuser.test_swift.subuser
  key_type   = "swift"
  secret_key = "swift_secret_password_12345"
}

# =============================================================================
# Outputs
# =============================================================================

output "s3_auto_key" {
  value = {
    access_key = radosgw_iam_access_key.test_s3_auto.access_key
    generated  = radosgw_iam_access_key.test_s3_auto.generated
  }
  sensitive = true
}

output "s3_custom_key" {
  value = {
    access_key = radosgw_iam_access_key.test_s3_custom.access_key
    generated  = radosgw_iam_access_key.test_s3_custom.generated
  }
  sensitive = true
}

output "swift_key" {
  value = {
    access_key = radosgw_iam_access_key.test_swift.access_key
    key_type   = radosgw_iam_access_key.test_swift.key_type
    subuser    = radosgw_iam_access_key.test_swift.subuser
  }
  sensitive = true
}
