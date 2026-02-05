# =============================================================================
# Role Data Source Tests
# =============================================================================
# Purpose: Test radosgw_iam_role data source for single role lookup
# Resources: 1 data source
# Dependencies: test-role.tf (radosgw_iam_role.test_role)
# =============================================================================

data "radosgw_iam_role" "test" {
  name = radosgw_iam_role.test_role.name

  depends_on = [radosgw_iam_role.test_role]
}

# =============================================================================
# Outputs - demonstrates all available attributes
# =============================================================================

output "role_data" {
  value = {
    name                 = data.radosgw_iam_role.test.name
    arn                  = data.radosgw_iam_role.test.arn
    path                 = data.radosgw_iam_role.test.path
    description          = data.radosgw_iam_role.test.description
    assume_role_policy   = data.radosgw_iam_role.test.assume_role_policy
    max_session_duration = data.radosgw_iam_role.test.max_session_duration
    create_date          = data.radosgw_iam_role.test.create_date
    unique_id            = data.radosgw_iam_role.test.unique_id
  }
}
