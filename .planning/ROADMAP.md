# Roadmap: gitid — v1.0 TUI-First Redesign

## Overview

v1.0 rebuilds `gitid` as a **design-driven, screenshot-verified** terminal app that
creates and manages coherent SSH + Git identities (the `recipes/` end state: alias per
identity, `Port 443` alt-SSH, `IdentitiesOnly yes`, `includeIf` `hasconfig:`/`gitdir:`,
`allowed_signers` signing). **Phase 1** lays the non-UI foundations — screenshot
tooling, multi-algorithm keygen + local-capability probing, the dual SSH-storage
strategy (in-file / Include'd / adopt / migrate), the identity state-taxonomy core, and
a cross-OS GitHub Actions CI. **Phase 2** is the single human checkpoint: every surface
is designed as an HTML/`mui` mockup presented as an interactive web demo, mirrored by
a live, executable Go TUI demo, and **approved by the user**. From Phase 3 onward,
the approved Bubble Tea mockup (`cmd/gitid-dummy`) is the UI/UX reference. **Phases
3–9** wire each surface's backend behind that reference — create flow, git screen,
identity manager, global SSH options, global git options, health+fixer, and credential
upload — each gated by real-binary PTY workflow tests and a semantic comparison with
the Bubble Tea mockup. **Phase 10** validates the whole app end-to-end on Linux and
ships tagged, checksummed release binaries.

> **Active UI policy (Phases 3–10):** Historical HTML/MUI artifacts document the
> Phase-2 design process only. Automated verification compares the real compiled TUI
> against the live Bubble Tea mockup. Each difference is classified as an improvement
> or a defect; defects fail, improvements are recorded for the user's final milestone
> review. Pixel, PNG-byte, and HTML parity are not requirements.

The autonomous build run (`.planning/ONESHOT-GOAL-PROMPT.md`, driven via `/goal`) runs unattended except
for the **one** design-approval checkpoint (Phase 2); credential upload (Phase 9)
auto-runs when `gh`/`glab` is authenticated and a valid identity exists — it is not a
checkpoint. Phase numbering is **reset for this milestone**; the prior POC is archived
under `.planning/archive/0.0.1-poc-product-features-in-tui/`.

> **Granularity note:** config granularity is `coarse`, but this milestone's
> defining constraint is a **per-surface design-first UI wave** (PRD "Execution
> Phases" / DLV-01..06). The 10 phases are derived 1:1 from that delivery method, not
> padded — each of Phases 3–9 is one distinct user-facing surface that wires its
> backend behind the approved Bubble Tea mockup, then clears PTY e2e and semantic
> UI review.

## Phases

**Phase Numbering:**

- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [x] **Phase 1: Foundations, Spikes & CI** - Non-UI core (screenshot tooling, multi-algo keygen + probing, dual SSH storage, state taxonomy) + cross-OS CI, no product UI (completed 2026-07-03)
- [x] **Phase 2: DESIGN — All Mockups (★ CHECKPOINT #1)** - HTML/`mui` mockups for every surface + an interactive web demo + a live Go TUI demo, user-approved (completed 2026-07-06 — **APPROVED:** 2026-07-06 by Pepe)
- [x] **Phase 3: Create Flow Backend** - Algorithm → SSH screen → two-stage test → store, behind the approved design (completed 2026-08-24)
- [x] **Phase 4: Git Configuration Screen** - Per-identity git fragment + `includeIf` + `allowed_signers`, review → confirm → write (completed 2026-08-25)
- [x] **Phase 5: Identity Manager** - State-taxonomy list, SSH-first detail, clone / new-key / rotate / delete-choice, app view set (completed 2026-08-26)
- [x] **Phase 6: Global SSH Options** - Danger-aware, explained SSH config options; advisory + fixable (completed 2026-08-27)
- [x] **Phase 7: Global Git Options** - Baseline git config (main/master, eol, case, email) + recipe defaults, explained (completed 2026-08-28)
- [x] **Phase 8: Health + Fixer** - Two-section (SSH + Git) health with redundancy/contradiction detection and in-place fixes (completed 2026-08-28)
- [x] **Phase 9: Upload / Credentials Assist** - Auto-upload the `.pub` (auth + signing) when `gh`/`glab` authenticated; manual fallback (completed 2026-08-30)
- [x] **Phase 9.1: GitLab Real-Account Validation** - Prove the GitLab upload/delete path against a real, authenticated GitLab account, mirroring Wave 8's disposable-key protocol for GitHub
- [x] **Phase 9.2: Global Git Ignore Management** - A TUI view for managing a curated global gitignore (common tmp/venv/env patterns), reviewable before write
- [x] **Phase 9.3: Release CI/CD + Installer** - Tagged-release CI publishing checksummed binaries, plus a curl\|bash install script (completed 2026-09-01)
- [x] **Phase 9.4: TUI UX Consistency & Doctor/Fixer Parity** - Fix focus/read-only/toggle/submenu-affordance inconsistencies and Doctor-vs-Fixer count discoverability found in a user audit of Global SSH, Global Git, and Fixer (completed 2026-09-03)
- [x] **Phase 9.5: Full SSH/Git Properties Browser & Custom Key Entry** - Browse the full set of SSH/Git config properties beyond gitid's curated policy tables, plus add a custom global key (free-form for Git, directive-validated for SSH) (completed 2026-09-04)
- [ ] **Phase 9.6: Global Git/SSH Options Consistency Fixes** (INSERTED) - Type-aware rendering (bool/enum/text), `e`-to-edit, clarify the orange `!` ambiguity, fix the user.email/name duplicate-read-path bug across Options vs Set keys — found in a user audit of the Global Git/SSH Options screens
- [ ] **Phase 9.7: New-Identity Wizard Consistency Fixes** (INSERTED) - Visual grouping for the Key section (step 1) and Git identity fields (step 3), Test-connection color/type differentiation and warning persistence, typed log entries, fixed-width + consistent hint behavior across wizard text inputs — found in the same audit
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

**Goal**: Every product surface is designed as an HTML/`mui` mockup presented as an interactive web demo, mirrored by a live, executable Go TUI demo, and **approved by the user** — establishing the reference design the whole build is verified against.
**Depends on**: Phase 1 (screenshot tooling + core seams)
**Requirements**: DLV-01, DLV-02, DLV-05, DLV-08
**Success Criteria** (what must be TRUE — **gated on user approval**):

  1. Every surface (create flow, git screen, identity manager, global SSH, global git, health, fixer) has an HTML/`mui` mockup produced with the `/mui` skill and `agent-ui-ux-designer`, presented as a live interactive web demo (static per-screen reference routes kept alongside). (DLV-01, DLV-02)
  2. A LIVE, executable Go TUI **demo** (dummy data, in-memory state, **no backend logic**) provides full navigation and every interactive flow, mirroring the web demo 1:1. (DLV-05)
  3. `agent-ui-ux-designer` critiques the HTML ↔ TUI-dummy visual diff, and its findings are resolved before approval. (DLV-02)
  4. **★ The user approves the complete design** (interactive web demo + live Go TUI demo); the approved demos become the design reference for every later UI wave, and **no backend logic is written for any surface before this approval**. (DLV-08, DLV-05)

**Plans**: 15 plans in 9 waves
**UI hint**: yes

Plans:

**Wave 1** — foundation (parallel)

- [x] 02-01-PLAN.md — MUI v7 terminal-skin mockup workspace + shared app shell + recipe fixtures (DLV-01/02)
- [x] 02-02-PLAN.md — Go TUI dummy skeleton + surface registry + no-backend import-graph gate (DLV-05/02)

**Wave 2** — tooling (blocked on Wave 1)

- [x] 02-03-PLAN.md — Manifest-driven dual capture driver + dummy-nav PTY e2e + Makefile targets (DLV-01/05/02)

**Wave 3** — pilot (blocked on Wave 2)

- [x] 02-04-PLAN.md — PILOT: create-flow (12 states) mockup+dummy+capture+parity, de-risks the pattern (DLV-01/02/05)

**Wave 4** — fan-out, 6 surfaces (parallel, blocked on the pilot)

- [x] 02-05-PLAN.md — git-screen (7 states) (DLV-01/02/05)
- [x] 02-06-PLAN.md — identity-manager (8 states, modals) (DLV-01/02/05)
- [x] 02-07-PLAN.md — global-ssh (6 states) (DLV-01/02/05)
- [x] 02-08-PLAN.md — global-git (6 states) (DLV-01/02/05)
- [x] 02-09-PLAN.md — health (5 states, read-only) (DLV-01/02/05)
- [x] 02-10-PLAN.md — fixer (6 states) (DLV-01/02/05)

**Wave 5** — assembly (blocked on fan-out)

- [x] 02-11-PLAN.md — Comprehensive nav-proof e2e + full 50+50 capture + reference-set assembly (DLV-01/05/02)

**Wave 6** — live TUI demo (replaces the removed static reference set; blocked on Wave 5)

- [x] 02-13-PLAN.md — LIVE interactive Go TUI demo (cmd/gitid-dummy, dummy data, no backend) mirroring the web demo (DLV-05/02)

**Wave 7** — checkpoint-feedback polish (round-2 cross-AI consensus; blocked on Wave 6)

- [x] 02-14-PLAN.md — checkpoint feedback polish: semantic style contract (02-STYLE-SPEC.md + central Go Theme ↔ web theme.ts), ←/→ wizard navigation, first-class TUI stepper, focused/blurred field contours, stable hint zones, bounded previews, dimmed disabled nav, atomic slide-3 copy freeze (DLV-01/02/05)

**Wave 8** — checkpoint-2 route-back (operationalizes the binding 02-DESIGN-DECISIONS-CHECKPOINT-2 contract; blocked on Wave 7)

- [x] 02-15-PLAN.md — checkpoint-2 route-back polish: single-row color-only fields (kills the field box), always-expanded radios + header hint, terminal-glyph checkbox/radio, bracketed main nav + ActiveNavDimmed + plain-arrow view switch, reverted `Step n/4` stepper + per-step Shift-chord hints, one-row Git buttons, hoisted chord gate (Shift works at every step), click-to-focus, editable global-fallback user.email, affordance-audit footers (DLV-01/02/05)

**Wave 9** — ★ checkpoint (blocked on Wave 8)

- [x] 02-12-PLAN.md — ★ DLV-08 single human approval of the live demos; record **APPROVED:** in APPROVAL.md (DLV-08/02) — **APPROVED:** 2026-07-06 by Pepe

### Phase 3: Create Flow Backend

**Goal**: A developer creates an identity end-to-end — pick an algorithm, fill the SSH screen, test it against throwaway configs with the exact commands shown, and store it — with the live TUI matching the approved design.
**Depends on**: Phase 2 (approved design)
**Requirements**: SSHUI-01, SSHUI-02, SSHUI-03, SSHUI-04, SSHUI-05, TEST-01, TEST-02, TEST-03, KEY-06, DLV-04, DLV-06
**Success Criteria** (what must be TRUE):

  1. User picks a key algorithm from the catalog, fills the SSH screen (`Alias prefix` → `SSH Host` → `Real hostname` → `Port` default 443; fields clickable by mouse **and** keyboard-navigable, none buried), and sees a live `Host` block preview; a blank prefix yields the provider host verbatim (WYSIWYG). (SSHUI-01, SSHUI-02, SSHUI-03)
  2. User can reuse an existing key instead of generating one, and the macOS `Host *` globals block (`UseKeychain` + `AddKeysToAgent` guarded by `IgnoreUnknown`) is emitted correctly. (KEY-06, SSHUI-05)
  3. User runs the two-stage connectivity test (direct, then targeted-by-alias), each stage showing the **exact command run** and its real output, with `ssh -G` proving which `IdentityFile` resolves — all against throwaway temp files, never mutating live config until confirm. (TEST-01, TEST-02, SSHUI-04)
  4. On pass + confirmation, the identity persists to `~/.ssh/config` **or** the gitid-owned Include'd file, with backup. (TEST-03)
   5. **UI-wave gate**: each create-flow screen has a PTY e2e test driving the
   **real** built binary; the live TUI is compared semantically with the approved
   Bubble Tea mockup, and every difference is classified as an improvement or a
   defect. (DLV-04, DLV-06)

**Plans**: 11/11 plans executed in 8 waves
**UI hint**: yes

Plans:

- [x] 03-11-PLAN.md

- [x] 03-10-PLAN.md

**Wave 1** — parallel (disjoint files)

- [x] 03-01-PLAN.md — Backend gap functions: ensurePub encrypted-key fix, ScanReusableKeys, ResolvedViaCommand (KEY-06, TEST-01/02)
- [x] 03-02-PLAN.md — D-17 internal/tuikit extraction + Backend seam + FixtureBackend + restored no-backend gate (DLV-04, SSHUI-03)

**Wave 2** — real app shell (blocked on 03-02)

- [x] 03-03-PLAN.md — POC + tui/ archival, main rewire, real Backend composition root, storage-auto-detect persist + macOS globals (SSHUI-04/05, TEST-03)

**Wave 3** — create-flow form deltas (blocked on 03-03)

- [x] 03-04-PLAN.md — Provider-reactive autofill, recipe-faithful preview, alias-collision, reuse-key picker, D-16 banner (SSHUI-01/02/03, KEY-06)

**Wave 4** — test-outcome + scoped divergences (blocked on 03-04)

- [x] 03-05-PLAN.md — D-02 ReachableNotUploaded + D-03 copy-pub, exact-command render, D-19 git-disabled reason, D-18 functional skip (TEST-01/02)

**Wave 5** — UI-wave gates (blocked on 03-05) — IN PROGRESS: 2/3 tasks done

- [x] 03-06-PLAN.md — IN PROGRESS (2/3 tasks done, 1 remaining): Task 1 DLV-06 per-screen PTY e2e on the real binary (DONE, `57bda7b`) + Task 2 DLV-04.1 golden-text visual-regression gate (DONE, `4a9c939`) + Task 3 DLV-04.2 cross-AI review (packet assembled `9ddd102`, review NOT YET RUN — orchestrator-owned, see 03-06-SUMMARY.md "Cross-AI visual-regression review")

**Wave 6** — review-blocker remediation (blocked on 03-06)

- [x] 03-07-PLAN.md — Safe pre-confirm staging, automatic test chaining, transactional confirmed persistence, truthful failure/result ceremony, and fail-closed config state (SSHUI-04, TEST-01/03, KEY-06, DLV-06)

**Wave 7** — SSH trust-boundary correctness (blocked on 03-07)

- [x] 03-08-PLAN.md — Four-field D-20 form, strict SSH validation/effective collision checks, consistent algorithm catalog, verified key pairs, and complete unpinned `ssh -G` proof (SSHUI-01/02/03, TEST-01/02, KEY-06, DLV-06)

**Wave 8** — final visual evidence and independent reviews (blocked on 03-08)

- [x] 03-09-PLAN.md — Deterministic region-scoped visual gate, approval-provenance PNG evidence, and agent-ui-ux-designer plus Codex review closure (DLV-04, DLV-06)

### Phase 4: Git Configuration Screen

**Goal**: After the SSH screens, a developer configures the per-identity Git fragment on its own screen, reviews it, and confirms the write of fragment + `includeIf` + `allowed_signers`.
**Depends on**: Phase 3 (SSH create flow precedes the git screen)
**Requirements**: GITUI-01, GITUI-02, GITUI-03, GITUI-04, GITUI-05
**Success Criteria** (what must be TRUE):

  1. A separate Git-config screen (**after** the SSH screens) collects per-identity fields — `user.name`/`user.email`, `gpg.format=ssh`, `user.signingkey` (path, not literal), `commit.gpgsign` — written to `~/.gitconfig.d/<identity>`. (GITUI-01, GITUI-02)
  2. User chooses the match strategy (`gitdir:` and/or `hasconfig:remote.*.url`, default `gitdir`, combinable) with a live `includeIf` preview. (GITUI-03)
  3. The `~/.ssh/allowed_signers` line is written with the email **byte-identical** to `user.email`. (GITUI-04)
  4. A read-only review screen precedes the write; on confirm, fragment + `includeIf` + `allowed_signers` are written with backup and idempotent managed blocks. (GITUI-05)
   5. **UI-wave gate**: PTY e2e drives every screen in the real binary; automated
   review compares it with `cmd/gitid-dummy` and classifies every difference as an
   improvement or a defect. (DLV-04, DLV-06)

**Plans**: 1/4 plans executed in 4 waves
**UI hint**: yes

Plans:

- [x] 04-01-PLAN.md — Default Git-configuration tracer through the compiled real TUI and combined confirmed write
- [x] 04-02-PLAN.md — Round-trip includeIf strategies, provider insteadOf block, and doctor reservation
- [x] 04-03-PLAN.md — Reusable create/edit Git flow, truthful diffs, collision resume, and all-or-nothing transaction
- [x] 04-04-PLAN.md — Per-state real PTY and compiled real-vs-live-dummy semantic UI gate

### Phase 5: Identity Manager

**Goal**: A developer manages all identities from the app's main view — seeing completeness/health state at a glance, opening SSH-first detail, and cloning, adding keys, rotating, or deleting with the right choices.
**Depends on**: Phase 4 (manager reconstructs identities from SSH + git artifacts)
**Requirements**: MGR-01, MGR-03, MGR-04, MGR-05, MGR-06, MGR-07, MGR-08, KEY-05, KEY-07, SHELL-01, SHELL-02, SHELL-03
**Success Criteria** (what must be TRUE):

  1. The identity list shows each identity's completeness/health state per row (complete / incomplete / git-only / key-unused / key-missing / fragment-path-missing, etc.), reconstructed from parsed managed blocks with **no sidecar DB**. (MGR-01, MGR-08)
  2. The detail view shows **SSH details first**, then Git, never rendering nonexistent git attributes for an SSH-only identity, and shows whether **that** identity is healthy (key resolves, fragment exists, signing wired). (MGR-03, MGR-07)
  3. User can clone an identity into a new **distinct** name (reusing the same key **or** generating a new one), generate a new key for an existing identity, and rotate an identity's key (artifacts re-point, the test flow re-runs). (MGR-04, MGR-05, KEY-05, KEY-07)
  4. Delete asks **"delete everything (SSH + Git + key)"** vs **"delete the Git identity only"** (applied with backup); all five primary views (Identities, Global SSH, Global Git, Health, Fixer) are reachable via palette + number keys, and every action is available from both the TUI and the Cobra CLI (completions for bash/zsh/fish). (MGR-06, SHELL-01, SHELL-02, SHELL-03)
   5. **UI-wave gate**: PTY e2e drives every screen in the real binary; automated
   review compares it with `cmd/gitid-dummy` and classifies every difference as an
   improvement or a defect. (DLV-04, DLV-06)

**Plans**: 9 plans

Plans:

- [x] 05-01-PLAN.md — Tracer: one write chokepoint proven from the TUI and the CLI (Git-only delete) + the `--json`/table read surface + the D-01 command tree
- [x] 05-02-PLAN.md — Key-lifecycle primitives: archive dir + doctor-reserved registration, append-aware allowed_signers writer, per-provider rewrite remover
- [x] 05-03-PLAN.md — Pipeline decomposed into phases; rotate becomes a retirement ceremony; new-key becomes a repair action at its own key path; one state-and-ownership router (fully autonomous — the repair signer decision is resolved in-plan per DLV-08)
- [x] 05-04-PLAN.md — Delete-everything semantics: provider ref-count, recoverable key removal, shared-key downgrade, unmanaged-reference scan, pure DeletePlan
- [x] 05-05-PLAN.md — Clone: copy-versus-re-derive domain function, pre-filled wizard entry, full two-stage gate on same-key clones
- [x] 05-06-PLAN.md — tuikit surface: RotateIdentity action, the approved action menu, the delete-screen additions, and the key ceremony
- [x] 05-07-PLAN.md — Real backend wiring: rotate/new-key/delete transactions, exhaustive Persist, honest detail view, real per-identity health, five reachable views
- [x] 05-08-PLAN.md — CLI parity completion: adaptive-depth resolver, remaining write verbs, requirement-keyed parity matrix, headless parity e2e
- [x] 05-09-PLAN.md — DLV-04/DLV-06 gates: per-state PTY coverage, paired real-versus-dummy comparison, visual-gate registration, full battery

**UI hint**: yes

### Phase 6: Global SSH Options

**Goal**: A developer reviews and safely fixes global SSH options that are dangerous when unset/misconfigured, with every option explained.
**Depends on**: Phase 5 (app shell / view set)
**Requirements**: GSSH-01
**Success Criteria** (what must be TRUE):

  1. A global-SSH-options screen surfaces dangerous-by-default options (e.g. `StrictHostKeyChecking`, `ForwardAgent`, `HashKnownHosts`, `IdentitiesOnly`, `AddKeysToAgent`, `UseKeychain`) and **explains each option's risk and recommended value**. (GSSH-01)
  2. Recommendations are advisory and fixable, **never blocking**; applying a change writes through the backup + idempotent managed-block chokepoint with confirmation. (GSSH-01)
   3. **UI-wave gate**: PTY e2e drives every screen in the real binary; automated
   review compares it with `cmd/gitid-dummy` and classifies every difference as an
   improvement or a defect. (DLV-04, DLV-06)

**Plans**: 7 plans in 7 waves (run SEQUENTIALLY per LEARNINGS L11 — the pre-commit hooks lint the whole module). Re-planned after the cross-AI review in `06-REVIEWS.md`: the write-authority conflict was resolved onto one per-verb ceremony, and the two oversized waves were split (the registry/migration work out of Wave 1, and the CLI apart from the visual gate).
**UI hint**: yes

**Wave 1**

- [x] 06-01-PLAN.md — TRACER: one option end-to-end (probe set → provable provenance render → backed-up idempotent write) through `runGlobalSSHApply`, the SINGLE write authority; `EnsureGlobals` as the single `Host *` owner with `RenderGlobalBlock` deleted and every create/rotate/repair call site retargeted; placement in the resolved storage target, last (GSSH-01)

**Wave 2** *(blocked on Wave 1)*

- [x] 06-02-PLAN.md — Reserved-name registry consolidation (`global-ssh` + legacy), and the migration classification that registration would otherwise break: the globals block MOVES with the identities, the Include wiring stays put (GSSH-01)

**Wave 3** *(blocked on Wave 2)*

- [x] 06-03-PLAN.md — All six options: four-state model keyed on source class (never misattributing an external value to the user), per-source probe-error handling, UseKeychain/IdentitiesOnly special cases, platform + OpenSSH-version gates with a compatibility-unverified refusal, D-10 fixture correction pinned by a parity test (GSSH-01)

**Wave 4** *(blocked on Wave 3)*

- [x] 06-04-PLAN.md — Whole-config-graph shadowing simulation before the write and re-verification after it, as extra stages of the one write authority; the combined advisory ceremony with empty opt-in selection and journal-backed rollback; Options-sub-tab PTY e2e including inconclusive and commit-failure cases (GSSH-01, DLV-04, DLV-06)

**Wave 5** *(blocked on Wave 4)*

- [x] 06-05-PLAN.md — Migration engine hardened before a button reaches it (pure `PlanMigration` preview, backup-only seam, concurrency detection before the backups, abort that preserves the external edit); Storage & preview sub-tab wired to it; Global SSH demo banner removed; Storage-sub-tab PTY e2e (GSSH-01; re-exercises STORE-01/03/04, DLV-04, DLV-06)

**Wave 6** *(blocked on Wave 5)*

- [x] 06-06-PLAN.md — `gitid ssh` command group replacing the reserved noun, with a frozen command tree, a frozen versioned JSON schema and a frozen exit-status contract, wired to the same per-verb ceremonies the TUI calls; parity-matrix rows (GSSH-01, SHELL-03)

**Wave 7** *(blocked on Wave 6)*

- [x] 06-07-PLAN.md — Global SSH screens registered in the visual-regression gate with a classified allowlist, explicit HTML non-applicability and four negative controls; phase exit battery; cross-AI review packet with a `06-REVIEWS.md` closure table (GSSH-01, DLV-04, DLV-06)

### Phase 7: Global Git Options

**Goal**: A developer manages shared Git config — default branch, line endings, case, email, and recipe defaults — each option explained.
**Depends on**: Phase 6
**Requirements**: GGIT-01
**Success Criteria** (what must be TRUE):

  1. A global-git-options screen manages `init.defaultBranch` (highlighting **main vs master**), `core.ignorecase` (false), `core.autocrlf`/eol policy, global `user.email`, and recipe defaults (`push.autoSetupRemote`, `pull.rebase`, `fetch.prune`, aliases, color, `merge.conflictstyle`, `diff.colorMoved`) — each explained. (GGIT-01)
  2. Changes write through the backup + idempotent managed-block chokepoint with confirmation; content outside managed blocks is preserved verbatim. (GGIT-01)
   3. **UI-wave gate**: PTY e2e drives every screen in the real binary; automated
   review compares it with `cmd/gitid-dummy` and classifies every difference as an
   improvement or a defect. (DLV-04, DLV-06)

**Plans**: 6 plans in 6 sequential waves (LEARNINGS L11 — the whole-module pre-commit hooks make parallel executors block each other's commits)
**UI hint**: yes

**Wave 1**

- [ ] 07-01-PLAN.md — TRACER: the UI-free `internal/globalgit` probe/policy/classify engine, the `global-git` sentinel with POC-name adoption + reserved registration, `EnsureGlobalGit` as the one block owner, `runGlobalGitApply` as the one journal-backed write ceremony, and the `GlobalGitPlanner` seam — proven end to end on `init.defaultBranch` (GGIT-01; D-01, D-03, D-11.2)

**Wave 2** *(blocked on Wave 1)*

- [ ] 07-02-PLAN.md — The D9 fallback author as a two-field pair: `InsertBlockAfter`, its own early sentinel block above every `includeIf`, `runGitFallbackAuthorApply`, and the post-write matched/unmatched precedence proof (GGIT-01; D-04, D-05, D-06)

**Wave 3** *(blocked on Wave 2)*

- [ ] 07-03-PLAN.md — All twelve rows honest: the complete pinned table, the informational-vs-hard version gates and the `zdiff3`/`diff3` write substitution, the `set, differs` informational state, three-tier provenance, bundle aggregates, the pinned tally rule, the fixture corrections and the new frozen copy (GGIT-01; D-02, D-03, D-07, D-08, D-09, D-10)

**Wave 4** *(blocked on Wave 3)*

- [ ] 07-04-PLAN.md — The TUI surface at real size: the scrolling master list with a scroll-aware click mapping, the git-probe error state, the verified ceremony preview budget, the demo banner retired, and raw-keystroke PTY coverage of every state (GGIT-01, DLV-04, DLV-06)

**Wave 5** *(blocked on Wave 4)*

- [ ] 07-05-PLAN.md — The frozen `gitid git` CLI surface: four commands over the same two ceremonies, four versioned JSON envelopes, the exit-status table, the parity-matrix rows, the REQUIREMENTS §J correction, and headless e2e (GGIT-01; D-11.1)

**Wave 6** *(blocked on Wave 5)*

- [ ] 07-06-PLAN.md — Visual-regression closure: every Global Git state registered with a classified divergence allowlist and four working negative controls; the cross-AI review packet; the full exit battery run for real; GGIT-01 closed against the roadmap criteria (GGIT-01, DLV-04, DLV-06)

### Phase 8: Health + Fixer

**Goal**: A developer opens a Health screen split into SSH and Git sections, sees redundant/contradictory config and per-identity health, and fixes problems in place.
**Depends on**: Phase 5 (per-identity health feeds the manager; reuses the doctor substrate)
**Requirements**: HLTH-01, HLTH-02, HLTH-03, HLTH-04, HLTH-05, HLTH-06, FIX-01, FIX-02
**Success Criteria** (what must be TRUE):

  1. The Health screen has **SSH** and **Git** sections and checks that config files exist and parse (syntax valid). (HLTH-01, HLTH-02)
  2. It detects repeated/overridden directives and duplicate managed/global blocks (e.g. multiple `Host *`) and contradictory settings where possible (e.g. `IdentitiesOnly no` with a specific `IdentityFile`; an `includeIf` targeting a missing fragment). (HLTH-03, HLTH-04)
  3. Health is computable for a **single identity** (feeding the manager's per-identity health) and globally, reusing the existing doctor families (deps/perms/coherence/orphans/signing/agent). (HLTH-05, HLTH-06)
  4. The Fixer presents SSH and Git problems in the two sections with severity + explanation + suggested fix, applied only with **confirmation and backup**, fixed in place. (FIX-01, FIX-02)
   5. **UI-wave gate**: PTY e2e drives every screen in the real binary; automated
   review compares it with `cmd/gitid-dummy` and classifies every difference as an
   improvement or a defect. (DLV-04, DLV-06)

**Plans**: 8 plans in 8 sequential waves (LEARNINGS L11 — the whole-module pre-commit hooks make parallel executors block each other's commits)
**UI hint**: yes

Plans:

**Wave 1**

- [x] 08-01-PLAN.md — TRACER: two-pipeline convergence (doctor.Run() replaces identity.Problem as the sole findings source), Finding.Target field, TabID 4→5 split, Health/Fixer tabs wired to real data, minimal `gitid health --json` (HLTH-01, HLTH-03, HLTH-06, FIX-02, DLV-06)

**Wave 2** *(blocked on Wave 1)*

- [x] 08-02-PLAN.md — The flagship fix-in-place pipeline: hand-written IdentitiesOnly+IdentityFile contradiction check, the D-09 surgical single-directive rewrite primitive, D-10 verification loop, D-11 typed confirm, D-13 re-run-all-after-fix, D-14 convergence alarm, `gitid fix` CLI (HLTH-04, FIX-01)

**Wave 3** *(blocked on Wave 2)*

- [x] 08-03-PLAN.md — D-06 tolerance fixes (Orphans Class-1 downgrade, reserved-path registry for Deps.KeyPaths) closing the false-positive-loop precedent, plus HLTH-02's Files-family parse gates and the parse-error render frame (HLTH-02, HLTH-06)

**Wave 4** *(blocked on Wave 3)*

- [x] 08-04-PLAN.md — Baseline-family new checks: the global-gitignore pair fix (dormant Phase 7 substrate) and the "set, differs" informational hard cap (HLTH-03, HLTH-04, FIX-01)

**Wave 5** *(blocked on Wave 4)*

- [x] 08-05-PLAN.md — Remaining Coherence-family new checks reusing Phase 6/7 probes verbatim: shadowed-option, author-resolution, directive-above-block, and the includeIf-missing-fragment Family reconciliation (HLTH-03, HLTH-04, HLTH-06)

**Wave 6** *(blocked on Wave 5)*

- [x] 08-06-PLAN.md — Render-layer completion: Health's negatively-asserted read-only gate, the Fixer's complete fixable set, D-04's per-identity health deep-link from the Identity Manager, D-16's batch queue-halt message (HLTH-01, HLTH-05, FIX-01, FIX-02)

**Wave 7** *(blocked on Wave 6)*

- [x] 08-07-PLAN.md — CLI parity completion: the hidden `doctor` alias + `doctor --fix` shim, a versioned JSON envelope, tiered exit codes, parity-matrix rows, headless CLI e2e (FIX-01, HLTH-05)

**Wave 8** *(blocked on Wave 7)*

- [x] 08-08-PLAN.md — DLV-04/DLV-06 UI-wave gates: raw-keystroke PTY e2e per screen state, the visual-regression gate with a classified divergence allowlist, the cross-AI review packet, and REQUIREMENTS.md closure (HLTH-01..06, FIX-01, FIX-02, DLV-04, DLV-06)

### Phase 9: Upload / Credentials Assist

**Goal**: After a valid identity exists, gitid uploads the public key for auth + signing **autonomously** when possible, falling back to clear manual instructions otherwise — never a checkpoint.
**Depends on**: Phase 3 (a valid identity + `.pub` must exist); Phase 5 (manager-triggered upload)
**Requirements**: UP-01, UP-02, UP-03
**Success Criteria** (what must be TRUE):

  1. gitid provides concrete steps to register the `.pub` for **authentication and signing** (GitHub = two registrations; GitLab = one). (UP-01)
  2. When `gh`/`glab` is present + **authenticated** and a valid identity exists, credential upload runs **autonomously** (no stop); the shown command equals the run command. (UP-02, UP-03)
  3. When `gh`/`glab` is absent or unauthenticated, upload falls back to a manual step and **never gates** create/copy. (UP-02, UP-03)
   4. **UI-wave gate**: PTY e2e drives the real binary; automated review compares
   it with `cmd/gitid-dummy` and classifies every difference as an improvement or
   a defect. (DLV-04, DLV-06)

**Plans**: 8 plans (8 sequential waves; delivered 2026-08-30)
**UI hint**: yes

Plans:

**Wave 1**

- [x] 09-01-PLAN.md — D-08 design-contract amendments (create-flow + identity-manager FIELDS.md, APPROVAL addendum, the `u` key claim) and the frozen upload copy block registered in `gate-copy-freeze` (UP-01, UP-03)

**Wave 2** *(blocked on Wave 1)*

- [x] 09-02-PLAN.md — TRACER: one GitHub authentication-key upload wired end-to-end — `DetectFor`, boundary-matched `ProviderForHostname`, `buildUploaderDeps` + nil-guard + bounded runner, the async Backend seam, the `testUpload` sub-beat, the step-0 checkbox, the fixture, a raw-keystroke PTY proof, AND the suite-wide hermetic provider-PATH boundary that stops any e2e run from reaching a real `gh`/`glab` (UP-02, UP-03)

**Wave 3** *(blocked on Wave 2)*

- [x] 09-03-PLAN.md — `internal/uploader` engine completion: the D-16 per-registration result, D-12's combined GitLab usage type, D-15's provider inventory and validated delete call, D-14's failure classifiers, D-07's machine-scoped title, per-registration upload titles, and the ASVS V5 public-key-only invariant enforced by CONTENT rather than by filename suffix (UP-01, UP-02, UP-03)

**Wave 4** *(blocked on Wave 3)*

- [x] 09-04-PLAN.md — the complete wizard upload section: all four checkbox states, the inventory-driven missing-type diff, classified reasons, the byte-identical manual fallback, D-17's post-upload confirmation, and D-18's no-persisted-state assertion (UP-01, UP-02, UP-03)

**Wave 5** *(blocked on Wave 4)*

- [x] 09-05-PLAN.md — CLI surface: the ONE shared orchestration and outcome printer, the `register-key` verb, `--no-upload` on the four write verbs, parity-matrix rows, and headless e2e (UP-01, UP-02, UP-03)

**Wave 6** *(blocked on Wave 5)*

- [x] 09-06-PLAN.md — Identity Manager: the `paneRegisterKey` modal, the action-menu fifth row with the row-count desync fix, the rotate/repair upload beat, and D-04's interactive old-key delete offer (UP-01, UP-03)

**Wave 7** *(blocked on Wave 6)*

- [x] 09-07-PLAN.md — DLV-04/DLV-06 UI-wave gates: raw-keystroke PTY coverage per new state, D-09's fresh approved frames, the visual-regression registry with classified divergences and four negative controls, and the paired real-vs-dummy PTY comparison (UP-01, UP-02, UP-03)

**Wave 8** *(blocked on Wave 7)*

- [x] 09-08-PLAN.md — the ONESHOT-policy real-account GitHub validation (opt-in, disposable prefixed keys, ID-scoped cleanup, final sweep) plus UP-01/UP-02/UP-03 closure (UP-01, UP-02, UP-03)

### Phase 09.7: New-Identity Wizard Consistency Fixes: visual grouping for Key section (step 1) and Git identity fields (step 3), Test-connection color/type differentiation and warning persistence, typed log entries, fixed-width + consistent hint behavior for all wizard text inputs (INSERTED)

**Goal:** [Urgent work - to be planned]
**Requirements**: TBD
**Depends on:** Phase 9
**Plans:** 0 plans

Plans:

- [ ] TBD (run /gsd-plan-phase 09.7 to break down)

### Phase 9.1: GitLab Real-Account Validation

**Goal**: The GitLab upload/delete path is proven against a real, authenticated GitLab account — not just offline `glab` PATH shims.
**Depends on**: Phase 9 (GitHub real-account protocol proven in Wave 8, to be mirrored here)
**Requirements**: UP-04
**Success Criteria** (what must be TRUE):

  1. A real-account validation run, following the same disposable-key/idempotent-cleanup/final-sweep protocol Wave 8 established for GitHub, executes against a real GitLab account and closes UP-04 with recorded evidence. (UP-04)
  2. The run is opt-in, behind its own build tag, never a prerequisite of any routine gate (`make test`/`make lint`/`make test-e2e`/CI). (UP-04)

**Plans**: 2 plans

Plans:

- [x] 09.1-01-PLAN.md — (wave 1) authorize the real-GitLab contact, then land the read-only end-to-end tracer: the `realaccountgitlab`-tagged test file, its full Makefile wiring, the exact-scope preflight proven live, and the LIFO cleanup machinery with account-free unit tests (UP-04)
- [x] 09.1-02-PLAN.md — (wave 2) the single disposable-key round trip against the real GitLab account through the compiled binary, plus the ONESHOT GitLab policy addendum and UP-04's outcome-conditional closure (UP-04)

### Phase 9.2: Global Git Ignore Management

**Goal**: A TUI view lets the user manage a global gitignore file with a curated, reviewable set of common ignore patterns.
**Depends on**: Phase 9 (whole product complete)
**Requirements**: GIGN-01
**Success Criteria** (what must be TRUE):

  1. A TUI surface pre-loads a curated set of common ignore patterns (tmp/venv directories, `.env`/`.env.*` with `!.env.example` negation, and similar common patterns) and lets the user review and freely edit them — with a Reset to defaults action — before writing to the global ignore file (`core.excludesFile`). (GIGN-01) — *the original "toggle each" wording is superseded by 09.2-CONTEXT.md D-02/D-03: the user chose a pre-loaded, freely editable text block over per-pattern toggle rows, which also removes any enforced coupling between the `.env` family lines.*
  2. Nothing is written without explicit review/confirmation, matching every other gitid write path's discipline. (GIGN-01)

**Plans**: 3 plans

Plans:

- [x] 09.2-01-PLAN.md — (wave 1) tracer: a 6th top-level nav entry that reads the managed `~/.gitignore_global` block, refuses a malformed or unwired machine, and carries one preview-pinned candidate through the shared review ceremony to a backed-up write of the FULL `core.excludesFile` pair — wired in both binaries and proven over a real PTY (GIGN-01)
- [x] 09.2-02-PLAN.md — (wave 2) extend the one curated catalog to the venv/env/`!.env.example` seed under comment-header groups, and make doctor's five-branch pair check plus its excludesfile fix tolerate — and never revert — a user-edited block while still catching an unset key, a wrong target, a dangling pointer and an empty block (GIGN-01)
- [x] 09.2-03-PLAN.md — (wave 3) replace the read-only body with the pre-loaded editable text area plus Reset to defaults, prove edit→review→write and a typed-sentinel refusal over a real PTY, register the screen in `RequiredScreenSpecs`, capture the phase frames, and close GIGN-01 behind the phase's UI-SPEC/UI-REVIEW artifacts (GIGN-01)

### Phase 9.3: Release CI/CD + Installer

**Goal**: Tagged releases publish versioned, checksummed binaries, and a hosted curl\|bash script installs the right one for the caller's OS/arch.
**Depends on**: Phase 9 (whole product complete)
**Requirements**: BUILD-03, BUILD-05
**Success Criteria** (what must be TRUE):

  1. On a version tag, CI publishes the built binaries to GitHub Releases with SHA-256 checksums; the binary reports its build-stamped version (`gitid --version`). (BUILD-03)
  2. A hosted install script detects the caller's OS/arch, downloads the matching released binary, verifies its checksum, and installs it to `PATH`. (BUILD-05)

**Plans**: 2 plans

Plans:

- [x] 09.3-01-PLAN.md — (wave 1) tracer: version-stamp `gitid --version` through optional-default ldflags on the EXISTING `build-cross`, add the `checksums` manifest target, and prove the whole chain locally — stamped build → manifest → `scripts/install.sh` download/verify/install → the installed binary reporting its own stamp — then wire the `v*`-tag-scoped `release:` job (job-scoped `contents: write`, `needs:` both routine gates, five published assets, prerelease/latest state classified from the tag AT publication with no post-publish correction) and expand the installer across all four platforms and every refusal path (BUILD-03, BUILD-05)
- [x] 09.3-02-PLAN.md — (wave 2) the `sh -n` shell gate wired into `make lint`, the README install documentation (one-liner plus a manual path filtered to the single downloaded asset) and Status refresh, the three stale comments this phase invalidates, then — behind a blocking-human decision on the first public tag, its prerelease-encoding suffix and the ref it is cut from — cut the real release and verify it over the real network (five assets, checksums re-verified after download, `--version` matched against `<tag>^{commit}`, `curl | sh` into a throwaway HOME), never moving a published tag, and close BUILD-03/BUILD-05 strictly per the evidence-to-closure table (BUILD-03, BUILD-05)

### Phase 9.4: TUI UX Consistency & Doctor/Fixer Parity

**Goal**: Fix the focus, read-only-affordance, toggle-visibility, submenu-discoverability, and Doctor/Fixer count-parity inconsistencies found in a direct user audit of the Global SSH, Global Git, and Fixer screens.
**Depends on**: Phase 9 (whole product complete)
**Requirements**: UXP-01, UXP-02, UXP-03, UXP-04, UXP-05
**Success Criteria** (what must be TRUE):

  1. Global SSH's Options screen focuses a row on activation by the same rule Global Git's Options screen already uses (reset on every activation), with no construction-time hardcoded row surviving re-activation. (UXP-01)
  2. A non-selectable options row renders with the same visual pattern (checkbox-column placeholder) across Global SSH and Global Git, and the reason it is non-selectable is visible on the row, not only inferable. (UXP-02)
  3. The selectable-row toggle indicator is visually more prominent than the current glyph. (UXP-03)
  4. A screen with local sub-tabs (e.g. Global SSH's Options/Storage & preview) is visually distinguishable from top-level main-menu navigation. (UXP-04)
  5. Health and Fixer are merged back into a single "Doctor" tab (reversing Phase 8's FIX-02 split) that lists every finding and offers inline fix/fix-all, so no separate screen's finding count can read as contradicting another's; the write-confirmation ceremony (preview + typed-confirm + backup) is unchanged. (UXP-05)

*Note: criterion 5's tab arithmetic was drafted as "5 tabs to 4" against Phase 8's shipped shell. Phase 9.2 has since added a sixth top-level tab (Global Git Ignore), so the shipped math is **6 tabs to 5** — see 09.4-UI-SPEC.md Known Divergence #1, which is binding. REQUIREMENTS.md UXP-05 now states the same 6-to-5 arithmetic.*

**Plans**: 4 plans

Plans:

- [x] 09.4-01-PLAN.md — (wave 1) the one-way `checkpoint:decision` gate for reversing FIX-02, then the compile-atomic Doctor merge tracer: the `TabID` enum 6→5, `App.screens` arity, the number-key/palette/right-arrow plumbing, `checkFixBatchHalt`'s type assertion, all three `TabHealth` routing sites, a merged `doctorModel` in a new `doctor_screen.go` (Health's unfiltered findings scope + Fixer's ceremony machinery verbatim), every compile-forced test call site, three regression guards proven RED by a wrong-but-compiling source reversion, and the PTY/screenshot retarget across all seven runtime-only surfaces — including hardening the two e2e header-status extractors that would otherwise degrade to a vacuous pass — that closes the wave GREEN on all four gates (UXP-05)
- [x] 09.4-02-PLAN.md — (wave 2) the stale tab-name sweep across every user-visible surface (including the doctor engine's own SuggestedFix output, the help key map, and the one string frozen by `gate-copy-freeze`), the merged status copy, the REQUIREMENTS/ROADMAP 6→5 correction, the visual-regression `ScreenSpec`/region/allowlist consolidation with its fail-path re-verified and its screenshot-tagged tests actually RUN, and the rename plus semantic strengthening of all eight real-PTY behaviors on the merged tab (UXP-05)
- [x] 09.4-03-PLAN.md — (wave 3) one derived initial-focus rule on both option screens, one uniform checkbox column with a dot placeholder for non-selectable rows and a bracket toggle for selectable ones (with both click needles moved in lockstep), the strengthened set-but-differs no-op explanation pinned as a full sentence in the copy-freeze gate, Global Git's missing zero-row render branch, and real-PTY proof of all three requirements in the compiled binary including a raw SGR mouse toggle (UXP-01, UXP-02, UXP-03)
- [x] 09.4-04-PLAN.md — (wave 4) the missing `TestSubTabStrip*` scaffold committed GREEN, then the bordered accent-colored sub-tab strip (its RED test observed but never committed) with its row accounting collapsed from four literals to one source and its click hit-test moved with it, the fixed-geometry fit settled by measurement across all five strip-bearing states, and the Global SSH visual evidence reconciled plus proven through a real SGR mouse click in a PTY (UXP-04)

### Phase 9.5: Full SSH/Git Properties Browser & Custom Key Entry

**Goal**: The user can browse the full set of SSH and Git configuration properties resolved on their machine, beyond gitid's curated policy tables, and add a custom global key on either side.
**Depends on**: Phase 9 (whole product complete)
**Requirements**: PROP-01, PROP-02, PROP-03, PROP-04
**Success Criteria** (what must be TRUE):

  1. A new Global SSH sub-tab lists every SSH directive `ssh -G` resolves (the full, bounded set), as a flat type-to-filter list. (PROP-01) — MET: `TestAllDirectivesReturnsEveryResolvedKey` (internal/globalssh/properties_test.go) + real-PTY `TestGlobalSSH_RealPTYAllDirectivesBrowse`/`...Filter`.
  2. A new Global Git sub-tab lists every git config key actually set across all scopes, with its origin file, as a flat type-to-filter list. (PROP-02) — MET: `TestAllSetKeysReturnsEverySetKeyWithProvenance` (internal/globalgit/properties_test.go) + real-PTY `TestGlobalGit_RealPTYSetKeysBrowse`/`...Filter`.
  3. From the Git properties browser, the user can add an arbitrary global key=value pair, free-form, through the standard review-before-write ceremony. (PROP-03) — MET: real-PTY `TestGlobalGit_RealPTYCustomKeyWrite` asserts the on-disk managed-block content after write.
  4. From the SSH properties browser, the user can add a custom global SSH directive, but its name is validated against OpenSSH's known-directive list plus the existing prove-before-write `ssh -G` verification before any write. (PROP-04) — MET: real-PTY `TestGlobalSSH_RealPTYCustomDirectiveRejectedNameNeverWrites` (byte-identical target on rejection) + `TestGlobalSSH_RealPTYCustomDirectiveWrite` (accepted write on disk).

**Plans**: 5 plans

Plans:

- [x] 09.5-01-PLAN.md — (wave 1) TRACER: the SSH `All directives` vertical slice end-to-end — `globalssh.AllDirectives` atop the existing `ssh -G` probe, the `SSHDirectiveView` DTO and one-method `SSHPropertiesBrowser` seam with both implementations, Global SSH's third sub-tab with its flat filterable master-detail body, the frozen copy pinned byte-exactly and registered in the grep gate, and real-PTY proof through the compiled binary (PROP-01)
- [x] 09.5-02-PLAN.md — (wave 2) the sub-tab strip extracted into one shared renderer, Global Git's FIRST strip with its Options content re-homed byte-identically and its keyboard/click/row-budget contracts moved in lockstep, then `globalgit.AllSetKeys` through a new seam into the honestly-scoped `Set keys` list with origin and scope, its two distinct empty states, and real-PTY proof (PROP-02)
- [x] 09.5-03-PLAN.md — (wave 3) the `customkeys.go` engine with its last-dot key split proven against the real `git` binary and its sentinel name reserved-registered in the same commit, `runCustomGitKeyWrite` as the ONE backed-up rollback-proven writer, and the two-stage free-form custom-key flow proven in a real terminal against the bytes on disk (PROP-03)
- [x] 09.5-04-PLAN.md — (wave 4) the combined-output probe seam and `ProveCustomDirective`'s three-way staged-config `ssh -G` classification, `runCustomSSHDirectiveWrite` into the existing `Host *` block with its own non-vacuous post-write re-read, and the three-stage flow whose un-skippable validation gate is proven in a real PTY to leave the config byte-identical on rejection (PROP-04)
- [x] 09.5-05-PLAN.md — (wave 5) visual-regression registration for every new render state with its fail path hand-verified, phase-parameterised frame promotion with machine-checked provenance, and the clause-by-clause requirement closeout behind a full gate battery run at close (PROP-01, PROP-02, PROP-03, PROP-04)

### Phase 9.6: Global Git/SSH Options Consistency Fixes

**Goal**: An option row on either Global screen renders and edits according to what its value actually is — a toggle where gitid can only apply-or-not, a value plus an arrow-cycle where the value set is closed, a value plus the wizard's own text field where it is free text — with one uniform `e` / Enter / Esc gesture across both screens, an unambiguous warning glyph, and no key ever shown in two places at once.
**Depends on**: Phase 9.5 (this phase's sub-tab exclusion and relabel act on the properties browser 9.5 shipped)
**Requirements**: UXG-01, UXG-02, UXG-03, UXG-04, UXG-05
**Success Criteria** (what must be TRUE):

  1. A closed-value-set row (`StrictHostKeyChecking`, `AddKeysToAgent`, `merge.conflictstyle`, `diff.colorMoved`) shows its resolved value on the row and opens an arrow-cycle selector on `e`; a free-text row (`user.email` global fallback, `init.defaultBranch`) shows its value and opens the New-Identity wizard's own fixed-width field; an apply-or-not row and a multi-key preset row render exactly as they do today. (UXG-01)
  2. The trigger key, the commit key and the dismiss key are identical across every editable row on both Options screens, and Esc restores the value the row held before editing began rather than merely leaving edit mode. (UXG-01)
  3. The orange `!` glyph fires only for the needs-action state; a set-but-differs row renders no `!` and keeps its explanatory second line, identically on Global Git and Global SSH. (UXG-02)
  4. No git config key appears on both the Global Git Options sub-tab and the general-keys sub-tab — bundle member keys and the two fallback-author keys included — and the two sub-tabs' key sets still partition the raw probe result with nothing hidden. (UXG-03)
  5. The general-keys sub-tab is relabelled to match what it now contains, with the frozen-copy gate, both string constants and all eight pinned assertions moved in one commit. (UXG-04)
  6. A user-chosen value is staged by Enter and reaches disk only through the existing preview + typed-confirm + timestamped-backup ceremony; the per-alias scope refusal and the hard version gate still apply to an overridden value, and free text is validated by the existing `internal/gitconfig` validators. (UXG-05)

*Note: the Global Git / Global SSH `Selectable()` asymmetry for set-but-differs rows (Git never selectable, SSH always selectable) is a documented, deliberate NON-goal of this phase — see 09.6-01-PLAN.md decision PD-4. The 4-state classifier enums in `internal/globalgit/classify.go` and `internal/globalssh/classify.go` are likewise untouched; every fix here is rendering- and read-path-level.*

**Plans**: 6 plans in 6 sequential waves
**UI hint**: yes — UI contract: `.planning/phases/09.6-global-git-ssh-options-consistency-fixes-type-aware-renderin/09.6-UI-SPEC.md` (`.planning/ONESHOT.md` Per-Phase Checklist item 2)

*Plan-set note (cycle 2, cross-AI review 4a799a8): the phase was restructured from five plans to
six. Review found that the per-screen editor plans each invented a piece of an architecture with
no shared contract defined first, and that the override write path as described would have
written to disk during preview. Plan 09.6-01 is now a dedicated wave-1 contract plan — the
value-kind vocabulary, the override request type, both option-value seams, one overlay builder
per screen shared by preview and commit, and a real git ref-name validator — landing before any
screen grows an editor. Every plan carries `cross_ai: true` per `.planning/ONESHOT.md` rule 12,
and each plan carries a Review Dispositions Ledger recording how it resolves the findings routed
to it.*

*Plan-set note (cycle 3, cross-AI review a49a50c): the six-plan structure is unchanged; every
plan was revised in place and each ledger gained a Round 2 section. Cycle 2's three HIGH
findings are resolved: the mandatory `09.6-UI-SPEC.md` was written (ONESHOT.md item 2 carries no
exemption clause for a phase with a TUI surface, and this phase changes rendering and
interaction on two shipped screens); Route-B visual evidence became machine-checkable via a new
`EvidenceTest` field, a `ValidateScreenSpecs` requirement, a source walk resolving it against
`e2e/`, and an actual anchored run (09.6-06 PD41); and 09.6-04's "always-reserved helper row"
parity claim was corrected against the live source — the wizard's `formFieldLine`/`helperLine`
primitives are genuinely reused and the always-reserved PROPERTY is delivered at the Options
editor's own call site, while wizard-wide unification stays Phase 9.7's D-07 (09.6-04 PD36).
Twelve of the fourteen actionable MEDIUM/LOW items were also closed with concrete tests
(PD31-PD43); the two not taken are recorded as deferrals with reasoning in the plans' Round 2
ledgers.*

Plans:

- [ ] 09.6-01-PLAN.md — (wave 1) CONTRACT: the value-kind vocabulary across both policy tables, both DTOs and both fixtures with an explicit boundary conversion; the override request type, the two option-value seams embedded in `Backend`, one overlay builder per screen shared by preview and commit, a plan method that provably writes nothing, `WriteRequestedValueFor` so an override can never route around the hard gate, and `gitconfig.ValidateDefaultBranch` pinned against the real `git check-ref-format` binary; plus the orange `!` narrowed to needs-action on BOTH screens with the SSH test that pins today's defect inverted in the same commit (UXG-01, UXG-02, UXG-05)
- [ ] 09.6-02-PLAN.md — (wave 2) TRACER: the `StrictHostKeyChecking` vertical slice end-to-end — the shared editor state machine with one mutually-exclusive mode enum, the key guard ahead of the sub-tab-switch binding, the inline value cell, edit- and apply-eligibility stated per row state so a staged override on an already-set row is not silently dropped, the explicit conditional dispatch at both named ceremony call sites, the editor copy frozen in both gates, and real-PTY proof through the compiled binary against the bytes on disk (UXG-01, UXG-05)
- [ ] 09.6-03-PLAN.md — (wave 3) Global Git's enum rows: the inline value cell with the checkbox column provably unchanged, the SHARED editor reused rather than ported, the guard placed ahead of BOTH sub-tab-switch sites including the zero-options branch, and the hard version gate re-proven from this screen's own dispatch path so a cycled `zdiff3` still downgrades on old git (UXG-01, UXG-05)
- [ ] 09.6-04-PLAN.md — (wave 4) the free-text half plus the reported read-path defect: the Options fallback row re-sourced from `ReadGitFallbackAuthor` so it shows both owned fields on first entry, the fallback pair regestured off Enter with a real commit/dismiss split and a two-field snapshot, `fieldEditing` folded into the one mode enum, and `init.defaultBranch` given the wizard's own `formFieldLine`/`helperLine` editor validated by git's real ref grammar (UXG-01, UXG-05)
- [ ] 09.6-05-PLAN.md — (wave 5) one key, one home: the raw-key exclusion that skips rather than tags, `ManagedRawGitKeys()` and a real-machine partition proof stated in raw-key space, an externally-set author pair kept visible with its origin on the Options row, `GitSetKeyView.PolicyBacked` removed rather than left permanently false, and the sub-tab relabel enforced by a source-walk absence test rather than a one-off search (UXG-03, UXG-04)
- [ ] 09.6-06-PLAN.md — (wave 6) visual-regression registration where entry, capture driver and fixture data ship as ONE commit per state with an explicit reachability route and each fail path observed and reverted, re-promotion of the PTY evidence the rename invalidated, and clause-by-clause requirement closeout behind a full gate battery run at close (UXG-01, UXG-02, UXG-03, UXG-04, UXG-05)

### Phase 10: Linux Validation + Release Pipeline

**Goal**: The whole app is validated end-to-end on a mainstream Linux distro, alongside macOS.
**Depends on**: Phase 9.3 (release pipeline validates the same build matrix this phase exercises on Linux)
**Requirements**: PLAT-03, BUILD-03
**Success Criteria** (what must be TRUE):

  1. The full create → test → store → manage → health flow is validated **end-to-end on at least one mainstream Linux distro** (in addition to macOS); portability gaps are fixed or logged as accepted limitations. (PLAT-03)
  2. On a version tag, CI publishes checksummed, provenance-attested `tar.gz` release archives via goreleaser, with a build-stamped `gitid --version`, a hardened install script, and a Homebrew tap — replacing Phase 9.3's raw-binary pipeline with the D-05..D-16 shape. (BUILD-03)

**Plans**: 6 plans in 4 waves
**UI hint**: no

Plans:

**Wave 1** — parallel (disjoint files)

- [ ] 10-01-PLAN.md — TRACER: internal/version hybrid resolve (D-09/D-10/D-11), wired through the Makefile, `gitid --version`, `gitid version` (+ `--json`), and an additive TUI help line
- [ ] 10-02-PLAN.md — Fedora container CI job (D-01/D-02) + a named ssh -V distro-suffix regression case (D-03)
- [ ] 10-03-PLAN.md — PLATFORM-NOTES.md ledger (D-04) + the Bazzite manual UAT checklist (D-03)

**Wave 2** — release pipeline (blocked on Wave 1)

- [ ] 10-04-PLAN.md — .goreleaser.yaml + make release/release-snapshot (D-05/D-07/D-08/D-13) + release.yml, retiring ci.yml's old release job (D-06)

**Wave 3** — artifact-shape migration (blocked on Wave 2)

- [ ] 10-05-PLAN.md — scripts/install.sh + e2e/release_e2e_test.go rewritten for the D-07 tar.gz archive shape, GITID_VERSION pin, and custom install dir (D-14/D-15)

**Wave 4** — closing documentation (blocked on Wave 3)

- [ ] 10-06-PLAN.md — README.md refresh via the README-crafting skill (D-16)

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 → 9 → 9.1 → 9.2 → 9.3 → 9.4 → 9.5 → 9.6 → 9.7 → 10

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Foundations, Spikes & CI | 7/7 | Complete | 2026-07-03 |
| 2. DESIGN — All Mockups (★ CHECKPOINT #1) | 14/15 | In Progress|  |
| 3. Create Flow Backend | 19/19 | Complete | 2026-08-24 |
| 4. Git Configuration Screen | 4/4 | Complete    | 2026-08-25 |
| 5. Identity Manager | 9/9 | Complete    | 2026-08-26 |
| 6. Global SSH Options | 7/7 | Complete    | 2026-08-27 |
| 7. Global Git Options | 0/TBD | Not started | - |
| 8. Health + Fixer | 8/8 | Complete    | 2026-08-30 |
| 9. Upload / Credentials Assist | 8/8 | Complete    | 2026-08-30 |
| 9.1. GitLab Real-Account Validation | 2/2 | Complete | 2026-08-31 |
| 9.2. Global Git Ignore Management | 3/3 | Complete | 2026-08-31 |
| 9.3. Release CI/CD + Installer | 2/2 | Complete    | 2026-09-01 |
| 9.4. TUI UX Consistency & Doctor/Fixer Parity | 4/4 | Complete    | 2026-09-03 |
| 9.5. Full SSH/Git Properties Browser | 5/5 | Complete    | 2026-09-04 |
| 9.6. Global Git/SSH Options Consistency Fixes (INSERTED) | 0/6 | Planned | - |
| 9.7. New-Identity Wizard Consistency Fixes (INSERTED) | 0/TBD | Context gathered | - |
| 10. Linux Validation + Release Pipeline | 0/6 | Planned | - |
