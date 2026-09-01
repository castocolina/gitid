#!/bin/sh
# gitid installer — detect OS/arch, download the matching GitHub Release
# asset plus checksums.txt from the same base URL, verify SHA-256, then
# install to ~/.local/bin.
#
# Canonical one-liner (D-07):
#   curl -fsSL https://raw.githubusercontent.com/castocolina/gitid/main/scripts/install.sh | sh
#
# POSIX only: piped into whatever /bin/sh the caller has. pipefail is not
# POSIX and must not appear. The script must never read from stdin: under
# a curl pipe, stdin IS the script.
#
# An optional download-base override is a testing seam that substitutes
# the fetch origin for both the binary AND checksums.txt. It is not a
# verification bypass — both fetches always come from the SAME base, so
# overriding it substitutes one self-consistent release for another
# rather than disabling the check.

set -eu

REPO="castocolina/gitid"
BASE_URL="${GITID_INSTALL_BASE_URL:-https://github.com/${REPO}/releases/latest/download}"
INSTALL_DIR="${HOME}/.local/bin"
BIN_NAME="gitid"

fail() {
	printf 'gitid: %s\n' "$1" >&2
	exit 1
}

if ! command -v curl >/dev/null 2>&1; then
	fail "curl is required to download the release binary"
fi

# Allowlists, not convenience mappings: an unexpected uname output must
# never reach the URL (T-09.3-02 / ASVS V5). Both refusals happen BEFORE
# any URL is built.
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

asset="gitid-${os_tag}-${arch_tag}"

tmp=$(mktemp -d)
# POSIX guarantees 0 as the exit-time condition and numeric signals 1/2/15
# (HUP/INT/TERM); symbolic trap names are a widely-implemented extension,
# not a portable guarantee for a #!/bin/sh script piped into a stranger's
# shell.
trap 'rm -rf "$tmp"' 0 1 2 15

if ! curl -fsSL "${BASE_URL}/${asset}" -o "${tmp}/${BIN_NAME}"; then
	fail "download failed: ${BASE_URL}/${asset}"
fi
if ! curl -fsSL "${BASE_URL}/checksums.txt" -o "${tmp}/checksums.txt"; then
	fail "download failed: ${BASE_URL}/checksums.txt"
fi

if command -v sha256sum >/dev/null 2>&1; then
	checksum_cmd="sha256sum"
elif command -v shasum >/dev/null 2>&1; then
	checksum_cmd="shasum -a 256"
else
	fail "no SHA-256 tool found (need sha256sum or shasum)"
fi

expected=$(grep -E "^[0-9a-f]{64}  ${asset}\$" "${tmp}/checksums.txt" | head -n 1 | cut -d ' ' -f 1)
if [ -z "$expected" ]; then
	fail "checksums.txt has no entry for ${asset} — refusing to install an unverified binary"
fi

# checksum_cmd is deliberately unquoted: `shasum -a 256` must word-split
# into three arguments.
actual=$($checksum_cmd "${tmp}/${BIN_NAME}" | cut -d ' ' -f 1)
if [ "$expected" != "$actual" ]; then
	fail "checksum verification FAILED for ${asset} (expected ${expected}, actual ${actual}) — refusing to install an unverified binary"
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
