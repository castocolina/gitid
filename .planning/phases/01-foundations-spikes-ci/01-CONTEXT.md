# Phase 1: Foundations, Spikes & CI - Context

**Gathered:** 2026-07-02
**Status:** Ready for planning (recommended defaults pending user confirmation — see ⚠ note)

> ⚠ **Decision provenance.** The user selected all four gray areas to discuss, then
> stepped away mid-discussion. Per harness guidance I proceeded with best judgment and
> captured **recommended defaults** grounded in `recipes/`, the existing Go substrate,
> and the Charm/Go ecosystem. Every non-obvious choice below is tagged
> **◆ Claude's discretion (recommended)** — the user should skim these and override any
> before `/gsd-plan-phase 1`. Locked facts (from REQUIREMENTS/ROADMAP/recipes) are not tagged.

<domain>
## Phase Boundary

Deliver every **non-UI** capability, tool, and CI gate that later phases depend on —
test-proven, with **no product UI**. Five workstreams:

1. **Screenshot tooling** (TOOL-05, DLV-03) — repeatable capture of TUI + HTML screens to versioned PNG references.
2. **Multi-algorithm keygen + local-capability probing** (KEY-01/02/03/04, PLAT-01/02) — real ed25519 + rsa-4096 keygen, a top-5 catalog with per-OS availability, driven by a local `ssh-keygen`/`ssh` probe, surfaced by a debug/list command.
3. **Dual SSH-config storage** (STORE-01/02/03/04) — in-file managed blocks OR a gitid-owned Include'd file; adopt an existing external Include'd file; reversible backed-up migration between the two; proven by round-trip + real `ssh -G`.
4. **Identity state-taxonomy core** (MGR-02, DLV-07) — the 8-state classification computed by the UI-free TDD core from parsed managed blocks, no sidecar DB.
5. **Cross-OS CI** (BUILD-01/02/04, TOOL-01..04) — GitHub Actions building darwin/amd64, darwin/arm64, linux/amd64 and running `make test` (race) + `make lint` (golangci-lint + gosec) + `make test-e2e` green on macOS and Linux, reproducible from a fresh clone via `make setup-env`.

**Explicitly NOT in this phase:** any product UI (Phase 2 is the design checkpoint),
visual-regression *diffing* logic (DLV-04, Phase 3), release artifacts/tagging
(BUILD-03, Phase 10), the create/git/manager surfaces (Phases 3+). KEY-01 catalog
final **ordering/copy** is deferred to the design phase (Phase 2) per REQUIREMENTS.

</domain>

<decisions>
## Implementation Decisions

