# Phase 9 — UI Review

**Audited:** 2026-08-30
**Baseline:** 09-UI-SPEC.md (approved 2026-08-28) — TUI design contract, translated 6-pillar template
**Screenshots:** Not applicable (Go Bubble Tea v2 TUI, not a web app) — audited against the committed PTY frame captures in `.planning/phases/09-upload-credentials-assist/ui-frames/*.txt` (real-binary, 100×30 raw-keystroke PTY evidence, per D-09) plus direct source inspection of `internal/tuikit/identities.go` and `internal/tuikit/design.go`.

---

## Pillar Scores

| Pillar | Score | Key Finding |
|--------|-------|-------------|
| 1. Copywriting | 2/4 | Frozen `UploadCheckboxLabelUnauth` copy structurally cannot fit its 60-col row — the actionable "run `gh auth login`… or check anyway" guidance is truncated away in every captured frame |
| 2. Visuals | 2/4 | Checkbox-unauth and checkbox-disabled frames render byte-identical content — the disabled-state copy (`UploadCheckboxLabelDisabledFmt`) has no distinguishing evidence it ever renders; checked-state glyph shown in a state the spec defines as unchecked |
| 3. Color | 3/4 | Role usage matches the spec's Warning/Healthy/Error/Info/Faint table exactly; glyph+word pairing (NO_COLOR contract) is honored everywhere observed |
| 4. Typography | 3/4 | Bold/Faint/plain roles match the spec table; no unauthorized new roles found |
| 5. Spacing | 2/4 | The `Running:` announce-line contract ("1 row per attempted command") is violated in every captured multi-command frame — long paths wrap to 3-5 physical lines with no truncation/scroll mitigation, unlike the manual-fallback block which has one |
| 6. Experience Design | 2/4 | Approved visual-regression baseline (ui-frames/) is stale relative to a later fix commit (WR-06), so the committed "approved" evidence misrepresents current behavior; D-09's mandatory `agent-ui-ux-designer` parity critique is still explicitly OPEN per the frame packet's own README |

**Overall: 14/24**

---

## Top 3 Priority Fixes

1. **Unauth checkbox label truncates away its own call-to-action** — user impact: the only actionable guidance for the most common "not yet logged in" path ("run `gh auth login` first, or check anyway") never reaches the screen; the user sees "…not logged in to gith" and nothing else — concrete fix: shorten the frozen `UploadCheckboxLabelUnauthFmt` string (design.go:582) to fit ≤60 cols including the checkbox glyph and provider/tool interpolation, e.g. drop the "or check anyway" clause and move the `gh auth login` hint to the existing `Hint`/`Faint` helper line pattern used elsewhere in the wizard, rather than cramming it into the single checkbox row — requires a `09-UI-SPEC.md` copy amendment, not just a code truncation band-aid (`ansi.Truncate(..., "…")` at identities.go:4704 only marks the loss, it doesn't fix it).

2. **`checkbox-disabled` frame is evidentially indistinguishable from `checkbox-unauth`** — user impact: cannot verify from the committed baseline that a user on a self-hosted/no-CLI host ever sees the correct `UploadCheckboxLabelDisabledFmt` ("Auto-registration unavailable — … Manual steps are shown after create.") rather than the unauth copy; if this is a real rendering bug (not just a mislabeled/duplicate capture) it means disabled-state users get a misleading "not logged in" message implying login would help, when no CLI match exists at all — concrete fix: re-run `go run ./cmd/gitid-frame-promote` after verifying `TestCreateFlow_UploadCheckboxDisabledState` actually forces `UploadEligibilityDisabled` (not `UploadEligibilityUnauth`) in its shim, and diff the two frames byte-for-byte to confirm they diverge.

3. **`Running:` command-announce lines break the "1 row per command" spacing contract** — user impact: on any real filesystem path longer than the CI temp-dir's, the `gh ssh-key add …` command wraps to 3-5 physical lines per invocation (confirmed in `create-flow-upload-autonomous-github.txt` and `-partial-scope.txt`), eating into the 30-row frame budget the spec explicitly protects and pushing the subsequent test-loop content further down/off-screen — concrete fix: apply the same truncation/"+N more" viewport treatment the manual-fallback block already has (07-UI-SPEC precedent, cited in the spec's own Spacing table) to the `UploadRunningLineFmt` line, or truncate the path portion of the command the way `renderStageOutcome`'s existing command rows do elsewhere.

