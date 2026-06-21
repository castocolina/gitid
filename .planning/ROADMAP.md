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
- [x] **Phase 4: Doctor** — Deep health checks (deps, permissions, coherence/drift, orphans, signing wiring, agent) with severity + fix; `gitid doctor` CLI command (completed 2026-06-12 — 5 plans + 2 gap-closure plans 04-06/04-07; initial verification found 3 critical wiring gaps DOC-GAP-01/02/03, all closed and re-verified passed, see 04-VERIFICATION.md)
- [x] **Phase 5: CLI Surface + TUI** — Full Cobra command surface with shell completion; Bubble Tea TUI launching to doctor dashboard with identity/account navigation (built 2026-06-13 — ⚠️ guided UAT found core + UX gaps, see VERIFICATION-PLAYBOOK.md; superseded by Phase 5.5 reconciliation + Phase 5.6 TUI rebuild)
- [x] **Phase 5.5: Core & CLI Reconciliation** *(INSERTED — 2026-06-13)* — Fix the core/CLI defects the playbook surfaced: auth-gated create-flow (generate→upload→loop-test until PASS→persist-only-after-PASS, with skip escape), correct provider reconstruction, install PATH feedback, per-URL (`hasconfig`) matching alongside `gitdir`, and a doctor check for overlapping/ambiguous matches. CLI/core only — no TUI cosmetics. (completed 2026-06-14)
- [x] **Phase 5.6: Integrated TUI App** *(INSERTED — 2026-06-13)* — Replace the thin doctor-dashboard TUI with one integrated terminal app (Bubble Tea v2): persistent header + identity sidebar + master-detail pane + bold footer; view switcher (Identities · Health · Global Options); in-app create/add wizard with live feedback (the proven create-flow); per-site + global options editable in-pane; delete/rotate in-app; copy = public key only. Ergonomics modeled on `../tools-installer`. (completed 2026-06-21)
- [x] **Phase 5.7: Complete v1.0 Product Features in TUI** *(INSERTED — 2026-06-21)* — Close the four genuinely-unbuilt whole-product v1.0 gaps, reachable from both the integrated TUI and the CLI (D-10 parity): finish the deferred 5.6 D-06 match-strategy selector (gitdir default + hasconfig/both, live includeIf preview), adopt/migrate plain-style `~/.gitconfig_<name>` fragments into `~/.gitconfig.d/` (ADOPT-01), `gitid add repo <url>` clone+verify-pull workflow (REPO-01), and assisted `gh`/`glab` key upload (AUTOUP-01). Global toggles / `insteadOf` / alt-SSH 443 / completions are already shipped (Phase 3.1 + 5) and OUT of scope per D-01. Core stays UI-free + TDD; every feature gets unit-TDD + real-entry e2e; `recipes/` is the canonical end state. (see `docs/prds/ssh-git-identity-manager-v1.0-prd.md`) (completed 2026-06-21)
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
- [x] 04-03-PLAN.md — Coherence + Orphans families: existence/resolution + locked-value carve-outs; block-vs-disk orphans + unused-key (DOC-03, DOC-04)
- [x] 04-04-PLAN.md — Signing + Agent families: ssh-add probe + fingerprint match + git<2.36 hasconfig: gate (DOC-05)

**Wave 3** *(blocked on Waves 1-2 — shares cmd/gitid/doctor.go)*

- [x] 04-05-PLAN.md — Auto-fix slice: D-04 gate/per-finding-confirm/--yes flow + permission batching; fixes routed through filewriter chokepoint (DOC-06)

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

**Plans**: 4 plans
**UI hint**: yes

Plans:

**Wave 1**

