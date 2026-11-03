package clitext

import (
	"bytes"
	"strings"
	"testing"
)

func TestLong(t *testing.T) {
	got := Long(`
		First line.

		Second line.
	`)
	if want := "First line.\n\nSecond line."; got != want {
		t.Errorf("Long() = %q, want %q", got, want)
	}
}

func TestExamples(t *testing.T) {
	got := Examples(`
		# comment
		tool run

		# other
		tool run x
	`)
	want := "  # comment\n  tool run\n\n  # other\n  tool run x"
	if got != want {
		t.Errorf("Examples() = %q, want %q", got, want)
	}
}

func TestConfirm(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"yes", "yes\n", true},
		{"y upper", "Y\n", true},
		{"no", "n\n", false},
		{"empty", "\n", false},
		{"eof", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			got, err := Confirm("Proceed?", strings.NewReader(tt.input), &out)
			if err != nil {
				t.Fatalf("Confirm() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("Confirm() = %v, want %v", got, tt.want)
			}
			if want := "Proceed? [y/N]: "; out.String() != want {
				t.Errorf("prompt = %q, want %q", out.String(), want)
			}
		})
	}
}
