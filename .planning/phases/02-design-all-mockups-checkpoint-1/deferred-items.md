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
