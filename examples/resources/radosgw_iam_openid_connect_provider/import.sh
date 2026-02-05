# Import an OIDC provider by ARN
terraform import radosgw_iam_openid_connect_provider.google "arn:aws:iam:::oidc-provider/accounts.google.com"

# Import a Keycloak OIDC provider
terraform import radosgw_iam_openid_connect_provider.keycloak "arn:aws:iam:::oidc-provider/keycloak.example.com/auth/realms/myrealm"
