# Makefile — single task-runner surface for gitid.
# All targets are .PHONY (no file artifacts tracked by make).
# pre-commit hooks and future CI call these same targets — single source of truth.
#
# Targets:
#   setup-env      Bootstrap a Go toolchain first, if missing (quick task 260907-eda),
#                  then install development tools (goimports, golangci-lint (its embedded
#                  gosec linter is the real gosec coverage `make lint` uses — no
#                  standalone gosec binary is installed, round-3 code-review WR-01),
#                  pre-commit, freeze, goreleaser) and provision the pinned Chromium
#                  revision; wire git hooks via install-hooks (completed in plan 01-03;
#                  screenshot tooling in 01-05; goreleaser in Phase 10 plan 10-04).
#   setup-env-release  Narrower bootstrap for release.yml (REVIEW C-7, Phase 10 plan
#                  10-04): installs ONLY golangci-lint (again, its embedded gosec linter,
#                  not a standalone binary) and goreleaser — the two tools `make
#                  test`/`make lint`/`make release` actually need — skipping goimports,
#                  pre-commit/install-hooks, freeze, and the pinned-Chromium provisioning
#                  step, none of which release.yml's test+lint+release gate requires.
#   build          Compile the gitid binary to bin/gitid.
#   build-cross    Cross-compile the release build matrix (darwin/amd64, darwin/arm64,
#                  linux/amd64, linux/arm64) to bin/gitid-<os>-<arch> (BUILD-01).
#                  Cross-compilation via GOOS/GOARCH is OS-independent, so CI runs this
#                  ONCE on ubuntu-latest rather than on every matrix runner. Optional
#                  VERSION/COMMIT/DATE overrides stamp gitid --version (BUILD-03).
#   release        Goreleaser-driven, tag-published release (D-05/D-07/D-08/D-13, Phase
#                  10 plan 10-04): exports the SAME Makefile-computed VERSION/COMMIT/DATE
#                  build-cross/build already use into the goreleaser subprocess's
#                  environment and runs `goreleaser release --clean`. Requires a real
#                  pushed tag plus GITHUB_TOKEN/HOMEBREW_TAP_GITHUB_TOKEN (release.yml's
#                  job, never run locally with real secrets).
#   release-snapshot  Local, zero-secrets dry run of the SAME .goreleaser.yaml (D-05):
#                  `goreleaser release --snapshot --clean` skips git-tag validation and
#                  ALL publish steps (release:/brews:), reproducing the D-07 artifact
#                  shape into dist/ for local verification.
#   install        Install gitid to $GOPATH/bin via go install.
#   uninstall      Remove gitid from $GOPATH/bin.
#   test           Run the race-enabled test harness with a coverage profile (TDD harness,
#                  D-06), then the fast/hermetic subset of internal/screenshot's own
#                  `-tags screenshot` suite (WR-28, see lint-tagged below).
#   lint           Run golangci-lint (reads .golangci.yml); hard-fails on any finding (D-04).
#                  Depends on lint-tagged (WR-28, CR-13) so every isolated build tag's
#                  static analysis can never be silently skipped again, and on
#                  lint-shell so a syntactically broken scripts/*.sh cannot pass.
#   lint-shell     POSIX `sh -n` parse check over scripts/*.sh. Catches an unterminated
#                  quote or `case` in a script users pipe into their shell. Not
#                  shellcheck: `sh -n` is on every host with no addition to setup-env
#                  or the three CI runners; the behavioral contract is
#                  e2e/release_e2e_test.go. macOS `sh -n` is bash in POSIX mode and
#                  does not detect bashisms — the e2e suite covers that.
#   lint-tagged    `go vet` under EVERY isolated build tag (screenshot, smoke, e2e) plus
#                  golangci-lint under `screenshot` (WR-28: internal/screenshot was
#                  previously invisible to both `make lint` and `make test` — no gate ever
#                  compiled or ran it, so the WR-19/WR-22 regression tests it carries
#                  executed nowhere and a real regression, WR-26, shipped undetected. CR-13:
#                  the identical blindspot was still open for `smoke` after WR-28 closed it
#                  only for `screenshot` — a stale 3-arg call in
#                  cmd/gitid/smoke_network_test.go rotted there uncompiled). A guard loop
#                  fails the build the moment a NEW `//go:build <tag>` appears without a
#                  matching `go vet -tags <tag>` line here, so this cannot recur a third
#                  time. The package's own `go test` execution lives in the `test` target
#                  instead (kept OUT of lint-tagged so the pre-commit hook — make fmt +
#                  make lint — stays fast; test already runs at the higher-latency-tolerant
#                  pre-push stage). Excludes TestCaptureTUI/TestCaptureHTML*/
#                  TestProvisionPinnedChromium from that test run: those are the heavy,
#                  tool-provisioning/network-dependent capture entry points
#                  `make screenshot-tui`/`make screenshot-html`/`make setup-env` already
#                  own — everything else in the package is fast, hermetic unit-style
#                  coverage.
#   fmt            Run goimports then gofmt over all packages.
#   screenshot-tui  Render the TUI View()-dump golden to a deterministic PNG via freeze
#                   (TOOL-05, DLV-03; build-tag isolated behind `screenshot`).
#   screenshot-html Render the fixture HTML page to a deterministic PNG via headless
#                   Chromium (go-rod, pinned revision; TOOL-05, DLV-03).
#   gate-no-backend-files  Fail if any commit on this branch (since it diverged from
#                   main) touches a file outside the Phase 2 design-only allowlist
#                   (SECURITY.md Finding 1 / T-02-BEGATE) -- automates what was
#                   previously only a one-off shell line in a plan file. Standalone
#                   target; run it directly on design-only branches.
#   demo-web       (Re)launch the web design mockup dev server (Vite) on the
#                   dedicated $(DEMO_WEB_PORT) and open it.

.PHONY: setup-env setup-env-release build build-cross release release-snapshot run install uninstall test lint lint-shell lint-tagged fmt install-hooks test-e2e screenshot-tui screenshot-html gate-no-backend-files gate-visual-regression smoke-network-test verify-upload-real-account verify-upload-real-account-gitlab demo-web

# Binary output directory.
BIN_DIR := bin
BINARY  := $(BIN_DIR)/gitid

# Optional-default ldflags so a routine `make build` / `make build-cross` keeps
# producing a traceable, git-describe-derived dev-stamped binary and only an
# explicit override produces a release stamp (D-06/D-10, Pattern 1).
# Invocation:
#   make build-cross VERSION=1.2.3 COMMIT=abc1234 DATE=2026-08-30
# VERSION's default is `git describe --tags --match "v*" --always --dirty`
# with the leading `v` stripped via patsubst (Phase 10 D-10/D-11 REVIEW C-1):
# `--match "v*"` excludes this repo's non-release tags (`poc-0.0.1`,
# `backup/*`); the strip keeps the LOCAL default and the release pipeline's
# `${GITHUB_REF_NAME#v}` computation agreeing byte-for-byte on D-11's
# no-leading-`v` `--version` format. The three -X paths target
# internal/version's unexported version/commit/buildDate vars (Phase 10,
# D-09) — matching Go identifier names, not main.<name> anymore.
VERSION ?= $(patsubst v%,%,$(shell git describe --tags --match "v*" --always --dirty))
COMMIT  ?= none
DATE    ?= unknown
LDFLAGS := -X github.com/castocolina/gitid/internal/version.version=$(VERSION) -X github.com/castocolina/gitid/internal/version.commit=$(COMMIT) -X github.com/castocolina/gitid/internal/version.buildDate=$(DATE)

# test-e2e-shard defaults: 1 of 1 (the whole suite) unless CI overrides both.
E2E_SHARD  ?= 1
E2E_SHARDS ?= 1

# Keep Go commands and golangci-lint's type checker on the documented toolchain.
# Go 1.27's standard library is newer than this pinned linter supports.
export GOTOOLCHAIN := go1.26.4

# Pinned Go version for setup-env's tarball-fallback bootstrap (below), derived from
# GOTOOLCHAIN so there is only ONE Go version to keep in sync — never a second
# hardcoded literal that could drift out of sync (quick task 260907-eda).
GO_BOOTSTRAP_VERSION := $(patsubst go%,%,$(GOTOOLCHAIN))

# Go binary locations.
# GOPATH_BIN falls back to $(HOME)/go/bin (Go's own documented default GOPATH) when
# `go` is not yet on PATH. Without this fallback, a from-cold-start `make setup-env`
# run (no Go toolchain installed yet) computes `go env GOPATH` as empty, so
# GOPATH_BIN silently collapses to the literal string "/bin" — an unwritable path
# without root — breaking the golangci-lint installer on that same run. Confirmed
# empirically during planning of quick task 260907-eda. With `go` present, this is
# byte-identical to the prior `$(shell go env GOPATH)/bin` value.
GOPATH_BIN := $(if $(shell command -v go 2>/dev/null),$(shell go env GOPATH)/bin,$(HOME)/go/bin)
GOFMT      := $(shell GOTOOLCHAIN=$(GOTOOLCHAIN) go env GOROOT)/bin/gofmt

