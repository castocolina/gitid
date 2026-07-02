# Phase 2: DESIGN — All Mockups (CHECKPOINT #1) - Pattern Map

**Mapped:** 2026-07-02
**Files analyzed:** 12 (Go: 6, Makefile: 1, npm/React: 1 workspace treated as 1 unit, e2e: 1, screenshot-driver: 1, design-artifact scaffolding: not code)
**Analogs found:** 5 exact/role-match / 6 total code-bearing targets; 1 explicit "no analog" (Node/React toolchain)

**IMPORTANT — repo state note:** `internal/screenshot/` does **not exist on disk yet**. Phase 1
(`01-foundations-spikes-ci`) planned it in `01-05-PLAN.md` but that plan is **unexecuted**
(confirmed via `ls internal/` — no `screenshot` directory; `Makefile` has no
`screenshot-tui`/`screenshot-html` targets yet, only `test`, `build`, `install`, `test-e2e`).
Phase 2 plans that "extend" `internal/screenshot` must therefore either (a) depend on Phase 1
having executed first, or (b) treat 01-05-PLAN.md's `must_haves.artifacts` table as the contract
to build against directly. Cite the PLAN, not on-disk code, per the orchestrator's instruction.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/dummytui/model.go` | provider (`tea.Model`) | event-driven | `tui/model.go` | role-match (same event-driven Elm-architecture shape; new file has zero backend deps, analog has many) |
| `internal/dummytui/screens.go` | model/config (screen enum + hardcoded fixtures) | CRUD (static in-memory read) | `tui/model.go` lines 19-47 (`viewKind`/`modalKind` enums) | role-match |
| `internal/dummytui/data.go` | model (hardcoded fixture data) | CRUD (static reads) | no direct analog (real `tui/` sources live data from `internal/identity` etc.) — pattern the shape after `tui/model.go`'s `tuiDeps` struct fields but hardcode literals instead of injecting deps | partial match |
| `internal/dummytui/overlay.go` | utility (modal compositing) | transform | `tui/overlay.go` (full file, 204 lines) | exact — reimplement verbatim algorithm, backend-free |
| `cmd/gitid-dummy/main.go` | controller (binary entrypoint) | request-response (program bootstrap) | `tui/tui.go` (`Run()`, lines 1-30) + `cmd/gitid/main.go` (`main()`, lines 37-60) | role-match (bootstrap shape from `tui.go`; NOT the Cobra-command-tree shape from `cmd/gitid/main.go`, which is over-scoped for a nav-only dummy) |
| `e2e/dummy_nav_e2e_test.go` | test (PTY e2e) | event-driven / streaming (raw keystroke → PTY → vt.Emulator) | `e2e/ui_pty_e2e_test.go` (full file, 718 lines; excerpted below) + `e2e/harness_test.go` (`BuildBinary`, `SandboxHome`) | exact — reuse harness verbatim, swap binary + keystroke script |
| `internal/screenshot/design_capture_test.go` | test (capture driver) | batch (iterate N screens) | 01-05-PLAN.md `must_haves.artifacts` contract for `internal/screenshot/tui_capture_test.go` / `html_capture_test.go` (`TestCaptureTUI`/`TestCaptureHTML`) — **not on-disk code, cite the PLAN** | role-match (PLAN's intended API is the contract) |
| Makefile targets `screenshot-html-mockups`, `screenshot-tui-mockups`, `dummy-nav-e2e` | config (build orchestration) | batch / request-response | `Makefile` targets `test-e2e` (lines 123-126) and 01-05-PLAN.md's undelivered `screenshot-tui`/`screenshot-html` target spec | role-match |
| `.planning/design/mockup-src/` (Vite+React+MUI v7 workspace) | component (SPA) | request-response (client render) | **none in this Go repo** — first Node.js toolchain | no analog — see below |

## Pattern Assignments

### `internal/dummytui/model.go` (provider, event-driven)

**Analog:** `tui/model.go`

**Imports pattern** (`tui/model.go` lines 1-17):
```go
package tui

import (
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/castocolina/gitid/internal/adopter"
	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
	"github.com/castocolina/gitid/internal/repoclone"
	"github.com/castocolina/gitid/internal/uploader"
)
```
**Deviation the new file MUST make:** `internal/dummytui` imports ONLY
`charm.land/bubbletea/v2`, `charm.land/lipgloss/v2`, `charm.land/bubbles/v2` (RESEARCH.md
Standard Stack) — the entire `github.com/castocolina/gitid/internal/*` backend block is
forbidden. This is the exact DLV-05 boundary; RESEARCH.md Pattern 2 gives the `go list -deps`
grep check that enforces it in CI.

**View-enum + number-key routing pattern** (`tui/model.go` lines 19-26 and 594-599):
```go
type viewKind int

const (
	identitiesView    viewKind = iota // View 1: Identities (default)
	healthView                        // View 2: Health (doctor)
	globalOptionsView                 // View 3: Global Options
)
...
	case "1":
		return m.switchTo(identitiesView)
	case "2":
		return m.switchTo(healthView)
	case "3":
		return m.switchTo(globalOptionsView)
```
**Deviation:** the dummy's screen enum has 5 primary numbered views per
02-UX-DIRECTION.md §2 (`1 Identities · 2 Global SSH · 3 Global Git · 4 Health · 5 Fixer`),
not `tui/model.go`'s current 3 (`identitiesView`/`healthView`/`globalOptionsView` — the real
product apparently hasn't grown Global Git/Fixer as top-level numbers yet). Copy the
`switchTo`-per-number-key structure, extend to `case "4"`/`case "5"`, and add the
create-flow/git-screen modal entry points launched FROM the Identities view (not top-level
numbers) per UX-DIRECTION §2.

**View() + AltScreen pattern** (`tui/model.go` lines 954-958):
```go
func (m rootModel) View() tea.View {
	v := tea.NewView(m.renderContent())
	v.AltScreen = true
	return v
}
```
Copy verbatim shape — `tea.View.AltScreen = true` is the v2 substitute for the removed
`tea.WithAltScreen()` (RESEARCH.md "State of the Art" note in `tui/tui.go` lines 11-12).

**Modal compositing dispatch pattern** (`tui/model.go` lines 963-999, `renderContent`):
```go
	layout := m.renderPersistentLayout()

	if m.activeModal == noModal {
		return layout
	}

	// Dim the persistent layout before compositing the modal overlay (D-02).
	dimmed := StyleDimmed.Render(layout)

	var modalContent string
	switch m.activeModal {
	case helpModal:
		modalContent = renderHelpModal(m.width)
	...
	}
```
Copy this dim-then-composite dispatch shape for the dummy's clone/new-key/delete-choice
modals (02-UX-DIRECTION.md §4.3 Identity Manager states).

---

### `internal/dummytui/overlay.go` (utility, transform)

**Analog:** `tui/overlay.go` — copy the FULL file's algorithm verbatim (204 lines), reimplemented
inside `internal/dummytui` with no import of `tui/`.

**Why an exact copy, not adaptation:** RESEARCH.md Anti-Patterns explicitly forbids importing
`tui/` "just for the overlay helper" — `tui/` transitively imports `internal/doctor`,
`internal/identity`, etc. via `tui/deps.go`, which would break the DLV-05 import-graph check.

**Core algorithm to copy** (`tui/overlay.go` lines 56-121, `placeOverlay`/`overlayLine`/`modalOrigin`):
```go
func placeOverlay(x, y int, fg, bg string) string {
	bgLines := strings.Split(bg, "\n")
	fgLines := strings.Split(fg, "\n")

	for i, fgLine := range fgLines {
		bgRow := y + i
		if bgRow < 0 || bgRow >= len(bgLines) {
			continue
		}
		bgLines[bgRow] = overlayLine(x, fgLine, bgLines[bgRow])
	}
	return strings.Join(bgLines, "\n")
}
```
Also copy `overlayLine` (ANSI-width-safe splice via `lipgloss.Width` + `ansi.Truncate`/
`ansi.TruncateLeft`, lines 83-106), `modalOrigin` (centering, lines 111-121), and
`boundModalToViewport` (scroll-clamped modal-in-viewport, lines 133-186) — the dummy's modal
screens (clone/new-key/delete-choice) will need all four, exactly as documented in
RESEARCH.md Pitfall 1.

**Imports** (`tui/overlay.go` lines 32-37):
```go
import (
	"strings"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)
```
No deviation needed — this import block is already backend-free.

---

### `cmd/gitid-dummy/main.go` (controller, request-response bootstrap)

**Analog:** `tui/tui.go` (`Run()`, full file) for the `tea.NewProgram(...).Run()` shape;
`cmd/gitid/main.go` (`main()`, lines 37-60) for the thin-main pattern.

**Bootstrap pattern to copy** (`tui/tui.go` lines 19-30):
```go
func Run() error {
	doctorDeps, identityDeps, updateDeps, deleteDeps, adoptDeps, repoCloneDeps, uploaderDeps, err := buildTUIDeps()
	if err != nil {
		return fmt.Errorf("tui: building deps: %w", err)
	}
	m := newRootModelFull(doctorDeps, identityDeps, updateDeps, deleteDeps, adoptDeps, repoCloneDeps, uploaderDeps)
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("tui: program error: %w", err)
	}
	return nil
}
```
**Deviation:** the dummy has NO deps to build (no `buildTUIDeps()` call, no backend structs) —
`cmd/gitid-dummy/main.go` calls `dummytui.NewModel()` directly with zero arguments (hardcoded
data lives inside `internal/dummytui/data.go`), then `tea.NewProgram(m).Run()`. Do not mirror
`cmd/gitid/main.go`'s Cobra command-tree (`newRootCmd()`, `identity`/`baseline`/`doctor`
subcommand groups, lines 71-139) — that is the real product's CLI surface and is entirely out
of scope; `gitid-dummy` is a single always-launches-TUI binary, closer to the `isTTY` branch
of `cmd/gitid/main.go` lines 37-43 alone:
```go
func main() {
	if len(os.Args) == 1 {
		isTTY := term.IsTerminal(int(os.Stdout.Fd()))
		code := noArgsAction(isTTY, tui.Run, os.Stdout, os.Stderr)
		os.Exit(code)
	}
	...
}
```
Copy only this TTY-gate + `os.Exit(code)` idiom (drop the Cobra `Execute()` branch entirely).

