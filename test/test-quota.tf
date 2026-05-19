# =============================================================================
# Quota Resource Tests
# =============================================================================
# Purpose: Test radosgw_iam_quota resource with various configurations
# Resources: 3 users, 5 quotas (user/bucket plus tenant user quota)
# Dependencies: None (standalone)
# =============================================================================

# -----------------------------------------------------------------------------
# User 1: Test quota defaults and disabled state
# -----------------------------------------------------------------------------
resource "radosgw_iam_user" "quota_defaults" {
  user_id      = "quota-defaults-user"
  display_name = "Quota Defaults Test User"
  email        = "quota-defaults@example.com"
}

# Bucket quota - disabled with partial config (tests defaults)
resource "radosgw_iam_quota" "defaults_bucket" {
  user_id     = radosgw_iam_user.quota_defaults.user_id
  type        = "bucket"
  enabled     = false
  max_objects = 2500
  # max_size omitted - should use default (-1 unlimited)
}

# User quota - enabled by default with partial config
resource "radosgw_iam_quota" "defaults_user" {
  user_id     = radosgw_iam_user.quota_defaults.user_id
  type        = "user"
  max_objects = 10000
  # enabled omitted - should default to true
  # max_size omitted - should use default (-1 unlimited)
}

output "quota_defaults_bucket_enabled" {
  value = radosgw_iam_quota.defaults_bucket.enabled
}

output "quota_defaults_user_enabled" {
  value = radosgw_iam_quota.defaults_user.enabled
}

# -----------------------------------------------------------------------------
# User 2: Test full quota configuration with all values specified
# -----------------------------------------------------------------------------
resource "radosgw_iam_user" "quota_full" {
  user_id      = "quota-full-user"
  display_name = "Quota Full Config Test User"
}

# Bucket quota - fully specified
resource "radosgw_iam_quota" "full_bucket" {
  user_id     = radosgw_iam_user.quota_full.user_id
  type        = "bucket"
  enabled     = true
  max_size    = 1000000000 # 1GB per bucket
  max_objects = 5000
}

# User quota - fully specified
resource "radosgw_iam_quota" "full_user" {
  user_id     = radosgw_iam_user.quota_full.user_id
  type        = "user"
  enabled     = true
  max_size    = 100000000000 # 100GB total
  max_objects = 12345678
}

output "quota_full_bucket" {
  value = {
    enabled     = radosgw_iam_quota.full_bucket.enabled
    max_size    = radosgw_iam_quota.full_bucket.max_size
    max_objects = radosgw_iam_quota.full_bucket.max_objects
  }
}

output "quota_full_user" {
  value = {
    enabled     = radosgw_iam_quota.full_user.enabled
    max_size    = radosgw_iam_quota.full_user.max_size
    max_objects = radosgw_iam_quota.full_user.max_objects
  }
}

# -----------------------------------------------------------------------------
# User 3: Test tenant user quota with local user_id reference
# -----------------------------------------------------------------------------
resource "radosgw_iam_user" "quota_tenant" {
  user_id      = "quota-tenant-user"
  tenant       = "quotatenant"
  display_name = "Quota Tenant Test User"
}

# User quota - references user_id, not id, to verify tenant users resolve correctly
resource "radosgw_iam_quota" "tenant_user" {
  user_id     = radosgw_iam_user.quota_tenant.user_id
  type        = "user"
  enabled     = true
  max_size    = 10737418240 # 10GB total
  max_objects = 10000
}

output "quota_tenant_user" {
  value = {
    user_id     = radosgw_iam_user.quota_tenant.user_id
    rgw_id      = radosgw_iam_user.quota_tenant.id
    tenant      = radosgw_iam_user.quota_tenant.tenant
    enabled     = radosgw_iam_quota.tenant_user.enabled
    max_size    = radosgw_iam_quota.tenant_user.max_size
    max_objects = radosgw_iam_quota.tenant_user.max_objects
  }
}
