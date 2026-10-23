package sources

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yv35.com/ppa/ppa-utils/internal/ppa"
)

func TestSudoTogglerRefusesOfficialSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ubuntu.sources")
	content := "Types: deb\nURIs: http://archive.ubuntu.com/ubuntu\nSuites: noble\nComponents: main\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	msg := SudoToggler{}.SetEnabled(path, false)()
	got, ok := msg.(toggledMsg)
	if !ok || !errors.Is(got.err, ppa.ErrOfficialSource) {
		t.Fatalf("msg = %+v, want ErrOfficialSource", msg)
	}

	if after, _ := os.ReadFile(path); string(after) != content {
		t.Errorf("the file was modified:\n%s", after)
	}
}

func TestWriteFileAtomicKeepsModeAndReplacesContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.sources")
	if err := os.WriteFile(path, []byte("old"), 0o640); err != nil {
		t.Fatal(err)
	}

	if err := writeFileAtomic(path, []byte("new")); err != nil {
		t.Fatalf("writeFileAtomic() error = %v", err)
	}

	if got, _ := os.ReadFile(path); string(got) != "new" {
		t.Errorf("content = %q", got)
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o640 {
		t.Errorf("mode = %v, want 0640", info.Mode().Perm())
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("temporary files left behind: %s", strings.Join(names, ", "))
	}
}
