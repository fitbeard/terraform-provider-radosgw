package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccRadosgwIAMAccount_basic(t *testing.T) {
	t.Parallel()

	accountID := "RGW" + randomAccountID()
	accountName := randomName("account")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSkipForVersion(t, CephVersion_Squid) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMAccountDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMAccountConfig_basic(accountID, accountName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMAccountExists("radosgw_iam_account.test"),
					resource.TestCheckResourceAttr("radosgw_iam_account.test", "account_id", accountID),
					resource.TestCheckResourceAttr("radosgw_iam_account.test", "name", accountName),
					// Unspecified limits are omitted from the request, so RadosGW
					// applies its own defaults (1000 for buckets/users, 4 for keys).
					resource.TestCheckResourceAttr("radosgw_iam_account.test", "max_buckets", "1000"),
					resource.TestCheckResourceAttr("radosgw_iam_account.test", "max_access_keys", "4"),
					resource.TestCheckResourceAttr("radosgw_iam_account.test", "max_users", "1000"),
				),
			},
			{
				ResourceName:                         "radosgw_iam_account.test",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateId:                        accountID,
				ImportStateVerifyIdentifierAttribute: "account_id",
			},
		},
	})
}

func TestAccRadosgwIAMAccount_withEmail(t *testing.T) {
	t.Parallel()

	accountID := "RGW" + randomAccountID()
	accountName := randomName("account")
	email := randomEmail()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSkipForVersion(t, CephVersion_Squid) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMAccountDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMAccountConfig_withEmail(accountID, accountName, email),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMAccountExists("radosgw_iam_account.test"),
					resource.TestCheckResourceAttr("radosgw_iam_account.test", "email", email),
				),
			},
		},
	})
}

func TestAccRadosgwIAMAccount_withLimits(t *testing.T) {
	t.Parallel()

	accountID := "RGW" + randomAccountID()
	accountName := randomName("account")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSkipForVersion(t, CephVersion_Squid) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMAccountDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMAccountConfig_withLimits(accountID, accountName, 100, 25, 500),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMAccountExists("radosgw_iam_account.test"),
					resource.TestCheckResourceAttr("radosgw_iam_account.test", "max_users", "100"),
					resource.TestCheckResourceAttr("radosgw_iam_account.test", "max_roles", "25"),
					resource.TestCheckResourceAttr("radosgw_iam_account.test", "max_buckets", "500"),
				),
			},
			{
				// Update name and limits in place (no replacement).
				Config: testAccRadosgwIAMAccountConfig_withLimits(accountID, accountName+"-renamed", 200, 50, 600),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMAccountExists("radosgw_iam_account.test"),
					resource.TestCheckResourceAttr("radosgw_iam_account.test", "name", accountName+"-renamed"),
					resource.TestCheckResourceAttr("radosgw_iam_account.test", "max_users", "200"),
					resource.TestCheckResourceAttr("radosgw_iam_account.test", "max_roles", "50"),
					resource.TestCheckResourceAttr("radosgw_iam_account.test", "max_buckets", "600"),
				),
			},
			{
				// Removing the limits from config does NOT reset them. They are
				// Optional+Computed and retain their last-applied values: RadosGW
				// does not revert an omitted limit, and UseStateForUnknown keeps
				// the known value stable. This deliberately avoids ever sending a
				// sentinel like -1 that would disable bucket creation.
				Config: testAccRadosgwIAMAccountConfig_basic(accountID, accountName+"-renamed"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("radosgw_iam_account.test", plancheck.ResourceActionNoop),
					},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_iam_account.test", "max_users", "200"),
					resource.TestCheckResourceAttr("radosgw_iam_account.test", "max_roles", "50"),
					resource.TestCheckResourceAttr("radosgw_iam_account.test", "max_buckets", "600"),
				),
			},
		},
	})
}

