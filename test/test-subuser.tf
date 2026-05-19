# =============================================================================
# Subuser Resource Tests
# =============================================================================
# Purpose: Test radosgw_iam_subuser resource with various access levels
# Resources: 2 users, 3 subusers (different access levels plus tenant user)
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

# -----------------------------------------------------------------------------
# Tenant user subuser using local user_id reference
# -----------------------------------------------------------------------------
resource "radosgw_iam_user" "subuser_tenant" {
  user_id      = "subuser-tenant-user"
  tenant       = "subusertenant"
  display_name = "Subuser Tenant Test User"
}

# References user_id, not id, to verify tenant users resolve correctly
resource "radosgw_iam_subuser" "tenant_swift_subuser" {
  user_id = radosgw_iam_user.subuser_tenant.user_id
  subuser = "swift"
  access  = "read-write"
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

output "tenant_swift_subuser" {
  value = {
    user_id    = radosgw_iam_user.subuser_tenant.user_id
    rgw_id     = radosgw_iam_user.subuser_tenant.id
    tenant     = radosgw_iam_user.subuser_tenant.tenant
    subuser_id = radosgw_iam_subuser.tenant_swift_subuser.id
    access     = radosgw_iam_subuser.tenant_swift_subuser.access
  }
}
