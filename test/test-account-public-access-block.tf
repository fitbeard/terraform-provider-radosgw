# =============================================================================
# S3 Account Public Access Block (S3 Control) Resource Test
# =============================================================================
# Purpose: Test radosgw_s3_account_public_access_block (+ its data source).
# Resources: 1 account + 1 account-root user + 1 access key + an aliased provider
#   + 1 account public-access-block configuration + 1 data source.
#
# Account-wide Public Access Block is managed by the account's OWN root
# credentials (the admin user gets MethodNotAllowed), so this uses the
# two-provider pattern from test-account-alias.tf: the admin provider creates the
# account + root key, and an aliased provider authenticated as that root manages
# the public-access-block. The provider sends the account id in the
# x-amz-account-id header (no wildcard DNS needed).
#
# Requires Ceph 21 (Umbrella) — the S3 Control PublicAccessBlock API was added in
# 21.1.1 and is not in Squid/Tentacle, so the provider rejects this resource at
# plan time on older releases. The dev bootstrap runs Umbrella with s3control
# enabled in rgw_enable_apis.
#
# Manual verification after `tofu apply` (as the account root):
#   AWS_ACCESS_KEY_ID=<root ak> AWS_SECRET_ACCESS_KEY=<root sk> \
#     aws --endpoint-url http://127.0.0.1:7480 --region default \
#     s3control get-public-access-block --account-id <account id>
#   (CLI needs <account-id>.127.0.0.1 in /etc/hosts; the provider does not.)
# =============================================================================

resource "radosgw_iam_account" "pab_demo" {
  name = "pab-demo-account"
}

resource "radosgw_iam_user" "pab_demo_root" {
  user_id      = "pab-demo-root"
  display_name = "PabDemoRoot" # account root display_name must not contain spaces
  account_id   = radosgw_iam_account.pab_demo.account_id
  account_root = true
}

resource "radosgw_iam_access_key" "pab_demo_root" {
  user_id = radosgw_iam_user.pab_demo_root.user_id
}

# Provider authenticated as the account root (keys unknown until apply).
provider "radosgw" {
  alias      = "pab_account_root"
  endpoint   = "http://127.0.0.1:7480"
  access_key = radosgw_iam_access_key.pab_demo_root.access_key
  secret_key = radosgw_iam_access_key.pab_demo_root.secret_key
}

resource "radosgw_s3_account_public_access_block" "pab_demo" {
  provider = radosgw.pab_account_root

  account_id = radosgw_iam_account.pab_demo.account_id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true

  depends_on = [radosgw_iam_access_key.pab_demo_root]
}

data "radosgw_s3_account_public_access_block" "pab_demo" {
  provider   = radosgw.pab_account_root
  account_id = radosgw_s3_account_public_access_block.pab_demo.account_id
}

# =============================================================================
# Outputs
# =============================================================================

output "pab_demo_account_id" {
  value = radosgw_iam_account.pab_demo.account_id
}

output "pab_demo_settings" {
  value = {
    block_public_acls       = data.radosgw_s3_account_public_access_block.pab_demo.block_public_acls
    block_public_policy     = data.radosgw_s3_account_public_access_block.pab_demo.block_public_policy
    ignore_public_acls      = data.radosgw_s3_account_public_access_block.pab_demo.ignore_public_acls
    restrict_public_buckets = data.radosgw_s3_account_public_access_block.pab_demo.restrict_public_buckets
  }
}
