# Phase 06 — UI Review

**Audited:** 2026-08-27
**Baseline:** `.planning/phases/06-global-ssh-options/06-UI-SPEC.md` (approved-pending), cross-checked against `cmd/gitid-dummy` (approved reference TUI) and the prior classified allowlist at `.planning/design/global-ssh/visual-divergence-allowlist.txt`
**Evidence:** real-PTY frame captures (17 files) at `.planning/phases/06-global-ssh-options/ui-frames/`, traced back to source (`internal/tuikit/globalssh.go`, `internal/tuikit/design.go`, `cmd/gitid/wiring.go`, `cmd/gitid/lifecycle.go`). No browser/screenshot tooling used — this is a Go Bubble Tea v2 TUI, per this project's standing audit method. Screenshots: not applicable (TUI, not a web app).

---

## Pillar Scores

| Pillar | Score | Key Finding |
|--------|-------|-------------|
| 1. Copywriting | 1/4 | Explanatory copy for AddKeysToAgent/UseKeychain unconditionally renders the frozen "Already set" wording even when the live probe says the option is unset — a confirmed, code-traced factual contradiction on screen |
| 2. Visuals | 3/4 | Rows are visually consistent and glyph-disciplined except the `IdentitiesOnly` not-applicable row, which renders with zero glyph (blank where every other row has one), reading as an un-rendered/broken row |
| 3. Color | 4/4 | Zero new hues/roles introduced; glyph+word always accompanies color exactly per spec; advisory stays yellow (never red), matches the contract |
| 4. Typography | 4/4 | Zero new roles; Bold/Faint/Selected usage matches the spec's role table across every captured frame |
| 5. Spacing | 4/4 | 100×30 geometry intact; 2-line option rows, sub-tab strip, ceremony viewport widths all match the spec's row budget with no overflow beyond the documented clip-cue |
| 6. Experience Design | 2/4 | The provenance/one-liner contradiction (see Copywriting) is a real state-vs-copy desync bug; shadowing (D-04), version-gating (D-13), and error/retry (commit-failure) states are otherwise well covered and honestly rendered |

**Overall: 18/24**

---

## Top 3 Priority Fixes

1. **AddKeysToAgent/UseKeychain explanation text contradicts the live state it sits next to** — user impact: a user sees `now: no → yes` (needs action) directly beside detail-pane text reading "Already set — keys stay available in the agent for the session," an outright factual contradiction that undermines trust in the whole screen's honesty (GSSH-01's entire premise is accurate provenance). Concrete fix: in `cmd/gitid/wiring.go:GlobalSSHOptionStates` (~line 1388-1394), stop unconditionally assigning `oneLiner := fixture[st.Key].OneLiner` for every key. `AddKeysToAgent` and `UseKeychain` are the only two fixture rows whose static `OneLiner` text embeds a state claim ("Already set — ..."); either give those two keys the same per-state `explanation` treatment already carried out for `IdentitiesOnly` (swap in a state-neutral explanation string when `st.State != GlobalSSHAlreadySet`), or split the fixture's `OneLiner` into a state-neutral risk explanation (used everywhere) plus the "already set" framing folded into `optionRowLine2`'s existing `GlobalSSHWordAlreadySet`/`GlobalSSHWordSafeByDefault` machinery, which already correctly derives state-appropriate row-2 text and should be the copy's only source of state claims.

2. **`IdentitiesOnly` not-applicable row renders with zero glyph** — user impact: every other option row carries a `✓`/`!`/`☑`/`☐` marker; the not-applicable row (`box = "   "`, `tone = " "` in `optionRow()`, `internal/tuikit/globalssh.go:757-767`) shows neither, so scanning the list it looks like a rendering glitch or a row that failed to load rather than a deliberate "nothing to verify here" state. Concrete fix: give `GlobalSSHNotApplicable` its own neutral glyph+word in `tone` (e.g. a dim `○` or `–` paired with the existing "not applicable" row-2 text), consistent with the project's own "glyph AND word, never blank" convention already applied to every other state on this screen.

3. **Mid-word truncation on the `IdentitiesOnly` list row reads as broken text** — user impact: `not applicable (nothing on this machi…` clips inside the word "machine," which — while technically the documented VISIBLE clip-cue behavior (`truncLine`/`ansi.Truncate`) — looks like a typo/cutoff bug rather than an intentional truncation to anyone scanning the screen quickly, since it doesn't clip at a word boundary. Concrete fix: either shorten `GlobalSSHNANothingToVerify` for the row-2 slot specifically (it already has a full, longer text used in the detail pane) or truncate at the nearest preceding space so the clip cue reads `not applicable (nothing …` instead of splitting a word.

---

## Detailed Findings

