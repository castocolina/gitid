# gitid — Roadmap

**Milestone:** v1 MVP
**Granularity:** standard
**Mode:** mvp (Vertical MVP)
**Coverage:** 45/45 v1 requirements mapped

---

## Phases

- [x] **Phase 1: Bootstrap** — Makefile, go.mod, golangci-lint v2, gosec, pre-commit hooks wired to make targets, TDD harness green (completed 2026-06-09)
- [x] **Phase 2: First Identity End-to-End** — Create one identity (ed25519 auth+signing), produce all four coordinated artifacts with backup + idempotent managed blocks + confirmation, prove authentication and config resolution with the two-phase test flow, clipboard copy, upload instructions (completed 2026-06-09)
- [x] **Phase 3: Full Identity CRUD + Multi-Identity** — List, update, delete identities; reconstruct from managed blocks (no sidecar DB); multiple identities on one provider via distinct aliases (completed 2026-06-10)
- [x] **Phase 3.1: Baseline Global Git Config + Global Gitignore** *(INSERTED)* — Seed and manage a shared baseline git config (core/push/pull/fetch/color toggles + aliases, `ignorecase=false`) and a curated global gitignore via `core.excludesfile`, in idempotent managed blocks with backup→preview→confirm; optional HTTPS→SSH `insteadOf` rewrites; baseline readable back from disk (completed 2026-06-11)
- [ ] **Phase 4: Doctor** — Deep health checks (deps, permissions, coherence/drift, orphans, signing wiring, agent) with severity + fix; `gitid doctor` CLI command
- [ ] **Phase 5: CLI Surface + TUI** — Full Cobra command surface with shell completion; Bubble Tea TUI launching to doctor dashboard with identity/account navigation
- [ ] **Phase 6: Linux Cross-Platform Validation** *(DEFERRED — post-v1)* — Validate the whole tool end-to-end on Linux (developed on macOS only): clipboard dispatch, per-OS install hints, file permissions, config-path resolution, the make/pre-commit toolchain, and the two-phase ssh test flow

---

## Phase Details

### Phase 1: Bootstrap

**Goal**: The development environment and quality toolchain are proven operational; any engineer can run `make setup-env` on a fresh clone and be ready to write TDD-tested Go code.
**Mode:** mvp
**Depends on**: Nothing
**Requirements**: TOOL-01, TOOL-02, TOOL-03, TOOL-04
**Success Criteria** (what must be TRUE):

  1. `make setup-env` on a clean checkout installs golangci-lint v2, gosec, and pre-commit hooks without errors
  2. `make test` runs the TDD harness and exits 0 (with coverage report)
  3. `make lint` and `make fmt` succeed; pre-commit hooks block a malformed commit
  4. `make build` produces a `gitid` binary and `make install` / `make uninstall` manage it

**Plans**: 3 plansPlans:
**Wave 1**

- [x] 01-01-PLAN.md — Initialize go.mod (github.com/castocolina/gitid, go 1.26) + full package skeleton (cmd/gitid + 10 internal packages + tui) with green stub tests; minimal LICENSE/README/.gitignore

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 01-02-PLAN.md — Makefile target surface (setup-env/build/install/uninstall/test/lint/fmt) + golangci-lint v2 curated config with hard-fail

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 01-03-PLAN.md — repo:local pre-commit (fast fmt+lint) + pre-push (full test) hooks wired to make targets; complete setup-env hook install; no CI

### Phase 2: First Identity End-to-End

**Goal**: A user can create one identity, see all four coordinated artifacts (SSH Host block, gitconfig includeIf, per-identity fragment, allowed_signers) written safely with backup and confirmation, and prove authentication plus resolved-config correctness via the two-phase test flow; the public key is on the clipboard and upload steps are shown.
**Mode:** mvp
**Depends on**: Phase 1
**Requirements**: IDENT-01, IDENT-02, IDENT-06, KEY-01, KEY-02, SSH-01, SSH-02, SSH-03, GIT-01, GIT-02, GIT-03, SIGN-01, SIGN-02, TEST-01, TEST-02, TEST-03, SAFE-01, SAFE-02, SAFE-03, CLIP-01, CLIP-02, UP-01, UP-02
**Success Criteria** (what must be TRUE):

  1. Running create produces four artifacts; `ssh -G <alias>` returns correct `identityfile`, `identitiesonly yes`, `user git`, correct `hostname` and `port`
  2. Both test phases print the exact command run and its real output; phase 1 (`ssh -i <key> -T <host>`) and phase 2 (`ssh -T <alias>` + `ssh -G <alias>`) both show pass
  3. A timestamped backup of every mutated file exists before any change; a second run is idempotent (no diff)
  4. The public key is in the clipboard on generate and on demand; upload instructions for GitHub/GitLab (auth + signing) are shown without consulting external docs
  5. `git log --show-signature` on a test commit inside the matched directory shows "Good signature" (signing wired end-to-end)

