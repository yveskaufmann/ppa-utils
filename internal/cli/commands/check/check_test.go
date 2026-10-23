package check

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"yv35.com/ppa-utils/internal/ppa"
)

var testKnown = releases{
	isCode: func(token string) bool {
		return slices.Contains([]string{"lucid", "focal", "jammy", "noble", "resolute", "stonking"}, token)
	},
	isVersion: func(token string) bool {
		return yearMonth.MatchString(token) && slices.Contains([]string{"20.04", "22.04", "24.04", "26.04", "26.10"}, token)
	},
}

var (
	noble    = release{code: "noble", version: "24.04"}
	stonking = release{code: "stonking", version: "26.10"}
)

func TestIsUsingUbuntuReleaseCode(t *testing.T) {
	tests := []struct {
		name   string
		source ppa.SourceFile
		want   bool
	}{
		{"code in suite", ppa.SourceFile{Suites: []string{"noble"}}, true},
		{"code with suffix in suite", ppa.SourceFile{Suites: []string{"noble-updates"}}, true},
		{"code inside suite path", ppa.SourceFile{Suites: []string{"resolute/mongodb-org/8.3"}}, true},
		{"code in URI", ppa.SourceFile{URIs: []string{"https://example.com/noble/repo"}, Suites: []string{"stable"}}, true},
		{"code in component", ppa.SourceFile{URIs: []string{"https://example.com"}, Suites: []string{"stable"}, Components: []string{"noble"}}, true},
		{"official sources with multiverse", ppa.SourceFile{Suites: []string{"noble"}, Components: []string{"main", "multiverse"}}, true},
		{"stable suite", ppa.SourceFile{URIs: []string{"https://example.com"}, Suites: []string{"stable"}, Components: []string{"main"}}, false},
		{"flat repo", ppa.SourceFile{URIs: []string{"https://example.com"}, Suites: []string{"/"}}, false},
		{"code only as part of a URI name", ppa.SourceFile{URIs: []string{"https://ppa.example.com/lucid-studio/ppa/ubuntu"}, Suites: []string{"stable"}}, false},
		{"code only as part of a host name", ppa.SourceFile{URIs: []string{"https://noblesse.example.com"}, Suites: []string{"stable"}}, false},
		{"version in URI path", ppa.SourceFile{URIs: []string{"https://packages.microsoft.com/ubuntu/26.04/prod"}, Suites: []string{"stable"}}, true},
		{"version in URI name", ppa.SourceFile{URIs: []string{"http://download.opensuse.org/repositories/home:/x/xUbuntu_26.04/"}, Suites: []string{"/"}}, true},
		{"version in suite", ppa.SourceFile{URIs: []string{"https://example.com"}, Suites: []string{"22.04"}}, true},
		{"non-Ubuntu version", ppa.SourceFile{URIs: []string{"https://pkgs.k8s.io/core:/stable:/v1.28/deb/"}, Suites: []string{"/"}}, false},
		{"year-month that is no Ubuntu release", ppa.SourceFile{URIs: []string{"https://example.com/23.07/deb"}, Suites: []string{"stable"}}, false},
		{"version only part of a longer number", ppa.SourceFile{URIs: []string{"https://example.com/1.26.04/deb"}, Suites: []string{"stable"}}, false},
		{"empty", ppa.SourceFile{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isUsingUbuntuReleaseCode(&tt.source, testKnown); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRetarget(t *testing.T) {
	source := ppa.SourceFile{
		URIs:       []string{"https://example.com/resolute/lucid-studio/"},
		Suites:     []string{"resolute", "resolute-updates", "resolute/mongodb-org/8.3"},
		Components: []string{"main", "resolute"},
	}
	got := retarget(&source, testKnown, noble)

	want := ppa.SourceFile{
		URIs:       []string{"https://example.com/noble/lucid-studio/"},
		Suites:     []string{"noble", "noble-updates", "noble/mongodb-org/8.3"},
		Components: []string{"main", "noble"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("retarget() = %+v, want %+v", got, want)
	}
	if source.Suites[0] != "resolute" {
		t.Error("retarget() must not modify the original source")
	}
}

func TestRetargetReplacesVersions(t *testing.T) {
	source := ppa.SourceFile{
		URIs:   []string{"https://packages.microsoft.com/ubuntu/26.04/prod", "http://example.com/xUbuntu_26.04/", "https://pkgs.k8s.io/core:/stable:/v1.28/deb/"},
		Suites: []string{"resolute"},
	}

	got := retarget(&source, testKnown, noble)
	want := []string{"https://packages.microsoft.com/ubuntu/24.04/prod", "http://example.com/xUbuntu_24.04/", "https://pkgs.k8s.io/core:/stable:/v1.28/deb/"}
	if !slices.Equal(got.URIs, want) {
		t.Errorf("URIs = %v, want %v", got.URIs, want)
	}

	got = retarget(&source, testKnown, release{code: "noble"})
	if got.URIs[0] != source.URIs[0] {
		t.Errorf("without a target version the version must stay, got %q", got.URIs[0])
	}
	if got.Suites[0] != "noble" {
		t.Errorf("Suites[0] = %q, want noble", got.Suites[0])
	}
}

func TestToInReleaseURL(t *testing.T) {
	tests := []struct {
		name, uri, suite, want string
	}{
		{"plain", "https://example.com/ubuntu", "noble", "https://example.com/ubuntu/dists/noble/InRelease"},
		{"trailing slash", "https://example.com/ubuntu/", "noble", "https://example.com/ubuntu/dists/noble/InRelease"},
		{"flat repository", "http://example.com/xUbuntu_24.04/", "/", "http://example.com/xUbuntu_24.04/InRelease"},
		{"flat repository with directory", "http://example.com", "sub/dir/", "http://example.com/sub/dir/InRelease"},
		{"suite with path", "https://example.com", "noble/mongodb-org/8.3", "https://example.com/dists/noble/mongodb-org/8.3/InRelease"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toInReleaseURL(tt.uri, tt.suite); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsUbuntuReleaseCodeSupported(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ubuntu/dists/noble/InRelease", "/noble/dists/stable/InRelease", "/ubuntu/24.04/prod/dists/noble/InRelease":
		case "/ubuntu/dists/jammy/InRelease":
			io.WriteString(w, "Origin: Example\nComponents: main universe\n")
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	tests := []struct {
		name   string
		source ppa.SourceFile
		target release
		want   bool
	}{
		{"suite replaced, target published", ppa.SourceFile{URIs: []string{srv.URL + "/ubuntu"}, Suites: []string{"resolute"}}, noble, true},
		{"suite replaced, target not published", ppa.SourceFile{URIs: []string{srv.URL + "/ubuntu"}, Suites: []string{"resolute"}}, stonking, false},
		{"URI replaced, target published", ppa.SourceFile{URIs: []string{srv.URL + "/resolute"}, Suites: []string{"stable"}}, noble, true},
		{"URI replaced, target not published", ppa.SourceFile{URIs: []string{srv.URL + "/resolute"}, Suites: []string{"stable"}}, stonking, false},
		{"component replaced, not listed in release", ppa.SourceFile{URIs: []string{srv.URL + "/ubuntu"}, Suites: []string{"resolute"}, Components: []string{"main", "resolute"}}, release{code: "jammy"}, false},
		{"version and suite replaced, target published", ppa.SourceFile{URIs: []string{srv.URL + "/ubuntu/26.04/prod"}, Suites: []string{"resolute"}}, noble, true},
		{"version and suite replaced, target not published", ppa.SourceFile{URIs: []string{srv.URL + "/ubuntu/26.04/prod"}, Suites: []string{"resolute"}}, stonking, false},
		{"no URIs", ppa.SourceFile{Suites: []string{"noble"}}, noble, false},
		{"no suites", ppa.SourceFile{URIs: []string{srv.URL}}, noble, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isUbuntuReleaseCodeSupported(&tt.source, testKnown, tt.target); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsUbuntuReleaseCodeSupportedChecksComponents(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "-----BEGIN PGP SIGNED MESSAGE-----\n\nComponents: main focal\n")
	}))
	t.Cleanup(srv.Close)

	source := ppa.SourceFile{URIs: []string{srv.URL}, Suites: []string{"stable"}, Components: []string{"main", "resolute"}}
	if !isUbuntuReleaseCodeSupported(&source, testKnown, release{code: "focal"}) {
		t.Error("focal: component is listed, expected supported")
	}
	if isUbuntuReleaseCodeSupported(&source, testKnown, noble) {
		t.Error("noble: component is not listed, expected unsupported")
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
