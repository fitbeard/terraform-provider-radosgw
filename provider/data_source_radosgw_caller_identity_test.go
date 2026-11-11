package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccRadosgwCallerIdentityDataSource covers the STS GetCallerIdentity data
// source. Gated to Umbrella (21.x), where GetCallerIdentity is available.
func TestAccRadosgwCallerIdentityDataSource(t *testing.T) {
	t.Parallel()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSkipForVersion(t, CephVersion_Umbrella) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwCallerIdentityConfig(),
				Check: resource.ComposeTestCheckFunc(
					// The acceptance user is "admin" (see scripts/create-rgw-user.sh).
					resource.TestCheckResourceAttr("data.radosgw_caller_identity.current", "user_id", "admin"),
					resource.TestCheckResourceAttrSet("data.radosgw_caller_identity.current", "arn"),
					// id mirrors account_id (AWS parity).
					resource.TestCheckResourceAttrPair(
						"data.radosgw_caller_identity.current", "id",
						"data.radosgw_caller_identity.current", "account_id",
					),
				),
			},
		},
	})
}

func testAccRadosgwCallerIdentityConfig() string {
	return providerConfig() + `
data "radosgw_caller_identity" "current" {}
`
}

// TestAccRadosgwCallerIdentityDataSource_unsupportedVersion verifies that the data
// source errors clearly when the provider targets a release older than Umbrella.
// The gate fires before any STS call, so this runs on any cluster.
func TestAccRadosgwCallerIdentityDataSource_unsupportedVersion(t *testing.T) {
	t.Parallel()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "radosgw" {
  ceph_version = "squid"
}

data "radosgw_caller_identity" "current" {}
`,
				ExpectError: regexp.MustCompile(`requires Ceph 21 \(Umbrella\)`),
			},
		},
	})
}
