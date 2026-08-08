# Import using "role/policy_arn"
terraform import radosgw_iam_role_policy_attachment.read_only "app-role/arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess"
