# Get capabilities for an existing user
data "radosgw_iam_user_caps" "admin" {
  user_id = "admin-user"
}

# Get capabilities for a user created with a resource
data "radosgw_iam_user_caps" "example" {
  user_id = radosgw_iam_user.example.user_id

  depends_on = [radosgw_iam_user_caps.example]
}

# Reference user and caps resources
resource "radosgw_iam_user" "example" {
  user_id      = "example-user"
  display_name = "Example User"
}

resource "radosgw_iam_user_caps" "example" {
  user_id = radosgw_iam_user.example.user_id

  caps = [
    {
      type = "buckets"
      perm = "write"
    },
    {
      type = "users"
      perm = "*"
    }
  ]
}

# Output all capabilities
output "user_caps" {
  description = "All capabilities for the user"
  value       = data.radosgw_iam_user_caps.example.caps
}

# Check if user has specific capability
output "has_users_cap" {
  description = "Whether user has 'users' capability"
  value       = length([for cap in data.radosgw_iam_user_caps.example.caps : cap if cap.type == "users"]) > 0
}
