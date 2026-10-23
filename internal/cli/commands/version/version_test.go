package version

import (
	"bytes"
	"testing"
)

func TestRunPrintsVersion(t *testing.T) {
	var out bytes.Buffer
	if err := run(&Options{Version: "1.2.3", Out: &out}); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got, want := out.String(), "ppautil 1.2.3\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestNewPassesOptionsToFn(t *testing.T) {
	var got *Options
	cmd := New(func(opts *Options) error {
		got = opts
		return nil
	})
	cmd.SetArgs(nil)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got == nil || got.Version == "" || got.Out == nil {
		t.Errorf("options not populated: %+v", got)
	}
}

func TestVersionCommandRejectsArgs(t *testing.T) {
	cmd := New(func(*Options) error { return nil })
	cmd.SetArgs([]string{"extra"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	if err := cmd.Execute(); err == nil {
		t.Error("Execute() expected error for extra args, got nil")
	}
}
