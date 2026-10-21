package main

import (
	"fmt"
	"os"

	"yv35.com/ppa/ppa-utils/internal/cli/check"
	"yv35.com/ppa/ppa-utils/internal/cli/root"
	"yv35.com/ppa/ppa-utils/internal/cli/version"
)

func main() {
	rootCmd := root.New()
	rootCmd.AddCommand(
		check.New(nil),
		version.New(nil),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
