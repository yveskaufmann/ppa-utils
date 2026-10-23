package main

import (
	"fmt"
	"os"

	"yv35.com/ppa-utils/internal/cli/commands/check"
	"yv35.com/ppa-utils/internal/cli/commands/list"
	"yv35.com/ppa-utils/internal/cli/commands/version"
	"yv35.com/ppa-utils/internal/cli/root"
)

func main() {
	rootCmd := root.New()
	rootCmd.AddCommand(
		check.New(nil),
		list.New(list.DefaultDeps(), nil),
		version.New(nil),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
