package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccRadosgwIAMUsersDataSource_basic(t *testing.T) {
	t.Parallel()

	userID := randomName("tf-acc-user")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMUsersDataSourceConfig_basic(userID),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.radosgw_iam_users.test", "user_ids.#"),
				),
			},
		},
	})
}

// Test configurations

func testAccRadosgwIAMUsersDataSourceConfig_basic(userID string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_user" "test" {
  user_id      = %q
  display_name = "Test User for Users Data Source"
}

data "radosgw_iam_users" "test" {
  depends_on = [radosgw_iam_user.test]
}
`, userID)
}
