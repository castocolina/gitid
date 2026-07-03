# Deferred Items

Out-of-scope findings discovered during plan execution, logged per the executor's
SCOPE BOUNDARY rule (fix only what the current task's changes directly touch).

## From 02-03 (2026-07-03)

`make lint` (`golangci-lint run ./...`) does not compile `//go:build screenshot` or
`//go:build e2e` tagged files at all (no `--build-tags` configured in `.golangci.yml`
or the `lint` Makefile target), so pre-existing lint findings in those files have never
surfaced via `make lint`. Running `golangci-lint run --build-tags=e2e ./e2e/...`
manually during 02-03's verification surfaced 11 PRE-EXISTING findings, none in files
this plan touched (`e2e/dummy_nav_e2e_test.go`, `e2e/harness_test.go`):

- `e2e/ui_pty_e2e_test.go:203` — errcheck: `s.ptmx.Close()` return value unchecked
- `e2e/ui_pty_e2e_test.go:287` — gosec G301: `os.MkdirAll(gitconfigD, 0o755)` — expects 0750 or less
- `e2e/ui_pty_e2e_test.go:298` — gosec G306: `os.WriteFile(pubKey, ..., 0o644)` — expects 0600 or less
- `e2e/addrepo_e2e_test.go:30,78,97` — gosec G204: subprocess launched with variable (3 occurrences)
- `e2e/adopt_e2e_test.go:29,77` — gosec G306: `os.WriteFile(fragPath, ..., 0o644)` — expects 0600 or less (2 occurrences)
- `e2e/adopt_e2e_test.go:55,98,108` — gosec G304: potential file inclusion via variable (3 occurrences)

These predate 02-03 and are out of this plan's file scope (`e2e/harness_test.go`,
`e2e/dummy_nav_e2e_test.go` only). `golangci-lint run --build-tags=screenshot
./internal/screenshot/...` found 0 issues (Phase 1's `html.go`/`tui.go` and this plan's
new `screenshot`-tagged files are clean).

**Recommendation:** a future plan should either (a) add `run.build-tags: [screenshot,
e2e]` to `.golangci.yml` so `make lint` actually covers these files going forward, and
(b) fix the 11 findings above. Not addressed here — out of 02-03's declared file scope
and none of the findings are new regressions this plan introduced.

## From 02-11 (2026-07-03)

`make test` (`go test -race -coverprofile=coverage.out ./...`) fails with `go: no
such tool "covdata"` specifically on `github.com/castocolina/gitid/cmd/gitid-dummy`
— a package with zero `_test.go` files (only `main.go`, added in 02-02). This is a
pre-existing local-toolchain gap (the `covdata` binary is missing from this
machine's `GOROOT/bin` — coverage merging for a no-test-file package under `-race
-coverprofile` invokes it), present since `cmd/gitid-dummy/main.go` was added in
02-02 and unrelated to any file this plan (02-11) touches
(`internal/dummytui/keyowners_test.go`, `.planning/design/REFERENCE-INDEX.md`,
`.planning/design/APPROVAL.md`). `go test -race ./...` (without `-coverprofile`)
passes cleanly across every package, including `internal/dummytui` with this
plan's new `keyowners_test.go`. Not addressed here — a Makefile/toolchain
provisioning fix (installing `covdata` via `go install
golang.org/x/tools/...` or reworking the `test` target's coverage flags) is out of
this plan's declared file scope and not a regression this plan introduced.
