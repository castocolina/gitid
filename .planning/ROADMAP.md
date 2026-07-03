# Roadmap: gitid — v1.0 TUI-First Redesign

## Overview

v1.0 rebuilds `gitid` as a **design-driven, screenshot-verified** terminal app that
creates and manages coherent SSH + Git identities (the `recipes/` end state: alias per
identity, `Port 443` alt-SSH, `IdentitiesOnly yes`, `includeIf` `hasconfig:`/`gitdir:`,
`allowed_signers` signing). **Phase 1** lays the non-UI foundations — screenshot
tooling, multi-algorithm keygen + local-capability probing, the dual SSH-storage
strategy (in-file / Include'd / adopt / migrate), the identity state-taxonomy core, and
a cross-OS GitHub Actions CI. **Phase 2** is the single human checkpoint: every surface
is designed as an HTML/`mui` mockup and a navigable Go TUI dummy, screenshot-captured,
and **approved by the user** — those images become the reference set for every later
wave. **Phases 3–9** wire each surface's backend *behind the approved design* — create
flow, git screen, identity manager, global SSH options, global git options,
health+fixer, and credential upload — each gated by a per-surface UI wave (`/mui` +
`agent-ui-ux-designer`, PTY e2e on the real binary, visual-regression diff vs the
approved screenshots). **Phase 10** validates the whole app end-to-end on Linux and
ships tagged, checksummed release binaries.

The autonomous build loop (`.planning/ONESHOT-LOOP-PROMPT.md`) runs unattended except
for the **one** design-approval checkpoint (Phase 2); credential upload (Phase 9)
auto-runs when `gh`/`glab` is authenticated and a valid identity exists — it is not a
checkpoint. Phase numbering is **reset for this milestone**; the prior POC is archived
under `.planning/archive/0.0.1-poc-product-features-in-tui/`.

> **Granularity note:** config granularity is `coarse`, but this milestone's
> defining constraint is a **per-surface design-first UI wave** (PRD "Execution
> Phases" / DLV-01..06). The 10 phases are derived 1:1 from that delivery method, not
> padded — each of Phases 3–9 is one distinct user-facing surface that must clear its
> own mockup → dummy → approval → backend → e2e → visual-regression gate.

## Phases

**Phase Numbering:**

- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [ ] **Phase 1: Foundations, Spikes & CI** - Non-UI core (screenshot tooling, multi-algo keygen + probing, dual SSH storage, state taxonomy) + cross-OS CI, no product UI
- [ ] **Phase 2: DESIGN — All Mockups (★ CHECKPOINT #1)** - HTML/`mui` mockups + Go TUI dummy for every surface, screenshot-captured and user-approved
- [ ] **Phase 3: Create Flow Backend** - Algorithm → SSH screen → two-stage test → store, behind the approved design
- [ ] **Phase 4: Git Configuration Screen** - Per-identity git fragment + `includeIf` + `allowed_signers`, review → confirm → write
- [ ] **Phase 5: Identity Manager** - State-taxonomy list, SSH-first detail, clone / new-key / rotate / delete-choice, app view set
- [ ] **Phase 6: Global SSH Options** - Danger-aware, explained SSH config options; advisory + fixable
- [ ] **Phase 7: Global Git Options** - Baseline git config (main/master, eol, case, email) + recipe defaults, explained
- [ ] **Phase 8: Health + Fixer** - Two-section (SSH + Git) health with redundancy/contradiction detection and in-place fixes
- [ ] **Phase 9: Upload / Credentials Assist** - Auto-upload the `.pub` (auth + signing) when `gh`/`glab` authenticated; manual fallback
- [ ] **Phase 10: Linux Validation + Release Pipeline** - End-to-end Linux validation + tagged, checksummed release artifacts

## Phase Details

### Phase 1: Foundations, Spikes & CI

**Goal**: Every non-UI capability, tool, and CI gate that later phases depend on exists and is test-proven — with **no product UI** yet.
**Depends on**: Nothing (first phase)
**Requirements**: TOOL-01, TOOL-02, TOOL-03, TOOL-04, TOOL-05, DLV-03, DLV-07, KEY-01, KEY-02, KEY-03, KEY-04, STORE-01, STORE-02, STORE-03, STORE-04, MGR-02, PLAT-01, PLAT-02, BUILD-01, BUILD-02, BUILD-04
**Success Criteria** (what must be TRUE):

  1. A repeatable capture step (a `make` target / scripted step the loop can call) produces PNG screenshots of a TUI screen and of an HTML page, stored as versioned reference artifacts. (TOOL-05, DLV-03)
  2. gitid generates real ed25519 (default) and rsa-4096 keys with correct permissions, and a local-capability probe (`ssh-keygen -Q`, `ssh -V`, libfido2 / agent / keychain) drives a top-5 algorithm catalog with per-algorithm macOS/Linux availability + variant/troubleshooting notes — surfaced by a debug/list command and proven by tests. (KEY-01, KEY-02, KEY-03, KEY-04, PLAT-01, PLAT-02)
  3. gitid can write SSH config as in-file managed blocks **or** a gitid-owned Include'd file, adopt an existing external Include'd file, and migrate reversibly between the two — each with timestamped backup, proven by round-trip tests and real `ssh -G` resolution. (STORE-01, STORE-02, STORE-03, STORE-04)
  4. The identity state-taxonomy (complete / incomplete / git-only / key-unused / key-used-ssh-only / key-used-both / key-missing / fragment-missing) is computed by the UI-free, TDD core from parsed managed blocks (no sidecar DB). (MGR-02, DLV-07)
  5. GitHub Actions builds gitid for darwin/amd64, darwin/arm64, and linux/amd64, and runs `make test` (race) + `make lint` (golangci-lint + gosec) + `make test-e2e` **green on both macOS and Linux** runners, reproducible from a fresh clone via `make setup-env`. (BUILD-01, BUILD-02, BUILD-04, TOOL-01, TOOL-02, TOOL-03, TOOL-04)**Plans**: 7 plans in 3 waves

**Wave 1**

- [x] 01-01-PLAN.md — Local capability probing: ssh -V/-Q parse, libfido2/agent/keychain seam (PLAT-01/02, KEY-03)
- [x] 01-02-PLAN.md — Multi-algorithm keygen registry + top-5 catalog (KEY-01/02/04)
- [x] 01-03-PLAN.md — Dual SSH-config storage: Include'd file, adopt, reversible migrate + reserved-block guard (STORE-01..04, TOOL-04)
- [x] 01-04-PLAN.md — Identity 8-state taxonomy core, table-driven (MGR-02, DLV-07)
- [x] 01-05-PLAN.md — Screenshot tooling: freeze TUI capture + go-rod HTML capture make targets (TOOL-05, DLV-03, TOOL-02)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 01-06-PLAN.md — Debug/list command surfacing catalog + probe + state; real-wiring e2e (KEY-01, PLAT-01, MGR-02, DLV-07)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 01-07-PLAN.md — Cross-OS GitHub Actions CI (3-runner) + build matrix (BUILD-01/02/04, TOOL-01..04)

### Phase 2: DESIGN — All Mockups (★ CHECKPOINT #1)

**Goal**: Every product surface is designed as an HTML/`mui` mockup and a navigable Go TUI dummy, screenshot-captured, and **approved by the user** — establishing the reference design the whole build is verified against.
**Depends on**: Phase 1 (screenshot tooling + core seams)
**Requirements**: DLV-01, DLV-02, DLV-05, DLV-08
**Success Criteria** (what must be TRUE — **gated on user approval**):

  1. Every surface (create flow, git screen, identity manager, global SSH, global git, health, fixer) has an HTML/`mui` mockup produced with the `/mui` skill and `agent-ui-ux-designer`, with every flow/screen screenshot-captured to versioned reference artifacts. (DLV-01, DLV-02)
  2. A Go TUI **dummy** mockup provides full navigation across all views with **no backend logic**, and every screen is screenshot-captured. (DLV-05)
  3. `agent-ui-ux-designer` critiques the HTML ↔ TUI-dummy visual diff, and its findings are resolved before approval. (DLV-02)
  4. **★ The user approves the complete design** (HTML + TUI-dummy screenshots); the approved images become the reference set for every later UI wave, and **no backend logic is written for any surface before this approval**. (DLV-08, DLV-05)

**Plans**: 12 plans in 6 waves
**UI hint**: yes

Plans:

**Wave 1** — foundation (parallel)

- [x] 02-01-PLAN.md — MUI v7 terminal-skin mockup workspace + shared app shell + recipe fixtures (DLV-01/02)
- [x] 02-02-PLAN.md — Go TUI dummy skeleton + surface registry + no-backend import-graph gate (DLV-05/02)

**Wave 2** — tooling (blocked on Wave 1)

- [x] 02-03-PLAN.md — Manifest-driven dual capture driver + dummy-nav PTY e2e + Makefile targets (DLV-01/05/02)

**Wave 3** — pilot (blocked on Wave 2)

- [ ] 02-04-PLAN.md — PILOT: create-flow (12 states) mockup+dummy+capture+parity, de-risks the pattern (DLV-01/02/05)

**Wave 4** — fan-out, 6 surfaces (parallel, blocked on the pilot)

- [ ] 02-05-PLAN.md — git-screen (7 states) (DLV-01/02/05)
- [ ] 02-06-PLAN.md — identity-manager (8 states, modals) (DLV-01/02/05)
- [ ] 02-07-PLAN.md — global-ssh (6 states) (DLV-01/02/05)
- [ ] 02-08-PLAN.md — global-git (6 states) (DLV-01/02/05)
- [ ] 02-09-PLAN.md — health (5 states, read-only) (DLV-01/02/05)
- [ ] 02-10-PLAN.md — fixer (6 states) (DLV-01/02/05)

**Wave 5** — assembly (blocked on fan-out)

- [ ] 02-11-PLAN.md — Comprehensive nav-proof e2e + full 50+50 capture + reference-set assembly (DLV-01/05/02)

**Wave 6** — ★ checkpoint (blocked on Wave 5)

- [ ] 02-12-PLAN.md — ★ DLV-08 single human approval; record **APPROVED:** in APPROVAL.md (DLV-08/02)

### Phase 3: Create Flow Backend

**Goal**: A developer creates an identity end-to-end — pick an algorithm, fill the SSH screen, test it against throwaway configs with the exact commands shown, and store it — with the live TUI matching the approved design.
**Depends on**: Phase 2 (approved design)
**Requirements**: SSHUI-01, SSHUI-02, SSHUI-03, SSHUI-04, SSHUI-05, TEST-01, TEST-02, TEST-03, KEY-06, DLV-04, DLV-06
**Success Criteria** (what must be TRUE):

  1. User picks a key algorithm from the catalog, fills the SSH screen (`Alias prefix` → `SSH Host` → `Real hostname` → `Port` default 443; fields clickable by mouse **and** keyboard-navigable, none buried), and sees a live `Host` block preview; a blank prefix yields the provider host verbatim (WYSIWYG). (SSHUI-01, SSHUI-02, SSHUI-03)
  2. User can reuse an existing key instead of generating one, and the macOS `Host *` globals block (`UseKeychain` + `AddKeysToAgent` guarded by `IgnoreUnknown`) is emitted correctly. (KEY-06, SSHUI-05)
  3. User runs the two-stage connectivity test (direct, then targeted-by-alias), each stage showing the **exact command run** and its real output, with `ssh -G` proving which `IdentityFile` resolves — all against throwaway temp files, never mutating live config until confirm. (TEST-01, TEST-02, SSHUI-04)
  4. On pass + confirmation, the identity persists to `~/.ssh/config` **or** the gitid-owned Include'd file, with backup. (TEST-03)
  5. **UI-wave gate**: `/mui` + `agent-ui-ux-designer` are engaged in plan/build/review; each create-flow screen has a PTY e2e test driving the **real** built binary; the live TUI passes the visual-regression diff against the approved screenshots. (DLV-04, DLV-06)

**Plans**: TBD
**UI hint**: yes

### Phase 4: Git Configuration Screen

**Goal**: After the SSH screens, a developer configures the per-identity Git fragment on its own screen, reviews it, and confirms the write of fragment + `includeIf` + `allowed_signers`.
**Depends on**: Phase 3 (SSH create flow precedes the git screen)
**Requirements**: GITUI-01, GITUI-02, GITUI-03, GITUI-04, GITUI-05
**Success Criteria** (what must be TRUE):

  1. A separate Git-config screen (**after** the SSH screens) collects per-identity fields — `user.name`/`user.email`, `gpg.format=ssh`, `user.signingkey` (path, not literal), `commit.gpgsign` — written to `~/.gitconfig.d/<identity>`. (GITUI-01, GITUI-02)
  2. User chooses the match strategy (`gitdir:` and/or `hasconfig:remote.*.url`, default `gitdir`, combinable) with a live `includeIf` preview. (GITUI-03)
  3. The `~/.ssh/allowed_signers` line is written with the email **byte-identical** to `user.email`. (GITUI-04)
  4. A read-only review screen precedes the write; on confirm, fragment + `includeIf` + `allowed_signers` are written with backup and idempotent managed blocks. (GITUI-05)
  5. **UI-wave gate**: `/mui` + `agent-ui-ux-designer` in plan/build/review; PTY e2e per screen on the real binary; the live TUI passes the visual-regression diff vs the approved screenshots. (DLV-04, DLV-06)

**Plans**: TBD
**UI hint**: yes

### Phase 5: Identity Manager

**Goal**: A developer manages all identities from the app's main view — seeing completeness/health state at a glance, opening SSH-first detail, and cloning, adding keys, rotating, or deleting with the right choices.
**Depends on**: Phase 4 (manager reconstructs identities from SSH + git artifacts)
**Requirements**: MGR-01, MGR-03, MGR-04, MGR-05, MGR-06, MGR-07, MGR-08, KEY-05, KEY-07, SHELL-01, SHELL-02, SHELL-03
**Success Criteria** (what must be TRUE):

  1. The identity list shows each identity's completeness/health state per row (complete / incomplete / git-only / key-unused / key-missing / fragment-path-missing, etc.), reconstructed from parsed managed blocks with **no sidecar DB**. (MGR-01, MGR-08)
  2. The detail view shows **SSH details first**, then Git, never rendering nonexistent git attributes for an SSH-only identity, and shows whether **that** identity is healthy (key resolves, fragment exists, signing wired). (MGR-03, MGR-07)
  3. User can clone an identity into a new **distinct** name (reusing the same key **or** generating a new one), generate a new key for an existing identity, and rotate an identity's key (artifacts re-point, the test flow re-runs). (MGR-04, MGR-05, KEY-05, KEY-07)
  4. Delete asks **"delete everything (SSH + Git + key)"** vs **"delete the Git identity only"** (applied with backup); all five primary views (Identities, Global SSH, Global Git, Health, Fixer) are reachable via palette + number keys, and every action is available from both the TUI and the Cobra CLI (completions for bash/zsh/fish). (MGR-06, SHELL-01, SHELL-02, SHELL-03)
  5. **UI-wave gate**: `/mui` + `agent-ui-ux-designer` in plan/build/review; PTY e2e per screen on the real binary; the live TUI passes the visual-regression diff vs the approved screenshots. (DLV-04, DLV-06)

**Plans**: TBD
**UI hint**: yes

### Phase 6: Global SSH Options

**Goal**: A developer reviews and safely fixes global SSH options that are dangerous when unset/misconfigured, with every option explained.
**Depends on**: Phase 5 (app shell / view set)
**Requirements**: GSSH-01
**Success Criteria** (what must be TRUE):

  1. A global-SSH-options screen surfaces dangerous-by-default options (e.g. `StrictHostKeyChecking`, `ForwardAgent`, `HashKnownHosts`, `IdentitiesOnly`, `AddKeysToAgent`, `UseKeychain`) and **explains each option's risk and recommended value**. (GSSH-01)
  2. Recommendations are advisory and fixable, **never blocking**; applying a change writes through the backup + idempotent managed-block chokepoint with confirmation. (GSSH-01)
  3. **UI-wave gate**: `/mui` + `agent-ui-ux-designer` in plan/build/review; PTY e2e per screen on the real binary; the live TUI passes the visual-regression diff vs the approved screenshots. (DLV-04, DLV-06)

**Plans**: TBD
**UI hint**: yes

### Phase 7: Global Git Options

**Goal**: A developer manages shared Git config — default branch, line endings, case, email, and recipe defaults — each option explained.
**Depends on**: Phase 6
**Requirements**: GGIT-01
**Success Criteria** (what must be TRUE):

  1. A global-git-options screen manages `init.defaultBranch` (highlighting **main vs master**), `core.ignorecase` (false), `core.autocrlf`/eol policy, global `user.email`, and recipe defaults (`push.autoSetupRemote`, `pull.rebase`, `fetch.prune`, aliases, color, `merge.conflictstyle`, `diff.colorMoved`) — each explained. (GGIT-01)
  2. Changes write through the backup + idempotent managed-block chokepoint with confirmation; content outside managed blocks is preserved verbatim. (GGIT-01)
  3. **UI-wave gate**: `/mui` + `agent-ui-ux-designer` in plan/build/review; PTY e2e per screen on the real binary; the live TUI passes the visual-regression diff vs the approved screenshots. (DLV-04, DLV-06)

**Plans**: TBD
**UI hint**: yes

### Phase 8: Health + Fixer

**Goal**: A developer opens a Health screen split into SSH and Git sections, sees redundant/contradictory config and per-identity health, and fixes problems in place.
**Depends on**: Phase 5 (per-identity health feeds the manager; reuses the doctor substrate)
**Requirements**: HLTH-01, HLTH-02, HLTH-03, HLTH-04, HLTH-05, HLTH-06, FIX-01, FIX-02
**Success Criteria** (what must be TRUE):

  1. The Health screen has **SSH** and **Git** sections and checks that config files exist and parse (syntax valid). (HLTH-01, HLTH-02)
  2. It detects repeated/overridden directives and duplicate managed/global blocks (e.g. multiple `Host *`) and contradictory settings where possible (e.g. `IdentitiesOnly no` with a specific `IdentityFile`; an `includeIf` targeting a missing fragment). (HLTH-03, HLTH-04)
  3. Health is computable for a **single identity** (feeding the manager's per-identity health) and globally, reusing the existing doctor families (deps/perms/coherence/orphans/signing/agent). (HLTH-05, HLTH-06)
  4. The Fixer presents SSH and Git problems in the two sections with severity + explanation + suggested fix, applied only with **confirmation and backup**, fixed in place. (FIX-01, FIX-02)
  5. **UI-wave gate**: `/mui` + `agent-ui-ux-designer` in plan/build/review; PTY e2e per screen on the real binary; the live TUI passes the visual-regression diff vs the approved screenshots. (DLV-04, DLV-06)

**Plans**: TBD
**UI hint**: yes

### Phase 9: Upload / Credentials Assist

**Goal**: After a valid identity exists, gitid uploads the public key for auth + signing **autonomously** when possible, falling back to clear manual instructions otherwise — never a checkpoint.
**Depends on**: Phase 3 (a valid identity + `.pub` must exist); Phase 5 (manager-triggered upload)
**Requirements**: UP-01, UP-02, UP-03
**Success Criteria** (what must be TRUE):

  1. gitid provides concrete steps to register the `.pub` for **authentication and signing** (GitHub = two registrations; GitLab = one). (UP-01)
  2. When `gh`/`glab` is present + **authenticated** and a valid identity exists, credential upload runs **autonomously** (no stop); the shown command equals the run command. (UP-02, UP-03)
  3. When `gh`/`glab` is absent or unauthenticated, upload falls back to a manual step and **never gates** create/copy. (UP-02, UP-03)
  4. **UI-wave gate**: `/mui` + `agent-ui-ux-designer` in plan/build/review; PTY e2e on the real binary; the live TUI passes the visual-regression diff vs the approved screenshots. (DLV-04, DLV-06)

**Plans**: TBD
**UI hint**: yes

### Phase 10: Linux Validation + Release Pipeline

**Goal**: The whole app is validated end-to-end on a mainstream Linux distro (alongside macOS), and tagged releases publish versioned, checksummed binaries.
**Depends on**: Phase 9 (whole product complete)
**Requirements**: PLAT-03, BUILD-03
**Success Criteria** (what must be TRUE):

  1. The full create → test → store → manage → health flow is validated **end-to-end on at least one mainstream Linux distro** (in addition to macOS); portability gaps are fixed or logged as accepted limitations. (PLAT-03)
  2. On a version tag, CI publishes the built binaries (darwin amd64/arm64, linux amd64) to **GitHub Releases with SHA-256 checksums**. (BUILD-03)
  3. The binary reports its build-stamped version (`gitid --version`, ldflags). (BUILD-03)

**Plans**: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 → 9 → 10

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Foundations, Spikes & CI | 7/7 | Complete | 2026-07-03 |
| 2. DESIGN — All Mockups (★ CHECKPOINT #1) | 3/12 | In Progress|  |
| 3. Create Flow Backend | 0/TBD | Not started | - |
| 4. Git Configuration Screen | 0/TBD | Not started | - |
| 5. Identity Manager | 0/TBD | Not started | - |
| 6. Global SSH Options | 0/TBD | Not started | - |
| 7. Global Git Options | 0/TBD | Not started | - |
| 8. Health + Fixer | 0/TBD | Not started | - |
| 9. Upload / Credentials Assist | 0/TBD | Not started | - |
| 10. Linux Validation + Release Pipeline | 0/TBD | Not started | - |
