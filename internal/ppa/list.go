package ppa

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

const DEFAULT_SOURCE_FILE_PATH = "/etc/apt/sources.list.d"

func ListSourceFilePaths() []string {

	entries, err := os.ReadDir(DEFAULT_SOURCE_FILE_PATH)
	if err != nil {
		slog.Error("Failed to read source file directory:", err)
		return []string{}
	}

	// If the directory is empty, return the default source file path
	if len(entries) == 0 {
		return []string{}
	}

	var paths []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if !strings.HasSuffix(entry.Name(), "sources") {
			continue
		}

		srcPath := filepath.Join(DEFAULT_SOURCE_FILE_PATH, entry.Name())
		paths = append(paths, srcPath)
	}
	return paths
}

func ListSourceFiles() ([]SourceFile, error) {
	paths := ListSourceFilePaths()
	var sourceFiles []SourceFile
	for _, path := range paths {
		sf, err := ReadSourceFile(path)
		if err != nil {
			return nil, err
		}
		sourceFiles = append(sourceFiles, *sf)
	}
	return sourceFiles, nil
}
