package provider

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/ceph/go-ceph/rgw/admin"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// =============================================================================
// Shared helpers: bootstrap an account + root user out-of-band (admin), so an
// aliased provider configured as the root can drive the IAM-plane resources.
// =============================================================================

type testAccountRoot struct {
	accountID string
	rootUID   string
	accessKey string
	secretKey string
}

func newTestAccountRoot() testAccountRoot {
	return testAccountRoot{
		accountID: "RGW" + randomAccountID(),
		rootUID:   randomName("acctroot"),
		accessKey: strings.ReplaceAll(randomName("rootak"), "-", ""),
		secretKey: strings.ReplaceAll(randomName("rootsk"), "-", ""),
	}
}

func (ar testAccountRoot) create(t *testing.T) {
	t.Helper()
	if _, err := testAccAdminClient.CreateAccount(testCtx, admin.Account{ID: ar.accountID, Name: randomName("account")}); err != nil {
		t.Fatalf("failed to create account: %s", err)
	}
	accountRoot := true
	if _, err := testAccAdminClient.CreateUser(testCtx, admin.User{
		ID: ar.rootUID, DisplayName: ar.rootUID, AccountID: ar.accountID, AccountRoot: &accountRoot,
	}); err != nil {
		t.Fatalf("failed to create account root user: %s", err)
	}
	gen := false
	if _, err := testAccAdminClient.CreateKey(testCtx, admin.UserKeySpec{
		UID: ar.rootUID, AccessKey: ar.accessKey, SecretKey: ar.secretKey, GenerateKey: &gen,
	}); err != nil {
		t.Fatalf("failed to create account root key: %s", err)
	}
}

func (ar testAccountRoot) destroy(_ *terraform.State) error {
	_ = testAccAdminClient.RemoveUser(testCtx, admin.User{ID: ar.rootUID})
	_ = testAccAdminClient.DeleteAccount(testCtx, ar.accountID)
	return nil
}

// iamClient returns an IAM client authenticated as this account root.
func (ar testAccountRoot) iamClient() *IAMClient {
	return NewIAMClient(os.Getenv("RADOSGW_ENDPOINT"), ar.accessKey, ar.secretKey, nil)
}

// rootProviderConfig emits the default provider plus an aliased "root" provider.
func (ar testAccountRoot) rootProviderConfig() string {
	return providerConfig() + fmt.Sprintf(`
provider "radosgw" {
  alias      = "root"
  endpoint   = %q
  access_key = %q
  secret_key = %q
}
`, os.Getenv("RADOSGW_ENDPOINT"), ar.accessKey, ar.secretKey)
}

// =============================================================================
// Tests
// =============================================================================

// TestAccRadosgwIAMAccountUser_basic: an account root (no admin caps)
// creating an IAM user and access key within its account, plus the
// three account-IAM data sources (users list filtered by path AND name_regex).
func TestAccRadosgwIAMAccountUser_basic(t *testing.T) {
	t.Parallel()

	ar := newTestAccountRoot()
	userName := strings.ReplaceAll(randomName("iamuser"), "-", "")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSkipForVersion(t, CephVersion_Squid) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             ar.destroy,
		Steps: []resource.TestStep{
			{
				PreConfig: func() { ar.create(t) },
				Config:    ar.rootProviderConfig() + testAccIAMAccountUserAndKey(userName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_iam_account_user.test", "name", userName),
					resource.TestCheckResourceAttr("radosgw_iam_account_user.test", "path", "/eng/"),
					resource.TestCheckResourceAttrSet("radosgw_iam_account_user.test", "arn"),
					resource.TestCheckResourceAttrSet("radosgw_iam_account_user.test", "unique_id"),
					resource.TestCheckResourceAttrSet("radosgw_iam_account_access_key.test", "access_key"),
					resource.TestCheckResourceAttrSet("radosgw_iam_account_access_key.test", "secret_key"),
					resource.TestCheckResourceAttr("radosgw_iam_account_access_key.test", "status", "active"),
					resource.TestCheckResourceAttrPair("data.radosgw_iam_account_user.test", "arn", "radosgw_iam_account_user.test", "arn"),
					// users list filtered by path_prefix AND name_regex
					resource.TestCheckResourceAttr("data.radosgw_iam_account_users.by_path", "names.#", "1"),
					resource.TestCheckResourceAttr("data.radosgw_iam_account_users.by_regex", "names.#", "1"),
					resource.TestCheckResourceAttr("data.radosgw_iam_account_users.no_match", "names.#", "0"),
					// access keys list
					resource.TestCheckResourceAttr("data.radosgw_iam_account_access_keys.test", "access_keys.#", "1"),
					resource.TestCheckResourceAttrPair("data.radosgw_iam_account_access_keys.test", "access_keys.0.access_key_id", "radosgw_iam_account_access_key.test", "access_key"),
				),
			},
		},
	})
}

