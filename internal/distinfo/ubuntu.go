package distinfo

import (
	"encoding/csv"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"
)

type UbuntuRegistry struct {
	distributions    []Distribution
	releaseCodes     []string
	versionToDistIdx map[string]int
}

const distroInfoPath = "/usr/share/distro-info/ubuntu.csv"

func NewUbuntuRegistry() (*UbuntuRegistry, error) {

	registry := &UbuntuRegistry{
		distributions:    []Distribution{},
		releaseCodes:     []string{},
		versionToDistIdx: make(map[string]int),
	}
	if err := registry.loadDistributions(); err != nil {
		return nil, err
	}
	return registry, nil
}

func (r *UbuntuRegistry) loadDistributions() error {

	file, err := os.Open(distroInfoPath)
	if err != nil {
		return err
	}

	defer file.Close()

	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = false
	reader.FieldsPerRecord = -1

	rows, err := reader.ReadAll()
	if err != nil {
		return err
	}

	head := rows[0]
	rows = rows[1:]

	// Process each row here
	for rowIdx, row := range rows {
		dist := Distribution{}
		for colIdx, field := range row {

			filedName := head[colIdx]

			switch filedName {
			case "version":
				dist.Version = field

				if strings.HasSuffix(dist.Version, " LTS") {
					dist.Version = strings.TrimSuffix(dist.Version, " LTS")
					dist.IsLTS = true
				}

			case "codename":
				dist.Codename = field
			case "series":
				dist.Series = field
				r.releaseCodes = append(r.releaseCodes, dist.Series)
				r.versionToDistIdx[dist.Version] = rowIdx
			case "created":
				dist.CreatedAt, _ = time.Parse("2006-01-02", field)
			case "release":
				dist.ReleasedAt, _ = time.Parse("2006-01-02", field)
			case "eol":
				dist.EOLAt, _ = time.Parse("2006-01-02", field)
			case "eol-esm":
				dist.EOLEsmDate, _ = time.Parse("2006-01-02", field)
			case "eol-legacy":
				dist.EOLLegacyDate, _ = time.Parse("2006-01-02", field)
			}

		}
		r.distributions = append(r.distributions, dist)

	}

	return nil
}

func (r *UbuntuRegistry) ListAllReleaseCodes() []string {
	return r.releaseCodes
}

func (r *UbuntuRegistry) ListLTSReleaseCodes() []string {
	releaseCode := []string{}

	for _, dist := range r.distributions {
		if !dist.EOLLegacyDate.IsZero() {
			releaseCode = append(releaseCode, dist.Series)
		}
	}
	return releaseCode
}

func (r *UbuntuRegistry) IsValidReleaseCode(code string) bool {
	return slices.Contains(r.releaseCodes, code)
}

func (r *UbuntuRegistry) FindDistributionByVersion(version string) (Distribution, bool) {
	distIdx, ok := r.versionToDistIdx[version]

	if ok {
		return r.distributions[distIdx], true
	}

	return Distribution{}, false
}

func (r *UbuntuRegistry) FindDistributionByCode(code string) (Distribution, bool) {
	for _, dist := range r.distributions {
		if dist.Series == code {
			return dist, true
		}
	}

	return Distribution{}, false
}

func (r *UbuntuRegistry) FindDistribution(filter func(Distribution) bool) []Distribution {
	var result []Distribution
	for _, dist := range r.distributions {
		if filter(dist) {
			result = append(result, dist)
		}
	}
	return result
}

var (
	ErrNoNextRelease    = fmt.Errorf("no next release found")
	ErrNoCurrentRelease = fmt.Errorf("no current release found")
)

func (r *UbuntuRegistry) FindCurrentRelease() (Distribution, error) {
	info, err := GetCurrentDistributionInfo()
	if err != nil {
		return Distribution{}, fmt.Errorf("failed to get current distribution info: %w: %w", ErrNoCurrentRelease, err)
	}
	dist, ok := r.FindDistributionByVersion(info.Version)
	if !ok {
		return Distribution{}, ErrNoCurrentRelease
	}
	return dist, nil
}

func (r *UbuntuRegistry) FindNextRelease() (Distribution, error) {
	info, err := GetCurrentDistributionInfo()
	if err != nil {
		return Distribution{}, fmt.Errorf("failed to get current distribution info: %w: %w", ErrNoCurrentRelease, err)
	}

	distIdx, ok := r.versionToDistIdx[info.Version]
	if !ok {
		return Distribution{}, fmt.Errorf("no next release found for current version: %s: %w", info.Version, ErrNoCurrentRelease)
	}

	nextDistIdx := distIdx + 1
	if nextDistIdx < len(r.distributions) {
		return r.distributions[nextDistIdx], nil
	}

	return r.distributions[distIdx], ErrNoNextRelease
}