# golangci-lint version to install (pinned — do NOT change without updating STACK.md).
GOLANGCI_LINT_VERSION := v2.12.2

# freeze version to install (pinned — dev/build tool only, never a runtime dep of the
# shipped gitid binary; see internal/screenshot/tui.go, build-tag isolated). Supply-chain
# provenance recorded in .planning/design/_spike/GOLDENS.md (01-05 Task 1).
FREEZE_VERSION := v0.2.2

# goreleaser version to install (pinned). Repinned 2026-09-21 (quick task
# 260921-t6g) from v2.18.0 to v2.17.0: goreleaser 2.18.x+ declares a `go`
# directive of 1.27.0 or newer in its own go.mod, while this Makefile
# deliberately exports a 1.26-series GOTOOLCHAIN above (golangci-lint
# v2.12.2 cannot handle the Go 1.27 standard library) — so the newer line
# cannot be built here at all, and every workflow calling `make setup-env`/
# `make setup-env-release` died at this exact bootstrap step from
# 2026-09-05 onward. v2.17.0 is the newest release whose own `go` directive
# (1.26.4) the exported GOTOOLCHAIN satisfies exactly. Every candidate
# tag's `go` directive was read live from the Go module proxy
# (proxy.golang.org/github.com/goreleaser/goreleaser/v2/@v/<ver>.mod) on
# 2026-09-21, discharging this pin's fresh-verification policy (originally
# stated as `git ls-remote --tags`, see 10-RESEARCH.md Package Legitimacy
# Audit — that verdict is unchanged, see its 2026-09-21 addendum). Do NOT
# bump this past v2.17.0 without also raising GOTOOLCHAIN in the SAME
# commit — cmd/gitid/goreleaser_pin_test.go now enforces the constraint
# mechanically instead of by prose alone. Dev/build tool only, never a
# runtime dep of the shipped gitid binary (Phase 10 plan 10-04, D-05).
GORELEASER_VERSION := v2.17.0

# Vendored monospace font + fixed theme for deterministic screenshot-tui rendering
# (Pitfall 6 — freeze's default font discovery is not CI-deterministic). These are the
# same values internal/screenshot/tui_capture_test.go passes to freeze's --font.file /
# --theme flags at a fixed 100x30 (cols x rows) capture geometry (D-04); recorded here
# too so a fresh clone can see, without reading Go source, which font/theme/geometry a
# reproduced golden depends on.
SCREENSHOT_FONT  := $(CURDIR)/.planning/design/fonts/JetBrainsMono-Regular.ttf
SCREENSHOT_THEME := dracula

# demo-web: dedicated NON-standard port for the web design mockup dev server.
# 45173 is memorable (Vite's default 5173 with a 4-prefix), sits below macOS's
# ephemeral port range (49152+) so it is stable to bind, and is off 5173 so
# lsof-kill-by-port can never collide with another Vite project on the default
# port (T-qw-01). DEMO_WEB_DIR uses $(CURDIR) so the recipe is independent of
# the caller's working directory (SCREENSHOT_FONT precedent above).
DEMO_WEB_PORT := 45173
DEMO_WEB_DIR  := $(CURDIR)/.planning/design/mockup-src
DEMO_WEB_LOG  := /tmp/gitid-demo-web.log

# Resolved tool binaries, referenced by absolute path so recipes run regardless of the
# caller's PATH. GNU Make 3.81 (macOS) direct-execs a bare command (no shell metacharacters)
# using its ORIGINAL PATH, ignoring the `export PATH` below — so a bare `golangci-lint` fails
# when ~/go/bin isn't already on PATH. Absolute paths sidestep that entirely. setup-env
# installs both binaries into $(GOPATH_BIN).
GOLANGCI_LINT := $(GOPATH_BIN)/golangci-lint
GOIMPORTS     := $(GOPATH_BIN)/goimports
GORELEASER    := $(GOPATH_BIN)/goreleaser

# Capture the caller's REAL interactive PATH *before* the export below clobbers it.
# The `install` target must judge PATH membership against what the user's shell will
# actually see — not against the make-augmented PATH (which always contains GOPATH_BIN,
# making the check a guaranteed false "PATH: OK"). FIX-INSTALL-01 / F-1.
ORIGINAL_PATH := $(PATH)

# Ensure tool bin dirs are on PATH for EVERY recipe line, the install-hooks sub-make,
# and make-invoked git hooks — so a fresh clone bootstraps without relying on the
# caller's interactive PATH (review WR-01). uv installs pre-commit into ~/.local/bin;
# go install and the golangci-lint installer place binaries in $(GOPATH_BIN).
# $(HOME)/.local/go/bin is where setup-env's own Go-toolchain bootstrap (below)
# extracts the golang.org/dl tarball fallback when neither an existing `go` nor
# Homebrew is available. Unlike $(GOPATH_BIN), this is a FIXED path known before the
# bootstrap step runs, so listing it here is safe even before the directory exists on
# this particular invocation: each subsequent recipe line runs in a NEW shell that
# inherits this exported PATH, and a PATH lookup only checks for the binary's
# existence at the moment a command is dispatched — so it resolves correctly the
# instant the bootstrap step has finished extracting the tarball, within the SAME
# `make setup-env` invocation (same class of gap this file already documents for
# uv/pre-commit above; quick task 260907-eda).
export PATH := $(HOME)/.local/go/bin:$(HOME)/.local/bin:$(GOPATH_BIN):$(PATH)

## setup-env: install all development tools and prepare the git hooks.
##
## Tools installed:
##   Go toolchain  — bootstrapped FIRST, only when `go` is missing from PATH: installs
##                   via Homebrew/Linuxbrew when available, otherwise falls back to the
##                   pinned official golang.org/dl tarball. Never overwrites an existing
##                   toolchain (CI's actions/setup-go, a prior local install, or a
##                   version manager) — the whole step is a silent no-op whenever any
##                   `go` is already on PATH (quick task 260907-eda).
##   goimports     — import block formatter (run as standalone + via golangci-lint)
##   golangci-lint — lint aggregator, v2.12.2, installed via the official binary
##                   installer (NOT go install — avoids Go-version-mismatch silent breakage,
##                   per STACK.md and CLAUDE.md).
##   (gosec coverage comes entirely from golangci-lint's own embedded gosec linter,
##                   enabled in .golangci.yml — no standalone gosec binary is installed;
##                   removed round 3, WR-01: it was never invoked by any make target,
##                   CI job, or pre-commit hook, so it was unpinned (`@latest`) dead
##                   weight widening this repo's supply-chain surface for zero benefit.)
##   pre-commit    — git hook runner; hooks point at make targets.
##   freeze        — ANSI terminal-output -> PNG renderer for `screenshot-tui`, pinned
##                   @v0.2.2 (dev/build tool only — never a runtime dep of the shipped
##                   gitid binary; Pitfall 8: unlike golangci-lint, `go install` is fine
##                   for freeze).
##   pinned Chromium revision — headless-Chromium build `screenshot-html` drives via
##                   go-rod, pre-downloaded into the fixed cache path so a later
##                   `make screenshot-html` never pays the download cost (or fails
##                   offline) on a fresh clone (T-01-SC2).
##   goreleaser    — release build/archive/checksum/publish tool, pinned
##                   @$(GORELEASER_VERSION) (Phase 10 plan 10-04, D-05) — `go install`
##                   is fine here (unlike golangci-lint, Pitfall 8 precedent); never a
##                   runtime dep of the shipped gitid binary.
##
## Git hook wiring (pre-commit install, pre-push install) is completed in plan 01-03
## via the install-hooks sub-target below.  setup-env calls install-hooks so that once
## 01-03 defines it fully, a single `make setup-env` bootstraps a fresh clone end-to-end.
setup-env:
	@echo "==> Checking for a Go toolchain"
	command -v go >/dev/null 2>&1 && echo "go already on PATH ($$(go version)) -- nothing to bootstrap" || { \
	    echo "go not found on PATH -- this never overwrites an existing toolchain"; \
	    if command -v brew >/dev/null 2>&1; then \
	        echo "==> Installing a Go toolchain via Homebrew"; \
	        brew install go; \
	    else \
	        echo "==> No package manager found -- falling back to the pinned go$(GO_BOOTSTRAP_VERSION) golang.org/dl tarball (local-dev safety net; CI always already has go via actions/setup-go)"; \
	        os=$$(uname -s | tr '[:upper:]' '[:lower:]'); \
	        case "$$(uname -m)" in \
	            x86_64|amd64) arch=amd64 ;; \
	            aarch64|arm64) arch=arm64 ;; \
	            *) echo "unsupported architecture $$(uname -m) -- install Go manually from https://go.dev/dl/"; exit 1 ;; \
	        esac; \
	        tarball="go$(GO_BOOTSTRAP_VERSION).$${os}-$${arch}.tar.gz"; \
	        tmpdir=$$(mktemp -d); \
	        curl -sSfL "https://go.dev/dl/$${tarball}" -o "$${tmpdir}/$${tarball}"; \
	        rm -rf "$$HOME/.local/go"; \
	        mkdir -p "$$HOME/.local"; \
	        tar -C "$$HOME/.local" -xzf "$${tmpdir}/$${tarball}"; \
	        rm -rf "$${tmpdir}"; \
	        echo "installed go $(GO_BOOTSTRAP_VERSION) to $$HOME/.local/go/bin"; \
	    fi; \
	}
	@echo "==> Installing goimports"
	go install golang.org/x/tools/cmd/goimports@latest
	@echo "==> Installing golangci-lint $(GOLANGCI_LINT_VERSION) via official binary installer (includes the embedded gosec linter; no standalone gosec binary needed — REVIEW round-3 WR-01)"
	curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -b "$(GOPATH_BIN)" $(GOLANGCI_LINT_VERSION)
	@echo "==> Installing pre-commit (via uv; bootstrap uv with the Astral installer if missing — not a system package manager)"
	command -v uv >/dev/null 2>&1 || curl -LsSf https://astral.sh/uv/install.sh | UV_INSTALL_DIR="$$HOME/.local/bin" sh
	# The Astral installer drops uv in ~/.local/bin, but make exec's the next
	# metacharacter-free recipe line directly (bypassing the line-69 PATH export),
	# so a bare `uv` is not found on a runner without a pre-installed uv (seen on
	# macos-15-intel). Prepend ~/.local/bin inline so the freshly-bootstrapped uv
	# resolves regardless of whether it pre-existed on PATH.
	PATH="$$HOME/.local/bin:$$PATH" uv tool install pre-commit
	@echo "==> Installing freeze $(FREEZE_VERSION) (screenshot-tui rendering; dev/build tool only)"
	go install github.com/charmbracelet/freeze@v0.2.2
	@echo "==> Provisioning the pinned Chromium revision for screenshot-html (headless, go-rod)"
	go test -tags screenshot -run TestProvisionPinnedChromium ./internal/screenshot/...
	@echo "==> Installing goreleaser $(GORELEASER_VERSION) (release build/archive/checksum/publish tool)"
	go install github.com/goreleaser/goreleaser/v2@$(GORELEASER_VERSION)
	@echo "==> Wiring git hooks"
	$(MAKE) install-hooks
	@echo "==> setup-env complete"

