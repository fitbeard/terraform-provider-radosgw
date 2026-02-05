# =============================================================================
# Roles Data Source Tests
# =============================================================================
# Purpose: Test radosgw_iam_roles data source for listing roles
# Resources: 2 data sources (all roles, filtered by regex)
# Dependencies: test-role.tf (creates roles to list)
# =============================================================================

# Get all roles in the cluster
data "radosgw_iam_roles" "all" {
  depends_on = [radosgw_iam_role.test_role, radosgw_iam_role.test_role_inline]
}

# Filter roles by name regex pattern
data "radosgw_iam_roles" "filtered" {
  name_regex = "^Test.*"

  depends_on = [radosgw_iam_role.test_role, radosgw_iam_role.test_role_inline]
}

# =============================================================================
# Outputs
# =============================================================================

output "all_roles_names" {
  description = "Names of all roles"
  value       = data.radosgw_iam_roles.all.names
}

output "all_roles_arns" {
  description = "ARNs of all roles"
  value       = data.radosgw_iam_roles.all.arns
}

output "filtered_roles_names" {
  description = "Names of roles matching the regex pattern"
  value       = data.radosgw_iam_roles.filtered.names
}
