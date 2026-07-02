---
phase: 05-cli-surface-tui
verified: 2026-06-13T00:00:00Z
status: human_needed
score: 9/9
overrides_applied: 0
human_verification:
  - test: "Run 'gitid' with no args in a real terminal (TTY). Observe that the Bubble Tea TUI launches immediately and the doctor dashboard is displayed as the first screen, with seven family panels visible (some may show spinner, some may show results)."
    expected: "Alternate screen activates; dashboard title 'gitid — Doctor Dashboard' appears; seven family sections render with spinners or findings; 'r' triggers a refresh."
    why_human: "alt-screen TUI launch, spinner animation, and terminal rendering cannot be exercised by grep or unit tests without a real terminal."
  - test: "From the TUI dashboard, press Enter to navigate to the Identity List. If identities exist, press Enter on one to navigate to Identity Detail. From detail, press 'e' to open the Update form, and 'H' to open the Add-account form. Press Esc to pop back at each level."
    expected: "Drill-down stack navigation (Dashboard → List → Detail → Form) works without leaving the app. Esc pops one level at each point. No crash or blank screen."
    why_human: "Interactive push/pop TUI navigation and screen rendering cannot be verified without a real TTY session."
  - test: "From the Identity List, press 'a' to open the Create Identity form. Tab through all 8 fields and Shift+Tab back. Fill the Identity Name with 'test identity' (with a space) and press Enter — verify an inline error is shown and no navigation occurs."
    expected: "Tab advances focus correctly through 8 fields; Shift+Tab retreats. Invalid name shows error and blocks submission."
    why_human: "Tab/Shift+Tab focus ring and inline validation error rendering are TUI visual behaviors."
  - test: "Navigate to an identity that has a .pub key file. From Identity Detail, press 'c' and verify the clipboard receives the public key and the overlay shows upload instructions."
    expected: "Copy action reads the .pub, copies to clipboard, overlay displays provider instructions."
    why_human: "Clipboard interaction and rendered overlay content require a real session and a clipboard tool to be present."
  - test: "Run 'gitid completion bash' and 'gitid completion zsh' in a shell. Pipe each to a file and source it or review its content for correctness beyond containing 'gitid'."
    expected: "Generated completion scripts are syntactically valid and define completions for all registered subcommands."
    why_human: "Script syntax correctness and real shell loading of completion cannot be verified by the unit tests (which only assert non-empty + contains 'gitid')."
  - test: "Run 'gitid copy <existing-identity-name>' in a terminal where clipboard is available. Verify the public key is copied and upload instructions are printed."
    expected: "Output contains 'Copied public key for ... to clipboard.' and provider-specific upload steps."
    why_human: "End-to-end test of clipboard + upload instructions requires a real identity on disk and a clipboard tool."
---

# Phase 5: CLI Surface + TUI — Verification Report

**Phase Goal:** The full `gitid` command surface is available with shell completion, and running `gitid` with no arguments launches a Bubble Tea TUI that opens on the doctor dashboard and lets users navigate to identity and account management.
**Verified:** 2026-06-13T00:00:00Z
**Status:** human_needed
**Re-verification:** No — initial verification

---

## Goal Achievement

