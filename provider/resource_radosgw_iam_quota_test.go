package provider

import (
	"fmt"
	"testing"

	"github.com/ceph/go-ceph/rgw/admin"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccRadosgwIAMQuota_basic(t *testing.T) {
	t.Parallel()

	userID := randomName("tf-acc-user")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMQuotaConfig_basic(userID),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMQuotaExists("radosgw_iam_quota.test"),
					resource.TestCheckResourceAttr("radosgw_iam_quota.test", "user_id", userID),
					resource.TestCheckResourceAttr("radosgw_iam_quota.test", "type", "user"),
					resource.TestCheckResourceAttr("radosgw_iam_quota.test", "enabled", "true"),
					resource.TestCheckResourceAttr("radosgw_iam_quota.test", "max_size", "1073741824"),
					resource.TestCheckResourceAttr("radosgw_iam_quota.test", "max_objects", "1000"),
				),
			},
			// Import test - format: user_id:type
			{
				ResourceName:                         "radosgw_iam_quota.test",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateId:                        userID + ":user",
				ImportStateVerifyIdentifierAttribute: "user_id",
			},
		},
	})
}

func TestAccRadosgwIAMQuota_bucketType(t *testing.T) {
	t.Parallel()

	userID := randomName("tf-acc-user")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMQuotaConfig_bucketType(userID),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMQuotaExists("radosgw_iam_quota.test"),
					resource.TestCheckResourceAttr("radosgw_iam_quota.test", "type", "bucket"),
				),
			},
		},
	})
}

func TestAccRadosgwIAMQuota_update(t *testing.T) {
	t.Parallel()

	userID := randomName("tf-acc-user")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMQuotaConfig_full(userID, "user", true, 1073741824, 1000),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMQuotaExists("radosgw_iam_quota.test"),
					resource.TestCheckResourceAttr("radosgw_iam_quota.test", "enabled", "true"),
					resource.TestCheckResourceAttr("radosgw_iam_quota.test", "max_size", "1073741824"),
					resource.TestCheckResourceAttr("radosgw_iam_quota.test", "max_objects", "1000"),
				),
			},
			{
				Config: testAccRadosgwIAMQuotaConfig_full(userID, "user", true, 2147483648, 5000),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMQuotaExists("radosgw_iam_quota.test"),
					resource.TestCheckResourceAttr("radosgw_iam_quota.test", "max_size", "2147483648"),
					resource.TestCheckResourceAttr("radosgw_iam_quota.test", "max_objects", "5000"),
				),
			},
		},
	})
}

func TestAccRadosgwIAMQuota_disable(t *testing.T) {
	t.Parallel()

	userID := randomName("tf-acc-user")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMQuotaConfig_full(userID, "user", true, 1073741824, 1000),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_iam_quota.test", "enabled", "true"),
				),
			},
			{
				Config: testAccRadosgwIAMQuotaConfig_full(userID, "user", false, 1073741824, 1000),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_iam_quota.test", "enabled", "false"),
				),
			},
		},
	})
}

// Helper functions

func testAccCheckRadosgwIAMQuotaExists(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		userID := rs.Primary.Attributes["user_id"]
		if userID == "" {
			return fmt.Errorf("user_id not set")
		}

		user, err := testAccAdminClient.GetUser(testCtx, admin.User{ID: userID})
		if err != nil {
			return fmt.Errorf("error fetching user %s: %s", userID, err)
		}

		if user.UserQuota.MaxSize == nil {
			return fmt.Errorf("user quota not set for user %s", userID)
		}

		return nil
	}
}

// Test configurations

func testAccRadosgwIAMQuotaConfig_basic(userID string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_user" "test" {
  user_id      = %q
  display_name = "Test User for Quota"
}

resource "radosgw_iam_quota" "test" {
  user_id     = radosgw_iam_user.test.user_id
  type        = "user"
  enabled     = true
  max_size    = 1073741824
  max_objects = 1000
}
`, userID)
}

func testAccRadosgwIAMQuotaConfig_bucketType(userID string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_user" "test" {
  user_id      = %q
  display_name = "Test User for Bucket Quota"
}

resource "radosgw_iam_quota" "test" {
  user_id     = radosgw_iam_user.test.user_id
  type        = "bucket"
  enabled     = true
  max_size    = 536870912
  max_objects = 500
}
`, userID)
}

func testAccRadosgwIAMQuotaConfig_full(userID, quotaType string, enabled bool, maxSize, maxObjects int64) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_user" "test" {
  user_id      = %q
  display_name = "Test User for Quota"
}

resource "radosgw_iam_quota" "test" {
  user_id     = radosgw_iam_user.test.user_id
  type        = %q
  enabled     = %t
  max_size    = %d
  max_objects = %d
}
`, userID, quotaType, enabled, maxSize, maxObjects)
}
