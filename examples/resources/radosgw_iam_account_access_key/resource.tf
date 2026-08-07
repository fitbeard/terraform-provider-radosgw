resource "radosgw_iam_account_user" "developer" {
  provider = radosgw.account_root
  name     = "developer"
}

# RadosGW generates the key pair; the secret is only available at create time.
resource "radosgw_iam_account_access_key" "developer" {
  provider = radosgw.account_root
  user     = radosgw_iam_account_user.developer.name
  status   = "active"
}

output "developer_access_key_id" {
  value = radosgw_iam_account_access_key.developer.access_key
}

output "developer_secret" {
  value     = radosgw_iam_account_access_key.developer.secret_key
  sensitive = true
}
