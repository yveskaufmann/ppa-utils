package ppa

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
)

// ErrOfficialSource is returned when disabling an official Ubuntu source is attempted.
var ErrOfficialSource = errors.New("official Ubuntu sources cannot be disabled")

// IsOfficialUbuntu reports whether the source points to an official Ubuntu
// repository (archive.ubuntu.com, security.ubuntu.com, ports.ubuntu.com, ...).
// Launchpad PPAs are not official.
func IsOfficialUbuntu(source *SourceFile) bool {
	for _, uri := range source.URIs {
		parsed, err := url.Parse(uri)
		if err != nil {
			continue
		}
		host := strings.ToLower(parsed.Hostname())
		if host == "ubuntu.com" || strings.HasSuffix(host, ".ubuntu.com") {
			return true
		}
	}
	return false
}

// ToggledContent returns the content of the source file at path with all its
// entries enabled or disabled. Disabling a file that contains an official
// Ubuntu source fails with ErrOfficialSource.
func ToggledContent(path string, enabled bool) ([]byte, error) {
	if !enabled {
		sources, err := ReadSourceFiles(path)
		if err != nil {
			return nil, err
		}
		for i := range sources {
			if IsOfficialUbuntu(&sources[i]) {
				return nil, fmt.Errorf("%s: %w", path, ErrOfficialSource)
			}
		}
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return SetEnabled(content, enabled)
}

// SetEnabled sets the Enabled field of every entry in the deb822 source file
// content. Everything else, including comments, custom fields and embedded
// keys, is left byte-identical. Enabling an entry without an Enabled field
// changes nothing, as entries are enabled by default.
func SetEnabled(content []byte, enabled bool) ([]byte, error) {
	lines := strings.Split(string(content), "\n")
	want := EnabledStatusNo
	if enabled {
		want = EnabledStatusYes
	}

	var out []string
	foundEntry := false

	for i := 0; i < len(lines); {
		// A block is a run of non-blank lines.
		if strings.TrimSpace(lines[i]) == "" {
			out = append(out, lines[i])
			i++
			continue
		}
		start := i
		for i < len(lines) && strings.TrimSpace(lines[i]) != "" {
			i++
		}
		block := lines[start:i]

		if !hasFields(block) {
			out = append(out, block...)
			continue
		}
		foundEntry = true
		out = append(out, setEnabledInBlock(block, want)...)
	}

	if !foundEntry {
		return nil, errors.New("no entry found in source file")
	}
	return []byte(strings.Join(out, "\n")), nil
}

func hasFields(block []string) bool {
	for _, line := range block {
		if isFieldLine(line) {
			return true
		}
	}
	return false
}

// isFieldLine reports whether the line starts a field, i.e. it is neither a
// comment nor a continuation line.
func isFieldLine(line string) bool {
	return line != "" && line[0] != '#' && line[0] != ' ' && line[0] != '\t'
}

func setEnabledInBlock(block []string, want EnabledStatus) []string {
	for i, line := range block {
		if !isFieldLine(line) {
			continue
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok || !strings.EqualFold(strings.TrimSpace(name), "Enabled") {
			continue
		}

		if ParseEnabledStatus(strings.TrimSpace(value)) == want {
			return block
		}
		changed := append([]string(nil), block...)
		changed[i] = "Enabled: " + strings.ToLower(string(want))
		return changed
	}

	if want == EnabledStatusYes {
		return block
	}
	return append(append([]string(nil), block...), "Enabled: no")
}
