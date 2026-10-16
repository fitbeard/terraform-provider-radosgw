package provider

import (
	"context"
	"crypto/tls"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAccSTSAssumeRoleWithWebIdentity exercises the full OIDC -> STS WebIdentity
// rail end to end against the local OIDC provider (scripts/compose.yml): the
// provider registers an OIDC provider and a role whose trust policy federates it,
// then the test mints a JWT from the IdP and calls AssumeRoleWithWebIdentity,
// asserting temporary credentials are returned for the federated subject.
//
// Requires the OIDC provider fixture to be running (`make oidc-up`); the test is
// skipped when RADOSGW_OIDC_ISSUER is unset. Runs on all Ceph releases: it only
// creates the OIDC provider + role (no import/update), so the in-place-update
// limitation that gates the OIDC provider resource test to Tentacle does not
// apply. The provider is created with allow_updates = false as a safety belt so
// no in-place update API is ever reached on Reef/Squid.
func TestAccSTSAssumeRoleWithWebIdentity(t *testing.T) {
	t.Parallel()

	issuer := os.Getenv("RADOSGW_OIDC_ISSUER")
	if issuer == "" {
		t.Skip("RADOSGW_OIDC_ISSUER not set; start the OIDC provider with `make oidc-up`")
	}
	// Host without scheme, used in the role trust policy condition key.
	host := strings.TrimPrefix(strings.TrimPrefix(issuer, "https://"), "http://")
	roleName := randomName("tf-acc-webid")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSTSWebIdentityConfig(issuer, host, roleName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("radosgw_iam_openid_connect_provider.idp", "arn"),
					resource.TestCheckResourceAttrSet("radosgw_iam_role.webid", "arn"),
					testAccCheckAssumeRoleWithWebIdentity("radosgw_iam_role.webid", issuer),
				),
			},
		},
	})
}

func testAccSTSWebIdentityConfig(issuer, host, roleName string) string {
	return providerConfig() + fmt.Sprintf(`
resource "radosgw_iam_openid_connect_provider" "idp" {
  url            = %[1]q
  client_id_list = ["test-client"]

  # Force replace-on-change instead of in-place updates, which are unsupported on
  # Reef/Squid. Not exercised here (single create), but keeps the test safe on all
  # releases.
  allow_updates = false

  # RadosGW only verifies the thumbprint against the IdP certificate when the
  # JWKS exposes x5c certificate chains; this provider's JWKS has none, so a
  # placeholder satisfies the IAM API (the JWT is still validated via the JWKS
  # public key). When the JWKS includes x5c, supply real SHA-1 thumbprints of
  # those certs.
  thumbprint_list = ["0000000000000000000000000000000000000000"]
}

resource "radosgw_iam_role" "webid" {
  name = %[2]q
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Federated = radosgw_iam_openid_connect_provider.idp.arn }
      Action    = "sts:AssumeRoleWithWebIdentity"
      Condition = { StringEquals = { "%[3]s:app_id" = "test-client" } }
    }]
  })
}
`, issuer, roleName, host)
}

