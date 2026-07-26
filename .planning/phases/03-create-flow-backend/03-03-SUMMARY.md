---
phase: 03-create-flow-backend
plan: 03
subsystem: real-binary-composition-root
tags: [teardown, wiring, storage, doctor-reserved, L2, L4]
requires:
  - "03-01 (identity.Deps.ReadPub seam, keygen.ScanReusableKeys, tester.ResolvedViaCommand)"
  - "03-02 (internal/tuikit render stack + Backend seam + view DTOs)"
provides:
  - "cmd/gitid/wiring.go — the REAL tuikit.Backend composition root"
  - "cmd/gitid/main.go — bare `gitid` opens the real app shell (D-15)"
  - "sshconfig.IsReservedPath / ReservedPaths / ManagedBlockNames (L4 registration + Include-aware discovery)"
  - "sshconfig.IsReservedBlockName covering the macOS `_global` block"
affects:
  - "03-04 / 03-05 (render deltas layer onto these seams)"
  - "03-06 (PTY e2e closes the L2 obligation; the golden gate consumes the divergences flagged below)"
  - "Phase 8 D-06.2 (consumes sshconfig.ReservedPaths)"
  - "Phase 6 D-08 (`_global` -> `global-ssh` rename hand-off)"
tech-stack:
  added: []
  patterns:
    - "Composition root as the single backend<->view conversion boundary"
    - "Reflection guard over an injected-Deps struct (fails naming the field)"
    - "Include-aware managed-block discovery as the L4 prerequisite for a fix path"
key-files:
  created:
    - cmd/gitid/wiring.go
    - cmd/gitid/wiring_test.go
    - internal/doctor/checks/reserved_test.go
  modified:
    - cmd/gitid/main.go
    - cmd/gitid/main_test.go
    - cmd/gitid/debug.go
    - internal/sshconfig/include.go
    - internal/sshconfig/include_test.go
    - e2e/ui_pty_e2e_test.go
    - .planning/REQUIREMENTS.md
decisions:
  - "The POC e2e suite was archived with the surface it drove (deviation, Rule 3) — it exercised commands D-14 deletes"
  - "Task 1 + Task 2 + Task 3 landed as ONE commit: the teardown does not compile without the composition root that replaces it"
  - "Persist has no error channel; failures are recorded on the backend (PersistError) rather than swallowed — the render is 03-05's"
metrics:
  duration: "~1 session"
  completed: 2026-07-25
---

# Phase 3 Plan 03: Real Binary Entry Point + Backend Composition Root Summary

The real `gitid` binary now opens the approved tuikit shell driven by a live
Backend that reads and writes the user's actual configuration, the 0.0.1 POC
command surface and TUI are archived, and every managed SSH artifact this phase
creates is registered doctor-reserved and proven non-destructible.

## What Was Built

### Task 1 — D-14 archive + the LEGACY-TRIAGE Phase-3 DROP

Removed from the module as ONE build-safe unit (they are mutually importing, so
no subset compiles alone):

- `cmd/gitid/{add,addrepo,adopt,baseline,copy,delete,doctor,list,match,rotate,test,update,upload}.go` + their tests
- the entire pre-redesign `tui/` package
- `internal/repoclone/` (the POC "clone a repo into a managed dir" engine)

Everything is preserved under
`.planning/archive/0.0.1-poc-product-features-in-tui/{cmd-gitid,tui,internal-repoclone,e2e}/`.

**KEPT and untouched:** `internal/adopter` (the existing-Include detection ENGINE
behind D-05) — `git status --porcelain internal/adopter/` is empty. `debug` (+
`debug caps`) and Cobra `completion` are the whole remaining CLI surface.

`cmd/gitid/main_test.go` swapped its archived-command assertions for the inverse
guard: `TestNewRootCmdArchivedPOCCommandsAreGone` plus
`TestNewRootCmdSurfaceIsDebugAndCompletionOnly`, which asserts the WHOLE
remaining surface so a POC command cannot silently reappear.

