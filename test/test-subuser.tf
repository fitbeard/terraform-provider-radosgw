# =============================================================================
# Subuser Resource Tests
# =============================================================================
# Purpose: Test radosgw_iam_subuser resource with various access levels
# Resources: 1 user, 2 subusers (different access levels)
# Dependencies: None (standalone)
# =============================================================================

# Parent user for subuser testing
resource "radosgw_iam_user" "subuser_test" {
  user_id      = "subuser-test-user"
  display_name = "Subuser Test User"
}

# -----------------------------------------------------------------------------
# Swift subuser with read-write access
# -----------------------------------------------------------------------------
resource "radosgw_iam_subuser" "swift_subuser" {
  user_id = radosgw_iam_user.subuser_test.user_id
  subuser = "swift"
  access  = "read-write"
}

# -----------------------------------------------------------------------------
# Swift2 subuser with full-control access
# -----------------------------------------------------------------------------
resource "radosgw_iam_subuser" "swift2_subuser" {
  user_id = radosgw_iam_user.subuser_test.user_id
  subuser = "swift2"
  access  = "full-control"
}

# =============================================================================
# Outputs
# =============================================================================

output "swift_subuser_id" {
  value = radosgw_iam_subuser.swift_subuser.id
}

output "swift2_subuser_id" {
  value = radosgw_iam_subuser.swift2_subuser.id
}

output "swift_secret_key" {
  description = "Auto-generated Swift secret key"
  value       = radosgw_iam_subuser.swift_subuser.secret_key
  sensitive   = true
}
