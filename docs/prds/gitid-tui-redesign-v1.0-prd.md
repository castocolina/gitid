# gitid TUI-First Redesign — Product Requirements Document (v1.0)

> Companion to [`.planning/REQUIREMENTS.md`](../../.planning/REQUIREMENTS.md) (the
> authoritative REQ ledger). This PRD gives the narrative, design decisions,
> acceptance criteria, and the **phased roadmap** the autonomous loop executes.
> Nothing was released; this is the real v1.0 and supersedes the archived 0.0.1 POC
> spec wholesale. Existing Go packages are reusable substrate, not a behavior contract.

## Requirements Description

### Background

- **Business problem**: gitid manages coherent SSH + Git identity artifacts
  (`~/.ssh/config`, `~/.gitconfig`, `~/.gitconfig.d/*`, `~/.ssh/allowed_signers`,
  ed25519/rsa keys) proven to authenticate and resolve **before** anything is
  written. The v1.0 build shipped the logic but the TUI create/manage flow accreted
  UX debt (confusing host/alias fields, mixed SSH/Git detail, no design contract).
- **Target users**: developers juggling multiple Git identities (personal + several
  clients) on **local macOS and Linux** machines. Local-use tool — no CI/CD concern.
- **Value proposition**: an app whose every screen is **designed and approved as an
  HTML mockup first**, then faithfully realized in the terminal and continuously
  **screenshot-diffed** against that approved design — so UX quality is a gate, not
  an afterthought.

### Feature Overview

- **Core surfaces**: (1) Create-identity flow — Algorithm catalog → SSH screen →
  connectivity test → Git screen → review/confirm; (2) Identity Manager with a rich
  completeness/health state taxonomy, clone, new-key, delete-all-vs-git-only;
  (3) Global SSH options (danger-aware); (4) Global Git options (main-vs-master, line
  endings, case, email, recipe defaults); (5) Health (SSH + Git sections);
  (6) Fixer.
- **Delivery method as a feature**: design-first HTML mockups (`/mui` +
  `agent-ui-ux-designer`), a screenshot pipeline, a Go TUI dummy mockup, and a
  **visual-regression review gate** in every UI wave.
- **Boundaries**: the shipped product is the terminal app; HTML mockups are living
  design/review artifacts, **not** a shipped web UI. macOS + Linux only.

### Detailed Requirements

See `.planning/REQUIREMENTS.md` sections A–O. Highlights:

- **SSH field model (SSHUI-01)**: `Alias prefix` → `SSH Host` (full `git@…` target,
  `<prefix>.<provider>`, editable) → `Real hostname` (endpoint, provider-linked,
  editable) → `Port` (443). Fields clickable (SSHUI-02). Blank prefix ⇒ provider host
  (WYSIWYG).
- **Algorithm (KEY)**: top-5 catalog, ed25519 default, real ed25519 + rsa-4096
  keygen, **platform-aware** (macOS/Linux) troubleshooting via local capability
  probing.
- **SSH storage (STORE)**: in-file managed blocks (default) **or** a gitid-owned
  Include'd file (`Include ~/.ssh/config.d/*.config` near top; absolute paths;
  first-match-wins — all verified) **or** adopt an existing external file.
- **Manager state taxonomy (MGR-02)**: complete / incomplete / git-only /
  key-unused / key-used-ssh-only / key-used-both / key-missing / fragment-missing.
- **Health (HLTH)**: SSH + Git sections; files, syntax, redundancy/override,
  contradictions, per-identity.

## Design Decisions