- [x] 05-01-PLAN.md — Foundation: add charm.land/*/v2 deps; extract internal/upload.Instructions + identity.ValidateName; TUI skeleton (tui.Run, root view-stack model, keymap, lipgloss v2 styles, messages, deps builder)

**Wave 2** *(blocked on Wave 1 — 05-02 and 05-03 run in parallel; cmd/gitid vs tui/ have zero file overlap)*

- [x] 05-02-PLAN.md — CLI surface: no-args TTY→TUI branch + top-level rotate/copy/host-add aliases + identity copy; copy command (clipboard + upload instructions); bash/zsh/fish completion tests (CLI-01, CLI-02)
- [x] 05-03-PLAN.md — TUI dashboard slice: async per-family doctor streaming (runID stale-guard) + lipgloss finding render; Enter→Identity list (bubbles list) → Esc pop (TUI-01, TUI-02)

**Wave 3** *(blocked on Wave 2 — shares tui/model.go + tui/identitylist.go with 05-03)*

- [x] 05-04-PLAN.md — TUI forms slice: identity detail + Create/Update/Add-account forms + inline Copy action + shared Prove-Before-Write screen (two-phase async test, confirm gate, write via identity.Deps) (TUI-02, D-02/D-03/D-04/D-06)

### Phase 5.5: Core & CLI Reconciliation (INSERTED)

**Goal**: The defects the guided UAT surfaced in the CLI/core are fixed so the
engine is trustworthy before the TUI is rebuilt on top of it. Create no longer
persists artifacts for an un-authenticated key; reconstruction reports the correct
provider; install tells the user where the binary went and whether it is on PATH;
identities can match by repository URL (`hasconfig:remote.*.url`) as well as by
folder (`gitdir`); and doctor detects ambiguous/overlapping matches.
**Mode:** mvp (reconciliation — derived from VERIFICATION-PLAYBOOK.md findings)
**Depends on**: Phase 5 (built surface)
**Source**: `.planning/phases/05-cli-surface-tui/VERIFICATION-PLAYBOOK.md` (G-05, F-1, F-2, F-3, F-5, F-6, F-7)
**Requirements**: FIX-CREATE-01, FIX-RECON-01, FIX-INSTALL-01, MATCH-URL-01, DOC-08 (new, UAT-derived)
**Success Criteria** (what must be TRUE):

  1. Creating a new identity with an un-uploaded key does NOT persist any artifact; after the key is uploaded and the connectivity test authenticates (PASS), it persists; the flow loops test → retry / skip / quit, and a skip escape persists only on an explicit confirm (G-05/F-2)
  2. `gitid identity add` prompts for a match strategy — folder (`gitdir`), repository URL (`hasconfig:remote.*.url`), or both — and writes the chosen condition(s); the prompt labels the field "Host alias" (F-5/F-6)
  3. `gitid identity list` reconstructs and displays the correct `provider` (e.g. `github`) even when the host-alias does not follow `name.provider` (F-3)
  4. `make install` reports the install path and whether it is on `PATH`, with a hint when it is not (F-1)
  5. `gitid doctor` reports an error/warning when two identities' match conditions overlap (e.g. both match the same `gitdir`), with guidance; the detection is shared so `add`/`update` can warn at write time (F-7)

**Plans**: 7 plans (7 waves)
**UI hint**: no (CLI/core only; TUI cosmetics deferred to Phase 5.6)

Plans:

**Wave 1**

- [x] 05.5-01-PLAN.md — Wave-0 test infra: hermetic E2E harness (fake-ssh on PATH + sandbox HOME), RED E2E shells for all 5 reqs, A4 marker-stability test, `make test-e2e` (D-18/D-20)

**Wave 2** *(blocked on Wave 1)*

- [x] 05.5-02-PLAN.md — Provider marker write/read + reconstruction: `# gitid: provider=` marker, hostname-map fallback, remove TrimPrefix derivation (FIX-RECON-01, D-11/D-12/D-13)

**Wave 3** *(blocked on Wave 2 — needs RenderHostBlock provider arg)*

- [x] 05.5-03-PLAN.md — Auth-gated create-flow: generate key to ~/.ssh first, loop test→PASS→persist (retry/skip/quit), PersistAll split, drop Confirmed (FIX-CREATE-01, D-01..D-06)

**Wave 4** *(blocked on Wave 3 — shares cmd/gitid/add.go)*

- [x] 05.5-04-PLAN.md — Match-strategy picker (gitdir/url/both) on add+update + `--name/--gitdir/--url/--provider` flags + "Host alias" label (MATCH-URL-01, D-07..D-10)

**Wave 5** *(blocked on Wave 4 — shares cmd/gitid/add.go+update.go)*

- [x] 05.5-05-PLAN.md — Shared overlap detector (DetectOverlaps/CheckOverlap) + FamilyOverlap doctor wiring + add/update write-time warn+y/N (DOC-08, D-14..D-16)

**Wave 6** *(blocked on Waves 1+5 — shares Makefile + cmd/gitid/doctor.go)*

- [x] 05.5-06-PLAN.md — Install path + PATH feedback: `platform.BinaryInstallInfo` self-report via doctor + Makefile install echo (FIX-INSTALL-01, D-17)

**Wave 7** *(blocked on Waves 1-6)*

- [x] 05.5-07-PLAN.md — E2E green across all 5 reqs (D-18 completion gate) + agent-driven experience evaluation report (D-19, advisory → Phase 5.6)

### Phase 5.6: Integrated TUI App (INSERTED)

**Goal**: One integrated terminal application (Bubble Tea v2 + Lipgloss v2 + Bubbles
v2) replaces the thin doctor-dashboard TUI. It presents a persistent layout — header

+ left identity sidebar + master-detail main pane + bold footer — with a view switcher

across Identities, Health (doctor), and Global Options (baseline). Creating/adding an
identity, editing per-site and global options, copying the public key, and deleting or
rotating are all done in-app with live feedback; nothing hands off to the CLI.
Ergonomics are modeled on `../tools-installer` (a Textual app). Runs over SSH.
**Mode:** mvp (vertical: build the app shell first, then fold each capability in)
**Depends on**: Phase 5.5 (reuses the proven create-flow, provider/match reconstruction, and doctor overlap detection)
**Source**: `.planning/phases/05-cli-surface-tui/VERIFICATION-PLAYBOOK.md` (Target TUI App; G-01, G-02, G-03, G-04, F-4, F-8)
**Requirements**: TUI-03, TUI-04, TUI-05, TUI-06 (new, UAT-derived); supersedes the Phase 5 TUI-01/TUI-02 surface
**Success Criteria** (what must be TRUE):

  1. Launching `gitid` shows identities in a sidebar (each row labeled name · provider · alias/site count — closes F-4); selecting one fills a detail pane (fields, aliases/sites, match conditions, signing, health badges) without the list disappearing
  2. A view switcher (command palette + `1..N` keys) moves between Identities, Health (doctor findings as badges + actionable fixes), and Global Options (baseline)
  3. The in-app create/add wizard runs keygen → copy public key + upload instructions → live connectivity test → PASS/fail → retry / skip / quit, persisting only after PASS (the Phase 5.5 flow, surfaced in the UI)
  4. Per-site options (match strategy gitdir/URL/both, signing, port, hostname) and global baseline options are editable in-pane
  5. Delete and rotate are performed in-app (no CLI handoff); copy copies the public key only (never the private key)
  6. The footer shows bold, comma-separated key hints; empty-state and list rows render correctly (closes G-01/G-02/G-04/F-4); `?` opens an in-app help overlay (closes G-03)

**Plans**: 6 plans (6 waves: Wave 0 foundation → vertical slices)
**UI hint**: yes

Plans:

**Wave 0**

- [x] 05.6-01-PLAN.md — Foundation: PlaceOverlay spike (ABSENT at v2.0.3 → line-merge overlay fallback) + extend styles/keymap/messages + RED test scaffolds

**Wave 1** *(blocked on Wave 0)*

- [x] 05.6-02-PLAN.md — App shell slice: two-pane persistent layout (sidebar managed+unmanaged, always visible) + 1/2/3 + Ctrl+P view switch + footer + help overlay, via the real `gitid` no-args entry (TUI-03/04/06)

**Wave 2** *(blocked on Wave 1 — shares tui/model.go)*

- [x] 05.6-03-PLAN.md — Health view slice: async 8-family streaming (port dashboard) + distinct severity glyphs (warning `!` vs error `✗`) + per-identity badges + in-app fix confirm (TUI-04/06)

**Wave 3** *(blocked on Wave 2 — shares tui/model.go)*

- [x] 05.6-04-PLAN.md — Detail + editing slice: master-detail pane + inline edit + match-strategy live includeIf preview + structural prove-before-write gate + Global Options view (TUI-04/05)

**Wave 4** *(blocked on Wave 3 — shares tui/wizard.go + model.go)*

- [x] 05.6-05-PLAN.md — Create/add wizard slice: form→keygen→upload→test-loop→write (persist only after PASS) + copy-pubkey-only modal (TUI-05/06)

**Wave 5** *(blocked on Wave 4 — shares tui/deps.go + model.go; ends with blocking D-16 manual smoke)*

- [ ] 05.6-06-PLAN.md — Delete/rotate slice: wire identity.DeleteDeps + in-app delete/rotate confirms + unmanaged affordances (reveal/copy/open) + TestBuildTUIDepsNilGuard + manual real-entry smoke (TUI-04/06, D-16)

### Phase 5.7: Complete v1.0 Product Features in TUI (INSERTED)

**Goal**: Bring gitid from the shipped identity-lifecycle MVP up to the full
whole-product v1.0 PRD by closing the FOUR genuinely-unbuilt gaps, each reachable
from both the integrated two-pane TUI and the CLI (D-10 parity): (1) finish the
deferred Phase 5.6 D-06 match-strategy selector — keep `gitdir` as the default but
expose `hasconfig`/`both` with a live `includeIf` preview (D-02/D-03); (2) ADOPT-01
— migrate/reference plain-style `~/.gitconfig_<name>` fragments into
`~/.gitconfig.d/`, never destructively (D-04/D-05); (3) REPO-01 — `gitid add repo
<url>` provider-detect → client picker → alias rewrite → clone → verify-pull
(D-07/D-08/D-09); (4) AUTOUP-01 — assisted `gh`/`glab` key upload, detect+prompt,
never auto (D-11/D-12). Global toggles / `insteadOf` / alt-SSH 443 / completions
are already shipped (Phase 3.1 + 5) and OUT of scope (D-01). Core stays UI-free +
TDD; every feature gets unit-TDD + real-entry e2e (D-13); `recipes/` is canonical.

**Depends on**: Phase 5.6 (integrated TUI app)

**Source PRD**: `docs/prds/ssh-git-identity-manager-v1.0-prd.md` (§4.6–4.8, §7 Phase 2/3)

**Requirements**: ADOPT-01 (fragment adoption), REPO-01 (`add repo`), AUTOUP-01
(assisted key upload), plus the deferred 5.6 D-06 match-strategy selector
completion (mapped to GIT-02 — gitdir/hasconfig combinable). GLOBAL-01 / URLRW-01 /
CLI-02 are COMPLETE and explicitly NOT replanned (D-01).

**Success Criteria** (what must be TRUE):

  1. The TUI create wizard exposes gitdir (default), hasconfig, and both as selectable
     match strategies with a live `includeIf` preview; `hasconfig` auto-derives as
     `git@<alias>:*/**` (recipe form), editable before write; `both` writes two
     OR-applied `includeIf` blocks; `--match gitdir|hasconfig|both` mirrors on the CLI

  2. `gitid adopt <path>` and the TUI unmanaged-pane Adopt affordance migrate (default)
     or reference a plain-style fragment; migrate copies into `~/.gitconfig.d/<name>`
     and repoints the managed `includeIf`, never deleting the original (removal is a
     separate explicit confirm)

  3. `gitid add repo <url>` (and the TUI Add Repo modal) detect the provider, rewrite
     the URL to the matching alias, clone into `~/git/<client>/<repo>`, and verify with
     `git -C <dest> pull` — both clone and pull output shown; no-match launches inline
     create then resumes the clone (continuous, no abort)

  4. When `gh`/`glab` is present and authenticated, the copy modal + wizard step 3 +
     `gitid identity copy --upload-keys` offer per-key (auth + signing) upload with
     explicit confirmation; the shown command equals the command run; absent/unauthenticated
     falls back to manual instructions and never gates create/copy

  5. Every feature has unit-TDD coverage AND at least one e2e test that drives the real
     `gitid` binary in a sandbox HOME (clone against a local bare remote; PATH-stubbed
     fake gh/glab) — closing the recurring injected-seam blindspot (D-13/D-16)

**Out of scope**: Global toggles / `insteadOf` / alt-SSH 443 / completions (already
shipped — D-01); configurable clone base dir beyond `~/git`; interactive gh/glab login;
Windows, GPG signing, web UI, automatic key rotation, secret-vault (v1 non-goals).

**Plans**: 13 plans (8 feature plans + 5 gap-closure plans 09–13 from 05.7-UAT.md — Wave 0 test infra → 3 parallel core packages → CLI+wiring & match selector → TUI modals → review gates → core split + doctor (W5) → staged wizard SCREEN 1 (W6) → SCREEN 2 (W7) → SCREENS 3+4 (W8))
**UI hint**: yes

Plans:

**Wave 0**

- [x] 05.7-01-PLAN.md — Test infra: extend e2e harness (FakeGHDir/FakeGLabDir/FakeGitDir + local bare remote), RED e2e for adopt/addrepo/upload, RED `TestBuildTUIDepsNilGuard_Phase57`, RED unit stubs + new package dirs (D-13/D-16)

**Wave 1** *(blocked on Wave 0 — three new packages, zero file overlap, parallel)*

- [x] 05.7-02-PLAN.md — `internal/adopter` (TDD): Adopt migrate/reference (never-delete); ListCandidates built FROM SCRATCH via filepath.Glob(~/.gitconfig_*) — NOT doctor.CheckOrphans (corrected premise); best-effort name matching (ADOPT-01, D-04/D-05/D-06)
- [x] 05.7-03-PLAN.md — `internal/repoclone` (TDD): ProviderFromURL/RewriteToAlias/DestPath + Clone/Pull seam with dest-exists + dest-under-base guard (base = deps.UserHomeDir()+/git), arg-slice exec (REPO-01, D-09)
- [x] 05.7-04-PLAN.md — `internal/uploader` (TDD): Detect (gh/glab + auth status) + UploadKey/CommandPreview arg-slice, detect+prompt only (AUTOUP-01, D-11/D-12)

**Wave 2** *(blocked on Wave 1 — 05 and 06 parallel; match-selector files vs adopt/addrepo/copy/deps files, zero overlap)*

- [x] 05.7-05-PLAN.md — Match-strategy selector: turn the match e2e GREEN via the EXISTING stdin interactive picker (gatherCreateInput already calls promptMatchStrategy) + TUI wizard gitdir/hasconfig/both with live includeIf preview + an ADDITIONAL --match CLI flag for non-interactive parity (GIT-02, D-02/D-03/D-10; completes 5.6 D-06)
- [x] 05.7-06-PLAN.md — CLI commands + live deps wiring: `gitid adopt` (root), `gitid add repo` (under a NEW top-level `add` group — none exists today), `gitid identity copy --upload-keys`; add adopt/repoclone/uploader fields to tuiDeps in tui/model.go + widen buildTUIDeps to 8-value + rewire nil-guard GREEN; adopt/addrepo/upload e2e GREEN (ADOPT-01/REPO-01/AUTOUP-01, D-10/D-13/D-16)

**Wave 3** *(blocked on Waves 2 — shares tui/model.go, reuses wizard for inline-create)*

- [x] 05.7-07-PLAN.md — TUI surfaces: sidebar unmanagedEntry gains a fragment `kind` discriminator + populates ~/.gitconfig_* rows from deps.adopt.ListCandidates FIRST, then the Adopt modal/affordance dispatches on fragment rows; Add Repo modal (detect→picker→rewrite→stream→result + inline-create resume); copy/wizard gh/glab upload-assist (ADOPT-01/REPO-01/AUTOUP-01, D-04..D-12)

**Wave 4** *(blocked on Waves 2-3 — review gates)*

- [x] 05.7-08-PLAN.md — Review wave: agent-ui-ux-designer critique of the 4 new TUI surfaces (teatest-independent View()-dump frame capture fallback) + requesting-code-review over the phase diff + blocking manual TTY smoke test (D-13)

**Wave 5 (gap closure)** *(from 05.7-UAT.md; 09 + 11 parallel — zero file overlap: 09 = internal/identity + cmd/gitid/add.go core refactor, 11 = doctor + cmd + tui/deps/health)*

- [x] 05.7-09-PLAN.md — CORE REFACTOR (UI-free, TDD): centralize the provider→alt-SSH map in identity.DefaultHostname/DefaultPort (one source, CLI+TUI parity, add bitbucket) + SPLIT identity.PersistAll into composable PersistSSH (LEG 1: key + Host block) and PersistGitconfig (LEG 2: fragment + includeIf + allowed_signers); PersistAll kept as a thin composition so the CLI single-shot flow is unchanged; staged-key model preserved (foundation for the staged wizard; G-1/G-2/G-3)
- [x] 05.7-11-PLAN.md — LEG-1 doctor (G-4): new ADVISORY-ONLY FamilyRedundancy check (CheckRedundancy) for multiple `Host *` stanzas + duplicate UseKeychain/AddKeysToAgent/IgnoreUnknown across pre-existing config and managed `_global`; SeverityWarning + Fix nil (never blocks doctor/write); wired into Run/Families + cmd + tui *(UNCHANGED by the wizard restructure)*

**Wave 6 (gap closure)** *(blocked on Wave 5 plan 09 — shares tui/wizard.go; staged wizard SCREEN 1)*

- [x] 05.7-10-PLAN.md — STAGED WIZARD scaffolding + SCREEN 1 (SSH Identity, LEG 1 inputs) + viewport fix: wizardScreen enum (1 SSH / 2 Test / 3 Git / 4 Review); Screen 1 shows name/algo/provider/alias/Hostname/Port/folder ALL editable + always visible (Hostname/Port pre-filled from the alt-SSH helper, editable back to github.com:22) + a LIVE Host-block preview via RenderHostBlock; viewport-aware modal so tall content never silently clamps (structurally dissolves G-1; G-2 preview half)

**Wave 7 (gap closure)** *(blocked on Wave 6 plan 10 — shares tui/wizard.go; staged wizard SCREEN 2)*

- [x] 05.7-12-PLAN.md — STAGED WIZARD SCREEN 2 (SSH Connectivity Test, LEG 1 write): upload-manually→test (no gh/glab auto-upload in wizard) against the STAGED key; FULL command + key path visible on pre-run, SUCCESS, and failure (G-3); on SUCCESS write LEG 1 via identity.PersistSSH (key + Host block, backup + idempotent) then advance; SECONDARY [s] skip-&-write-offline with clear no-write feedback (double-confirm + unauth warning preserved) (G-2)

**Wave 8 (gap closure)** *(blocked on Wave 7 plan 12 — shares tui/wizard.go; staged wizard SCREENS 3+4)*

- [ ] 05.7-13-PLAN.md — STAGED WIZARD SCREEN 3 (Git Configuration, LEG 2) + SCREEN 4 (Review): Screen 3 collects user.name/email + match selector (gitdir/hasconfig/both, editable sub-fields, match panel ALONE so it never overflows) + signing toggle, live includeIf preview, email-validated; on confirm write LEG 2 via identity.PersistGitconfig (fragment + includeIf + allowed_signers); Screen 4 is a read-only review of the SSH block + includeIf + fragment + allowed_signers + live ssh -G resolution (completes G-1/G-2)

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

**Plans:** 12/13 plans executed

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
| 4. Doctor | 7/7 | Complete   | 2026-06-12 |
| 5. CLI Surface + TUI | 4/4 | Built (UAT found gaps → 5.5 + 5.6) | 2026-06-13 |
| 5.5. Core & CLI Reconciliation | 7/7 | Complete    | 2026-06-14 |
| 5.6. Integrated TUI App | 5/7 | Complete    | 2026-06-21 |
| 5.7. Complete v1.0 Product Features in TUI | 12/13 | In Progress|  |
| 6. Linux Cross-Platform Validation | 0/? | Deferred (post-v1) | - |