---

### `e2e/dummy_nav_e2e_test.go` (test, event-driven PTY)

**Analog:** `e2e/ui_pty_e2e_test.go` (reuse the harness verbatim) + `e2e/harness_test.go`
(`BuildBinary`, `SandboxHome`).

**Harness types/functions to reuse UNCHANGED** (`e2e/ui_pty_e2e_test.go`):
- `ptySession` struct (lines 68-76) and `startPTY(t, cmd)` (lines 96-186) — single-owner
  `emu` goroutine design; do not re-derive.
- `(s *ptySession) close(t)` (lines 190-204), `sendKey(raw, delay)` (lines 209-212),
  `snapshot()` (lines 217-230), `waitFor(timeout, predicate)` (lines 234-244).
- `saveFrame(t, name, s)` (lines 248-260) — **deviation:** the dummy test must write frames
  to a Phase-2-appropriate directory, not
  `.planning/phases/05.7-complete-v1-0-product-features-in-tui/ui-frames/`. Point it at
  something under `.planning/design/` (e.g. `.planning/design/dummy-nav-frames/`) so evidence
  collocates with the rest of Phase 2's design artifacts.

**New helper needed (mirrors `BuildBinary`):** `e2e/harness_test.go`'s `BuildBinary` (lines
44-73) builds `./cmd/gitid` via `sync.Once`-cached `go build`. The dummy test needs an
equivalent `BuildDummyBinary(t)` that runs
`exec.Command("go", "build", "-o", bin, "./cmd/gitid-dummy")` — copy the exact
`sync.Once`/temp-dir/`HOME=realHome`-restore structure (lines 44-73), just retarget the
build path and use a separate package-level `sync.Once`/binary-path pair so it doesn't
collide with the real-binary cache.

