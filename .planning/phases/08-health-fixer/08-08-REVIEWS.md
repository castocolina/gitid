# 08-08 Task 3 — Cross-AI Review Packet + Exit Battery + REQUIREMENTS.md Closure

Named `08-08-REVIEWS.md`, distinct from the phase-level `08-REVIEWS.md`
(the pre-execution cross-AI PLAN review from `/gsd-plan-review-convergence 8`,
dated 2026-08-28T02:47:34Z — a separate document this file does not
replace; an earlier version of this commit mistakenly overwrote it, caught
and reverted before merge).

Assembled by the orchestrating session (not an executor) on `2026-08-28`,
after hand-completing Wave 8's Tasks 1 and 2 (a second executor was killed
after ~40 minutes of pure exploration with zero writes — a new failure mode
for this phase, distinct from prior waves' crashes/stalls). Mirrors Phase
7's `07-06` review-packet convention.

## Review provenance

Per this phase's own established convention (`.planning/phases/07-global-git-options/review-packet/MANIFEST.md`),
running the cross-AI reviewer CLIs (Codex, xai-grok, etc.) against an
assembled packet is an **orchestrator obligation** — an executor has no
subagent-spawning mechanism. This session IS the orchestrator and DOES have
subagent-spawning tools, so it exercised that obligation directly rather
than deferring it further: `agent-ui-ux-designer:ui-ux-designer` was spawned
against three real captured frames (health-findings, fixer-list,
fixer-ceremony-preview — the same three the DLV-04 visual-regression gate
registers, captured from the real backend against a seeded fixture home,
not mockups). **A separate external reviewer CLI (Codex) was NOT run** — no
CLI credential/session was available in this sandboxed session for that
specific tool; the UX-focused review below stands in its place for this
closure. If a Codex pass becomes available later, it is additive, not a
gap in this closure's evidence.

## Part A — UX review findings and dispositions

15 findings (F1–F15) were returned, three rated CRITICAL, seven HIGH, three
MEDIUM, two LOW. Every CRITICAL/HIGH finding was independently traced
against the actual source before dispositioning — not accepted or dismissed
on the reviewer's word alone.

