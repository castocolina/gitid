---
phase: 05-cli-surface-tui
plan: "04"
subsystem: tui-forms-prove-copy
tags: [tui, bubbletea-v2, lipgloss-v2, bubbles-v2, tdd, forms, prove-before-write, D-02, D-03, D-04, D-06]
dependency_graph:
  requires:
    - tui/model.go (rootModel/screenModel from 05-01)
    - tui/messages.go (preWriteResultMsg, resolvedResultMsg, writeResultMsg, clipboardResultMsg from 05-01)
    - tui/styles.go (StyleLabel/StyleInputActive/StyleInputInactive/StyleFaint/StylePass from 05-01)
    - tui/keymap.go (keys.Edit/AddHost/Copy/Delete/Rotate/Next/Prev/Submit/Confirm/Back/Quit from 05-01)
    - tui/deps.go (tuiDeps, buildTUIDeps from 05-01)
    - tui/dashboard.go + tui/identitylist.go (navigation stack from 05-03)
    - internal/identity.ValidateName (from 05-01 quick fix)
    - internal/tester.PreWrite + internal/tester.Resolved
    - internal/upload.Instructions
    - internal/clipboard.Copy
  provides:
    - tui/identitydetail.go — identityDetailModel, e/h/c/d/R actions (D-02/D-03/D-06)
    - tui/createform.go — createFormModel (8 textinput fields, ValidateName gate, Tab/Shift+Tab focus)
    - tui/updateform.go — updateFormModel (pre-filled, name read-only)
    - tui/addaccountform.go — addAccountFormModel (pre-filled from existing identity)
    - tui/prove.go — proveModel (two-phase async SSH test + confirm gate, write via identity.Deps)
    - tui/copy.go — runClipboardCopyCmd + renderCopyOverlay
    - tui/form_test.go + tui/prove_test.go — 9 tests covering TUI-02/D-02/D-03/D-04/D-06
  affects:
    - tui/identitylist.go — placeholder stubs removed; newCreateFormScreen/newIdentityDetailScreen now real
