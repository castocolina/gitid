---
phase: 3
reviewers: [codex]
reviewed_at: 2026-07-08T14:13:28Z
plans_reviewed: [03-01-PLAN.md, 03-02-PLAN.md, 03-03-PLAN.md, 03-04-PLAN.md, 03-05-PLAN.md, 03-06-PLAN.md]
overall_risk: HIGH
verdict: replan-required
---

# Cross-AI Plan Review — Phase 3 (Create Flow Backend)

Reviewer: **Codex** (codex-cli 0.142.5), invoked as the mandatory external
cross-vendor gate. The review prompt was augmented with the binding
LEARNINGS ledger (L2 injected-seam wiring, L4 doctor reserved-block) and the
LEGACY-TRIAGE Phase-3 obligations (repoclone/adopt DROP, tuikit wholesale
extraction, no-backend-test restoration) as an explicit mandatory-check list.

## Codex Review

**Summary**

The plan set is directionally strong and mostly aligned with Phase 3: it preserves the recipe shape, extracts `internal/tuikit`, archives the POC surface, wires a real backend, and adds PTY + visual gates. However, I would not approve it as-is. There are several blocking gaps against the mandatory ledger/legacy checks, plus one serious import-graph contradiction that could make the `tuikit` split fail its own gate.

**Overall Risk: HIGH**

Justification: the phase is achievable, but the plans currently miss two mandatory obligations (`L2`, `L4`), have an incorrect dependency edge, and contain conflicting instructions about keeping `internal/tuikit` backend-free while later importing/referencing backend types.

**Mandatory Check Findings**

- **HIGH: L2 is not fully satisfied for the new `ReadPub` seam.**  
  03-01 introduces `identity.Deps.ReadPub` at `.planning/phases/03-create-flow-backend/03-01-PLAN.md:82` and explicitly does not wire it until 03-03 at line 106. 03-03 wires it at `.planning/phases/03-create-flow-backend/03-03-PLAN.md:132`. But 03-06’s PTY plan covers form/test/git/review screens, not the reuse-key path that exercises `ReadPub` through the real binary. This leaves the seam covered by unit/wiring tests only, violating L2’s raw-keystroke PTY requirement.  
  **Suggestion:** add a 03-06 PTY case that selects reuse-existing-key, uses an encrypted key with existing `.pub`, proceeds through confirm, and proves the real constructor path uses `ReadPub`. Also add explicit nil-guard coverage for `ReadPub`.

- **HIGH: L4 doctor-reserved registration is missing.**  
  03-03 introduces/changes reserved SSH paths and managed artifacts: fresh default `~/.ssh/config.d/gitid.config`, `Include ~/.ssh/config.d/*.config`, and the idempotent macOS `Host *` block at `.planning/phases/03-create-flow-backend/03-03-PLAN.md:169`. No plan registers these as doctor-reserved. This is a binding L4 miss and risks `health --fix` treating Phase 3-owned config as foreign/destructive.  
  **Suggestion:** add a task in 03-03 or a new blocking subtask to update doctor reserved-block/path registration and tests for `health --fix` not deleting or rewriting these artifacts falsely.

- **HIGH: legacy triage only partially drops the POC add-repo feature.**  
  03-03 archives `cmd/gitid/addrepo*.go` and the whole `tui/` package at `.planning/phases/03-create-flow-backend/03-03-PLAN.md:101`, which covers `tui/addrepo.go` and `tui/adopt.go`. It does not mention dropping `internal/repoclone`, which the mandatory triage explicitly names.  
  **Suggestion:** add `internal/repoclone` to 03-03’s archive/removal scope or state why it remains as non-POC substrate. As written, it fails “DROP the POC add-repo feature entirely.”

- **PASS: POC Cobra surface archive + debug kept + real shell rewire.**  
  03-03 covers this clearly at lines 85, 101, 132, and 142.

- **PASS: no-backend import-graph test restoration and allowlist reshaping.**  
  03-02 covers this at `.planning/phases/03-create-flow-backend/03-02-PLAN.md:181-199`.

- **PASS with caution: plans do not patch `tui/`; they archive it wholesale.**  
  This matches the legacy-triage obligation.

**Cross-Plan Concerns**

