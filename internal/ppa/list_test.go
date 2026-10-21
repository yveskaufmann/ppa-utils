package ppa

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListSourceFilePaths(t *testing.T) {
	if _, err := os.Stat(DEFAULT_SOURCE_FILE_PATH); err != nil {
		t.Skipf("%s not available: %v", DEFAULT_SOURCE_FILE_PATH, err)
	}

	for _, p := range ListSourceFilePaths() {
		if filepath.Dir(p) != DEFAULT_SOURCE_FILE_PATH {
			t.Errorf("%s is not directly inside %s", p, DEFAULT_SOURCE_FILE_PATH)
		}
		if !strings.HasSuffix(p, "sources") {
			t.Errorf("%s does not end in \"sources\"", p)
		}
	}
}
