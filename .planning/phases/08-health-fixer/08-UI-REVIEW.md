# Phase 08 — UI Review

**Audited:** 2026-08-28
**Baseline:** `.planning/phases/08-health-fixer/08-UI-SPEC.md`, cross-checked against `.planning/design/health/FIELDS.md` and `.planning/design/fixer/FIELDS.md` (the frozen field-parity contracts), and against the approved Bubble Tea mockup workflow this project's binding rule (CLAUDE.md, `02-UX-DIRECTION.md`) names as the SOLE parity target for Phases 3-10 — `cmd/gitid-dummy`, which shares `internal/tuikit`'s render code with `cmd/gitid` byte-for-byte, so mockup-vs-live divergence for Phase 8 reduces to a same-code-path check (confirmed below).
**Evidence:** the real `cmd/gitid` binary, built fresh (`go build -o bin/gitid ./cmd/gitid`, exit 0) and driven through a real PTY session (raw keystrokes, 100×30 geometry) against a hand-seeded fixture home — an independent capture, not a re-read of `e2e/health_fixer_pty_e2e_test.go`'s own assertions. That test suite (7 tests) was ALSO independently re-run for real in this session (`go test -tags e2e -run TestHealthFixer_RealPTY -v ./e2e/...` → 7 passed) as a second, corroborating evidence source, plus 4 targeted unit tests re-run live (`TestHealthNegativeAssertion`, `TestFixerCompleteFixableSet`, `TestBatchWalkHalt`, `TestFixerSuggestedFixDropsStaleFixerHandoff` → all PASS). `make gate-visual-regression` re-run for real in this session (44 frames, symmetric real/dummy union) — PASS. Screenshots: not applicable (Go Bubble Tea v2 TUI, not a web app), per this project's own standing audit method.

**Known Divergences (pre-approved, out of scope — NOT flagged below):** #1 (5 tabs, not the historical mockup's 4 — confirmed live: `frame.go:75`'s `tabLabels` is exactly `["Identities", "Global SSH", "Global Git", "Health", "Fixer"]`) and #2 (the compressed 2-state ceremony, not FIELDS.md's literal `v→x→y→z` 4-screen chain — confirmed live: `fixCeremonyFor`/`ceremony.go` renders diff+destructive-confirm+backup-notice as ONE preview state, receipt as ONE result state, in the PTY capture below).

---

## Pillar Scores

| Pillar | Score | Key Finding |
|--------|-------|-------------|
| 1. Copywriting | 4/4 | Every frozen string (read-only banner, safety banner, D-11 typed-confirm warning, D-14/D-16 new copy, result receipt) verified byte-exact in real PTY output and via source grep — zero drift from the Copywriting Contract |
| 2. Visuals | 3/4 | Master-detail layout, severity glyphs, and read-only/safety banners render correctly on both screens; but the shared `ceremony.go` component the flagship D-09 rewrite depends on has 3 pre-existing, already-documented CRITICAL-rated layout defects (confirm keybinding can clip off-screen, cramped non-full-width ceremony pane) that directly touch the Fixer's own highest-risk affordance |
| 3. Color | 4/4 | Zero new hues/roles confirmed by source grep (`styleBold/Error/Faint/Healthy/Info/Selected` only); glyph+word pairing holds in every captured frame, NO_COLOR-legible |
| 4. Typography | 4/4 | Zero new roles; Label/Field/Selected/Hint/Info/Warning/Healthy/Error/Preview usage matches the spec's role table exactly, confirmed live and by source read |
| 5. Spacing | 3/4 | 100×30 geometry holds for the pinned fixture (6-finding capture, no scroll cue needed); but the finding-list overflow behavior for the realistic 8+-check D-05 queue remains an UNTESTED "backstop" per the spec's own admission — never independently exercised by this audit or, as far as the SUMMARY.md files show, by the phase itself |
| 6. Experience Design | 3/4 | Every named state (all-green, nothing-to-fix, per-identity, parse-error, batch-walk, batch-halt, destructive typed-confirm) is real-PTY-verified and byte-correct; but the tab bar stays live and switchable mid-ceremony (digit keys can abandon a half-typed destructive confirm), a pre-existing app-level gap that lands squarely on the Fixer's own flagship destructive action |

**Overall: 21/24**

---

## Top 3 Priority Fixes

