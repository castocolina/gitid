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

## Quick Start

```sh
make setup-env   # install toolchain dependencies and git hooks
make build       # build the gitid binary
make test        # run the test suite
make lint        # run the linter
```

## Status

Work in progress — Phase 1 (Bootstrap) scaffolding only.
