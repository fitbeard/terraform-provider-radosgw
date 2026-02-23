# Create an SNS topic with an HTTP push endpoint
resource "radosgw_sns_topic" "notifications" {
  name          = "bucket-notifications"
  push_endpoint = "http://my-service.example.com:8080/notifications"
}

# Create a persistent topic with retry settings
resource "radosgw_sns_topic" "persistent" {
  name                 = "persistent-notifications"
  push_endpoint        = "http://my-service.example.com:8080/notifications"
  persistent           = true
  time_to_live         = 3600
  max_retries          = 10
  retry_sleep_duration = 30
  opaque_data          = "env=production"
}

# Create a topic with CloudEvents headers and custom SSL settings
resource "radosgw_sns_topic" "cloudevents" {
  name          = "cloudevents-topic"
  push_endpoint = "https://my-service.example.com/events"
  cloudevents   = true
  verify_ssl    = true
}

# Create a topic for Kafka endpoint
resource "radosgw_sns_topic" "kafka" {
  name            = "kafka-notifications"
  push_endpoint   = "kafka://kafka-broker1:9092"
  persistent      = true
  use_ssl         = true
  kafka_ack_level = "broker"
  kafka_brokers   = "kafka-broker1:9092,kafka-broker2:9092"
  mechanism       = "PLAIN"
}

# Create a topic for AMQP endpoint
resource "radosgw_sns_topic" "amqp" {
  name           = "amqp-notifications"
  push_endpoint  = "amqp://rabbitmq.example.com:5672/vhost"
  amqp_exchange  = "ceph-exchange"
  amqp_ack_level = "broker"
}
