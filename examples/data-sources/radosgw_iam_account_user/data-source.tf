data "radosgw_iam_account_user" "developer" {
  provider = radosgw.account_root
  name     = "developer"
}

output "developer_arn" {
  value = data.radosgw_iam_account_user.developer.arn
}
