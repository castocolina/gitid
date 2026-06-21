---
gsd_state_version: 1.0
milestone: v1.0
milestone_name: Product Features in TUI
status: executing
stopped_at: Completed 05.6-03-PLAN.md
last_updated: "2026-06-21T13:26:49.437Z"
last_activity: 2026-06-21 -- Phase 05.7 execution started
progress:
  total_phases: 10
  completed_phases: 7
  total_plans: 51
  completed_plans: 48
  percent: 70
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-06-08)

**Core value:** Managing a Git identity produces coordinated, coherent SSH + Git artifacts that are proven to authenticate and resolve correctly before any file is written, and existing hand-written config is never corrupted.
**Current focus:** Phase 05.7 — complete-v1-0-product-features-in-tui

## Current Position

Phase: 05.7 (complete-v1-0-product-features-in-tui) — EXECUTING
Plan: 4 of 8
Status: Ready to execute
Last activity: 2026-06-21 -- Phase 05.7 execution started

Progress: [██████████] Phase 04 complete (7/7 plans incl. gap closure)

## Performance Metrics

**Velocity:**

- Total plans completed: 32
- Average duration: — min
- Total execution time: 0 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 1. Bootstrap | 0 | - | - |
| 2. First Identity End-to-End | 0 | - | - |
| 3. Full Identity CRUD + Multi-Identity | 0 | - | - |
| 4. Doctor | 0 | - | - |
| 5. CLI Surface + TUI | 0 | - | - |
| 03 | 4 | - | - |
| 03.1 | 4 | - | - |
| 04 | 7 | - | - |
| 05.5 | 7 | - | - |
| 5.6 | 5 | - | - |

