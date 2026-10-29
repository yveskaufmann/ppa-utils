package create

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"

	"yv35.com/ppa-utils/internal/cli/clitext"
	"yv35.com/ppa-utils/internal/distinfo"
	"yv35.com/ppa-utils/internal/ppa"
)

// Options holds everything needed to create a .source file.
type Options struct {
	FromListFile  string
	FromListEntry string
	Types         string
	URI           string
	Suite         string
	Components    string
	Architectures string
	SignedBy      string
	Output        string
	Force         bool
	Dry           bool
	In            io.Reader
	Out           io.Writer
	ErrOut        io.Writer
	AskPrompt     func(string, string) string
}

// New returns the create command. When fn is not nil it is called with the parsed options.
func New(fn func(*Options) error) *cobra.Command {
	opts := &Options{}
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a Deb822 .source file from an apt list file or entry",
		Long: `Create a .source file for apt by converting a classic .list entry into the modern Deb822 format.

Use one of the following modes:
  - --from-list-file /path/to/file.list
  - --from-list-entry 'deb [arch=amd64,arm64 signed-by=/usr/share/keyrings/example-keyring.asc] https://example.com/repo stable main'

The --from-list-entry value is a single shell argument, so entries containing spaces must be quoted.
When neither flag is supplied it prompts interactively for the values to write.
`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts.Out = cmd.OutOrStdout()
			opts.ErrOut = cmd.ErrOrStderr()
			opts.In = cmd.InOrStdin()
			if opts.AskPrompt == nil {
				opts.AskPrompt = func(label, defaultValue string) string {
					return clitext.Prompt(label, defaultValue, opts.In, opts.Out)
				}
			}
			if fn != nil {
				return fn(opts)
			}
			return run(opts)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&opts.FromListFile, "from-list-file", "", "create the source file from a given apt list file")
	flags.StringVar(&opts.FromListEntry, "from-list-entry", "", "create the source file from a given apt list entry; quote the value if it contains spaces")
	flags.StringVar(&opts.Types, "types", "", "source types to write (comma- or space-separated, e.g. deb or deb,deb-src)")
	flags.StringVar(&opts.URI, "uri", "", "URI for the source entry; overrides values from list input")
	flags.StringVar(&opts.Suite, "suite", "", "suite for the source entry; overrides values from list input")
	flags.StringVar(&opts.Components, "components", "", "components for the source entry; overrides values from list input")
	flags.StringVar(&opts.Architectures, "architectures", "", "architectures for the source entry, e.g. amd64,arm64")
	flags.StringVar(&opts.SignedBy, "signed-by", "", "path to the signing key; overrides values from list input")
	flags.StringVarP(&opts.Output, "output", "o", "", "path of the .source file to create")
	flags.BoolVarP(&opts.Force, "force", "f", false, "overwrite the target file without prompting")
	flags.BoolVarP(&opts.Dry, "dry", "d", false, "show what would be written without changing the filesystem")

	_ = cmd.RegisterFlagCompletionFunc("from-list-file", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("types", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return completeSourceTypes(), cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("uri", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("suite", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return completeReleaseCodes(), cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveKeepOrder
	})
	_ = cmd.RegisterFlagCompletionFunc("components", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return []string{"main", "universe", "restricted", "multiverse"}, cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("architectures", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return completeArchitectures(), cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("signed-by", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("output", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	})
	return cmd
}

func run(opts *Options) error {
	if opts.FromListFile != "" && opts.FromListEntry != "" {
		return errors.New("--from-list-file and --from-list-entry are mutually exclusive")
	}

	var source *ppa.SourceFile
	var err error
	input := opts.FromListFile
	if input == "" {
		input = opts.FromListEntry
	}

	if opts.FromListFile != "" {
		content, readErr := os.ReadFile(opts.FromListFile)
		if readErr != nil {
			return fmt.Errorf("reading list file %q: %w", opts.FromListFile, readErr)
		}
		text := strings.TrimSpace(string(content))
		if text == "" {
			return errors.New("list file is empty")
		}
		if source, err = parseListContent(text); err != nil {
			return fmt.Errorf("parsing list file %q: %w", opts.FromListFile, err)
		}
	} else if opts.FromListEntry != "" {
		if source, err = ppa.ParseListLine(opts.FromListEntry); err != nil {
			return fmt.Errorf("parsing apt list input %q: %w", opts.FromListEntry, err)
		}
	} else {
		if source, err = promptSource(opts); err != nil {
			return err
		}
	}

	if err := applyExplicitSourceOptions(source, opts); err != nil {
		return err
	}

	if opts.Output == "" {
		opts.Output = defaultOutputPath(input, source)
	}
	if err := ensureWriteAccess(opts.Output, opts.Dry); err != nil {
		return err
	}

	if err := handleMissingSignedBy(opts, source); err != nil {
		return err
	}

	if opts.Dry {
		fmt.Fprintf(opts.Out, "\n\033[1m=== DRY RUN ===\033[0m\n")
		fmt.Fprintf(opts.Out, "Would write the source file to %s\n", opts.Output)
		if _, err := os.Stat(opts.Output); err == nil {
			fmt.Fprintf(opts.Out, "Would overwrite the existing file %s\n", opts.Output)
		}
		if source.SignedBy != "" {
			if _, statErr := os.Stat(source.SignedBy); statErr != nil {
				fmt.Fprintf(opts.Out, "Would download or install the signing key %q before the source can be used\n", source.SignedBy)
			}
		}
		fmt.Fprint(opts.Out, ppa.EncodeSourceFile(source))
		return nil
	}

	if err := ensureOutputPath(opts.Output, opts.Force, opts.Out, opts.ErrOut); err != nil {
		return err
	}

	content := ppa.EncodeSourceFile(source)
	if err := os.WriteFile(opts.Output, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing source file %q: %w", opts.Output, err)
	}

	fmt.Fprintf(opts.Out, "Created %s\n", opts.Output)
	fmt.Fprint(opts.Out, content)
	return nil
}

func completeSourceTypes() []string {
	return []string{"deb", "deb-src"}
}

func completeArchitectures() []string {
	return []string{string(ppa.ArchitectureAMD64), string(ppa.ArchitectureARM64), string(ppa.ArchitectureI386)}
}

func completeReleaseCodes() []string {
	registry, err := distinfo.NewUbuntuRegistry()
	if err != nil {
		return []string{"amd64", "arm64", "i386"}
	}
	return registry.ListAllReleaseCodes()
}

func defaultOutputPath(input string, source *ppa.SourceFile) string {
	if input != "" {
		if stat, err := os.Stat(input); err == nil && !stat.IsDir() {
			if ext := filepath.Ext(input); ext != "" {
				return strings.TrimSuffix(input, ext) + ".sources"
			}
			return input + ".sources"
		}
	}
	if source == nil || len(source.URIs) == 0 {
		return filepath.Join(ppa.DEFAULT_SOURCE_FILE_PATH, "ppa.sources")
	}
	name := deriveSourceNameFromURI(source.URIs[0])
	if name == "" {
		name = "ppa"
	}
	return filepath.Join(ppa.DEFAULT_SOURCE_FILE_PATH, sanitizeName(name)+".sources")
}

func deriveSourceNameFromURI(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || parsed == nil {
		return lastNonEmptySegment(trimmed)
	}

	segments := splitURLSegments(parsed.EscapedPath())
	if host := strings.ToLower(parsed.Hostname()); host != "" {
		if strings.Contains(host, "launchpad") {
			for i := len(segments) - 1; i >= 0; i-- {
				if segment := sanitizeName(segments[i]); segment != "" && !isGenericRepoSegment(segment) {
					return segment
				}
			}
		}
		for i := len(segments) - 1; i >= 0; i-- {
			if segment := sanitizeName(segments[i]); segment != "" && !isGenericRepoSegment(segment) {
				return segment
			}
		}
		if len(segments) > 0 {
			if segment := sanitizeName(segments[len(segments)-1]); segment != "" {
				return segment
			}
		}
		if hostName := deriveHostName(host); hostName != "" {
			return hostName
		}
	}
	if len(segments) > 0 {
		if segment := sanitizeName(segments[len(segments)-1]); segment != "" {
			return segment
		}
	}
	return lastNonEmptySegment(trimmed)
}

func splitURLSegments(rawPath string) []string {
	cleaned := strings.TrimSpace(rawPath)
	cleaned = strings.Trim(cleaned, "/")
	if cleaned == "" {
		return nil
	}
	parts := strings.Split(cleaned, "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		part = strings.Trim(part, ".")
		part = strings.TrimSuffix(part, ".list")
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func isGenericRepoSegment(value string) bool {
	v := strings.ToLower(sanitizeName(value))
	if v == "" {
		return true
	}
	generic := map[string]struct{}{
		"apt": {}, "archive": {}, "ubuntu": {}, "debian": {}, "stable": {},
		"main": {}, "universe": {}, "restricted": {}, "multiverse": {},
		"repo": {}, "repos": {}, "repository": {}, "repositories": {},
		"dists": {}, "pool": {}, "packages": {}, "ppa": {}, "launchpad": {},
		"inrelease": {}, "aptrepo": {}, "apt-repo": {}, "download": {}, "downloads": {},
	}
	_, ok := generic[v]
	return ok
}

func deriveHostName(host string) string {
	if host == "" {
		return ""
	}
	labels := strings.Split(host, ".")
	for i := len(labels) - 1; i >= 0; i-- {
		label := strings.TrimSpace(labels[i])
		label = strings.TrimPrefix(label, "www-")
		if label == "" || label == "com" || label == "net" || label == "org" || label == "io" || label == "ai" || label == "de" {
			continue
		}
		if strings.Contains(label, "launchpad") || label == "ppa" || strings.Contains(label, "apt") || strings.Contains(label, "repo") || strings.Contains(label, "archive") {
			continue
		}
		return sanitizeName(label)
	}
	return ""
}

func lastNonEmptySegment(value string) string {
	for i := len(value) - 1; i >= 0; i-- {
		if value[i] == '/' || value[i] == '\\' {
			continue
		}
		segment := strings.Trim(value[i:], "/\\")
		if segment == "" {
			continue
		}
		return sanitizeName(segment)
	}
	return sanitizeName(value)
}

func sanitizeName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '-', r == '_', r == '.':
			return r
		default:
			return '-'
		}
	}, name)
	return strings.Trim(name, "-_.")
}

func applyExplicitSourceOptions(source *ppa.SourceFile, opts *Options) error {
	if source == nil {
		return nil
	}
	if opts == nil {
		return nil
	}
	if opts.Types != "" {
		types := splitList(opts.Types)
		if len(types) == 0 {
			return errors.New("--types cannot be empty")
		}
		source.Types = types
	}
	if opts.URI != "" {
		source.URIs = []string{opts.URI}
	}
	if opts.Suite != "" {
		source.Suites = []string{opts.Suite}
	}
	if opts.Components != "" {
		source.Components = splitList(opts.Components)
		if len(source.Components) == 0 {
			return errors.New("--components cannot be empty")
		}
	}
	if opts.Architectures != "" {
		archs := splitList(opts.Architectures)
		if len(archs) == 0 {
			return errors.New("--architectures cannot be empty")
		}
		source.Architectures = make([]ppa.Architecture, 0, len(archs))
		for _, arch := range archs {
			source.Architectures = append(source.Architectures, ppa.ParseArchitecture(arch))
		}
	}
	if opts.SignedBy != "" {
		source.SignedBy = opts.SignedBy
	}
	return nil
}

func splitList(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func parseListContent(content string) (*ppa.SourceFile, error) {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		return ppa.ParseListLine(line)
	}
	return nil, errors.New("no list entries found")
}

func ensureOutputPath(path string, force bool, out io.Writer, errOut io.Writer) error {
	if path == "" {
		return errors.New("output path is empty")
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("checking output path %q: %w", path, err)
	}
	if force {
		return nil
	}
	ok, err := clitext.ConfirmOverwrite(path, os.Stdin, errOut)
	if err != nil {
		return fmt.Errorf("reading overwrite confirmation: %w", err)
	}
	if !ok {
		return fmt.Errorf("aborted: %s was not overwritten", path)
	}
	return nil
}

func ensureWriteAccess(path string, dry bool) error {
	if path == "" {
		return errors.New("output path is empty")
	}
	if dry || os.Geteuid() == 0 {
		return nil
	}
	if path == "" {
		return nil
	}
	dir := filepath.Dir(path)
	if dir == "" {
		dir = "."
	}
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("target directory %q does not exist: this command can only write system files when run with sudo.%s", dir, sudoCommandBlock())
		}
		return fmt.Errorf("checking target directory %q: %w; this command can only write system files when run with sudo.%s", dir, err, sudoCommandBlock())
	}
	file, err := os.CreateTemp(dir, ".ppa-utils-write-check-*")
	if err != nil {
		if os.IsPermission(err) || errors.Is(err, os.ErrPermission) {
			return fmt.Errorf("write permission denied for %q: this command can only write system files when run with sudo.%s", path, sudoCommandBlock())
		}
		return fmt.Errorf("checking write access for %q: %w", path, err)
	}
	name := file.Name()
	_ = file.Close()
	_ = os.Remove(name)
	return nil
}

