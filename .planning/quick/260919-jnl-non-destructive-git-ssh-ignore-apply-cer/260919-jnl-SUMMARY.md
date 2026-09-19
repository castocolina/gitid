---
status: complete
quick_id: 260919-jnl
---

# Summary: Enter confirms Git/SSH/Ignore apply

**Commit:** 3dfdb15

## What changed

Git/SSH Options: Space still toggles selection; Enter now opens the apply preview (same as `a`). Ignore still uses `a` then the ceremony. Apply ceremonies use `newApplyCeremony` so Confirm starts focused and Enter writes without typing `yes`. Destructive delete/fixer ceremonies still require the typed word.

## Evidence

- RED: `go test ./internal/tuikit -run TestGlobalGitOptionsSpaceThenEnter|TestGlobalSSHOptionsSpaceThenEnter|TestGitIgnoreApplyCeremonyEnterConfirms` — 3 failed (Enter did not open apply; Ignore focus was Primary).
- GREEN: same tests pass; `TERM=dumb SSH_AUTH_SOCK= go test ./internal/tuikit -count=1` — 761 passed.
- `make lint` — 0 issues.

## Out of scope (still open)

Orange `!` on unset Git Options, copy/paste on Ignore, `e` vs Enter editor unification, CLI `Type "yes"`.
