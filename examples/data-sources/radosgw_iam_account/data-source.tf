# Lookup an existing IAM account by its account ID
data "radosgw_iam_account" "existing" {
  account_id = "RGW00000000000000001"
}

# Output account details
output "account_name" {
  description = "Name of the account"
  value       = data.radosgw_iam_account.existing.name
}

output "account_email" {
  description = "Email of the account"
  value       = data.radosgw_iam_account.existing.email
}

output "account_tenant" {
  description = "Tenant of the account"
  value       = data.radosgw_iam_account.existing.tenant
}

output "account_max_users" {
  description = "Max users for the account"
  value       = radosgw_iam_account.account.max_users
}

output "account_max_roles" {
  description = "Max roles for the account"
  value       = radosgw_iam_account.account.max_roles
}

output "account_max_groups" {
  description = "Max groups for the account"
  value       = radosgw_iam_account.account.max_groups
}

output "account_max_access_keys" {
  description = "Max access keys for the account"
  value       = radosgw_iam_account.account.max_access_keys
}

output "account_max_buckets" {
  description = "Max buckets for the account"
  value       = radosgw_iam_account.account.max_buckets
}
