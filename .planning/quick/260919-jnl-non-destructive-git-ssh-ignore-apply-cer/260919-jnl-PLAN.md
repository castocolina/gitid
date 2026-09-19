---
id: 260919-jnl
slug: non-destructive-git-ssh-ignore-apply-cer
status: active
---

# Quick: Enter confirms Git/SSH/Ignore apply (no typed yes)

User 2026-09-19: Space + Enter confirms Git/SSH/Ignore toggles; do not type `yes`. Keep the exact-change preview and silent backup. Typed-confirm stays only for destructive rewrites.

Orchestrator wrote this plan after `gsd-planner` Task spawn failed (`task_id` must start with `ses`).

## Task 1 — TDD: Space then Enter opens apply; Enter confirms without a word

Files: `internal/tuikit/globalgit_test.go`, `internal/tuikit/globalssh_test.go`, `internal/tuikit/gitignore_test.go`, `internal/tuikit/ceremony.go`, `internal/tuikit/globalgit.go`, `internal/tuikit/globalssh.go`, `internal/tuikit/gitignore.go`

Action:
1. Add failing tests: Git/SSH Options Space then Enter opens the apply ceremony; empty selection Enter does not; apply ceremony `Destructive == nil` and default focus is Confirm so a second Enter confirms without typing. Ignore: `a` then Enter confirms without a typed word; Confirm is default-focused.
2. Bind Options-list Enter to the existing `a` apply path (do not steal editor/filter/custom-form/storage Enter).
3. Git/SSH/Ignore apply ceremony builders set focus to Confirm (scoped exception to §6 Cancel-autofocus). Do not change delete/fixer/identity-edit ceremonies.
4. Footer may advertise `a/Enter` when a selection exists. Keep `a` as an alias.

Verify: `TERM=dumb SSH_AUTH_SOCK= go test ./internal/tuikit -count=1 -run 'TestGlobalGitOptionsSpaceThenEnter|TestGlobalSSHOptionsSpaceThenEnter|TestGitIgnoreApplyCeremonyEnterConfirms|TestCeremonyDestructiveAffirmativeNeverDefaultFocused'`
Done: those tests pass; `go test ./internal/tuikit -count=1` passes; `make lint` is 0.

Out of scope: classify.go orange `!`, copy/paste, `e` vs Enter editor unification, CLI `Type "yes"`.
