package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// Note: Bucket link tests use unlink_to_uid="admin" to transfer ownership back
// to the admin user on destroy, so the bucket can be properly cleaned up.
// Without this, the admin user loses access to the bucket after linking it
// to another user, and cleanup fails with AccessDenied.

func TestAccRadosgwS3BucketLink_basic(t *testing.T) {
	t.Parallel()

	bucketName := randomName("tf-acc-bucket")
	userID := randomName("tf-acc-user")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwS3BucketDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwS3BucketLinkConfig_basic(bucketName, userID),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_s3_bucket_link.test", "bucket", bucketName),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_link.test", "uid", userID),
					resource.TestCheckResourceAttrSet("radosgw_s3_bucket_link.test", "bucket_id"),
				),
			},
		},
	})
}

func TestAccRadosgwS3BucketLink_import(t *testing.T) {
	t.Parallel()

	bucketName := randomName("tf-acc-bucket")
	userID := randomName("tf-acc-user")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwS3BucketDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwS3BucketLinkConfig_basic(bucketName, userID),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_s3_bucket_link.test", "bucket", bucketName),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_link.test", "uid", userID),
				),
			},
			// Import test - format: bucket:uid
			{
				ResourceName:                         "radosgw_s3_bucket_link.test",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateVerifyIgnore:              []string{"unlink_to_uid"},
				ImportStateId:                        bucketName + ":" + userID,
				ImportStateVerifyIdentifierAttribute: "bucket",
			},
		},
	})
}

// Test configurations

func testAccRadosgwS3BucketLinkConfig_basic(bucketName, userID string) string {
	// Uses unlink_to_uid="admin" to transfer ownership back to admin on destroy
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_user" "test" {
  user_id      = %q
  display_name = "Test User for Bucket Link"
}

resource "radosgw_s3_bucket" "test" {
  bucket = %q
}

resource "radosgw_s3_bucket_link" "test" {
  bucket        = radosgw_s3_bucket.test.bucket
  uid           = radosgw_iam_user.test.user_id
  unlink_to_uid = "admin"
}
`, userID, bucketName)
}
