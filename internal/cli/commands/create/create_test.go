package create

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yv35.com/ppa-utils/internal/ppa"
)

func TestParseListLine(t *testing.T) {
	got, err := ppa.ParseListLine("deb [arch=amd64,arm64 signed-by=/usr/share/keyrings/claude-desktop-archive-keyring.asc] https://downloads.claude.ai/claude-desktop/apt/stable stable main")
	if err != nil {
		t.Fatalf("ParseListLine() error = %v", err)
	}
	if len(got.Types) != 1 || got.Types[0] != "deb" {
		t.Fatalf("Types = %v, want [deb]", got.Types)
	}
	if got.URIs[0] != "https://downloads.claude.ai/claude-desktop/apt/stable" {
		t.Fatalf("URIs = %v, want [https://downloads.claude.ai/claude-desktop/apt/stable]", got.URIs)
	}
	if got.Suites[0] != "stable" {
		t.Fatalf("Suites = %v, want [stable]", got.Suites)
	}
	if got.Components[0] != "main" {
		t.Fatalf("Components = %v, want [main]", got.Components)
	}
	if len(got.Architectures) != 2 || got.Architectures[0] != ppa.ArchitectureAMD64 || got.Architectures[1] != ppa.ArchitectureARM64 {
		t.Fatalf("Architectures = %v, want [amd64 arm64]", got.Architectures)
	}
	if got.SignedBy != "/usr/share/keyrings/claude-desktop-archive-keyring.asc" {
		t.Fatalf("SignedBy = %q, want %q", got.SignedBy, "/usr/share/keyrings/claude-desktop-archive-keyring.asc")
	}
}