## setup-env-release: narrower bootstrap for release.yml (REVIEW C-7, Phase 10 plan
## 10-04). Installs ONLY golangci-lint (its embedded gosec linter is the real gosec
## coverage `make lint` uses — no standalone gosec binary is installed here, see
## round-2 code-review WR-01: a standalone `gosec@latest` install was dead weight in
## this exact secrets-bearing job, widening its supply-chain surface via an unpinned
## dependency resolution for zero linting benefit) and goreleaser — the two tools
## `make test`/`make lint`/`make release` actually need — explicitly SKIPPING
## goimports, pre-commit/install-hooks, freeze, and the pinned-Chromium provisioning
## step setup-env's full bootstrap performs. None of those are needed by `make
## test`/`make lint`/`make release` (golangci-lint's own `--build-tags screenshot` run
## is static analysis only, never test execution, and `make test`'s own
## screenshot-tagged line already `-skip`s the Chromium-dependent tests) —
## downloading Chromium on the one workflow where a failure means a pushed tag
## doesn't publish adds avoidable minutes and a network-flake failure mode for zero
## benefit. release.yml calls this target, not the full `make setup-env`.
setup-env-release:
	@echo "==> Installing golangci-lint $(GOLANGCI_LINT_VERSION) via official binary installer (includes the embedded gosec linter; no standalone gosec binary needed — REVIEW WR-01)"
	curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -b "$(GOPATH_BIN)" $(GOLANGCI_LINT_VERSION)
	@echo "==> Installing goreleaser $(GORELEASER_VERSION) (release build/archive/checksum/publish tool)"
	go install github.com/goreleaser/goreleaser/v2@$(GORELEASER_VERSION)
	@echo "==> setup-env-release complete"

## install-hooks: wire pre-commit and pre-push git hooks.
## Installs the pre-commit hook (runs make fmt + make lint on git commit)
## and the pre-push hook (runs make test before push).
## Called by setup-env — run `make setup-env` on a fresh clone to bootstrap fully.
install-hooks:
	# Chained with && so make runs this through a shell, which resolves `pre-commit`
	# via the exported PATH. GNU Make 3.81 (macOS) direct-execs bare commands using its
	# original PATH, bypassing the `export PATH` above — forcing a shell avoids that.
	pre-commit install && pre-commit install --hook-type pre-push

## fmt: format all Go source files.
## Runs goimports (manages import blocks) then gofmt (canonical formatting).
## Neither goimports nor gofmt accept the Go ./... wildcard pattern — use find to enumerate
## .go files and pass the repo root to gofmt.
fmt:
	find . -name "*.go" -not -path "./.planning/*" -exec $(GOIMPORTS) -w {} +
	find . -name "*.go" -not -path "./.planning/*" -exec $(GOFMT) -w {} +

## lint-tagged: WR-28 (screenshot) / CR-13 (smoke, e2e) -- gate the
## SYNTAX/STATIC coverage of EVERY build-tag-isolated file in the tree, not
## just `screenshot`. Before WR-28, no gate ever compiled OR ran anything
## behind `//go:build screenshot`: `make lint` ran golangci-lint untagged,
## `make test` ran go test untagged, and no other target ran the PACKAGE's
## own tests. That blindspot is why WR-19's and WR-22's regression tests
## (internal/screenshot/region_disposition_test.go,
## TestExtractRegion_GitCeremonyMatchesReceiptHeadingAcrossWrap) executed in
## NO gate and a real regression (WR-26, an off-by-one in the very window
## WR-22 fixed) shipped undetected. WR-28 closed that for `screenshot` only;
## CR-13 (iteration 4) found the identical blindspot still wide open for
## `smoke` -- the fixer's own repair of a stale 3-arg `tester.PreWrite` call
## in cmd/gitid/smoke_network_test.go had rotted there, uncompiled by any
## gate, exactly like WR-26 rotted behind `screenshot`.
##
## `go vet -tags <tag> ./...` runs for EVERY isolated build tag below (cheap
## -- no test execution, no network, ~1s per tag). golangci-lint stays
## scoped to internal/screenshot itself: the other tagged files already have
## dedicated gates (`gate-visual-regression`, `generate-visual-review-packet`,
## `test-e2e`), and widening golangci-lint's tagged scope to e2e-/smoke-tagged
## files elsewhere would pull in a large pre-existing, unrelated lint backlog
## this fix is not scoped to clear.
##
## The guard loop below fails the instant a NEW `//go:build <tag>` line
## appears anywhere in the tree without a matching `go vet -tags <tag>` line
## added here -- so this exact blindspot (closed once for `screenshot`, then
## rediscovered open for `smoke`) cannot recur a third time on a future tag.
##
## The package's own TEST execution (WR-19/WR-22/WR-26/WR-27's actual
## regression coverage) is wired into the `test` target below, NOT here: a
## first version of this fix ran `go test -tags screenshot ...` (~65s) as
## part of this target, which `lint` depends on -- that made every
## pre-commit hook invocation (`make fmt` + `make lint`, per
## .pre-commit-config.yaml) 5-7x slower, defeating the fast-feedback point of
## a pre-commit gate. `test` already runs at pre-push (a naturally
## higher-latency-tolerant point per .pre-commit-config.yaml's own staging),
## so that is where the package's real `go test` coverage belongs -- `lint`
## stays fast (vet + static analysis only) while the tests still execute in
## an unconditional gate, satisfying "somewhere in make lint or make test".
KNOWN_BUILD_TAGS := screenshot smoke e2e realaccount realaccountgitlab
lint-tagged:
	@echo "==> lint-tagged: guarding against a new ungated //go:build tag (CR-13)"
	@found_tags=$$(find . -name '*.go' -not -path './.planning/*' -print0 \
	    | xargs -0 grep -hoE '^//go:build [A-Za-z0-9_]+' 2>/dev/null \
	    | awk '{print $$2}' | sort -u); \
	for t in $$found_tags; do \
	    case " $(KNOWN_BUILD_TAGS) " in \
	        *" $$t "*) ;; \
	        *) echo "lint-tagged: //go:build $$t found with no 'go vet -tags $$t' line wired into this target -- add one (CR-13's root cause: a build tag no gate compiles rots silently)"; exit 1 ;; \
	    esac; \
	done
	go vet -tags screenshot ./...
	go vet -tags smoke ./...
	go vet -tags e2e ./...
	go vet -tags realaccount ./...
	go vet -tags realaccountgitlab ./...
	go vet -tags=realaccount,realaccountgitlab ./...
	$(GOLANGCI_LINT) run --build-tags screenshot ./internal/screenshot/...

