# gitid — Requirements (v1.0 — TUI-First Redesign, first real release)

> **Redefinition (2026-07-02).** Nothing was ever released — the prior build was a
> **POC now archived as milestone `0.0.1`** (we discovered the real goals through it).
> This spec is the **real v1.0**: a clean redefinition of the whole product around a
> **design-driven, TUI-first, screenshot-verified** delivery method. The POC spec +
> roadmap + phases are archived under
> [`.planning/archive/0.0.1-poc-product-features-in-tui/`](./archive/0.0.1-poc-product-features-in-tui/).
> The existing Go
> packages (`internal/identity`, `internal/sshconfig`, `internal/gitconfig`,
> `internal/doctor`, `internal/tester`, `internal/filewriter`, `tui/`) are treated
> as **reusable substrate to refactor**, NOT as a behavior contract.
>
> Canonical end state is still [`recipes/`](../recipes/) (SSH alias per identity,
> `Hostname`/`Port 443` alt-SSH, `IdentitiesOnly yes`, `includeIf`
> `hasconfig:`/`gitdir:`, `insteadOf`). Structure from the recipes; key algorithm
> is now user-selectable (see KEY).

REQ-ID format: `[CATEGORY]-[NUMBER]`. Status legend: `[x]` built substrate to
reuse · `[~]` partially built, needs rework · `[ ]` new.

Derived from the clarification session `.planning/DISCUSSION-v2-redesign.md`
(this conversation) and grounded in real `ssh -G` verification of the `Include`
strategy (OpenSSH 9.7, absolute-path Include resolves; first-match-wins).

---

## A. Delivery Method (DLV) — how every feature is built

These are first-class, enforced requirements — the user's core process ask.

- [ ] **DLV-01** (Design-first): Every UI-bearing phase produces an **HTML mockup**
  (React or similar + the `/mui` skill) BEFORE any Go/TUI code is written for that
  surface. The mockup encodes layout, field order, labels, copy, and flow.
- [ ] **DLV-02** (Agents on every UI task): The `agent-ui-ux-designer` agent AND the
  `/mui` skill are engaged on every UI-related task — during planning, execution,
  AND review. Any plan touching an interface names both in its task list.
- [ ] **DLV-03** (Screenshot pipeline): Every flow/screen of the HTML mockup is
  captured to image files; every screen of the Go **TUI dummy mockup** is captured;
  both sets are stored as versioned reference artifacts (e.g. under
  `.planning/design/<surface>/{html,tui}/*.png`).
- [ ] **DLV-04** (Visual-regression gate): In every UI wave's review step, reviewer
  agents diff the **live TUI screens** against the **approved** HTML + TUI-mockup
  screenshots (appearance, fields, options, labels). Divergence from approved design
  is a review finding.
