# =============================================================================
# S3 Bucket Notification Resource Tests
# =============================================================================
# Purpose: Test radosgw_s3_bucket_notification resource
# Resources: 1 bucket notification (using basic HTTP topic)
# Dependencies: test-sns-topic.tf (radosgw_sns_topic.basic)
#               test-bucket.tf (radosgw_s3_bucket.test_basic)
# =============================================================================

# -----------------------------------------------------------------------------
# Bucket notification — send object creation and removal events to basic HTTP topic
# Uses the basic HTTP topic (from test-sns-topic.tf)
# -----------------------------------------------------------------------------
resource "radosgw_s3_bucket_notification" "test" {
  bucket = radosgw_s3_bucket.test_basic.bucket

  topic {
    id        = "created-objects"
    topic_arn = radosgw_sns_topic.basic.arn
    events    = ["s3:ObjectCreated:*"]
  }

  topic {
    id            = "deleted-images"
    topic_arn     = radosgw_sns_topic.basic.arn
    events        = ["s3:ObjectRemoved:*"]
    filter_prefix = "images/"
    filter_suffix = ".jpg"
  }
}

# =============================================================================
# Outputs
# =============================================================================

output "bucket_notification_bucket" {
  description = "Bucket with notification configuration"
  value       = radosgw_s3_bucket_notification.test.bucket
}
