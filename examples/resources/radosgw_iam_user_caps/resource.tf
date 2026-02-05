# Grant administrative capabilities to a user
resource "radosgw_iam_user_caps" "admin_caps" {
  user_id = radosgw_iam_user.example.user_id

  caps = [
    {
      type = "buckets"
      perm = "*"
    },
    {
      type = "users"
      perm = "*"
    }
  ]
}

# Grant read-only capabilities
resource "radosgw_iam_user_caps" "readonly_caps" {
  user_id = radosgw_iam_user.readonly.user_id

  caps = [
    {
      type = "buckets"
      perm = "read"
    },
    {
      type = "users"
      perm = "read"
    },
    {
      type = "usage"
      perm = "read"
    }
  ]
}

# Grant specific capabilities for bucket management
resource "radosgw_iam_user_caps" "bucket_admin" {
  user_id = radosgw_iam_user.bucket_admin.user_id

  caps = [
    {
      type = "buckets"
      perm = "*"
    },
    {
      type = "metadata"
      perm = "read"
    }
  ]
}

# Use perm = "*" for full access (read + write)
# Note: Each capability type can only appear once in the caps set
resource "radosgw_iam_user_caps" "full_access_caps" {
  user_id = radosgw_iam_user.example.user_id

  caps = [
    {
      type = "users"
      perm = "*" # Full access (equivalent to read + write)
    },
    {
      type = "usage"
      perm = "read" # Read-only access
    }
  ]
}

# Reference user resources
resource "radosgw_iam_user" "example" {
  user_id      = "caps-example-user"
  display_name = "Caps Example User"
}

resource "radosgw_iam_user" "readonly" {
  user_id      = "readonly-user"
  display_name = "Read-only User"
}

resource "radosgw_iam_user" "bucket_admin" {
  user_id      = "bucket-admin-user"
  display_name = "Bucket Admin User"
}
