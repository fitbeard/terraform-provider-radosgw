package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccRadosgwIAMAccountDataSource(t *testing.T) {
	t.Parallel()

	accountID := "RGW" + randomAccountID()
	accountName := randomName("account")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSkipForVersion(t, CephVersion_Squid) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMAccountDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMAccountDataSourceConfig(accountID, accountName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.radosgw_iam_account.test", "account_id", accountID),
					resource.TestCheckResourceAttr("data.radosgw_iam_account.test", "name", accountName),
					resource.TestCheckResourceAttr("data.radosgw_iam_account.test", "max_users", "-1"),
					resource.TestCheckResourceAttr("data.radosgw_iam_account.test", "max_buckets", "1000"),
				),
			},
		},
	})
}

func testAccRadosgwIAMAccountDataSourceConfig(accountID, name string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_account" "account" {
  account_id = %q
  name       = %q
}

data "radosgw_iam_account" "test" {
  account_id = radosgw_iam_account.account.account_id
}
`, accountID, name)
}
