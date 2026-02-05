# Lookup a user by user_id
data "radosgw_iam_user" "example" {
  user_id = "my-user"
}

# Lookup a user in a tenant
data "radosgw_iam_user" "tenant_user" {
  user_id = "my-tenant$my-user"
}

# Output user details
output "user_display_name" {
  description = "Display name of the user"
  value       = data.radosgw_iam_user.example.display_name
}

output "user_email" {
  description = "Email address of the user"
  value       = data.radosgw_iam_user.example.email
}

output "user_max_buckets" {
  description = "Maximum buckets the user can own"
  value       = data.radosgw_iam_user.example.max_buckets
}

output "user_suspended" {
  description = "Whether the user is suspended"
  value       = data.radosgw_iam_user.example.suspended
}

output "user_tenant" {
  description = "Tenant the user belongs to"
  value       = data.radosgw_iam_user.example.tenant
}

output "user_type" {
  description = "Type of the user"
  value       = data.radosgw_iam_user.example.type
}
