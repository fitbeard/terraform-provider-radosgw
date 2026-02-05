# Create an auto-generated S3 access key
resource "radosgw_iam_access_key" "auto_generated" {
  user_id = radosgw_iam_user.example.user_id
  # access_key and secret_key will be auto-generated
}

# Create an S3 access key with custom credentials
resource "radosgw_iam_access_key" "custom" {
  user_id    = radosgw_iam_user.example.user_id
  access_key = "MY_CUSTOM_ACCESS_KEY"
  secret_key = "MyCustomSecretKey123456789012345678901234"
}

# Create a Swift access key for a subuser
resource "radosgw_iam_access_key" "swift" {
  user_id    = radosgw_iam_user.example.user_id
  subuser    = radosgw_iam_subuser.swift.subuser
  key_type   = "swift"
  secret_key = "swift_secret_password"
}

# Reference resources
resource "radosgw_iam_user" "example" {
  user_id      = "key-example-user"
  display_name = "Key Example User"
}

resource "radosgw_iam_subuser" "swift" {
  user_id = radosgw_iam_user.example.user_id
  subuser = "swiftuser"
  access  = "full-control"
}

# Output the auto-generated keys
output "auto_access_key" {
  value     = radosgw_iam_access_key.auto_generated.access_key
  sensitive = true
}

output "auto_secret_key" {
  value     = radosgw_iam_access_key.auto_generated.secret_key
  sensitive = true
}
