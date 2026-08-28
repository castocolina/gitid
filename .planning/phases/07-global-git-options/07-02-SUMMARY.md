---
phase: 07-global-git-options
plan: 02
type: summary
wave: 2
---

# 07-02 Summary — two-field fallback author, anchored block, D-06 precedence proof

## What was built

Three tasks, three commits. The fallback author is now two independent fields whose empty state means unset, whose block sits between the floor include and the first `includeIf`, and whose precedence is re-proven on the real machine after every write.

### Task 1 — `InsertBlockAfter` and the fallback-author block owner (`7485bd8`)

`internal/filewriter/block.go` gained `InsertBlockAfter`, the positional sibling of `PrependBlockIfNotFound`. A plain `git config --global user.email` appends, so on a recipe-shaped `~/.gitconfig` the new `[user]` section lands AFTER the `[includeIf …]` blocks and hijacks every identity's author (D-05). Position is the whole point of this function; a missing anchor returns the input bytes plus an error naming the anchor. The primitive does not create anchors — that is the ceremony's job (R-4).

`internal/gitconfig/fallbackauthor.go` is the ONE owner of the fallback-author managed block: `GitFallbackAuthorBlockName = "global-git-author"`, `EnsureGitFallbackAuthor` (compose both halves / name-only / email-only, remove when both empty), `ReadGitFallbackAuthor` (round-trip). Empty values are omitted, never written as empty-string keys. `IsReservedBlockName` was extended in the same commit, and the doctor reserved-survival / orphans fixtures gained a block under this name so the doctor's destructive fix path can never delete it (project learning L4, T-07-13).

### Task 2 — the write ceremony and the D-06 post-write precedence proof (`a6c1d83`)

`internal/globalgit/authorresolve.go` is read-only and runs through the SAME injected `RunGitConfig` seam plan 07-01 established. `VerifyAuthorResolution` reports per-key per-directory value and origin; an empty `matchedDir` is `MatchedNotVerifiable`, never indistinguishable from a pass.

`cmd/gitid/lifecycle.go`'s `runGitFallbackAuthorApply` is a SECOND verb, not a mode of `runGlobalGitApply`. `lifecycleStages["global-git-author"]` is `plan, confirm, backup, write, verify`. WRITE (R-4): one read of `~/.gitconfig`, `ComposeBaselineInclude` then `EnsureGitFallbackAuthor` over the same bytes, ONE `filewriter.Write`. The first-run sequence has no ordering dependency on the options screen. A no-op (empty pair over no block) short-circuits before backup; an authorized write that happens to produce identical bytes still takes one.

`cmd/gitid/wiring.go` added `GitFallbackAuthorState` / `GitFallbackAuthorPlan` / `CommitGitFallbackAuthor` and the compile-time assertion `var _ tuikit.GitFallbackAuthorPlanner = (*realBackend)(nil)`. A reflection test asserts `realBackend` does NOT embed `NoopGitFallbackAuthorPlanner`.

### Task 3 — the two-field pane and its own ceremony instance (`c589dc2`)

`GitFallbackAuthorPlanner` is a SEPARATE interface from `GlobalGitPlanner` — that is the structural expression of D-05's "own dedicated ceremony, never folded into the baseline block". The fallback detail pane is two independently focusable `formFieldLine` rows (name, email), seeded from `GitFallbackAuthorState()` on `activate()`. The checkbox is gone. The apply guard is the 07-UI-SPEC.md resolved "partial" row: offer `a` when the email is empty-or-valid AND (a field is non-empty OR the current block is non-empty — the removal case). A comment on the guard names plan 07-05 so nobody later "fixes" the TUI into per-field flags or the CLI into a snapshot. Confirming the fallback ceremony dispatches `CommitGitFallbackAuthor` and never `CommitGlobalGit`; the reverse is also true.

`GlobalGitNameFallbackKey = "user.name (global fallback)"` is registered in `gate-copy-freeze` alongside its sibling. The six existing D9 copy constants were not rewritten (D-04 is a behavioural amendment).

## D-06 evidence — matched vs unmatched `git config --show-origin --get user.email`

Hermetic temp HOME, recipe-shaped config (floor include + fallback block + `includeIf gitdir:~/git/work/` pointing at a fragment that sets `work@example.com`), a real git repository inside the matching directory, and a second directory that is not a repository:

