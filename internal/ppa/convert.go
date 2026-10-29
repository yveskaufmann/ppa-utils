package ppa

import (
	"fmt"
	"regexp"
	"strings"
)

var listLinePattern = regexp.MustCompile(`^\s*(?P<type>deb(?:-src)?|deb)\s*(?:\[(?P<options>[^\]]*)\])?\s+(?P<uri>\S+)\s+(?P<suite>\S+)(?:\s+(?P<components>\S.*))?\s*$`)

// ParseListLine converts an apt .list entry such as:
//
//	deb [arch=amd64,arm64 signed-by=/path/to/key.asc] https://example.com/repo stable main
//
// into the canonical SourceFile used throughout this project.
func ParseListLine(line string) (*SourceFile, error) {
	text := strings.TrimSpace(line)
	if text == "" {
		return nil, fmt.Errorf("empty list entry")
	}
	if idx := strings.Index(text, "#"); idx >= 0 {
		text = strings.TrimSpace(text[:idx])
		if text == "" {
			return nil, fmt.Errorf("empty list entry")
		}
	}

	matches := listLinePattern.FindStringSubmatch(text)
	if matches == nil {
		return nil, fmt.Errorf("invalid apt list line: %q", line)
	}

	typeName := matches[1]
	options := matches[2]
	uri := matches[3]
	suite := matches[4]
	componentsText := matches[5]

	if strings.TrimSpace(uri) == "" {
		return nil, fmt.Errorf("invalid apt list line: missing URI: %q", line)
	}

	sourceFile := &SourceFile{
		Types:      []string{typeName},
		URIs:       []string{uri},
		Suites:     []string{suite},
		Components: []string{"main"},
	}
	if componentsText != "" {
		sourceFile.Components = strings.Fields(componentsText)
	}

	for _, token := range strings.Fields(options) {
		key, value, ok := strings.Cut(token, "=")
		if !ok {
			continue
		}
		switch strings.ToLower(key) {
		case "arch":
			for _, arch := range strings.Split(value, ",") {
				arch = strings.TrimSpace(arch)
				if arch == "" {
					continue
				}
				sourceFile.Architectures = append(sourceFile.Architectures, ParseArchitecture(arch))
			}
		case "signed-by":
			sourceFile.SignedBy = strings.TrimSpace(value)
		}
	}

	if len(sourceFile.Components) == 0 {
		sourceFile.Components = []string{"main"}
	}
	return sourceFile, nil
}

// EncodeSourceFile writes the source in deb822 form.
func EncodeSourceFile(source *SourceFile) string {
	var b strings.Builder
	if len(source.Types) > 0 {
		b.WriteString("Types: ")
		b.WriteString(strings.Join(source.Types, " "))
		b.WriteByte('\n')
	}
	if len(source.URIs) > 0 {
		b.WriteString("URIs: ")
		b.WriteString(strings.Join(source.URIs, " "))
		b.WriteByte('\n')
	}
	if len(source.Suites) > 0 {
		b.WriteString("Suites: ")
		b.WriteString(strings.Join(source.Suites, " "))
		b.WriteByte('\n')
	}
	if len(source.Components) > 0 {
		b.WriteString("Components: ")
		b.WriteString(strings.Join(source.Components, " "))
		b.WriteByte('\n')
	}
	if len(source.Architectures) > 0 {
		b.WriteString("Architectures: ")
		archs := make([]string, 0, len(source.Architectures))
		for _, arch := range source.Architectures {
			archs = append(archs, string(arch))
		}
		b.WriteString(strings.Join(archs, ", "))
		b.WriteByte('\n')
	}
	if source.SignedBy != "" {
		b.WriteString("Signed-By: ")
		b.WriteString(source.SignedBy)
		b.WriteByte('\n')
	}
	if source.Enabled != "" && source.Enabled != EnabledStatusYes {
		b.WriteString("Enabled: ")
		b.WriteString(strings.ToLower(string(source.Enabled)))
		b.WriteByte('\n')
	}
	return b.String()
}
