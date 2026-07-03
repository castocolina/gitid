# Makefile — single task-runner surface for gitid.
# All targets are .PHONY (no file artifacts tracked by make).
# pre-commit hooks and future CI call these same targets — single source of truth.
#
# Targets:
#   setup-env      Install development tools (goimports, golangci-lint, gosec, pre-commit,
#                  freeze) and provision the pinned Chromium revision; wire git hooks via
#                  install-hooks (completed in plan 01-03; screenshot tooling in 01-05).
#   build          Compile the gitid binary to bin/gitid.
#   build-cross    Cross-compile the release build matrix (darwin/amd64, darwin/arm64,
#                  linux/amd64, linux/arm64 [build-only]) to bin/gitid-<os>-<arch>
#                  (BUILD-01). Cross-compilation via GOOS/GOARCH is OS-independent, so
#                  CI runs this ONCE on ubuntu-latest rather than on every matrix runner.
#                  No release/tag/checksum packaging here — that is BUILD-03, Phase 10.
#   install        Install gitid to $GOPATH/bin via go install.
#   uninstall      Remove gitid from $GOPATH/bin.
#   test           Run the race-enabled test harness with a coverage profile (TDD harness, D-06).
#   lint           Run golangci-lint (reads .golangci.yml); hard-fails on any finding (D-04).
#   fmt            Run goimports then gofmt over all packages.
#   screenshot-tui  Render the TUI View()-dump golden to a deterministic PNG via freeze
#                   (TOOL-05, DLV-03; build-tag isolated behind `screenshot`).
#   screenshot-html Render the fixture HTML page to a deterministic PNG via headless
#                   Chromium (go-rod, pinned revision; TOOL-05, DLV-03).
#   screenshot-html-mockups Build the Phase 2 MUI mockup SPA (dist/) and capture one PNG
#                   per .planning/design/*/manifest.json HTML screen (extends Phase 1's
#                   screenshot-html pipeline via internal/screenshot/design_adapter.go).
#   screenshot-tui-mockups  Capture one PNG per manifest TUI screen via
#                   internal/dummytui.RenderScreen (extends Phase 1's screenshot-tui
#                   pipeline).
#   dummy-nav-e2e   Drive the real cmd/gitid-dummy binary over a PTY, proving every
#                   manifest screen is reachable via absolute keystrokes before any
#                   design-review presentation (DLV-05).

.PHONY: setup-env build build-cross install uninstall test lint fmt install-hooks test-e2e screenshot-tui screenshot-html screenshot-html-mockups screenshot-tui-mockups dummy-nav-e2e

# Binary output directory.
BIN_DIR := bin
BINARY  := $(BIN_DIR)/gitid

# Go binary locations.
GOPATH_BIN := $(shell go env GOPATH)/bin

# golangci-lint version to install (pinned — do NOT change without updating STACK.md).
GOLANGCI_LINT_VERSION := v2.12.2

# freeze version to install (pinned — dev/build tool only, never a runtime dep of the
# shipped gitid binary; see internal/screenshot/tui.go, build-tag isolated). Supply-chain
# provenance recorded in .planning/design/_spike/GOLDENS.md (01-05 Task 1).
FREEZE_VERSION := v0.2.2

# Vendored monospace font + fixed theme for deterministic screenshot-tui rendering
# (Pitfall 6 — freeze's default font discovery is not CI-deterministic). These are the
# same values internal/screenshot/tui_capture_test.go passes to freeze's --font.file /
# --theme flags at a fixed 100x30 (cols x rows) capture geometry (D-04); recorded here
# too so a fresh clone can see, without reading Go source, which font/theme/geometry a
# reproduced golden depends on.
SCREENSHOT_FONT  := $(CURDIR)/.planning/design/fonts/JetBrainsMono-Regular.ttf
SCREENSHOT_THEME := dracula