## lint-shell: POSIX `sh -n` parse check over scripts/*.sh.
## Not shellcheck: `sh -n` is a POSIX parse check every host already has, with
## no addition to setup-env or to the three CI runners. The authoritative
## behavioral proof is e2e/release_e2e_test.go, which executes the real script
## — this gate exists to catch the one failure mode that would be catastrophic
## in a script users pipe into their shell, an unterminated quote or `case`
## that only manifests at parse time on a stranger's machine. `sh -n` on macOS
## runs bash in POSIX mode and therefore does not detect bashisms; the e2e
## suite's real execution is what covers that.
lint-shell:
	@echo "==> lint-shell: POSIX parse check on scripts/*.sh"
	@for f in scripts/*.sh; do \
		sh -n "$$f" || exit 1; \
		echo "  ok   $$f"; \
	done

## lint: run golangci-lint against all packages.
## Hard-fails on any finding — zero tolerance (D-04).
## Configuration lives in .golangci.yml.
## Depends on lint-tagged (WR-28, CR-13) so every isolated build tag's static
## analysis can never be silently skipped again -- a caller running `make
## lint` directly (not just CI) always exercises all of them. Depends on
## lint-shell so a syntactically broken installer cannot pass `make lint`.
lint: lint-tagged lint-shell
	$(GOLANGCI_LINT) run ./...

## test: run the TDD harness with race detection and a coverage profile.
## Coverage is report-only in Phase 1; no hard threshold (D-09 discretion).
## This is the same command pre-push hooks and future CI will call (D-06).
##
## D-04's requirement-keyed parity matrix check runs here via the cmd/gitid
## TestParityMatrix* suite: the first `go test ./...` line exercises the
## two-directional matrix-vs-tree checker (shipped rows resolve, deferred
## nouns agree on their phase, every runnable command is named), so any drift
## between docs/cli-parity-matrix.md and the built command tree fails
## `make test` (plan 05-08 Task 2).
##
## The second `go test` line is WR-28's actual test-execution half (see
## lint-screenshot's comment above for why it lives here, not in `lint`):
## the fast, hermetic subset of internal/screenshot's OWN suite, excluding
## TestCaptureTUI / TestCaptureHTML* / TestProvisionPinnedChromium — the
## heavy, external-tool (freeze)/headless-Chromium/network-provisioning
## capture entry points that `make screenshot-tui` / `make screenshot-html` /
## `make setup-env` already own as their explicit, opt-in single-test
## invocations.
test: gate-copy-freeze
	go test -race -coverprofile=coverage.out ./...
	go test -tags screenshot -skip 'TestCaptureTUI|TestCaptureHTML|TestProvisionPinnedChromium' ./internal/screenshot/...

## gate-copy-freeze: the 02-STYLE-SPEC.md §6 copy-freeze grep gate.
## Every string below is FROZEN by an approved design artifact: reword it and
## the render silently drifts from the contract the mockups were signed off
## against. A plain presence grep over the render stack is the whole mechanism
## — cheap, mechanical, and impossible to satisfy by accident.
##
## Phase 3 adds the D-16 banner copy (02-UI-SPEC.md "Scoped Divergences") plus
## plan 03-05's D-02/D-01 warning-state copy (below). The demo's OWN frozen
## string (`-- needs user.name + a valid email`, D7) is asserted too. Phase 4
## removes the real binary's retired capability override, so both binaries use
## the same form-validity gate.
##
## WR-08 (05-REVIEW.md): cmd/gitid's own rotateDryRunCaveat sentence claimed
## this gate protected it, but the grep roots below never covered cmd/gitid
## and the sentence was never added to the list — a false verification claim.
## Both are now real: the grep roots include cmd/gitid, and the caveat's
## byte-exact text is registered below.
##
## WR-13 (06-REVIEW.md): the D-13 exclusion check below (the dynamic version
## line's prefix, VersionNotePrefix, must never be frozen — it changes with
## the user's OpenSSH build) is assembled at runtime so the check line cannot
## satisfy (or trip) the grep gate itself. This explanation used to live as a
## `#` comment INSIDE the backslash-continued recipe below; a `fi; \` line
## continues straight into a `#` line, which comments out the REST of that
## logical shell line — so the next two `#` lines and the `dyn_prefix=`
## assignment silently lost their `@` prefix and became raw, un-quieted
## recipe lines. Worse: a one-character edit (adding a trailing `\` to any of
## those comment lines) would silently swallow the entire D-13 check with the
## gate still reporting success. Keeping the explanation OUT of the
## continued recipe removes that trap entirely.
##
## Phase 9 (09-UI-SPEC.md's Copywriting Contract, D-08) registers the
## internal/tuikit/design.go Upload/RotateDeleteOffer copy block below. The
## grep roots already cover internal/tuikit, where design.go declares them,
## so no root change was needed. `UploadKeyTitleFmt` ("gitid: %s @ %s") is
## deliberately EXCLUDED — its rendered output varies per machine (the local
## short hostname, D-07), matching the four existing per-machine/dynamic-text
## exclusion precedents below (D-13's OpenSSH prefix, 07-03's git-version
## prefix, D-09's bundle aggregate, the applied/selected counts); a fifth
## runtime-assembled exclusion check proves it stays out.
##
## 09.2-REVIEW.md WR-07: the Phase 9.2 Global Git Ignore screen's copy
## (09.2-UI-SPEC.md's Copywriting Contract) was declared in
## internal/tuikit/design.go across the whole phase but never registered in
## this gate — the block below closes that gap. The dynamic-message pieces
## (GitIgnoreMalformedFileMessage, GitIgnoreWiringPointsElsewhere,
## GitIgnoreReceiptWrongTarget) register their FIXED literal fragments —
## the interpolated displayPath/otherPath varies per machine/file, matching
## the existing dynamic-text exclusion precedents above. As WR-07 itself
## notes, this grep is a SECONDARY source-presence guard (it only proves the
## string appears SOMEWHERE, including a comment); `TestFrozenGitIgnoreCopy`
## (internal/tuikit/gitignore_copy_test.go) is the AUTHORITATIVE byte-exact
## contract.
##
## IMPORTANT: this gate is a SECONDARY source-presence guard — `grep -rqF`
## only proves a string appears SOMEWHERE under the scanned roots (a comment
## or a dead declaration would satisfy it too). `TestFrozenUploadCopy`
## (internal/tuikit/upload_copy_test.go) is the AUTHORITATIVE byte-exact
## contract for the Phase 9 strings below; a green gate here is not proof of
## the value (R21, 09-01-PLAN.md cross-AI review).
gate-copy-freeze:
	@echo "==> gate-copy-freeze: 02-STYLE-SPEC.md §6 frozen copy"
	@fail=0; \
	for s in \
		'Preview — demo data, not wired to your system yet' \
		'[ Skip Git ]' \
		'[ Continue ]' \
		'Skip keeps this identity SSH-only and marks it incomplete.' \
		'Continue reviews the Git fragment, includeIf, and allowed_signers entries before writing.' \
		'— needs user.name + a valid email' \
		'Write it' \
		'Blank prefix → SSH Host = the provider host itself' \
		'! Reachable — key not uploaded yet' \
		'Stored — key not uploaded yet; this identity is not proven for Git yet' \
		'This key is also used by ' \
		' — it will be kept. Only this identity'\''s SSH and Git artifacts are removed.' \
		'(+%d more)' \
		'↓ (+%d more options)' \
		'↑ (+%d more options)' \
		'Found %q referenced in %s: %d — review before continuing.' \
		'This action is irreversible' \
		'%s will be removed from active use; a copy of the key pair exists at %s.' \
		'Repo remotes using git@<alias>: cannot be scanned and will break after this delete.' \
		'The old key stays valid at %s during this window — upload the new key, verify it, then remove the old one there.' \
		'Old key archived to %s' \
		'gitid always writes' \
		'Write Host * managed block to ' \
		'apply global git option(s) ' \
		'user.name (global fallback)' \
		'user.email (global fallback)' \
		'This dry run tests only the current key'\''s reachability — the new key has not been generated, uploaded, or resolved, so nothing about the post-rotation state is proven.' \
		'OpenSSH version could not be read; run ssh -V to check compatibility' \
		'set by you at ' \
		'set in ' \
		'gitid cannot change this' \
		'not set (OpenSSH default: ' \
		'set outside your config' \
		'set, differs — yours, would be a no-op here' \
		'set, differs — external, would be a no-op here' \
		'safe by default' \
		'not applicable (macOS-only setting)' \
		'not applicable (OpenSSH too old for accept-new)' \
		'not applicable (OpenSSH version could not be verified)' \
		'not applicable (nothing on this machine to verify)' \
		'not applicable (could not be probed)' \
		'The option states could not be read from this machine.' \
		'shadow warning: ' \
		'simulation inconclusive — gitid could not fully read your config graph' \
		'advisory: ' \
		'set by gitid in ' \
		'set by you in ' \
		'set somewhere gitid cannot name' \
		'not set (git'\''s built-in default: ' \
		'git'\''s own init and clone commands probe the filesystem and may write a repository-local core.ignorecase that overrides this global setting.' \
		'Requires git 2.35 or newer to write zdiff3' \
		'user.useConfigOnly is selected but the fallback author has no name set' \
		'user.useConfigOnly is selected but the fallback author has no email set' \
		'The fallback email is set but the fallback name is empty' \
		'Global user.email was left alone, as always -- each identity'\''s commits use their own includeIf fragment.' \
		' — your value differs, so yours wins' \
		'Register with %s automatically (auth + signing)' \
		'Register with %s automatically — not logged in to %s; run \"%s auth login\" first, or check anyway' \
		'Auto-registration unavailable — %s has no gh/glab match here. Manual steps are shown after create.' \
		'Running: %s' \
		'✓ %s key registered' \
		'✓ %s key already registered (skipped)' \
		'✗ %s key registration failed: %s' \
		'insufficient scope — run \"gh auth refresh -h %s -s admin:public_key\", then retry from the Identity Manager' \
		'insufficient scope — run \"gh auth refresh -h %s -s admin:ssh_signing_key\", then retry from the Identity Manager' \
		'GitLab rejected this key — it is already registered to a DIFFERENT account. If that'\''s expected, remove it there first; otherwise check \"glab auth status\".' \
		'Could not check %s for existing keys — uploading anyway; duplicates are handled safely.' \
		'--dry-run: the command(s) above were shown, not run.' \
		'Auto-registration wasn'\''t available. Register it yourself:' \
		'Auto-upload skipped (--no-upload).' \
		'✓ Already registered with %s — nothing to do.' \
		'Authentication' \
		'Signing' \
		'Key' \
		'Remove the old key from %s?' \
		'The old key (\"gitid: %s @ %s\") still authenticates there until you remove it. Delete it now?' \
		'[ Delete old key from %s ]' \
		'[ Leave it — I'\''ll remove it myself ]' \
		'✓ Old key removed from %s.' \
		'Left in place — remove it yourself: %s' \
		'Register key with provider now (u)' \
		'Register %s'\''s key with %s' \
		'Global Git Ignore' \
		'✓ Wired — core.excludesfile points at this file; Git reads it.' \
		'! core.excludesfile is not set — Git does not read any global ignore file yet. Confirming here will set it.' \
		'! No gitid-managed Git baseline configuration was found on this machine — open the Doctor to set that up before this screen can wire core.excludesfile.' \
		'No managed block found yet in ~/.gitignore_global — showing the curated defaults below. Nothing has been written.' \
		'This write will also set core.excludesfile in your Git baseline, since it is not set yet.' \
		'Global gitignore written to ~/.gitignore_global.' \
		'core.excludesfile was also set to point at this file.' \
		'No backup was needed — the content was unchanged or the file is new.' \
		'This file changed since you last reviewed it — press a to review the current content again before writing.' \
		'Review your global gitignore before writing.' \
		'Reset to defaults' \
		'Review & write' \
		'Done editing' \
		'Leaving this screen discards unsaved edits.' \
		'an opening marker with no matching closing marker' \
		'a closing marker with no valid matching opening marker' \
		'two complete gitid blocks in one file' \
		'a malformed gitid marker' \
		' has a broken gitid marker at line ' \
		' — repair the file by hand before this screen can read or write it.' \
		'looks like a gitid managed-block marker and can'\''t be part of your content — edit or remove that line before applying.' \
		'! core.excludesfile points at ' \
		'instead of this file — that choice is left alone; writing here only affects the file below.' \
		'core.excludesfile still points at ' \
		'so Git is not reading this file — that setting was left as you configured it.' \
		'All directives' \
		'Type to filter…' \
		'%d of %d shown' \
		'! The SSH configuration could not be resolved.' \
		'ssh -G could not be run against this host — re-enter the screen to retry.' \
		'No directives match \"%s\".' \
		'Also tracked as a recommended option — see the Options tab for gitid'\''s guidance.' \
		'Resolved via ssh -G — reflects Include/Match precedence already applied.' \
		'Other keys' \
		'! Git config could not be read.' \
		'git config --list --show-origin failed — re-enter the screen to retry.' \
		'No git config keys are set yet.' \
		'No keys match \"%s\".' \
		'Add custom key' \
		'Write custom Git key to %s' \
		'%s = %s written.' \
		'That key/value can'\''t be written: %s' \
		'Add custom directive' \
		'Checking '\''%s'\'' against OpenSSH'\''s known-directive list…' \
		''\''%s'\'' is not a recognized SSH directive — nothing was written.' \
		'ssh -G rejected this value: %s — nothing was written.' \
		'Write custom SSH directive to %s' \
		'%s %s written.' \
		'edit' \
		'commit' \
		'dismiss' \
		'Git'\''s default branch for new repositories' \
		'Edit author (name and email)' \
		''\''%s'\'' already has a problem in your current configuration — unrelated to what you just entered. Nothing was written.'; \
	do \
		if grep -rqF -- "$$s" internal/tuikit internal/identity cmd/gitid internal/globalssh internal/globalgit; then \
			echo "    ok   $$s"; \
		else \
			echo "    MISSING  $$s"; fail=1; \
		fi; \
	done; \
	if [ $$fail -ne 0 ]; then \
		echo "gate-copy-freeze: FROZEN COPY MISSING (02-STYLE-SPEC.md §6)"; \
		exit 1; \
	fi; \
	dyn_prefix="Your OpenSSH"; dyn_prefix="$$dyn_prefix:"; \
	if grep -qF -- "$$dyn_prefix" Makefile; then \
		echo "    FAIL  dynamic version prefix must stay out of the frozen list (D-13)"; \
		exit 1; \
	else \
		echo "    ok   D-13 exclusion (dynamic version prefix not frozen)"; \
	fi; \
	git_dyn_prefix="Your git"; git_dyn_prefix="$$git_dyn_prefix:"; \
	if grep -qF -- "$$git_dyn_prefix" Makefile; then \
		echo "    FAIL  dynamic git-version prefix must stay out of the frozen list (07-03 D-13 precedent)"; \
		exit 1; \
	else \
		echo "    ok   07-03 exclusion (dynamic git-version prefix not frozen)"; \
	fi; \
	bundle_dyn="of"; bundle_dyn="$$bundle_dyn set"; \
	if grep -qF -- "$$bundle_dyn" Makefile; then \
		echo "    FAIL  dynamic bundle aggregate count must stay out of the frozen list (D-09)"; \
		exit 1; \
	else \
		echo "    ok   D-09 exclusion (dynamic bundle aggregate count not frozen)"; \
	fi; \
	counts_dyn="baseline options applied"; counts_dyn="$$counts_dyn to"; \
	if grep -qF -- "$$counts_dyn" Makefile; then \
		echo "    FAIL  dynamic applied/selected counts must stay out of the frozen list (result message)"; \
		exit 1; \
	else \
		echo "    ok   result-message exclusion (dynamic applied/selected counts not frozen)"; \
	fi; \
	keytitle_dyn="'gitid: %s"; keytitle_dyn="$$keytitle_dyn @ %s'"; \
	if grep -qF -- "$$keytitle_dyn" Makefile; then \
		echo "    FAIL  dynamic key-title format (UploadKeyTitleFmt, D-07) must stay out of the frozen list — its rendered output varies per machine"; \
		exit 1; \
	else \
		echo "    ok   D-07 exclusion (dynamic key-title format not frozen)"; \
	fi

## build: compile the gitid binary.
build:
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/gitid

## build-cross: cross-compile the release build matrix reproducibly (BUILD-01).
## darwin/amd64, darwin/arm64, linux/amd64, and linux/arm64 are the published
## matrix (D-02). GOOS/GOARCH cross-compilation is OS-independent (no cgo in this
## module), so this target is invoked ONCE on a single Linux runner in CI rather
## than redundantly on every matrix OS. Output binaries are named
## bin/gitid-<os>-<arch>. Optional VERSION/COMMIT/DATE stamp gitid --version
## (BUILD-03); a routine invocation with no overrides keeps the dev defaults.
## This is a local unstamped cross-build convenience only — the actual release
## build/archive/checksum pipeline is `make release`/`make release-snapshot`
## (goreleaser, Phase 10 plan 10-04/10-05), not this target.
build-cross:
	@mkdir -p $(BIN_DIR)
	GOOS=darwin  GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/gitid-darwin-amd64 ./cmd/gitid
	GOOS=darwin  GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/gitid-darwin-arm64 ./cmd/gitid
	GOOS=linux   GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/gitid-linux-amd64  ./cmd/gitid
	GOOS=linux   GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/gitid-linux-arm64  ./cmd/gitid

## release: goreleaser-driven publish (D-05/D-07/D-08/D-13, Phase 10 plan 10-04).
## Exports the SAME VERSION/COMMIT/DATE make variables build/build-cross already read
## into the goreleaser subprocess's environment (D-05: no drift between two independent
## version descriptions) and runs `goreleaser release --clean`, which reads
## .goreleaser.yaml, builds the 4-target matrix, archives to tar.gz, writes the
## checksums manifest, publishes the GitHub Release, and pushes the Homebrew tap
## formula. Requires a real pushed tag plus GITHUB_TOKEN in the environment — this is
## release.yml's own job, never run locally with real secrets. `dist:` (goreleaser's
## own, never `bin/` — REVIEW C-4) is where output lands.
##
## D-18 (10-CONTEXT.md addendum, 2026-09-05): the D-13 Homebrew `brews:` publish leg
## is GATED, not deleted — castocolina/homebrew-tap does not exist yet (10-VERIFICATION.md
## human_verification item 2). SKIP_HOMEBREW resolves to `--skip=homebrew` whenever
## HOMEBREW_TAP_GITHUB_TOKEN is empty/unset, so a real tag push succeeds without the tap
## repo/PAT; the instant that secret is added as a real repo secret, this same target
## activates the brews: publish with ZERO code changes (the `.goreleaser.yaml` brews:
## stanza is untouched — see the comment above it).
SKIP_HOMEBREW := $(if $(HOMEBREW_TAP_GITHUB_TOKEN),,--skip=homebrew)
release:
	VERSION=$(VERSION) COMMIT=$(COMMIT) DATE=$(DATE) $(GORELEASER) release --clean $(SKIP_HOMEBREW)

## release-snapshot: local, zero-secrets dry run of the SAME .goreleaser.yaml (D-05).
## `--snapshot` skips git-tag validation and ALL publish steps (release:/brews:) —
## safe to run with no secrets, no real tag, on any commit. Reproduces the D-07
## artifact shape (4 platform tar.gz archives + one checksums.txt) into dist/ for
## local verification; this is the phase's own dry-run verification vehicle.
release-snapshot:
	VERSION=$(VERSION) COMMIT=$(COMMIT) DATE=$(DATE) $(GORELEASER) release --snapshot --clean

## release-nightly: D-19 (10-CONTEXT.md addendum, 2026-09-05). GoReleaser's native
## `--nightly` mode / `nightly:` config is GoReleaser-Pro-only — EMPIRICALLY VERIFIED
## against this repo's pinned OSS binary (`goreleaser release --help` lists no
## `--nightly` flag; `goreleaser jsonschema` has zero `nightly` occurrences). Rather
## than a hand-rolled version-resolution script (explicitly rejected — the user's own
## castocolina/wezterm-setup does that and was reviewed as the anti-pattern NOT to
## repeat), this target reuses goreleaser's ORDINARY `release` command against a
## freshly created, valid-prerelease-semver git tag: the only new logic is computing a
## timestamped tag string and `git tag`/`git push`ing it — ordinary git tagging, not a
## release-selection engine. The tag's non-empty prerelease suffix
## (`nightly.<ts>.<sha>`) makes `.goreleaser.yaml`'s existing `prerelease: auto` /
## `make_latest: "{{ not .Prerelease }}"` mark it prerelease and never GitHub's
## "latest", with zero new config. Homebrew is ALWAYS skipped (token present or not) —
## a rolling nightly must never touch the stable tap formula. Best-effort deletes prior
## nightly tags/releases first (approximating GoReleaser-Pro's `keep_single_release`,
## also Pro-only, with ordinary `gh`/`git` calls); a missing/unauthenticated `gh` logs a
## note and continues rather than failing the build — CI (nightly.yml) always has both.
NIGHTLY_TAG := v0.0.0-nightly.$(shell date -u +%Y%m%d%H%M%S).$(shell git rev-parse --short HEAD)
# REVIEW cycle-1 finding #3 (10-REVIEWS.md): nightly binaries must never fall
# back to the Makefile's placeholder COMMIT/DATE defaults ("none"/"unknown")
# just because nightly.yml has no metadata-computation step of its own (unlike
# release.yml). Compute both directly here so `make release-nightly` is
# correctly stamped no matter what invokes it.
NIGHTLY_COMMIT := $(shell git rev-parse --short HEAD)
NIGHTLY_DATE := $(shell date -u +%Y-%m-%d)
.PHONY: release-nightly
release-nightly:
	@if command -v gh >/dev/null 2>&1 && gh auth status >/dev/null 2>&1; then \
		echo "==> release-nightly: pruning prior v0.0.0-nightly.* releases/tags"; \
		for tag in $$(gh release list --limit 100 --json tagName --jq '.[].tagName' 2>/dev/null | grep '^v0\.0\.0-nightly\.' || true); do \
			gh release delete "$$tag" --yes --cleanup-tag 2>/dev/null || true; \
		done; \
	else \
		echo "==> release-nightly: gh not available/authenticated — skipping prior-nightly cleanup (best-effort only)"; \
	fi
	# Lightweight tag (NOT `git tag -a`/`-m`): an annotated tag creates a tag
	# OBJECT, which requires a configured committer identity
	# (user.name/user.email) — a fresh GitHub-hosted runner (nightly.yml) has
	# none, and `git tag -a` would fail with "empty ident name" on its very
	# first scheduled run (code-review finding). A lightweight tag is a bare
	# ref with no object/identity requirement and is exactly as valid a
	# goreleaser `git describe` target as an annotated one.
	git tag "$(NIGHTLY_TAG)"
	git push origin "$(NIGHTLY_TAG)"
	VERSION=$(patsubst v%,%,$(NIGHTLY_TAG)) COMMIT=$(NIGHTLY_COMMIT) DATE=$(NIGHTLY_DATE) $(GORELEASER) release --clean --skip=homebrew,announce

## run: build (if needed) and run the gitid binary locally.
## Depends on build so bin/gitid is always current before launch. Extra args
## can be passed via ARGS, e.g. `make run ARGS="doctor"`.
run: build
	$(BINARY) $(ARGS)

## install: install gitid to $GOPATH/bin and report the install path + PATH status.
install:
	go install -ldflags "$(LDFLAGS)" ./cmd/gitid
	@INSTALL_PATH="$(GOPATH_BIN)/gitid"; \
	echo "  installed: $$INSTALL_PATH"; \
	printf '%s' "$(ORIGINAL_PATH)" | tr ':' '\n' | grep -qxF "$(GOPATH_BIN)" \
	  && echo "  PATH: OK (gitid is on PATH)" \
	  || echo "  PATH: $(GOPATH_BIN) is NOT on your PATH — add to shell: export PATH=\"\$$PATH:$(GOPATH_BIN)\""

## uninstall: remove gitid from $GOPATH/bin.
uninstall:
	rm -f "$(GOPATH_BIN)/gitid"

## test-e2e: run end-to-end agent-driven tests (builds binary first).
## E2E tests use a hermetic sandbox HOME and a fake ssh script injected on PATH.
## Tests are tagged //go:build e2e and are excluded from the normal make test target.
## Timeout 2400s (raised 900s -> 1200s -> 1800s -> 2400s across v0.1.0-rc.1/
## rc.2/rc.3's CI-only test-e2e failures). Two independent CI-only budgets
## were undersized against GitHub Actions' shared runners, never reproducing
## locally: e2e/ui_pty_e2e_test.go's ptySession.waitFor (PTY content polls)
## AND every per-test context.WithTimeout(context.Background(), N*time.Second)
## that bounds the driven subprocess itself — the latter was the dominant
## ceiling (many failures clustered right under its unscaled 60s). Both now
## multiply by ciTimeoutMultiplier() (3x under GITHUB_ACTIONS=true), so this
## Makefile ceiling needs matching headroom for the stragglers that actually
## use it — most tests still pass in their original time.
## rc.1 also surfaced one deterministic (not timing) bug: the D-04
## rotate-delete-offer title embeds os.Hostname(), which is far longer on a
## GitHub-hosted runner than any local machine and hard-wraps across lines —
## no timeout fixes that; see mustSeeUnwrapped in identity_manager_pty_e2e_test.go.
##
## Phase 4 (04-04-PLAN.md Task 2/3, D-12): this target ALSO runs
## TestGitConfiguration_CompiledRealVsLiveDummyPTY — the paired compiled PTY
## workflow that drives the REAL cmd/gitid binary AND the compiled
## cmd/gitid-dummy binary over raw pseudo-terminals and compares NORMALIZED
## semantic checkpoints (never raw bytes, never HTML/MUI/Chromium/PNG). It is
## the DLV-06 real-keystroke counterpart to `make gate-visual-regression`'s
## in-process capture gate below — both classify divergences against the SAME
## .planning/design/git-screen/visual-divergence-allowlist.txt.
##
## Phase 5 (05-09-PLAN.md Tasks 1/2, DLV-06/DLV-04): this target ALSO runs the
## full per-state identity-manager PTY suite (TestIdentityManager_*, every
## approved manager state plus both key-ceremony modes, driven with raw
## keystrokes against the REAL compiled binary) PLUS
## TestIdentityManager_CompiledRealVsLiveDummyPTY — the SAME paired-PTY
## pattern as the git-screen case above, classifying divergences against
## .planning/design/identity-manager/visual-divergence-allowlist.txt. The
## per-state suite's independent FIELDS.md manifest backstop (review R-26)
## is what still catches a shared-renderer defect this paired comparison
## structurally cannot see.
test-e2e: build
	go test -tags e2e -race -timeout 2400s ./e2e/...

## test-e2e-shard: run one round-robin slice of the e2e package's top-level
## Test functions (E2E_SHARD of E2E_SHARDS, both 1-based) instead of the
## whole suite in one process. v0.1.0-rc.1 through rc.6 all failed
## test-e2e on GitHub Actions with a shifting set of failures across the
## ~950s single-process -race run, never reproducing locally even under a
## matched core count (GOMAXPROCS=2) — most consistent with cumulative
## resource contention building up over one long run on a shared runner.
## Splitting the SAME suite across several shorter, parallel CI jobs
## reduces both per-job wall time and that cumulative pressure.
## See scripts/e2e-shard.sh for the partitioning.
test-e2e-shard: build
	./scripts/e2e-shard.sh "$(E2E_SHARD)" "$(E2E_SHARDS)" -timeout 900s

## screenshot-tui: render the Bubble Tea View()-dump golden to a deterministic PNG
## via freeze (TOOL-05, DLV-03). Invokes TestCaptureTUI — the concrete runnable
## entry point under the `screenshot` build tag that actually writes the PNG —
## which pins the vendored $(SCREENSHOT_FONT) via --font.file, the fixed
## $(SCREENSHOT_THEME) --theme, and a fixed 100x30 (cols x rows) capture geometry
## (D-04). Writes to .planning/design/_spike/tui/ and asserts the golden SHA-256
## recorded in .planning/design/_spike/GOLDENS.md reproduces on re-run.
## internal/screenshot/tui.go is //go:build screenshot isolated — this target,
## not `go build ./cmd/gitid`, is the only thing that ever compiles it.
screenshot-tui:
	go test -tags screenshot -run TestCaptureTUI ./internal/screenshot/...

## screenshot-html: render the fixture HTML page to a deterministic PNG via
## headless Chromium (go-rod, PINNED revision — see ChromiumRevision in
## internal/screenshot/html.go and the provenance note in
## .planning/design/_spike/GOLDENS.md) at a fixed viewport/scale/color-scheme.
## Invokes TestCaptureHTML — the concrete runnable entry point under the
## `screenshot` build tag that actually writes the PNG. Writes to
## .planning/design/_spike/html/ and asserts the golden SHA-256 recorded in
## .planning/design/_spike/GOLDENS.md reproduces on re-run.
## internal/screenshot/html.go is //go:build screenshot isolated — this target,
## not `go build ./cmd/gitid`, is the only thing that ever compiles it (go-rod
## never enters the shipped binary's dependency graph).
screenshot-html:
	go test -tags screenshot ./internal/screenshot/... -run TestCaptureHTML

## gate-visual-regression: DLV-04.1/D-24.1 golden-text visual-regression
## gate (plan 03-06 Task 2, corrected plan 03-10 Task 2 — CR-01/CR-04/CR-05).
## Drives the shared internal/tuikit render stack in-process through a fixed
## script for BOTH the real cmd/gitid Backend and cmd/gitid-dummy's
## FixtureBackend. RequiredScreenSpecs defines the symmetric inventory; every
## unequal or one-sided named region needs an explicit ux-improvement/defect
## classification. HTML, pixel parity, and real/dummy byte parity are excluded.
##
## Phase 4 (04-04-PLAN.md Task 3, D-12): RequiredScreenSpecs is a MERGED
## registry — the create-flow specs above PLUS five Phase 4 git-screen
## checkpoints (git-form-filled, git-form-empty, match-strategy-select,
## review-readonly, result-success), captured separately (their own seeded
## HOME — see cmd/gitid/gate_visual_regression_test.go's
## deterministicGitIdentityFixture/mergeGitScreenCaptures) so the git-screen
## fixture's extra identities never shift the create-flow wizard's own
## sidebar layout. This target runs the Phase 4 SEMANTIC gate (in-process,
## no PTY); `make test-e2e` above runs the PAIRED compiled PTY workflow —
## both classify against the SAME
## .planning/design/git-screen/visual-divergence-allowlist.txt.
##
## READ-ONLY (CR-01): writes ONLY to temp directories. Never modifies
## .planning/phases/03-create-flow-backend/ or any tracked path.
## Runs TWO independent captures per surface and asserts within-surface
## determinism before validating classified region evidence.
##
## Invokes TestGateVisualRegression + TestGateVisualRegressionReadOnly +
## TestAllScreensCapturedAndNonEmpty + TestNegativeControl_* (Phase 4's own
## missing-state/unclassified-difference/exhaustive-mutation-sensitivity/
## cross-registry-leakage controls, CR-11) under the `screenshot` build tag.
##
## Phase 5 (05-09-PLAN.md Task 3): RequiredScreenSpecs is now a THREE-way
## merged registry — create-flow + Phase 4 git-screen + Phase 5
## identity-manager (action-menu, delete-choice, confirm-destructive,
## detail-ssh-first — captured separately via its own seeded HOME, see
## deterministicIdentityManagerFixture/mergeIdentityManagerCaptures, same
## isolation reason as git-screen's own fixture). rotate-result/repair-result
## are DELIBERATELY NOT registered in this in-process gate: both require a
## real (or FakeSSHDir-substituted) SSH connectivity probe this no-subprocess
## gate has no way to inject — `make test-e2e`'s PTY suite above carries that
## evidence instead. This target classifies identity-manager divergences
## against .planning/design/identity-manager/visual-divergence-allowlist.txt,
## and also runs the Phase 5 negative controls
## (TestNegativeControl_MissingIdentityManagerState,
## TestNegativeControl_IdentityManagerUnclassifiedDifferenceRejected,
## TestNegativeControl_AllIdentityManagerComparableEqualRegionsAreMutationSensitive,
## TestNegativeControl_IdentityManagerCrossRegistryLeakage).
##
## Phase 6 (06-07-PLAN.md Task 1): RequiredScreenSpecs is now a FOUR-way
## merged registry — the three above PLUS seven Phase 6 Global SSH checkpoints
## (gss-options-list, gss-storage-current, gss-storage-other, gss-apply-preview,
## gss-apply-receipt, gss-storage-migrate-preview, gss-storage-migrate-receipt;
## captured via deterministicGlobalSSHFixture/mergeGlobalSSHCaptures against
## their OWN Include-layout seeded HOME, same isolation reason). The two
## receipt states are DELIBERATELY non-applicable on BOTH surfaces in this
## in-process gate: each receipt requires a real journal-backed write that
## neither surface performs here — the PTY frames
## ui-frames/global-ssh-apply-confirm.txt (06-04) and
## ui-frames/storage-migrate-confirm-post.txt (06-05) carry that evidence
## instead, named by the specs' non-applicability records and asserted to
## exist. The Phase 6 surface-classified divergences live in
## .planning/design/global-ssh/visual-divergence-allowlist.txt (kept in byte
## 1:1 sync with the code dispositions by
## TestGlobalSSHAllowlistMatchesRegistry), and the Phase 6 negative controls
## (TestNegativeControl_MissingGlobalSSHState,
## TestNegativeControl_GlobalSSHUnclassifiedDifferenceRejected,
## TestNegativeControl_AllGlobalSSHComparableEqualRegionsAreMutationSensitive,
## TestNegativeControl_GlobalSSHCrossRegistryLeakage) plus the TestGlobalSSH*
## acceptance tests (HTML non-applicability, PTY-frame evidence existence,
## four-state fixture coverage, allowlist schema, Makefile filter selection,
## cross-run determinism, prior-surface stability) run under the filter below.
##
## Phase 8 (08-08-PLAN.md Task 2 / 09.4-02, DLV-04): RequiredScreenSpecs is now a
## SEVEN-way merged registry — the six above PLUS three Doctor checkpoints
## (doctor-findings, doctor-selected, doctor-ceremony-preview; captured
## via CaptureDoctorScreens/mergeDoctorCaptures against their OWN
## seeded fixture HOME, the same isolation reason as
## every later-phase surface). Classified against
## .planning/design/health-fixer/visual-divergence-allowlist.txt (kept in
## sync by TestDoctorAllowlistMatchesRegistry); the four Doctor
## negative controls (TestNegativeControl_DoctorMissingState,
## TestNegativeControl_DoctorUnclassifiedDifference,
## TestNegativeControl_DoctorPerturbedComparableRegion,
## TestNegativeControl_DoctorCrossSurfaceAllowlistLeakage) plus the
## TestDoctorAllowlist*/TestDoctorHTML*/TestDoctorMakefile* acceptance
## tests run under the filter below.
##
## Phase 9 (09-07-PLAN.md Task 2, UP-01/UP-02/UP-03): RequiredScreenSpecs is
## now an EIGHT-way merged registry — the seven above PLUS eight upload-
## surface checkpoints spanning TWO existing surfaces (the create-flow
## wizard's step-2 "Test connection" pane and the identity-manager's
## register-key pane), captured via CaptureUploadScreens against its own
## seeded HOME (deterministicUploadFixture/mergeUploadCaptures), with its
## real-side uploaderDeps swapped to a deterministic "gh ok" fake so the
## announce/result states resolve without a real gh/glab on the machine
## running the gate. rotate-delete-offer is registered but NON-APPLICABLE in
## this in-process gate (both ApplicableLive/ApplicableApprovedTUI false) —
## the D-04 offer requires a completed key-rotation commit this no-subprocess
## capture path never performs, mirroring gss-apply-receipt/ggit-apply-
## receipt's own precedent; its evidence lives in the PTY frame
## .planning/phases/09-upload-credentials-assist/ui-frames/
## identity-manager-rotate-delete-offer-default.txt instead. Classified
## against .planning/design/create-flow/visual-divergence-allowlist.txt and
## .planning/design/identity-manager/visual-divergence-allowlist.txt's Phase
## 9 rows (kept in sync by TestUploadVisualAllowlistMatchesRegistry); the
## four Phase 9 negative controls (TestNegativeControl_UploadVisualMissingState,
## ...UnclassifiedDifference, ...PerturbedComparableRegion,
## ...CrossSurfaceAllowlistLeakage) plus the TestUploadVisual* acceptance
## tests run under the filter below.
gate-visual-regression:
	go test -tags screenshot -run 'Test(GateVisualRegression|ApprovalCommitRecorded|AllScreensCapturedAndNonEmpty|GlobalSSH|GlobalGit|DoctorAllowlist|DoctorHTML|DoctorMakefile|UploadVisual|UploadFrameProvenanceMatches|NegativeControl_)' -v ./cmd/gitid/...

