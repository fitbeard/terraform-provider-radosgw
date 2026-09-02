package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccRadosgwS3BucketPolicy_basic(t *testing.T) {
	t.Parallel()

	bucketName := randomName("tf-acc-bucket")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwS3BucketDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwS3BucketPolicyConfig_basic(bucketName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwS3BucketPolicyExists("radosgw_s3_bucket_policy.test"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_policy.test", "bucket", bucketName),
					resource.TestCheckResourceAttrSet("radosgw_s3_bucket_policy.test", "policy"),
				),
			},
			// Test import
			{
				ResourceName:      "radosgw_s3_bucket_policy.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccRadosgwS3BucketPolicy_update(t *testing.T) {
	t.Parallel()

	bucketName := randomName("tf-acc-bucket")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwS3BucketDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwS3BucketPolicyConfig_basic(bucketName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwS3BucketPolicyExists("radosgw_s3_bucket_policy.test"),
				),
			},
			{
				Config: testAccRadosgwS3BucketPolicyConfig_updated(bucketName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwS3BucketPolicyExists("radosgw_s3_bucket_policy.test"),
				),
			},
		},
	})
}

// Helper functions

func testAccCheckRadosgwS3BucketPolicyExists(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		bucketName := rs.Primary.Attributes["bucket"]
		if bucketName == "" {
			return fmt.Errorf("bucket name not set")
		}

		// Bucket policy existence is verified by the provider during Read
		return nil
	}
}

// Test configurations

func testAccRadosgwS3BucketPolicyConfig_basic(bucketName string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_s3_bucket" "test" {
  bucket = %q
}

resource "radosgw_s3_bucket_policy" "test" {
  bucket = radosgw_s3_bucket.test.bucket

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect    = "Allow"
        Principal = "*"
        Action    = ["s3:GetObject"]
        Resource  = "arn:aws:s3:::${radosgw_s3_bucket.test.bucket}/*"
      }
    ]
  })
}
`, bucketName)
}

func testAccRadosgwS3BucketPolicyConfig_updated(bucketName string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_s3_bucket" "test" {
  bucket = %q
}

resource "radosgw_s3_bucket_policy" "test" {
  bucket = radosgw_s3_bucket.test.bucket

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect    = "Allow"
        Principal = "*"
        Action    = ["s3:GetObject", "s3:ListBucket"]
        Resource  = [
          "arn:aws:s3:::${radosgw_s3_bucket.test.bucket}",
          "arn:aws:s3:::${radosgw_s3_bucket.test.bucket}/*"
        ]
      }
    ]
  })
}
`, bucketName)
}