*Updated after each plan completion*
| Phase 02 P01 | 18 | 2 tasks | 5 files |
| Phase 02 P03 | 7min | 2 tasks | 10 files |
| Phase 02 P05 | 6min | 2 tasks | 8 files |
| Phase 02 P04 | 4min | 2 tasks | 7 files |
| Phase 03.1 P01 | 2min | 2 tasks | 2 files |
| Phase 03.1 P02 | 3 | 2 tasks | 2 files |
| Phase 03.1 P04 | 8 | 2 tasks | 5 files |
| Phase 04 P01 | 40 | 3 tasks | 12 files |
| Phase 04-doctor P02 | 50 | 2 tasks | 9 files |
| Phase 04-doctor P03 | 10 | 2 tasks | 8 files |
| Phase 04-doctor P05 | 25 | 2 tasks | 1 files |
| Phase 04-doctor P06 | 90 | 2 tasks | 8 files |
| Phase 04 P07 | 45 | 2 tasks | 5 files |
| Phase 05 P01 | 13 | 2 tasks | 15 files |
| Phase 05 P02 | 18 | 2 tasks | 7 files |
| Phase 05 P03 | 35 | 2 tasks | 7 files |
| Phase 05-cli-surface-tui P04 | 13 min | 2 tasks | 9 files |
| Phase 05.5 P01 | 5 | 2 tasks | 7 files |
| Phase 05.5 P02 | 20 | 2 tasks | 14 files |
| Phase 05.5 P03 | 32 | 2 tasks | 9 files |
| Phase 05.5 P04 | 45 | 2 tasks | 6 files |
| Phase 05.5 P05 | 7 | 2 tasks | 8 files |
| Phase 05.5 P06 | 10 | 2 tasks | 4 files |
| Phase 05.5 P07 | 15 | 1 tasks | 6 files |
| Phase 05.6 P01 | 25 | 3 tasks | 23 files |
| Phase 05.6 P02 | 40 | 2 tasks | 7 files |
| Phase 05.6 P03 | 9 | 2 tasks | 7 files |
| Phase 05.6 P04 | 11 | 2 tasks | 8 files |
| Phase 05.6 P05 | 13 | 2 tasks | 5 files |
| Phase 05.7 P01 | 6 | 3 tasks | 11 files |
| Phase 05.7 P06 | 9 | 2 tasks | 11 files |
| Phase 05.7 P07 | 90 | 2 tasks | 11 files |

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- Init: Tool/binary name = `gitid`; default clone base = `~/git`
- Init: Defer `insteadOf` + `add repo` to v2; keep `includeIf` match strategy in Phase 1
- Init: ed25519 only, one key per identity, auth + signing, no GPG
- Init: charm.land v2 vanity import paths (NOT github.com/charmbracelet/*) — confirmed v2.0.7/v2.0.3/v2.1.0
- Init: Custom gitconfig line parser required for `includeIf` write-back (no Go library supports it)
- [Phase ?]: 02-01: filewriter is the single safe-write chokepoint (backup+atomic+chmod); no os.WriteFile elsewhere
- [Phase ?]: 02-03: RED stubs return zero+sentinel (not panic) to satisfy lint-gated pre-commit hook while failing genuinely
- [Phase ?]: 02-03: clipboard no-tool detection keys on atotto clipboard.Unsupported bool (v0.1.4 has no exported sentinel error)
- [Phase ?]: 02-05: RenderIncludeIf returns full sentinel-wrapped block; WriteIncludeIf renders body-only for ReplaceBlock to avoid double-wrap
- [Phase ?]: 02-05: tester unexported runner seam + preWriteWith unit-tests 3-way classifier with fixtures (no live SSH)
- [Phase ?]: 02-04: macOS Host * emits IgnoreUnknown UseKeychain first (Linux ssh -G safe); _global block ordered last via separate sentinel key
- [Phase ?]: 02-04: sshconfig render/parse/write commit RED+GREEN combined — lint-gated hook rejects signature-bearing zero-value stubs; RED proven via local test runs
- [Phase 3]: 03-01: ListBlocks/RemoveBlock mirror ReplaceBlock splice; RemoveBlock consumes one trailing blank line (Pitfall B anti-accumulation)
- [Phase 3]: 03-01: ReadFragment via git config --file --list (arg-slice G204-clean, Pitfall E: literal signingkey path)
- [Phase 3]: 03-01: Reconstruct joins SSH+gitconfig by sentinel identity name (D-01); Incomplete csv marks missing artifacts (D-02)
- [Phase 3]: 03-01: RemoveAllowedSignersLine requires BOTH email AND namespaces="git" match (T-03-01/Pitfall D)
- [Phase 3]: 03-04: Delete passes ONLY acct.Name to RemoveBlock — never "_global"; keepKey=!confirm (D-07/D-08)
- [Phase ?]: 03.1-02: validateValue panics used in renderers — matches renderBlockBody precedent; newline in render input is programming error not user data
- [Phase ?]: 03.1-02: WriteBaselineInclude hardcodes literal ~ path per RESEARCH Q2 (git expands ~ at runtime)
- [Phase ?]: 03.1-03: ScanConflicts block-stripped algorithm (RESEARCH C2)
- [Phase ?]: 03.1-03: ReadBaselineState sidecar-free ListBlocks across three files (SC-5/IDENT-07); BaselineKeySet = authoritative Tier-1 source
- [Phase ?]: 03.1-04: GitVersionAtLeast seam in internal/deps for zdiff3 gate; idempotency skip in baseline writers (bytes.Equal before Write)
- [Phase ?]: InstallHint(tool,os) signature; doctor.Deps DetectTools+ReadBaselineState seams; Baseline Fix descriptors set for Plan 05 wiring
- [Phase ?]: CheckCoherence byte-exact email check (Pitfall 6); Incomplete→Coherence not Orphans (D-09/Pitfall 5); Orphan class 1/2 cross-ref SSH vs gitconfig block names; class 3 key cross-ref AllSSHHostIdentityFiles (D-12)
- [Phase ?]: 04-05: D-07 pre-fix exit code captured before applyFixes, returned unconditionally (WARNING 5)
- [Phase ?]: 04-05: AddWiring dispatcher uses line-prefix encoding (ssh-host:/signers:/baseline-include:) — no new internal/sshconfig or internal/gitconfig function (BLOCKER 2 resolved)
- [Phase ?]: 04-05: applyFixes injectable *bufio.Reader enables gate/confirm/batching tests without real stdin; FamilyPerms batched, others individual (D-04)
- [Phase ?]: 04-06: incompleteNames guard removed from orphan Classes 1+2 — single-sided managed block is both Incomplete (Coherence) and Orphan; guard was preventing any orphan finding from triggering
- [Phase ?]: 04-06: WR-01 findSignerLine all-candidate scan: exact match anywhere wins over earlier case-fold match; WR-02: RemoveBlock mode derived from path (0644 for allowed_signers, 0600 for config files)
- [Phase ?]: DOC-GAP-02: RunSSHAdd+RunSSHKeygenFingerprint wired in buildDoctorDeps
- [Phase ?]: DOC-GAP-03: isTerminalInput TTY guard gates applyFixes; non-interactive doctor skips Apply prompt
- [Phase ?]: IN-03: doctorExitCode pkg-level var bridges RunE tiered code to main os.Exit
- [Phase ?]: WR-03: checkGitconfigPath warns only on group/world-write bits (0o022 mask); default 0644 gitconfig not flagged
- [Phase ?]: 05-01: bubbletea v2 alt-screen via View.AltScreen=true (not WithAltScreen — v1 only)
- [Phase ?]: 05-01: doctor-deps wiring duplicated in tui/deps.go (not extracted to internal/) per RESEARCH assumption A3
- [Phase ?]: 05-03: dashboardModel seeds rootModel stack; Init() returns tea.Batch of 7 per-family doctor cmds (D-09 async streaming)
- [Phase ?]: 05-03: identity.Reconstruct via ReadFile seam in newIdentityListScreen; fallback to d.Identities for test mode
- [Phase ?]: 05-03: newCreateFormScreen/newIdentityDetailScreen are stable-name placeholder stubs for 05-04
- [Phase ?]: proveModel confirm gate: T-05-15 - open only after both SSH phases pass
- [Phase ?]: Write path via identity.Create(deps): no direct filewriter in tui/prove.go (T-05-14)
- [Phase ?]: Shift+Tab in bubbletea v2: msg.String() == shift+tab checked before msg.Code == tea.KeyTab
- [Phase ?]: 05.5-01: A4 proved — kevinburke/ssh_config preserves # gitid: provider= comment inside Host stanza across parse→String()→parse; provider marker design safe for Plan 02
- [Phase ?]: 05.5-01: E2E harness uses fake-ssh via os.WriteFile 0o700 static literal + PATH prepend via cmd.Env (D-20); never exec.Command(sh,-c)
- [Phase ?]: 05.5-02: marker-prefer+hostname-map replaces TrimPrefix(ssh.Alias) for provider derivation (F-3 fix); RenderHostBlock panics on newline in provider (T-05.5-04)
- [Phase ?]: 05.5-03: PersistAll/RenderPreviews factored out; Confirmed removed; persist gate is auth-loop (D-03/D-05/D-06)
- [Phase ?]: 05.5-03: buildDeps Generate writes key to final ~/.ssh path immediately (D-01); TempPrivatePath==FinalPrivatePath; Cleanup unconditional no-op
- [Phase ?]: 05.5-03: runCreateLoop in cmd/gitid: PASS auto-persists (D-03); [s]+confirm persists with warning (D-05); [q] keeps key no config (D-04); ReachableNotUploaded never auto-persists (D-06)
- [Phase ?]: 05.5-04: strategyNumFromKind(matchKinds(existing.Matches)) drives update picker pre-fill; prevents Pitfall 6 (hasconfig → gitdir collapse on update)
- [Phase ?]: 05.5-04: buildMatches stores remote.*.url: prefix internally; callers pass bare URL pattern; conditionToMatch round-trip proved via TestHasconfigRoundTrip (T-05.5-14)
- [Phase ?]: 05.5-04: promptMatchStrategy variadic strategyDefault; add passes nothing (default "1"), update passes strategyNumFromKind result
- [Phase ?]: 05.5-04: addFlags{name,gitdir,url,provider} on add; updateFlags{gitdir,url,provider} on update (NO --name per Q2); provider flag-only on update per Q3
- [Phase ?]: 05.5-06: BinaryInstallInfo uses os.Executable + EvalSymlinks (tolerates evalErr A1) + pure binaryOnPath helper; SeverityInfo finding under FamilyDeps with export hint when off-PATH (D-17)
- [Phase ?]: true
- [Phase ?]: 05.6-03: Finding.IdentityName additive field for badge scoping (D-08); zero value = global finding; all existing struct literals unaffected
- [Phase ?]: 05.6-03: healthViewModel sub-model (port from dashboardModel); fixConfirmModal = fixConfirm Enter-gated; badgesFromFindings via IdentityName field; healthReady flag seeds init() only on first view-2 press
- [Phase ?]: 05.7-01 D-01: TestBuildTUIDepsNilGuard_Phase57 uses zero-value Deps form — buildTUIDeps still returns 5 values in Plan 01; Plan 06 rewires to 8-value return
- [Phase ?]: 05.7-01 D-02: adopter.Deps.WriteIncludeIf is a 3-arg seam (id, fragPath, matches) with gitconfigPath captured by live closure — not a 4-arg pass-through
- [Phase ?]: WriteIncludeIf 3-arg closure captures gitconfigPath in buildAdoptDeps; adopter seam kept narrow (Plan 06)
- [Phase ?]: TestBuildTUIDepsNilGuard_Phase57 drives live buildTUIDeps() 8-value return — D-16 anti-blindspot for adopter/repoclone/uploader seams (Plan 06)
- [Phase ?]: copy --upload-keys is non-blocking by design (D-11): AuthToolNotFound is silent-skip; upload never gates the key copy (Plan 06)
- [Phase ?]: Unified commit for both tasks: CLAUDE.md buildable-boundary rule
- [Phase ?]: adoptPhaseDone before adoptPhaseOfferRemove: UX + test contract

### Roadmap Evolution

- Phase 3.1 (Baseline Global Git Config + Global Gitignore) inserted after Phase 3, before Doctor — scope correction: GLOBAL-01 and URLRW-01 promoted v2→v1, new GITIGNORE-01 added (45/45 v1 coverage). Canonical refs: samples/gist-60f2f1d-gitconfig, samples/gist-2c98cff-ssh-config.
- Phase 5.7 (Complete v1.0 Product Features in TUI) inserted after Phase 5.6, before Linux validation (URGENT — 2026-06-21). Closes the gap between the shipped identity-lifecycle MVP and the whole-product v1.0 PRD (`docs/prds/ssh-git-identity-manager-v1.0-prd.md`): global-config toggles, `insteadOf` rewriting, fragment adoption, alt-SSH 443, shell completions, `add repo` workflow. Picks up dangling REQ-IDs ADOPT-01/REPO-01/AUTOUP-01. `recipes/` remains the canonical end state.

### Pending Todos

None yet.

### Blockers/Concerns

None yet.

### Quick Tasks Completed

| # | Description | Date | Commit | Directory |
|---|-------------|------|--------|-----------|
| 260609-qd6 | Fix security gap T-02-23: apply identityNameRe charset validation to add.go create-new and add-account name inputs | 2026-06-09 | 6711bb1 | [260609-qd6-fix-security-gap-t-02-23-apply-identityn](./quick/260609-qd6-fix-security-gap-t-02-23-apply-identityn/) |
| 260609-s0m | Fix create-new pre-write gate (E2E bugs 1-3): dial real hostname+port with accept-new, not the unwritten SSH alias | 2026-06-09 | cb88a10 | [260609-s0m-fix-create-new-pre-write-connectivity-ga](./quick/260609-s0m-fix-create-new-pre-write-connectivity-ga/) |
| 260609-s8j | Fix WriteFragment: ensure parent ~/.gitconfig.d dir exists before git config (E2E bug 5) | 2026-06-09 | 5532352 | [260609-s8j-fix-writefragment-ensure-parent-gitconfi](./quick/260609-s8j-fix-writefragment-ensure-parent-gitconfi/) |
| 260610-a54 | Fix BUG-4 (temp-then-promote): key staged to temp, gated, persisted to ~/.ssh only after gate-pass + confirm; dry-run/abort leave ~/.ssh untouched | 2026-06-10 | f085e5d | [260610-a54-fix-bug-4-temp-then-promote-generate-the](./quick/260610-a54-fix-bug-4-temp-then-promote-generate-the/) |
| 260612-dc7 | Fix doctor perms widening bug (Phase-4 code-review Important): checkPath tighten-only (`got&^want` guard + `got&want` fix mode) — a 0400 key no longer false-flagged/loosened; T-04-02/T-04-19 evidence updated | 2026-06-12 | 34f15c2 | [260612-dc7-fix-doctor-perms-widening-bug-code-revie](./quick/260612-dc7-fix-doctor-perms-widening-bug-code-revie/) |
| 260612-dtm | Add `depguard` D-01 gate to .golangci.yml (denies internal/filewriter under internal/doctor/**) — automates the write-free-core invariant, fire-tested; closes Phase-4 SECURITY WARNING-01 (T-04-03/T-04-21 CLOSED) | 2026-06-12 | c9924bd | [260612-dtm-add-a-depguard-rule-to-golangci-yml-deny](./quick/260612-dtm-add-a-depguard-rule-to-golangci-yml-deny/) |

## Session Continuity

Last session: 2026-06-21T13:26:45.711Z
Stopped at: Completed 05.6-03-PLAN.md
Resume file: None
