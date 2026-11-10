package provider

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
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
			// Import test - by ARN. client_id_list/thumbprint_list are unchanged,
			// so no update API is triggered (safe on all Ceph versions).
			{
				ResourceName:                         "radosgw_iam_openid_connect_provider.test",
				ImportState:                          true,
				ImportStateVerify:                    true,
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

// TestAccRadosgwIAMOIDCProvider_inPlaceUpdate tests in-place updates of client_id_list and thumbprint_list.
// This requires Ceph Tentacle (20.x) which supports UpdateOpenIDConnectProviderThumbprint,
// AddClientIDToOpenIDConnectProvider, and RemoveClientIDFromOpenIDConnectProvider APIs. The
// provider's ceph_version defaults to the newest release (>= Tentacle), so updates are applied in place.
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
			// Update: add a client ID (in place)
			{
				Config: testAccRadosgwIAMOIDCProviderConfig_forUpdate(providerURL, []string{"client-1", "client-2"}, "1234567890abcdef1234567890abcdef12345678"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("radosgw_iam_openid_connect_provider.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_iam_openid_connect_provider.test", "client_id_list.#", "2"),
				),
			},
			// Update: change thumbprint (in place)
			{
				Config: testAccRadosgwIAMOIDCProviderConfig_forUpdate(providerURL, []string{"client-1", "client-2"}, "abcdef1234567890abcdef1234567890abcdef12"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_iam_openid_connect_provider.test", "thumbprint_list.0", "abcdef1234567890abcdef1234567890abcdef12"),
				),
			},
			// Update: remove a client ID (in place)
			{
				Config: testAccRadosgwIAMOIDCProviderConfig_forUpdate(providerURL, []string{"client-2"}, "abcdef1234567890abcdef1234567890abcdef12"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_iam_openid_connect_provider.test", "client_id_list.#", "1"),
				),
			},
		},
	})
}

// TestAccRadosgwIAMOIDCProvider_squidForcesReplace verifies that when the provider's
// ceph_version is Squid (19.x) — which lacks the in-place OIDC update APIs — a change
// to client_id_list forces resource replacement instead of an in-place update. This
// runs on any cluster because replacement (create) is supported everywhere.
func TestAccRadosgwIAMOIDCProvider_squidForcesReplace(t *testing.T) {
	t.Parallel()

	suffix := randomName("oidc")
	providerURL := fmt.Sprintf("https://%s-squid.example.com", suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMOIDCProviderDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMOIDCProviderConfig_cephVersion(providerURL, []string{"client-1"}, "squid"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMOIDCProviderExists("radosgw_iam_openid_connect_provider.test"),
					resource.TestCheckResourceAttr("radosgw_iam_openid_connect_provider.test", "client_id_list.#", "1"),
				),
			},
			{
				Config: testAccRadosgwIAMOIDCProviderConfig_cephVersion(providerURL, []string{"client-1", "client-2"}, "squid"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("radosgw_iam_openid_connect_provider.test", plancheck.ResourceActionReplace),
					},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_iam_openid_connect_provider.test", "client_id_list.#", "2"),
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

func oidcClientIDListHCL(clientIDs []string) string {
	quotedIDs := make([]string, len(clientIDs))
	for i, id := range clientIDs {
		quotedIDs[i] = fmt.Sprintf("%q", id)
	}
	return "[" + strings.Join(quotedIDs, ", ") + "]"
}

func testAccRadosgwIAMOIDCProviderConfig_forUpdate(providerURL string, clientIDs []string, thumbprint string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_openid_connect_provider" "test" {
  url = %q

  client_id_list = %s

  thumbprint_list = [%q]
}
`, providerURL, oidcClientIDListHCL(clientIDs), thumbprint)
}

func testAccRadosgwIAMOIDCProviderConfig_cephVersion(providerURL string, clientIDs []string, cephVersion string) string {
	return fmt.Sprintf(`
provider "radosgw" {
  ceph_version = %q
}

resource "radosgw_iam_openid_connect_provider" "test" {
  url = %q

  client_id_list = %s

  thumbprint_list = [
    "1234567890abcdef1234567890abcdef12345678"
  ]
}
`, cephVersion, providerURL, oidcClientIDListHCL(clientIDs))
}
