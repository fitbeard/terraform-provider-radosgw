# Create a basic bucket
resource "radosgw_s3_bucket" "example" {
  bucket = "my-example-bucket"
}

# Create a bucket with force_destroy enabled
# This allows the bucket to be deleted even if it contains objects
resource "radosgw_s3_bucket" "with_force_destroy" {
  bucket        = "my-temporary-bucket"
  force_destroy = true
}

# Create a bucket with object lock enabled
# Note: object_lock_enabled cannot be changed after creation
resource "radosgw_s3_bucket" "with_object_lock" {
  bucket              = "my-compliance-bucket"
  object_lock_enabled = true
}

# Create a bucket with versioning enabled
resource "radosgw_s3_bucket" "with_versioning" {
  bucket     = "my-versioned-bucket"
  versioning = "enabled"
}

# Create a bucket with quota limits
resource "radosgw_s3_bucket" "with_quota" {
  bucket = "my-quota-bucket"

  bucket_quota = {
    enabled     = true
    max_size    = 10737418240 # 10 GB in bytes
    max_objects = 10000
  }
}

# Create a bucket for a specific tenant
# Note: tenant cannot be changed after creation
resource "radosgw_s3_bucket" "with_tenant" {
  bucket = "my-tenant-bucket"
  tenant = "mytenant"
}

# Create a fully configured bucket
resource "radosgw_s3_bucket" "full_example" {
  bucket        = "my-full-bucket"
  force_destroy = true
  versioning    = "enabled"

  bucket_quota = {
    enabled     = true
    max_size    = 53687091200 # 50 GB
    max_objects = 100000
  }
}

# To manage bucket ACLs, use the radosgw_s3_bucket_acl resource
# See radosgw_s3_bucket_acl documentation for examples

# To transfer bucket ownership, use the radosgw_s3_bucket_link resource
# Note: After transferring ownership, bucket_acl and bucket_policy
# can only be managed with the new owner's credentials
# See radosgw_s3_bucket_link documentation for examples
