# Phase 4 — UI Review

**Audited:** 2026-08-25
**Baseline:** `.planning/phases/04-git-configuration-screen/04-UI-SPEC.md` (approved design contract, D-01..D-12 scoped divergences)
**Screenshots:** not applicable (TUI, not a web surface) — audited real PTY-captured terminal frames instead:
- `.planning/phases/05.7-complete-v1-0-product-features-in-tui/ui-frames/git-configuration-complete-edit-result.txt`
- `.planning/phases/05.7-complete-v1-0-product-features-in-tui/ui-frames/git-configuration-mouse-field-focus.txt`
- `.planning/phases/05.7-complete-v1-0-product-features-in-tui/ui-frames/git-configuration-ssh-only-completion-result.txt`
- `.planning/phases/05.7-complete-v1-0-product-features-in-tui/ui-frames/git-configuration-write-failure-rollback.txt`

Per this project's binding rule (CLAUDE.md / `.planning/ONESHOT.md` rule 8), the compiled real binary exercised through a real PTY against the approved `cmd/gitid-dummy` mockup is the authoritative acceptance evidence — no browser/Playwright/HTML parity applies here. The visual-regression divergence classification file (`.planning/design/git-screen/visual-divergence-allowlist.txt`) was read and its 8 classified real-vs-dummy divergences (all `ux-improvement`, none `defect`) are treated as already-resolved and are NOT re-litigated below — this audit looks for defects the allowlist and the DLV-04/DLV-06 gates do not cover: raw UI quality against the 6 pillars, independent of dummy-vs-real parity.

---

## Pillar Scores

| Pillar | Score | Key Finding |
|--------|-------|-------------|
| 1. Copywriting | 3/4 | D-11 failure message renders the raw Go error string, not the documented `Writing <target> failed: <error>` pattern |
| 2. Visuals | 2/4 | Absolute temp-dir filesystem paths bleed into the UI un-shortened, wrapping mid-word across 2-3 lines — illegible and visually inconsistent with the `~`-shorthand used elsewhere on the same screen |
| 3. Color | 3/4 | Role usage (✓ green success, ✗ red error/broken) is consistent with the ANSI-16 contract in all 4 captured frames; no violation observed in the text evidence available |
| 4. Typography | 3/4 | Glyph+word pairing and bold-label conventions hold throughout; no unpaired color-only state found |
| 5. Spacing | 2/4 | The backup-receipt / restored-file list has no `maxLines` cap or clip-cue, unlike every other preview surface the spec mandates — it grows unbounded with target count and already spans ~19 of 25 body rows for a 4-target edit |
| 6. Experience Design | 3/4 | Strong state coverage (loading via async ceremony, error/rollback, SSH-only completion, mouse focus, edit result) verified through 4 real-PTY captures + `e2e/git_configuration_pty_e2e_test.go`; failure path shows the raw error with no reassuring "nothing was written" framing when `Restored` is empty |

**Overall: 16/24**

---

## Top 3 Priority Fixes