- [ ] **DLV-05** (Per-surface build order): For each UI surface the order is fixed:
  HTML mockup → screenshots → Go TUI **dummy** mockup (full navigation, no backend
  logic) → screenshots → **user approval (checkpoint #1)** → backend logic wiring →
  e2e → visual-regression review. Backend logic is never written before the dummy
  mockup is approved.
- [ ] **DLV-06** (e2e per screen): Every screen has at least one e2e test that drives
  the **real** built binary via raw keystrokes (PTY), not only unit stubs — closes
  the recurring injected-seam blindspot.
- [x] **DLV-07** (UI-free TDD core): Core logic stays in UI-free packages and is
  built test-first; config parse→render→parse is round-trip stable. *(carried from
  CLAUDE.md; already the project norm)*
- [ ] **DLV-08** (Single human checkpoint): The autonomous build loop runs unattended
  except for **ONE** hard stop — **design approval** (HTML + TUI mockup screenshots).
  Credential upload is **automated** when `gh`/`glab` is present + authenticated AND a
  valid identity exists; only if that is unavailable does upload fall back to a manual
  step. *(Resolved 2026-07-02 — was two checkpoints.)*

## B. Project Tooling & Standards (TOOL)

- [x] **TOOL-01**: `Makefile` exposes `setup-env`, `build`, `install`, `uninstall`,
  `test`, `lint`, `fmt`.
- [x] **TOOL-02**: `make setup-env` bootstraps golangci-lint, gosec, pre-commit, hooks.
- [x] **TOOL-03**: pre-commit hooks run fmt/lint/security/tests via the same `make`
  targets CI uses; `--no-verify` is forbidden.
- [x] **TOOL-04**: Core is TDD; parse→render→parse round-trip proven by tests.
- [ ] **TOOL-05** (Screenshot tooling): A repeatable way to capture TUI screens
  (View()-dump / teatest frame capture / PTY snapshot) and HTML screens (headless
  browser) exists as a `make` target or scripted step the loop can call.

## C. Key & Algorithm (KEY)

> **Scope note:** gitid is a **local-use tool** — there is NO CI/CD-oriented
> algorithm fallback logic. Algorithm choice, catalog info, and troubleshooting are
> framed around **local macOS and Linux availability and variants** (the local
> `ssh-keygen`, LibreSSL-on-macOS vs OpenSSL-on-Linux, agent/keychain differences,
> hardware-backed keys), not server/CI compatibility.

- [ ] **KEY-01** (Algorithm catalog): The create flow shows a **top-5 catalog** of
  key algorithms with per-algorithm info (security, and **macOS/Linux local
  availability + variant notes**) and a clear "best/default" recommendation.
  Default = **ed25519**. Candidate catalog: `ed25519` (best), `ed25519-sk`
  (hardware/FIDO2 — needs libfido2/security key), `rsa-4096`, `ecdsa-p256`,
  `ecdsa-sk`. Final ordering set during the design phase.
- [ ] **KEY-02** (Real multi-algorithm keygen): gitid generates real keys for at
  least **ed25519** (default) and **rsa-4096**. Architecture leaves room for
  `ecdsa-p256` and the `-sk` hardware variants without a redesign. Selection is a
  local user preference, not a compatibility workaround.
- [ ] **KEY-03** (Platform-aware troubleshooting): When the chosen algorithm is
  unavailable or misbehaves **on the local machine** — e.g. an older Linux distro's
  `ssh-keygen` lacks a variant, macOS LibreSSL vs Linux OpenSSL differences,
  `ed25519-sk` with no FIDO2 device / `libfido2`, agent/keychain quirks
  (`UseKeychain` macOS-only vs `ssh-agent` on Linux) — the flow surfaces concrete,
  platform-specific troubleshooting hints and a fallback recommendation from the
  catalog. gitid probes the local `ssh-keygen`/`ssh` capabilities to drive this.
- [x] **KEY-04** (Permissions): keys/files get correct perms (`~/.ssh` 700, key 600,
  `.pub` 644, `config` 600).
- [~] **KEY-05** (Rotate): rotate/replace an identity's key; artifacts re-point and
  the test flow re-runs. *(built; must be re-fitted to the new manager + algorithms)*
- [~] **KEY-06** (Reuse existing key): create an identity that reuses an existing key
  instead of generating one. *(built; re-fit to new create flow)*
- [ ] **KEY-07** (New key for existing identity): from the manager, generate a new
  key for an existing identity (distinct from rotate — see MGR-05).

## D. SSH Identity Screen (SSHUI)

- [ ] **SSHUI-01** (Field model): The SSH screen fields, in order, are —
  **`Alias prefix`** (e.g. `personal`) → **`SSH Host`** = the recipe `Host`, the full
  `git@…` target, auto-joined as `<prefix>.<provider>` but **editable** →
  **`Real hostname`** = recipe `Hostname`, the true SSH endpoint (provider-linked,
  editable, e.g. `ssh.github.com`) → **`Port`** (default **443**, editable). Blank
  prefix → `SSH Host` = the provider host itself (WYSIWYG, no invented suffix).
- [ ] **SSHUI-02** (Clickable fields): Fields are focusable by mouse click as well as
  keyboard (Tab/arrows). All fields always visible; none buried in an overflowing
  panel.
- [ ] **SSHUI-03** (Live preview): A live `Host` block preview reflects the current
  field values exactly as it will be written.
- [x] **SSHUI-04** (tmp-file testing): All options are tested against throwaway temp
  files (`ssh -F <tmp> -i <key>`), never mutating the live config until confirm.
  *(built — the temp-config pivot)*
- [x] **SSHUI-05** (macOS globals): a `Host *` block emits `UseKeychain yes` +
  `AddKeysToAgent yes` guarded by `IgnoreUnknown UseKeychain`, ordered after specific
  hosts. *(carried)*

## E. SSH Connectivity Test Screen (TEST)

- [x] **TEST-01** (Two-stage, command-visible): Test the key **direct** (provider
  URL, no alias) first, then **targeted** (the alias). Each stage shows the **exact
  command run** and its real output, on every phase (pre-run, success, failure).
  *(built; re-verify under redesign)*
- [x] **TEST-02** (`ssh -G` proof): the resolved test proves which `IdentityFile` the
  config actually resolves. *(built)*
- [ ] **TEST-03** (Store or adopt): On pass + user agreement, persist to
  `~/.ssh/config` **or** to the gitid-owned Include'd file (see STORE), with backup.

## F. SSH Config Storage (STORE) — research-backed

- [ ] **STORE-01** (Dual strategy): gitid manages SSH config as either (a) sentinel
  blocks in `~/.ssh/config` (default, current) or (b) a gitid-owned file
  (`~/.ssh/config.d/gitid.config` or per-identity files) pulled in via a single
  `Include ~/.ssh/config.d/*.config` line placed **near the top** of `~/.ssh/config`.
  Include paths MUST be absolute or `~/.ssh`-relative (verified: relative paths
  resolve against `~/.ssh/` and silently fail otherwise).
- [ ] **STORE-02** (Adopt external): Detect and adopt an existing external Include'd
  ssh file so users who already split their config keep that layout.
- [ ] **STORE-03** (Migration): A safe, backed-up, reversible migration between the
  in-file and Include'd layouts.
- [x] **STORE-04** (Safe writes): every mutation = timestamped backup + idempotent
  sentinel-block rewrite + atomic write-temp→rename→chmod + explicit confirmation;
  content outside managed blocks preserved verbatim. *(carried; the invariant)*

## G. Git Configuration Screen (GITUI)

- [ ] **GITUI-01** (Separate, post-SSH screen): Git config is its own screen AFTER
  the SSH screens. The planner is given the **git recipe** and must model all
  parameters (global vs per-identity); this screen exposes **per-identity
  (individual)** options only.
- [x] **GITUI-02** (Fragment fields): user.name / user.email, `gpg.format=ssh`,
  `user.signingkey` (path, not literal), `commit.gpgsign`, written to
  `~/.gitconfig.d/<identity>`. *(built — LEG 2)*
- [x] **GITUI-03** (Match strategy): `gitdir:` and `hasconfig:remote.*.url` (default
  `gitdir`), combinable, with a **live `includeIf` preview**. *(built)*
- [x] **GITUI-04** (Signing line): `~/.ssh/allowed_signers` line
  `<email> namespaces="git" ssh-ed25519 …`, email byte-identical to `user.email`.
  *(built)*
- [ ] **GITUI-05** (Review → confirm → write): a read-only review screen precedes the
  git write; confirm writes fragment + `includeIf` + `allowed_signers` (backup +
  idempotent).

## H. Identity Manager (MGR) — app main view

- [ ] **MGR-01** (Completeness in list): The identity list shows, per row, whether the
  identity is **complete** (SSH + Git) or **incomplete** (flagged; e.g. SSH-only, no
  git config).
- [ ] **MGR-02** (State taxonomy): Each identity/key is classified and visually
  distinguished across at least: **complete** (ssh+git) · **incomplete** (ssh, no
  git) · **git-only** (git identity relying on global SSH, no own Host block) ·
  **key-unused** (key exists, no identity references it) · **key-used-ssh-only** ·
  **key-used-both** (auth + signing) · **key-missing** (an identity references a key
  file that is absent) · **fragment-path-missing** (an `includeIf` points at a
  non-existent fragment).
- [ ] **MGR-03** (SSH-first detail): The identity detail view shows **SSH details
  first**, then Git details, and never renders nonexistent git attributes for an
  SSH-only identity (no mixed/blank fields).
- [ ] **MGR-04** (Clone): Clone an identity into a **new name** (must differ from the
  source) that either **references the same SSH key** or **generates a new key**;
  user customizes the clone before writing.
- [ ] **MGR-05** (New key): Generate a new key for an existing identity from the
  manager (see also KEY-07).
- [~] **MGR-06** (Delete choice): Delete asks **"delete everything (SSH + Git + key)"**
  vs **"delete the Git identity only"** (keep the key / SSH). *(current delete only
  removes managed blocks/keys — must add the two-way choice.)*
- [ ] **MGR-07** (Per-identity health): Opening an identity shows whether **that
  identity** is healthy (its key resolves, fragment exists, signing wired, etc.).
- [x] **MGR-08** (No sidecar DB): the identity list is reconstructed by parsing
  managed blocks. *(carried)*

## I. Global SSH Options (GSSH)

- [ ] **GSSH-01** (Danger-aware): A global-SSH-options screen surfaces SSH config
  options that are **dangerous by default when unset/misconfigured** (e.g.
  `StrictHostKeyChecking`, `ForwardAgent`, `HashKnownHosts`, `IdentitiesOnly`,
  `AddKeysToAgent`, `UseKeychain`) and **explains every option** with its risk and
  recommended value. Advisory + fixable, never blocking.

## J. Global Git Options (GGIT)

- [ ] **GGIT-01** (Baseline + defaults): A global-git-options screen manages the
  shared config: `init.defaultBranch` (highlight **main vs master** since distros
  still default to master), `core.ignorecase` (false), `core.autocrlf`/eol (line-feed
  policy), global `user.email`, plus recipe defaults (`push.autoSetupRemote`,
  `pull.rebase`, `fetch.prune`, aliases, color, `merge.conflictstyle`,
  `diff.colorMoved`). Each option explained. *(GLOBAL-01/GITIGNORE-01/URLRW-01 built
  as substrate to fold in.)*

## K. Health (HLTH)

- [ ] **HLTH-01** (Two sections): The health screen has **SSH** and **Git** sections.
- [ ] **HLTH-02** (Files + syntax): Checks config files exist and parse (syntax valid).
- [ ] **HLTH-03** (Redundancy/override): Detects repeated or overridden directives and
  duplicate managed/global blocks (e.g. multiple `Host *`). *(CheckRedundancy built.)*
- [ ] **HLTH-04** (Contradictions): Detects contradictory settings where possible
  (e.g. `IdentitiesOnly no` with a specific `IdentityFile`; alias whose `includeIf`
  targets a missing fragment).
- [ ] **HLTH-05** (Per-identity): Health is computable for a single identity (feeds
  MGR-07) and globally.
- [x] **HLTH-06** (Deps/perms/coherence/orphans/signing/agent): the existing doctor
  families are the substrate. *(carried — re-home into the two-section screen.)*

## L. Fixer (FIX)

- [x] **FIX-01** (Confirmed, backed-up fixes): Detected problems have a severity +
  explanation + suggested fix, applied only with confirmation and backup. *(doctor
  fix engine built — re-home into the health screen.)*
- [ ] **FIX-02** (Two-section fixer UX): The fixer presents SSH and Git problems in
  the health screen's two sections and fixes them in place.

## M. Upload / Credentials (UP)

- [x] **UP-01** (Auth + signing instructions): concrete steps to add the `.pub` for
  authentication and signing (GitHub = two registrations; GitLab = one). *(built —
  `internal/upload`.)*
- [x] **UP-02** (Assisted upload): `gh`/`glab` detect + prompt + upload; shown command
  == run command; absent/unauth falls back to manual, never gates create/copy.
  *(built — `internal/uploader`.)*
- [ ] **UP-03** (Auto-upload when possible): When `gh`/`glab` is authenticated and a
  valid identity exists, credential upload runs **autonomously** (no stop); the shown
  command equals the run command. Otherwise it falls back to a manual step. Not a
  mandatory checkpoint.

## N. TUI Shell & CLI Parity (SHELL)

- [x] **SHELL-01** (Integrated app): a single Bubble Tea v2 app — header + identity
  sidebar + master-detail pane + footer — composed under focus moves, collapsible on
  narrow terminals, SSH-safe. *(built; redesign refines the views.)*
- [ ] **SHELL-02** (View set): the app's primary views are — **1) Identities
  (manager)**, **2) Global SSH options**, **3) Global Git options**, **4) Health**,
  **5) Fixer** — reachable via palette + number keys.
