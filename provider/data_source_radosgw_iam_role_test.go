package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccRadosgwIAMRoleDataSource_basic(t *testing.T) {
	t.Parallel()

	roleName := randomName("tf-acc-role")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMRoleDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMRoleDataSourceConfig_basic(roleName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.radosgw_iam_role.test", "name", "radosgw_iam_role.test", "name"),
					resource.TestCheckResourceAttrPair("data.radosgw_iam_role.test", "arn", "radosgw_iam_role.test", "arn"),
					resource.TestCheckResourceAttrSet("data.radosgw_iam_role.test", "assume_role_policy"),
				),
			},
		},
	})
}

func TestAccRadosgwIAMRoleDataSource_withPath(t *testing.T) {
	t.Parallel()

	roleName := randomName("tf-acc-role")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMRoleDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMRoleDataSourceConfig_withPath(roleName, "/test/"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.radosgw_iam_role.test", "path", "/test/"),
				),
			},
		},
	})
}

// Test configurations

func testAccRadosgwIAMRoleDataSourceConfig_basic(roleName string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_role" "test" {
  name = %q

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Principal = {
          AWS = "*"
        }
        Action = "sts:AssumeRole"
      }
    ]
  })
}

data "radosgw_iam_role" "test" {
  name = radosgw_iam_role.test.name

  depends_on = [radosgw_iam_role.test]
}
`, roleName)
}

func testAccRadosgwIAMRoleDataSourceConfig_withPath(roleName, path string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_role" "test" {
  name = %q
  path = %q

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Principal = {
          AWS = "*"
        }
        Action = "sts:AssumeRole"
      }
    ]
  })
}

data "radosgw_iam_role" "test" {
  name = radosgw_iam_role.test.name

  depends_on = [radosgw_iam_role.test]
}
`, roleName, path)
}
