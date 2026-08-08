data "radosgw_iam_policy_document" "s3_read" {
  statement {
    effect    = "Allow"
    actions   = ["s3:ListBucket", "s3:GetObject"]
    resources = ["*"]
  }
}

# Grant an account user permissions with an inline policy.
resource "radosgw_iam_account_user_policy" "read" {
  provider = radosgw.account_root
  user     = radosgw_iam_account_user.developer.name
  name     = "s3-read"
  policy   = data.radosgw_iam_policy_document.s3_read.json
}
