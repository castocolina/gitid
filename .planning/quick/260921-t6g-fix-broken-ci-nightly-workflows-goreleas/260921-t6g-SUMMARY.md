---
id: 260921-t6g
slug: fix-broken-ci-nightly-workflows-goreleas
status: complete
type: execute
requirements: [BUILD-01, BUILD-02, BUILD-03, BUILD-04]
key-files:
  created:
    - cmd/gitid/goreleaser_pin_test.go
  modified:
    - Makefile
    - cmd/gitid/nightly_yml_test.go
    - cmd/gitid/goreleaser_config_test.go
    - e2e/release_homebrew_gate_e2e_test.go
    - .github/workflows/nightly.yml
    - .goreleaser.yaml
    - .planning/phases/10-linux-validation-release-pipeline/10-RESEARCH.md
decisions:
  - "Repinned goreleaser to v2.17.0 (Option A from the plan), not the task brief's original v2.17.1 — v2.17.1 still requires go >= 1.26.5, one patch above this Makefile's pinned go1.26.4 GOTOOLCHAIN, and would have reproduced the same failure. v2.17.0 is the newest tag whose own go.mod requirement (go 1.26.4) the exported GOTOOLCHAIN satisfies exactly. No GOTOOLCHAIN/go.mod/go-version: edit (Option B) was made — none was authorized."
metrics:
  duration: ~35min
  completed: 2026-09-21
actuals:
  tokens: 5232
  tasks: 2
  commits: 1
  plan_head_before: 88c37cc1b09d5cac515dc183a65e20977fa6f266
---

# Quick 260921-t6g: unbreak CI/Nightly/Release — repin goreleaser to v2.17.0 Summary

Repinned `GORELEASER_VERSION` from `v2.18.0` to `v2.17.0` in the Makefile —
the newest goreleaser release buildable under this repo's own deliberately
1.26-series exported `GOTOOLCHAIN` (`go1.26.4`) — added a network-free guard
test that fails `make test` the moment a future bump reintroduces an
incompatible pin, and swept every in-repo comment/test-failure-message that
named the stale `v2.18.0` literal so they can no longer drift out of sync
with the Makefile's own pin.

## What Was Built

**Task 1 (TDD, tracer):** `cmd/gitid/goreleaser_pin_test.go` —
`TestGoreleaserPinIsBuildableByPinnedToolchain`, a network-free test that
extracts `GORELEASER_VERSION` and the exported `GOTOOLCHAIN` from the
Makefile, compares the pinned goreleaser version's own upstream `go`
requirement (from a package-level, dated table sourced live from the Go
module proxy) against the toolchain as parsed major/minor/patch ints, and
fails with a message naming both concrete values and both Makefile
variables if the toolchain is too old to build the pin. A second subtest
(the "drift gate") fails if a future pin has no table entry at all, forcing
a bumper to fetch that version's `go` directive from the proxy before
changing the pin. Written and run RED first, then the Makefile pin was
changed to `v2.17.0` with its 4-line comment rewritten into a dated record
of the constraint; the test went GREEN.

**Task 2 (docs coherence sweep):** Appended a dated addendum to
`10-RESEARCH.md`'s `## Package Legitimacy Audit` (original audit text
untouched) recording the corrected pin, the live proxy verification table,
and the unchanged Approved verdict. Reworded `nightly.yml`'s header and
`.goreleaser.yaml`'s schema-key comment off the hardcoded `v2.18.0` literal.
Replaced every hardcoded-version comment/`t.Fatal` message in
`cmd/gitid/nightly_yml_test.go`, `cmd/gitid/goreleaser_config_test.go`, and
`e2e/release_homebrew_gate_e2e_test.go` with a reference to the Makefile's
`GORELEASER_VERSION` pin instead of a number that can go stale again.
`.goreleaser.yaml`'s `brews:` "functional through v2.18.0" note was left
byte-for-byte unchanged, as the plan required (it's a correct upper bound
that still covers v2.17.0).

## Observed Output (per the plan's `<output>` instruction)

### RED — guard test against the unchanged Makefile (still pinned v2.18.0)

Command: `go test ./cmd/gitid -count=1 -run 'TestGoreleaserPin' -v`