The phase goal decomposes into four ROADMAP success criteria and the four requirement IDs (CLI-01, CLI-02, TUI-01, TUI-02). All nine observable truths derived from those criteria are VERIFIED by codebase evidence and passing tests. Human verification items exist for visual/interactive behaviors that cannot be exercised programmatically.

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `gitid` no-args on TTY launches `tui.Run()` | VERIFIED | `cmd/gitid/main.go:40-41` — `term.IsTerminal` branch calls `noArgsAction(isTTY, tui.Run, ...)` |
| 2 | `gitid` no-args on non-TTY prints usage hint + exits 1 | VERIFIED | `TestNoArgsActionNonTTY` PASS; `noArgsAction` returns 1 + prints "gitid: no subcommand given" |
| 3 | Doctor dashboard is the TUI's first screen (TUI-01) | VERIFIED | `tui/model.go:55` — `newRootModel` pushes `newDashboardModel(d)` as home; `Init()` returns its Batch of 7 cmds |
| 4 | Dashboard streams 7 families async with runID stale-guard (TUI-01) | VERIFIED | `tui/dashboard.go:72-82` — `init()` batches 7 family cmds; `TestDashboardInit/FamilyResult/Refresh/StaleResult` PASS |
| 5 | TUI navigation: Enter drills dashboard→list→detail→form; Esc pops (TUI-02) | VERIFIED | `TestDashboardEnterNavigates`, `TestIdentityListEscPops`, `TestIdentityDetailEditKey` PASS; deps threaded dashboard→list→form→prove per `TestDepsThreadedEndToEnd` PASS |
| 6 | In-app forms (Create/Update/Add-account) with prove-before-write, gated write via identity.Deps (TUI-02, D-04) | VERIFIED | CR-01..CR-04 all fixed in commit a8bdf86; `TestPushInvokesProveInit`, `TestProveScreenConfirmGate`, `TestProveConfirmWritesViaDeps`, `TestRunWriteCmdDispatchesUpdate`, `TestRunWriteCmdDispatchesAddAccount`, `TestProveKeyPathIsPrivateKeyNotSSHConfig` all PASS |
| 7 | `gitid completion bash/zsh/fish` produce non-empty scripts containing "gitid" (CLI-02) | VERIFIED | `TestCompletionBash`, `TestCompletionZsh`, `TestCompletionFish` PASS; Cobra auto-registers completion subcommand |
| 8 | Every Phase 2-4 capability reachable as a gitid subcommand (CLI-01, SC-4) | VERIFIED | `TestNewRootCmdTopLevelAliases` PASS; `newRootCmd` registers: `identity {add/list/test/rotate/update/delete/copy}`, `baseline {setup/show}`, `doctor`, `rotate`, `copy`, `host {add}` |
| 9 | `go build ./...` succeeds and `make lint` reports 0 issues | VERIFIED | `go build ./...` exit 0; `golangci-lint run ./... — 0 issues.`; `go test -race ./...` all PASS |

**Score:** 9/9 truths verified

