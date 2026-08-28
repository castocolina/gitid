# 08-06 SUMMARY — Render-layer gap closures: Health negative assertion, Fixer completeness, D-04 deep-link, D-16 batch halt

## Outcome

The cross-AI executor did NOT crash this time — it went quiet mid-run
instead, a new failure mode for this phase (previously only seen as instant
crashes in Waves 1/2/5). It stayed alive (visible in `ps`) but stopped
writing any file or log output for 19+ minutes with near-zero CPU and no
active build/test child process. Task 1 (Health negative assertion, Fixer
fixable-set completeness fixture) and Task 2 (D-04 per-identity health
deep-link, both TUI and CLI) were fully completed and committed-quality
before the stall; Task 3 (D-16 batch-walk halt) was left mid-progress.

Before touching any file, the stalled process (and its child processes —
`npx gsd-mcp-server` and the codegraph node helper) were killed and their
death confirmed via `ps`, after an `Edit` call failed with "file has been
modified since you last read it" revealed the process was still alive and
writing concurrently with the hand-recovery attempt. Task 3 was then
completed by hand, following this phase's established hand-recovery pattern
(read the uncommitted diff, cross-reference the plan's own task blocks,
finish what's missing).

## Task 1 — Health negative assertion + Fixer fixable-set completeness

Both fully done by the executor. `TestHealthNegativeAssertion`
(`internal/tuikit/health_screen_test.go`) asserts across all of Health's
render states (with-findings, all-green, per-identity, parse-error) that no
write-ceremony marker string ("Apply fix", "Confirm write", "Backed up ->",
"Wrote ->", "f ·") ever appears — Health is read-only end to end, including
any inherited render path. `wave2to5FixableFindings` / `TestFixerCompleteFixableSet`
(`internal/tuikit/fixer_screen_test.go`, new file) fixture-covers every
fixable finding shipped through Wave 5 (the D-09 IdentitiesOnly/IdentityFile
flagship, the D-05 gitignore pair, Permissions/Coherence/Orphans findings)
and proves the Wave 3/5 report-only checks (shadowed option,
directive-above-block, author-resolution, baseline-tolerance) are correctly
excluded from the fixable list.

## Task 2 — D-04 per-identity Health deep-link

Both TUI and CLI sides fully done by the executor. TUI:
`internal/tuikit/identities.go`'s `handleDetailKey` adds an `"h"` binding
that returns `keyResult{healthIdentity: sel.Name}`; `app.go`'s
`screenView.healthIdentity` field routes that result through `handleKey`/
`handleMouse` to switch to `TabHealth` scoped to the selected identity via
the pre-existing `store.FindingsFor`. CLI: `cmd/gitid/health.go` adds an
`--identity` flag with shell-completion (`RegisterFlagCompletionFunc`) and a
`findingsForIdentity` helper.

**D-04's "Claude's discretion" resolution, made consistently on both
sides**: global (empty-`IdentityName`) findings are EXCLUDED entirely from a
per-identity scoped view, not just de-prioritized — the same choice in both
`tuikit.FindingsFor` (TUI) and `findingsForIdentity` (CLI), documented
explicitly in both code comments. Rationale: a per-identity view answers
"is *this* identity healthy", and a global finding (e.g. missing baseline
gitignore) isn't attributable to any one identity, so surfacing it under an
arbitrary identity's scoped view would be misleading rather than helpful.

## Task 3 — D-16 batch-walk halt (hand-completed after the stall)

**Real pre-existing bug found and fixed en route**: `App.apply()` calls
`a.backend.Persist` SYNCHRONOUSLY within the same `Update` cycle — unlike
the globalssh/globalgit/identities ceremonies, which dispatch a `tea.Cmd`
and only later call `commitFailed`/`commitSucceeded` from an
asynchronously-delivered `Msg`. No async round-trip was needed for the D-16
halt check; a same-cycle post-`apply()` check in `handleKey`/`handleMouse`
suffices. Investigating this also surfaced that `fixCeremonyFor` builds its
ceremony with `cfg.Async` unset (false), and `ceremonyModel.handleKey`'s
confirm branch set `done=true` on the FIRST Enter for any non-async
ceremony — showing the receipt screen optimistically, before the real
`Persist` dispatch (the second Enter) ever ran. Since `view()` checks
`c.done` before `c.commitErr`, the pre-existing `commitFailed` (which
cleared `pending`/`commitErr`/`focus` but never `done`) could never make its
own retry/failure UI visible for a fix ceremony — it was silently
unreachable until this wave. Fixed in `internal/tuikit/ceremony.go`:
`commitFailed` now also clears `done`, and the retry-key branch sets
`done = true` (not `pending = true`) for non-async ceremonies so a retried
confirm reaches the receipt the same way the first attempt would have.

**Wiring**: `internal/tuikit/backend.go`'s `Backend` interface gains
`PersistError() error` — pre-existed only on `*realBackend`
(`cmd/gitid/wiring.go`), never consumed anywhere in `internal/tuikit`, a
genuine gap now closed. `app.go` captures `prevScreen := a.screens[a.tab]`
before dispatch in both `handleKey`/`handleMouse` and calls a new
`checkFixBatchHalt(prevScreen)` right after `a.apply(res.actions)`: on a
`PersistError`, it converts the current fixer screen into its halted state
via `fixerModel.haltBatch`; on success, it appends the just-finished fix's
name to `batchSucceeded` and advances. `fixer_screen.go`'s `haltBatch`
builds the exact frozen message: `"Fix %d of %d failed and was rolled back
from its own backup -- the first %d fixes already applied stand. Nothing
else in this batch was attempted."` and clears `m.batch` to nil so the
walk's queue cannot resume — the third fix is never attempted, not merely
paused. `pendingFixID`/`pendingFixName` are now actually set at the
`ceremonyFinished` dispatch point (the executor's own left-behind comment
described a nonexistent `fixApplyCheckMsg` async message type; corrected).

`TestBatchWalkHalt` (`internal/tuikit/fixer_screen_test.go`) drives a
3-finding batch (`fix-1` Critical, `fix-2` Error, `fix-3` Warning — strictly
descending severity so `orderedFindings`' stable sort makes the walk order
deterministic) through a `stubBackend` configured to fail exactly `fix-2`'s
`Persist`. Asserts: fix-1 succeeds and is removed from `state.Findings`,
`batchSucceeded == ["Fix One"]`, walk auto-advances to fix-2; fix-2's
`Persist` failure halts the batch (`batch == nil`), the ceremony's own
`commitErr` is set and rendered, the exact halt message renders (checked in
per-segment substrings since the rendered pane word-wraps the sentence at
100 columns), fix-2 remains in `state.Findings` (Persist returned state
unchanged), and fix-3 remains in `state.Findings` untouched (never
dispatched at all).

## Stall + kill + hand-recovery incident

Detecting the stall required more than `ps aux` — the process was alive
(not zombied, not crashed) but had stopped doing work. Confirmed via file
mtimes (`stat -f "%Sm"` on the worktree's most-recently-touched files) and
CPU% (near 0 for the executor and every child) staying flat for 19+ minutes
with no active build/test subprocess. Before any hand-edit, killed the
executor PID and its two children (`npx gsd-mcp-server`, the codegraph node
helper) and verified death via `ps` — this was necessary because one `Edit`
attempt against `fixer_screen_test.go` had already failed with "file has
been modified since you last read it," proving the still-alive process was
writing to the same file concurrently with the recovery attempt. After the
kill, the on-disk state of that file was re-read and found to already
contain a correct, equivalent implementation for the portion in progress
(a segment-based substring check), so it was kept rather than overwritten.

New standing lesson for future waves (added to this loop's dispatch
instructions): watch for a STALLED, not just crashed, executor —
`ps aux` alone under-detects it; check mtimes and CPU% too, and always kill
before hand-editing a worktree an executor might still be writing to.

## Verification

- `go build ./...` — exit 0.
- `go vet -tags e2e ./...` — exit 0.
- `TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./...` — 2144 passed in
  21 packages.
- `GOLANGCI_LINT_CACHE="$PWD/.golangci-cache" make lint` — 0 issues;
  worktree-local cache cleaned before and after.
- `make gate-visual-regression` — PASS.
- `make test` (includes `gate-copy-freeze`) — PASS.
- `make test-e2e` — PASS: `ok github.com/castocolina/gitid/e2e 602.260s`.
- Fresh-home check: `HOME=<fresh empty ~/.ssh> gitid health --identity
  nobody --json` -> `[]`, exit 0, no crash — the new `--identity` flag
  degrades gracefully when the identity doesn't exist.

`make test-e2e`/`make gate-visual-regression` rewrote 15
`06-global-ssh-options`/`07-global-git-options` UI-frame fixtures, but every
one of those diffs was PURE re-run noise (backup timestamps only, no content
change) — reverted via `git checkout --`, per this phase's established
"mixed diff kept whole, pure noise reverted" convention. The untracked
`WAVE6_PROMPT.md` dispatch scratch file was deleted, not committed.
