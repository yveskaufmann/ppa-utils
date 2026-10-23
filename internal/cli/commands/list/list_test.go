package list

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"yv35.com/ppa/ppa-utils/internal/cli/sourcelist"
	"yv35.com/ppa/ppa-utils/internal/cli/tui/sources"
	"yv35.com/ppa/ppa-utils/internal/ppa"
)

func testRows() []sourcelist.Row {
	return sourcelist.ToRows([]ppa.SourceFile{
		{
			Path:          "/etc/apt/sources.list.d/docker.sources",
			URIs:          []string{"https://download.docker.com/linux/ubuntu/"},
			Suites:        []string{"resolute"},
			Components:    []string{"stable"},
			Architectures: []ppa.Architecture{ppa.ArchitectureAMD64},
			SignedBy:      "/etc/apt/keyrings/docker.asc",
			Enabled:       ppa.EnabledStatusYes,
		},
		{
			Path:       "/etc/apt/sources.list.d/kisak-mesa.sources",
			URIs:       []string{"https://ppa.launchpadcontent.net/kisak/kisak-mesa/ubuntu/"},
			Suites:     []string{"noble", "noble-updates"},
			Components: []string{"main", "universe"},
			SignedBy:   "-----BEGIN PGP PUBLIC KEY BLOCK-----\n\nabc\n-----END PGP PUBLIC KEY BLOCK-----",
			Enabled:    ppa.EnabledStatusNo,
		},
	})
}

func TestWriteJSON(t *testing.T) {
	var out bytes.Buffer
	if err := writeJSON(&out, testRows()); err != nil {
		t.Fatalf("writeJSON() error = %v", err)
	}

	var got []sourcelist.Row
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out.String())
	}
	if len(got) != 2 || got[1].SignedBy != "inline-key" {
		t.Errorf("unexpected rows: %+v", got)
	}
}

