package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccRadosgwIAMPolicyDocumentDataSource_basic(t *testing.T) {
	t.Parallel()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMPolicyDocumentDataSourceConfig_basic(),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.radosgw_iam_policy_document.test", "json"),
				),
			},
		},
	})
}

func TestAccRadosgwIAMPolicyDocumentDataSource_multipleStatements(t *testing.T) {
	t.Parallel()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMPolicyDocumentDataSourceConfig_multipleStatements(),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.radosgw_iam_policy_document.test", "json"),
				),
			},
		},
	})
}

func TestAccRadosgwIAMPolicyDocumentDataSource_withCondition(t *testing.T) {
	t.Parallel()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMPolicyDocumentDataSourceConfig_withCondition(),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.radosgw_iam_policy_document.test", "json"),
				),
			},
		},
	})
}

// Test configurations

func testAccRadosgwIAMPolicyDocumentDataSourceConfig_basic() string {
	return providerConfig() + `
data "radosgw_iam_policy_document" "test" {
  statement {
    effect = "Allow"

    actions = [
      "s3:GetObject",
    ]

    resources = [
      "arn:aws:s3:::my-bucket/*",
    ]
  }
}
`
}

func testAccRadosgwIAMPolicyDocumentDataSourceConfig_multipleStatements() string {
	return providerConfig() + `
data "radosgw_iam_policy_document" "test" {
  statement {
    sid    = "AllowGetObject"
    effect = "Allow"

    actions = [
      "s3:GetObject",
    ]

    resources = [
      "arn:aws:s3:::my-bucket/*",
    ]
  }

  statement {
    sid    = "AllowListBucket"
    effect = "Allow"

    actions = [
      "s3:ListBucket",
    ]

    resources = [
      "arn:aws:s3:::my-bucket",
    ]
  }
}
`
}

func testAccRadosgwIAMPolicyDocumentDataSourceConfig_withCondition() string {
	return providerConfig() + `
data "radosgw_iam_policy_document" "test" {
  statement {
    effect = "Allow"

    principals {
      type        = "AWS"
      identifiers = ["*"]
    }

    actions = [
      "s3:GetObject",
    ]

    resources = [
      "arn:aws:s3:::my-bucket/*",
    ]

    condition {
      test     = "IpAddress"
      variable = "aws:SourceIp"
      values   = ["192.168.1.0/24"]
    }
  }
}
`
}