---

## Detailed Findings

### Pillar 1: Copywriting (2/4)

- `internal/tuikit/design.go:582` — `UploadCheckboxLabelUnauthFmt` byte-matches the spec's DRAFT string, so the *source string* is compliant with the Copywriting Contract table. However the render path (`identities.go:4675-4705`, fixed row width 60) cannot fit it: captured frame `create-flow-upload-checkbox-unauth.txt` shows `"☑ Register with GitHub automatically — not logged in to gith"` — cut mid-word ("gith" instead of "gh"), losing the entire actionable clause (`run "gh auth login" first, or check anyway`). This is confirmed a known, tracked defect (code comment "WR-06" at identities.go:4696-4703) that was *partially* mitigated (an ellipsis truncation marker was added in commit `c3af8b9`) but the committed approved-frame evidence in `ui-frames/` predates that fix and still shows the unmarked, silent cut — the copy itself was never shortened to actually fit.
- Same file, `checkbox-disabled` frame shows the identical truncated "not logged in" text rather than `UploadCheckboxLabelDisabledFmt`'s distinct "Auto-registration unavailable…" copy — see Pillar 2 for the visual duplication finding this implies.
- `RotateDeleteOfferHeadingFmt`/`BodyFmt`/`ChoiceLeave`/`ChoiceDelete` all byte-match the spec table exactly in the captured `identity-manager-rotate-delete-offer-default.txt` frame — this half of the phase's copy is fully compliant.
- `UploadResultOK`/`Skipped`/`Failed` result rows match the spec's glyph+word format exactly (`✓ Authentication key registered`, `✗ Signing key registration failed: insufficient scope — run "gh auth refresh…"`) — compliant.
- Manual-fallback heading + body (`identity-manager-register-key-modal-manual-fallback.txt`) reproduce `internal/upload.Instructions(provider)` verbatim as required — compliant.

### Pillar 2: Visuals (2/4)

- No distinguishable evidence that `checkbox-disabled` and `checkbox-unchecked-unauth` are actually two different visual states — both captured frames (`create-flow-upload-checkbox-disabled.txt`, `create-flow-upload-checkbox-unauth.txt`) render byte-identical checkbox rows (`☑ Register with GitHub automatically — not logged in to gith`). Per the spec's own state table, `checkbox-disabled` must render UNCHECKED (`☐`) and dimmed, with the "Auto-registration unavailable" reason; the captured evidence shows CHECKED (`☑`) and the unauth reason in both cases. Either the disabled-state test shim in `TestCreateFlow_UploadCheckboxDisabledState` fails to force the disabled branch (a test-authoring bug that silently passes because it happens to match the unauth path), or the disabled-state render branch is dead code that never triggers — both are Pillar-2 defects, not cosmetic.
- Where states ARE correctly differentiated (checked-and-ready `create-flow-upload-checkbox-tab-and-click.txt` vs the announcing/per-key-results transition in `create-flow-upload-autonomous-github.txt`), the visual hierarchy matches `renderStageOutcome`'s existing faint-command/glyph-result treatment as the spec's Focal Points section demands — good, no new UI language introduced there.
- Rotate delete-offer's radio-button focus indicator (`●`/`○`) is a clear, single, unambiguous focal point defaulting to "Leave" — compliant with the D-04 non-destructive-default mandate.
- Identity Manager's `register-key-modal-u-key.txt` frame shows an undocumented interim state, `"Checking eligibility…"`, not listed anywhere in the spec's 8-state table. This is a reasonable loading-state addition (arguably satisfying the spec's own "🧪 backstop" loading-state gap noted in the UI Considerations table) but it was never added back to the design contract as an approved state — a documentation gap, not a visual defect.

### Pillar 3: Color (3/4)

- `styleWarning` used for both checkbox-unauth/disabled labels and the D-04 delete-offer heading/body — matches the spec's explicit instruction that the delete-offer is Warning-yellow, not Error-red, because the action is reversible (identities.go:2994-2995, 4688-4690).
- `styleHealthy` (green) used only for `✓ … registered` result rows; `styleError` (red) only for `✗ … failed` rows; `styleInfo`/idempotent-skip role not directly observed in the captured frames (no `already registered (skipped)` frame was captured — a coverage gap, see Pillar 6) but the source (`identities.go:4649-4653`) wires it correctly per role.
- No hardcoded ANSI escape codes or off-palette colors found in the grepped render functions — all severity coloring routes through the existing `Theme` role styles, consistent with the "zero new theme roles" contract.
- Accent (`styleSelected`) is applied only to the focused checkbox row (identities.go:4693-4695), matching the spec's "accent reserved for the checkbox row's focus highlight only" rule — not used to color announce/result lines.
- Deduction to 3/4 (not 4/4): could not independently verify the 60/30/10 dominant-surface/severity/accent balance across a full real session — evidence is limited to isolated single-screen captures, not a full flow walkthrough, so the color-dominance claim in the spec is plausible but unverified end-to-end.

### Pillar 4: Typography (3/4)

- `Label`→`Bold(true)` observed on `styleBold.Render(UploadManualHeading)` (identities.go:4581) and the checkbox label — matches spec.
- `Command`→`Faint(true)` observed on the `Running:` lines (identities.go:4643, `styleFaint.Render(fmt.Sprintf(UploadRunningLineFmt, …))`) — matches spec, and visually matches `renderStageOutcome`'s existing command-line faint treatment as the Focal Points section requires.
- No new font-weight/role classes introduced beyond the spec's declared 7-role table.
- Deduction: the `checkbox-disabled` label is double-wrapped (`styleFaint.Render(styleWarning.Render(...))`, identities.go:4690) — nesting two role styles on one string is a minor departure from the single-role-per-element pattern used everywhere else in this codebase; not spec-violating on its face, but worth flagging as it wasn't independently verifiable in the frame evidence (see Pillar 2's disabled/unauth duplication finding — this nested style might never actually render).

