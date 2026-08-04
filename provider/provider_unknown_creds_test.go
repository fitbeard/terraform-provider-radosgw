package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccProvider_unknownCredentials: a provider instance
// whose access_key/secret_key are unknown at plan time — because they are
// sourced from a radosgw_iam_access_key created in the same apply — must NOT
// fail planning with "Missing ... Key". Before the fix, an unknown value read
// back as an empty string and tripped the required-field check; the provider
// now defers client creation until apply, letting the plan succeed.
//
// This is a plan-time regression test: if the provider
// errors on unknown configuration values, `plan` fails and this test fails.
func TestAccProvider_unknownCredentials(t *testing.T) {
	t.Parallel()

	userID := randomName("tf-acc-boot")
	bucketName := randomName("tf-acc-bootbucket")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:             testAccProviderConfig_unknownCreds(userID, bucketName),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func testAccProviderConfig_unknownCreds(userID, bucketName string) string {
	endpoint := os.Getenv("RADOSGW_ENDPOINT")
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_user" "boot" {
  user_id      = %[1]q
  display_name = "BootUser"
}

# Auto-generated key pair — its values are unknown until apply.
resource "radosgw_iam_access_key" "boot" {
  user_id = radosgw_iam_user.boot.user_id
}

# Aliased provider authenticated with the unknown-until-apply credentials.
# Planning this previously failed with "Missing RadosGW Access Key".
provider "radosgw" {
  alias      = "boot"
  endpoint   = %[2]q
  access_key = radosgw_iam_access_key.boot.access_key
  secret_key = radosgw_iam_access_key.boot.secret_key
}

resource "radosgw_s3_bucket" "boot" {
  provider      = radosgw.boot
  bucket        = %[3]q
  force_destroy = true

  depends_on = [radosgw_iam_access_key.boot]
}
`, userID, endpoint, bucketName)
}
