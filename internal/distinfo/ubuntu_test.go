package distinfo

import (
	"errors"
	"slices"
	"testing"
	"time"
)

// newTestRegistry builds a registry the same way loadDistributions does,
// without reading /usr/share/distro-info.
func newTestRegistry(dists ...Distribution) *UbuntuRegistry {
	r := &UbuntuRegistry{versionToDistIdx: map[string]int{}}
	for i, d := range dists {
		r.distributions = append(r.distributions, d)
		r.releaseCodes = append(r.releaseCodes, d.Series)
		r.versionToDistIdx[d.Version] = i
	}
	return r
}

func testDists() []Distribution {
	return []Distribution{
		{Version: "22.04", Series: "jammy", IsLTS: true, EOLLegacyDate: time.Unix(1, 0)},
		{Version: "22.10", Series: "kinetic"},
		{Version: "24.04", Series: "noble", IsLTS: true, EOLLegacyDate: time.Unix(2, 0)},
	}
}

func TestListAllReleaseCodes(t *testing.T) {
	r := newTestRegistry(testDists()...)
	want := []string{"jammy", "kinetic", "noble"}
	if got := r.ListAllReleaseCodes(); !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestListLTSReleaseCodes(t *testing.T) {
	r := newTestRegistry(testDists()...)
	want := []string{"jammy", "noble"}
	if got := r.ListLTSReleaseCodes(); !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestIsValidReleaseCode(t *testing.T) {
	r := newTestRegistry(testDists()...)
	if !r.IsValidReleaseCode("noble") {
		t.Error("noble should be valid")
	}
	if r.IsValidReleaseCode("nope") {
		t.Error("nope should be invalid")
	}
}

func TestFindDistributionByVersion(t *testing.T) {
	r := newTestRegistry(testDists()...)

	got, ok := r.FindDistributionByVersion("24.04")
	if !ok || got.Series != "noble" {
		t.Errorf("got (%v, %v), want noble", got.Series, ok)
	}
	if _, ok := r.FindDistributionByVersion("1.00"); ok {
		t.Error("unknown version should not be found")
	}
}

func TestFindDistribution(t *testing.T) {
	r := newTestRegistry(testDists()...)
	got := r.FindDistribution(func(d Distribution) bool { return d.IsLTS })
	if len(got) != 2 || got[0].Series != "jammy" || got[1].Series != "noble" {
		t.Errorf("unexpected result: %+v", got)
	}
	if got := r.FindDistribution(func(Distribution) bool { return false }); got != nil {
		t.Errorf("got %+v, want nil", got)
	}
}

func TestErrorsAreDistinct(t *testing.T) {
	if errors.Is(ErrNoNextRelease, ErrNoCurrentRelease) {
		t.Error("ErrNoNextRelease and ErrNoCurrentRelease must be distinct")
	}
}
