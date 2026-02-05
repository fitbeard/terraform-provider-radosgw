# Import a subuser
# Format: user_id:subuser_name
terraform import radosgw_iam_subuser.swift "example-user:swift"
terraform import radosgw_iam_subuser.s3_full "example-user:s3admin"
