# Phase 10: Linux Validation + Release Pipeline - Context

**Gathered:** 2026-07-08
**Status:** Ready for planning

<domain>
## Phase Boundary

The whole app is validated end-to-end on Fedora-family Linux — a `fedora:latest`
container CI job plus one documented manual UAT on the user's real **Bazzite**
machine (USER CONSTRAINT: Fedora/Bazzite coverage is required, not Ubuntu-only) —
with portability gaps fixed or logged (PLAT-03). On a version tag, CI publishes
versioned, checksummed, provenance-attested binaries to GitHub Releases, with a
build-stamped `gitid --version`, a hardened install script, and a Homebrew tap
(BUILD-03). Ends with a README refresh via the README-crafting skill.

</domain>

<decisions>
## Implementation Decisions

### Linux validation (PLAT-03)
- **D-01 (Vehicle):** Two-part validation. (i) A `fedora:latest` container job
  in CI (on the ubuntu-latest runner) runs the FULL automated suite — `make
  test` (-race), `make lint`, `make test-e2e` (PTY e2e works in containers) —
  after a `dnf install` step (fedora image lacks git/openssh/make by default).
  (ii) One documented manual UAT on the user's real Bazzite machine **per
  release**, covering exactly the container-invisible residue. Bazzite's
  userland IS Fedora's (image delivery + gaming additions are the delta), so
  this pair is covering.
- **D-02 (Cadence):** The fedora container job runs on **push-to-main and
  release tags only** (reuses the Phase 1 D-13 cost-tier lever). Ubuntu is
  already triple-covered per-PR. No debian/openSUSE/Arch containers, no VM
  jobs (GitHub has no Fedora runners; VM actions are flaky third-party trust).
- **D-03 (Risk checklist):** REAL risks, actively tested: `ssh -V`
  distro-suffix parsing (bitten before), `wl-clipboard` presence on Bazzite
  KDE/GNOME images, `ssh-add` graceful degradation when NO agent exists
  (Fedora KDE ships none by default; GNOME uses the `gcr-ssh-agent` systemd
  user socket at `$XDG_RUNTIME_DIR/gcr/ssh`). VERIFY-ONCE-AND-LOG: SELinux
  spot-check (`ls -Z ~/.ssh` after a write — the documented `ssh_home_t`
  failure mode is server-side `authorized_keys`, not user-context writes),
  `/home → /var/home` symlink includeIf resolution, real-terminal TUI
  rendering (Ptyxis/Konsole, Wayland). Theoretical (container covers): git
  version drift, `~/.gitconfig.d` fragment includes.
- **D-04 (Limitations ledger):** New root **`PLATFORM-NOTES.md`** with
  per-distro rows (Distro | Aspect | Status | Workaround | Issue), linked from
  README. Every Bazzite UAT finding lands as a row (fixed → issue link,
  accepted → workaround). The UAT evidence log itself lives in
  `.planning/phases/10-*/`.

### Release pipeline (BUILD-03)
- **D-05 (Tooling — USER CHOICE):** **goreleaser, wrapped in make targets.**
  make remains the single entry point: `make release` / `make release-snapshot`
  invoke a pinned goreleaser; `.goreleaser.yaml` is the release build
  definition; existing targets are extended/redefined so the Makefile and
  goreleaser share ONE version/ldflags definition (no drift between two build
  descriptions). Dev builds keep `make build`. Local dry-run parity via
  `goreleaser --snapshot` behind make. (User initially asked "why not GitHub
  workflows?" — clarified: all options run inside release.yml; this choice is
  the tooling within it.)
- **D-06 (Trigger + permissions + safety):** Separate `.github/workflows/release.yml`
  on `push: tags: ['v*']`; single ubuntu job; job-scoped
  `permissions: contents: write` (+ `id-token: write`, `attestations: write`
  for D-08). Default GITHUB_TOKEN — the ONLY secret in the pipeline is the
  tap-repo PAT (D-13). The job re-runs `make test` + `make lint` before
  publishing, so a tag on an untested commit cannot ship. ci.yml stays
  `contents: read`. SHA-pinning discipline applies to every action used.
- **D-07 (Artifact format):** tar.gz archives per platform —
  `gitid_<version>_<os>_<arch>.tar.gz` containing binary + LICENSE + README —
  plus ONE `gitid_<version>_checksums.txt` over the archives (goreleaser
  default naming; what gh/lazygit/fzf ship; what the tap formula expects).
  Naming is stable forever once published.
- **D-08 (Integrity + notes):** `actions/attest-build-provenance` (first-party
  keyless SLSA provenance; requires the repo to be public — verify at
  execution). Release created with `--verify-tag` + auto-generated notes and a
  curated header line (auto-notes are PR-based; this repo merges fast-forward,
  so the header carries the story — goreleaser's changelog integration may
  render the body; planner detail). No cosign/GPG; no draft step — the tag
  push is the human gate.