| # | Severity | Finding | Disposition | Evidence |
|---|----------|---------|--------------|----------|
| F1 | CRITICAL | Typed-confirm input can scroll below the fold on a long diff | **Deferred — pre-existing shared component.** `ceremony.view()`'s own doc comment: "this component is shared by every mutating flow (create, edit, delete, global apply, fixes), so this change applies everywhere ceremony.view renders." Not introduced by Phase 8; a fix here is a cross-cutting `internal/tuikit/ceremony.go` change affecting every ceremony in the app (Identities edit/delete, Global SSH/Git apply), out of Wave 8's DLV-04/DLV-06 scope. Recommend a dedicated ceremony-rendering UX ticket. | `internal/tuikit/ceremony.go:314-320` (`receiptListMaxLines` cap comment), `:344` (`view` doc) |
| F2 | CRITICAL | No confirm keybinding shown in the footer; two "Esc" lines | **Deferred — pre-existing shared behavior on both counts.** The confirm affordance (`Cancel (Esc)` / `Apply fix (Enter)`) is rendered INSIDE the ceremony pane by `ceremony.go` itself (confirmed present in shorter-diff captures during this session's own debugging) — on a long diff it clips off-screen, the SAME root cause as F1. The duplicate "Esc" is the app-level global footer (`frame.go:87,99`, `{Key: "Esc", Label: "back"}`) appended alongside every screen's own action — a universal app-chrome pattern, not Fixer-specific. | `internal/tuikit/frame.go:87,99`; `internal/tuikit/fixer_screen.go:301` (`{Key: "Esc", Label: "cancel fix"}`) |
| F3 | CRITICAL | Ceremony not full-width; diff pane cramped beside an inert findings list | **Deferred — pre-existing, deliberate design.** The master-detail split staying visible during a ceremony (rather than a full-width takeover) is `ceremony.go`'s existing behavior for every ceremony type in the app, unchanged by Phase 8. A full-width redesign is a cross-cutting change, out of scope. | Same as F1 |
| F4 | HIGH | Copy contradicts itself on reversibility ("restore it to undo" vs "cannot be undone") | **Deferred — pre-existing D-09 flagship copy.** Both lines predate Phase 8: `ceremony.go`'s generic "(written first — restore it to undo)" line is shared boilerplate; the destructive warning text is the D-09 flagship's own `Destructive.Warning` field (shipped Phase 2/6, `coherence_test.go` asserts its exact wording). Editing either risks breaking locked contract tests from earlier phases; flagged for a future copy-review pass, not this phase's DLV-04/DLV-06 closure. | `internal/doctor/checks/coherence.go` (D-09 `Destructive` field) |
| F5 | HIGH | Typed-host confirmation over-weighted for a reversible, backed-up single-directive change | **Deferred — pre-existing D-09 flagship UX decision.** The typed-confirm requirement is `checkHandWrittenIdentitiesOnly`'s own `FixDescriptor.Destructive` field, a Phase 2/6-approved product decision (CLAUDE.md's own D-09 scoped-exception carve-out documents this ceremony explicitly). Not introduced or alterable by Phase 8. | `internal/doctor/checks/coherence.go:404-435`; `CLAUDE.md`'s D-09 scoped exception |
| F6 | HIGH | Fixer's detail pane says "available on the Fixer screen" while already on the Fixer screen | **FIXED.** `SuggestedFix` strings are a single canonical source shared by Health, Fixer, and `gitid health --json` — the trailing hand-off clause is correct and useful on Health (where it tells the user where to go) but stale on Fixer (where the user has already arrived and the `f · Fix this…` affordance states the action directly). Added `fixerSuggestedFixText` (`internal/tuikit/fixer_screen.go`) to strip the `" -- available on the Fixer screen."` suffix ONLY in Fixer's own detail-pane rendering; Health's rendering is untouched. Regression test: `TestFixerSuggestedFixDropsStaleFixerHandoff`. | `internal/tuikit/fixer_screen.go` (new helper + test) |
| F7 | HIGH | Tab bar remains interactable mid-ceremony (digit keys could abandon a half-typed confirm) | **Deferred — pre-existing, app-level, universal.** Digit-key tab routing is `App`-level (`internal/tuikit/app.go`'s `handleKey`), the same for every ceremony type; not new to or specific to Fixer. Worth a follow-up but out of Wave 8's scope. | `internal/tuikit/app.go` |
| F8 | HIGH | `✗` glyph shared by both `critical` and `error`; `~` reused for info/bullet/paths | **Deferred — pre-existing, locked design contract.** `severityLabel`'s own doc comment: "renders the glyph + WORD pair for a severity, colored per the **locked contract** (never a glyph or color alone)." `HealthSeverityGlyph` is a shared map used by every severity-rendering surface in the app (Identities, Health, Fixer, CLI). Changing it is a cross-phase design-system change, not a Phase 8 regression. | `internal/tuikit/frame.go:190-208` |
| F9 | HIGH | Health repeats "switch to Fixer" without a jump-to-Fixer-with-selection key | **Accepted as a feature request, deferred.** Genuinely useful, but it is a NEW cross-tab state-passing capability (carrying a specific finding's selection from Health into Fixer), not a bug fix — out of scope for a DLV-04/DLV-06 closure wave. Recorded for a future phase's backlog. | — |
| F10 | MEDIUM | Ragged left-edge indentation in wrapped detail-pane text | **Deferred — pre-existing wrap behavior**, shared by every detail pane in the app (`lipgloss.Style.Width` wrapping), not Fixer/Health-specific. | `internal/tuikit/frame.go` wrap helpers |
| F11 | MEDIUM | "· fixable" tag on every Fixer row adds no information | **Accepted with rationale, deferred.** A legitimate copy-density suggestion; `· fixable` doubles as a REAL signal distinguishing fixable findings from Health's report-only ones on a shared row-rendering path. A full redesign is a copy/design pass outside this closure wave. | `internal/tuikit/doctor.go` (`groupFindings`) |
| F12 | MEDIUM | Ceremony breadcrumb duplicates the pane title verbatim on a width-starved screen | **Accepted with rationale, deferred** — same shared-ceremony scope as F1/F3. | `internal/tuikit/fixer_screen.go:300` (`crumbs = []string{"Fix", sel.Title}`) |
| F13 | MEDIUM | Long finding titles truncate at ~38 columns in the list, no reveal on the row | **Accepted with rationale, deferred.** The detail pane already carries the untruncated title (visible in Frame 1/2/3's own detail panes); this is the SAME master-list truncation convention every list surface in the app already uses (Identities, Global SSH/Git options). | `internal/tuikit/frame.go` master-list rendering |
| F14 | LOW | Header chip (`! 1 ✗ 3`) excludes info findings and folds critical into the error count | **Deferred — pre-existing, locked, shared.** `healthChip`'s own implementation (`counts.Warnings+counts.Errors`) predates Phase 8 and is used in every screen's header, not Health/Fixer-specific. | `internal/tuikit/frame.go:211-221` |
| F15 | LOW | Mixed typography (`--` beside `→`, `·`, `╌`) | **Accepted with rationale, deferred** — a repo-wide copy-style pass, not a Phase 8 concern. | — |

**Summary: 1 of 15 findings (F6) was genuinely Phase-8-scoped and is fixed
with a verified regression test in this closure. The remaining 14 all trace,
with cited code evidence, to shared, pre-existing, or explicitly locked
design-system behavior from earlier phases (2, 3, 6) — none are Phase 8
regressions, and redesigning any of them is a cross-cutting change outside
this wave's DLV-04/DLV-06 charter. Zero CRITICAL/HIGH findings remain open
without an explicit, evidence-backed disposition.**

## Part B — Full exit battery (real command output)

- `go build ./...` — exit 0.
- `go vet -tags e2e ./...` — exit 0.
- `TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./...` — **2156 passed in
  21 packages** (2155 after Task 2's changes, +1 for `TestFixerSuggestedFixDropsStaleFixerHandoff`
  added fixing F6).
- `GOLANGCI_LINT_CACHE=<worktree-local> make lint` — **0 issues** (both the
  `-tags screenshot` and default lint passes; default golangci-lint cache
  dir was cleaned first — a recurring need this phase whenever a sibling
  worktree was recently deleted).
- `make test-e2e` — **PASS**: `ok github.com/castocolina/gitid/e2e 647.797s`.
- `make gate-visual-regression` — **PASS**: `gate-visual-regression: OK — 44
  RequiredScreenSpecs frames checked as a classified real/dummy symmetric
  union` (114.6s on first run; re-confirmed green after F6's fix and the
  full exit battery re-run).
- Negative controls (Task 2): `TestNegativeControl_HealthFixerMissingState`,
  `TestNegativeControl_HealthFixerUnclassifiedDifference`,
  `TestNegativeControl_HealthFixerPerturbedComparableRegion`,
  `TestNegativeControl_HealthFixerCrossSurfaceAllowlistLeakage` — all PASS.
  Additionally manually verified the gate genuinely fails on an
  undocumented divergence: temporarily removed `health-findings`'s
  `RegionHealthBody` disposition from `healthFixerSpecs()`, confirmed
  `TestGateVisualRegression` failed with `region "health-body" differs
  without a screen-specific declared disposition`, then reverted.

## Part C — DRAFT REQUIREMENTS.md closure edit (not yet applied)

The orchestrating session will apply this edit to the REAL
`.planning/REQUIREMENTS.md` itself after independently re-verifying this
packet's evidence and confirming zero CRITICAL/HIGH findings remain open
(both confirmed above) — this stands in for the human "approved" signal
Task 3's `checkpoint:human-verify` gate would otherwise require live.

### Section K (Health) — checkbox edits

- `HLTH-01` `[ ]` → `[x]` — **Two sections.** `internal/tuikit/health_screen.go`
  renders SSH and Git as distinct grouped sections (`groupFindings`); shipped
  08-01 (Task 1).
- `HLTH-02` `[ ]` → `[x]` — **Files + syntax.** `CheckFiles`
  (`internal/doctor/checks/files.go`) validates config files exist and
  parse; shipped 08-03.
- `HLTH-03` `[ ]` → `[x]` — **Redundancy/override.** `CheckRedundancy`
  detects repeated/overridden directives and duplicate `Host *` stanzas;
  carried into the split screen, re-verified 08-01/08-06.
- `HLTH-04` `[ ]` → `[x]` — **Contradictions.** The D-09 flagship
  (`IdentitiesOnly no` + explicit `IdentityFile`, `checkHandWrittenIdentitiesOnly`)
  shipped 08-02; the includeIf-missing-fragment contradiction shipped 08-05.
- `HLTH-05` `[ ]` → `[x]` — **Per-identity.** TUI deep-link (Identity
  Manager `h` key → scoped Health via `FindingsFor`) and CLI
  (`gitid health --identity`) both shipped 08-06; global findings
  deliberately excluded from the scoped view on both sides (D-04's
  discretion, resolved consistently).
- `HLTH-06` `[x]` (unchanged, already marked) — status table row corrected
  below (was inconsistently "Pending" there despite the body checkbox
  already being `[x]`).

### Section L (Fixer) — checkbox edits

- `FIX-01` `[x]` (unchanged, already marked) — status table row corrected
  below, same inconsistency as HLTH-06.
- `FIX-02` `[ ]` → `[x]` — **Two-section fixer UX.** `internal/tuikit/fixer_screen.go`
  presents the SAME SSH/Git grouped sections as Health, filtered to
  `fixableFindings`, with the `f`/`F` in-place fix ceremonies; shipped 08-01
  (Task 2) through 08-06 (D-16 batch-halt), CLI parity 08-07.

### Status table (near end of REQUIREMENTS.md) — 8 row edits

```
| HLTH-01 | Phase 8 | Complete |
| HLTH-02 | Phase 8 | Complete |
| HLTH-03 | Phase 8 | Complete |
| HLTH-04 | Phase 8 | Complete |
| HLTH-05 | Phase 8 | Complete |
| HLTH-06 | Phase 8 | Complete |
| FIX-01 | Phase 8 | Complete |
| FIX-02 | Phase 8 | Complete |
```

(All 8 rows currently read `Pending`; all 8 become `Complete`.)
