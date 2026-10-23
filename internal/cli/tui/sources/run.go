package sources

import (
	"io"

	tea "github.com/charmbracelet/bubbletea"
)

// Run shows the view until the user quits.
func Run(cfg Config, in io.Reader, out io.Writer) error {
	program := tea.NewProgram(New(cfg), tea.WithInput(in), tea.WithOutput(out), tea.WithAltScreen())
	_, err := program.Run()
	return err
}
