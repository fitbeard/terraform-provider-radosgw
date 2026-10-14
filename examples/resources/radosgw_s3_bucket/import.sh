# The import ID is the bucket name.

# Import a bucket (provider configured with admin credentials — full attribute set).
terraform import radosgw_s3_bucket.example "my-bucket-name"

# Import a bucket with special characters in the name.
terraform import radosgw_s3_bucket.logs "my-app-logs-2024"

# Import as the bucket owner, without admin capabilities (over the S3 API).
# S3-derivable attributes are populated; admin-only attributes import as null.
# Do not declare admin-only settings such as bucket_quota in a config you manage
# with non-admin credentials — see the note below.
terraform import radosgw_s3_bucket.owned "my-bucket-name"
