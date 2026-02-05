# =============================================================================
# User Without Keys Test
# =============================================================================
# Purpose: Test creating a user without any access keys (minimal configuration)
# Resources: 1 user
# Dependencies: None (standalone)
# =============================================================================

resource "radosgw_iam_user" "no_keys" {
  user_id      = "user-without-keys"
  display_name = "User Without Keys"
  email        = "nokeys@example.com"
  max_buckets  = 500
}

output "no_keys_user_id" {
  value = radosgw_iam_user.no_keys.user_id
}
