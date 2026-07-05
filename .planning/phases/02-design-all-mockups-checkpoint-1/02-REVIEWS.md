---
phase: 2
round: 3 (delta review of the 02-14/02-12 replan; round 2 preserved at 02-REVIEWS-2026-07-05-round2.md, round 1 at 02-REVIEWS-2026-07-02-planning-round.md)
reviewers: [claude, codex]
reviewed_at: 2026-07-05T17:40:00Z
plans_reviewed: [02-14-PLAN.md, 02-12-PLAN.md]
claude_model: default (claude CLI -p, separate non-interactive session)
codex_model: default (codex-cli 0.142.5)
verdicts: { claude: "MATCHED (fix D1 mandatory, D2, D3 before executing)", codex: "NOT MATCHED as written (same class of fixes)" }
verdict: AMEND PLAN TEXT BEFORE EXECUTING — no consensus violation, 3 shared + 3 unique plan-authoring defects
---

# Cross-AI Plan Review — Phase 2, Round 3 (Replan Delta)

> Round 3 verifies that the 02-14/02-12 replan faithfully implements the round-2
> consensus. Both reviewers received the full round-2 REVIEWS.md, the new 02-14,
> the amended 02-12, and 02-13 as dependency context. Feed back with
> `/gsd-plan-phase 2 --reviews`.

## Claude Review

All verification checks are done — every line number, symbol, script, and make target the plan cites has been confirmed against the working tree, and I found one real gate-breaking omission. Here is the review.

---

# Cross-AI Plan Review — Phase 2, Round 3 (Delta Review of the Replan)

**Reviewer:** Claude (Fable 5, this session) · **Reviewed:** 02-14-PLAN.md (new) + 02-12-PLAN.md (amended) against the round-2 consensus · **Date:** 2026-07-05

## 1. Summary

The replan is a faithful, unusually well-grounded implementation of the round-2 consensus: every one of the 8 feedback items, both HIGH implementation traps, and the copy-freeze atomicity requirement are covered by concrete, testable tasks, and nearly every code citation in the plan checks out against the working tree (I verified `stepDots` at `identities.go:1717`, the button constants at 1695–1696, the copy pins at `identities_test.go:368/714/787` and `batch3_test.go:85`, the web buttons at `Identities.tsx:603/613`, the `step`/`step0Valid`/`testPhase==='stage2'` gating symbols, the `capturesKeys` threading at `app.go:415–435`, the `pnpm typecheck` script, the `gate-no-backend-files`/`test-e2e` make targets, and the `github.com/castocolina/gitid` module path in the allowlist grep). The amended 02-12 correctly adds `depends_on: [02-14]`, the E2/E3 checklist items, and the STYLE-SPEC/02-14-SUMMARY evidence — exactly what the consensus asked the checkpoint to sign. However, the plan has **one gate-breaking omission**: the PTY e2e (`e2e/dummy_demo_e2e_test.go:209/222/231`) pins the literal strings `Step 1/4`/`Step 2/4`/`Step 3/4`, which the mandated stepper replacement removes — and that file is in neither Task 3's `<files>` nor its `read_first`, so the plan's own `make test-e2e` exit gate will fail inside a file the executor is not scoped to touch. Two smaller factual defects (the MUI-Select guard claim and a stepper label-source contradiction) also need pre-execution fixes. All three are plan-authoring bugs, not consensus violations.

## 2. Consensus Compliance Matrix