// testAccCheckAssumeRoleWithWebIdentity mints a JWT from the OIDC provider and
// calls STS AssumeRoleWithWebIdentity (unsigned — the token is the credential)
// against RadosGW, asserting temporary credentials come back for the subject.
func testAccCheckAssumeRoleWithWebIdentity(resourceName, issuer string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}
		roleARN := rs.Primary.Attributes["arn"]
		if roleARN == "" {
			return fmt.Errorf("role %s has no arn in state", resourceName)
		}

		token, err := mintOIDCToken(issuer)
		if err != nil {
			return err
		}

		form := url.Values{}
		form.Set("Action", "AssumeRoleWithWebIdentity")
		form.Set("RoleArn", roleARN)
		form.Set("RoleSessionName", "tf-acc-webid")
		form.Set("WebIdentityToken", token)
		form.Set("Version", "2011-06-15")

		// AssumeRoleWithWebIdentity is an anonymous STS action — the JWT is the
		// credential, so the request is NOT SigV4-signed (in dev or production).
		// Over https the only difference is TLS transport, honored here via the
		// same RADOSGW_TLS_INSECURE_SKIP_VERIFY knob the provider itself uses.
		endpoint := strings.TrimRight(os.Getenv("RADOSGW_ENDPOINT"), "/") + "/"
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, endpoint, strings.NewReader(form.Encode()))
		if err != nil {
			return fmt.Errorf("building AssumeRoleWithWebIdentity request: %w", err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := testAccSTSHTTPClient().Do(req)
		if err != nil {
			return fmt.Errorf("AssumeRoleWithWebIdentity request failed: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("AssumeRoleWithWebIdentity returned HTTP %d: %s", resp.StatusCode, string(body))
		}

		var out struct {
			AccessKeyID string `xml:"AssumeRoleWithWebIdentityResult>Credentials>AccessKeyId"`
			Subject     string `xml:"AssumeRoleWithWebIdentityResult>SubjectFromWebIdentityToken"`
			Audience    string `xml:"AssumeRoleWithWebIdentityResult>Audience"`
		}
		if err := xml.Unmarshal(body, &out); err != nil {
			return fmt.Errorf("could not parse STS response: %w (body: %s)", err, string(body))
		}
		if out.AccessKeyID == "" {
			return fmt.Errorf("STS returned no temporary credentials (body: %s)", string(body))
		}
		if out.Subject != "radosgw-user" {
			return fmt.Errorf("unexpected federated subject: got %q, want %q", out.Subject, "radosgw-user")
		}
		if out.Audience != "test-client" {
			return fmt.Errorf("unexpected audience: got %q, want %q", out.Audience, "test-client")
		}
		return nil
	}
}

// testAccSTSHTTPClient returns an HTTP client for the (unsigned) STS call that
// honors RADOSGW_TLS_INSECURE_SKIP_VERIFY, matching the provider's own TLS
// handling so the test works against an https RadosGW with a self-signed cert.
func testAccSTSHTTPClient() *http.Client {
	tr := &http.Transport{}
	if os.Getenv("RADOSGW_TLS_INSECURE_SKIP_VERIFY") == "true" {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // opt-in via env, test against self-signed dev certs
	}
	return &http.Client{Timeout: 30 * time.Second, Transport: tr}
}

// mintOIDCToken runs scripts/jwt-token.sh to obtain an id_token (JWT) from the
// local OIDC provider for the given issuer.
func mintOIDCToken(issuer string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "../scripts/jwt-token.sh")
	cmd.Env = append(os.Environ(), "OIDC_ISSUER="+issuer)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("jwt-token.sh failed: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}
	token := strings.TrimSpace(string(out))
	if strings.Count(token, ".") != 2 {
		return "", fmt.Errorf("jwt-token.sh did not return a JWT: %q", token)
	}
	return token, nil
}

// TestAccSTSAssumeRoleWithWebIdentityProvider exercises the provider's
// assume_role_with_web_identity block: an OIDC provider and a federated role are
// registered out-of-band, a JWT is minted to a file, and a provider configured
// with web identity (no static credentials) creates an S3 bucket.
//
// Not parallel: it shares the single OIDC provider registration (one per issuer
// URL) with TestAccSTSAssumeRoleWithWebIdentity; running in the sequential phase
// avoids a registration collision.
func TestAccSTSAssumeRoleWithWebIdentityProvider(t *testing.T) {
	issuer := os.Getenv("RADOSGW_OIDC_ISSUER")
	if issuer == "" {
		t.Skip("RADOSGW_OIDC_ISSUER not set; start the OIDC provider with `make oidc-up`")
	}
	testAccPreCheck(t)

	host := strings.TrimPrefix(strings.TrimPrefix(issuer, "https://"), "http://")
	roleName := randomName("tfaccwebidprov")
	roleARN := "arn:aws:iam:::role/" + roleName
	bucketName := randomName("tf-acc-wi-bucket")

	iam := testAccAdminIAMClient()
	if err := testAccRegisterOIDCProvider(iam, issuer); err != nil {
		t.Fatalf("register OIDC provider: %s", err)
	}
	t.Cleanup(func() { _ = testAccDeleteOIDCProvider(iam, host) })

	trust := fmt.Sprintf(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Federated":"arn:aws:iam:::oidc-provider/%s"},"Action":"sts:AssumeRoleWithWebIdentity","Condition":{"StringEquals":{"%s:app_id":"test-client"}}}]}`, host, host)
	if err := testAccCreateRoleFixture(iam, roleName, trust); err != nil {
		t.Fatalf("create federated role: %s", err)
	}
	t.Cleanup(func() { _ = testAccDeleteRoleFixture(iam, roleName) })

	token, err := mintOIDCToken(issuer)
	if err != nil {
		t.Fatalf("mint token: %s", err)
	}
	tokenFile := filepath.Join(t.TempDir(), "token.jwt")
	if err := os.WriteFile(tokenFile, []byte(token), 0o600); err != nil {
		t.Fatalf("write token file: %s", err)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwS3BucketDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccWebIdentityProviderConfig(roleARN, tokenFile, bucketName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_s3_bucket.test", "bucket", bucketName),
					resource.TestCheckResourceAttrSet("radosgw_s3_bucket.test", "owner"),
					// Admin-only attributes are null (Admin API has no session token).
					resource.TestCheckNoResourceAttr("radosgw_s3_bucket.test", "num_shards"),
				),
			},
		},
	})
}