**Plans**: 7 plans

Plans:

**Wave 1**

- [x] 02-01-PLAN.md — internal/filewriter safe-write chokepoint (backup, atomic temp→rename→chmod, idempotent sentinel managed-block)
- [x] 02-02-PLAN.md — internal/platform + internal/deps (ssh -Q key probe, ed25519→rsa→ecdsa fallback, D-14 install hints, tool detection)

**Wave 2** *(blocked on Wave 1)*

- [x] 02-03-PLAN.md — internal/keygen (ed25519 + allowed_signers) + internal/clipboard (copy .pub, graceful failure); adds x/crypto + atotto
- [x] 02-05-PLAN.md — internal/gitconfig (includeIf + fragment + signing wiring) + internal/tester (two-phase output-substring classifier + ssh -G parse)

**Wave 3** *(blocked on Wave 2)*

- [x] 02-04-PLAN.md — internal/sshconfig render/parse/write (Host block, macOS Host * ordered last, idempotent round-trip); adds kevinburke/ssh_config

**Wave 4** *(blocked on Wave 3)*

- [x] 02-06-PLAN.md — identity.Create orchestration + `gitid identity add` Cobra command (create-new end-to-end vertical slice, upload steps, dry-run); adds cobra

**Wave 5** *(blocked on Wave 4)*

- [x] 02-07-PLAN.md — fast-follow modes: reuse-existing-key (IDENT-02), add-account/alias (IDENT-06), key rotation (KEY-01)

### Phase 3: Full Identity CRUD + Multi-Identity

**Goal**: Users can list, update, and delete identities; two identities on the same provider coexist via distinct aliases and each resolves to its own key; the tool reconstructs all state from managed blocks with no sidecar database.
**Mode:** mvp
**Depends on**: Phase 2
**Requirements**: IDENT-03, IDENT-04, IDENT-05, IDENT-07
**Success Criteria** (what must be TRUE):

  1. `gitid identity list` displays all identities with key path, alias, provider, port, and match strategy
  2. Two identities on the same provider coexist; `ssh -G <alias-A>` and `ssh -G <alias-B>` each resolve to their own distinct `IdentityFile`
  3. Deleting an identity removes its managed blocks from all four artifacts while preserving all content outside those blocks verbatim
  4. On a cold start with no running state, the tool reconstructs the exact identity list from the managed blocks in `~/.ssh/config` and `~/.gitconfig`

**Plans**: 4 plans

Plans:

**Wave 1**

- [x] 03-01-PLAN.md — Read-side primitives (ListBlocks/RemoveBlock/BackupAndRemove) + SSH/gitconfig reconstruction readers + identity.Reconstruct join (IDENT-07, foundation)

**Wave 2** *(blocked on Wave 1)*

- [x] 03-02-PLAN.md — `gitid identity list` slice: reconstruct-from-disk + render key path/alias/provider/port/match + light incomplete marker; multi-identity coexistence proof (IDENT-03, SC-2)

**Wave 3** *(blocked on Wave 2 — shares cmd/gitid/main.go)*

- [x] 03-03-PLAN.md — `gitid identity update` slice: edit fields (name immutable), WriteFragment signing toggle, structural-change re-test gate (IDENT-04)

**Wave 4** *(blocked on Wave 3 — shares cmd/gitid/main.go)*

- [x] 03-04-PLAN.md — `gitid identity delete` slice: per-identity removal manifest + two-step confirm (keep key default), global blocks untouched (IDENT-05)

### Phase 03.1: Baseline Global Git Config + Global Gitignore (INSERTED)