## generate-visual-review-packet: ONE-SHOT explicit publication of a new
## content-addressed evidence packet for Task 3 review publication.
## Usage: make generate-visual-review-packet SOURCE_COMMIT=<full-sha> OUTPUT_DIR=<new-empty-dir>
## Refuses: existing/nonempty destination, dirty corrected-source set, short/unknown SHA.
## This target creates the 03-10 review packet; the routine gate never writes to
## tracked paths.
generate-visual-review-packet:
	@if [ -z "$(SOURCE_COMMIT)" ]; then echo "ERROR: SOURCE_COMMIT=<full-sha> required"; exit 1; fi
	@if [ -z "$(OUTPUT_DIR)" ]; then echo "ERROR: OUTPUT_DIR=<new-empty-dir> required"; exit 1; fi
	go run -tags screenshot ./cmd/gitid-evidence \
	    --source-commit "$(SOURCE_COMMIT)" \
	    --output-root   "$(OUTPUT_DIR)"

## smoke-network-test: D-23 skippable REAL-network two-stage connectivity
## smoke check against github.com's real alt-SSH endpoint
## (ssh.github.com:443). LOCAL/UAT convenience only — NEVER a `make
## test`/`make test-e2e`/CI prerequisite; CI stays fully deterministic via
## the FakeSSHDir PTY e2e suite (plan 03-06 Task 1) instead. Auto-skips
## (not a failure) when the network/provider itself is unreachable, vs. a
## genuine PASS/ReachableNotUploaded/Failure classification. Invokes
## TestSmokeNetworkConnectivity under its own `smoke` build tag, isolated
## from every other gate.
smoke-network-test:
	go test -tags smoke -run TestSmokeNetworkConnectivity -v ./cmd/gitid/...

