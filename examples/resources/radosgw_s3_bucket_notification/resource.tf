# Create an SNS topic for receiving notifications
resource "radosgw_sns_topic" "notifications" {
  name          = "bucket-notifications"
  push_endpoint = "http://my-service.example.com:8080/notifications"
}

# Create a bucket
resource "radosgw_s3_bucket" "data" {
  bucket = "my-data-bucket"
}

# Basic notification: notify on all object creation events
resource "radosgw_s3_bucket_notification" "basic" {
  bucket = radosgw_s3_bucket.data.bucket

  topic {
    topic_arn = radosgw_sns_topic.notifications.arn
    events    = ["s3:ObjectCreated:*"]
  }
}

# Notification with key filters: only notify for JPEG images
resource "radosgw_s3_bucket_notification" "filtered" {
  bucket = radosgw_s3_bucket.data.bucket

  topic {
    id            = "jpeg-uploads"
    topic_arn     = radosgw_sns_topic.notifications.arn
    events        = ["s3:ObjectCreated:*"]
    filter_prefix = "images/"
    filter_suffix = ".jpg"
  }
}

# Multiple topic configurations on the same bucket
resource "radosgw_sns_topic" "created" {
  name          = "object-created"
  push_endpoint = "http://my-service.example.com:8080/created"
}

resource "radosgw_sns_topic" "removed" {
  name          = "object-removed"
  push_endpoint = "http://my-service.example.com:8080/removed"
}

resource "radosgw_s3_bucket_notification" "multi" {
  bucket = radosgw_s3_bucket.data.bucket

  topic {
    id        = "created-events"
    topic_arn = radosgw_sns_topic.created.arn
    events    = ["s3:ObjectCreated:*"]
  }

  topic {
    id        = "removed-events"
    topic_arn = radosgw_sns_topic.removed.arn
    events    = ["s3:ObjectRemoved:*"]
  }
}