---

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/upload/upload.go` | `Instructions(provider string) string` extracted from cmd/gitid | VERIFIED | Line 26: `func Instructions(provider string) string` |
| `internal/identity/validate.go` | `ValidateName(name string) error` domain-layer gate | VERIFIED | Line 18: `func ValidateName(name string) error` |
| `tui/tui.go` | `Run() error` entry point calling `buildTUIDeps` + `tea.NewProgram` | VERIFIED | Lines 19-30 complete; calls `buildTUIDeps()`, `newRootModel`, `tea.NewProgram(m)`, `p.Run()` |
| `tui/model.go` | `rootModel` v2-correct `View() tea.View`, push/pop stack, initializer interface (CR-01) | VERIFIED | Line 137: `func (m rootModel) View() tea.View`; initializer interface at line 25; push handler invokes `initScreen()` at line 94 |
| `tui/dashboard.go` | `dashboardModel` with `makeFamilyCmd`, 7-family async init, runID stale-guard | VERIFIED | `func makeFamilyCmd` at line 87; `init()` batches 7 cmds + 7 spinner ticks; stale guard at line 133 |
| `tui/identitylist.go` | `identityListModel` over `bubbles/v2 list.Model`, deps-threaded navigation | VERIFIED | `list.Model` field at line 28; `newIdentityListScreen(deps tuiDeps)` accepts full deps (CR-02) at line 43; `fillAccountPaths` wires managed paths at line 126 |
| `tui/identitydetail.go` | `identityDetailModel` with e/h/c actions + pubLine caching (WR-02) | VERIFIED | `newIdentityDetailModel` reads pubLine via `readPubLineForCopy` at line 53; e→updateForm, H→addAccountForm, c→clipboard |
| `tui/createform.go` | `createFormModel` with 8 textinput fields, `ValidateName` gate, path population (CR-03/CR-04) | VERIFIED | Line 140: `identity.ValidateName`; lines 161-188: populates all managed paths including `keyPath` via `keygen.KeyPaths` |
| `tui/prove.go` | `proveModel` with `runPreWriteCmd/runResolvedCmd` as tea.Cmds, confirm gate, action-dispatching `runWriteCmd` (CR-03) | VERIFIED | `runPreWriteCmd`/`runResolvedCmd` are tea.Cmd closures (lines 99-122); `runWriteCmd` switches on action (lines 137-163); confirm gate at line 211 |
| `tui/copy.go` | `runClipboardCopyCmd` + `renderCopyOverlay` with `upload.Instructions` | VERIFIED | `clipboard.Copy` at line 15; `upload.Instructions` at line 34 |
| `tui/deps.go` | `buildTUIDeps()` returning `(doctor.Deps, identity.Deps, identity.UpdateDeps, error)` | VERIFIED | Signature confirmed at line 28 |
| `tui/keymap.go` | `var keys` with all bindings | VERIFIED | Plan must_have confirmed present; used throughout screens |
| `tui/styles.go` | lipgloss v2 style tokens via `lipgloss.NewStyle()` (no `NewRenderer`) | VERIFIED | `grep -n "NewRenderer" tui/styles.go` → no matches |
| `cmd/gitid/copy.go` | `runCopy`, `newCopyCmd`, `newIdentityCopyCmd`, `newHostAddCmd` | VERIFIED | All four functions present; `runCopy` calls `clipboard.Copy` + `upload.Instructions` |
| `cmd/gitid/main.go` | `noArgsAction` + TTY branch + `tui.Run()` + alias registrations | VERIFIED | `term.IsTerminal` at line 40; `tui.Run` at line 41; all aliases registered in `newRootCmd` |

---

### Key Link Verification

| From | To | Via | Status | Details |
|------|---|-----|--------|---------|
| `cmd/gitid/main.go main()` | `tui.Run()` | `noArgsAction(isTTY, tui.Run, ...)` on `len(os.Args)==1` | WIRED | Line 38-42; `term.IsTerminal` guards the TTY branch |
| `cmd/gitid/copy.go runCopy` | `internal/clipboard.Copy` + `internal/upload.Instructions` | Direct calls after reconstruct | WIRED | Lines 113 + 128; `TestRunCopyOutputContainsKeyAndInstructions` PASS |
| `newRootCmd` | `newCopyCmd / newHostAddCmd / rotate alias / newIdentityCopyCmd` | `root.AddCommand(...)` / `identity.AddCommand(...)` | WIRED | Lines 100-119 in main.go |
| `tui/model.go pushScreenMsg` | `initializer.initScreen()` | Optional interface check on push | WIRED | Lines 93-95; `TestPushInvokesProveInit` PASS (CR-01) |
| `tui/dashboard.go` | `tui/identitylist.go` via full `tuiDeps` | `pushCmd(newIdentityListScreen(m.deps))` at line 163 | WIRED | Full deps threaded; `TestDepsThreadedEndToEnd` PASS (CR-02) |
| `tui/identitylist.go` | `tui/createform.go` via `tuiDeps` | `pushCmd(newCreateFormScreen(m.deps))` at line 88 | WIRED | Deps propagated to form; proven in `TestDepsThreadedEndToEnd` |
| `tui/createform.go` | `tui/prove.go` via `keyPath` (not SSHConfigPath) | `newProveScreen("create", in, identity.Account{}, keyPath, m.deps)` at line 188 | WIRED | `keygen.KeyPaths` derives real key path; `TestProveKeyPathIsPrivateKeyNotSSHConfig` PASS (CR-04) |
| `tui/prove.go runWriteCmd` | `identity.Create / Update / AddAccount` | Switch on action; `identity.Update` via `deps.update`, `identity.AddAccount` via `deps.identity` | WIRED | Lines 137-163; `TestRunWriteCmdDispatchesUpdate`, `TestRunWriteCmdDispatchesAddAccount` PASS (CR-03) |
| `tui/prove.go` | `tester.PreWrite / tester.Resolved` | Inside `tea.Cmd` closures only (never in update()) | WIRED | Lines 99-122; `TestProveScreenRunsPhasesAsTeaCmds` PASS; Pitfall 7 honored |
| `tui/dashboard.go makeFamilyCmd` | Per-family `doctor.Deps.Check*` fields | Switch on family; never `doctor.Run` | WIRED | Lines 89-104; `grep -n "doctor.Run" tui/dashboard.go` → comment only |
| `tui/identitydetail.go` | `tui/prove.go` via update/add forms | `newUpdateFormModel(m.account, m.deps)` → prove; `newAddAccountFormModel(m.account, m.deps)` → prove | WIRED | Lines 109-113; `TestIdentityDetailEditKey` PASS |

---

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|--------------|--------|--------------------|--------|
| `tui/dashboard.go` | `m.findings[fam]` | `makeFamilyCmd` → real `doctor.Deps.Check*` fields → `familyResultMsg` | Yes — each Check* calls into internal/doctor/checks | FLOWING |
| `tui/identitylist.go` | `accounts` / `m.list` items | `identity.Reconstruct(sshBytes, gcBytes, ...)` from real `~/.ssh/config` + `~/.gitconfig` via `deps.doctor.ReadFile` | Yes — reads real config files; falls back to `d.Identities` in test mode | FLOWING |
| `tui/prove.go` | `phase1Result`, `phase2Result` | `runPreWriteCmd(deps.identity.PreWrite, keyPath, ...)` → `tester.PreWrite`; `runResolvedCmd(deps.identity.Resolved, alias)` → `tester.Resolved` | Yes — real SSH exec in production; injectable seam in tests | FLOWING |
| `tui/prove.go` (write) | `writeResultMsg.backupPath` | `runWriteCmd` → `identity.Create/Update/AddAccount(deps)` → returns `CreateResult.SSHBackup` | Yes — real filewriter-backed write in production (WR-05 fixed) | FLOWING |
| `tui/identitydetail.go` | `m.pubLine` | `readPubLineForCopy(acct.PubPath, deps)` → `deps.doctor.ReadFile(resolved)` | Yes — reads real .pub file via ReadFile seam; `TestDetailPubLineCachedFromPub` PASS (WR-02 fixed) | FLOWING |
| `cmd/gitid/copy.go` | `pubLine` | `os.ReadFile(acct.PubPath)` → `clipboard.Copy(pubLine)` + `upload.Instructions(provider)` | Yes — reads real .pub, calls real clipboard | FLOWING |

---

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| All packages build | `go build ./...` | exit 0, no output | PASS |
| Full test suite with race detector | `go test -race ./...` | 14 packages ok (all cached) | PASS |
| Lint produces 0 issues | `make lint` | `0 issues.` | PASS |
| Wiring tests (CR-01..CR-04 + WR-02/WR-05) | `go test ./tui/... -run TestPushInvokesProveInit|TestDepsThreadedEndToEnd|TestRunWriteCmdDispatchesUpdate|TestRunWriteCmdDispatchesAddAccount|TestProveKeyPathIsPrivateKeyNotSSHConfig|TestDetailPubLineCachedFromPub` | 6/6 PASS | PASS |
| Completion scripts | `go test ./cmd/gitid/... -run TestCompletionBash|TestCompletionZsh|TestCompletionFish` | 3/3 PASS | PASS |
| CLI command registration | `go test ./cmd/gitid/... -run TestNewRootCmd|TestNoArgsAction` | 6/6 PASS | PASS |
| Dashboard async streaming | `go test ./tui/... -run TestDashboardInit|TestDashboardFamilyResult|TestDashboardRefresh|TestDashboardStaleResult` | 4/4 PASS | PASS |
| Navigation push/pop | `go test ./tui/... -run TestDashboardEnterNavigates|TestIdentityListEscPops|TestWindowSizePropagation|TestQuitFromAnyScreen` | 4/4 PASS | PASS |
| Prove screen confirm gate and failure path | `go test ./tui/... -run TestProveScreenConfirmGate|TestProveScreenPhase1Failure|TestProveScreenRunsPhasesAsTeaCmds|TestProveConfirmWritesViaDeps` | 4/4 PASS | PASS |
| Form Tab/Shift+Tab + ValidateName gate | `go test ./tui/... -run TestFormTabNavigation|TestCreateFormNameValidation|TestIdentityDetailEditKey|TestIdentityDetailCopyAction|TestDetailDeleteHandoff` | 5/5 PASS | PASS |

---

### Probe Execution

No probe scripts declared in this phase's PLAN files. Step 7c: SKIPPED (no probe-*.sh files declared or present for this phase).

---

### Requirements Coverage

| Requirement | Source Plans | Description | Status | Evidence |
|------------|-------------|-------------|--------|---------|
| CLI-01 | 05-01, 05-02, 05-03, 05-04 | Cobra CLI exposes `doctor`, `identity add/list/test`, `host add`; Phase 5 adds `copy`, `rotate` alias, `identity copy` | SATISFIED | `newRootCmd` registers all; `TestNewRootCmdTopLevelAliases`, `TestNewRootCmdIdentityCopyRegistered` PASS |
| CLI-02 | 05-02 | CLI generates shell completion for bash, zsh, fish | SATISFIED | `TestCompletionBash`, `TestCompletionZsh`, `TestCompletionFish` PASS |
| TUI-01 | 05-01, 05-02, 05-03 | Bubble Tea TUI launches into doctor dashboard; no-args entry | SATISFIED | `tui/tui.go:Run()` wired; `newRootModel` seeds dashboard; `TestDashboard*` PASS |
| TUI-02 | 05-01, 05-03, 05-04 | From dashboard, navigate to identity/account managers (list, detail, forms) | SATISFIED | Full nav chain wired; `TestDepsThreadedEndToEnd`, `TestIdentityDetailEditKey`, `TestProveScreen*` all PASS; CR-01..CR-04 fixed |

No orphaned requirements — all 4 phase requirements are covered by the 4 plans.

---

### Anti-Patterns Found

No TBD / FIXME / XXX markers were found in phase-modified files (read-verified by direct file inspection across all tui/*.go, cmd/gitid/copy.go, cmd/gitid/main.go, internal/upload/upload.go, internal/identity/validate.go).

| File | Pattern | Severity | Impact |
|------|---------|----------|--------|
| `tui/identitylist.go:148-149` | Comment-only stubs: `// newCreateFormScreen is provided in tui/createform.go (05-04)` | Info | Documentation only — the actual `newCreateFormScreen` and `newIdentityDetailScreen` functions are implemented in createform.go and identitydetail.go respectively. Not a stub. |
| `cmd/gitid/main.go:33` | `_ = out // out is reserved for future use` (IN-02 from code review) | Info | Unused parameter in `noArgsAction`. Accepted as a documented seam; code review classified it Info. No blocker. |
| `tui/deps.go` (IN-03 from code review) | Near-verbatim duplication of cmd/gitid deps wiring | Info | Maintenance hazard acknowledged. No extract to `internal/wiring` package yet. Code review classified it Info; no gate on this. |