func sudoCommandHint() string {
	if len(os.Args) == 0 {
		return "sudo ppa-utils"
	}
	quoted := make([]string, 0, len(os.Args))
	for _, arg := range os.Args[1:] {
		quoted = append(quoted, shellQuote(arg))
	}
	return "sudo " + shellQuote(os.Args[0]) + " " + strings.Join(quoted, " ")
}

func sudoCommandBlock() string {
	return "\n\n    " + sudoCommandHint() + "\n\n"
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	if !strings.ContainsAny(value, " \t\n\r'\"\\$`()[]{};&|<>*") {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func promptSource(opts *Options) (*ppa.SourceFile, error) {
	fmt.Fprintf(opts.Out, "Creating apt PPA source file.\n\n")
	info, err := distinfo.GetCurrentDistributionInfo()
	if err != nil {
		info.Codename = "stable"
	}

	var archs []ppa.Architecture
	if runtime.GOARCH == "arm64" {
		archs = []ppa.Architecture{ppa.ArchitectureARM64}
	} else {
		archs = []ppa.Architecture{ppa.ArchitectureAMD64, ppa.ArchitectureARM64}
	}

	source := &ppa.SourceFile{
		Types:         []string{"deb"},
		Suites:        []string{info.Codename},
		Components:    []string{"main"},
		Architectures: archs,
	}
	if info.Codename == "" {
		source.Suites = []string{"stable"}
	}
	if opts.Types != "" {
		source.Types = splitList(opts.Types)
	}
	if len(source.Types) == 0 {
		source.Types = []string{"deb"}
	}

	uriDefault := opts.URI
	if uriDefault == "" {
		uriDefault = "https://example.com/repo"
	}
	source.URIs = []string{opts.AskPrompt("URI", uriDefault)}
	if source.URIs[0] == "" {
		source.URIs = []string{uriDefault}
	}

	suiteDefault := opts.Suite
	if suiteDefault == "" {
		suiteDefault = source.Suites[0]
	}
	source.Suites = []string{opts.AskPrompt("Suite", suiteDefault)}
	if source.Suites[0] == "" {
		source.Suites = []string{suiteDefault}
	}
	if source.Suites[0] == "" {
		source.Suites = []string{"stable"}
	}

	componentsDefault := opts.Components
	if componentsDefault == "" {
		componentsDefault = "main"
	}
	components := opts.AskPrompt("Components", componentsDefault)
	if components == "" {
		components = componentsDefault
	}
	source.Components = splitList(components)
	if len(source.Components) == 0 {
		source.Components = []string{"main"}
	}

	archDefault := opts.Architectures
	if archDefault == "" {
		archDefault = strings.Join(toStrings(source.Architectures), ", ")
	}
	archInput := opts.AskPrompt("Architectures", archDefault)
	if archInput != "" {
		source.Architectures = nil
		for _, arch := range splitList(archInput) {
			source.Architectures = append(source.Architectures, ppa.ParseArchitecture(arch))
		}
	}
	if len(source.Architectures) == 0 {
		source.Architectures = archs
	}

	signedByDefault := opts.SignedBy
	signedBy := opts.AskPrompt("Signed-By (leave empty to skip)", signedByDefault)
	if signedBy != "" {
		source.SignedBy = signedBy
	}
	if signedBy == "" && opts.SignedBy != "" {
		source.SignedBy = opts.SignedBy
	}

	if opts.Output == "" {
		name := deriveSourceNameFromURI(source.URIs[0])
		if name == "" {
			name = "ppa"
		}
		defaultFile := name + ".sources"
		fileName := opts.AskPrompt("Source file name", defaultFile)
		if fileName == "" {
			fileName = defaultFile
		}
		fileName = filepath.Base(fileName)
		if fileName == "." || fileName == "" {
			fileName = defaultFile
		}
		opts.Output = filepath.Join(ppa.DEFAULT_SOURCE_FILE_PATH, fileName)
	}
	fmt.Fprintf(opts.Out, "\n----------------------------------------\n")
	return source, nil
}

func handleMissingSignedBy(opts *Options, source *ppa.SourceFile) error {
	if source == nil || source.SignedBy == "" {
		return nil
	}
	if _, err := os.Stat(source.SignedBy); err == nil {
		return nil
	}

	if opts.Dry {
		fmt.Fprintf(opts.Out, "Dry run: signed-by key %q is missing and would need to be downloaded or installed before the source is usable\n", source.SignedBy)
		return nil
	}

	if !isInteractive(opts) {
		return fmt.Errorf("signed-by file %q is missing; download it or provide a valid path before creating this source", source.SignedBy)
	}

	choice := clitext.Prompt("Signing key file is missing; choose [s]kip, [d]ownload URL, or [a]bort", "s", opts.In, opts.Out)
	switch strings.ToLower(strings.TrimSpace(choice)) {
	case "", "s", "skip":
		fmt.Fprintf(opts.Out, "Continuing without Signed-By: download the key manually before using this source.\n")
		source.SignedBy = ""
		return nil
	case "d", "download":
		url := clitext.Prompt("Download key URL", "", opts.In, opts.Out)
		if strings.TrimSpace(url) == "" {
			return errors.New("no key download URL provided; aborting source creation")
		}
		if err := downloadKey(url, source.SignedBy); err != nil {
			return fmt.Errorf("downloading key from %q: %w", url, err)
		}
		return nil
	case "a", "abort":
		return errors.New("aborted source creation because the signing key file is missing")
	default:
		return fmt.Errorf("unknown choice %q; choose skip, download, or abort", choice)
	}
}

func isInteractive(opts *Options) bool {
	if opts == nil || opts.In == nil {
		return true
	}
	if file, ok := opts.In.(*os.File); ok {
		return term.IsTerminal(file.Fd())
	}
	return true
}

func downloadKey(url, target string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %s", resp.Status)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return os.WriteFile(target, content, 0o644)
}

func toStrings(in []ppa.Architecture) []string {
	out := make([]string, 0, len(in))
	for _, arch := range in {
		out = append(out, string(arch))
	}
	return out
}
