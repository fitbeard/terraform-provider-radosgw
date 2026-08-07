# A second provider authenticated as the account root user (no admin caps).
provider "radosgw" {
  alias      = "account_root"
  endpoint   = "http://rgw.example.com:7480"
  access_key = "ACCOUNTROOTACCESSKEY"
  secret_key = "accountRootSecretKey"
}

# An IAM user inside the account, created via the IAM API (least privilege).
resource "radosgw_iam_account_user" "developer" {
  provider = radosgw.account_root
  name     = "developer"
  path     = "/engineering/"

  # Remove the user's access keys and policies when destroyed.
  force_destroy = true
}