| # | Consensus item | Compliance | Where | Notes |
|---|---|---|---|---|
| 1 | Web ←/→ wizard nav (validity-gated fwd, back always) | **Full** | 02-14 Task 2 | Gates on `step0Valid` / `testPhase === 'stage2'` / git name+email — all symbols verified in `Identities.tsx:357/378-383`. One enforcement gap: see Defect D2 |
| 2 | TUI ←/→ + first-class stepper (not faint) | **Full** | 02-14 Task 3 (1)(2) | Table-driven precedence test + SGR assertion that active ≠ `styleFaint`; correctly identifies the three existing ←/→ claimants (verified at 1165/1261/1364 region) |
| 3 | TUI field contours (focused-only rounded box, blurred 1-row dim) | **Full** | 02-14 Task 3 (3) | Explicitly forbids the +12-row all-fields border; pairs with `textinput.SetStyles`; height asserted in-test AND via the 100×30 PTY walk |
| 4 | Stable hint zone (select pushes, never replaces) | **Full** | Task 3 (4) TUI, Task 2 web | Keeps 02-13's focused-only descriptive helpers while reserving a persistent hint row — the right compromise for the row budget |
| 5 | Bounded previews (fixed width/height, clip cue, title-in-border) | **Full** | Task 1 (`PreviewBlock`) + Task 3 (5) + Task 2 (`MutationCeremony.tsx`) | Reuses the existing `… (+n more lines)` machinery (`frame.go:414-441`) |
| 6 | Slide-3 copy `[ Skip Git ]` / `[ Continue ]` + hint lines | **Full** | Task 1 freeze → Task 2 web → Task 3 atomic | Frozen strings live in ONE place (STYLE-SPEC §4); Tasks 2/3 explicitly forbidden from re-deriving them |
| 7 | Semantic style contract (11 roles, Theme ↔ theme.ts, new parity dimensions) | **Full** | Task 1 | Promotes `frame.go:102-112` vars behavior-preservingly; 6 parity dimensions; correctly documents that `parity.json` is gone (verified via REFERENCE-INDEX.md) so the executor won't hunt a ghost artifact |
| 8 | Dim disabled nav + active-area accent | **Full** | Task 1 (`RenderFrame` via `capturesKeys`) + Task 2 (`Frame.tsx`) | Uses the exact signal round 2 identified; SGR-difference test specified |
| 9 | Theming question answered | **Full** | Objective + Task 1 | ANSI-16 retained, no adaptive light/dark — matches the consensus's "more portable" ruling and blocks the over-engineering risk |
| **T1** | HIGH trap: arrow-key precedence written down first | **Full** | Task 1 STYLE-SPEC §2 (5 numbered clauses, verbatim), implemented identically in Tasks 2 & 3 | Text-input cursor keys marked non-negotiable in both media |
| **T2** | HIGH trap: contours within the 100×30 budget | **Full** | Task 3 (3) + objective trap #1 | ~+2 rows quantified; PTY e2e re-run is the proof — but see Defect D1, which currently breaks that same gate for an unrelated reason |
| **A** | Copy-freeze atomicity (both demos + FIELDS.md + pins in one change) | **Full** (gate scope slightly narrow) | Task 3 (6) + repo-wide grep gate | Grep gate covers `internal/dummytui` + `mockup-src/src` but NOT `.planning/design` — see Concern C3 |
| **R** | Re-run everything (gates + fresh designer critique) | **Full** | Task 3 exit gates + 02-12 read_first | All six gates enumerated in order; fresh `agent-ui-ux-designer` critique is a blocking exit gate |
| **12** | 02-12 checklist updated to sign what was asked | **Full** | 02-12 must_haves + §6 E2/E3 + `depends_on: [02-14]` | APPROVAL.md refresh handles the stale static-PNG wording I confirmed still exists in the scaffold |

## 3. Defects in the New Plan

**D1 — HIGH: the stepper replacement breaks PTY e2e assertions in a file the plan does not own.**
`e2e/dummy_demo_e2e_test.go` asserts the literal old stepper copy — `mustSee(t, s, "Step 1/4", …)` at line 209, `"Step 2/4"` at 222, `"Step 3/4"` at 231. Task 3 mandates *replacing* the `Step n/4 · label ● ○` line with `[1] SSH · [2] Test · [3] Git · [4] Review`, and runs `make test-e2e` as a blocking exit gate — but `e2e/dummy_demo_e2e_test.go` appears in neither `files_modified`, nor Task 3's `<files>`, nor its `read_first`. As written, the plan's own gate fails and the fix is out of scope. Unit-side, `identities_test.go:119/131/237/347` pin the same old copy — that file *is* in scope, but the plan's enumeration only cites the button pins, not the stepper pins.