- **HIGH: dependency metadata is wrong.**  
  03-03 depends only on `03-02`, but it uses `ReadPub`, `ScanReusableKeys`, and `ResolvedViaCommand` from 03-01. See 03-03’s wiring task at `.planning/phases/03-create-flow-backend/03-03-PLAN.md:120-138`.  
  **Suggestion:** set `03-03 depends_on: ["03-01", "03-02"]`. 03-04/05 then inherit correctly.

- **HIGH: `tuikit` backend-free contract conflicts with later tasks.**  
  03-02 says `internal/tuikit` must import zero first-party backend packages and the no-backend gate must fail on `identity/tester/sshconfig/keygen/filewriter/...` at `.planning/phases/03-create-flow-backend/03-02-PLAN.md:30` and `190`. But 03-04 says the picker consumes `keygen.ReusableKey` via the seam at `.planning/phases/03-create-flow-backend/03-04-PLAN.md:183`, and 03-05 tells `tuikit` to branch on `tester.ReachableNotUploaded` at `.planning/phases/03-create-flow-backend/03-05-PLAN.md:116`. That would either fail the gate or weaken it.  
  **Suggestion:** define `tuikit`-local DTOs/enums: `ReusableKeyView`, `TestOutcome`, `TestResultView`, etc. Convert from `keygen`/`tester` types only in `cmd/gitid/wiring.go`.

- **MEDIUM: mouse-clickability is under-tested.**  
  SSHUI-02 requires mouse and keyboard. 03-04 says mouse routing is “preserved” at line 59, but 03-06’s PTY list at `.planning/phases/03-create-flow-backend/03-06-PLAN.md:109-126` only commits to raw keystrokes.  
  **Suggestion:** add at least one PTY mouse CSI test that clicks each SSH field and verifies focus/value entry.

- **MEDIUM: STORE-01 supersession is not reflected in docs.**  
  03-03 implements Include-by-default at line 169, but no plan updates `REQUIREMENTS.md` despite the context saying D-06 supersedes STORE-01 wording when touched.  
  **Suggestion:** include a documentation/traceability update in 03-03 or explicitly defer with a recorded reason.

**Per-Plan Review**

**03-01 — Backend Gap Functions**  
Strengths: tight scope, good TDD posture, covers the encrypted-key `.pub` gap and command parity.  
Concerns: **HIGH** L2 seam coverage gap for `ReadPub`; **LOW** `ReusableKey.ParseError` is specified but non-encrypted parse errors are skipped, so the field may be dead/ambiguous.  
Risk: **MEDIUM** alone, **HIGH** because of L2.

**03-02 — `internal/tuikit` Extraction**  
Strengths: correctly centers D-17, restores no-backend gate, keeps dummy byte-identical.  
Concerns: **HIGH** seam method set risks importing backend types despite the backend-free gate; **MEDIUM** very large extraction plus interface design in one wave increases churn.  
Risk: **HIGH** until DTO boundary is made explicit.

**03-03 — Real App Shell + Archive + Persist**  
Strengths: correctly removes old entry, keeps `debug`, wires real constructor, uses backup/atomic write path.  
Concerns: **HIGH** missing `03-01` dependency; **HIGH** missing L4 doctor-reserved registration; **HIGH** missing `internal/repoclone` legacy drop; **MEDIUM** requirements update for Include-default is absent.  
Risk: **HIGH**.

**03-04 — Form Deltas**  
Strengths: good coverage of provider reactivity, recipe-faithful preview, collision blocking, reuse picker.  
Concerns: **HIGH** planned use of `keygen.ReusableKey` in `tuikit` conflicts with no-backend gate; **MEDIUM** mouse behavior lacks real PTY coverage.  
Risk: **MEDIUM-HIGH**.

**03-05 — Test Outcome + Git Skip**  
Strengths: correctly identifies `Permission denied (publickey)` as `ReachableNotUploaded`, preserves dummy/real copy divergence, avoids Phase 9 upload scope creep.  
Concerns: **HIGH** planned use of `tester` outcome constants inside `tuikit` conflicts with backend-free rule; **MEDIUM** result copy for “key-unused, not success” needs exact acceptance text to avoid regressions.  
Risk: **MEDIUM-HIGH**.

**03-06 — Gates**  
Strengths: finally closes DLV-06/DLV-04 with real-binary PTY, fake `ssh`, hard visual diff, and cross-AI review.  
Concerns: **HIGH** does not include PTY coverage for the new `ReadPub`/reuse-key seam; **MEDIUM** “every create-flow screen” may miss reuse picker/manual path/mouse-focus states; **LOW** visual `View()` capture must be careful not to bypass real composition.  
Risk: **MEDIUM**, but **HIGH** as the only place L2 can be closed.