# Resolved tool binaries, referenced by absolute path so recipes run regardless of the
# caller's PATH. GNU Make 3.81 (macOS) direct-execs a bare command (no shell metacharacters)
# using its ORIGINAL PATH, ignoring the `export PATH` below — so a bare `golangci-lint` fails
# when ~/go/bin isn't already on PATH. Absolute paths sidestep that entirely. setup-env
# installs both binaries into $(GOPATH_BIN).
GOLANGCI_LINT := $(GOPATH_BIN)/golangci-lint
GOIMPORTS     := $(GOPATH_BIN)/goimports

# Capture the caller's REAL interactive PATH *before* the export below clobbers it.
# The `install` target must judge PATH membership against what the user's shell will
# actually see — not against the make-augmented PATH (which always contains GOPATH_BIN,
# making the check a guaranteed false "PATH: OK"). FIX-INSTALL-01 / F-1.
ORIGINAL_PATH := $(PATH)

# Ensure tool bin dirs are on PATH for EVERY recipe line, the install-hooks sub-make,
# and make-invoked git hooks — so a fresh clone bootstraps without relying on the
# caller's interactive PATH (review WR-01). uv installs pre-commit into ~/.local/bin;
# go install and the golangci-lint installer place binaries in $(GOPATH_BIN).
export PATH := $(HOME)/.local/bin:$(GOPATH_BIN):$(PATH)

## setup-env: install all development tools and prepare the git hooks.
##
## Tools installed:
##   goimports     — import block formatter (run as standalone + via golangci-lint)
##   golangci-lint — lint aggregator, v2.12.2, installed via the official binary
##                   installer (NOT go install — avoids Go-version-mismatch silent breakage,
##                   per STACK.md and CLAUDE.md).
##   gosec         — standalone security linter binary (also embedded in golangci-lint;
##                   installed separately for direct invocation if needed).
##   pre-commit    — git hook runner; hooks point at make targets.
##   freeze        — ANSI terminal-output -> PNG renderer for `screenshot-tui`, pinned
##                   @v0.2.2 (dev/build tool only — never a runtime dep of the shipped
##                   gitid binary; Pitfall 8: unlike golangci-lint, `go install` is fine
##                   for freeze).
##   pinned Chromium revision — headless-Chromium build `screenshot-html` drives via
##                   go-rod, pre-downloaded into the fixed cache path so a later
##                   `make screenshot-html` never pays the download cost (or fails
##                   offline) on a fresh clone (T-01-SC2).
##
## Git hook wiring (pre-commit install, pre-push install) is completed in plan 01-03
## via the install-hooks sub-target below.  setup-env calls install-hooks so that once
## 01-03 defines it fully, a single `make setup-env` bootstraps a fresh clone end-to-end.
setup-env:
	@echo "==> Installing goimports"
	go install golang.org/x/tools/cmd/goimports@latest
	@echo "==> Installing golangci-lint $(GOLANGCI_LINT_VERSION) via official binary installer"
	curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -b "$(GOPATH_BIN)" $(GOLANGCI_LINT_VERSION)
	@echo "==> Installing gosec (standalone binary)"
	go install github.com/securego/gosec/v2/cmd/gosec@latest
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
	@echo "==> Wiring git hooks"
	$(MAKE) install-hooks
	@echo "==> setup-env complete"

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
	find . -name "*.go" -not -path "./.planning/*" -exec gofmt -w {} +

## lint: run golangci-lint against all packages.
## Hard-fails on any finding — zero tolerance (D-04).
## Configuration lives in .golangci.yml.
lint:
	$(GOLANGCI_LINT) run ./...

## test: run the TDD harness with race detection and a coverage profile.
## Coverage is report-only in Phase 1; no hard threshold (D-09 discretion).
## This is the same command pre-push hooks and future CI will call (D-06).
test:
	go test -race -coverprofile=coverage.out ./...

## build: compile the gitid binary.
build:
	@mkdir -p $(BIN_DIR)
	go build -o $(BINARY) ./cmd/gitid