```
=== RUN   TestGoreleaserPinIsBuildableByPinnedToolchain
=== RUN   TestGoreleaserPinIsBuildableByPinnedToolchain/pinned_version_builds_under_the_pinned_toolchain
    goreleaser_pin_test.go:132: Makefile GORELEASER_VERSION := v2.18.0 requires go >= 1.27.0, but Makefile's `export GOTOOLCHAIN := go1.26.4` pins an OLDER toolchain — `go install github.com/goreleaser/goreleaser/v2@v2.18.0` will fail with "requires go >= 1.27.0 (running go 1.26.4)" in every CI job that runs `make setup-env`/`make setup-env-release`. Either downgrade GORELEASER_VERSION to a tag whose go.mod requirement is <= 1.26.4, or raise GOTOOLCHAIN to >= 1.27.0 in the same commit.
=== RUN   TestGoreleaserPinIsBuildableByPinnedToolchain/pinned_version_has_a_table_entry_(drift_gate)
--- FAIL: TestGoreleaserPinIsBuildableByPinnedToolchain (0.00s)
    --- FAIL: TestGoreleaserPinIsBuildableByPinnedToolchain/pinned_version_builds_under_the_pinned_toolchain (0.00s)
    --- PASS: TestGoreleaserPinIsBuildableByPinnedToolchain/pinned_version_has_a_table_entry_(drift_gate) (0.00s)
FAIL
FAIL	github.com/castocolina/gitid/cmd/gitid	0.002s
```

Exactly as predicted: required `1.27.0` > toolchain `1.26.4`.

### GREEN — same test after the pin change

Command: `go test ./cmd/gitid -count=1 -run 'TestGoreleaserPin' -v`

```
Go test: 3 passed in 1 packages
```

### `make setup-env-release` (after the pin change)

```
golangci/golangci-lint info checking GitHub for tag 'v2.12.2'
golangci/golangci-lint info found version: 2.12.2 for v2.12.2/linux/amd64
golangci/golangci-lint info installed /home/bazzite/go/bin/golangci-lint
==> Installing golangci-lint v2.12.2 via official binary installer (includes the embedded gosec linter; no standalone gosec binary needed — REVIEW WR-01)
curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -b "/home/bazzite/go/bin" v2.12.2
==> Installing goreleaser v2.17.0 (release build/archive/checksum/publish tool)
go install github.com/goreleaser/goreleaser/v2@v2.17.0
==> setup-env-release complete
```

Exit 0. No `requires go >=` error — the exact failure that broke CI is gone.

### `goreleaser --version` (after the pin change)

```
GitVersion:    v2.17.0
GitCommit:     unknown
GitTreeState:  unknown
BuildDate:     unknown
BuiltBy:       unknown
GoVersion:     go1.26.4
Compiler:      gc
ModuleSum:     h1:v1Cd9+0GSeHICqEnKjVTLBjA3uu0Wq8GiLJNct9SN+U=
Platform:      linux/amd64
```

Confirms `GitVersion: v2.17.0` built with `GoVersion: go1.26.4`.

### `make release-snapshot`

```
  • starting release
  • skipping announce, publish, and validate...
  • cleaning distribution directory
  • loading environment variables
  • getting and validating git state
    • using tags                                     previous=v0.1.0-rc.8 current=v0.1.0-rc.9
    • pipe skipped or partially skipped              reason=disabled during snapshot mode
  • parsing tag
  • setting defaults
    • DEPRECATED:  brews  should not be used anymore, check https://goreleaser.com/deprecations#brews for more info
  • snapshotting
    • building snapshot...                           version=0.1.0-rc.9-SNAPSHOT-88c37cc
  • ensuring distribution directory
  • setting up metadata
  • writing release metadata
  • loading go mod information
  • build prerequisites
  • building binaries
    • building                                       paths=cmd/gitid binaries=gitid target=linux_amd64_v1
    • building                                       paths=cmd/gitid binaries=gitid target=darwin_arm64_v8.0
    • building                                       paths=cmd/gitid binaries=gitid target=darwin_amd64_v1
    • building                                       paths=cmd/gitid binaries=gitid target=linux_arm64_v8.0
  • archives
    • archiving                                      name=dist/gitid_0.1.0-rc.9-SNAPSHOT-88c37cc_darwin_amd64.tar.gz
    • archiving                                      name=dist/gitid_0.1.0-rc.9-SNAPSHOT-88c37cc_linux_amd64.tar.gz
    • archiving                                      name=dist/gitid_0.1.0-rc.9-SNAPSHOT-88c37cc_linux_arm64.tar.gz
    • archiving                                      name=dist/gitid_0.1.0-rc.9-SNAPSHOT-88c37cc_darwin_arm64.tar.gz
  • calculating checksums
  • homebrew formula
    • writing                                        formula=dist/homebrew/Formula/gitid.rb
  • writing artifacts metadata
  • you are using deprecated options, check the output above for details
  • release succeeded after 7s
  • thanks for using GoReleaser!
```

