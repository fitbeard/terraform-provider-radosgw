# Look up the identity the provider's credentials resolve to (requires Ceph 21, Umbrella).
data "radosgw_caller_identity" "current" {}

output "caller_arn" {
  value = data.radosgw_caller_identity.current.arn
}

output "caller_user_id" {
  value = data.radosgw_caller_identity.current.user_id
}

output "caller_account_id" {
  value = data.radosgw_caller_identity.current.account_id
}
