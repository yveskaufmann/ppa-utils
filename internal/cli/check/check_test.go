package check

import (
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"yv35.com/ppa/ppa-utils/internal/ppa"
)

func TestIsUsingUbuntuReleaseCode(t *testing.T) {
	tests := []struct {
		name   string
		source ppa.SourceFile
		want   bool
	}{
		{"release suite", ppa.SourceFile{Suites: []string{"noble"}}, true},
		{"stable suite", ppa.SourceFile{Suites: []string{"stable"}}, false},
		{"flat repo", ppa.SourceFile{Suites: []string{"/"}}, false},
		{"multiverse component", ppa.SourceFile{Suites: []string{"noble"}, Components: []string{"multiverse"}}, false},
		{"mixed suites", ppa.SourceFile{Suites: []string{"stable", "noble"}}, true},
		{"no suites", ppa.SourceFile{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isUsingUbuntuReleaseCode(&tt.source); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToInReleaseURL(t *testing.T) {
	tests := []struct {
		name   string
		source ppa.SourceFile
		code   string
		want   string
	}{
		{"explicit code", ppa.SourceFile{URIs: []string{"https://example.com/ubuntu"}, Suites: []string{"noble"}}, "plucky", "https://example.com/ubuntu/dists/plucky/InRelease"},
		{"trailing slash", ppa.SourceFile{URIs: []string{"https://example.com/ubuntu/"}, Suites: []string{"noble"}}, "plucky", "https://example.com/ubuntu/dists/plucky/InRelease"},
		{"falls back to first suite", ppa.SourceFile{URIs: []string{"https://example.com"}, Suites: []string{"noble"}}, "", "https://example.com/dists/noble/InRelease"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toInReleaseURL(&tt.source, tt.code); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsUbuntuReleaseCodeSupported(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/dists/noble/InRelease" {
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	source := ppa.SourceFile{URIs: []string{srv.URL}, Suites: []string{"noble"}}
	if !isUbuntuReleaseCodeSupported(&source, "noble") {
		t.Error("noble: expected supported")
	}
	if isUbuntuReleaseCodeSupported(&source, "plucky") {
		t.Error("plucky: expected unsupported")
	}
}

func TestCheckCommandRejectsTooManyArgs(t *testing.T) {
	cmd := New(nil)
	cmd.SetArgs([]string{"noble", "plucky"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	if err := cmd.Execute(); err == nil {
		t.Error("Execute() expected error for two arguments, got nil")
	}
}

func TestCheckCommandRejectsUnknownRelease(t *testing.T) {
	if _, err := os.Stat("/usr/share/distro-info/ubuntu.csv"); err != nil {
		t.Skipf("distro-info data not available: %v", err)
	}

	cmd := New(nil)
	cmd.SetArgs([]string{"not-a-release"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "unknown Ubuntu release") {
		t.Errorf("Execute() error = %v, want unknown release error", err)
	}
}

func TestCompleteReleaseCodes(t *testing.T) {
	if _, err := os.Stat("/usr/share/distro-info/ubuntu.csv"); err != nil {
		t.Skipf("distro-info data not available: %v", err)
	}

	got, directive := completeReleaseCodes(New(nil), nil, "")
	if want := cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveKeepOrder; directive != want {
		t.Errorf("directive = %v, want %v", directive, want)
	}
	if len(got) < 2 || slices.Index(got, "noble") > slices.Index(got, "jammy") {
		t.Errorf("completions should be newest first: %v", got)
	}
	for _, code := range []string{"jammy", "noble"} {
		if !slices.Contains(got, code) {
			t.Errorf("completions missing %q: %v", code, got)
		}
	}

	got, directive = completeReleaseCodes(New(nil), []string{"noble"}, "")
	if len(got) != 0 || directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("second argument: got (%v, %v), want no completions", got, directive)
	}
}

func TestNewPassesOptionsToFn(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantRelease string
	}{
		{"no argument", nil, ""},
		{"release argument", []string{"noble"}, "noble"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got *Options
			cmd := New(func(opts *Options) error {
				got = opts
				return nil
			})
			cmd.SetArgs(tt.args)

			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if got == nil {
				t.Fatal("fn was not called")
			}
			if got.Release != tt.wantRelease {
				t.Errorf("Release = %q, want %q", got.Release, tt.wantRelease)
			}
			if got.Out == nil || got.ErrOut == nil {
				t.Error("Out and ErrOut should be set")
			}
		})
	}
}
