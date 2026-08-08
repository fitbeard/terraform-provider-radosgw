data "radosgw_iam_account_group" "engineering" {
  provider = radosgw.account_root
  name     = "engineering"
}

output "engineering_members" {
  value = data.radosgw_iam_account_group.engineering.users
}
