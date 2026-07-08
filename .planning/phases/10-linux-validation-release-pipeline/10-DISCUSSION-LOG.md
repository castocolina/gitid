# Phase 10: Linux Validation + Release Pipeline - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-08
**Phase:** 10-linux-validation-release-pipeline
**Areas discussed:** Linux validation scope, Release pipeline shape, Version stamping, Artifact set & install story
**Mode:** `--research` — four parallel `gsd-advisor-researcher` agents; CI/Makefile/version substrate pre-gathered and injected. USER constraints up front: Bazzite/Fedora validation (not Ubuntu-only) + a curl|bash install script. One agent verified Go 1.26 buildvcs behavior empirically; one verified the cross-built binary is statically linked.

---

## Linux validation scope

| Option | Description | Selected |
|--------|-------------|----------|
| Container + Bazzite UAT (Recommended) | fedora:latest container CI job (full suite incl. PTY e2e) + one manual Bazzite UAT per release for the container-invisible residue | ✓ |
| Container only | Desktop surface (clipboard/agent/rendering) unvalidated — where PLAT-02 bugs were actually found | |
| Manual UAT only | No durable regression coverage as fedora:latest rebases | |

| Option | Description | Selected |
|--------|-------------|----------|
| Fedora, push-to-main+tags (Recommended) | fedora:latest only; reuses D-13 cost-tier lever; Ubuntu triple-covered per-PR | ✓ |
| Fedora per-PR | Environment drift changes on rebases, not per PR | |
| Broader matrix | debian/openSUSE = filler coverage | |

| Option | Description | Selected |
|--------|-------------|----------|
| Approve checklist (Recommended) | REAL: ssh -V suffix, wl-clipboard presence, no-agent KDE degradation. VERIFY-ONCE: SELinux ls -Z, /var/home includeIf, real-terminal rendering | ✓ |
| Revise priorities | | |

| Option | Description | Selected |
|--------|-------------|----------|
| PLATFORM-NOTES.md (Recommended) | Root ledger, per-distro rows, README-linked; UAT evidence feeds it | ✓ |
| README LIMITATIONS section | Bloat + marketing/defect mix | |
| .planning only | Invisible to users | |

**Notes:** Research verified Bazzite userland == Fedora userland (image delivery is the delta); GitHub has no Fedora runners; PTY e2e works in containers; the classic SELinux ssh_home_t failure is server-side authorized_keys (theoretical for gitid's user-context writes); Fedora KDE ships NO ssh-agent by default (the divergent axis to test).

---

## Release pipeline shape

| Option | Description | Selected |
|--------|-------------|----------|
| make + gh CLI (Recommended) | Zero new tools; single build definition | |
| goreleaser | Archives/checksums/changelog/tap automation; second build definition | ✓ (USER, with make-wrapping nuance) |
| goreleaser as pinned binary from make | Middle ground | (subsumed by user's shape) |

**User's choice:** First asked "Why not GitHub workflows?" — clarified that ALL options run inside a release.yml workflow; the question was the tooling within it. Then chose goreleaser: "If the second is best, we can extend or redefine the make targets" → goreleaser wrapped in make targets (make release / make release-snapshot), one shared version/ldflags definition, make remains the entry point.

| Option | Description | Selected |
|--------|-------------|----------|
| release.yml + re-gate (Recommended) | tags v*; job-scoped contents: write; default GITHUB_TOKEN; re-run test+lint pre-publish | ✓ |
| workflow_run gate | Tag→run correlation footgun | |
| Trust the tag | Untested binaries can ship | |

| Option | Description | Selected |
|--------|-------------|----------|
| tar.gz archives (Recommended) | gitid_<v>_<os>_<arch>.tar.gz + checksums.txt; Go CLI norm; tap-compatible naming | ✓ |
| Raw binaries + SHA256SUMS | Off-norm; later format switch breaks early adopters | |

| Option | Description | Selected |
|--------|-------------|----------|
| Attest + auto-notes (Recommended) | attest-build-provenance (first-party keyless SLSA; public repo) + --verify-tag --generate-notes + curated header | ✓ |
| Checksums only | If repo stays private | |
| Also commit-log changelog | More code for cosmetic output | |

---

## Version stamping

| Option | Description | Selected |
|--------|-------------|----------|
| Approve package (Recommended) | internal/version + Resolve() (ldflags ⊕ ReadBuildInfo — truthful on all 4 build paths, EMPIRICALLY VERIFIED on Go 1.26); git describe --match "v*" in Makefile; rich SetVersionTemplate + gitid version --json; bin/ gitignore + fetch-depth: 0 guardrails | ✓ |
| Flag-only, ldflags-only | Version lies on go-install/hand builds; no TUI access | |

**Notes:** Verified: go build stamps Main.Version from VCS since Go 1.24 (+dirty on untracked files — including un-ignored bin/); go install @version reports the module version with no vcs.* data; const at main.go:15 must become a var.

---

## Artifact set & install story

| Option | Description | Selected |
|--------|-------------|----------|
| 4 targets + CGO=0 (Recommended) | + linux-arm64 best-effort (already built); CGO_ENABLED=0 -trimpath explicit (static today only by accident) | ✓ |
| 3 targets literal | Discards a free artifact | |
| Also darwin universal | Doubles download; brew picks arch | |

| Option | Description | Selected |
|--------|-------------|----------|
| Full hardening (Recommended) | USER LOCKED the script itself; hardening: verify SHA-256 BEFORE extract, ~/.local/bin, GITID_VERSION pin, inspect-first README phrasing, CI-tested on 3 platforms | ✓ |
| Minimal script | No embedded verification — bad optics for a security tool | |

| Option | Description | Selected |
|--------|-------------|----------|
| Tap in v1.0 (Recommended) | Bazzite docs recommend brew (preinstalled on Universal Blue); goreleaser brews: automation; one PAT secret (pipeline's only) | ✓ |
| Fast-follow | Secretless v1.0 pipeline | |
| No tap | Leaves Bazzite's preferred channel unused | |

**Notes:** Research had recommended deferring the install script (curl|bash optics); the user had pre-locked it — hardening details presented instead. COPR/RPM rejected (useless on Bazzite; lazygit COPR bit-rot). Flatpak rejected (sandbox blocks ~/.ssh + host tools). Late USER ADDITION at the create gate: end the phase by executing the README-crafting skill to update README.md, including the per-OS checksum verify commands (sha256sum / shasum -a 256, --ignore-missing -c).

---

## Claude's Discretion

- fedora container job internals (dnf list, setup-go-in-container, env shims).
- Bazzite UAT checklist format + evidence log location.
- .goreleaser.yaml internals within the make-wrapped, one-version-definition contract.
- install.sh implementation details (sh vs bash, wget fallback) within the hardening contract.
- Curated release-header wording; best-effort arm64 note; version --json field names.

## Deferred Ideas

- openSUSE/Arch/debian containers; Fedora VM job.
- cosign/GPG; commit-log changelog script.
- .rpm via nfpms (free byproduct only); darwin universal; riscv64.
- Per-artifact .sha256 files.
