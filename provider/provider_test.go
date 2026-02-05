package provider

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/ceph/go-ceph/rgw/admin"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// testAccProtoV6ProviderFactories are used to instantiate a provider during
// acceptance testing. The factory function will be invoked for every Terraform
// CLI command executed to create a provider server to which the CLI can
// reattach.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"radosgw": providerserver.NewProtocol6WithError(New("test")()),
}

// testAccAdminClient is the RadosGW admin API client used in acceptance tests
// for direct API operations (e.g., checking if resources exist, cleanup).
var testAccAdminClient *admin.API

// testCtx is the context used for test operations.
var testCtx context.Context

func init() {
	testCtx = context.Background()
}

// TestMain sets up the test environment before running tests.
func TestMain(m *testing.M) {
	// Create admin client for test verification
	endpoint := os.Getenv("RADOSGW_ENDPOINT")
	accessKey := os.Getenv("RADOSGW_ACCESS_KEY")
	secretKey := os.Getenv("RADOSGW_SECRET_KEY")

	if endpoint != "" && accessKey != "" && secretKey != "" {
		var err error
		testAccAdminClient, err = admin.New(endpoint, accessKey, secretKey, nil)
		if err != nil {
			fmt.Printf("Warning: Failed to create admin client: %s\n", err)
		}
	}

	code := m.Run()
	os.Exit(code)
}

// TestProvider validates the provider can be properly instantiated.
func TestProvider(t *testing.T) {
	t.Parallel()

	// Simple validation that the provider can be created
	p := New("test")()
	if p == nil {
		t.Fatal("Provider returned nil")
	}
}

// testAccPreCheck validates required environment variables are set before running acceptance tests.
func testAccPreCheck(t *testing.T) {
	t.Helper()

	required := []string{
		"RADOSGW_ENDPOINT",
		"RADOSGW_ACCESS_KEY",
		"RADOSGW_SECRET_KEY",
	}

	for _, env := range required {
		if os.Getenv(env) == "" {
			t.Fatalf("Environment variable %s must be set for acceptance tests", env)
		}
	}

	// Verify we can connect to RadosGW
	if testAccAdminClient == nil {
		t.Fatal("Admin client not initialized - check RADOSGW_* environment variables")
	}
}

// testAccPreCheckSkipForVersion skips the test if the Ceph version doesn't meet requirements.
// Use this for features that were added in specific Ceph versions.
func testAccPreCheckSkipForVersion(t *testing.T, minVersion CephVersion) {
	t.Helper()
	testAccPreCheck(t)

	version := getCephVersion()
	if version.LessThan(minVersion) {
		t.Skipf("Skipping test: requires Ceph version %s or higher, got %s", minVersion, version)
	}
}

// testAccPreCheckSkipAfterVersion skips the test if the Ceph version is greater than specified.
// Use this for features that were deprecated or changed in newer versions.
//
//nolint:unused // Keep for future version-specific tests
func testAccPreCheckSkipAfterVersion(t *testing.T, maxVersion CephVersion) {
	t.Helper()
	testAccPreCheck(t)

	version := getCephVersion()
	if version.GreaterThan(maxVersion) {
		t.Skipf("Skipping test: test only valid for Ceph version %s or lower, got %s", maxVersion, version)
	}
}
