# Look up an existing RadosGW account by its account ID
data "radosgw_iam_account" "example" {
  account_id = "RGW00000000000000001"
}

output "account_name" {
  value = data.radosgw_iam_account.example.name
}

output "account_max_buckets" {
  value = data.radosgw_iam_account.example.max_buckets
}
