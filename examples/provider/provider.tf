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
