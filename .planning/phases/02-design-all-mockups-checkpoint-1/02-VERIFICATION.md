---
phase: 02-design-all-mockups-checkpoint-1
verified: 2026-07-06T00:00:00Z
status: passed
score: 4/4 must-haves verified
overrides_applied: 0
re_verification: false
warnings:
  - id: W1
    concern: "insteadOf URL rewriting (recipes/ core wiring #3) is not rendered on any live-demo screen"
    detail: >
      recipeFixtures.ts defines insteadOfBlockText (provider-level, recipe-correct
      shape) but no interactive web-demo screen and no dummytui screen renders it;
      GlobalGitOptions (data.go:509-531 / globalGitOptions in recipeFixtures.ts)
      has no url.insteadOf row. APPROVAL.md checklist C's sub-claim "insteadOf URL
      rewrite ... all visible in the relevant previews" is not true of the current
      live demos (it is a static-set-era carry-over). Not a Phase 2 SC failure
      (SCs do not enumerate insteadOf; the user approved the live demos as
      presented), but neither Phase 4 nor Phase 7 success criteria explicitly
      cover insteadOf — route design coverage or a documented divergence into the
      Phase 4/7 planning.
  - id: W2
    concern: "the committed no-backend import-allowlist test no longer exists"
    detail: >
      internal/dummytui/nobackend_test.go was deleted in 7453561 (static-set
      removal) and not recreated by the 02-13 rebuild. APPROVAL.md still cites it
      as the "runtime-checked complement". The no-backend truth itself is VERIFIED
      directly in this pass (go list -deps: only first-party deps are
      internal/dummytui + cmd/gitid-dummy) and `make gate-no-backend-files` is
      green, but enforcement is now a manual go-list check + a branch-scoped file
      gate, not a committed test. Consider restoring an import-graph test before
      Phase 3 adds backend wiring.
  - id: W3
    concern: "full `make test-e2e` not re-run in this verification session"
    detail: >
      The dummy-demo PTY subset (TestDummyDemo_LiveWalk / MouseAndGitApply /
      ShiftChordRawBytes, includes the DLV-05 zero-writes sandbox assertion) was
      re-run live in this pass and is green (ok, 14.064s). The full suite's prior
      green evidence stands at commits d6438bd / 3c3130e (02-14-SUMMARY records
      the forced 100x30 PTY re-run).
---

# Phase 2: DESIGN — All Mockups (★ CHECKPOINT #1) Verification Report

**Phase Goal:** Every product surface is designed as an HTML/mui mockup presented
as an interactive web demo, mirrored by a live, executable Go TUI demo, and
approved by the user — establishing the reference design the whole build is
verified against.
**Verified:** 2026-07-06
**Status:** passed (4/4 truths verified; 3 warnings, none goal-blocking)
**Re-verification:** No — initial verification
**Verifier stance:** goal-backward; SUMMARY claims were checked against the
codebase, real commands were run, and stale claims are flagged below.

