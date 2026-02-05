package provider

import (
	"fmt"
	"testing"

	"github.com/ceph/go-ceph/rgw/admin"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccRadosgwIAMAccessKey_basic(t *testing.T) {
	t.Parallel()

	userID := randomName("tf-acc-user")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMAccessKeyDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMAccessKeyConfig_basic(userID),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMAccessKeyExists("radosgw_iam_access_key.test"),
					resource.TestCheckResourceAttr("radosgw_iam_access_key.test", "user_id", userID),
					resource.TestCheckResourceAttrSet("radosgw_iam_access_key.test", "access_key"),
					resource.TestCheckResourceAttrSet("radosgw_iam_access_key.test", "secret_key"),
				),
			},
		},
	})
}

func TestAccRadosgwIAMAccessKey_withSpecifiedKeys(t *testing.T) {
	t.Parallel()

	userID := randomName("tf-acc-user")
	accessKey := "TESTACCKEY" + randomName("")[:10]
	secretKey := "testsecretkey" + randomName("")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMAccessKeyDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMAccessKeyConfig_withKeys(userID, accessKey, secretKey),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMAccessKeyExists("radosgw_iam_access_key.test"),
					resource.TestCheckResourceAttr("radosgw_iam_access_key.test", "access_key", accessKey),
					resource.TestCheckResourceAttr("radosgw_iam_access_key.test", "secret_key", secretKey),
				),
			},
			// Import test - format: s3:user_id:access_key
			{
				ResourceName:                         "radosgw_iam_access_key.test",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateVerifyIgnore:              []string{"secret_key"},
				ImportStateId:                        "s3:" + userID + ":" + accessKey,
				ImportStateVerifyIdentifierAttribute: "id",
			},
		},
	})
}

// TestAccRadosgwIAMAccessKey_multiple tests creating multiple access keys for a user.
// Multiple access key management has issues in Reef (18.x), requires Squid (19.x) or higher.
func TestAccRadosgwIAMAccessKey_multiple(t *testing.T) {
	t.Parallel()

	userID := randomName("tf-acc-user")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSkipForVersion(t, CephVersion_Squid) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMAccessKeyDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMAccessKeyConfig_multiple(userID),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMAccessKeyExists("radosgw_iam_access_key.test1"),
					testAccCheckRadosgwIAMAccessKeyExists("radosgw_iam_access_key.test2"),
				),
			},
		},
	})
}

func TestAccRadosgwIAMAccessKey_updateSecretKey(t *testing.T) {
	t.Parallel()

	userID := randomName("tf-acc-user")
	accessKey := "TESTACCKEY" + randomName("")[:10]
	secretKey1 := "testsecretkey1_" + randomName("")
	secretKey2 := "testsecretkey2_" + randomName("")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMAccessKeyDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMAccessKeyConfig_withKeys(userID, accessKey, secretKey1),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMAccessKeyExists("radosgw_iam_access_key.test"),
					resource.TestCheckResourceAttr("radosgw_iam_access_key.test", "access_key", accessKey),
					resource.TestCheckResourceAttr("radosgw_iam_access_key.test", "secret_key", secretKey1),
				),
			},
			// Update secret_key in place
			{
				Config: testAccRadosgwIAMAccessKeyConfig_withKeys(userID, accessKey, secretKey2),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMAccessKeyExists("radosgw_iam_access_key.test"),
					resource.TestCheckResourceAttr("radosgw_iam_access_key.test", "access_key", accessKey),
					resource.TestCheckResourceAttr("radosgw_iam_access_key.test", "secret_key", secretKey2),
				),
			},
		},
	})
}

func TestAccRadosgwIAMAccessKey_swift(t *testing.T) {
	t.Parallel()

	userID := randomName("tf-acc-user")
	subuserName := "swiftuser"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMAccessKeyDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMAccessKeyConfig_swift(userID, subuserName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_iam_access_key.swift", "key_type", "swift"),
					resource.TestCheckResourceAttr("radosgw_iam_access_key.swift", "subuser", subuserName),
					resource.TestCheckResourceAttrSet("radosgw_iam_access_key.swift", "secret_key"),
				),
			},
			// Import test - format: swift:user_id:subuser
			{
				ResourceName:                         "radosgw_iam_access_key.swift",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateVerifyIgnore:              []string{"secret_key"},
				ImportStateId:                        "swift:" + userID + ":" + subuserName,
				ImportStateVerifyIdentifierAttribute: "id",
			},
		},
	})
}

