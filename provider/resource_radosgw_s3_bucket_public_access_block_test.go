package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAccRadosgwS3BucketPublicAccessBlock covers create/import/update of the
// public-access-block resource, plus the read-back data source. The happy path
// sets restrict_public_buckets=true, which requires Tentacle (20.x) or higher.
func TestAccRadosgwS3BucketPublicAccessBlock(t *testing.T) {
	t.Parallel()

	bucketName := randomName("tf-acc-pab")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSkipForVersion(t, CephVersion_Tentacle) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwS3BucketDestroy,
		Steps: []resource.TestStep{
			{
				// Create: all four settings on.
				Config: testAccS3BucketPublicAccessBlockConfig(bucketName, true, true, true, true),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckS3PublicAccessBlock(bucketName, true, true, true, true),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_public_access_block.test", "bucket", bucketName),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_public_access_block.test", "block_public_acls", "true"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_public_access_block.test", "block_public_policy", "true"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_public_access_block.test", "ignore_public_acls", "true"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_public_access_block.test", "restrict_public_buckets", "true"),
					// Data source reads back the same values.
					resource.TestCheckResourceAttr("data.radosgw_s3_bucket_public_access_block.test", "block_public_acls", "true"),
					resource.TestCheckResourceAttr("data.radosgw_s3_bucket_public_access_block.test", "restrict_public_buckets", "true"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				ResourceName:                         "radosgw_s3_bucket_public_access_block.test",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateId:                        bucketName,
				ImportStateVerifyIdentifierAttribute: "bucket",
			},
			{
				// Update: flip a mix of flags.
				Config: testAccS3BucketPublicAccessBlockConfig(bucketName, true, false, false, false),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckS3PublicAccessBlock(bucketName, true, false, false, false),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_public_access_block.test", "block_public_acls", "true"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_public_access_block.test", "block_public_policy", "false"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_public_access_block.test", "ignore_public_acls", "false"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_public_access_block.test", "restrict_public_buckets", "false"),
				),
			},
		},
	})
}

// TestAccRadosgwS3BucketPublicAccessBlock_defaults verifies that omitting all
// settings defaults them to false and applies cleanly on any supported release.
func TestAccRadosgwS3BucketPublicAccessBlock_defaults(t *testing.T) {
	t.Parallel()

	bucketName := randomName("tf-acc-pab-def")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwS3BucketDestroy,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + fmt.Sprintf(`
resource "radosgw_s3_bucket" "test" {
  bucket        = %[1]q
  force_destroy = true
}

resource "radosgw_s3_bucket_public_access_block" "test" {
  bucket              = radosgw_s3_bucket.test.bucket
  block_public_acls   = true
  ignore_public_acls  = true
  block_public_policy = true
}
`, bucketName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckS3PublicAccessBlock(bucketName, true, true, true, false),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_public_access_block.test", "restrict_public_buckets", "false"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAccRadosgwS3BucketPublicAccessBlock_restrictRequiresTentacle verifies the
// plan-time rejection of restrict_public_buckets on a pre-Tentacle release. Runs
// anywhere (the error is raised at plan time without contacting the cluster).
func TestAccRadosgwS3BucketPublicAccessBlock_restrictRequiresTentacle(t *testing.T) {
	t.Parallel()

	bucketName := randomName("tf-acc-pab-sq")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
provider "radosgw" {
  ceph_version = "squid"
}

resource "radosgw_s3_bucket" "test" {
  bucket        = %[1]q
  force_destroy = true
}

resource "radosgw_s3_bucket_public_access_block" "test" {
  bucket                  = radosgw_s3_bucket.test.bucket
  restrict_public_buckets = true
}
`, bucketName),
				ExpectError: regexp.MustCompile(`requires Ceph 20 \(Tentacle\)`),
			},
		},
	})
}

// testAccCheckS3PublicAccessBlock verifies via the S3 API that the bucket's
// public access block matches the expected flags.
func testAccCheckS3PublicAccessBlock(bucket string, acls, policy, ignore, restrict bool) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		out, err := testAccS3Client().GetPublicAccessBlock(testCtx, &s3.GetPublicAccessBlockInput{Bucket: aws.String(bucket)})
		if err != nil {
			return fmt.Errorf("GetPublicAccessBlock(%s): %w", bucket, err)
		}
		cfg := out.PublicAccessBlockConfiguration
		if cfg == nil {
			return fmt.Errorf("bucket %s: expected a public access block configuration, got nil", bucket)
		}
		if got := aws.ToBool(cfg.BlockPublicAcls); got != acls {
			return fmt.Errorf("bucket %s: block_public_acls = %t, want %t", bucket, got, acls)
		}
		if got := aws.ToBool(cfg.BlockPublicPolicy); got != policy {
			return fmt.Errorf("bucket %s: block_public_policy = %t, want %t", bucket, got, policy)
		}
		if got := aws.ToBool(cfg.IgnorePublicAcls); got != ignore {
			return fmt.Errorf("bucket %s: ignore_public_acls = %t, want %t", bucket, got, ignore)
		}
		if got := aws.ToBool(cfg.RestrictPublicBuckets); got != restrict {
			return fmt.Errorf("bucket %s: restrict_public_buckets = %t, want %t", bucket, got, restrict)
		}
		return nil
	}
}

func testAccS3BucketPublicAccessBlockConfig(bucket string, acls, policy, ignore, restrict bool) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_s3_bucket" "test" {
  bucket        = %[1]q
  force_destroy = true
}

resource "radosgw_s3_bucket_public_access_block" "test" {
  bucket                  = radosgw_s3_bucket.test.bucket
  block_public_acls       = %[2]t
  block_public_policy     = %[3]t
  ignore_public_acls      = %[4]t
  restrict_public_buckets = %[5]t
}

data "radosgw_s3_bucket_public_access_block" "test" {
  bucket = radosgw_s3_bucket_public_access_block.test.bucket
}
`, bucket, acls, policy, ignore, restrict)
}
