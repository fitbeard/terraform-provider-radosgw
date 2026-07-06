package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccRadosgwIAMAccountDataSource_basic(t *testing.T) {
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
				Config: testAccRadosgwIAMAccountDataSourceConfig(accountID, accountName, email),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.radosgw_iam_account.test", "account_id", accountID),
					resource.TestCheckResourceAttr("data.radosgw_iam_account.test", "name", accountName),
					resource.TestCheckResourceAttr("data.radosgw_iam_account.test", "email", email),
					resource.TestCheckResourceAttrSet("data.radosgw_iam_account.test", "max_buckets"),
				),
			},
		},
	})
}

func testAccRadosgwIAMAccountDataSourceConfig(accountID, name, email string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_account" "test" {
  account_id = %[1]q
  name       = %[2]q
  email      = %[3]q
}

data "radosgw_iam_account" "test" {
  account_id = radosgw_iam_account.test.account_id
}
`, accountID, name, email)
}
