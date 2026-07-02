---
phase: 01-bootstrap
reviewed: 2026-06-08T00:00:00Z
depth: standard
files_reviewed: 32
files_reviewed_list:
  - go.mod
  - cmd/gitid/main.go
  - cmd/gitid/main_test.go
  - Makefile
  - .golangci.yml
  - .pre-commit-config.yaml
  - .gitignore
  - LICENSE
  - README.md
  - internal/clipboard/doc.go
  - internal/clipboard/clipboard_stub_test.go
  - internal/deps/doc.go
  - internal/deps/deps_stub_test.go
  - internal/doctor/doc.go
  - internal/doctor/doctor_stub_test.go
  - internal/filewriter/doc.go
  - internal/filewriter/filewriter_stub_test.go
  - internal/gitconfig/doc.go
  - internal/gitconfig/gitconfig_stub_test.go
  - internal/identity/doc.go
  - internal/identity/identity_stub_test.go
  - internal/keygen/doc.go
  - internal/keygen/keygen_stub_test.go
  - internal/platform/doc.go
  - internal/platform/platform_stub_test.go
  - internal/sshconfig/doc.go
  - internal/sshconfig/sshconfig_stub_test.go
  - internal/tester/doc.go
  - internal/tester/tester_stub_test.go
  - tui/doc.go
  - tui/tui_stub_test.go
findings:
  critical: 0
  warning: 4
  info: 5
  total: 9
status: issues_found
---

# Phase 1: Code Review Report

**Reviewed:** 2026-06-08T00:00:00Z
**Depth:** standard
**Files Reviewed:** 32
**Status:** issues_found

## Summary

This is Phase 1 (Bootstrap): scaffolding plus quality toolchain. The Go code
compiles, `go vet ./...` is clean, `go test ./...` is green, and the binary runs
(`gitid version 0.0.0-dev`). The placeholder `internal/*` and `tui/` packages
(doc.go + green stub test each) are by design and were not flagged for "missing
implementation."

No Critical issues. The findings concentrate in the build/tooling surface — the
Makefile and config files — which is appropriate, since the Go is intentionally
thin this phase. The most consequential finding is a PATH-propagation fragility
in `setup-env` that can make a fresh-clone bootstrap fail at the hook-wiring step
(WR-01). The rest are reproducibility and consistency concerns.

No security vulnerabilities were found. The `curl | sh` toolchain installs are an
explicit, documented project decision (CLAUDE.md / STACK.md), so they are noted
but not raised as defects. Note this phase does not yet touch `~/.ssh` or
`~/.gitconfig`, so the backup/confirmation guarantees from CLAUDE.md are not in
scope to verify here.

## Warnings

### WR-01: `setup-env` may fail at `install-hooks` because the modified PATH does not propagate

**File:** `Makefile:48-52`
**Issue:** `pre-commit` is installed via `uv tool install` (line 50), which places
the binary under `~/.local/bin`. The PATH that exposes it is set inline only for
line 50's shell:

```make
PATH="$$HOME/.local/bin:$$PATH"; uv tool install pre-commit
@echo "==> Wiring git hooks"
$(MAKE) install-hooks
```

Each recipe line runs in its own shell, and `$(MAKE) install-hooks` spawns a new
process that inherits make's *original* environment — not the `PATH` modified on
line 50. If `~/.local/bin` is not already on the user's PATH (common on a fresh
machine, especially right after the `uv` bootstrap on line 49), `install-hooks`
runs bare `pre-commit install` and fails with "command not found", aborting the
end-to-end bootstrap that the recipe's own docstring promises.
**Fix:** Export the augmented PATH so it reaches the sub-make, or invoke
pre-commit by absolute path. For example:

```make
export PATH := $(HOME)/.local/bin:$(PATH)
```
at the top of the Makefile, or call `~/.local/bin/pre-commit install` directly in
`install-hooks`. Verify with a `PATH`-stripped shell that `make setup-env` reaches
"setup-env complete".

### WR-02: Toolchain installs use `@latest`, defeating the project's pinning discipline

**File:** `Makefile:43,47`
**Issue:** `goimports` and `gosec` are installed with `@latest`:

```make
go install golang.org/x/tools/cmd/goimports@latest
go install github.com/securego/gosec/v2/cmd/gosec@latest
```

CLAUDE.md/STACK.md pin every other tool (golangci-lint is pinned to v2.12.2 on the
very next lines). Unpinned `@latest` makes the dev/CI toolchain non-reproducible: a
future upstream release can silently change formatting or introduce new gosec
findings, breaking `make fmt`/`make lint` for a clean checkout with no code change.
This directly contradicts the "do NOT change without updating STACK.md" intent that
governs the adjacent golangci-lint pin.
**Fix:** Pin both to explicit versions, e.g.
`go install golang.org/x/tools/cmd/goimports@v0.x.y` and
`gosec@vX.Y.Z`, and record the versions in STACK.md alongside the golangci-lint pin.

### WR-03: `make fmt` scope is inconsistent between goimports and gofmt

**File:** `Makefile:68-69`
**Issue:**

