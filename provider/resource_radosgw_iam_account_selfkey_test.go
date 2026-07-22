package provider

import (
	"encoding/xml"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAccRadosgwIAMAccount_memberSelfCreateAccessKey documents and locks the
// account permission model: a non-root account member can create its own access
// key only when an IAM policy granting iam:CreateAccessKey is attached. Without
// the policy the self-service call is denied.
//
// There is no provider resource for account-internal IAM users, policies, or
// keys, so the member (and its policy/keys) are managed out-of-band via the IAM
// API as the account root. The provider is exercised for the account lifecycle:
// it creates the account, the root user, and the root's access key, and the
// account-root credentials drive the in-account IAM operations.
func TestAccRadosgwIAMAccount_memberSelfCreateAccessKey(t *testing.T) {
	// Not Parallel: the check performs out-of-band IAM mutations and must clean
	// up the member before Terraform destroys the enclosing account.
	accountID := "RGW" + randomAccountID()
	accountName := randomName("account")
	rootUser := randomName("selfkeyroot")
	member := randomName("selfkeymember")
	// Explicit, collision-free root credentials so the account-root IAM client
	// can authenticate as the user the provider creates.
	rootAK := strings.ReplaceAll(randomName("selfkeyrootak"), "-", "")
	rootSK := strings.ReplaceAll(randomName("selfkeyrootsk"), "-", "")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckSkipForVersion(t, CephVersion_Squid) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwIAMAccountDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRadosgwIAMAccountConfig_rootWithKey(accountID, accountName, rootUser, rootAK, rootSK),
				Check:  testAccCheckMemberSelfCreateAccessKey(rootAK, rootSK, member),
			},
		},
	})
}

func testAccRadosgwIAMAccountConfig_rootWithKey(accountID, name, rootUser, accessKey, secretKey string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_account" "test" {
  account_id = %[1]q
  name       = %[2]q
}

resource "radosgw_iam_user" "root" {
  user_id      = %[3]q
  display_name = "SelfKeyRoot"
  account_id   = radosgw_iam_account.test.account_id
  account_root = true
}

resource "radosgw_iam_access_key" "root" {
  user_id    = radosgw_iam_user.root.user_id
  access_key = %[4]q
  secret_key = %[5]q
}
`, accountID, name, rootUser, accessKey, secretKey)
}

// testAccCheckMemberSelfCreateAccessKey exercises the self-service key flow as
// the account root and member users over the IAM API.
func testAccCheckMemberSelfCreateAccessKey(rootAK, rootSK, member string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		endpoint := os.Getenv("RADOSGW_ENDPOINT")
		root := NewIAMClient(endpoint, rootAK, rootSK, nil)

		// 1. Root creates the member user.
		if _, err := root.DoRequest(testCtx, iamParams("CreateUser", map[string]string{"UserName": member}), "iam"); err != nil {
			return fmt.Errorf("root CreateUser(%s): %w", member, err)
		}
		// Always remove the out-of-band member so the account can be destroyed.
		defer testAccCleanupAccountMember(root, member)

		// 2. Root issues the member's initial key so it can authenticate.
		memberAK, memberSK, err := iamCreateAccessKey(root, member)
		if err != nil {
			return fmt.Errorf("root CreateAccessKey(%s): %w", member, err)
		}
		memberClient := NewIAMClient(endpoint, memberAK, memberSK, nil)

		// 3. NEGATIVE control: without a policy, self-service key creation is denied.
		_, err = memberClient.DoRequest(testCtx, iamParams("CreateAccessKey", map[string]string{"UserName": member}), "iam")
		if err == nil {
			return fmt.Errorf("expected member self CreateAccessKey to be denied without a policy, but it succeeded")
		}
		if !errors.Is(err, ErrAccessDenied) {
			return fmt.Errorf("expected AccessDenied for self CreateAccessKey without policy, got: %w", err)
		}

		// 4. Root attaches an inline policy granting iam:CreateAccessKey.
		policyDoc := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"iam:CreateAccessKey","Resource":"*"}]}`
		if _, err := root.DoRequest(testCtx, iamParams("PutUserPolicy", map[string]string{
			"UserName":       member,
			"PolicyName":     "allow-self-createkey",
			"PolicyDocument": policyDoc,
		}), "iam"); err != nil {
			return fmt.Errorf("root PutUserPolicy(%s): %w", member, err)
		}

		// 5. POSITIVE: with the policy attached, the member can self-create a key.
		newAK, _, err := iamCreateAccessKey(memberClient, member)
		if err != nil {
			return fmt.Errorf("member self CreateAccessKey with iam:CreateAccessKey policy: %w", err)
		}
		if newAK == "" {
			return fmt.Errorf("member self CreateAccessKey returned an empty AccessKeyId")
		}

		return nil
	}
}

func iamParams(action string, extra map[string]string) url.Values {
	v := url.Values{}
	v.Set("Action", action)
	for k, val := range extra {
		v.Set(k, val)
	}
	return v
}

// iamCreateAccessKey calls CreateAccessKey and returns the new key pair.
func iamCreateAccessKey(c *IAMClient, userName string) (accessKey, secretKey string, err error) {
	body, err := c.DoRequest(testCtx, iamParams("CreateAccessKey", map[string]string{"UserName": userName}), "iam")
	if err != nil {
		return "", "", err
	}
	var parsed struct {
		XMLName xml.Name `xml:"CreateAccessKeyResponse"`
		Key     struct {
			AccessKeyID     string `xml:"AccessKeyId"`
			SecretAccessKey string `xml:"SecretAccessKey"`
		} `xml:"CreateAccessKeyResult>AccessKey"`
	}
	if err := xml.Unmarshal(body, &parsed); err != nil {
		return "", "", fmt.Errorf("parsing CreateAccessKey response: %w", err)
	}
	return parsed.Key.AccessKeyID, parsed.Key.SecretAccessKey, nil
}

// testAccCleanupAccountMember removes an out-of-band member user (its inline
// policy and all access keys first) so Terraform can destroy the account.
func testAccCleanupAccountMember(root *IAMClient, member string) {
	_, _ = root.DoRequest(testCtx, iamParams("DeleteUserPolicy", map[string]string{
		"UserName": member, "PolicyName": "allow-self-createkey",
	}), "iam")

	if body, err := root.DoRequest(testCtx, iamParams("ListAccessKeys", map[string]string{"UserName": member}), "iam"); err == nil {
		var parsed struct {
			Keys []string `xml:"ListAccessKeysResult>AccessKeyMetadata>member>AccessKeyId"`
		}
		if xml.Unmarshal(body, &parsed) == nil {
			for _, k := range parsed.Keys {
				_, _ = root.DoRequest(testCtx, iamParams("DeleteAccessKey", map[string]string{
					"UserName": member, "AccessKeyId": k,
				}), "iam")
			}
		}
	}

	_, _ = root.DoRequest(testCtx, iamParams("DeleteUser", map[string]string{"UserName": member}), "iam")
}
