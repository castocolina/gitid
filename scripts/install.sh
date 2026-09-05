#!/bin/sh
# gitid installer — detect OS/arch, resolve the release tag (pinned via
# GITID_VERSION, a channel via GITID_CHANNEL, an interactive menu, or the
# default /releases/latest redirect), download the matching
# gitid_<version>_<os>_<arch>.tar.gz archive plus its versioned
# gitid_<version>_checksums.txt manifest from the same base, verify SHA-256
# BEFORE extracting, then install the extracted "gitid" binary to the
# install directory.
#
# Canonical one-liner (D-14):
#   curl -fsSL https://raw.githubusercontent.com/castocolina/gitid/main/scripts/install.sh | sh
#
# POSIX only: piped into whatever /bin/sh the caller has. pipefail is not
# POSIX and must not appear. The script must never read from stdin: under
# a curl pipe, stdin IS the script — every interactive `read` below is
# scoped to the usable-tty branch and always reads from the explicit
# /dev/tty fd (D-20), never bare stdin.
#
# GITID_INSTALL_BASE_URL is a pure origin substitution (a testing seam),
# never a verification or logic bypass (REVIEW C-2): it replaces ONLY the
# scheme+host part of every DOWNLOAD/REDIRECT URL this script builds
# (GITHUB_ORIGIN below — release page redirects, archive/checksums
# downloads). GITID_INSTALL_API_BASE_URL is the analogous seam for the
# GitHub releases API (API_ORIGIN below) — added in D-20 because, in real
# production use, the API lives at a genuinely DIFFERENT host
# (api.github.com) than github.com's download/redirect URLs; one shared
# origin cannot correctly serve both URL families for a real install. Tests
# point BOTH seams at the same fixture server, which serves both path
# shapes, so this remains a pure origin substitution — never a simplified
# or bypassed logic path — just doubled for the one new host family. Every
# other line of logic — tag resolution, asset/checksums filename
# construction, download paths, checksum-before-extract — runs
# UNCONDITIONALLY whether these are the real hosts or a test fixture
# server.
#
# GITID_VERSION pins an exact release tag (e.g. "v1.0.0", WITH the "v"
# prefix) and skips latest-resolution / channel / menu logic entirely —
# this is the real, already-specified pinned-install production path
# (D-14), not a test shortcut.
#
# GITID_CHANNEL=stable|nightly (D-20) selects a resolution channel when
# GITID_VERSION is unset. "stable" behaves EXACTLY like the pre-D-20
# default (latest non-prerelease via the /releases/latest redirect) — it
# adds no new code path, only a name for the existing one. "nightly"
# resolves the newest release whose tag matches the `release-nightly` make
# target's `v0.0.0-nightly.<timestamp>.<sha>` shape via the GitHub releases
# API (no gh/git dependency — curl + POSIX text tools only, matching this
# script's existing constraints); /releases/latest cannot resolve a
# nightly since GitHub marks it prerelease and excludes it from that
# redirect.
#
# Left with BOTH GITID_VERSION and GITID_CHANNEL unset, and run under a
# genuinely usable controlling terminal (see the /dev/tty open-probe
# below), the script instead lists the most recent releases (stable and
# nightly) and lets the user pick one from a numbered menu — reusing the
# proven `{ : < /dev/tty; } 2>/dev/null` technique from
# castocolina/wezterm-setup's tools/install.sh (an actual open-attempt, not
# `test -t 0` or `[ -e /dev/tty ]` — some containers expose a /dev/tty node
# that exists/is readable but fails to open with ENXIO) adapted for this
# script's own tar.gz asset shape; wezterm-setup's own repo-fetch/asset-
# naming mechanics are NOT reused, only the /dev/tty-revival idea. Left
# unset with NO usable tty (the ordinary curl|sh case — stdin is the
# script), the script keeps today's exact headless behavior with zero
# behavior change (must_have: byte-for-byte unchanged).
#
# GITID_ASSUME_HEADLESS=1 is a test-only seam (mirrors wezterm-setup's
# WEZ_ASSUME_HEADLESS) that forces the headless branch deterministically
# regardless of whether /dev/tty is actually openable — the real detection
# in normal use is always the open-probe itself.
#
# GITID_INSTALL_DIR overrides the install target directory (D-14; absent
# from the Phase 9.3 script). Left unset, installs to ~/.local/bin
# (immutable-distro-safe, identical on Bazzite/Fedora/Ubuntu/macOS).

