# 08-02 SUMMARY — Flagship fix-in-place: D-09 surgical rewrite, D-10/D-13/D-14, `gitid fix`

## Outcome

All 3 tasks complete. The Fixer can now rewrite exactly ONE existing
directive on a hand-written (non-gitid-managed) `Host` stanza — the
`IdentitiesOnly no -> yes` flagship contradiction — through a full
ceremony: real diff, typed confirm, timestamped backup, mandatory
parse-render-re-parse + real `ssh -G` re-verification, automatic restore on
any mismatch. Every fix (not only the flagship) now re-runs the full
`doctor.Run(deps)` scan after applying and replaces `state.Findings` with
the fresh result; a finding that survives its own successful fix is
replaced with a convergence-alarm finding and withdrawn from re-offer for
the rest of the session. `gitid fix [--yes] [--dry-run]` applies the same
real write path the TUI ceremony uses, capped by a `maxPasses` backstop.

This wave was executed as a **hand-recovery**, same pattern as Wave 1: the
cross-AI executor (`opencode`/`local-llm-env`) crashed on the same
banned-`/tmp`-write permission wall mid-Task-1, despite an explicit
instruction not to. I (the orchestrator) reviewed its already-generated
code (found sound), wrote the missing tests and the CLAUDE.md amendment,
then implemented Tasks 2 and 3 directly.

## Task 1 (commit `7c725ad`)

- `internal/sshconfig/reader.go`: `HostBlockFacts` + `ParseAllHostBlocks`
  parse every `Host` stanza — managed and hand-written — with an explicit
  `*bool` `IdentitiesOnly` state (nil is never conflated with explicit
  `false`) and the directive's line number.
- `internal/doctor/doctor.go`: `Finding.Rewrite` (`*FixRewrite`) carries the
  D-09 target so the cmd layer can render a real diff and typed confirm
  without re-deriving them from prose; `Deps.AllHostBlocks` is the new
  check's data source.
