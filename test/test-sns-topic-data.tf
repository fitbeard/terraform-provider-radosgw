# =============================================================================
# SNS Topic Data Source Tests
# =============================================================================
# Purpose: Test radosgw_sns_topic data source by reading existing topics
# Data Sources: 3 SNS topic lookups (basic HTTP, persistent HTTP, AMQP)
# Dependencies: test-sns-topic.tf (resources must exist first)
# =============================================================================

# -----------------------------------------------------------------------------
# Look up the basic HTTP topic
# -----------------------------------------------------------------------------
data "radosgw_sns_topic" "basic" {
  name = radosgw_sns_topic.basic.name

  depends_on = [radosgw_sns_topic.basic]
}

# -----------------------------------------------------------------------------
# Look up the persistent HTTP topic
# -----------------------------------------------------------------------------
data "radosgw_sns_topic" "persistent" {
  name = radosgw_sns_topic.persistent.name

  depends_on = [radosgw_sns_topic.persistent]
}

# -----------------------------------------------------------------------------
# Look up the AMQP topic
# -----------------------------------------------------------------------------
data "radosgw_sns_topic" "amqp" {
  name = radosgw_sns_topic.amqp.name

  depends_on = [radosgw_sns_topic.amqp]
}

# =============================================================================
# Outputs
# =============================================================================

output "data_basic_topic_arn" {
  description = "ARN of the basic topic (from data source)"
  value       = data.radosgw_sns_topic.basic.arn
}

output "data_basic_topic_endpoint" {
  description = "Push endpoint of the basic topic (from data source)"
  value       = data.radosgw_sns_topic.basic.push_endpoint
}

output "data_basic_topic_user" {
  description = "Owner of the basic topic (from data source)"
  value       = data.radosgw_sns_topic.basic.user
}

output "data_persistent_topic_arn" {
  description = "ARN of the persistent topic (from data source)"
  value       = data.radosgw_sns_topic.persistent.arn
}

output "data_persistent_topic_persistent" {
  description = "Persistent flag of the persistent topic (from data source)"
  value       = data.radosgw_sns_topic.persistent.persistent
}

output "data_persistent_topic_cloudevents" {
  description = "CloudEvents flag of the persistent topic (from data source)"
  value       = data.radosgw_sns_topic.persistent.cloudevents
}

output "data_persistent_topic_opaque_data" {
  description = "Opaque data of the persistent topic (from data source)"
  value       = data.radosgw_sns_topic.persistent.opaque_data
}

output "data_persistent_topic_time_to_live" {
  description = "Time to live of the persistent topic (from data source)"
  value       = data.radosgw_sns_topic.persistent.time_to_live
}

output "data_persistent_topic_max_retries" {
  description = "Max retries of the persistent topic (from data source)"
  value       = data.radosgw_sns_topic.persistent.max_retries
}

output "data_persistent_topic_retry_sleep" {
  description = "Retry sleep duration of the persistent topic (from data source)"
  value       = data.radosgw_sns_topic.persistent.retry_sleep_duration
}

output "data_amqp_topic_arn" {
  description = "ARN of the AMQP topic (from data source)"
  value       = data.radosgw_sns_topic.amqp.arn
}

output "data_amqp_topic_exchange" {
  description = "AMQP exchange of the AMQP topic (from data source)"
  value       = data.radosgw_sns_topic.amqp.amqp_exchange
}

output "data_amqp_topic_ack_level" {
  description = "AMQP ack level of the AMQP topic (from data source)"
  value       = data.radosgw_sns_topic.amqp.amqp_ack_level
}
