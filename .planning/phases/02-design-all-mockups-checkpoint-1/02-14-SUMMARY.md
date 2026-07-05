---
phase: 02-design-all-mockups-checkpoint-1
plan: 14
subsystem: ui
tags: [lipgloss-v2, mui, theme-contract, arrow-key-nav, field-contour, dummy-tui, live-demo, dlv-01, dlv-02, dlv-05]

# Dependency graph
requires:
  - phase: 02-design-all-mockups-checkpoint-1
    provides: cmd/gitid-dummy + internal/dummytui (the LIVE interactive Go TUI demo, 02-13), the interactive web demo at .planning/design/mockup-src/src/demo/, and 02-REVIEWS.md's round-2/round-3 cross-AI consensus (the eight feedback items and two HIGH implementation traps this plan absorbs)
provides:
  - .planning/phases/02-design-all-mockups-checkpoint-1/02-STYLE-SPEC.md — the cross-media semantic style contract (11-role table, numbered arrow-key precedence rule, six new parity dimensions, frozen slide-3 copy, frozen stepper short<->long label map)
  - internal/dummytui/theme.go — central Go `Theme` + `DefaultTheme`, promoted behavior-preservingly from frame.go's package-level style vars
  - internal/dummytui/frame.go — bounded/titled/stable-height `PreviewBlock`, header DisabledNav dimming + ActiveArea accent on the breadcrumb divider while a pane captures keys
  - internal/dummytui/identities.go — `renderStepper` (first-class `[1] SSH · [2] Test · [3] Git · [4] Review`), the written arrow-key precedence rule across all three wizard steps (+ Shift focus-override chord), focused/blurred field contours, a persistent match-strategy hint row, and the frozen `[ Skip Git ]`/`[ Continue ]` copy
  - .planning/design/mockup-src/src/theme.ts + demo/{DemoApp,Frame,MutationCeremony,screens/Identities}.tsx — the web mirror: Shift+<-/-> focus-override chord, wizard <-/-> nav with a local MUI-Select guard, shortened slide-3 buttons + persistent hints, disabled-nav/active-area chrome, preview role token
  - .planning/design/create-flow/FIELDS.md — the git-form button-copy row + the six emphasis-role parity rows
affects: [02-12 (the human checkpoint presents both live demos and must also close the deferred designer-critique + code-review gates this plan could not run), Phase 3+ (the real product TUI grows out of this frame)]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Theme promotion: package-level lipgloss style vars (styleBold/styleFaint/styleHealthy/styleWarning/styleError/styleInfo) now derive from a central DefaultTheme struct, proven behavior-preserving by a byte-identical-render test rather than by re-reading every call site"
    - "Title-in-border-top-edge splice: render a bordered box first, then rewrite its first (top-border) line to embed a label between the corners — reused for both PreviewBlock's title and the focused field's label, without needing lipgloss to natively support titled borders"
    - "Arrow-key precedence as an explicit, written, numbered rule (not a mental model): expanded-select-owns-arrows > text-input-cursor-never-intercepted > button/non-editing-focus-navigates-wizard-steps (validity-gated forward, always-allowed back) > Shift+<-/-> as a focus-override (never a validity override) — implemented identically in both media from one shared spec document"
    - "Row-budget-first field contour: only the FOCUSED field gets a full rounded box (+2 rows); every blurred field gets a single-row dim bracket contour, never a border on every field"

key-files:
  created:
    - .planning/phases/02-design-all-mockups-checkpoint-1/02-STYLE-SPEC.md
    - internal/dummytui/theme.go
    - internal/dummytui/theme_test.go
  modified:
    - internal/dummytui/frame.go (PreviewBlock title/bounded/stable-height, RenderFrame DisabledNav+ActiveArea dimming)
    - internal/dummytui/frame_test.go
    - internal/dummytui/identities.go (renderStepper, arrow-key precedence, field contours, stable hint zone, frozen copy)
    - internal/dummytui/identities_test.go
    - internal/dummytui/batch3_test.go
    - e2e/dummy_demo_e2e_test.go
    - .planning/design/mockup-src/src/theme.ts (roles export)
    - .planning/design/mockup-src/src/demo/DemoApp.tsx (Shift+<-/-> focus-override)
    - .planning/design/mockup-src/src/demo/Frame.tsx (capturesKeys prop)
    - .planning/design/mockup-src/src/demo/MutationCeremony.tsx (preview role, title/maxHeight)
    - .planning/design/mockup-src/src/demo/screens/Identities.tsx (wizard arrow nav, shortened buttons+hints, focused-field accent, persistent strategy hint)
    - .planning/design/create-flow/FIELDS.md

