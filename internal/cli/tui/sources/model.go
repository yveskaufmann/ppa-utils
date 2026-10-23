// Package sources implements the interactive, htop-like view of the apt sources.
package sources

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"yv35.com/ppa-utils/internal/cli/sourcelist"
)

// Config holds the dependencies of the view.
type Config struct {
	// Load returns the current rows.
	Load func() ([]sourcelist.Row, error)
	// Toggler changes the enabled state of a source file.
	Toggler Toggler
	// Filter is the initial text of the filter line.
	Filter string
}

type mode int

const (
	modeNormal mode = iota
	modeFilter
	modeConfirm
)

type rowsLoadedMsg struct {
	rows []sourcelist.Row
	err  error
}

type toggledMsg struct {
	path    string
	enabled bool
	err     error
}

// toggle is an enable or disable waiting for confirmation.
type toggle struct {
	row    sourcelist.Row
	enable bool
}

// Model is the Bubble Tea model of the view.
type Model struct {
	cfg Config

	rows []sourcelist.Row // all rows
	view []sourcelist.Row // rows after filtering and sorting

	cursor int
	offset int
	width  int
	height int

	sortCol  int
	sortDesc bool

	mode      mode
	filter    textinput.Model
	pending   *toggle
	status    string
	statusErr bool
	showHelp  bool
}

// New returns the model of the view.
func New(cfg Config) *Model {
	input := textinput.New()
	input.Prompt = "Filter: "
	input.Placeholder = "text or column:text (name, status, uri, suite, component, arch, signed-by)"
	input.SetValue(cfg.Filter)

	return &Model{
		cfg:    cfg,
		width:  100,
		height: 24,
		filter: input,
	}
}

// Init loads the rows.
func (m *Model) Init() tea.Cmd {
	return m.load()
}

func (m *Model) load() tea.Cmd {
	return func() tea.Msg {
		rows, err := m.cfg.Load()
		return rowsLoadedMsg{rows: rows, err: err}
	}
}

// Update handles messages and keys.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.clamp()

	case rowsLoadedMsg:
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("Loading failed: %v", msg.err), true)
			return m, nil
		}
		m.setRows(msg.rows)

	case toggledMsg:
		name := sourcelist.SourceName(msg.path)
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("%s: %v", name, msg.err), true)
			return m, nil
		}
		verb := "Disabled"
		if msg.enabled {
			verb = "Enabled"
		}
		m.setStatus(fmt.Sprintf("%s %s. Run 'sudo apt update' to apply the change.", verb, name), false)
		return m, m.load()

	case tea.KeyMsg:
		switch m.mode {
		case modeFilter:
			return m.updateFilter(msg)
		case modeConfirm:
			return m.updateConfirm(msg)
		default:
			return m.updateNormal(msg)
		}
	}
	return m, nil
}

func (m *Model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.showHelp {
		switch msg.String() {
		case "ctrl+c", "q", "f10":
			return m, tea.Quit
		}
		m.showHelp = false
		return m, nil
	}

	switch msg.String() {
	case "ctrl+c", "q", "f10":
		return m, tea.Quit
	case "up", "k":
		m.moveCursor(-1)
	case "down", "j":
		m.moveCursor(1)
	case "pgup":
		m.moveCursor(-m.bodyHeight())
	case "pgdown":
		m.moveCursor(m.bodyHeight())
	case "home", "g":
		m.moveCursor(-len(m.view))
	case "end", "G":
		m.moveCursor(len(m.view))
	case "/", "f4":
		m.mode = modeFilter
		m.filter.CursorEnd()
		return m, m.filter.Focus()
	case "esc":
		if m.filter.Value() != "" {
			m.filter.SetValue("")
			m.applyView(m.selectedPath())
		}
	case ">", ".", "f6":
		m.sortCol = (m.sortCol + 1) % len(columns)
		m.applyView(m.selectedPath())
	case "<", ",":
		m.sortCol = (m.sortCol + len(columns) - 1) % len(columns)
		m.applyView(m.selectedPath())
	case "I":
		m.sortDesc = !m.sortDesc
		m.applyView(m.selectedPath())
	case "e":
		m.requestToggle(true)
	case "d":
		m.requestToggle(false)
	case " ":
		if row, ok := m.selected(); ok {
			m.requestToggle(row.Status != sourcelist.StatusEnabled)
		}
	case "R", "f5":
		m.setStatus("Reloading…", false)
		return m, m.load()
	case "?", "f1":
		m.showHelp = true
	}
	return m, nil
}

func (m *Model) updateFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.mode = modeNormal
		m.filter.Blur()
		return m, nil
	case "esc":
		m.mode = modeNormal
		m.filter.Blur()
		m.filter.SetValue("")
		m.applyView(m.selectedPath())
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	}

	selected := m.selectedPath()
	var cmd tea.Cmd
	m.filter, cmd = m.filter.Update(msg)
	m.applyView(selected)
	return m, cmd
}

func (m *Model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	pending := m.pending
	m.mode = modeNormal
	m.pending = nil

	switch msg.String() {
	case "y", "Y", "enter":
		m.setStatus("Working…", false)
		return m, m.cfg.Toggler.SetEnabled(pending.row.Path, pending.enable)
	case "ctrl+c":
		return m, tea.Quit
	}
	m.setStatus("Cancelled", false)
	return m, nil
}

// requestToggle asks for confirmation to enable or disable the selected source.
func (m *Model) requestToggle(enable bool) {
	row, ok := m.selected()
	if !ok {
		return
	}

	switch {
	case enable && row.Status == sourcelist.StatusEnabled:
		m.setStatus(row.Name+" is already enabled", false)
	case !enable && row.Status == sourcelist.StatusDisabled:
		m.setStatus(row.Name+" is already disabled", false)
	case !enable && row.Official:
		m.setStatus("Official Ubuntu sources cannot be disabled", true)
	default:
		m.mode = modeConfirm
		m.pending = &toggle{row: row, enable: enable}
	}
}

func (m *Model) setStatus(text string, isErr bool) {
	m.status, m.statusErr = text, isErr
}

func (m *Model) setRows(rows []sourcelist.Row) {
	selected := m.selectedPath()
	m.rows = rows
	m.applyView(selected)
}

// applyView filters and sorts the rows and keeps the selection on the row of
// the source file at selectedPath, if it is still shown.
func (m *Model) applyView(selectedPath string) {
	m.view = applyFilter(m.rows, m.filter.Value())
	sortRows(m.view, m.sortCol, m.sortDesc)

	m.cursor = 0
	for i, row := range m.view {
		if selectedPath != "" && row.Path == selectedPath {
			m.cursor = i
			break
		}
	}
	m.clamp()
}

func (m *Model) selected() (sourcelist.Row, bool) {
	if m.cursor < 0 || m.cursor >= len(m.view) {
		return sourcelist.Row{}, false
	}
	return m.view[m.cursor], true
}

func (m *Model) selectedPath() string {
	row, _ := m.selected()
	return row.Path
}

func (m *Model) moveCursor(delta int) {
	m.cursor += delta
	m.clamp()
}

// bodyHeight is the number of table rows that fit on the screen.
func (m *Model) bodyHeight() int {
	return max(m.height-4, 1) // title, column header, status line, key bar
}

// clamp keeps the cursor inside the rows and scrolls to keep it visible.
func (m *Model) clamp() {
	m.cursor = max(min(m.cursor, len(m.view)-1), 0)

	body := m.bodyHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+body {
		m.offset = m.cursor - body + 1
	}
	m.offset = max(min(m.offset, len(m.view)-body), 0)
}
