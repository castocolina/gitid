---
phase: 06-global-ssh-options
plan: 04
subsystem: global-ssh
status: complete
completed: 2026-08-26
commits:
  - a70cab3
  - 4d3eb0a
  - a75c444
  - pending
---

# Phase 06-04: Whole-Graph Shadowing and Real PTY Coverage Summary

**Global SSH applies now simulate the complete Include graph before writing, verify the live resolution afterward, and are covered by eight raw-keystroke workflows against the compiled real binary.**

## Performance
- **Tasks:** 3
- **Task commits:** 3
- **Final documentation commit:** this commit
- **E2E duration:** `make test-e2e` completed in 498.926s; the existing 900s timeout was sufficient and unchanged.

## Accomplishments
- Added recursive, cycle-safe whole-graph simulation and first-obtained-value shadow naming, with shared exported Host-pattern matching.
- Extended the single `runGlobalSSHApply` write authority with simulation and post-write verification, an empty opt-in selection, honest preview warnings, and receipt advisories.
- Added a real-binary PTY suite covering browse, empty selection, cancel, confirm, shadowed and non-shadowed precedence pair, inconclusive probing, and failure/retry.

## Task Commits
1. **Task 1: Whole-graph simulation, shadow naming, and host-pattern matcher export** — `a70cab3` (`feat(06-04): whole-graph simulation, shadow naming, and host-pattern matcher export`)
2. **Task 2: Wire simulation and verification into the single apply authority; D-15 empty selection** — `4d3eb0a` (`feat(06-04): wire simulate/verify stages into runGlobalSSHApply, D-15 empty selection`)
3. **Task 3: Raw-keystroke PTY coverage and fake SSH probe proof** — `a75c444` (`feat(06-04): add Global SSH real PTY coverage`)

**Plan metadata:** this commit (`docs(06-04): add plan summary`)

## Files Created/Modified
- `internal/globalssh/shadow.go` — records a missing, newly-created Include target in the simulation graph; fixes include-line parsing safety and lets mirror rewriting recognize graph-known yet not-yet-real Include paths.
- `e2e/harness_test.go` — `FakeSSHDir` global SSH mode handles plain `ssh -G`, isolated `ssh -G -F <path>`, and `ssh -V`; its dedicated test proves isolated answers vary with the supplied file.
- `e2e/global_ssh_pty_e2e_test.go` — eight real-PTY cases against the compiled `cmd/gitid` binary.
- `.planning/phases/06-global-ssh-options/ui-frames/` — committed evidence frames listed below.

## Verification
All checks passed from the repository root after Task 3:

- `go build ./...`
- `make lint`
- `TERM=dumb SSH_AUTH_SOCK= go test -race -count=1 ./...` — 1640 tests across 20 packages
- `make test-e2e` — passed in 498.926s
- `make gate-copy-freeze`

The focused TDD check also passed:

- `go test -tags e2e ./e2e -run 'TestFakeSSHDirGlobalSSHProbesReadTheirConfig|TestGlobalSSH_RealPTY' -count=1` — 9 tests

## PTY Coverage and Captured Frames
- `global-ssh-browse.txt`
- `global-ssh-empty-selection.txt`
- `global-ssh-apply-cancel.txt`
- `global-ssh-apply-confirm.txt`
- `global-ssh-shadowed-preview.txt`
- `global-ssh-shadowed-receipt.txt`
- `global-ssh-later-directive.txt`
- `global-ssh-probe-inconclusive-preview.txt`
- `global-ssh-probe-inconclusive-receipt.txt`
- `global-ssh-commit-failure.txt`
- `global-ssh-commit-retry.txt`

The shadowing pair differs only in the placement of `StrictHostKeyChecking no`: before the floored `Include` produces a preview warning and receipt advisory; below it produces neither, preserving OpenSSH's first-obtained-value rule.

## Technical Decisions
- The fake SSH reads the isolated `-F` config path and resolves first `Host *` values, including recursively included files. It never supplies a constant isolated result, avoiding vacuous simulation coverage.
- The mirror preserves a candidate target even when it does not yet exist on disk; this allows a fresh Include-layout apply to simulate the file that will be created.
- The simulation mirror root is created under the OS temporary directory with mode `0700`; mirrored files use `0600` and are removed after every simulation outcome.
- `HostPatternsMatch(patterns []string, candidate string) bool` and `HostLineMatches(patternLine, candidate string) bool` are the shared negation-aware matcher APIs. `aliasCollides` now projects parser patterns through `pat.String()` into this one implementation. Existing AliasCollision tests passed unmodified.
- Recursive Include discovery uses `active` as a popped recursion stack for true cycles and `expanded` as a never-popped memo for diamonds. `maxIncludeDepth` remains 8; cycles and over-depth graphs become inconclusive, and unresolved includes are redirected inside the mirror.

## Deviations from Plan

### Auto-fixed Issues

**1. `EnsureIncludeLine` pre-existing Include-cycle parse safety**
- **Found during:** Task 2 history
- **Issue:** The round-trip parse safety check rejected an otherwise valid write when the existing configuration already contained an unrelated Include cycle.
- **Fix:** Compare `Parse(composed)` with `Parse(existing)` and refuse only when gitid's own edit breaks a configuration that was previously parseable.
- **Verification:** `TestEnsureIncludeLinePreExistingCycleProceeds` and `TestRunGlobalSSHApplyInconclusiveSimulationPermitsWrite`.
- **Committed in:** `4d3eb0a`.

**2. Fresh Include-layout simulation omitted its not-yet-created managed target**
- **Found during:** Task 3 PTY TDD
- **Issue:** A new Include-layout target did not exist at graph discovery time, so mirror rewriting treated the Include as unresolved and the probe could not see the candidate.
- **Fix:** Append the absent managed target to `SimulationGraph.Files` and treat graph-known paths as resolved during Include rewriting.
- **Verification:** the clean confirm, shadowed, and paired later-directive PTY cases.
- **Committed in:** `a75c444`.

**3. Include-line matcher could slice empty fields**
- **Found during:** Task 3 shadowed PTY case
- **Issue:** `findIncludeLine` indexed a fields slice before checking that an Include line had a value.
- **Fix:** Guard `len(fields) < 2` before accessing the directive token.
- **Verification:** all focused global SSH PTY cases and `internal/globalssh` simulation tests.
- **Committed in:** `a75c444`.

## Issues Encountered
- The initial global fake resolved only the mirror itself, while post-write verification must reflect the live main-config precedence. The final fake intentionally models both invocation shapes and has a dedicated proof test.
- The existing 900-second E2E timeout remained adequate; no Makefile change was needed.

## User Setup Required
None. All tests use isolated sandbox homes and test-owned fake SSH executables.

## Next Phase Readiness
06-05 can build on a fully PTY-proven Options sub-tab and its actual rendered evidence frames. No changes were made to `.planning/STATE.md` or `.planning/ROADMAP.md`.

---
*Phase: 06-global-ssh-options*
*Completed: 2026-08-26*
