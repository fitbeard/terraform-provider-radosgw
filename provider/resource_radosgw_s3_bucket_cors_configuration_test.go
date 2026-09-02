package provider

import (
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccRadosgwS3BucketCorsConfiguration_basic(t *testing.T) {
	t.Parallel()

	bucketName := randomName("tf-acc-cors")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwS3BucketDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccS3BucketCorsConfig_basic(bucketName),
				Check: resource.ComposeTestCheckFunc(
					// Verify over the S3 API that RGW actually stored the rules.
					testAccCheckS3BucketCorsRuleCount(bucketName, 2),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_cors_configuration.test", "bucket", bucketName),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_cors_configuration.test", "cors_rule.#", "2"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_cors_configuration.test", "cors_rule.0.id", "rule1"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_cors_configuration.test", "cors_rule.0.allowed_methods.#", "5"),
					// allowed_methods/origins/expose_headers are Sets (order-insensitive) — check membership,
					// not index position, since RGW reorders them (e.g. HEAD is not kept at index 0).
					resource.TestCheckTypeSetElemAttr("radosgw_s3_bucket_cors_configuration.test", "cors_rule.0.allowed_methods.*", "HEAD"),
					resource.TestCheckTypeSetElemAttr("radosgw_s3_bucket_cors_configuration.test", "cors_rule.0.allowed_methods.*", "DELETE"),
					resource.TestCheckTypeSetElemAttr("radosgw_s3_bucket_cors_configuration.test", "cors_rule.0.allowed_origins.*", "https://app.example.com"),
					resource.TestCheckTypeSetElemAttr("radosgw_s3_bucket_cors_configuration.test", "cors_rule.0.expose_headers.*", "ETag"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_cors_configuration.test", "cors_rule.0.max_age_seconds", "3000"),
					// The second rule omits id/max_age_seconds — they must stay null.
					resource.TestCheckNoResourceAttr("radosgw_s3_bucket_cors_configuration.test", "cors_rule.1.id"),
					resource.TestCheckNoResourceAttr("radosgw_s3_bucket_cors_configuration.test", "cors_rule.1.max_age_seconds"),
				),
				// Regression guard: RGW normalizes the order of methods/headers/origins. With the
				// fields modeled as Sets this must NOT produce a perpetual diff, so the post-apply
				// refreshed plan is expected to be empty.
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				ResourceName:                         "radosgw_s3_bucket_cors_configuration.test",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateId:                        bucketName,
				ImportStateVerifyIdentifierAttribute: "bucket",
			},
			{
				// Update: replace with a single, different rule.
				Config: testAccS3BucketCorsConfig_updated(bucketName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckS3BucketCorsRuleCount(bucketName, 1),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_cors_configuration.test", "cors_rule.#", "1"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_cors_configuration.test", "cors_rule.0.allowed_methods.#", "2"),
					resource.TestCheckTypeSetElemAttr("radosgw_s3_bucket_cors_configuration.test", "cors_rule.0.allowed_origins.*", "https://updated.example.com"),
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

// testAccCheckS3BucketCorsRuleCount verifies via the S3 API that the bucket has
// the expected number of CORS rules.
func testAccCheckS3BucketCorsRuleCount(bucket string, want int) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		out, err := testAccS3Client().GetBucketCors(testCtx, &s3.GetBucketCorsInput{Bucket: aws.String(bucket)})
		if err != nil {
			return fmt.Errorf("GetBucketCors(%s): %w", bucket, err)
		}
		if len(out.CORSRules) != want {
			return fmt.Errorf("bucket %s: expected %d CORS rules, got %d", bucket, want, len(out.CORSRules))
		}
		return nil
	}
}

func testAccS3BucketCorsConfig_basic(bucket string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_s3_bucket" "test" {
  bucket        = %[1]q
  force_destroy = true
}

resource "radosgw_s3_bucket_cors_configuration" "test" {
  bucket = radosgw_s3_bucket.test.bucket

  cors_rule {
    id              = "rule1"
    allowed_headers = ["*"]
    # Deliberately not in RGW's canonical order — RGW reorders these on read.
    # Modeled as a Set, that reordering must not surface as drift.
    allowed_methods = ["HEAD", "GET", "PUT", "DELETE", "POST"]
    allowed_origins = ["https://app.example.com"]
    expose_headers  = ["ETag", "x-amz-request-id"]
    max_age_seconds = 3000
  }

  cors_rule {
    allowed_methods = ["GET"]
    allowed_origins = ["*"]
  }
}
`, bucket)
}

func testAccS3BucketCorsConfig_updated(bucket string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_s3_bucket" "test" {
  bucket        = %[1]q
  force_destroy = true
}

resource "radosgw_s3_bucket_cors_configuration" "test" {
  bucket = radosgw_s3_bucket.test.bucket

  cors_rule {
    allowed_methods = ["GET", "HEAD"]
    allowed_origins = ["https://updated.example.com"]
  }
}
`, bucket)
}
