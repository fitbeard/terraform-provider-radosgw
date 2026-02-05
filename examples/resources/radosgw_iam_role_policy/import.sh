# Import a role policy
# Format: role_name:policy_name
terraform import radosgw_iam_role_policy.s3_access "ExampleRole:S3AccessPolicy"
terraform import radosgw_iam_role_policy.s3_readonly "ExampleRole:S3ReadOnlyPolicy"