// TestAccRadosgwIAMAccount_withRootUser verifies a user can be created inside an
// account and marked as the account root.
func TestAccRadosgwIAMAccount_withRootUser(t *testing.T) {
	t.Parallel()

	accountID := "RGW" + randomAccountID()
	accountName := randomName("account")
	userID := randomName("acctroot")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSkipForVersion(t, CephVersion_Squid) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMAccountDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMAccountConfig_withRootUser(accountID, accountName, userID, true),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMAccountExists("radosgw_iam_account.test"),
					resource.TestCheckResourceAttr("radosgw_iam_user.root", "account_id", accountID),
					resource.TestCheckResourceAttr("radosgw_iam_user.root", "account_root", "true"),
				),
			},
			{
				// account_root is mutable: toggling it must update in place, not
				// force replacement.
				Config: testAccRadosgwIAMAccountConfig_withRootUser(accountID, accountName, userID, false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_iam_user.root", "account_root", "false"),
				),
			},
			{
				// ...and back to root again.
				Config: testAccRadosgwIAMAccountConfig_withRootUser(accountID, accountName, userID, true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_iam_user.root", "account_root", "true"),
				),
			},
			{
				// Omitting account_root entirely must demote the user (default
				// false), not silently keep the prior root value.
				Config: testAccRadosgwIAMAccountConfig_withAccountUser(accountID, accountName, userID),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_iam_user.root", "account_root", "false"),
				),
			},
		},
	})
}

// TestAccRadosgwIAMAccount_autoIDStableOnUpdate verifies that when the account
// ID is auto-generated by RadosGW (account_id not set), changing another
// attribute updates in place instead of forcing a replacement.
func TestAccRadosgwIAMAccount_autoIDStableOnUpdate(t *testing.T) {
	t.Parallel()

	accountName := randomName("account")

	config := func(maxAccessKeys int64) string {
		return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_account" "auto" {
  name            = %[1]q
  max_access_keys = %[2]d
}
`, accountName, maxAccessKeys)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSkipForVersion(t, CephVersion_Squid) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMAccountDestroy,
		Steps: []resource.TestStep{
			{
				Config: config(4),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMAccountExists("radosgw_iam_account.auto"),
					resource.TestMatchResourceAttr("radosgw_iam_account.auto", "account_id", accountIDRegexp),
				),
			},
			{
				// Changing max_access_keys must be an in-place update; the
				// auto-generated account_id must remain stable (no replacement).
				Config: config(8),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("radosgw_iam_account.auto", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_iam_account.auto", "max_access_keys", "8"),
				),
			},
		},
	})
}

// TestAccRadosgwIAMAccount_userAccountRemovalRecreates verifies that removing a
// user's account_id (returning it to no account) forces a recreate rather than
// being silently ignored.
func TestAccRadosgwIAMAccount_userAccountRemovalRecreates(t *testing.T) {
	t.Parallel()

	accountID := "RGW" + randomAccountID()
	accountName := randomName("account")
	userID := randomName("member")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSkipForVersion(t, CephVersion_Squid) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMAccountDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMAccountConfig_withAccountUser(accountID, accountName, userID),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_iam_user.root", "account_id", accountID),
				),
			},
			{
				// Removing account_id must plan a replacement and clear the
				// association on the recreated user.
				Config: testAccRadosgwIAMAccountConfig_userNoAccount(accountID, accountName, userID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("radosgw_iam_user.root", plancheck.ResourceActionReplace),
					},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_iam_user.root", "account_id", ""),
					resource.TestCheckResourceAttr("radosgw_iam_user.root", "account_root", "false"),
				),
			},
		},
	})
}

// TestAccRadosgwIAMAccount_duplicateNameError verifies that creating an account
// whose name already exists returns a clear, actionable diagnostic.
func TestAccRadosgwIAMAccount_duplicateNameError(t *testing.T) {
	t.Parallel()

	accountName := randomName("account")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSkipForVersion(t, CephVersion_Squid) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMAccountDestroy,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_account" "a" {
  name = %[1]q
}

resource "radosgw_iam_account" "b" {
  name = %[1]q

  # Force sequential creation so "b" is attempted after "a" exists; otherwise
  # Terraform creates them concurrently and RGW's name-uniqueness check can race.
  depends_on = [radosgw_iam_account.a]
}
`, accountName),
				ExpectError: regexp.MustCompile(`(?s)Account Already Exists.*must be unique`),
			},
		},
	})
}

// TestAccRadosgwIAMAccount_userDisplayNameSpacesRejected verifies the plan-time
// validation that rejects whitespace in an account user's display_name before
// any API call is made.
func TestAccRadosgwIAMAccount_userDisplayNameSpacesRejected(t *testing.T) {
	t.Parallel()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + `
resource "radosgw_iam_user" "invalid" {
  user_id      = "acct-user-invalid"
  display_name = "Has Spaces"
  account_id   = "RGW00000000000000001"
}
`,
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`(?s)Invalid display_name for account user.*must not contain spaces`),
			},
		},
	})
}

