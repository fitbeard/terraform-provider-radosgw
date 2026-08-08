# =============================================================================
# Account IAM policies & groups
# =============================================================================
# Purpose: manage inline/managed policies, groups, group membership and group
#   policies INSIDE an account via the IAM API — as the account root, no admin
#   caps. Reuses the account + root user + key bootstrapped in
#   test-iam-account-user.tf (radosgw.iam_root provider alias + the "developer"
#   user), so apply this together with that file.
# =============================================================================

# Inline policy on the developer user, built with the policy-document data source.
data "radosgw_iam_policy_document" "dev_s3_read" {
  provider = radosgw.iam_root
  statement {
    effect    = "Allow"
    actions   = ["s3:ListBucket", "s3:GetObject"]
    resources = ["*"]
  }
}

resource "radosgw_iam_account_user_policy" "developer_read" {
  provider = radosgw.iam_root
  user     = radosgw_iam_account_user.developer.name
  name     = "s3-read"
  policy   = data.radosgw_iam_policy_document.dev_s3_read.json
}

# Managed (AWS predefined) policy attached to the developer user.
resource "radosgw_iam_account_user_policy_attachment" "developer_ro" {
  provider   = radosgw.iam_root
  user       = radosgw_iam_account_user.developer.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess"
}

# A group, its membership (developer), an inline policy, and a managed policy.
resource "radosgw_iam_account_group" "engineering" {
  provider = radosgw.iam_root
  name     = "engineering"
  path     = "/eng/"
}

resource "radosgw_iam_account_group_membership" "engineering" {
  provider = radosgw.iam_root
  group    = radosgw_iam_account_group.engineering.name
  users    = [radosgw_iam_account_user.developer.name]
}

resource "radosgw_iam_account_group_policy" "engineering_list" {
  provider = radosgw.iam_root
  group    = radosgw_iam_account_group.engineering.name
  name     = "s3-list"
  policy   = data.radosgw_iam_policy_document.dev_s3_read.json
}

resource "radosgw_iam_account_group_policy_attachment" "engineering_full" {
  provider   = radosgw.iam_root
  group      = radosgw_iam_account_group.engineering.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonS3FullAccess"
}

data "radosgw_iam_account_group" "engineering" {
  provider   = radosgw.iam_root
  name       = radosgw_iam_account_group.engineering.name
  depends_on = [radosgw_iam_account_group_membership.engineering]
}

output "iam_engineering_group_members" {
  value = data.radosgw_iam_account_group.engineering.users
}
