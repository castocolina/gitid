# Phase 3 — Independent TUI UI Review

**Audited:** 2026-08-21
**Verdict:** **FAIL — BLOCKED**
**Baseline:** Approved Phase-2 design contract, `03-UI-SPEC.md`, and the immutable packet manifest.
**Screenshots:** 24 packet panels inspected (8 live, 8 approved TUI, 8 approved HTML); no new captures were made.

## Reviewer Provenance

- **Reviewer:** independent UI audit, `openai/gpt-5.6-terra`
- **Packet source commit:** `e130012c3016e4a652f62fc71269fbf849495594`
- **Approval commit:** `3c3130e404329cf42baafdf63a6c22758437edc6`
- **Manifest:** `MANIFEST.json`, declared digest `7280e01184cdf0dc1f9de3caadbb1b5a6ccf932bb82be1a2c33537d66e7b4137`
- **Integrity check:** all 48 declared member hashes matched; inventory has 24 PNGs and 49 files including the manifest. This review did not rely on an executor-authored review verdict.
- **Review limitation:** the packet contains no `EVIDENCE.json`, `REGION-DIFFS.json`, or pre-existing `REVIEW-PROVENANCE.json`, despite `03-11-PLAN.md:204-215` requiring them. It therefore cannot establish complete publisher/review provenance.

---

## Pillar Scores

| Pillar | Score | Key Finding |
|---|---:|---|
| 1. Copywriting | 2/4 | The Git step simultaneously says Continue is unavailable until the next build and promises that Continue will review Git writes. |
| 2. Visuals | 1/4 | The evidence substitutes the same test-stage image for two distinct stages and clips required preview/proof content. |
| 3. Color | 2/4 | No warning, hard-error, or ReachableNotUploaded panel exists, so required semantic-state readability is unproven. |
| 4. Typography | 2/4 | Long commands and proof values are ellipsized rather than readable at the fixed geometry. |
| 5. Spacing | 1/4 | The 100x30 form clips the Host preview before the required `IdentitiesOnly yes` line. |
| 6. Experience Design | 1/4 | Users cannot visually distinguish or independently inspect stage 1 versus stage 2 proof; packet provenance is incomplete. |

**Overall: 9/24**

---

## Top 3 Priority Fixes

1. **[CRITICAL] Recreate the stage panels from distinct real states** — TEST-01 and TEST-02 proof is not reviewable when their PNG bytes are identical — capture each state independently and fail the publisher when distinct screen IDs share a full PNG hash unless explicitly approved as intentional.
2. **[CRITICAL] Complete packet provenance** — a hash-valid panel set is not sufficient without the required evidence, region-diff, and review-provenance records — publish the missing artifacts and bind their hashes to the manifest before another review.
3. **[HIGH] Fit the required SSH preview and proof into the fixed frame** — the user cannot inspect the full recipe-shaped result or exact commands before proceeding — reserve rows/columns or provide a deliberate, discoverable preview/proof viewport with an explicit continuation cue.

---

## Detailed Findings

### Pillar 1: Copywriting (2/4)

- **[HIGH] Contradictory Git-step capability copy.** `live/git-form-demo.txt:23-27` shows `[ Continue ] — Git configuration arrives with the next build`, then says `Continue reviews the Git fragment, includeIf, and allowed_signers entries before writing.` The latter promises an action that the former permanently disables. This is a semantic departure from the approved panel, which presents an actionable Continue control (`approved-tui/git-form-demo.png`), and it leaves users unable to tell whether Git review is available. Keep the disabled reason, but replace or suppress the future-tense Continue hint while Phase 4 owns the flow.
- **[WARNING] The command/proof copy is not exact as required.** `live/test-stage1-direct.txt:8-14` and `live/test-stage2-by-alias.txt:18-24` render a `$`, a temporary path fragment, and ellipses. `03-UI-SPEC.md:140` and `FIELDS.md:92-105` require the exact command and real output. A label cannot compensate for inaccessible content.

### Pillar 2: Visuals (1/4)

- **[CRITICAL] Distinct stage evidence is visually fabricated/duplicated.** `MANIFEST.json:177-198` assigns the same SHA-256 (`3e08…f1ad`) to both approved-TUI stage PNGs; `MANIFEST.json:273-294` assigns the same SHA-256 (`eeab…1a43`) to both live stage PNGs. The two live images are therefore byte-identical although their text sidecars name different screen IDs. This prevents visual verification of the required stage-1/direct versus stage-2/alias distinction.
- **[HIGH] The live SSH preview is below the visible frame.** `live/ssh-form-filled.txt:22-28` ends after `IdentityFile`; `IdentitiesOnly yes`, mandated by `03-UI-SPEC.md:295-298` and `FIELDS.md:51`, is absent from the visible live panel. The image similarly cuts the preview at the lower chrome. The `confirm-write` panel contains the line (`live/confirm-write.txt:11-20`), but that is too late for the live-preview requirement.
- **[WARNING] Mouse proof is only a postcondition.** `live/mouse-focused-field.txt:10` shows a visible `▸` on Port, which proves focus visibility, but neither the PNG nor its text records the mouse action/target that caused it. The required full-row mouse reachability remains unproven by this packet.

