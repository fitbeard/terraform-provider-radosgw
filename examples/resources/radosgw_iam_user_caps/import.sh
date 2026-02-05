# Import user capabilities by user_id
# The capabilities will be read from the existing user in RadosGW
terraform import radosgw_iam_user_caps.admin_caps "example-user"