**Goal**: gitid manages a coherent global baseline — a shared git-config layer (sensible `core`/`push`/`pull`/`fetch`/`color` toggles and curated aliases, `core.ignorecase=false`) and a curated global gitignore wired via `core.excludesfile` — all in idempotent managed blocks written with the same backup → preview → confirm safety as identity artifacts, plus optional HTTPS→SSH `insteadOf` rewrites; the baseline is viewable and re-derivable from disk (no sidecar DB).
**Mode:** mvp
**Depends on**: Phase 3
**Requirements**: GLOBAL-01, URLRW-01, GITIGNORE-01
**Success Criteria** (what must be TRUE):

  1. Running the baseline setup writes a managed shared-config block with `core.ignorecase=false`, `push.autoSetupRemote=true`, `pull.rebase=true`, `fetch.prune=true`, `color.ui=auto`, and a curated alias set — idempotent (a second run produces no diff) and all content outside the managed block preserved verbatim
  2. `core.excludesfile` points to a gitid-managed `~/.gitignore_global` containing curated OS/editor/build excludes (`.DS_Store`, `Thumbs.db`, `*.log`, `*.bak`, `*.tmp`, `*.swp`); re-running is idempotent
  3. The user previews the full baseline before it is written and confirms once; a timestamped backup of `~/.gitconfig` (and any other mutated file) exists before any change (SAFE-01/02/03)
  4. HTTPS→SSH `insteadOf` rewrites are offered with the suggested mapping shown and editable before they are written (URLRW-01)
  5. gitid reads back and displays the current managed baseline state from disk with no sidecar database, consistent with the identity-reconstruction model (IDENT-07)

**Plans**: 4 plans
**UI hint**: yes
**Canonical refs**: samples/gist-60f2f1d-gitconfig, samples/gist-2c98cff-ssh-config

Plans:

**Wave 1**

- [x] 03.1-01-PLAN.md — `PrependBlockIfNotFound` floor-placement primitive in internal/filewriter (TDD foundation)

**Wave 2** *(blocked on Wave 1)*

- [x] 03.1-02-PLAN.md — baseline/url-rewrites/gitignore byte-stable renderers + writers via filewriter chokepoint (GLOBAL-01, URLRW-01, GITIGNORE-01)

**Wave 3** *(blocked on Wave 2 — shares internal/gitconfig/baseline.go)*

- [x] 03.1-03-PLAN.md — `ScanConflicts` (block-stripped) + `ReadBaselineState` sidecar-free read-back + independent url-rewrites removal

**Wave 4** *(blocked on Waves 2-3 — shares cmd/gitid/main.go)*

- [x] 03.1-04-PLAN.md — `gitid baseline setup`/`show` Cobra commands (preview→confirm→write, --dry-run, idempotency)

### Phase 4: Doctor

**Goal**: Users can run `gitid doctor` and receive a complete, actionable health report covering dependencies, permissions, coherence/drift, orphans, signing wiring, and ssh-agent state — each finding with severity and a suggested fix.
**Mode:** mvp
**Depends on**: Phase 3
**Requirements**: DOC-01, DOC-02, DOC-03, DOC-04, DOC-05, DOC-06, DOC-07
**Success Criteria** (what must be TRUE):

  1. `gitid doctor` reports each missing dependency with a per-OS install hint (brew / apt / dnf / pacman)
  2. Doctor finds and reports incorrect permissions on `~/.ssh`, keys, `.pub`, and config files with a specific fix command
  3. Doctor detects at least one coherence gap (e.g., an `IdentityFile` that no longer resolves, a missing `allowed_signers` entry) and reports it with severity and suggested fix
  4. Doctor detects orphaned artifacts (key file with no managed block, fragment with no `includeIf`) and reports them distinctly from coherence failures
  5. Each finding includes severity, a plain-English explanation, and a suggested fix; auto-fix is offered with confirmation where applicable

**Plans**: 5 plans
**UI hint**: yes

Plans:

**Wave 1**

- [x] 04-01-PLAN.md — Foundation slice: Finding/Severity/Family model + doctor.Deps + Run/ExitCode + Permissions family + minimal `gitid doctor` grouped renderer end-to-end (DOC-02, DOC-06, DOC-07)