### Pillar 5: Spacing (2/4)

- The frozen Spacing Scale explicitly caps `testUpload` announce lines at "1 row per attempted command (max 2, GitHub only)". Every captured multi-command frame violates this: `create-flow-upload-autonomous-github.txt`'s two `Running:` lines each wrap across 4 physical rows (long temp-dir paths), and `create-flow-upload-partial-scope.txt` shows the same wrapping. While the wrapping is partly an artifact of the PTY test harness's long `TestCreateFlow_UploadAutonomousGitHubTracer4095510523/003/...` temp paths, no truncation or scroll mitigation exists in the render path for this line (unlike the manual-fallback block, which the spec explicitly gives a "+N more" escape hatch) — a real production home path (`~/.ssh/id_ed25519_acme.pub`, a realistic identity name, and a real `--title "gitid: <name> @ <hostname>"`) can still comfortably exceed the ~68-col pane width and wrap, with no documented behavior for that case.
- Checkbox row correctly stays to its declared "1 row, last row of the SSH form" budget (confirmed truncated-but-single-line in all captured frames) — that specific sub-contract is honored, even though the truncation itself is a copy defect (Pillar 1).
- Rotate delete-offer's ~3-row budget (question + key identifier + Yes/No row) is honored in the captured frames — compliant.
- No arbitrary/non-standard spacing units found (TUI has no px/rem scale to violate; row-budget adherence is the applicable equivalent, and it fails on the command-line case above).

### Pillar 6: Experience Design (2/4)

