resource "radosgw_iam_account_group_policy_attachment" "full" {
  provider   = radosgw.account_root
  group      = radosgw_iam_account_group.engineering.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonS3FullAccess"
}
