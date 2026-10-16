# Configure the RadosGW Provider
provider "radosgw" {
  # RadosGW endpoint URL (required)
  # Can also be set via RADOSGW_ENDPOINT environment variable
  endpoint = "http://127.0.0.1:7480"

  # Access key for authentication (required)
  # Can also be set via RADOSGW_ACCESS_KEY environment variable
  access_key = "admin-access-key"

  # Secret key for authentication (required)
  # Can also be set via RADOSGW_SECRET_KEY environment variable
  secret_key = "admin-secret-key"
}

# Example using environment variables (recommended for production)
# export RADOSGW_ENDPOINT="http://rgw.example.com:7480"
# export RADOSGW_ACCESS_KEY="your-access-key"
# export RADOSGW_SECRET_KEY="your-secret-key"
#
# provider "radosgw" {}

# Example with TLS configuration using a CA certificate file
# provider "radosgw" {
#   endpoint                  = "https://rgw.example.com:7480"
#   access_key                = "admin-access-key"
#   secret_key                = "admin-secret-key"
#   root_ca_certificate_file  = "/path/to/ca-cert.pem"
# }

# Example with TLS configuration using inline CA certificate
# provider "radosgw" {
#   endpoint            = "https://rgw.example.com:7480"
#   access_key          = "admin-access-key"
#   secret_key          = "admin-secret-key"
#   root_ca_certificate = <<-EOT
#     -----BEGIN CERTIFICATE-----
#     MIIDXTCCAkWgAwIBAgIJAJC1...
#     -----END CERTIFICATE-----
#   EOT
# }

# Example with insecure TLS (skip certificate verification for HTTPS)
# WARNING: Only use this for development or testing environments with self-signed certs
# Note: This option has no effect on plain HTTP connections
# provider "radosgw" {
#   endpoint                   = "https://rgw.example.com:7480"
#   access_key                 = "admin-access-key"
#   secret_key                 = "admin-secret-key"
#   tls_insecure_skip_verify   = true
# }

# Assuming an IAM role (STS AssumeRole)
# The static access_key/secret_key are exchanged for temporary credentials for the
# role, which are then used for all S3 and IAM/STS calls. The role must already
# exist and its trust policy must allow these credentials to assume it.
# Note: the Admin Ops API cannot use the session token that comes with temporary
# credentials, so Admin-API-backed data is unavailable under an assumed role (the
# same behavior as a user without admin capabilities) — use it for S3 and IAM
# role/policy/OIDC resources.
# provider "radosgw" {
#   endpoint   = "http://rgw.example.com:7480"
#   access_key = "admin-access-key"
#   secret_key = "admin-secret-key"
#
#   assume_role {
#     # role_arn accepts the non-account form (arn:aws:iam:::role/<name>) or the
#     # account-scoped form (arn:aws:iam::<account-id>:role/<name>).
#     role_arn     = "arn:aws:iam:::role/terraform"
#     session_name = "terraform"
#     # duration  = "1h"    # session lifetime (default 1h)
#     # Note: RadosGW does not support an external ID, so there is no external_id argument.
#   }
# }

# Assuming a role with an inline session policy (jsonencode)
# The session policy further restricts the assumed session. On RadosGW, use
# explicit Deny statements — allow-only scope-down (AWS implicit deny) is not
# enforced.
# provider "radosgw" {
#   endpoint   = "http://rgw.example.com:7480"
#   access_key = "admin-access-key"
#   secret_key = "admin-secret-key"
#
#   assume_role {
#     role_arn     = "arn:aws:iam:::role/terraform"
#     session_name = "terraform"
#
#     policy = jsonencode({
#       Version = "2012-10-17"
#       Statement = [
#         {
#           Sid      = "DenyBucketDeletion"
#           Effect   = "Deny"
#           Action   = ["s3:DeleteBucket"]
#           Resource = ["*"]
#         }
#       ]
#     })
#   }
# }

# Assuming a role with the session policy read from a file
# provider "radosgw" {
#   endpoint   = "http://rgw.example.com:7480"
#   access_key = "admin-access-key"
#   secret_key = "admin-secret-key"
#
#   assume_role {
#     role_arn     = "arn:aws:iam:::role/terraform"
#     session_name = "terraform"
#     policy       = file("${path.module}/session-policy.json")
#   }
# }