**D2 — MEDIUM: Task 2's guard claim about MUI Select is factually wrong.**
Task 2 says the wizard arrow handler can "rely on the existing DemoApp.tsx guard (an HTMLInputElement/HTMLTextAreaElement/contentEditable/**MUI Select** target already short-circuits…)". The actual guard (`DemoApp.tsx:102-120`, verified) checks `HTMLInputElement`, `HTMLTextAreaElement`, `isContentEditable`, and radio/checkbox toggles — nothing that matches a MUI Select, whose focusable element is a `div` (role `combobox`), as used by the match-strategy and algorithm `TextField select` at `Identities.tsx:274-281/464`. With a *closed but focused* select, ←/→ would fall through to the new wizard handler and switch steps — violating precedence clause [1] the plan itself freezes into the STYLE-SPEC. The web handler needs an explicit select-target check; "rely on the existing guard" is insufficient as specified.

**D3 — MEDIUM: internal contradiction in the stepper label source.**
Task 3 says "promote `stepDots` to a first-class `renderStepper` (**keep `wizardSteps` as the label source**)" and simultaneously mandates the segments `SSH · Test · Git · Review`. `wizardSteps` (verified at `identities.go:519`) holds `"SSH details" / "Test connection" / "Git identity" / "Review & write"` — those cannot both be the label source and produce the short segments. The short-label mapping must be pinned (in the plan or the STYLE-SPEC), or the executor will invent it — exactly the class of unfrozen copy this phase exists to eliminate.

**D4 — LOW: the atomicity grep gate's scope is narrower than the must_have truth.**
The truth says "NO artifact (Go, TSX, **FIELDS.md**) still contains the old long labels", but the gate greps only `internal/dummytui` and `.planning/design/mockup-src/src`. Today `create-flow/FIELDS.md` contains no old copy (verified), so this is latent — but the gate cannot prove the truth as stated. Also note the old copy appears in *comments* the plan doesn't enumerate (`identities.go:367`, `identities_test.go:373/435/756`); the gate will force those updates since the files are in scope, which is fine — just expect more edit sites than the three cited pin lines.

**D5 — LOW: active-area accent mechanism under-specified in Task 1.**
"The active pane region carries the ActiveArea accent" names no mechanism (accent divider? border? section headers?) and the only test specified covers header-tab dimming, not the accent. Also unstated: whether the active tab keeps its reverse-video while the tab row renders through DisabledNav. Small, but this is a parity-bearing rendering decision — the STYLE-SPEC role table should pin it.

No wave/dependency errors (wave 7/8 vs. the reviews' "6.5" label is cosmetic; the roadmap, `depends_on: [02-13]`, and 02-12's `depends_on: [02-11, 02-13, 02-14]` are all consistent). No allowlist/scope violations — every file in both plans sits inside `.planning/`, `internal/dummytui/`, or `mockup-src/`, except the e2e file that *should* be added (e2e/ is already on the Phase-2 allowlist per 02-13). Threat model is sound and correctly registers the copy-consistency risk (T-02-14-COPY). TDD assignment is appropriate (tasks 1/3 `tdd="true"`; Task 2 has no TS test harness — `package.json` has no test script — so typecheck+build+human-check is the honest verification there).

## 4. Concerns