key-decisions:
  - "The ActiveArea accent mechanism is the breadcrumb/divider line (zero extra rows), not a frame-wide border — the 100x30 budget could not absorb a bordered active-pane region"
  - "PreviewBlock (the new bounded/titled variant) and previewBlockClipped (the pre-existing shrink-wrap variant every other screen already calls) are kept as SEPARATE render paths — only PreviewBlock fills to the pane width and splices a title, so no other screen's rendered output changed"
  - "The frozen skip/continue hint lines render on their OWN dedicated row, never appended inline after the button text — a hyphenated wrap of \"SSH-only\" mid-word corrupted the frozen text when re-flowed at the pane's narrower width during testing"
  - "The Git-form step's redundant 'Signing: ... a PATH, never key material' line was dropped (the signingkey path is already visible in the fragment preview block) to make row-budget room for the field-contour box and the two new frozen hint lines — this is a real, documented row-budget tradeoff, not silent scope creep"
  - "wizardSteps (long labels) stays the breadcrumb/help source; a new stepShortLabels list is the independent source renderStepper draws its short segments from — the two were never meant to be the same list (round-3 defect D3 resolved)"

patterns-established:
  - "One arrow-key precedence rule, one document (02-STYLE-SPEC.md §2), implemented identically in Go and TypeScript — future contended-key features should extend this table rather than inventing per-surface rules"
  - "Theme roles are named contracts (info/label/field/focused-field/blurred-field/hint/warning/error/preview/disabled-nav/active-area) mirrored 1:1 by name across Go's Theme struct and the web's theme.ts roles export — future UI work should add new visual states as named roles in both places, not ad-hoc styles"

requirements-completed: [DLV-01, DLV-02, DLV-05]

# Metrics
duration: ~100min
completed: 2026-07-05
---

# Phase 02 Plan 14: Round-2/Round-3 Checkpoint-Feedback Polish Summary

**Absorbed the cross-AI round-2/round-3 consensus as one polish wave: a shared 11-role Go/TypeScript style contract, a written and identically-implemented arrow-key precedence rule (incl. a Shift+←/→ focus-override chord), a first-class `[1] SSH · [2] Test · [3] Git · [4] Review` stepper replacing the dim old `Step n/4` line, focused-accent/blurred-dim field contours that still fit 100×30, a persistent match-strategy hint, bounded/titled preview blocks, and an atomic `[ Skip Git ]`/`[ Continue ]` copy freeze across both live demos, FIELDS.md, and every Go/TSX pin.**

## Performance

- **Duration:** ~100 min
- **Tasks:** 3 (all `type="auto"`, Tasks 1 and 3 `tdd="true"`)
- **Files modified:** 14 (3 created, 11 modified)

## Accomplishments

- `02-STYLE-SPEC.md` is now the single cross-media source of truth for emphasis roles, the arrow-key precedence rule, the six new parity dimensions, and the frozen slide-3/stepper copy — resolving all three plan-authoring defects the round-3 review flagged (D1 e2e-file scope, D2 MUI-Select guard, D3 stepper label-source contradiction) as already-fixed in the plan text this executor read.
- A central `internal/dummytui.Theme` (`DefaultTheme`) now drives every TUI style, promoted from frame.go's ad-hoc vars in a provably behavior-preserving refactor (byte-identical render output pinned by a dedicated test) — mirrored 1:1 by role name with a new `roles` export in the web `theme.ts`.
- The wizard's `Step n/4 · <label>` line (which read dimmer than body text — the opposite of a nav affordance) is gone, replaced by a first-class `[1] SSH · [2] Test · [3] Git · [4] Review` stepper with an accent-bold active segment and ✓-marked completed segments.
- The arrow-key precedence rule — expanded-select-owns-arrows, text-input-cursor-never-intercepted, button/non-editing-focus-navigates-wizard-steps (validity-gated forward, always-allowed back), Shift+←/→-as-focus-override — is implemented identically in the Go wizard (all three steps) and the web `CreateWizard`, each backed by table-driven tests covering every focus context.
- Field contours now exist without breaking the 100×30 budget: the one focused field gets a full rounded accent box (title spliced into its top border edge), every blurred field gets a single-row dim bracket — proven by both a unit test and a forced (`-count=1`) re-run of the 100×30 raw-keystroke PTY walk.
- The slide-3 button copy freeze (`[ Skip Git ]` / `[ Continue ]` + their frozen hint lines) is atomic across both demos, `FIELDS.md`, and every Go/TSX test pin — proven by a repo-wide grep gate with zero matches.