**Test body shape to copy** (RESEARCH.md Code Examples, itself sourced from this harness):
```go
func TestDummyNavReachesAllScreens(t *testing.T) {
	bin := BuildDummyBinary(t)
	home := SandboxHome(t)
	cmd := exec.Command(bin) //nolint:gosec // arg-slice, binary from BuildDummyBinary
	cmd.Env = append(os.Environ(), "HOME="+home)
	s := startPTY(t, cmd)
	defer s.close(t)

	s.sendKey([]byte("2"), 100*time.Millisecond)
	if _, ok := s.waitFor(2*time.Second, func(txt string) bool {
		return strings.Contains(txt, "StrictHostKeyChecking")
	}); !ok {
		t.Fatal("dummy nav: Global SSH options screen not reached on '2'")
	}
	saveFrame(t, "global-ssh-reached", s)
}
```
**Deviation:** number-key targets must be re-verified against the dummy's own 5-view enum
(1 Identities · 2 Global SSH · 3 Global Git · 4 Health · 5 Fixer per UX-DIRECTION §2) — the
example above (`"2"` → Global SSH) already matches that mapping, unlike `tui/model.go`'s
current 3-view enum.

**Build-tag convention** (`e2e/ui_pty_e2e_test.go` line 1, `e2e/harness_test.go` line 1):
```go
//go:build e2e
```
Copy verbatim — same tag, same exclusion from `make test`, included only in `make test-e2e`.

