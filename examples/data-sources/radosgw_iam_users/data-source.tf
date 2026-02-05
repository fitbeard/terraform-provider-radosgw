# Get all users
data "radosgw_iam_users" "all" {}

# Filter users by regex pattern
data "radosgw_iam_users" "test_users" {
  name_regex = "^test-.*"
}

# Filter users containing specific string
data "radosgw_iam_users" "admin_users" {
  name_regex = ".*admin.*"
}

# Output all user IDs
output "all_user_ids" {
  description = "All RadosGW user IDs"
  value       = data.radosgw_iam_users.all.user_ids
}

# Output filtered user IDs
output "test_user_ids" {
  description = "User IDs matching test-* pattern"
  value       = data.radosgw_iam_users.test_users.user_ids
}
