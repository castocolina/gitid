#!/bin/sh
# gitid installer — detect OS/arch, resolve the release tag (pinned via
# GITID_VERSION or discovered from GitHub's /releases/latest redirect),
# download the matching gitid_<version>_<os>_<arch>.tar.gz archive plus its
# versioned gitid_<version>_checksums.txt manifest from the same base,
# verify SHA-256 BEFORE extracting, then install the extracted "gitid"
# binary to the install directory.
#
# Canonical one-liner (D-14):
#   curl -fsSL https://raw.githubusercontent.com/castocolina/gitid/main/scripts/install.sh | sh
#
# POSIX only: piped into whatever /bin/sh the caller has. pipefail is not
# POSIX and must not appear. The script must never read from stdin: under
# a curl pipe, stdin IS the script.
#
# GITID_INSTALL_BASE_URL is a pure origin substitution (a testing seam),
# never a verification or logic bypass (REVIEW C-2): it replaces ONLY the
# scheme+host part of every URL this script builds (GITHUB_ORIGIN below).
# Every other line of logic — tag resolution, asset/checksums filename
# construction, download paths, checksum-before-extract — runs
# UNCONDITIONALLY whether GITHUB_ORIGIN is the real github.com or a test
# fixture server, so overriding it substitutes one self-consistent origin
# for another rather than disabling anything.
#
# GITID_VERSION pins an exact release tag (e.g. "v1.0.0", WITH the "v"
# prefix) and skips latest-resolution entirely — this is the real,
# already-specified pinned-install production path (D-14), not a test
# shortcut. Left unset (the default), the script resolves the latest
# release tag via GitHub's own /releases/latest redirect.
#
# GITID_INSTALL_DIR overrides the install target directory (D-14; absent
# from the Phase 9.3 script). Left unset, installs to ~/.local/bin
# (immutable-distro-safe, identical on Bazzite/Fedora/Ubuntu/macOS).

set -eu

REPO="castocolina/gitid"
GITHUB_ORIGIN="${GITID_INSTALL_BASE_URL:-https://github.com}"
INSTALL_DIR="${GITID_INSTALL_DIR:-${HOME}/.local/bin}"
BIN_NAME="gitid"

fail() {
	printf 'gitid: %s\n' "$1" >&2
	exit 1
}

if ! command -v curl >/dev/null 2>&1; then
	fail "curl is required to download the release archive"
fi

if ! command -v tar >/dev/null 2>&1; then
	fail "tar is required to extract the release archive"
fi

# Allowlists, not convenience mappings: an unexpected uname output must
# never reach a URL (T-09.3-02 / ASVS V5). Both refusals happen BEFORE any
# network activity — including latest-tag redirect resolution below.
os_raw=$(uname -s)
arch_raw=$(uname -m)

case "$os_raw" in
	Darwin) os_tag=darwin ;;
	Linux) os_tag=linux ;;
	*) fail "unsupported operating system: ${os_raw}" ;;
esac

case "$arch_raw" in
	x86_64|amd64) arch_tag=amd64 ;;
	arm64|aarch64) arch_tag=arm64 ;;
	*) fail "unsupported architecture: ${arch_raw}" ;;
esac

# Version resolution (REVIEW C-2 FIX): GITHUB_ORIGIN is the ONE seam a test
# fixture server plugs into — every line below runs unconditionally,
# whether GITHUB_ORIGIN is the real github.com or a test fixture server.
if [ -n "${GITID_VERSION:-}" ]; then
	tag="$GITID_VERSION"
else
	latest_url="${GITHUB_ORIGIN}/${REPO}/releases/latest"
	resolved=$(curl -fsSL -o /dev/null -w '%{url_effective}' "$latest_url") || fail "could not resolve latest release: ${latest_url}"
	case "$resolved" in
		*/tag/*) tag="${resolved##*/tag/}" ;;
		*) fail "could not determine release tag from redirect: ${resolved}" ;;
	esac
fi

# goreleaser's {{.Version}} template (which the archive/checksums filenames
# embed, D-07/D-15) strips a leading "v" from the git tag — mirror that
# here so the constructed filenames match exactly.
version_num="${tag#v}"

asset="gitid_${version_num}_${os_tag}_${arch_tag}.tar.gz"
checksums_name="gitid_${version_num}_checksums.txt"

tmp=$(mktemp -d)
# POSIX guarantees 0 as the exit-time condition and numeric signals 1/2/15
# (HUP/INT/TERM); symbolic trap names are a widely-implemented extension,
# not a portable guarantee for a #!/bin/sh script piped into a stranger's
# shell.
trap 'rm -rf "$tmp"' 0 1 2 15

asset_url="${GITHUB_ORIGIN}/${REPO}/releases/download/${tag}/${asset}"
checksums_url="${GITHUB_ORIGIN}/${REPO}/releases/download/${tag}/${checksums_name}"

if ! curl -fsSL "$asset_url" -o "${tmp}/${asset}"; then
	fail "download failed: ${asset_url}"
fi
if ! curl -fsSL "$checksums_url" -o "${tmp}/${checksums_name}"; then
	fail "download failed: ${checksums_url}"
fi

if command -v sha256sum >/dev/null 2>&1; then
	checksum_cmd="sha256sum"
elif command -v shasum >/dev/null 2>&1; then
	checksum_cmd="shasum -a 256"
else
	fail "no SHA-256 tool found (need sha256sum or shasum)"
fi

expected=$(grep -E "^[0-9a-f]{64}  ${asset}\$" "${tmp}/${checksums_name}" | head -n 1 | cut -d ' ' -f 1)
if [ -z "$expected" ]; then
	fail "${checksums_name} has no entry for ${asset} — refusing to install an unverified archive"
fi

# checksum_cmd is deliberately unquoted: `shasum -a 256` must word-split
# into three arguments.
actual=$($checksum_cmd "${tmp}/${asset}" | cut -d ' ' -f 1)
if [ "$expected" != "$actual" ]; then
	fail "checksum verification FAILED for ${asset} (expected ${expected}, actual ${actual}) — refusing to install an unverified archive"
fi

# Extraction happens strictly AFTER verification (D-14). Only the known,
# exact relative path "gitid" is pulled out of the archive — never a
# trusted glob/arbitrary archive entry (tar path-traversal mitigation,
# T-10-05-03).
if ! (cd "$tmp" && tar -xzf "$asset" gitid); then
	fail "extraction failed for ${asset}"
fi
if [ ! -f "${tmp}/${BIN_NAME}" ]; then
	fail "extracted archive did not contain ${BIN_NAME}"
fi

chmod 0755 "${tmp}/${BIN_NAME}"
mkdir -p "$INSTALL_DIR"
dest="${INSTALL_DIR}/${BIN_NAME}"
if ! mv "${tmp}/${BIN_NAME}" "$dest"; then
	fail "install failed: could not write ${dest}"
fi

printf '  verified: SHA-256 %s\n' "$actual"
printf '  installed: %s\n' "$dest"
case ":$PATH:" in
	*":${INSTALL_DIR}:"*)
		printf '  PATH: OK (gitid is on PATH)\n'
		;;
	*)
		printf '  PATH: %s is NOT on your PATH — add to shell: export PATH="$PATH:%s"\n' "$INSTALL_DIR" "$INSTALL_DIR"
		;;
esac