// Helper functions

func testAccCheckRadosgwIAMAccessKeyExists(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		userID := rs.Primary.Attributes["user_id"]
		accessKey := rs.Primary.Attributes["access_key"]
		if userID == "" || accessKey == "" {
			return fmt.Errorf("user_id or access_key not set")
		}

		user, err := testAccAdminClient.GetUser(testCtx, admin.User{ID: userID})
		if err != nil {
			return fmt.Errorf("error fetching user %s: %s", userID, err)
		}

		// Check if key exists
		for _, key := range user.Keys {
			if key.AccessKey == accessKey {
				return nil
			}
		}

		return fmt.Errorf("access key %s not found for user %s", accessKey, userID)
	}
}

func testAccCheckRadosgwIAMAccessKeyDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "radosgw_iam_access_key" {
			continue
		}

		userID := rs.Primary.Attributes["user_id"]
		accessKey := rs.Primary.Attributes["access_key"]

		// First check if user still exists
		user, err := testAccAdminClient.GetUser(testCtx, admin.User{ID: userID})
		if err != nil {
			// User doesn't exist, key is definitely destroyed
			continue
		}

		// Check if key still exists
		for _, key := range user.Keys {
			if key.AccessKey == accessKey {
				return fmt.Errorf("access key %s still exists for user %s", accessKey, userID)
			}
		}
	}

	return nil
}

// Test configurations

func testAccRadosgwIAMAccessKeyConfig_basic(userID string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_user" "test" {
  user_id      = %q
  display_name = "Test User for Access Key"
}

resource "radosgw_iam_access_key" "test" {
  user_id = radosgw_iam_user.test.user_id
}
`, userID)
}

func testAccRadosgwIAMAccessKeyConfig_withKeys(userID, accessKey, secretKey string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_user" "test" {
  user_id      = %q
  display_name = "Test User for Access Key"
}

resource "radosgw_iam_access_key" "test" {
  user_id    = radosgw_iam_user.test.user_id
  access_key = %q
  secret_key = %q
}
`, userID, accessKey, secretKey)
}

func testAccRadosgwIAMAccessKeyConfig_multiple(userID string) string {
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
`, userID)
}

func TestAccRadosgwIAMAccessKey_swiftUpdateSecretKey(t *testing.T) {
	t.Parallel()

	userID := randomName("tf-acc-user")
	subuserName := "swiftuser"
	secretKey1 := "swiftsecret1_" + randomName("")
	secretKey2 := "swiftsecret2_" + randomName("")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMAccessKeyDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMAccessKeyConfig_swiftWithSecret(userID, subuserName, secretKey1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_iam_access_key.swift", "key_type", "swift"),
					resource.TestCheckResourceAttr("radosgw_iam_access_key.swift", "secret_key", secretKey1),
				),
			},
			// Update secret_key in place
			{
				Config: testAccRadosgwIAMAccessKeyConfig_swiftWithSecret(userID, subuserName, secretKey2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_iam_access_key.swift", "key_type", "swift"),
					resource.TestCheckResourceAttr("radosgw_iam_access_key.swift", "secret_key", secretKey2),
				),
			},
		},
	})
}

func testAccRadosgwIAMAccessKeyConfig_swiftWithSecret(userID, subuserName, secretKey string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_user" "test" {
  user_id      = %q
  display_name = "Test User for Swift Key"
}

resource "radosgw_iam_subuser" "test" {
  user_id = radosgw_iam_user.test.user_id
  subuser = %q
  access  = "full-control"
}

resource "radosgw_iam_access_key" "swift" {
  user_id    = radosgw_iam_user.test.user_id
  subuser    = radosgw_iam_subuser.test.subuser
  key_type   = "swift"
  secret_key = %q
}
`, userID, subuserName, secretKey)
}

func testAccRadosgwIAMAccessKeyConfig_swift(userID, subuserName string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_user" "test" {
  user_id      = %q
  display_name = "Test User for Swift Key"
}

resource "radosgw_iam_subuser" "test" {
  user_id = radosgw_iam_user.test.user_id
  subuser = %q
  access  = "full-control"
}

resource "radosgw_iam_access_key" "swift" {
  user_id  = radosgw_iam_user.test.user_id
  subuser  = radosgw_iam_subuser.test.subuser
  key_type = "swift"
}
`, userID, subuserName)
}