- `internal/doctor/checks/coherence.go`: `checkHandWrittenIdentitiesOnly`
  flags a hand-written stanza with explicit `IdentitiesOnly no` + a
  non-empty `IdentityFile` — never double-reporting a gitid-managed block
  (`coherenceForAccount`'s existing Check 3 already covers that case).
- `internal/sshconfig/rewrite.go`: `RewriteHostDirective` (surgical
  single-line splice, re-locates the target at write time, CR-18
  control-byte validation, writes through the `filewriter` chokepoint),
  `ApplyVerifiedHostDirective` (D-10: round-trip stability + real `ssh -G`
  re-verification when available, degrading to the round-trip check alone
  when `ssh` is absent, auto-restore from backup on any mismatch), and
  `DiffHostDirective` (the true before/after diff).
- `CLAUDE.md`: scoped D-09 amendment documenting this one narrow exception
  to the managed-blocks-only write rule.

**Test-design lesson (caught empirically):** the D-10 auto-restore
regression test initially used an unterminated-quote corruption to force a
verification failure — `kevinburke/ssh_config`'s parser tolerated it
silently, so the test never actually exercised the restore path. A NUL byte
(genuinely unparseable) was needed; verified the test fails without the
fix and passes with it before trusting it.

## Task 2 (commit `53add56`)

- `internal/tuikit/backend.go`: new `Backend.FixPlanFor(finding)` seam —
  every `internal/tuikit` call site (`fixCeremonyFor`, `fixer_screen.go`'s
  three `PlanFor` call sites) now routes through it.
- `internal/tuikit/store.go`: `DemoFinding.Rewrite *FixRewriteTarget` (a
  tuikit-local mirror of `doctor.FixRewrite` — tuikit imports zero
  `internal/doctor` identifiers).
- `internal/dummytui/fixturebackend.go`: `FixtureBackend.FixPlanFor`
  delegates UNCHANGED to the frozen free `PlanFor` — the Phase-2
  visual-regression fixture is untouched.
- `cmd/gitid/wiring.go`: `realBackend.FixPlanFor` reads the ACTUAL
  `~/.ssh/config` and renders the true diff via `sshconfig.DiffHostDirective`
  for a `Rewrite`-carrying finding, falling back to `PlanFor` otherwise.
  `realBackend.Persist`'s `FixFinding` case (`persistFixFinding`) is no
  longer demo-only: locates the raw `doctor.Finding` by stable ID, calls its
  `Fix.Fn`, re-runs the full scan (D-13), and replaces `state.Findings`. A
  finding whose signature survives its own successful fix becomes the D-14
  alarm and is tracked in a new session-scoped `realBackend.convergenceAlarmed`
  set so it is never re-offered.
- `.planning/design/fixer/FIELDS.md`: D-11 correction — the confirm-destructive
  row's stale "short of a typed confirmation" language replaced with the
  actual, already-implemented typed-hostname confirmation.

**Real bug found and fixed (empirically):** the D-13 re-scan initially
reused the pre-fix `doctor.Deps` object. `buildDoctorDeps` reads every
config file EAGERLY (not lazily), so reusing it for the re-scan silently
re-read the SAME stale bytes the fix had just changed on disk — the fixed
finding never disappeared. Caught by `TestPersistFixFindingAppliesRealRewrite`
failing before the fix (`deps := buildDoctorDeps(b.home)` → build a FRESH
one for the re-scan instead of reusing the captured `deps` var).

4 new tests, all real-fixture-driven (a genuine hand-written `Host` block, a
real rewrite, a real re-scan). The convergence-alarm test needed a new
`fixFnOverride` test-only seam (mirrors this file's existing `failCommitAt`
precedent) since every REAL check's `Fix.Fn` either genuinely fixes the
condition or genuinely fails — only a test double can report success while
changing nothing.

## Task 3 (commit `0981ab5`)

D-13/D-14 landed with Task 2 (the same code path `realBackend.Persist`'s
`FixFinding` case owns). This commit's own scope:

- `cmd/gitid/fix.go`: `gitid fix [--yes] [--dry-run]` walks fixable
  findings one at a time — print real diff (via `Backend.FixPlanFor`),
  apply via the SAME cmd-layer precedence `doctor.go`'s `FixDescriptor` doc
  comment documents (`Fix.Interactive` preferred over `Fix.Fn` when
  non-nil, threading stdin/stdout/`assumeYes`), re-run the full scan after
  EVERY apply. `fixMaxPasses = 10` backstop (the convergence-loop algorithm
  ported from the archived POC's `convergeFixes`, adapted to this wave's
  one-at-a-time apply+rescan shape per the plan's own action text).
  `--dry-run` previews every fixable finding's diff and writes nothing.
- Registered in `main.go`, replacing the `fix` reserved-noun stub — the
  LAST one (both `health` and `fix` are now real), so the now-fully-unused
  `newReservedNounCmd` helper and its dedicated test were removed; the
  `reservedNoun()` parity-matrix detection function stays (still live
  infrastructure).
- `docs/cli-parity-matrix.md`: Fix row promoted from deferred to shipped.

**Discovery (working as intended, not a bug):** a manual `gitid fix --yes`
run against a home with no baseline installed hit `maxPasses` — `CheckBaseline`'s
`Fn`-only fallback (when `SetupBaseline` is nil, the current real wiring)
restores only the baseline-include pointer, never the fragment file
`ReadBaselineState` also requires, so it genuinely never converges. This is
a documented, pre-existing incompleteness (`baseline.go`'s own comment,
`SetupBaseline` wiring is a future addition) — `maxPasses` existed exactly
to catch this class of disagreement, and it did, correctly, matching the
plan's own `<done>` criterion verbatim. Test fixtures now seed a
fully-installed baseline (`seedInstalledBaseline`) to isolate each test's
own assertion from this out-of-scope limitation.

5 new tests. The Interactive-fix and maxPasses tests use a new
`fixScanOverride` seam (mirrors `persistFixFinding`'s `fixFnOverride`
precedent) since no real check currently produces an
Interactive-without-`Fn` finding or a genuinely non-convergent `Fn`.

## Gates (every command run for real by the orchestrator; none trusted from
a self-report)

- `go build ./...` — exit 0 (checked after every task).
- `TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./...` — **2074 passed**,
  0 failed, 21 packages (final count after all 3 tasks; started at 2046
  from Wave 1's baseline, +20 Task 1, +4 Task 2, +4 Task 3, −1 removed
  reserved-noun test net of the CLI-registration fallout it and the parity
  matrix required).
- `GOLANGCI_LINT_CACHE=$PWD/.golangci-cache make lint` (cache cleaned after
  each run, plus `golangci-lint cache clean` when a stale sibling-worktree
  cache reference surfaced) — **0 issues** at every checkpoint. Fixed
  real findings along the way: an unused-parameter revive finding, a gosec
  G301 on a deliberately-loose-permissions test fixture, a gosec G302 on a
  directory `chmod 0700` (correct for a directory, gosec's default ceiling
  is file-oriented), and a `goimports` formatting drift after a function
  removal.
- `go list -deps ./internal/tuikit` — no `internal/doctor` import (no-backend
  gate holds throughout).
- `make gate-visual-regression` — PASS at every checkpoint (~54-55s each).
- `make test` (includes `gate-copy-freeze`) — PASS, 88.9s.
- `make test-e2e` — **PASS**, 597.96s — no rename-ripple this wave (unlike
  Wave 1's 4 rounds); the repo-wide grep sweep for stale `FixFinding`/
  `PlanFor`/`fixCeremonyFor` call sites confirmed clean before and after.
- Manual `gitid fix --yes`/`gitid health --json` runs against a real,
  deliberately-misconfigured fixture home (a hand-written
  `Host clientb.github.com` block) proved the flagship end-to-end through
  the real CLI binary: the rewrite applied, byte-diffed clean against every
  other line, and a real `ssh -G` afterward resolved `identitiesonly yes`.

## Files modified

`internal/sshconfig/reader.go`, `internal/sshconfig/rewrite.go` (new),
`internal/sshconfig/rewrite_test.go` (new), `internal/doctor/doctor.go`,
`internal/doctor/checks/coherence.go`, `internal/doctor/checks/coherence_test.go`,
`internal/tuikit/backend.go`, `internal/tuikit/backend_stub_test.go`,
`internal/tuikit/store.go`, `internal/tuikit/fixer_screen.go`,
`internal/tuikit/identities.go`, `internal/dummytui/fixturebackend.go`,
`cmd/gitid/wiring.go`, `cmd/gitid/wiring_test.go`, `cmd/gitid/fix.go` (new),
`cmd/gitid/fix_test.go` (new), `cmd/gitid/main.go`, `cmd/gitid/identity.go`,
`cmd/gitid/identity_test.go`, `docs/cli-parity-matrix.md`,
`.planning/design/fixer/FIELDS.md`, `CLAUDE.md`.

## Next Phase Readiness

Wave 3 (08-03-PLAN.md: D-06 tolerance fixes + HLTH-02 parse gates) can
proceed. The `fixFnOverride`/`fixScanOverride` test-seam pattern established
this wave (mirroring the project's existing `failCommitAt` precedent) is
reusable for any future wave needing to reproduce a "the fix reports
success but doesn't actually change anything" scenario without a flaky
real-world race.
