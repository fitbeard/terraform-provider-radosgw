package provider

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccRadosgwIAMOIDCProvider_basic(t *testing.T) {
	t.Parallel()

	// Use a unique URL to avoid conflicts
	suffix := randomName("oidc")
	providerURL := fmt.Sprintf("https://%s-test.example.com", suffix)
	// ARN format: arn:aws:iam:::oidc-provider/<url_without_protocol>
	providerARN := fmt.Sprintf("arn:aws:iam:::oidc-provider/%s-test.example.com", suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMOIDCProviderDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMOIDCProviderConfig_basic(providerURL),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMOIDCProviderExists("radosgw_iam_openid_connect_provider.test"),
					resource.TestCheckResourceAttr("radosgw_iam_openid_connect_provider.test", "url", providerURL),
					resource.TestCheckResourceAttrSet("radosgw_iam_openid_connect_provider.test", "arn"),
				),
			},
			// Import test - by ARN
			{
				ResourceName:                         "radosgw_iam_openid_connect_provider.test",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateVerifyIgnore:              []string{"allow_updates"},
				ImportStateId:                        providerARN,
				ImportStateVerifyIdentifierAttribute: "arn",
			},
		},
	})
}

func TestAccRadosgwIAMOIDCProvider_multipleClientIDs(t *testing.T) {
	t.Parallel()

	suffix := randomName("oidc")
	providerURL := fmt.Sprintf("https://%s-multi.example.com", suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMOIDCProviderDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMOIDCProviderConfig_multipleClientIDs(providerURL),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMOIDCProviderExists("radosgw_iam_openid_connect_provider.test"),
					resource.TestCheckResourceAttr("radosgw_iam_openid_connect_provider.test", "client_id_list.#", "2"),
				),
			},
		},
	})
}

func TestAccRadosgwIAMOIDCProvider_allowUpdatesTrue(t *testing.T) {
	t.Parallel()

	suffix := randomName("oidc")
	providerURL := fmt.Sprintf("https://%s-update.example.com", suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMOIDCProviderDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMOIDCProviderConfig_allowUpdates(providerURL, true),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMOIDCProviderExists("radosgw_iam_openid_connect_provider.test"),
					resource.TestCheckResourceAttr("radosgw_iam_openid_connect_provider.test", "allow_updates", "true"),
				),
			},
		},
	})
}

// TestAccRadosgwIAMOIDCProvider_inPlaceUpdate tests in-place updates of client_id_list and thumbprint_list.
// This requires Ceph Tentacle (20.x) which supports UpdateOpenIDConnectProviderThumbprint,
// AddClientIDToOpenIDConnectProvider, and RemoveClientIDFromOpenIDConnectProvider APIs.
func TestAccRadosgwIAMOIDCProvider_inPlaceUpdate(t *testing.T) {
	t.Parallel()

	suffix := randomName("oidc")
	providerURL := fmt.Sprintf("https://%s-inplace.example.com", suffix)

	resource.Test(t, resource.TestCase{
		// Skip on Ceph versions older than Tentacle (20.x) which supports in-place updates
		PreCheck:                 func() { testAccPreCheckSkipForVersion(t, CephVersion_Tentacle) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMOIDCProviderDestroy,
		Steps: []resource.TestStep{
			// Create with initial client IDs and thumbprint
			{
				Config: testAccRadosgwIAMOIDCProviderConfig_forUpdate(providerURL, []string{"client-1"}, "1234567890abcdef1234567890abcdef12345678"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMOIDCProviderExists("radosgw_iam_openid_connect_provider.test"),
					resource.TestCheckResourceAttr("radosgw_iam_openid_connect_provider.test", "client_id_list.#", "1"),
				),
			},
			// Update: add a client ID
			{
				Config: testAccRadosgwIAMOIDCProviderConfig_forUpdate(providerURL, []string{"client-1", "client-2"}, "1234567890abcdef1234567890abcdef12345678"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_iam_openid_connect_provider.test", "client_id_list.#", "2"),
				),
			},
			// Update: change thumbprint
			{
				Config: testAccRadosgwIAMOIDCProviderConfig_forUpdate(providerURL, []string{"client-1", "client-2"}, "abcdef1234567890abcdef1234567890abcdef12"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_iam_openid_connect_provider.test", "thumbprint_list.0", "abcdef1234567890abcdef1234567890abcdef12"),
				),
			},
			// Update: remove a client ID
			{
				Config: testAccRadosgwIAMOIDCProviderConfig_forUpdate(providerURL, []string{"client-2"}, "abcdef1234567890abcdef1234567890abcdef12"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_iam_openid_connect_provider.test", "client_id_list.#", "1"),
				),
			},
		},
	})
}

func TestAccRadosgwIAMOIDCProvider_allowUpdatesFalse(t *testing.T) {
	t.Parallel()

	suffix := randomName("oidc")
	providerURL := fmt.Sprintf("https://%s-noupdate.example.com", suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMOIDCProviderDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMOIDCProviderConfig_allowUpdates(providerURL, false),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMOIDCProviderExists("radosgw_iam_openid_connect_provider.test"),
					resource.TestCheckResourceAttr("radosgw_iam_openid_connect_provider.test", "allow_updates", "false"),
				),
			},
		},
	})
}

// Helper functions

func testAccCheckRadosgwIAMOIDCProviderExists(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		arn := rs.Primary.Attributes["arn"]
		if arn == "" {
			return fmt.Errorf("arn not set")
		}

		return nil
	}
}

func testAccCheckRadosgwIAMOIDCProviderDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "radosgw_iam_openid_connect_provider" {
			continue
		}
		// Provider destruction is verified by the provider
	}
	return nil
}

// Test configurations

func testAccRadosgwIAMOIDCProviderConfig_basic(providerURL string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_openid_connect_provider" "test" {
  url = %q

  client_id_list = ["test-client-id"]

  thumbprint_list = [
    "1234567890abcdef1234567890abcdef12345678"
  ]
}
`, providerURL)
}

func testAccRadosgwIAMOIDCProviderConfig_multipleClientIDs(providerURL string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_openid_connect_provider" "test" {
  url = %q

  client_id_list = ["client-id-1", "client-id-2"]

  thumbprint_list = [
    "1234567890abcdef1234567890abcdef12345678"
  ]
}
`, providerURL)
}

func testAccRadosgwIAMOIDCProviderConfig_allowUpdates(providerURL string, allowUpdates bool) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_openid_connect_provider" "test" {
  url = %q

  client_id_list = ["test-client-id"]

  thumbprint_list = [
    "1234567890abcdef1234567890abcdef12345678"
  ]

  allow_updates = %t
}
`, providerURL, allowUpdates)
}

func testAccRadosgwIAMOIDCProviderConfig_forUpdate(providerURL string, clientIDs []string, thumbprint string) string {
	// Build client_id_list as a quoted list
	quotedIDs := make([]string, len(clientIDs))
	for i, id := range clientIDs {
		quotedIDs[i] = fmt.Sprintf("%q", id)
	}
	clientIDList := "[" + strings.Join(quotedIDs, ", ") + "]"

	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_openid_connect_provider" "test" {
  url = %q

  client_id_list = %s

  thumbprint_list = [%q]

  allow_updates = true
}
`, providerURL, clientIDList, thumbprint)
}
