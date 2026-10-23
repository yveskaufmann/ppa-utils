// Package check defines the ppautil check command.
package check

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

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

	var targetDist distinfo.Distribution
	if opts.Release != "" {
		dist, ok := distInfoReader.FindDistributionByCode(opts.Release)
		if !ok {
			return fmt.Errorf("unknown Ubuntu release %q, valid releases: %s",
				opts.Release, strings.Join(distInfoReader.ListAllReleaseCodes(), ", "))
		}
		targetDist = dist
	} else {
		nextRelease, err := distInfoReader.FindNextRelease()
		if err != nil {
			if !errors.Is(err, distinfo.ErrNoNextRelease) {
				return fmt.Errorf("getting current distribution info: %w", err)
			}
			fmt.Fprintln(opts.ErrOut, "No next release found:", err.Error())
		} else {
			fmt.Fprintln(opts.ErrOut, "Next release found:", nextRelease.Series)
		}
		targetDist = nextRelease
	}

	known := newReleases(distInfoReader)
	target := release{code: targetDist.Series, version: targetDist.Version}

	sourceFiles, err := ppa.ListSourceFiles()
	if err != nil {
		return fmt.Errorf("listing source files: %w", err)
	}
	for _, sf := range sourceFiles {
		if !isUsingUbuntuReleaseCode(&sf, known) {
			continue
		}

		check := "❌ " + sf.URIs[0]
		if isUbuntuReleaseCodeSupported(&sf, known, target) {
			check = "✅"
		}
		fmt.Fprintf(opts.Out, "%s - %s\n", sf.Path, check)
	}
	return nil
}

// Release codes are looked for as whole tokens. URIs keep "-" inside a token so
// that names like "lucid-studio" are not mistaken for a release code, while
// suites and components are split on it so that "noble-updates" is recognized.
// Release versions like "26.04" are dotted numbers.
var (
	uriTokens     = regexp.MustCompile(`[A-Za-z0-9-]+`)
	plainTokens   = regexp.MustCompile(`[A-Za-z0-9]+`)
	versionTokens = regexp.MustCompile(`[0-9]+(?:\.[0-9]+)+`)

	// yearMonth is the format of Ubuntu release versions, e.g. "26.04".
	yearMonth = regexp.MustCompile(`^[0-9]{2}\.[0-9]{2}$`)
)

// maxInReleaseSize limits how much of an InRelease file is read.
const maxInReleaseSize = 8 << 20

var httpClient = &http.Client{Timeout: 15 * time.Second}

// releases recognizes the identifiers of Ubuntu releases within source files.
type releases struct {
	isCode    func(token string) bool
	isVersion func(token string) bool
}

// newReleases recognizes the release codes and the year-month versions
// (e.g. "26.04") of the Ubuntu releases known to the registry.
func newReleases(registry *distinfo.UbuntuRegistry) releases {
	return releases{
		isCode: registry.IsValidReleaseCode,
		isVersion: func(token string) bool {
			if !yearMonth.MatchString(token) {
				return false
			}
			_, ok := registry.FindDistributionByVersion(token)
			return ok
		},
	}
}

// release identifies a single Ubuntu release. An empty version means it is unknown.
type release struct {
	code    string
	version string
}

func (k releases) hasRelease(tokens *regexp.Regexp, values []string) bool {
	return slices.ContainsFunc(values, func(value string) bool {
		return slices.ContainsFunc(tokens.FindAllString(value, -1), k.isCode) ||
			slices.ContainsFunc(versionTokens.FindAllString(value, -1), k.isVersion)
	})
}

func replaceMatching(tokens *regexp.Regexp, value string, match func(string) bool, replacement string) string {
	return tokens.ReplaceAllStringFunc(value, func(token string) string {
		if match(token) {
			return replacement
		}
		return token
	})
}

func (k releases) replace(tokens *regexp.Regexp, values []string, target release) []string {
	replaced := make([]string, len(values))
	for i, value := range values {
		value = replaceMatching(tokens, value, k.isCode, target.code)
		if target.version != "" {
			value = replaceMatching(versionTokens, value, k.isVersion, target.version)
		}
		replaced[i] = value
	}
	return replaced
}

// isUsingUbuntuReleaseCode reports whether the URIs, suites or components of
// the source contain an Ubuntu release code or version and are therefore
// specific to one Ubuntu release. Sources without such an indicator work for
// every release.
func isUsingUbuntuReleaseCode(source *ppa.SourceFile, known releases) bool {
	return known.hasRelease(uriTokens, source.URIs) ||
		known.hasRelease(plainTokens, source.Suites) ||
		known.hasRelease(plainTokens, source.Components)
}

// retarget returns a copy of the source in which every release code and
// version in the URIs, suites and components is replaced by the target's.
func retarget(source *ppa.SourceFile, known releases, target release) ppa.SourceFile {
	retargeted := *source
	retargeted.URIs = known.replace(uriTokens, source.URIs, target)
	retargeted.Suites = known.replace(plainTokens, source.Suites, target)
	retargeted.Components = known.replace(plainTokens, source.Components, target)
	return retargeted
}

// toInReleaseURL returns the URL of the InRelease file. A suite ending in "/"
// denotes a flat repository whose InRelease file lies directly below the URI.
func toInReleaseURL(uri, suite string) string {
	base := strings.TrimSuffix(uri, "/")
	if strings.HasSuffix(suite, "/") {
		if dir := strings.Trim(suite, "/"); dir != "" {
			base += "/" + dir
		}
		return base + "/InRelease"
	}
	return fmt.Sprintf("%s/dists/%s/InRelease", base, suite)
}

// isUbuntuReleaseCodeSupported reports whether the repository of the source
// publishes the target release. For that, release codes and versions in the
// source are replaced by the target and the resulting InRelease file has to exist. When
// components contained a release code they have to be listed in it as well.
func isUbuntuReleaseCodeSupported(source *ppa.SourceFile, known releases, target release) bool {
	if len(source.URIs) == 0 || len(source.Suites) == 0 {
		return false
	}

	targetSource := retarget(source, known, target)

	resp, err := httpClient.Get(toInReleaseURL(targetSource.URIs[0], targetSource.Suites[0]))
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}
	if slices.Equal(source.Components, targetSource.Components) {
		return true
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxInReleaseSize))
	if err != nil {
		return false
	}
	return releaseListsComponents(body, targetSource.Components)
}

// releaseListsComponents reports whether the "Components:" field of the
// release file lists all given components.
func releaseListsComponents(release []byte, components []string) bool {
	for _, line := range strings.Split(string(release), "\n") {
		value, ok := strings.CutPrefix(line, "Components:")
		if !ok {
			continue
		}
		listed := strings.Fields(value)
		return !slices.ContainsFunc(components, func(c string) bool {
			return !slices.Contains(listed, c)
		})
	}
	return false
}
