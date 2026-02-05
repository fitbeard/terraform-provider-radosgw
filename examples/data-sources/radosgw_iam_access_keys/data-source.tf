# Get all access keys (S3 and Swift) for a user
data "radosgw_iam_access_keys" "all" {
  user_id = radosgw_iam_user.example.user_id
}

# Get only S3 access keys
data "radosgw_iam_access_keys" "s3_only" {
  user_id  = radosgw_iam_user.example.user_id
  key_type = "s3"
}

# Get only Swift access keys
data "radosgw_iam_access_keys" "swift_only" {
  user_id  = radosgw_iam_user.example.user_id
  key_type = "swift"
}

# Reference user resource
resource "radosgw_iam_user" "example" {
  user_id      = "example-user"
  display_name = "Example User"
}

# Output all access keys
output "all_access_keys" {
  description = "All access keys for the user"
  value       = data.radosgw_iam_access_keys.all.access_keys
}

# Output S3 access key IDs only
output "s3_access_key_ids" {
  description = "S3 access key IDs for the user"
  value       = [for key in data.radosgw_iam_access_keys.s3_only.access_keys : key.access_key_id]
}

# Output Swift access key IDs only
output "swift_access_key_ids" {
  description = "Swift access key IDs for the user"
  value       = [for key in data.radosgw_iam_access_keys.swift_only.access_keys : key.access_key_id]
}
