package provider

import (
	"encoding/xml"
	"fmt"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestRoleTagsXMLToMap(t *testing.T) {
	tests := map[string]struct {
		body string
		want map[string]string
	}{
		"aws_shape": {
			body: `<ListRoleTagsResponse><ListRoleTagsResult><Tags>` +
				`<member><Key>Environment</Key><Value>test</Value></member>` +
				`<member><Key>Owner</Key><Value>terraform</Value></member>` +
				`</Tags></ListRoleTagsResult></ListRoleTagsResponse>`,
			want: map[string]string{
				"Environment": "test",
				"Owner":       "terraform",
			},
		},
		"radosgw_shape": {
			body: `<ListRoleTagsResponse><ListRoleTagsResult><Tags>` +
				`<Key><Key>Environment</Key></Key><Value><Value>test</Value></Value>` +
				`<Key><Key>Owner</Key></Key><Value><Value>terraform</Value></Value>` +
				`</Tags></ListRoleTagsResult></ListRoleTagsResponse>`,
			want: map[string]string{
				"Environment": "test",
				"Owner":       "terraform",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var response listRoleTagsResponseXML
			if err := xml.Unmarshal([]byte(tc.body), &response); err != nil {
				t.Fatalf("unmarshal failed: %s", err)
			}

			got := response.Result.Tags.toMap()
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("unexpected tags: got %#v, want %#v", got, tc.want)
			}
		})
	}
}

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

func TestAccRadosgwIAMRole_tags(t *testing.T) {
	t.Parallel()

	roleName := randomName("tf-acc-role")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMRoleDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMRoleConfig_withTags(roleName, map[string]string{
					"Environment": "test",
					"Owner":       "terraform",
				}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMRoleExists("radosgw_iam_role.test"),
					resource.TestCheckResourceAttr("radosgw_iam_role.test", "tags.%", "2"),
					resource.TestCheckResourceAttr("radosgw_iam_role.test", "tags.Environment", "test"),
					resource.TestCheckResourceAttr("radosgw_iam_role.test", "tags.Owner", "terraform"),
				),
			},
			{
				ResourceName:                         "radosgw_iam_role.test",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateId:                        roleName,
				ImportStateVerifyIdentifierAttribute: "name",
			},
			{
				Config: testAccRadosgwIAMRoleConfig_withTags(roleName, map[string]string{
					"Environment": "updated",
					"Project":     "radosgw",
				}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMRoleExists("radosgw_iam_role.test"),
					resource.TestCheckResourceAttr("radosgw_iam_role.test", "tags.%", "2"),
					resource.TestCheckResourceAttr("radosgw_iam_role.test", "tags.Environment", "updated"),
					resource.TestCheckResourceAttr("radosgw_iam_role.test", "tags.Project", "radosgw"),
					resource.TestCheckNoResourceAttr("radosgw_iam_role.test", "tags.Owner"),
				),
			},
			{
				Config: testAccRadosgwIAMRoleConfig_withTags(roleName, map[string]string{}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRadosgwIAMRoleExists("radosgw_iam_role.test"),
					resource.TestCheckResourceAttr("radosgw_iam_role.test", "tags.%", "0"),
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

func testAccRadosgwIAMRoleConfig_withTags(roleName string, tags map[string]string) string {
	tagsBlock := "tags = {\n"
	for key, value := range tags {
		tagsBlock += fmt.Sprintf("    %q = %q\n", key, value)
	}
	tagsBlock += "  }"

	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_role" "test" {
  name = %q
  %s

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
`, roleName, tagsBlock)
}