---

### `internal/screenshot/design_capture_test.go` (test, batch capture driver)

**Analog:** 01-05-PLAN.md's `must_haves.artifacts` contract for `TestCaptureTUI`/
`TestCaptureHTML` (file does not exist on disk — Phase 1 unexecuted; cite the PLAN, not code).

**Contract to build against** (01-05-PLAN.md lines ~40-58, `must_haves.artifacts` /
`key_links`):
```
provides: "TestCaptureTUI — runnable capture entry point make screenshot-tui invokes (build-tag: screenshot)"
contains: "func TestCaptureTUI"
...
provides: "TestCaptureHTML — runnable capture entry point make screenshot-html invokes (build-tag: screenshot)"
contains: "func TestCaptureHTML"
...
to: "go test -tags screenshot -run TestCaptureTUI|TestCaptureHTML ./internal/screenshot/..."
```

**Extension pattern** (RESEARCH.md § Pattern 1, already written against this same
not-yet-built contract):
```go
// internal/screenshot/design_capture_test.go (NEW, //go:build screenshot)
var htmlScreens = []struct{ surface, screen, route string }{
    {"create-flow", "algorithm-catalog", "/create/algorithm"},
    // ... one row per screen across all 7 surfaces, lifted from
    // 02-UX-DIRECTION.md §4's per-surface state manifests
}

func TestCaptureAllMockupScreens(t *testing.T) {
    distIndex := filepath.Join(repoRoot(t), ".planning/design/mockup-src/dist/index.html")
    for _, s := range htmlScreens {
        url := "file://" + distIndex + "#" + s.route
        out := filepath.Join(repoRoot(t), ".planning/design", s.surface, "html", s.screen+".png")
        if err := captureHTML(url, out); err != nil {
            t.Fatalf("%s/%s: %v", s.surface, s.screen, err)
        }
    }
}
```
**Deviation/dependency note:** this file calls `captureHTML`/`captureTUI` functions that
Phase 1's `internal/screenshot/html.go`/`tui.go` are supposed to export but have not yet been
committed. The plan for this file MUST either (a) declare a hard dependency on Phase 1 Plan
05 executing first, or (b) the phase's own plan set builds a minimal version of those Phase-1
functions itself, clearly flagged as "duplicating Phase 1 scope, remove when Phase 1 lands."
Flag this dependency explicitly for the planner — do not silently assume the functions exist.

---

### Makefile targets `screenshot-html-mockups` / `screenshot-tui-mockups` / `dummy-nav-e2e`

**Analog:** `Makefile` `test-e2e` target (lines 123-126) for the build-then-test shape, and
01-05-PLAN.md's undelivered `screenshot-tui`/`screenshot-html` target spec for the
`-tags screenshot -run TestCapture...` invocation shape.