func testAccWebIdentityProviderConfig(roleARN, tokenFile, bucketName string) string {
	// Web identity is anonymous — no access_key/secret_key.
	return fmt.Sprintf(`
provider "radosgw" {
  endpoint = %[1]q
  assume_role_with_web_identity {
    role_arn                = %[2]q
    session_name            = "tf-acc"
    web_identity_token_file = %[3]q
  }
}

resource "radosgw_s3_bucket" "test" {
  bucket        = %[4]q
  force_destroy = true
}
`, os.Getenv("RADOSGW_ENDPOINT"), roleARN, tokenFile, bucketName)
}

func testAccRegisterOIDCProvider(iam *IAMClient, issuer string) error {
	ctx := context.Background()
	p := url.Values{}
	p.Set("Action", "CreateOpenIDConnectProvider")
	p.Set("Version", "2010-05-08")
	p.Set("Url", issuer)
	p.Set("ClientIDList.member.1", "test-client")
	p.Set("ThumbprintList.member.1", "0000000000000000000000000000000000000000")
	if _, err := iam.DoPostRequest(ctx, p, "iam"); err != nil {
		// Tolerate a leftover provider from a previous run.
		if strings.Contains(err.Error(), "EntityAlreadyExists") {
			return nil
		}
		return err
	}
	return nil
}

func testAccDeleteOIDCProvider(iam *IAMClient, host string) error {
	ctx := context.Background()
	p := url.Values{}
	p.Set("Action", "DeleteOpenIDConnectProvider")
	p.Set("Version", "2010-05-08")
	p.Set("OpenIDConnectProviderArn", "arn:aws:iam:::oidc-provider/"+host)
	_, err := iam.DoPostRequest(ctx, p, "iam")
	return err
}

// TestAccSTSAssumeRoleWithWebIdentityEnv verifies web identity can be configured
// purely from environment variables (no assume_role_with_web_identity block, and
// with static RADOSGW_ACCESS_KEY/SECRET_KEY present but ignored). Not parallel: it
// uses t.Setenv and shares the single OIDC provider registration.
func TestAccSTSAssumeRoleWithWebIdentityEnv(t *testing.T) {
	issuer := os.Getenv("RADOSGW_OIDC_ISSUER")
	if issuer == "" {
		t.Skip("RADOSGW_OIDC_ISSUER not set; start the OIDC provider with `make oidc-up`")
	}
	testAccPreCheck(t)

	host := strings.TrimPrefix(strings.TrimPrefix(issuer, "https://"), "http://")
	roleName := randomName("tfaccwebidenv")
	roleARN := "arn:aws:iam:::role/" + roleName
	bucketName := randomName("tf-acc-wi-bucket")

	iam := testAccAdminIAMClient()
	if err := testAccRegisterOIDCProvider(iam, issuer); err != nil {
		t.Fatalf("register OIDC provider: %s", err)
	}
	t.Cleanup(func() { _ = testAccDeleteOIDCProvider(iam, host) })

	trust := fmt.Sprintf(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Federated":"arn:aws:iam:::oidc-provider/%s"},"Action":"sts:AssumeRoleWithWebIdentity","Condition":{"StringEquals":{"%s:app_id":"test-client"}}}]}`, host, host)
	if err := testAccCreateRoleFixture(iam, roleName, trust); err != nil {
		t.Fatalf("create federated role: %s", err)
	}
	t.Cleanup(func() { _ = testAccDeleteRoleFixture(iam, roleName) })

	token, err := mintOIDCToken(issuer)
	if err != nil {
		t.Fatalf("mint token: %s", err)
	}
	tokenFile := filepath.Join(t.TempDir(), "token.jwt")
	if err := os.WriteFile(tokenFile, []byte(token), 0o600); err != nil {
		t.Fatalf("write token file: %s", err)
	}

	// Configure web identity entirely via environment variables. The static
	// RADOSGW_ACCESS_KEY/SECRET_KEY (set by the test harness) must be ignored.
	t.Setenv("RADOSGW_ROLE_ARN", roleARN)
	t.Setenv("RADOSGW_WEB_IDENTITY_TOKEN_FILE", tokenFile)
	t.Setenv("RADOSGW_ROLE_SESSION_NAME", "tf-acc-env")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRadosgwS3BucketDestroy,
		Steps: []resource.TestStep{
			{
				// Empty provider block — web identity comes from the environment.
				Config: providerConfig() + fmt.Sprintf(`
resource "radosgw_s3_bucket" "test" {
  bucket        = %q
  force_destroy = true
}
`, bucketName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("radosgw_s3_bucket.test", "bucket", bucketName),
					resource.TestCheckResourceAttrSet("radosgw_s3_bucket.test", "owner"),
					resource.TestCheckNoResourceAttr("radosgw_s3_bucket.test", "num_shards"),
				),
			},
		},
	})
}
