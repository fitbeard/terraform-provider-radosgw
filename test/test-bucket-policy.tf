# =============================================================================
# Bucket Policy Resource Tests
# =============================================================================
# Purpose: Test radosgw_s3_bucket_policy resource with policy document
# Resources: 1 user, 1 bucket, 1 policy document, 1 bucket policy
# Dependencies: None (standalone)
# =============================================================================

# Test user for policy principal
resource "radosgw_iam_user" "policy_test_user" {
  user_id      = "policy-test-user"
  display_name = "Policy Test User"
}

# Bucket for policy testing
resource "radosgw_s3_bucket" "policy_test" {
  bucket        = "policy-test-bucket"
  force_destroy = true
}

# -----------------------------------------------------------------------------
# Policy document using HCL DSL (recommended)
# -----------------------------------------------------------------------------
data "radosgw_iam_policy_document" "test_policy" {
  statement {
    sid    = "AllowUserAccess"
    effect = "Allow"

    principals {
      type        = "AWS"
      identifiers = ["arn:aws:iam:::user/${radosgw_iam_user.policy_test_user.user_id}"]
    }

    actions = [
      "s3:GetObject",
      "s3:PutObject",
      "s3:ListBucket",
    ]

    resources = [
      "arn:aws:s3:::${radosgw_s3_bucket.policy_test.bucket}",
      "arn:aws:s3:::${radosgw_s3_bucket.policy_test.bucket}/*",
    ]
  }
}

# Attach policy to bucket
resource "radosgw_s3_bucket_policy" "test" {
  bucket = radosgw_s3_bucket.policy_test.bucket
  policy = data.radosgw_iam_policy_document.test_policy.json
}

# =============================================================================
# Outputs
# =============================================================================

output "bucket_policy_id" {
  value = radosgw_s3_bucket_policy.test.id
}

output "bucket_policy_json" {
  value = radosgw_s3_bucket_policy.test.policy
}