No blockers or warning anti-patterns found.

---

### Human Verification Required

Six items require human testing (cannot be verified programmatically):

#### 1. TUI Launches to Doctor Dashboard

**Test:** Run `gitid` with no arguments in a real terminal (TTY). Observe that the Bubble Tea TUI launches immediately and the doctor dashboard is displayed as the first screen, with seven family panels visible (some may show spinner, some results).
**Expected:** Alternate screen activates; dashboard title "gitid — Doctor Dashboard" appears; seven family sections render with spinners or findings; `r` triggers a refresh.
**Why human:** Alt-screen TUI launch, spinner animation, and terminal rendering cannot be exercised by grep or unit tests without a real terminal.

#### 2. TUI Navigation (Dashboard → List → Detail → Form → Esc pops)

**Test:** From the TUI dashboard, press Enter to navigate to the Identity List. If identities exist, press Enter on one to navigate to Identity Detail. From detail, press 'e' to open the Update form and 'H' to open the Add-account form. Press Esc to pop back at each level.
**Expected:** Drill-down stack navigation works without leaving the app. Esc pops one level at each point. No crash or blank screen.
**Why human:** Interactive push/pop TUI navigation and screen rendering cannot be verified without a real TTY session.

#### 3. Create Form Tab/Shift+Tab and Invalid Name Error Rendering

