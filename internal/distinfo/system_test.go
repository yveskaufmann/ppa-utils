package distinfo

import (
	"errors"
	"os"
	"testing"
)

// These tests read the standard system files and skip when the host does not
// provide them, so they only run meaningfully on Ubuntu/Debian machines.

func requireFile(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Skipf("%s not available: %v", path, err)
	}
}

func TestGetCurrentDistributionInfo(t *testing.T) {
	requireFile(t, "/etc/os-release")

	info, err := GetCurrentDistributionInfo()
	if err != nil {
		t.Fatalf("GetCurrentDistributionInfo() error = %v", err)
	}
	if info.Name == "" {
		t.Error("Name (ID) should not be empty")
	}
}

func TestNewUbuntuRegistry(t *testing.T) {
	requireFile(t, distroInfoPath)

	r, err := NewUbuntuRegistry()
	if err != nil {
		t.Fatalf("NewUbuntuRegistry() error = %v", err)
	}
	if len(r.ListAllReleaseCodes()) == 0 {
		t.Fatal("registry should contain release codes")
	}
	for _, code := range []string{"focal", "jammy", "noble"} {
		if !r.IsValidReleaseCode(code) {
			t.Errorf("%s should be a valid release code", code)
		}
	}
	if d, ok := r.FindDistributionByVersion("24.04"); !ok || d.Series != "noble" || !d.IsLTS {
		t.Errorf("24.04 = (%+v, %v), want LTS noble", d, ok)
	}
}

func TestFindCurrentRelease(t *testing.T) {
	requireFile(t, distroInfoPath)
	requireFile(t, "/etc/os-release")

	info, err := GetCurrentDistributionInfo()
	if err != nil || info.Name != "ubuntu" {
		t.Skip("host is not Ubuntu")
	}
	r, err := NewUbuntuRegistry()
	if err != nil {
		t.Fatalf("NewUbuntuRegistry() error = %v", err)
	}

	dist, err := r.FindCurrentRelease()
	if errors.Is(err, ErrNoCurrentRelease) {
		t.Skipf("host version %s not in distro-info data", info.Version)
	}
	if err != nil {
		t.Fatalf("FindCurrentRelease() error = %v", err)
	}
	if dist.Series != info.Codename {
		t.Errorf("Series = %q, want %q", dist.Series, info.Codename)
	}
}