```make
find . -name "*.go" -not -path "./.planning/*" -exec goimports -w {} +
gofmt -w .
```

`goimports` is deliberately scoped to exclude `.planning/`, but the immediately
following `gofmt -w .` recurses the entire tree with no exclusion. Today this is
harmless (no `.go` files exist under `.planning/` — verified), but the asymmetry is
a latent trap: the moment a Go fixture/example is added under `.planning/`, the two
formatters disagree on scope, and `gofmt` will rewrite a file that `goimports`
intentionally skipped. Additionally, `goimports` already applies gofmt formatting
internally, so the second `gofmt -w .` is largely redundant for the in-scope files.
**Fix:** Make both formatters share one scope. Either drop the `.planning/`
exclusion (if those files should be formatted) or apply the same `find`-based
exclusion to gofmt:

```make
GO_FILES := $(shell find . -name '*.go' -not -path './.planning/*')
fmt:
	goimports -w $(GO_FILES)
```

(goimports covers gofmt; the separate gofmt call can be dropped.)

### WR-04: `TestRunDoesNotPanic` discards `*testing.T`, so it can only ever fail via raw panic

**File:** `cmd/gitid/main_test.go:15-18`
**Issue:**

```go
func TestRunDoesNotPanic(_ *testing.T) {
	run()
}
```

By binding the `*testing.T` parameter to `_`, the test forfeits any ability to make
a controlled assertion or to use `t.Helper`/`t.Cleanup`/recover-and-`t.Errorf`. It
"passes" purely because `run()` does not panic; an unexpected panic would surface as
a hard test-binary crash rather than a clean failure report, and any future
non-panic regression (e.g. `run()` returning an error or writing nothing) cannot be
caught here. For a test whose stated purpose is asserting "does not panic," the
idiomatic form recovers explicitly and reports through `t`.
**Fix:** Keep the parameter and assert via recover:

```go
func TestRunDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("run() panicked: %v", r)
		}
	}()
	run()
}
```

## Info

### IN-01: `go.mod` lacks a `toolchain` directive despite CLAUDE.md guidance

**File:** `go.mod:3`
**Issue:** CLAUDE.md states "Pin minimum at 1.23+ for `go.mod` toolchain directive
support." The module declares `go 1.26` but no `toolchain` line, so the build uses
whatever toolchain the developer happens to have. This is acceptable but leaves the
toolchain unpinned relative to the documented intent.
**Fix:** Optionally add `toolchain go1.26.x` to make the toolchain explicit and
reproducible across contributors.

### IN-02: gosec `audit: false` setting could not be verified

**File:** `.golangci.yml:27-29`
**Issue:** The gosec block sets `config.global.audit: false`. golangci-lint is not
installed in this environment (`golangci-lint config verify` could not run), so the
validity of this key against the v2 schema is unverified. `audit` is a recognized
gosec global option, so this is likely valid, but it is also a no-op default
(`false` is gosec's default), adding configuration with no behavioral effect.
**Fix:** Run `golangci-lint config verify` in CI to confirm the schema, and consider
dropping the `audit: false` line since it only restates the default.

### IN-03: README quick-start omits `make install` / `make setup-env` prerequisites

**File:** `README.md:8-13`
**Issue:** The quick-start lists `setup-env`, `build`, `test`, `lint` but does not
mention that `setup-env` must succeed first (it installs goimports/golangci-lint
that `fmt`/`lint` depend on). A reader running `make lint` before `make setup-env`
gets a "golangci-lint: command not found" failure with no guidance.
**Fix:** Add a one-line note that `make setup-env` is a prerequisite for `fmt`/`lint`,
or order the block to make the dependency explicit.

### IN-04: Stub tests rely on a permanently-dead `if false` branch

**File:** `internal/*/{*}_stub_test.go` (all 11 stub tests), e.g. `internal/clipboard/clipboard_stub_test.go:8-10`
**Issue:** Each stub test is:

```go
if false {
	t.Fatal("unreachable — stub always passes")
}
```

This is intentional placeholder scaffolding, but the `if false { ... }` form is
exactly the dead-code shape that `staticcheck`/`unused`/`revive` are configured to
flag (`.golangci.yml` enables all three). If the linter flags these, every stub
trips it; if it does not, the branch is pure noise. The `t` parameter is otherwise
unused.
**Fix:** Replace with a trivially-true, lint-clean assertion, e.g.
`t.Log("stub package compiles")` or simply an empty body documented as a compile
smoke-test, and confirm `make lint` stays green on the stubs.

### IN-05: `make test` writes `coverage.out` but no target cleans it

**File:** `Makefile:80-81`
**Issue:** `make test` always emits `coverage.out` at the repo root. It is correctly
gitignored (`.gitignore:2`), but there is no `clean` target to remove it (or the
`bin/` output), so build artifacts accumulate locally with no make-driven cleanup.
**Fix:** Add a `clean` target (`rm -f coverage.out; rm -rf $(BIN_DIR)`) and include
it in `.PHONY`.

---

_Reviewed: 2026-06-08T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
