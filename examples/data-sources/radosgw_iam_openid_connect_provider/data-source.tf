# Lookup OIDC provider by ARN
data "radosgw_iam_openid_connect_provider" "by_arn" {
  arn = "arn:aws:iam:::oidc-provider/accounts.google.com"
}

# Lookup OIDC provider by URL (without protocol)
data "radosgw_iam_openid_connect_provider" "by_url" {
  url = "accounts.google.com"
}

# Lookup OIDC provider by URL (with https:// protocol)
data "radosgw_iam_openid_connect_provider" "by_url_https" {
  url = "https://keycloak.example.com/auth/realms/myrealm"
}

# Use data source to reference an existing OIDC provider in a trust policy
data "radosgw_iam_openid_connect_provider" "google" {
  url = "accounts.google.com"
}

data "radosgw_policy_document" "assume_role_with_oidc" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRoleWithWebIdentity"]

    principals {
      type        = "Federated"
      identifiers = [data.radosgw_iam_openid_connect_provider.google.arn]
    }

    condition {
      test     = "StringEquals"
      variable = "${data.radosgw_iam_openid_connect_provider.google.url}:aud"
      values   = tolist(data.radosgw_iam_openid_connect_provider.google.client_id_list)
    }
  }
}

resource "radosgw_role" "oidc_role" {
  name               = "OIDCFederatedRole"
  assume_role_policy = data.radosgw_policy_document.assume_role_with_oidc.json
}

# Output OIDC provider details
output "google_oidc_arn" {
  description = "ARN of the Google OIDC provider"
  value       = data.radosgw_iam_openid_connect_provider.google.arn
}

output "google_oidc_url" {
  description = "URL of the Google OIDC provider"
  value       = data.radosgw_iam_openid_connect_provider.google.url
}

output "google_oidc_client_ids" {
  description = "Client IDs registered with the Google OIDC provider"
  value       = data.radosgw_iam_openid_connect_provider.google.client_id_list
}

output "google_oidc_thumbprints" {
  description = "Certificate thumbprints for the Google OIDC provider"
  value       = data.radosgw_iam_openid_connect_provider.google.thumbprint_list
}