**Test:** From the Identity List, press 'a' to open the Create Identity form. Tab through all 8 fields and Shift+Tab back. Fill the Identity Name with "test identity" (with a space) and press Enter — verify an inline error is shown and no navigation occurs.
**Expected:** Tab advances focus correctly through 8 fields; Shift+Tab retreats. Invalid name shows an inline error and blocks submission.
**Why human:** Tab/Shift+Tab focus ring and inline validation error rendering are TUI visual behaviors.

#### 4. Copy Action (Detail Screen 'c') and Clipboard Overlay

**Test:** Navigate to an identity that has a .pub key file. From Identity Detail, press 'c' and verify the clipboard receives the public key and the overlay shows upload instructions.
**Expected:** Copy action reads the .pub, copies to clipboard, overlay displays provider instructions.
**Why human:** Clipboard interaction and rendered overlay content require a real session and a clipboard tool to be present.

#### 5. Shell Completion Script Validity

**Test:** Run `gitid completion bash` and `gitid completion zsh` in a shell. Review or source the generated files to verify they are syntactically valid and define completions for registered subcommands.
**Expected:** Generated completion scripts are syntactically valid and complete known subcommands.
**Why human:** Script syntax correctness and real shell loading of completion cannot be verified by the unit tests (which only assert non-empty + contains "gitid").

#### 6. `gitid copy <name>` End-to-End

**Test:** Run `gitid copy <existing-identity-name>` in a terminal where clipboard is available. Verify the public key is copied and upload instructions are printed.
**Expected:** Output contains "Copied public key for ... to clipboard." and provider-specific upload steps.
**Why human:** End-to-end test of clipboard + upload instructions requires a real identity on disk and a clipboard tool.

---

### Gaps Summary

No gaps. All must-haves verified. The four ROADMAP success criteria and all four requirement IDs (CLI-01, CLI-02, TUI-01, TUI-02) are satisfied by the codebase evidence.

The code review blockers (CR-01..CR-04) and warnings (WR-01..WR-05) that were found during the mid-phase review were all addressed in commit a8bdf86 (`tui/wiring_test.go` added; `tui/model.go`, `tui/prove.go`, `tui/dashboard.go`, `tui/identitydetail.go`, `tui/identitylist.go` updated). All six fix-verification tests pass.

The three Info items from the code review (IN-01..IN-03) remain as tracked technical debt; none is a verification blocker.

---

_Verified: 2026-06-13T00:00:00Z_
_Verifier: Claude (gsd-verifier)_