```
MATCHED (cwd = <home>/git/work/repo)
  stdout: 'file:<home>/.gitconfig.d/work\twork@example.com\n'
  stderr: ''
  rc: 0

UNMATCHED (cwd = <home>/unmatched)
  stdout: 'file:<home>/.gitconfig\tfallback@example.com\n'
  stderr: ''
  rc: 0
```

The matched read names the identity fragment. The unmatched read names the main config. That is the permanent proof D-06 asks for.

## R-4 evidence — missing include target is silent

Hermetic temp HOME, floor include pointing at a NONEXISTENT `~/.gitconfig.d/00-baseline`, plus the fallback block. Observed:

```
$ git config --show-origin --get user.email
file:<home>/.gitconfig	fallback@example.com
err: nil
```

git silently ignores a missing include target. No compensating baseline-file creation was required. The ceremony therefore writes exactly one file.

## First-run `~/.gitconfig` (no floor include beforehand)

```
# BEGIN gitid managed: baseline-include
[include]
	path = ~/.gitconfig.d/00-baseline
# END gitid managed: baseline-include
# BEGIN gitid managed: global-git-author
[user]
	name = Pat Example
	email = pat@example.com
# END gitid managed: global-git-author
```

The floor include is the first managed block; the fallback block sits immediately after it. Both written in one `filewriter.Write`.

## Composed fallback block — four field combinations

On the recipe-shaped fixture (floor include, then two `includeIf` sections), both-halves compose produces:

```
# BEGIN gitid managed: global-git-author
[user]
	name = Pat Example
	email = pat@example.com
# END gitid managed: global-git-author
```

Offsets on that fixture: floor-include end-marker at byte 85, fallback begin-marker at byte 123, first `[includeIf` at byte 283. The block sits strictly between them.

Name-only omits the email key entirely:

```
# BEGIN gitid managed: global-git-author
[user]
	name = Pat Example
# END gitid managed: global-git-author
```

Email-only omits the name key entirely:

```
# BEGIN gitid managed: global-git-author
[user]
	email = pat@example.com
# END gitid managed: global-git-author
```

Both-empty removes the block: no `global-git-author` name and no orphaned begin- or end-marker line remain.

## Injected-failure restore output

`TestRunGitFallbackAuthorApply_InjectedFailureRestores` injects at `global-git-author-after-write` after a seeded `# user-written\n[core]\n\teditor = vim\n` preamble. The write is rolled back; `assertUnchanged` confirms byte/mode equality with the pre-transaction snapshot. Restore outcomes logged:

```
restore outcomes: [~/.gitconfig: restored]
```

## Dummy fallback pane — before / after (plan 07-06 visual-divergence allowlist)

BEFORE (07-01, one field + checkbox):

```
   user.email (global fallback) [                  ]
     Fallback author for repos no identity matches. Identities always override this through their includeIf fragment — setting it never changes an identity's author.
     Recipes leave this unset by default. Set it only if you want a catch-all author for unmatched repos.
```

Master-list row rendered a checkbox glyph (`☐` / `☑`) and responded to space.

AFTER (07-02, two fields, no checkbox):

```
   user.name (global fallback)  [                  ]
   user.email (global fallback) [                  ]
     Fallback author for repos no identity matches. Identities always override this through their includeIf fragment — setting it never changes an identity's author.
     Recipes leave this unset by default. Set it only if you want a catch-all author for unmatched repos.
```

Master-list row renders no checkbox glyph and ignores space. Tab moves focus between the two fields; Enter starts text-edit on the focused field. This is an intended, documented design amendment (D-04), not a regression.

## `gate-copy-freeze` entries added

```
'user.name (global fallback)'
'user.email (global fallback)'
```

The six existing D9 copy constants (`GlobalGitEmailFallbackHelper`, `Advisory`, `CeremonyHeading`, `DiffAnnotation`, `ResultMessage`, and the original email key) were already frozen and were not rewritten.

## Deviations