### Pillar 1: Copywriting (1/4)
- **BLOCKER**, confirmed by source trace: `cmd/gitid/wiring.go:1388-1394` — `oneLiner := fixture[st.Key].OneLiner` is assigned from the static Phase-2 dummy fixture (`internal/tuikit/design.go:217-218`) for every option key regardless of the live-computed `st.State`. `AddKeysToAgent`'s frozen `OneLiner` reads *"Already set — keys stay available in the agent for the session..."* and `UseKeychain`'s reads *"Already set — stores the key passphrase..."* — both hardcoded on the assumption (true in the frozen demo fixture, `NeedsAction: false`) that these two options are always already correctly set. The real backend's probe can (and does, per `ui-frames/global-ssh-browse.txt`) compute `NeedsAction: true` for `AddKeysToAgent` on a real machine (`now: no → yes`), yet the detail pane still shows the "Already set" wording verbatim. This is a genuine, reproducible on-screen contradiction, not a cosmetic nit — GSSH-01's whole value proposition is "explain the current state honestly," and this breaks that for the two most commonly-already-correct options.
- Only `IdentitiesOnly` got the correct treatment (`explanation = tuikit.GlobalSSHDetailExplanation` overrides the static fixture text, `wiring.go:1391-1393`) — proving the team recognized the pattern needed a per-state override for at least one key but didn't generalize it to the other two rows carrying the same defect class.
- Everything else in the Copywriting Contract checks out: provenance three-tier labels render correctly and consistently (`not set (OpenSSH default: no)`, `set by you at...` pattern implied by `AttributedToUser`), the D-12 "differs" word states (`GlobalSSHWordDiffersUser`/`Outside`) and D-11 not-applicable sentences are all present verbatim per `design.go:238-257`, and D-13's dynamic version line renders correctly and is excluded from copy-freeze per spec (`ui-frames/global-ssh-apply-cancel.txt`: `Your OpenSSH: 9.9p2 — StrictHostKeyChecking accept-new is available`).

### Pillar 2: Visuals (3/4)
- **WARNING**: `IdentitiesOnly`'s not-applicable row has no glyph at all (`optionRow()`, `internal/tuikit/globalssh.go:757-767`: `box`/`tone` both stay blank for `GlobalSSHNotApplicable`), the only row on the whole screen with no marker of any kind. Every other state (`AlreadySet`, `NeedsAction`, `Differs`, and even the applied-checkmark path) carries a `✓`/`!`/`☑`/`☐`. This breaks the screen's own visual consistency contract and reads as a missing/broken row rather than an intentional "nothing to check here" state.
- Master-detail layout, sub-tab strip, findings-banner slot, and ceremony diff/preview all render exactly as the spec's row budget describes across every captured frame (browse, apply preview/receipt, shadowed preview/receipt, storage browse/migrate). No unbudgeted rows, no frame-size drift.
- Focal point is clear: the always-visible detail pane correctly foregrounds the selected option's explanation, matching the spec's stated focal point for `options-list`.

### Pillar 3: Color (4/4)
- Zero new ANSI roles/hues confirmed by grep of `internal/tuikit/globalssh.go` against the spec's role table — `styleWarning`/`styleHealthy`/`styleSelected`/`styleFaint`/`styleInfo`/`styleReverse` are the only roles used, all pre-existing.
- Advisory states stay yellow, never red, across every captured frame including the shadowed-fix warning and the commit-failure error (`✗ gitid: refusing symlinked managed path...`, `ui-frames/global-ssh-commit-failure.txt`) — consistent with "advisory options are never a compliance gate," though the commit-failure `✗` itself is a legitimate hard error (file safety refusal), correctly distinct from the advisory taxonomy.
- D-12's "set, differs" state correctly reuses `Warning`/`!` rather than inventing a new color, per spec's explicit non-negotiable.

### Pillar 4: Typography (4/4)
- No new roles. `Bold` for key names/headings, `Faint` for `now:` lines and risk chips, `Selected`/accent for the highlighted row, `Reverse` for the active sub-tab — all confirmed present and used exactly where the spec's role table assigns them, across every captured frame.

### Pillar 5: Spacing (4/4)
- 100×30 frame geometry held in every capture; option rows are exactly 2 lines each (`optionRowLines = 2`), confirmed by counting the 6-row/12-line list body in `global-ssh-browse.txt`.
- Ceremony preview viewport (10 visible lines, scroll indicator `cols 1–94 of 123`) matches `ceremony.go:145`'s documented `ExactTextViewport`.
- Storage sub-tab's two-stacked-preview layout for Include renders within its documented `maxLines` caps (`… (+3 more lines)` / `… (+11 more lines)` clip cues present and visible, not silent).

