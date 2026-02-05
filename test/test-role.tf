# =============================================================================
# IAM Role Resource Tests
# =============================================================================
# Purpose: Test radosgw_iam_role and radosgw_iam_role_policy resources
# Resources: 2 roles, 3 role policies, 2 policy documents
# Dependencies: None (standalone)
# =============================================================================

# -----------------------------------------------------------------------------
# Policy Documents - Using HCL DSL (recommended approach)
# -----------------------------------------------------------------------------

# Trust policy using radosgw_iam_policy_document data source
data "radosgw_iam_policy_document" "trust_policy" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRoleWithWebIdentity"]

    principals {
      type        = "Federated"
      identifiers = ["arn:aws:iam:::oidc-provider/accounts.google.com"]
    }

    condition {
      test     = "StringEquals"
      variable = "accounts.google.com:aud"
      values   = ["my-client-id"]
    }
  }
}

# Permission policy using radosgw_iam_policy_document data source
data "radosgw_iam_policy_document" "s3_access" {
  statement {
    sid     = "AllowS3Access"
    effect  = "Allow"
    actions = ["s3:GetObject", "s3:PutObject", "s3:ListBucket", "s3:DeleteObject"]
    resources = [
      "arn:aws:s3:::my-bucket",
      "arn:aws:s3:::my-bucket/*"
    ]
  }
}

# -----------------------------------------------------------------------------
# Role 1: Using policy document data source
# -----------------------------------------------------------------------------

resource "radosgw_iam_role" "test_role" {
  name                 = "TestRole"
  path                 = "/"
  description          = "Test role using policy document data source"
  assume_role_policy   = data.radosgw_iam_policy_document.trust_policy.json
  max_session_duration = 10800 # 3 hours
}

# Inline policy using policy document data source
resource "radosgw_iam_role_policy" "s3_access_policy" {
  role   = radosgw_iam_role.test_role.name
  name   = "S3AccessPolicy"
  policy = data.radosgw_iam_policy_document.s3_access.json
}

# Second inline policy using jsonencode (demonstrates multiple policies on same role)
resource "radosgw_iam_role_policy" "s3_readonly_policy" {
  role = radosgw_iam_role.test_role.name
  name = "S3ReadOnlyPolicy"
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid      = "AllowS3ReadOnly"
        Effect   = "Allow"
        Action   = ["s3:GetObject", "s3:ListBucket"]
        Resource = "*"
      }
    ]
  })
}

# -----------------------------------------------------------------------------
# Role 2: Using inline jsonencode (alternative approach)
# -----------------------------------------------------------------------------

resource "radosgw_iam_role" "test_role_inline" {
  name = "TestRoleInline"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Principal = {
          Federated = "arn:aws:iam:::oidc-provider/accounts.google.com"
        }
        Action = "sts:AssumeRoleWithWebIdentity"
        Condition = {
          StringEquals = {
            "accounts.google.com:aud" = "my-client-id"
          }
        }
      }
    ]
  })
}

resource "radosgw_iam_role_policy" "inline_policy" {
  role = radosgw_iam_role.test_role_inline.name
  name = "InlineS3Policy"
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = "s3:*"
        Resource = "*"
      }
    ]
  })
}

# =============================================================================
# Outputs
# =============================================================================

output "test_role_arn" {
  value = radosgw_iam_role.test_role.arn
}

output "test_role_inline_arn" {
  value = radosgw_iam_role.test_role_inline.arn
}

output "trust_policy_json" {
  description = "Generated trust policy JSON from data source"
  value       = data.radosgw_iam_policy_document.trust_policy.json
}

output "s3_access_policy_json" {
  description = "Generated S3 access policy JSON from data source"
  value       = data.radosgw_iam_policy_document.s3_access.json
}
