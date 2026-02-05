package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccRadosgwIAMRolesDataSource_basic(t *testing.T) {
	t.Parallel()

	roleName := randomName("tf-acc-role")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMRoleDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMRolesDataSourceConfig_basic(roleName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.radosgw_iam_roles.test", "names.#"),
				),
			},
		},
	})
}

func TestAccRadosgwIAMRolesDataSource_withPathPrefix(t *testing.T) {
	t.Parallel()

	roleName := randomName("tf-acc-role")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMRoleDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMRolesDataSourceConfig_withPathPrefix(roleName, "/test/"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.radosgw_iam_roles.test", "path_prefix", "/test/"),
				),
			},
		},
	})
}

// Test configurations

func testAccRadosgwIAMRolesDataSourceConfig_basic(roleName string) string {
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

data "radosgw_iam_roles" "test" {
  depends_on = [radosgw_iam_role.test]
}
`, roleName)
}

func testAccRadosgwIAMRolesDataSourceConfig_withPathPrefix(roleName, pathPrefix string) string {
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

data "radosgw_iam_roles" "test" {
  path_prefix = %q

  depends_on = [radosgw_iam_role.test]
}
`, roleName, pathPrefix, pathPrefix)
}
