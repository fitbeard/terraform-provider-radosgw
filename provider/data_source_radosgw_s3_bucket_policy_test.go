package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccRadosgwS3BucketPolicyDataSource_basic(t *testing.T) {
	t.Parallel()

	bucketName := randomName("tf-acc-bucket")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwS3BucketDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwS3BucketPolicyDataSourceConfig_basic(bucketName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.radosgw_s3_bucket_policy.test", "bucket", bucketName),
					resource.TestCheckResourceAttrSet("data.radosgw_s3_bucket_policy.test", "policy"),
				),
			},
		},
	})
}

// Test configurations

func testAccRadosgwS3BucketPolicyDataSourceConfig_basic(bucketName string) string {
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
        Sid       = "PublicReadGetObject"
        Effect    = "Allow"
        Principal = "*"
        Action    = ["s3:GetObject"]
        Resource  = ["arn:aws:s3:::%s/*"]
      }
    ]
  })
}

data "radosgw_s3_bucket_policy" "test" {
  bucket = radosgw_s3_bucket.test.bucket

  depends_on = [radosgw_s3_bucket_policy.test]
}
`, bucketName, bucketName)
}
