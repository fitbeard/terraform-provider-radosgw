package provider

import (
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/ceph/go-ceph/rgw/admin"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAccRadosgwS3BucketDataSource_caplessUser verifies the data source works for
// a user without admin caps: it reads the bucket over the S3 API,
// populating owner/creation_time/versioning, with admin-only fields null.
func TestAccRadosgwS3BucketDataSource_caplessUser(t *testing.T) {
	t.Parallel()

	bucketName := randomName("tf-acc-bucket")
	uid := randomName("tf-acc-dscapless")
	accessKey := strings.ReplaceAll(randomName("dscaplesskey"), "-", "")
	secretKey := strings.ReplaceAll(randomName("dscaplesssecret"), "-", "")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(_ *terraform.State) error {
			_, _ = testAccS3ClientWithCreds(accessKey, secretKey).DeleteBucket(testCtx, &s3.DeleteBucketInput{Bucket: aws.String(bucketName)})
			_ = testAccAdminClient.RemoveUser(testCtx, admin.User{ID: uid})
			return nil
		},
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					if _, err := testAccAdminClient.CreateUser(testCtx, admin.User{ID: uid, DisplayName: uid}); err != nil {
						t.Fatalf("failed to create capless user %s: %s", uid, err)
					}
					gen := false
					if _, err := testAccAdminClient.CreateKey(testCtx, admin.UserKeySpec{UID: uid, AccessKey: accessKey, SecretKey: secretKey, GenerateKey: &gen}); err != nil {
						t.Fatalf("failed to add key to capless user %s: %s", uid, err)
					}
					// Bucket owned by the capless user, created over S3.
					if _, err := testAccS3ClientWithCreds(accessKey, secretKey).CreateBucket(testCtx, &s3.CreateBucketInput{Bucket: aws.String(bucketName)}); err != nil {
						t.Fatalf("failed to create bucket %s: %s", bucketName, err)
					}
				},
				Config: testAccRadosgwS3BucketDataSourceConfig_capless(accessKey, secretKey, bucketName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.radosgw_s3_bucket.test", "bucket", bucketName),
					resource.TestCheckResourceAttrSet("data.radosgw_s3_bucket.test", "owner"),
					resource.TestCheckResourceAttrSet("data.radosgw_s3_bucket.test", "creation_time"),
					resource.TestCheckNoResourceAttr("data.radosgw_s3_bucket.test", "num_shards"),
				),
			},
		},
	})
}

func testAccRadosgwS3BucketDataSourceConfig_capless(accessKey, secretKey, bucketName string) string {
	return fmt.Sprintf(`
provider "radosgw" {
  access_key = %q
  secret_key = %q
}

data "radosgw_s3_bucket" "test" {
  bucket = %q
}
`, accessKey, secretKey, bucketName)
}

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
