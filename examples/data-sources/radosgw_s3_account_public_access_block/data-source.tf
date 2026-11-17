# Look up the account-wide public access block for a RadosGW account.
# The provider must authenticate as the account's root user.
data "radosgw_s3_account_public_access_block" "example" {
  account_id = "RGW12345678901234567"
}

output "account_public_access_block" {
  description = "Account-wide public access block settings"
  value = {
    block_public_acls       = data.radosgw_s3_account_public_access_block.example.block_public_acls
    block_public_policy     = data.radosgw_s3_account_public_access_block.example.block_public_policy
    ignore_public_acls      = data.radosgw_s3_account_public_access_block.example.ignore_public_acls
    restrict_public_buckets = data.radosgw_s3_account_public_access_block.example.restrict_public_buckets
  }
}