set -eu

REPO="castocolina/gitid"
GITHUB_ORIGIN="${GITID_INSTALL_BASE_URL:-https://github.com}"
API_ORIGIN="${GITID_INSTALL_API_BASE_URL:-https://api.github.com}"
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

# GITID_CHANNEL is documented as an enum of exactly stable|nightly (D-20) —
# reject anything else loudly instead of silently falling through to the
# stable default (a typo like "Nightly"/"nighty" must not be misread as
# "unset", which would install the wrong channel with no warning).
case "${GITID_CHANNEL:-}" in
	""|stable|nightly) ;;
	*) fail "unrecognized GITID_CHANNEL: ${GITID_CHANNEL} (expected \"stable\" or \"nightly\")" ;;
esac

# fetch_release_tags prints the "tag_name" values from a GitHub releases-API
# JSON array response, one per line, in the order the API returned them
# (GitHub sorts releases newest-first) — no jq dependency (grep + awk only,
# matching this script's existing curl+tar-only constraint). Field-splitting
# on the literal `"` character is robust to either compact or pretty-printed
# JSON: for a grep -o match like `"tag_name": "v1.0.0"`, splitting on `"`
# yields fields 1="" 2=tag_name 3=": " 4=v1.0.0 5="" — so $4 is always the
# value, regardless of the whitespace the API used around the colon.
fetch_release_tags() {
	grep -o '"tag_name"[[:space:]]*:[[:space:]]*"[^"]*"' | awk -F'"' '{ print $4 }'
}

# Version/tag resolution (REVIEW C-2 FIX / D-20 extension): GITHUB_ORIGIN and
# API_ORIGIN are the ONLY seams a test fixture server plugs into — every line
# below runs unconditionally, whether they are the real hosts or a test
# fixture server.
if [ -n "${GITID_VERSION:-}" ]; then
	tag="$GITID_VERSION"
elif [ "${GITID_CHANNEL:-}" = "nightly" ]; then
	releases_url="${API_ORIGIN}/repos/${REPO}/releases"
	releases_json=$(curl -fsSL "$releases_url") || fail "could not fetch release list: ${releases_url}"
	nightly_tag=""
	for candidate in $(printf '%s' "$releases_json" | fetch_release_tags); do
		case "$candidate" in
			v0.0.0-nightly.*)
				nightly_tag="$candidate"
				break
				;;
		esac
	done
	if [ -z "$nightly_tag" ]; then
		fail "no nightly release found at ${releases_url}"
	fi
	tag="$nightly_tag"
