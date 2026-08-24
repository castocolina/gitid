# Phase 3 — Independent TUI UI Review

**Audited:** 2026-08-21  
**Verdict:** **FAIL — BLOCKED**  
**Baseline:** `03-UI-SPEC.md`, `design/create-flow/FIELDS.md`, recipes, and the Phase 3 packet contract.  
**Screenshots:** 24 packet PNG panels and all 24 associated text sidecars inspected; no new capture was made (no dev server on ports 3000, 5173, or 8080).

## Scope and Integrity

- Candidate packet: `56c83c47d33068d391184c5c7faddaf3fcb5fe6e`
- Approval commit: `3c3130e404329cf42baafdf63a6c22758437edc6`
- All 50 declared members existed and their declared SHA-256 hashes matched. The packet contained exactly 24 declared PNGs and no undeclared pre-review file.
- `MANIFEST.json`'s declared `manifest_sha256` (`d1e834…966da`) does **not** equal the SHA-256 of the stored manifest bytes (`46a97d…aca5bb`), and the packet provides no canonicalization rule to reconcile them.
- `REVIEW-PROVENANCE.json`, mandated by `03-11-PLAN.md:204-215`, is absent. `REGION-DIFFS.json:5` has an empty `screens` list, so there is no region-diff evidence to audit.

---

## Pillar Scores

| Pillar | Score | Key Finding |
|---|---:|---|
| 1. Copywriting | 1/4 | The required always-visible Continue hint is absent from the disabled real Git step. |
| 2. Visuals | 1/4 | The stage panels distinguish states, but neither shows readable exact proof at 100×30. |
| 3. Color | 2/4 | No packet panel covers either required warning or hard-failure semantic state. |
| 4. Typography | 1/4 | Commands, output, and the confirmation summary are ellipsized mid-token. |
| 5. Spacing | 2/4 | Recipe preview fits, but the proof layout wastes rows while concealing required content. |
| 6. Experience Design | 1/4 | Required proof, failure coverage, review provenance, and region evidence are not independently auditable. |

**Overall: 8/24**

---

## Top 3 Priority Fixes

1. **[BLOCKER] Make both test stages fully readable in the fixed frame** — users cannot verify what ran or the complete `ssh -G` result before progressing — render a navigable/scrollable proof viewport with a visible continuation cue, preserving both exact commands, raw output, and `User`, `Hostname`, `Port`, `IdentitiesOnly`, and first `IdentityFile`.
2. **[BLOCKER] Publish actual warning and failure panels** — the user cannot inspect the path that permits a key-unused write or the path that blocks it — include distinct `ReachableNotUploaded` and hard-failure captures with glyph, word, semantic color, retry/copy affordances, and text sidecars.
3. **[BLOCKER] Repair packet provenance and region hygiene** — the packet cannot prove its review/evidence chain — publish hash-bound `REVIEW-PROVENANCE.json`, non-empty per-screen region diffs, and a verifiable manifest-digest rule; regenerate the immutable packet before review.

---

## Recheck of Prior Critical/High Findings

| Prior finding | Recheck | Result |
|---|---|---|
| Duplicate stage-1/stage-2 PNGs | Live stage panels have different hashes and visibly different content. | **Resolved** |
| Missing `EVIDENCE.json` / `REGION-DIFFS.json` | Both files now exist. | **Partially resolved:** region evidence is empty; review provenance is still absent. |
| SSH preview clipped before `IdentitiesOnly yes` | `live/ssh-form-filled.txt:20-27` and PNG show `IdentitiesOnly yes` and provider marker. | **Resolved** |
| Exact proof clipped / incomplete | `live/test-stage1-direct.txt:9-14` and `live/test-stage2-by-alias.txt:19-24` remain ellipsized and omit the validated stage-two fields. | **Open — BLOCKER** |
| Warning/error readability absent | The eight screen IDs remain limited to success/in-progress paths. | **Open — BLOCKER** |
| 100×30 preview budget failure | The recipe preview now fits in the form panel. | **Resolved for recipe preview; proof layout remains failing** |
| Git-step capability copy contradiction | The real panel correctly says `— Git configuration arrives with the next build`, but omits the required frozen Continue hint entirely. | **Open — BLOCKER** |

---

## Detailed Findings

### Pillar 1: Copywriting (1/4)

- **BLOCKER — D-19 capability copy is incomplete.** `live/git-form-demo.txt:23-25` presents the disabled reason and the Skip hint, but drops the required `Continue reviews the Git fragment, includeIf, and allowed_signers entries before writing.` hint. `03-UI-SPEC.md:142-143` and `FIELDS.md:159-164` require both hints to remain visible; D-19 changes only the disabled reason. Restore the frozen Continue hint in its reserved row, or make a documented, approved contract change rather than silently omitting it.
- **BLOCKER — proof copy is not exact or inspectable.** Both live stage panels show a `$`, a temporary-path fragment, and `-…`; stage two ends with `identityfile …` (`live/test-stage1-direct.txt:8-14`, `live/test-stage2-by-alias.txt:18-24`). This fails the exact-command/real-output contract in `FIELDS.md:92-105` and `03-UI-SPEC.md:140`.
- **WARNING — confirmation summary truncates the key path.** `live/confirm-write.txt:9-10` shows `~/.ssh/id…`, contradicting its own “Exact change” label. Show the full path or a deliberate inspectable detail view.

