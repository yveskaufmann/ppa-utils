// Package selfupdate defines the ppa-utils self-update command.
package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"yv35.com/ppa-utils/internal/cli/clitext"
	"yv35.com/ppa-utils/internal/version"
)

const (
	defaultBaseURL = "https://github.com/yveskaufmann/ppa-utils"
	binaryName     = "ppa-utils"
	// maxDownload caps every download to keep a bad response from filling memory.
	maxDownload = 200 << 20
)

// Options holds everything the self-update command needs to run.
type Options struct {
	Check bool
	Yes   bool

	CurrentVersion string
	BaseURL        string
	GOOS           string
	GOARCH         string
	Executable     func() (string, error)
	Client         *http.Client

	In  io.Reader
	Out io.Writer
}

// New returns the self-update command. When fn is not nil it is called with the
// parsed options instead of running the command.
func New(fn func(*Options) error) *cobra.Command {
	opts := &Options{
		CurrentVersion: version.Version,
		BaseURL:        defaultBaseURL,
		GOOS:           runtime.GOOS,
		GOARCH:         runtime.GOARCH,
		Executable:     os.Executable,
		Client:         &http.Client{Timeout: 5 * time.Minute},
	}

	cmd := &cobra.Command{
		Use:   "self-update",
		Short: "Update ppa-utils to the latest GitHub release",
		Long: clitext.Long(`
			Update ppa-utils to the latest release published on GitHub.

			The release archive is verified against the release's checksums.txt and
			then replaces the running binary. If the binary lives in a directory you
			cannot write to, run the command with sudo.
		`),
		Example: clitext.Examples(`
			# Check whether a newer release exists
			ppa-utils self-update --check

			# Update without asking for confirmation
			ppa-utils self-update --yes
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts.In = cmd.InOrStdin()
			opts.Out = cmd.OutOrStdout()

			if fn != nil {
				return fn(opts)
			}
			return run(opts)
		},
	}
	cmd.Flags().BoolVar(&opts.Check, "check", false, "only report whether a newer release exists")
	cmd.Flags().BoolVar(&opts.Yes, "yes", false, "do not ask for confirmation before updating")
	return cmd
}

func run(opts *Options) error {
	if opts.CurrentVersion == "dev" {
		return errors.New(`cannot update a development build (version "dev")`)
	}
	arch, err := releaseArch(opts.GOOS, opts.GOARCH)
	if err != nil {
		return err
	}

	tag, err := latestTag(opts.Client, opts.BaseURL)
	if err != nil {
		return err
	}
	cmp, err := compareVersions(opts.CurrentVersion, tag)
	if err != nil {
		return err
	}
	current, latest := strings.TrimPrefix(opts.CurrentVersion, "v"), strings.TrimPrefix(tag, "v")
	if cmp >= 0 {
		fmt.Fprintf(opts.Out, "ppa-utils %s is up to date.\n", current)
		return nil
	}
	if opts.Check {
		fmt.Fprintf(opts.Out, "Update available: %s -> %s\n", current, latest)
		return nil
	}

	exe, err := opts.Executable()
	if err != nil {
		return fmt.Errorf("find the running binary: %w", err)
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return fmt.Errorf("resolve the running binary: %w", err)
	}
	if err := checkWritable(filepath.Dir(exe)); err != nil {
		return err
	}

	if !opts.Yes {
		ok, err := clitext.Confirm(fmt.Sprintf("Update ppa-utils %s -> %s?", current, latest), opts.In, opts.Out)
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(opts.Out, "Aborted.")
			return nil
		}
	}

	binary, err := fetchBinary(opts.Client, opts.BaseURL, tag, arch)
	if err != nil {
		return err
	}
	if err := replaceBinary(exe, binary); err != nil {
		return err
	}
	fmt.Fprintf(opts.Out, "Updated ppa-utils %s -> %s (%s)\n", current, latest, exe)
	return nil
}

// releaseArch maps the platform to the architecture used in release archive names.
func releaseArch(goos, goarch string) (string, error) {
	if goos != "linux" {
		return "", fmt.Errorf("self-update only supports Linux, not %s", goos)
	}
	switch goarch {
	case "amd64", "arm64":
		return goarch, nil
	}
	return "", fmt.Errorf("unsupported architecture: %s", goarch)
}

// latestTag resolves the newest release tag by following the /releases/latest
// redirect, the same way install.sh does.
func latestTag(client *http.Client, baseURL string) (string, error) {
	resp, err := client.Get(baseURL + "/releases/latest")
	if err != nil {
		return "", fmt.Errorf("find the latest release: %w", err)
	}
	resp.Body.Close()

	p := resp.Request.URL.Path
	if resp.StatusCode != http.StatusOK || !strings.Contains(p, "/releases/tag/") {
		return "", errors.New("no release found")
	}
	return path.Base(p), nil
}

// fetchBinary downloads the release archive for the tag, verifies it against
// checksums.txt and returns the ppa-utils binary from it.
//
// The archive naming matches install.sh and .goreleaser.yaml.
func fetchBinary(client *http.Client, baseURL, tag, arch string) ([]byte, error) {
	archive := fmt.Sprintf("%s_%s_linux_%s.tar.gz", binaryName, strings.TrimPrefix(tag, "v"), arch)
	base := baseURL + "/releases/download/" + tag + "/"

	data, err := download(client, base+archive)
	if err != nil {
		return nil, err
	}
	sums, err := download(client, base+"checksums.txt")
	if err != nil {
		return nil, err
	}
	if err := verifyChecksum(sums, archive, data); err != nil {
		return nil, err
	}
	return extractBinary(data, binaryName)
}

func download(client *http.Client, url string) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: %s", url, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxDownload+1))
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	if len(data) > maxDownload {
		return nil, fmt.Errorf("download %s: file too large", url)
	}
	return data, nil
}

// verifyChecksum compares the sha256 of data with the entry for name in sums,
// which uses the sha256sum format "<hash>  <name>".
func verifyChecksum(sums []byte, name string, data []byte) error {
	for _, line := range strings.Split(string(sums), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || strings.TrimPrefix(fields[1], "*") != name {
			continue
		}
		sum := sha256.Sum256(data)
		if !strings.EqualFold(fields[0], hex.EncodeToString(sum[:])) {
			return fmt.Errorf("checksum mismatch for %s", name)
		}
		return nil
	}
	return fmt.Errorf("no checksum for %s in checksums.txt", name)
}

// extractBinary returns the regular file called name from a tar.gz archive.
func extractBinary(archive []byte, name string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("read archive: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("%s not found in the archive", name)
		}
		if err != nil {
			return nil, fmt.Errorf("read archive: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg || path.Base(hdr.Name) != name {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(tr, maxDownload+1))
		if err != nil {
			return nil, fmt.Errorf("read archive: %w", err)
		}
		if len(data) > maxDownload {
			return nil, errors.New("read archive: binary too large")
		}
		return data, nil
	}
}

// createTemp creates a temporary file next to the binary and turns a
// permission error into a hint to rerun with sudo.
func createTemp(dir string) (*os.File, error) {
	f, err := os.CreateTemp(dir, ".ppa-utils-update-*")
	if errors.Is(err, fs.ErrPermission) {
		return nil, fmt.Errorf("%s is not writable; rerun with sudo", dir)
	}
	return f, err
}

// checkWritable fails early if the binary's directory cannot be written to.
func checkWritable(dir string) error {
	f, err := createTemp(dir)
	if err != nil {
		return err
	}
	f.Close()
	return os.Remove(f.Name())
}

// replaceBinary swaps the file at exe for data. The new file is written next
// to it first and renamed over it, so the replacement is atomic.
func replaceBinary(exe string, data []byte) (err error) {
	f, err := createTemp(filepath.Dir(exe))
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			os.Remove(f.Name())
		}
	}()

	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Chmod(f.Name(), 0o755); err != nil {
		return err
	}
	return os.Rename(f.Name(), exe)
}

// compareVersions returns -1, 0 or 1 if a is older than, equal to or newer than b.
// Both are numeric X.Y.Z versions with an optional leading "v".
func compareVersions(a, b string) (int, error) {
	pa, err := parseVersion(a)
	if err != nil {
		return 0, err
	}
	pb, err := parseVersion(b)
	if err != nil {
		return 0, err
	}
	for i := range pa {
		switch {
		case pa[i] < pb[i]:
			return -1, nil
		case pa[i] > pb[i]:
			return 1, nil
		}
	}
	return 0, nil
}

func parseVersion(v string) ([3]int, error) {
	var out [3]int
	parts := strings.Split(strings.TrimPrefix(v, "v"), ".")
	if len(parts) != 3 {
		return out, fmt.Errorf("unsupported version %q, expected X.Y.Z", v)
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, fmt.Errorf("unsupported version %q, expected X.Y.Z", v)
		}
		out[i] = n
	}
	return out, nil
}
