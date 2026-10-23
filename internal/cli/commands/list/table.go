package list

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"yv35.com/ppa/ppa-utils/internal/cli/sourcelist"
)

func writeJSON(w io.Writer, rows []sourcelist.Row) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(rows)
}

func writeTable(w io.Writer, rows []sourcelist.Row) error {
	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	fmt.Fprintln(table, "NAME\tSTATUS\tURI\tSUITES\tCOMPONENTS\tARCH\tSIGNED BY")
	for _, row := range rows {
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			dash(row.Name),
			dash(row.Status),
			dash(strings.Join(row.URIs, ", ")),
			dash(strings.Join(row.Suites, ", ")),
			dash(strings.Join(row.Components, ", ")),
			dash(strings.Join(row.Architectures, ", ")),
			dash(row.SignedBy),
		)
	}
	return table.Flush()
}

// dash returns a placeholder for empty cells, so columns stay aligned.
func dash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}