### Pillar 2: Visuals (1/4)

- **BLOCKER — stage distinction exists but proof is visually unreadable.** Unlike the prior packet, the panels are distinct: `live/test-stage1-direct.png` is an in-progress stage-one panel and `live/test-stage2-by-alias.png` shows both stages. However, their large bordered boxes visibly contain ellipsized path fragments rather than the evidence needed to understand either test. The visual hierarchy promotes decoration over the safety-critical proof.
- **WARNING — state coverage is visually incomplete.** The packet has no `test-fail` or `ReachableNotUploaded` screen ID despite the contract defining both visual states (`03-UI-SPEC.md:156-191`; `FIELDS.md:107-115`).
- **WARNING — approved HTML route mappings are explicit and internally consistent.** `EVIDENCE.json:102-139` maps the two intentional shared-route cases (`reuse-manual-path`/`reuse-key-vs-generate`, `mouse-focused-field`/`ssh-form-filled`) correctly. Duplicate approved-HTML PNG hashes therefore do not reproduce the prior stage-evidence defect.

### Pillar 3: Color (2/4)

- **BLOCKER — warning and failure palette cannot be reviewed.** No panel renders the yellow `! Reachable — key not uploaded yet` state or red `✗` test-failure state. This leaves `Theme.Warning`/`Theme.Error`, glyph-plus-word pairing, contrast, and follow-on affordances unproven (`03-UI-SPEC.md:91-99,156-181`).
- **WARNING — success evidence is present but insufficient.** Stage-two has a green `✓` success line (`live/test-stage2-by-alias.txt:14,24`), and the live form reserves blue focus markers. These do not substitute for the missing semantic outcomes.

### Pillar 4: Typography (1/4)

- **BLOCKER — security-critical monospace text is truncated mid-token.** The live proof uses `/var/folders/.../gitid` and `identityfile ...…`; the confirm panel uses `~/.ssh/id…` (`live/test-stage1-direct.txt:9-11`, `live/test-stage2-by-alias.txt:19-24`, `live/confirm-write.txt:9-10`). Exact configuration and command text is primary content here and cannot be reduced to ellipses.
- **WARNING — line wrapping fragments controls.** The live Git disabled reason wraps after `— Git` (`live/git-form-demo.txt:23-24`), while the missing Continue hint leaves unused rows below. Reallocate that vacant row to preserve the complete action area and frozen hint.

### Pillar 5: Spacing (2/4)

- **WARNING — recipe preview is now complete.** The prior 100×30 preview failure is resolved: the live panel includes Host, Hostname, Port 443, User git, IdentityFile, `IdentitiesOnly yes`, and provider marker in the visible preview (`live/ssh-form-filled.txt:20-27`).
- **BLOCKER — the proof layout fails the same 100×30 usability requirement.** `live/test-stage2-by-alias.txt:8-25` allocates two bordered regions plus blank rows but does not expose either full command or the five required resolved fields. A fixed frame is acceptable only when the bounded content has a discoverable continuation/viewport; no such affordance is visible (`03-UI-SPEC.md:46,58-68`; `03-11-PLAN.md:41-42,164`).

### Pillar 6: Experience Design (1/4)

- **BLOCKER — full stage-two proof is not available to the user.** The stage-two screen never visibly proves `User=git`, expected hostname/port, `IdentitiesOnly=yes`, and the first effective `IdentityFile`; it exposes only an abbreviated `identityfile` result (`live/test-stage2-by-alias.txt:18-24`). This blocks informed progression through the safety gate.
- **BLOCKER — failure and warning journeys are absent.** No panel demonstrates the hard failure/retry branch or the `ReachableNotUploaded` branch that permits confirmation while clearly warning of a key-unused identity. The successful route is insufficient coverage for a stateful security flow.
- **BLOCKER — provenance chain remains incomplete.** `03-11-PLAN.md:204-215` requires `REVIEW-PROVENANCE.json`; it is absent. `REGION-DIFFS.json:5` contains no screen records. Furthermore, the manifest self-digest does not match the raw stored bytes and no canonicalization method is supplied. Member file hashes alone cannot establish a complete immutable review packet.
- **WARNING — packet member files otherwise pass basic hygiene.** Before this review was added, all 50 declared members existed, SHA-256 checks passed, the inventory contained 24 PNGs, and no undeclared file existed. This does not cure the missing provenance/region evidence.

---

## Files Audited

- `MANIFEST.json`, `EVIDENCE.json`, and `REGION-DIFFS.json`
- All 24 PNG panels and all 24 text sidecars under `live/`, `approved-tui/`, and `approved-html/`
- `03-UI-SPEC.md`, `design/create-flow/FIELDS.md`, and `03-11-PLAN.md`
- Prior failed review: `03-11-final-review-packet/e130012c3016e4a652f62fc71269fbf849495594/UI-REVIEW.md`
- `recipes/README.md`, `recipes/ssh-config.recipe`, and `recipes/gitconfig.recipe`
