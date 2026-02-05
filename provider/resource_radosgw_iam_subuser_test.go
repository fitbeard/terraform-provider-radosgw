package provider

import (
	"fmt"
	"testing"

	"github.com/ceph/go-ceph/rgw/admin"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccRadosgwIAMSubuser_basic(t *testing.T) {
	t.Parallel()

	userID := randomName("tf-acc-user")
	subuser := "swift"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMSubuserConfig_basic(userID, subuser),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMSubuserExists("radosgw_iam_subuser.test"),
					resource.TestCheckResourceAttr("radosgw_iam_subuser.test", "user_id", userID),
					resource.TestCheckResourceAttr("radosgw_iam_subuser.test", "subuser", subuser),
					resource.TestCheckResourceAttr("radosgw_iam_subuser.test", "access", "full-control"),
				),
			},
			// Import test - format: user_id:subuser
			{
				ResourceName:                         "radosgw_iam_subuser.test",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateVerifyIgnore:              []string{"secret_key"},
				ImportStateId:                        userID + ":" + subuser,
				ImportStateVerifyIdentifierAttribute: "id",
			},
		},
	})
}

func TestAccRadosgwIAMSubuser_readOnly(t *testing.T) {
	t.Parallel()

	userID := randomName("tf-acc-user")
	subuser := "readonly"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMSubuserConfig_access(userID, subuser, "read"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMSubuserExists("radosgw_iam_subuser.test"),
					resource.TestCheckResourceAttr("radosgw_iam_subuser.test", "access", "read"),
				),
			},
		},
	})
}

func TestAccRadosgwIAMSubuser_update(t *testing.T) {
	t.Parallel()

	userID := randomName("tf-acc-user")
	subuser := "swift"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMSubuserConfig_access(userID, subuser, "read"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_iam_subuser.test", "access", "read"),
				),
			},
			{
				Config: testAccRadosgwIAMSubuserConfig_access(userID, subuser, "full-control"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_iam_subuser.test", "access", "full-control"),
				),
			},
		},
	})
}

// Helper functions

func testAccCheckRadosgwIAMSubuserExists(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		userID := rs.Primary.Attributes["user_id"]
		subuser := rs.Primary.Attributes["subuser"]
		if userID == "" || subuser == "" {
			return fmt.Errorf("user_id or subuser not set")
		}

		user, err := testAccAdminClient.GetUser(testCtx, admin.User{ID: userID})
		if err != nil {
			return fmt.Errorf("error fetching user %s: %s", userID, err)
		}

		fullSubuserID := userID + ":" + subuser
		for _, su := range user.Subusers {
			if su.Name == fullSubuserID {
				return nil
			}
		}

		return fmt.Errorf("subuser %s not found for user %s", subuser, userID)
	}
}

// Test configurations

func testAccRadosgwIAMSubuserConfig_basic(userID, subuser string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_user" "test" {
  user_id      = %q
  display_name = "Test User for Subuser"
}

resource "radosgw_iam_subuser" "test" {
  user_id = radosgw_iam_user.test.user_id
  subuser = %q
  access  = "full-control"
}
`, userID, subuser)
}

func testAccRadosgwIAMSubuserConfig_access(userID, subuser, access string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_user" "test" {
  user_id      = %q
  display_name = "Test User for Subuser"
}

resource "radosgw_iam_subuser" "test" {
  user_id = radosgw_iam_user.test.user_id
  subuser = %q
  access  = %q
}
`, userID, subuser, access)
}
