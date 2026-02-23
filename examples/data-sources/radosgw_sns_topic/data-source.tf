# Look up a topic by name
data "radosgw_sns_topic" "example" {
  name = "my-notifications"
}

# Output topic details
output "topic_arn" {
  description = "ARN of the SNS topic"
  value       = data.radosgw_sns_topic.example.arn
}

output "topic_endpoint" {
  description = "Push endpoint of the SNS topic"
  value       = data.radosgw_sns_topic.example.push_endpoint
}

output "topic_persistent" {
  description = "Whether the topic is persistent"
  value       = data.radosgw_sns_topic.example.persistent
}