// TestAccRadosgwIAMAccountUser_updateAndImport covers rename + re-path (which
// recomputes the ARN — regression guard for the arn plan modifier) and import.
func TestAccRadosgwIAMAccountUser_updateAndImport(t *testing.T) {
	t.Parallel()

	ar := newTestAccountRoot()
	name1 := strings.ReplaceAll(randomName("u1"), "-", "")
	name2 := strings.ReplaceAll(randomName("u2"), "-", "")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSkipForVersion(t, CephVersion_Squid) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             ar.destroy,
		Steps: []resource.TestStep{
			{
				PreConfig: func() { ar.create(t) },
				Config:    ar.rootProviderConfig() + testAccIAMAccountUserOnly(name1, "/eng/"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_iam_account_user.test", "name", name1),
					resource.TestCheckResourceAttrSet("radosgw_iam_account_user.test", "arn"),
				),
			},
			{
				// rename + re-path in place; the ARN must be recomputed.
				Config: ar.rootProviderConfig() + testAccIAMAccountUserOnly(name2, "/ops/"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_iam_account_user.test", "name", name2),
					resource.TestCheckResourceAttr("radosgw_iam_account_user.test", "path", "/ops/"),
					resource.TestMatchResourceAttr("radosgw_iam_account_user.test", "arn", regexp.MustCompile("user/ops/"+name2+"$")),
				),
			},
			{
				ResourceName:                         "radosgw_iam_account_user.test",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateId:                        name2,
				ImportStateVerifyIdentifierAttribute: "name",
				// force_destroy has no API representation, so it is not restored on import.
				ImportStateVerifyIgnore: []string{"force_destroy"},
			},
		},
	})
}

// TestAccRadosgwIAMAccountAccessKey_statusAndImport covers the access key status
// toggle (UpdateAccessKey) and import by "user:access_key".
func TestAccRadosgwIAMAccountAccessKey_statusAndImport(t *testing.T) {
	t.Parallel()

	ar := newTestAccountRoot()
	userName := strings.ReplaceAll(randomName("keyuser"), "-", "")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSkipForVersion(t, CephVersion_Squid) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             ar.destroy,
		Steps: []resource.TestStep{
			{
				PreConfig: func() { ar.create(t) },
				Config:    ar.rootProviderConfig() + testAccIAMAccountKeyWithStatus(userName, "active"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_iam_account_access_key.test", "status", "active"),
					resource.TestCheckResourceAttrSet("radosgw_iam_account_access_key.test", "secret_key"),
				),
			},
			{
				// toggle to inactive in place
				Config: ar.rootProviderConfig() + testAccIAMAccountKeyWithStatus(userName, "inactive"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_iam_account_access_key.test", "status", "inactive"),
				),
			},
			{
				ResourceName:      "radosgw_iam_account_access_key.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs := s.RootModule().Resources["radosgw_iam_account_access_key.test"]
					return rs.Primary.Attributes["user"] + ":" + rs.Primary.Attributes["access_key"], nil
				},
				ImportStateVerifyIdentifierAttribute: "access_key",
				// secret_key is only returned at creation and cannot be recovered on import.
				ImportStateVerifyIgnore: []string{"secret_key"},
			},
		},
	})
}

