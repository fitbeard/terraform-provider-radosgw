package provider

import (
	"fmt"
	"testing"

	"github.com/ceph/go-ceph/rgw/admin"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccRadosgwIAMUser_basic(t *testing.T) {
	t.Parallel()

	userID := randomName("tf-acc-user")
	displayName := "Test User"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMUserConfig_basic(userID, displayName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMUserExists("radosgw_iam_user.test"),
					resource.TestCheckResourceAttr("radosgw_iam_user.test", "user_id", userID),
					resource.TestCheckResourceAttr("radosgw_iam_user.test", "display_name", displayName),
					resource.TestCheckResourceAttr("radosgw_iam_user.test", "suspended", "false"),
					resource.TestCheckResourceAttr("radosgw_iam_user.test", "max_buckets", "1000"),
				),
			},
			// Import test
			{
				ResourceName:                         "radosgw_iam_user.test",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateId:                        userID,
				ImportStateVerifyIdentifierAttribute: "user_id",
			},
		},
	})
}

func TestAccRadosgwIAMUser_withEmail(t *testing.T) {
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
				Config: testAccRadosgwIAMUserConfig_withEmail(userID, displayName, email),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMUserExists("radosgw_iam_user.test"),
					resource.TestCheckResourceAttr("radosgw_iam_user.test", "user_id", userID),
					resource.TestCheckResourceAttr("radosgw_iam_user.test", "display_name", displayName),
					resource.TestCheckResourceAttr("radosgw_iam_user.test", "email", email),
				),
			},
		},
	})
}

func TestAccRadosgwIAMUser_withTenant(t *testing.T) {
	t.Parallel()

	userID := randomName("tfaccuser")
	displayName := "Tenant User"
	// Tenant names in RadosGW cannot contain hyphens, use alphanumeric only
	tenant := "tfacctenant"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMUserDestroyWithTenant,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMUserConfig_withTenant(userID, displayName, tenant),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMUserExistsWithTenant("radosgw_iam_user.test", tenant),
					resource.TestCheckResourceAttr("radosgw_iam_user.test", "user_id", userID),
					resource.TestCheckResourceAttr("radosgw_iam_user.test", "tenant", tenant),
				),
			},
			// Import test for tenant user
			{
				ResourceName:                         "radosgw_iam_user.test",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateId:                        tenant + "$" + userID,
				ImportStateVerifyIdentifierAttribute: "user_id",
			},
		},
	})
}

func TestAccRadosgwIAMUser_update(t *testing.T) {
	t.Parallel()

	userID := randomName("tf-acc-user")
	displayNameBefore := "Test User Before"
	displayNameAfter := "Test User After"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMUserConfig_basic(userID, displayNameBefore),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMUserExists("radosgw_iam_user.test"),
					resource.TestCheckResourceAttr("radosgw_iam_user.test", "display_name", displayNameBefore),
				),
			},
			{
				Config: testAccRadosgwIAMUserConfig_basic(userID, displayNameAfter),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMUserExists("radosgw_iam_user.test"),
					resource.TestCheckResourceAttr("radosgw_iam_user.test", "display_name", displayNameAfter),
				),
			},
		},
	})
}

func TestAccRadosgwIAMUser_suspended(t *testing.T) {
	t.Parallel()

	userID := randomName("tf-acc-user")
	displayName := "Suspended User"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMUserConfig_suspended(userID, displayName, false),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMUserExists("radosgw_iam_user.test"),
					resource.TestCheckResourceAttr("radosgw_iam_user.test", "suspended", "false"),
				),
			},
			{
				Config: testAccRadosgwIAMUserConfig_suspended(userID, displayName, true),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMUserExists("radosgw_iam_user.test"),
					resource.TestCheckResourceAttr("radosgw_iam_user.test", "suspended", "true"),
				),
			},
		},
	})
}

func TestAccRadosgwIAMUser_maxBuckets(t *testing.T) {
	t.Parallel()

	userID := randomName("tf-acc-user")
	displayName := "Max Buckets User"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMUserDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMUserConfig_maxBuckets(userID, displayName, 10),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMUserExists("radosgw_iam_user.test"),
					resource.TestCheckResourceAttr("radosgw_iam_user.test", "max_buckets", "10"),
				),
			},
			{
				Config: testAccRadosgwIAMUserConfig_maxBuckets(userID, displayName, 50),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMUserExists("radosgw_iam_user.test"),
					resource.TestCheckResourceAttr("radosgw_iam_user.test", "max_buckets", "50"),
				),
			},
		},
	})
}

// Helper functions

func testAccCheckRadosgwIAMUserExists(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		userID := rs.Primary.Attributes["user_id"]
		if userID == "" {
			return fmt.Errorf("user_id not set")
		}

		_, err := testAccAdminClient.GetUser(testCtx, admin.User{ID: userID})
		if err != nil {
			return fmt.Errorf("error fetching user %s: %s", userID, err)
		}

		return nil
	}
}

func testAccCheckRadosgwIAMUserExistsWithTenant(resourceName, tenant string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		userID := rs.Primary.Attributes["user_id"]
		if userID == "" {
			return fmt.Errorf("user_id not set")
		}

		// When user has a tenant, the full user ID is "tenant$user_id"
		fullUserID := tenant + "$" + userID
		_, err := testAccAdminClient.GetUser(testCtx, admin.User{ID: fullUserID})
		if err != nil {
			return fmt.Errorf("error fetching user %s: %s", fullUserID, err)
		}

		return nil
	}
}

func testAccCheckRadosgwIAMUserDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "radosgw_iam_user" {
			continue
		}

		userID := rs.Primary.Attributes["user_id"]
		tenant := rs.Primary.Attributes["tenant"]

		// Build full user ID for API call
		fullUserID := userID
		if tenant != "" {
			fullUserID = tenant + "$" + userID
		}

		_, err := testAccAdminClient.GetUser(testCtx, admin.User{ID: fullUserID})
		if err == nil {
			return fmt.Errorf("user %s still exists", fullUserID)
		}
	}

	return nil
}

func testAccCheckRadosgwIAMUserDestroyWithTenant(s *terraform.State) error {
	return testAccCheckRadosgwIAMUserDestroy(s)
}

// Test configurations

func testAccRadosgwIAMUserConfig_basic(userID, displayName string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_user" "test" {
  user_id      = %q
  display_name = %q
}
`, userID, displayName)
}

func testAccRadosgwIAMUserConfig_withEmail(userID, displayName, email string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_user" "test" {
  user_id      = %q
  display_name = %q
  email        = %q
}
`, userID, displayName, email)
}

func testAccRadosgwIAMUserConfig_withTenant(userID, displayName, tenant string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_user" "test" {
  user_id      = %q
  display_name = %q
  tenant       = %q
}
`, userID, displayName, tenant)
}

func testAccRadosgwIAMUserConfig_suspended(userID, displayName string, suspended bool) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_user" "test" {
  user_id      = %q
  display_name = %q
  suspended    = %t
}
`, userID, displayName, suspended)
}

func testAccRadosgwIAMUserConfig_maxBuckets(userID, displayName string, maxBuckets int) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_user" "test" {
  user_id      = %q
  display_name = %q
  max_buckets  = %d
}
`, userID, displayName, maxBuckets)
}
