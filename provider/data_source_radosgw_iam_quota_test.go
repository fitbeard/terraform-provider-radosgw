package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccRadosgwIAMQuotaDataSource_basic(t *testing.T) {
	t.Parallel()

	userID := randomName("tf-acc-user")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMQuotaDataSourceConfig_basic(userID),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.radosgw_iam_quota.test", "user_id", userID),
					resource.TestCheckResourceAttr("data.radosgw_iam_quota.test", "type", "user"),
					resource.TestCheckResourceAttrSet("data.radosgw_iam_quota.test", "enabled"),
				),
			},
		},
	})
}

func TestAccRadosgwIAMQuotaDataSource_withQuota(t *testing.T) {
	t.Parallel()

	userID := randomName("tf-acc-user")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMQuotaDataSourceConfig_withQuota(userID),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.radosgw_iam_quota.test", "enabled", "true"),
					resource.TestCheckResourceAttr("data.radosgw_iam_quota.test", "max_size", "1073741824"),
					resource.TestCheckResourceAttr("data.radosgw_iam_quota.test", "max_objects", "1000"),
				),
			},
		},
	})
}

// Test configurations

func testAccRadosgwIAMQuotaDataSourceConfig_basic(userID string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_user" "test" {
  user_id      = %q
  display_name = "Test User for Quota Data Source"
}

data "radosgw_iam_quota" "test" {
  user_id = radosgw_iam_user.test.user_id
  type    = "user"

  depends_on = [radosgw_iam_user.test]
}
`, userID)
}

func testAccRadosgwIAMQuotaDataSourceConfig_withQuota(userID string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_user" "test" {
  user_id      = %q
  display_name = "Test User for Quota Data Source"
}

resource "radosgw_iam_quota" "test" {
  user_id     = radosgw_iam_user.test.user_id
  type        = "user"
  enabled     = true
  max_size    = 1073741824
  max_objects = 1000
}

data "radosgw_iam_quota" "test" {
  user_id = radosgw_iam_user.test.user_id
  type    = "user"

  depends_on = [radosgw_iam_quota.test]
}
`, userID)
}
