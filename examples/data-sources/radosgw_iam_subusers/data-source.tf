# Get all subusers for a user
data "radosgw_iam_subusers" "example" {
  user_id = radosgw_iam_user.example.user_id

  depends_on = [radosgw_iam_subuser.swift]
}

# Reference user and subuser resources
resource "radosgw_iam_user" "example" {
  user_id      = "example-user"
  display_name = "Example User"
}

resource "radosgw_iam_subuser" "swift" {
  user_id = radosgw_iam_user.example.user_id
  subuser = "swift"
  access  = "full-control"
}

# Output all subusers
output "subusers" {
  description = "All subusers for the user"
  value       = data.radosgw_iam_subusers.example.subusers
}

# Output subuser names only
output "subuser_names" {
  description = "Names of all subusers"
  value       = [for s in data.radosgw_iam_subusers.example.subusers : s.name]
}

# Output subuser count
output "subuser_count" {
  description = "Number of subusers"
  value       = length(data.radosgw_iam_subusers.example.subusers)
}
