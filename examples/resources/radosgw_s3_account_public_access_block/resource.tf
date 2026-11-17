# Block all public access for every bucket in a RadosGW account.
#
# Account-wide Public Access Block is managed by the account's own root
# credentials (the admin user cannot set it). Configure the provider — here an
# aliased provider — to authenticate as the account root, and set account_id to
# that same account.
provider "radosgw" {
  alias      = "account_root"
  endpoint   = "http://127.0.0.1:7480"
  access_key = var.account_root_access_key
  secret_key = var.account_root_secret_key
}

resource "radosgw_s3_account_public_access_block" "example" {
  provider = radosgw.account_root

  account_id = "RGW12345678901234567"

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}