- Loading state: the spec flagged `announcing→per-key-results` transition legibility as a "🧪 backstop" concern requiring PTY verification of non-frozen-screen behavior. The `register-key-modal-u-key.txt` frame's `"Checking eligibility…"` state suggests this was addressed, but it is undocumented in the approved-states table — a traceability gap.
- Error states: gh partial-success (auth registered, signing scope-blocked) is correctly rendered as two independent rows exactly as D-16 requires (`create-flow-upload-partial-scope.txt`: `✓ Authentication key registered` + `✗ Signing key registration failed: insufficient scope…`) — compliant, non-opaque failure.
- Empty/omitted state: `create-flow-reachable-not-uploaded-evidence.txt` confirms the section renders zero new rows when omitted, falling straight to the existing `! Reachable — key not uploaded yet` warning — compliant with the "omitted adds nothing" contract.
- Confirmation for destructive-adjacent action: D-04's delete-offer correctly defaults focus to "Leave" (non-destructive) in both captured frames — compliant.
- **Process/evidence integrity finding**: `git log` shows commit `c3af8b9` ("fix(09): WR-06 mark truncated upload-checkbox rows with an ellipsis, test at the real production width") lands AFTER the frame-promotion commit `8a7c678` that produced the currently-committed `ui-frames/*.txt` baseline. The "approved" visual-regression baseline this review was asked to audit against is therefore stale relative to the code's actual current behavior on at least the checkbox-truncation defect — the frames were never re-promoted post-fix. This means the D-09 gate's own baseline cannot be trusted as ground truth without a re-promotion + re-approval pass.
- `ui-frames/REVIEW.md` itself states in its own words: **"D-09's parity-critique obligation stays OPEN"** — the mandatory `agent-ui-ux-designer` real-vs-mockup critique that the design contract requires before this phase's visual gate can be considered closed has never been run. This is a self-reported, unresolved gap in the phase's own evidence trail, not an auditor inference.
- No captured evidence exists for the `UploadResultSkipped` (idempotent-skip) row, the `UploadInventoryDegraded` line, the `UploadCrossAccountConflict` glab finding, or the `--dry-run` CLI-only state — several of the spec's explicitly-covered UI-Considerations rows (glab cross-account conflict, inventory-degraded) have no corresponding frame in the committed baseline to verify against.

---

## Files Audited

- `.planning/phases/09-upload-credentials-assist/09-UI-SPEC.md`
- `.planning/phases/09-upload-credentials-assist/09-CONTEXT.md`
- `.planning/phases/09-upload-credentials-assist/09-01-SUMMARY.md` through `09-08-SUMMARY.md`
- `.planning/phases/09-upload-credentials-assist/09-01-PLAN.md` through `09-08-PLAN.md`
- `.planning/phases/09-upload-credentials-assist/ui-frames/README.md`
- `.planning/phases/09-upload-credentials-assist/ui-frames/REVIEW.md`
- `.planning/phases/09-upload-credentials-assist/ui-frames/create-flow-upload-autonomous-github.txt`
- `.planning/phases/09-upload-credentials-assist/ui-frames/create-flow-upload-checkbox-disabled.txt`
- `.planning/phases/09-upload-credentials-assist/ui-frames/create-flow-upload-checkbox-unauth.txt`
- `.planning/phases/09-upload-credentials-assist/ui-frames/create-flow-upload-checkbox-tab-and-click.txt`
- `.planning/phases/09-upload-credentials-assist/ui-frames/create-flow-upload-partial-scope.txt`
- `.planning/phases/09-upload-credentials-assist/ui-frames/create-flow-upload-already-complete.txt`
- `.planning/phases/09-upload-credentials-assist/ui-frames/create-flow-reachable-not-uploaded-evidence.txt`
- `.planning/phases/09-upload-credentials-assist/ui-frames/identity-manager-register-key-modal-runs.txt`
- `.planning/phases/09-upload-credentials-assist/ui-frames/identity-manager-register-key-modal-manual-fallback.txt`
- `.planning/phases/09-upload-credentials-assist/ui-frames/identity-manager-register-key-modal-u-key.txt`
- `.planning/phases/09-upload-credentials-assist/ui-frames/identity-manager-rotate-delete-offer-absent.txt`
- `.planning/phases/09-upload-credentials-assist/ui-frames/identity-manager-rotate-delete-offer-default.txt`
- `.planning/phases/09-upload-credentials-assist/ui-frames/identity-manager-rotate-delete-offer-delete.txt`
- `internal/tuikit/identities.go` (checkbox row rendering, upload-section rendering, rotate delete-offer rendering — lines 2096-3020, 4247-4720)
- `internal/tuikit/design.go` (copywriting contract constants, lines 575-620)
- `internal/tuikit/backend.go` (`RotateDeleteOffer` seam, line 570-580)
- `internal/tuikit/views.go` (`RotateDeleteOfferView`/`RotateDeleteOfferMsg`, lines 761-805)
- Git history: `c3af8b9`, `8a7c678`, `60f679b` (chronology check for stale-baseline finding)
