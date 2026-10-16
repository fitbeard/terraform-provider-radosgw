package provider

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAccSTSAssumeRole configures the provider with an assume_role block using a
// non-account role ARN (arn:aws:iam:::role/<name>) and verifies it can create an
// S3 bucket with the temporary (assumed-role) credentials. The role is created
// out-of-band in PreConfig because the provider assumes it at Configure time,
// before any resource in the config is created.
//
// Runs on all Ceph releases. Admin-only attributes come back null under an
// assumed role (the Admin Ops API can't carry the STS session token).
func TestAccSTSAssumeRole(t *testing.T) {
	t.Parallel()

	roleName := randomName("tfaccassume")
	roleARN := "arn:aws:iam:::role/" + roleName
	bucketName := randomName("tf-acc-ar-bucket")
	trust := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":"arn:aws:iam:::user/admin"},"Action":"sts:AssumeRole"}]}`

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			_ = testAccDeleteRoleFixture(testAccAdminIAMClient(), roleName)
			return testAccCheckRadosgwS3BucketDestroy(s)
		},
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					if err := testAccCreateRoleFixture(testAccAdminIAMClient(), roleName, trust); err != nil {
						t.Fatalf("failed to create assume-role fixture: %s", err)
					}
				},
				Config: testAccSTSAssumeRoleConfig("", roleARN, bucketName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_s3_bucket.test", "bucket", bucketName),
					resource.TestCheckResourceAttrSet("radosgw_s3_bucket.test", "owner"),
					resource.TestCheckNoResourceAttr("radosgw_s3_bucket.test", "num_shards"),
				),
			},
		},
	})
}

// TestAccSTSAssumeRoleAccountARN uses an account-scoped role ARN
// (arn:aws:iam::<account-id>:role/<name>). The provider authenticates as the
// account root and assumes a role created under that account.
func TestAccSTSAssumeRoleAccountARN(t *testing.T) {
	t.Parallel()

	ar := newTestAccountRoot()
	roleName := randomName("tfaccassume")
	roleARN := fmt.Sprintf("arn:aws:iam::%s:role/%s", ar.accountID, roleName)
	bucketName := randomName("tf-acc-ar-bucket")
	trust := fmt.Sprintf(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":"arn:aws:iam::%s:root"},"Action":"sts:AssumeRole"}]}`, ar.accountID)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			_ = testAccDeleteRoleFixture(testAccIAMClientWithCreds(ar.accessKey, ar.secretKey), roleName)
			_ = ar.destroy(s)
			return testAccCheckRadosgwS3BucketDestroy(s)
		},
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					ar.create(t)
					if err := testAccCreateRoleFixture(testAccIAMClientWithCreds(ar.accessKey, ar.secretKey), roleName, trust); err != nil {
						t.Fatalf("failed to create account role fixture: %s", err)
					}
				},
				// Provider authenticates as the account root (base creds) and assumes
				// the account-scoped role.
				Config: testAccSTSAssumeRoleConfigWithCreds(ar.accessKey, ar.secretKey, roleARN, bucketName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_s3_bucket.test", "bucket", bucketName),
					resource.TestCheckResourceAttrSet("radosgw_s3_bucket.test", "owner"),
				),
			},
		},
	})
}

// TestAccSTSAssumeRoleParams verifies the supported assume_role parameters at the
// STS level: session_name (reflected in the assumed-role ARN), duration (honored
// in the credential expiration), and policy (an inline session policy that
// further restricts the session). external_id is intentionally not tested — a
// role with an sts:ExternalId trust condition cannot be assumed on RadosGW.
func TestAccSTSAssumeRoleParams(t *testing.T) {
	t.Parallel()

	roleName := randomName("tfaccparams")
	roleARN := "arn:aws:iam:::role/" + roleName
	trust := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":"arn:aws:iam:::user/admin"},"Action":"sts:AssumeRole"}]}`

	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC not set")
	}
	testAccPreCheck(t)

	iam := testAccAdminIAMClient()
	if err := testAccCreateRoleFixture(iam, roleName, trust); err != nil {
		t.Fatalf("create role: %s", err)
	}
	defer func() { _ = testAccDeleteRoleFixture(iam, roleName) }()
	ctx := context.Background()

	// session_name + duration
	creds, err := assumeRole(ctx, iam, assumeRoleParams{RoleARN: roleARN, RoleSessionName: "customsession", DurationSeconds: 900})
	if err != nil {
		t.Fatalf("assumeRole: %s", err)
	}
	if !strings.HasSuffix(creds.AssumedRoleARN, "/customsession") {
		t.Errorf("session_name not reflected: assumed-role ARN = %q, want suffix %q", creds.AssumedRoleARN, "/customsession")
	}
	exp, perr := time.Parse(time.RFC3339, creds.Expiration)
	if perr != nil {
		t.Fatalf("parse expiration %q: %s", creds.Expiration, perr)
	}
	if ttl := time.Until(exp); ttl < 600*time.Second || ttl > 1000*time.Second {
		t.Errorf("duration not honored: ttl = %s, want ~900s", ttl)
	}

	// policy: an inline deny-all session policy must restrict the session.
	denyAll := `{"Version":"2012-10-17","Statement":[{"Effect":"Deny","Action":"s3:*","Resource":"*"}]}`
	restricted, err := assumeRole(ctx, iam, assumeRoleParams{RoleARN: roleARN, RoleSessionName: "restricted", Policy: denyAll})
	if err != nil {
		t.Fatalf("assumeRole with policy: %s", err)
	}
	s3c := testAccS3ClientWithToken(restricted.AccessKeyID, restricted.SecretAccessKey, restricted.SessionToken)
	if _, err := s3c.ListBuckets(ctx, &s3.ListBucketsInput{}); err == nil {
		t.Error("session policy not enforced: ListBuckets succeeded under a deny-all session policy")
	}
}

