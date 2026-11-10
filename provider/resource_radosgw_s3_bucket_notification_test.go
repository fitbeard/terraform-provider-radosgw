package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccRadosgwS3BucketNotification_basic(t *testing.T) {
	t.Parallel()

	bucketName := randomName("tf-acc-bucket")
	topicName := randomName("tf-acc-topic")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwS3BucketDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwS3BucketNotificationConfig_basic(bucketName, topicName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_s3_bucket_notification.test", "bucket", bucketName),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_notification.test", "topic.#", "1"),
					resource.TestCheckResourceAttrSet("radosgw_s3_bucket_notification.test", "topic.0.topic_arn"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_notification.test", "topic.0.events.#", "1"),
				),
			},
			// Import test - by bucket name
			{
				ResourceName:                         "radosgw_s3_bucket_notification.test",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateId:                        bucketName,
				ImportStateVerifyIdentifierAttribute: "bucket",
			},
		},
	})
}

func TestAccRadosgwS3BucketNotification_withFilters(t *testing.T) {
	t.Parallel()

	bucketName := randomName("tf-acc-bucket")
	topicName := randomName("tf-acc-topic")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwS3BucketDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwS3BucketNotificationConfig_withFilters(bucketName, topicName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_s3_bucket_notification.test", "bucket", bucketName),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_notification.test", "topic.#", "1"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_notification.test", "topic.0.filter_prefix", "images/"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_notification.test", "topic.0.filter_suffix", ".jpg"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_notification.test", "topic.0.events.#", "2"),
				),
			},
		},
	})
}

func TestAccRadosgwS3BucketNotification_multipleTopics(t *testing.T) {
	t.Parallel()

	bucketName := randomName("tf-acc-bucket")
	topicName1 := randomName("tf-acc-topic")
	topicName2 := randomName("tf-acc-topic")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwS3BucketDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwS3BucketNotificationConfig_multipleTopics(bucketName, topicName1, topicName2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_s3_bucket_notification.test", "bucket", bucketName),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_notification.test", "topic.#", "2"),
				),
			},
		},
	})
}

func TestAccRadosgwS3BucketNotification_update(t *testing.T) {
	t.Parallel()

	bucketName := randomName("tf-acc-bucket")
	topicName1 := randomName("tf-acc-topic")
	topicName2 := randomName("tf-acc-topic")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwS3BucketDestroy,
		Steps: []resource.TestStep{
			// Step 1: Create with one topic
			{
				Config: testAccRadosgwS3BucketNotificationConfig_basic(bucketName, topicName1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_s3_bucket_notification.test", "topic.#", "1"),
					resource.TestCheckResourceAttrSet("radosgw_s3_bucket_notification.test", "topic.0.topic_arn"),
				),
			},
			// Step 2: Update to a different topic
			{
				Config: testAccRadosgwS3BucketNotificationConfig_basic(bucketName, topicName2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_s3_bucket_notification.test", "topic.#", "1"),
					resource.TestCheckResourceAttrSet("radosgw_s3_bucket_notification.test", "topic.0.topic_arn"),
				),
			},
			// Step 3: Update to multiple topics
			{
				Config: testAccRadosgwS3BucketNotificationConfig_multipleTopics(bucketName, topicName1, topicName2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_s3_bucket_notification.test", "topic.#", "2"),
				),
			},
		},
	})
}

func TestAccRadosgwS3BucketNotification_withID(t *testing.T) {
	t.Parallel()

	bucketName := randomName("tf-acc-bucket")
	topicName := randomName("tf-acc-topic")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwS3BucketDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwS3BucketNotificationConfig_withID(bucketName, topicName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_s3_bucket_notification.test", "bucket", bucketName),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_notification.test", "topic.0.id", "my-notification"),
				),
			},
		},
	})
}

// =============================================================================
// Config Helpers
// =============================================================================

func testAccRadosgwS3BucketNotificationConfig_basic(bucketName, topicName string) string {
	return fmt.Sprintf(`
%s

resource "radosgw_s3_bucket" "test" {
  bucket = %q
}

resource "radosgw_sns_topic" "test" {
  name          = %q
  push_endpoint = "http://localhost:10900"
}

resource "radosgw_s3_bucket_notification" "test" {
  bucket = radosgw_s3_bucket.test.bucket

  topic {
    topic_arn = radosgw_sns_topic.test.arn
    events    = ["s3:ObjectCreated:*"]
  }
}
`, providerConfig(), bucketName, topicName)
}

func testAccRadosgwS3BucketNotificationConfig_withFilters(bucketName, topicName string) string {
	return fmt.Sprintf(`
%s

resource "radosgw_s3_bucket" "test" {
  bucket = %q
}

resource "radosgw_sns_topic" "test" {
  name          = %q
  push_endpoint = "http://localhost:10900"
}

resource "radosgw_s3_bucket_notification" "test" {
  bucket = radosgw_s3_bucket.test.bucket

  topic {
    topic_arn     = radosgw_sns_topic.test.arn
    events        = ["s3:ObjectCreated:*", "s3:ObjectRemoved:*"]
    filter_prefix = "images/"
    filter_suffix = ".jpg"
  }
}
`, providerConfig(), bucketName, topicName)
}

func testAccRadosgwS3BucketNotificationConfig_multipleTopics(bucketName, topicName1, topicName2 string) string {
	return fmt.Sprintf(`
%s

resource "radosgw_s3_bucket" "test" {
  bucket = %q
}

resource "radosgw_sns_topic" "test1" {
  name          = %q
  push_endpoint = "http://localhost:10900"
}

resource "radosgw_sns_topic" "test2" {
  name          = %q
  push_endpoint = "http://localhost:10901"
}

resource "radosgw_s3_bucket_notification" "test" {
  bucket = radosgw_s3_bucket.test.bucket

  topic {
    id        = "created-events"
    topic_arn = radosgw_sns_topic.test1.arn
    events    = ["s3:ObjectCreated:*"]
  }

  topic {
    id        = "removed-events"
    topic_arn = radosgw_sns_topic.test2.arn
    events    = ["s3:ObjectRemoved:*"]
  }
}
`, providerConfig(), bucketName, topicName1, topicName2)
}

func testAccRadosgwS3BucketNotificationConfig_withID(bucketName, topicName string) string {
	return fmt.Sprintf(`
%s

resource "radosgw_s3_bucket" "test" {
  bucket = %q
}

resource "radosgw_sns_topic" "test" {
  name          = %q
  push_endpoint = "http://localhost:10900"
}

resource "radosgw_s3_bucket_notification" "test" {
  bucket = radosgw_s3_bucket.test.bucket

  topic {
    id        = "my-notification"
    topic_arn = radosgw_sns_topic.test.arn
    events    = ["s3:ObjectCreated:*"]
  }
}
`, providerConfig(), bucketName, topicName)
}