### Version stamping (BUILD-03 criterion 3)
- **D-09 (Hybrid resolve):** New **`internal/version`** package:
  ldflags-injectable vars + `Resolve()` that prefers ldflags and falls back to
  `debug.ReadBuildInfo()` (`Main.Version`, `vcs.revision`, `vcs.time`,
  `vcs.modified`) so `--version` is truthful on ALL four build paths
  (empirically verified on Go 1.26: release tag exact; `go install @v1.0.0`
  reports the module version; dev builds report pseudo-version, `+dirty` when
  modified). Importable by the TUI (footer/help). `const version` at
  `cmd/gitid/main.go:15` becomes the package var. Table-test `Resolve()` per
  build-path case.
- **D-10 (Source of truth):** Git tag via
  `VERSION ?= $(shell git describe --tags --match "v*" --always --dirty)` in
  the Makefile (the `--match "v*"` filter excludes `poc-0.0.1`/`backup/*`
  tags). Local `make build`/`make install` also stamp (traceable
  `v1.0.0-3-gabc123-dirty` binaries). goreleaser injects the same var path.
  **Guardrails (verified):** `bin/` must be gitignored — untracked build
  output flips `vcs.modified` → `+dirty` on release stamps; release checkout
  needs tags available (`fetch-depth: 0`).
- **D-11 (Output):** `gitid --version` prints
  `gitid version 1.0.0 (abc1234, 2026-07-08, darwin/arm64)` via
  `SetVersionTemplate`, PLUS a `gitid version` subcommand with `--json`
  (Phase 5 "--json on reads" contract; both surfaces share one struct/render
  path).

### Artifact set & install story
- **D-12 (Targets):** Four: darwin amd64/arm64, linux amd64/arm64 — linux-arm64
  is already built at zero cost; ship it marked **best-effort** (built, never
  CI-gated) in the release notes. `CGO_ENABLED=0 -trimpath -ldflags "-s -w
  -X …version=$(VERSION)"` made EXPLICIT in the build definition: today's
  static linking is accidental (CI's native linux-amd64 build defaults
  CGO_ENABLED=1; one future `net`/`os/user` import would silently go
  glibc-dynamic). No darwin universal, no riscv64/musl variants.
- **D-13 (Homebrew tap — v1.0):** `castocolina/homebrew-tap` repo; goreleaser
  `brews:` stanza auto-updates the formula each release. One formula serves
  macOS + all Linux including Bazzite (Homebrew is preinstalled on Universal
  Blue images and is the docs-recommended CLI channel there). Cost: one new
  repo + ONE PAT secret in release.yml scoped to the tap repo — the
  pipeline's only secret. REJECTED: COPR/RPM (useless on Bazzite without
  discouraged layering; lazygit's COPR bit-rot precedent), Flatpak (sandbox
  blocks `~/.ssh` mutation and host ssh/git shelling).
- **D-14 (Install script — USER LOCKED, hardening approved):** v1.0 ships a
  curl|bash-able `install.sh`: auto-detects OS/arch; downloads the release
  archive AND `checksums.txt`; **verifies SHA-256 BEFORE extracting**;
  installs to `~/.local/bin` (immutable-distro-safe, identical on
  Bazzite/Fedora/Ubuntu/macOS); supports a `GITID_VERSION` pin and custom
  install dir. README leads with "download, inspect, then run" phrasing and
  shows the manual path first (security optics for a tool whose pitch is safe
  `~/.ssh` mutation). The script itself is CI-tested on ubuntu + the fedora
  container + macos (curl vs wget, GNU vs BSD tool differences).
- **D-15 (Checksum UX):** Single `checksums.txt`; document the per-OS verify
  commands verbatim in README + release notes:
  Linux `sha256sum --ignore-missing -c gitid_<v>_checksums.txt`;
  macOS `shasum -a 256 --ignore-missing -c gitid_<v>_checksums.txt`.
- **D-16 (README refresh — USER ADDED):** Phase 10 ends with a README.md
  update **executed via the user's README-crafting skill** (the executor must
  invoke that skill, not hand-write). Must include: install paths (script,
  brew tap, manual `~/.local/bin`, `go install` with its version-fallback
  caveat), the D-15 verify commands, and the PLATFORM-NOTES.md link.

### Claude's Discretion
- fedora container job details (dnf package list, setup-go-in-container
  handling, whether e2e needs TERM/agent env shims already used locally).
- Bazzite UAT checklist document format and where results are logged.
- `.goreleaser.yaml` internals (archive contents, changelog config, snapshot
  naming) as long as make wraps it and version/ldflags are defined once.
