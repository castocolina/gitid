# Phase 10: Linux Validation + Release Pipeline - Research

**Researched:** 2026-09-04
**Domain:** GitHub Actions CI (container jobs), goreleaser release engineering, Go build-info version stamping, POSIX install scripts, Homebrew tap distribution
**Confidence:** HIGH (every load-bearing claim below was independently exercised in this session — real `docker run`/`dnf install` against `fedora:latest`, real `go build`/`go version -m` against this repo, real `git ls-remote` against the upstream action repos — not just cited from docs)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Linux validation (PLAT-03)**
- **D-01 (Vehicle):** Two-part validation. (i) A `fedora:latest` container job in CI (on the ubuntu-latest runner) runs the FULL automated suite — `make test` (-race), `make lint`, `make test-e2e` (PTY e2e works in containers) — after a `dnf install` step (fedora image lacks git/openssh/make by default). (ii) One documented manual UAT on the user's real Bazzite machine **per release**, covering exactly the container-invisible residue. Bazzite's userland IS Fedora's (image delivery + gaming additions are the delta), so this pair is covering.
- **D-02 (Cadence):** The fedora container job runs on **push-to-main and release tags only** (reuses the Phase 1 D-13 cost-tier lever). Ubuntu is already triple-covered per-PR. No debian/openSUSE/Arch containers, no VM jobs.
- **D-03 (Risk checklist):** REAL risks, actively tested: `ssh -V` distro-suffix parsing, `wl-clipboard` presence on Bazzite KDE/GNOME images, `ssh-add` graceful degradation when NO agent exists. VERIFY-ONCE-AND-LOG: SELinux spot-check, `/home → /var/home` symlink includeIf resolution, real-terminal TUI rendering. Theoretical (container covers): git version drift, `~/.gitconfig.d` fragment includes.
- **D-04 (Limitations ledger):** New root **`PLATFORM-NOTES.md`** with per-distro rows (Distro | Aspect | Status | Workaround | Issue), linked from README. Every Bazzite UAT finding lands as a row. The UAT evidence log itself lives in `.planning/phases/10-*/`.

**Release pipeline (BUILD-03)**
- **D-05 (Tooling — USER CHOICE):** **goreleaser, wrapped in make targets.** make remains the single entry point: `make release` / `make release-snapshot` invoke a pinned goreleaser; `.goreleaser.yaml` is the release build definition; existing targets are extended/redefined so the Makefile and goreleaser share ONE version/ldflags definition. Dev builds keep `make build`. Local dry-run parity via `goreleaser --snapshot` behind make.
- **D-06 (Trigger + permissions + safety):** Separate `.github/workflows/release.yml` on `push: tags: ['v*']`; single ubuntu job; job-scoped `permissions: contents: write` (+ `id-token: write`, `attestations: write` for D-08). Default GITHUB_TOKEN — the ONLY secret in the pipeline is the tap-repo PAT (D-13). The job re-runs `make test` + `make lint` before publishing. ci.yml stays `contents: read`. SHA-pinning discipline applies to every action used.
- **D-07 (Artifact format):** tar.gz archives per platform — `gitid_<version>_<os>_<arch>.tar.gz` containing binary + LICENSE + README — plus ONE `gitid_<version>_checksums.txt` over the archives. Naming is stable forever once published.
- **D-08 (Integrity + notes):** `actions/attest-build-provenance` (first-party keyless SLSA provenance; requires the repo to be public). Release created with `--verify-tag` + auto-generated notes and a curated header line. No cosign/GPG; no draft step — the tag push is the human gate.

**Version stamping (BUILD-03 criterion 3)**
- **D-09 (Hybrid resolve):** New **`internal/version`** package: ldflags-injectable vars + `Resolve()` that prefers ldflags and falls back to `debug.ReadBuildInfo()` (`Main.Version`, `vcs.revision`, `vcs.time`, `vcs.modified`) so `--version` is truthful on ALL four build paths. Importable by the TUI (footer/help). `const version` at `cmd/gitid/main.go:15` becomes the package var. Table-test `Resolve()` per build-path case.
- **D-10 (Source of truth):** Git tag via `VERSION ?= $(shell git describe --tags --match "v*" --always --dirty)` in the Makefile. Local `make build`/`make install` also stamp. goreleaser injects the same var path. **Guardrails (verified):** `bin/` must be gitignored — untracked build output flips `vcs.modified` → `+dirty` on release stamps; release checkout needs tags available (`fetch-depth: 0`).
- **D-11 (Output):** `gitid --version` prints `gitid version 1.0.0 (abc1234, 2026-07-08, darwin/arm64)` via `SetVersionTemplate`, PLUS a `gitid version` subcommand with `--json` (Phase 5 "--json on reads" contract; both surfaces share one struct/render path).

**Artifact set & install story**
- **D-12 (Targets):** Four: darwin amd64/arm64, linux amd64/arm64 — linux-arm64 is best-effort (built, never CI-gated). `CGO_ENABLED=0 -trimpath -ldflags "-s -w -X …version=$(VERSION)"` made EXPLICIT. No darwin universal, no riscv64/musl variants.
- **D-13 (Homebrew tap — v1.0):** `castocolina/homebrew-tap` repo; goreleaser `brews:` stanza auto-updates the formula each release. One formula serves macOS + all Linux including Bazzite. Cost: one new repo + ONE PAT secret in release.yml scoped to the tap repo. REJECTED: COPR/RPM, Flatpak.
- **D-14 (Install script — USER LOCKED, hardening approved):** v1.0 ships a curl|bash-able `install.sh`: auto-detects OS/arch; downloads the release archive AND `checksums.txt`; **verifies SHA-256 BEFORE extracting**; installs to `~/.local/bin`; supports a `GITID_VERSION` pin and custom install dir. README leads with "download, inspect, then run" phrasing. The script itself is CI-tested on ubuntu + the fedora container + macos (curl vs wget, GNU vs BSD tool differences).
- **D-15 (Checksum UX):** Single `checksums.txt`; document the per-OS verify commands verbatim in README + release notes: Linux `sha256sum --ignore-missing -c gitid_<v>_checksums.txt`; macOS `shasum -a 256 --ignore-missing -c gitid_<v>_checksums.txt`.
- **D-16 (README refresh — USER ADDED):** Phase 10 ends with a README.md update **executed via the user's README-crafting skill**. Must include: install paths (script, brew tap, manual `~/.local/bin`, `go install` with its version-fallback caveat), the D-15 verify commands, and the PLATFORM-NOTES.md link.

### Claude's Discretion
- fedora container job details (dnf package list, setup-go-in-container handling, whether e2e needs TERM/agent env shims already used locally).
- Bazzite UAT checklist document format and where results are logged.
- `.goreleaser.yaml` internals (archive contents, changelog config, snapshot naming) as long as make wraps it and version/ldflags are defined once.
- install.sh implementation details (POSIX sh vs bash, wget fallback, arch aliases) within the D-14 hardening contract.
- Release-notes curated-header wording; best-effort arm64 note copy.
- `gitid version --json` field names (align with internal/version struct).

### Deferred Ideas (OUT OF SCOPE)
- openSUSE/Arch/debian container jobs — only on demonstrated distro-bug demand.
- Fedora VM job — the manual Bazzite UAT covers the VM-only residue cheaply.
- cosign/GPG signing — attestations cover provenance without key management.
- Commit-log-grouped changelog script — when external users track changes.
- `.rpm` via goreleaser nfpms — free byproduct only if demand appears.
- darwin universal (lipo) binary; linux-riscv64.
- Per-artifact `.sha256` files — add only if download-friction reports appear.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| PLAT-03 | Full flow validated end-to-end on a mainstream Linux distro (Fedora-family), portability gaps fixed/logged | Verified exact `dnf install` package list against the real `fedora:latest` image (git, openssh-clients, make, nodejs); confirmed the container-vs-Bazzite risk split (D-01/D-03) maps cleanly onto what a container CAN and CANNOT exercise (no SELinux enforcement, no real Wayland session, no gcr-ssh-agent) |
| BUILD-03 | On a version tag, CI publishes checksummed, provenance-attested binaries via goreleaser, with a build-stamped `--version`, hardened install.sh, Homebrew tap | Verified goreleaser v2.18.0's exact `archives`/`checksum`/`brews`/`release` config surface, `actions/attest-build-provenance` v4.2.2's exact usage + permission model, and — critically — that this phase REPLACES Phase 9.3's already-"Complete" raw-binary release shape with a materially different tar.gz-archive shape, which invalidates 17 existing e2e test functions and the current `ci.yml`/`install.sh` (see Common Pitfalls #1) |
</phase_requirements>