### Pillar 6: Experience Design (2/4)
- **BLOCKER** (same root cause as Copywriting #1): the state/copy desync is as much an Experience Design failure as a copy one — a user who trusts what they read is actively misinformed about their own machine's SSH configuration.
- **WARNING**: the not-applicable row's missing glyph (Visuals #1) also degrades scanability — a user skimming by glyph alone will miss that `IdentitiesOnly` is a distinct, intentional state rather than a rendering gap.
- Positive: shadowing simulation (D-04) is well-modeled and honestly rendered in both directions — pre-write warning (`shadowed-preview.txt`: `shadow warning: StrictHostKeyChecking will be shadowed by ~/.ssh/config (line 2)`) and post-write advisory (`shadowed-receipt.txt`: `advisory: StrictHostKeyChecking was applied but is still shadowed by an external directive — the fix...`) both appear, matching D-04's "honest even when the fix does nothing" requirement.
- Positive: the inconclusive-simulation fallback (`global-ssh-probe-inconclusive-preview.txt`) and the hard-failure/retry path (`commit-failure.txt` → `commit-retry.txt`, symlink-refusal error with a `Retry (Enter)` affordance) are both modeled — advisory-not-blocking and fail-open-with-visibility are respected.
- Positive: the apply ceremony's backup-path promise (state A) and `Backed up →` receipt (state B) are both first-class and never buried, across every capture, per the "backup is the undo story" non-negotiable.
- Not independently re-verified in this pass (relying on 06-07's own accepted work, per this audit's scope instructions): the DLV-4 fixture-vs-live classification set and the two already-accepted deferrals WR-02/WR-16.

---

## Registry Safety

Not applicable — no `components.json` / shadcn in this stack (Go Bubble Tea v2 TUI). Per the UI-SPEC's own Registry Safety section: "not applicable — no component registry in this stack."

---

## Files Audited

- `.planning/phases/06-global-ssh-options/06-UI-SPEC.md` (design contract)
- `.planning/phases/06-global-ssh-options/06-CONTEXT.md` (D-01..D-16 decisions)
- `.planning/phases/06-global-ssh-options/06-0{1..7}-SUMMARY.md` / `06-0{1..7}-PLAN.md`
- `.planning/design/global-ssh/visual-divergence-allowlist.txt` (prior classified divergence record, spot-checked not re-derived)
- `.planning/phases/06-global-ssh-options/ui-frames/*.txt` (17 real-PTY frame captures — primary visual evidence)
- `internal/tuikit/globalssh.go` (render logic — `optionRow`, `optionRowLine2`, `notApplicableSentence`, ceremony wiring)
- `internal/tuikit/design.go` (frozen `GlobalSSHOptions` fixture, D-10/D-11/D-12 word constants)
- `internal/tuikit/views.go` (`GlobalSSHOptionView` DTO, `Selectable()`)
- `cmd/gitid/wiring.go` (`GlobalSSHOptionStates` — real-backend DTO construction, the source of the confirmed Copywriting defect)
- `cmd/gitid/lifecycle.go` (shadow-warning / advisory message formatting)

---

## Resolution

Both findings fixed by the orchestrator immediately after this audit, independently re-verified (build, `-race` unit suite, `make gate-visual-regression`, full `make test-e2e`), and visually confirmed in the regenerated real-PTY frames.

1. **Copywriting/Experience Design BLOCKER — state/copy desync.** `internal/tuikit/design.go`: stripped the baked-in "Already set — " state claim from `AddKeysToAgent`'s and `UseKeychain`'s `OneLiner` fixture strings, matching every other option's already state-neutral phrasing (they describe risk/rationale, never assert a current-state fact). The live "already set" vs. "needs action" distinction is already communicated elsewhere on the row (the tone glyph and the `now: X → Y` line), so the detail-pane text no longer needs to — and must not — also assert it. Both the real backend and the dummy fixture backend read the same `GlobalSSHOptions` table, so the fix is consistent across both surfaces with no new real-vs-dummy divergence and no allowlist change required.
2. **Visuals WARNING — zero-glyph not-applicable row.** `internal/tuikit/globalssh.go`'s `optionRow`: added a `styleFaint.Render("·")` tone marker for `GlobalSSHNotApplicable`, reusing the row's existing faint styling role (already used elsewhere on the same row) rather than introducing a new health-tone glyph or color — D-12's "existing glyph, existing theme role, no new health state" constraint stays intact, since `·` is a neutral placeholder, not a fourth member of the ✓/!/✗ health vocabulary.

The third item (mid-word clip on the not-applicable reason text) was left as-is: it is the plan's own documented clip-cue behavior, explicitly flagged by the audit as "minor," not a defect.
