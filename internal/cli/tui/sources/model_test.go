package sources

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"yv35.com/ppa-utils/internal/cli/sourcelist"
)

func testRows() []sourcelist.Row {
	return []sourcelist.Row{
		{Name: "docker", Path: "/etc/apt/sources.list.d/docker.sources", URIs: []string{"https://download.docker.com/linux/ubuntu/"}, Suites: []string{"resolute"}, Components: []string{"stable"}, Architectures: []string{"amd64"}, SignedBy: "/etc/apt/keyrings/docker.asc", Status: sourcelist.StatusEnabled},
		{Name: "kisak", Path: "/etc/apt/sources.list.d/kisak.sources", URIs: []string{"https://ppa.launchpadcontent.net/kisak/mesa/ubuntu/"}, Suites: []string{"noble"}, Components: []string{"main"}, Architectures: []string{}, SignedBy: "inline-key", Status: sourcelist.StatusDisabled},
		{Name: "ubuntu", Path: "/etc/apt/sources.list.d/ubuntu.sources", URIs: []string{"http://archive.ubuntu.com/ubuntu"}, Suites: []string{"noble"}, Components: []string{"main"}, Architectures: []string{}, SignedBy: "/usr/share/keyrings/ubuntu.gpg", Status: sourcelist.StatusEnabled, Official: true},
	}
}

type call struct {
	path    string
	enabled bool
}

type fakeToggler struct{ calls []call }

func (f *fakeToggler) SetEnabled(path string, enabled bool) tea.Cmd {
	f.calls = append(f.calls, call{path, enabled})
	return func() tea.Msg { return toggledMsg{path: path, enabled: enabled} }
}

func newTestModel(t *testing.T, filter string) (*Model, *fakeToggler) {
	t.Helper()
	toggler := &fakeToggler{}
	m := New(Config{
		Load:    func() ([]sourcelist.Row, error) { return testRows(), nil },
		Toggler: toggler,
		Filter:  filter,
	})
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 20})
	m.Update(m.Init()())
	return m, toggler
}

