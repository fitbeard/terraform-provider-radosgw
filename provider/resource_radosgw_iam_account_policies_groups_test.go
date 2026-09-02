package provider

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccRadosgwIAMAccountPoliciesAndGroups exercises the full account-IAM policy
// and group surface as an account root: inline + managed policies for users and
// groups, group membership, a role managed-policy attachment, and the group data
// source.
func TestAccRadosgwIAMAccountPoliciesAndGroups(t *testing.T) {
	t.Parallel()

	ar := newTestAccountRoot()
	user := strings.ReplaceAll(randomName("u"), "-", "")
	group := strings.ReplaceAll(randomName("g"), "-", "")
	role := strings.ReplaceAll(randomName("r"), "-", "")
	roReadOnly := "arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess"
	fullAccess := "arn:aws:iam::aws:policy/AmazonS3FullAccess"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSkipForVersion(t, CephVersion_Squid) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             ar.destroy,
		Steps: []resource.TestStep{
			{
				PreConfig: func() { ar.create(t) },
				Config:    ar.rootProviderConfig() + testAccIAMPoliciesGroupsConfig(user, group, role),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_iam_account_user_policy.inline", "id", user+":s3-read"),
					resource.TestCheckResourceAttr("radosgw_iam_account_user_policy_attachment.managed", "policy_arn", roReadOnly),
					resource.TestCheckResourceAttrSet("radosgw_iam_account_group.g", "arn"),
					resource.TestCheckResourceAttrSet("radosgw_iam_account_group.g", "unique_id"),
					resource.TestCheckResourceAttr("radosgw_iam_account_group_membership.m", "users.#", "1"),
					resource.TestCheckResourceAttr("radosgw_iam_account_group_policy.inline", "id", group+":s3-list"),
					resource.TestCheckResourceAttr("radosgw_iam_account_group_policy_attachment.managed", "policy_arn", fullAccess),
					resource.TestCheckResourceAttr("radosgw_iam_role_policy_attachment.managed", "policy_arn", roReadOnly),
					resource.TestCheckResourceAttr("data.radosgw_iam_account_group.g", "users.#", "1"),
					resource.TestCheckResourceAttrPair("data.radosgw_iam_account_group.g", "users.0", "radosgw_iam_account_user.u", "name"),
				),
			},
			{
				ResourceName:      "radosgw_iam_account_user_policy.inline",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				ResourceName:                         "radosgw_iam_account_group.g",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateId:                        group,
				ImportStateVerifyIdentifierAttribute: "name",
			},
			{
				ResourceName:      "radosgw_iam_account_group_policy_attachment.managed",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccIAMPoliciesGroupsConfig(user, group, role string) string {
	return fmt.Sprintf(`
data "radosgw_iam_policy_document" "s3" {
  provider = radosgw.root
  statement {
    effect    = "Allow"
    actions   = ["s3:ListBucket", "s3:GetObject"]
    resources = ["*"]
  }
}

resource "radosgw_iam_account_user" "u" {
  provider      = radosgw.root
  name          = %[1]q
  force_destroy = true
}

resource "radosgw_iam_account_user_policy" "inline" {
  provider = radosgw.root
  user     = radosgw_iam_account_user.u.name
  name     = "s3-read"
  policy   = data.radosgw_iam_policy_document.s3.json
}

resource "radosgw_iam_account_user_policy_attachment" "managed" {
  provider   = radosgw.root
  user       = radosgw_iam_account_user.u.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess"
}

resource "radosgw_iam_account_group" "g" {
  provider = radosgw.root
  name     = %[2]q
  path     = "/eng/"
}

resource "radosgw_iam_account_group_membership" "m" {
  provider = radosgw.root
  group    = radosgw_iam_account_group.g.name
  users    = [radosgw_iam_account_user.u.name]
}

resource "radosgw_iam_account_group_policy" "inline" {
  provider = radosgw.root
  group    = radosgw_iam_account_group.g.name
  name     = "s3-list"
  policy   = data.radosgw_iam_policy_document.s3.json
}

resource "radosgw_iam_account_group_policy_attachment" "managed" {
  provider   = radosgw.root
  group      = radosgw_iam_account_group.g.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonS3FullAccess"
}

resource "radosgw_iam_role" "r" {
  provider           = radosgw.root
  name               = %[3]q
  assume_role_policy = jsonencode({ Version = "2012-10-17", Statement = [{ Effect = "Allow", Principal = { AWS = "*" }, Action = "sts:AssumeRole" }] })
}

resource "radosgw_iam_role_policy_attachment" "managed" {
  provider   = radosgw.root
  role       = radosgw_iam_role.r.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess"
}

data "radosgw_iam_account_group" "g" {
  provider   = radosgw.root
  name       = radosgw_iam_account_group.g.name
  depends_on = [radosgw_iam_account_group_membership.m]
}
`, user, group, role)
}
