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

**PARTIALLY ADDRESSED (audited 2026-08-26, Phase 5 closeout):** `make lint`'s
`lint-tagged` target now runs `go vet` under `screenshot`/`smoke`/`e2e` tags and
`golangci-lint run --build-tags screenshot ./internal/screenshot/...` (0 issues).
The `e2e` tag is still not covered by `golangci-lint` itself (only `go vet`) —
`golangci-lint run --build-tags=e2e ./e2e/...` on this machine hits an unrelated
Go 1.27/golangci-lint typecheck incompatibility in the stdlib's own
`crypto/internal/randutil`, not this project's code. Still open; out of Phase 5's
scope to chase a toolchain compatibility issue.

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

## From 02-13 review convergence (2026-07-04)

Three cosmetic-robustness minors left open by the final three-reviewer convergence
pass on 0169ae7 (all reviewers verdict: ready to present; none affects behavior today):

- ~~`internal/dummytui/globalssh.go` (`handleStorageClick` radio rows) and
  `internal/dummytui/identities.go` (`paneDeleteScope` click) match option labels with
  whole-line `strings.Contains`...~~ **STALE (audited 2026-08-26, Phase 5 closeout):**
  neither file exists anymore — `internal/dummytui` was restructured to
  `data.go`/`fixturebackend.go`/`doc.go` only; the mouse layer this note referred to
  now lives in `internal/tuikit`.
- ~~`footerActionAt` maps click spans from the untruncated hint list...~~ **FIXED
  (2026-08-26, commit `313e559`):** confirmed no longer theoretical — this session's
  own `renderFooterLine` fix (dropping whole trailing actions instead of padding to
  width, commit `7468bcf`) made the mismatch live. `footerActionAt` now shares a
  `footerFit` decision with `renderFooterLine` so a click can never target a
  dropped/unrendered action. See `TestFooterActionAtNeverHitsADroppedAction`.
- Ceremony primary state renders Cancel with the default-focused (reverse) look while
  Enter still confirms — a deliberate, test-pinned mirror of the web's ceremony-level
  Enter handler; noted as a possible visual surprise on non-destructive ceremonies.
  Still open — unchanged, low-priority design note, not touched by this audit.
