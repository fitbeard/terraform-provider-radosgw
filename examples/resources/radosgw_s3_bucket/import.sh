# Import a bucket by name
terraform import radosgw_s3_bucket.example "my-bucket-name"

# Import a bucket with special characters in the name
terraform import radosgw_s3_bucket.logs "my-app-logs-2024"
