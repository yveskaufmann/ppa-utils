package clitext

import "testing"

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
