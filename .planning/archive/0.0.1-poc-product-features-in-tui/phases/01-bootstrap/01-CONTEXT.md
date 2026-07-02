# Phase 1: Bootstrap - Context

**Gathered:** 2026-06-08
**Status:** Ready for planning

<domain>
## Phase Boundary

Phase 1 delivers the project's **scaffolding and quality toolchain** — the pieces
that make TDD possible from commit #1. It produces `go.mod`, a `Makefile` as the
single task runner, a `golangci-lint v2` config, a `gosec` setup, `pre-commit`
hooks wired to `make` targets, and a **green test harness**.

**In scope:** TOOL-01..04 — the Makefile target surface, `make setup-env` bootstrap,
pre-commit hooks invoking make, and a TDD harness proven green (`make test` exits 0).

**Out of scope:** Any `gitid` feature/domain logic (identity, sshconfig, gitconfig,
doctor, keygen, etc.) — that begins in Phase 2. CI on a hosted runner — deferred
until a git remote exists. The `internal/` packages are created as *empty placeholders*
here, but contain no real logic.

</domain>

<decisions>
## Implementation Decisions

### Module & Go version
- **D-01:** Module path is `github.com/castocolina/gitid` (matches the user's GitHub username from the reference gists; valid whether or not a remote is ever added).
- **D-02:** Pin the Go floor at **1.26** in `go.mod` (`go 1.26` toolchain directive). This is newer than the research's suggested 1.23 floor — the user opted for the current branch; all chosen libraries (x/crypto, cobra, charm.land v2) support it.

### Lint & security strictness
- **D-03:** golangci-lint v2 uses a **curated** linter set: the locked `staticcheck`, `gosec`, `unused`, plus `errcheck`, `ineffassign`, `misspell`, `revive`, `govet`. Strong signal, low noise (chosen over `linters.default: all`). Config uses the mandatory `version: "2"` key.
- **D-04:** **Hard-fail on any finding.** `make lint` (golangci-lint + gosec) fails on any reported issue; pre-commit and (future) CI inherit this. Matches the project's clean-history / clean-main standard. (Note: this is stricter than config's `security_block_on: high` — the curated gosec set is expected to be quiet enough to block on all.)

### Hooks & CI split
- **D-05:** **pre-commit = fast:** `gofmt`/`goimports` + `golangci-lint` + `gosec`. Keeps commits snappy.
- **D-06:** **pre-push = full:** `go test -race` + coverage. Tests gate before push, not on every commit.
- **D-07:** Hooks invoke `make` targets (single source of truth shared with CI), not ad-hoc commands or upstream hook repos.
- **D-08:** **CI deferred.** No `.github/workflows` in Phase 1 (no remote yet). Makefile + pre-commit are the Phase-1 gates. CI is added in a later phase when a remote exists.

### Scaffolding extent
- **D-09:** **Full package skeleton.** Phase 1 creates `cmd/gitid/main.go` (trivial, builds) plus all `internal/` package directories — `filewriter`, `sshconfig`, `gitconfig`, `identity`, `doctor`, `keygen`, `tester`, `clipboard`, `deps`, `platform` — and `tui/`. Each package gets a placeholder (e.g. `doc.go`) so it compiles, and at least one passing stub test so `make test` is green. No real logic — Phase 2 fills the slices.

### Makefile targets
- **D-10:** Targets: `setup-env`, `build`, `install`, `uninstall`, `test`, `lint`, `fmt` (per TOOL-01). `setup-env` installs golangci-lint v2 (binary install, not `go install`), gosec, pre-commit, and installs the git hooks.

### Claude's Discretion (planner decides)
- `make install` location — default to `go install ./cmd/gitid` → `$GOBIN`/`$GOPATH/bin`; `uninstall` removes that binary.
- **Coverage floor** — start **report-only, no hard threshold** in Phase 1 (raise later once real code lands).
- LICENSE / README stub — planner's call; keep minimal.
- Exact golangci-lint binary-install mechanism (official `install.sh` vs pinned release) and pre-commit framework wiring details.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Stack & toolchain (authoritative versions/config)
- `.planning/research/STACK.md` — pinned versions + import paths; golangci-lint v2 config notes (`version: "2"`, binary install), charm.land v2 vanity paths, why `git config`-via-exec for gitconfig. **The version source of truth.**
- `.planning/research/SUMMARY.md` §"Phase 0: Bootstrap" — the bootstrap rationale and what it must avoid.
- `CLAUDE.md` — appended Technology Stack section + working agreements (TDD, English-only, safe-writes); GSD Workflow Enforcement.

### Project intent & scope
- `.planning/PROJECT.md` — constraints (Quality tooling, Build automation, Commit hygiene), Key Decisions table.
- `.planning/REQUIREMENTS.md` §"Project Tooling & Standards (TOOL)" — TOOL-01..04, and the Definition of Done (make test/lint green; hooks via setup-env).
- `.planning/ROADMAP.md` §"Phase 1: Bootstrap" — goal + 4 success criteria.

### Architecture (for the skeleton layout)
- `.planning/research/ARCHITECTURE.md` — the `cmd/` / `internal/` / `tui/` layout and the package list to scaffold; one-directional dependency rule.

### Pitfalls (bake guardrails into config now)
- `.planning/research/PITFALLS.md` — atomic-write and permission pitfalls the `filewriter` package (Phase 2) must honor; gosec is enabled now partly to catch these.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- No Go code exists yet (greenfield). The reference configs in `.planning/references/` (target ssh/git config, legacy script) inform later phases, not Bootstrap.

### Established Patterns
- Repo convention: GSD planning lives in `.planning/`; commits go to `main` (no remote); history squashed/compacted at each plan close + user review.
- Node tooling (`gsd-tools.cjs`) requires Volta's bin on PATH in non-interactive shells — relevant for any scripted GSD calls, not the Go build.

### Integration Points
- The `Makefile` is the integration seam: pre-commit, pre-push, and future CI all call the same targets. Build it as the single source of truth.

</code_context>

<specifics>
## Specific Ideas

- golangci-lint **v2.12.2**, config key `version: "2"`, **binary install** (not `go install` — avoids Go-version-mismatch silent breakage).
- pre-commit framework with hooks pointing at `make` targets via `repo: local` (not `TekWizely/pre-commit-golang` upstream — keeps make as the single source of truth).
- TDD harness command: `go test -race -coverprofile=coverage.out ./...`.
- Each scaffolded `internal/*` package needs a `doc.go` (so it compiles) + a passing stub test (so `make test` is green from the start).

</specifics>

<deferred>
## Deferred Ideas

- **Hosted CI (GitHub Actions)** — add `.github/workflows` running `make lint` + `make test` on push/PR once a git remote exists. (Encodes D-04..D-07 gates in CI.)
- **Coverage threshold gate** — introduce a hard `make test` coverage floor after real code lands (Phase 2+).
- **`linters.default: all`** strict lint mode — could revisit if the curated set misses issues.

None of these expand Phase 1 scope — discussion stayed within the Bootstrap boundary.

</deferred>

---

*Phase: 1-bootstrap*
*Context gathered: 2026-06-08*
