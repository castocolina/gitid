---
status: testing
phase: 05-cli-surface-tui
source: [05-VERIFICATION.md]
started: 2026-06-13
updated: 2026-06-13
---

## Current Test

number: 1
name: TUI launches to the doctor dashboard
expected: |
  Running `gitid` with no arguments in a real terminal enters alt-screen mode and
  shows the doctor dashboard as the first screen, with the seven check families
  streaming in progressively and animated spinners while each loads.
awaiting: user response

## Tests

### 1. TUI launches to the doctor dashboard (TUI-01, SC-1)
expected: `gitid` (no args) enters alt-screen and opens on the doctor dashboard; the seven families stream in progressively with animated spinners; `r` refreshes.
result: [pending]

### 2. Drill-down navigation and Esc-pop (TUI-02, SC-2)
expected: From the dashboard, Enter drills Dashboard → Identity List → Identity Detail → add/edit form; Esc pops back up the stack one screen at a time; arrows + j/k/h/l move; `?` shows help; `q` quits.
result: [pending]

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
passed: 0
issues: 0
pending: 7
skipped: 0
blocked: 0

## Gaps
