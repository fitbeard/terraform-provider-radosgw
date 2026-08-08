data "radosgw_iam_policy_document" "list" {
  statement {
    effect    = "Allow"
    actions   = ["s3:ListBucket"]
    resources = ["*"]
  }
}

resource "radosgw_iam_account_group_policy" "list" {
  provider = radosgw.account_root
  group    = radosgw_iam_account_group.engineering.name
  name     = "s3-list"
  policy   = data.radosgw_iam_policy_document.list.json
}
