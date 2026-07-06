# =============================================================================
# Account Resource Tests
# =============================================================================
# Purpose: Test radosgw_iam_account resource and user<->account association
# Resources: 3 accounts, 2 users (1 account root, 1 account member)
# Dependencies: main.tf (provider configuration)
# Requires: Ceph Squid (19.x)+ and the `accounts=*` capability. Use 20.2.2+
#           (20.2.1 has a broken account read/delete admin op).
# =============================================================================

# Account with an auto-generated ID
resource "radosgw_iam_account" "basic" {
  name  = "terraform-basic-account"
  email = "basic-account@example.com"
}

# Account with an explicit ID and custom limits
resource "radosgw_iam_account" "with_limits" {
  account_id      = "RGW00000000000000042"
  name            = "terraform-limited-account"
  max_users       = 50
  max_roles       = 10
  max_groups      = 10
  max_buckets     = 200
  max_access_keys = 8
}

# Account with a root (admin) user and a regular member user.
# NOTE: an account root user's display_name must NOT contain spaces.
resource "radosgw_iam_account" "team" {
  name = "terraform-team-account"
}

resource "radosgw_iam_user" "team_root" {
  user_id      = "terraform-team-root"
  display_name = "TerraformTeamRoot"
  account_id   = radosgw_iam_account.team.account_id
  account_root = true
}

resource "radosgw_iam_user" "team_member" {
  user_id      = "terraform-team-member"
  display_name = "TerraformTeamMember" # account users' display_name cannot contain spaces
  account_id   = radosgw_iam_account.team.account_id
}

resource "radosgw_iam_account" "team2" {
  name = "terraform-team-account1"
  account_id = "RGW66666666666666666"
}

# =============================================================================
# Outputs
# =============================================================================

output "account_basic" {
  value = {
    account_id      = radosgw_iam_account.basic.account_id
    name            = radosgw_iam_account.basic.name
    email           = radosgw_iam_account.basic.email
    tenant          = radosgw_iam_account.basic.tenant
    max_users       = radosgw_iam_account.basic.max_users
    max_roles       = radosgw_iam_account.basic.max_roles
    max_groups      = radosgw_iam_account.basic.max_groups
    max_access_keys = radosgw_iam_account.basic.max_access_keys
    max_buckets     = radosgw_iam_account.basic.max_buckets
  }
}

output "account_with_limits_id" {
  value = radosgw_iam_account.with_limits.account_id
}

output "team_account_users" {
  value = {
    account_id  = radosgw_iam_account.team.account_id
    root_user   = radosgw_iam_user.team_root.user_id
    root_flag   = radosgw_iam_user.team_root.account_root
    member_user = radosgw_iam_user.team_member.user_id
    member_flag = radosgw_iam_user.team_member.account_root
  }
}
