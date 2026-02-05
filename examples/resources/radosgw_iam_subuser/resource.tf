# Create a Swift subuser with read-write access
resource "radosgw_iam_subuser" "swift" {
  user_id = radosgw_iam_user.example.user_id
  subuser = "swift"
  access  = "read-write"
}

# Create an S3 subuser with full control
resource "radosgw_iam_subuser" "s3_full" {
  user_id = radosgw_iam_user.example.user_id
  subuser = "s3admin"
  access  = "full-control"
}

# Create a read-only subuser
resource "radosgw_iam_subuser" "readonly" {
  user_id = radosgw_iam_user.example.user_id
  subuser = "reader"
  access  = "read"
}

# Reference user resource
resource "radosgw_iam_user" "example" {
  user_id      = "subuser-example"
  display_name = "Subuser Example User"
}

# Output the auto-generated secret key
output "swift_secret_key" {
  value     = radosgw_iam_subuser.swift.secret_key
  sensitive = true
}
