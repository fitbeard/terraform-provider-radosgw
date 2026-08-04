package provider

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/ceph/go-ceph/rgw/admin"
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

// TestAccRadosgwS3BucketLink_tenantOwnedBucket:
// linking a bucket that already lives in a TENANT namespace — because it was
// created by the tenant user over S3 (e.g. a Keystone implicit-tenant user).
// The provider addresses the source bucket in the
// tenant namespace (with a default-namespace fallback), and the read compares the
// bare bucket name so the link is idempotent.
func TestAccRadosgwS3BucketLink_tenantOwnedBucket(t *testing.T) {
	t.Parallel()

	bucketName := randomName("tf-acc-bucket")
	uid := strings.ReplaceAll(randomName("tfaccten"), "-", "")
	tenant := strings.ReplaceAll(randomName("tftenant"), "-", "")
	accessKey := strings.ReplaceAll(randomName("tenownak"), "-", "")
	secretKey := strings.ReplaceAll(randomName("tenownsk"), "-", "")
	qualifiedUID := tenant + "$" + uid

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(_ *terraform.State) error {
			// unlink_to_uid returns the bucket to admin (default namespace) on destroy.
			_, _ = testAccS3Client().DeleteBucket(testCtx, &s3.DeleteBucketInput{Bucket: aws.String(bucketName)})
			_ = testAccAdminClient.RemoveUser(testCtx, admin.User{ID: qualifiedUID})
			return nil
		},
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					// Tenant user (Keystone implicit-tenant style), created out-of-band.
					if _, err := testAccAdminClient.CreateUser(testCtx, admin.User{ID: uid, Tenant: tenant, DisplayName: uid}); err != nil {
						t.Fatalf("failed to create tenant user %s: %s", qualifiedUID, err)
					}
					gen := false
					if _, err := testAccAdminClient.CreateKey(testCtx, admin.UserKeySpec{UID: qualifiedUID, AccessKey: accessKey, SecretKey: secretKey, GenerateKey: &gen}); err != nil {
						t.Fatalf("failed to add key to tenant user %s: %s", qualifiedUID, err)
					}
					// Bucket created AS the tenant user -> lands in the tenant namespace.
					if _, err := testAccS3ClientWithCreds(accessKey, secretKey).CreateBucket(testCtx, &s3.CreateBucketInput{Bucket: aws.String(bucketName)}); err != nil {
						t.Fatalf("failed to create tenant-owned bucket %s: %s", bucketName, err)
					}
				},
				Config: testAccRadosgwS3BucketLinkConfig_tenantOwned(bucketName, uid),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_s3_bucket_link.test", "bucket", bucketName),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_link.test", "uid", uid),
					resource.TestCheckResourceAttrSet("radosgw_s3_bucket_link.test", "bucket_id"),
				),
			},
		},
	})
}

func testAccRadosgwS3BucketLinkConfig_tenantOwned(bucketName, uid string) string {
	// No radosgw_iam_user resource: the tenant user and its tenant-namespace
	// bucket are created out-of-band; only ownership is managed here.
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_s3_bucket_link" "test" {
  bucket        = %q
  uid           = %q
  unlink_to_uid = "admin"
}
`, bucketName, uid)
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
