package provider

import "testing"

func TestCephVersionString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   CephVersion
		want string
	}{
		{
			name: "reef",
			in:   CephVersion_Reef,
			want: "Reef (18.x)",
		},
		{
			name: "squid",
			in:   CephVersion_Squid,
			want: "Squid (19.x)",
		},
		{
			name: "tentacle",
			in:   CephVersion_Tentacle,
			want: "Tentacle (20.x)",
		},
		{
			name: "unknown",
			in:   CephVersion(99),
			want: "Unknown (99.x)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.in.String(); got != tt.want {
				t.Fatalf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCephVersionComparisons(t *testing.T) {
	t.Parallel()

	if !CephVersion_Reef.LessThan(CephVersion_Squid) {
		t.Fatal("expected Reef to be less than Squid")
	}
	if !CephVersion_Tentacle.GreaterThan(CephVersion_Squid) {
		t.Fatal("expected Tentacle to be greater than Squid")
	}
	if !CephVersion_Squid.GreaterThanOrEqual(CephVersion_Squid) {
		t.Fatal("expected Squid to be greater than or equal to Squid")
	}
	if !CephVersion_Squid.LessThanOrEqual(CephVersion_Tentacle) {
		t.Fatal("expected Squid to be less than or equal to Tentacle")
	}
}

func TestParseCephVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want CephVersion
	}{
		{
			name: "named version",
			in:   "Tentacle",
			want: CephVersion_Tentacle,
		},
		{
			name: "major version",
			in:   "19",
			want: CephVersion_Squid,
		},
		{
			name: "dotted version",
			in:   "20.1.0",
			want: CephVersion_Tentacle,
		},
		{
			name: "hyphenated version",
			in:   "18-reef",
			want: CephVersion_Reef,
		},
		{
			name: "unknown version",
			in:   "dev",
			want: CephVersion_Unknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := parseCephVersion(tt.in); got != tt.want {
				t.Fatalf("parseCephVersion(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestGetCephVersion(t *testing.T) {
	t.Setenv("CEPH_VERSION", "")

	if got := getCephVersion(); got != CephVersion(99) {
		t.Fatalf("getCephVersion() with empty env = %v, want 99", got)
	}

	t.Setenv("CEPH_VERSION", "squid")
	if got := getCephVersion(); got != CephVersion_Squid {
		t.Fatalf("getCephVersion() with squid env = %v, want %v", got, CephVersion_Squid)
	}
}
