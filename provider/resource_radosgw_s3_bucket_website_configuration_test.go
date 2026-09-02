package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccRadosgwS3BucketWebsiteConfiguration_basic(t *testing.T) {
	t.Parallel()

	bucketName := randomName("tf-acc-bucket")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwS3BucketDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwS3BucketWebsiteConfigurationConfig_basic(bucketName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_s3_bucket_website_configuration.test", "bucket", bucketName),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_website_configuration.test", "index_document.#", "1"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_website_configuration.test", "index_document.0.suffix", "index.html"),
				),
			},
			// Import test - by bucket name
			{
				ResourceName:                         "radosgw_s3_bucket_website_configuration.test",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateId:                        bucketName,
				ImportStateVerifyIdentifierAttribute: "bucket",
			},
		},
	})
}

func TestAccRadosgwS3BucketWebsiteConfiguration_withErrorDocument(t *testing.T) {
	t.Parallel()

	bucketName := randomName("tf-acc-bucket")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwS3BucketDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwS3BucketWebsiteConfigurationConfig_withErrorDocument(bucketName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_s3_bucket_website_configuration.test", "bucket", bucketName),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_website_configuration.test", "index_document.0.suffix", "index.html"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_website_configuration.test", "error_document.#", "1"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_website_configuration.test", "error_document.0.key", "error.html"),
				),
			},
		},
	})
}

func TestAccRadosgwS3BucketWebsiteConfiguration_redirectAllRequests(t *testing.T) {
	t.Parallel()

	bucketName := randomName("tf-acc-bucket")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwS3BucketDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwS3BucketWebsiteConfigurationConfig_redirectAll(bucketName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_s3_bucket_website_configuration.test", "bucket", bucketName),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_website_configuration.test", "redirect_all_requests_to.#", "1"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_website_configuration.test", "redirect_all_requests_to.0.host_name", "example.com"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_website_configuration.test", "redirect_all_requests_to.0.protocol", "https"),
				),
			},
			// Import test
			{
				ResourceName:                         "radosgw_s3_bucket_website_configuration.test",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateId:                        bucketName,
				ImportStateVerifyIdentifierAttribute: "bucket",
			},
		},
	})
}

func TestAccRadosgwS3BucketWebsiteConfiguration_routingRule(t *testing.T) {
	t.Parallel()

	bucketName := randomName("tf-acc-bucket")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwS3BucketDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwS3BucketWebsiteConfigurationConfig_routingRule(bucketName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_s3_bucket_website_configuration.test", "bucket", bucketName),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_website_configuration.test", "index_document.0.suffix", "index.html"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_website_configuration.test", "routing_rule.#", "1"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_website_configuration.test", "routing_rule.0.condition.0.key_prefix_equals", "docs/"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_website_configuration.test", "routing_rule.0.redirect.0.replace_key_prefix_with", "documents/"),
				),
			},
		},
	})
}

func TestAccRadosgwS3BucketWebsiteConfiguration_routingRuleMultiple(t *testing.T) {
	t.Parallel()

	bucketName := randomName("tf-acc-bucket")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwS3BucketDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwS3BucketWebsiteConfigurationConfig_routingRuleMultiple(bucketName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_s3_bucket_website_configuration.test", "routing_rule.#", "2"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_website_configuration.test", "routing_rule.0.condition.0.http_error_code_returned_equals", "404"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_website_configuration.test", "routing_rule.0.redirect.0.replace_key_with", "error.html"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_website_configuration.test", "routing_rule.1.condition.0.key_prefix_equals", "old/"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_website_configuration.test", "routing_rule.1.redirect.0.replace_key_prefix_with", "new/"),
				),
			},
		},
	})
}

func TestAccRadosgwS3BucketWebsiteConfiguration_routingRuleRedirectOnly(t *testing.T) {
	t.Parallel()

	bucketName := randomName("tf-acc-bucket")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwS3BucketDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwS3BucketWebsiteConfigurationConfig_routingRuleRedirectOnly(bucketName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_s3_bucket_website_configuration.test", "routing_rule.#", "1"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_website_configuration.test", "routing_rule.0.redirect.0.host_name", "backup.example.com"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_website_configuration.test", "routing_rule.0.redirect.0.protocol", "https"),
				),
			},
		},
	})
}

