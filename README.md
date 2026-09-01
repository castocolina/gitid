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

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/castocolina/gitid/main/scripts/install.sh | sh
```

The script detects the machine's OS and architecture, downloads the matching
binary and `checksums.txt` from the latest GitHub Release, verifies the SHA-256,
and installs to `~/.local/bin/gitid` — no sudo. It refuses to install a binary
whose checksum does not match.

Published platforms: macOS Intel, macOS Apple Silicon, Linux x86_64, Linux arm64.

After install, run `gitid --version`. Expect a line of the form
`gitid version 1.2.3 (abc1234, 2026-08-30)`.

If `~/.local/bin` is not on PATH, the installer prints the exact `export PATH`
line to add.

### Manual install

Download the one asset for your platform plus `checksums.txt` from the
[Releases](https://github.com/castocolina/gitid/releases) page. Verify against
the matching manifest line — `checksums.txt` covers all four binaries, so
checking the whole file reports the other three as missing. Substitute your
asset name in the filter:

```sh
# Linux
grep ' gitid-linux-amd64$' checksums.txt | sha256sum -c -

# macOS
grep ' gitid-linux-amd64$' checksums.txt | shasum -a 256 -c -
```

Then `chmod +x` the binary and move it onto PATH.

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
