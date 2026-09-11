package provider

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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

// TestAccRadosgwS3Bucket_versioningCannotDisable verifies that trying to turn
// versioning back "off" once it has been enabled fails at PLAN time with a clear,
// actionable error — instead of a generic post-apply "inconsistent result"
// provider bug.
func TestAccRadosgwS3Bucket_versioningCannotDisable(t *testing.T) {
	t.Parallel()

	bucketName := randomName("tf-acc-bucket")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwS3BucketDestroy,
		Steps: []resource.TestStep{
			{
				// Enable versioning.
				Config: testAccRadosgwS3BucketConfig_versioning(bucketName, "enabled"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_s3_bucket.test", "versioning", "enabled"),
				),
			},
			{
				// enabled -> off must be rejected during plan.
				Config:      testAccRadosgwS3BucketConfig_versioning(bucketName, "off"),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Cannot Disable Bucket Versioning`),
			},
			{
				// enabled -> suspended is the supported way to stop versioning.
				Config: testAccRadosgwS3BucketConfig_versioning(bucketName, "suspended"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_s3_bucket.test", "versioning", "suspended"),
				),
			},
			{
				// suspended -> off is likewise rejected (still a one-way constraint).
				Config:      testAccRadosgwS3BucketConfig_versioning(bucketName, "off"),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Cannot Disable Bucket Versioning`),
			},
		},
	})
}

// TestAccRadosgwS3Bucket_tags exercises the bucket tags attribute: create with
// tags, update (change/add/remove keys), and remove all tags by omission. Each
// step is cross-checked against the S3 GetBucketTagging API, and idempotency is
// asserted with an empty post-apply plan.
func TestAccRadosgwS3Bucket_tags(t *testing.T) {
	t.Parallel()

	bucketName := randomName("tf-acc-bucket")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwS3BucketDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwS3BucketConfig_tags(bucketName, `
    env  = "prod"
    team = "storage"`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_s3_bucket.test", "tags.%", "2"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket.test", "tags.env", "prod"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket.test", "tags.team", "storage"),
					testAccCheckS3BucketTags(bucketName, map[string]string{"env": "prod", "team": "storage"}),
				),
			},
			// Import round-trips the tags.
			{
				ResourceName:                         "radosgw_s3_bucket.test",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateVerifyIgnore:              []string{"force_destroy"},
				ImportStateId:                        bucketName,
				ImportStateVerifyIdentifierAttribute: "bucket",
			},
			// Change one value, add a key, drop a key.
			{
				Config: testAccRadosgwS3BucketConfig_tags(bucketName, `
    env   = "staging"
    owner = "data-team"`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_s3_bucket.test", "tags.%", "2"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket.test", "tags.env", "staging"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket.test", "tags.owner", "data-team"),
					resource.TestCheckNoResourceAttr("radosgw_s3_bucket.test", "tags.team"),
					testAccCheckS3BucketTags(bucketName, map[string]string{"env": "staging", "owner": "data-team"}),
				),
			},
			// Remove all tags by omitting the attribute.
			{
				Config: testAccRadosgwS3BucketConfig_forceDestroy(bucketName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("radosgw_s3_bucket.test", "tags.%"),
					testAccCheckS3BucketTags(bucketName, nil),
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

func testAccRadosgwS3BucketConfig_tags(bucketName, tagsBody string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_s3_bucket" "test" {
  bucket        = %q
  force_destroy = true
  tags = {%s
  }
}
`, bucketName, tagsBody)
}

// testAccCheckS3BucketTags verifies via the S3 API that the bucket's tag set
// matches want exactly. A nil/empty want asserts the bucket has no tags (RGW
// returns NoSuchTagSet).
func testAccCheckS3BucketTags(bucket string, want map[string]string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		out, err := testAccS3Client().GetBucketTagging(testCtx, &s3.GetBucketTaggingInput{Bucket: aws.String(bucket)})
		if err != nil {
			if len(want) == 0 && isS3NoSuchTagSet(err) {
				return nil
			}
			return fmt.Errorf("GetBucketTagging(%s): %w", bucket, err)
		}
		got := make(map[string]string, len(out.TagSet))
		for _, tg := range out.TagSet {
			got[aws.ToString(tg.Key)] = aws.ToString(tg.Value)
		}
		if len(got) != len(want) {
			return fmt.Errorf("bucket %s: expected %d tags %v, got %d %v", bucket, len(want), want, len(got), got)
		}
		for k, v := range want {
			if got[k] != v {
				return fmt.Errorf("bucket %s: tag %q = %q, want %q", bucket, k, got[k], v)
			}
		}
		return nil
	}
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
