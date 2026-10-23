// Package sourcelist turns apt source files into rows for display and filtering.
package sourcelist

import (
	"path/filepath"
	"strings"

	"yv35.com/ppa-utils/internal/ppa"
)

// InlineKey is shown instead of the signing key when it is embedded in the source file.
const InlineKey = "inline-key"

// Status values of a row.
const (
	StatusEnabled  = "enabled"
	StatusDisabled = "disabled"
)

// Row is one entry of a source file.
type Row struct {
	Name          string   `json:"name"`
	Path          string   `json:"path"`
	URIs          []string `json:"uris"`
	Suites        []string `json:"suites"`
	Components    []string `json:"components"`
	Architectures []string `json:"architectures"`
	// SignedBy is the path of the keyring, or "inline-key" when the key is embedded.
	SignedBy string `json:"signedBy"`
	// Status is "enabled" or "disabled".
	Status string `json:"status"`

	// Official marks sources of the Ubuntu archives, which must stay enabled.
	Official bool `json:"-"`
}

// Filter selects rows. Every non-empty field has to match at least one value
// of the column, ignoring case and matching substrings.
type Filter struct {
	Name      string
	URI       string
	Suite     string
	Component string
	Arch      string
	SignedBy  string
	Status    string
}

// ToRows converts the entries of source files into rows.
func ToRows(sources []ppa.SourceFile) []Row {
	rows := make([]Row, 0, len(sources))
	for i := range sources {
		source := &sources[i]

		archs := make([]string, len(source.Architectures))
		for j, arch := range source.Architectures {
			archs[j] = string(arch)
		}

		status := StatusEnabled
		if !source.IsEnabled() {
			status = StatusDisabled
		}

		rows = append(rows, Row{
			Name:          SourceName(source.Path),
			Path:          source.Path,
			URIs:          nonNil(source.URIs),
			Suites:        nonNil(source.Suites),
			Components:    nonNil(source.Components),
			Architectures: archs,
			SignedBy:      signedBy(source.SignedBy),
			Status:        status,
			Official:      ppa.IsOfficialUbuntu(source),
		})
	}
	return rows
}

// SourceName returns the file name of the source file without its extension.
func SourceName(path string) string {
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
}

// signedBy returns the keyring path, or a placeholder for an embedded key.
func signedBy(value string) string {
	if strings.Contains(value, "\n") || strings.Contains(value, "BEGIN PGP") {
		return InlineKey
	}
	return value
}

func nonNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func contains(values []string, needle string) bool {
	needle = strings.ToLower(needle)
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), needle) {
			return true
		}
	}
	return false
}

// Matches reports whether the row passes the filter.
func (f Filter) Matches(row Row) bool {
	for _, c := range []struct {
		needle string
		values []string
	}{
		{f.Name, []string{row.Name}},
		{f.URI, row.URIs},
		{f.Suite, row.Suites},
		{f.Component, row.Components},
		{f.Arch, row.Architectures},
		{f.SignedBy, []string{row.SignedBy}},
		{f.Status, []string{row.Status}},
	} {
		if c.needle != "" && !contains(c.values, c.needle) {
			return false
		}
	}
	return true
}

// Apply returns the rows that pass the filter.
func (f Filter) Apply(rows []Row) []Row {
	filtered := make([]Row, 0, len(rows))
	for _, row := range rows {
		if f.Matches(row) {
			filtered = append(filtered, row)
		}
	}
	return filtered
}