1. **Un-shortened absolute paths leak into user-facing text** — user impact: every backup line and the write-failure error message shows the full sandbox/real filesystem path (e.g. `/var/folders/5w/2d0vm3b96_.../TestGitConfiguration_RealPTYWriteFailureRollback3606953788/001/.gitconfig.d/acme`), wrapping mid-word across up to 3 lines and breaking the `~`-relative convention every "Wrote →" line on the same screen already follows — concrete fix: apply the same tilde-collapsing helper already used to render `Wrote → ~/.gitconfig.d/acme` (`internal/tuikit/identities.go`) to the backup-path list and to `commit.Err` before it reaches `commitFailed` (`internal/tuikit/identities.go:1770-1776`).
2. **Backup-receipt list has no `maxLines`/clip-cue bound** — user impact: `04-UI-SPEC.md`'s Spacing table requires every preview surface to be `bounded to pane width, maxLines cap + clip cue`; the ceremony's backup-receipt rendering (feeding `commitSucceeded(commit.Backups)`, `internal/tuikit/identities.go:1778`) has no such cap and already consumes ~19 of the 25-row body budget for a 4-target edit (see `git-configuration-complete-edit-result.txt`) — adding a 5th write target (e.g. Phase 7's shared Git defaults) will overflow the fixed 100×30 frame — concrete fix: cap the rendered backup lines with `maxLines` + a `… (+n more)` clip cue, matching the `PreviewBlock` convention used everywhere else in the same file.
3. **D-11 failure copy does not match the documented pattern** — user impact: `04-UI-SPEC.md`'s D-11 body contract specifies `Writing <target> failed: <error>` (e.g. `Writing ~/.ssh/allowed_signers failed: permission denied`); the captured rollback frame instead shows the raw internal error verbatim, `gitid: refusing symlinked managed path: <path>`, without the `Writing <target> failed:` framing and without ever stating "nothing was written" — concrete fix: wrap known failure classes (symlink refusal, permission denied, write error) in the documented template at `internal/tuikit/identities.go:1770-1776` before calling `commitFailed`.

---

## Detailed Findings

### Pillar 1: Copywriting (3/4)

- **PASS** — Frozen/approved strings verified present and correct in the captured frames: `Wrote → ~/.gitconfig.d/acme`, `Wrote → ~/.gitconfig`, `Wrote → ~/.ssh/allowed_signers` (D-10 combined receipt ordering matches the spec's block order: Host → fragment → includeIf+insteadOf → allowed_signers); `Done (Enter)`; `Git identity "acme" configured — applies via the gitdir strategy.` (result-success, matches `FIELDS.md`'s approved state).
- **PASS** — `Esc returns to the identity detail without writing anything.` (write-failure-rollback frame) correctly reassures the user pre-write, matching the "no destructive confirmation needed, cancel-first" convention.
- **WARNING** — `internal/tuikit/identities.go:1770-1776`: on failure, `message := commit.Err` is used verbatim (only appending `(restored: ...)` when `commit.Restored` is non-empty). The captured `git-configuration-write-failure-rollback.txt` frame shows the literal string `gitid: refusing symlinked managed path: /var/folders/.../.gitconfig.d/acme`, not the D-11-documented pattern `Writing <target> failed: <error>`. This is a real divergence from the frozen copy contract in `04-UI-SPEC.md` ("D-11 body — what failed": `exact target file + exact error, e.g. "Writing ~/.ssh/allowed_signers failed: permission denied"`).
- **WARNING** — No "restored" framing appears at all in the captured failure frame (this specific case is pre-write, so `commit.Restored` is legitimately empty) — but the UI never explicitly reassures the user that nothing was written, relying entirely on the footer's generic `Esc returns to the identity detail without writing anything.` line rather than the D-11-specified `result_message`/`restore_hint` two-part shape.

### Pillar 2: Visuals (2/4)

- **BLOCKER-adjacent (WARNING, not task-breaking, but a genuine defect)** — All 3 result/rollback frames (`git-configuration-complete-edit-result.txt`, `git-configuration-mouse-field-focus.txt`, `git-configuration-write-failure-rollback.txt`) show absolute sandbox filesystem paths wrapping mid-identifier across 2-3 lines, e.g.:
  ```
  │ /var/folders/5w/2d0vm3b96_qdc9q9x1_m59tw0000gn/T/TestGitConfig
  │ uration_RealPTYCompleteEditFlow1369369275/001/.gitconfig.d/acm
  │ e.bak.1787620084922038000
  ```
  This is illegible (word "TestGitConfiguration" itself is split across two lines with no hyphenation) and inconsistent with the SAME screen's `Wrote → ~/.gitconfig.d/acme` lines, which correctly use `~`-relative shorthand. This is test-environment path leakage in the sense that a real user's HOME would render shorter, but the underlying code path (`commit.Backups`/`commit.Err` rendered without home-relativization) is real production code, not test-only — a real user with a deeply nested HOME (e.g. corporate-managed profile paths) would see the same mid-word wrapping.
- **PASS** — Clear focal point maintained on every captured screen: the sidebar (`▸` marker + status glyphs) stays visually secondary, the ceremony/result pane is the dominant right-hand focus, matching the spec's "byte-identity affordance" and "result glyph + message" focal-point guidance.
- **PASS** — No icon-only affordances found in the captured frames; every glyph (`✓`, `✗`, `S✓`, `G✓`) is paired with adjacent text per the glyph contract.

### Pillar 3: Color (3/4)

- **PASS (bounded by evidence)** — Plain-text PTY captures don't carry ANSI codes, so exact hue verification isn't directly possible from these `.txt` files; however, the glyph/role pairing (`✓` for success/healthy, `✗` for error/broken) is structurally consistent with the Theme role table (`04-UI-SPEC.md` Color section) in every frame, and `internal/tuikit/identities.go`'s cited call sites (`Theme.Error`, `Theme.Healthy`) match the spec's role assignments for D-05/D-06/D-11.
- **WARNING** — Cannot independently confirm the 60/30/10 accent distribution or that Accent (blue) is confined to the declared elements (focused-field marker, active nav, breadcrumb divider, stepper) without raw ANSI capture; scored 3 rather than 4 to reflect this residual verification gap rather than a confirmed pass.

### Pillar 4: Typography (3/4)

- **PASS** — Label/value distinction (`▸ ✓ acme  S✓ G✓` vs. plain body text) is consistent with the spec's `Label: Bold(true)` / `Field (value): plain` roles across all 4 frames.
- **PASS** — The D-08/D-12 diff-line convention (`+`/`-` glyph paired with real changed text) was not exercised in these 4 specific captures (none is an edit-mode diff view), so it cannot be independently re-verified here; deferred to the allowlist's own CTX-D-12 classification, which already covers it structurally.
- **WARNING** — No violation found, but same ANSI-invisibility caveat as Color reduces confidence from 4 to 3.

### Pillar 5: Spacing (2/4)

- **DEFECT** — The result-success ceremony receipt (`internal/tuikit/identities.go` `commitSucceeded`/backup rendering path feeding the `result-success` screen) has no `maxLines` cap or `… (+n more)` clip cue, unlike every `PreviewBlock` call site in the same file (e.g. `internal/tuikit/identities.go:966-967`, `:1306`, `:3534`, `:3575`, all of which pass an explicit `maxLines`). `04-UI-SPEC.md`'s Spacing table explicitly requires: *"Preview block ... bounded to pane width, `maxLines` cap + `… (+n more)` clip cue, title spliced into the top border edge"* for exactly this kind of block. The captured 4-backup edit receipt already occupies ~19 of the 25 body rows (`git-configuration-complete-edit-result.txt`); this leaves effectively zero headroom for D-10's future stacking (Phase 7 shared Git defaults would add more write targets to the SAME combined ceremony) and directly risks the frame overflow the spec's row-budget section says must never happen ("never grow the frame... tighten `maxLines` instead").
- **PASS** — Field-row height (1 row per field, no reflow) holds in all 4 frames; the D1 marker-gutter/label-column convention (`▸ ✓ acme`, `padRight(label, 16)`-style alignment) is intact.

### Pillar 6: Experience Design (3/4)

- **PASS** — Strong, verified state coverage: `TestGitConfiguration_RealPTYCompleteEditFlow`, `TestGitConfiguration_RealPTYSSHOnlyCompletionFlow`, `TestGitConfiguration_RealPTYWriteFailureRollback`, `TestGitConfiguration_RealPTYMouseFieldFocus` all pass against the compiled real binary (`e2e/git_configuration_pty_e2e_test.go`), covering success (edit + SSH-only-completion), failure/rollback, and mouse-driven focus — a materially more thorough state matrix than a typical web audit's loading/error/empty checklist.
- **PASS** — Disabled/blocked states handled correctly: the write-failure frame correctly offers only `Cancel (Esc)` / `Retry (Enter)`, no destructive action exposed, and explicitly states nothing was written.
- **PASS** — All-or-nothing rollback is exercised and asserted (`commit.Restored`, `04-03-SUMMARY.md`'s transactional-write coverage), matching D-10/D-11's "back up before mutate, restore in reverse order on failure" contract.
- **WARNING** — The failure path's copy gap (see Pillar 1) also degrades the experience-design pillar: a user seeing a raw Go error string with no explicit "nothing was written" framing has to infer safety from the footer hint rather than from the primary message/restore_hint the D-11 contract specifies.

---

## Files Audited

- `.planning/phases/04-git-configuration-screen/04-UI-SPEC.md`
- `.planning/phases/04-git-configuration-screen/04-CONTEXT.md`
- `.planning/phases/04-git-configuration-screen/04-01-SUMMARY.md` through `04-04-SUMMARY.md`
- `.planning/phases/04-git-configuration-screen/04-01-PLAN.md` through `04-04-PLAN.md` (read as part of required_reading)
- `.planning/design/git-screen/visual-divergence-allowlist.txt`
- `.planning/phases/05.7-complete-v1-0-product-features-in-tui/ui-frames/git-configuration-complete-edit-result.txt`
- `.planning/phases/05.7-complete-v1-0-product-features-in-tui/ui-frames/git-configuration-mouse-field-focus.txt`
- `.planning/phases/05.7-complete-v1-0-product-features-in-tui/ui-frames/git-configuration-ssh-only-completion-result.txt`
- `.planning/phases/05.7-complete-v1-0-product-features-in-tui/ui-frames/git-configuration-write-failure-rollback.txt`
- `internal/tuikit/identities.go` (rendering + ceremony/backup/failure message code paths)
- `e2e/git_configuration_pty_e2e_test.go` (test names/coverage confirmed via grep; not fully read line-by-line)