func TestCommandWriteFromString(t *testing.T) {
	keyPath := filepath.Join(t.TempDir(), "claude-desktop-archive-keyring.asc")
	if err := os.WriteFile(keyPath, []byte("key-data"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	out := filepath.Join(t.TempDir(), "claude.sources")
	cmd := New(nil)
	cmd.SetArgs([]string{"--output", out, "--from-list-entry", "deb [arch=amd64,arm64 signed-by=" + keyPath + "] https://downloads.claude.ai/claude-desktop/apt/stable stable main"})
	cmd.SetOut(os.Stdout)
	cmd.SetErr(os.Stderr)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	content, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	text := string(content)
	if !strings.Contains(text, "Types: deb") || !strings.Contains(text, "URIs: https://downloads.claude.ai/claude-desktop/apt/stable") {
		t.Fatalf("source file content = %q, want deb source stanza", text)
	}
	if !strings.Contains(text, "Suites: stable") || !strings.Contains(text, "Components: main") {
		t.Fatalf("source file content = %q, want stable/main fields", text)
	}
	if !strings.Contains(text, "Architectures: amd64, arm64") {
		t.Fatalf("source file content = %q, want architecture list", text)
	}
	if !strings.Contains(text, "Signed-By: "+keyPath) {
		t.Fatalf("source file content = %q, want signed-by path %q", text, keyPath)
	}
}

func TestCommandWriteFromListFile(t *testing.T) {
	inputDir := t.TempDir()
	keyPath := filepath.Join(inputDir, "claude-desktop-archive-keyring.asc")
	if err := os.WriteFile(keyPath, []byte("key-data"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	inputPath := filepath.Join(inputDir, "claude.list")
	if err := os.WriteFile(inputPath, []byte("deb [arch=amd64 signed-by="+keyPath+"] https://downloads.claude.ai/claude-desktop/apt/stable stable main\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	outputPath := filepath.Join(inputDir, "claude.sources")
	cmd := New(nil)
	cmd.SetArgs([]string{"--output", outputPath, "--from-list-file", inputPath})
	cmd.SetOut(os.Stdout)
	cmd.SetErr(os.Stderr)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), "Architectures: amd64") {
		t.Fatalf("output = %q, want amd64 architecture", string(data))
	}
}

func TestCommandDryRunDoesNotWriteAndReportsPlan(t *testing.T) {
	inputDir := t.TempDir()
	outPath := filepath.Join(inputDir, "claude.sources")
	if err := os.WriteFile(outPath, []byte("old\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cmd := New(nil)
	cmd.SetArgs([]string{"--dry", "--output", outPath, "--from-list-entry", "deb [arch=amd64] https://downloads.claude.ai/claude-desktop/apt/stable stable main"})
	var stdout, stderr strings.Builder
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("output file should still exist in dry-run mode: %v", err)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if got := string(data); got != "old\n" {
		t.Fatalf("dry run modified existing file: got %q, want %q", got, "old\n")
	}
	if !strings.Contains(stdout.String(), "DRY RUN") || !strings.Contains(stdout.String(), "Would overwrite") {
		t.Fatalf("dry run output = %q, want DRY RUN and overwrite message", stdout.String())
	}
	if strings.Contains(stdout.String(), "Created ") {
		t.Fatalf("dry run should not claim file creation: %q", stdout.String())
	}
}

func TestPromptSourceAsksForFileNameOnly(t *testing.T) {
	opts := &Options{
		In:  strings.NewReader(""),
		Out: &strings.Builder{},
		AskPrompt: func(label, defaultValue string) string {
			if label == "URI" {
				return "https://example.com/repo"
			}
			if label == "Suite" {
				return "stable"
			}
			if label == "Components" {
				return "main"
			}
			if label == "Architectures" {
				return "amd64"
			}
			if label == "Signed-By (leave empty to skip)" {
				return ""
			}
			if label == "Source file name" {
				return "repo.sources"
			}
			return defaultValue
		},
	}

	source, err := promptSource(opts)
	if err != nil {
		t.Fatalf("promptSource() error = %v", err)
	}
	if source == nil {
		t.Fatal("promptSource() = nil source")
	}
	wantPath := filepath.Join(ppa.DEFAULT_SOURCE_FILE_PATH, "repo.sources")
	if opts.Output != wantPath {
		t.Fatalf("opts.Output = %q, want %q", opts.Output, wantPath)
	}
	if strings.Contains(opts.Out.(*strings.Builder).String(), filepath.Join("/", "tmp")) {
		t.Fatalf("prompt output should ask for a filename, not a full path: %q", opts.Out.(*strings.Builder).String())
	}
}

func TestCommandNonInteractiveFlagsOverrideParsedValues(t *testing.T) {
	keyPath := filepath.Join(t.TempDir(), "cli-key.asc")
	if err := os.WriteFile(keyPath, []byte("key-data"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	outPath := filepath.Join(t.TempDir(), "cli-created.sources")
	cmd := New(nil)
	cmd.SetArgs([]string{
		"--output", outPath,
		"--from-list-entry", "deb [arch=amd64] https://downloads.claude.ai/claude-desktop/apt/stable stable main",
		"--types", "deb",
		"--uri", "https://example.com/repo",
		"--suite", "noble",
		"--components", "main universe",
		"--architectures", "amd64,arm64",
		"--signed-by", keyPath,
	})
	cmd.SetOut(os.Stdout)
	cmd.SetErr(os.Stderr)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	content, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	text := string(content)
	if !strings.Contains(text, "URIs: https://example.com/repo") {
		t.Fatalf("output = %q, want overridden URI", text)
	}
	if !strings.Contains(text, "Suites: noble") {
		t.Fatalf("output = %q, want overridden suite", text)
	}
	if !strings.Contains(text, "Components: main universe") {
		t.Fatalf("output = %q, want overridden components", text)
	}
	if !strings.Contains(text, "Architectures: amd64, arm64") {
		t.Fatalf("output = %q, want overridden architectures", text)
	}
	if !strings.Contains(text, "Signed-By: "+keyPath) {
		t.Fatalf("output = %q, want overridden signed-by path %q", text, keyPath)
	}
}

func TestPromptSourceUsesExplicitFlagsAsDefaults(t *testing.T) {
	opts := &Options{
		URI:           "https://example.com/repo",
		Suite:         "noble",
		Components:    "main universe",
		Architectures: "amd64,arm64",
		SignedBy:      "/tmp/default-key.asc",
		In:            strings.NewReader(""),
		Out:           &strings.Builder{},
	}
	opts.AskPrompt = func(label, defaultValue string) string {
		switch label {
		case "URI":
			if defaultValue != opts.URI {
				t.Fatalf("URI default = %q, want %q", defaultValue, opts.URI)
			}
			return defaultValue
		case "Suite":
			if defaultValue != opts.Suite {
				t.Fatalf("Suite default = %q, want %q", defaultValue, opts.Suite)
			}
			return defaultValue
		case "Components":
			if defaultValue != opts.Components {
				t.Fatalf("Components default = %q, want %q", defaultValue, opts.Components)
			}
			return defaultValue
		case "Architectures":
			if defaultValue != opts.Architectures {
				t.Fatalf("Architectures default = %q, want %q", defaultValue, opts.Architectures)
			}
			return defaultValue
		case "Signed-By (leave empty to skip)":
			if defaultValue != opts.SignedBy {
				t.Fatalf("SignedBy default = %q, want %q", defaultValue, opts.SignedBy)
			}
			return defaultValue
		default:
			return defaultValue
		}
	}

	source, err := promptSource(opts)
	if err != nil {
		t.Fatalf("promptSource() error = %v", err)
	}
	if source.URIs[0] != opts.URI {
		t.Fatalf("source.URIs[0] = %q, want %q", source.URIs[0], opts.URI)
	}
	if source.Suites[0] != opts.Suite {
		t.Fatalf("source.Suites[0] = %q, want %q", source.Suites[0], opts.Suite)
	}
	if len(source.Components) != 2 || source.Components[0] != "main" || source.Components[1] != "universe" {
		t.Fatalf("source.Components = %v, want [main universe]", source.Components)
	}
	if len(source.Architectures) != 2 {
		t.Fatalf("source.Architectures = %v, want 2 values", source.Architectures)
	}
	if source.SignedBy != opts.SignedBy {
		t.Fatalf("source.SignedBy = %q, want %q", source.SignedBy, opts.SignedBy)
	}
}

func TestDeriveSourceNameFromURI(t *testing.T) {
	tests := []struct {
		uri  string
		want string
	}{
		{uri: "https://downloads.claude.ai/claude-desktop/apt/stable", want: "claude-desktop"},
		{uri: "https://ppa.launchpadcontent.net/kisak/kisak-mesa/ubuntu", want: "kisak-mesa"},
		{uri: "https://apt.releases.hashicorp.com", want: "hashicorp"},
		{uri: "http://de.archive.ubuntu.com/ubuntu", want: "ubuntu"},
		{uri: "https://repo.steampowered.com/steam", want: "steam"},
		{uri: "https://packages.microsoft.com/repos/code", want: "code"},
	}
	for _, tt := range tests {
		if got := deriveSourceNameFromURI(tt.uri); got != tt.want {
			t.Fatalf("deriveSourceNameFromURI(%q) = %q, want %q", tt.uri, got, tt.want)
		}
	}
}

func TestCreateCommandCompletionSuggestions(t *testing.T) {
	if got := completeSourceTypes(); len(got) == 0 || got[0] != "deb" {
		t.Fatalf("completeSourceTypes() = %v, want deb first", got)
	}
	if got := completeArchitectures(); len(got) == 0 || !contains(got, string(ppa.ArchitectureAMD64)) {
		t.Fatalf("completeArchitectures() = %v, want amd64", got)
	}
	if got := completeReleaseCodes(); len(got) == 0 {
		t.Fatal("completeReleaseCodes() returned no suggestions")
	}
}

func TestSudoCommandBlockIsIndentedAndSeparated(t *testing.T) {
	oldArgs := os.Args
	os.Args = []string{"/tmp/ppa-utils", "create", "--from-list-entry", "deb [arch=amd64] https://example.com/repo stable main"}
	defer func() { os.Args = oldArgs }()

	got := sudoCommandBlock()
	if !strings.HasPrefix(got, "\n\n    sudo ") {
		t.Fatalf("sudoCommandBlock() = %q, want empty line before indented sudo command", got)
	}
	if !strings.HasSuffix(got, "\n\n") {
		t.Fatalf("sudoCommandBlock() = %q, want trailing empty line after the sudo command", got)
	}
	if !strings.Contains(got, "--from-list-entry") {
		t.Fatalf("sudoCommandBlock() = %q, want original args preserved in retry command", got)
	}
}

func TestCommandFailsWhenSignedByMissingAndNotInteractive(t *testing.T) {
	cmd := New(nil)
	cmd.SetArgs([]string{"--output", filepath.Join(t.TempDir(), "claude.sources"), "--from-list-entry", "deb [arch=amd64 signed-by=/tmp/definitely-missing-key.asc] https://example.com/repo stable main"})
	cmd.SetOut(os.Stdout)
	cmd.SetErr(os.Stderr)

	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute() expected error for missing signed-by file, got nil")
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
