# Create an IAM account with an auto-generated account ID
resource "radosgw_iam_account" "example" {
  name  = "example-account"
  email = "account@example.com"
}

# Create an IAM account with a predefined account ID and email
resource "radosgw_iam_account" "admin" {
  account_id = "RGW00000000000000002"
  name       = "admin-account"
  email      = "admin@example.com"
}

# Create an IAM account with custom resource limits
resource "radosgw_iam_account" "restricted" {
  account_id      = "RGW00000000000000003"
  name            = "restricted-account"
  max_users       = 10
  max_roles       = 5
  max_access_keys = 20
  max_buckets     = 50
}
