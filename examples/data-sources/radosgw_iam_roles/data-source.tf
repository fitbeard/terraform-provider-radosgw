# Get all roles
data "radosgw_iam_roles" "all" {}

# Filter roles by path prefix
data "radosgw_iam_roles" "app_roles" {
  path_prefix = "/application/"
}

# Filter roles by regex pattern
data "radosgw_iam_roles" "test_roles" {
  name_regex = "^Test.*"
}

# Output all role names and ARNs
output "all_role_names" {
  description = "All RadosGW role names"
  value       = data.radosgw_iam_roles.all.names
}

output "all_role_arns" {
  description = "All RadosGW role ARNs"
  value       = data.radosgw_iam_roles.all.arns
}

# Output filtered role names
output "test_role_names" {
  description = "Role names matching Test* pattern"
  value       = data.radosgw_iam_roles.test_roles.names
}