func TestWriteJSONEmptyIsArray(t *testing.T) {
	var out bytes.Buffer
	if err := writeJSON(&out, sourcelist.ToRows(nil)); err != nil {
		t.Fatalf("writeJSON() error = %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != "[]" {
		t.Errorf("output = %q, want []", got)
	}
}

func TestWriteTable(t *testing.T) {
	var out bytes.Buffer
	if err := writeTable(&out, testRows()); err != nil {
		t.Fatalf("writeTable() error = %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want header and 2 rows:\n%s", len(lines), out.String())
	}
	for _, header := range []string{"NAME", "STATUS", "URI", "SUITES", "COMPONENTS", "ARCH", "SIGNED BY"} {
		if !strings.Contains(lines[0], header) {
			t.Errorf("header %q missing: %s", header, lines[0])
		}
	}
	for _, want := range []string{"kisak-mesa", "disabled", "noble, noble-updates", "main, universe", "inline-key"} {
		if !strings.Contains(lines[2], want) {
			t.Errorf("row missing %q: %s", want, lines[2])
		}
	}
	// A source without architectures shows a placeholder in that column.
	if !strings.Contains(lines[2], " -  ") {
		t.Errorf("empty architecture should be shown as '-': %s", lines[2])
	}
}

func TestNewPassesOptionsToFn(t *testing.T) {
	var got *Options
	cmd := New(Deps{}, func(opts *Options) error {
		got = opts
		return nil
	})
	cmd.SetArgs([]string{"--name", "docker", "--uri", "example", "--suite", "noble", "--component", "main", "--arch", "arm64", "--signed-by", "keyrings", "--status", "disabled", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got == nil {
		t.Fatal("fn was not called")
	}
	want := sourcelist.Filter{Name: "docker", URI: "example", Suite: "noble", Component: "main", Arch: "arm64", SignedBy: "keyrings", Status: "disabled"}
	if got.Filter != want {
		t.Errorf("Filter = %+v, want %+v", got.Filter, want)
	}
	if !got.JSON {
		t.Error("JSON should be set")
	}
	if got.Interactive {
		t.Error("Interactive should not be set")
	}
	if got.Out == nil || got.ErrOut == nil {
		t.Error("Out and ErrOut should be set")
	}
}

func TestListCommandRejectsArgs(t *testing.T) {
	cmd := New(Deps{}, func(*Options) error { return nil })
	cmd.SetArgs([]string{"extra"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	if err := cmd.Execute(); err == nil {
		t.Error("Execute() expected error for extra args, got nil")
	}
}

func fakeDeps(terminal bool, tuiCalls *[]sources.Config) Deps {
	return Deps{
		Lister: func() ([]ppa.SourceFile, error) {
			return []ppa.SourceFile{
				{Path: "/etc/apt/sources.list.d/a.sources", URIs: []string{"https://a.example.com"}, Suites: []string{"noble"}},
				{Path: "/etc/apt/sources.list.d/b.sources", URIs: []string{"https://b.example.com"}, Suites: []string{"resolute"}},
			}, nil
		},
		IsTerminal: func(io.Writer) bool { return terminal },
		RunTUI: func(cfg sources.Config) error {
			*tuiCalls = append(*tuiCalls, cfg)
			return nil
		},
	}
}

func execute(t *testing.T, deps Deps, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var out, errOut bytes.Buffer
	cmd := New(deps, nil)
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	err = cmd.Execute()
	return out.String(), errOut.String(), err
}

func TestRunPrintsFilteredTable(t *testing.T) {
	var tui []sources.Config
	out, _, err := execute(t, fakeDeps(false, &tui), "--suite", "resolute")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "a.example.com") || !strings.Contains(out, "b.example.com") {
		t.Errorf("unexpected output:\n%s", out)
	}
	if len(tui) != 0 {
		t.Error("the interactive view must not start without --interactive")
	}
}

func TestRunPrintsJSON(t *testing.T) {
	var tui []sources.Config
	out, _, err := execute(t, fakeDeps(false, &tui), "--json", "--name", "a")
	if err != nil {
		t.Fatal(err)
	}
	var rows []sourcelist.Row
	if err := json.Unmarshal([]byte(out), &rows); err != nil || len(rows) != 1 || rows[0].Name != "a" {
		t.Errorf("rows = %+v, err = %v\n%s", rows, err, out)
	}
}

func TestRunInteractiveStartsView(t *testing.T) {
	var tui []sources.Config
	out, _, err := execute(t, fakeDeps(true, &tui), "-i", "--suite", "noble", "--status", "enabled")
	if err != nil {
		t.Fatal(err)
	}
	if out != "" || len(tui) != 1 {
		t.Fatalf("output = %q, view started %d times", out, len(tui))
	}
	if tui[0].Filter != "status:enabled suite:noble" {
		t.Errorf("initial filter = %q", tui[0].Filter)
	}
	rows, err := tui[0].Load()
	if err != nil || len(rows) != 2 {
		t.Errorf("Load() = %v, %v", rows, err)
	}
}

func TestRunInteractiveFallsBackWithoutTerminal(t *testing.T) {
	var tui []sources.Config
	out, errOut, err := execute(t, fakeDeps(false, &tui), "--interactive")
	if err != nil {
		t.Fatal(err)
	}
	if len(tui) != 0 || !strings.Contains(out, "NAME") {
		t.Errorf("expected the plain table, got:\n%s", out)
	}
	if !strings.Contains(errOut, "not a terminal") {
		t.Errorf("missing hint on stderr: %q", errOut)
	}
}

func TestRunInteractiveRejectsJSON(t *testing.T) {
	var tui []sources.Config
	if _, _, err := execute(t, fakeDeps(true, &tui), "-i", "--json"); err == nil {
		t.Error("expected an error for --interactive with --json")
	}
}

func TestRunReportsListingError(t *testing.T) {
	deps := fakeDeps(false, new([]sources.Config))
	deps.Lister = func() ([]ppa.SourceFile, error) { return nil, errors.New("boom") }
	if _, _, err := execute(t, deps); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Errorf("err = %v", err)
	}
}
