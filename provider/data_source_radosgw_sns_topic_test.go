package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccRadosgwSNSTopicDataSource_basic(t *testing.T) {
	t.Parallel()

	topicName := randomName("tf-acc-ds-topic")

	checks := []resource.TestCheckFunc{
		resource.TestCheckResourceAttrPair("data.radosgw_sns_topic.test", "name", "radosgw_sns_topic.test", "name"),
		resource.TestCheckResourceAttrPair("data.radosgw_sns_topic.test", "arn", "radosgw_sns_topic.test", "arn"),
		resource.TestCheckResourceAttr("data.radosgw_sns_topic.test", "push_endpoint", "http://localhost:10900"),
		resource.TestCheckResourceAttr("data.radosgw_sns_topic.test", "persistent", "false"),
	}
	// User is only returned by GetTopicAttributes on Squid+
	if !getCephVersion().LessThan(CephVersion_Squid) {
		checks = append(checks,
			resource.TestCheckResourceAttrSet("data.radosgw_sns_topic.test", "user"),
		)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwSNSTopicDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwSNSTopicDataSourceConfig_basic(topicName),
				Check:  resource.ComposeTestCheckFunc(checks...),
			},
		},
	})
}

func TestAccRadosgwSNSTopicDataSource_persistent(t *testing.T) {
	t.Parallel()

	topicName := randomName("tf-acc-ds-topic")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t); testAccPreCheckSkipForVersion(t, CephVersion_Squid) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwSNSTopicDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwSNSTopicDataSourceConfig_persistent(topicName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.radosgw_sns_topic.test", "name", "radosgw_sns_topic.test", "name"),
					resource.TestCheckResourceAttrPair("data.radosgw_sns_topic.test", "arn", "radosgw_sns_topic.test", "arn"),
					resource.TestCheckResourceAttr("data.radosgw_sns_topic.test", "push_endpoint", "http://localhost:10901"),
					resource.TestCheckResourceAttr("data.radosgw_sns_topic.test", "persistent", "true"),
					resource.TestCheckResourceAttr("data.radosgw_sns_topic.test", "opaque_data", "test-opaque"),
					resource.TestCheckResourceAttr("data.radosgw_sns_topic.test", "cloudevents", "true"),
					resource.TestCheckResourceAttr("data.radosgw_sns_topic.test", "verify_ssl", "false"),
					resource.TestCheckResourceAttr("data.radosgw_sns_topic.test", "time_to_live", "300"),
					resource.TestCheckResourceAttr("data.radosgw_sns_topic.test", "max_retries", "3"),
					resource.TestCheckResourceAttr("data.radosgw_sns_topic.test", "retry_sleep_duration", "5"),
				),
			},
		},
	})
}

func TestAccRadosgwSNSTopicDataSource_amqp(t *testing.T) {
	t.Parallel()

	topicName := randomName("tf-acc-ds-topic")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t); testAccPreCheckSkipForVersion(t, CephVersion_Squid) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwSNSTopicDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwSNSTopicDataSourceConfig_amqp(topicName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.radosgw_sns_topic.test", "name", "radosgw_sns_topic.test", "name"),
					resource.TestCheckResourceAttrPair("data.radosgw_sns_topic.test", "arn", "radosgw_sns_topic.test", "arn"),
					resource.TestCheckResourceAttr("data.radosgw_sns_topic.test", "push_endpoint", "amqp://localhost:5672"),
					resource.TestCheckResourceAttr("data.radosgw_sns_topic.test", "amqp_exchange", "test-exchange"),
					resource.TestCheckResourceAttr("data.radosgw_sns_topic.test", "amqp_ack_level", "broker"),
				),
			},
		},
	})
}

// Test configurations

func testAccRadosgwSNSTopicDataSourceConfig_basic(topicName string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_sns_topic" "test" {
  name          = %q
  push_endpoint = "http://localhost:10900"
}

data "radosgw_sns_topic" "test" {
  name = radosgw_sns_topic.test.name
}
`, topicName)
}

func testAccRadosgwSNSTopicDataSourceConfig_persistent(topicName string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_sns_topic" "test" {
  name                 = %q
  push_endpoint        = "http://localhost:10901"
  opaque_data          = "test-opaque"
  persistent           = true
  cloudevents          = true
  verify_ssl           = false
  time_to_live         = 300
  max_retries          = 3
  retry_sleep_duration = 5
}

data "radosgw_sns_topic" "test" {
  name = radosgw_sns_topic.test.name
}
`, topicName)
}

func testAccRadosgwSNSTopicDataSourceConfig_amqp(topicName string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_sns_topic" "test" {
  name           = %q
  push_endpoint  = "amqp://localhost:5672"
  amqp_exchange  = "test-exchange"
  amqp_ack_level = "broker"
}

data "radosgw_sns_topic" "test" {
  name = radosgw_sns_topic.test.name
}
`, topicName)
}
