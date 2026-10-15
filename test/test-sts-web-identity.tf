# =============================================================================
# STS AssumeRoleWithWebIdentity fixture (OIDC federation)
# =============================================================================
# Purpose: register the local OIDC provider as an IdP and a role that federates
#   it, so a JWT minted from the provider can be exchanged for temporary RGW
#   credentials via STS AssumeRoleWithWebIdentity.
# Resources: 1 OIDC provider + 1 role (with a WebIdentity trust policy)
# Dependencies: the local OIDC provider (scripts/compose.yml)
#
# The acceptance test (provider/sts_web_identity_test.go) automates this end to
# end. To drive it by hand:
#
#   1. Start the IdP:            make oidc-up
#   2. Apply this fixture:       make tf-apply      (registers the provider + role)
#   3. Mint a token:             TOKEN=$(make -s oidc-token)
#   4. Exchange it for creds (unsigned — the JWT is the credential):
#        curl -s http://127.0.0.1:7480/ \
#          --data-urlencode Action=AssumeRoleWithWebIdentity \
#          --data-urlencode RoleArn=arn:aws:iam:::role/sts-web-identity \
#          --data-urlencode RoleSessionName=demo \
#          --data-urlencode WebIdentityToken="$TOKEN" \
#          --data-urlencode Version=2011-06-15
#
# Note: on a normal host/CI the issuer is http://localhost:9080; in a
# docker-outside-of-docker dev container use http://host.docker.internal:9080
# (that is what `make oidc-up` auto-detects). Set `sts_oidc_issuer` to match.
# =============================================================================

variable "sts_oidc_issuer" {
  type        = string
  description = "Issuer URL of the local OIDC provider (must match the minted token's iss)."
  default     = "http://localhost:9080"
}

resource "radosgw_iam_openid_connect_provider" "sts_idp" {
  url            = var.sts_oidc_issuer
  client_id_list = ["test-client"]

  # In-place updates of client_id_list/thumbprint_list are unsupported on
  # Reef/Squid; allow_updates = false forces replace-on-change instead.
  allow_updates = false

  # RadosGW only verifies the thumbprint against the IdP certificate when the
  # JWKS exposes x5c certificate chains; this provider's JWKS has none, so a
  # placeholder satisfies the IAM API (the JWT is still validated via the JWKS
  # public key). When the JWKS includes x5c, supply real SHA-1 thumbprints of
  # those certs (see fitbeard/radosgw-assume get_thumbprints.sh).
  thumbprint_list = ["0000000000000000000000000000000000000000"]
}

resource "radosgw_iam_role" "sts_web_identity" {
  name = "sts-web-identity"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Federated = radosgw_iam_openid_connect_provider.sts_idp.arn }
      Action    = "sts:AssumeRoleWithWebIdentity"
      Condition = {
        StringEquals = {
          "${replace(replace(var.sts_oidc_issuer, "https://", ""), "http://", "")}:app_id" = "test-client"
        }
      }
    }]
  })
}

output "sts_role_arn" {
  value = radosgw_iam_role.sts_web_identity.arn
}

output "sts_oidc_provider_arn" {
  value = radosgw_iam_openid_connect_provider.sts_idp.arn
}
