package sources

import (
	"sort"
	"strings"

	"yv35.com/ppa-utils/internal/cli/sourcelist"
)

// column is one column of the table.
type column struct {
	title string
	cell  func(sourcelist.Row) string
	// maxWidth limits the column. Zero means the column is flexible and
	// shares the remaining space with the other flexible columns by weight.
	maxWidth int
	weight   int
}

var columns = []column{
	{"NAME", func(r sourcelist.Row) string { return r.Name }, 24, 0},
	{"STATUS", func(r sourcelist.Row) string { return r.Status }, 8, 0},
	{"URI", func(r sourcelist.Row) string { return strings.Join(r.URIs, ", ") }, 0, 3},
	{"SUITES", func(r sourcelist.Row) string { return strings.Join(r.Suites, ", ") }, 20, 0},
	{"COMPONENTS", func(r sourcelist.Row) string { return strings.Join(r.Components, ", ") }, 20, 0},
	{"ARCH", func(r sourcelist.Row) string { return strings.Join(r.Architectures, ", ") }, 8, 0},
	{"SIGNED BY", func(r sourcelist.Row) string { return r.SignedBy }, 0, 2},
}

// rowCells returns the text of every column of the row.
func rowCells(row sourcelist.Row) []string {
	cells := make([]string, len(columns))
	for i, c := range columns {
		cells[i] = c.cell(row)
	}
	return cells
}

// sortRows sorts the rows by the given column, keeping the order of equal rows.
func sortRows(rows []sourcelist.Row, col int, desc bool) {
	sort.SliceStable(rows, func(i, j int) bool {
		a := strings.ToLower(columns[col].cell(rows[i]))
		b := strings.ToLower(columns[col].cell(rows[j]))
		if desc {
			return a > b
		}
		return a < b
	})
}
