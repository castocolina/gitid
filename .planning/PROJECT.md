# gitid — SSH/Git Identity Manager

## What This Is

`gitid` is a single-binary CLI + TUI tool (Go) for a terminal-based developer on
macOS or Linux who juggles multiple Git identities (personal, several clients,
work GitHub/GitLab/Bitbucket). It **owns the identity lifecycle** and keeps
`~/.ssh/config`, `~/.gitconfig`, the per-identity Git fragments, and
`~/.ssh/allowed_signers` coherent. A built-in **doctor** detects and explains
problems, and a **hypothesis → test → implement** flow means nothing is written
until it is proven to work.

## Core Value

Managing a Git identity produces coordinated, coherent SSH + Git artifacts that
are **proven to authenticate and resolve correctly (`ssh -G`) before any file is
written**, and existing hand-written config is never corrupted.

## Requirements

### Validated

<!-- Shipped and confirmed valuable. -->

(None yet — greenfield; ship to validate)

### Active

<!-- Current scope. Building toward these. Detailed, ID'd set lives in REQUIREMENTS.md. -->

- [ ] Identity/Account CRUD: create (ed25519 auth+signing), read/list, update, delete
- [ ] Generate the four coordinated artifacts (ssh/config, gitconfig includeIf, per-identity fragment, allowed_signers) with timestamped backup + idempotent managed blocks + confirmation
- [ ] Two-phase test flow showing input (command) and real output: explicit `ssh -i` test, then resolved `ssh -T <alias>` + `ssh -G`
- [ ] Key rotation/replacement with re-pointed artifacts and re-run test flow
- [ ] Cross-platform clipboard copy of public key (on generate and on demand)
- [ ] Upload instructions for GitHub/GitLab (auth + signing keys)
- [ ] Doctor health checks (deps, permissions, coherence/drift, orphans, signing wiring, agent) with per-OS fixes; runs first in the TUI and as `gitid doctor`
- [ ] `includeIf` match strategy support: `gitdir:` (default) and `hasconfig:remote.*.url` (Phase 1 — needed to render the gitconfig artifact)
- [ ] Minimal TUI (launches to doctor dashboard) + CLI with Cobra shell completion
- [ ] Global/shared git config toggles (Phase 2)
- [ ] `insteadOf` HTTPS→SSH rewriting with editable HTTPS suggestion (Phase 2)
- [ ] Adopt existing plain-style fragments into `~/.gitconfig.d/` (Phase 2)
- [ ] `gitid add repo <url>` workflow: detect provider, personal/client disambiguation, alias rewrite, clone into `~/git/<client>`, verify with pull (Phase 2)

### Out of Scope

<!-- Explicit boundaries with reasoning. -->

- Windows — v1 targets macOS + Linux only; SSH/keychain/clipboard behavior differs materially
- GPG commit signing — replaced by ssh-key signing (`gpg.format=ssh` + `allowed_signers`); one key for auth + signing
- Web UI — terminal-first tool; TUI/CLI cover the use case
- Scheduled / automatic key rotation — rotation is user-initiated only
- Secret-vault integration — keys live in `~/.ssh` with correct permissions

## Context

- **Starting point:** a monolithic Bash script (`ssh-keygen.sh`) that only generates
  an RSA key and blindly appends blocks to `~/.ssh/config`. It manages no lifecycle
  and keeps no coherence. `gitid` replaces it.
- **Reference end-state:** two user-provided gists capture the desired
  `~/.ssh/config` and `~/.gitconfig` structure (host aliases, port 443,
  `IdentitiesOnly yes`, `include`/`includeIf` with `gitdir:` + `hasconfig:`,
  `insteadOf`). Captured verbatim in `.planning/references/`; source pointers in
  project memory. The gists use RSA — the PRD supersedes this with ed25519.
- **Directory layout:** the user organizes repos under `~/git/<client>/…`, which
  drives the default `gitdir:` match strategy and the `add repo` clone destination.
- **Source of truth:** the real files only — no sidecar database. On startup the
  tool parses its sentinel-delimited managed blocks to reconstruct identities and
  accounts (zero drift). Anything outside managed blocks is preserved verbatim.

## Constraints

- **Tech stack**: Go + Bubble Tea (TUI) + Cobra (CLI) — single static binary, no runtime dependency, native macOS/Linux
- **Architecture**: thin CLI/TUI shells over a tested, UI-free core (`identity`, `sshconfig`, `gitconfig`, `doctor`, `keygen`, `tester`, `clipboard`, `deps`, `platform`)
- **Safety**: never mutate user files without timestamped backup, idempotent whole-block managed-block rewrite (never blind append), correct permissions (`~/.ssh` 700, key 600, `.pub` 644, `config` 600), and explicit confirmation
- **Engineering**: core is built test-first (TDD); config parse/render is round-trip safe (parse → render → parse is stable)
- **Quality tooling**: enforced via `pre-commit` hooks — `gofmt`/`goimports` (format), `golangci-lint` (lint aggregator), `staticcheck` (static analysis), `gosec` (security), `unused`/`deadcode` (dead-code), and `go test` (unit tests with coverage). Pre-commit and CI invoke the same `make` targets
- **Build automation**: a `Makefile` is the single task runner — at minimum `setup-env` (bootstrap: install tools + pre-commit hooks), `build`, `install`, `uninstall`, `test`, `lint`, `fmt`. Hooks and CI call these targets, not ad-hoc commands
- **Commit hygiene**: no repetitive fix-up commits; keep history clear and **compact (squash) after every plan close + user review**
- **Signing**: one ed25519 key per identity for both auth and signing via `gpg.format=ssh` + `allowed_signers`; no GPG
- **Platform**: macOS + Linux only; macOS `Host *` uses `UseKeychain yes` + `AddKeysToAgent yes` guarded by `IgnoreUnknown UseKeychain`; clipboard/dependency hints branch per OS
- **Language**: all generated content (code, comments, UI text, logs, docs, commits) in English

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Tool/binary name = `gitid` | Chosen over `gid`/`sgc` — explicit, discoverable, lower collision risk | — Pending |
| Default clone base dir = `~/git` | Matches the user's existing `~/git/<client>/` layout | — Pending |
| Defer `insteadOf` + `add repo` to Phase 2 | Core value is delivered without them; `add repo` depends on the Phase 1 identity foundation; keeps the MVP lean and low-risk | — Pending |
| `includeIf` match strategy stays in Phase 1 | The `gitdir:`/`hasconfig:` match is required to render the `~/.gitconfig` artifact when creating an identity | — Pending |
| ed25519, one key per identity, auth + signing, no GPG | PRD signing model; simpler than GPG, single key to manage | — Pending |
| Go quality toolchain: golangci-lint + staticcheck + gosec + unused/deadcode + go test | Go equivalents of the requested ruff/pyright/vulture/bandit/unittest standard | — Pending |
| `Makefile` as single task runner; `pre-commit` hooks call `make` targets | One source of truth for build/test/lint across local + CI; `make setup-env` bootstraps the dev environment | — Pending |
| Squash/compact commit history after each plan close + user review | Keep history clear; avoid repetitive fix-up commits | — Pending |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd-complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-06-08 after initialization*
