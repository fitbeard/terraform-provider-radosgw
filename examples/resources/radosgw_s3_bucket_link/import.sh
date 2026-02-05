# Import a bucket link by bucket name (owner is auto-detected)
terraform import radosgw_s3_bucket_link.example "my-bucket"

# Import with explicit owner (bucket:uid format)
terraform import radosgw_s3_bucket_link.example "my-bucket:bucket-owner"

# Import a bucket in a tenant
terraform import radosgw_s3_bucket_link.tenant "my-bucket:tenant1\$user1"