`ls dist/*.tar.gz | wc -l` → `4`. `ls dist/*_checksums.txt` →
`dist/gitid_0.1.0-rc.9-SNAPSHOT-88c37cc_checksums.txt`. (The `DEPRECATED:
brews` warning is the same pre-existing D-13/D-18 soft-deprecation the plan
itself documents as not a gate — unrelated to this change.)

### `go build ./...` / `go vet ./...`

Both exit 0, no output (clean).

### `make lint`

```
==> lint-tagged: guarding against a new ungated //go:build tag (CR-13)
go vet -tags screenshot ./...
go vet -tags smoke ./...
go vet -tags e2e ./...
go vet -tags realaccount ./...
go vet -tags realaccountgitlab ./...
go vet -tags=realaccount,realaccountgitlab ./...
/home/bazzite/go/bin/golangci-lint run --build-tags screenshot ./internal/screenshot/...
0 issues.
==> lint-shell: POSIX parse check on scripts/*.sh
  ok   scripts/e2e-shard.sh
  ok   scripts/install.sh
/home/bazzite/go/bin/golangci-lint run ./...
0 issues.
```

### `go test ./cmd/gitid -count=1 -run 'TestGoreleaser|TestNightly|TestRelease|TestMakefile'`

`Go test: 24 passed in 1 packages` — the full cmd/gitid guard suite named
in Task 2's `<verify>` block, green.

## Verification Grep Checks

- `grep -c '^GORELEASER_VERSION := v2\.17\.0$' Makefile` → `1`
- `grep -v '^#' Makefile | grep -c 'v2\.18'` → `0` (no live, non-comment
  2.18-series reference remains; the historical narrative in the pin
  comment itself is comment-only, as the plan allows)
- `grep -c 'Addendum 2026-09-21' 10-RESEARCH.md` → `1`
- `grep -c 'Package Legitimacy Audit' 10-RESEARCH.md` → `1` (original
  heading survived — addendum was appended, not a rewrite)
- `grep -rc 'v2\.18\.0' cmd/gitid/nightly_yml_test.go
  cmd/gitid/goreleaser_config_test.go e2e/release_homebrew_gate_e2e_test.go
  .github/workflows/nightly.yml` → `0` for all four files
- `git diff --stat .goreleaser.yaml` → 1 file changed, 4 insertions(+), 3
  deletions(-) — confirmed via full diff that only the schema-key comment
  changed; the `brews:` block is byte-identical to before

## Empirical Re-Check on the Newly Pinned Binary

Per D-19's original finding and the plan's requirement to redo it against
the new pin: ran the same two checks against the freshly installed
`v2.17.0` binary.

- `goreleaser release --help | grep -c -- '--nightly'` → `0`
- `goreleaser jsonschema | grep -ci nightly` → `0`

Both unchanged from the v2.18.0-era finding — the `--nightly` flag is
Pro-only in every OSS release, so the downgrade could not have introduced
it.

## Compatibility Probes (from the plan, re-confirmed live during this session)

- `GOTOOLCHAIN=go1.26.4 go install .../goreleaser/v2@v2.18.0` → fails
  (`requires go >= 1.27.0`)
- `GOTOOLCHAIN=go1.26.4 go install .../goreleaser/v2@v2.17.1` → would fail
  too (`requires go >= 1.26.5`) — not attempted directly this session
  (`v2.17.1` was never installed), but its `go` directive was independently
  re-verified live against the proxy: `go 1.26.5` (see below)
- `GOTOOLCHAIN=go1.26.4 go install .../goreleaser/v2@v2.17.0` → succeeds,
  `GoVersion: go1.26.4` — this session's own `make setup-env-release`
  reproduces this exactly

