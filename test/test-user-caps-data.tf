# =============================================================================
# User Capabilities Data Source Tests
# =============================================================================
# Purpose: Test radosgw_iam_user_caps data source
# Resources: 1 data source
# Dependencies: test-user-caps.tf (caps_test user and user_caps)
# =============================================================================

# Get capabilities for the test user
data "radosgw_iam_user_caps" "test" {
  user_id = radosgw_iam_user.caps_test.user_id

  depends_on = [radosgw_iam_user_caps.test]
}

output "user_caps" {
  value = data.radosgw_iam_user_caps.test.caps
}

output "user_caps_count" {
  value = length(data.radosgw_iam_user_caps.test.caps)
}

output "user_caps_types" {
  value = [for cap in data.radosgw_iam_user_caps.test.caps : cap.type]
}
