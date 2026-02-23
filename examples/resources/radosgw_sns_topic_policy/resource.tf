# Basic SNS topic policy using jsonencode
resource "radosgw_sns_topic" "example" {
  name          = "my-topic-with-policy"
  push_endpoint = "http://my-service.example.com:8080/notifications"
}

resource "radosgw_sns_topic_policy" "example" {
  arn = radosgw_sns_topic.example.arn

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid       = "AllowGetTopicAttributes"
        Effect    = "Allow"
        Principal = { AWS = ["arn:aws:iam::usfolks:user/fred:subuser"] }
        Action    = ["sns:GetTopicAttributes", "sns:Publish"]
        Resource  = [radosgw_sns_topic.example.arn]
      }
    ]
  })
}

# SNS topic policy using the radosgw_iam_policy_document data source
resource "radosgw_sns_topic" "notifications" {
  name          = "notifications-topic"
  push_endpoint = "http://my-service.example.com:8080/events"
  persistent    = true
}

data "radosgw_iam_policy_document" "sns_topic_policy" {
  statement {
    sid    = "AllowSubscribe"
    effect = "Allow"

    principals {
      type        = "AWS"
      identifiers = ["arn:aws:iam::mytenant:user/publisher"]
    }

    actions = [
      "sns:Publish",
    ]

    resources = [
      radosgw_sns_topic.notifications.arn,
    ]
  }

  statement {
    sid    = "AllowRead"
    effect = "Allow"

    principals {
      type        = "AWS"
      identifiers = ["arn:aws:iam::mytenant:user/reader"]
    }

    actions = [
      "sns:GetTopicAttributes",
    ]

    resources = [
      radosgw_sns_topic.notifications.arn,
    ]
  }
}

resource "radosgw_sns_topic_policy" "notifications" {
  arn    = radosgw_sns_topic.notifications.arn
  policy = data.radosgw_iam_policy_document.sns_topic_policy.json
}
