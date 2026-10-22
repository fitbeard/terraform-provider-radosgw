package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccRadosgwS3BucketRateLimit_basic(t *testing.T) {
	t.Parallel()

	bucketName := randomName("tf-acc-ratelimit")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwS3BucketDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccS3BucketRateLimitConfig_basic(bucketName),
				Check: resource.ComposeTestCheckFunc(
					// Verify over the Admin API that RGW actually stored the limits.
					testAccCheckS3BucketRateLimit(bucketName, true, 100, 50, 1048576, 524288),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_rate_limit.test", "bucket", bucketName),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_rate_limit.test", "enabled", "true"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_rate_limit.test", "max_read_ops", "100"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_rate_limit.test", "max_write_ops", "50"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_rate_limit.test", "max_read_bytes", "1048576"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_rate_limit.test", "max_write_bytes", "524288"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				ResourceName:                         "radosgw_s3_bucket_rate_limit.test",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateId:                        bucketName,
				ImportStateVerifyIdentifierAttribute: "bucket",
			},
			{
				// Update: change max_read_ops and OMIT the byte limits. Because the
				// settings default to 0 (authoritative), removing them from config must
				// reset them to 0, not keep the previously-applied values.
				Config: testAccS3BucketRateLimitConfig_updated(bucketName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckS3BucketRateLimit(bucketName, true, 200, 0, 0, 0),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_rate_limit.test", "max_read_ops", "200"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_rate_limit.test", "max_write_ops", "0"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_rate_limit.test", "max_read_bytes", "0"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_rate_limit.test", "max_write_bytes", "0"),
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

// Minimal config: only enabled is set, so the four limits default to 0. Verifies
// the Optional+Computed fields do not produce drift when omitted.
func TestAccRadosgwS3BucketRateLimit_enabledOnly(t *testing.T) {
	t.Parallel()

	bucketName := randomName("tf-acc-ratelimit-min")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwS3BucketDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccS3BucketRateLimitConfig_enabledOnly(bucketName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckS3BucketRateLimit(bucketName, true, 0, 0, 0, 0),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_rate_limit.test", "enabled", "true"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_rate_limit.test", "max_read_ops", "0"),
					resource.TestCheckResourceAttr("radosgw_s3_bucket_rate_limit.test", "max_write_bytes", "0"),
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

func TestAccRadosgwS3BucketRateLimit_dataSource(t *testing.T) {
	t.Parallel()

	bucketName := randomName("tf-acc-ratelimit-ds")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwS3BucketDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccS3BucketRateLimitConfig_dataSource(bucketName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.radosgw_s3_bucket_rate_limit.test", "bucket", bucketName),
					resource.TestCheckResourceAttr("data.radosgw_s3_bucket_rate_limit.test", "enabled", "true"),
					resource.TestCheckResourceAttr("data.radosgw_s3_bucket_rate_limit.test", "max_read_ops", "100"),
					resource.TestCheckResourceAttr("data.radosgw_s3_bucket_rate_limit.test", "max_write_bytes", "524288"),
				),
			},
		},
	})
}

// testAccCheckS3BucketRateLimit verifies via the Admin API that the bucket's rate
// limit matches the expected values.
func testAccCheckS3BucketRateLimit(bucket string, enabled bool, readOps, writeOps, readBytes, writeBytes int64) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		if testAccAdminClient == nil {
			return fmt.Errorf("admin client not initialized")
		}
		spec, err := testAccAdminClient.GetIndividualBucketRateLimit(testCtx, bucket)
		if err != nil {
			return fmt.Errorf("GetIndividualBucketRateLimit(%s): %w", bucket, err)
		}
		if spec.Enabled == nil || *spec.Enabled != enabled {
			return fmt.Errorf("bucket %s: expected enabled=%t, got %v", bucket, enabled, spec.Enabled)
		}
		checks := []struct {
			name string
			got  *int64
			want int64
		}{
			{"max_read_ops", spec.MaxReadOps, readOps},
			{"max_write_ops", spec.MaxWriteOps, writeOps},
			{"max_read_bytes", spec.MaxReadBytes, readBytes},
			{"max_write_bytes", spec.MaxWriteBytes, writeBytes},
		}
		for _, c := range checks {
			got := int64(0)
			if c.got != nil {
				got = *c.got
			}
			if got != c.want {
				return fmt.Errorf("bucket %s: expected %s=%d, got %d", bucket, c.name, c.want, got)
			}
		}
		return nil
	}
}

func testAccS3BucketRateLimitConfig_basic(bucket string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_s3_bucket" "test" {
  bucket        = %[1]q
  force_destroy = true
}

resource "radosgw_s3_bucket_rate_limit" "test" {
  bucket          = radosgw_s3_bucket.test.bucket
  enabled         = true
  max_read_ops    = 100
  max_write_ops   = 50
  max_read_bytes  = 1048576
  max_write_bytes = 524288
}
`, bucket)
}

func testAccS3BucketRateLimitConfig_updated(bucket string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_s3_bucket" "test" {
  bucket        = %[1]q
  force_destroy = true
}

resource "radosgw_s3_bucket_rate_limit" "test" {
  bucket        = radosgw_s3_bucket.test.bucket
  enabled       = true
  max_read_ops  = 200
  max_write_ops = 0
  # max_read_bytes and max_write_bytes omitted on purpose: they must reset to 0.
}
`, bucket)
}

func testAccS3BucketRateLimitConfig_enabledOnly(bucket string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_s3_bucket" "test" {
  bucket        = %[1]q
  force_destroy = true
}

resource "radosgw_s3_bucket_rate_limit" "test" {
  bucket  = radosgw_s3_bucket.test.bucket
  enabled = true
}
`, bucket)
}

func testAccS3BucketRateLimitConfig_dataSource(bucket string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_s3_bucket" "test" {
  bucket        = %[1]q
  force_destroy = true
}

resource "radosgw_s3_bucket_rate_limit" "test" {
  bucket          = radosgw_s3_bucket.test.bucket
  enabled         = true
  max_read_ops    = 100
  max_write_ops   = 50
  max_read_bytes  = 1048576
  max_write_bytes = 524288
}

data "radosgw_s3_bucket_rate_limit" "test" {
  bucket     = radosgw_s3_bucket_rate_limit.test.bucket
  depends_on = [radosgw_s3_bucket_rate_limit.test]
}
`, bucket)
}
