resource "radosgw_iam_account_group" "engineering" {
  provider = radosgw.account_root
  name     = "engineering"
  path     = "/teams/"
}
