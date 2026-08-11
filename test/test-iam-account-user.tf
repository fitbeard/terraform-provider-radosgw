# =============================================================================
# IAM account user / access key
# =============================================================================
# Purpose: manage users and access keys INSIDE an account via the IAM API, as
#   the account root — no admin `users` cap required.
# Requires: Ceph 20.2.2+ (accounts) and the default provider's admin `accounts`
#   cap (only to bootstrap the account + root user below).
#
# The default (admin) provider creates the account, its root user, and the
# root's access key. A second provider, aliased as the account root, then manages
# the account's IAM users and their keys WITHOUT any admin capability.
# =============================================================================

resource "radosgw_iam_account" "iamdemo" {
  name = "iam-account-demo"
}

resource "radosgw_iam_user" "iamdemo_root" {
  user_id      = "iamdemo-root"
  display_name = "IamDemoRoot" # account root display_name must not contain spaces
  account_id   = radosgw_iam_account.iamdemo.account_id
  account_root = true
}

resource "radosgw_iam_access_key" "iamdemo_root" {
  user_id = radosgw_iam_user.iamdemo_root.user_id
  # auto-generated; fed to the aliased provider below (unknown until apply)
}

# Provider authenticated as the account root (no admin caps).
provider "radosgw" {
  alias      = "iam_root"
  endpoint   = "http://127.0.0.1:7480"
  access_key = radosgw_iam_access_key.iamdemo_root.access_key
  secret_key = radosgw_iam_access_key.iamdemo_root.secret_key
}

# --- managed by the account root, over the IAM API ---

resource "radosgw_iam_account_user" "developer" {
  provider      = radosgw.iam_root
  name          = "developer"
  path          = "/engineering/"
  force_destroy = true

  depends_on = [radosgw_iam_access_key.iamdemo_root]
}

resource "radosgw_iam_account_access_key" "developer" {
  provider = radosgw.iam_root
  user     = radosgw_iam_account_user.developer.name
  status   = "active"
}

# --- data sources (also over the IAM API, as the account root) ---

data "radosgw_iam_account_user" "developer" {
  provider   = radosgw.iam_root
  name       = radosgw_iam_account_user.developer.name
  depends_on = [radosgw_iam_account_user.developer]
}

data "radosgw_iam_account_users" "engineering" {
  provider    = radosgw.iam_root
  path_prefix = "/engineering/"
  depends_on  = [radosgw_iam_account_user.developer]
}

data "radosgw_iam_account_access_keys" "developer" {
  provider   = radosgw.iam_root
  user       = radosgw_iam_account_user.developer.name
  depends_on = [radosgw_iam_account_access_key.developer]
}

output "iam_developer_arn" {
  value = radosgw_iam_account_user.developer.arn
}

output "iam_developer_key_id" {
  value = radosgw_iam_account_access_key.developer.access_key
}

output "iam_engineering_users" {
  value = data.radosgw_iam_account_users.engineering.names
}

# --- a second (non-root) account user with TWO access keys ---
# Managed entirely by the account root over the IAM API. secret_key is only
# returned at creation and is sensitive, so the outputs below set sensitive = true.

resource "radosgw_iam_account_user" "analyst" {
  provider      = radosgw.iam_root
  name          = "analyst"
  path          = "/analytics/"
  force_destroy = true

  depends_on = [radosgw_iam_access_key.iamdemo_root]
}

resource "radosgw_iam_account_access_key" "analyst_primary" {
  provider = radosgw.iam_root
  user     = radosgw_iam_account_user.analyst.name
  status   = "active"
}

resource "radosgw_iam_account_access_key" "analyst_secondary" {
  provider = radosgw.iam_root
  user     = radosgw_iam_account_user.analyst.name
  status   = "inactive"
}

output "iam_analyst_key1_secret" {
  value     = radosgw_iam_account_access_key.analyst_primary.secret_key
  sensitive = true
}

output "iam_analyst_key2_secret" {
  value     = radosgw_iam_account_access_key.analyst_secondary.secret_key
  sensitive = true
}
