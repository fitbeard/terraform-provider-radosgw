package provider

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccRadosgwSNSTopic_basic(t *testing.T) {
	t.Parallel()

	topicName := randomName("tf-acc-topic")

	checks := []resource.TestCheckFunc{
		testAccCheckRadosgwSNSTopicExists("radosgw_sns_topic.test"),
		resource.TestCheckResourceAttr("radosgw_sns_topic.test", "name", topicName),
		resource.TestCheckResourceAttrSet("radosgw_sns_topic.test", "arn"),
		resource.TestCheckResourceAttr("radosgw_sns_topic.test", "push_endpoint", "http://localhost:10900"),
		resource.TestCheckResourceAttr("radosgw_sns_topic.test", "persistent", "false"),
	}
	// User is only returned by GetTopicAttributes on Squid+
	if !getCephVersion().LessThan(CephVersion_Squid) {
		checks = append(checks,
			resource.TestCheckResourceAttrSet("radosgw_sns_topic.test", "user"),
			resource.TestCheckResourceAttr("radosgw_sns_topic.test", "verify_ssl", "true"),
			resource.TestCheckResourceAttr("radosgw_sns_topic.test", "cloudevents", "false"),
			resource.TestCheckResourceAttr("radosgw_sns_topic.test", "use_ssl", "false"),
		)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwSNSTopicDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwSNSTopicConfig_basic(topicName),
				Check:  resource.ComposeTestCheckFunc(checks...),
			},
			// Import by ARN
			{
				ResourceName:                         "radosgw_sns_topic.test",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateIdFunc:                    testAccRadosgwSNSTopicImportStateIDFunc("radosgw_sns_topic.test"),
				ImportStateVerifyIdentifierAttribute: "arn",
			},
		},
	})
}

func TestAccRadosgwSNSTopic_persistent(t *testing.T) {
	t.Parallel()

	topicName := randomName("tf-acc-topic")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwSNSTopicDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwSNSTopicConfig_persistent(topicName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwSNSTopicExists("radosgw_sns_topic.test"),
					resource.TestCheckResourceAttr("radosgw_sns_topic.test", "name", topicName),
					resource.TestCheckResourceAttr("radosgw_sns_topic.test", "persistent", "true"),
					resource.TestCheckResourceAttr("radosgw_sns_topic.test", "time_to_live", "300"),
					resource.TestCheckResourceAttr("radosgw_sns_topic.test", "max_retries", "5"),
					resource.TestCheckResourceAttr("radosgw_sns_topic.test", "retry_sleep_duration", "10"),
				),
			},
		},
	})
}

func TestAccRadosgwSNSTopic_withOpaqueData(t *testing.T) {
	t.Parallel()

	topicName := randomName("tf-acc-topic")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwSNSTopicDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwSNSTopicConfig_withOpaqueData(topicName, "my-opaque-data"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwSNSTopicExists("radosgw_sns_topic.test"),
					resource.TestCheckResourceAttr("radosgw_sns_topic.test", "opaque_data", "my-opaque-data"),
				),
			},
		},
	})
}

func TestAccRadosgwSNSTopic_withCloudEvents(t *testing.T) {
	t.Parallel()

	topicName := randomName("tf-acc-topic")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwSNSTopicDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwSNSTopicConfig_withCloudEvents(topicName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwSNSTopicExists("radosgw_sns_topic.test"),
					resource.TestCheckResourceAttr("radosgw_sns_topic.test", "cloudevents", "true"),
					resource.TestCheckResourceAttr("radosgw_sns_topic.test", "verify_ssl", "false"),
				),
			},
		},
	})
}