**Existing target to copy the shape of** (`Makefile` lines 123-126):
```makefile
## test-e2e: run end-to-end agent-driven tests (builds binary first).
## E2E tests use a hermetic sandbox HOME and a fake ssh script injected on PATH.
## Tests are tagged //go:build e2e and are excluded from the normal make test target.
test-e2e: build
	go test -tags e2e -race -timeout 60s ./e2e/...
```
**New target shape (dummy-nav-e2e), following this exact convention:**
```makefile
## dummy-nav-e2e: run the dummy-tui PTY navigation proof (builds gitid-dummy first).
dummy-nav-e2e:
	go build -o bin/gitid-dummy ./cmd/gitid-dummy
	go test -tags e2e -race -timeout 60s -run TestDummyNav ./e2e/...
```
**New targets (screenshot-html-mockups / screenshot-tui-mockups), following 01-05-PLAN.md's
key_links pattern** (`to: "go test -tags screenshot -run TestCaptureTUI|TestCaptureHTML ./internal/screenshot/..."`):
```makefile
screenshot-html-mockups:
	cd .planning/design/mockup-src && pnpm install --frozen-lockfile && pnpm build
	go test -tags screenshot -run TestCaptureAllMockupScreens/html ./internal/screenshot/...

screenshot-tui-mockups:
	go test -tags screenshot -run TestCaptureAllMockupScreens/tui ./internal/screenshot/...
```
**Deviation:** these are new targets (not modifications of `screenshot-tui`/`screenshot-html`,
which Phase 1 owns and hasn't shipped) — name them distinctly (`-mockups` suffix) so Phase 1's
single-fixture targets and Phase 2's N-screen targets don't collide once both exist. Add
`.PHONY` entries alongside the existing `.PHONY: setup-env build install uninstall test lint fmt install-hooks test-e2e` line (Makefile line 15).

---

## Shared Patterns

### No-backend-import boundary (DLV-05)
**Source:** RESEARCH.md § Pattern 2 (`go list -deps` grep check), mirroring the technique
already used to keep `freeze`/`go-rod` out of `cmd/gitid`'s dependency graph.
**Apply to:** `internal/dummytui/*.go` and `cmd/gitid-dummy/main.go` — every file in this new
package tree.
```bash
for pkg in identity keygen sshconfig gitconfig filewriter tester doctor adopter uploader; do
  if go list -deps ./cmd/gitid-dummy/... ./internal/dummytui/... | grep -q "internal/${pkg}"; then
    echo "FAIL: dummy imports backend package internal/${pkg}"
    exit 1
  fi
done
```

### Build-tag isolation for dev/design tooling
**Source:** `e2e/ui_pty_e2e_test.go` line 1 (`//go:build e2e`), and 01-05-PLAN.md's
`//go:build screenshot` convention for `internal/screenshot/*.go`.
**Apply to:** `e2e/dummy_nav_e2e_test.go` (`//go:build e2e`), `internal/screenshot/design_capture_test.go` (`//go:build screenshot`) — never let design/test-only deps (`creack/pty`, `x/vt`, `go-rod`, `freeze`) leak into the default `go build ./...` graph.

### Terminal-native theming discipline (not a Go code pattern, but binds `internal/dummytui`'s
static View() output AND the MUI mockup's theme)
**Source:** 02-UX-DIRECTION.md §0 Risk 1 + §2 (shared shell: header/body/status/keybar
regions, ANSI-safe semantic color table, number-key nav 1-5 + palette, reserved keys
`Esc`/`q`/`?`/`/`/`Enter`).
**Apply to:** every screen `internal/dummytui` renders (reuse `tui/model.go`'s
`renderPersistentLayout`/region-composition shape, not its business logic) AND every MUI route
under `.planning/design/mockup-src/src/routes/`.

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `.planning/design/mockup-src/` (React 19 + MUI v7 + Vite + TypeScript SPA) | component (SPA) | request-response | **First Node.js/npm toolchain in this all-Go repo** (RESEARCH.md § Package Legitimacy Audit "New supply-chain surface note"). No Go analog exists or is meaningful. Map instead to: the project's `/mui` skill conventions (v7 slots/slotProps, `sx` prop, package-exports-only imports per RESEARCH.md § State of the Art), 02-UX-DIRECTION.md §0-§2 for the terminal-skin theme constraints, and RESEARCH.md § Pattern 3 (`base: './'` + `HashRouter` for `file://` capture compatibility) as the concrete build-config pattern to follow. Package legitimacy for all 10 new npm packages is already audited (RESEARCH.md table, all `[OK]`) — no further supply-chain check needed by this agent. |
| `internal/dummytui/data.go` (hardcoded fixture data) | model | CRUD (static) | No real fixture-data file exists in this repo to imitate structurally (the real `tui/` sources its data live from `internal/identity`/`internal/doctor` structs, not literals). Build from `02-UX-DIRECTION.md` §4's per-surface state manifests directly — each named state (e.g. `list-empty`, `list-populated`, `detail-ssh-first`) becomes one hardcoded fixture value. |

## Metadata

**Analog search scope:** `tui/`, `cmd/gitid/`, `e2e/`, `Makefile`, `.planning/phases/01-foundations-spikes-ci/01-05-PLAN.md` (Phase 1's unexecuted screenshot-tooling plan), `internal/` (confirmed `internal/screenshot` absent).
**Files scanned:** `tui/model.go` (1341 lines, targeted reads), `tui/tui.go` (30 lines, full), `tui/overlay.go` (204 lines, full), `cmd/gitid/main.go` (139 lines, full), `e2e/ui_pty_e2e_test.go` (718 lines, first 260 read), `e2e/harness_test.go` (283 lines, first 80 read), `Makefile` (targeted `test-e2e`/`build`/`install` section), `01-05-PLAN.md` (first 140 lines).
**Pattern extraction date:** 2026-07-02
