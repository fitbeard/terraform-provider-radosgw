package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccRadosgwIAMUserCapsDataSource_basic(t *testing.T) {
	t.Parallel()

	userID := randomName("tf-acc-user")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMUserCapsDataSourceConfig_basic(userID),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.radosgw_iam_user_caps.test", "user_id", userID),
					resource.TestCheckResourceAttr("data.radosgw_iam_user_caps.test", "caps.#", "1"),
				),
			},
		},
	})
}

func TestAccRadosgwIAMUserCapsDataSource_multiple(t *testing.T) {
	t.Parallel()

	userID := randomName("tf-acc-user")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMUserCapsDataSourceConfig_multiple(userID),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.radosgw_iam_user_caps.test", "caps.#", "2"),
				),
			},
		},
	})
}

// Test configurations

func testAccRadosgwIAMUserCapsDataSourceConfig_basic(userID string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_user" "test" {
  user_id      = %q
  display_name = "Test User for Caps Data Source"
}

resource "radosgw_iam_user_caps" "test" {
  user_id = radosgw_iam_user.test.user_id

  caps = [
    {
      type = "users"
      perm = "read"
    }
  ]
}

data "radosgw_iam_user_caps" "test" {
  user_id = radosgw_iam_user.test.user_id

  depends_on = [radosgw_iam_user_caps.test]
}
`, userID)
}

func testAccRadosgwIAMUserCapsDataSourceConfig_multiple(userID string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_user" "test" {
  user_id      = %q
  display_name = "Test User for Multiple Caps Data Source"
}

resource "radosgw_iam_user_caps" "test" {
  user_id = radosgw_iam_user.test.user_id

  caps = [
    {
      type = "users"
      perm = "*"
    },
    {
      type = "buckets"
      perm = "read"
    }
  ]
}

data "radosgw_iam_user_caps" "test" {
  user_id = radosgw_iam_user.test.user_id

  depends_on = [radosgw_iam_user_caps.test]
}
`, userID)
}
