# =============================================================================
# Account Data Source Test
# =============================================================================
# Purpose: Test radosgw_iam_account data source
# Resources: 1 data source
# Dependencies: test-account.tf
# =============================================================================

# Look up the account by ID
data "radosgw_iam_account" "test" {
  account_id = radosgw_iam_account.test.account_id
}

# =============================================================================
# Outputs
# =============================================================================

output "ds_account_id" {
  description = "Account ID from data source"
  value       = data.radosgw_iam_account.test.account_id
}

output "ds_account_name" {
  description = "Account name from data source"
  value       = data.radosgw_iam_account.test.name
}

output "ds_account_email" {
  description = "Account email from data source"
  value       = data.radosgw_iam_account.test.email
}

output "ds_account_tenant" {
  description = "Account tenant from data source"
  value       = data.radosgw_iam_account.test.tenant
}

output "ds_max_users" {
  description = "Max users from data source"
  value       = data.radosgw_iam_account.test.max_users
}

output "ds_max_buckets" {
  description = "Max buckets from data source"
  value       = data.radosgw_iam_account.test.max_buckets
}
