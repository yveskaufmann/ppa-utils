package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	oldBinary = "old binary"
	newBinary = "new binary"
)

// newArchive returns a tar.gz holding a single file called ppa-utils.
func newArchive(t *testing.T, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: "ppa-utils", Mode: 0o755, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// newRelease serves a fake GitHub release for tag v0.2.0. A bad checksum makes
// checksums.txt list a wrong hash.
func newRelease(t *testing.T, badChecksum bool) *httptest.Server {
	t.Helper()
	archiveName := "ppa-utils_0.2.0_linux_amd64.tar.gz"
	archive := newArchive(t, newBinary)
	sum := sha256.Sum256(archive)
	hash := hex.EncodeToString(sum[:])
	if badChecksum {
		hash = strings.Repeat("0", 64)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/releases/tag/v0.2.0", http.StatusFound)
	})
	mux.HandleFunc("/releases/tag/v0.2.0", func(w http.ResponseWriter, _ *http.Request) {})
	mux.HandleFunc("/releases/download/v0.2.0/"+archiveName, func(w http.ResponseWriter, _ *http.Request) {
		w.Write(archive)
	})
	mux.HandleFunc("/releases/download/v0.2.0/checksums.txt", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("0000  other.tar.gz\n" + hash + "  " + archiveName + "\n"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// testOptions returns options that update a temp binary from srv.
func testOptions(t *testing.T, srv *httptest.Server, current string) (*Options, string, *bytes.Buffer) {
	t.Helper()
	exe := filepath.Join(t.TempDir(), "ppa-utils")
	if err := os.WriteFile(exe, []byte(oldBinary), 0o755); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	return &Options{
		CurrentVersion: current,
		BaseURL:        srv.URL,
		GOOS:           "linux",
		GOARCH:         "amd64",
		Executable:     func() (string, error) { return exe, nil },
		Client:         srv.Client(),
		In:             strings.NewReader(""),
		Out:            out,
	}, exe, out
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestRunReplacesBinary(t *testing.T) {
	opts, exe, out := testOptions(t, newRelease(t, false), "0.1.0")
	opts.Yes = true

	if err := run(opts); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got := readFile(t, exe); got != newBinary {
		t.Errorf("binary = %q, want %q", got, newBinary)
	}
	info, err := os.Stat(exe)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("mode = %v, want 0755", info.Mode().Perm())
	}
	if !strings.Contains(out.String(), "0.1.0 -> 0.2.0") {
		t.Errorf("output = %q, want the version change", out.String())
	}
	entries, _ := os.ReadDir(filepath.Dir(exe))
	if len(entries) != 1 {
		t.Errorf("temp files left behind: %v", entries)
	}
}

func TestRunAcceptsVPrefixedCurrentVersion(t *testing.T) {
	opts, exe, _ := testOptions(t, newRelease(t, false), "v0.1.0")
	opts.Yes = true

	if err := run(opts); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got := readFile(t, exe); got != newBinary {
		t.Errorf("binary = %q, want %q", got, newBinary)
	}
}

func TestRunUpToDate(t *testing.T) {
	opts, exe, out := testOptions(t, newRelease(t, false), "0.2.0")
	opts.Yes = true

	if err := run(opts); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got := readFile(t, exe); got != oldBinary {
		t.Errorf("binary changed to %q", got)
	}
	if !strings.Contains(out.String(), "up to date") {
		t.Errorf("output = %q, want up to date", out.String())
	}
}

func TestRunCheckDoesNotWrite(t *testing.T) {
	opts, exe, out := testOptions(t, newRelease(t, false), "0.1.0")
	opts.Check = true

	if err := run(opts); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got := readFile(t, exe); got != oldBinary {
		t.Errorf("binary changed to %q", got)
	}
	if want := "Update available: 0.1.0 -> 0.2.0"; !strings.Contains(out.String(), want) {
		t.Errorf("output = %q, want %q", out.String(), want)
	}
}

func TestRunChecksumMismatchKeepsBinary(t *testing.T) {
	opts, exe, _ := testOptions(t, newRelease(t, true), "0.1.0")
	opts.Yes = true

	err := run(opts)
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("run() error = %v, want checksum mismatch", err)
	}
	if got := readFile(t, exe); got != oldBinary {
		t.Errorf("binary changed to %q", got)
	}
}

func TestRunConfirmation(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"yes", "y\n", newBinary},
		{"no", "n\n", oldBinary},
		{"empty", "\n", oldBinary},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, exe, out := testOptions(t, newRelease(t, false), "0.1.0")
			opts.In = strings.NewReader(tt.input)

			if err := run(opts); err != nil {
				t.Fatalf("run() error = %v", err)
			}
			if got := readFile(t, exe); got != tt.want {
				t.Errorf("binary = %q, want %q", got, tt.want)
			}
			if !strings.Contains(out.String(), "Update ppa-utils 0.1.0 -> 0.2.0?") {
				t.Errorf("output = %q, want the confirmation prompt", out.String())
			}
		})
	}
}

