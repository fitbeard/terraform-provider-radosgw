# Create a basic RadosGW user
resource "radosgw_iam_user" "example" {
  user_id      = "example-user"
  display_name = "Example User"
  email        = "user@example.com"
  max_buckets  = 1000
}

# Create a user with custom settings
resource "radosgw_iam_user" "custom" {
  user_id           = "custom-user"
  display_name      = "Custom User"
  email             = "custom@example.com"
  tenant            = "my-tenant"
  max_buckets       = 500
  suspended         = false
  op_mask           = "read, write, delete"
  default_placement = "default-placement"
}

# Create a suspended user
resource "radosgw_iam_user" "suspended" {
  user_id      = "suspended-user"
  display_name = "Suspended User"
  suspended    = true
}