Mandatory pre-read done: `recipes/README.md`, `recipes/ssh-config.recipe`,
`recipes/gitconfig.recipe`.

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Every surface (create flow, git screen, identity manager, global SSH, global git, health, fixer) has an HTML/mui mockup presented as a live interactive web demo, static reference routes kept alongside (DLV-01, DLV-02) | ✓ VERIFIED | `.planning/design/mockup-src/` — MUI v7 (`@mui/material` 7.3.11, exact-pinned), 52 `*.route.tsx` modules: 50 static per-surface reference routes across all 7 surface dirs + `_shell/shell-demo` + `demo/interactive` at path `/` rendering `DemoApp` (4 tabs: Identities incl. 4-step create wizard + Git step, Global SSH, Global Git, Doctor absorbing the Fixer). `pnpm typecheck` (tsc --noEmit) clean; `pnpm build` succeeds. All 7 surfaces have `FIELDS.md` + `CRITIQUE.md`. `make demo-web` target exists (Makefile:29/58). |
| 2 | A LIVE, executable Go TUI demo (dummy data, in-memory state, NO backend logic) provides full navigation and every interactive flow, mirroring the web demo 1:1 (DLV-05) | ✓ VERIFIED | `cmd/gitid-dummy` + `internal/dummytui` (app/frame/identities/globalssh/globalgit/doctor/ceremony/store/theme/data, 164 test funcs). Ran: `go test -race -count=1 ./internal/dummytui/... ./cmd/gitid-dummy/...` → both `ok`. Backend-free proven directly: `go list -deps ./internal/dummytui ./cmd/gitid-dummy` → only first-party packages are `internal/dummytui` + `cmd/gitid-dummy` (no keygen/sshconfig/gitconfig/identity/doctor imports). PTY e2e re-run live: `go test -tags e2e -run TestDummyDemo ./e2e/` → `ok 14.064s`, including the DLV-05 zero-writes sandboxed-HOME walk (dummy_demo_e2e_test.go:287-302). Mirroring contract: `theme.go` ↔ `theme.ts` role-by-role (02-STYLE-SPEC.md, 12 roles), `data.go` ↔ `recipeFixtures.ts` byte-mirrored copy pinned by tests. |
| 3 | agent-ui-ux-designer critiqued the HTML↔TUI diff; findings resolved before approval (DLV-02) | ✓ VERIFIED | Per-surface `CRITIQUE.md` × 7 — each records "0 open findings" (parity.json rows all resolved; SECURITY.md T-02-PAR CLOSED). Per-surface passes were executor-applied designer methodology (transparently flagged in each file), then the phase-level orchestrator ran a FRESH agent-ui-ux-designer parity critique + fresh-context code review (02-14-SUMMARY §"Review findings resolution", F1–F11) — every claimed fix commit verified to exist: c2a329b (TUI batch), 04a00b8 (web batch), 50f890c (docs batch), plus checkpoint-feedback commits dd6d4c2/2116bfe, plus the 02-13 three-reviewer convergence round (489f422). All fixes predate the approval commit 3c3130e. |
| 4 | ★ User approved the complete design; approval recorded; no backend logic written for any surface before it (DLV-08, DLV-05) | ✓ VERIFIED | `.planning/design/APPROVAL.md:205` — `**APPROVED:** 2026-07-06 by Pepe` (matches required regex), Status APPROVED, checklist §A–F + E2/E3 fully ticked. 02-12-SUMMARY records the approver string as user-supplied (never inferred). ROADMAP.md:43 marks the phase complete with the same sign-off. No-backend-before-approval: `make gate-no-backend-files` → OK (no files outside the design allowlist changed since main @321884c); `git diff --name-only main...HEAD` on code paths touches only `internal/dummytui/`, `cmd/gitid-dummy/`, `internal/screenshot/`, `Makefile`, and e2e test harness files — zero backend-package modifications. |

