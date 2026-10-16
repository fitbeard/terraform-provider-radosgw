# =============================================================================
# Provider assume_role (STS AssumeRole) manual test
# =============================================================================
# Purpose: exercise the provider's assume_role block end to end — create a role,
#   then an aliased provider assumes it and creates an S3 bucket with the
#   temporary credentials.
# Resources: 1 role + 1 role policy (default/admin provider) and 1 bucket
#   (aliased assume-role provider)
# Dependencies: main.tf (default provider)
#
# How it works in a single apply: the aliased provider's role_arn references the
# role's computed ARN, so the provider's configuration is deferred until the role
# exists (unknown at plan time). At apply, the role is created first, the aliased
# provider assumes it, and the bucket is created with the session credentials.
#
# Note: the Admin Ops API cannot use the session token that comes with temporary
# credentials, so Admin-API-only attributes (e.g. num_shards, bucket_quota) are
# null on a bucket managed through the assumed-role provider — the same behavior
# as a user without admin capabilities.
# =============================================================================

# Role the provider will assume. Its trust policy allows the admin user (the
# default provider's identity) to assume it; its inline policy grants S3 access.
resource "radosgw_iam_role" "assume_target" {
  name = "terraform-assume-target"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { AWS = "arn:aws:iam:::user/admin" }
      Action    = "sts:AssumeRole"
    }]
  })
}

resource "radosgw_iam_role_policy" "assume_target" {
  role = radosgw_iam_role.assume_target.name
  name = "s3-full"
  policy = jsonencode({
    Version   = "2012-10-17"
    Statement = [{ Effect = "Allow", Action = "s3:*", Resource = "*" }]
  })
}

# Aliased provider that authenticates by assuming the role above. role_arn is the
# role's computed ARN, which defers this provider's configuration to apply time.
provider "radosgw" {
  alias      = "assumed"
  endpoint   = "http://127.0.0.1:7480"
  access_key = "admin"
  secret_key = "secretkey"

  assume_role {
    role_arn     = radosgw_iam_role.assume_target.arn
    session_name = "terraform-test"
    duration     = "1h"

    # Optional session policy (RadosGW enforces explicit Deny; allow-only
    # scope-down is not applied). Uncomment to try it:
    # policy = jsonencode({
    #   Version   = "2012-10-17"
    #   Statement = [{ Sid = "DenyBucketDeletion", Effect = "Deny", Action = ["s3:DeleteBucket"], Resource = ["*"] }]
    # })
  }
}

# Bucket created using the temporary (assumed-role) credentials.
resource "radosgw_s3_bucket" "assumed" {
  provider      = radosgw.assumed
  bucket        = "terraform-assume-role-bucket"
  force_destroy = true

  depends_on = [radosgw_iam_role_policy.assume_target]
}

output "assume_role_target_arn" {
  value = radosgw_iam_role.assume_target.arn
}

output "assume_role_bucket" {
  value = radosgw_s3_bucket.assumed.bucket
}

output "assume_role_bucket_owner" {
  value = radosgw_s3_bucket.assumed.owner
}

# Admin-only attribute: null under the assumed role (Admin API has no session token).
output "assume_role_bucket_num_shards" {
  value = radosgw_s3_bucket.assumed.num_shards
}