## verify-upload-real-account: ONESHOT.md Phase 9 External Account Policy
## validation. LOCAL/UAT convenience only — never a prerequisite of `make test`,
## `make test-e2e`, `make lint`, or CI. The test is opt-in under the separate
## realaccount build tag and auto-skips rather than fails if gh authentication or
## either required upload scope is unavailable.
verify-upload-real-account:
	go test -tags realaccount -run '^TestRealAccountGitHubUploadRoundTrip$$' -v ./e2e/...

## GitLab variant of the target above (Phase 9.1): ONESHOT.md Phase 9 External
## Account Policy validation against a real, authenticated glab session.
## LOCAL/UAT convenience only — never a prerequisite of `make test`,
## `make test-e2e`, `make lint`, or CI. The test is opt-in under the separate
## realaccountgitlab build tag and auto-skips rather than fails if glab
## authentication or the required upload scope/capability is unavailable.
verify-upload-real-account-gitlab:
	go test -tags realaccountgitlab -run '^TestRealAccountGitLabUploadRoundTrip$$' -v ./e2e/...

## gate-no-backend-files: fail if any file changed on this branch since it
## diverged from main falls outside the Phase 2 design-only allowlist
## (SECURITY.md Finding 1 / T-02-BEGATE). Before this target existed, the
## SAME check ran only as a one-off shell line inside 02-11-PLAN.md's
## <verify> block -- executed once by the plan executor and never
## automated, so any commit added to the branch afterward (before the
## single 02-12 human-approval checkpoint) would only be caught by a human
## manually re-running that exact command from memory. This target makes
## the check repeatable and CI-able; run it standalone on design-only
## branches. The allowlist intentionally still names internal/dummytui/ and
## cmd/gitid-dummy/ (the latter now removed) -- historical Phase 2 commits
## on this branch legitimately touched them. .gitignore is allowlisted too:
## commit 13f11c4 (interactive demo, checkpoint-1 feedback) added a local
## tooling-state ignore rule (.playwright-mcp/) -- ignore-rule hygiene, not
## backend logic, which is what this gate defends against (T-02-BEGATE).
##
## RETIRED AT PHASE 3 (D-17). This target's premise -- "a branch changes only
## design files" -- is structurally false from Phase 3 onward: Phase 3 is the
## first BACKEND phase and legitimately changes internal/identity,
## internal/keygen, internal/tester, internal/tuikit and cmd/gitid. Widening
## the allowlist cannot rescue it (03-01's backend deliverables would still be
## "offending"), so the file-path form is retired rather than weakened.
##
## Its SUBSTANTIVE property -- the demo/render stack imports no backend
## package -- is now enforced durably and more strongly by an import-graph
## ALLOWLIST test that runs in the normal suite:
##
##     go test ./internal/dummytui/ -run TestNoBackendAllowlist
##
## That test allows exactly {internal/dummytui, cmd/gitid-dummy,
## internal/tuikit} and fails on ANY other first-party import by
## construction, so it catches new/renamed backend packages automatically --
## which a path-diff against main never could.
gate-no-backend-files:
	@echo "gate-no-backend-files: RETIRED at Phase 3 (D-17)."
	@echo "  The Phase-2 design-only path allowlist no longer applies: backend"
	@echo "  phases legitimately change internal/{identity,keygen,tester,tuikit}."
	@echo "  The durable guard is the import-graph allowlist test:"
	@echo "    go test ./internal/dummytui/ -run TestNoBackendAllowlist"
	@go test ./internal/dummytui/ -run TestNoBackendAllowlist

