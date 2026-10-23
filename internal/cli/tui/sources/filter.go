package sources

import (
	"strings"

	"yv35.com/ppa-utils/internal/cli/sourcelist"
)

// ParseFilter splits the text of the filter line into column filters
// ("suite:noble") and free text terms, which have to match any column.
func ParseFilter(text string) (sourcelist.Filter, []string) {
	var filter sourcelist.Filter
	var terms []string

	for _, token := range strings.Fields(text) {
		key, value, ok := strings.Cut(token, ":")
		if ok && value != "" && setColumnFilter(&filter, strings.ToLower(key), value) {
			continue
		}
		terms = append(terms, strings.ToLower(token))
	}
	return filter, terms
}

func setColumnFilter(filter *sourcelist.Filter, key, value string) bool {
	switch key {
	case "name":
		filter.Name = value
	case "status":
		filter.Status = value
	case "uri":
		filter.URI = value
	case "suite", "suites":
		filter.Suite = value
	case "component", "components":
		filter.Component = value
	case "arch":
		filter.Arch = value
	case "signedby", "signed-by":
		filter.SignedBy = value
	default:
		return false
	}
	return true
}

// FormatFilter returns the filter line text for the given column filters.
func FormatFilter(f sourcelist.Filter) string {
	var parts []string
	for _, c := range []struct{ key, value string }{
		{"name", f.Name},
		{"status", f.Status},
		{"uri", f.URI},
		{"suite", f.Suite},
		{"component", f.Component},
		{"arch", f.Arch},
		{"signed-by", f.SignedBy},
	} {
		if c.value != "" {
			parts = append(parts, c.key+":"+c.value)
		}
	}
	return strings.Join(parts, " ")
}

// applyFilter returns the rows matching the filter line text.
func applyFilter(rows []sourcelist.Row, text string) []sourcelist.Row {
	filter, terms := ParseFilter(text)
	matching := filter.Apply(rows)
	if len(terms) == 0 {
		return matching
	}

	result := make([]sourcelist.Row, 0, len(matching))
	for _, row := range matching {
		haystack := strings.ToLower(strings.Join(rowCells(row), " "))
		all := true
		for _, term := range terms {
			if !strings.Contains(haystack, term) {
				all = false
				break
			}
		}
		if all {
			result = append(result, row)
		}
	}
	return result
}
