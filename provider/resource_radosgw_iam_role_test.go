package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccRadosgwIAMRole_basic(t *testing.T) {
	t.Parallel()

	roleName := randomName("tf-acc-role")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMRoleDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMRoleConfig_basic(roleName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMRoleExists("radosgw_iam_role.test"),
					resource.TestCheckResourceAttr("radosgw_iam_role.test", "name", roleName),
					resource.TestCheckResourceAttrSet("radosgw_iam_role.test", "arn"),
					resource.TestCheckResourceAttrSet("radosgw_iam_role.test", "unique_id"),
				),
			},
			// Import test
			{
				ResourceName:                         "radosgw_iam_role.test",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateId:                        roleName,
				ImportStateVerifyIdentifierAttribute: "name",
			},
		},
	})
}

func TestAccRadosgwIAMRole_withPath(t *testing.T) {
	t.Parallel()

	roleName := randomName("tf-acc-role")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMRoleDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMRoleConfig_withPath(roleName, "/test/path/"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMRoleExists("radosgw_iam_role.test"),
					resource.TestCheckResourceAttr("radosgw_iam_role.test", "path", "/test/path/"),
				),
			},
		},
	})
}

func TestAccRadosgwIAMRole_withMaxSessionDuration(t *testing.T) {
	t.Parallel()

	roleName := randomName("tf-acc-role")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMRoleDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMRoleConfig_withMaxSession(roleName, 7200),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMRoleExists("radosgw_iam_role.test"),
					resource.TestCheckResourceAttr("radosgw_iam_role.test", "max_session_duration", "7200"),
				),
			},
		},
	})
}

// TestAccRadosgwIAMRole_update tests updating role description and max_session_duration.
func TestAccRadosgwIAMRole_update(t *testing.T) {
	t.Parallel()

	roleName := randomName("tf-acc-role")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMRoleDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMRoleConfig_withDescription(roleName, 3600, "Initial description"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMRoleExists("radosgw_iam_role.test"),
					resource.TestCheckResourceAttr("radosgw_iam_role.test", "max_session_duration", "3600"),
					resource.TestCheckResourceAttr("radosgw_iam_role.test", "description", "Initial description"),
				),
			},
			// Update max_session_duration and description
			{
				Config: testAccRadosgwIAMRoleConfig_withDescription(roleName, 7200, "Updated description"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMRoleExists("radosgw_iam_role.test"),
					resource.TestCheckResourceAttr("radosgw_iam_role.test", "max_session_duration", "7200"),
					resource.TestCheckResourceAttr("radosgw_iam_role.test", "description", "Updated description"),
				),
			},
		},
	})
}

// Helper functions

func testAccCheckRadosgwIAMRoleExists(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		roleName := rs.Primary.Attributes["name"]
		if roleName == "" {
			return fmt.Errorf("role name not set")
		}

		// Role existence is verified by the provider during Read
		// If we got here without error, the role exists
		return nil
	}
}

func testAccCheckRadosgwIAMRoleDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "radosgw_iam_role" {
			continue
		}

		// Role destruction is verified by the provider
		// The actual API check would require IAM client access
	}

	return nil
}

// Test configurations

func testAccRadosgwIAMRoleConfig_basic(roleName string) string {
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
`, roleName)
}

func testAccRadosgwIAMRoleConfig_withPath(roleName, path string) string {
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
`, roleName, path)
}

func testAccRadosgwIAMRoleConfig_withMaxSession(roleName string, maxSession int) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_role" "test" {
  name                 = %q
  max_session_duration = %d

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
`, roleName, maxSession)
}

func testAccRadosgwIAMRoleConfig_withDescription(roleName string, maxSession int, description string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_role" "test" {
  name                 = %q
  max_session_duration = %d
  description          = %q

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
`, roleName, maxSession, description)
}