## Summary

This phase does not need new design decisions — CONTEXT.md's D-01 through D-16 are locked. What it needs is grounded, current implementation detail for two large, interacting changes: (1) standing up a `fedora:latest` **container job** inside the existing `ci.yml`, and (2) **replacing** Phase 9.3's already-shipped, raw-binary release pipeline (`build-cross` + `checksums` + `softprops/action-gh-release`, currently live in `ci.yml`'s `release:` job) with a goreleaser-driven, tar.gz-archive pipeline in a new `release.yml`. That replacement is the single biggest risk in this phase: it changes the published artifact shape from `bin/gitid-<os>-<arch>` (raw binary) to `gitid_<version>_<os>_<arch>.tar.gz` (archive containing binary + LICENSE + README), which is a **breaking change** to `scripts/install.sh` (currently downloads and chmods a raw binary directly) and to all 17 test functions in `e2e/release_e2e_test.go` (currently assert on the raw-binary shape). The planner must treat "migrate install.sh + its e2e suite to the archive format" as first-class work, not a footnote.

Every technical claim below that could be checked directly WAS checked directly in this session: `docker run fedora:latest` (confirmed exact missing packages and the one-line `dnf install` fix), a real `go build` + `go version -m` against this repo (confirmed `vcs.modified`/`vcs.revision`/`vcs.time` stamping behavior and validated the `bin/`-gitignore guardrail empirically, including the FRESH-CLONE negative case), and `git ls-remote --tags` against every third-party GitHub Action referenced (confirmed live, correct SHA pins — not the training-data guesses that produce slopsquat-adjacent staleness).

**Primary recommendation:** Build the fedora job as a `container: image: fedora:latest` step inside the existing `check` job's matrix pattern (not a new workflow), gated by `D-02`'s cost-tier `if:` condition; install `git openssh-clients make nodejs tar gzip` via `dnf install -y --setopt=install_weak_deps=False` as the FIRST step (before `actions/checkout`, which needs Node.js to run at all inside the container). Build `release.yml` around a `make release` target that wraps a **pinned** `goreleaser` binary (`go install github.com/goreleaser/goreleaser/v2@v2.18.0`, mirroring the existing `golangci-lint`/`gosec`/`freeze` pattern in `setup-env`), with `.goreleaser.yaml`'s `builds.ldflags` referencing the SAME `{{.Env.VERSION}}`/`{{.Env.COMMIT}}`/`{{.Env.DATE}}` vars the Makefile already computes — not goreleaser's own internal `git describe`, which does not respect the Makefile's `--match "v*"` tag filter and would silently pick up `poc-0.0.1`/`backup/*` tags unless separately configured via `git.ignore_tags`.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Linux runtime validation (PLAT-03) | CI / Backend (container job) | Manual UAT (Bazzite) | The automated suite (`make test`/`lint`/`test-e2e`) IS the backend's own test tier, run inside a Fedora container substrate rather than a new architectural layer; the manual UAT is a human-in-the-loop tier that exists specifically because SELinux/Wayland/gcr-ssh-agent are properties of a REAL machine a container cannot reproduce |
| Version stamping (`internal/version`) | Backend (Go package) | CLI / TUI (consumers) | `Resolve()` is pure backend logic (ldflags vars + `debug.ReadBuildInfo()`); Cobra's root cmd and the TUI footer are consumers, not owners |
| Release build + packaging | CI / Build tooling (goreleaser via make) | — | Cross-compilation, archiving, checksumming, and GitHub Release creation are all CI-tier build concerns; no application-tier code is involved |
| Homebrew tap distribution | External repo (`castocolina/homebrew-tap`) | CI (goreleaser `brews:` publish step) | The tap repo is a separate GitHub repo goreleaser pushes a formula file into; gitid's own repo only holds the PAT secret and the `.goreleaser.yaml` stanza that targets it |
| Install script (`scripts/install.sh`) | CLI / Distribution (POSIX shell) | GitHub Releases (download source) | Runs entirely client-side on the user's machine; its only coupling to the backend is the checksums.txt format goreleaser produces |
| Provenance attestation | CI (GitHub-native Sigstore) | — | `actions/attest-build-provenance` is a GitHub Actions primitive with no application-tier surface; it operates on already-built artifacts as a CI post-processing step |

## Standard Stack

### Core
| Library / Tool | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/goreleaser/goreleaser/v2` | v2.18.0 `[VERIFIED: git ls-remote --tags https://github.com/goreleaser/goreleaser.git — refs/tags/v2.18.0 → 3fc1d21a1a701a587a3837374cb94e9d8aede5e3]` | Cross-platform Go release build, archiving, checksumming, GitHub Release publishing, Homebrew tap formula push | De-facto standard Go release tool (D-05 user choice); a single `.goreleaser.yaml` replaces the hand-rolled `build-cross`/`checksums`/`action-gh-release` combination already in `ci.yml`, and natively expresses every D-07/D-08/D-13 requirement (archives, checksum manifest, provenance-friendly artifact list, `brews:` stanza) |
| `actions/attest-build-provenance` | v4.2.2 `[VERIFIED: git ls-remote --tags https://github.com/actions/attest-build-provenance.git — refs/tags/v4.2.2 → 4d101475d8b20a2381f78447822ac1eab6504dd8 (same commit as the v4 branch tip)]` | First-party keyless SLSA build provenance attestation | GitHub-native, no key management (fits D-08's "no cosign/GPG" constraint); requires `id-token: write` + `attestations: write` and a public repo — both already satisfiable per D-06's permission grant |
| `runtime/debug.ReadBuildInfo` | stdlib (Go 1.26/1.27) | Fallback version resolution for non-ldflags-stamped builds (`go install`, plain `go build`) | Stdlib, zero new dependency; `[VERIFIED: go build -o /tmp/gitid-probe ./cmd/gitid && go version -m /tmp/gitid-probe]` — real run in this repo confirmed `vcs.revision`, `vcs.time`, `vcs.modified` are populated by a PLAIN `go build` (not just `go install`) on this toolchain, contradicting an older (2022-era) golang/go issue that claimed vcs info was `go install`-only |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `git` `dnf` package `git` | 2.55.0 (Fedora 44 container) `[VERIFIED: docker run fedora:latest — dnf install output, "Installing git-core-0:2.55.0-1.fc44"]` | Provides `git` inside the fedora container job | Fedora job's `dnf install` step |
| `openssh-clients` (dnf package) | ships `ssh`/`ssh-keygen`/`ssh-add` `[VERIFIED: docker run fedora:latest sh -c 'dnf install ... openssh-clients; command -v ssh ssh-keygen ssh-add' — all three resolved]` | SSH tooling gitid shells out to (`internal/deps`) | Fedora job's `dnf install` step; `ssh -V` on this image reported `OpenSSH_10.2p1, OpenSSL 3.5.7 9 Jun 2026` — no distro version suffix (unlike Debian/Ubuntu's `Ubuntu-3ubuntu13`-style suffix), a real, useful data point for D-03's "ssh -V distro-suffix parsing" risk item |
| `nodejs` (dnf package) | v22.23.1 `[VERIFIED: docker run fedora:latest sh -c 'dnf install nodejs; node --version']` | Runtime every Node-based GitHub Action (`actions/checkout`, `actions/setup-go`, etc.) needs to execute AT ALL inside a bare container | MUST be installed BEFORE `actions/checkout` runs — fedora's stock image ships no Node.js (`[VERIFIED: docker run fedora:latest sh -c 'command -v node nodejs'` → both MISSING before install]`) |
| `make` (dnf package) | GNU Make 4.4.1 `[VERIFIED: docker run fedora:latest — same dnf install run]` | Runs the same `make test`/`make lint`/`make test-e2e` targets used everywhere else in this repo | Fedora job's `dnf install` step |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| goreleaser `brews:` stanza (D-13 literal wording) | goreleaser `homebrew_casks:` stanza | `brews` is soft-deprecated since goreleaser v2.10 (planned removal only in an unreleased v3, no date) `[CITED: https://goreleaser.com/blog/goreleaser-v2.10/ + https://goreleaser.com/customization/publish/homebrew_formulas/, cross-checked]`; `homebrew_casks` is the maintainer-recommended replacement and ALSO installs a CLI binary to PATH (not GUI-only). Since D-13 names `brews:` explicitly and it is still functional, use `brews:` as locked — but flag the deprecation path as a documented future-migration note, not a blocker (see Open Questions) |
| A raw `go install github.com/goreleaser/goreleaser/v2@vX.Y.Z` pin in `setup-env`/a release-only Makefile step | `goreleaser/goreleaser-action` (official GitHub Action) | D-05 explicitly forbids invoking goreleaser "raw in YAML" — it must go through make. The official Action is the more common pattern in the wild but is incompatible with this repo's "CI invokes the SAME make targets a human runs locally" discipline (already the header comment in `ci.yml`); pin goreleaser as a versioned Go-installable tool instead, exactly like `golangci-lint`/`gosec`/`freeze` already are |
| `softprops/action-gh-release` (currently used in `ci.yml`'s `release:` job) | goreleaser's own `release:` pipe (creates the GitHub Release itself) | goreleaser natively creates/publishes the GitHub Release, uploads all archives + checksums, and supports `prerelease: auto` (tag-suffix detection) and `header:`/`footer:` templates — functionally superseding the current job's hand-rolled bash `case` statement AND the separate `action-gh-release` step. Migrating to goreleaser's native release creation removes an entire action dependency (see Don't Hand-Roll) |

**Installation:**
```bash
# In setup-env (mirrors the existing golangci-lint/gosec/freeze pattern):
go install github.com/goreleaser/goreleaser/v2@v2.18.0

# Fedora container job's first step (before actions/checkout):
dnf install -y --setopt=install_weak_deps=False git openssh-clients make nodejs tar gzip
```

**Version verification:** Both `goreleaser` and `actions/attest-build-provenance` versions above were confirmed live against their upstream repos this session (`git ls-remote --tags`), not sourced from training data. Re-verify at execution time with the same command if this research goes stale — goreleaser ships frequent minor releases (v2.13→v2.18 across roughly this session's research window).

## Package Legitimacy Audit

**Not applicable in the standard sense.** This phase adds NO new `go.mod` dependencies — `internal/version` uses only `runtime/debug` (stdlib). `goreleaser` is a **dev-tool binary** (like `golangci-lint`/`gosec`/`freeze` already in this Makefile), installed via `go install ...@version` and never imported into the shipped `gitid` binary's dependency graph. The `package-legitimacy` seam (`npm`/`pypi`/`crates` only) does not cover Go, so no automated verdict was obtainable; legitimacy was instead established by:
- `goreleaser/goreleaser` — official, first-party GitHub org, `git ls-remote` confirms an active, current tag cadence (v2.13.2 → v2.18.0 across recent history), matches the project's own `.goreleaser.yaml`-driven docs at `goreleaser.com` `[VERIFIED: git ls-remote + cross-checked against goreleaser.com docs]`
- `actions/attest-build-provenance` — first-party `actions` GitHub org (same org that publishes `actions/checkout`/`actions/setup-go`, already trusted and pinned in this repo's `ci.yml`) `[VERIFIED: git ls-remote]`

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| `goreleaser/goreleaser/v2` | Go install target (not go.mod) | Long-established OSS project, active release cadence confirmed live | N/A (binary tool, not a package registry) | `github.com/goreleaser/goreleaser` (first-party, well-known) | OK (manual verification — not npm/pypi/crates) | Approved |
| `actions/attest-build-provenance` | GitHub Actions | First-party `actions` org | N/A | `github.com/actions/attest-build-provenance` | OK (manual verification) | Approved |

**Packages removed due to [SLOP] verdict:** none.
**Packages flagged as suspicious [SUS]:** none.

### Addendum 2026-09-21 — pin corrected downward

Quick task 260921-t6g. `GORELEASER_VERSION` is now pinned to `v2.17.0` (was
`v2.18.0` above). The 2.18 series declares a `go` directive of 1.27.0 or
newer in its own `go.mod`, and this repo's Makefile deliberately exports a
1.26-series `GOTOOLCHAIN` (`go1.26.4`) — golangci-lint v2.12.2 cannot handle
the Go 1.27 standard library. That mismatch is unbuildable, not merely
undesirable: `go install github.com/goreleaser/goreleaser/v2@v2.18.0` under
`GOTOOLCHAIN=go1.26.4` fails outright with `requires go >= 1.27.0 (running
go 1.26.4)`, so every workflow calling `make setup-env`/`make
setup-env-release` (CI's `check`/`fedora` jobs, `nightly.yml`,
`release.yml`) died at the bootstrap step from 2026-09-05 onward.

`go` directives read live from the Go module proxy
(`proxy.golang.org/github.com/goreleaser/goreleaser/v2/@v/<ver>.mod`) on
2026-09-21:

| goreleaser | requires |
|------------|----------|
| v2.18.2 / v2.18.1 | go 1.27.1 |
| v2.18.0 | go 1.27.0 |
| v2.17.1 | go 1.26.5 |
| **v2.17.0** | **go 1.26.4** |
| v2.16.0 | go 1.26.3 |

`v2.17.0` is the newest release whose own requirement the exported
`GOTOOLCHAIN` satisfies exactly.

The legitimacy verdict for `goreleaser/goreleaser/v2` in the table above is
**unchanged — still Approved**: this is an earlier tag of the same
first-party upstream module already audited, not a new dependency.

`cmd/gitid/goreleaser_pin_test.go` (`TestGoreleaserPinIsBuildableByPinnedToolchain`)
now enforces the toolchain constraint mechanically — a future incompatible
bump of `GORELEASER_VERSION` fails `make test` instead of silently
reddening CI for weeks, as this one did.

## Architecture Patterns

### System Architecture Diagram

```
                         ┌─────────────────────────────┐
                         │   Developer: git push        │
                         │   (branch push OR `git tag   │
                         │    v1.0.0 && git push --tags`)│
                         └───────────────┬──────────────┘
                                         │
                 ┌───────────────────────┼─────────────────────────┐
                 │ push to main / PR     │ push tag v*
                 ▼                        ▼
     ┌──────────────────────┐   ┌─────────────────────────────┐
     │  ci.yml               │   │  release.yml (NEW, D-06)     │
     │  ┌──────────────────┐ │   │  runs-on: ubuntu-latest       │
     │  │ check (matrix):   │ │   │  permissions: contents:write, │
     │  │  ubuntu/macos×2   │ │   │  id-token:write,               │
     │  └──────────────────┘ │   │  attestations:write            │
     │  ┌──────────────────┐ │   │                                 │
     │  │ fedora (NEW, D-01)│ │   │  1. checkout (fetch-depth: 0)  │
     │  │ container:         │ │   │  2. re-run make test + lint    │
     │  │  fedora:latest      │ │   │     (D-06: red gate blocks     │
     │  │  dnf install FIRST  │ │   │      publish)                  │
     │  │  (git,openssh-      │ │   │  3. make release                │
     │  │   clients,make,     │ │   │     └─▶ pinned goreleaser        │
     │  │   nodejs,tar,gzip)  │ │   │          reads .goreleaser.yaml   │
     │  │  → checkout → setup-│ │   │          builds 4 targets          │
     │  │    go → make test/  │ │   │          (CGO_ENABLED=0,-trimpath) │
     │  │    lint/test-e2e    │ │   │          archives → .tar.gz          │
     │  │  cadence: push-main  │ │   │          checksums.txt (sha256)      │
     │  │  + tags only (D-02)  │ │   │          creates GH Release           │
     │  └──────────────────────┘ │   │          (prerelease:auto from tag)   │
     │                             │   │  4. actions/attest-build-provenance   │
     │                             │   │     (subject-path: archives+checksums)│
     │                             │   │  5. goreleaser brews: pipe             │
     │                             │   │     → pushes Formula to                │
     │                             │   │       castocolina/homebrew-tap          │
     │                             │   │       (uses PAT secret, D-13)           │
     │                             │   └────────────────┬────────────────────────┘
     └────────────────────────────┘                     │
                                                          ▼
                                         ┌───────────────────────────────┐
                                         │  GitHub Release (public)        │
                                         │  gitid_<v>_darwin_amd64.tar.gz  │
                                         │  gitid_<v>_darwin_arm64.tar.gz  │
                                         │  gitid_<v>_linux_amd64.tar.gz   │
                                         │  gitid_<v>_linux_arm64.tar.gz   │
                                         │  gitid_<v>_checksums.txt        │
                                         │  + provenance attestation        │
                                         └───────────────┬─────────────────┘
                                                          │
                          ┌───────────────────────────────┼──────────────────────┐
                          ▼                                ▼                       ▼
              ┌────────────────────┐         ┌─────────────────────┐   ┌──────────────────┐
              │ scripts/install.sh  │         │ brew install         │   │ manual curl +      │
              │ (curl|bash, D-14)    │         │ castocolina/homebrew-│   │ sha256sum -c        │
              │ downloads .tar.gz +  │         │ tap/gitid (Bazzite/  │   │ (D-15 verify         │
              │ checksums.txt,        │         │ Fedora/macOS)         │   │  commands)            │
              │ verifies BEFORE        │         └─────────────────────┘   └──────────────────┘
              │ extract, installs to   │
              │ ~/.local/bin            │
              └─────────────────────────┘

   Runtime: `gitid --version` / `gitid version --json`
       └─▶ internal/version.Resolve() (D-09)
            prefers ldflags (main.version/commit/buildDate,
            injected identically by `make build`/`make build-cross`/
            goreleaser via {{.Env.VERSION}})
            falls back to debug.ReadBuildInfo() for `go install`/
            plain `go build` paths (Main.Version, vcs.revision,
            vcs.time, vcs.modified)
```

### Recommended Project Structure
```
.github/workflows/
├── ci.yml                    # extended: adds a `fedora` container job (D-01/D-02)
└── release.yml               # NEW: tag-triggered goreleaser release (D-06)
internal/version/
├── version.go                # ldflags vars + Resolve() (D-09)
└── version_test.go           # table test, one case per build path
cmd/gitid/
├── main.go                   # version/commit/buildDate vars now sourced from internal/version
└── version_cmd.go             # NEW: `gitid version` subcommand + --json (D-11)
.goreleaser.yaml               # NEW: build/archive/checksum/release/brews pipes (D-05/D-07/D-08/D-13)
scripts/install.sh             # REWRITTEN: tar.gz download+extract instead of raw binary (D-14, breaking change from Phase 9.3)
PLATFORM-NOTES.md              # NEW: per-distro ledger (D-04)
.planning/phases/10-.../
└── bazzite-uat-checklist.md   # NEW: manual UAT evidence log (Claude's discretion on format)
e2e/
└── release_e2e_test.go        # REWRITTEN: 17 existing test functions assume raw-binary
                                # shape; must be updated for tar.gz archives (see Pitfall 1)
```

### Pattern 1: goreleaser build config sharing the Makefile's version vars
**What:** `.goreleaser.yaml`'s `builds.ldflags` references `{{.Env.VERSION}}`/`{{.Env.COMMIT}}`/`{{.Env.DATE}}` — the SAME environment variables the Makefile's `VERSION ?= $(shell git describe ...)` computes — rather than letting goreleaser compute its own `{{.Version}}` internally via its own `git describe`.
**When to use:** Any time D-05's "no drift between two build descriptions" constraint applies — i.e., always, for this phase.
**Why this matters (Pitfall, not just style):** goreleaser's own internal version detection defaults to `git describe --tags --dirty --always` with NO `--match "v*"` filter `[CITED: goreleaser.com/customization/templates/]`. This repo has non-release tags (`poc-0.0.1`, `backup/*` per D-10's own comment) that the Makefile's `--match "v*"` deliberately excludes. Left unconfigured, goreleaser could silently compute a DIFFERENT version string than `make build`/`make install` do for the same commit. Two independent fixes both work: (a) export `VERSION`/`COMMIT`/`DATE` from the `make release` recipe and reference `{{.Env.VERSION}}` in ldflags (matches D-05's letter — "share ONE version/ldflags definition"), or (b) add `git: ignore_tags: ["poc-*", "backup/*"]` to `.goreleaser.yaml` so goreleaser's own git-describe-based `{{.Version}}` agrees with the Makefile's filtered one `[CITED: goreleaser.com/customization/git/ — ignore_tags supports glob patterns]`. Recommend (a): it is simpler, provably identical, and matches the existing Makefile-owns-the-truth pattern this repo already uses for `golangci-lint`/`freeze` pinning.
**Example:**
```yaml
# Source: goreleaser.com/customization/build/ + templates/ (cross-checked)
builds:
  - id: gitid
    main: ./cmd/gitid
    binary: gitid
    env:
      - CGO_ENABLED=0
    goos: [darwin, linux]
    goarch: [amd64, arm64]
    flags:
      - -trimpath
    ldflags:
      - -s -w -X main.version={{.Env.VERSION}} -X main.commit={{.Env.COMMIT}} -X main.buildDate={{.Env.DATE}}
```

### Pattern 2: Fedora container job — install Node.js BEFORE actions/checkout
**What:** The `container:` key at job level pins every step (including `actions/checkout`) to run INSIDE the `fedora:latest` image. GitHub-authored actions (`actions/checkout`, `actions/setup-go`) are Node.js-based and will fail with a cryptic error if Node is missing — and `fedora:latest` ships no Node.js by default.
**When to use:** The fedora job in `ci.yml` (D-01).
**Verified empirically this session** (`docker run --rm fedora:latest sh -c '...'`):
- Missing by default: `wget`, `git`, `ssh`, `ssh-keygen`, `make`, `node`/`nodejs`, `which`
- Present by default: `curl`, `sha256sum`, `tar`, `gzip`
- `dnf install -y --setopt=install_weak_deps=False git openssh-clients make nodejs tar gzip` resolved cleanly (exit 0) and installed `git 2.55.0`, `ssh`/`ssh-keygen`/`ssh-add` (from `openssh-clients`), `make 4.4.1` (GNU Make), `node v22.23.1`
**Example:**
```yaml
# Source: docs.github.com/en/actions/using-jobs/running-jobs-in-a-container
# (cross-checked with a real `docker run fedora:latest` dnf install this session)
fedora:
  name: fedora (container)
  runs-on: ubuntu-latest
  if: github.event_name == 'push'   # D-02: push-to-main + tags only, not every PR
  container:
    image: fedora:latest
  steps:
    - name: Install prerequisites (dnf; fedora image ships no node/git/make)
      run: dnf install -y --setopt=install_weak_deps=False git openssh-clients make nodejs tar gzip
    - name: Checkout
      uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0 # v7.0.0 (reuse existing pin)
    - name: Set up Go
      uses: actions/setup-go@924ae3a1cded613372ab5595356fb5720e22ba16 # v6.5.0 (reuse existing pin)
      with:
        go-version: "1.26.x"
    - run: make test
    - run: make lint
    - run: make test-e2e
```
**Note:** the default shell for `run:` steps inside a container is `sh`, not `bash` `[CITED: docs.github.com/en/actions/using-jobs/running-jobs-in-a-container]` — every existing Makefile recipe here already targets POSIX-safe constructs where it matters (`lint-shell`'s own comment notes macOS `sh -n` runs bash in POSIX mode), so this is a low-risk note, not a blocker.

### Pattern 3: internal/version.Resolve() hybrid ldflags/buildinfo
**What:** Prefer linker-injected vars; fall back to `debug.ReadBuildInfo()` when they're empty (the `go install`/plain-`go build` paths).
**When to use:** D-09.
**Verified empirically this session:**
```
$ go build -o /tmp/gitid-probe ./cmd/gitid && go version -m /tmp/gitid-probe
	mod	github.com/castocolina/gitid	v0.1.0-rc.9.0.20260904205517-7e1f66d66385+dirty
	build	vcs=git
	build	vcs.revision=7e1f66d6638585bf0474980c61e14126a3ac7b3c
	build	vcs.time=2026-09-04T20:55:17Z
	build	vcs.modified=true          # working tree had real uncommitted changes at build time
```
This directly refutes an older (2022) golang/go tracker claim that `vcs.*` fields are `go install`-only — on this repo's toolchain (go1.27.1, and by inheritance go1.26.x per the `GOTOOLCHAIN` pin), a PLAIN `go build` already stamps `vcs.revision`/`vcs.time`/`vcs.modified`. Second, targeted check — a FRESH CLONE with only an ignored `bin/untracked-test-file` present (no tracked-file changes) — produced `vcs.modified=false`, empirically confirming D-10's "`bin/` must be gitignored" guardrail is both necessary and SUFFICIENT: an ignored, untracked file in `bin/` does not flip the dirty flag.
**Example:**
```go
// internal/version/version.go
package version

import "runtime/debug"

var (
	version   string // set via -X main.version, wired through by cmd/gitid
	commit    string
	buildDate string
)

type Info struct {
	Version   string
	Commit    string
	BuildDate string
}

func Resolve() Info {
	if version != "" {
		return Info{Version: version, Commit: commit, BuildDate: buildDate}
	}
	info := Info{Version: "(devel)"}
	if bi, ok := debug.ReadBuildInfo(); ok {
		if bi.Main.Version != "" && bi.Main.Version != "(devel)" {
			info.Version = bi.Main.Version // e.g. go install github.com/.../gitid@v1.0.0
		}
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				info.Commit = s.Value
			case "vcs.time":
				info.BuildDate = s.Value
			case "vcs.modified":
				if s.Value == "true" {
					info.Version += "+dirty"
				}
			}
		}
	}
	return info
}
```

### Pattern 4: install.sh migrating from raw-binary to tar.gz-archive (D-07 breaking change)
**What:** The CURRENT `scripts/install.sh` (Phase 9.3) downloads `${BASE_URL}/gitid-<os>-<arch>` directly as an executable and verifies its checksum against `checksums.txt`'s entry for that exact filename. D-07 changes the published asset to `gitid_<version>_<os>_<arch>.tar.gz` (an archive containing the binary PLUS LICENSE/README), and `checksums.txt` will hash the ARCHIVE, not the raw binary.
**When to use:** D-14's rewrite, D-15's verify-command documentation.
**Required changes to `scripts/install.sh`:**
1. Asset name changes from `gitid-${os_tag}-${arch_tag}` to `gitid_${version}_${os_tag}_${arch_tag}.tar.gz` (note: underscore-separated, matches goreleaser's default `name_template`).
2. Download target becomes the `.tar.gz`; verify its SHA-256 against `checksums.txt` BEFORE extracting (D-14's "verify BEFORE extracting" ceremony is easier now — verify the single archive file, not a post-extraction binary).
3. `tar -xzf` the archive into the temp dir, THEN `chmod 0755` + `mv` the extracted `gitid` binary (LICENSE/README inside the archive are discarded or optionally kept for reference).
4. Add `GITID_VERSION` env var support (currently ABSENT from the script — `[VERIFIED: grep -n "GITID_VERSION" scripts/install.sh` → no match]`) so the D-14 pin requirement is actually met; without it, `${BASE_URL}` always resolves `releases/latest/download/...`, which cannot install a specific older version.
5. Add a custom-install-dir override (currently ABSENT — `INSTALL_DIR="${HOME}/.local/bin"` is a hardcoded literal with no env-var override point).
**Downstream blast radius:** all 17 test functions in `e2e/release_e2e_test.go` (`TestInstallScript_*`, `TestRelease_*`) assume the raw-binary shape and the OLD checksums format; they need a coordinated rewrite alongside the script itself, not a follow-on fix. This is the single largest concrete work item CONTEXT.md's decisions imply but do not spell out — flag it prominently for the planner (see Common Pitfalls #1).

### Anti-Patterns to Avoid
- **Re-implementing goreleaser's `prerelease: auto` as a hand-rolled bash `case` statement:** the CURRENT `ci.yml` release job does exactly this (lines computing `prerelease`/`make_latest` from the tag suffix via a shell `case`). goreleaser has this natively (`prerelease: auto` — "mark the release as not ready for production in case there is an indicator... e.g. v1.0.0-rc1" `[CITED: goreleaser.com/customization/release/]`). Migrating removes ~15 lines of custom shell logic and its test surface.
- **Invoking `goreleaser`/`goreleaser-action` raw in YAML:** violates D-05 and the repo's own "CI invokes the SAME make targets a human runs locally" header comment in `ci.yml`. Always route through `make release`/`make release-snapshot`.
- **Assuming the fedora container job can reuse `ci.yml`'s existing `check` job matrix:** it cannot — `runs-on: fedora-latest` does not exist as a GitHub-hosted runner label (confirmed by D-02's own rationale: "GitHub has no Fedora runners"). It must be a `container:`-keyed job on `ubuntu-latest`, structurally different from the `check` job's `matrix.os` pattern.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Cross-platform archive + checksum manifest generation | A custom shell loop wrapping `tar`/`sha256sum` (the CURRENT `make checksums` target does this today) | goreleaser's `archives:`/`checksum:` pipes | goreleaser's default `name_template` for both archives (`{{.ProjectName}}_{{.Version}}_{{.Os}}_{{.Arch}}`) and checksums (`{{.ProjectName}}_{{.Version}}_checksums.txt`) already produce EXACTLY D-07/D-15's target naming with zero custom code `[VERIFIED via WebFetch of goreleaser.com/customization/archive/ and .../checksum/, cross-checked against D-07's literal naming spec — they match]` |
| Prerelease/latest classification from tag suffix | Bash `case "${GITHUB_REF_NAME}" in *-*) ... esac` (exists today in `ci.yml`) | goreleaser `release: prerelease: auto` | Native tag-suffix detection; removes an entire hand-maintained branch of shell logic and its corresponding test coverage burden |
| GitHub Release creation + asset upload | `softprops/action-gh-release` (works today, but is now a SEPARATE action goreleaser's own `release:` pipe duplicates) | goreleaser's built-in `release:` pipe (reads `GITHUB_TOKEN` from env, same as the action does today) `[CITED: goreleaser.com/customization/release/ + WebSearch cross-check on GITHUB_TOKEN env var name]` | One fewer third-party action to SHA-pin and audit; the release-notes `header:`/`footer:` template fields directly satisfy D-08's "curated header line" requirement |
| Homebrew formula file generation/push | A script that clones the tap repo, edits a `.rb` file, and pushes | goreleaser's `brews:` pipe | Handles the git clone/commit/push cycle against the tap repo automatically, using the same PAT the pipeline already needs per D-13 |
| Build provenance / supply-chain attestation | A custom signing step (GPG, custom Sigstore client calls) | `actions/attest-build-provenance` | First-party, keyless (no key management burden), and explicitly what D-08 already specifies — implementing this by hand would be reinventing a GitHub-native primitive for no benefit |

**Key insight:** virtually everything this phase's Release pipeline decisions (D-05 through D-15) ask for is a goreleaser DEFAULT, not a customization. The main implementation risk is not "how do I build this feature" but "how do I make goreleaser's defaults line up EXACTLY with names/paths CONTEXT.md already locked" (D-07's literal `gitid_<version>_<os>_<arch>.tar.gz`, D-15's literal `gitid_<v>_checksums.txt`) — both of which this research confirmed ARE goreleaser's defaults when `ProjectName: gitid` is set, requiring no `name_template` override at all.

## Common Pitfalls

### Pitfall 1: The artifact-format migration is a breaking change to already-shipped, working code
**What goes wrong:** Phase 9.3 already shipped a WORKING BUILD-03 (raw binaries + `checksums.txt`, `[x] Complete` in REQUIREMENTS.md, verified live against `v0.1.0-rc.9` per that phase's own evidence). This phase's D-07 changes the artifact shape entirely (tar.gz archives). If the planner treats this as "add goreleaser alongside the existing pipeline," the two will conflict (same GitHub Release, different asset naming conventions) or silently ship both shapes. If treated as "replace," `scripts/install.sh` (which currently downloads and installs the raw binary directly) and all 17 `TestInstallScript_*`/`TestRelease_*` functions in `e2e/release_e2e_test.go` break simultaneously.
**Why it happens:** CONTEXT.md's D-07/D-14 describe the END state without calling out that Phase 9.3's END state is DIFFERENT and already merged/working.
**How to avoid:** Plan explicit tasks to (a) retire `ci.yml`'s current `release:` job (or repoint it at goreleaser via `make release`), (b) rewrite `install.sh` for the archive format including checksum-before-extract on the ARCHIVE file, (c) rewrite the e2e suite's fixtures/assertions for the new naming and archive-extraction flow, in that dependency order, within the SAME wave/commit-set per CLAUDE.md's buildable-boundary rule (a half-migrated state would fail `make lint`/`make test` module-wide).
**Warning signs:** `make test-e2e` failing on `TestInstallScript_OSArchMatrix` or similar after `.goreleaser.yaml` lands but `install.sh` hasn't been touched yet.

### Pitfall 2: goreleaser's own version detection ignores the Makefile's tag filter
**What goes wrong:** Without explicit wiring, `.goreleaser.yaml`'s `{{.Version}}` (computed via goreleaser's internal `git describe --tags --dirty --always`, no `--match` filter) can diverge from the Makefile's `VERSION ?= $(shell git describe --tags --match "v*" --always --dirty)` if `poc-0.0.1`/`backup/*` tags are ever the most-recent tag reachable from HEAD on some commit.
**Why it happens:** goreleaser and the Makefile are two independent `git describe` invocations with different flags by default.
**How to avoid:** Either export `VERSION`/`COMMIT`/`DATE` and reference `{{.Env.VERSION}}` in `.goreleaser.yaml`'s `ldflags` (Pattern 1 above), or add `git: ignore_tags: ["poc-*", "backup/*"]` to `.goreleaser.yaml`.
**Warning signs:** `gitid --version` on a goreleaser-built binary reporting something unexpected, e.g. `poc-0.0.1-N-g<sha>` instead of the pushed `v1.0.0` tag.

### Pitfall 3: fedora container job needs Node.js BEFORE the first Node-based action runs
**What goes wrong:** `actions/checkout` (and every other first-party action) is a Node.js action; running it as the first step inside a bare `fedora:latest` container fails because Node isn't present.
**Why it happens:** `fedora:latest`'s container image (unlike GitHub's `ubuntu-latest` HOSTED runner) ships almost nothing beyond a minimal base — confirmed empirically this session (`curl`, `sha256sum`, `tar`, `gzip` present; `git`, `ssh`, `make`, `node`, `wget`, `which` all MISSING).
**How to avoid:** The job's FIRST step must be a `run: dnf install -y ... nodejs ...` (does not require checkout — `dnf` is already on the image), BEFORE `actions/checkout`.
**Warning signs:** A cryptic `node: not found` or similar failure on the very first `uses:` step of the fedora job.

### Pitfall 4: `brews:` is soft-deprecated; `homebrew_casks:` is the forward-looking replacement
**What goes wrong:** A planner or executor blindly following goreleaser's OWN current documentation (which foregrounds `homebrew_casks:` and marks the `brews:` docs page "(deprecated)") might silently deviate from D-13's literal locked wording ("goreleaser `brews:` stanza").
**Why it happens:** goreleaser's docs actively steer new users toward `homebrew_casks:` since v2.10; `brews:` still works (confirmed no removal in the current v2.18.0 line — removal is only planned for an as-yet-unreleased v3) but its docs page is labeled deprecated.
**How to avoid:** Honor D-13 literally — use `brews:` — since it is still fully functional and explicitly the user's locked choice; do not silently substitute `homebrew_casks:`. Document the future-migration note in a code comment (mirroring this repo's existing style of documenting locked-but-evolving decisions, e.g. the `ci.yml` header's own deprecation commentary about macOS runner labels).
**Warning signs:** A future `goreleaser check`/`goreleaser release` emitting a deprecation warning about `brews:` — expected and non-blocking, not a bug.

### Pitfall 5: D-08's "`--verify-tag`" is a `gh` CLI flag, not a goreleaser or action-gh-release config key
**What goes wrong:** Searching goreleaser's or `softprops/action-gh-release`'s config surface for a `verify-tag`/`verify_tag` key will find nothing — it does not exist in either.
**Why it happens:** `--verify-tag` is specifically a `gh release create` CLI flag that "aborts the release if the tag doesn't already exist... preventing the automatic tag creation that normally occurs by default" `[VERIFIED via WebSearch cross-check of cli.github.com/manual/gh_release_create + community discussion, both agreeing]`. Since `release.yml` triggers ONLY `on: push: tags: ['v*']`, the tag by construction already exists on the ref being built — the risk `--verify-tag` guards against (a typo'd tag name auto-creating a NEW, unintended tag) structurally cannot occur in this trigger shape.
**How to avoid:** Treat D-08's literal `--verify-tag` wording as satisfied STRUCTURALLY by the `on: push: tags: ['v*']` trigger + `needs: [check, build-cross]`-equivalent gating (D-06 already requires re-running `make test`+`make lint` before publish), rather than searching for a nonexistent goreleaser config key. If the planner wants defense-in-depth, wire `gh release create --verify-tag ...` in a `make release-notes`-style helper step INSTEAD of relying solely on goreleaser's own release creation — but this is genuinely Claude's Discretion territory (CONTEXT.md itself hedges this as "planner detail").
**Warning signs:** Time spent searching goreleaser's config schema for a "verify tag" key that will not be found.

## Code Examples

### GitHub Release provenance attestation step (D-08)
```yaml
# Source: docs.github.com/actions/security-for-github-actions/using-artifact-attestations
# + github.com/actions/attest-build-provenance (v4.2.2, SHA verified via git ls-remote)
permissions:
  contents: write
  id-token: write
  attestations: write
steps:
  # ... after `make release` has produced bin/*.tar.gz + checksums.txt ...
  - name: Attest build provenance
    uses: actions/attest-build-provenance@4d101475d8b20a2381f78447822ac1eab6504dd8 # v4.2.2
    with:
      subject-path: "bin/*.tar.gz"
```
Note: `subject-path` supports glob (`@actions/glob` internally) so all four archives can be attested in one step; `subject-checksums` (pointing at the already-produced `checksums.txt`) is an alternative if the planner prefers hashing once via the Makefile's existing `SHA256SUM` variable rather than letting the action re-hash each file itself.

### goreleaser archives + checksum config matching D-07/D-15 EXACTLY (defaults, no override needed)
```yaml
# Source: goreleaser.com/customization/archive/ + .../checksum/ (WebFetch, cross-checked)
project_name: gitid
archives:
  - formats: [tar.gz]
    files:
      - LICENSE*
      - README*
    # name_template defaults to {{.ProjectName}}_{{.Version}}_{{.Os}}_{{.Arch}} — matches
    # D-07's gitid_<version>_<os>_<arch>.tar.gz with ZERO override needed.
checksum:
  algorithm: sha256
  # name_template defaults to {{.ProjectName}}_{{.Version}}_checksums.txt — matches
  # D-15's gitid_<v>_checksums.txt with ZERO override needed.
```

### goreleaser brews: stanza matching D-13
```yaml
# Source: goreleaser.com/customization/publish/homebrew_formulas/ (the current-but-deprecated
# `brews:` docs page; D-13 explicitly names `brews:`, honored per Pitfall 4)
brews:
  - name: gitid
    repository:
      owner: castocolina
      name: homebrew-tap
      token: "{{ .Env.HOMEBREW_TAP_GITHUB_TOKEN }}"   # the ONE PAT secret (D-13)
    homepage: "https://github.com/castocolina/gitid"
    description: "Manage multiple Git identities by coordinating SSH and Git configuration"
    directory: Formula
    install: |
      bin.install "gitid"
    test: |
      system "#{bin}/gitid", "--version"
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| Hand-rolled `build-cross` + `checksums` Makefile targets + `softprops/action-gh-release` for GitHub Release publishing (Phase 9.3, currently live in `ci.yml`) | goreleaser-driven `.goreleaser.yaml` (archives, checksum, release, brews pipes) invoked via `make release` | This phase (Phase 10, D-05) | Raw-binary artifacts become tar.gz archives (D-07); `install.sh` and its e2e suite must migrate in lockstep (Pitfall 1) |
| goreleaser `brews:` Homebrew stanza | goreleaser `homebrew_casks:` (maintainer-recommended since v2.10) | v2.10 (soft-deprecation, no forced removal until an unreleased v3) `[CITED: goreleaser.com/blog/goreleaser-v2.10/]` | D-13 explicitly locks `brews:`, which remains fully functional through v2.18.0 — no forced action this phase, but document the future migration path (Pitfall 4) |
| `actions/attest-build-provenance` as a standalone action | Action itself is now "a wrapper around `actions/attest`" as of its own v4; maintainers note "new implementations should use `actions/attest` instead" `[CITED: github.com/actions/attest-build-provenance]` | v4 (current) | `attest-build-provenance` v4.2.2 remains fully supported and is what D-08 names explicitly — no action needed, but worth a one-line code comment for future maintainers |
| golang/go issue #51637 (2022): `vcs.modified`/`vcs.revision` populated by `go install` but NOT plain `go build` | On this repo's current toolchain (go1.27.1 / GOTOOLCHAIN go1.26.4), a PLAIN `go build` already stamps full `vcs.*` info | Fixed sometime after Go 1.18's initial buildvcs rollout, confirmed still true today `[VERIFIED empirically this session — see Pattern 3]` | D-09's `Resolve()` fallback path works correctly for `go build`-produced dev binaries, not just `go install`-produced ones — slightly BROADER coverage than the old issue would suggest |

**Deprecated/outdated:**
- `brews:` (goreleaser): soft-deprecated but still fully functional; D-13 locks it explicitly — honor as-is.
- The Phase-9.3-era `ci.yml` `release:` job's bash `case` statement for prerelease detection: superseded by goreleaser's native `prerelease: auto`.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The Homebrew formula's `test:` block (`system "#{bin}/gitid", "--version"`) is sufficient for goreleaser's own `brews:` pipe validation — not independently verified against a real `brew audit`/`brew test` run this session | Code Examples | Low — this is a minimal, conventional Homebrew test block pattern; worst case the planner needs to adjust the assertion, not the mechanism |
| A2 | `HOMEBREW_TAP_GITHUB_TOKEN` is a reasonable secret name for D-13's tap-repo PAT — the exact literal secret name is Claude's Discretion, not locked by CONTEXT.md | Code Examples, D-13 | None — naming only, no functional risk |
| A3 | Fedora's `gcr-ssh-agent` (mentioned in D-03 for Bazzite GNOME) was NOT independently probed this session (container has no GNOME session to test against) — this is a Bazzite-UAT-time verification, not a CI-container one, per D-01/D-03's own split | D-03/PLATFORM-NOTES.md | None for this phase's CI work; the UAT checklist itself must carry this check, which CONTEXT.md already scopes correctly |
| A4 | Bazzite's claim that Homebrew is preinstalled on Universal Blue images (cited in CONTEXT.md's "Specific Ideas" as already user-verified) was taken as given, not re-verified in this research session | D-13 rationale | None — this is a pre-existing locked/verified claim from CONTEXT.md's own prior research, out of this session's scope to re-litigate |

**If this table is empty:** N/A — see entries above; none of them threaten a CORE architectural decision, all are either out-of-session-scope verifications already covered by CONTEXT.md or cosmetic naming choices.

## Open Questions

1. **Should `ci.yml`'s existing `release:` job be deleted outright, or repointed to call `make release`?**
   - What we know: D-06 says release publishing moves to a NEW, separate `release.yml`. `ci.yml`'s current header comment explicitly documents its own `release:` job (raw-binary shape) as the Phase 9.3 BUILD-03 implementation.
   - What's unclear: Whether the planner should delete `ci.yml`'s `release:` job entirely (avoiding a double-publish race on the same tag push) or whether some transitional overlap is desired.
   - Recommendation: Delete `ci.yml`'s `release:` job in the same commit set that adds `release.yml` — both trigger on `push: tags: ['v*']`, and having both active would either double-publish or race on the same GitHub Release object. This is implied by D-06 but not stated as a removal instruction; flag to the planner as a required companion change.

2. **`brews:` vs `homebrew_casks:` long-term migration timing.**
   - What we know: `brews:` works today (v2.18.0) and is what D-13 locks. Deprecation removal is targeted at an unreleased goreleaser v3 with no date.
   - What's unclear: Whether the user wants a forward-looking TODO comment in `.goreleaser.yaml` now, or would rather this be revisited only when goreleaser v3 actually ships.
   - Recommendation: Add a one-line code comment noting the deprecation status (low-cost, prevents future confusion); no functional action needed this phase.

3. **D-08's `--verify-tag` — literal gh-CLI wiring vs. structural satisfaction.**
   - What we know: `--verify-tag` is a `gh release create` flag with no goreleaser/action-gh-release equivalent; the release trigger's tag-push shape structurally prevents the failure mode it guards against.
   - What's unclear: Whether the user wants an EXPLICIT `gh release create --verify-tag` invocation somewhere in the pipeline (defense-in-depth, matching the literal decision text) or considers the trigger shape sufficient.
   - Recommendation: CONTEXT.md itself flags this as "planner detail" — surface it as a discuss-phase or plan-review question rather than guessing; both interpretations are defensible and low-risk either way.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Docker | Local fedora-container reproduction/testing (this research session's own verification method) | ✓ `[VERIFIED: docker run fedora:latest — succeeded]` | Docker Desktop (darwin/amd64 host) | — |
| `go` toolchain | All Go build/test work | ✓ `[VERIFIED: go version → go1.27.1 darwin/amd64]` | 1.27.1 local; repo pins `go 1.26` + `GOTOOLCHAIN=go1.26.4` in Makefile | Toolchain auto-download via `GOTOOLCHAIN` already handles this |
| `git` | Version stamping, goreleaser, tag-based triggers | ✓ (repo is a git working tree) | — | — |
| `gh` CLI | Potential D-08 `--verify-tag` wiring (Open Question 3) | Not probed this session (not required for research; only relevant if the planner chooses the explicit-`gh`-CLI resolution of Open Question 3) | — | goreleaser's native `release:` pipe does not require `gh` CLI at all — it uses the GitHub API directly via `GITHUB_TOKEN` |
| GitHub-hosted Fedora runner label | D-02's rejected alternative | ✗ (confirmed: GitHub has no `fedora-latest`-style hosted runner) | — | `container: image: fedora:latest` on `ubuntu-latest` (the D-01/D-02 chosen design) |

**Missing dependencies with no fallback:** none — every dependency this phase needs is either already present, installable via `dnf`/`go install`, or has a documented fallback.
**Missing dependencies with fallback:** GitHub-hosted Fedora runner (fallback: container job on ubuntu-latest, already the locked design).

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go's built-in `testing` package (`go test`), same as every other phase in this repo |
| Config file | none — build-tag isolation via `//go:build e2e` (existing pattern) |
| Quick run command | `go test ./internal/version/...` (new package, fast/hermetic) |
| Full suite command | `make test` (unit, -race) + `make test-e2e` (includes the rewritten `e2e/release_e2e_test.go`) + `make lint-shell` (POSIX parse check on the rewritten `scripts/install.sh`) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PLAT-03 | Full flow passes on Fedora container | CI job (not a `go test`) | The fedora job itself IS the test — `make test`/`make lint`/`make test-e2e` run inside the container | ✅ Wave 0 — job doesn't exist yet, must be added to `ci.yml` |
| PLAT-03 | Bazzite manual UAT residue (SELinux, real Wayland TUI rendering, `gcr-ssh-agent`) | manual-only | N/A — human-executed checklist | ❌ Wave 0 — `bazzite-uat-checklist.md` does not exist yet |
| BUILD-03 | `internal/version.Resolve()` correctness per build path | unit (table-driven) | `go test ./internal/version/... -run TestResolve -v` | ❌ Wave 0 — package doesn't exist yet |
| BUILD-03 | `gitid --version` / `gitid version --json` output format | unit + e2e | `go test ./cmd/gitid/... -run TestVersion` + PTY/subprocess check | ❌ Wave 0 — new subcommand |
| BUILD-03 | goreleaser produces correctly-named tar.gz archives + checksums.txt | e2e (rewritten) | `go test -tags e2e -run TestRelease` (rewritten `e2e/release_e2e_test.go`) | ⚠️ EXISTS but requires full rewrite (Pitfall 1) — not a gap so much as a migration |
| BUILD-03 | `install.sh` verifies-before-extract, GITID_VERSION pin, custom install dir | e2e (rewritten) | `go test -tags e2e -run TestInstallScript` (rewritten) | ⚠️ EXISTS but requires full rewrite — 17 test functions, GITID_VERSION/custom-dir cases are NET NEW (not present in current script at all) |
| BUILD-03 | `actions/attest-build-provenance` step runs and produces an attestation | CI-only (cannot be unit-tested; requires a real tag push against a public repo) | manual/CI verification post-first-real-release, matching Phase 9.3's own "Verified live against v0.1.0-rc.9" precedent | N/A — CI-only validation |

### Sampling Rate
- **Per task commit:** `go test ./internal/version/...` (new), `make lint-shell` (install.sh syntax)
- **Per wave merge:** `make test && make lint && make test-e2e`
- **Phase gate:** Full suite green before `/gsd-verify-work`, PLUS a real tag push to a scratch/prerelease tag to observe the actual `release.yml` run (mirrors Phase 9.3's "Verified live against v0.1.0-rc.9" precedent — this phase cannot be considered done on unit tests alone, since GitHub Release publishing and provenance attestation are fundamentally CI-environment-only behaviors)

### Wave 0 Gaps
- [ ] `internal/version/version_test.go` — table test per D-09 build-path case (ldflags-stamped, `go install`, plain `go build` clean, plain `go build` dirty)
- [ ] `cmd/gitid/version_cmd_test.go` — `gitid version` / `gitid version --json` output format
- [ ] `.planning/phases/10-linux-validation-release-pipeline/bazzite-uat-checklist.md` — manual UAT template (Claude's discretion on exact format, per D-04)
- [ ] `e2e/release_e2e_test.go` — full rewrite for tar.gz-archive shape (not a NEW file, but Wave-0-scale surgery)
- [ ] `.goreleaser.yaml` — does not exist yet
- [ ] `.github/workflows/release.yml` — does not exist yet
- [ ] `PLATFORM-NOTES.md` — does not exist yet (D-04)

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V5 Input Validation | yes | `install.sh`'s existing OS/arch ALLOWLIST pattern (`case "$os_raw" in Darwin) ... *) fail ...`) already follows this correctly — extend the SAME allowlist discipline to any new `GITID_VERSION`/install-dir env var handling (never interpolate an unvalidated env var directly into a URL or `rm -rf` path) |
| V10 Malicious Code / Supply Chain | yes | `actions/attest-build-provenance` (D-08) is the standard control here — first-party, keyless SLSA provenance rather than a custom signing scheme; goreleaser itself must be a PINNED version (`go install ...@v2.18.0`, not `@latest`) mirroring the existing `golangci-lint`/`gosec` pinning discipline already enforced in this Makefile |
| V14 Configuration / Build | yes | SHA-pinning every new GitHub Action (`actions/attest-build-provenance`) per this repo's existing, already-enforced discipline; job-scoped `permissions:` (never workflow-wide `write`) per D-06; the tap-repo PAT (D-13) must be scoped to ONLY the `homebrew-tap` repo, never a broad `repo` scope across the user's whole account |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Curl-pipe-to-shell install script tampering (MITM, compromised download) | Tampering | D-14's verify-BEFORE-extract SHA-256 check against `checksums.txt` (already implemented in the current script's core logic; must be preserved through the tar.gz rewrite) + README's "download, inspect, then run" phrasing showing the manual path first |
| Malicious/typosquatted goreleaser or attest-build-provenance action version | Tampering / Elevation of Privilege | Pin to a specific, `git ls-remote`-verified SHA/version (already this repo's discipline for every other action); never use `@latest`/floating tags |
| Over-privileged PAT for the Homebrew tap push | Elevation of Privilege | Scope the D-13 PAT narrowly to the `homebrew-tap` repo only (fine-grained PAT), never a classic PAT with account-wide `repo` scope |
| Release published from an untested/red commit (a tag pushed on a broken commit publishing a broken binary to the public) | Tampering / Denial of Service (for downstream users) | D-06's `re-runs make test + make lint before publishing` requirement — already the plan; the planner must wire this as a real BLOCKING step (e.g. `needs:` on a `check` job within `release.yml` itself, or an inline `make test && make lint` step before `make release` runs), matching `ci.yml`'s existing `needs: [check, build-cross]` pattern on its own soon-to-be-retired `release:` job |

## Sources

### Primary (HIGH confidence — empirically verified this session)
- `docker run --rm fedora:latest` (real container inspection) — confirmed missing/present packages, confirmed the exact `dnf install` command resolves cleanly, confirmed `ssh -V` output format
- `go build` + `go version -m` against this repo (real build) — confirmed `vcs.revision`/`vcs.time`/`vcs.modified` stamping behavior on plain `go build`, confirmed the `bin/`-gitignore guardrail empirically via a fresh-clone negative-case test
- `git ls-remote --tags` against `goreleaser/goreleaser`, `actions/attest-build-provenance`, `actions/checkout`, `actions/setup-go`, `softprops/action-gh-release`, `goreleaser/goreleaser-action` — confirmed live SHA pins, confirmed the existing `ci.yml` pins for `checkout@v7.0.0`/`setup-go@v6.5.0`/`action-gh-release@v3.0.3` are all still current and correct

### Secondary (MEDIUM confidence — WebFetch/WebSearch cross-checked against goreleaser.com official docs)
- `goreleaser.com/customization/archive/`, `.../checksum/`, `.../build/`, `.../templates/`, `.../release/`, `.../git/` — archive/checksum default name_templates, ldflags env-var referencing, `ignore_tags`, `prerelease: auto`
- `goreleaser.com/blog/goreleaser-v2.10/`, `.../customization/publish/homebrew_formulas/`, `.../deprecations/` — `brews:` vs `homebrew_casks:` deprecation status and timeline (cross-checked across 3 independent fetches for consistency)
- `docs.github.com/en/actions/using-jobs/running-jobs-in-a-container` + WebSearch cross-check on Node.js-in-fedora-containers
- `github.com/actions/attest-build-provenance` README + WebSearch on permissions/public-repo requirement
- `cli.github.com/manual/gh_release_create` (via WebSearch) — `--verify-tag` flag semantics

### Tertiary (LOW confidence — single WebSearch summary, flagged for validation)
- None retained as load-bearing; every LOW-confidence WebSearch finding in this session was either cross-checked to MEDIUM or independently falsified/confirmed via a real local command to HIGH.

## Metadata

**Confidence breakdown:**
- Fedora container job mechanics: HIGH — directly reproduced with `docker run`, not just cited
- goreleaser config surface (archives/checksum/brews/release): MEDIUM-HIGH — WebFetch-verified against official docs, cross-checked across multiple pages, but not run against a live goreleaser binary this session (no tag to release against)
- Version stamping (`internal/version`/`debug.ReadBuildInfo`): HIGH — directly reproduced with `go build`/`go version -m` in this repo
- GitHub Actions SHA pins: HIGH — directly verified via `git ls-remote` against upstream repos
- `brews:` vs `homebrew_casks:` deprecation status: MEDIUM — WebFetch-based, internally consistent across 3 fetches, but not independently confirmed via a live `goreleaser check` run

**Research date:** 2026-09-04
**Valid until:** ~30 days for the architecture/pattern guidance (stable); ~7-14 days specifically for the goreleaser version pin (v2.18.0) and any SHA pins, given goreleaser's observed release cadence — re-run `git ls-remote --tags` at execution time if this research is more than 2 weeks old.