**Score:** 4/4 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `.planning/design/APPROVAL.md` | Sign-off line, Status APPROVED, ticked checklist | ✓ VERIFIED | Line 205 matches `\*\*APPROVED:\*\* [0-9]{4}-[0-9]{2}-[0-9]{2} by .+`; recorded by commit 3c3130e |
| 15 plan SUMMARYs 02-01..02-15 | All present | ✓ VERIFIED | All 15 `02-NN-SUMMARY.md` present in the phase dir |
| `REVIEW.md` | Phase code review | ✓ VERIFIED (with note) | `status: issues_found` (1 HIGH, 1 MEDIUM, 2 LOW; static-set era, 2026-07-03); all findings fixed in `02-REVIEW-FIXES.md` (finding → fix → proof format); superseded by the fresh 02-14 F1–F11 review pass on the live demos |
| `SECURITY.md` | Security audit | ✓ VERIFIED | Threat register entries CLOSED (T-02-SC deps exact-pinned, T-02-PAR parity, etc.) |
| `02-REDESIGN-SPEC.md`, `02-STYLE-SPEC.md`, `02-DESIGN-DECISIONS-CHECKPOINT-2.md` | Design contracts | ✓ VERIFIED | Present; 02-STYLE-SPEC carries the 12-role table and §"Conscious divergences from recipes/" documenting D9 (editable global-fallback user.email) |
| `.planning/design/mockup-src/` | MUI v7 web demo SPA | ✓ VERIFIED | Typechecks and builds; route auto-discovery validates path/title uniqueness at build time |
| Per-surface `FIELDS.md` + `CRITIQUE.md` × 7 | All surfaces | ✓ VERIFIED | create-flow, git-screen, identity-manager, global-ssh, global-git, health, fixer |
| `cmd/gitid-dummy` + `internal/dummytui` | Live TUI demo | ✓ VERIFIED | Substantive (≈11.4k insertions on branch), tested, wired (see key links) |
| `internal/dummytui/nobackend_test.go` | Claimed no-backend allowlist test | ✗ MISSING (W2) | Deleted in 7453561, never restored; the no-backend TRUTH holds via direct `go list -deps` + `gate-no-backend-files`, but APPROVAL.md's reference to this file is stale |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `cmd/gitid-dummy/main.go` | `internal/dummytui` | import + `NewApp` | ✓ WIRED | Binary runs the demo; PTY e2e walks the real binary |
| `internal/dummytui` screens | `internal/dummytui/data.go` | seeded fixtures → reducer store | ✓ WIRED | Screens render recipe-faithful seed data; store.go reducer mutates in-memory state (BY DESIGN dummy — DLV-05, not a stub) |
| `internal/dummytui` | backend packages (keygen/sshconfig/gitconfig/...) | imports | ✓ ABSENT (required) | `go list -deps`: zero backend imports — the DLV-05 contract |
| Web demo `/` index | `DemoApp` (all 7 surfaces) | `routes/demo/interactive.route.tsx` | ✓ WIRED | path `/`, 4 tabs + Ctrl+P palette opening the 50 static reference routes |
| `theme.go` (Go Theme) | `theme.ts` (web roles) | 02-STYLE-SPEC role table | ✓ WIRED | 12 named roles mirrored; SGR pins in theme_test.go |
| `data.go` | `recipeFixtures.ts` | byte-mirrored fixture copy | ✓ WIRED | Pinned by tests (e.g. `TestFixtureConsistency`: allowed_signers email byte-matches user.email, GITUI-04) |
| Demos | `recipes/` canonical shape | seeded previews | ✓ WIRED (1 gap → W1) | See recipe-fidelity table below |

### Recipe Fidelity (recipes/ North Star)

