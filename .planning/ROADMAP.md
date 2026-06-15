# gitid — Roadmap

**Milestone:** v1 MVP
**Granularity:** standard
**Mode:** mvp (Vertical MVP)
**Coverage:** 42/42 v1 requirements mapped

---

## Phases

- [x] **Phase 1: Bootstrap** — Makefile, go.mod, golangci-lint v2, gosec, pre-commit hooks wired to make targets, TDD harness green (completed 2026-06-09)
- [ ] **Phase 2: First Identity End-to-End** — Create one identity (ed25519 auth+signing), produce all four coordinated artifacts with backup + idempotent managed blocks + confirmation, prove authentication and config resolution with the two-phase test flow, clipboard copy, upload instructions
- [ ] **Phase 3: Full Identity CRUD + Multi-Identity** — List, update, delete identities; reconstruct from managed blocks (no sidecar DB); multiple identities on one provider via distinct aliases
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
- [ ] 02-05-PLAN.md — internal/gitconfig (includeIf + fragment + signing wiring) + internal/tester (two-phase output-substring classifier + ssh -G parse)

**Wave 3** *(blocked on Wave 2)*

- [ ] 02-04-PLAN.md — internal/sshconfig render/parse/write (Host block, macOS Host * ordered last, idempotent round-trip); adds kevinburke/ssh_config

**Wave 4** *(blocked on Wave 3)*

- [ ] 02-06-PLAN.md — identity.Create orchestration + `gitid identity add` Cobra command (create-new end-to-end vertical slice, upload steps, dry-run); adds cobra

**Wave 5** *(blocked on Wave 4)*

- [ ] 02-07-PLAN.md — fast-follow modes: reuse-existing-key (IDENT-02), add-account/alias (IDENT-06), key rotation (KEY-01)

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

**Plans**: TBD

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

**Plans**: TBD
**UI hint**: yes

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

**Plans:** 3/7 plans executed

Plans:

- [ ] TBD (deferred — run /gsd-plan-phase 6 to break down once Phases 1–5 are done)

---

## Progress Table

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Bootstrap | 3/3 | Complete   | 2026-06-09 |
| 2. First Identity End-to-End | 3/7 | In Progress|  |
| 3. Full Identity CRUD + Multi-Identity | 0/? | Not started | - |
| 4. Doctor | 0/? | Not started | - |
| 5. CLI Surface + TUI | 0/? | Not started | - |
| 6. Linux Cross-Platform Validation | 0/? | Deferred (post-v1) | - |