func TestAccRadosgwS3BucketWebsiteConfiguration_update(t *testing.T) {
	t.Parallel()

	bucketName := randomName("tf-acc-bucket")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwS3BucketDestroy,
		Steps: []resource.TestStep{
			// Step 1: Create with index document only
			{
				Config: testAccRadosgwS3BucketWebsiteConfigurationConfig_basic(bucketName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_s3_bucket_website_configuration.test", "index_document.0.suffix", "index.html"),
					resource.TestCheckNoResourceAttr("radosgw_s3_bucket_website_configuration.test", "error_document.0.key"),
				),
			},
			// Step 2: Add error document
			{
				Config: testAccRadosgwS3BucketWebsiteConfigurationConfig_withErrorDocument(bucketName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_s3_bucket_website_configuration.test", "index_document.0.suffix", "index.html"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_website_configuration.test", "error_document.0.key", "error.html"),
				),
			},
			// Step 3: Update index document suffix
			{
				Config: testAccRadosgwS3BucketWebsiteConfigurationConfig_updatedSuffix(bucketName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_s3_bucket_website_configuration.test", "index_document.0.suffix", "default.html"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_website_configuration.test", "error_document.0.key", "404.html"),
				),
			},
		},
	})
}

// =============================================================================
// Config Helpers
// =============================================================================

func testAccRadosgwS3BucketWebsiteConfigurationConfig_basic(bucketName string) string {
	return fmt.Sprintf(`
%s

resource "radosgw_s3_bucket" "test" {
  bucket = %q
}

resource "radosgw_s3_bucket_website_configuration" "test" {
  bucket = radosgw_s3_bucket.test.bucket

  index_document {
    suffix = "index.html"
  }
}
`, providerConfig(), bucketName)
}

func testAccRadosgwS3BucketWebsiteConfigurationConfig_withErrorDocument(bucketName string) string {
	return fmt.Sprintf(`
%s

resource "radosgw_s3_bucket" "test" {
  bucket = %q
}

resource "radosgw_s3_bucket_website_configuration" "test" {
  bucket = radosgw_s3_bucket.test.bucket

  index_document {
    suffix = "index.html"
  }

  error_document {
    key = "error.html"
  }
}
`, providerConfig(), bucketName)
}

func testAccRadosgwS3BucketWebsiteConfigurationConfig_updatedSuffix(bucketName string) string {
	return fmt.Sprintf(`
%s

resource "radosgw_s3_bucket" "test" {
  bucket = %q
}

resource "radosgw_s3_bucket_website_configuration" "test" {
  bucket = radosgw_s3_bucket.test.bucket

  index_document {
    suffix = "default.html"
  }

  error_document {
    key = "404.html"
  }
}
`, providerConfig(), bucketName)
}

func testAccRadosgwS3BucketWebsiteConfigurationConfig_redirectAll(bucketName string) string {
	return fmt.Sprintf(`
%s

resource "radosgw_s3_bucket" "test" {
  bucket = %q
}

resource "radosgw_s3_bucket_website_configuration" "test" {
  bucket = radosgw_s3_bucket.test.bucket

  redirect_all_requests_to {
    host_name = "example.com"
    protocol  = "https"
  }
}
`, providerConfig(), bucketName)
}

func testAccRadosgwS3BucketWebsiteConfigurationConfig_routingRule(bucketName string) string {
	return fmt.Sprintf(`
%s

resource "radosgw_s3_bucket" "test" {
  bucket = %q
}

resource "radosgw_s3_bucket_website_configuration" "test" {
  bucket = radosgw_s3_bucket.test.bucket

  index_document {
    suffix = "index.html"
  }

  routing_rule {
    condition {
      key_prefix_equals = "docs/"
    }
    redirect {
      replace_key_prefix_with = "documents/"
    }
  }
}
`, providerConfig(), bucketName)
}

func testAccRadosgwS3BucketWebsiteConfigurationConfig_routingRuleMultiple(bucketName string) string {
	return fmt.Sprintf(`
%s

resource "radosgw_s3_bucket" "test" {
  bucket = %q
}

resource "radosgw_s3_bucket_website_configuration" "test" {
  bucket = radosgw_s3_bucket.test.bucket

  index_document {
    suffix = "index.html"
  }

  routing_rule {
    condition {
      http_error_code_returned_equals = "404"
    }
    redirect {
      replace_key_with = "error.html"
    }
  }

  routing_rule {
    condition {
      key_prefix_equals = "old/"
    }
    redirect {
      replace_key_prefix_with = "new/"
    }
  }
}
`, providerConfig(), bucketName)
}

func testAccRadosgwS3BucketWebsiteConfigurationConfig_routingRuleRedirectOnly(bucketName string) string {
	return fmt.Sprintf(`
%s

resource "radosgw_s3_bucket" "test" {
  bucket = %q
}

resource "radosgw_s3_bucket_website_configuration" "test" {
  bucket = radosgw_s3_bucket.test.bucket

  index_document {
    suffix = "index.html"
  }

  routing_rule {
    redirect {
      host_name = "backup.example.com"
      protocol  = "https"
    }
  }
}
`, providerConfig(), bucketName)
}