- **HIGH (C1 = D1):** `make test-e2e` is simultaneously the plan's proof that the contours fit 100×30 *and* guaranteed to fail on the stale `Step n/4` assertions. If the executor "fixes" this by keeping `Step n/4` text inside the new stepper, item 2's intent survives; if they deviate into e2e/ without the plan saying so, the change is unreviewed. Resolve at plan level, not execution level.
- **MEDIUM (C2 = D2):** Arrow-precedence enforcement on the web is only as good as the select guard. This was the consensus's #1 HIGH trap; the plan writes the rule down perfectly and then under-specifies the one place the existing code doesn't enforce it.
- **MEDIUM (C3):** `Shift+←/→` as an *unconditional* chord inside web text inputs collides with native Shift+Arrow text selection (extending a selection in the Provider field will now switch wizard steps). This is consensus-inherited (round 2 proposed it), so the plan is compliant — but the STYLE-SPEC should acknowledge the tradeoff explicitly, since it is the one place the replan deliberately overrides a native text-editing gesture, immediately after declaring cursor keys non-negotiable.
- **LOW (C4):** Task 1 requires the theme promotion to be "byte-identical … behavior-preserving" while the *same task* changes `RenderFrame` header rendering under `capturesKeys`. Existing frame tests appear to assert ANSI-stripped content (so faint tabs won't break them), but the acceptance criterion "zero edits to their expected strings" should be scoped to the *promotion*, not the whole task, to avoid a false contradiction.
- **LOW (C5):** 02-12's `read_first` includes `02-14-SUMMARY.md`, which does not exist until 02-14 completes — correct given the dependency, just ensure the checkpoint executor treats a missing file as a blocker (evidence 02-14 actually ran), not as skippable.

## 5. Suggestions

1. **(D1)** Add `e2e/dummy_demo_e2e_test.go` to Task 3's `<files>` and `read_first`, with an explicit instruction: update the three `mustSee` stepper signatures (209/222/231) to the new stepper markers (e.g. `[1] SSH`, `[2] Test`, `[3] Git`) — or, alternatively, mandate the stepper keep a `Step n/4` textual prefix and pin that in the STYLE-SPEC. Either is one line in the plan; pick one now.
2. **(D2)** Replace "rely on the existing DemoApp.tsx guard" with: the CreateWizard handler must also no-op when `e.target` matches a select (`[role="combobox"]` / `.MuiSelect-select` / `aria-haspopup`), and add this to Task 2's acceptance criteria. Keep the guard-extension in `Identities.tsx`'s local handler (not the global guard) to avoid disturbing Global SSH's existing sub-tab arrows.
3. **(D3)** Pin the short↔long stepper label mapping in the STYLE-SPEC's frozen-copy section (`SSH ↔ SSH details`, `Test ↔ Test connection`, `Git ↔ Git identity`, `Review ↔ Review & write`) and drop "keep wizardSteps as the label source" or reword it to "derive short segments from a new pinned map; wizardSteps remains for crumbs/help".
4. **(D4)** Widen the atomicity gate to `! grep -rn '…' internal/dummytui .planning/design/mockup-src/src .planning/design/create-flow` so the gate proves the truth as written.
5. **(D5/C3)** One STYLE-SPEC paragraph each: (a) the ActiveArea mechanism and the active-tab treatment while chrome is dimmed; (b) the acknowledged Shift+Arrow-vs-text-selection tradeoff for the unconditional chord.

## 6. Risk Assessment

**Risk: LOW-MEDIUM.** The replan converts every consensus item into a concrete, gated, mostly line-number-accurate task set; the two round-2 HIGH traps are not just mentioned but structurally enforced (precedence table test; focused-only contour + PTY re-run). The residual risk is concentrated in three plan-authoring defects — one of which (D1) will deterministically fail the plan's own exit gate and force an out-of-scope deviation if not fixed before dispatch — plus one enforcement gap (D2) on the exact key-collision the consensus rated HIGH. All three are one-line-to-one-paragraph plan edits; none require re-litigating the consensus or reopening executed plans.

**Verdict: MATCHED** — the replan faithfully and completely implements the round-2 consensus (all 8 items, both HIGH traps, the atomicity requirement, the gate re-runs, and the 02-12 checklist/dependency amendments). Fix D1 (mandatory), D2, and D3 in the plan text before executing 02-14; D4/D5 are discretionary hardening.

---

## Codex Review

**Summary**

Verdict: the replan is structurally correct and captures almost all round-2 consensus in one new 02-14 wave before 02-12. However, it is **not fully matched as an executable plan yet** because the arrow-key precedence is internally contradictory around `Shift+←/→` and TUI button-ring focus, and the copy-freeze grep gate does not actually cover `FIELDS.md`. These are small edits, but they affect consensus HIGH/MEDIUM items.

**Consensus Compliance Matrix**

| Item | Status | Evidence |
|---|---:|---|
| Web ←/→ wizard navigation | Partial | 02-14 Task 2 adds it, but relies on the existing input guard for expanded MUI selects; current guard only clearly protects inputs/textareas/contenteditable. |
| TUI ←/→ wizard navigation + distinct stepper | Partial | Task 3 adds stepper/tests, but conflicts on whether button focus uses step nav or existing button-ring nav; `Shift+Right` may bypass validity. |
| TUI focused/blurred field contours | Full | Task 3 explicitly uses focused-only rounded accent contour and blurred one-row dim contour, with 100×30 e2e. |
| Stable hint zones | Full | Tasks 2/3 require persistent helper rows and expanded selects pushing content. |
| Bounded previews | Full | Task 1 updates `PreviewBlock`; Tasks 2/3 consume it. |
| Slide-3 short buttons + hint copy | Partial | Both demos and tests are covered, but old-copy grep excludes `.planning/design/create-flow/FIELDS.md`. |
| Semantic style contract / Theme ↔ theme.ts | Full | Task 1 creates `02-STYLE-SPEC.md`, Go `Theme`, and web role tokens. |
| Disabled nav dim + active-area accent | Full | Task 1 frame behavior; Task 2 web chrome; 02-12 checklist E3. |
| HIGH trap: explicit arrow precedence | Partial | Written down, but execution text contradicts validity gating and button-ring precedence. |
| HIGH trap: focused-only border within 100×30 | Full | Clearly specified and gated by PTY e2e. |
| Copy-freeze atomicity | Partial | Atomic cluster exists, but verification misses `FIELDS.md`. |

**Defects In The New Plan**

1. **HIGH:** `Shift+←/→` is underspecified and can be read as bypassing validity gates. 02-14 says normal forward nav is validity-gated, but Task 3 behavior/acceptance says `shift+left`/`shift+right` switch “unconditionally.” That conflicts with the consensus that forward navigation must not skip the two-stage test. Fix: define Shift as an unconditional **focus override**, not a validity override.

2. **HIGH:** TUI button-focus precedence contradicts itself. Task 3 says button focus switches wizard steps, then later says not to regress existing button-ring `←/→` behavior. Those cannot both be true under the consensus rule. Fix: choose one rule. Per round-2 consensus, button/stepper/non-editing focus should navigate wizard sections.

3. **MEDIUM:** Web expanded-select protection is not verifiably implemented. Task 2 relies on the global active-element guard, but current code clearly guards `HTMLInputElement`, `HTMLTextAreaElement`, and contenteditable, not an opened MUI listbox/popover. Fix: add explicit select-open state or a robust role/listbox guard, plus a manual or automated check for expanded select owning arrows.

4. **MEDIUM:** Copy-freeze grep omits `FIELDS.md`. The must-have says no artifact including `FIELDS.md` keeps the old labels, but the grep only scans `internal/dummytui` and `mockup-src/src`. Fix: include `.planning/design/create-flow/FIELDS.md` in the grep gate.

5. **LOW:** 02-12 has stale wording: “all implementation… complete (02-01..02-11 + 02-13)” omits 02-14. Fix that sentence so the checkpoint cannot be interpreted as skipping 02-14.

6. **LOW:** `.planning/STATE.md` still says next is 02-13 then 02-12, missing 02-14. Roadmap is correct, but GSD resume context is stale. Update STATE as part of the replan bookkeeping.

**Concerns**

- **HIGH:** Arrow-key behavior is the approval-critical trap from round 2; ambiguity here can recreate the exact bug the replan exists to prevent.
- **MEDIUM:** Copy consistency is almost covered, but the verification gate needs to match the stated artifact set.
- **LOW:** 02-12 route-back text says TUI issues route to 02-13; post-02-14 polish issues should route to 02-14.

**Suggestions**

- Amend `02-STYLE-SPEC.md` wording: “Shift+←/→ ignores focus ownership only; forward movement remains validity-gated.”
- Change Task 3 acceptance to assert `Shift+Right` is still blocked until stage 2 passes.
- Remove or rewrite the “Do NOT regress button-ring ←/→” sentence.
- Extend grep to: `internal/dummytui .planning/design/mockup-src/src .planning/design/create-flow/FIELDS.md`.
- Update `02-12-PLAN.md` stale implementation sentence and `.planning/STATE.md`.

**Risk Assessment**

Risk: **MEDIUM** after the small amendments; **MEDIUM-HIGH** if executed as written because the arrow-nav ambiguity sits on a round-2 HIGH concern.

Final verdict: **NOT MATCHED** as written. The structure and coverage are very close, but the high-priority arrow precedence rule and copy-freeze verification need the amendments above before this faithfully implements the round-2 consensus.

---

## Consensus Summary

Split verdict on the label, agreement on the substance: **Claude says MATCHED
(with 3 mandatory pre-execution plan edits), Codex says NOT MATCHED as written
(same class of fixes).** Neither found a consensus violation — all 8 feedback
items, both HIGH traps, and the atomicity requirement are structurally covered.
Every finding is a plan-text amendment; nothing requires re-litigating round 2
or reopening executed plans.

### Agreed Defects (raised by both — fix before executing)
1. **Copy-freeze grep gate too narrow.** The must_have truth covers FIELDS.md but
   the gate greps only `internal/dummytui` + `mockup-src/src`. Widen to include
   `.planning/design/create-flow` (Codex #4 / Claude D4).
2. **Web MUI-Select guard gap.** Task 2 "rely on the existing DemoApp.tsx guard"
   is factually insufficient — the guard covers inputs/textareas/contenteditable,
   not a focused MUI Select (`div[role="combobox"]`). A closed-but-focused select
   would leak ←/→ into step navigation, violating precedence clause [1]. Add an
   explicit select-target check in the CreateWizard local handler + acceptance
   criterion (Codex #3 / Claude D2).
3. **Shift+←/→ semantics under-specified.** Codex (HIGH): as written it can be
   read as bypassing the step-validity gate — must be a focus override only,
   never a validity override; assert `Shift+Right` still blocked until stage-2
   passes. Claude (C3): the chord also collides with native Shift+Arrow text
   selection inside web inputs — acknowledge the tradeoff in the STYLE-SPEC.

### Unique Findings
- **Claude D1 — HIGH, gate-breaking (mandatory):** `e2e/dummy_demo_e2e_test.go`
  pins the literal `Step 1/4`/`Step 2/4`/`Step 3/4` (lines 209/222/231) that the
  stepper replacement removes, and the file is in neither Task 3's `<files>` nor
  `read_first` — the plan's own `make test-e2e` exit gate deterministically fails
  in a file the executor is not scoped to touch. Add the file + explicit
  instruction to update the three `mustSee` signatures.
- **Claude D3:** stepper label-source contradiction — `wizardSteps` holds the long
  labels but the plan mandates short segments; pin the short↔long mapping in the
  STYLE-SPEC frozen-copy section.
- **Codex #2 — HIGH:** TUI button-focus arrow rule contradicts itself (buttons
  switch steps vs. "do not regress button-ring ←/→"); pick one per the consensus
  rule (non-editing focus navigates sections).
- **Codex #5/#6 — LOW:** stale wording — 02-12 "implementation complete
  (02-01..02-11 + 02-13)" omits 02-14; STATE.md "next is 02-13 then 02-12" is
  stale; 02-12 route-back text should route polish issues to 02-14.
- **Claude D5/C4/C5 — LOW:** ActiveArea accent mechanism unpinned; "byte-identical"
  criterion should scope to the theme promotion only; 02-12 must treat a missing
  02-14-SUMMARY.md as a blocker.

### Recommendation
One more `/gsd-plan-phase 2 --reviews` pass (or a targeted plan edit) applying:
D1 + Codex #2 + Shift-semantics fix (the two HIGHs + the gate-breaker), the
select-guard and grep-scope fixes, the label mapping, and the stale-wording
cleanups. Then execute wave 7.
