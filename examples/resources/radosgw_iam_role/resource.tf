# Create a role with trust policy for OIDC provider
resource "radosgw_iam_role" "web_identity" {
  name        = "WebIdentityRole"
  path        = "/"
  description = "Role for web identity federation via Google OIDC"

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

  max_session_duration = 3600 # 1 hour
}

# Create a role using the policy document data source
resource "radosgw_iam_role" "service_role" {
  name                 = "ServiceRole"
  path                 = "/service-roles/"
  assume_role_policy   = data.radosgw_iam_policy_document.trust_policy.json
  max_session_duration = 7200 # 2 hours
}

# Trust policy using data source
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
      variable = "accounts.google.com:sub"
      values   = ["user@example.com"]
    }
  }
}
