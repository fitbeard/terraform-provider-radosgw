# =============================================================================
# Account Data Source Tests
# =============================================================================
# Purpose: Test radosgw_iam_account data source for account lookup by ID
# Resources: 1 data source
# Dependencies: test-account.tf (radosgw_iam_account.basic)
# =============================================================================

data "radosgw_iam_account" "basic" {
  account_id = radosgw_iam_account.basic.account_id

  depends_on = [radosgw_iam_account.basic]
}

# =============================================================================
# Outputs - demonstrates all available attributes
# =============================================================================

output "account_data" {
  value = {
    account_id      = data.radosgw_iam_account.basic.account_id
    name            = data.radosgw_iam_account.basic.name
    email           = data.radosgw_iam_account.basic.email
    tenant          = data.radosgw_iam_account.basic.tenant
    max_users       = data.radosgw_iam_account.basic.max_users
    max_roles       = data.radosgw_iam_account.basic.max_roles
    max_groups      = data.radosgw_iam_account.basic.max_groups
    max_access_keys = data.radosgw_iam_account.basic.max_access_keys
    max_buckets     = data.radosgw_iam_account.basic.max_buckets
  }
}
