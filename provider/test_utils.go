package provider

import (
	"fmt"
	"os"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
)

// getCephVersion returns the Ceph major version from the CEPH_VERSION environment
// variable, used to gate acceptance tests. Accepts formats like "21", "21.2.0",
// "umbrella", "Umbrella". Falls back to a high version if not set (to run all
// tests by default).
func getCephVersion() CephVersion {
	versionStr := os.Getenv("CEPH_VERSION")
	if versionStr == "" {
		// Default to a high version to run all tests if not specified
		return CephVersion(99)
	}

	return parseCephVersion(versionStr)
}

// randomName generates a random name with the given prefix for test resources.
func randomName(prefix string) string {
	return acctest.RandomWithPrefix(prefix)
}

// randomEmail generates a random email address for test resources.
func randomEmail() string {
	return fmt.Sprintf("%s@example.com", acctest.RandString(10))
}

// randomAccountID generates the numeric portion of a RadosGW account ID.
// Account IDs have the form "RGW" followed by 17 numeric digits, so callers
// typically prefix the result with "RGW".
func randomAccountID() string {
	return acctest.RandStringFromCharSet(17, "0123456789")
}

// providerConfig returns the provider configuration block for tests.
func providerConfig() string {
	return `
provider "radosgw" {
  # Configuration is read from environment variables:
  # RADOSGW_ENDPOINT, RADOSGW_ACCESS_KEY, RADOSGW_SECRET_KEY
}
`
}