## Task Commits

1. **Task 1: Semantic style contract — 02-STYLE-SPEC.md + central Go Theme + web theme.ts role tokens + bounded PreviewBlock + dimmed chrome** - `f074d8b` (feat, tdd)
2. **Task 2: Web demo — ←/→ wizard navigation, shortened slide-3 buttons, stable hint zones, role tokens applied** - `02518c7` (feat)
3. **Task 3: TUI demo — first-class stepper, ←/→ precedence, focused/blurred field contours, stable hint zone, atomic copy freeze + all exit gates** - `e320e5d` (feat, tdd)

## Files Created/Modified

- `.planning/phases/02-design-all-mockups-checkpoint-1/02-STYLE-SPEC.md` - the cross-media semantic style contract (role table, precedence rule, parity dimensions, frozen copy)
- `internal/dummytui/theme.go` / `theme_test.go` - central `Theme`/`DefaultTheme`, promotion-preserving tests
- `internal/dummytui/frame.go` / `frame_test.go` - bounded/titled/stable-height `PreviewBlock`, header dimming + breadcrumb accent
- `internal/dummytui/identities.go` / `identities_test.go` / `batch3_test.go` - stepper, arrow precedence, field contours, hint zone, copy freeze
- `e2e/dummy_demo_e2e_test.go` - stepper PTY pins updated to the new markers
- `.planning/design/mockup-src/src/theme.ts` - `roles` export mirroring the Go Theme
- `.planning/design/mockup-src/src/demo/{DemoApp,Frame,MutationCeremony,screens/Identities}.tsx` - Shift focus-override chord, wizard arrow nav + MUI-Select guard, shortened buttons + hints, disabled-nav/active-area chrome, preview role
- `.planning/design/create-flow/FIELDS.md` - git-form button-copy row + six emphasis-role parity rows

## Decisions Made

See `key-decisions` in the frontmatter above (ActiveArea-via-breadcrumb mechanism; PreviewBlock/previewBlockClipped kept as separate render paths; frozen hints on their own row to avoid hyphen-wrap corruption; the "Signing:" line drop for row budget; stepShortLabels as an independent source from wizardSteps).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Row-budget overflow from the new field-contour box + persistent hints + frozen copy lines**
- **Found during:** Task 3, while re-running `make test-e2e` against the Git-identity wizard step
- **Issue:** Stacking the focused-field rounded box (+2 rows), the always-drawn match-strategy hint (+1 row), and the two frozen skip/continue hint lines (each needing 1-2 rows once word-wrapped at the 62-column detail pane) overflowed the 100×30 frame's fixed 25-row body budget — the bottom of the pane (the Skip/Continue buttons and/or their hints) was silently truncated by `RenderFrame`'s hard line-count clip, discovered via a temporary debug harness that dumped the raw rendered frame line-by-line.
- **Fix:** (a) dropped the redundant "Signing: ... a PATH, never key material" line (the signingkey path is already visible in the fragment preview block below it); (b) tightened the fragment/includeIf preview `maxLines` from 4/2 down to 1/1 (both already clip with the `… (+n more lines)` cue, so no information is silently lost, only more aggressively summarized); (c) shaved one character off the compact match-strategy line's spacing so it stops wrapping at 63 vs the 62-column pane width; (d) moved the frozen skip/continue hint lines onto their OWN dedicated row instead of appending them inline after the button text (which also fixed a hyphen-wrap corruption of "SSH-only" — see Issue 2 below).
- **Files modified:** internal/dummytui/identities.go
- **Verification:** re-ran the raw rendered-frame dump after each change until all content (buttons + both frozen hint lines) rendered inside the 25-row body budget with the last row exactly at the boundary; `make test-e2e` (forced `-count=1`) passed clean at 160.7s.
- **Committed in:** e320e5d (Task 3 commit)

**2. [Rule 1 - Bug] A word-wrapped hyphenated word ("SSH-only") corrupted a frozen test assertion**
- **Found during:** Task 3, debugging the inline hint-line layout above
- **Issue:** When the frozen skip hint (`"Skip keeps this identity SSH-only and marks it incomplete."`) was appended inline after the button text and the combined line exceeded the 62-column pane width, lipgloss's word-wrap broke the line exactly at the hyphen in "SSH-only" (rendering "SSH-" at the end of one row and "only" at the start of the next) — the test helper `paneFlat` (which collapses wrapped lines by joining them with a single space) then reconstructed "SSH- only" instead of "SSH-only", breaking the exact-substring assertion for genuinely frozen copy.
- **Fix:** moved the hint onto its own dedicated row (see Issue 1(d)) so it either fits on one line without wrapping, or — for the longer Continue hint — wraps only at word-space boundaries (no hyphens in that string), which `paneFlat`'s space-joining correctly reconstructs.
- **Files modified:** internal/dummytui/identities.go
- **Verification:** `TestWizardGitStepButtonsAreFocusable` and `TestWizardFullFlowCreatesCompleteIdentity` (both assert the frozen hint strings verbatim) pass.
- **Committed in:** e320e5d (Task 3 commit)

