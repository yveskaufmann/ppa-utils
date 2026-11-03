// Package clitext formats multi-line help texts of cobra commands.
package clitext

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
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

// Prompt asks the user for an input value using the provided reader and writer.
func Prompt(label, defaultValue string, in io.Reader, out io.Writer) string {
	if in == nil {
		in = os.Stdin
	}
	if out == nil {
		out = io.Discard
	}

	reader := bufio.NewReader(in)
	if defaultValue == "" {
		fmt.Fprintf(out, "%s: ", label)
	} else {
		fmt.Fprintf(out, "%s [%s]: ", label, defaultValue)
	}
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return defaultValue
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultValue
	}
	return line
}

// ConfirmOverwrite asks the user to confirm overwriting an existing file.
func ConfirmOverwrite(path string, in io.Reader, out io.Writer) (bool, error) {
	return Confirm(fmt.Sprintf("Target file %s already exists. Overwrite it?", path), in, out)
}

// Confirm asks a yes/no question and returns true only for "y" or "yes".
// An empty answer or end of input counts as no.
func Confirm(question string, in io.Reader, out io.Writer) (bool, error) {
	if in == nil {
		in = os.Stdin
	}
	if out == nil {
		out = io.Discard
	}

	fmt.Fprintf(out, "%s [y/N]: ", question)
	reader := bufio.NewReader(in)
	answer, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	answer = strings.TrimSpace(answer)
	return strings.EqualFold(answer, "y") || strings.EqualFold(answer, "yes"), nil
}
