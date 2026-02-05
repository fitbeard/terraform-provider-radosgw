# =============================================================================
# OIDC Provider Data Source Tests
# =============================================================================
# Purpose: Test radosgw_iam_openid_connect_provider data source lookups
# Resources: 3 data sources (lookup by ARN, by URL, by URL with https)
# Dependencies: test-oidc-provider.tf (google), test-oidc-minimal.tf (keycloak)
# =============================================================================

# -----------------------------------------------------------------------------
# Lookup by ARN (recommended for precise matching)
# -----------------------------------------------------------------------------
data "radosgw_iam_openid_connect_provider" "by_arn" {
  arn = radosgw_iam_openid_connect_provider.google.arn

  depends_on = [radosgw_iam_openid_connect_provider.google]
}

# -----------------------------------------------------------------------------
# Lookup by URL (without https:// prefix)
# -----------------------------------------------------------------------------
data "radosgw_iam_openid_connect_provider" "by_url" {
  url = "accounts.google.com"

  depends_on = [radosgw_iam_openid_connect_provider.google]
}

# -----------------------------------------------------------------------------
# Lookup by URL (with https:// prefix - both formats supported)
# -----------------------------------------------------------------------------
data "radosgw_iam_openid_connect_provider" "by_url_https" {
  url = "https://localhost:8080/auth/realms/quickstart"

  depends_on = [radosgw_iam_openid_connect_provider.test_keycloak]
}

# =============================================================================
# Outputs
# =============================================================================

output "oidc_data_by_arn" {
  description = "OIDC provider looked up by ARN"
  value = {
    arn             = data.radosgw_iam_openid_connect_provider.by_arn.arn
    url             = data.radosgw_iam_openid_connect_provider.by_arn.url
    client_id_list  = data.radosgw_iam_openid_connect_provider.by_arn.client_id_list
    thumbprint_list = data.radosgw_iam_openid_connect_provider.by_arn.thumbprint_list
  }
}

output "oidc_data_by_url" {
  description = "OIDC provider looked up by URL (without https)"
  value = {
    arn             = data.radosgw_iam_openid_connect_provider.by_url.arn
    url             = data.radosgw_iam_openid_connect_provider.by_url.url
    client_id_list  = data.radosgw_iam_openid_connect_provider.by_url.client_id_list
    thumbprint_list = data.radosgw_iam_openid_connect_provider.by_url.thumbprint_list
  }
}

output "oidc_data_by_url_https" {
  description = "OIDC provider looked up by URL (with https)"
  value = {
    arn             = data.radosgw_iam_openid_connect_provider.by_url_https.arn
    url             = data.radosgw_iam_openid_connect_provider.by_url_https.url
    client_id_list  = data.radosgw_iam_openid_connect_provider.by_url_https.client_id_list
    thumbprint_list = data.radosgw_iam_openid_connect_provider.by_url_https.thumbprint_list
  }
}
