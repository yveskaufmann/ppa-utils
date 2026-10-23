package sources

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"yv35.com/ppa-utils/internal/ppa"
)

// Toggler enables or disables the source file at a path.
type Toggler interface {
	// SetEnabled returns a command that changes the source file and then
	// reports the result as a toggledMsg.
	SetEnabled(path string, enabled bool) tea.Cmd
}

// SudoToggler writes the changed source file directly when running as root and
// otherwise through sudo, which asks for the password on the terminal.
type SudoToggler struct{}

// SetEnabled implements Toggler. Disabling an official Ubuntu source is refused.
func (SudoToggler) SetEnabled(path string, enabled bool) tea.Cmd {
	done := func(err error) tea.Msg {
		return toggledMsg{path: path, enabled: enabled, err: err}
	}

	content, err := ppa.ToggledContent(path, enabled)
	if err != nil {
		return func() tea.Msg { return done(err) }
	}

	if os.Geteuid() == 0 {
		return func() tea.Msg { return done(writeFileAtomic(path, content)) }
	}

	tmp, err := os.CreateTemp("", "ppa-utils-*.sources")
	if err != nil {
		return func() tea.Msg { return done(err) }
	}
	_, writeErr := tmp.Write(content)
	closeErr := tmp.Close()
	if err := firstError(writeErr, closeErr); err != nil {
		os.Remove(tmp.Name())
		return func() tea.Msg { return done(err) }
	}

	install := exec.Command("sudo", "install", "-o", "root", "-g", "root", "-m", "0644", tmp.Name(), path)
	return tea.ExecProcess(install, func(err error) tea.Msg {
		os.Remove(tmp.Name())
		if err != nil {
			err = fmt.Errorf("sudo install failed: %w", err)
		}
		return done(err)
	})
}

func firstError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// writeFileAtomic replaces the file at path with content via a temporary file
// in the same directory, keeping the file mode.
func writeFileAtomic(path string, content []byte) error {
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".ppa-utils-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), mode); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