1. **The Fixer's own highest-risk ceremony (the D-09 typed-confirm rewrite) can clip its Enter/Esc keybindings off-screen on a long diff, and the confirm pane is not full-width** — user impact: this is not a cosmetic nit on an ordinary screen — it is the flagship "rewrite an existing hand-written directive" action the whole UI-SPEC calls out as the single highest-risk affordance in the phase (`08-UI-SPEC.md` "Focal points… Fixer's D-09 scoped power — primary focus"), and the confirm/cancel affordance can scroll below the fold exactly when the user most needs to see it. This is `08-08-REVIEWS.md`'s F1/F2/F3 (CRITICAL-rated by the independent `agent-ui-ux-designer` review that phase already ran), correctly root-caused to the shared `internal/tuikit/ceremony.go` component (used by every mutating flow project-wide) and deferred as a cross-cutting fix outside Phase 8's DLV-04/DLV-06 charter — a reasonable scoping call, but it means the defect is still live in the shipped Fixer today. Concrete fix: give `ceremony.go`'s preview pane a scroll-aware footer (pin the Enter/Esc keybar to the bottom of the visible viewport, never let it scroll with the diff body) and widen the ceremony pane to full-width during an active confirm (hiding the now-irrelevant findings list), independent of the diff length.

2. **Digit-key tab navigation stays live during an active destructive-confirm ceremony** — user impact: a user mid-way through typing `"clientb.github.com"` to confirm the D-09 flagship rewrite (a case-sensitive, no-partial-match typed confirm) can accidentally hit a digit key that is itself part of a hostname (there is no numeral in "clientb.github.com", but future D-05-added fixable rewrites are not guaranteed to avoid one) or simply fat-finger a `1`-`5` press, silently abandoning the half-typed confirm and jumping to a different tab with no warning — this is `08-08-REVIEWS.md`'s F7 (HIGH), deferred as an app-level (`app.go`) gap affecting every ceremony, not Fixer-specific. Concrete fix: while a `ceremonyModel` is active and `confirmEnabled`, have `App.handleKey` swallow the `1`-`5` tab-switch keys (or route them to the ceremony's own Esc-cancel handling first) rather than passing them through to tab navigation.

3. **The finding-list overflow behavior for a realistic (8+ check) D-05 queue is unverified** — user impact: `08-UI-SPEC.md`'s own "UI Considerations" table lists this as a `🧪 backstop` — explicitly not yet triggered by the pinned 5-6-finding fixture, with Phase 7's "+N more" scrolling-window vocabulary named as the fallback if `frameBodyRows(30)` overflows. Neither this audit's own PTY capture (6 findings, fits cleanly with margin) nor any of the 8 SUMMARY.md files' verification sections exercise a finding count large enough to hit the budget ceiling — so it is genuinely unknown today whether Health/Fixer silently truncate, panic, or scroll correctly once the real doctor engine's full check set (9 `CheckFn` families, per-identity × per-section) is exercised against a realistically messy home. Concrete fix: build one more e2e fixture that seeds enough real findings (10+) to force the 100×30 budget, and assert the "+N more" cue renders correctly (or that no cue is needed) rather than leaving this as a standing backstop.

---

## Detailed Findings