### Pillar 3: Color (2/4)

- **[HIGH] Warning and error readability are not auditable.** The eight screen IDs in `MANIFEST.json:7-295` contain no `test-fail` or `ReachableNotUploaded` panel. This omits both Phase-3 semantic outcomes defined by `03-UI-SPEC.md:156-191` and the red error state required by `03-UI-SPEC.md:140`. The reviewed success panels cannot demonstrate the glyph-plus-word/color contract under failure or warning.
- **[WARNING] Healthy-state evidence exists but is insufficient for the semantic palette.** The live test text includes `✓` at `live/test-stage2-by-alias.txt:14,24`, but this does not establish ANSI warning/error contrast or monochrome readability for the omitted states.

### Pillar 4: Typography (2/4)

- **[HIGH] Important monospace content is truncated mid-token.** The stage proof pane turns command/output content into `/var/folders/.../gitid -…` and `identityfile ...…` (`live/test-stage2-by-alias.txt:9-24`). The `git-form-demo` panel also splits and truncates its disabled-reason area (`live/git-form-demo.txt:22-27`). For a security/configuration TUI, exact technical text is primary content, not secondary decoration.
- **[WARNING] Long capability prose consumes form readability.** `live/ssh-form-filled.txt:13-21` allocates nine lines to disabled algorithm explanations, leaving the preview at the fold. Preserve the descriptions, but place them behind a focused detail/viewport rather than displacing recipe-critical preview lines.

### Pillar 5: Spacing (1/4)

- **[CRITICAL] The fixed 100x30 budget is not met for the live recipe preview.** `03-UI-SPEC.md:46,58-68` makes 100x30 and the 25-body-row budget non-negotiable. The live form has exactly 30 text rows (`live/ssh-form-filled.txt:1-30`) while its preview is visibly incomplete (`:22-28`). This is clipping, not a bounded preview with a usable continuation affordance.
- **[WARNING] The proof screen uses most of the pane for two boxed fragments but does not expose either full command.** Compare `live/test-stage2-by-alias.txt:8-24`; the visual allocation consumes rows without meeting TEST-01/02's exact-output purpose. A scrollable/focusable proof viewport would use the same geometry more effectively.

### Pillar 6: Experience Design (1/4)

- **[CRITICAL] Evidence integrity is incomplete and internally contradictory.** Hashes of declared files match, but exactly two pairs of differently named stage PNGs are duplicate, as listed above. Further, the packet lacks the planned `EVIDENCE.json`, `REGION-DIFFS.json`, and `REVIEW-PROVENANCE.json` (`03-11-PLAN.md:204-215`). The packet cannot support the plan's claim of independently reproducible per-stage visual evidence.
- **[HIGH] The user journey cannot prove the required complete stage-two resolution.** `live/test-stage2-by-alias.txt:18-24` does not visibly show the complete `ssh -G` invocation/output or all required effective fields. The one visible `identityfile` summary is insufficient to establish `User`, `Hostname`, `Port`, `IdentitiesOnly`, and the first effective `IdentityFile` required by `03-11-PLAN.md:41-42,156-165`.
- **[WARNING] Stage-state coverage is narrowed to a successful, locked demo path.** `live/test-stage1-direct.txt:7` says the failure control is locked, while no error/warning panels exist. The failure/retry and not-uploaded paths therefore have neither interaction proof nor visual review coverage.

---

## Evidence Integrity Summary

- **PASS:** 48 declared member hashes verified; 24 PNG inventory verified.
- **FAIL:** Duplicate PNG bytes make live and approved TUI stage-1/stage-2 panels non-distinct.
- **FAIL:** Required provenance/evidence companion files are absent.

## Files Audited

- `.planning/phases/03-create-flow-backend/03-11-final-review-packet/e130012c3016e4a652f62fc71269fbf849495594/MANIFEST.json`
- All 24 PNG panels and representative live/approved text sidecars in `live/`, `approved-tui/`, and `approved-html/`
- `.planning/phases/03-create-flow-backend/03-UI-SPEC.md`
- `.planning/phases/03-create-flow-backend/03-11-PLAN.md`
- `.planning/design/create-flow/FIELDS.md`
- `.planning/design/APPROVAL.md`
- `recipes/{README.md,ssh-config.recipe,gitconfig.recipe}`
- `.planning/{ONESHOT.md,STATE.md,ROADMAP.md,LEARNINGS.md}`