- **Design-first, screenshot-verified pipeline** (the defining decision): no backend
  logic for a surface until its HTML mockup **and** Go TUI dummy mockup screenshots
  are approved by the user (checkpoint #1). Thereafter every UI wave's review diffs
  the live TUI against the approved images.
- **`ssh -G` against temp files** remains the test substrate (verified, already
  built) — the create flow never mutates live config to test.
- **SSH `Include` strategy is feasible** — verified live: OpenSSH 9.7, absolute
  Include path resolves the block; relative paths resolve against `~/.ssh/` (gotcha);
  first-match-wins requires the Include line near the top.
- **`git config` via `os/exec`** for gitconfig reads/writes; `includeIf` blocks as
  sentinel-delimited managed text; `kevinburke/ssh_config` for SSH round-trip. *(all
  carried — the toolchain is proven.)*
- **UI-free TDD core**; every write through the `filewriter` chokepoint (backup +
  atomic + idempotent + confirm).

### Constraints

- **Local-use only**; no CI/CD algorithm fallback. macOS + Linux, no Windows.
- **One human checkpoint**: design (HTML mockup) approval. Credential upload
  auto-runs when `gh`/`glab` is authenticated + a valid identity exists.
- `make test` (race) / `make lint` (golangci-lint + gosec) / `make test-e2e` green;
  no `--no-verify`; English-only artifacts.

### Risk Assessment

- **Screenshot tooling for a TUI** is the biggest unknown — mitigated by a Phase-0
  spike (View()-dump / teatest / PTY capture) before any design work depends on it.
- **Include migration** could disturb a user's hand-written config — mitigated by
  backup + reversible migration + adopt-in-place.
- **Autonomous drift** while unattended — mitigated by the visual-regression gate,
  `verifier` + `deep` code review, and the two hard checkpoints.
- **Multi-algorithm keygen** widens the security surface — mitigated by keeping the
  `filewriter`/perms invariants and adding e2e per algorithm.

## Acceptance Criteria

### Functional
- [ ] Create an identity end-to-end: algorithm picked from catalog → SSH screen
  (clickable fields, live preview) → two-stage test showing exact commands → Git
  screen → review → confirm; four artifacts written with backup; both tests shown.
- [ ] Manager shows every identity's completeness/health state; clone (same key OR
  new key, new name), new-key, and delete (all vs git-only) all work with backup.
- [ ] Health screen renders SSH + Git sections; flags redundant/overridden/
  contradictory config; per-identity health resolves; Fixer applies fixes with
  confirmation.
- [ ] SSH storage works in-file AND via Include'd file; existing external file
  adoptable; migration reversible.

### Quality
- [ ] Every UI surface has approved HTML + TUI-mockup screenshots; live TUI passes
  the visual-regression diff.
- [ ] UI-free core, TDD, round-trip stable; e2e per screen drives the real binary.
- [ ] `make test`/`lint`/`test-e2e` green; no write path lacks backup + confirmation.

### User acceptance
- [ ] Hand-written config outside managed blocks preserved verbatim.
- [ ] Every TUI screen matches the design the user approved.
- [ ] Works on macOS and one Linux distro.

## Execution Phases

> Each **UI phase** wave = `/mui` + `agent-ui-ux-designer` (plan + build + review) →
> implement TUI screen → e2e (PTY) → **visual-regression diff vs approved
> screenshots** → deep code review. Human stops only at ★ checkpoints.

### Phase 0 — Foundations, spikes & CI (no product UI)
- [ ] Screenshot tooling: repeatable TUI capture + HTML headless capture (TOOL-05/DLV-03)
- [ ] Algorithm capability probing + multi-algo keygen (ed25519 + rsa-4096) (KEY-01..03, PLAT-01)
- [ ] SSH Include strategy: own-file writer + adopt-external + reversible migration (STORE)
- [ ] Identity state-taxonomy model in core (MGR-02)
- [ ] **CI/CD**: GitHub Actions cross-build matrix (darwin amd64/arm64, linux amd64)
  + `make test`/`lint`/`test-e2e` gates on macOS + Linux runners (BUILD-01/02/04)
- **Deliverables**: spikes proven with tests; core seams ready; CI green on both OSes.

### Phase 1 — DESIGN: all HTML mockups + TUI dummy mockup ★ CHECKPOINT #1
- [ ] HTML+mui mockups for every surface (create flow, manager, global ssh, global
  git, health, fixer); screenshot every flow
- [ ] Go TUI **dummy** mockup — full navigation, no backend; screenshot every screen
- [ ] Visual diff HTML↔TUI-mockup; `agent-ui-ux-designer` critique
- **★ Human checkpoint**: user approves design (screenshots). Approved images become
  the reference set for all later waves.

### Phase 2 — Create flow backend (Algorithm → SSH → Test → Store)
### Phase 3 — Git configuration screen backend
### Phase 4 — Identity Manager (states, detail SSH-first, clone, new-key, delete-choice, per-identity health)
### Phase 5 — Global SSH options (danger-aware, explained)
### Phase 6 — Global Git options (main/master, eol, case, email, recipe defaults)
### Phase 7 — Health (SSH + Git sections) + Fixer
### Phase 8 — Upload / credentials assist (auto when gh+auth+valid identity; else manual)
### Phase 9 — Linux validation (PLAT-03) + Release pipeline (BUILD-03: tagged artifacts + checksums)

Each of Phases 2–9 that touches UI repeats the per-wave UI gate above.

---

**Document Version**: 2.0
**Created**: 2026-07-02
**Clarification Rounds**: 3 (algorithm/storage/milestone/mockup + local-use/platform refinement)
**Quality Score**: ~90/100 (4 open assumptions documented in REQUIREMENTS.md)