**Specific Approval Conditions**

1. Add doctor-reserved registration and tests for new Include path/managed blocks.
2. Add `03-01` to 03-03 dependencies.
3. Remove or archive `internal/repoclone`, or explicitly justify it as non-POC substrate.
4. Keep `internal/tuikit` backend-free by using local DTOs/enums only.
5. Add real-binary PTY coverage for reuse-existing-key, encrypted key with existing `.pub`, and mouse field focus.
6. Add STORE-01 documentation/traceability update for Include-by-default.

---

## Consensus Summary

Single external reviewer (Codex); its findings are the consensus by
construction. Overall verdict: **HIGH risk, replan required** — the plan set
is directionally correct (recipe-faithful, tuikit extraction, POC archive,
real backend, PTY + visual gates) but has blocking gaps.

### Agreed Concerns (must be resolved in replan)

**Blocking (HIGH):**
1. **L2 seam coverage gap** — the new `identity.Deps.ReadPub` seam (introduced
   03-01:82, wired 03-03:132) is exercised only by unit/wiring tests; 03-06's
   PTY suite never drives the reuse-existing-key path through the real binary.
   Fix: add a 03-06 raw-keystroke PTY case selecting reuse-existing-key with an
   encrypted key + existing `.pub` through confirm, plus explicit `ReadPub`
   nil-guard coverage.
2. **L4 doctor-reserved registration missing** — 03-03 creates
   `~/.ssh/config.d/gitid.config`, the `Include ~/.ssh/config.d/*.config` line,
   and the idempotent macOS `Host *` block (03-03:169) but no plan registers
   them as doctor-reserved. Fix: add a blocking task in 03-03 registering these
   in the reserved-block/path registry + a `health --fix` non-destruction test.
3. **Legacy drop incomplete** — 03-03 archives `cmd/gitid/addrepo*.go` and the
   whole `tui/` package (covers `tui/addrepo.go`, `tui/adopt.go`) but never
   drops `internal/repoclone`. Fix: add `internal/repoclone` to 03-03's
   archive/removal scope (matches LEGACY-TRIAGE's Phase-3 DROP obligation).
4. **Wrong dependency edge** — 03-03 uses `ReadPub`/`ScanReusableKeys`/
   `ResolvedViaCommand` from 03-01 but declares `depends_on: [03-02]` only.
   Fix: `03-03 depends_on: [03-01, 03-02]`.
5. **tuikit backend-free contract contradiction** — 03-02 mandates
   `internal/tuikit` import zero first-party backend packages (no-backend gate
   fails on identity/tester/sshconfig/keygen/filewriter), yet 03-04:183 has the
   picker consume `keygen.ReusableKey` and 03-05:116 has tuikit branch on
   `tester.ReachableNotUploaded`. That would fail or weaken the gate. Fix:
   define tuikit-local DTOs/enums (`ReusableKeyView`, `TestOutcome`,
   `TestResultView`) and convert from backend types only in `cmd/gitid/wiring.go`.

**Non-blocking (MEDIUM):**
6. Mouse-clickability under-tested — SSHUI-02 requires mouse+keyboard; 03-06
   commits only to raw keystrokes. Add ≥1 PTY mouse-CSI click test per SSH field.
7. STORE-01 supersession not reflected — 03-03 implements Include-by-default
   (D-06 supersedes STORE-01 wording) but no plan updates REQUIREMENTS.md.

### Agreed Strengths
- Recipe-faithful config shape; correct D-17 tuikit centering; POC Cobra
  archive keeping `debug caps`; real constructor wiring with backup/atomic
  writes; correct `Permission denied (publickey)` → ReachableNotUploaded
  classification; no-backend import-graph gate restoration; wholesale `tui/`
  archive (not patched).

### Divergent Views
- None (single reviewer).

### Six Codex approval conditions (replan checklist)
1. Doctor-reserved registration + tests for new Include path/managed blocks.
2. `03-03 depends_on` adds `03-01`.
3. Drop/archive `internal/repoclone` (or justify as non-POC substrate).
4. Keep `internal/tuikit` backend-free via local DTOs/enums.
5. Real-binary PTY coverage for reuse-existing-key, encrypted key w/ existing
   `.pub`, and mouse field focus.
6. STORE-01 documentation/traceability update for Include-by-default.
