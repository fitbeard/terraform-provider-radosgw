package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccRadosgwIAMUserDataSource_basic(t *testing.T) {
	t.Parallel()

	userID := randomName("tf-acc-user")
	displayName := "Test User Data Source"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMUserDataSourceConfig_basic(userID, displayName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.radosgw_iam_user.test", "user_id", "radosgw_iam_user.test", "user_id"),
					resource.TestCheckResourceAttrPair("data.radosgw_iam_user.test", "display_name", "radosgw_iam_user.test", "display_name"),
					resource.TestCheckResourceAttrPair("data.radosgw_iam_user.test", "max_buckets", "radosgw_iam_user.test", "max_buckets"),
				),
			},
		},
	})
}

func TestAccRadosgwIAMUserDataSource_withEmail(t *testing.T) {
	t.Parallel()

	userID := randomName("tf-acc-user")
	displayName := "Test User With Email"
	email := randomEmail()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMUserDataSourceConfig_withEmail(userID, displayName, email),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.radosgw_iam_user.test", "email", "radosgw_iam_user.test", "email"),
				),
			},
		},
	})
}

// Test configurations

func testAccRadosgwIAMUserDataSourceConfig_basic(userID, displayName string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_user" "test" {
  user_id      = %q
  display_name = %q
}

data "radosgw_iam_user" "test" {
  user_id = radosgw_iam_user.test.user_id

  depends_on = [radosgw_iam_user.test]
}
`, userID, displayName)
}

func testAccRadosgwIAMUserDataSourceConfig_withEmail(userID, displayName, email string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_user" "test" {
  user_id      = %q
  display_name = %q
  email        = %q
}

data "radosgw_iam_user" "test" {
  user_id = radosgw_iam_user.test.user_id

  depends_on = [radosgw_iam_user.test]
}
`, userID, displayName, email)
}
