# Import a bucket ACL by bucket name
# The current ACL will be read from the bucket
terraform import radosgw_s3_bucket_acl.example "my-bucket-name"

# Note: This resource can only manage ACLs for buckets owned by
# the user configured in the provider. The S3 API restricts ACL
# modifications to the bucket owner.
