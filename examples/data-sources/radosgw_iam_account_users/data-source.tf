# All IAM users under the /engineering/ path in the account.
data "radosgw_iam_account_users" "engineering" {
  provider    = radosgw.account_root
  path_prefix = "/engineering/"
}

output "engineering_user_names" {
  value = data.radosgw_iam_account_users.engineering.names
}
