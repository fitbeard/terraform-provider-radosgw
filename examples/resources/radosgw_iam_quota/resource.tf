# User quota - limits total storage across ALL buckets owned by the user
resource "radosgw_iam_quota" "user_quota" {
  user_id     = radosgw_iam_user.example.user_id
  type        = "user"
  enabled     = true
  max_size    = 10737418240 # 10 GB in bytes
  max_objects = 10000
}

# Bucket quota - per-bucket limit applied to EACH bucket the user owns
resource "radosgw_iam_quota" "bucket_quota" {
  user_id     = radosgw_iam_user.example.user_id
  type        = "bucket"
  enabled     = true
  max_size    = 5368709120 # 5 GB in bytes
  max_objects = 5000
}

# Disabled quota (unlimited)
resource "radosgw_iam_quota" "unlimited" {
  user_id     = radosgw_iam_user.example.user_id
  type        = "user"
  enabled     = false
  max_size    = -1 # unlimited
  max_objects = -1 # unlimited
}

# Reference user resource
resource "radosgw_iam_user" "example" {
  user_id      = "quota-example-user"
  display_name = "Quota Example User"
}