## build-cross: cross-compile the release build matrix reproducibly (BUILD-01).
## darwin/amd64, darwin/arm64, and linux/amd64 are the gated matrix targets; linux/arm64
## is included build-only ("if cheap" per D-14) and is NOT part of any CI gate. GOOS/GOARCH
## cross-compilation is OS-independent (no cgo in this module), so this target is invoked
## ONCE on a single Linux runner in CI rather than redundantly on every matrix OS. Output
## binaries are named bin/gitid-<os>-<arch> — no release/tag/checksum packaging here
## (BUILD-03 is Phase 10, out of scope).
build-cross:
	@mkdir -p $(BIN_DIR)
	GOOS=darwin  GOARCH=amd64 go build -o $(BIN_DIR)/gitid-darwin-amd64 ./cmd/gitid
	GOOS=darwin  GOARCH=arm64 go build -o $(BIN_DIR)/gitid-darwin-arm64 ./cmd/gitid
	GOOS=linux   GOARCH=amd64 go build -o $(BIN_DIR)/gitid-linux-amd64  ./cmd/gitid
	GOOS=linux   GOARCH=arm64 go build -o $(BIN_DIR)/gitid-linux-arm64  ./cmd/gitid

## install: install gitid to $GOPATH/bin and report the install path + PATH status.
install:
	go install ./cmd/gitid
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
## Timeout raised 60s -> 180s (02-11): the whole ./e2e/... package now includes the
## comprehensive dummy-nav-e2e walk (all 50 screens across all 7 Phase-2 surfaces,
## finalized this plan) alongside the pre-existing real-TUI PTY suite -- observed
## ~80s under -race locally; 180s gives CI-variance headroom without masking a
## genuine hang (dummy-nav-e2e/TestUIPTY_* each carry their own inner waitFor
## timeouts, so a real hang still fails fast well under 180s).
test-e2e: build
	go test -tags e2e -race -timeout 180s ./e2e/...

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

## screenshot-html-mockups: build the Phase 2 MUI mockup SPA and capture one
## PNG per .planning/design/*/manifest.json HTML screen (DLV-01, DLV-02,
## extends Phase 1's screenshot-html pipeline -- 02-03-PLAN.md).
## `pnpm build` runs scripts/verify-routes.mjs (the route-uniqueness/shape
## gate) before `vite build` -- a bad or duplicate mockup route fails the
## build loudly, before any capture step runs. ALWAYS --frozen-lockfile
## (never an unpinned dependency install, which could pull drifted deps in
## CI -- T-02-SC3). Invokes TestCaptureAllMockupScreens's "html" subtests --
## the concrete runnable entry point that actually writes each PNG under
## .planning/design/<surface>/html/, gated on the "<surface>/<screen>"
## breadcrumb rendering correctly (never a blank/wrong-route PNG).
screenshot-html-mockups:
	cd .planning/design/mockup-src && pnpm i --frozen-lockfile && pnpm build
	go test -tags screenshot -run 'TestCaptureAllMockupScreens/.*/html' ./internal/screenshot/...

## screenshot-tui-mockups: capture one PNG per .planning/design/*/manifest.json
## TUI screen via internal/dummytui.RenderScreen (DLV-01, DLV-02, extends
## Phase 1's screenshot-tui pipeline -- 02-03-PLAN.md). Invokes
## TestCaptureAllMockupScreens's "tui" subtests -- the concrete runnable
## entry point that actually writes each PNG under
## .planning/design/<surface>/tui/, gated on the same breadcrumb assertion
## as the html-mockups target.
screenshot-tui-mockups:
	go test -tags screenshot -run 'TestCaptureAllMockupScreens/.*/tui' ./internal/screenshot/...

## dummy-nav-e2e: run the dummy-tui PTY navigation proof (builds gitid-dummy
## first). Drives the REAL cmd/gitid-dummy binary via raw PTY keystrokes,
## re-homing before each manifest entry and asserting the active screen
## breadcrumb + a screen-specific signature, then asserts zero files were
## written under a sandboxed HOME (DLV-05). Tests are tagged //go:build e2e
## and are excluded from the normal make test target (same convention as
## test-e2e).
dummy-nav-e2e:
	go build -o $(BIN_DIR)/gitid-dummy ./cmd/gitid-dummy
	go test -tags e2e -race -timeout 60s -run TestDummyNav ./e2e/...
