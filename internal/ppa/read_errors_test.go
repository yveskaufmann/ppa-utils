package ppa

import (
	"reflect"
	"testing"
)

func TestReadSourceFileErrors(t *testing.T) {
	for _, file := range []string{
		"unknown-field.sources",
		"orphan-continuation.sources",
		"invalid-line.sources",
	} {
		t.Run(file, func(t *testing.T) {
			if _, err := ReadSourceFile(testdataPath(file)); err == nil {
				t.Error("ReadSourceFile() expected error, got nil")
			}
		})
	}
}

func TestReadSourceFileIgnoresCustomFields(t *testing.T) {
	path := testdataPath("custom-field.sources")
	got, err := ReadSourceFile(path)
	if err != nil {
		t.Fatalf("ReadSourceFile() error = %v", err)
	}
	want := &SourceFile{
		Path:   path,
		Types:  []string{"deb"},
		URIs:   []string{"https://example.com"},
		Suites: []string{"noble"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ReadSourceFile() = %+v, want %+v", got, want)
	}
}

func TestParseArchitecture(t *testing.T) {
	tests := map[string]Architecture{
		"amd64":   ArchitectureAMD64,
		"arm64":   ArchitectureARM64,
		"i386":    ArchitectureI386,
		"unknown": ArchitectureAMD64,
		"":        ArchitectureAMD64,
	}
	for in, want := range tests {
		if got := ParseArchitecture(in); got != want {
			t.Errorf("ParseArchitecture(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseEnabledStatus(t *testing.T) {
	tests := map[string]EnabledStatus{
		"Yes":     EnabledStatusYes,
		"yes":     EnabledStatusYes,
		"No":      EnabledStatusNo,
		"no":      EnabledStatusNo,
		"unknown": EnabledStatusYes,
		"":        EnabledStatusYes,
	}
	for in, want := range tests {
		if got := ParseEnabledStatus(in); got != want {
			t.Errorf("ParseEnabledStatus(%q) = %q, want %q", in, got, want)
		}
	}
}