// TestAccRadosgwIAMAccount_userTenantMismatch verifies that assigning a user to
// an account with a non-matching tenant is rejected (RadosGW requires the user
// tenant to equal the account tenant). Requires an account API that can read and
// delete accounts (Ceph 20.2.2+).
func TestAccRadosgwIAMAccount_userTenantMismatch(t *testing.T) {
	t.Parallel()

	accountID := "RGW" + randomAccountID()
	accountName := randomName("account")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSkipForVersion(t, CephVersion_Squid) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMAccountDestroy,
		Steps: []resource.TestStep{
			{
				Config: providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_account" "test" {
  account_id = %[1]q
  name       = %[2]q
}

resource "radosgw_iam_user" "mismatch" {
  user_id      = "acct-tenant-mismatch"
  display_name = "MismatchUser"
  account_id   = radosgw_iam_account.test.account_id
  tenant       = "wrongtenant"
}
`, accountID, accountName),
				// Matches both the provider's pre-check message and the raw
				// RadosGW error ("User tenant does not match account tenant").
				ExpectError: regexp.MustCompile(`(?i)tenant.*does not match`),
			},
		},
	})
}

func testAccCheckRadosgwIAMAccountExists(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		accountID := rs.Primary.Attributes["account_id"]
		if accountID == "" {
			return fmt.Errorf("account_id not set")
		}

		_, err := testAccAdminClient.GetAccount(testCtx, accountID)
		if err != nil {
			return fmt.Errorf("error fetching account %s: %s", accountID, err)
		}

		return nil
	}
}

func testAccCheckRadosgwIAMAccountDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "radosgw_iam_account" {
			continue
		}

		accountID := rs.Primary.Attributes["account_id"]

		_, err := testAccAdminClient.GetAccount(testCtx, accountID)
		if err == nil {
			return fmt.Errorf("account %s still exists", accountID)
		}
		if !isAccountNotFoundError(err) {
			return fmt.Errorf("unexpected error checking account %s destruction: %s", accountID, err)
		}
	}

	return nil
}

func testAccRadosgwIAMAccountConfig_basic(accountID, name string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_account" "test" {
  account_id = %[1]q
  name       = %[2]q
}
`, accountID, name)
}

func testAccRadosgwIAMAccountConfig_withEmail(accountID, name, email string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_account" "test" {
  account_id = %[1]q
  name       = %[2]q
  email      = %[3]q
}
`, accountID, name, email)
}

func testAccRadosgwIAMAccountConfig_withLimits(accountID, name string, maxUsers, maxRoles, maxBuckets int64) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_account" "test" {
  account_id  = %[1]q
  name        = %[2]q
  max_users   = %[3]d
  max_roles   = %[4]d
  max_buckets = %[5]d
}
`, accountID, name, maxUsers, maxRoles, maxBuckets)
}

// testAccRadosgwIAMAccountConfig_withAccountUser configures the same user as an
// account member but omits account_root entirely (exercises the default-false /
// demote-on-removal behavior).
func testAccRadosgwIAMAccountConfig_withAccountUser(accountID, name, userID string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_account" "test" {
  account_id = %[1]q
  name       = %[2]q
}

resource "radosgw_iam_user" "root" {
  user_id      = %[3]q
  display_name = "AccountRootUser"
  account_id   = radosgw_iam_account.test.account_id
}
`, accountID, name, userID)
}

// testAccRadosgwIAMAccountConfig_userNoAccount keeps the account resource but
// detaches the user from it (no account_id), so removing membership can be
// observed as a recreate.
func testAccRadosgwIAMAccountConfig_userNoAccount(accountID, name, userID string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_account" "test" {
  account_id = %[1]q
  name       = %[2]q
}

resource "radosgw_iam_user" "root" {
  user_id      = %[3]q
  display_name = "AccountRootUser"
}
`, accountID, name, userID)
}

func testAccRadosgwIAMAccountConfig_withRootUser(accountID, name, userID string, accountRoot bool) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_account" "test" {
  account_id = %[1]q
  name       = %[2]q
}

resource "radosgw_iam_user" "root" {
  user_id      = %[3]q
  display_name = "AccountRootUser"
  account_id   = radosgw_iam_account.test.account_id
  account_root = %[4]t
}
`, accountID, name, userID, accountRoot)
}
