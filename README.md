# gitid

`gitid` manages Git identities by coordinating SSH keys and Git configuration
in a single, safe, auditable operation.

## What gitid manages (objective)

The north star is the annotated reference configuration in [`recipes/`](./recipes/) —
real `~/.ssh/config` and `~/.gitconfig` setups that let multiple identities
(work / personal / …) resolve **automatically per repository**, with no manual
switching. `gitid` exists to produce and keep that wiring coherent, end to end:

- **`~/.ssh/config`** — one `Host <identity>.<provider>` alias per identity, with
  an explicit `IdentityFile` and `IdentitiesOnly yes`.
- **`~/.gitconfig`** — `includeIf` rules (by remote URL `hasconfig:` or by
  `gitdir:`) that load the right per-identity fragment, plus `insteadOf` URL
  rewriting.
- **`~/.gitconfig.d/<identity>`** — per-identity fragment (name, email, signing).
- **`~/.ssh/allowed_signers`** + one **ed25519** key per identity (auth + commit
  signing via `gpg.format=ssh`).
- **…and more** (doctor diagnostics, key rotation, multi-account aliases).

See [`recipes/README.md`](./recipes/README.md) for provenance and the exact target shape.

## Platforms

Released binaries cover four targets:

| OS | Architecture | CI-gated |
|----|--------------|----------|
| macOS | Intel (`darwin/amd64`) | Yes |
| macOS | Apple Silicon (`darwin/arm64`) | Yes |
| Linux | x86_64 (`linux/amd64`) | Yes |
| Linux | arm64 (`linux/arm64`) | Best-effort (built, not CI-gated) |

Linux validation runs against the Fedora family (a `fedora:latest` container job
in CI, plus a manual UAT on real Bazzite hardware per release). See
[`PLATFORM-NOTES.md`](./PLATFORM-NOTES.md) for the current per-distro status —
check it before installing on an uncommon Linux setup.

## Install

`gitid` mutates `~/.ssh` and `~/.gitconfig`, so before running anything,
inspect what you're about to run. The manual path below lets you do that;
the one-liner further down automates the same steps.

### Manual install (inspect first)

1. Download the archive for your platform and the checksums manifest from the
   [Releases](https://github.com/castocolina/gitid/releases) page — e.g. for
   Linux x86_64:

   ```sh
   curl -fsSLO https://github.com/castocolina/gitid/releases/download/v<version>/gitid_<version>_linux_amd64.tar.gz
   curl -fsSLO https://github.com/castocolina/gitid/releases/download/v<version>/gitid_<version>_checksums.txt
   ```

2. Verify the SHA-256 checksum **before** extracting anything:

   ```sh
   # Linux
   sha256sum --ignore-missing -c gitid_<version>_checksums.txt

   # macOS
   shasum -a 256 --ignore-missing -c gitid_<version>_checksums.txt
   ```

3. Extract and install:

   ```sh
   tar -xzf gitid_<version>_linux_amd64.tar.gz gitid
   chmod +x gitid
   mv gitid ~/.local/bin/   # or any directory on your PATH
   ```

### curl | sh one-liner (`scripts/install.sh`)

Once you've read the script (it's the same download/verify/extract sequence
above, automated):

```sh
curl -fsSL https://raw.githubusercontent.com/castocolina/gitid/main/scripts/install.sh | sh
```

The script auto-detects your OS/architecture, downloads the matching
`gitid_<version>_<os>_<arch>.tar.gz` archive and its `gitid_<version>_checksums.txt`
manifest from the latest GitHub Release, verifies the SHA-256 **before**
extracting, and installs to `~/.local/bin/gitid` — no sudo. It refuses to
install an archive whose checksum does not match.

Two environment variables configure it:

| Variable | Effect | Default |
|----------|--------|---------|
| `GITID_VERSION` | Pin an exact release tag (e.g. `v1.0.0`, with the `v` prefix) instead of resolving GitHub's latest release | resolves `/releases/latest` |
| `GITID_INSTALL_DIR` | Override the install directory | `~/.local/bin` |

```sh
GITID_VERSION=v1.0.0 GITID_INSTALL_DIR="$HOME/bin" \
  curl -fsSL https://raw.githubusercontent.com/castocolina/gitid/main/scripts/install.sh | sh
```

If the install directory is not on PATH, the installer prints the exact
`export PATH` line to add.

### Homebrew tap

```sh
brew install castocolina/homebrew-tap/gitid
```

Works on macOS and on Linux, including Bazzite/Fedora — Homebrew ships
preinstalled on Universal Blue images and is the documented CLI channel there.

### `go install`

```sh
go install github.com/castocolina/gitid/cmd/gitid@latest
```

Caveat: this path builds from source rather than downloading a goreleaser-built
release archive, so it isn't stamped with `-ldflags`. `gitid --version` falls
back to Go's `runtime/debug.ReadBuildInfo()`-derived module version (with a
`+dirty` suffix if applicable) instead of an exact release tag — the binary is
fully functional, only the reported version string's provenance differs.

### Verify the install

```sh
gitid --version
```

Expect a line of the form `gitid version 1.2.3 (abc1234, 2026-08-30, darwin/arm64)`.

## Quick Start

```sh
make setup-env   # install toolchain dependencies and git hooks
make build       # build the gitid binary
make test        # run the test suite
make lint        # run the linter
```

## Status

The identity workflow described above is implemented and covered by tests.
The project is pre-1.0; the roadmap is still open.
