package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccRadosgwS3BucketAcl_basic(t *testing.T) {
	t.Parallel()

	bucketName := randomName("tf-acc-bucket")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwS3BucketDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwS3BucketAclConfig_basic(bucketName, "private"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_s3_bucket_acl.test", "bucket", bucketName),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_acl.test", "acl", "private"),
				),
			},
			// Import test - by bucket name
			{
				ResourceName:                         "radosgw_s3_bucket_acl.test",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateId:                        bucketName,
				ImportStateVerifyIdentifierAttribute: "bucket",
			},
		},
	})
}

func TestAccRadosgwS3BucketAcl_update(t *testing.T) {
	t.Parallel()

	bucketName := randomName("tf-acc-bucket")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwS3BucketDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwS3BucketAclConfig_basic(bucketName, "private"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_s3_bucket_acl.test", "acl", "private"),
				),
			},
			{
				Config: testAccRadosgwS3BucketAclConfig_basic(bucketName, "public-read"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_s3_bucket_acl.test", "acl", "public-read"),
				),
			},
			{
				Config: testAccRadosgwS3BucketAclConfig_basic(bucketName, "public-read-write"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_s3_bucket_acl.test", "acl", "public-read-write"),
				),
			},
			{
				Config: testAccRadosgwS3BucketAclConfig_basic(bucketName, "authenticated-read"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_s3_bucket_acl.test", "acl", "authenticated-read"),
				),
			},
		},
	})
}

// Test configurations

func testAccRadosgwS3BucketAclConfig_basic(bucketName, acl string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_s3_bucket" "test" {
  bucket = %q
}

resource "radosgw_s3_bucket_acl" "test" {
  bucket = radosgw_s3_bucket.test.bucket
  acl    = %q
}
`, bucketName, acl)
}
