package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccRadosgwIAMSubusersDataSource_basic(t *testing.T) {
	t.Parallel()

	userID := randomName("tf-acc-user")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMSubusersDataSourceConfig_basic(userID),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.radosgw_iam_subusers.test", "user_id", userID),
					resource.TestCheckResourceAttr("data.radosgw_iam_subusers.test", "subusers.#", "1"),
				),
			},
		},
	})
}

// TestAccRadosgwIAMSubusersDataSource_multiple tests multiple subusers.
// Multiple subuser creation has issues in Reef (18.x), requires Squid (19.x) or higher.
func TestAccRadosgwIAMSubusersDataSource_multiple(t *testing.T) {
	t.Parallel()

	userID := randomName("tf-acc-user")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSkipForVersion(t, CephVersion_Squid) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMSubusersDataSourceConfig_multiple(userID),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.radosgw_iam_subusers.test", "subusers.#", "2"),
				),
			},
		},
	})
}

// Test configurations

func testAccRadosgwIAMSubusersDataSourceConfig_basic(userID string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_user" "test" {
  user_id      = %q
  display_name = "Test User for Subusers Data Source"
}

resource "radosgw_iam_subuser" "test" {
  user_id = radosgw_iam_user.test.user_id
  subuser = "swift"
  access  = "full-control"
}

data "radosgw_iam_subusers" "test" {
  user_id = radosgw_iam_user.test.user_id

  depends_on = [radosgw_iam_subuser.test]
}
`, userID)
}

func testAccRadosgwIAMSubusersDataSourceConfig_multiple(userID string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_user" "test" {
  user_id      = %q
  display_name = "Test User for Multiple Subusers Data Source"
}

resource "radosgw_iam_subuser" "test1" {
  user_id = radosgw_iam_user.test.user_id
  subuser = "swift"
  access  = "full-control"
}

resource "radosgw_iam_subuser" "test2" {
  user_id = radosgw_iam_user.test.user_id
  subuser = "readonly"
  access  = "read"
}

data "radosgw_iam_subusers" "test" {
  user_id = radosgw_iam_user.test.user_id

  depends_on = [
    radosgw_iam_subuser.test1,
    radosgw_iam_subuser.test2,
  ]
}
`, userID)
}