**Wave 2** *(blocked on Wave 1 — each adds only its own checks/*.go, no shared-file edits)*

- [x] 04-02-PLAN.md — Dependencies + Baseline families: deps.Detect compose + extended platform.InstallHint (git/clipboard), ReadBaselineState fold-in (DOC-01, D-16)
- [ ] 04-03-PLAN.md — Coherence + Orphans families: existence/resolution + locked-value carve-outs; block-vs-disk orphans + unused-key (DOC-03, DOC-04)
- [ ] 04-04-PLAN.md — Signing + Agent families: ssh-add probe + fingerprint match + git<2.36 hasconfig: gate (DOC-05)

**Wave 3** *(blocked on Waves 1-2 — shares cmd/gitid/doctor.go)*

- [ ] 04-05-PLAN.md — Auto-fix slice: D-04 gate/per-finding-confirm/--yes flow + permission batching; fixes routed through filewriter chokepoint (DOC-06)

### Phase 5: CLI Surface + TUI

**Goal**: The full `gitid` command surface is available with shell completion, and running `gitid` with no arguments launches a Bubble Tea TUI that opens on the doctor dashboard and lets users navigate to identity and account management.
**Mode:** mvp
**Depends on**: Phase 4
**Requirements**: CLI-01, CLI-02, TUI-01, TUI-02
**Success Criteria** (what must be TRUE):

  1. `gitid` (no args) launches the TUI; the doctor dashboard is the first screen shown
  2. From the TUI dashboard the user can navigate to the identity list and add/edit forms without leaving the application
  3. `gitid completion bash`, `gitid completion zsh`, and `gitid completion fish` each produce valid shell completion scripts
  4. Every Phase 2–4 capability (`doctor`, `identity add/list/test`, `host add`, `rotate`, `copy`) is reachable as a `gitid` subcommand

**Plans**: TBD
**UI hint**: yes

### Phase 6: Linux Cross-Platform Validation

**Status:** DEFERRED (post-v1) — do not plan or execute until Phases 1–5 are complete on macOS and a Linux environment is available.
**Goal**: The entire gitid tool is proven to work end-to-end on Linux. The product is developed and tested exclusively on macOS/darwin; this phase exercises every platform-specific surface on at least one mainstream Linux distribution (e.g. Ubuntu/Debian + one of Fedora/Arch) and fixes any portability gaps found.
**Mode:** validation/hardening (no new product requirements — re-verifies existing ones on Linux)
**Depends on**: Phase 5
**Requirements**: none new — cross-platform re-verification of TOOL-01..04, KEY-*, SSH-*, GIT-*, SIGN-*, TEST-*, CLIP-*, DOC-* on Linux
**Scope** (platform-specific surfaces to validate on Linux):

  1. Clipboard dispatch resolves to `wl-copy`/`xclip` (Wayland/X11) instead of `pbcopy`; copy-on-generate and copy-on-demand both work
  2. `gitid doctor` per-OS dependency install hints render the correct package manager (`apt` / `dnf` / `pacman`) and missing-dep detection works
  3. File-permission handling on `~/.ssh`, private keys, `.pub`, and config files behaves correctly under Linux defaults/umask; doctor permission checks and fixes are accurate
  4. SSH/git config path resolution (`~/.ssh/config`, `~/.gitconfig`, `includeIf`, `allowed_signers`) works with Linux home/XDG conventions
  5. The toolchain bootstraps on Linux: `make setup-env` installs golangci-lint, gosec, and pre-commit; `make build/test/lint/fmt` and the git hooks all run
  6. The two-phase ssh test flow (`ssh -i`, `ssh -T`, `ssh -G`) produces correct real output on Linux

**Success Criteria** (what must be TRUE):

  1. `make setup-env` + `make test` + `make lint` + the pre-commit/pre-push hooks all succeed on a fresh Linux clone
  2. Creating, listing, testing, and deleting an identity works end-to-end on Linux with all four artifacts written/backed-up correctly
  3. Clipboard copy works via the Linux clipboard backend; `gitid doctor` shows correct per-OS install hints and permission findings
  4. Any portability defects found are fixed (or explicitly logged as accepted limitations) and the macOS suite still passes (no regressions)

**Plans:** 2/5 plans executed

Plans:

- [ ] TBD (deferred — run /gsd-plan-phase 6 to break down once Phases 1–5 are done)

---

## Progress Table

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Bootstrap | 3/3 | Complete   | 2026-06-09 |
| 2. First Identity End-to-End | 7/7 | Complete   | 2026-06-09 |
| 3. Full Identity CRUD + Multi-Identity | 4/4 | Complete    | 2026-06-10 |
| 3.1. Baseline Global Git Config + Global Gitignore | 4/4 | Complete    | 2026-06-11 |
| 4. Doctor | 2/5 | In Progress|  |
| 5. CLI Surface + TUI | 0/? | Not started | - |
| 6. Linux Cross-Platform Validation | 0/? | Deferred (post-v1) | - |