**3. [Rule 1 - Bug] `TestWizardGitButtonsArrowMovement` tested the now-removed button-ring-arrow behavior**
- **Found during:** Task 3, after implementing the arrow-key precedence rule's button-slot-navigates-wizard-steps clause
- **Issue:** The plan explicitly REPLACES the old rule (←/→ on a focused button moves between Back/Skip/Continue) with wizard-step navigation, but the pre-existing `TestWizardGitButtonsArrowMovement` (batch3_test.go) still asserted the old behavior — it would have failed by design once the new behavior was implemented, and is exactly the kind of pin the plan's own `<read_first>` flagged for update.
- **Fix:** rewrote the test as `TestWizardGitButtonsArrowNavigatesWizardSteps` (asserting arrow-driven step transitions instead of button-ring movement) and added a companion `TestWizardShiftArrowIsFocusOverrideNotValidityOverride` covering the Shift-chord semantics from a focused text field.
- **Files modified:** internal/dummytui/batch3_test.go
- **Verification:** both new tests pass; no other test asserted the old button-ring-arrow behavior (verified via `go test ./internal/dummytui/...`).
- **Committed in:** e320e5d (Task 3 commit)

---

**Total deviations:** 3 auto-fixed (3 Rule 1 — all bugs/behavior corrections surfaced by implementing this plan's own row-budget and precedence requirements, not scope creep).
**Impact on plan:** All three are direct consequences of correctly implementing the plan's stated requirements (field contour + frozen hints + arrow-nav replacement) within the plan's own stated constraint (100×30 fit). No functionality was added beyond what the plan specifies; some pre-existing supplementary copy ("Signing: ...") was trimmed for row budget and is called out above and in the SUMMARY/test comments for traceability.

## Issues Encountered

- **The fresh `agent-ui-ux-designer` critique (Task 3's final exit-gate item, DLV-02) and the `superpowers:requesting-code-review` skill (the plan's overall `<success_criteria>` requirement) could not be run** — this executor's toolset does not expose a subagent-spawning or skill-invocation mechanism (Read/Write/Edit/Bash only). Every automatable gate (`go test -race`, the atomicity grep gate, the no-backend import allowlist, `make test`, `make lint`, `make test-e2e` incl. the 100×30 PTY walk, `make gate-no-backend-files`) is green and re-verified in this session. The two agent-mediated reviews are explicitly flagged here as **open items the 02-12 checkpoint (or the phase-level orchestrator, which has agent-spawning access) must close** before the design approval is signed — 02-12's own `read_first` already includes this SUMMARY, so a missing critique/review should read as a blocker there, not as silently satisfied.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- All eight round-2 consensus items, both HIGH implementation traps (row budget, arrow-key precedence), and the copy-freeze atomicity requirement are implemented and gate-verified in both live demos.
- 02-12 (wave 8, the single DLV-08 design-approval checkpoint) can now proceed — its own `read_first` already expects this plan's SUMMARY and the amended must_haves/E2/E3 checklist items.
- **Blocker for 02-12 sign-off:** the fresh `agent-ui-ux-designer` critique of both live demos on the six new emphasis-role dimensions, and the `superpowers:requesting-code-review` pass against this plan's `must_haves`/`<acceptance_criteria>`, are both still outstanding (see "Issues Encountered") and must be run by an agent with the appropriate tool access before the checkpoint is signed.

---
*Phase: 02-design-all-mockups-checkpoint-1*
*Completed: 2026-07-05*

## Self-Check: PASSED

- All key files (02-STYLE-SPEC.md, theme.go/theme_test.go, frame.go, identities.go, e2e/dummy_demo_e2e_test.go, FIELDS.md, theme.ts, DemoApp.tsx, Frame.tsx, MutationCeremony.tsx, screens/Identities.tsx) verified present on disk.
- Commits f074d8b (Task 1), 02518c7 (Task 2), e320e5d (Task 3), 523cfce (this SUMMARY) verified present in `git log`.
