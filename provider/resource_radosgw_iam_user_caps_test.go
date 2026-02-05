package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/ceph/go-ceph/rgw/admin"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccRadosgwIAMUserCaps_basic(t *testing.T) {
	t.Parallel()

	userID := randomName("tf-acc-user")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMUserCapsConfig_basic(userID),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMUserCapsExists("radosgw_iam_user_caps.test"),
					resource.TestCheckResourceAttr("radosgw_iam_user_caps.test", "user_id", userID),
					resource.TestCheckResourceAttr("radosgw_iam_user_caps.test", "caps.#", "1"),
				),
			},
			// Import test - format: user_id
			{
				ResourceName:                         "radosgw_iam_user_caps.test",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateId:                        userID,
				ImportStateVerifyIdentifierAttribute: "user_id",
			},
		},
	})
}

func TestAccRadosgwIAMUserCaps_multiple(t *testing.T) {
	t.Parallel()

	userID := randomName("tf-acc-user")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMUserCapsConfig_multiple(userID),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMUserCapsExists("radosgw_iam_user_caps.test"),
					resource.TestCheckResourceAttr("radosgw_iam_user_caps.test", "caps.#", "3"),
				),
			},
		},
	})
}

func TestAccRadosgwIAMUserCaps_update(t *testing.T) {
	t.Parallel()

	userID := randomName("tf-acc-user")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMUserCapsConfig_basic(userID),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMUserCapsExists("radosgw_iam_user_caps.test"),
					resource.TestCheckResourceAttr("radosgw_iam_user_caps.test", "caps.#", "1"),
				),
			},
			{
				Config: testAccRadosgwIAMUserCapsConfig_multiple(userID),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMUserCapsExists("radosgw_iam_user_caps.test"),
					resource.TestCheckResourceAttr("radosgw_iam_user_caps.test", "caps.#", "3"),
				),
			},
		},
	})
}

func TestAccRadosgwIAMUserCaps_duplicateTypesValidation(t *testing.T) {
	t.Parallel()

	userID := randomName("tf-acc-user")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccRadosgwIAMUserCapsConfig_duplicateTypes(userID),
				ExpectError: regexp.MustCompile(`Duplicate Capability Type`),
			},
		},
	})
}

// Unit test for the validator that doesn't require a live RadosGW instance
func TestUniqueCapTypesValidator(t *testing.T) {
	t.Parallel()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testUniqueCapTypesValidatorConfig_duplicateTypes(),
				ExpectError: regexp.MustCompile(`Duplicate Capability Type`),
			},
		},
	})
}

// Helper functions

func testAccCheckRadosgwIAMUserCapsExists(resourceName string) resource.TestCheckFunc {
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

		if len(user.Caps) == 0 {
			return fmt.Errorf("user %s has no caps", userID)
		}

		return nil
	}
}

// Test configurations

func testAccRadosgwIAMUserCapsConfig_basic(userID string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_user" "test" {
  user_id      = %q
  display_name = "Test User for Caps"
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
`, userID)
}

func testAccRadosgwIAMUserCapsConfig_multiple(userID string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_user" "test" {
  user_id      = %q
  display_name = "Test User for Multiple Caps"
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
    },
    {
      type = "metadata"
      perm = "read"
    }
  ]
}
`, userID)
}

func testAccRadosgwIAMUserCapsConfig_duplicateTypes(userID string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_user" "test" {
  user_id      = %q
  display_name = "Test User for Duplicate Caps"
}

resource "radosgw_iam_user_caps" "test" {
  user_id = radosgw_iam_user.test.user_id

  caps = [
    {
      type = "users"
      perm = "read"
    },
    {
      type = "users"
      perm = "write"
    }
  ]
}
`, userID)
}

func testUniqueCapTypesValidatorConfig_duplicateTypes() string {
	return `
provider "radosgw" {
  endpoint   = "http://localhost:7480"
  access_key = "test"
  secret_key = "test"
}

resource "radosgw_iam_user_caps" "test" {
  user_id = "testuser"

  caps = [
    {
      type = "users"
      perm = "read"
    },
    {
      type = "users"
      perm = "write"
    }
  ]
}
`
}
