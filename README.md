# ppautil

Check and manage your PPAs before an Ubuntu upgrade.

When you upgrade Ubuntu, every PPA you use has to publish packages for the new
release. If one does not, apt can no longer update or install packages from it
afterwards. `ppautil` tells you which of your PPAs will not be supported by the
next Ubuntu release, so you know whether it is safe to upgrade, which tells you if
you would lose access to one or more PPAs.

## Requirements

- An Ubuntu system
- Go 1.25 or newer, to install from source

## Installation

```sh
go install yv35.com/ppa/ppa-utils/cmd/ppautil@latest
```

This puts the `ppautil` binary into `$(go env GOPATH)/bin`. Make sure that
directory is on your `PATH`.

## Usage

### Check if it is safe to upgrade

`check` looks at every PPA in `/etc/apt/sources.list.d` that is tied to an
Ubuntu release and asks its repository whether it publishes the target release.
Each source is marked as supported (✅) or not supported (❌).

```sh
# Check against the release you are about to upgrade to
ppautil check

# Check against a specific release, e.g. "noble"
ppautil check noble
```

Without an argument the target is the next Ubuntu release after the one you
are running. Sources that do not mention a release (for example a PPA that
publishes one suite for all releases) are treated as supported and not listed.

### List your sources

`list` shows the apt sources on your system with their status, URI, suites,
components, architectures and signing key.

```sh
# All sources as a table
ppautil list

# Only sources that use the suite "resolute"
ppautil list --suite resolute

# Enabled Launchpad PPAs for arm64
ppautil list --uri ppa.launchpadcontent.net --arch arm64 --status enabled

# As JSON, for scripts
ppautil list --json
```

Filters match when the column contains the given text, ignoring case. When you
give several filters, all of them must match.

### Browse and manage sources interactively

```sh
ppautil list --interactive
```

This opens a view similar to `htop`. You can filter and sort live and enable or
disable the selected source file. Press `?` for the key bindings.

- `/` filters. Plain text matches any column, `suite:noble` matches one column.
- `<` and `>` choose the sort column, `I` inverts it.
- `e`, `d` and Space enable, disable or toggle a source, after a confirmation.
- `q` quits.

Changing a file needs root. `sudo` asks for your password when required.
Official Ubuntu sources cannot be disabled. Run `sudo apt update` afterwards.

If the output is not a terminal, `--interactive` prints the plain table.

## Shell completion

`ppautil` can generate completion scripts for bash, zsh, fish and PowerShell.
Release codes are completed for `check`, and flag values for `list`.

```sh
# bash: load it in the current session
source <(ppautil completion bash)

# zsh: load it in the current session
source <(ppautil completion zsh)

# fish
ppautil completion fish | source
```

To load it in every new session, write the script to your shell's completion
directory, for example:

```sh
ppautil completion bash | sudo tee /etc/bash_completion.d/ppautil
```

Run `ppautil completion <shell> --help` for the details of each shell.

## Limitations

- Only deb822 `.sources` files are read. Legacy one-line `.list` files are not.
- Enabling or disabling applies to the whole file, including all its entries.
- A release is recognized by its code (`noble`) or its version (`24.04`) in the
  URI, suites or components. Other naming schemes are not detected.

## Development

Clone the repository and use the `Makefile`. Run `make help` to list the goals.

| Goal          | Example                                | Usage                                                                                   |
| ------------- | -------------------------------------- | --------------------------------------------------------------------------------------- |
| `build`       | `make build`                           | Build all binaries into `bin/`. Currently the same as `build-cli`.                      |
| `build-cli`   | `make build-cli VERSION=0.1.0`         | Build `bin/ppautil` and set the version shown by `ppautil version`.                     |
| `run`         | `make run ARGS="check noble"`          | Run the CLI with `go run`, without building a binary. `ARGS` are passed to `ppautil`.   |
| `test`        | `make test ARGS="-run TestSetEnabled"` | Run the unit tests of `./internal/...`. `ARGS` are passed to `go test`.                 |
| `clean`       | `make clean`                           | Delete the built binaries in `bin/`.                                                    |
| `clean-cache` | `make clean-cache`                     | Clear the Go build cache and the module cache. Modules are downloaded again afterwards. |
| `help`        | `make help`                            | Show the available goals.                                                               |

A typical change looks like this:

```sh
make test
make run ARGS="list --suite noble"
make build-cli && ./bin/ppautil version
```

### Project layout

- `cmd/ppautil` only registers the commands.
- `internal/cli/commands/<name>` holds one package per command.
- `internal/cli/root`, `clitext`, `sourcelist` and `tui` are shared by the commands.
- `internal/ppa` and `internal/distinfo` read the apt sources and the Ubuntu release data.