// --- config helpers ---

func testAccSTSAssumeRoleConfig(_ string, roleARN, bucketName string) string {
	// Base credentials come from RADOSGW_* env vars; the assume_role block drives
	// the STS exchange.
	return fmt.Sprintf(`
provider "radosgw" {
  assume_role {
    role_arn     = %[1]q
    session_name = "tf-acc"
  }
}

resource "radosgw_s3_bucket" "test" {
  bucket        = %[2]q
  force_destroy = true
}
`, roleARN, bucketName)
}

func testAccSTSAssumeRoleConfigWithCreds(accessKey, secretKey, roleARN, bucketName string) string {
	return fmt.Sprintf(`
provider "radosgw" {
  endpoint   = %[1]q
  access_key = %[2]q
  secret_key = %[3]q
  assume_role {
    role_arn     = %[4]q
    session_name = "tf-acc"
  }
}

resource "radosgw_s3_bucket" "test" {
  bucket        = %[5]q
  force_destroy = true
}
`, os.Getenv("RADOSGW_ENDPOINT"), accessKey, secretKey, roleARN, bucketName)
}

// --- IAM fixtures (roles), created over the IAM API ---

func testAccAdminIAMClient() *IAMClient {
	return testAccIAMClientWithCreds(testAccAdminClient.AccessKey, testAccAdminClient.SecretKey)
}

func testAccIAMClientWithCreds(accessKey, secretKey string) *IAMClient {
	return NewIAMClient(testAccAdminClient.Endpoint, accessKey, secretKey, "", testAccAdminClient.HTTPClient)
}

func testAccS3ClientWithToken(accessKey, secretKey, sessionToken string) *s3.Client {
	endpoint := os.Getenv("RADOSGW_ENDPOINT")
	return s3.NewFromConfig(aws.Config{
		Region:      "default",
		Credentials: credentials.NewStaticCredentialsProvider(accessKey, secretKey, sessionToken),
	}, func(o *s3.Options) {
		o.BaseEndpoint = &endpoint
		o.UsePathStyle = true
	})
}

func testAccCreateRoleFixture(iam *IAMClient, roleName, trustPolicy string) error {
	ctx := context.Background()
	perm := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"s3:*","Resource":"*"}]}`

	cr := url.Values{}
	cr.Set("Action", "CreateRole")
	cr.Set("Version", "2010-05-08")
	cr.Set("RoleName", roleName)
	cr.Set("AssumeRolePolicyDocument", trustPolicy)
	if _, err := iam.DoPostRequest(ctx, cr, "iam"); err != nil {
		return fmt.Errorf("CreateRole: %w", err)
	}

	pp := url.Values{}
	pp.Set("Action", "PutRolePolicy")
	pp.Set("Version", "2010-05-08")
	pp.Set("RoleName", roleName)
	pp.Set("PolicyName", "s3all")
	pp.Set("PolicyDocument", perm)
	if _, err := iam.DoPostRequest(ctx, pp, "iam"); err != nil {
		return fmt.Errorf("PutRolePolicy: %w", err)
	}
	return nil
}

func testAccDeleteRoleFixture(iam *IAMClient, roleName string) error {
	ctx := context.Background()

	dp := url.Values{}
	dp.Set("Action", "DeleteRolePolicy")
	dp.Set("Version", "2010-05-08")
	dp.Set("RoleName", roleName)
	dp.Set("PolicyName", "s3all")
	_, _ = iam.DoPostRequest(ctx, dp, "iam")

	dr := url.Values{}
	dr.Set("Action", "DeleteRole")
	dr.Set("Version", "2010-05-08")
	dr.Set("RoleName", roleName)
	_, err := iam.DoPostRequest(ctx, dr, "iam")
	return err
}
