package provider

import (
	"encoding/xml"
	"fmt"
	"net/url"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccRadosgwSNSTopicPolicy_basic(t *testing.T) {
	t.Parallel()

	topicName := randomName("tf-acc-policy")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwSNSTopicPolicyDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwSNSTopicPolicyConfig_basic(topicName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwSNSTopicPolicyExists("radosgw_sns_topic_policy.test"),
					resource.TestCheckResourceAttrSet("radosgw_sns_topic_policy.test", "arn"),
					resource.TestCheckResourceAttrSet("radosgw_sns_topic_policy.test", "policy"),
					resource.TestCheckResourceAttrSet("radosgw_sns_topic_policy.test", "owner"),
				),
			},
			// Import by ARN
			{
				ResourceName:                         "radosgw_sns_topic_policy.test",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateIdFunc:                    testAccRadosgwSNSTopicPolicyImportStateIDFunc("radosgw_sns_topic_policy.test"),
				ImportStateVerifyIdentifierAttribute: "arn",
			},
		},
	})
}

func TestAccRadosgwSNSTopicPolicy_update(t *testing.T) {
	t.Parallel()

	topicName := randomName("tf-acc-policy")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwSNSTopicPolicyDestroy,
		Steps: []resource.TestStep{
			// Step 1: create with basic policy
			{
				Config: testAccRadosgwSNSTopicPolicyConfig_basic(topicName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwSNSTopicPolicyExists("radosgw_sns_topic_policy.test"),
				),
			},
			// Step 2: update with a different policy
			{
				Config: testAccRadosgwSNSTopicPolicyConfig_updated(topicName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwSNSTopicPolicyExists("radosgw_sns_topic_policy.test"),
				),
			},
		},
	})
}

func TestAccRadosgwSNSTopicPolicy_disappears(t *testing.T) {
	t.Parallel()

	topicName := randomName("tf-acc-policy")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwSNSTopicPolicyDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwSNSTopicPolicyConfig_basic(topicName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwSNSTopicPolicyExists("radosgw_sns_topic_policy.test"),
					// Delete the underlying topic to simulate disappearance
					testAccDeleteSNSTopic("radosgw_sns_topic.test"),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// =============================================================================
// Test Check Functions
// =============================================================================

func testAccCheckRadosgwSNSTopicPolicyExists(resourceName string) resource.TestCheckFunc {
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

		body, err := iamClient.DoPostRequest(testCtx, params, "sns")
		if err != nil {
			return fmt.Errorf("error verifying SNS topic policy %s exists: %s", arn, err)
		}

		// Parse and check that Policy is non-empty
		attrs, err := parseSNSTopicAttributesMap(body)
		if err != nil {
			return fmt.Errorf("error parsing topic attributes for %s: %s", arn, err)
		}

		policy := attrs["Policy"]
		if policy == "" {
			return fmt.Errorf("SNS topic %s has no policy set", arn)
		}

		return nil
	}
}

func testAccCheckRadosgwSNSTopicPolicyDestroy(s *terraform.State) error {
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
		if rs.Type != "radosgw_sns_topic_policy" {
			continue
		}

		arn := rs.Primary.Attributes["arn"]
		if arn == "" {
			continue
		}

		params := url.Values{}
		params.Set("Action", "GetTopicAttributes")
		params.Set("TopicArn", arn)

		body, err := iamClient.DoPostRequest(testCtx, params, "sns")
		if err != nil {
			// Topic itself is gone — policy is destroyed
			if isSNSTopicNotFound(err) {
				continue
			}
			return fmt.Errorf("unexpected error checking topic %s after destroy: %s", arn, err)
		}

		// Topic exists — check that policy is empty
		attrs, err := parseSNSTopicAttributesMap(body)
		if err != nil {
			return fmt.Errorf("error parsing topic attributes for %s: %s", arn, err)
		}

		policy := attrs["Policy"]
		if policy != "" {
			return fmt.Errorf("SNS topic policy %s still has a non-empty policy after destroy: %s", arn, policy)
		}
	}
	return nil
}

func testAccDeleteSNSTopic(resourceName string) resource.TestCheckFunc {
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
		params.Set("Action", "DeleteTopic")
		params.Set("TopicArn", arn)

		_, err := iamClient.DoPostRequest(testCtx, params, "sns")
		return err
	}
}

func testAccRadosgwSNSTopicPolicyImportStateIDFunc(resourceName string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return "", fmt.Errorf("resource not found: %s", resourceName)
		}
		return rs.Primary.Attributes["arn"], nil
	}
}

// parseSNSTopicAttributesMap is a test helper that parses GetTopicAttributes XML
// response into a string map.
func parseSNSTopicAttributesMap(body []byte) (map[string]string, error) {
	var response getTopicAttributesResponseXML
	if err := xml.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	attrs := make(map[string]string)
	for _, entry := range response.Result.Attributes.Entries {
		attrs[entry.Key] = entry.Value
	}
	return attrs, nil
}

// =============================================================================
// Test Configurations
// =============================================================================

func testAccRadosgwSNSTopicPolicyConfig_basic(name string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_sns_topic" "test" {
  name          = %[1]q
  push_endpoint = "http://localhost:10900"
}

data "radosgw_iam_policy_document" "test" {
  statement {
    effect = "Allow"

    principals {
      type        = "AWS"
      identifiers = ["arn:aws:iam::testuser:user/testuser"]
    }

    actions   = ["sns:GetTopicAttributes"]
    resources = [radosgw_sns_topic.test.arn]
  }
}

resource "radosgw_sns_topic_policy" "test" {
  arn    = radosgw_sns_topic.test.arn
  policy = data.radosgw_iam_policy_document.test.json
}
`, name)
}

func testAccRadosgwSNSTopicPolicyConfig_updated(name string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_sns_topic" "test" {
  name          = %[1]q
  push_endpoint = "http://localhost:10900"
}

data "radosgw_iam_policy_document" "test" {
  statement {
    effect = "Allow"

    principals {
      type        = "AWS"
      identifiers = ["arn:aws:iam::testuser:user/testuser"]
    }

    actions   = ["sns:GetTopicAttributes", "sns:Publish"]
    resources = [radosgw_sns_topic.test.arn]
  }
}

resource "radosgw_sns_topic_policy" "test" {
  arn    = radosgw_sns_topic.test.arn
  policy = data.radosgw_iam_policy_document.test.json
}
`, name)
}