func TestRunRefusesDevBuild(t *testing.T) {
	opts, _, _ := testOptions(t, newRelease(t, false), "dev")

	if err := run(opts); err == nil || !strings.Contains(err.Error(), "development build") {
		t.Fatalf("run() error = %v, want development build error", err)
	}
}

func TestRunRejectsUnsupportedPlatform(t *testing.T) {
	for _, p := range [][2]string{{"darwin", "arm64"}, {"linux", "386"}} {
		opts, _, _ := testOptions(t, newRelease(t, false), "0.1.0")
		opts.GOOS, opts.GOARCH = p[0], p[1]

		if err := run(opts); err == nil {
			t.Errorf("run() for %s/%s expected error, got nil", p[0], p[1])
		}
	}
}

func TestRunUnwritableDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write to any directory")
	}
	opts, exe, _ := testOptions(t, newRelease(t, false), "0.1.0")
	opts.Yes = true
	dir := filepath.Dir(exe)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	err := run(opts)
	if err == nil || !strings.Contains(err.Error(), "rerun with sudo") {
		t.Fatalf("run() error = %v, want sudo hint", err)
	}
	if got := readFile(t, exe); got != oldBinary {
		t.Errorf("binary changed to %q", got)
	}
}

func TestLatestTagWithoutRelease(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()

	if _, err := latestTag(srv.Client(), srv.URL); err == nil {
		t.Error("latestTag() expected error, got nil")
	}
}

func TestExtractBinaryMissing(t *testing.T) {
	if _, err := extractBinary(newArchive(t, "x"), "other"); err == nil {
		t.Error("extractBinary() expected error, got nil")
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"0.1.0", "v0.2.0", -1},
		{"v1.0.0", "1.0.0", 0},
		{"1.10.0", "1.9.9", 1},
		{"2.0.0", "10.0.0", -1},
	}
	for _, tt := range tests {
		got, err := compareVersions(tt.a, tt.b)
		if err != nil {
			t.Errorf("compareVersions(%q, %q) error = %v", tt.a, tt.b, err)
			continue
		}
		if got != tt.want {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
	for _, bad := range []string{"", "1.2", "1.2.x", "dev"} {
		if _, err := compareVersions(bad, "1.0.0"); err == nil {
			t.Errorf("compareVersions(%q) expected error, got nil", bad)
		}
	}
}

func TestNewParsesFlags(t *testing.T) {
	var got *Options
	cmd := New(func(opts *Options) error {
		got = opts
		return nil
	})
	cmd.SetArgs([]string{"--check", "--yes"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got == nil || !got.Check || !got.Yes || got.Out == nil || got.In == nil || got.CurrentVersion == "" {
		t.Errorf("options not populated: %+v", got)
	}
}

func TestNewRejectsArgs(t *testing.T) {
	cmd := New(func(*Options) error { return nil })
	cmd.SetArgs([]string{"extra"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	if err := cmd.Execute(); err == nil {
		t.Error("Execute() expected error for extra args, got nil")
	}
}
