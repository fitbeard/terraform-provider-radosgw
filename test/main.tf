# =============================================================================
# Provider Configuration and Shared Resources
# =============================================================================
# Purpose: Configure provider and create shared test user
# Resources: 1 IAM user (shared across multiple test files)
# Dependencies: None (base configuration)
# =============================================================================

terraform {
  required_providers {
    radosgw = {
      source = "registry.local/fitbeard/radosgw"
    }
  }
}

provider "radosgw" {
  endpoint   = "http://127.0.0.1:7480"
  access_key = "admin"
  secret_key = "secretkey"
}

# -----------------------------------------------------------------------------
# Shared test user - used by test-keys.tf and test-access-keys-data.tf
# -----------------------------------------------------------------------------
resource "radosgw_iam_user" "test" {
  user_id      = "terraform-test-user"
  display_name = "Terraform Test User"
  email        = "terraform@example.com"
  max_buckets  = 250
  suspended    = false
  op_mask      = "read, write"
}

output "user_id" {
  value = radosgw_iam_user.test.user_id
}