elif [ -z "${GITID_VERSION:-}" ] && [ -z "${GITID_CHANNEL:-}" ] && [ "${GITID_ASSUME_HEADLESS:-0}" != "1" ] && { true < /dev/tty; } 2>/dev/null; then
	# Usable controlling terminal AND no explicit pin/channel: offer an
	# interactive menu instead of silently defaulting (D-20). The open-probe
	# above `{ true < /dev/tty; } 2>/dev/null` is the ONLY gate — see the
	# header comment for why this is not `test -t 0`. EMPIRICALLY VERIFIED
	# (dash): the probe must use `true`, NOT `:` (the wezterm-setup bash
	# original uses `:`) — POSIX classifies `:` as a "special built-in", and
	# a redirection error on a special built-in unconditionally terminates a
	# non-interactive POSIX-conformant shell (dash enforces this; bash does
	# not), even inside an `if`/`&&` guard and even with `2>/dev/null`. `true`
	# is an ordinary builtin, so a failed `/dev/tty` open here just returns
	# non-zero, exactly as intended, without aborting the whole script — a
	# real, reproduced difference from the bash-only precedent this technique
	# is adapted from.
	releases_url="${API_ORIGIN}/repos/${REPO}/releases"
	releases_json=$(curl -fsSL "$releases_url") || fail "could not fetch release list: ${releases_url}"
	menu_tags=$(printf '%s' "$releases_json" | fetch_release_tags | head -n 10)
	if [ -z "$menu_tags" ]; then
		fail "no releases found at ${releases_url}"
	fi
	printf 'gitid: select a release to install:\n' >&2
	# Word-splitting $menu_tags into positional params is intentional here
	# (tag names never contain whitespace).
	set -- $menu_tags
	n=$#
	i=1
	for t in "$@"; do
		printf '  %d) %s\n' "$i" "$t" >&2
		i=$((i + 1))
	done
	printf 'Enter choice [1-%d] (default: 1): ' "$n" >&2
	choice=""
	# Explicit /dev/tty read (never bare stdin — see header comment); `||
	# true` covers a /dev/tty that is openable but yields EOF on read.
	read -r choice < /dev/tty || true
	if [ -z "$choice" ]; then
		choice=1
	fi
	case "$choice" in
		''|*[!0-9]*) fail "invalid choice: ${choice}" ;;
	esac
	if [ "$choice" -lt 1 ] || [ "$choice" -gt "$n" ]; then
		fail "choice out of range: ${choice} (expected 1-${n})"
	fi
	i=1
	tag=""
	for t in "$@"; do
		if [ "$i" -eq "$choice" ]; then
			tag="$t"
			break
		fi
		i=$((i + 1))
	done
	if [ -z "$tag" ]; then
		fail "internal error resolving menu choice ${choice}"
	fi
else
	# Headless default (byte-for-byte unchanged from the pre-D-20 script):
	# GITID_CHANNEL unset or "stable" both land here, identically — "stable"
	# adds a NAME for this existing path, not a second implementation of it.
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

# WR-02 fix (Phase 10 round-1 review): match the asset name as an exact
# field, not as a `grep -E` regex. version_num — and therefore ${asset} —
# is unvalidated user input (GITID_VERSION); splicing it unescaped into an
# extended-regex pattern let regex metacharacters (parentheses, `.`, `+`,
# etc.) change matching semantics instead of matching the literal
# filename. The checksums manifest's `<hash>  <filename>` two-space format
# makes this an exact-field comparison with no regex involved: awk's
# default field splitting (any whitespace) yields $1=hash, $2=filename.
expected=$(awk -v name="$asset" '$2 == name { print $1; exit }' "${tmp}/${checksums_name}")
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
# Round-2 code-review IN-01 raised: does a glob metacharacter in
# $INSTALL_DIR (possibly the user-supplied GITID_INSTALL_DIR) make this
# `case` pattern match too loosely? No — POSIX shell quoting rules turn OFF
# glob/wildcard interpretation for a quoted substring of a case pattern, and
# "${INSTALL_DIR}" here is quoted; only the bare, unquoted `*` on either side
# is a real wildcard. Verified empirically against bash (macOS's /bin/sh)
# and dash: a directory containing `*`/`?` is matched LITERALLY, never as a
# wildcard, so this was never glob-vulnerable. Left as-is rather than
# rewritten to a manual IFS-split loop, which would add untested complexity
# for a check that was already correct.
case ":$PATH:" in
	*":${INSTALL_DIR}:"*)
		printf '  PATH: OK (gitid is on PATH)\n'
		;;
	*)
		printf '  PATH: %s is NOT on your PATH — add to shell: export PATH="$PATH:%s"\n' "$INSTALL_DIR" "$INSTALL_DIR"
		;;
esac
