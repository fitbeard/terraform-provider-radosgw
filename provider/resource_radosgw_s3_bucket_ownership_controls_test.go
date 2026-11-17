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

// TestAccRadosgwS3BucketOwnershipControls covers create/import/update of the
// ownership-controls resource. Gated to Umbrella (21.x), where S3 Object
// Ownership is supported.
func TestAccRadosgwS3BucketOwnershipControls(t *testing.T) {
	t.Parallel()

	bucketName := randomName("tf-acc-ownership")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSkipForVersion(t, CephVersion_Umbrella) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwS3BucketDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccS3BucketOwnershipControlsConfig(bucketName, "BucketOwnerEnforced"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckS3BucketObjectOwnership(bucketName, "BucketOwnerEnforced"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_ownership_controls.test", "bucket", bucketName),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_ownership_controls.test", "rule.#", "1"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_ownership_controls.test", "rule.0.object_ownership", "BucketOwnerEnforced"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				ResourceName:                         "radosgw_s3_bucket_ownership_controls.test",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateId:                        bucketName,
				ImportStateVerifyIdentifierAttribute: "bucket",
			},
			{
				// Update: switch ownership mode.
				Config: testAccS3BucketOwnershipControlsConfig(bucketName, "ObjectWriter"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckS3BucketObjectOwnership(bucketName, "ObjectWriter"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_ownership_controls.test", "rule.0.object_ownership", "ObjectWriter"),
				),
			},
		},
	})
}

// TestAccRadosgwS3BucketOwnershipControls_unsupportedVersion verifies the plan-time
// rejection when the provider targets a release older than Umbrella. Runs anywhere.
func TestAccRadosgwS3BucketOwnershipControls_unsupportedVersion(t *testing.T) {
	t.Parallel()

	bucketName := randomName("tf-acc-ownership-sq")

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

resource "radosgw_s3_bucket_ownership_controls" "test" {
  bucket = radosgw_s3_bucket.test.bucket
  rule {
    object_ownership = "BucketOwnerEnforced"
  }
}
`, bucketName),
				ExpectError: regexp.MustCompile(`requires Ceph 21 \(Umbrella\)`),
			},
		},
	})
}

// testAccCheckS3BucketObjectOwnership verifies via the S3 API that the bucket's
// ownership controls match the expected ObjectOwnership value.
func testAccCheckS3BucketObjectOwnership(bucket, want string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		out, err := testAccS3Client().GetBucketOwnershipControls(testCtx, &s3.GetBucketOwnershipControlsInput{Bucket: aws.String(bucket)})
		if err != nil {
			return fmt.Errorf("GetBucketOwnershipControls(%s): %w", bucket, err)
		}
		if out.OwnershipControls == nil || len(out.OwnershipControls.Rules) != 1 {
			return fmt.Errorf("bucket %s: expected exactly 1 ownership rule, got %+v", bucket, out.OwnershipControls)
		}
		if got := string(out.OwnershipControls.Rules[0].ObjectOwnership); got != want {
			return fmt.Errorf("bucket %s: expected object_ownership %q, got %q", bucket, want, got)
		}
		return nil
	}
}

func testAccS3BucketOwnershipControlsConfig(bucket, ownership string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_s3_bucket" "test" {
  bucket        = %[1]q
  force_destroy = true
}

resource "radosgw_s3_bucket_ownership_controls" "test" {
  bucket = radosgw_s3_bucket.test.bucket

  rule {
    object_ownership = %[2]q
  }
}
`, bucket, ownership)
}
