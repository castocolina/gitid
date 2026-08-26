# Phase 5 — UI Review

**Audited:** 2026-08-26
**Baseline:** `.planning/phases/05-identity-manager/05-UI-SPEC.md` (FROZEN, derivation of `identity-manager/FIELDS.md`), cross-checked against `cmd/gitid-dummy`'s live mockup per project policy (`AGENTS.md` "UI Reference").
**Method:** Real-PTY evidence — captured frames from the compiled `gitid` binary under `tmp/ui-frames/identity-manager-*.txt` (produced by the project's own `e2e/identity_manager_pty_e2e_test.go`-style harness; `make test-e2e` already passed independently per task brief), read verbatim; source cross-reference via `codegraph`/`rg` on `internal/tuikit/identities.go`, `internal/tuikit/ceremony.go`, `internal/tuikit/frame.go`, `internal/identity/scan.go`. No fabricated/re-run screenshots — audited the existing evidence trail plus source.
**Screenshots:** N/A (TUI, not a web app) — real-PTY terminal-frame captures used instead, per project's UI-reference policy.

Scope covered: action menu, rotate ceremony (incl. CR-03's now-honest not-tested disclosure), repair ceremony, delete (git-only and everything-scope, incl. typed-name confirm and D-13 unmanaged-reference scan), list-empty, detail-ssh-first. `05-REVIEW-FIX.md`'s 18 fixed findings (CR-01..05, WR-01..13) were read and folded into this audit as "already-fixed" context, not re-litigated.

---

## Pillar Scores

| Pillar | Score | Key Finding |
|--------|-------|-------------|
| 1. Copywriting | 1/4 | Real production status bar literally says "every action is dummy" — leftover mockup copy shipped to users |
| 2. Visuals | 2/4 | Delete-everything result screen's backup/write list bleeds past the 25-row body budget into the footer/status area |
| 3. Color | 4/4 | Role-based ANSI-16 usage matches Theme exactly; no hardcoded hex; D-13 warning correctly wired |
| 4. Typography | 4/4 | Consistent `styleBold`/`styleFaint`/`styleHealthy`/`styleWarning`/`styleError` role usage, no ad hoc styling |
| 5. Spacing | 1/4 | Ceremony receipt view (`ceremony.go:view`) has NO `maxLines`/clip-cue on the `Wrote →`/`Backed up →` lists, contradicting the UI-SPEC's explicit backup-notice budget contract |
| 6. Experience Design | 3/4 | CR-03's honest not-tested disclosure genuinely improved trustworthiness; but the unbounded receipt list undermines "the backup is the undo story" exactly where the spec calls that out as the primary focal point |

**Overall: 15/24**

---

## Top 3 Priority Fixes

1. **Leftover mockup copy in production status bar** — `internal/tuikit/identities.go:4327` renders `"%d identities — selection renders the detail live; every action is dummy but really changes this state."` on every Identities list/detail screen of the REAL compiled binary. User impact: every session, the user reads that their real, file-mutating actions are "dummy" — directly contradicts the product's own safety story (backups, confirms, real writes) and looks unshipped/unprofessional. Concrete fix: replace with real-behavior status copy, e.g. `"%d identities · ↑↓ select · every change here writes and backs up your real config."` (or shorter, following `02-STYLE-SPEC.md` §6's copy-freeze conventions), and add the exact string to the copy-freeze gate (`make gate-copy-freeze`) so it cannot regress silently the way WR-09 already had to catch once this phase.

2. **Unbounded receipt list overflows the 100×30 frame** — `internal/tuikit/ceremony.go:288-309`'s `view()` writes every `Targets`/`Backups` entry with no `fitPane`/`PreviewBlock` `maxLines` cap, unlike every other stacked preview in the codebase (`frame.go`'s `fitPane`/`PreviewBlock`/`previewBlockClipped` all exist and are used elsewhere). Confirmed in the real captured frame `tmp/ui-frames/identity-manager-delete-everything-clean.txt`: the `Wrote →`/`Backed up →` list runs past row 27 and is hard-cut mid-line by the terminal region right where the footer/status text should start ("Backed up → ~/.ssh/gitid-" is truncated with no `… (+n more)` cue, and the "Done (Enter)" line is pushed off-frame entirely in that capture). This directly violates the UI-SPEC's "Preview block ... `maxLines` cap + `… (+n more)` clip cue" contract and its explicit statement that `backup-notice`'s combined path list "reuses the EXISTING clip-cue/`maxLines` mechanism ... never grows the frame." It is also the exact screen the spec names as the primary focal point for "the backup is the undo story" — an overflowing, silently-truncated backup list is the worst place for this gap to exist. Concrete fix: route `ceremony.go`'s done-state `Targets`/`Backups` rendering through `fitPane` (or `PreviewBlock`) with a `maxLines` budget sized to the remaining body rows, exactly like Phase 4's D-10 precedent the spec cites.

