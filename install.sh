#!/usr/bin/env bash
# Installs the latest ppa-utils release from GitHub.
#
#   curl -fsSL https://raw.githubusercontent.com/yveskaufmann/ppa-utils/main/install.sh | bash
#
# Environment variables:
#   VERSION      Release to install, e.g. v1.2.3 (default: latest release)
#   INSTALL_DIR  Target directory (default: $HOME/.local/bin)

set -euo pipefail

REPO="yveskaufmann/ppa-utils"
BINARY="ppa-utils"

# die prints an error message and exits.
die() {
  echo "error: $*" >&2
  exit 1
}

# need aborts if a required command is missing.
need() {
  command -v "$1" >/dev/null 2>&1 || die "'$1' is required but not installed"
}

# detect_arch maps the machine architecture to the GoReleaser archive name.
detect_arch() {
  case "$(uname -m)" in
    x86_64 | amd64) echo "amd64" ;;
    aarch64 | arm64) echo "arm64" ;;
    *) die "unsupported architecture: $(uname -m)" ;;
  esac
}

# latest_tag resolves the newest release tag by following the
# /releases/latest redirect, which needs no API token and has no rate limit.
latest_tag() {
  local url
  url="$(curl -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/${REPO}/releases/latest")" ||
    die "could not reach GitHub to find the latest release"
  case "$url" in
    */releases/tag/*) echo "${url##*/}" ;;
    *) die "no release found for ${REPO}" ;;
  esac
}

# verify_checksum checks the archive against the line in checksums.txt.
verify_checksum() {
  local dir="$1" archive="$2" line
  line="$(grep -F " ${archive}" "${dir}/checksums.txt" || true)"
  [ -n "$line" ] || die "no checksum for ${archive} in checksums.txt"
  if command -v sha256sum >/dev/null 2>&1; then
    (cd "$dir" && echo "$line" | sha256sum -c --status -) || die "checksum mismatch for ${archive}"
  elif command -v shasum >/dev/null 2>&1; then
    (cd "$dir" && echo "$line" | shasum -a 256 -c --status -) || die "checksum mismatch for ${archive}"
  else
    die "sha256sum or shasum is required to verify the download"
  fi
}

# main downloads, verifies and installs the release.
main() {
  need curl
  need tar

  [ "$(uname -s)" = "Linux" ] || die "ppa-utils only supports Linux"

  local arch tag version archive base tmp install_dir
  arch="$(detect_arch)"
  tag="${VERSION:-$(latest_tag)}"
  case "$tag" in v*) ;; *) tag="v${tag}" ;; esac
  version="${tag#v}"
  archive="${BINARY}_${version}_linux_${arch}.tar.gz"
  base="https://github.com/${REPO}/releases/download/${tag}"
  install_dir="${INSTALL_DIR:-$HOME/.local/bin}"

  tmp="$(mktemp -d)"
  # Expand now: $tmp is local to main and gone when the EXIT trap runs.
  trap "rm -rf '${tmp}'" EXIT

  echo "Installing ${BINARY} ${tag} (linux/${arch}) to ${install_dir}"
  curl -fsSL -o "${tmp}/${archive}" "${base}/${archive}" ||
    die "could not download ${base}/${archive}"
  curl -fsSL -o "${tmp}/checksums.txt" "${base}/checksums.txt" ||
    die "could not download ${base}/checksums.txt"

  verify_checksum "$tmp" "$archive"

  tar -xzf "${tmp}/${archive}" -C "$tmp" "$BINARY"
  mkdir -p "$install_dir"
  install -m 0755 "${tmp}/${BINARY}" "${install_dir}/${BINARY}"

  echo "Installed ${install_dir}/${BINARY}"
  case ":${PATH}:" in
    *":${install_dir}:"*) ;;
    *) echo "Note: ${install_dir} is not on your PATH. Add it, e.g. to ~/.profile." ;;
  esac
}

# The script runs only after it was downloaded completely.
main "$@"
