# =============================================================================
# Account Resource Test
# =============================================================================
# Purpose: Test radosgw_iam_account resource with basic configuration
# Resources: 2 IAM accounts
# Dependencies: None (standalone)
# =============================================================================

# Basic account with auto-generated ID
resource "radosgw_iam_account" "test" {
  name  = "terraform-test-account"
  email = "terraform-test@example.com"
}

# Account with max limits
resource "radosgw_iam_account" "limited" {
  account_id      = "RGW00000000000000001"
  name            = "limited-account"
  max_users       = 10
  max_roles       = 5
  max_access_keys = 20
  max_buckets     = 50
}

# =============================================================================
# Outputs
# =============================================================================

output "test_account_id" {
  description = "Auto-generated account ID"
  value       = radosgw_iam_account.test.account_id
}

output "test_account_name" {
  description = "Account name"
  value       = radosgw_iam_account.test.name
}

output "limited_account_id" {
  description = "Predefined account ID"
  value       = radosgw_iam_account.limited.account_id
}

output "limited_max_users" {
  description = "Max users for limited account"
  value       = radosgw_iam_account.limited.max_users
}

output "limited_max_buckets" {
  description = "Max buckets for limited account"
  value       = radosgw_iam_account.limited.max_buckets
}
