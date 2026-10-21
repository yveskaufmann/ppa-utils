// Package root defines the ppautil root command.
package root

import "github.com/spf13/cobra"

// New returns the ppautil root command. Subcommands are registered by the caller.
func New() *cobra.Command {
	return &cobra.Command{
		Use:           "ppautil",
		Short:         "Utilities for managing Ubuntu PPAs",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
}