tech_stack:
  patterns:
    - TDD RED/GREEN with lint-gated pre-commit hook (//nolint:unused stubs pass lint before implementation)
    - textinput Tab/Shift+Tab focus advance via msg.String() == "shift+tab" before msg.Code == tea.KeyTab
    - Async two-phase SSH test as tea.Cmd goroutines (RESEARCH Pattern 5 / Pitfall 7)
    - provePhase state machine: phase1Running → phase1Done → phase2Running → phase2Done (or Failed variants)
    - confirmActive gate: only true after BOTH preWriteResultMsg(pass) + resolvedResultMsg(pass) arrive
    - identity.Create(in, deps) with Confirmed=true as the single confirmed write path (D-04/T-05-14)
    - Direct Update(msg) → assert model state unit tests (no teatest needed)
key_files:
  created:
    - tui/identitydetail.go
    - tui/createform.go
    - tui/updateform.go
    - tui/addaccountform.go
    - tui/prove.go
    - tui/copy.go
    - tui/form_test.go
    - tui/prove_test.go
  modified:
    - tui/identitylist.go (placeholder stubs removed)
decisions:
  - "proveModel.init() uses input.SSHConfigPath as keyPath; empty in test mode (tests inject preWriteResultMsg directly)"
  - "runWriteCmd calls identity.Create(in, deps.identity) with Confirmed=true — single write path, no new filewriter call"
  - "confirmActive set only when BOTH preWriteResultMsg(pass) + resolvedResultMsg(pass) arrive (T-05-15)"
  - "Shift+Tab handled via msg.String() == 'shift+tab' check before msg.Code == tea.KeyTab to avoid ModShift collision"
  - "SeverityStyle(doctor.SeverityCritical/Error) used in prove.go — imports internal/doctor for severity constants"
  - "proveModel holds keyPath field (separate from CreateInput) to support existing-key flows in future"
  - "identitydetail.go uses keys.AddHost (maps to 'H') and keys.Rotate (maps to 'R') — uppercase per keymap.go"
metrics:
  duration: "~13 min"
  completed: "2026-06-13"
  tasks: 2
  files_created: 8
  files_modified: 1
---

# Phase 05 Plan 04: TUI Forms + Prove-Before-Write Screen Summary

Completes the TUI navigation tree (TUI-02). Delivers the identity detail screen, three in-app forms (Create, Update, Add-account — D-02), the inline Copy-pubkey action (D-06), and the shared Prove-Before-Write screen (D-04). Closes SC-2/TUI-02: users can add and edit identities without leaving the app, with every mutation gated by the two-phase SSH test + explicit confirm.

## What Was Built

**tui/identitydetail.go — identityDetailModel (Screen 3):**
- Two-column metadata block per UI-SPEC (StyleLabel 16-wide + StyleBody/StyleFaint)
- `e` → pushes `updateFormModel`; `H` → pushes `addAccountFormModel`; `c` → `runClipboardCopyCmd` + sets overlay
- `d` → deleteHandoffMsg (inline, no write, D-03); `R` → rotateHandoffMsg (inline, no write, D-03)
- `Esc` → popCmd; overlay dismissed on any neutral key

**tui/createform.go — createFormModel (Screen 4):**
- 8 `textinput.Model` fields: Identity Name, Git Name, Git Email, Provider, Port, SSH Alias, Match Strategy, Passphrase
- Tab/Shift+Tab focus advance via `msg.String() == "shift+tab"` (checked before `msg.Code == tea.KeyTab` to handle ModShift)
- `identity.ValidateName` gate (T-05-13): invalid name sets `m.err`, no pushScreenMsg emitted
- On Enter at last field with valid name → `pushCmd(newProveScreen("create", in, deps))`

**tui/updateform.go — updateFormModel (Screen 5):**
- 5 editable fields (name shown as read-only via StyleFaint header)
- Same Tab/Shift+Tab focus logic as create form
- Pre-filled from `identity.Account` values; submits to prove screen

**tui/addaccountform.go — addAccountFormModel (Screen 5b):**
- 4 editable fields (Provider, SSH Alias, Port, Match Strategy); identity + key path read-only
- Pre-filled from existing account with `identity.DefaultAlias(name, provider)`
- ValidateName check on existing identity name before submit

**tui/prove.go — proveModel (Screen 6):**
- `provePhase` state machine: phase1Running → phase1Done → phase2Running → phase2Done (or Failed variants)
- `init()` issues `runPreWriteCmd(keyPath, hostname, port)` as tea.Cmd (RESEARCH Pattern 5)
- `runPreWriteCmd` / `runResolvedCmd` wrap `tester.PreWrite` / `tester.Resolved` in goroutines — NEVER called in Update (Pitfall 7)
- `confirmActive` set only after BOTH `preWriteResultMsg(pass)` + `resolvedResultMsg(pass)` arrive (T-05-15/D-04)
- Phase 1 failure: view shows failure output + "Cannot proceed"; Enter is inert; only Esc active
- On Enter when confirmActive: `runWriteCmd` calls `identity.Create(in, deps.identity)` with Confirmed=true
- No filewriter import — writes exclusively through identity.Deps seams (T-05-14/D-04)

**tui/copy.go — inline copy action:**
- `runClipboardCopyCmd(pubLine)` → `clipboardResultMsg` via tea.Cmd goroutine
- `renderCopyOverlay(pubLine, provider, copyErr)` → success/failure overlay with `upload.Instructions`

**tui/identitylist.go (modified):**
- Placeholder stubs removed; newCreateFormScreen/newIdentityDetailScreen now reference real implementations

## Verification Results

```
go test ./tui/... -count=1 -run 'TestFormTabNavigation|TestCreateFormNameValidation|TestIdentityDetailEditKey|TestIdentityDetailCopyAction|TestDetailDeleteHandoff'
PASS (5/5)

go test ./tui/... -count=1 -run 'TestProveScreenConfirmGate|TestProveScreenPhase1Failure|TestProveScreenRunsPhasesAsTeaCmds|TestProveConfirmWritesViaDeps'
PASS (4/4)

go test ./tui/... -count=1  → PASS (30 tests total)
go test -race ./... → PASS (all packages)
go build ./... → PASS

grep -n "tester.PreWrite\|tester.Resolved" tui/prove.go
→ lines 77, 87 only — inside func() tea.Msg closures, never in update()

grep -n "filewriter" tui/prove.go
→ comment lines only (no import, no function call)
```

## TDD Gate Compliance

- Task 1 RED: commit `fcc9379` — `test(05-04): add failing tests for form tab navigation, name validation, detail actions + copy/delete handoff`
- Task 1 GREEN: commit `fb0aa60` — `feat(05-04): implement identity detail + three in-app forms + inline copy action (Task 1 GREEN)`
- Task 2 RED: commit `4b90948` — `test(05-04): add failing tests for prove screen confirm gate, phase-1 failure, async cmd pattern, write via deps`
- Task 2 GREEN: commit `3221dbf` — `feat(05-04): implement Prove-Before-Write screen with two-phase async test + confirm gate (Task 2 GREEN)`

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] tea.KeyBackTab does not exist in bubbletea v2**
- **Found during:** Task 1 RED (compile error)
- **Issue:** The test used `tea.KeyBackTab` which is not a constant in bubbletea v2. Shift+Tab is represented as `tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}`, and `msg.String()` returns `"shift+tab"`.
- **Fix:** Changed test to `tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}`; changed form `update()` to check `msg.String() == "shift+tab"` before `msg.Code == tea.KeyTab` (order matters — ModShift causes String() to return "shift+tab", not "tab").
- **Files modified:** tui/form_test.go, tui/createform.go
- **Commit:** fcc9379 / fb0aa60