func TestAccRadosgwSNSTopic_update(t *testing.T) {
	t.Parallel()

	topicName := randomName("tf-acc-topic")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwSNSTopicDestroy,
		Steps: []resource.TestStep{
			// Step 1: basic topic
			{
				Config: testAccRadosgwSNSTopicConfig_basic(topicName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwSNSTopicExists("radosgw_sns_topic.test"),
					resource.TestCheckResourceAttr("radosgw_sns_topic.test", "persistent", "false"),
					resource.TestCheckResourceAttr("radosgw_sns_topic.test", "verify_ssl", "true"),
				),
			},
			// Step 2: update to persistent with opaque data and different SSL setting
			{
				Config: testAccRadosgwSNSTopicConfig_updated(topicName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwSNSTopicExists("radosgw_sns_topic.test"),
					resource.TestCheckResourceAttr("radosgw_sns_topic.test", "persistent", "true"),
					resource.TestCheckResourceAttr("radosgw_sns_topic.test", "opaque_data", "updated-data"),
					resource.TestCheckResourceAttr("radosgw_sns_topic.test", "verify_ssl", "false"),
					resource.TestCheckResourceAttr("radosgw_sns_topic.test", "time_to_live", "600"),
				),
			},
			// Step 3: revert to basic (remove persistence, opaque data)
			{
				Config: testAccRadosgwSNSTopicConfig_basic(topicName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwSNSTopicExists("radosgw_sns_topic.test"),
					resource.TestCheckResourceAttr("radosgw_sns_topic.test", "persistent", "false"),
					resource.TestCheckResourceAttr("radosgw_sns_topic.test", "verify_ssl", "true"),
					resource.TestCheckNoResourceAttr("radosgw_sns_topic.test", "opaque_data"),
				),
			},
		},
	})
}

// =============================================================================
// Test Check Functions
// =============================================================================

func testAccCheckRadosgwSNSTopicExists(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		arn := rs.Primary.Attributes["arn"]
		if arn == "" {
			return fmt.Errorf("no ARN set for %s", resourceName)
		}

		if testAccAdminClient == nil {
			return nil
		}

		iamClient := NewIAMClient(
			testAccAdminClient.Endpoint,
			testAccAdminClient.AccessKey,
			testAccAdminClient.SecretKey,
			testAccAdminClient.HTTPClient,
		)

		params := url.Values{}
		params.Set("Action", "GetTopicAttributes")
		params.Set("TopicArn", arn)

		_, err := iamClient.DoRequest(testCtx, params, "sns")
		if err != nil {
			return fmt.Errorf("error verifying SNS topic %s exists: %s", arn, err)
		}

		return nil
	}
}

func testAccCheckRadosgwSNSTopicDestroy(s *terraform.State) error {
	if testAccAdminClient == nil {
		return nil
	}

	iamClient := NewIAMClient(
		testAccAdminClient.Endpoint,
		testAccAdminClient.AccessKey,
		testAccAdminClient.SecretKey,
		testAccAdminClient.HTTPClient,
	)

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "radosgw_sns_topic" {
			continue
		}

		arn := rs.Primary.Attributes["arn"]
		if arn == "" {
			continue
		}

		params := url.Values{}
		params.Set("Action", "GetTopicAttributes")
		params.Set("TopicArn", arn)

		_, err := iamClient.DoRequest(testCtx, params, "sns")
		if err == nil {
			return fmt.Errorf("SNS topic %s still exists after destroy", arn)
		}

		if !isSNSTopicNotFound(err) {
			return fmt.Errorf("unexpected error checking topic %s after destroy: %s", arn, err)
		}
	}
	return nil
}

func testAccRadosgwSNSTopicImportStateIDFunc(resourceName string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return "", fmt.Errorf("resource not found: %s", resourceName)
		}
		return rs.Primary.Attributes["arn"], nil
	}
}

// =============================================================================
// Test Configurations
// =============================================================================

func testAccRadosgwSNSTopicConfig_basic(name string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_sns_topic" "test" {
  name          = %q
  push_endpoint = "http://localhost:10900"
}
`, name)
}

func testAccRadosgwSNSTopicConfig_persistent(name string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_sns_topic" "test" {
  name                 = %q
  push_endpoint        = "http://localhost:10900"
  persistent           = true
  time_to_live         = 300
  max_retries          = 5
  retry_sleep_duration = 10
}
`, name)
}

func testAccRadosgwSNSTopicConfig_withOpaqueData(name, opaqueData string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_sns_topic" "test" {
  name          = %q
  push_endpoint = "http://localhost:10900"
  opaque_data   = %q
}
`, name, opaqueData)
}

func testAccRadosgwSNSTopicConfig_withCloudEvents(name string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_sns_topic" "test" {
  name          = %q
  push_endpoint = "http://localhost:10900"
  cloudevents   = true
  verify_ssl    = false
}
`, name)
}

func testAccRadosgwSNSTopicConfig_updated(name string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_sns_topic" "test" {
  name          = %q
  push_endpoint = "http://localhost:10900"
  persistent    = true
  opaque_data   = "updated-data"
  verify_ssl    = false
  time_to_live  = 600
}
`, name)
}