- **D-07-02-1 — R-4 / findGitWorkTree**: `includeIf gitdir:` matching only fires inside a repository. The verify stage's first draft `os.Stat`'d the pattern directory itself (`~/git/work`), which exists but is not a repo, so the matched half would have been unverifiable even when a child repo was present. `findGitWorkTree` now returns the directory if it is a git work tree, or a direct child that is. Pinned by `TestRunGitFallbackAuthorApply_VerifyNoPrecedenceAdvisory`.
- **D-07-02-2 — Task 2/3 seam split**: the plan listed `GitFallbackAuthorPlanner` under Task 3's files, but Task 2's conversion site and compile-time assertion need the interface to exist. The interface, noop, and view DTOs landed in Task 2's commit (`a6c1d83`); Task 3 embedded the interface in `Backend`, wired the pane, and implemented the dummy/stub. The real backend still does not embed the noop (reflection test in `wiring_test.go`).
- **D-07-02-3 — `MatchedNotVerifiable` advisory on machines with no managed identity**: the verify stage appends `"advisory: matched-identity author resolution could not be verified on this machine"` when no gitdir match exists. That is the plan's required unverifiable outcome, not a pass.
- **D-07-02-4 — No compensating baseline-file creation (R-4)**: git silently ignores a missing include target (quoted above). The ceremony therefore writes exactly one file, as designed.

## Review

- **R-4 (cycle 1)**: first-run missing-anchor failure — mitigated by composing `ComposeBaselineInclude` into the same byte stream before `InsertBlockAfter`. Pinned by `TestRunGitFallbackAuthorApply_FirstRunCreatesAnchor`.
- **D-04 / 07-UI-SPEC.md partial row**: two independent fields, empty means unset, apply is a pair snapshot. The TUI/CLI surface difference is documented on the apply guard, naming plan 07-05.
- **D-05**: own dedicated ceremony, never folded into the baseline block. `runGitFallbackAuthorApply` and `runGlobalGitApply` are distinct functions and neither calls the other (`TestRunGitFallbackAuthorApply_DistinctFromGlobalGitApply`). Confirming one ceremony never dispatches the other's commit (`TestGitFallbackCeremonyHeadingAndCommitAreDistinct`, `TestGitFallbackBaselineCeremonyNeverDispatchesFallbackCommit`).
- **D-06**: post-write matched/unmatched origins quoted above.
- **T-07-10 / T-07-43 / T-07-13 / T-07-14**: placement offset, first-run single write, reserved registration in the same commit as the block owner, unconditional backup once authorized with no-op short-circuit. All pinned by tests.
- **Cycle-1 TUI/CLI consistency flag**: left open as a comment, not a code change. Both surfaces call `EnsureGitFallbackAuthor` with a resolved pair.

## Task commits

1. **Task 1: InsertBlockAfter and the fallback-author block owner** — `7485bd8`
2. **Task 2: fallback-author write ceremony and D-06 precedence proof** — `a6c1d83`
3. **Task 3: two-field fallback pane and its own ceremony instance** — `c589dc2`

## Files created/modified

- `internal/filewriter/block.go` — `InsertBlockAfter`
- `internal/filewriter/block_insert_test.go` — placement, update-in-place, missing-anchor, CRLF
- `internal/gitconfig/fallbackauthor.go` — block owner
- `internal/gitconfig/fallbackauthor_test.go` — four combinations, round-trip, placement, newline reject
- `internal/gitconfig/reader.go` — reserved registration
- `internal/doctor/checks/orphans_test.go`, `reserved_test.go` — reserved-survival fixtures
- `internal/globalgit/authorresolve.go` — D-06 probe
- `internal/globalgit/authorresolve_test.go` — hermetic real-git matched/unmatched + silent missing include
- `internal/globalgit/probe.go` — shared `stripFileOrigin`
- `cmd/gitid/lifecycle.go` — `runGitFallbackAuthorApply`
- `cmd/gitid/lifecycle_fallbackauthor_test.go` — ceremony acceptance
- `cmd/gitid/wiring.go` — conversion site + compile-time assertion
- `cmd/gitid/wiring_test.go` — no-embed + nil-guard
- `internal/tuikit/backend.go` — `GitFallbackAuthorPlanner` + noop, embedded in `Backend`
- `internal/tuikit/views.go` — view DTOs
- `internal/tuikit/design.go` — `GlobalGitNameFallbackKey`
- `internal/tuikit/globalgit.go` — two-field pane, apply guard, second ceremony
- `internal/tuikit/globalgit_test.go` — pane/ceremony acceptance
- `internal/tuikit/backend_stub_test.go` — stub implementations, no noop embed
- `internal/tuikit/store.go` — `GitGlobalName` on the pair-snapshot action
- `internal/dummytui/fixturebackend.go` — dummy seam
- `Makefile` — copy-freeze entries
