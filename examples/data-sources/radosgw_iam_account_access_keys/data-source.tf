data "radosgw_iam_account_access_keys" "developer" {
  provider = radosgw.account_root
  user     = "developer"
}

output "developer_key_ids" {
  value = [for k in data.radosgw_iam_account_access_keys.developer.access_keys : k.access_key_id]
}