### 1. Screenshot tooling (TOOL-05, DLV-03)
- **D-01 — TUI capture = View()-dump → PNG.** ◆ Claude's discretion (recommended).
  Capture the Bubble Tea model's `View()` string deterministically at a fixed size
  (teatest-style, **no real PTY**), write it as a versioned `.txt` golden, then render
  that ANSI to PNG. **Rationale:** deterministic + CI-friendly + no flaky terminal
  timing; matches the TDD ethos. Real-PTY driving is reserved for DLV-06 e2e (Phase 3),
  a separate concern. *Rejected:* real-PTY-and-snapshot (flaky, slow, headless-CI hard);
  text-golden-only (fails DLV-03's PNG requirement).
- **D-02 — ANSI→PNG renderer = `charmbracelet/freeze`.** ◆ Claude's discretion (recommended).
  Same ecosystem as Bubble Tea/Lipgloss; purpose-built to render terminal output to
  PNG/SVG. Pin it as a dev tool installed by `make setup-env` (not a runtime dep of the
  gitid binary). *Alternative if freeze proves unfit in the spike:* `aha` (ANSI→HTML) +
  headless-chrome screenshot — reuses the HTML path below.
- **D-03 — HTML capture = scripted headless Chromium via a `make` target**, callable by
  the autonomous loop (NOT the Playwright **MCP**, which is agent-only and can't run
  inside a `make`/CI step). ◆ Claude's discretion (recommended). The mockups (React/`mui`)
  don't exist until Phase 2 — Phase 1 builds the **tooling** and proves it against a
  trivial fixture HTML page. *Note:* the Playwright MCP remains available for
  interactive agent use during Phase 2; the make target is what the loop/CI calls.
- **D-04 — Artifact layout.** PNGs (and TUI `.txt` goldens) under
  `.planning/design/<surface>/{html,tui}/*.png`, versioned in git. Fixed capture
  geometry (e.g. 100×30 cols/rows for TUI) recorded so later diffs are apples-to-apples.

### 2. Keygen + probing scope (KEY-01/02/03, PLAT-01/02)
- **D-05 — Real generators in Phase 1 = ed25519 (default) + rsa-4096 ONLY** (locked by
  KEY-02). The architecture is an **algorithm registry/interface** so `ecdsa-p256` and
  the `-sk` hardware variants slot in later **without redesign** — but they are not
  generated in this phase.
- **D-06 — Catalog carries all 5 entries** (`ed25519`, `ed25519-sk`, `rsa-4096`,
  `ecdsa-p256`, `ecdsa-sk`) with security + per-OS availability/variant metadata;
  the 3 without generators are marked "not-yet-implemented / probe-gated." Final
  **ordering + copy is deferred to Phase 2 design** (per REQUIREMENTS "Still Open").
- **D-07 — Probe depth (PLAT-01).** Probe: `ssh-keygen -Q key` (supported key types),
  `ssh -V` (version + LibreSSL-vs-OpenSSL flavor), presence of `libfido2`/`ssh-sk-helper`
  (for `-sk`), a running `ssh-agent`, and macOS keychain support
  (`ssh-add --apple-use-keychain` / `UseKeychain`). ◆ Claude's discretion on the exact
  probe set — this is the recommended floor; deeper checks can be added if the spike
  surfaces a need. The probe is behind an **injectable seam** (mockable in tests) to
  avoid the recurring injected-seam blindspot.
- **D-08 — Surface = a debug/list command** (e.g. `gitid keygen catalog` / a `debug caps`
  subcommand) that prints the catalog + resolved local availability; proven by tests.
  ◆ Claude's discretion on the exact command name. This same debug surface hosts the
  state-taxonomy readout (D-11).

### 3. Include'd SSH layout (STORE-01/02/03)
- **D-09 — Include layout = ONE gitid-owned file** `~/.ssh/config.d/gitid.config`,
  pulled in by a single `Include ~/.ssh/config.d/*.config` line placed **near the TOP**
  of `~/.ssh/config` (first-match-wins, verified with real `ssh -G`). ◆ Claude's
  discretion (recommended). **Rationale:** keeps parity with in-file mode — the same
  per-identity sentinel-block renderer just targets a different file; the recipe models
  identity *blocks*, not separate files. The `*.config` glob leaves room for per-identity
  files later without changing the Include line. *Rejected for now:* per-identity files
  (filesystem sprawl + ordering complexity, no parity payoff).
- **D-10 — Adopt + migrate.**
  - *Adopt (STORE-02):* detect an existing `Include` directive already in
    `~/.ssh/config`; if it targets a dir/file where gitid's blocks belong, adopt that
    path instead of creating `config.d`. Detection scans for `Include` lines + gitid
    sentinels (reuse `internal/adopter`).
  - *Migrate (STORE-03):* reversible move of managed blocks between in-file and Include'd
    layouts, each direction with timestamped backup + idempotent whole-block rewrite,
    proven by round-trip + real `ssh -G`. Include-line placement uses the existing
    `filewriter` block-**prepend** capability. Include paths MUST be absolute or
    `~/.ssh`-relative (verified: relative paths silently fail).

### 4. Identity state-taxonomy core (MGR-02, DLV-07)
- **D-11 — States are LOCKED by MGR-02** (8: complete / incomplete / git-only /
  key-unused / key-used-ssh-only / key-used-both / key-missing / fragment-path-missing).
  Computed by the UI-free TDD core from parsed managed blocks, **no sidecar DB**.
  Phase-1 surface = the same debug/list command as D-08 (prints each identity's state).
  No UI. Proven by table-driven tests over fixture configs.

### 5. CI matrix + gate depth (BUILD-01/02/04)
- **D-12 — Runners = 3 native:** `ubuntu-latest` (linux/amd64), `macos-13` (Intel,
  darwin/amd64), `macos-14` (Apple Silicon, darwin/arm64). ◆ Claude's discretion
  (recommended).
- **D-13 — Gate depth = FULL native gates on all three runners:** `make test` (`-race`)
  + `make lint` (golangci-lint + gosec) + `make test-e2e`. ◆ Claude's discretion
  (recommended). **Rationale:** BUILD-02 exists specifically to catch PLAT-02/03
  divergences (macOS Keychain vs Linux ssh-agent; LibreSSL vs OpenSSL `ssh-keygen`;
  clipboard); Intel **and** ARM macOS both matter. *Cost lever if PR minutes bite:*
  keep test+lint on every PR across all 3, but gate `test-e2e` to `push`/`main` only —
  flagged for the user, default is full-on-PR.
- **D-14 — Build matrix (BUILD-01):** cross-compile all targets reproducibly via `make
  build` (GOOS/GOARCH); darwin/arm64 additionally verified natively on macos-14. Add a
  **build-only** (ungated) `linux/arm64` cross-compile "if cheap." Release/tag publishing
  (BUILD-03) is Phase 10, out of scope here.
- **D-15 — Bootstrap (BUILD-04):** CI verifies `make setup-env` reproduces the toolchain
  (golangci-lint, gosec, pre-commit, hooks) from a fresh clone on both macOS and Linux.

### Claude's Discretion
Tagged inline as **◆** on D-01, D-02, D-03, D-07 (probe set), D-08 (command name),
D-09, D-12, D-13. These are recommended defaults the user should confirm or override
before planning. All other decisions are locked by REQUIREMENTS/ROADMAP/recipes.

### Planning-granularity guidance (not a scope decision)
Phase 1 spans 21 requirements across 5 independent workstreams. Recommend the planner
produce roughly **one plan per workstream** (screenshot tooling / keygen+probe / dual
storage / state taxonomy / CI), with CI last so it gates the others. This is the
planner's call — noted here, not locked. The handoff's "should Phase 1 be split?" is a
ROADMAP concern, not a discuss-phase decision; plan granularity handles the density.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### North Star — canonical config end state
- `recipes/README.md` — what the recipes establish (alias per identity, Port 443, `IdentitiesOnly yes`, `includeIf hasconfig:`/`gitdir:`, `insteadOf`); structure not key type.
- `recipes/ssh-config.recipe` — canonical `~/.ssh/config` shape (Host/Hostname/Port/User/IdentityFile/IdentitiesOnly).
- `recipes/gitconfig.recipe` — canonical `~/.gitconfig` shape (`includeIf`, per-identity fragment, `insteadOf`).

### Goal / spec files
- `.planning/REQUIREMENTS.md` §B TOOL, §C KEY, §F STORE, §H MGR-02, §O PLAT, §P BUILD, §A DLV-03/07 — the Phase-1 requirement set (authoritative).
- `.planning/ROADMAP.md` §"Phase 1" — goal + 5 success criteria (authoritative).
- `docs/prds/gitid-tui-redesign-v1.0-prd.md` — design rationale + delivery-method backbone.

### Verified constraints
- SSH `Include`: absolute or `~/.ssh`-relative paths only; relative paths silently fail; first-match-wins ⇒ Include near TOP (verified live, OpenSSH 9.7 `ssh -G`).

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets (substrate to refactor, not a behavior contract)
- `internal/keygen/` (keygen.go, derive.go, signers.go) — home for the algorithm registry + multi-algo generators (ed25519/rsa-4096) and `allowed_signers` derivation.
- `internal/platform/` (platform.go, install.go) — home for PLAT-01 capability probing + per-OS install hints (brew/apt/dnf/pacman) and macOS-vs-Linux variant handling.
- `internal/sshconfig/` (parser/reader/renderer/writer + marker_roundtrip_test, coexistence_test, provisional) — parse/render round-trip for both in-file and Include'd layouts.
- `internal/filewriter/` (block.go, block_prepend_test.go, provisional.go) — the safe-write chokepoint: timestamped backup + idempotent sentinel-block rewrite + atomic write; **block-prepend** already supports Include-near-top.
- `internal/adopter/` — STORE-02 adopt-existing detection substrate.
- `internal/identity/` (loader, modes, validate) — reconstruct identities from managed blocks; home for the MGR-02 state taxonomy.
- `internal/doctor/` — deps/perms/coherence families; capability-hint consumer.
- `Makefile` — setup-env/fmt/lint/test/build/test-e2e already exist; CI calls these targets (BUILD-01/02/04). Add `screenshot-tui` / `screenshot-html` targets (TOOL-05).
- `cmd/gitid/` (Cobra) — home for the new `keygen catalog` / debug-caps command (D-08).

### Established Patterns
- Safe-write invariant: backup + idempotent whole-block rewrite + atomic temp→rename→chmod + confirm; content outside managed blocks preserved verbatim (STORE-04, locked).
- Injectable seams: probes/generators must be mockable in tests (recurring "injected-seam wiring blindspot" — real `build*Deps()` closures must be exercised, e.g. via the debug command's real wiring, not only stubs).
- TDD + round-trip stability: parse→render→parse must be stable (marker_roundtrip_test pattern).

### Integration Points
- New `.github/workflows/ci.yml` (none exists yet) wiring the 3-runner matrix to `make` targets.
- Screenshot make targets write into `.planning/design/<surface>/{html,tui}/` (versioned).
- Algorithm registry ↔ platform probe ↔ debug/list command ↔ (later) Phase-3 create flow.

</code_context>

<specifics>
## Specific Ideas

- Use `charmbracelet/freeze` for ANSI→PNG (same ecosystem as the TUI stack) — recommended, spike-validated in Phase 1.
- Single Include file `~/.ssh/config.d/gitid.config` via `Include ~/.ssh/config.d/*.config` near the top — mirrors the in-file block model.
- Reuse the algorithm registry seam so `-sk`/ecdsa are additive, never a redesign.

</specifics>

<deferred>
## Deferred Ideas

- **Real-PTY visual capture** for the highest-fidelity TUI screenshots — deferred; DLV-06 e2e (Phase 3) already drives the real binary via PTY. Revisit only if View()-dump PNGs prove insufficient for the visual-regression gate.
- **Per-identity Include files** (`config.d/<identity>.config`) — the `*.config` glob leaves room; not built now (single-file parity chosen).
- **ecdsa-p256 / ed25519-sk / ecdsa-sk real generators** — catalog lists them; generators are additive in a later phase (KEY architecture leaves room).
- **`linux/arm64` gated CI** (native runner) — Phase 1 does build-only cross-compile; native gating deferred (BUILD-01 "if cheap").
- **KEY-01 final catalog ordering + copy** — Phase 2 (design phase), per REQUIREMENTS.
- **Visual-regression diff engine** (pixel/structural compare) — DLV-04, Phase 3.

</deferred>

---

*Phase: 1-Foundations, Spikes & CI*
*Context gathered: 2026-07-02*
