---
phase: 04-git-configuration-screen
plan: 03
status: complete
---

# Phase 04 Plan 03 Summary

Implemented the reusable Git-flow contract, including UI-local Git DTOs, exact SSH alias matching, empty SSH-only completion fields, default rewrite intent, and transactional real-backend writes with rollback receipts.

## Outcome

- Preserved the `tuikit.Backend` boundary: real, dummy fixture, and test stub backends implement asynchronous `CommitGit` without exposing backend types to the TUI.
- Added Git request/original/result DTOs, editable Git-directory projection, and changed-lines-only signer-email replacements in the reusable Git form.
- `hasconfig` matching now uses the configured SSH alias exactly; user-selected `gitdir` paths retain their trailing slash.
- Confirmed Git writes validate HOME containment and reject symlinked managed paths, write includeIf and signer blocks transactionally, and report restored targets on rollback.
- Managed allowed-signer updates replace the identity block using the exact `user.email` bytes rather than appending a stale principal.
- The standalone `paneGit` ceremony dispatches `CommitGit` after explicit confirmation and reduces `ConfigureGit` only after a successful `GitCommitMsg`; failures remain visible with rollback detail.
- `hasconfig` rendering no longer invents a fallback host when the validated SSH alias is absent.
- The Git mutation journal now materializes the selected contained `gitdir` path and removes it on injected rollback.
- Alias collision with an SSH-only or complete identity now routes Enter into that identity's own Git edit/completion flow (D-07 collision resume) instead of only blocking advance.
- Combined-create's rollback journal was unified with the standalone transaction path.

## Commits

| Commit | Description |
| --- | --- |
| `004c501` | feat(04-03): add async Git flow contract |
| `8a5a76e` | test(04-03): cover exact Git match inputs |
| `8ec66fc` | feat(04-03): preserve Git flow match inputs |
| `41de81b` | feat(04-03): make Git writes transactional |
| `4ac2792` | fix(04-03): commit standalone Git flow asynchronously |
| `10d7c81` | fix(04-03): reject synthetic Git match hosts |
| `3c5b0cb` | fix(04-03): complete reusable Git form |
| `3925b19` | fix(04-03): journal Git artifact mutations |
| `2d944cd` | fix(04-03): create selected Git directories |
| `4b4257c` | fix(04-03): unify combined create rollback journal |
| `ea94c14` | fix(04-03): resume Git flow from alias collisions |
| `da03009` | fix(identity): expand tilde before reading identity Git fragments (post-hoc correction, see Deviations) |

## Verification

| Command | Result |
| --- | --- |
| `go test -race -count=1 ./internal/tuikit ./cmd/gitid` | PASS — 338 tests |
| `make test` | PASS |
| `make lint` | PASS — 0 issues |
| `make test-e2e` | PASS (after `da03009`; see Deviations) |
| `go test -race -count=1 ./internal/tuikit -run TestStandaloneGitCeremonyCommitsBeforeConfigureGit` | PASS |

## Deviations from Plan

The first `make test-e2e` run after this plan's final commit (`ea94c14`) failed
`TestCreateFlow_SSHFormAliasCollision`: the new D-07 collision-resume message
reported a complete, fully-configured seeded identity as `"(SSH-only)"`.
Root cause was a **pre-existing** bug this plan's new `hasGitConfiguration`
branch exposed rather than introduced: `internal/identity/loader.go`'s
`Reconstruct` passed the identity's fragment path — always
`~/.gitconfig.d/<name>`, written verbatim by `IncludeIfPreview`/
`WriteIncludeIf` — straight to `readFrag`, which opens the file via
`os.Stat`/`exec.Command`. Neither expands `~`, so every real identity's
own fragment read-back silently failed and `GitName`/`GitEmail` never
populated. Fixed in `da03009` by expanding `~` (reusing the existing
`expandTilde` WR-02 helper) before the `readFrag` call only — the stored
`FragmentPath` itself stays verbatim, since other callers display it and
re-derive write targets from it. Added a regression test
(`TestReconstruct_FragmentPathTildeExpansion`) proven RED without the fix
and GREEN with it, corrected a Phase-4-added unit test that had baked the
buggy unexpanded path in as its expected value, and updated the affected
Phase-3 e2e test's assertions to match the new, correct D-07 message and
Enter-resumes-Git-edit behavior — a deliberate UX improvement over the
old block-only behavior, not a regression.

## Self-Check: PASSED
