package sources

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"yv35.com/ppa-utils/internal/cli/sourcelist"
)

var (
	headerStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(lipgloss.Color("2"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("6"))
	disabledStyle = lipgloss.NewStyle().Faint(true)
	errorStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("1"))
	statusStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	keyStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	labelStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("6"))
)

// marker column in front of every row; "*" marks official Ubuntu sources.
const markerWidth = 2

// View renders the screen.
func (m *Model) View() string {
	var lines []string
	lines = append(lines, m.titleLine())

	if m.showHelp {
		lines = append(lines, m.helpLines()...)
	} else {
		lines = append(lines, m.tableLines()...)
	}

	// Pad so that the status line and the key bar stay at the bottom.
	for len(lines) < m.height-2 {
		lines = append(lines, "")
	}
	lines = append(lines, m.statusLine(), m.keyBar())
	return strings.Join(lines, "\n")
}

func (m *Model) titleLine() string {
	title := fmt.Sprintf("ppa-utils list  %d of %d sources", len(m.view), len(m.rows))
	if filter := m.filter.Value(); filter != "" && m.mode != modeFilter {
		title += "  filter: " + filter
	}
	return ansi.Truncate(title, m.width, "…")
}

func (m *Model) tableLines() []string {
	widths := m.columnWidths()

	header := make([]string, len(columns))
	for i, c := range columns {
		title := c.title
		if i == m.sortCol {
			title += map[bool]string{false: "▲", true: "▼"}[m.sortDesc]
		}
		header[i] = pad(title, widths[i])
	}
	lines := []string{headerStyle.Render(pad("", markerWidth) + strings.Join(header, " "))}

	end := min(m.offset+m.bodyHeight(), len(m.view))
	for i := m.offset; i < end; i++ {
		row := m.view[i]

		marker := "  "
		if row.Official {
			marker = "* "
		}
		cells := rowCells(row)
		for j := range cells {
			cells[j] = pad(cells[j], widths[j])
		}
		text := ansi.Truncate(marker+strings.Join(cells, " "), m.width, "")

		switch {
		case i == m.cursor:
			text = selectedStyle.Render(pad(text, m.width))
		case row.Status == sourcelist.StatusDisabled:
			text = disabledStyle.Render(text)
		}
		lines = append(lines, text)
	}

	if len(m.view) == 0 {
		lines = append(lines, "No sources match.")
	}
	return lines
}

// minFlexWidth is the least width of a flexible column.
const minFlexWidth = 12

// columnWidths fits the columns into the screen width. Columns with a
// maximum width take their content width up to it. The remaining space is
// shared by the flexible columns by weight; a column that needs less than its
// share gives the rest to the others.
func (m *Model) columnWidths() []int {
	widths := make([]int, len(columns))
	natural := make([]int, len(columns))
	remaining := m.width - markerWidth - (len(columns) - 1) // marker and separators

	var flexible []int
	for i, c := range columns {
		w := lipgloss.Width(c.title) + 1 // room for the sort arrow
		for _, row := range m.view {
			w = max(w, lipgloss.Width(c.cell(row)))
		}
		natural[i] = w

		if c.maxWidth == 0 {
			flexible = append(flexible, i)
			continue
		}
		widths[i] = min(w, c.maxWidth)
		remaining -= widths[i]
	}

	for len(flexible) > 0 {
		totalWeight := 0
		for _, i := range flexible {
			totalWeight += columns[i].weight
		}

		var rest []int
		for _, i := range flexible {
			if share := remaining * columns[i].weight / totalWeight; natural[i] <= max(share, minFlexWidth) {
				widths[i] = natural[i]
				remaining -= natural[i]
			} else {
				rest = append(rest, i)
			}
		}

		if len(rest) == len(flexible) { // everyone needs more than the share
			for _, i := range rest {
				widths[i] = max(remaining*columns[i].weight/totalWeight, minFlexWidth)
			}
			break
		}
		flexible = rest
	}
	return widths
}

func (m *Model) statusLine() string {
	switch m.mode {
	case modeFilter:
		return m.filter.View()
	case modeConfirm:
		verb := "Disable"
		if m.pending.enable {
			verb = "Enable"
		}
		prompt := fmt.Sprintf("%s %s (%s)? [y/N]", verb, m.pending.row.Name, m.pending.row.Path)
		return errorStyle.Render(ansi.Truncate(prompt, m.width, "…"))
	}

	text := ansi.Truncate(m.status, m.width, "…")
	if m.statusErr {
		return errorStyle.Render(text)
	}
	return statusStyle.Render(text)
}

func (m *Model) keyBar() string {
	items := [][2]string{
		{"F1", "Help"}, {"F4", "Filter"}, {"F5", "Reload"}, {"F6", "Sort"},
		{"e", "Enable"}, {"d", "Disable"}, {"F10", "Quit"},
	}
	var bar strings.Builder
	for _, item := range items {
		bar.WriteString(keyStyle.Render(item[0]) + labelStyle.Render(item[1]) + " ")
	}
	return bar.String()
}

func (m *Model) helpLines() []string {
	return []string{
		"",
		"  Up/Down, j/k       move the selection",
		"  PgUp/PgDn          move by a page",
		"  Home/End, g/G      first / last source",
		"  / or F4            filter: free text or column:text, e.g. suite:noble status:disabled",
		"  Esc                clear the filter",
		"  < > or F6          sort by the previous / next column",
		"  I                  invert the sort order",
		"  e / d / Space      enable / disable / toggle the selected source file",
		"  R or F5            reload",
		"  q or F10           quit",
		"",
		"  * marks official Ubuntu sources, which cannot be disabled.",
		"  Changes need root: sudo asks for the password when required.",
		"",
		"  Press any key to go back.",
	}
}

// pad cuts the text to the width and fills it up with spaces.
func pad(text string, width int) string {
	text = ansi.Truncate(text, width, "…")
	return text + strings.Repeat(" ", max(width-lipgloss.Width(text), 0))
}
