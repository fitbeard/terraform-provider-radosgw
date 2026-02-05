# Lookup a role by name
data "radosgw_iam_role" "example" {
  name = "MyExampleRole"
}

# Output role details
output "role_arn" {
  description = "ARN of the role"
  value       = data.radosgw_iam_role.example.arn
}

output "role_assume_policy" {
  description = "Trust policy document for the role"
  value       = data.radosgw_iam_role.example.assume_role_policy
}

output "role_max_session_duration" {
  description = "Maximum session duration for the role"
  value       = data.radosgw_iam_role.example.max_session_duration
}

output "role_create_date" {
  description = "Date the role was created"
  value       = data.radosgw_iam_role.example.create_date
}