## demo-web: (re)launch the web design mockup dev server (Vite) on the
## dedicated $(DEMO_WEB_PORT) and open it in the browser.
## Stops any previous instance already listening on $(DEMO_WEB_PORT) (lsof-ti +
## kill, with a kill -9 fallback for survivors), starts Vite in the background
## via nohup (logs -> $(DEMO_WEB_LOG)) so the server survives this `make`
## invocation returning, waits for the port to accept connections, then opens
## http://localhost:$(DEMO_WEB_PORT). This is a design-only surface
## (.planning/design/mockup-src) -- never part of the Go module or any
## build/test/lint gate.
demo-web:
	@echo "==> demo-web: (re)launching the web design mockup dev server"
	@PREV_PIDS=$$(lsof -ti tcp:$(DEMO_WEB_PORT) 2>/dev/null || true); \
	if [ -n "$$PREV_PIDS" ]; then \
		echo "    stopping previous instance on port $(DEMO_WEB_PORT): $$PREV_PIDS"; \
		kill $$PREV_PIDS 2>/dev/null || true; \
		sleep 1; \
		SURVIVORS=$$(lsof -ti tcp:$(DEMO_WEB_PORT) 2>/dev/null || true); \
		if [ -n "$$SURVIVORS" ]; then \
			echo "    force-killing survivors: $$SURVIVORS"; \
			kill -9 $$SURVIVORS 2>/dev/null || true; \
		fi; \
	fi; \
	cd $(DEMO_WEB_DIR); \
	if [ ! -d node_modules ]; then \
		echo "    node_modules missing -- running pnpm install --frozen-lockfile"; \
		pnpm install --frozen-lockfile; \
	fi; \
	echo "    starting vite on port $(DEMO_WEB_PORT) (logs -> $(DEMO_WEB_LOG))"; \
	nohup pnpm exec vite --port $(DEMO_WEB_PORT) --strictPort >$(DEMO_WEB_LOG) 2>&1 & \
	echo "    waiting for the dev server to accept connections"; \
	i=0; \
	while [ $$i -lt 40 ]; do \
		if [ -n "$$(lsof -ti tcp:$(DEMO_WEB_PORT) 2>/dev/null || true)" ]; then \
			break; \
		fi; \
		sleep 0.25; \
		i=$$((i + 1)); \
	done; \
	echo "    opening http://localhost:$(DEMO_WEB_PORT)"; \
	open http://localhost:$(DEMO_WEB_PORT)
