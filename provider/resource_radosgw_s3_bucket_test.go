package provider

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ceph/go-ceph/rgw/admin"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccRadosgwS3Bucket_basic(t *testing.T) {
	t.Parallel()

	bucketName := randomName("tf-acc-bucket")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwS3BucketDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwS3BucketConfig_basic(bucketName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwS3BucketExists("radosgw_s3_bucket.test"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket.test", "bucket", bucketName),
					resource.TestCheckResourceAttrSet("radosgw_s3_bucket.test", "owner"),
				),
			},
			// Import test - by bucket name
			{
				ResourceName:                         "radosgw_s3_bucket.test",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateVerifyIgnore:              []string{"force_destroy"},
				ImportStateId:                        bucketName,
				ImportStateVerifyIdentifierAttribute: "bucket",
			},
		},
	})
}

func TestAccRadosgwS3Bucket_forceDestroy(t *testing.T) {
	t.Parallel()

	bucketName := randomName("tf-acc-bucket")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwS3BucketDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwS3BucketConfig_forceDestroy(bucketName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwS3BucketExists("radosgw_s3_bucket.test"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket.test", "force_destroy", "true"),
				),
			},
		},
	})
}

func TestAccRadosgwS3Bucket_versioning(t *testing.T) {
	t.Parallel()

	bucketName := randomName("tf-acc-bucket")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwS3BucketDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwS3BucketConfig_versioning(bucketName, "enabled"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwS3BucketExists("radosgw_s3_bucket.test"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket.test", "versioning", "enabled"),
				),
			},
			{
				Config: testAccRadosgwS3BucketConfig_versioning(bucketName, "suspended"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwS3BucketExists("radosgw_s3_bucket.test"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket.test", "versioning", "suspended"),
				),
			},
		},
	})
}

func TestAccRadosgwS3Bucket_quota(t *testing.T) {
	t.Parallel()

	bucketName := randomName("tf-acc-bucket")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwS3BucketDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwS3BucketConfig_quota(bucketName, 1048576, 100),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwS3BucketExists("radosgw_s3_bucket.test"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket.test", "bucket_quota.enabled", "true"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket.test", "bucket_quota.max_size", "1048576"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket.test", "bucket_quota.max_objects", "100"),
				),
			},
			// Import test - by bucket name
			{
				ResourceName:                         "radosgw_s3_bucket.test",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateVerifyIgnore:              []string{"force_destroy"},
				ImportStateId:                        bucketName,
				ImportStateVerifyIdentifierAttribute: "bucket",
			},
		},
	})
}

// Helper functions

func testAccCheckRadosgwS3BucketExists(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		bucketName := rs.Primary.Attributes["bucket"]
		if bucketName == "" {
			return fmt.Errorf("bucket name not set")
		}

		// Check bucket exists using admin API
		_, err := testAccAdminClient.GetBucketInfo(testCtx, admin.Bucket{Bucket: bucketName})
		if err != nil {
			return fmt.Errorf("error fetching bucket %s: %s", bucketName, err)
		}

		return nil
	}
}

func testAccCheckRadosgwS3BucketDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "radosgw_s3_bucket" {
			continue
		}

		bucketName := rs.Primary.Attributes["bucket"]
		_, err := testAccAdminClient.GetBucketInfo(testCtx, admin.Bucket{Bucket: bucketName})
		if err == nil {
			return fmt.Errorf("bucket %s still exists", bucketName)
		}
	}

	return nil
}

// TestAccRadosgwS3Bucket_caplessUser: a user without admin
// caps (e.g. authenticated via OpenStack Keystone federation) can create a
// bucket via the S3 API, but the Admin API is unavailable. The provider must
// still persist the resource — populating S3-derivable fields and leaving
// admin-only fields null — with no drift on refresh.
func TestAccRadosgwS3Bucket_caplessUser(t *testing.T) {
	t.Parallel()

	bucketName := randomName("tf-acc-bucket")
	uid := randomName("tf-acc-capless")
	accessKey := strings.ReplaceAll(randomName("caplesskey"), "-", "")
	secretKey := strings.ReplaceAll(randomName("caplesssecret"), "-", "")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckCaplessBucketDestroy(uid, bucketName),
		Steps: []resource.TestStep{
			{
				// Create a capless user with known keys out-of-band, then drive the
				// bucket resource with a provider configured as that user.
				PreConfig: func() {
					if _, err := testAccAdminClient.CreateUser(testCtx, admin.User{
						ID:          uid,
						DisplayName: uid,
					}); err != nil {
						t.Fatalf("failed to create capless user %s: %s", uid, err)
					}
					gen := false
					if _, err := testAccAdminClient.CreateKey(testCtx, admin.UserKeySpec{
						UID:         uid,
						AccessKey:   accessKey,
						SecretKey:   secretKey,
						GenerateKey: &gen,
					}); err != nil {
						t.Fatalf("failed to add key to capless user %s: %s", uid, err)
					}
				},
				Config: testAccRadosgwS3BucketConfig_caplessUser(accessKey, secretKey, bucketName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_s3_bucket.test", "bucket", bucketName),
					// S3-derivable fields are populated even without admin caps.
					resource.TestCheckResourceAttrSet("radosgw_s3_bucket.test", "owner"),
					resource.TestCheckResourceAttrSet("radosgw_s3_bucket.test", "creation_time"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket.test", "versioning", "off"),
					// Admin-only fields are null when the Admin API is unavailable.
					resource.TestCheckNoResourceAttr("radosgw_s3_bucket.test", "num_shards"),
					resource.TestCheckNoResourceAttr("radosgw_s3_bucket.test", "marker"),
				),
			},
		},
	})
}

func testAccCheckCaplessBucketDestroy(uid, bucketName string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		// Remove the out-of-band user regardless of the outcome.
		defer func() { _ = testAccAdminClient.RemoveUser(testCtx, admin.User{ID: uid}) }()
		if _, err := testAccAdminClient.GetBucketInfo(testCtx, admin.Bucket{Bucket: bucketName}); err == nil {
			return fmt.Errorf("bucket %s still exists after destroy", bucketName)
		}
		return nil
	}
}

// Test configurations

func testAccRadosgwS3BucketConfig_caplessUser(accessKey, secretKey, bucketName string) string {
	// Provider configured as the capless user (endpoint still from the
	// RADOSGW_ENDPOINT env var). No radosgw_iam_* resources here — the bucket is
	// managed purely over S3.
	return fmt.Sprintf(`
provider "radosgw" {
  access_key = %q
  secret_key = %q
}

resource "radosgw_s3_bucket" "test" {
  bucket        = %q
  force_destroy = true
}
`, accessKey, secretKey, bucketName)
}

func testAccRadosgwS3BucketConfig_basic(bucketName string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_s3_bucket" "test" {
  bucket = %q
}
`, bucketName)
}

func testAccRadosgwS3BucketConfig_forceDestroy(bucketName string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_s3_bucket" "test" {
  bucket        = %q
  force_destroy = true
}
`, bucketName)
}

func testAccRadosgwS3BucketConfig_versioning(bucketName, versioning string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_s3_bucket" "test" {
  bucket     = %q
  versioning = %q
}
`, bucketName, versioning)
}

func testAccRadosgwS3BucketConfig_quota(bucketName string, maxSize, maxObjects int64) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_s3_bucket" "test" {
  bucket = %q

  bucket_quota = {
    enabled     = true
    max_size    = %d
    max_objects = %d
  }
}
`, bucketName, maxSize, maxObjects)
}
