# =============================================================================
# User Data Source Tests
# =============================================================================
# Purpose: Test radosgw_iam_user data source for single user lookup
# Resources: 1 data source
# Dependencies: main.tf (radosgw_iam_user.test)
# =============================================================================

data "radosgw_iam_user" "test" {
  user_id = radosgw_iam_user.test.user_id

  depends_on = [radosgw_iam_user.test]
}

# =============================================================================
# Outputs - demonstrates all available attributes
# =============================================================================

output "user_data" {
  value = {
    user_id               = data.radosgw_iam_user.test.user_id
    display_name          = data.radosgw_iam_user.test.display_name
    email                 = data.radosgw_iam_user.test.email
    tenant                = data.radosgw_iam_user.test.tenant
    max_buckets           = data.radosgw_iam_user.test.max_buckets
    suspended             = data.radosgw_iam_user.test.suspended
    op_mask               = data.radosgw_iam_user.test.op_mask
    default_placement     = data.radosgw_iam_user.test.default_placement
    default_storage_class = data.radosgw_iam_user.test.default_storage_class
    type                  = data.radosgw_iam_user.test.type
  }
}
