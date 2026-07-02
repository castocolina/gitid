---
phase: 05-cli-surface-tui
plan: "02"
subsystem: cli-surface
tags: [cobra, copy-command, completion, tui-launch, aliases, tdd, cli-01, cli-02]
dependency_graph:
  requires:
    - 05-01 (tui.Run, internal/upload.Instructions, internal/identity.ValidateName)
  provides:
    - cmd/gitid/copy.go — runCopy, newCopyCmd, newIdentityCopyCmd, newHostAddCmd
    - cmd/gitid/main.go — noArgsAction, no-args TTY→TUI branch, top-level aliases
    - completion — Cobra auto-registered bash/zsh/fish completion (CLI-02)
  affects:
    - cmd/gitid/main.go — TTY branch added; rotate/copy/host aliases registered
    - go.mod — charm.land/* promoted from indirect to direct
tech_stack:
  added: []
  patterns:
    - noArgsAction injectable helper (isTTY bool, run func() error) for testable no-args decision
    - Cobra top-level alias commands delegating to inner handlers (D-05/D-06/D-07)
    - runCopy CLIP-02 graceful degradation (clipboard fail → print key, continue)
    - RED stub under strict lint (zero-return stub satisfies typecheck; tests fail on output assertions)
key_files:
  created:
    - cmd/gitid/copy.go
    - cmd/gitid/copy_test.go
    - cmd/gitid/completion_test.go
  modified:
    - cmd/gitid/main.go
    - cmd/gitid/main_test.go
    - go.mod
    - go.sum
decisions:
  - "noArgsAction extracted as named helper (isTTY bool, run func() error, out, errw io.Writer) to enable unit-testing without real TUI or os.Exit"
  - "runCopy uses identity.ValidateName from internal/identity (T-05-05) instead of package-main identityNameRe, preserving the domain-layer validation contract"
  - "newHostAddCmd delegates directly to runAddAccount with bufio.NewReader(cmd.InOrStdin()) matching existing add.go pattern"
  - "charm.land/* promoted to direct deps after cmd/gitid imports tui (go mod tidy)"
metrics:
  duration: "18 min"
  completed: "2026-06-13"
  tasks: 2
  files_created: 3
  files_modified: 4
---

# Phase 05 Plan 02: CLI Surface + Completion Summary

Full gitid command surface wired: copy command, top-level aliases, no-args TTY→TUI launch, and shell completion verified for bash/zsh/fish.

## What Was Built

**cmd/gitid/copy.go:** Three new Cobra commands and one handler:
- `newCopyCmd()` — top-level `gitid copy <name>` (D-06)
- `newIdentityCopyCmd()` — canonical `gitid identity copy <name>` (D-06)
- `newHostAddCmd()` — `gitid host add` delegating to `runAddAccount` (D-07)
- `runCopy(out io.Writer, name string) error` — validates name via `identity.ValidateName` (T-05-05), reconstructs account via `identity.Reconstruct`, reads `.pub`, copies to clipboard (CLIP-02 graceful degradation), prints `upload.Instructions` (UP-02)

**cmd/gitid/main.go:**
- `noArgsAction(isTTY bool, run func() error, out, errw io.Writer) int` — extracted no-args decision helper; TTY=true calls `run()` (returns 0/1); TTY=false writes usage hint + returns 1 (TUI-01 non-TTY contract)
- `main()` now checks `len(os.Args) == 1` and branches: `term.IsTerminal(os.Stdout.Fd())` → `tui.Run()` or usage hint
- `newRootCmd()` registers: `rotate <name>` top-level alias (D-05), `copy <name>` top-level (D-06), `identity copy <name>` subcommand (D-06), `host` group + `host add` subcommand (D-07)

**Shell completion (CLI-02):** Cobra's auto-registered `completion` subcommand verified for bash, zsh, and fish — each produces non-empty output containing "gitid". No hand-written completion code needed.

**go.mod:** `charm.land/bubbletea/v2`, `charm.land/lipgloss/v2`, `charm.land/bubbles/v2` promoted from `// indirect` to direct deps after `cmd/gitid/main.go` imports `tui`.

## Verification Results

```
go test ./cmd/gitid/... -count=1 -run 'TestRunCopy|TestCopyCmd|TestIdentityCopy|TestHostAdd'
PASS  TestRunCopyInvalidName
PASS  TestRunCopyNotFound
PASS  TestCopyCmdUseString
PASS  TestIdentityCopyCmdUseString
PASS  TestHostAddCmdUseString
PASS  TestRunCopyOutputContainsKeyAndInstructions

go test ./cmd/gitid/... -count=1 -run 'TestCompletionBash|TestCompletionZsh|TestCompletionFish'
PASS  TestCompletionBash
PASS  TestCompletionZsh
PASS  TestCompletionFish

go test ./cmd/gitid/... -count=1 -run 'TestNewRootCmd|TestNoArgsAction'
PASS  TestNewRootCmdDoesNotPanic
PASS  TestNewRootCmdTopLevelAliases
PASS  TestNewRootCmdIdentityCopyRegistered
PASS  TestNoArgsActionNonTTY
PASS  TestNoArgsActionTTYSuccess
PASS  TestNoArgsActionTTYRunError

go test ./... -count=1
ok  github.com/castocolina/gitid/cmd/gitid
ok  github.com/castocolina/gitid/internal/clipboard
ok  github.com/castocolina/gitid/internal/deps
ok  github.com/castocolina/gitid/internal/doctor
ok  github.com/castocolina/gitid/internal/doctor/checks
ok  github.com/castocolina/gitid/internal/filewriter
ok  github.com/castocolina/gitid/internal/gitconfig
ok  github.com/castocolina/gitid/internal/identity
ok  github.com/castocolina/gitid/internal/keygen
ok  github.com/castocolina/gitid/internal/platform
ok  github.com/castocolina/gitid/internal/sshconfig
ok  github.com/castocolina/gitid/internal/tester
ok  github.com/castocolina/gitid/internal/upload
ok  github.com/castocolina/gitid/tui

go build ./... — PASS
```

## TDD Gate Compliance

- Task 1 RED: commit `b0b7c4e` — `test(05-02): add failing tests for copy command (Task 1 RED)`
- Task 1 GREEN: commit `c2f61e5` — `feat(05-02): implement copy/identity-copy/host-add commands (Task 1 GREEN)`
- Task 2 RED: commit `650e6ac` — `test(05-02): add failing tests for alias registration, no-args, completion (Task 2 RED)`
- Task 2 GREEN: commit `42ec8bc` — `feat(05-02): wire no-args TUI branch, top-level aliases, identity copy (Task 2 GREEN)`

All RED/GREEN gate commits present.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] RED stub required lint-safe approach**
- **Found during:** Task 1 RED commit
- **Issue:** The initial RED test file referenced `runCopy`, `newCopyCmd`, `newIdentityCopyCmd`, `newHostAddCmd` which were undefined — golangci-lint's `typecheck` failed the pre-commit hook.
- **Fix:** Created `copy.go` with RED stubs (real function signatures, body returns sentinel error `"copy: not implemented"`) so lint passes while tests fail genuinely on output assertions.
- **Files modified:** cmd/gitid/copy.go (stub created), then replaced in GREEN
- **Commit:** b0b7c4e (RED stub + tests)

**2. [Rule 1 - Bug] errcheck flagged fmt.Fprintln/Fprintf in noArgsAction**
- **Found during:** Task 2 GREEN commit (pre-commit hook failure)
- **Issue:** golangci-lint's `errcheck` flagged unhandled return values from `fmt.Fprintln` and `fmt.Fprintf` to `io.Writer`.
- **Fix:** Changed to `_, _ = fmt.Fprintln(...)` and `_, _ = fmt.Fprintf(...)` — consistent with the `fp()` helper pattern elsewhere in the codebase.
- **Files modified:** cmd/gitid/main.go
- **Commit:** 42ec8bc (GREEN with errcheck fix)

**3. [Rule 2 - Missing functionality] go mod tidy promoted charm.land to direct**
- **Found during:** Task 2 GREEN implementation
- **Issue:** 05-01-SUMMARY noted charm.land packages as `// indirect` because tui/ wasn't yet imported by cmd/gitid. After wiring `tui.Run()` import in main.go, `go mod tidy` correctly promoted them to direct deps.
- **Fix:** Ran `go mod tidy`; go.mod and go.sum updated.
- **Files modified:** go.mod, go.sum
- **Commit:** 42ec8bc

## Known Stubs

None — all functions in this plan are fully implemented. The RED stubs were replaced in GREEN.

## Threat Surface Scan

| Flag | File | Description |
|------|------|-------------|
| threat_flag: filesystem-read | cmd/gitid/copy.go | os.ReadFile on ~/.ssh/config, ~/.gitconfig, and the identity .pub path. All paths are gitid-managed trusted paths annotated //nolint:gosec (T-05-06). |

No new network endpoints, auth paths, or schema changes. The clipboard write (T-05-07) is accepted as public-key non-secret data per the threat model.

## Self-Check: PASSED

- cmd/gitid/copy.go: FOUND (func runCopy, func newCopyCmd, func newIdentityCopyCmd, func newHostAddCmd)
- cmd/gitid/copy_test.go: FOUND
- cmd/gitid/completion_test.go: FOUND
- cmd/gitid/main.go: FOUND (tui.Run(), term.IsTerminal, noArgsAction, rotate/copy/host aliases)
- cmd/gitid/main_test.go: FOUND (TestNewRootCmdTopLevelAliases, TestNoArgsActionNonTTY)
- Commit b0b7c4e: FOUND (test RED Task 1)
- Commit c2f61e5: FOUND (feat GREEN Task 1)
- Commit 650e6ac: FOUND (test RED Task 2)
- Commit 42ec8bc: FOUND (feat GREEN Task 2)
