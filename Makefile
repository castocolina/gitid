# Makefile — single task-runner surface for gitid.
# All targets are .PHONY (no file artifacts tracked by make).
# pre-commit hooks and future CI call these same targets — single source of truth.
#
# Targets:
#   setup-env   Install development tools (goimports, golangci-lint, gosec, pre-commit)
#               and wire git hooks via install-hooks (completed in plan 01-03).
#   build       Compile the gitid binary to bin/gitid.
#   install     Install gitid to $GOPATH/bin via go install.
#   uninstall   Remove gitid from $GOPATH/bin.
#   test        Run the race-enabled test harness with a coverage profile (TDD harness, D-06).
#   lint        Run golangci-lint (reads .golangci.yml); hard-fails on any finding (D-04).
#   fmt         Run goimports then gofmt over all packages.

.PHONY: setup-env build install uninstall test lint fmt install-hooks test-e2e

# Binary output directory.
BIN_DIR := bin
BINARY  := $(BIN_DIR)/gitid

# Go binary locations.
GOPATH_BIN := $(shell go env GOPATH)/bin

# golangci-lint version to install (pinned — do NOT change without updating STACK.md).
GOLANGCI_LINT_VERSION := v2.12.2

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
	uv tool install pre-commit
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
test-e2e: build
	go test -tags e2e -race -timeout 60s ./e2e/...
