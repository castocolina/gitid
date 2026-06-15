---
status: testing
phase: 05-cli-surface-tui
source: [05-VERIFICATION.md]
started: 2026-06-13
updated: 2026-06-13
---

## Current Test

number: 3
name: Create form focus ring + inline validation
expected: |
  In the Create form, Tab/Shift+Tab cycle field focus; invalid input (e.g. a bad
  identity name) renders an inline validation error and blocks submit.
awaiting: user response

## Tests

### 1. TUI launches to the doctor dashboard (TUI-01, SC-1)
expected: `gitid` (no args) enters alt-screen and opens on the doctor dashboard; the seven families stream in progressively with animated spinners; `r` refreshes.
result: pass

### 2. Drill-down navigation and Esc-pop (TUI-02, SC-2)
expected: From the dashboard, Enter drills Dashboard → Identity List → Identity Detail → add/edit form; Esc pops back up the stack one screen at a time; arrows + j/k/h/l move; `?` shows help; `q` quits.
result: pass (Enter/Esc drill+pop + detail nav verified) BUT `?` help is broken (G-03, functional defect). Styling/empty-state gaps G-01/G-02/G-04. Identity list correctly empty (no gitid-managed identities; CLI agrees).

### 3. Create form focus ring + inline validation (TUI-02)
expected: In the Create form, Tab/Shift+Tab cycle field focus; invalid input (e.g. a bad identity name) renders an inline validation error and blocks submit.
result: [pending]

### 4. Inline copy action in Identity Detail (D-06, CLIP-02)
expected: Pressing `c` in Identity Detail copies the identity's public key to the clipboard and renders the key preview overlay (non-empty).
result: [pending]

### 5. Shell completion scripts source cleanly (CLI-02, SC-3)
expected: `gitid completion bash | bash -n`, `gitid completion zsh` sourced in zsh, and `gitid completion fish | fish -c 'source -'` (on a machine with fish) all load without syntax errors.
result: [pending]

### 6. `gitid copy <name>` end-to-end (CLI-01, CLIP-02, UP-02)
expected: For a real existing identity, `gitid copy <name>` copies the public key to the clipboard via the real clipboard tool and prints the provider upload instructions.
result: [pending]

### 7. Prove-before-write in an in-app Create (D-04, SC-2) — optional, destructive
expected: Creating an identity through the TUI form shows the exact two-phase ssh test commands + their real output, notes the timestamped backup, and only enables the write confirm after both phases pass; the write routes through the proven core. (Only run against a throwaway identity — this mutates ~/.ssh and ~/.gitconfig with backup.)
result: [pending]

## Summary

total: 7
passed: 2
issues: 5
pending: 5
skipped: 0
blocked: 0
note: Tests 4 & 6 are BLOCKED on G-05 (no working new-identity create → no managed identity to copy/test). Test 7 is the substance of G-05.

## Gaps

- **G-01 (minor, UX):** TUI Identity List has no empty-state message. When no
  gitid-managed identities exist it renders blank; should show guidance (e.g.
  "No managed identities yet — press `a` to create one"). CLI already prints
  "no gitid-managed identities found." Found in Test 2. Fold into next-phase TUI.
- **G-02 (minor, UX) [CORRECTED]:** The dashboard DOES have a footer hint
  (`dashboard.go:251`: "q quit  Enter identities  r refresh  ? help") — my earlier
  "no hints" note was wrong. Real issue: it is too faint/linear (`StyleFaint`,
  space-separated) and easily missed. Subsumed by G-04 (styling).
- **G-03 (moderate, FUNCTIONAL DEFECT):** the `?` Help key is defined in
  `keymap.go:43` but never handled in any screen's Update — pressing `?` does
  nothing. Test 2 expected "`?` shows help". 3rd recurrence of the
  binding/seam-defined-but-unwired pattern. Wire a help overlay (the keymap
  already carries WithHelp text). Fix as a Phase-5 quick gap OR fold into the
  next-phase TUI rework.
- **G-04 (minor, UX/styling):** hint/help bars across screens (dashboard, forms,
  detail) are "muy lineal" — `StyleFaint`, space-separated, no emphasis. Suggested:
  bold/accent the key tokens, comma-separate options. Next-phase TUI polish.
- **G-05 (HIGH, create-flow design — Phase 2):** new-key creation does NOT match
  the intended workflow "generate → show upload instructions → WAIT → loop-test
  until auth PASS → or quit". Current flow (cmd/gitid/add.go + identity.runPipeline)
  asks "Write all four artifacts now?" BEFORE upload, runs the connectivity test
  ONCE, treats `ReachableNotUploaded` ("Permission denied (publickey)") as
  good-enough, and either writes blindly (on `y`) or writes nothing (on default
  `N`). It never verifies a real `PASS` (authenticated) before committing — so
  prove-before-write only proves *reachability* for new keys. Found via CLI
  `identity add` during UAT 2026-06-13. Blocks Tests 4 & 6 (need a created
  identity) and is the substance of Test 7. Also confusing: default-N write prompt
  + temp staging key make it look like "nothing happened / key vanished."
  Fix = redesign create into an upload→wait→retry-until-PASS-or-quit loop (CLI +
  TUI). Candidate for its own gap-closure or the next phase's "verify existing /
  useful workflow" scope.
  **[RESOLVED by Phase 5.5 — FIX-CREATE-01]** The CLI create-flow was redesigned
  to the auth-gated shape: key written to `~/.ssh` up front; loop test→authenticated
  PASS→auto-persist with retry/skip(typed-confirm)/quit; the "Write all four
  artifacts now?" pre-test prompt removed; `ReachableNotUploaded` no longer counts
  as success. Proven by `e2e/create_e2e_test.go` (PASS-gate, quit-keeps-key,
  skip-confirm, denied→pass). TUI wiring of the same flow remains for Phase 5.6.
- **Note (next-phase scope, not a Phase 5 defect):** user expected hand-written
  Host blocks (github.com/gitlab.com/bitbucket.org) and existing keys to be
  visualized; current TUI surfaces only gitid-managed identities. Confirms the
  "real visualization TUI" scope.
