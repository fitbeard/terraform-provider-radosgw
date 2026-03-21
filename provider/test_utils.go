package provider

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
)

// CephVersion represents a major Ceph release.
type CephVersion int

// Ceph major releases (named versions).
const (
	CephVersion_Unknown  CephVersion = 0
	CephVersion_Reef     CephVersion = 18 // Reef (18.x)
	CephVersion_Squid    CephVersion = 19 // Squid (19.x)
	CephVersion_Tentacle CephVersion = 20 // Tentacle (20.x)
)

// String returns the version name.
func (v CephVersion) String() string {
	switch v {
	case CephVersion_Reef:
		return "Reef (18.x)"
	case CephVersion_Squid:
		return "Squid (19.x)"
	case CephVersion_Tentacle:
		return "Tentacle (20.x)"
	default:
		return fmt.Sprintf("Unknown (%d.x)", int(v))
	}
}

// LessThan returns true if v is older than other.
func (v CephVersion) LessThan(other CephVersion) bool {
	return int(v) < int(other)
}

// GreaterThan returns true if v is newer than other.
func (v CephVersion) GreaterThan(other CephVersion) bool {
	return int(v) > int(other)
}

// GreaterThanOrEqual returns true if v is the same or newer than other.
func (v CephVersion) GreaterThanOrEqual(other CephVersion) bool {
	return int(v) >= int(other)
}

// LessThanOrEqual returns true if v is the same or older than other.
func (v CephVersion) LessThanOrEqual(other CephVersion) bool {
	return int(v) <= int(other)
}

// getCephVersion returns the Ceph major version from CEPH_VERSION environment variable.
// Accepts formats like "20", "20.1.0", "tentacle", "Tentacle".
// Falls back to a high version if not set (to run all tests by default).
func getCephVersion() CephVersion {
	versionStr := os.Getenv("CEPH_VERSION")
	if versionStr == "" {
		// Default to a high version to run all tests if not specified
		return CephVersion(99)
	}

	return parseCephVersion(versionStr)
}

// parseCephVersion parses a version string into a CephVersion.
// Accepts: "20", "20.1.0", "tentacle", "Tentacle", "20-tentacle"
func parseCephVersion(version string) CephVersion {
	version = strings.ToLower(strings.TrimSpace(version))

	// Check for named versions first
	switch version {
	case "reef":
		return CephVersion_Reef
	case "squid":
		return CephVersion_Squid
	case "tentacle":
		return CephVersion_Tentacle
	}

	// Handle versions like "20.1.0" or "20-tentacle" - extract major version
	parts := strings.Split(version, "-")
	version = parts[0]

	vParts := strings.Split(version, ".")
	if len(vParts) >= 1 {
		if major, err := strconv.Atoi(vParts[0]); err == nil {
			return CephVersion(major)
		}
	}

	return CephVersion_Unknown
}

// randomName generates a random name with the given prefix for test resources.
func randomName(prefix string) string {
	return acctest.RandomWithPrefix(prefix)
}

// randomEmail generates a random email address for test resources.
func randomEmail() string {
	return fmt.Sprintf("%s@example.com", acctest.RandString(10))
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
