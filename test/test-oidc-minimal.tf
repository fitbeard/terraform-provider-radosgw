# =============================================================================
# OIDC Provider Resource Test - Keycloak Example
# =============================================================================
# Purpose: Test radosgw_iam_openid_connect_provider with multiple clients/thumbprints
# Resources: 1 OIDC provider (Keycloak)
# Dependencies: None (standalone)
# =============================================================================

# Keycloak OIDC Provider - multiple clients and thumbprints
resource "radosgw_iam_openid_connect_provider" "test_keycloak" {
  url = "https://localhost:8080/auth/realms/quickstart"

  # Multiple client IDs
  client_id_list = [
    "app-profile-jsp",
    "another-client",
  ]

  # Multiple thumbprints
  thumbprint_list = [
    "F7D7B3515DD0D319DD219A43A9EA727AD6065287",
    "F7D7B3515DD0D319DD219A43A9EA727AD6065288",
  ]

  # Allow in-place updates (Ceph >= 20.2.0)
  allow_updates = true
}

# =============================================================================
# Outputs
# =============================================================================

output "test_oidc_arn" {
  description = "ARN of the Keycloak OIDC provider"
  value       = radosgw_iam_openid_connect_provider.test_keycloak.arn
}
