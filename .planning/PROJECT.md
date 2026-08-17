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

## Current Milestone: v1.0 — TUI-First Redesign

**Goal:** Ship the **real first release** of gitid — a design-driven,
screenshot-verified terminal app that creates and manages coherent SSH + Git
identities. Redefined from the archived **0.0.1 POC** (never released; it surfaced
the real goals and the better way to build them).

**Target features:**
- **Design-first delivery** (a first-class requirement): HTML/`mui` mockups →
  screenshots → Go TUI dummy mockup → **user design approval (the one checkpoint)** →
  backend → e2e → visual-regression review, with `agent-ui-ux-designer` on every UI task.
- **Create flow**: algorithm catalog (ed25519 default + rsa-4096, platform-aware) →
  SSH screen (`Alias prefix` / `SSH Host` / `Real hostname` / `Port`, clickable) →
  two-stage test showing exact commands → Git screen → review.
- **Identity manager**: completeness/health state taxonomy, clone (same or new key),
  delete all-vs-git-only, per-identity health.
- **Global SSH options** (danger-aware) + **Global Git options** (main/master, eol,
  case, email, recipe defaults).
- **Health** (SSH + Git sections) + **Fixer**.
- **SSH storage**: in-file blocks / gitid-owned `Include` file / adopt external (feasibility verified with real `ssh -G`).
- **CI/CD**: cross-platform builds (macOS Intel/ARM, Linux); credential upload auto when `gh`/`glab` authenticated.

**Full ID'd set:** `.planning/REQUIREMENTS.md` (sections A–P). Goal driver: `.planning/ONESHOT-GOAL-PROMPT.md`.

## Requirements

### Validated

<!-- Shipped and confirmed valuable. -->

(None released. The 0.0.1 POC proved the core mechanics — safe writes, `ssh -G`
prove-before-write, doctor, temp-config testing — which are reused as substrate.)

### Active

<!-- Current scope = v1.0 redesign. Detailed, ID'd set lives in REQUIREMENTS.md (A–P). -->

- [ ] **Delivery method (DLV)**: design-first HTML mockups + screenshot pipeline +
  visual-regression review gate on every UI wave
- [ ] **Key/Algorithm (KEY)**: platform-aware algorithm catalog, ed25519 + rsa-4096 keygen
- [ ] **Create flow (SSHUI/TEST/STORE/GITUI)**: new SSH field model, two-stage test,
  dual SSH storage, separate git-config screen + review
- [ ] **Identity manager (MGR)**: state taxonomy, clone, new-key, delete all-vs-git-only, per-identity health
- [ ] **Global SSH options (GSSH)** — danger-aware; **Global Git options (GGIT)**
- [ ] **Health (HLTH)** two-section + **Fixer (FIX)**
- [ ] **Platform (PLAT)**: macOS/Linux capability probing + variants
- [ ] **Build CI/CD (BUILD)**: cross-platform release builds + CI gates on both OSes

### Out of Scope

<!-- Explicit boundaries with reasoning. -->

- Windows — macOS + Linux only; SSH/keychain/clipboard behavior differs materially
- GPG commit signing — replaced by ssh-key signing (`gpg.format=ssh` + `allowed_signers`)
- **Shippable Web UI** — the HTML/`mui` mockups are **design + review artifacts only**
  (living design docs + screenshot references); the shipped product is the terminal app
- Scheduled / automatic key rotation — user-initiated only
- Secret-vault integration — keys live in `~/.ssh` with correct permissions
- CI/CD algorithm *fallback* — gitid is local-use; algorithm choice is a local user
  preference, not a server/CI compatibility workaround (build CI/CD is in scope, see BUILD)

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
| Prior build reframed as archived **0.0.1 POC**; redesign is the real **v1.0** | Nothing was ever released; POC surfaced the real goals + a better build method. Honest semver (0.x = pre-release) beats "2.0 with no released 1.0" | ✅ 2026-07-02 |
| **Design-first, screenshot-verified** delivery (HTML `mui` mockup → TUI dummy → visual-regression gate) | UX quality becomes a gate, not an afterthought; `agent-ui-ux-designer` + `/mui` on every UI task | ✅ 2026-07-02 |
| **One human checkpoint** (design approval); credential upload auto when `gh`/`glab` authenticated | Maximize autonomy of the build loop while keeping the irreversible design decision human-owned | ✅ 2026-07-02 |
| Algorithm **picker** (ed25519 default + rsa-4096), local-use, **macOS/Linux variant-aware** | Supersedes ed25519-only; probes local `ssh-keygen`; no CI/CD fallback logic | ✅ 2026-07-02 |
| SSH storage **dual**: in-file blocks / gitid-owned `Include` file / adopt external | Verified with real `ssh -G`: absolute Include paths resolve, first-match-wins ⇒ Include near top | ✅ 2026-07-02 |
| **Build CI/CD** for macOS Intel/ARM + Linux (GitHub Actions) | Cross-platform release binaries + CI gates on both OSes catch PLAT divergences | ✅ 2026-07-02 |

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
*Last updated: 2026-07-02 — milestone v1.0 (TUI-First Redesign) started; prior build archived as 0.0.1 POC*
