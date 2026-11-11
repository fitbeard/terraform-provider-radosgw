# =============================================================================
# STS Caller Identity Data Source Test
# =============================================================================
# Purpose: Test radosgw_caller_identity data source (STS GetCallerIdentity)
# Resources: 1 data source
# Dependencies: None (standalone)
#
# Requires Ceph 21 (Umbrella). GetCallerIdentity is Umbrella-only, so the
# provider gates this data source on `ceph_version`: on an older release (set
# explicitly, or on a cluster you've pointed `ceph_version` at) it returns a
# clear "requires Ceph 21 (Umbrella)" error. The dev bootstrap runs Umbrella.
#
# Manual verification after `tofu apply`:
#   tofu output caller_identity
#   -> arn = "arn:aws:iam:::user/admin", user_id = "admin", account_id = ""
#
# Compare with the raw API:
#   AWS_ACCESS_KEY_ID=admin AWS_SECRET_ACCESS_KEY=secretkey \
#     aws --endpoint-url http://127.0.0.1:7480 --region default sts get-caller-identity
# =============================================================================

data "radosgw_caller_identity" "current" {}

# =============================================================================
# Outputs - demonstrates all available attributes
# =============================================================================

output "caller_identity" {
  description = "Identity the provider's credentials resolve to"
  value = {
    id         = data.radosgw_caller_identity.current.id
    account_id = data.radosgw_caller_identity.current.account_id
    arn        = data.radosgw_caller_identity.current.arn
    user_id    = data.radosgw_caller_identity.current.user_id
  }
}
