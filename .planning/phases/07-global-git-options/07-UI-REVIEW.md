# Phase 07 — UI Review

**RESOLVED (2026-08-27, same session):** Both findings below were fixed
immediately after this report was generated. (1) The checkbox-blank-cell
defect (Pillar 2/6, Top Fix #1-2): `internal/tuikit/globalgit.go`'s row-render
loop now renders `styleFaint.Render("·") + "  "` for a non-selectable row's
checkbox column instead of three blank spaces, exactly the fix this report
recommended — proven by a new regression test
(`TestGlobalGitNonSelectableRowCheckboxNeverBlank`) confirmed to fail without
the fix and pass with it. (2) The low-confidence "stale Baseline applied
status line" observation (Top Fix #3) was investigated and CONFIRMED as a
real, separate bug, unrelated to test-sequencing leftover: `view()`'s default
`status` string is computed from a `pending` count derived from `options`,
which `activate()`'s error path leaves empty on a probe failure — so
`pending == 0` vacuously and the misleading "Baseline applied" text rendered
even though nothing was applied. Fixed by leaving `status` blank on the
`optionsErr != ""` branch instead of falling through to the outer default,
proven by a second new regression test
(`TestGlobalGitProbeErrorDoesNotClaimBaselineApplied`). `make gate-visual-regression`
and the full `internal/tuikit` package suite pass with both fixes applied.
The rest of this report (Pillars 1, 3, 4, 5, and the majority of Pillar 6)
remains accurate as originally recorded below.

**Audited:** 2026-08-27
**Baseline:** `.planning/phases/07-global-git-options/07-UI-SPEC.md`, cross-checked against `cmd/gitid-dummy` (approved reference TUI) and the classified allowlist at `.planning/design/global-git/visual-divergence-allowlist.txt`
**Evidence:** real-PTY frame captures (14 files) at `.planning/phases/07-global-git-options/ui-frames/`, traced back to source (`internal/tuikit/globalgit.go`, `internal/tuikit/views.go`, `internal/tuikit/design.go`). No browser/screenshot tooling used — this is a Go Bubble Tea v2 TUI, per this project's standing audit method (07-UI-SPEC.md, `ONESHOT.md` Non-Negotiable Rule 8). Screenshots: not applicable (TUI, not a web app).

The in-process screenshot-registry evidence (`review-packet/MANIFEST.md` Part A/B, `visual-divergence-allowlist.txt`) is treated as render-parity evidence only, never substituted for real-PTY terminal-decoding evidence, per the packet's own evidence-class discipline. The finding below was independently confirmed by byte-level inspection of two real PTY frames plus a source-code trace — it is not present in, and could not have been caught by, the fixture-vs-live allowlist (both real and dummy rows in `design.go`/`globalgit.go` share the same rendering function, so this is a same-surface bug, not a real-vs-dummy divergence).

---

## Pillar Scores

| Pillar | Score | Key Finding |
|--------|-------|-------------|
| 1. Copywriting | 4/4 | All 12 option one-liners, the D-03 provenance three-tier labels, the D-02 "set, differs" word, the D-07 cross-warning, and the D-08 version-gate note render exactly as frozen, verified across 8+ frames |
| 2. Visuals | 2/4 | A confirmed, code-traced defect: option rows in the `GlobalGitSetButDiffers` state (and the D9 fallback row) render a fully blank checkbox cell instead of the neutral/absent-marker treatment Phase 6 established for this exact defect class — breaks the "every row: checkbox + tone glyph + name" template promised by the spec |
| 3. Color | 4/4 | Zero new hues/roles; advisory stays yellow (never red); D9's `Error`/red role confirmed used only for the `needs @` inline validation |
| 4. Typography | 4/4 | Zero new roles; Bold/Faint/Selected/Warning/Healthy usage matches the spec's role table across every captured frame |
| 5. Spacing | 4/4 | 100×30 geometry intact in every frame; 12-row list (24 body lines) fits the measured 24–25-line budget with no scroll cue needed, matching the phase's own re-measured finding; ceremony preview widened correctly for the 43-line managed block with no sentinel truncation |
| 6. Experience Design | 3/4 | Probe-failure state (advisory, nav stays live), mid-transaction failure+retry, and the D-07 cross-warning are all honestly and correctly rendered; the checkbox-defect (Visuals #1) also degrades scanability of the differs-row's actual state |

**Overall: 21/24**

---

## Top 3 Priority Fixes

1. **The "set, differs from recommendation" row's checkbox cell renders as three blank spaces, not a neutral marker** — user impact: every other row on this screen carries a `☐`/`☑` checkbox in a fixed column position; scanning the list, the differs-row (e.g. `init.defaultBranch` when the user already has `trunk`/`master` set) reads as a rendering glitch or an unloaded row rather than a deliberate "this row can't be applied" state — the exact same defect class Phase 6's own audit flagged (BLOCKER) and fixed for its `IdentitiesOnly` not-applicable row's tone glyph. Concrete fix: in `internal/tuikit/globalgit.go`'s row-render loop (~line 913-922), when `!o.Selectable()`, render a faint neutral placeholder in the checkbox slot (e.g. `styleFaint.Render("· ")`, matching this same file's own precedent for the tone-glyph slot at line 927-931) instead of `box := "   "`. This affects both the D-02 differs state and the D9 fallback row (`user.email (global fallback)`, line 909) — though the latter is a documented, spec-approved checkbox removal (07-UI-SPEC.md: "D-04 already removed the checkbox"), so the fix should be scoped to non-selectable *option* rows only, not the D9 field row, to avoid reintroducing a checkbox the spec explicitly says should not exist there.

2. **Root cause verified by direct frame comparison, not assumption** — confirmed via `python3` byte inspection: `global-git-empty-selection.txt` line 3 (`init.defaultBranch`, needs-action state, value unset) shows `▸ ☐ ! init.defaultBranch`; `global-git-browse.txt`/`global-git-differs.txt` line 3 (same row, differs state, value `trunk`/`master` set) shows `▸    ! init.defaultBranch` — checkbox glyph fully absent. Source trace (`globalgit.go:913-922`, comment: "only selectable rows get a checkbox glyph") confirms this is deliberate-but-visually-unfinished code, not a rendering accident — the intent (don't let a user "select" an unapplyable row) is correct, but the empty-cell execution is the same visual mistake Phase 6's checker already caught once in this codebase. This is a regression of a known, already-fixed defect class, which raises its severity above a fresh first-time finding.

3. **Minor/unconfirmed — probe-failure frame's footer status line appears stale** — `global-git-probe-failure.txt` line 28 reads `Baseline applied. user.email stays untouched — identities own their author.` (an apply-success message) underneath a probe-failure body (`! git probe failed: ...`). This is very likely test-sequencing leftover (the PTY test probably applies something before triggering the probe failure in the same session) rather than a genuine bug, and was not independently re-run to confirm — flagged as a low-confidence, low-priority item for the next PTY pass to verify the footer clears/updates correctly on a probe failure that occurs after a successful apply in the same session, rather than to block on it now.

---

## Detailed Findings

### Pillar 1: Copywriting (4/4)
- `global-git-browse.txt`/`differs.txt`: the `init.defaultBranch` detail pane renders the frozen historical explanation verbatim, plus the D-08 dynamic version line pattern is confirmed present on `diff.colorMoved` in `global-git-scroll-down.txt` (`Your git: 2.55.0 — diff.colorMoved zebra is available`) — non-contractual dynamic text correctly excluded from the copy-freeze per the D-13 precedent, matching `make gate-copy-freeze`'s green result recorded in 07-04/07-06 SUMMARY.md.
- D-03 provenance three-tier labels confirmed present: `not set (git's built-in default: no)` visible in `global-git-scroll-down.txt` line 13.
- D-02 "set, differs" word confirmed present verbatim (truncated to `"set, differs from …"` by the row's fixed column width, matching the project's own documented truncation-test workaround per 07-04-SUMMARY.md D-07-04-2 — an accepted, tested truncation, not a defect).
- D-07 cross-warning copy confirmed byte-matching the frozen string in `global-git-cross-warning.txt`: `user.useConfigOnly is selected but the fallback author has no name set — a commit with no matching identity will hard-fail instead of falling back, because only the email half is configured.`
- D-08 version-gate note confirmed present and correctly conditional in `global-git-version-gate.txt`: `advisory: merge.conflictstyle is below the git version gate — wrote "diff3" instead of "zdiff3"` plus the effective-value follow-up advisory — both frozen strings render together, matching the spec's "higher-priority than the other two informational version lines" instruction.
- Result-message template confirmed present in `global-git-fallback-set.txt`/`version-gate.txt`, correct `N of M` counts observed (`1 of 11`, includes the D-07 row per the D-10 tally as implemented).

### Pillar 2: Visuals (2/4)
- **BLOCKER**, confirmed by direct frame byte-comparison plus source trace: the `GlobalGitSetButDiffers` state's checkbox cell renders as `"   "` (three literal spaces) rather than any visible marker (`globalgit.go:916`, gated by `o.Selectable()` at line 917, which explicitly excludes `GlobalGitSetButDiffers` per the state list at `views.go:463-468`). Every other row on the same screen — including the `GlobalGitNotApplicable` state, which correctly received a neutral `·` tone-glyph treatment (line 927-931) — carries some visible marker in every column. Only the checkbox column for non-selectable rows was left blank, reintroducing exactly the "zero-glyph reads as broken row" defect Phase 6's own UI-REVIEW.md flagged and fixed for a different glyph slot on a sibling screen just one phase earlier in this project's history.
- Master-detail layout, D9 two-field pane, ceremony preview/receipt, and cross-warning banner all render exactly per the spec's row budget across every captured frame — no unbudgeted rows, no frame-size drift, confirmed in `global-git-browse.txt`, `global-git-cross-warning.txt`, `global-git-version-gate.txt`.
- Focal point is clear: the always-visible detail pane correctly foregrounds the selected option's explanation across all 12(+) rows, matching the spec's stated `options-list` focal point.
- The `[main vs master]` highlight chip renders correctly and only on `init.defaultBranch`, confirmed across every frame containing that row.

### Pillar 3: Color (4/4)
- Zero new ANSI roles/hues confirmed by source read of `internal/tuikit/globalgit.go` against the spec's role table — `styleWarning`/`styleHealthy`/`styleSelected`/`styleFaint`/`styleInfo`/`styleError` are the only roles used, all pre-existing.
- Advisory states stay yellow, never red, across every captured frame including the D-07 cross-warning and the D-08 version-gate advisory (`global-git-cross-warning.txt`, `global-git-version-gate.txt`).
- `styleError`/red confirmed scoped to the D9 `needs @` inline validation only (per spec's explicit "the ONE place in this screen a compliance-style red is correct") — not observed misused elsewhere in any captured frame.
- Mid-transaction failure (`global-git-mid-transaction-failure.txt`) uses `✗` for the hard file-safety refusal, correctly distinct from the advisory taxonomy, matching the project's established pattern from Phase 6.

### Pillar 4: Typography (4/4)
- No new roles. `Bold` for key names/headings, `Faint` for `now:` lines and the ceremony preview text, `Selected` for the highlighted row, `Warning`/`Healthy` glyphs for tone — all confirmed present and used exactly where the spec's role table assigns them, across every captured frame.

### Pillar 5: Spacing (4/4)
- 100×30 frame geometry held in every capture; option rows are exactly 2 lines each (`optionRowLines`), confirmed by counting the 12-row/24-line list body in `global-git-browse.txt`.
- The phase's own re-measurement (07-06-SUMMARY.md, MANIFEST.md D.4) that the 12-row list never scrolls at the real 100×30 geometry (24 body lines fits the 24-25-line budget) is independently corroborated: `global-git-scroll-down.txt` and `global-git-scroll-up.txt` both show a full, un-cued row set — no `↓`/`↑ (+N more)` cue rendered in either direction, consistent with the documented "EQUAL, not a divergence" finding.
- Ceremony preview widened correctly for the longer 43-line managed block (`R-07-04-VIEWPORT`) — `global-git-cross-warning.txt`'s preview shows the full `# BEGIN ... # END` sentinel pair without truncation, confirming the fix scoped to Global Git only (Global SSH's own ceremony budget is separately pinned unchanged by `TestGlobalSSHCeremonyPreviewUsesUnchangedDefaultBudget`).

### Pillar 6: Experience Design (3/4)
- **WARNING** (same root cause as Visuals #1): the checkbox-blank defect degrades scanability — a user skimming by checkbox alone could miss that the differs-row is a distinct, intentional "you already diverged, this is informational" state rather than a rendering gap or an accidentally-unselectable healthy row.
- Positive: probe-failure state is well-modeled and honestly rendered — `global-git-probe-failure.txt` shows the `!`-styled inline error plus the advisory sentence in place of the option rows, and the footer hotkey row confirms navigation stays live (`↑↓ select option · ←→ switch view`, no dead-end), matching the spec's fail-open contract exactly.
- Positive: the mid-transaction failure + retry sequence (`global-git-mid-transaction-failure.txt` → `global-git-mid-transaction-retry.txt`) is modeled and honestly rendered, mirroring the SSH-side's existing symlink-refusal precedent — a real stat-based `mutationJournal.watchFile` rejection, not a test-only hook, per 07-04-SUMMARY.md D-07-04-3.
- Positive: the D-07 cross-warning renders BEFORE the ceremony commits, inside the existing preview/note slot, exactly as the spec requires ("must render HONESTLY when applicable — never a silent 'applied' outcome that hides a hard-fail risk").
- Positive: the apply ceremony's backup-path promise (state A, `global-git-cross-warning.txt`: `Backup → ~/.gitconfig.bak.<timestamp> (written first — restore it to undo)`) and the state-B `Backed up →` receipt (`global-git-version-gate.txt`, `global-git-fallback-set.txt`) are both first-class and never buried, matching the "backup is the undo story" non-negotiable.
- **Low-confidence, unconfirmed observation**: `global-git-probe-failure.txt`'s footer status line (`Baseline applied. user.email stays untouched...`) appears to be a stale success message left over from a prior action in the same PTY session rather than a probe-failure-appropriate footer — plausible test-sequencing artifact, not independently re-verified (see Top 3 Fix #3).
- Not independently re-verified in this pass (relying on the phase's own accepted work, per this audit's scope): the D-04 two-field empty-means-unset semantics' exact visual unambiguity claim, and the D-10 apply-count tally rule's full correctness across every partial-selection combination — both are covered by unit/e2e tests cited in 07-02/07-05-SUMMARY.md but not independently re-derived from PTY frames here.

---

## Registry Safety

Not applicable — no `components.json` / shadcn in this stack (Go Bubble Tea v2 TUI). Per the UI-SPEC's own Registry Safety section: "not applicable — no component registry in this stack."

---

## Files Audited

- `.planning/phases/07-global-git-options/07-UI-SPEC.md` (design contract)
- `.planning/phases/07-global-git-options/07-CONTEXT.md`, `07-DISCUSSION-LOG.md` (decisions, spot-checked)
- `.planning/phases/07-global-git-options/07-0{1..6}-SUMMARY.md` / `07-0{1..6}-PLAN.md`
- `.planning/design/global-git/visual-divergence-allowlist.txt` (classified real-vs-dummy divergence record)
- `.planning/phases/07-global-git-options/review-packet/MANIFEST.md` (evidence-class-labeled review packet)
- `.planning/phases/07-global-git-options/ui-frames/*.txt` (14 real-PTY frame captures — primary visual evidence; all 14 read, 9 quoted directly above)
- `internal/tuikit/globalgit.go` (render logic — row loop, checkbox/tone-glyph assembly, ceremony wiring)
- `internal/tuikit/views.go` (`GlobalGitOptionView.Selectable()`, state enum)
- `internal/tuikit/design.go` (frozen `GlobalGitOptions` fixture — spot-checked, not re-derived)

---

## Independent verification performed

Ran, in the working tree, to corroborate the SUMMARY files' own claims rather than accept them uncritically:

```
$ python3 -c "byte-level comparison of the checkbox column across 3 real PTY frames"
global-git-empty-selection.txt: ' ▸ ☐ ! init.defaultBranch  [main vs mast'   (checkbox present)
global-git-browse.txt:          ' ▸    ! init.defaultBranch  [main vs mas'   (checkbox ABSENT)
global-git-scroll-down.txt:     '   ☐ ! init.defaultBranch  [main vs mast'   (checkbox present, unselected, needs-action state)
```

This is genuine new evidence produced by this audit, not merely re-stated from the SUMMARY/MANIFEST files, which do not mention the checkbox-blank defect anywhere in their Deviations/Review sections or the 38-row closure table.
