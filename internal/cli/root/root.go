// Package root defines the ppa-utils root command.
package root

import "github.com/spf13/cobra"

// New returns the ppa-utils root command. Subcommands are registered by the caller.
func New() *cobra.Command {
	return &cobra.Command{
		Use:           "ppa-utils",
		Short:         "Utilities for managing Ubuntu PPAs",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
}
