package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccRadosgwIAMAccessKeysDataSource_basic(t *testing.T) {
	t.Parallel()

	userID := randomName("tf-acc-user")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMAccessKeysDataSourceConfig_basic(userID),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.radosgw_iam_access_keys.test", "user_id", userID),
					resource.TestCheckResourceAttr("data.radosgw_iam_access_keys.test", "access_keys.#", "1"),
				),
			},
		},
	})
}

// TestAccRadosgwIAMAccessKeysDataSource_multiple tests listing multiple access keys.
// Multiple access key management has issues in Reef (18.x), requires Squid (19.x) or higher.
func TestAccRadosgwIAMAccessKeysDataSource_multiple(t *testing.T) {
	t.Parallel()

	userID := randomName("tf-acc-user")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSkipForVersion(t, CephVersion_Squid) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMAccessKeysDataSourceConfig_multiple(userID),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.radosgw_iam_access_keys.test", "access_keys.#", "2"),
				),
			},
		},
	})
}

func TestAccRadosgwIAMAccessKeysDataSource_withKeyType(t *testing.T) {
	t.Parallel()

	userID := randomName("tf-acc-user")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMAccessKeysDataSourceConfig_withKeyType(userID, "s3"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.radosgw_iam_access_keys.test", "key_type", "s3"),
				),
			},
		},
	})
}

// Test configurations

func testAccRadosgwIAMAccessKeysDataSourceConfig_basic(userID string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_user" "test" {
  user_id      = %q
  display_name = "Test User for Access Keys Data Source"
}

resource "radosgw_iam_access_key" "test" {
  user_id = radosgw_iam_user.test.user_id
}

data "radosgw_iam_access_keys" "test" {
  user_id = radosgw_iam_user.test.user_id

  depends_on = [radosgw_iam_access_key.test]
}
`, userID)
}

func testAccRadosgwIAMAccessKeysDataSourceConfig_multiple(userID string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_user" "test" {
  user_id      = %q
  display_name = "Test User for Multiple Access Keys"
}

resource "radosgw_iam_access_key" "test1" {
  user_id = radosgw_iam_user.test.user_id
}

resource "radosgw_iam_access_key" "test2" {
  user_id = radosgw_iam_user.test.user_id
}

data "radosgw_iam_access_keys" "test" {
  user_id = radosgw_iam_user.test.user_id

  depends_on = [
    radosgw_iam_access_key.test1,
    radosgw_iam_access_key.test2,
  ]
}
`, userID)
}

func testAccRadosgwIAMAccessKeysDataSourceConfig_withKeyType(userID, keyType string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_user" "test" {
  user_id      = %q
  display_name = "Test User for Key Type Filter"
}

resource "radosgw_iam_access_key" "test" {
  user_id = radosgw_iam_user.test.user_id
}

data "radosgw_iam_access_keys" "test" {
  user_id  = radosgw_iam_user.test.user_id
  key_type = %q

  depends_on = [radosgw_iam_access_key.test]
}
`, userID, keyType)
}
