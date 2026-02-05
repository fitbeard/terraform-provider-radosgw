# Import an S3 access key
# Format: s3:user_id:access_key
terraform import radosgw_iam_access_key.custom "s3:example-user:MY_CUSTOM_ACCESS_KEY"

# Import a Swift access key
# Format: swift:user_id:subuser
terraform import radosgw_iam_access_key.swift "swift:example-user:swiftuser"
