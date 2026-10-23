package ppa

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadSourceFilesSplitsStanzas(t *testing.T) {
	sources, err := ReadSourceFiles(testdataPath("two-stanzas.sources"))
	if err != nil {
		t.Fatalf("ReadSourceFiles() error = %v", err)
	}
	if len(sources) != 2 {
		t.Fatalf("got %d entries, want 2", len(sources))
	}
	if sources[0].Stanza != 0 || sources[1].Stanza != 1 {
		t.Errorf("unexpected stanza indices: %d, %d", sources[0].Stanza, sources[1].Stanza)
	}
	if sources[0].URIs[0] != "http://de.archive.ubuntu.com/ubuntu/" || sources[1].URIs[0] != "http://security.ubuntu.com/ubuntu" {
		t.Errorf("entries were merged: %+v", sources)
	}
	if got := sources[1].Suites; len(got) != 1 || got[0] != "noble-security" {
		t.Errorf("Suites of second entry = %v", got)
	}
}

func TestIsEnabledDefaultsToTrue(t *testing.T) {
	source, err := ReadSourceFile(testdataPath("no-enabled-field.sources"))
	if err != nil {
		t.Fatal(err)
	}
	if !source.IsEnabled() {
		t.Error("source without Enabled field should be enabled")
	}
}

func TestIsOfficialUbuntu(t *testing.T) {
	tests := map[string]bool{
		"http://de.archive.ubuntu.com/ubuntu/":                      true,
		"http://security.ubuntu.com/ubuntu":                         true,
		"http://ports.ubuntu.com/ubuntu-ports":                      true,
		"https://esm.ubuntu.com/apps/ubuntu":                        true,
		"https://ubuntu.com/foo":                                    true,
		"https://ppa.launchpadcontent.net/kisak/kisak-mesa/ubuntu/": false,
		"https://download.docker.com/linux/ubuntu/":                 false,
		"https://notubuntu.com/ubuntu":                              false,
		"https://example.com/ubuntu.com/":                           false,
	}
	for uri, want := range tests {
		if got := IsOfficialUbuntu(&SourceFile{URIs: []string{uri}}); got != want {
			t.Errorf("IsOfficialUbuntu(%q) = %v, want %v", uri, got, want)
		}
	}
}

func TestSetEnabledDisableAddsField(t *testing.T) {
	content, err := os.ReadFile(testdataPath("no-enabled-field.sources"))
	if err != nil {
		t.Fatal(err)
	}

	got, err := SetEnabled(content, false)
	if err != nil {
		t.Fatalf("SetEnabled() error = %v", err)
	}

	if !strings.HasSuffix(string(content), "\n") {
		t.Fatal("fixture should end with a newline")
	}
	// The field is appended to the entry, before the trailing newline.
	want := strings.TrimSuffix(string(content), "\n") + "\nEnabled: no\n"
	if string(got) != want {
		t.Errorf("SetEnabled() =\n%s\nwant\n%s", got, want)
	}
}

func TestSetEnabledReplacesExistingValue(t *testing.T) {
	content, err := os.ReadFile(testdataPath("valid.sources"))
	if err != nil {
		t.Fatal(err)
	}

	disabled, err := SetEnabled(content, false)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(string(content), "Enabled: yes", "Enabled: no", 1)
	if string(disabled) != want {
		t.Errorf("disabled =\n%s\nwant\n%s", disabled, want)
	}

	enabled, err := SetEnabled(disabled, true)
	if err != nil {
		t.Fatal(err)
	}
	if string(enabled) != string(content) {
		t.Errorf("round trip changed the file:\n%s", enabled)
	}
}

func TestSetEnabledIsIdempotentAndMinimal(t *testing.T) {
	content, _ := os.ReadFile(testdataPath("no-enabled-field.sources"))

	// Enabling an entry without an Enabled field changes nothing.
	got, err := SetEnabled(content, true)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(content) {
		t.Errorf("enabling changed the file:\n%s", got)
	}

	once, _ := SetEnabled(content, false)
	twice, _ := SetEnabled(once, false)
	if string(once) != string(twice) {
		t.Errorf("disabling twice changed the file:\n%s", twice)
	}
}

func TestSetEnabledPreservesEverythingElse(t *testing.T) {
	content, _ := os.ReadFile(testdataPath("no-enabled-field.sources"))
	got, _ := SetEnabled(content, false)

	for _, line := range strings.Split(strings.TrimSuffix(string(content), "\n"), "\n") {
		if !strings.Contains(string(got), line+"\n") {
			t.Errorf("line %q was lost or changed", line)
		}
	}

	source := mustParse(t, got)
	if source.IsEnabled() {
		t.Error("entry should be disabled")
	}
	if !strings.Contains(source.SignedBy, "BEGIN PGP PUBLIC KEY BLOCK") {
		t.Errorf("inline key was damaged: %q", source.SignedBy)
	}
}

func TestSetEnabledEveryStanza(t *testing.T) {
	content, _ := os.ReadFile(testdataPath("two-stanzas.sources"))
	got, err := SetEnabled(content, false)
	if err != nil {
		t.Fatal(err)
	}

	if n := strings.Count(string(got), "Enabled: no"); n != 2 {
		t.Errorf("got %d Enabled fields, want 2:\n%s", n, got)
	}
	if !strings.HasPrefix(string(got), "# Official archive\n") {
		t.Errorf("leading comment was lost:\n%s", got)
	}
}

func TestSetEnabledWithoutEntry(t *testing.T) {
	if _, err := SetEnabled([]byte("# only a comment\n"), false); err == nil {
		t.Error("expected an error for a file without entries")
	}
}

func TestToggledContent(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}

	ppaFile := write("ppa.sources", "Types: deb\nURIs: https://ppa.launchpadcontent.net/a/b/ubuntu/\nSuites: noble\nComponents: main\n")
	got, err := ToggledContent(ppaFile, false)
	if err != nil {
		t.Fatalf("ToggledContent() error = %v", err)
	}
	if !strings.Contains(string(got), "Enabled: no") {
		t.Errorf("PPA was not disabled:\n%s", got)
	}

	official := write("ubuntu.sources", "Types: deb\nURIs: http://archive.ubuntu.com/ubuntu\nSuites: noble\nComponents: main\n")
	if _, err := ToggledContent(official, false); !errors.Is(err, ErrOfficialSource) {
		t.Errorf("disabling official source: error = %v, want ErrOfficialSource", err)
	}
	if _, err := ToggledContent(official, true); err != nil {
		t.Errorf("enabling official source should be allowed, got %v", err)
	}

	mixed := write("mixed.sources", "Types: deb\nURIs: https://example.com\nSuites: noble\n\nTypes: deb\nURIs: http://security.ubuntu.com/ubuntu\nSuites: noble-security\n")
	if _, err := ToggledContent(mixed, false); !errors.Is(err, ErrOfficialSource) {
		t.Errorf("file with an official entry: error = %v, want ErrOfficialSource", err)
	}
}

func mustParse(t *testing.T, content []byte) *SourceFile {
	t.Helper()
	path := filepath.Join(t.TempDir(), "x.sources")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	source, err := ReadSourceFile(path)
	if err != nil {
		t.Fatalf("ReadSourceFile() error = %v", err)
	}
	return source
}