- [x] **SHELL-03** (CLI parity): every product action reachable from both TUI and a
  Cobra CLI; shell completion for bash/zsh/fish. *(carried; keep parity for new
  actions.)*

## O. Platform Availability & Variants (PLAT)

> gitid is local-use and must behave correctly across **macOS and Linux** and their
> variants. No Windows.

- [ ] **PLAT-01** (Capability probing): gitid probes the **local** toolchain
  (`ssh-keygen -Q key` / `ssh -V` / presence of `libfido2`, agent, keychain) to
  drive the algorithm catalog (KEY-01), troubleshooting (KEY-03), and doctor hints —
  instead of assuming a fixed feature set.
- [ ] **PLAT-02** (macOS vs Linux variants): Surfaces and handles the known
  divergences — `UseKeychain`/`AddKeysToAgent` (macOS Keychain) vs `ssh-agent`
  (Linux); LibreSSL (macOS) vs OpenSSL (Linux) `ssh-keygen`; clipboard `pbcopy` vs
  `wl-copy`/`xclip`; per-OS install hints (`brew`/`apt`/`dnf`/`pacman`).
- [ ] **PLAT-03** (Linux validation): The full flow is validated end-to-end on at
  least one mainstream Linux distro (in addition to macOS) as a dedicated phase; any
  portability gaps are fixed or logged as accepted limitations. *(carried intent.)*

