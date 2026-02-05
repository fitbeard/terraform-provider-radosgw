# Basic policy document for S3 access
data "radosgw_iam_policy_document" "s3_access" {
  statement {
    sid    = "AllowS3Access"
    effect = "Allow"

    actions = [
      "s3:GetObject",
      "s3:PutObject",
      "s3:DeleteObject",
      "s3:ListBucket"
    ]

    resources = [
      "arn:aws:s3:::my-bucket",
      "arn:aws:s3:::my-bucket/*"
    ]
  }
}

# Trust policy for OIDC provider with conditions
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

    condition {
      test     = "StringEquals"
      variable = "accounts.google.com:sub"
      values   = ["user@example.com"]
    }
  }
}

# Policy document with multiple statements
data "radosgw_iam_policy_document" "multi_statement" {
  statement {
    sid     = "AllowReadAccess"
    effect  = "Allow"
    actions = ["s3:GetObject", "s3:ListBucket"]
    resources = [
      "arn:aws:s3:::public-bucket",
      "arn:aws:s3:::public-bucket/*"
    ]
  }

  statement {
    sid     = "AllowWriteAccess"
    effect  = "Allow"
    actions = ["s3:PutObject"]
    resources = [
      "arn:aws:s3:::upload-bucket/*"
    ]
  }

  statement {
    sid       = "DenyDeleteAccess"
    effect    = "Deny"
    actions   = ["s3:DeleteObject"]
    resources = ["*"]
  }
}

# Use the policy documents with roles
resource "radosgw_role" "example" {
  name               = "ExampleRole"
  assume_role_policy = data.radosgw_iam_policy_document.trust_policy.json
}

resource "radosgw_role_policy" "example" {
  role   = radosgw_role.example.name
  name   = "S3AccessPolicy"
  policy = data.radosgw_iam_policy_document.s3_access.json
}

# Bucket policy example - public read access
data "radosgw_iam_policy_document" "public_bucket" {
  statement {
    sid    = "PublicReadAccess"
    effect = "Allow"

    principals {
      type        = "*"
      identifiers = ["*"]
    }

    actions   = ["s3:GetObject"]
    resources = ["arn:aws:s3:::my-public-bucket/*"]
  }
}

# Use with bucket_policy resource
resource "radosgw_s3_bucket" "public" {
  bucket = "my-public-bucket"
}

resource "radosgw_s3_bucket_policy" "public" {
  bucket = radosgw_s3_bucket.public.bucket
  policy = data.radosgw_iam_policy_document.public_bucket.json
}

# Output the generated JSON
output "s3_policy_json" {
  value = data.radosgw_iam_policy_document.s3_access.json
}

output "trust_policy_json" {
  value = data.radosgw_iam_policy_document.trust_policy.json
}

output "bucket_policy_json" {
  value = data.radosgw_iam_policy_document.public_bucket.json
}
