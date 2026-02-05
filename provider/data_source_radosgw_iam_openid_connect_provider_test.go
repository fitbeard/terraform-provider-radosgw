package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccRadosgwIAMOIDCProviderDataSource_basic(t *testing.T) {
	t.Parallel()

	suffix := randomName("oidc")
	providerURL := fmt.Sprintf("https://%s-ds.example.com", suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMOIDCProviderDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMOIDCProviderDataSourceConfig_basic(providerURL),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.radosgw_iam_openid_connect_provider.test", "url", "radosgw_iam_openid_connect_provider.test", "url"),
					resource.TestCheckResourceAttrPair("data.radosgw_iam_openid_connect_provider.test", "arn", "radosgw_iam_openid_connect_provider.test", "arn"),
					resource.TestCheckResourceAttrSet("data.radosgw_iam_openid_connect_provider.test", "client_id_list.#"),
					resource.TestCheckResourceAttrSet("data.radosgw_iam_openid_connect_provider.test", "thumbprint_list.#"),
				),
			},
		},
	})
}

func TestAccRadosgwIAMOIDCProviderDataSource_byURL(t *testing.T) {
	t.Parallel()

	suffix := randomName("oidc")
	providerURL := fmt.Sprintf("https://%s-url.example.com", suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMOIDCProviderDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMOIDCProviderDataSourceConfig_byURL(providerURL),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.radosgw_iam_openid_connect_provider.test", "url", "radosgw_iam_openid_connect_provider.test", "url"),
					resource.TestCheckResourceAttrPair("data.radosgw_iam_openid_connect_provider.test", "arn", "radosgw_iam_openid_connect_provider.test", "arn"),
					resource.TestCheckResourceAttrSet("data.radosgw_iam_openid_connect_provider.test", "client_id_list.#"),
					resource.TestCheckResourceAttrSet("data.radosgw_iam_openid_connect_provider.test", "thumbprint_list.#"),
				),
			},
		},
	})
}

// Test configurations

func testAccRadosgwIAMOIDCProviderDataSourceConfig_basic(providerURL string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_openid_connect_provider" "test" {
  url = %q

  client_id_list = ["test-client-id"]

  thumbprint_list = [
    "1234567890abcdef1234567890abcdef12345678"
  ]
}

data "radosgw_iam_openid_connect_provider" "test" {
  arn = radosgw_iam_openid_connect_provider.test.arn

  depends_on = [radosgw_iam_openid_connect_provider.test]
}
`, providerURL)
}

func testAccRadosgwIAMOIDCProviderDataSourceConfig_byURL(providerURL string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_openid_connect_provider" "test" {
  url = %q

  client_id_list = ["test-client-id"]

  thumbprint_list = [
    "1234567890abcdef1234567890abcdef12345678"
  ]
}

data "radosgw_iam_openid_connect_provider" "test" {
  url = radosgw_iam_openid_connect_provider.test.url

  depends_on = [radosgw_iam_openid_connect_provider.test]
}
`, providerURL)
}