## P. Build & Distribution CI/CD (BUILD)

> Distinct from the "local-use, no CI/CD algorithm fallback" runtime note (KEY): that
> is about runtime behavior; this is about **building and shipping the binary**.

- [ ] **BUILD-01** (Cross-platform build matrix): CI/CD (GitHub Actions) builds gitid
  binaries for **darwin/amd64** (macOS Intel), **darwin/arm64** (Apple Silicon), and
  **linux/amd64** (plus **linux/arm64** if cheap) — reproducible via `make` targets so
  the same build works locally and in CI.
- [ ] **BUILD-02** (CI gates on both OSes): On PR/push, CI runs `make test` (race) +
  `make lint` (golangci-lint + gosec) + `make test-e2e` on **macOS and Linux**
  runners; red gates block merge. This is where PLAT-02/PLAT-03 divergences are caught.
- [ ] **BUILD-03** (Release artifacts): On a version tag, CI publishes the built
  binaries to GitHub Releases with **SHA-256 checksums**; the binary reports its
  version (`gitid --version`) stamped at build time (ldflags).
- [ ] **BUILD-04** (Reproducible dev bootstrap): `make setup-env` on a fresh macOS or
  Linux clone reproduces the CI toolchain (golangci-lint, gosec, pre-commit, hooks).
  *(TOOL-02 substrate; verify on both OSes.)*

