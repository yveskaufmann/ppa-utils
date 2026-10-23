package ppa

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// sourceField is a single field of a deb822 source file. Value holds the
// complete value, with continuation lines joined by newlines.
type sourceField struct {
	Name  string
	Value string
}

// ReadSourceFile reads the first entry of the source file at the given path.
func ReadSourceFile(path string) (*SourceFile, error) {
	sources, err := ReadSourceFiles(path)
	if err != nil {
		return nil, err
	}
	if len(sources) == 0 {
		return nil, fmt.Errorf("no entry in source file %s", path)
	}
	return &sources[0], nil
}

// ReadSourceFiles reads all entries (stanzas separated by blank lines) of the
// source file at the given path.
func ReadSourceFiles(path string) ([]SourceFile, error) {
	stanzas, err := readSourceStanzas(path)
	if err != nil {
		return nil, err
	}

	sources := make([]SourceFile, 0, len(stanzas))
	for i, fields := range stanzas {
		source, err := parseSourceFields(path, fields)
		if err != nil {
			return nil, err
		}
		source.Stanza = i
		sources = append(sources, *source)
	}
	return sources, nil
}

func parseSourceFields(path string, fields []sourceField) (*SourceFile, error) {
	sourceFile := &SourceFile{
		Path: path,
	}

	for _, f := range fields {
		switch f.Name {
		case "Types":
			sourceFile.Types = strings.Fields(f.Value)
		case "URIs":
			sourceFile.URIs = strings.Fields(f.Value)
		case "Suites":
			sourceFile.Suites = strings.Fields(f.Value)
		case "Components":
			sourceFile.Components = strings.Fields(f.Value)
		case "Architectures":
			for arch := range strings.FieldsSeq(f.Value) {
				sourceFile.Architectures = append(sourceFile.Architectures, ParseArchitecture(arch))
			}
		case "Signed-By", "Signed-by":
			// Either a path to a keyring file or an embedded ASCII-armored
			// PGP public key spanning multiple lines.
			sourceFile.SignedBy = f.Value
		case "Enabled":
			sourceFile.Enabled = ParseEnabledStatus(f.Value)
		default:
			// We allow custom fields that start with "X-"
			if strings.HasPrefix(f.Name, "X-") {
				// TODO: Store custom fields in a map within the SourceFile struct.
				continue
			}
			return nil, fmt.Errorf("unknown field in source file %s: %s", path, f.Name)
		}
	}

	return sourceFile, nil
}

// readSourceStanzas reads the deb822 stanzas of the source file at the given path.
//
// Stanzas are separated by blank lines. Lines starting with a space or tab
// continue the value of the previous field. A continuation line consisting of
// a single "." represents an empty line, as used within embedded PGP public keys.
func readSourceStanzas(path string) ([][]sourceField, error) {

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)

	var stanzas [][]sourceField
	var fields []sourceField

	endStanza := func() {
		if len(fields) > 0 {
			stanzas = append(stanzas, fields)
			fields = nil
		}
	}

	for scanner.Scan() {
		line := scanner.Text()

		if strings.TrimSpace(line) == "" {
			endStanza()
			continue
		}
		if line[0] == '#' {
			continue
		}

		// Continuation line of the previous field
		if line[0] == ' ' || line[0] == '\t' {
			if len(fields) == 0 {
				return nil, fmt.Errorf("continuation line without field in source file %s: %s", path, line)
			}

			value := strings.TrimSpace(line)
			if value == "." {
				value = ""
			}

			last := &fields[len(fields)-1]
			if last.Value == "" {
				last.Value = value
			} else {
				last.Value += "\n" + value
			}
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid line in source file %s: %s", path, line)
		}

		fields = append(fields, sourceField{
			Name:  strings.TrimSpace(parts[0]),
			Value: strings.TrimSpace(parts[1]),
		})
	}
	endStanza()

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to parse source file %s: %w", path, err)
	}

	return stanzas, nil
}
