# =============================================================================
# SNS Topic Resource Tests
# =============================================================================
# Purpose: Test radosgw_sns_topic resource with various endpoint configurations
# Resources: 4 SNS topics (basic HTTP, persistent HTTP, AMQP, Kafka)
#            1 SNS topic policy (on the Kafka topic)
# Dependencies: None (standalone)
# =============================================================================

# -----------------------------------------------------------------------------
# Basic HTTP topic - minimal configuration
# -----------------------------------------------------------------------------
resource "radosgw_sns_topic" "basic" {
  name          = "test-basic-topic"
  push_endpoint = "http://localhost:10900"
}

# -----------------------------------------------------------------------------
# Persistent HTTP topic with CloudEvents and custom retry settings
# -----------------------------------------------------------------------------
resource "radosgw_sns_topic" "persistent" {
  name                 = "test-persistent-topic"
  push_endpoint        = "http://localhost:10901"
  opaque_data          = "my-opaque-data"
  persistent           = true
  cloudevents          = true
  verify_ssl           = false
  time_to_live         = 600
  max_retries          = 5
  retry_sleep_duration = 10
}

# -----------------------------------------------------------------------------
# AMQP topic - demonstrates broker-based endpoint
# -----------------------------------------------------------------------------
resource "radosgw_sns_topic" "amqp" {
  name           = "test-amqp-topic"
  push_endpoint  = "amqp://localhost:5672"
  amqp_exchange  = "test-exchange"
  amqp_ack_level = "broker"
  use_ssl        = false
}

# -----------------------------------------------------------------------------
# Kafka topic - demonstrates Kafka endpoint with broker cluster
# -----------------------------------------------------------------------------
resource "radosgw_sns_topic" "kafka" {
  name            = "test-kafka-topic"
  push_endpoint   = "kafka://localhost:9092"
  persistent      = true
  kafka_ack_level = "broker"
  kafka_brokers   = "localhost:9092,localhost:9093"
  mechanism       = "PLAIN"
}

# -----------------------------------------------------------------------------
# SNS Topic Policy - demonstrates access control on a topic
# Uses radosgw_iam_policy_document data source for HCL-based policy definition
# Supported actions: sns:GetTopicAttributes, sns:SetTopicAttributes,
#                    sns:DeleteTopic, sns:Publish
# -----------------------------------------------------------------------------
data "radosgw_iam_policy_document" "kafka_topic_policy" {
  statement {
    sid    = "AllowPublish"
    effect = "Allow"

    principals {
      type        = "AWS"
      identifiers = ["arn:aws:iam::testaccount:user/publisher"]
    }

    actions = [
      "sns:Publish",
    ]

    resources = [
      radosgw_sns_topic.kafka.arn,
    ]
  }

  statement {
    sid    = "AllowReadOnly"
    effect = "Allow"

    principals {
      type        = "AWS"
      identifiers = ["arn:aws:iam::testaccount:user/reader"]
    }

    actions = [
      "sns:GetTopicAttributes",
    ]

    resources = [
      radosgw_sns_topic.kafka.arn,
    ]
  }
}

resource "radosgw_sns_topic_policy" "kafka" {
  arn    = radosgw_sns_topic.kafka.arn
  policy = data.radosgw_iam_policy_document.kafka_topic_policy.json
}

# =============================================================================
# Outputs
# =============================================================================

output "basic_topic_arn" {
  description = "ARN of the basic SNS topic"
  value       = radosgw_sns_topic.basic.arn
}

output "basic_topic_user" {
  description = "User that created the basic SNS topic"
  value       = radosgw_sns_topic.basic.user
}

output "persistent_topic_arn" {
  description = "ARN of the persistent SNS topic"
  value       = radosgw_sns_topic.persistent.arn
}

output "persistent_topic_opaque_data" {
  description = "Opaque data of the persistent SNS topic"
  value       = radosgw_sns_topic.persistent.opaque_data
}

output "amqp_topic_arn" {
  description = "ARN of the AMQP SNS topic"
  value       = radosgw_sns_topic.amqp.arn
}

output "kafka_topic_arn" {
  description = "ARN of the Kafka SNS topic"
  value       = radosgw_sns_topic.kafka.arn
}