## Out of Scope

- **Shippable Web UI** — the HTML/React/`mui` mockups are **design + review
  artifacts only** (living design docs + screenshot references); the shipped product
  is the terminal app. *(Assumption — user chose "living design doc"; not "ship a web
  UI".)*
- **Windows** — macOS + Linux only (Linux validation is its own phase).
- **GPG commit signing** — replaced by ssh-key signing.
- **Scheduled/automatic key rotation** — user-initiated only.
- **Secret-vault integration** — keys live in `~/.ssh` with correct perms.

## Resolved Decisions

1. ✅ **Checkpoints** — **ONE** hard checkpoint: **HTML mockup / design approval**.
   Credential upload auto-runs when `gh`/`glab` is authenticated + a valid identity
   exists; manual fallback otherwise (DLV-08 / UP-03). *(Resolved 2026-07-02.)*
2. ✅ **Sequencing** — the loop runs **at end of day, after the roadmap is built, all
   phases planned, and the plans reviewed**. Requirements + ROADMAP are the
   authoritative goal files (GSD). Not auto-launched.
3. ✅ **Provider catalog** — GitHub / GitLab / Bitbucket (recipe set) + custom/
   self-hosted. Local-use tool; no server/CI special-casing.
4. ✅ **STORE default** — in-file managed blocks default; Include'd file opt-in +
   adopt-existing. (Feasibility verified with real `ssh -G`.)
5. ✅ **Build CI/CD** — cross-platform release builds (macOS Intel/ARM, Linux) + CI
   gates on both OSes (section P, BUILD).

## Still Open (to reach ~100% before planning)

- **GSSH-01 option list** — the exact set of "dangerous-by-default" SSH options to
  surface/explain is not yet enumerated (candidates listed inline; pin during Phase 0
  research or here).
- **KEY-01 catalog ordering** — final top-5 order + per-algorithm copy (deferred to
  the design phase; acceptable).
- **Screenshot tooling** — the concrete TUI+HTML capture mechanism (TOOL-05/DLV-03)
  is a Phase-0 spike, not yet chosen.

## Definition of Done (v1.0)

- Every UI surface has: an approved HTML mockup + screenshots, an approved Go TUI
  dummy-mockup + screenshots, backend logic, e2e tests, and a passing
  visual-regression review against the approved design.
- All requirements above implemented and traced to a phase.
- `make test` (race), `make lint` (golangci-lint + gosec), `make test-e2e` green;
  pre-commit hooks enforce them; no `--no-verify`.
- No mutation path lacks backup + idempotent managed-block write + confirmation.
- All generated content in English.
- The two human checkpoints are the only manual stops.

## User Stories

- As a developer, I approve the whole app's look and flow from **HTML mockups**
  before any terminal code is written, and every screen I later see in the TUI
  matches what I approved (screenshot-verified).
- As a developer, I pick my key algorithm from an informed catalog (ed25519 by
  default, RSA-4096 when a server demands it), with troubleshooting when the best
  option won't work.
- As a developer, I create an identity by filling a clear SSH screen (alias prefix →
  full host → real hostname → port), test it against throwaway configs with the exact
  commands shown, then a separate Git screen, then review + confirm.
- As a developer, I manage all my identities in one place — seeing at a glance which
  are complete vs incomplete, which keys are unused or missing, and cloning or
  deleting (all vs git-only) with confirmation.
- As a developer, I open a Health screen split into SSH and Git, see redundant/
  contradictory config, and fix it in place.

## Traceability

Every v1.0 requirement (sections A–P) maps to **exactly one** phase in
[`.planning/ROADMAP.md`](./ROADMAP.md). Coverage: **68 / 68 (100%)**, no orphans, no
duplicates. `Status` is `Pending` until the owning phase completes; `[x]`/`[~]` items
are built/partial substrate re-homed into (and re-verified by) their phase. DLV-01..06
additionally recur as UI-wave success criteria in every UI-bearing phase (3–9); the
row below records each one's **home** phase.

| Requirement | Phase | Status |
|-------------|-------|--------|
| DLV-01 | Phase 2 | Pending |
| DLV-02 | Phase 2 | Pending |
| DLV-03 | Phase 1 | Pending |
| DLV-04 | Phase 3 | Pending |
| DLV-05 | Phase 2 | Pending |
| DLV-06 | Phase 3 | Pending |
| DLV-07 | Phase 1 | Pending |
| DLV-08 | Phase 2 | Pending |
| TOOL-01 | Phase 1 | Pending |
| TOOL-02 | Phase 1 | Pending |
| TOOL-03 | Phase 1 | Pending |
| TOOL-04 | Phase 1 | Pending |
| TOOL-05 | Phase 1 | Pending |
| KEY-01 | Phase 1 | Pending |
| KEY-02 | Phase 1 | Pending |
| KEY-03 | Phase 1 | Pending |
| KEY-04 | Phase 1 | Pending |
| KEY-05 | Phase 5 | Pending |
| KEY-06 | Phase 3 | Pending |
| KEY-07 | Phase 5 | Pending |
| SSHUI-01 | Phase 3 | Pending |
| SSHUI-02 | Phase 3 | Pending |
| SSHUI-03 | Phase 3 | Pending |
| SSHUI-04 | Phase 3 | Pending |
| SSHUI-05 | Phase 3 | Pending |
| TEST-01 | Phase 3 | Pending |
| TEST-02 | Phase 3 | Pending |
| TEST-03 | Phase 3 | Pending |
| STORE-01 | Phase 1 | Pending |
| STORE-02 | Phase 1 | Pending |
| STORE-03 | Phase 1 | Pending |
| STORE-04 | Phase 1 | Pending |
| GITUI-01 | Phase 4 | Pending |
| GITUI-02 | Phase 4 | Pending |
| GITUI-03 | Phase 4 | Pending |
| GITUI-04 | Phase 4 | Pending |
| GITUI-05 | Phase 4 | Pending |
| MGR-01 | Phase 5 | Pending |
| MGR-02 | Phase 1 | Pending |
| MGR-03 | Phase 5 | Pending |
| MGR-04 | Phase 5 | Pending |
| MGR-05 | Phase 5 | Pending |
| MGR-06 | Phase 5 | Pending |
| MGR-07 | Phase 5 | Pending |
| MGR-08 | Phase 5 | Pending |
| GSSH-01 | Phase 6 | Pending |
| GGIT-01 | Phase 7 | Pending |
| HLTH-01 | Phase 8 | Pending |
| HLTH-02 | Phase 8 | Pending |
| HLTH-03 | Phase 8 | Pending |
| HLTH-04 | Phase 8 | Pending |
| HLTH-05 | Phase 8 | Pending |
| HLTH-06 | Phase 8 | Pending |
| FIX-01 | Phase 8 | Pending |
| FIX-02 | Phase 8 | Pending |
| UP-01 | Phase 9 | Pending |
| UP-02 | Phase 9 | Pending |
| UP-03 | Phase 9 | Pending |
| SHELL-01 | Phase 5 | Pending |
| SHELL-02 | Phase 5 | Pending |
| SHELL-03 | Phase 5 | Pending |
| PLAT-01 | Phase 1 | Pending |
| PLAT-02 | Phase 1 | Pending |
| PLAT-03 | Phase 10 | Pending |
| BUILD-01 | Phase 1 | Pending |
| BUILD-02 | Phase 1 | Pending |
| BUILD-03 | Phase 10 | Pending |
| BUILD-04 | Phase 1 | Pending |