// TestAccRadosgwIAMAccountUser_forceDestroy verifies that force_destroy removes a
// user's access keys and inline policies (added out-of-band here) so the user
// can be deleted — a bare DeleteUser would fail with DeleteConflict.
func TestAccRadosgwIAMAccountUser_forceDestroy(t *testing.T) {
	t.Parallel()

	ar := newTestAccountRoot()
	userName := strings.ReplaceAll(randomName("fduser"), "-", "")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSkipForVersion(t, CephVersion_Squid) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             ar.destroy,
		Steps: []resource.TestStep{
			{
				PreConfig: func() { ar.create(t) },
				Config:    ar.rootProviderConfig() + testAccIAMAccountUserForceDestroy(userName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_iam_account_user.test", "name", userName),
					// Attach a key AND an inline policy OUT OF BAND so a plain DeleteUser
					// would DeleteConflict; force_destroy must purge them on teardown.
					func(_ *terraform.State) error {
						c := ar.iamClient()

						createKey := url.Values{}
						createKey.Set("Action", "CreateAccessKey")
						createKey.Set("UserName", userName)
						if _, err := c.DoRequest(testCtx, createKey, "iam"); err != nil {
							return fmt.Errorf("out-of-band CreateAccessKey: %w", err)
						}

						putPolicy := url.Values{}
						putPolicy.Set("Action", "PutUserPolicy")
						putPolicy.Set("UserName", userName)
						putPolicy.Set("PolicyName", "extra")
						putPolicy.Set("PolicyDocument", `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"s3:*","Resource":"*"}]}`)
						if _, err := c.DoRequest(testCtx, putPolicy, "iam"); err != nil {
							return fmt.Errorf("out-of-band PutUserPolicy: %w", err)
						}
						return nil
					},
				),
			},
		},
	})
}

// =============================================================================
// Config helpers
// =============================================================================

func testAccIAMAccountUserOnly(name, path string) string {
	return fmt.Sprintf(`
resource "radosgw_iam_account_user" "test" {
  provider      = radosgw.root
  name          = %q
  path          = %q
  force_destroy = true
}
`, name, path)
}

func testAccIAMAccountUserForceDestroy(name string) string {
	return fmt.Sprintf(`
resource "radosgw_iam_account_user" "test" {
  provider      = radosgw.root
  name          = %q
  force_destroy = true
}
`, name)
}

func testAccIAMAccountKeyWithStatus(userName, status string) string {
	return fmt.Sprintf(`
resource "radosgw_iam_account_user" "test" {
  provider      = radosgw.root
  name          = %q
  force_destroy = true
}

resource "radosgw_iam_account_access_key" "test" {
  provider = radosgw.root
  user     = radosgw_iam_account_user.test.name
  status   = %q
}
`, userName, status)
}

func testAccIAMAccountUserAndKey(userName string) string {
	return fmt.Sprintf(`
resource "radosgw_iam_account_user" "test" {
  provider      = radosgw.root
  name          = %[1]q
  path          = "/eng/"
  force_destroy = true
}

resource "radosgw_iam_account_access_key" "test" {
  provider = radosgw.root
  user     = radosgw_iam_account_user.test.name
}

data "radosgw_iam_account_user" "test" {
  provider   = radosgw.root
  name       = radosgw_iam_account_user.test.name
  depends_on = [radosgw_iam_account_user.test]
}

data "radosgw_iam_account_users" "by_path" {
  provider    = radosgw.root
  path_prefix = "/eng/"
  depends_on  = [radosgw_iam_account_user.test]
}

data "radosgw_iam_account_users" "by_regex" {
  provider   = radosgw.root
  name_regex = "^%[1]s$"
  depends_on = [radosgw_iam_account_user.test]
}

data "radosgw_iam_account_users" "no_match" {
  provider    = radosgw.root
  path_prefix = "/nonexistent/"
  depends_on  = [radosgw_iam_account_user.test]
}

data "radosgw_iam_account_access_keys" "test" {
  provider   = radosgw.root
  user       = radosgw_iam_account_user.test.name
  depends_on = [radosgw_iam_account_access_key.test]
}
`, userName)
}
