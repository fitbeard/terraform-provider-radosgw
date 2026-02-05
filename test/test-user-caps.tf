# =============================================================================
# User Capabilities Resource Tests
# =============================================================================
# Purpose: Test radosgw_iam_user_caps resource with various configurations
# Resources: 1 user, 1 user_caps
# Dependencies: None (standalone)
# =============================================================================

# User with capabilities attached
resource "radosgw_iam_user" "caps_test" {
  user_id      = "caps-test-user"
  display_name = "Capabilities Test User"
}

# User capabilities - multiple caps with different permission levels
resource "radosgw_iam_user_caps" "test" {
  user_id = radosgw_iam_user.caps_test.user_id

  caps = [
    {
      type = "buckets"
      perm = "write"
    },
    {
      type = "users"
      perm = "*"
    },
    {
      type = "metadata"
      perm = "read"
    }
  ]
}

output "caps_test_user_id" {
  value = radosgw_iam_user.caps_test.user_id
}
