# Phase 1: Foundations, Spikes & CI - Research

**Researched:** 2026-07-02
**Domain:** Go CLI/TUI tooling — screenshot capture pipelines, multi-algorithm SSH keygen,
local capability probing, SSH `Include`-file config management, identity state
classification, cross-OS GitHub Actions CI.
**Confidence:** MEDIUM-HIGH (mixed: HIGH for everything grounded in this repo's existing,
building code and empirically re-verified commands; MEDIUM for third-party tool choices
verified via WebSearch + successful `go get`/`go build`; LOW/flagged where noted)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**1. Screenshot tooling (TOOL-05, DLV-03)**
- **D-01 — TUI capture = View()-dump → PNG.** ◆ Claude's discretion (recommended).
  Capture the Bubble Tea model's `View()` string deterministically at a fixed size
  (teatest-style, **no real PTY**), write it as a versioned `.txt` golden, then render
  that ANSI to PNG. Real-PTY driving is reserved for DLV-06 e2e (Phase 3), a separate
  concern.
- **D-02 — ANSI→PNG renderer = `charmbracelet/freeze`.** ◆ Claude's discretion
  (recommended). Pin it as a dev tool installed by `make setup-env` (not a runtime dep
  of the gitid binary). *Alternative if freeze proves unfit:* `aha` (ANSI→HTML) +
  headless-chrome screenshot.