| Recipe wiring | Web demo | TUI demo | Status |
|---------------|----------|----------|--------|
| Alias `Host <identity>.<provider>` + `Hostname ssh.github.com` + `Port 443` + `User git` + `IdentityFile` + `IdentitiesOnly yes` | recipeFixtures.ts:37-41 | data.go:67-72 (`CreateFlowSSHHostBlock`) | ✓ |
| `includeIf hasconfig:remote.*.url:git@<alias>:*/**` and `gitdir:` alternative, default `gitdir`, `both` combinable, live preview | recipeFixtures.ts:61-71, 341-358 | data.go:205-254 (strategy preview map) | ✓ (gitdir default is the recorded Phase-5.7-resolved divergence) |
| `insteadOf` URL rewriting | fixture constant only (`insteadOfBlockText`, unused); `[url]` sections mentioned only as "preserved verbatim" copy on 2 static global-git routes | absent | ⚠ W1 — not rendered in either live demo; APPROVAL.md checklist C sub-claim stale |
| Per-identity fragment: `user.name/email`, `gpg.format=ssh`, `signingkey`, `commit.gpgsign` | recipeFixtures.ts:98-113 | data.go:174-191 | ✓ (ed25519/ssh-signing supersedes gists' RSA/GPG, per recipes/README caveat) |
| `allowed_signers` email byte-identical to `user.email` | recipeFixtures.ts:116+ | data.go:221-223; pinned by data_test.go:13 | ✓ |
| Recipe defaults (`push.autoSetupRemote`, `pull.rebase`, `fetch.prune`, aliases, color, `merge.conflictstyle`, `diff.colorMoved`, `init.defaultBranch=main`, `core.ignorecase=false`) | globalGitOptions | data.go:509-531 | ✓ |
| D9 documented divergence: editable global-fallback `user.email` | GlobalGit row + helper copy | data.go:513-523 | ✓ documented in 02-STYLE-SPEC §"Conscious divergences from recipes/" |

### Behavioral Spot-Checks (real commands, real output)

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| TUI demo unit/model tests (race) | `go test -race -count=1 ./internal/dummytui/... ./cmd/gitid-dummy/...` | `ok internal/dummytui 6.472s`, `ok cmd/gitid-dummy 1.567s` | ✓ PASS |
| Lint | `make lint` | `0 issues.` | ✓ PASS |
| Design-only branch gate | `make gate-no-backend-files` | `OK -- no files outside {...} changed since main (321884c)` | ✓ PASS |
| Backend-free import graph | `go list -deps ./internal/dummytui ./cmd/gitid-dummy \| grep gitid` | only `cmd/gitid-dummy` + `internal/dummytui` | ✓ PASS |
| Live PTY walk + zero-writes sandbox (DLV-05) | `go test -count=1 -tags e2e -run TestDummyDemo ./e2e/` | `ok github.com/castocolina/gitid/e2e 14.064s` | ✓ PASS |
| Web demo typecheck + build | `pnpm typecheck && pnpm build` | tsc clean; build succeeds (chunk-size warning only) | ✓ PASS |
| Full `make test-e2e` | not re-run this session | prior green at d6438bd/3c3130e (02-14-SUMMARY forced re-run) | ? SKIP (W3; dummy-demo subset re-run live above) |
| Claimed fix commits exist | `git log -1` on c2a329b, 04a00b8, 50f890c, dd6d4c2, 2116bfe, d6438bd, 3c3130e | all present with matching messages | ✓ PASS |

### Requirements Coverage

| Requirement | Description | Status | Evidence |
|-------------|-------------|--------|----------|
| DLV-01 | HTML mockup before any Go/TUI code per surface | ✓ SATISFIED | 50 static routes + FIELDS.md freeze per surface; mockups (02-01..02-10) preceded the live TUI rebuild (02-13) |
| DLV-02 | agent-ui-ux-designer + /mui engaged plan/build/review | ✓ SATISFIED | /mui skill build (mockup-src README/APPROVAL); per-surface critiques + fresh phase-level designer parity critique (F1–F11) resolved pre-approval |
| DLV-05 | Fixed order: mockup → dummy (no backend) → approval → backend | ✓ SATISFIED | Dummy proven backend-free (import graph) + write-free (PTY zero-writes); no backend package modified on the branch; approval recorded before any Phase 3 work |
| DLV-08 | Single human checkpoint: design approval | ✓ SATISFIED | `**APPROVED:** 2026-07-06 by Pepe` in APPROVAL.md, user-supplied approver, commit 3c3130e |

No orphaned Phase-2 requirements found in REQUIREMENTS.md.

### Anti-Patterns Found

| File | Pattern | Severity | Impact |
|------|---------|----------|--------|
| — | `TBD/FIXME/XXX/TODO/HACK/PLACEHOLDER` scan over `internal/dummytui/`, `cmd/gitid-dummy/`, `mockup-src/src/` | none found | — |
| `.planning/design/APPROVAL.md` | Stale claims: cites deleted `internal/dummytui/nobackend_test.go`; checklist C asserts insteadOf "visible in the relevant previews" | ℹ Info (W1/W2) | Auditability of the approval record; the underlying truths were re-proven directly in this pass |

Note: hardcoded in-memory dummy data throughout `internal/dummytui` and
`mockup-src/src/demo` is the DLV-05 requirement, not a stub pattern.

### Human Verification Required

None new. The phase's defining human check — "the two live demos read as one
product and the design is acceptable" — was performed by the user at the 02-12
checkpoint itself across a full feedback loop (02-14 F1–F11 + U1–U3, 02-15
route-back) and is recorded as the DLV-08 sign-off. That recorded human approval
IS success criterion 4; re-requesting it would be circular.

### Gaps Summary

No goal-blocking gaps. Three warnings for the record:

1. **W1 — insteadOf not in the approved live design.** recipes/ wiring #3 (URL
   rewriting) exists only as an unused fixture constant; no live screen in either
   medium shows it, no Global Git option row manages it, and no later phase's
   success criteria explicitly claim it. Recommend: add it to Phase 4 or Phase 7
   design scope (or record a conscious divergence next to D9).
2. **W2 — no committed import-allowlist test.** The no-backend property is
   currently enforced by a branch-scoped file gate + manual `go list` checks;
   restore a committed import-graph test before Phase 3 introduces backend
   packages next to the dummy.
3. **W3 — full `make test-e2e` skipped this session** (time budget); the
   dummy-demo PTY subset incl. the zero-writes assertion was re-run live and is
   green; full-suite prior evidence at d6438bd/3c3130e.

---

_Verified: 2026-07-06_
_Verifier: Claude (gsd-verifier)_
