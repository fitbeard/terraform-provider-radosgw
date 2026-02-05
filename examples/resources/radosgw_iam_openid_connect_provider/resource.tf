# Create an OIDC provider for Google
resource "radosgw_iam_openid_connect_provider" "google" {
  url = "https://accounts.google.com"

  client_id_list = [
    "123456789012-abcdefghijklmnopqrstuvwxyz.apps.googleusercontent.com"
  ]

  thumbprint_list = [
    "1234567890abcdef1234567890abcdef12345678"
  ]

  # Set to false to prevent updates after creation
  allow_updates = false
}

# Create an OIDC provider for Keycloak
resource "radosgw_iam_openid_connect_provider" "keycloak" {
  url = "https://keycloak.example.com/auth/realms/myrealm"

  client_id_list = [
    "my-app-client",
    "another-app-client"
  ]

  thumbprint_list = [
    "abcdef1234567890abcdef1234567890abcdef12"
  ]

  # Allow updates to client_ids and thumbprints
  allow_updates = true
}

# Create an OIDC provider for AWS Cognito
resource "radosgw_iam_openid_connect_provider" "cognito" {
  url = "https://cognito-idp.us-east-1.amazonaws.com/us-east-1_XXXXXXXXX"

  client_id_list = [
    "1234567890abcdefghij"
  ]

  thumbprint_list = [
    "9e99a48a9960b14926bb7f3b02e22da2b0ab7280"
  ]
}