### Pillar 1: Copywriting (4/4)
- `HealthReadOnlyNote` (`internal/dummytui/data.go:477`) — `"Health only diagnoses -- nothing here writes to your files. Open the Fixer (key 5) to change anything shown."` — confirmed verbatim, source-grepped.
- `FixerSafetyNote` (`data.go:521`) — `"Every fix is previewed, confirmed, and backed up before anything is written -- never a blind write."` — confirmed verbatim, and confirmed rendered live: real PTY capture shows `"every fix is previewed"` on opening the Fixer tab (key `5`).
- D-11 typed-confirm flagship warning confirmed rendered live, real PTY: `"Type the Host name \"clientb.github.com\""` on pressing `f` against the real seeded contradiction — matches `fixplans.go`'s `Warning` field exactly.
- Fix result receipt confirmed rendered live, real PTY, AND byte-verified against the real file: `"IdentitiesOnly set to yes on Host clientb.github.com in ~/.ssh/config."` plus `"Backed up →"`, and the real `~/.ssh/config` + its `.bak.*` file were diffed to prove only the ONE directive changed (re-executed in this session via `go test -tags e2e -run TestHealthFixer_RealPTYFixerCeremonyWritesAndBacksUp`, PASS).
- D-16 queue-halt message confirmed present verbatim in source (`fixer_screen.go:97`): `"Fix %d of %d failed and was rolled back from its own backup -- the first %d fixes already applied stand. Nothing else in this batch was attempted."` — matches the Copywriting Contract's DRAFT string exactly, and `TestBatchWalkHalt` (re-run live, PASS) proves it renders with real interpolated numbers.
- Health's read-only detail-pane hand-off line confirmed live: `"Switch to Fixer to apply this."` present in the captured frame's detail pane, immediately after `"Suggested fix: …"`.
- F6 from `08-08-REVIEWS.md` (Fixer's detail pane redundantly said "available on the Fixer screen" while already standing on Fixer) was found and FIXED within Phase 8 itself (`fixerSuggestedFixText`, regression test `TestFixerSuggestedFixDropsStaleFixerHandoff` — re-run live in this session, PASS) — correctly not re-flagged here.

### Pillar 2: Visuals (3/4)
- Master-detail layout confirmed live: SSH/Git sections render as distinct grouped lists with a right-side detail pane; the always-visible inline detail (Known Divergence #1's "already exceeds the frozen contract" claim) confirmed live — the detail pane shows the FULL explanation + suggested-fix line inline, not a `v full detail` hint placeholder, on both Health and Fixer.
- Severity glyph+word pairing confirmed live in the captured frame: `✗ error`, `! warning` both render with glyph AND word, never color alone (visible even in the ANSI-stripped capture).
- Health's negative assertion (no `f`/`F`, no "Fix this…" line, no ceremony marker anywhere) independently re-verified live via `TestHealthNegativeAssertion` (re-run in this session, PASS across all 4 render states: with-findings, all-green, per-identity, parse-error) — this is the phase's own highest-risk read-only-integrity affordance and it holds.
- **WARNING** (inherited, already dispositioned in `08-08-REVIEWS.md`, not a new finding): F1/F3 — the shared `ceremony.go` pane used for the D-09 flagship rewrite is not full-width and its footer keybinding can scroll below the fold on a long diff; correctly root-caused to a cross-cutting shared component and deferred outside this wave's scope, but still a live defect on the Fixer's own flagship screen (see Top Fix #1).
- No BLOCKER-class visual defect found independently — the checkbox-blank-cell class of bug Phase 7's audit caught does not recur here (Health/Fixer's row template is glyph+word+title, no checkbox column at all, so that specific defect class has no surface to occur on).

### Pillar 3: Color (4/4)
- Source grep of `internal/tuikit/health_screen.go`/`fixer_screen.go` confirms only 6 style roles in direct use (`styleBold`, `styleError`, `styleFaint`, `styleHealthy`, `styleInfo`, `styleSelected`) — `styleWarning` lives in the shared `severityLabel` helper (`doctor.go`), consistent with the spec's "zero new theme roles" claim; no hardcoded hex/rgb found.
- Accent (`styleSelected`) confirmed scoped to the currently-highlighted row only in the live capture — never a whole-screen treatment.
- `SeverityInfo`/`~` cyan confirmed used only for report-only/advisory rows (the D-05 "set, differs" hard-cap class); `styleError`/red confirmed scoped to the destructive-confirm warning and error/critical glyphs, not misused elsewhere.

### Pillar 4: Typography (4/4)
- No new roles found. `Bold` for finding titles/detail headings, `Faint` for `Family · fixNote` sub-lines and read-only/safety banner trailing clauses, `Selected` for the highlighted row, `Info`/`Warning`/`Healthy`/`Error` for severity glyphs+words — all confirmed present and correctly scoped in the live capture and by source read.

### Pillar 5: Spacing (3/4)
- 100×30 frame geometry held in the live capture; the 6-finding fixture (real doctor scan against a real seeded contradiction + baseline gaps) rendered with margin to spare, no scroll cue triggered.
- Read-only banner (Health) and safety banner (Fixer) both render as the spec's stated +1 fixed row at the top of the body, confirmed live.
- **WARNING**: the realistic-scale overflow case (`08-UI-SPEC.md`'s own `🧪 backstop` row: "5 fixtures today, growing with D-05's new-checks queue") remains genuinely untested — not by this audit's own capture, and not evidenced in any of the 8 SUMMARY.md files' verification sections, which all exercise small (2-6 finding) fixtures. This is a real, currently-open gap, not a resolved item mis-scored down (see Top Fix #3).

### Pillar 6: Experience Design (3/4)
- Positive, re-verified live in this session: all-green (`"SSH -- 0 fixable problems"`/`"Git -- 0 fixable problems"`), nothing-to-fix (same shape on Fixer), per-identity health (`legacy` identity scoped correctly, global findings excluded per D-04's resolved discretion), parse-error (`"configuration parse error"`, `"Checks paused until this configuration parses again."`, `"Raw error:"` all present), the D-09 flagship ceremony (real diff before/after lines, typed confirm, real backup, byte-verified rewrite), the `F` batch walk (auto-advancing queue, real permission-byte fixes verified), and the D-16 batch-halt message (`TestBatchWalkHalt`) — 7/7 real e2e tests plus 4/4 targeted unit tests re-run PASS in this session.
- D-13's re-run-ALL-checks invariant confirmed by source trace (`persistFixFinding` builds a FRESH `doctor.Deps` for the re-scan, not a stale reused one — the exact bug class 08-02-SUMMARY.md records finding and fixing).
- **WARNING** (inherited, already dispositioned in `08-08-REVIEWS.md` as F7, not a new finding): the app-level tab bar remains switchable mid-ceremony, meaning a digit keypress can silently abandon a half-typed destructive confirm with no warning — this specifically undermines the Fixer's own highest-risk affordance's safety story (see Top Fix #2).
- Not independently re-verified in this pass (relying on the phase's own accepted, cited unit-test coverage): the four-quadrant per-identity health matrix (only the pinned SSH-healthy/Git-broken quadrant was PTY-driven, both by this audit and by `08-UI-SPEC.md`'s own admitted `🧪 backstop` status for the other 3 quadrants) and the two-file gitignore diff's rendering inside one ceremony preview block.

---

## Registry Safety

Not applicable — no `components.json` / shadcn in this stack (Go Bubble Tea v2 TUI). Per the UI-SPEC's own Registry Safety section: "not applicable — no component registry in this stack."

---

## Files Audited

- `.planning/phases/08-health-fixer/08-UI-SPEC.md` (design contract, including both Known Divergence sections)
- `.planning/design/health/FIELDS.md`, `.planning/design/fixer/FIELDS.md` (frozen field-parity contracts)
- `.planning/phases/08-health-fixer/08-0{1..8}-SUMMARY.md` (execution summaries — scope decisions cross-checked)
- `.planning/phases/08-health-fixer/08-08-REVIEWS.md` (the phase's own independent `agent-ui-ux-designer` review packet, 15 findings F1-F15 — cross-referenced, not re-derived, for Top Fixes #1/#2)
- `internal/tuikit/health_screen.go`, `internal/tuikit/fixer_screen.go` (render logic for both new screens)
- `internal/tuikit/frame.go` (`tabLabels`, `TabID`, master-detail helpers, header chip)
- `internal/tuikit/ceremony.go` (shared 2-state ceremony component — Known Divergence #2)
- `internal/tuikit/doctor.go` (shared `groupFindings`/`orderedFindings`/`severityLabel`/`fixableFindings`)
- `internal/dummytui/data.go` (frozen copy constants: `HealthReadOnlyNote`, `FixerSafetyNote`)
- `internal/doctor/doctor.go`, `internal/doctor/checks/coherence.go` (D-09 flagship check, `FamilyFiles`)
- `cmd/gitid/wiring.go` (D-13/D-14 real-backend convergence logic, `realBackend.Persist`)
- `e2e/health_fixer_pty_e2e_test.go` (7 tests, re-run live for real in this session, all PASS — used as a second independent evidence source, not the primary one)
- `internal/tuikit/health_screen_test.go`, `internal/tuikit/fixer_screen_test.go` (4 targeted unit tests re-run live, all PASS)

## Independent verification performed

Ran, in the working tree, for real, in this session (not accepted from any prior report):

```
$ go build -o bin/gitid ./cmd/gitid && go build -o bin/gitid-dummy ./cmd/gitid-dummy
exit 0 (both)

$ python3 <PTY driver script>  -- raw-keystroke PTY drive of the real binary
against a hand-seeded fixture home (Host clientb.github.com: IdentitiesOnly no
+ IdentityFile ~/.ssh/id_ed25519_clientb, untouched.example.com)
→ confirmed live: SSH/Git sections, severity glyphs+words, inline detail pane,
  "Suggested fix:", "Switch to Fixer to apply this.", 6 findings rendered
  cleanly within the 100×30 budget, no scroll cue.

$ TERM=dumb SSH_AUTH_SOCK= go test -tags e2e -run TestHealthFixer_RealPTY -v ./e2e/...
Go test: 7 passed in 1 packages

$ TERM=dumb SSH_AUTH_SOCK= go test -count=1 ./internal/tuikit/... \
  -run 'TestHealthNegativeAssertion|TestFixerCompleteFixableSet|TestBatchWalkHalt|TestFixerSuggestedFixDropsStaleFixerHandoff' -v
PASS (all 4, including 4 sub-tests of TestHealthNegativeAssertion)

$ make gate-visual-regression
PASS (44 RequiredScreenSpecs frames, symmetric real/dummy union)
```

This is genuine, freshly-executed evidence produced by this audit against the
real compiled binary — the raw PTY capture and the unit-test re-runs are
independent of, and corroborate rather than merely restate, the phase's own
`e2e/health_fixer_pty_e2e_test.go` assertions.
