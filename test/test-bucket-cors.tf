# =============================================================================
# S3 Bucket CORS Configuration Resource Tests
# =============================================================================
# Purpose: Test radosgw_s3_bucket_cors_configuration resources
# Resources: 1 bucket + 1 CORS configuration (two cors_rule blocks)
# Dependencies: None (standalone)
#
# Manual verification (browser preflight) after `tofu apply`:
#   curl -s -o /dev/null -D - -X OPTIONS \
#     -H "Origin: https://app.example.com" \
#     -H "Access-Control-Request-Method: PUT" \
#     http://test-cors-bucket.storage.host:7480/
#   -> response should include:
#      Access-Control-Allow-Origin: https://app.example.com
#      Access-Control-Allow-Methods: GET, PUT, POST
#
# Or inspect the stored rules directly:
#   AWS_ACCESS_KEY_ID=admin AWS_SECRET_ACCESS_KEY=secretkey \
#     aws --endpoint-url http://127.0.0.1:7480 --region default \
#     s3api get-bucket-cors --bucket test-cors-bucket
#
# NOTE: this manages PER-BUCKET CORS (the S3 CORS API). RadosGW release Umbrella
# also supports a GLOBAL (gateway-wide) CORS policy set on the cluster, which is
# NOT managed here:
#   ceph config set client.rgw rgw_gcors_allow_origins "https://app.example.com"
#   ceph config set client.rgw rgw_gcors_allow_methods "GET, PUT, POST"
#   ceph config set client.rgw rgw_gcors_allow_headers "*"
#   ceph config set client.rgw rgw_gcors_expose_headers "ETag"
# (not runtime-updatable — restart the RGW daemon after changing these.)
# =============================================================================

resource "radosgw_s3_bucket" "cors" {
  bucket        = "test-cors-bucket"
  force_destroy = true
}

resource "radosgw_s3_bucket_cors_configuration" "cors" {
  bucket = radosgw_s3_bucket.cors.bucket

  # Named rule: allow a specific app origin to read/write with a preflight cache.
  cors_rule {
    id              = "allow-app"
    allowed_headers = ["*"]
    allowed_methods = ["GET", "PUT", "POST"]
    allowed_origins = ["https://app.example.com"]
    expose_headers  = ["ETag", "x-amz-request-id"]
    max_age_seconds = 3600
  }

  # Anonymous public GET from any origin.
  cors_rule {
    allowed_methods = ["GET"]
    allowed_origins = ["*"]
  }
}

output "cors_bucket" {
  value = radosgw_s3_bucket.cors.bucket
}
