package provider

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3control"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// testAccS3ControlClientWithCreds returns an S3 Control client for the given
// credentials, built the same header-only way as the provider (no account-id
// host prefix), for out-of-band verification in acceptance tests.
func testAccS3ControlClientWithCreds(accessKey, secretKey string) *s3control.Client {
	endpoint := os.Getenv("RADOSGW_ENDPOINT")
	return s3control.NewFromConfig(aws.Config{
		Region:      "default",
		Credentials: credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
	}, func(o *s3control.Options) {
		o.BaseEndpoint = &endpoint
		o.EndpointResolverV2 = s3controlPlainEndpointResolver{endpoint: endpoint}
		o.APIOptions = append(o.APIOptions, disableS3ControlHostPrefix)
	})
}

// TestAccRadosgwS3AccountPublicAccessBlock covers create/import/update of the
// account-wide public-access-block resource plus its data source. The account
// PAB is managed by the account's own root credentials (the provider must
// authenticate as the account root), so the config uses an aliased provider.
// Account-wide PAB is Umbrella (21) only.
func TestAccRadosgwS3AccountPublicAccessBlock(t *testing.T) {
	t.Parallel()

	ar := newTestAccountRoot()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSkipForVersion(t, CephVersion_Umbrella) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             ar.destroy,
		Steps: []resource.TestStep{
			{
				PreConfig: func() { ar.create(t) },
				// Create: all four settings on.
				Config: ar.rootProviderConfig() + testAccS3AccountPublicAccessBlockConfig(ar.accountID, true, true, true, true),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckS3AccountPublicAccessBlock(ar, true, true, true, true),
					resource.TestCheckResourceAttr("radosgw_s3_account_public_access_block.test", "account_id", ar.accountID),
					resource.TestCheckResourceAttr("radosgw_s3_account_public_access_block.test", "block_public_acls", "true"),
					resource.TestCheckResourceAttr("radosgw_s3_account_public_access_block.test", "block_public_policy", "true"),
					resource.TestCheckResourceAttr("radosgw_s3_account_public_access_block.test", "ignore_public_acls", "true"),
					resource.TestCheckResourceAttr("radosgw_s3_account_public_access_block.test", "restrict_public_buckets", "true"),
					// Data source reads back the same values.
					resource.TestCheckResourceAttr("data.radosgw_s3_account_public_access_block.test", "block_public_acls", "true"),
					resource.TestCheckResourceAttr("data.radosgw_s3_account_public_access_block.test", "restrict_public_buckets", "true"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				ResourceName:                         "radosgw_s3_account_public_access_block.test",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateId:                        ar.accountID,
				ImportStateVerifyIdentifierAttribute: "account_id",
			},
			{
				// Update: flip a mix of flags.
				Config: ar.rootProviderConfig() + testAccS3AccountPublicAccessBlockConfig(ar.accountID, true, false, false, false),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckS3AccountPublicAccessBlock(ar, true, false, false, false),
					resource.TestCheckResourceAttr("radosgw_s3_account_public_access_block.test", "block_public_acls", "true"),
					resource.TestCheckResourceAttr("radosgw_s3_account_public_access_block.test", "block_public_policy", "false"),
					resource.TestCheckResourceAttr("radosgw_s3_account_public_access_block.test", "ignore_public_acls", "false"),
					resource.TestCheckResourceAttr("radosgw_s3_account_public_access_block.test", "restrict_public_buckets", "false"),
				),
			},
		},
	})
}

// TestAccRadosgwS3AccountPublicAccessBlock_unsupportedVersion verifies the
// plan-time rejection when the provider targets a release older than Umbrella.
// Runs anywhere (the error is raised at plan time without contacting the cluster).
func TestAccRadosgwS3AccountPublicAccessBlock_unsupportedVersion(t *testing.T) {
	t.Parallel()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "radosgw" {
  ceph_version = "tentacle"
}

resource "radosgw_s3_account_public_access_block" "test" {
  account_id        = "RGW00000000000000000"
  block_public_acls = true
}
`,
				ExpectError: regexp.MustCompile(`requires Ceph 21 \(Umbrella\)`),
			},
		},
	})
}

// testAccCheckS3AccountPublicAccessBlock verifies via the S3 Control API (as the
// account root) that the account's public access block matches the expected flags.
func testAccCheckS3AccountPublicAccessBlock(ar testAccountRoot, acls, policy, ignore, restrict bool) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		client := testAccS3ControlClientWithCreds(ar.accessKey, ar.secretKey)
		out, err := client.GetPublicAccessBlock(testCtx, &s3control.GetPublicAccessBlockInput{AccountId: aws.String(ar.accountID)})
		if err != nil {
			return fmt.Errorf("GetPublicAccessBlock(%s): %w", ar.accountID, err)
		}
		cfg := out.PublicAccessBlockConfiguration
		if cfg == nil {
			return fmt.Errorf("account %s: expected a public access block configuration, got nil", ar.accountID)
		}
		if got := aws.ToBool(cfg.BlockPublicAcls); got != acls {
			return fmt.Errorf("account %s: block_public_acls = %t, want %t", ar.accountID, got, acls)
		}
		if got := aws.ToBool(cfg.BlockPublicPolicy); got != policy {
			return fmt.Errorf("account %s: block_public_policy = %t, want %t", ar.accountID, got, policy)
		}
		if got := aws.ToBool(cfg.IgnorePublicAcls); got != ignore {
			return fmt.Errorf("account %s: ignore_public_acls = %t, want %t", ar.accountID, got, ignore)
		}
		if got := aws.ToBool(cfg.RestrictPublicBuckets); got != restrict {
			return fmt.Errorf("account %s: restrict_public_buckets = %t, want %t", ar.accountID, got, restrict)
		}
		return nil
	}
}

func testAccS3AccountPublicAccessBlockConfig(accountID string, acls, policy, ignore, restrict bool) string {
	return fmt.Sprintf(`
resource "radosgw_s3_account_public_access_block" "test" {
  provider = radosgw.root

  account_id              = %[1]q
  block_public_acls       = %[2]t
  block_public_policy     = %[3]t
  ignore_public_acls      = %[4]t
  restrict_public_buckets = %[5]t
}

data "radosgw_s3_account_public_access_block" "test" {
  provider   = radosgw.root
  account_id = radosgw_s3_account_public_access_block.test.account_id
}
`, accountID, acls, policy, ignore, restrict)
}