### Task 2 — D-15 rewire + the real Backend composition root

`cmd/gitid/main.go`'s TTY branch now runs
`tea.NewProgram(tuikit.NewApp(buildBackend())).Run()`. The non-TTY usage-hint
branch and the thin `main()->Execute()` indirection are unchanged; the
`doctorExitCode` plumbing went with the archived doctor.

`cmd/gitid/wiring.go` is the real `tuikit.Backend`:

| Backend effect | Wired to |
|---|---|
| `AlgorithmCatalog` | `keygen.Catalog()` + `keygen.ResolveAvailability` over `platform.ProbeKeyTypes()` |
| `ProviderDefaults` | `identity.DefaultHostname` / `identity.DefaultPort` (no re-derived table in cmd/gitid) |
| `HostBlockPreview` | `sshconfig.RenderHostBlock` — the same renderer the write uses |
| `IncludeIfPreview` | `gitconfig.RenderIncludeIf` |
| `AliasCollision` | `sshconfig.ManagedBlockNames` (Include-aware) + every parsed Host pattern |
| `ScanReusableKeys` | `keygen.ScanReusableKeys(~/.ssh)` + inventory-derived `InUseBy` |
| `Stage1Command` / `TestStage1` | `tester.PreWriteCommand` / `identity.Deps.PreWrite` |
| `Stage2Command` / `TestStage2` | `tester.ResolvedViaCommand` / `StageTestConfig` + `ResolvedVia` |
| `CopyPublicKey` | `identity.Deps.ReadPub` + `clipboard.Copy` (`.pub` line only) |
| `Persist(AddIdentity)` | `identity.PersistSSH` into the resolved layout, then a fresh inventory read |