- **D-03 — HTML capture = scripted headless Chromium via a `make` target**, callable by
  the autonomous loop (NOT the Playwright MCP, which is agent-only). ◆ Claude's
  discretion (recommended). Phase 1 builds the **tooling** and proves it against a
  trivial fixture HTML page (mockups don't exist until Phase 2).
- **D-04 — Artifact layout.** PNGs (and TUI `.txt` goldens) under
  `.planning/design/<surface>/{html,tui}/*.png`, versioned in git. Fixed capture
  geometry (e.g. 100×30 cols/rows for TUI) recorded so later diffs are apples-to-apples.

**2. Keygen + probing scope (KEY-01/02/03, PLAT-01/02)**
- **D-05 — Real generators in Phase 1 = ed25519 (default) + rsa-4096 ONLY** (locked by
  KEY-02). The architecture is an **algorithm registry/interface** so `ecdsa-p256` and
  the `-sk` hardware variants slot in later **without redesign** — but they are not
  generated in this phase.
- **D-06 — Catalog carries all 5 entries** (`ed25519`, `ed25519-sk`, `rsa-4096`,
  `ecdsa-p256`, `ecdsa-sk`) with security + per-OS availability/variant metadata; the 3
  without generators are marked "not-yet-implemented / probe-gated." Final ordering +
  copy is deferred to Phase 2 design.
- **D-07 — Probe depth (PLAT-01).** Probe: `ssh-keygen -Q key` (supported key types),
  `ssh -V` (version + LibreSSL-vs-OpenSSL flavor), presence of `libfido2`/`ssh-sk-helper`
  (for `-sk`), a running `ssh-agent`, and macOS keychain support
  (`ssh-add --apple-use-keychain` / `UseKeychain`). ◆ Claude's discretion on the exact
  probe set — recommended floor. The probe is behind an **injectable seam** (mockable in
  tests).
- **D-08 — Surface = a debug/list command** (e.g. `gitid keygen catalog` / a
  `debug caps` subcommand) that prints the catalog + resolved local availability;
  proven by tests. ◆ Claude's discretion on the exact command name. This same debug
  surface hosts the state-taxonomy readout (D-11).

**3. Include'd SSH layout (STORE-01/02/03)**
- **D-09 — Include layout = ONE gitid-owned file** `~/.ssh/config.d/gitid.config`,
  pulled in by a single `Include ~/.ssh/config.d/*.config` line placed **near the TOP**
  of `~/.ssh/config` (first-match-wins, verified with real `ssh -G`). ◆ Claude's
  discretion (recommended). *Rejected for now:* per-identity files.
- **D-10 — Adopt + migrate.**
  - *Adopt (STORE-02):* detect an existing `Include` directive already in
    `~/.ssh/config`; if it targets a dir/file where gitid's blocks belong, adopt that
    path instead of creating `config.d`. Detection scans for `Include` lines + gitid
    sentinels (reuse `internal/adopter` **pattern**, not its current gitconfig-only code).
  - *Migrate (STORE-03):* reversible move of managed blocks between in-file and
    Include'd layouts, each direction with timestamped backup + idempotent whole-block
    rewrite, proven by round-trip + real `ssh -G`. Include-line placement uses the
    existing `filewriter` block-**prepend** capability. Include paths MUST be absolute
    or `~/.ssh`-relative (verified: relative paths silently fail).

**4. Identity state-taxonomy core (MGR-02, DLV-07)**
- **D-11 — States are LOCKED by MGR-02** (8: complete / incomplete / git-only /
  key-unused / key-used-ssh-only / key-used-both / key-missing / fragment-path-missing).
  Computed by the UI-free TDD core from parsed managed blocks, **no sidecar DB**.
  Phase-1 surface = the same debug/list command as D-08 (prints each identity's state).
  No UI. Proven by table-driven tests over fixture configs.

**5. CI matrix + gate depth (BUILD-01/02/04)**
- **D-12 — Runners = 3 native:** `ubuntu-latest` (linux/amd64), `macos-13` (Intel,
  darwin/amd64), `macos-14` (Apple Silicon, darwin/arm64). ◆ Claude's discretion
  (recommended). **⚠ SUPERSEDED — see "State of the Art" below: macos-13 is fully
  unsupported as of Dec 2025 and macos-14 begins deprecating Jul 6 2026 (this week).
  This research recommends `macos-15-intel` + `macos-15` instead — flagged for user
  confirmation.**
- **D-13 — Gate depth = FULL native gates on all three runners:** `make test` (`-race`)
  + `make lint` (golangci-lint + gosec) + `make test-e2e`. ◆ Claude's discretion
  (recommended). *Cost lever if PR minutes bite:* keep test+lint on every PR across all
  3, but gate `test-e2e` to `push`/`main` only — flagged for the user, default is
  full-on-PR.
- **D-14 — Build matrix (BUILD-01):** cross-compile all targets reproducibly via
  `make build` (GOOS/GOARCH); darwin/arm64 additionally verified natively. Add a
  **build-only** (ungated) `linux/arm64` cross-compile "if cheap." Release/tag
  publishing (BUILD-03) is Phase 10, out of scope here.
- **D-15 — Bootstrap (BUILD-04):** CI verifies `make setup-env` reproduces the
  toolchain (golangci-lint, gosec, pre-commit, hooks) from a fresh clone on both macOS
  and Linux.

### Claude's Discretion
Tagged inline as ◆ on D-01, D-02, D-03, D-07 (probe set), D-08 (command name), D-09,
D-12, D-13. Recommended defaults the user should confirm or override before planning.
All other decisions are locked by REQUIREMENTS/ROADMAP/recipes.

### Deferred Ideas (OUT OF SCOPE)
- **Real-PTY visual capture** for TUI screenshots — DLV-06 e2e (Phase 3) already drives
  the real binary via PTY (see `e2e/ui_pty_e2e_test.go`, already built). Revisit only if
  View()-dump PNGs prove insufficient.
- **Per-identity Include files** (`config.d/<identity>.config`) — the `*.config` glob
  leaves room; not built now.
- **ecdsa-p256 / ed25519-sk / ecdsa-sk real generators** — catalog lists them;
  generators are additive later.
- **`linux/arm64` gated CI** (native runner) — build-only cross-compile only.
- **KEY-01 final catalog ordering + copy** — Phase 2 (design phase).
- **Visual-regression diff engine** — DLV-04, Phase 3.

</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| TOOL-01 | Makefile exposes setup-env/build/install/uninstall/test/lint/fmt | Already exists (`Makefile`, verified by reading it) — no new work, CI must call these targets |
| TOOL-02 | `make setup-env` bootstraps golangci-lint, gosec, pre-commit, hooks | Already exists — add `freeze` + browser-driver toolchain per Standard Stack |
| TOOL-03 | pre-commit hooks run fmt/lint/security/tests via same make targets; `--no-verify` forbidden | Already wired (`install-hooks` target) — verify CI uses identical targets |
| TOOL-04 | Core is TDD; parse→render→parse round-trip proven by tests | Established pattern (`internal/sshconfig/marker_roundtrip_test.go`) — extend to Include-file round trip |
| TOOL-05 | Screenshot tooling: TUI + HTML capture as `make` target(s) | See "Screenshot Tooling" architecture pattern — `freeze` (TUI) + `go-rod` (HTML) |
| DLV-03 | Screenshot pipeline: both TUI and HTML screens captured to versioned artifacts | Same as TOOL-05; artifact layout under `.planning/design/<surface>/{html,tui}/` |
| DLV-07 | UI-free TDD core; parse→render→parse round-trip stable | Carried convention; extend round-trip tests to state taxonomy + Include-file writer |
| KEY-01 | Top-5 algorithm catalog with per-OS availability + default recommendation | Catalog data structure driven by `platform` probe; ordering deferred to Phase 2 |
| KEY-02 | Real multi-algorithm keygen (ed25519 default + rsa-4096); registry leaves room for ecdsa/-sk | Refactor `internal/keygen/keygen.go` from single-algo `if` into a registry map |
| KEY-03 | Platform-aware troubleshooting when chosen algorithm misbehaves locally | Probe-driven hints; reuse `platform.InstallHint` pattern, extend for libfido2/keychain/agent |
| KEY-04 | Correct permissions (`~/.ssh` 700, key 600, `.pub` 644, config 600) | Already correct via `filewriter.Write`/`filewriter.EnsureDir` — no change needed for new algos |
| STORE-01 | Dual strategy: in-file blocks OR gitid-owned Include'd file | New work in `internal/sshconfig`; reuse `filewriter.PrependBlockIfNotFound` pattern from `internal/gitconfig/baseline.go` |
| STORE-02 | Adopt an existing external Include'd ssh file | New work — detection logic distinct from `internal/adopter` (which is gitconfig-fragment-only today) |
| STORE-03 | Safe, backed-up, reversible migration between layouts | New work; reuse `filewriter.Write` (backup+atomic) + round-trip parse verification |
| STORE-04 | Safe writes: backup + idempotent block rewrite + atomic + confirm | Already the established invariant (`internal/filewriter`) — apply to the new Include writer |
| MGR-02 | 8-state identity taxonomy computed by UI-free core, no sidecar DB | New work in `internal/identity` (no existing `State` type — `Account.Incomplete` is a partial precursor) |
| PLAT-01 | Capability probing drives catalog/troubleshooting/doctor hints | Extend `internal/platform` (currently only `ProbeKeyTypes`) with agent/keychain/libfido2 probes |
| PLAT-02 | macOS vs Linux variant handling (Keychain/agent, LibreSSL/OpenSSL, clipboard, install hints) | `platform.SupportsUseKeychain` + `InstallHint` already exist; extend for libfido2/`ssh -V` parsing |
| BUILD-01 | Cross-platform build matrix (darwin/amd64, darwin/arm64, linux/amd64 [+linux/arm64 build-only]) | New `.github/workflows/ci.yml`; `make build` already supports GOOS/GOARGH cross-compile via `go build` |
| BUILD-02 | CI gates (`make test` -race, `make lint`, `make test-e2e`) on macOS + Linux, block merge | New CI YAML; targets already exist and pass locally |
| BUILD-04 | `make setup-env` reproduces toolchain from fresh clone on both OSes | Already exists; CI job must invoke it fresh (no cache reuse across the verification step) |

</phase_requirements>

## Project Constraints (from CLAUDE.md)

- **Recipes are the North Star** — read `recipes/README.md` + both `.recipe` files before
  planning (done — see Sources). Structure only, not RSA key type.
- **Hypothesis → test → implementation** working method: surface ambiguity BEFORE
  planning; every claim in this document that could not be empirically verified is
  marked `[ASSUMED]`.
- **All generated content is English-only** — code, comments, commit messages, this
  document.
- **Never write to `~/.ssh/config` / `~/.gitconfig` without backup + idempotent managed
  blocks + confirmation** — already the `internal/filewriter` invariant; the new
  Include-file writer and migration logic MUST go through the same chokepoint.
- **Commits in logical groups**, TDD authoring order (RED before GREEN), never
  `--no-verify`.
- **Pinned stack versions** (from CLAUDE.md's own Recommended Stack table, already
  installed and building in `go.mod`): Go 1.26.x, `github.com/spf13/cobra` v1.10.2,
  `charm.land/bubbletea/v2` v2.0.7, `charm.land/lipgloss/v2` v2.0.3,
  `charm.land/bubbles/v2` v2.1.0, `golang.org/x/crypto` v0.53.0,
  `github.com/kevinburke/ssh_config` v1.6.0, golangci-lint v2.12.2. **This research
  does not propose changing any of these** — new dependencies (below) are additive.
- **golangci-lint via binary installer, never `go install`** (Go-version mismatch risk)
  — already the Makefile pattern; keep it.
- **depguard rule**: `internal/doctor` must never import `internal/filewriter` — any new
  probe/state code that doctor consumes must stay read-only and receive writes via
  injected `Deps` closures, consistent with the existing pattern.

## Summary

Phase 1 is five largely independent workstreams bolted onto a **substantial, already-
building Go codebase** (`internal/keygen`, `internal/platform`, `internal/sshconfig`,
`internal/filewriter`, `internal/identity`, `internal/adopter`, `internal/doctor`,
`internal/tester`, plus a working `Makefile` and 8 e2e test files). This is not a
greenfield spike — it is a **refactor-and-extend** phase, and the research below is
grounded primarily in reading and empirically testing that existing code rather than in
external documentation, except for the three genuinely new third-party tools
(`charmbracelet/freeze`, a headless-Chromium Go driver, and the GitHub Actions runner
matrix).

The single most important finding of this research is a **correction to CONTEXT.md
D-12**: `macos-13` runner images are **fully unsupported since December 2025** and
`macos-14` **begins deprecating July 6, 2026 — four days from today's date**. The CI
matrix must target `macos-15-intel` (darwin/amd64) and `macos-15` or `macos-latest`
(darwin/arm64) instead. This is verified via GitHub's own changelog and docs (HIGH
confidence) and must be corrected before/during planning, not discovered when the
workflow file fails to schedule.

The second major finding is that **`internal/adopter` is not reusable for STORE-02**
despite the name: it adopts `~/.gitconfig_<name>` fragment files via gitconfig
`includeIf`, an entirely different file (`~/.gitconfig`) and directive
(`includeIf`) from the SSH `Include` directive in `~/.ssh/config` that STORE-01/02/03
need. The **pattern** is reusable (detect → migrate/reference → backup), but the
**code** is not — STORE-02/03 is new work in `internal/sshconfig`.

Third, the exact primitive STORE-01 needs already exists and is proven:
`filewriter.PrependBlockIfNotFound` places a managed block as a **floor** (top) rather
than a ceiling (append), and is already used for gitconfig's `baseline-include` block.
The SSH-side `Include ~/.ssh/config.d/*.config` line should use the identical function,
with a reserved-block-name guard analogous to `gitconfig.IsReservedBlockName` (this
guard doesn't exist yet in `internal/sshconfig` and must be added — its absence is a
documented recurring bug class in this project, see Common Pitfalls).

Fourth, this session **empirically verified** (not just searched) three facts on the
actual dev machine that matter for KEY-01/PLAT-01: (1) `ssh -Q key` reports the FIDO2
ed25519 variant as the token `sk-ssh-ed25519@openssh.com` — NOT `ed25519-sk` or
`ssh-ed25519-sk` as informally written elsewhere in the requirements/context docs; (2)
`ssh -V` on this machine prints `OpenSSH_9.7p1, LibreSSL 3.3.6`, confirming the
documented parse format; (3) `charmbracelet/freeze` was downloaded, built, and run
headlessly on this machine, producing a real 640×332 PNG from `--execute` output with
zero display/GUI dependency — confirming CI/headless-Linux viability in principle
(font handling still needs an explicit `--font.file` to be CI-deterministic, see
Pitfalls).

**Primary recommendation:** Treat this as a refactor phase — extend
`internal/keygen`/`internal/platform`/`internal/sshconfig`/`internal/identity` in place
using their established injectable-`Deps`/`filewriter`-chokepoint/round-trip-test
patterns, add two new third-party dev/build dependencies (`charmbracelet/freeze`,
`go-rod/rod`), and correct the CI runner matrix before writing `.github/workflows/ci.yml`.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| TUI screenshot capture (View()-dump → PNG) | Dev/build tooling (Makefile + `freeze`) | UI-free core (the `tea.Model.View()` under test) | Capture happens outside the shipped binary; it is a test/CI artifact generator, not runtime behavior |
| HTML screenshot capture (headless Chromium) | Dev/build tooling (Makefile + Go driver) | — | Same — mockups don't exist in the shipped product at all (design-review artifact only) |
| Multi-algorithm keygen | UI-free core (`internal/keygen`) | OS/filesystem (`~/.ssh/id_*`) | Pure crypto + serialization logic with injected file writes; no CLI/TUI concern |
| Local capability probing | UI-free core (`internal/platform`) | External tooling (shells to `ssh`, `ssh-keygen`) | Business logic (parsing/classifying probe output) stays pure; only the exec call touches the OS |
| SSH config dual storage (in-file / Include'd) | UI-free core (`internal/sshconfig`, `internal/filewriter`) | OS/filesystem (`~/.ssh/config`, `~/.ssh/config.d/`) | Parse/render/write logic is pure and round-trip-tested; the chokepoint (`filewriter.Write`) is the only OS touchpoint |
| Adopt/migrate SSH Include layout | UI-free core (new code in `internal/sshconfig`) | OS/filesystem | Detection is pure string/AST inspection over `ssh_config.Config`; migration re-uses the write chokepoint |
| Identity state taxonomy (8 states) | UI-free core (`internal/identity`) | — | Pure function over `Reconstruct()` output + key-file existence checks; explicitly "no sidecar DB" (MGR-02) |
| Debug/list command surface | CLI (Cobra, `cmd/gitid`) | UI-free core | Cobra command is thin glue that calls into `platform`/`keygen`/`identity` and prints; no logic lives in the command itself |
| Cross-OS CI build+gate | CI (GitHub Actions) | Dev/build tooling (Makefile targets) | CI is pure orchestration — it must call the SAME `make` targets a human runs locally (already the project's stated invariant) |

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `golang.org/x/crypto/ssh` | v0.53.0 (already pinned, in `go.mod`) | ed25519 + rsa-4096 key generation, OpenSSH PEM serialization | `[VERIFIED: codebase]` — already used in `internal/keygen/keygen.go`; `MarshalPrivateKey`/`MarshalAuthorizedKey`/`NewPublicKey` signatures confirmed working at this exact pinned version in this exact repo |
| `github.com/kevinburke/ssh_config` | v1.6.0 (already pinned) | Parse/render `~/.ssh/config`, including resolving `Include` directives via `filepath.Glob` | `[VERIFIED: codebase + official repo]` — already used in `internal/sshconfig/parser.go`; `Include` support confirmed present (resolves relative paths against `~/.ssh`, absolute paths as-is, per official repo description) |
| `github.com/spf13/cobra` | v1.10.2 (already pinned) | CLI command surface for the new debug/list command (D-08) | `[VERIFIED: codebase]` — already the CLI framework (`cmd/gitid`) |

### Supporting — new for this phase

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/charmbracelet/freeze` | v0.2.2 (latest at research time) | ANSI terminal output → PNG rendering (TOOL-05 TUI half) | Dev-only tool installed by `make setup-env` (`go install .../freeze@v0.2.2`), invoked by a new `screenshot-tui` make target — NOT a runtime import of the gitid binary |
| `github.com/go-rod/rod` | v0.116.2 (latest at research time) | Headless-Chromium driver for HTML screenshot capture (TOOL-05 HTML half) | Small Go program (`internal/screenshot` or a `tools/` package) invoked by a new `screenshot-html` make target; NOT a runtime import of the gitid binary — build with a separate `go.mod`-scoped build tag or a `tools/` submodule to avoid bloating the shipped binary's dependency graph (see Architecture Patterns) |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `charmbracelet/freeze` | `aha` (ANSI→HTML) + headless-Chromium screenshot | Adds a second external CLI binary dependency and an extra HTML round-trip; freeze is single-purpose and already produces PNG directly. Rejected by CONTEXT.md D-02 unless freeze proves unfit in the spike. |
| `go-rod/rod` for HTML capture | `chromedp/chromedp` | Both are viable, well-maintained pure-Go CDP drivers with no Node.js dependency. `go-rod` was chosen because its `launcher` package auto-downloads a headless-shell Chromium binary when none is found locally — this satisfies BUILD-04 ("`make setup-env` reproduces the toolchain from a fresh clone on both OSes") without adding a system-package-manager browser-install step. `chromedp` typically expects a pre-installed browser (community forks add auto-download but are less mainstream). Both are MEDIUM-confidence choices — no Context7 access this session; verify with a Phase-1 spike task before committing further. |
| `go-rod`/`chromedp` (pure Go) | Node.js + Playwright/Puppeteer CLI | Would introduce an entirely new language toolchain (npm/Node) into a project that is currently 100% Go (no `package.json` in the repo). Rejected — violates the project's single-toolchain simplicity and `make setup-env` reproducibility goal. |
| Custom algorithm `if`-chain (current `internal/keygen/keygen.go`) | Algorithm registry (`map[string]Generator`) | Current code hard-codes `if p.Algo != "ed25519" { error }`. KEY-02 explicitly requires "architecture leaves room... without a redesign" — a registry is the only way to satisfy that without a second refactor in a later phase. |

**Installation:**
```bash
# Dev tools (added to `make setup-env`, NOT go.mod runtime deps unless go-rod ends up
# imported by a `tools/` build):
go install github.com/charmbracelet/freeze@v0.2.2

# go-rod is a real Go module dependency of a small internal driver program —
# add via go.mod (isolate under a build tag or separate go.mod if it must not
# affect the shipped binary's dependency graph):
go get github.com/go-rod/rod@v0.116.2
```

**Version verification:** Both packages were resolved and successfully `go build`-compiled
in an isolated scratch Go module during this research session (`go get
github.com/charmbracelet/freeze github.com/go-rod/rod github.com/chromedp/chromedp` all
succeeded; `freeze` was additionally built to a binary and run, producing a real PNG —
see Package Legitimacy Audit and Code Examples). Go toolchain auto-upgraded to 1.26.4 to
satisfy a transitive dependency's `go >= 1.26` requirement during that test — consistent
with the project's already-pinned Go 1.26.x.

## Package Legitimacy Audit

| Package | Registry | Age | Downloads | Source Repo | slopcheck | Disposition |
|---------|----------|-----|-----------|--------------|-----------|-------------|
| `github.com/charmbracelet/freeze` | Go modules (proxy.golang.org) | Charmbracelet org, established since 2023; actively released (multiple GitHub releases found) | Not directly queryable for Go modules; org has 30k+ combined stars across its ecosystem | `github.com/charmbracelet/freeze` (confirmed via `go get` + successful local `go build`) | `[OK]` (ran in isolated scratch module, `--ecosystem go`) | Approved |
| `github.com/go-rod/rod` | Go modules (proxy.golang.org) | Established project, ~7k GitHub stars, active on 344/365 days per recent activity data (WebSearch, MEDIUM confidence — not independently re-verified via GitHub API this session) | Not directly queryable for Go modules | `github.com/go-rod/rod` (confirmed via `go get`) | `[OK]` | Approved |
| `github.com/chromedp/chromedp` | Go modules (proxy.golang.org) | Long-established (evaluated as the rejected alternative to go-rod, kept here for completeness) | — | `github.com/chromedp/chromedp` | `[OK]` | Not selected (alternative) |

*slopcheck output for all three flagged a generic informational note ("No source repository
linked. Harder to verify what this code actually does.") — this is a known slopcheck
limitation for Go modules (it does not always resolve the `go.mod`-declared repo URL to
GitHub metadata); it is NOT a suspicion flag and did not downgrade the `[OK]` verdict. All
three packages are well-known, widely-used libraries independently confirmed via WebSearch
from their official GitHub repos, so this note is not treated as a legitimacy concern.*

**Packages removed due to slopcheck `[SLOP]` verdict:** none
**Packages flagged as suspicious `[SUS]`:** none

**Package-name provenance note (per this agent's provenance rule):** `charmbracelet/freeze`
and `go-rod/rod` were both discovered via WebSearch/training knowledge before being
registry-checked, so per the stated provenance rule they remain tagged `[ASSUMED]` for
package-**name-correctness** purposes even though slopcheck passed and both packages were
independently `go build`-compiled successfully in this session. The planner should still
gate the actual `go get`/`go install` step behind a lightweight `checkpoint:human-verify`
or at minimum a CI-green check, consistent with the graceful-degradation guidance for
`[ASSUMED]` packages.

## Architecture Patterns

### System Architecture Diagram

```
                    ┌─────────────────────────────────────────────┐
                    │              cmd/gitid (Cobra)               │
                    │   new: `keygen catalog` / `debug caps`        │
                    │   command — thin glue only                    │
                    └───────────────┬───────────────────────────────┘
                                     │ calls
        ┌────────────────────────────┼────────────────────────────┐
        ▼                            ▼                             ▼
┌───────────────┐          ┌──────────────────┐          ┌──────────────────┐
│ internal/      │          │ internal/         │          │ internal/         │
│ platform       │◄────────►│ keygen            │          │ identity          │
│ (probe: ssh -Q,│  informs │ (algorithm        │          │ (NEW: state.go —  │
│  ssh -V,       │  catalog │  registry:        │          │  8-state taxonomy │
│  libfido2,     │          │  ed25519+rsa-4096;│          │  from Reconstruct │
│  agent,        │          │  stub entries for │          │  output)          │
│  keychain)     │          │  ecdsa/-sk)       │          └────────┬──────────┘
└───────┬────────┘          └─────────┬─────────┘                   │
        │ exec.Command                │ writes via                  │ reads via
        │ (ssh, ssh-keygen —          │ filewriter chokepoint        │ Reconstruct()
        │  no shell, arg-slice)       ▼                              ▼
        │                    ┌──────────────────────────────────────────────┐
        │                    │           internal/filewriter                 │
        │                    │  Write() = backup + atomic temp→rename→chmod  │
        │                    │  ReplaceBlock / PrependBlockIfNotFound        │
        │                    │  (sentinel-delimited managed blocks)          │
        │                    └───────────────────┬────────────────────────────┘
        │                                         │ writes
        ▼                                         ▼
┌────────────────┐                    ┌─────────────────────────────────┐
│  OS toolchain    │                   │  ~/.ssh/config  (in-file mode)   │
│  ssh, ssh-keygen,│                   │  OR                              │
│  libfido2 (opt.) │                   │  ~/.ssh/config.d/gitid.config    │
└────────────────┘                    │  (Include'd mode, STORE-01)      │
                                       │  ~/.ssh/id_<algo>_<identity>[.pub]│
                                       └─────────────────────────────────┘

  ── Screenshot tooling (dev/build-time only, NOT wired into the above) ──
┌──────────────────┐        ┌────────────────────┐        ┌──────────────────────┐
│ tea.Model.View()  │──text─►│ `freeze` (dev tool) │──PNG──►│ .planning/design/    │
│ fixed WxH, no PTY │        │ --font.file pinned  │        │  <surface>/tui/*.png  │
└──────────────────┘        └────────────────────┘        └──────────────────────┘
┌──────────────────┐        ┌────────────────────┐        ┌──────────────────────┐
│ fixture HTML page │──URL──►│ go-rod (headless    │──PNG──►│ .planning/design/    │
│ (local file://)   │        │  Chromium driver)    │        │  <surface>/html/*.png │
└──────────────────┘        └────────────────────┘        └──────────────────────┘

  ── CI (GitHub Actions) — orchestration only, calls the SAME make targets ──
┌───────────────────────────────────────────────────────────────────────┐
│ ubuntu-latest │ macos-15-intel │ macos-15 (or macos-latest)             │
│   make setup-env → make test -race → make lint → make test-e2e         │
│   + make build (cross-compile darwin/amd64, darwin/arm64, linux/amd64  │
│     [+ linux/arm64 build-only])                                        │
└───────────────────────────────────────────────────────────────────────┘
```

A reader can trace the primary use case (generate + surface a key) left to right: the
Cobra command asks `platform` what the local machine supports, asks `keygen`'s registry
to generate material for the chosen/default algorithm, and everything that touches disk
goes through the single `filewriter` chokepoint before landing in `~/.ssh/config` (or its
Include'd equivalent) and the key files. The screenshot and CI blocks are deliberately
drawn as separate, disconnected subgraphs — they are dev/build-time concerns that never
appear in the shipped binary's runtime call graph.

### Recommended Project Structure

```
internal/
├── keygen/
│   ├── keygen.go          # refactor: registry dispatch, keep GenerateMaterial() signature
│   ├── registry.go        # NEW: map[string]generatorFunc; ed25519 + rsa-4096 registered;
│   │                       #      ecdsa-p256/-sk/ed25519-sk registered as "not implemented" stubs
│   ├── derive.go          # unchanged
│   └── signers.go         # unchanged
├── platform/
│   ├── platform.go         # extend: parseSSHVersion (LibreSSL/OpenSSL), ProbeKeyTypes (keep)
│   ├── capabilities.go     # NEW: libfido2/ssh-sk-helper detection, agent detection,
│   │                       #      keychain detection — all behind an injectable Deps struct
│   └── install.go          # unchanged (already has per-OS install hints)
├── sshconfig/
│   ├── include.go          # NEW: Include-file detection/write (STORE-01), reserved-block
│   │                       #      guard analogous to gitconfig.IsReservedBlockName
│   ├── adopt.go            # NEW: STORE-02 detect-existing-Include logic (distinct from
│   │                       #      internal/adopter, which is gitconfig-fragment-only)
│   ├── migrate.go          # NEW: STORE-03 reversible in-file <-> Include'd migration
│   └── (existing parser/reader/renderer/writer unchanged)
├── identity/
│   └── state.go             # NEW: MGR-02 8-state classification, pure function over
│                             #      Reconstruct() output + key-existence checks
└── screenshot/               # NEW, dev/build-tool package (or a `tools/` submodule —
    ├── tui.go                #      see Anti-Patterns re: keeping this out of the
    └── html.go                #      shipped binary's dependency graph)
cmd/gitid/
└── debug.go                  # NEW: `gitid debug caps` (or similar) — D-08 surface,
                               #      hosts both the algorithm-catalog readout and the
                               #      MGR-02 state-taxonomy readout
.github/workflows/
└── ci.yml                    # NEW — see CI section
Makefile
└── + screenshot-tui, screenshot-html targets (TOOL-05)
.planning/design/
└── <surface>/{html,tui}/*.png  # NEW versioned artifact directory (D-04)
```

### Pattern 1: Algorithm Registry (KEY-02)
**What:** Replace the hard-coded `if p.Algo != "ed25519"` check in
`internal/keygen/keygen.go` with a `map[string]func(Params) (Material, error)` registry.
Register `"ed25519"` and `"rsa-4096"` now; register `"ecdsa-p256"`, `"ed25519-sk"`,
`"ecdsa-sk"` as stub entries returning a clear "not yet implemented" error so the catalog
(KEY-01) can list all 5 without generating from the unimplemented three.
**When to use:** Any time KEY-07/rotate/create needs to generate a key by algorithm name.
**Example:**
```go
// Source: derived from the existing, empirically-verified internal/keygen/keygen.go
// pattern in this repo — extending it to a registry, same signatures.
type generatorFunc func(p Params) (Material, error)

var registry = map[string]generatorFunc{
	"ed25519":  generateEd25519, // existing logic, extracted unchanged
	"rsa-4096": generateRSA4096, // NEW
	// Stubs — KEY-02 "architecture leaves room without redesign":
	"ecdsa-p256": notYetImplemented("ecdsa-p256"),
	"ed25519-sk": notYetImplemented("ed25519-sk"),
	"ecdsa-sk":   notYetImplemented("ecdsa-sk"),
}

func GenerateMaterial(p Params) (Material, error) {
	gen, ok := registry[p.Algo]
	if !ok {
		return Material{}, fmt.Errorf("keygen: unsupported algorithm %q", p.Algo)
	}
	return gen(p)
}

func generateRSA4096(p Params) (Material, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 4096) // crypto/rsa
	if err != nil {
		return Material{}, fmt.Errorf("keygen: generating rsa-4096 key: %w", err)
	}
	// NOTE: pass the POINTER (*rsa.PrivateKey), not a value — unlike ed25519,
	// RSA's Sign/Public methods are pointer-receiver (standard Go crypto convention).
	// This differs from the ed25519 case documented in this repo's own prior research
	// as "Pitfall 10" (archived 02-RESEARCH.md), which found VALUE works for ed25519.
	var block *pem.Block
	if p.Passphrase != "" {
		block, err = ssh.MarshalPrivateKeyWithPassphrase(priv, p.Comment, []byte(p.Passphrase))
	} else {
		block, err = ssh.MarshalPrivateKey(priv, p.Comment)
	}
	if err != nil {
		return Material{}, fmt.Errorf("keygen: serializing rsa-4096 key: %w", err)
	}
	privPEM := pem.EncodeToMemory(block)

	sshPub, err := ssh.NewPublicKey(&priv.PublicKey)
	if err != nil {
		return Material{}, fmt.Errorf("keygen: building rsa-4096 public key: %w", err)
	}
	return Material{PrivPEM: privPEM, PubLine: pubLineWithComment(sshPub, p.Comment)}, nil
}
```
`[CITED: pkg.go.dev/golang.org/x/crypto/ssh — MarshalPrivateKey documented to support
RSA/DSA/ECDSA/Ed25519 via crypto.PrivateKey; the pointer-vs-value RSA detail is
`[ASSUMED]` (standard Go crypto/rsa convention, not independently re-verified empirically
this session the way the ed25519 case was in the archived Phase-2 research)`.

### Pattern 2: Injectable Capability Probe (PLAT-01)
**What:** All new probes (`libfido2`/`ssh-sk-helper` presence, running `ssh-agent`,
macOS keychain support) must follow the SAME injectable-seam pattern already used for
`platform.ProbeKeyTypes` — a thin `exec.Command` wrapper plus a pure parsing function
that is unit-testable without shelling out. This closes the project's own documented
"injected-seam wiring blindspot" (see project memory) by ensuring the REAL
`build*Deps()` closure — not just a test fake — is exercised by at least one test (e.g.
the debug command's own e2e test).
**When to use:** Every new probe added for KEY-03/PLAT-01/PLAT-02.
**Example:**
```go
// Pattern already established by ProbeKeyTypes/parseKeyTypes in
// internal/platform/platform.go — mirror it exactly for the new probes:
func ProbeSSHVersion() (string, error) {
	out, err := exec.Command("ssh", "-V").CombinedOutput() // ssh -V writes to stderr
	if err != nil {
		// exit status may be non-zero even on success for some builds; still parse output
	}
	return parseSSHVersion(string(out)), nil
}

// parseSSHVersion is the pure, testable core.
// Empirically verified this session on macOS: "OpenSSH_9.7p1, LibreSSL 3.3.6\n"
// [VERIFIED: `ssh -V` run directly on the research/dev machine]
// Linux format (OpenSSL, not independently run this session): "OpenSSH_9.6p1, OpenSSL 3.0.13\n" [CITED: WebSearch, cross-referenced against multiple sources]
func parseSSHVersion(out string) (opensshVersion, sslFlavor, sslVersion string) { /* ... */ }
```

### Pattern 3: Managed-Block-as-Floor for the SSH `Include` Line (STORE-01)
**What:** Use `filewriter.PrependBlockIfNotFound` (already implemented and tested in
`internal/filewriter/block.go` / `block_prepend_test.go`) to place the
`Include ~/.ssh/config.d/*.config` line as a sentinel-delimited managed block at the
**top** of `~/.ssh/config`, exactly mirroring how `internal/gitconfig/baseline.go`
already places its `[include]` block at the top of `~/.gitconfig` via the same function.
**When to use:** STORE-01's Include-line write, and nowhere else — per-identity Host
blocks continue to use `ReplaceBlock` (append/update in place), only the Include line
itself needs floor semantics for first-match-wins.
**Example:**
```go
// Source: internal/gitconfig/baseline.go pattern (already shipping in this repo),
// mirrored for the SSH side.
const sshIncludeBlockName = "ssh-include" // reserved, non-identity block name

func EnsureIncludeLine(configPath string) (backupPath string, err error) {
	existing, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("reading %s: %w", configPath, err)
	}
	composed := filewriter.PrependBlockIfNotFound(
		existing, sshIncludeBlockName, "Include ~/.ssh/config.d/*.config")
	if _, perr := Parse(composed); perr != nil {
		return "", fmt.Errorf("composed ssh config with Include line is not parseable: %w", perr)
	}
	return filewriter.Write(configPath, composed, configMode) // configMode = 0o600, existing const
}

// IsReservedBlockName MUST be added to internal/sshconfig, mirroring
// gitconfig.IsReservedBlockName exactly — see Common Pitfalls "reserved block name gap".
func IsReservedBlockName(name string) bool {
	return name == sshIncludeBlockName
}
```

### Pattern 4: Real `ssh -G` Proof for Include-File Resolution (STORE-01/03)
**What:** Prove Include-file resolution the same way `internal/tester.ResolvedVia` and
`internal/sshconfig/coexistence_test.go` already prove in-file resolution today: build a
hermetic temp `~/.ssh/config` (or a `-F <tmp>` config) that references a real,
filesystem-backed `config.d/*.config` file, then run `ssh -G -F <tmp> <alias>` for real
and assert on the parsed `IdentityFile` line.
**When to use:** Every STORE-01/02/03 round-trip test that claims Include-file
resolution — do not fake `ssh -G` output for these tests, since the whole point (per
the CONTEXT.md-locked verified constraint) is proving first-match-wins with a REAL
binary.
**Example:**
```go
// Source: pattern already proven in internal/sshconfig/coexistence_test.go (existing,
// passing test in this repo) — extend it to also write a config.d/*.config file.
out, err := exec.Command("ssh", "-G", "-F", tmpConfigPath, alias).Output() //nolint:gosec // arg-slice, hermetic test paths
resolved := tester.ParseResolved(string(out)) // reuse existing lowercase-key parser
// assert resolved.IdentityFile points at the Include'd file's key, not any decoy
```

### Anti-Patterns to Avoid
- **Reusing `internal/adopter` code (not just its pattern) for STORE-02.** It is
  gitconfig-`includeIf`-fragment-specific (`ListCandidates` globs `~/.gitconfig_*`,
  `WriteIncludeIf` writes gitconfig `[includeIf]` blocks). Attempting to bend it to SSH
  `Include` detection would require changing its public API in ways that break its
  existing (passing) tests. Build new, parallel code in `internal/sshconfig` instead.
- **Letting `go-rod`/`chromedp` become a transitive dependency of the shipped `gitid`
  binary.** These are dev/build-tool concerns (TOOL-05 explicitly scopes screenshot
  tooling OUT of product runtime). Isolate them behind a build tag (e.g.
  `//go:build screenshot`) or a separate `tools/go.mod`, so `go build ./cmd/gitid`
  never pulls in a headless-browser driver.
- **Hard-coding the FIDO2 key-type token as `ed25519-sk` when probing.** The real
  `ssh -Q key` token is `sk-ssh-ed25519@openssh.com` (verified empirically this
  session, see Common Pitfalls) — a probe that string-matches on `"ed25519-sk"` will
  silently always report FIDO2 as unavailable, even when it IS available.
- **Treating a fixed `macos-13`/`macos-14` CI matrix as safe** because CONTEXT.md
  locked it — CONTEXT.md's runner choice was a recommendation made without access to
  GitHub's current (2026) deprecation schedule; it is empirically wrong today and must
  be corrected (see State of the Art).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| ANSI terminal → PNG rendering | A custom ANSI parser + font rasterizer | `charmbracelet/freeze` | Font metrics, ANSI SGR parsing, and PNG encoding are deceptively complex (this session confirmed freeze already handles font embedding via `--font.file`, window chrome, padding, and produces correct PNG headers) |
| Headless browser automation for HTML screenshots | Raw Chrome DevTools Protocol WebSocket client | `go-rod/rod` (or `chromedp`) | CDP is a large, versioned, session-stateful protocol; both libraries already handle protocol versioning, page-load waiting, and browser lifecycle (including auto-downloading a matching Chromium build) |
| OpenSSH private-key serialization (any algorithm) | A custom OpenSSH binary key-file writer | `golang.org/x/crypto/ssh.MarshalPrivateKey`/`MarshalPrivateKeyWithPassphrase` | Already the established, correct pattern in this repo (`internal/keygen/keygen.go`); the OpenSSH private-key format has padding/checksum rules that are easy to get subtly wrong |
| Detecting SSL flavor (LibreSSL vs OpenSSL) from `ssh -V` | Ad-hoc string `contains()` checks scattered across callers | One `parseSSHVersion` pure function, unit-tested against both real formats | Both formats were empirically captured this session (macOS) and via WebSearch (Linux) — codify them once, not per call site |
| Atomic, backed-up file writes to `~/.ssh`/`~/.gitconfig` | `os.WriteFile` + manual backup logic per package | `internal/filewriter.Write`/`EnsureDir`/`PrependBlockIfNotFound`/`ReplaceBlock` | Already the project's single write chokepoint (STORE-04 locked invariant); every new writer in this phase (Include-line, migration) MUST route through it, never call `os.WriteFile` directly |
| SSH config `Include`/glob resolution | A custom glob-and-merge parser for `~/.ssh/config.d/*.config` | `github.com/kevinburke/ssh_config`'s built-in `Include` resolution (already vendored, already used by `sshconfig.Parse`) | The library already implements `filepath.Glob` + `~`-and-relative-path resolution per the OpenSSH spec; re-implementing it risks diverging from real `ssh`/`ssh-keygen` behavior |

**Key insight:** Nearly every "don't hand-roll" item in this phase already has an
existing, working, tested implementation somewhere in this repo. The actual engineering
risk in Phase 1 is not "will we build a bad ANSI renderer" — it's "will a new package
accidentally bypass the `filewriter` chokepoint or misclassify a probe token" (both are
called out explicitly above and in Common Pitfalls).

## Common Pitfalls

### Pitfall 1: CONTEXT.md's CI runner matrix is stale (macos-13 unsupported, macos-14 deprecating this week)
**What goes wrong:** A workflow pinned to `macos-13` fails to schedule at all (image
fully removed since Dec 2025); a workflow pinned to `macos-14` will start failing to
schedule between now (2026-07-02, four days before deprecation begins) and Nov 2, 2026.
**Why it happens:** CONTEXT.md's D-12 recommendation predates awareness of GitHub's
2026 runner-image retirement schedule.
**How to avoid:** Use `macos-15-intel` for darwin/amd64 (Intel) — the last-supported
x86_64 macOS image, available until August 2027 — and `macos-15` or `macos-latest` for
darwin/arm64 (Apple Silicon). `[VERIFIED: GitHub Docs + GitHub Changelog + actions/runner-images issue tracker]`.
**Warning signs:** A GitHub Actions run stuck in "Waiting for a runner" indefinitely, or
a workflow-validation error citing an unknown/retired runner label.

### Pitfall 2: The FIDO2 `ssh -Q key` token is not `ed25519-sk`
**What goes wrong:** Probing code that string-matches `"ed25519-sk"` against
`ssh -Q key` output will never find a match, because the real OpenSSH token is
`sk-ssh-ed25519@openssh.com` (and for ECDSA, `sk-ecdsa-sha2-nistp256@openssh.com`).
**Why it happens:** The requirements/context documents use the human-friendly
`ed25519-sk` shorthand; the actual protocol token has an `sk-` prefix and an
`@openssh.com` vendor-extension suffix.
**How to avoid:** Maintain an explicit mapping table (human name → protocol token),
never assume they're identical. `[VERIFIED: `ssh -Q key` run directly on the research
machine this session, OpenSSH_9.7p1]`:
```
ssh-ed25519                          → ed25519
sk-ssh-ed25519@openssh.com           → ed25519-sk
ssh-rsa                              → rsa
ecdsa-sha2-nistp256                  → ecdsa-p256
sk-ecdsa-sha2-nistp256@openssh.com   → ecdsa-sk
```
**Warning signs:** The catalog (KEY-01) always reports `-sk` variants as unavailable
even on a machine with a FIDO2 key and libfido2 installed.

### Pitfall 3: `internal/adopter` name collision with STORE-02's actual need
**What goes wrong:** A planner or implementer sees `internal/adopter` exists, assumes
it covers "adopt an existing SSH Include file" (STORE-02), and either tries to
force-fit it or wires the wrong package into the SSH flow.
**Why it happens:** The package name "adopter" is generic; its actual scope (gitconfig
`~/.gitconfig_*` fragment adoption via `includeIf`) is a different file, different
directive, and different config format from SSH's `Include`.
**How to avoid:** Read `internal/adopter/adopter.go`'s package doc comment before
assuming reuse; build STORE-02 as new code in `internal/sshconfig` following the SAME
detect→migrate/reference→backup **pattern**, not the same **code**.
**Warning signs:** A PR that imports `internal/adopter` from any SSH-config-touching
code path.

### Pitfall 4: Missing reserved-block-name guard on the SSH side will recreate a documented recurring bug
**What goes wrong:** Without an `sshconfig.IsReservedBlockName` guard (mirroring
`gitconfig.IsReservedBlockName`), the doctor's Orphans check will treat the new
`ssh-include` managed block as an orphaned/incomplete identity (no matching gitconfig
counterpart) and offer to delete it — which then gets re-created on the next run,
looping forever.
**Why it happens:** This exact bug class already happened once in this project (see
project memory: "Doctor reserved-block false-positive loop") for the gitconfig
`baseline-include` block, and was fixed by adding `IsReservedBlockName` + excluding it
from `ParseManagedIncludeIf`/Orphans cross-referencing.
**How to avoid:** Add the SSH-side equivalent guard in the SAME phase that introduces
the `ssh-include` block, not as a follow-up fix. `internal/doctor/checks/orphans.go`'s
Class 1/2 cross-referencing must skip reserved SSH block names exactly as it already
skips `gitconfig.IsReservedBlockName` for gitconfig blocks (`orphans.go` currently only
checks the gitconfig side of this — verify/extend the SSH side too).
**Warning signs:** `gitid doctor` (or its Phase-1 debug-surface equivalent) repeatedly
flags the Include line as an orphan across consecutive runs even after a "fix."

### Pitfall 5: `ssh_config.Decode` performs REAL filesystem I/O when the content contains an `Include` line
**What goes wrong:** `kevinburke/ssh_config`'s `Include` resolution calls
`filepath.Glob`/reads files from the real filesystem (resolved against `~/.ssh` for
relative paths) as part of `Decode` — this happens even when the caller passed an
in-memory `[]byte` fixture, if that fixture's content contains an `Include` directive.
Unit tests that construct pure in-memory SSH-config fixtures containing an `Include`
line will therefore either (a) silently read whatever real files happen to exist on the
test-runner's `~/.ssh/config.d/`, or (b) fail/no-op if that path doesn't exist,
depending on how the library handles a missing glob target.
**Why it happens:** `Include` is fundamentally a filesystem-resolution directive — the
library can't defer it to an injected byte slice without a much larger redesign.
**How to avoid:** Any STORE-01/02/03 test that exercises `Include` parsing MUST use
`t.TempDir()` + a real, filesystem-backed fixture tree (set `HOME` or pass an explicit
config path), never a bare in-memory content string with an `Include` line inside it.
This is a genuine testability constraint the planner should budget test-writing time
for.
**Warning signs:** A round-trip test that "parses" an Include line without ever
touching `t.TempDir()` and yet passes — it is probably not actually exercising Include
resolution at all, or is leaking the real test-runner's home directory into the test.

### Pitfall 6: `freeze`'s font rendering is not CI-deterministic without an explicit `--font.file`
**What goes wrong:** Freeze renders text using system-discoverable fonts by default;
a headless Linux CI runner may have a different (or no) monospace font installed than
the local dev machine used to generate a "golden" reference PNG, causing visual
differences that are pure font-rendering noise, not real regressions.
**Why it happens:** Font availability is an OS/package-manager concern, not something
`freeze` controls unless told to.
**How to avoid:** Vendor a specific monospace TTF (e.g. under
`.planning/design/fonts/`) and always pass `--font.file <path>` in the `screenshot-tui`
make target, on every OS, so the same font is used everywhere. `[CITED: freeze --help,
run directly this session, confirms --font.file flag exists]`.
**Warning signs:** PNG diffs that show only anti-aliasing/kerning differences, never
content differences, between local and CI-generated screenshots.

### Pitfall 7: `ssh.MarshalPrivateKey` requires a POINTER for RSA, unlike the VALUE that already works for ed25519 in this repo
**What goes wrong:** Copy-pasting the existing `ed25519.GenerateKey` → pass-value
pattern for RSA (passing `rsa.PrivateKey` by value instead of `*rsa.PrivateKey`) will
fail to satisfy the `crypto.Signer`/`crypto.PrivateKey` interface, because
`(*rsa.PrivateKey)` has pointer-receiver methods.
**Why it happens:** ed25519's `GenerateKey` happens to return a value type that
satisfies the signer interface directly (this repo's own archived research, "Pitfall
10," empirically confirmed this at v0.53.0); RSA does not share that property.
**How to avoid:** Always pass `priv` (the pointer returned by
`rsa.GenerateKey`) directly, never `*priv`. `[ASSUMED — standard Go crypto/rsa
convention, not independently re-verified empirically this session for the specific
v0.53.0 pin the way the ed25519 case was]`.
**Warning signs:** A compile error citing "does not implement crypto.PrivateKey" or
similar, when adapting the existing ed25519 keygen code to RSA.

### Pitfall 8: `go install` for golangci-lint is explicitly forbidden project-wide, but this does not extend to `freeze`/`gosec`
**What goes wrong:** A planner might assume the "never `go install` golangci-lint"
rule (CLAUDE.md, Makefile comments) applies to every new dev tool added in this phase,
and over-engineer a binary-installer script for `freeze` too.
**Why it happens:** The golangci-lint restriction is specifically about Go-version
mismatch causing silently wrong lint behavior for a tool that reads/writes Go AST — a
risk that doesn't apply to `freeze` (a terminal-output renderer) or `gosec` (already
installed via `go install` in this repo's own Makefile today).
**How to avoid:** `go install github.com/charmbracelet/freeze@v0.2.2` is fine and
consistent with how `gosec` is already installed in this Makefile
(`go install github.com/securego/gosec/v2/cmd/gosec@latest`, verified by reading
`Makefile` `setup-env` target this session).
**Warning signs:** Over-scoped platform-specific binary-download logic for a tool that
doesn't need it.

## Code Examples

### Verified: `freeze` produces a real, headless-rendered PNG (empirically run this session)
```bash
# Source: run directly on the research/dev machine this session (macOS, Darwin 23.6.0)
go build -o /tmp/freezebin github.com/charmbracelet/freeze   # succeeded, no errors
/tmp/freezebin --execute "cat /tmp/sample.txt" -o /tmp/sample.png
# Output:  WROTE  /tmp/sample.png
# file /tmp/sample.png → "PNG image data, 640 x 332, 8-bit/color RGBA, non-interlaced"
```
This confirms freeze needs no display/GUI and produces a real, valid PNG purely from
captured terminal output — directly applicable to a `screenshot-tui` make target that
pipes a `View()`-dump golden `.txt` file through `freeze` (via `--execute "cat <golden>"`
or freeze's direct ANSI-file input mode — verify the exact flag in a Phase-1 spike task,
`freeze --help` lists `-o/--output` and `-x/--execute` but the research session did not
verify a direct "render this ANSI file, not a live command" invocation path).

### Verified: real `ssh -Q key` output on this machine (macOS, OpenSSH 9.7p1/LibreSSL 3.3.6)
```
$ ssh -Q key
ssh-ed25519
ssh-ed25519-cert-v01@openssh.com
sk-ssh-ed25519@openssh.com
sk-ssh-ed25519-cert-v01@openssh.com
ecdsa-sha2-nistp256
ecdsa-sha2-nistp256-cert-v01@openssh.com
ecdsa-sha2-nistp384
ecdsa-sha2-nistp384-cert-v01@openssh.com
ecdsa-sha2-nistp521
ecdsa-sha2-nistp521-cert-v01@openssh.com
sk-ecdsa-sha2-nistp256@openssh.com
sk-ecdsa-sha2-nistp256-cert-v01@openssh.com
ssh-dss
ssh-dss-cert-v01@openssh.com
ssh-rsa
ssh-rsa-cert-v01@openssh.com
```
No `libfido2`/`ssh-sk-helper` was installed on this dev machine (verified: no
`libfido2*` under Homebrew `lib`, `brew list libfido2` reports "No such keg") — this is
the expected "hardware key support absent" state the PLAT-01 probe must classify
correctly and non-fatally (fall back to ed25519/rsa-4096, per D-05/D-14 in the existing
`platform.SelectAlgorithm` fallback-chain pattern).

### Existing, reusable: `ssh -G` real-resolution proof pattern
```go
// Source: internal/tester/tester.go (already shipping, passing tests in this repo)
func ResolvedVia(configPath, keyPath, alias string) (Result, ResolvedConfig) {
	gOut, _ := exec.Command("ssh", "-F", configPath, "-G", alias).Output()
	return res, ParseResolved(string(gOut))
}
// ParseResolved matches on LOWERCASE keys — `ssh -G` always emits lowercase
// directive names regardless of the config file's original casing (documented
// "Pitfall 3" in internal/tester/tester_test.go, already handled).
```

### Existing, reusable: managed-block floor placement
```go
// Source: internal/filewriter/block.go PrependBlockIfNotFound (already shipping,
// already tested in block_prepend_test.go) — the exact primitive STORE-01 needs.
func PrependBlockIfNotFound(existing []byte, name, blockBody string) []byte
```

## State of the Art

| Old Approach (CONTEXT.md D-12) | Current Approach (this research) | When Changed | Impact |
|--------------------------------|-----------------------------------|---------------|--------|
| `macos-13` runner (darwin/amd64, Intel) | `macos-15-intel` | macos-13 deprecation began Sep 22 2025, fully unsupported Dec 8 2025 (per GitHub Changelog); `macos-15-intel` is the current, last-supported x86_64 label, available until August 2027 | Workflows pinned to `macos-13` cannot schedule at all — a hard CI-blocking failure, not a soft warning |
| `macos-14` runner (darwin/arm64, Apple Silicon) | `macos-15` or `macos-latest` | `macos-14` deprecation begins **July 6, 2026** (four days after this research's date) and completes Nov 2, 2026, per `actions/runner-images` issue tracker | A workflow written today against `macos-14` will work initially but start failing to schedule within the phase's likely execution window |

**Deprecated/outdated:**
- `macos-13`/Intel-default macOS runners generally — Apple/GitHub are winding down x86_64
  macOS support entirely; `macos-15-intel` is explicitly documented as "the last
  available x86_64 image" before Fall 2027. `[VERIFIED: GitHub Docs, GitHub Changelog,
  actions/runner-images issue #13045]`.
- The informal `ed25519-sk` naming for the FIDO2 `ssh -Q key` token — superseded by the
  real protocol token `sk-ssh-ed25519@openssh.com` throughout this phase's probing code.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `go-rod/rod` is the better choice over `chromedp` for HTML capture, based on its auto-downloading launcher | Standard Stack / Alternatives Considered | Low — both are viable pure-Go CDP drivers confirmed to `go get`/`go build` cleanly; switching later is a contained change isolated to the `internal/screenshot`/`tools/` package |
| A2 | `ssh.MarshalPrivateKey` requires a pointer `*rsa.PrivateKey` (not a value) at the pinned v0.53.0 | Architecture Patterns Pattern 1, Common Pitfalls #7 | Low-Medium — if wrong, this surfaces immediately as a compile error during TDD RED→GREEN, not a silent runtime bug; standard Go crypto convention makes this very likely correct |
| A3 | Linux `ssh -V` format is `OpenSSH_X.Xp#, OpenSSL X.X.X` (not independently run on a Linux machine this session, only cross-referenced via WebSearch) | Common Pitfalls #2 / Architecture Patterns Pattern 2 | Medium — if the exact separator/format differs on a specific distro's OpenSSH build, `parseSSHVersion`'s regex needs distro-specific test fixtures; recommend the planner add a CI-verified fixture from the actual `ubuntu-latest` runner rather than trusting this WebSearch-sourced format alone |
| A4 | `charmbracelet/freeze`'s `--font.file` flag is sufficient (alone) to make CI/local PNG rendering deterministic, with no other font-metric variance sources | Common Pitfalls #6 | Medium — sub-pixel rendering/anti-aliasing can still vary by OS-level font rasterizer (e.g. CoreText on macOS vs FreeType on Linux) even with an identical TTF; the planner should budget a small tolerance/threshold in any later visual-regression diff (Phase 3+), not byte-exact PNG comparison |
| A5 | `go-rod`'s `launcher` package can auto-download a headless-shell Chromium build on both macOS and Linux without additional system dependencies, satisfying BUILD-04 reproducibility | Standard Stack / Anti-Patterns | Medium — not independently verified by actually running the download in this sandboxed research session (network/time cost); if the auto-download requires system libraries not present on a minimal Linux CI image or a fresh dev machine, a Phase-1 spike task must add an explicit `apt install`/`brew install` fallback |

**If this table is empty:** N/A — see entries above; multiple non-trivial assumptions
remain and should be resolved by a Phase-1 spike task rather than deferred silently.

## Open Questions

1. **Exact `freeze` invocation for a pre-captured `View()`-dump `.txt` golden (not a live
   `--execute` command)**
   - What we know: `freeze --execute "<cmd>"` was empirically verified to work headlessly
     this session, producing a real PNG from live command output.
   - What's unclear: Whether `freeze` also supports rendering a static ANSI text FILE
     directly (as D-01 implies: capture `View()` to a `.txt` golden FIRST, then render
     that golden to PNG in a separate step) versus requiring the render to happen via
     `--execute "cat golden.txt"` (which re-invokes a subprocess and is less clean).
   - Recommendation: A Phase-1 spike task should run `freeze --help` in full (this
     session only captured the top of the help text) and confirm the exact flag —
     likely a bare positional-file argument analogous to `freeze main.go`.

2. **Exact TDD split point between `internal/identity/state.go` (MGR-02) and
   `internal/doctor/checks/orphans.go`'s existing "unused key" cross-referencing**
   - What we know: `CheckOrphans` (Class 3) already computes something very close to
     "key-unused" by cross-referencing `KeyPaths` against
     `ParseAllHostIdentityFiles`. MGR-02's 8-state taxonomy needs `key-unused` as ONE of
     its 8 states, computed by `internal/identity` per the D-11 "UI-free TDD core... no
     sidecar DB" requirement — but `internal/doctor` is explicitly the write-free,
     Finding-oriented diagnostic layer, a different consumer shape (list of advisory
     Findings vs. a per-account State enum).
   - What's unclear: Whether `internal/identity/state.go` should duplicate the
     unused-key cross-reference logic, or whether `internal/doctor/checks/orphans.go`
     should be refactored to call a shared helper that both packages use.
   - Recommendation: Planner should scope a small shared helper (e.g. a
     `crossReferenceUnusedKeys` pure function usable by both, living in whichever
     package has no import-cycle risk — likely `internal/identity`, since
     `internal/doctor` already depends on identity-shaped data) rather than duplicating
     the logic twice.

3. **Whether the `go-rod` (or `chromedp`) dependency should live in the main `go.mod` or
   an isolated `tools/go.mod`**
   - What we know: TOOL-05/DLV-03 explicitly scope screenshot tooling as dev/build-only,
     not a runtime dependency of the shipped `gitid` binary.
   - What's unclear: Whether a build-tag-gated file in the main module (simpler,
     single `go.mod`) is sufficient to keep it out of `go build ./cmd/gitid`'s
     dependency graph, or whether a fully separate `tools/go.mod` submodule is needed
     to avoid it ever appearing in `go.sum` reproducibility checks for the shipped
     binary.
   - Recommendation: Start with a build-tag-gated file (`//go:build screenshot`) inside
     the main module for simplicity; escalate to a separate `tools/go.mod` only if
     `go mod tidy`/dependency-graph bloat becomes an observed problem during planning.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go | All | ✓ | Go 1.26.x toolchain resolves per go.mod (verified via successful `go build`/`go get` this session) | — |
| git | TOOL/all | ✓ | 2.47.0 | — |
| ssh / ssh-keygen | KEY/PLAT | ✓ | OpenSSH_9.7p1, LibreSSL 3.3.6 (macOS dev machine) | — |
| golangci-lint | TOOL-02/BUILD-02 | ✗ (not yet installed on this research machine) | — | `make setup-env` installs it via the pinned binary installer (v2.12.2) — no fallback needed, this is expected pre-setup-env state |
| gosec | TOOL-02/BUILD-02 | ✗ (not yet installed) | — | `make setup-env` installs it via `go install` (already the Makefile pattern) |
| pre-commit | TOOL-02/TOOL-03 | ✓ | 4.6.0 (via `uv tool install pre-commit`, per project convention) | — |
| libfido2 / ssh-sk-helper | KEY-03/PLAT-01 (`-sk` probing) | ✗ (not installed on this research machine — confirmed via `brew list libfido2` → "No such keg") | — | PLAT-01's probe must treat this as a normal, non-fatal "hardware key support absent" state, not an error — this IS the expected common case per `[VERIFIED: WebSearch — macOS's bundled OpenSSH lacks FIDO2 middleware by default even when libfido2 IS installed via Homebrew, requiring an additional `sk-libfido2.dylib` wiring step]` |
| Google Chrome / Chromium | TOOL-05 HTML capture | ✓ (Chrome.app present in `/Applications` on this dev machine) | Not version-probed this session | `go-rod`'s launcher auto-downloads a matching headless-shell Chromium build if none found — see Open Question 3/Assumption A5 for CI/fresh-clone verification status |
| GitHub Actions `macos-15-intel`/`macos-15` runners | BUILD-01/02 | N/A (cloud-hosted, not locally probable) | — | See State of the Art — these labels are current and documented as available through at least 2027 |

**Missing dependencies with no fallback:** none — every missing item above has an
established, already-documented fallback (either `make setup-env` or an existing
graceful-degradation code path).

**Missing dependencies with fallback:** golangci-lint, gosec (both installed by
`make setup-env`, BUILD-04's own verification target); libfido2 (probe must degrade
gracefully, not error).

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` package + `go test -race` (no third-party test framework) |
| Config file | none — `go.mod` + `.golangci.yml` are the only config surfaces; e2e tests are gated by the `e2e` build tag |
| Quick run command | `go test -race ./internal/keygen/... ./internal/platform/... ./internal/sshconfig/... ./internal/identity/...` (scoped to this phase's touched packages) |
| Full suite command | `make test` (unit, `-race`, all packages) + `make test-e2e` (builds the real binary, runs `//go:build e2e` tests) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| KEY-02 | rsa-4096 generation produces valid OpenSSH PEM + correct pub line | unit | `go test ./internal/keygen/... -run TestGenerateRSA4096 -race` | ❌ Wave 0 |
| KEY-02 | Algorithm registry dispatches by name, unknown algo errors cleanly | unit | `go test ./internal/keygen/... -run TestRegistry -race` | ❌ Wave 0 |
| KEY-04 | Generated key files land at correct perms (600/644) | unit | `go test ./internal/keygen/... -run TestPermissions -race` | ✅ (existing pattern in `keygen_test.go`, extend) |
| PLAT-01 | `ssh -Q key` → algorithm-name mapping including `-sk` tokens | unit | `go test ./internal/platform/... -run TestKeyTypeMapping -race` | ❌ Wave 0 |
| PLAT-01 | `ssh -V` parses OpenSSH+SSL-flavor on both macOS and Linux formats | unit | `go test ./internal/platform/... -run TestParseSSHVersion -race` | ❌ Wave 0 |
| PLAT-01 | libfido2/agent/keychain probes are injectable and mockable | unit | `go test ./internal/platform/... -run TestCapabilities -race` | ❌ Wave 0 |
| STORE-01 | Include line placed as floor (top), idempotent on re-run | unit | `go test ./internal/sshconfig/... -run TestEnsureIncludeLine -race` | ❌ Wave 0 |
| STORE-01 | Real `ssh -G` resolves through the Include'd file (first-match-wins) | integration | `go test ./internal/sshconfig/... -run TestIncludeResolution -race` (requires `t.TempDir()` + real `ssh` binary, see Pitfall 5) | ❌ Wave 0 |
| STORE-02 | Detect existing external Include directive, adopt its path | unit | `go test ./internal/sshconfig/... -run TestAdoptExistingInclude -race` | ❌ Wave 0 |
| STORE-03 | Migrate in-file → Include'd and back, backup created both directions | integration | `go test ./internal/sshconfig/... -run TestMigrate -race` | ❌ Wave 0 |
| MGR-02 | All 8 states computed correctly from fixture managed-block configs | unit (table-driven) | `go test ./internal/identity/... -run TestClassifyState -race` | ❌ Wave 0 |
| TOOL-05 | `make screenshot-tui` produces a non-empty PNG from a fixture `View()` golden | smoke | `make screenshot-tui && test -s .planning/design/_spike/tui/*.png` | ❌ Wave 0 (target doesn't exist) |
| TOOL-05 | `make screenshot-html` produces a non-empty PNG from a fixture HTML page | smoke | `make screenshot-html && test -s .planning/design/_spike/html/*.png` | ❌ Wave 0 (target doesn't exist) |
| BUILD-01 | `make build` cross-compiles for darwin/amd64, darwin/arm64, linux/amd64 | glue/config | `GOOS=darwin GOARCH=amd64 make build` (repeat per target) — not unit-testable, verify by running locally + observing CI matrix green | N/A — infra, not a Go test |
| BUILD-02/04 | CI gates all pass on `ubuntu-latest`, `macos-15-intel`, `macos-15` | glue/config | Push a branch, observe GitHub Actions run status | N/A — verified by a real CI run, not local `go test` |

### Sampling Rate
- **Per task commit:** the scoped quick-run command for the package(s) touched by that
  task (e.g. `go test -race ./internal/keygen/...` after a keygen-registry task).
- **Per wave merge:** `make test` (full unit suite, `-race`) + `make lint` +
  `make test-e2e`.
- **Phase gate:** Full suite green locally AND at least one real GitHub Actions run
  green on all three (corrected) runners before `/gsd-verify-work`.

### Wave 0 Gaps
- [ ] `internal/keygen/registry_test.go` (or extend `keygen_test.go`) — covers KEY-02
      registry dispatch + rsa-4096 generation
- [ ] `internal/platform/capabilities_test.go` — covers PLAT-01 libfido2/agent/keychain
      probes, injectable seam
- [ ] `internal/platform/version_test.go` (or extend `platform_test.go`) — covers
      `ssh -V` parsing for both LibreSSL and OpenSSL formats (needs a Linux-format
      fixture string, since only the macOS format was empirically captured this
      session — see Assumption A3)
- [ ] `internal/sshconfig/include_test.go` — covers STORE-01 Include-line floor
      placement + idempotency
- [ ] `internal/sshconfig/adopt_test.go` — covers STORE-02 detection
- [ ] `internal/sshconfig/migrate_test.go` — covers STORE-03 reversible migration
      (needs `t.TempDir()`-backed real-filesystem fixtures per Pitfall 5)
- [ ] `internal/identity/state_test.go` — covers MGR-02, table-driven over all 8 states
- [ ] `Makefile` targets `screenshot-tui`/`screenshot-html` — do not exist yet
- [ ] `.github/workflows/ci.yml` — does not exist yet (confirmed: `.github/workflows/`
      directory is absent from this repo)
- [ ] A vendored monospace font file for deterministic `freeze` rendering (Pitfall 6) —
      does not exist yet

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-------------------|
| V2 Authentication | No | gitid is a local, single-user CLI/TUI tool — no login/session concept |
| V3 Session Management | No | Same as above |
| V4 Access Control | Partial | File-level access control only: correct Unix permissions (700/600/644) on `~/.ssh` and key files — already enforced via `filewriter` |
| V5 Input Validation | Yes | Identity names, provider names, Include paths, and probe output must all be validated/sanitized before use in `exec.Command` arg slices or file paths; reuse `internal/identity/validate.go` (`ValidateName`/`ValidateEmail`/`ValidateProvider`) pattern, extend for any new user-facing input (e.g. custom Include path) |
| V6 Cryptography | Yes | `golang.org/x/crypto/ssh` for all key generation/serialization — never hand-roll (already the established, locked pattern; extends unchanged to rsa-4096) |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|----------------------|
| OS command injection via `exec.Command` (probes shelling to `ssh`/`ssh-keygen`) | Tampering | Arg-slice `exec.Command` form, NEVER shell string interpolation — already the established pattern (`ProbeKeyTypes`, `tester.Resolved*`); extend identically for every new probe (libfido2 detection, agent detection) |
| Path traversal / arbitrary file write via a user-supplied Include path | Tampering | STORE-01's locked constraint already requires Include paths be "absolute or `~/.ssh`-relative"; STORE-02's adopt-detection must validate any user-confirmed external path before it becomes a migration target — reuse `filewriter`'s trusted-path assumption but add explicit path validation at the boundary where a path first becomes user-influenced (e.g. adopt-detection scanning an existing config) |
| Weak file permissions on newly generated rsa-4096 keys | Information Disclosure | `filewriter.Write(privPath, privPEM, 0600)` — same call already used for ed25519; no algorithm-specific permission difference, verify this is not accidentally lost during the registry refactor |
| Private key material leaking into logs/stdout during rsa-4096 debug printing | Information Disclosure | `Material.PrivPEM` is already documented "must never be logged or printed" (existing doc comment in `keygen.go`); the new debug/list command (D-08) must NEVER print `Material`/`PrivPEM`, only catalog metadata and public-key-derived info |
| Supply-chain risk from `curl \| sh` installers (`golangci-lint`, `gosec`-adjacent patterns) and `go install @latest`/pinned-version installs for new dev tools (`freeze`) | Tampering | Already an accepted, existing pattern in this repo's own `Makefile` (`curl -sSfL https://golangci-lint.run/install.sh \| sh`); pin `freeze` to an exact version (`@v0.2.2`) rather than `@latest`, consistent with how `golangci-lint`'s version is pinned via `GOLANGCI_LINT_VERSION` |
| Untrusted headless-Chromium binary download (`go-rod`'s launcher auto-fetch) | Tampering / Supply chain | `go-rod`'s launcher downloads from Google's official Chromium snapshot CDN and is a widely-used, audited path; the planner should still pin an explicit Chromium revision/version rather than "always latest" for CI reproducibility (Assumption A5 flags this as needing a Phase-1 spike verification) |

## Sources

### Primary (HIGH confidence)
- This repository's own source code, read directly this session:
  `internal/keygen/keygen.go`, `internal/platform/platform.go`, `internal/platform/install.go`,
  `internal/filewriter/block.go`, `internal/filewriter/filewriter.go`,
  `internal/sshconfig/parser.go`, `internal/sshconfig/reader.go`, `internal/sshconfig/writer.go`,
  `internal/identity/identity.go`, `internal/identity/loader.go`, `internal/identity/modes.go`,
  `internal/adopter/adopter.go`, `internal/gitconfig/reader.go`, `internal/gitconfig/baseline.go`,
  `internal/doctor/checks/orphans.go`, `internal/tester/tester.go`, `e2e/harness_test.go`,
  `e2e/ui_pty_e2e_test.go`, `Makefile`, `go.mod`, `.golangci.yml`, `recipes/README.md`,
  `recipes/ssh-config.recipe`, `recipes/gitconfig.recipe`
- Commands empirically run on the research/dev machine this session: `ssh -V`,
  `ssh -Q key`, `go build`/`go get` for `charmbracelet/freeze`, `go-rod/rod`,
  `chromedp/chromedp` in an isolated scratch module, `freeze --execute "..." -o
  sample.png` (produced a real, valid PNG), `slopcheck install ... --ecosystem go`
- This project's own archived prior-phase research (same codebase, same pinned
  `golang.org/x/crypto` v0.53.0):
  `.planning/archive/0.0.1-poc-product-features-in-tui/phases/02-first-identity-end-to-end/02-RESEARCH.md`
  ("Pitfall 10" ed25519 value-vs-pointer, empirically verified in that session)
- [GitHub Docs — GitHub-hosted runners reference](https://docs.github.com/en/actions/reference/runners/github-hosted-runners) — current macOS/Linux runner labels
- [GitHub Changelog — macOS 13 runner image closing down](https://github.blog/changelog/2025-09-19-github-actions-macos-13-runner-image-is-closing-down/)
- [actions/runner-images#13046 — macOS 13 deprecation schedule](https://github.com/actions/runner-images/issues/13046)
- [actions/runner-images#13045 — macos-15-intel availability](https://github.com/actions/runner-images/issues/13045)

### Secondary (MEDIUM confidence)
- [charmbracelet/freeze GitHub repo](https://github.com/charmbracelet/freeze) and
  [pkg.go.dev/github.com/charmbracelet/freeze](https://pkg.go.dev/github.com/charmbracelet/freeze)
  — install method, `--font.file` flag (cross-verified by running `freeze --help`
  locally this session)
- [go-rod/rod GitHub repo](https://github.com/go-rod/rod) — auto-download launcher,
  activity/maintenance signals (WebSearch-sourced star count/activity, not
  independently re-verified via GitHub API)
- [kevinburke/ssh_config GitHub repo](https://github.com/kevinburke/ssh_config) —
  `Include` directive glob/home-directory resolution behavior
- [Yubico/libfido2 issue #464, #469](https://github.com/Yubico/libfido2/issues/464) —
  macOS bundled OpenSSH lacking FIDO2 middleware, `ssh-sk-helper` homebrew-prefix issue
- WebSearch cross-referenced `ssh -V` Linux (OpenSSL) output format — not independently
  run on a real Linux machine this session (see Assumption A3)

### Tertiary (LOW confidence)
- go-rod vs chromedp performance/API-ergonomics comparison — WebSearch summaries only,
  not independently benchmarked

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH for everything already in `go.mod`/this repo (crypto, ssh_config,
  cobra); MEDIUM for the two new dev-tool packages (freeze, go-rod) — resolved and
  compiled successfully this session but not run through Context7 (unavailable) or a
  full production integration
- Architecture: HIGH — nearly every pattern recommended is a direct extension of an
  existing, already-tested pattern read directly from this repo's source
- Pitfalls: HIGH for the CI-runner-deprecation and `ssh -Q key` token findings (both
  independently verified via official sources / live commands this session); MEDIUM for
  the RSA pointer-vs-value and freeze-font-determinism claims (sound Go/tooling
  conventions, not exhaustively empirically re-proven this session)

**Research date:** 2026-07-02
**Valid until:** 30 days for the codebase-grounded findings (stable, this repo's own
code); re-verify the GitHub Actions runner-label finding closer to actual CI-workflow
authoring time, since the `macos-14` deprecation clock is actively running during this
exact window (begins 2026-07-06).
