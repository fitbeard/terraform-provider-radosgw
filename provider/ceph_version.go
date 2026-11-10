package provider

import (
	"fmt"
	"strconv"
	"strings"
)

// CephVersion represents a major Ceph release.
type CephVersion int

// Ceph major releases (named versions).
const (
	CephVersion_Unknown  CephVersion = 0
	CephVersion_Squid    CephVersion = 19 // Squid (19.x)
	CephVersion_Tentacle CephVersion = 20 // Tentacle (20.x)
	CephVersion_Umbrella CephVersion = 21 // Umbrella (21.x)

	// cephVersionNewest is the newest release the provider knows about. It is the
	// assumed version when the provider's ceph_version is not configured.
	cephVersionNewest = CephVersion_Umbrella
)

// String returns the version name.
func (v CephVersion) String() string {
	switch v {
	case CephVersion_Squid:
		return "Squid (19.x)"
	case CephVersion_Tentacle:
		return "Tentacle (20.x)"
	case CephVersion_Umbrella:
		return "Umbrella (21.x)"
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

// parseCephVersion parses a version string into a CephVersion.
// Accepts: "21", "21.2.0", "umbrella", "Umbrella", "21-umbrella".
func parseCephVersion(version string) CephVersion {
	version = strings.ToLower(strings.TrimSpace(version))

	// Check for named versions first
	switch version {
	case "squid":
		return CephVersion_Squid
	case "tentacle":
		return CephVersion_Tentacle
	case "umbrella":
		return CephVersion_Umbrella
	}

	// Handle versions like "21.2.0" or "21-umbrella" - extract major version
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
