# Screenshot tooling — golden hashes + supply-chain provenance

Phase 1, Plan 05 (TOOL-05, DLV-03, TOOL-02). This file records:

1. The automated supply-chain provenance review for `freeze` and `go-rod` (Task 1 —
   replaces the former blocking-human legitimacy checkpoint per the Codex MEDIUM
   resolution baked into 01-05-PLAN.md).
2. The recorded golden SHA-256 hashes `TestCaptureTUI` / `TestCaptureHTML` assert
   against on every re-run, so `make screenshot-tui` / `make screenshot-html`
   reproducibility is provable, not just claimed.

## 1. Supply-chain provenance (Task 1)

| Dependency | Exact pin | Role | Verification |
|------------|-----------|------|---------------|
| `github.com/charmbracelet/freeze` | `v0.2.2` | Dev/build tool (ANSI -> PNG renderer), installed via `go install` in `make setup-env`. Never a runtime import of the gitid binary. | RESEARCH.md § Package Legitimacy Audit: slopcheck `[OK]`, isolated-scratch-module `go build` succeeded, empirically run this session producing a real 640x332 PNG. Codex (01-REVIEWS.md) independently re-confirmed the package exists at this pin against live GitHub/proxy.golang.org sources. |
| `github.com/go-rod/rod` | `v0.116.2` | Real Go module dependency (headless-Chromium driver), imported only from `internal/screenshot/html.go` behind `//go:build screenshot`. Never enters `go build ./cmd/gitid`'s dependency graph — verified: `go list -deps ./cmd/gitid` does not contain `go-rod`. | RESEARCH.md § Package Legitimacy Audit: slopcheck `[OK]`, isolated-scratch-module `go build` succeeded. Codex independently re-confirmed the package exists at this pin. `go mod verify` passes (`all modules verified`) — every module's content hash matches the Go checksum database (sum.golang.org); no `GONOSUMCHECK`/`GOFLAGS=-insecure` was ever set. |

**Scope note:** the "no `@latest`" check for this plan is scoped to `go.mod` and the
freeze install line in the Makefile, NOT the whole Makefile — `setup-env` already
carries pre-existing, unrelated dev-tool installs (`goimports@latest`,
`gosec@latest`) from earlier phases that are out of scope for this plan's
supply-chain review (Codex HIGH, folded into 01-05-PLAN.md Task 1).

**Pinned Chromium revision (go-rod's own `launcher.RevisionDefault` at v0.116.2,
re-pinned explicitly as `screenshot.ChromiumRevision` in `internal/screenshot/html.go`
so it never silently drifts if a future go-rod upgrade changes its own default):**

```
1321438
```

Cache path (fixed, OS-appropriate; go-rod's `launcher.DefaultBrowserDir`):
`$HOME/.cache/rod/browser` on macOS/Linux, `%APPDATA%\rod\browser` on Windows.
`internal/screenshot/html.go`'s `resolveBrowserBinary` never falls back to a
different revision or a different browser — if the pinned revision is not already
valid at the cache path and downloading is disabled, it fails fast with an
actionable error naming both the revision and the cache path (T-01-SC2; proven by
`TestCaptureHTML_OfflineFailurePath`).

**Disposition:** Approved. Both dependencies are pinned, checksum-DB verified, and
build-tag isolated from the shipped binary. No second human checkpoint introduced
(Codex MEDIUM resolved) — this plan runs `autonomous: true`.

## 2. TUI golden (Task 2 — `make screenshot-tui`)

| Field | Value |
|-------|-------|
| Entry point | `go test -tags screenshot -run TestCaptureTUI ./internal/screenshot/...` |
| Fixture | `fixtureModel` in `internal/screenshot/tui_capture_test.go` — a trivial Bubble Tea `View()` dump, NOT product UI (Phase 2 replaces it with the real design-approved TUI mockups) |
| Capture geometry (D-04) | 100x30 (cols x rows) |
| Vendored font | `.planning/design/fonts/JetBrainsMono-Regular.ttf` (see `fonts/README.md` for provenance) — always passed via `--font.file` |
| Fixed theme | `dracula` (`--theme dracula`) |
| Output | `.planning/design/_spike/tui/spike.png` |
| Golden SHA-256 (post metadata-strip) | `32c8b8992c84e59e188460c9ee8bb0d9059c9f10a6355057aed63181ebc12c64` |

Verified reproducible: `TestCaptureTUI` was run 3 times in a row this session,
producing the identical SHA-256 every time (byte-identical PNGs).

## 3. HTML golden (Task 3 — `make screenshot-html`)

| Field | Value |
|-------|-------|
| Entry point | `go test -tags screenshot -run TestCaptureHTML ./internal/screenshot/...` |
| Fixture | `.planning/design/_spike/fixture.html` — a trivial local page, NOT product UI (Phase 2 replaces it with the real design-approved HTML mockups) |
| Viewport | 1280x800, device scale factor 1 |
| Color scheme | `prefers-color-scheme: light` (pinned via `Emulation.setEmulatedMedia`) |
| Chromium revision | `1321438` (see § 1 above) |
| Navigation timeout | 60s (`context.WithTimeout`, bounds launch + navigate + wait + screenshot) |
| Output | `.planning/design/_spike/html/spike.png` |
| Golden SHA-256 (post metadata-strip) | `74f9bebb57c67ba2ee12493ca3fd2b230fd6c257ad0284ebb9eccfb66b570645` |

Verified reproducible: `TestCaptureHTML` was run 3 times in a row this session
(one cold run that provisioned the pinned Chromium revision from scratch, two warm
re-runs against the cached binary), producing the identical SHA-256 every time.

Offline/failure path verified: `TestCaptureHTML_OfflineFailurePath` asserts that an
empty cache directory with provisioning disabled (`AllowDownload: false`) returns a
clear, actionable error naming both the pinned revision and the cache path, and
never attempts a network download or a different-browser fallback.