**L2 real-constructor half (Codex HIGH #1).** `identity.Deps.ReadPub` is wired to
a real `os.ReadFile` of the `.pub`, returning the trimmed line and wrapping
failures as `gitid: reading public key <path>: %w`.
`TestIdentityDepsEveryFieldIsWired` reflects over every `identity.Deps` func
field and fails **naming the field**, so a future field addition fails loudly
instead of being silently skipped;
`TestReadPubIsWiredToARealImplementation` proves the field is not merely
non-nil but actually reads from disk. The remaining half — driving the
encrypted-key reuse path through the real binary over a raw-keystroke PTY — is
plan 03-06's.

**View-DTO boundary (Codex HIGH #5).** `toReusableKeyViews` and
`toTestResultView` live only here. All three outcome mappings are covered
(`PASS`, `ReachableNotUploaded`, `Failure`) and the classification itself is
never re-derived from an exit code. Verified: `go list -deps ./internal/tuikit`
names no first-party backend package.

### Task 3 — D-05/06/08 persist

- **D-05 auto-detect, no in-flow choice.** An existing gitid-owned Include'd
  target (via `sshconfig.Adopt`, or the canonical `config.d/gitid.config`) wins;
  otherwise existing in-file identity blocks keep the in-file layout; otherwise
  the machine is fresh.
- **D-06 Include'd default on fresh.** `EnsureIncludeDir` (0700) +
  `EnsureIncludeLine` (floored near the top) + the Host block written into
  `~/.ssh/config.d/gitid.config`. `CreateWritePlan` reports **both** file changes
  with dynamic `~/`-shortened paths — never a hardcoded `~/.ssh/config`. Reserved
  wiring blocks alone do NOT pin the in-file layout.
- **D-08 macOS globals.** `RenderGlobalBlock(platform.CurrentOS())` is carried on
  `CreateInput.GlobalBlock` on every create; `sshconfig.Write` rewrites it
  idempotently after the specific host. Asserted present on darwin, absent off it.
- **SSHUI-04.** Both stages run against a throwaway staging config in a 0700
  temp dir; `TestStagedTestConfigLeavesLiveConfigUntouched` asserts the live
  `~/.ssh/config` is byte-identical pre-confirm.
- **TEST-03 / STORE-04.** Every write routes through `internal/sshconfig` ->
  `internal/filewriter`; there is no `os.WriteFile` in `wiring.go`. Backups and
  foreign-content preservation are asserted.
- **D-01 store gate at the backend seam.** `PASS` and `ReachableNotUploaded`
  unlock the write; only a hard `Failure` blocks it, and a blocked create writes
  nothing and records why.
- `.planning/REQUIREMENTS.md` §F STORE-01 now names the Include'd layout as the
  fresh-setup DEFAULT with an inline `(default superseded by Phase 3 D-06)` note;
  the traceability row reads `Phase 1, Phase 3 (D-06 default supersession)`.

### Task 4 (BLOCKING, L4) — doctor-reserved registration

- `sshconfig.IsReservedBlockName` now covers `_global` alongside `ssh-include`.
  **Additive only** — the `_global` -> `global-ssh` rename stays Phase 6 D-08 and
  `internal/sshconfig/writer.go` is unchanged.
- New `sshconfig.ReservedPaths(sshDir)` / `IsReservedPath(sshDir, path)` register
  the gitid-owned Include'd storage (`config.d` + its `*.config` glob), compared
  on `filepath.Clean`ed values.
- New `sshconfig.ManagedBlockNames(configPath)` — Include-AWARE discovery
  unioning `~/.ssh/config`'s blocks with every Include'd file's. Missing Include'd
  files are skipped; a match that exists but cannot be read errors as
  `sshconfig: reading included <path>: %w`.
- `internal/doctor/checks/reserved_test.go::TestOrphansReservedArtifactsSurviveFix`
  applies **every** `Fix.Fn` `CheckOrphans` offers over an Include'd fake home and
  asserts the Include line, `config.d/gitid.config` and the `_global` block are
  byte-identical afterwards. The non-Include-aware control
  (`TestOrphansNonIncludeAwareDepsAreDestructive`) asserts the opposite, so the
  guard cannot rot into a tautology.

**RED was proven, not assumed.** With the `_global` registration temporarily
reverted, the fix path reported
`SSH Host block "_global": no gitconfig includeIf` and **deleted the macOS
globals block from `~/.ssh/config`** — the exact destructive false-positive loop
L4 exists to prevent.

**Hand-offs recorded:** Phase 8 D-06.2 generalizes `sshconfig.ReservedPaths` into
the cross-cutting reserved-PATH registry; Phase 6 D-08 owns the `_global` ->
`global-ssh` rename. Both are documented in the doc comments.

## Gate Results (real commands, real output)

```
$ go build ./...
(no output — exit 0)

$ TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./...
ok  github.com/castocolina/gitid/cmd/gitid            1.576s
ok  github.com/castocolina/gitid/cmd/gitid-dummy      1.985s
ok  github.com/castocolina/gitid/internal/adopter     2.546s
ok  github.com/castocolina/gitid/internal/clipboard   1.674s
ok  github.com/castocolina/gitid/internal/deps        2.239s
ok  github.com/castocolina/gitid/internal/doctor      2.760s
ok  github.com/castocolina/gitid/internal/doctor/checks 3.027s
ok  github.com/castocolina/gitid/internal/dummytui    3.470s
ok  github.com/castocolina/gitid/internal/filewriter  3.889s
ok  github.com/castocolina/gitid/internal/gitconfig   5.798s
ok  github.com/castocolina/gitid/internal/identity    4.687s
ok  github.com/castocolina/gitid/internal/keygen     17.290s
ok  github.com/castocolina/gitid/internal/platform    3.580s
?   github.com/castocolina/gitid/internal/screenshot  [no test files]
ok  github.com/castocolina/gitid/internal/sshconfig   6.768s
ok  github.com/castocolina/gitid/internal/tester      3.642s
ok  github.com/castocolina/gitid/internal/tuikit      6.670s
ok  github.com/castocolina/gitid/internal/upload      3.756s
ok  github.com/castocolina/gitid/internal/uploader    3.739s
(796 tests, 19 packages, 0 failures)

$ make lint
/Users/ramon/go/bin/golangci-lint run ./...
0 issues.

$ make test-e2e
go build -o bin/gitid ./cmd/gitid
go test -tags e2e -race -timeout 180s ./e2e/...
ok  github.com/castocolina/gitid/e2e   20.624s

$ go test ./internal/dummytui/ -run TestNoBackendAllowlist -v
=== RUN   TestNoBackendAllowlist
--- PASS: TestNoBackendAllowlist (0.16s)
PASS
ok  github.com/castocolina/gitid/internal/dummytui  0.451s

$ go list -deps ./internal/tuikit | grep -E 'castocolina/gitid/internal/(identity|tester|sshconfig|keygen|filewriter|doctor|adopter|platform|clipboard|uploader)'
(no output; grep exit 1 — no matches, as required)

$ ./gitid --help
Available Commands:
  completion  Generate the autocompletion script for the specified shell
  debug       Print diagnostic information about the local environment (D-08)
  help        Help about any command
```

Test count moved 1122 -> 796 because the archived POC packages (`tui/`,
`internal/repoclone`, the POC `cmd/gitid` commands) took their tests with them.

## Deviations from Plan

### 1. [Rule 3 - Blocking] The POC e2e suite was archived with the surface it drove

- **Found during:** Task 1
- **Issue:** `e2e/{addrepo,adopt,create,match,overlap,upload,install}_e2e_test.go`
  invoke the archived Cobra commands (`identity add`, `adopt`, `doctor`, `copy`,
  `match`, `add repo`) by shelling out to the built binary, and
  `e2e/ui_pty_e2e_test.go`'s test functions drove the removed `tui/` modals
  (wizard, match-strategy selector, adopt modal, add-repo modal, copy/upload
  assist). None of them can pass once their target is gone, so `make test-e2e`
  would have been red from this wave until 03-06.
- **Fix:** archived those files under
  `.planning/archive/0.0.1-poc-product-features-in-tui/e2e/`. The PTY **harness**
  (`ptySession`, `startPTY`, `startPTYAt`, `sendKey`, `waitFor`, `snapshot`,
  `saveFrame`, `newPTYCmd`, `uiReady`) was deliberately kept in
  `e2e/ui_pty_e2e_test.go` — 03-06 explicitly says to reuse it, not reinvent it.
- **Note:** 03-06's plan already owns "remove `e2e/create_e2e_test.go`"; it will
  find it already archived. `e2e/debug_e2e_test.go` and
  `e2e/dummy_demo_e2e_test.go` are untouched and still pass.
- **No surviving assertion was weakened or deleted** — only tests of deleted
  features moved with their features.

### 2. [Rule 2 - Missing critical coverage] Added `TestUIPTY_RealShellBoots`

- **Found during:** Task 1
- **Issue:** with the POC PTY tests archived, nothing proved this plan's headline
  truth ("bare `gitid` in a TTY opens the real approved app shell"), and the
  surviving harness had no caller.
- **Fix:** a minimal boot smoke test opening the REAL binary at the approved
  100x30 geometry (D-04) and asserting all four nav tabs render and the shell
  keeps decoding raw keystrokes. It deliberately does NOT duplicate 03-06's
  per-screen suite.
- **Bug it caught immediately:** the first run failed with
  `Terminal too small — resize to at least 100x30` — the harness's default 80x24
  is below the tuikit minimum. Fixed by using the existing `startPTYAt` at 100x30.

### 3. Task 1 + 2 + 3 landed as ONE commit

- **Why:** the plan requires the teardown to land as one commit that compiles and
  passes the pre-commit hooks (never `--no-verify`). `main.go` cannot compile
  after `tui.Run` is removed until `buildBackend()` exists, and the persist
  effect lives inside the same `wiring.go`. CLAUDE.md's "let the buildable
  boundary set commit granularity" applies directly.
- Task 4's `internal/sshconfig` + doctor work landed FIRST as its own commit,
  because `wiring.go` consumes `sshconfig.ManagedBlockNames`.

### 4. `fp()` helper re-homed

- `cmd/gitid/debug.go` used `fp()`, which the archived `add.go` happened to own.
  Moved verbatim into `debug.go`, its only surviving caller.

### 5. `Persist` has no error channel — failures are recorded, not swallowed

- `tuikit.Backend.Persist(state, action) DemoState` returns no error. A failed
  write returns the PREVIOUS state (the user's config is never reported as
  changed when it was not) and records the cause on `realBackend.persistErr`,
  exposed via `PersistError()`. Surfacing it in the ceremony is 03-05's render
  work. This is called out so it is not mistaken for a swallow.

## Flagged for the 03-06 visual-regression allowlist

Real-backend values legitimately differ from the frozen dummy fixtures. These
are data/format divergences, not design changes, and 03-06's golden gate needs
allowlist entries or fixture-equivalent handling:

| Surface | Dummy | Real backend | Why |
|---|---|---|---|
| Host-block preview indent | 4 spaces, no provider marker | 2 spaces + `# gitid: provider=…` | `sshconfig.RenderHostBlock` is what actually gets written; "written exactly like this on confirm" requires the preview to equal the written text |
| `TestConfigPath()` | fixture constant | real `os.MkdirTemp` path | the staged config is a real throwaway file (SSHUI-04) |
| Stage 1/2 command strings | fixture strings | `tester.PreWriteCommand` / `ResolvedViaCommand` | shown == run (TEST-01) |
| `ProviderDefaults("gitlab.com")` | `gitlab.com:22` | `altssh.gitlab.com:443` | D-20 recipe-canonical alt-SSH pairing |
| Identity list / storage target | fixtures | the user's real config | D-15 |

## Known Stubs

None that block this plan's goal. Two intentional, phase-scoped placeholders:

- `InitialState().Findings` is empty — the Doctor tab is not wired in Phase 3 and
  carries the D-16 demo banner (`DemoBanner` returns true for every tab except
  Identities). Doctor wiring is a later phase.
- `Persist` performs a real write only for `AddIdentity` (the SSH leg,
  `identity.PersistSSH`); the Git leg is Phase 4 (D-18: a skipped Git step stores
  an SSH-only, incomplete identity). Other actions still reduce in memory so the
  banner-marked screens keep working.

## Hand-offs

- **Phase 8 (D-06.2):** consumes `sshconfig.ReservedPaths` as the SSH-side seed of
  the cross-cutting reserved-PATH registry.
- **Phase 6 (D-08):** owns the `_global` -> `global-ssh` rename;
  `internal/sshconfig/writer.go` was deliberately left unchanged here.
- **Plan 03-06:** closes the remaining L2 half (encrypted-key reuse through the
  real binary over a raw-keystroke PTY) and consumes the divergence table above.
- **Plan 03-05:** surfaces `realBackend.PersistError()` and the
  `ReachableNotUploaded` warning state in the render.

## Self-Check: PASSED

- `cmd/gitid/wiring.go` — FOUND
- `cmd/gitid/wiring_test.go` — FOUND
- `internal/doctor/checks/reserved_test.go` — FOUND
- `.planning/archive/0.0.1-poc-product-features-in-tui/cmd-gitid/add.go` — FOUND
- `.planning/archive/0.0.1-poc-product-features-in-tui/tui/model.go` — FOUND
- `.planning/archive/0.0.1-poc-product-features-in-tui/internal-repoclone/repoclone.go` — FOUND
- commit `e75e32d` — FOUND
- commit `d60a4d7` — FOUND
