# =============================================================================
# Account + provider-alias pattern
# =============================================================================
# Purpose: demonstrate the two-provider pattern for RGW accounts.
#   - The default (admin) provider creates the account, its root user, and an
#     access key for that root user (admin-ops resources need admin caps).
#   - A second provider, aliased as the account root, then manages resources
#     INSIDE the account (bucket over S3, role over IAM) — the root has no admin
#     caps, so this exercises the S3 fallback and the IAM (account-scoped) path.
# Requires: Ceph 20.2.2+ (account read/delete admin ops) and accounts=* on the
#   admin user. See test-account.tf for account resource notes.
#
# Note on the alias provider: its credentials are the RadosGW-generated keys of
# the radosgw_iam_access_key below, referenced directly. Those values are unknown
# until apply, and the provider defers client creation until they are known
# — so no hard-coded keys are needed. The in-account resources
# depend_on the key so it exists before the alias provider is used.
# =============================================================================

resource "radosgw_iam_account" "alias_demo" {
  name = "alias-demo-account"
}

resource "radosgw_iam_user" "alias_demo_root" {
  user_id      = "alias-demo-root"
  display_name = "AliasDemoRoot" # account root display_name must not contain spaces
  account_id   = radosgw_iam_account.alias_demo.account_id
  account_root = true
}

# RadosGW auto-generates the key pair (unknown until apply).
resource "radosgw_iam_access_key" "alias_demo_root" {
  user_id = radosgw_iam_user.alias_demo_root.user_id
}

# Second provider, authenticated as the account root user. Its credentials are
# the generated keys above — unknown at plan time, resolved at apply.
provider "radosgw" {
  alias      = "account_root"
  endpoint   = "http://127.0.0.1:7480"
  access_key = radosgw_iam_access_key.alias_demo_root.access_key
  secret_key = radosgw_iam_access_key.alias_demo_root.secret_key
}

# Bucket created inside the account by the account root (over S3; admin-only
# metadata like num_shards/id will be null since the root has no admin caps).
resource "radosgw_s3_bucket" "alias_demo_bucket" {
  provider      = radosgw.account_root
  bucket        = "alias-demo-account-bucket"
  force_destroy = true

  depends_on = [radosgw_iam_access_key.alias_demo_root]
}

# Role created inside the account by the account root (over the IAM API).
resource "radosgw_iam_role" "alias_demo_role" {
  provider = radosgw.account_root
  name     = "alias-demo-account-role"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { AWS = "*" }
      Action    = "sts:AssumeRole"
    }]
  })

  depends_on = [radosgw_iam_access_key.alias_demo_root]
}

# =============================================================================
# Outputs
# =============================================================================

output "alias_demo_account_id" {
  value = radosgw_iam_account.alias_demo.account_id
}

output "alias_demo_bucket" {
  value = radosgw_s3_bucket.alias_demo_bucket.bucket
}

output "alias_demo_role_arn" {
  value = radosgw_iam_role.alias_demo_role.arn
}
