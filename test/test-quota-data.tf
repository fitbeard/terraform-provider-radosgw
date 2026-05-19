# =============================================================================
# Quota Data Source Tests
# =============================================================================
# Purpose: Test radosgw_iam_quota data source
# Resources: 3 data sources (user quota, bucket quota, tenant user quota)
# Dependencies: test-quota.tf (quota_full/quota_tenant users and quotas)
# =============================================================================

# Get user quota for the fully configured test user
data "radosgw_iam_quota" "user_quota" {
  user_id = radosgw_iam_user.quota_full.user_id
  type    = "user"

  depends_on = [radosgw_iam_quota.full_user]
}

# Get bucket quota for the fully configured test user
data "radosgw_iam_quota" "bucket_quota" {
  user_id = radosgw_iam_user.quota_full.user_id
  type    = "bucket"

  depends_on = [radosgw_iam_quota.full_bucket]
}

output "data_user_quota" {
  value = {
    enabled     = data.radosgw_iam_quota.user_quota.enabled
    max_size    = data.radosgw_iam_quota.user_quota.max_size
    max_objects = data.radosgw_iam_quota.user_quota.max_objects
  }
}

output "data_bucket_quota" {
  value = {
    enabled     = data.radosgw_iam_quota.bucket_quota.enabled
    max_size    = data.radosgw_iam_quota.bucket_quota.max_size
    max_objects = data.radosgw_iam_quota.bucket_quota.max_objects
  }
}

# Get user quota for a tenant user using the local user_id reference
data "radosgw_iam_quota" "tenant_user_quota" {
  user_id = radosgw_iam_user.quota_tenant.user_id
  type    = "user"

  depends_on = [radosgw_iam_quota.tenant_user]
}

output "data_tenant_user_quota" {
  value = {
    user_id     = radosgw_iam_user.quota_tenant.user_id
    rgw_id      = radosgw_iam_user.quota_tenant.id
    enabled     = data.radosgw_iam_quota.tenant_user_quota.enabled
    max_size    = data.radosgw_iam_quota.tenant_user_quota.max_size
    max_objects = data.radosgw_iam_quota.tenant_user_quota.max_objects
  }
}
