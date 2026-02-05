# =============================================================================
# Subusers Data Source Tests
# =============================================================================
# Purpose: Test radosgw_iam_subusers data source
# Resources: 1 data source
# Dependencies: test-keys.tf (test_swift subuser), test-subuser.tf (additional subusers)
# =============================================================================

# Get all subusers for the shared test user
# Note: The shared test user has a subuser created in test-keys.tf
data "radosgw_iam_subusers" "test" {
  user_id = radosgw_iam_user.test.user_id

  depends_on = [radosgw_iam_subuser.test_swift]
}

# =============================================================================
# Outputs
# =============================================================================

output "subusers" {
  value = data.radosgw_iam_subusers.test.subusers
}

output "subusers_count" {
  value = length(data.radosgw_iam_subusers.test.subusers)
}

output "subuser_names" {
  value = [for s in data.radosgw_iam_subusers.test.subusers : s.name]
}

output "subuser_access_levels" {
  value = [for s in data.radosgw_iam_subusers.test.subusers : s.access]
}
