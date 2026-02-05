# =============================================================================
# Users Data Source Tests
# =============================================================================
# Purpose: Test radosgw_iam_users data source for listing users
# Resources: 2 data sources (all users, filtered by regex)
# Dependencies: None (reads all existing users in cluster)
# =============================================================================

# Get all users in the cluster
data "radosgw_iam_users" "all" {}

# Filter users by name regex pattern
data "radosgw_iam_users" "filtered" {
  name_regex = ".*test.*"
}

# =============================================================================
# Outputs
# =============================================================================

output "all_users" {
  description = "List of all user IDs in the cluster"
  value       = data.radosgw_iam_users.all.user_ids
}

output "filtered_users" {
  description = "User IDs matching the regex pattern"
  value       = data.radosgw_iam_users.filtered.user_ids
}
