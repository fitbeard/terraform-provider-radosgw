# Import a RadosGW user by user_id
terraform import radosgw_iam_user.example example-user

# Import a user with tenant
terraform import radosgw_iam_user.custom my-tenant$custom-user