func key(s string) tea.KeyMsg {
	named := map[string]tea.KeyType{
		"up": tea.KeyUp, "down": tea.KeyDown, "enter": tea.KeyEnter, "esc": tea.KeyEsc,
		"f4": tea.KeyF4, "f6": tea.KeyF6, "home": tea.KeyHome, "end": tea.KeyEnd,
	}
	if t, ok := named[s]; ok {
		return tea.KeyMsg{Type: t}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func press(m *Model, keys ...string) tea.Cmd {
	var cmd tea.Cmd
	for _, k := range keys {
		_, cmd = m.Update(key(k))
	}
	return cmd
}

func names(m *Model) []string {
	var out []string
	for _, row := range m.view {
		out = append(out, row.Name)
	}
	return out
}

func equal(a, b []string) bool { return strings.Join(a, ",") == strings.Join(b, ",") }

func TestInitialRowsAreSortedByName(t *testing.T) {
	m, _ := newTestModel(t, "")
	if got := names(m); !equal(got, []string{"docker", "kisak", "ubuntu"}) {
		t.Errorf("rows = %v", got)
	}
}

func TestCursorMovementIsClamped(t *testing.T) {
	m, _ := newTestModel(t, "")

	press(m, "up")
	if m.cursor != 0 {
		t.Errorf("cursor = %d after up at the top, want 0", m.cursor)
	}
	press(m, "down", "down", "down", "down")
	if m.cursor != 2 {
		t.Errorf("cursor = %d at the bottom, want 2", m.cursor)
	}
	press(m, "home")
	if m.cursor != 0 {
		t.Errorf("cursor = %d after home, want 0", m.cursor)
	}
	press(m, "end")
	if m.cursor != 2 {
		t.Errorf("cursor = %d after end, want 2", m.cursor)
	}
}

func TestFilterIsLiveAndSupportsColumns(t *testing.T) {
	tests := []struct {
		text string
		want []string
	}{
		{"kis", []string{"kisak"}},
		{"suite:noble", []string{"kisak", "ubuntu"}},
		{"suite:noble status:disabled", []string{"kisak"}},
		{"docker amd64", []string{"docker"}},
		{"docker noble", nil},
		{"unknown:thing", nil},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			m, _ := newTestModel(t, "")
			press(m, "/")
			for _, r := range tt.text {
				press(m, string(r))
			}
			if got := names(m); !equal(got, tt.want) {
				t.Errorf("rows = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterEnterKeepsAndEscClears(t *testing.T) {
	m, _ := newTestModel(t, "")
	press(m, "/", "k", "i", "s", "enter")
	if m.mode != modeNormal || !equal(names(m), []string{"kisak"}) {
		t.Fatalf("mode = %v, rows = %v", m.mode, names(m))
	}

	press(m, "esc")
	if len(m.view) != 3 {
		t.Errorf("esc should clear the filter, got %v", names(m))
	}

	press(m, "/", "d", "o", "esc")
	if m.mode != modeNormal || len(m.view) != 3 || m.filter.Value() != "" {
		t.Errorf("esc in the filter line should clear it, got mode %v rows %v", m.mode, names(m))
	}
}

func TestInitialFilterFromConfig(t *testing.T) {
	m, _ := newTestModel(t, "status:enabled")
	if got := names(m); !equal(got, []string{"docker", "ubuntu"}) {
		t.Errorf("rows = %v", got)
	}
}

func TestSortColumnAndDirection(t *testing.T) {
	m, _ := newTestModel(t, "")

	press(m, "I")
	if got := names(m); !equal(got, []string{"ubuntu", "kisak", "docker"}) {
		t.Errorf("descending by name = %v", got)
	}

	press(m, "I", "f6") // next column: STATUS ascending
	if got := names(m); !equal(got, []string{"kisak", "docker", "ubuntu"}) {
		t.Errorf("by status = %v", got)
	}

	press(m, "<", "<") // wraps around to the last column: SIGNED BY
	if m.sortCol != len(columns)-1 {
		t.Errorf("sortCol = %d, want %d", m.sortCol, len(columns)-1)
	}
}

func TestSelectionFollowsRowWhenSorting(t *testing.T) {
	m, _ := newTestModel(t, "")
	press(m, "down") // kisak
	press(m, "I")
	if row, _ := m.selected(); row.Name != "kisak" {
		t.Errorf("selected = %q, want kisak", row.Name)
	}
}

func TestDisableAsksForConfirmation(t *testing.T) {
	m, toggler := newTestModel(t, "")

	press(m, "d") // docker, enabled
	if m.mode != modeConfirm || m.pending == nil || m.pending.enable {
		t.Fatalf("expected a pending disable, mode = %v", m.mode)
	}
	if len(toggler.calls) != 0 {
		t.Fatal("nothing must happen before the confirmation")
	}
	if !strings.Contains(m.View(), "Disable docker") {
		t.Errorf("confirmation prompt missing:\n%s", m.View())
	}

	cmd := press(m, "y")
	if len(toggler.calls) != 1 || toggler.calls[0] != (call{"/etc/apt/sources.list.d/docker.sources", false}) {
		t.Fatalf("calls = %+v", toggler.calls)
	}

	// The result of the toggle shows a hint and reloads the rows.
	_, reload := m.Update(cmd())
	if !strings.Contains(m.status, "Disabled docker") || !strings.Contains(m.status, "apt update") {
		t.Errorf("status = %q", m.status)
	}
	if reload == nil {
		t.Error("a successful toggle should reload the rows")
	}
}

func TestConfirmationCanBeCancelled(t *testing.T) {
	m, toggler := newTestModel(t, "")
	press(m, "d", "n")
	if m.mode != modeNormal || len(toggler.calls) != 0 || m.status != "Cancelled" {
		t.Errorf("mode = %v, calls = %v, status = %q", m.mode, toggler.calls, m.status)
	}
}

func TestEnableAndToggle(t *testing.T) {
	m, toggler := newTestModel(t, "")

	press(m, "down", "e") // kisak is disabled
	press(m, "y")
	press(m, "up", " ", "y") // docker is enabled, space disables it

	want := []call{
		{"/etc/apt/sources.list.d/kisak.sources", true},
		{"/etc/apt/sources.list.d/docker.sources", false},
	}
	if len(toggler.calls) != 2 || toggler.calls[0] != want[0] || toggler.calls[1] != want[1] {
		t.Errorf("calls = %+v, want %+v", toggler.calls, want)
	}
}

func TestNoOpToggleIsReported(t *testing.T) {
	m, toggler := newTestModel(t, "")
	press(m, "e") // docker is already enabled
	if m.mode != modeNormal || len(toggler.calls) != 0 || !strings.Contains(m.status, "already enabled") {
		t.Errorf("mode = %v, status = %q", m.mode, m.status)
	}
}

func TestOfficialSourceCannotBeDisabled(t *testing.T) {
	m, toggler := newTestModel(t, "")
	press(m, "end") // ubuntu
	press(m, "d")

	if m.mode != modeNormal || len(toggler.calls) != 0 {
		t.Fatalf("mode = %v, calls = %v", m.mode, toggler.calls)
	}
	if !m.statusErr || !strings.Contains(m.status, "cannot be disabled") {
		t.Errorf("status = %q", m.status)
	}

	press(m, " ") // space would disable it as well
	if m.mode != modeNormal || len(toggler.calls) != 0 {
		t.Errorf("space must not disable an official source, mode = %v", m.mode)
	}
}

func TestToggleErrorIsShown(t *testing.T) {
	m, _ := newTestModel(t, "")
	_, cmd := m.Update(toggledMsg{path: "/etc/apt/sources.list.d/docker.sources", err: errors.New("boom")})
	if cmd != nil || !m.statusErr || !strings.Contains(m.status, "boom") {
		t.Errorf("status = %q, err = %v", m.status, m.statusErr)
	}
}

func TestLoadErrorIsShown(t *testing.T) {
	m := New(Config{Load: func() ([]sourcelist.Row, error) { return nil, errors.New("no access") }})
	m.Update(m.Init()())
	if !m.statusErr || !strings.Contains(m.status, "no access") {
		t.Errorf("status = %q", m.status)
	}
}

func TestQuit(t *testing.T) {
	m, _ := newTestModel(t, "")
	cmd := press(m, "q")
	if cmd == nil {
		t.Fatal("q should quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("q returned %T, want tea.QuitMsg", cmd())
	}
}

func TestViewShowsTableAndFitsScreen(t *testing.T) {
	m, _ := newTestModel(t, "")
	view := m.View()

	for _, want := range []string{"NAME▲", "STATUS", "URI", "SIGNED BY", "docker", "inline-key", "* ubuntu", "F10Quit", "3 of 3 sources"} {
		if !strings.Contains(view, want) {
			t.Errorf("view is missing %q:\n%s", want, view)
		}
	}
	if lines := strings.Count(view, "\n") + 1; lines != m.height {
		t.Errorf("view has %d lines, want %d", lines, m.height)
	}
}

func TestViewScrollsWithCursor(t *testing.T) {
	var rows []sourcelist.Row
	for i := 0; i < 30; i++ {
		rows = append(rows, sourcelist.Row{Name: string(rune('a'+i%26)) + string(rune('a'+i/26)), Path: "/p/" + string(rune('A'+i)), Status: sourcelist.StatusEnabled})
	}
	m := New(Config{Load: func() ([]sourcelist.Row, error) { return rows, nil }})
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 10})
	m.Update(m.Init()())

	press(m, "end")
	if m.cursor != 29 || m.offset != 29-m.bodyHeight()+1 {
		t.Errorf("cursor = %d, offset = %d", m.cursor, m.offset)
	}
	if strings.Count(m.View(), "\n")+1 != 10 {
		t.Error("view must keep the terminal height while scrolling")
	}
}

func TestHelpIsShownUntilNextKey(t *testing.T) {
	m, _ := newTestModel(t, "")
	press(m, "?")
	if !strings.Contains(m.View(), "official Ubuntu sources") {
		t.Errorf("help missing:\n%s", m.View())
	}
	press(m, "x")
	if m.showHelp {
		t.Error("any key should close the help")
	}
}

func TestParseFilterAndFormatRoundTrip(t *testing.T) {
	filter, terms := ParseFilter("suite:noble Foo status:disabled uri:x:y")
	if filter.Suite != "noble" || filter.Status != "disabled" || filter.URI != "x:y" {
		t.Errorf("filter = %+v", filter)
	}
	if len(terms) != 1 || terms[0] != "foo" {
		t.Errorf("terms = %v", terms)
	}
	if got := FormatFilter(filter); got != "status:disabled uri:x:y suite:noble" {
		t.Errorf("FormatFilter() = %q", got)
	}
}
