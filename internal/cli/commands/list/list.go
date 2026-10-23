// Package list defines the ppa-utils list command.
package list

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"

	"yv35.com/ppa-utils/internal/cli/clitext"
	"yv35.com/ppa-utils/internal/cli/sourcelist"
	"yv35.com/ppa-utils/internal/cli/tui/sources"
	"yv35.com/ppa-utils/internal/ppa"
)

// Options holds everything the list command needs to run.
type Options struct {
	Filter sourcelist.Filter
	// JSON prints the rows as a JSON array instead of a table.
	JSON bool
	// Interactive shows the sources in an interactive view instead of a table.
	Interactive bool

	Out    io.Writer
	ErrOut io.Writer
}

// Deps are the dependencies of the list command.
type Deps struct {
	// Lister returns the entries of all source files.
	Lister func() ([]ppa.SourceFile, error)
	// Toggler enables and disables source files in the interactive view.
	Toggler sources.Toggler
	// IsTerminal reports whether w is an interactive terminal.
	IsTerminal func(w io.Writer) bool
	// RunTUI shows the interactive view.
	RunTUI func(cfg sources.Config) error
}

// DefaultDeps returns the dependencies for running on the real system.
func DefaultDeps() Deps {
	return Deps{
		Lister:  ppa.ListSourceFiles,
		Toggler: sources.SudoToggler{},
		IsTerminal: func(w io.Writer) bool {
			f, ok := w.(*os.File)
			return ok && term.IsTerminal(f.Fd())
		},
		RunTUI: func(cfg sources.Config) error {
			return sources.Run(cfg, os.Stdin, os.Stdout)
		},
	}
}

// New returns the list command. When fn is not nil it is called with the
// parsed options instead of running the command.
func New(deps Deps, fn func(*Options) error) *cobra.Command {
	opts := &Options{}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the PPAs and other apt sources installed on this system",
		Long: clitext.Long(`
			List the apt sources in /etc/apt/sources.list.d as a table with their
			name, status, URI, suites, components, architectures and signing key.

			The signing key is shown as a path, or as "inline-key" when the key is
			embedded in the source file.

			With --interactive the sources are shown in a view similar to htop. There
			you filter and sort live, and enable or disable the selected source file.
			Official Ubuntu sources can not be disabled. Changing a file needs root,
			sudo asks for the password when required.

			Filters narrow the list down. A filter matches when the column contains
			the given text, ignoring case. Several filters must all match.
		`),
		Example: clitext.Examples(`
			# List all installed PPAs as a table
			ppa-utils list

			# List only the sources that use the suite "resolute"
			ppa-utils list --suite resolute

			# List the sources of Launchpad PPAs that are enabled for arm64
			ppa-utils list --uri ppa.launchpadcontent.net --arch arm64

			# Print the list as JSON array, for scripts
			ppa-utils list --json

			# Browse the sources interactively: filter, sort, enable and disable them
			ppa-utils list --interactive
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts.Out = cmd.OutOrStdout()
			opts.ErrOut = cmd.ErrOrStderr()

			if fn != nil {
				return fn(opts)
			}
			return run(opts, deps)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.Filter.Name, "name", "", "only sources whose file name contains this text")
	flags.StringVar(&opts.Filter.URI, "uri", "", "only sources with a URI containing this text")
	flags.StringVar(&opts.Filter.Suite, "suite", "", "only sources with a suite containing this text")
	flags.StringVar(&opts.Filter.Component, "component", "", "only sources with a component containing this text")
	flags.StringVar(&opts.Filter.Arch, "arch", "", "only sources for this architecture")
	flags.StringVar(&opts.Filter.SignedBy, "signed-by", "", "only sources whose signing key path contains this text")
	flags.StringVar(&opts.Filter.Status, "status", "", "only sources with this status: enabled or disabled")
	flags.BoolVar(&opts.JSON, "json", false, "print the sources as JSON array")
	flags.BoolVarP(&opts.Interactive, "interactive", "i", false, "browse the sources in an interactive view")

	_ = cmd.RegisterFlagCompletionFunc("status", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return []string{sourcelist.StatusEnabled, sourcelist.StatusDisabled}, cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("arch", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return []string{
			string(ppa.ArchitectureAMD64),
			string(ppa.ArchitectureARM64),
			string(ppa.ArchitectureI386),
		}, cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

func run(opts *Options, deps Deps) error {
	if opts.Interactive && opts.JSON {
		return errors.New("--interactive and --json cannot be combined")
	}
	if opts.Interactive {
		if deps.IsTerminal(opts.Out) {
			return deps.RunTUI(sources.Config{
				Load:    func() ([]sourcelist.Row, error) { return loadRows(deps) },
				Toggler: deps.Toggler,
				Filter:  sources.FormatFilter(opts.Filter),
			})
		}
		fmt.Fprintln(opts.ErrOut, "Output is not a terminal, printing the table instead")
	}

	rows, err := loadRows(deps)
	if err != nil {
		return err
	}
	rows = opts.Filter.Apply(rows)

	if opts.JSON {
		return writeJSON(opts.Out, rows)
	}
	if len(rows) == 0 {
		fmt.Fprintln(opts.ErrOut, "No sources found")
	}
	return writeTable(opts.Out, rows)
}

func loadRows(deps Deps) ([]sourcelist.Row, error) {
	sources, err := deps.Lister()
	if err != nil {
		return nil, fmt.Errorf("listing source files: %w", err)
	}
	return sourcelist.ToRows(sources), nil
}