Live proxy re-verification this session
(`curl -sS https://proxy.golang.org/github.com/goreleaser/goreleaser/v2/@v/<ver>.mod | grep '^go '`),
one version at a time, 2026-09-21:

| goreleaser | `go` directive (verified this session) |
|------------|------------------------------------------|
| v2.16.0 | 1.26.3 |
| v2.17.0 | 1.26.4 |
| v2.17.1 | 1.26.5 |
| v2.18.0 | 1.27.0 |
| v2.18.1 | 1.27.1 |
| v2.18.2 | 1.27.1 |

Matches the plan's table exactly.

## Deviations from Plan

### Auto-fixed Issues

None — no Rule 1/2/3 auto-fixes were needed for the two planned tasks
themselves; both executed as written.

### Known Issue (out of scope, pre-existing, NOT fixed)

Running the full `make test` (the plan's overall `<verification>` step 6)
surfaced 6 pre-existing, unrelated test failures in `cmd/gitid`, all in
`git_test.go`/`lifecycle_test.go`/`wiring_test.go` — files this quick task
never touched:

- `TestGitOptionsApplyBelowGateAdvisoryOriginatesFromCeremony`
- `TestCustomSSHDirectivePlanShowsRealDiff`
- `TestRunCustomSSHDirectiveWriteLandsInTheExistingGlobalBlock`
- `TestCommitGlobalSSHEndToEnd`
- `TestApplyThenCreatePreservesGlobalFix`
- `TestGlobalsFixThenCreateSurvivesCreate` / `TestGlobalsCreateThenFixLeavesIdentityUntouched`

Confirmed pre-existing and unrelated to this task's diff:
`git status --short` before writing this summary shows only the 8 files
this task intentionally modified/created — none of the three failing test
files are among them. Re-running the failing tests in isolation
(`-run '...'`, no `-race`, no full-package interaction) reproduces the same
failures, ruling out `-race`/test-order flakiness. The observed error —
`gitid: refusing path outside managed home: /home/bazzite/.ssh/config.d/gitid.config`
— names this machine's REAL `$HOME` (`/home/bazzite`), not a `t.TempDir()`
sandbox, which points at a pre-existing test-isolation gap: e.g.
`TestCommitGlobalSSHEndToEnd` (`wiring_test.go`) calls
`home := t.TempDir(); b := newBackendForHome(home)` but — unlike its sibling
`TestCommitDeleteEverythingSurfacesRemoved` a few lines above it, which
calls `t.Setenv("HOME", home)` — never overrides the process `$HOME`, so
some path resolution inside `newBackendForHome`/the sshconfig managed-home
check falls back to the real environment `$HOME`. This would likely pass
clean on a CI runner (no `~/.ssh/config.d/gitid.config` present there) and
only reproduces on a local dev machine with a real `~/.ssh`. Per
CLAUDE.md's/the executor's scope-boundary rule ("Only auto-fix issues
DIRECTLY caused by the current task's changes"), this was left unfixed and
is flagged here for a separate, dedicated fix pass — candidate root cause
above should make that fast. The plan's own Task 2 `<verify>` line (the
narrower `go test ./cmd/gitid -run
'TestGoreleaser|TestNightly|TestRelease|TestMakefile'`) is unaffected and
passed cleanly (24/24), as did `go build ./...`/`go vet ./...`/`make lint`.

## Follow-up Available (not run this session)

Per the orchestrator's instruction, the plan's optional `<human-check>`
step —
`gh workflow run nightly.yml --ref "$(git rev-parse --abbrev-ref HEAD)"`
followed by `gh run watch` — was deliberately NOT triggered here. It
publishes a real (ephemeral, self-pruning) nightly release and needs
explicit user confirmation first. Once this branch/commit is pushed, the
lighter proof (`gh run list --branch <branch> --limit 5`, confirming the
`check` matrix job now reaches `make test`/`make lint` instead of dying in
`make setup-env`) and, if the user opts in, the full nightly dispatch are
both available as a follow-up.

## Self-Check: PASSED

- `cmd/gitid/goreleaser_pin_test.go` — FOUND
- Makefile `GORELEASER_VERSION := v2.17.0` line — FOUND
- Commit `e24ee8c` — FOUND (`git log --oneline -3` shows it as HEAD)
