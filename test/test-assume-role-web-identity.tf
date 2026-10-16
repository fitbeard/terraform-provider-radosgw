# =============================================================================
# Provider assume_role_with_web_identity (STS AssumeRoleWithWebIdentity) — manual
# =============================================================================
# Unlike test-assume-role.tf, this one is COMMENTED OUT: it requires the local
# OIDC provider to be running and a freshly minted JWT, so it cannot be part of
# the default `make tf-apply`. Follow the steps below to run it by hand.
#
# Steps:
#   1. Start the OIDC provider:   make oidc-up
#   2. Mint a token:              TOKEN=$(make -s oidc-token)
#   3. Uncomment the block below and apply with the token + issuer:
#        make tf-apply TF_CLI_ARGS_apply="-var web_identity_token=$TOKEN \
#          -var oidc_issuer=$(make -s -p | awk -F' = ' '/^OIDC_ISSUER /{print $2;exit}')"
#      (On a normal host the issuer is http://localhost:9080; in a dev container
#      using docker-outside-of-docker it is http://host.docker.internal:9080 —
#      `make oidc-up` auto-detects it.)
#
# How it works in a single apply: the default (admin) provider registers the OIDC
# provider + a federated role, and the aliased web-identity provider assumes that
# role using the JWT. role_arn references the role's computed ARN, so the aliased
# provider's configuration is deferred until the role exists. Web identity is
# anonymous, so the aliased provider needs no access_key/secret_key.
# =============================================================================

# variable "web_identity_token" {
#   type        = string
#   sensitive   = true
#   description = "OIDC JWT minted from the local provider (make oidc-token)."
# }
#
# variable "oidc_issuer" {
#   type        = string
#   default     = "http://localhost:9080"
#   description = "Issuer URL of the local OIDC provider (must match the token's iss)."
# }
#
# locals {
#   # Issuer host without scheme, used in the trust-policy condition key and the
#   # OIDC-provider ARN (e.g. host.docker.internal:9080).
#   oidc_host = replace(replace(var.oidc_issuer, "https://", ""), "http://", "")
# }
#
# resource "radosgw_iam_openid_connect_provider" "wi" {
#   url            = var.oidc_issuer
#   client_id_list = ["test-client"]
#   # In-place updates are unsupported on Reef/Squid; force replace-on-change.
#   allow_updates = false
#   # Placeholder: RadosGW only verifies the thumbprint when the JWKS exposes x5c.
#   thumbprint_list = ["0000000000000000000000000000000000000000"]
# }
#
# resource "radosgw_iam_role" "wi" {
#   name = "terraform-webid-target"
#   assume_role_policy = jsonencode({
#     Version = "2012-10-17"
#     Statement = [{
#       Effect    = "Allow"
#       Principal = { Federated = radosgw_iam_openid_connect_provider.wi.arn }
#       Action    = "sts:AssumeRoleWithWebIdentity"
#       Condition = { StringEquals = { "${local.oidc_host}:app_id" = "test-client" } }
#     }]
#   })
# }
#
# resource "radosgw_iam_role_policy" "wi" {
#   role   = radosgw_iam_role.wi.name
#   name   = "s3-full"
#   policy = jsonencode({ Version = "2012-10-17", Statement = [{ Effect = "Allow", Action = "s3:*", Resource = "*" }] })
# }
#
# provider "radosgw" {
#   alias    = "wi"
#   endpoint = "http://127.0.0.1:7480"
#   # No access_key/secret_key — web identity is anonymous.
#   assume_role_with_web_identity {
#     role_arn           = radosgw_iam_role.wi.arn
#     session_name       = "terraform-webid"
#     web_identity_token = var.web_identity_token
#   }
# }
#
# resource "radosgw_s3_bucket" "wi" {
#   provider      = radosgw.wi
#   bucket        = "terraform-webid-bucket"
#   force_destroy = true
#   depends_on    = [radosgw_iam_role_policy.wi]
# }
#
# output "webid_bucket_owner" {
#   value = radosgw_s3_bucket.wi.owner
# }
