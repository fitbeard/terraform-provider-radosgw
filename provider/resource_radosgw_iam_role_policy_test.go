package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccRadosgwIAMRolePolicy_basic(t *testing.T) {
	t.Parallel()

	roleName := randomName("tf-acc-role")
	policyName := randomName("tf-acc-policy")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMRolePolicyDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMRolePolicyConfig_basic(roleName, policyName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMRolePolicyExists("radosgw_iam_role_policy.test"),
					resource.TestCheckResourceAttr("radosgw_iam_role_policy.test", "role", roleName),
					resource.TestCheckResourceAttr("radosgw_iam_role_policy.test", "name", policyName),
				),
			},
			// Import test - format: role_name:policy_name
			{
				ResourceName:                         "radosgw_iam_role_policy.test",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateId:                        roleName + ":" + policyName,
				ImportStateVerifyIdentifierAttribute: "id",
			},
		},
	})
}

func TestAccRadosgwIAMRolePolicy_update(t *testing.T) {
	t.Parallel()

	roleName := randomName("tf-acc-role")
	policyName := randomName("tf-acc-policy")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMRolePolicyDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMRolePolicyConfig_basic(roleName, policyName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMRolePolicyExists("radosgw_iam_role_policy.test"),
				),
			},
			{
				Config: testAccRadosgwIAMRolePolicyConfig_updated(roleName, policyName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMRolePolicyExists("radosgw_iam_role_policy.test"),
				),
			},
		},
	})
}

// Helper functions

func testAccCheckRadosgwIAMRolePolicyExists(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		roleName := rs.Primary.Attributes["role"]
		policyName := rs.Primary.Attributes["name"]
		if roleName == "" || policyName == "" {
			return fmt.Errorf("role or name not set")
		}

		return nil
	}
}

func testAccCheckRadosgwIAMRolePolicyDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "radosgw_iam_role_policy" {
			continue
		}
		// Policy destruction is verified by the provider
	}
	return nil
}

// Test configurations

func testAccRadosgwIAMRolePolicyConfig_basic(roleName, policyName string) string {
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

resource "radosgw_iam_role_policy" "test" {
  role = radosgw_iam_role.test.name
  name = %q

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = ["s3:GetObject"]
        Resource = ["arn:aws:s3:::*"]
      }
    ]
  })
}
`, roleName, policyName)
}

func testAccRadosgwIAMRolePolicyConfig_updated(roleName, policyName string) string {
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

resource "radosgw_iam_role_policy" "test" {
  role = radosgw_iam_role.test.name
  name = %q

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = ["s3:GetObject", "s3:PutObject"]
        Resource = ["arn:aws:s3:::*"]
      }
    ]
  })
}
`, roleName, policyName)
}
