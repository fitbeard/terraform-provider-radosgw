# =============================================================================
# OIDC Provider Resource Test - Google Example
# =============================================================================
# Purpose: Test radosgw_iam_openid_connect_provider with Google configuration
# Resources: 1 OIDC provider (Google)
# Dependencies: None (standalone)
# =============================================================================

# Google OIDC Provider - single client and thumbprint
resource "radosgw_iam_openid_connect_provider" "google" {
  url = "https://accounts.google.com"

  client_id_list = [
    "123456789012-abcdefghijklmnopqrstuvwxyz.apps.googleusercontent.com",
  ]

  thumbprint_list = [
    "1234567890abcdef1234567890abcdef12345678",
  ]
}

# =============================================================================
# Outputs
# =============================================================================

output "google_oidc_arn" {
  description = "ARN of the Google OIDC provider"
  value       = radosgw_iam_openid_connect_provider.google.arn
}
