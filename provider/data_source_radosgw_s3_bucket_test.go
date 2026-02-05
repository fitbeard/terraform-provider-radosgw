package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccRadosgwS3BucketDataSource_basic(t *testing.T) {
	t.Parallel()

	bucketName := randomName("tf-acc-bucket")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwS3BucketDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwS3BucketDataSourceConfig_basic(bucketName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.radosgw_s3_bucket.test", "bucket", "radosgw_s3_bucket.test", "bucket"),
					resource.TestCheckResourceAttrSet("data.radosgw_s3_bucket.test", "owner"),
					resource.TestCheckResourceAttrSet("data.radosgw_s3_bucket.test", "creation_time"),
				),
			},
		},
	})
}

func TestAccRadosgwS3BucketDataSource_withVersioning(t *testing.T) {
	t.Parallel()

	bucketName := randomName("tf-acc-bucket")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwS3BucketDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwS3BucketDataSourceConfig_versioning(bucketName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.radosgw_s3_bucket.test", "versioning", "enabled"),
				),
			},
		},
	})
}

// Test configurations

func testAccRadosgwS3BucketDataSourceConfig_basic(bucketName string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_s3_bucket" "test" {
  bucket = %q
}

data "radosgw_s3_bucket" "test" {
  bucket = radosgw_s3_bucket.test.bucket

  depends_on = [radosgw_s3_bucket.test]
}
`, bucketName)
}

func testAccRadosgwS3BucketDataSourceConfig_versioning(bucketName string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_s3_bucket" "test" {
  bucket     = %q
  versioning = "enabled"
}

data "radosgw_s3_bucket" "test" {
  bucket = radosgw_s3_bucket.test.bucket

  depends_on = [radosgw_s3_bucket.test]
}
`, bucketName)
}
