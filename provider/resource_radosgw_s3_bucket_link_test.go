package provider

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// testAccS3ClientWithCreds returns an S3 client for the given credentials, for
// out-of-band bucket setup/teardown in acceptance tests.
func testAccS3ClientWithCreds(accessKey, secretKey string) *s3.Client {
	endpoint := os.Getenv("RADOSGW_ENDPOINT")
	return s3.NewFromConfig(aws.Config{
		Region:      "default",
		Credentials: credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
	}, func(o *s3.Options) {
		o.BaseEndpoint = &endpoint
		o.UsePathStyle = true
	})
}

// testAccS3Client returns an S3 client using the admin credentials from the
// environment.
func testAccS3Client() *s3.Client {
	return testAccS3ClientWithCreds(os.Getenv("RADOSGW_ACCESS_KEY"), os.Getenv("RADOSGW_SECRET_KEY"))
}

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

// TestAccRadosgwS3BucketLink_tenantUser: linking a default-namespace bucket
// to a tenant user (e.g. Keystone implicit tenants)
// previously failed with NoSuchKey because RadosGW scopes buckets by tenant.
func TestAccRadosgwS3BucketLink_tenantUser(t *testing.T) {
	t.Parallel()

	bucketName := randomName("tf-acc-bucket")
	userID := randomName("tf-acc-user")
	// Tenant names allow only alphanumeric and underscore characters.
	tenant := strings.ReplaceAll(randomName("tftenant"), "-", "")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckBucketLinkTenantCleanup(bucketName),
		Steps: []resource.TestStep{
			{
				// Create the bucket out-of-band (default namespace, owned by admin),
				// mirroring the reported setup where the bucket is created outside
				// this provider. The link then moves it into the tenant namespace.
				PreConfig: func() {
					_, err := testAccS3Client().CreateBucket(testCtx, &s3.CreateBucketInput{Bucket: aws.String(bucketName)})
					if err != nil {
						t.Fatalf("failed to pre-create bucket %s: %s", bucketName, err)
					}
				},
				Config: testAccRadosgwS3BucketLinkConfig_tenant(bucketName, userID, tenant),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_s3_bucket_link.test", "bucket", bucketName),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_link.test", "uid", userID),
					resource.TestCheckResourceAttrSet("radosgw_s3_bucket_link.test", "bucket_id"),
				),
			},
		},
	})
}

// testAccCheckBucketLinkTenantCleanup deletes the out-of-band bucket (relinked
// back to admin on destroy) so the test leaves no residue.
func testAccCheckBucketLinkTenantCleanup(bucketName string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		_, _ = testAccS3Client().DeleteBucket(testCtx, &s3.DeleteBucketInput{Bucket: aws.String(bucketName)})
		return nil
	}
}

// Test configurations

func testAccRadosgwS3BucketLinkConfig_tenant(bucketName, userID, tenant string) string {
	// No radosgw_s3_bucket resource: the bucket is created out-of-band and only
	// its ownership is managed here. unlink_to_uid returns it to admin on destroy.
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_user" "test" {
  user_id      = %q
  display_name = "TenantLinkUser"
  tenant       = %q
}

resource "radosgw_s3_bucket_link" "test" {
  bucket        = %q
  uid           = radosgw_iam_user.test.user_id
  unlink_to_uid = "admin"

  depends_on = [radosgw_iam_user.test]
}
`, userID, tenant, bucketName)
}

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
