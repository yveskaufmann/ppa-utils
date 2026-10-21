// Package clitext formats multi-line help texts of cobra commands.
package clitext

import (
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
)

// Long trims the common indentation of a multi-line description.
func Long(s string) string {
	return strings.TrimSpace(heredoc.Doc(s))
}

// Examples trims the common indentation of the examples and indents every
// line by two spaces, as cobra prints them below the "Examples:" heading.
func Examples(s string) string {
	lines := strings.Split(strings.TrimSpace(heredoc.Doc(s)), "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = "  " + line
		}
	}
	return strings.Join(lines, "\n")
}