3. **Footer keybar truncates mid-word with no indication of hidden actions** — `internal/tuikit/frame.go:296`'s `renderFooterLine` does `ansi.Truncate(..., width, "…")` on the whole joined action string. Observed in real frames: `"↑↓ select identity · n new · e edit SSH · g configure Git · c clone · d delete · a actions · ←→ sw…"` — cuts "switch tabs" mid-word to "sw…", and it's not clear from the glyph whether `←→` is still a reachable/clickable action past the visible truncation point (project convention: "the ENTIRE rendered field/row is the mouse hit target" — a truncated footer entry breaks that guarantee for the last item). Concrete fix: either drop lowest-priority footer actions before truncating (prioritized hint list, common TUI pattern) so the visible text never truncates mid-label, or shorten the "switch tabs" label so it fits at 100 cols with the existing action set.

---

## Detailed Findings

### Pillar 1: Copywriting (1/4)

- **BLOCKER** — `internal/tuikit/identities.go:4327`: the status line rendered on every Identities list/detail screen (`list-populated`, `detail-ssh-first`, `list-empty`) says `"... every action is dummy but really changes this state."` This is a direct carry-over of `cmd/gitid-dummy` mockup phrasing (the demo app's own self-description) into the real, backend-wired product built in this phase. Confirmed present in `tmp/ui-frames/identity-manager-list-empty.txt` (line 28), `tmp/ui-frames/identity-manager-detail-ssh-first.txt` (line 28), and `tmp/ui-frames/identity-manager-delete-git-only-post-restart.txt` (line 28) — i.e. it survived the entire Phase 5 build and the 18-finding code-review-fix pass without being caught. Classification: **defect**, not an intentional UX choice — nothing in `05-UI-SPEC.md`'s Copywriting Contract table authorizes this string, and it directly contradicts the phase's own stated purpose (wiring the real backend behind the manager).
- **GOOD** — the frozen/DRAFT copy strings that ARE specified (`FIELDS.md`'s `No identities yet` / `Press n to create your first identity`, `delete-choice`'s two options, `confirm-destructive`'s heading) all render byte-for-byte as specified in the captured frames — no drift found there.
- **GOOD** — CR-03's fix (`keyCeremonyNotTestedDetail`) replaces a previously fabricated "Stage N test passed." string with an honest disclosure. This is a genuine copywriting improvement over what Phase 5 originally shipped, caught by code review before this audit.
- One BLOCKER-severity finding is enough to floor this pillar at 1/4 — it is not a cosmetic nit, it is user-facing, permanent (not test-only), and actively misleads about data-safety behavior.

### Pillar 2: Visuals (2/4)

- **BLOCKER** (shared with Spacing pillar below — visual consequence of the same root cause) — the delete-everything result screen's content overflow breaks the frame's visual integrity: in `tmp/ui-frames/identity-manager-delete-everything-clean.txt`, path text wraps character-by-character across multiple terminal rows (`TestIdentityM` / `anager_...` split mid-identifier), consuming the entire 25-row body budget and pushing the "Done (Enter)" CTA off-frame in that capture. A user cannot see whether the write succeeded or reach the confirm action without knowing to scroll/resize (not offered — Bubble Tea PTY app, no scroll affordance shown here).
- **GOOD** — master–detail archetype (list ~⅓ left / detail ~⅔ right) is implemented consistently across all captured states (`action-menu`, `detail-ssh-first`, `list-empty`), matching the UI-SPEC's single-archetype mandate.
- **GOOD** — focal point discipline holds where content fits: `detail-ssh-first` clearly leads with SSH block, `confirm_warning`-style destructive framing is legible in the delete-git-only-result captures.
- **GOOD** — glyph+word pairing (`✓`/`!`/`✗` + state word) present in every captured row (`▸ ! work 1⚑`, `▸ ✓ acme`), matching the NO_COLOR-legibility non-negotiable.
- No icon-only controls found without an accompanying word/label in the audited states.

### Pillar 3: Color (4/4)

- `rg -o "style[A-Za-z]*"` over `identities.go` returns only theme-role styles (`styleBold`, `styleError`, `styleFaint`, `styleFocusLink`, `styleHealthy`, `styleInfo`, `stylePipNone`, `styleSelected`, `styleStepperActive`, `styleWarning`) — zero ad hoc `lipgloss.Color(...)` literals, zero hex/RGB values found in `identities.go` or `theme.go`.
- D-13's unmanaged-reference scan warning is correctly wired: `internal/tuikit/design.go:170` defines `DeleteScanHitFmt` and `internal/identity/scan.go`'s `ScanUnmanagedReferences` computes real hits (`internal/identity/deleteplan.go:125`), consumed via `styleWarning` — matches the UI-SPEC's "Warning (yellow), advisory-never-blocking" color assignment; not colored `Error` (would have been wrong per spec's explicit note distinguishing D-12's `Hint` framing from D-13's `Warning` framing).
- Accent usage restricted to focus/active-nav roles per grep — no evidence of the 8-state row taxonomy being colored directly (spec requires glyph+word to carry meaning, color additive only); captured frames confirm this (row color absent from the plain-text dump, but glyph+word present, consistent with correct NO_COLOR-legible behavior).

### Pillar 4: Typography (4/4)

- Same style-role audit as Color pillar: `styleBold` for labels/headers, `styleFaint` for hints/blurred fields, `styleHealthy`/`styleWarning`/`styleError` for state — matches the UI-SPEC's Typography table role-for-role, zero new roles introduced.
- `internal/tuikit/identities.go:1493` (`keyCeremonyNotTestedDetail`) and its `renderStageNotTested` helper (line 1523) render through the same `Hint`/faint styling as every other advisory line, per spec.
- No evidence of size/weight drift — this is a TUI, "size" is not applicable; weight (`Bold`/`Faint`/plain) usage is consistent across all captured states.

### Pillar 5: Spacing (1/4)

- **BLOCKER** — `internal/tuikit/ceremony.go:288-309` (`ceremonyModel.view`, done-state branch): iterates `c.cfg.Targets` and `c.cfg.Backups` with a bare `for` loop and unconditional `b.WriteString`, with **no** `fitPane` or `PreviewBlock`/`previewBlockClipped` call — the exact clip-cue mechanisms that exist in `frame.go` (`fitPane` at line 160, `PreviewBlock` at line 476, `previewBlockClipped` at line 501) and that the UI-SPEC cites explicitly for this exact screen: "`backup-notice`'s combined path list ... reuses the EXISTING clip-cue/`maxLines` mechanism every stacked `PreviewBlock` in this project already uses (Phase 4 D-10 precedent) — never grows the frame." This is a direct, provable contract violation, not a judgment call.
- Confirmed via real capture, not just static reasoning: `tmp/ui-frames/identity-manager-delete-everything-clean.txt` shows the list overflowing rows 6-27 (21 of the 25 available body rows consumed by 6 target/backup lines whose long temp-dir paths wrap character-by-character with no width-aware truncation), with the trailing backup entries and the `Done (Enter)` CTA cut off the visible frame. A shorter real `$HOME`-relative path (as in `identity-manager-key-ceremony-rotate.txt`) fits fine — meaning this is a **live, reachable defect** triggered by realistic conditions (long `TMPDIR`, `~/git/<name>/` under a deep path, multiple backups from a delete-everything with archived key + key-pair backup, which D-11 explicitly adds as a THIRD backup line on top of the existing two), not a synthetic edge case.
- **GOOD** — every OTHER screen audited (action-menu, detail-ssh-first, list-empty) respects the fixed 25-row body budget cleanly with consistent blank-row padding below content, matching the spec's "no reflowing card" / "fixed row" conventions.
- This finding alone floors the pillar at 1/4: it is the exact spacing contract (`maxLines` cap + clip-cue) the spec calls out by name for this exact screen, and it is demonstrably unimplemented.

### Pillar 6: Experience Design (3/4)

- **GOOD, verified improvement** — CR-03's fix is real and effective: `keyCeremonyNotTestedDetail` (identities.go:1493) and `renderStageNotTested` (line 1523) replace what was a fabricated `TestOutcomePass` result with an honest "Not tested — no connectivity probe runs before this write" disclosure, and the code review's regression test (`TestKeyCeremonyStagesDoNotFabricateAPassResult`) proves it. This is a meaningful trust-preserving fix that this audit independently confirms is wired into the render path (not just present in a test).
- **GOOD** — async ceremony states are all covered: pending (`"Writing…"` + `"Will write → "` preview), retryable failure (`commitErr` branch with Retry/Cancel), and the done/receipt state — matches the four-beat mutation ceremony (preview → confirm → backup notice → result) the spec mandates, with no evidence of a missing beat.
- **GOOD** — destructive-action safety: delete-everything requires the typed identity name (WR-01 fix, confirmed via `05-REVIEW-FIX.md`), and `delete-choice`'s safer git-only option is default-focused per the captured `identity-manager-delete-git-only-result.txt`/`identity-manager-delete-everything-clean.txt` frames (`Tab/←→ Cancel / Confirm` shown, matching the "destructive actions never default-focus yes" non-negotiable).
- **WARNING** — the same overflow defect from the Spacing pillar directly degrades this pillar's "backup is the undo story" primary focal point: when the receipt list is long, the user cannot actually read/verify the backup paths that are supposed to be their undo mechanism. The spec explicitly names this screen's primary focus requirement, and the implementation doesn't meet it under realistic conditions.
- **WARNING** — footer keybar mid-word truncation (`renderFooterLine`, frame.go:296) obscures which actions are reachable when many `FooterAction`s are present (observed 8 actions on the `list-populated` footer, truncated to `"...sw…"`), a minor but real usability papercut, not scored as a blocker since the primary actions (arrows, n/e/g/c/d/a) all fit before the truncation point in every captured frame.
- Score held to 3/4 rather than 2/4 because the CR-03 fix demonstrates the team is actively closing UX-honesty gaps, and the core ceremony flow (preview/confirm/backup/result, retry-on-failure) is sound — the deduction is for the one concrete overflow defect that undermines a spec-named focal point, not a systemic gap in state coverage.

---

## Registry Safety

Not applicable — Go Bubble Tea v2 TUI, no shadcn/npm component registry (`components.json` absent, and `05-UI-SPEC.md`'s own Registry Safety table states "not applicable" for this stack).

---

## Files Audited

- `internal/tuikit/identities.go` (status bar, action-menu, detail-ssh-first, delete-choice, CR-03 not-tested rendering)
- `internal/tuikit/ceremony.go` (four-beat ceremony view, receipt/backup-list rendering — root cause of the Spacing/Visuals BLOCKER)
- `internal/tuikit/frame.go` (`fitPane`, `PreviewBlock`, `previewBlockClipped`, `renderFooterLine` — clip-cue mechanisms that exist but aren't applied to the ceremony receipt)
- `internal/tuikit/design.go` (D-13 `DeleteScanHitFmt`)
- `internal/identity/scan.go`, `internal/identity/deleteplan.go` (D-13 unmanaged-reference scan wiring)
- `.planning/phases/05-identity-manager/05-UI-SPEC.md` (audit baseline)
- `.planning/phases/05-identity-manager/05-REVIEW-FIX.md` (18 already-fixed findings, folded in as context)
- Real-PTY capture evidence: `tmp/ui-frames/identity-manager-action-menu.txt`, `identity-manager-key-ceremony-rotate.txt`, `identity-manager-key-ceremony-repair.txt`, `identity-manager-list-empty.txt`, `identity-manager-detail-ssh-first.txt`, `identity-manager-delete-git-only-post-restart.txt`, `identity-manager-delete-everything-clean.txt`, `identity-manager-delete-everything-planted-hits.txt`
