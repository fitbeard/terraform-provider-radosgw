package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccRadosgwS3BucketLifecycleConfiguration_basic(t *testing.T) {
	t.Parallel()

	bucketName := randomName("tf-acc-bucket")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwS3BucketDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwS3BucketLifecycleConfigurationConfig_basic(bucketName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_s3_bucket_lifecycle_configuration.test", "bucket", bucketName),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_lifecycle_configuration.test", "rule.#", "1"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_lifecycle_configuration.test", "rule.0.id", "expire-old-objects"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_lifecycle_configuration.test", "rule.0.status", "Enabled"),
				),
			},
			// Import test - by bucket name
			{
				ResourceName:                         "radosgw_s3_bucket_lifecycle_configuration.test",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateId:                        bucketName,
				ImportStateVerifyIdentifierAttribute: "bucket",
			},
		},
	})
}

func TestAccRadosgwS3BucketLifecycleConfiguration_withPrefix(t *testing.T) {
	t.Parallel()

	bucketName := randomName("tf-acc-bucket")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwS3BucketDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwS3BucketLifecycleConfigurationConfig_withPrefix(bucketName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_s3_bucket_lifecycle_configuration.test", "bucket", bucketName),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_lifecycle_configuration.test", "rule.0.filter.0.prefix", "logs/"),
				),
			},
		},
	})
}

func TestAccRadosgwS3BucketLifecycleConfiguration_multipleRules(t *testing.T) {
	t.Parallel()

	bucketName := randomName("tf-acc-bucket")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwS3BucketDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwS3BucketLifecycleConfigurationConfig_multipleRules(bucketName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_s3_bucket_lifecycle_configuration.test", "bucket", bucketName),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_lifecycle_configuration.test", "rule.#", "2"),
				),
			},
		},
	})
}

func TestAccRadosgwS3BucketLifecycleConfiguration_abortIncompleteMultipartUpload(t *testing.T) {
	t.Parallel()

	bucketName := randomName("tf-acc-bucket")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwS3BucketDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwS3BucketLifecycleConfigurationConfig_abortIncompleteMultipartUpload(bucketName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_s3_bucket_lifecycle_configuration.test", "bucket", bucketName),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_lifecycle_configuration.test", "rule.0.abort_incomplete_multipart_upload.0.days_after_initiation", "7"),
				),
			},
		},
	})
}

func TestAccRadosgwS3BucketLifecycleConfiguration_update(t *testing.T) {
	t.Parallel()

	bucketName := randomName("tf-acc-bucket")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwS3BucketDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwS3BucketLifecycleConfigurationConfig_basic(bucketName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_s3_bucket_lifecycle_configuration.test", "rule.0.expiration.0.days", "30"),
				),
			},
			{
				Config: testAccRadosgwS3BucketLifecycleConfigurationConfig_updated(bucketName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_s3_bucket_lifecycle_configuration.test", "rule.0.expiration.0.days", "60"),
				),
			},
		},
	})
}

// Test configurations

func testAccRadosgwS3BucketLifecycleConfigurationConfig_basic(bucketName string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_s3_bucket" "test" {
  bucket = %q
}

resource "radosgw_s3_bucket_lifecycle_configuration" "test" {
  bucket = radosgw_s3_bucket.test.bucket

  rule {
    id     = "expire-old-objects"
    status = "Enabled"

    expiration {
      days = 30
    }
  }
}
`, bucketName)
}

func testAccRadosgwS3BucketLifecycleConfigurationConfig_updated(bucketName string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_s3_bucket" "test" {
  bucket = %q
}

resource "radosgw_s3_bucket_lifecycle_configuration" "test" {
  bucket = radosgw_s3_bucket.test.bucket

  rule {
    id     = "expire-old-objects"
    status = "Enabled"

    expiration {
      days = 60
    }
  }
}
`, bucketName)
}

func testAccRadosgwS3BucketLifecycleConfigurationConfig_withPrefix(bucketName string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_s3_bucket" "test" {
  bucket = %q
}

resource "radosgw_s3_bucket_lifecycle_configuration" "test" {
  bucket = radosgw_s3_bucket.test.bucket

  rule {
    id     = "expire-logs"
    status = "Enabled"

    filter {
      prefix = "logs/"
    }

    expiration {
      days = 7
    }
  }
}
`, bucketName)
}

func testAccRadosgwS3BucketLifecycleConfigurationConfig_multipleRules(bucketName string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_s3_bucket" "test" {
  bucket = %q
}

resource "radosgw_s3_bucket_lifecycle_configuration" "test" {
  bucket = radosgw_s3_bucket.test.bucket

  rule {
    id     = "expire-logs"
    status = "Enabled"

    filter {
      prefix = "logs/"
    }

    expiration {
      days = 7
    }
  }

  rule {
    id     = "expire-temp"
    status = "Enabled"

    filter {
      prefix = "temp/"
    }

    expiration {
      days = 1
    }
  }
}
`, bucketName)
}

func testAccRadosgwS3BucketLifecycleConfigurationConfig_abortIncompleteMultipartUpload(bucketName string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_s3_bucket" "test" {
  bucket = %q
}

resource "radosgw_s3_bucket_lifecycle_configuration" "test" {
  bucket = radosgw_s3_bucket.test.bucket

  rule {
    id     = "abort-incomplete-uploads"
    status = "Enabled"

    abort_incomplete_multipart_upload {
      days_after_initiation = 7
    }
  }
}
`, bucketName)
}