**2. [Rule 1 - Bug] tuiDeps field name in test was wrong**
- **Found during:** Task 1 RED (compile error)
- **Issue:** `fakeWriteTUIDeps` used `identityDeps` as the field name but `tuiDeps` uses `identity` (as declared in model.go).
- **Fix:** Renamed to `identity` in the test.
- **Files modified:** tui/form_test.go
- **Commit:** fcc9379

**3. [Rule 2 - Missing functionality] Lint required //nolint:unused on stub fields**
- **Found during:** Task 1 RED commit (pre-commit hook rejected)
- **Issue:** All stub files had struct fields flagged as unused by the `unused` linter.
- **Fix:** Added `//nolint:unused` to all stub struct fields; ran goimports to fix import ordering.
- **Files modified:** tui/createform.go, tui/updateform.go, tui/addaccountform.go, tui/identitydetail.go, tui/prove.go
- **Commit:** fcc9379

**4. [Rule 1 - Bug] SeverityStyle called with integer literal**
- **Found during:** Task 2 GREEN (compile error)
- **Issue:** prove.go used `SeverityStyle(1)` but the function signature requires `doctor.Severity`.
- **Fix:** Imported `internal/doctor` and used `doctor.SeverityCritical`/`doctor.SeverityError`.
- **Files modified:** tui/prove.go
- **Commit:** 3221dbf

**5. [Rule 1 - Bug] provePhase state machine comparison needed `>=` not `==`**
- **Found during:** Task 2 GREEN (view() not showing Phase 2 section)
- **Issue:** `if m.phase == provePhase2Running` missed the Done and Failed states. Phase 2 section should appear whenever phase >= phase2Running.
- **Fix:** Changed condition to `if m.phase >= provePhase2Running`.
- **Files modified:** tui/prove.go
- **Commit:** 3221dbf

## Known Stubs

None. All placeholder factories from 05-03 have been replaced with real implementations.

## Threat Surface Scan

| Flag | File | Description |
|------|------|-------------|
| threat_flag: write-gate | tui/prove.go | Confirm gate implemented per T-05-15; confirmActive only true after both phases pass |
| threat_flag: identity-name-validation | tui/createform.go, tui/addaccountform.go | identity.ValidateName called before submit per T-05-13 |
| threat_flag: no-new-write-path | tui/prove.go | runWriteCmd uses identity.Create via deps.identity — no direct filewriter call (T-05-14) |
| threat_flag: async-ssh | tui/prove.go | tester.PreWrite/Resolved only in tea.Cmd goroutines (T-05-16/Pitfall 7) |

All threats from the plan's threat register (T-05-13 through T-05-17) are mitigated.

## Self-Check: PASSED
