// Package check defines the ppautil check command.
package check

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"yv35.com/ppa/ppa-utils/internal/cli/clitext"
	"yv35.com/ppa/ppa-utils/internal/distinfo"
	"yv35.com/ppa/ppa-utils/internal/ppa"
)

// Options holds everything the check command needs to run.
type Options struct {
	// Release is the target Ubuntu release code. Empty means: derive it from the running system.
	Release string

	Out    io.Writer
	ErrOut io.Writer
}

// New returns the check command. When fn is not nil it is called with the
// parsed options instead of running the command.
func New(fn func(*Options) error) *cobra.Command {
	opts := &Options{}

	return &cobra.Command{
		Use:   "check [release]",
		Short: "Check if it is safe to upgrade Ubuntu, i.e. all your PPAs support the new release",
		Long: clitext.Long(`
			Check whether it is safe to upgrade to a new Ubuntu release.

			Before an upgrade, every PPA in /etc/apt/sources.list.d needs to publish
			packages for the release you are upgrading to. Otherwise apt can no longer
			update or install packages from that PPA after the upgrade.

			The command checks each of your PPAs against the target release and marks
			it as supported (✅) or not supported (❌). Without an argument the target
			release is derived from the running system. With a release code (for
			example "noble") that release is the target.
		`),
		Example: clitext.Examples(`
			# Check if all my PPAs support the release I am about to upgrade to
			ppautil check

			# Check if all my PPAs already support Ubuntu "noble"
			ppautil check noble
		`),
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeReleaseCodes,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Out = cmd.OutOrStdout()
			opts.ErrOut = cmd.ErrOrStderr()
			if len(args) == 1 {
				opts.Release = args[0]
			}

			if fn != nil {
				return fn(opts)
			}
			return run(opts)
		},
	}
}

// completeReleaseCodes completes the release argument with all known Ubuntu release codes,
// newest first. KeepOrder stops the shell from sorting them alphabetically.
func completeReleaseCodes(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	registry, err := distinfo.NewUbuntuRegistry()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	codes := slices.Clone(registry.ListAllReleaseCodes())
	slices.Reverse(codes)
	return codes, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveKeepOrder
}

func run(opts *Options) error {
	distInfoReader, err := distinfo.NewUbuntuRegistry()
	if err != nil {
		return fmt.Errorf("creating ubuntu dist registry: %w", err)
	}

	var newCode string
	if opts.Release != "" {
		newCode = opts.Release
		if !distInfoReader.IsValidReleaseCode(newCode) {
			return fmt.Errorf("unknown Ubuntu release %q, valid releases: %s",
				newCode, strings.Join(distInfoReader.ListAllReleaseCodes(), ", "))
		}
	} else {
		nextRelease, err := distInfoReader.FindCurrentRelease()
		if err != nil {
			if !errors.Is(err, distinfo.ErrNoNextRelease) {
				return fmt.Errorf("getting current distribution info: %w", err)
			}
			fmt.Fprintln(opts.ErrOut, "No next release found:", err.Error())
		} else {
			fmt.Fprintln(opts.ErrOut, "Next release found:", nextRelease.Series)
		}
		newCode = nextRelease.Series
	}

	sourceFiles, err := ppa.ListSourceFiles()
	if err != nil {
		return fmt.Errorf("listing source files: %w", err)
	}
	for _, sf := range sourceFiles {
		if !isUsingUbuntuReleaseCode(&sf) {
			continue
		}

		check := "❌ " + sf.URIs[0]
		if isUbuntuReleaseCodeSupported(&sf, newCode) {
			check = "✅"
		}
		fmt.Fprintf(opts.Out, "%s - %s\n", sf.Path, check)
	}
	return nil
}

var nonUbuntuReleaseCodes = []string{
	"stable",
	"/",
	"",
	"multiverse",
}

func isUsingUbuntuReleaseCode(source *ppa.SourceFile) bool {
	if slices.Contains(source.Components, "multiverse") {
		return false
	}

	for _, suite := range source.Suites {
		if !slices.Contains(nonUbuntuReleaseCodes, suite) {
			return true
		}
	}
	return false
}

func toInReleaseURL(source *ppa.SourceFile, code string) string {
	if code == "" {
		code = source.Suites[0]
	}

	baseURI := source.URIs[0]
	if baseURI[len(baseURI)-1] == '/' {
		baseURI = baseURI[:len(baseURI)-1]
	}

	return fmt.Sprintf("%s/dists/%s/InRelease", baseURI, code)
}

func isUbuntuReleaseCodeSupported(source *ppa.SourceFile, code string) bool {
	inReleaseURL := toInReleaseURL(source, code)

	resp, err := http.Get(inReleaseURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}
