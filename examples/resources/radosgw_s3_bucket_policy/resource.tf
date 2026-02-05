# Basic bucket policy using jsonencode
resource "radosgw_s3_bucket" "example" {
  bucket = "my-example-bucket"
}

resource "radosgw_s3_bucket_policy" "example" {
  bucket = radosgw_s3_bucket.example.bucket

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid       = "PublicReadGetObject"
        Effect    = "Allow"
        Principal = "*"
        Action    = "s3:GetObject"
        Resource  = "arn:aws:s3:::my-example-bucket/*"
      }
    ]
  })
}

# Bucket policy using the policy_document data source
resource "radosgw_s3_bucket" "data_bucket" {
  bucket = "my-data-bucket"
}

data "radosgw_iam_policy_document" "bucket_policy" {
  statement {
    sid    = "AllowUserAccess"
    effect = "Allow"

    principals {
      type        = "AWS"
      identifiers = ["arn:aws:iam:::user/myuser"]
    }

    actions = [
      "s3:GetObject",
      "s3:PutObject",
      "s3:DeleteObject",
    ]

    resources = [
      "arn:aws:s3:::my-data-bucket/*",
    ]
  }

  statement {
    sid    = "AllowListBucket"
    effect = "Allow"

    principals {
      type        = "AWS"
      identifiers = ["arn:aws:iam:::user/myuser"]
    }

    actions = [
      "s3:ListBucket",
    ]

    resources = [
      "arn:aws:s3:::my-data-bucket",
    ]
  }
}

resource "radosgw_s3_bucket_policy" "data_bucket" {
  bucket = radosgw_s3_bucket.data_bucket.bucket
  policy = data.radosgw_iam_policy_document.bucket_policy.json
}

# Deny policy example - deny all except specific user
resource "radosgw_s3_bucket" "restricted" {
  bucket = "restricted-bucket"
}

data "radosgw_iam_policy_document" "restricted" {
  statement {
    sid    = "DenyAllExceptAdmin"
    effect = "Deny"

    not_principals {
      type        = "AWS"
      identifiers = ["arn:aws:iam:::user/admin"]
    }

    actions = ["s3:*"]

    resources = [
      "arn:aws:s3:::restricted-bucket",
      "arn:aws:s3:::restricted-bucket/*",
    ]
  }
}

resource "radosgw_s3_bucket_policy" "restricted" {
  bucket = radosgw_s3_bucket.restricted.bucket
  policy = data.radosgw_iam_policy_document.restricted.json
}

# Policy with conditions
resource "radosgw_s3_bucket" "conditional" {
  bucket = "conditional-bucket"
}

data "radosgw_iam_policy_document" "conditional" {
  statement {
    sid    = "AllowFromSpecificIP"
    effect = "Allow"

    principals {
      type        = "*"
      identifiers = ["*"]
    }

    actions = [
      "s3:GetObject",
    ]

    resources = [
      "arn:aws:s3:::conditional-bucket/*",
    ]

    condition {
      test     = "IpAddress"
      variable = "aws:SourceIp"
      values   = ["192.168.1.0/24", "10.0.0.0/8"]
    }
  }
}

resource "radosgw_s3_bucket_policy" "conditional" {
  bucket = radosgw_s3_bucket.conditional.bucket
  policy = data.radosgw_iam_policy_document.conditional.json
}
