// Package version defines the ppautil version command.
package version

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"yv35.com/ppa/ppa-utils/internal/version"
)

// Options holds everything the version command needs to run.
type Options struct {
	Version string

	Out io.Writer
}

// New returns the version command. When fn is not nil it is called with the
// parsed options instead of running the command.
func New(fn func(*Options) error) *cobra.Command {
	opts := &Options{
		Version: version.Version,
	}

	return &cobra.Command{
		Use:   "version",
		Short: "Print the ppautil version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts.Out = cmd.OutOrStdout()

			if fn != nil {
				return fn(opts)
			}
			return run(opts)
		},
	}
}

func run(opts *Options) error {
	_, err := fmt.Fprintf(opts.Out, "ppautil %s\n", opts.Version)
	return err
}