- install.sh implementation details (POSIX sh vs bash, wget fallback, arch
  aliases) within the D-14 hardening contract.
- Release-notes curated-header wording; best-effort arm64 note copy.
- `gitid version --json` field names (align with internal/version struct).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements & roadmap
- `.planning/ROADMAP.md` Phase 10 — goal + 3 success criteria
- `.planning/REQUIREMENTS.md` §O (PLAT-03), §P (BUILD-03; BUILD-01/02/04 built)

### CI & build substrate (this phase extends)
- `.github/workflows/ci.yml` — the LOCKED CI discipline to preserve: SHA-pinned
  actions (checkout `9c091bb2…` v7.0.0, setup-go `924ae3a1…` v6.5.0 — reuse
  these pins), least-privilege permissions, cost-tier lever (D-13 of Phase 1),
  make-only invocation rule
- `Makefile` — build/build-cross/test/test-e2e/lint targets; build-cross is
  the 4-target cross-compile D-12 formalizes; new release targets wrap goreleaser
- `cmd/gitid/main.go:15` — `const version = "0.0.0-dev"` → `internal/version` (D-09)
- `go.mod` — `go 1.26` (buildvcs stamping active)

### Platform substrate (validation targets)
- `recipes/README.md` + `recipes/` — the config surface the Fedora/Bazzite
  e2e flow must produce end-to-end
- `internal/deps/deps.go` — tool probing (ssh/git/clipboard) exercised by the
  Fedora checklist
- Phase 1 CI portability lessons — local repro recipe
  `TERM=dumb SSH_AUTH_SOCK= go test -race ./...` (ssh -V suffix, TERM glyphs,
  headless doctor, grandchild-pipe hang)

### Prior phase contracts that bind here
- `.planning/phases/05-identity-manager/05-CONTEXT.md` — CLI grammar the
  `gitid version` subcommand + `--json` must follow

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `make build-cross` already produces the exact 4-target matrix — D-12 adds
  explicit CGO_ENABLED=0/-trimpath/ldflags rather than new build logic.
- ci.yml's SHA pins, cost-tier pattern, and container-job support carry
  directly into the fedora job and release.yml.
- PTY e2e suite (`make test-e2e`) self-allocates PTYs — runs unmodified inside
  the fedora container.
- Cobra `Version` field is already wired — only the value source changes.

### Established Patterns
- CI runs ONLY make targets (single source of truth) — goreleaser must be
  invoked through make (D-05), never raw in YAML.
- SHA-pin every action; least-privilege permissions per job; secrets appear
  only where unavoidable (the tap PAT is the single exception, D-13).
- UAT-with-evidence tradition — the Bazzite UAT follows the existing phase-UAT
  format and feeds PLATFORM-NOTES.md.

### Integration Points
- `internal/version.Resolve()` → Cobra root cmd (`--version`), new `version`
  subcommand, and the TUI footer/help.
- release.yml → make release → goreleaser → GitHub Release + tap formula push.
- fedora container job slots into ci.yml beside the existing cost-tiered
  macos-15-intel e2e conditional.

</code_context>

<specifics>
## Specific Ideas

- USER: Bazzite/Fedora validation required, not Ubuntu-only ("Instead only
  Ubuntu I want Bazzite/Fedora release also") — drove D-01..D-03 and the
  brew-first install story.
- USER: "I want a curl bash distro shell script" — D-14 locked; research
  hardening adopted (verify-before-extract, inspect-first phrasing).
- USER: goreleaser with "extend or redefine the make targets" — D-05's
  make-wrapped shape.
- USER: end the phase by running the README-crafting skill to update README.md,
  including the distro SHA validation commands for Linux/macOS (D-15/D-16).
- Empirical anchors: Go 1.26 buildvcs behavior verified per build path;
  cross-built linux binary verified statically linked; gh CLI + GITHUB_TOKEN
  release auth verified; Bazzite docs verified recommending Homebrew
  (preinstalled) for CLIs.

</specifics>

<deferred>
## Deferred Ideas

- openSUSE/Arch/debian container jobs — only on demonstrated distro-bug demand.
- Fedora VM job — the manual Bazzite UAT covers the VM-only residue cheaply.
- cosign/GPG signing — attestations cover provenance without key management.
- Commit-log-grouped changelog script — when external users track changes.
- `.rpm` via goreleaser nfpms — free byproduct only if demand appears; never
  the Fedora answer for an immutable-first audience.
- darwin universal (lipo) binary; linux-riscv64.
- Per-artifact `.sha256` files — add only if download-friction reports appear.

</deferred>

---

*Phase: 10-linux-validation-release-pipeline*
*Context gathered: 2026-07-08*
