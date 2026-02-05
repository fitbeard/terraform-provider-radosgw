# Create an inline policy for a role using jsonencode
resource "radosgw_iam_role_policy" "s3_access" {
  role = radosgw_iam_role.example.name
  name = "S3AccessPolicy"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "AllowS3Access"
        Effect = "Allow"
        Action = [
          "s3:GetObject",
          "s3:PutObject",
          "s3:DeleteObject",
          "s3:ListBucket"
        ]
        Resource = [
          "arn:aws:s3:::my-bucket",
          "arn:aws:s3:::my-bucket/*"
        ]
      }
    ]
  })
}

# Create a policy using the policy document data source
resource "radosgw_iam_role_policy" "s3_readonly" {
  role   = radosgw_iam_role.example.name
  name   = "S3ReadOnlyPolicy"
  policy = data.radosgw_iam_policy_document.s3_readonly.json
}

# Multiple policies can be attached to the same role
resource "radosgw_iam_role_policy" "s3_list" {
  role = radosgw_iam_role.example.name
  name = "S3ListPolicy"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = "s3:ListAllMyBuckets"
        Resource = "*"
      }
    ]
  })
}

# Policy document data source
data "radosgw_iam_policy_document" "s3_readonly" {
  statement {
    sid     = "AllowS3ReadOnly"
    effect  = "Allow"
    actions = ["s3:GetObject", "s3:ListBucket"]
    resources = [
      "arn:aws:s3:::readonly-bucket",
      "arn:aws:s3:::readonly-bucket/*"
    ]
  }
}

# Reference role resource
resource "radosgw_iam_role" "example" {
  name = "ExampleRole"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Principal = {
          Federated = "arn:aws:iam:::oidc-provider/accounts.google.com"
        }
        Action = "sts:AssumeRoleWithWebIdentity"
      }
    ]
  })
}
