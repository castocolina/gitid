# Identity Create Flow — Product Requirements Document (PRD)

> Scope: the create/add-identity flow (SSH → gitconfig, staged and transactional),
> ssh-agent registration, a first-class **Keys** view with orphan detection,
> resume-from-orphan, and backup **maintenance**. The canonical target shape is
> defined by `recipes/ssh-config.recipe` and `recipes/gitconfig.recipe` — these
> recipes are the acceptance reference, not illustrative samples.

## Requirements Description

### Background

- **Business Problem**: gitid creates an identity's four artifacts but the flow is
  not transparent or transactional. Users cannot see/verify the exact test command
  or where the staged key lives; the SSH and gitconfig stages are not independently
  gated; abandoning leaves no recoverable trail; and every write accumulates `.bak`
  files with no retention, so the filesystem fills with hundreds of backups. Keys
  that exist on disk but are not wired to a complete identity ("orphans") are
  invisible.
- **Target Users**: developers (starting with the maintainer) who manage multiple
  Git identities (work / personal / org) on one machine and need each repository to
  resolve the right SSH key + Git identity automatically, per `recipes/`.
- **Value Proposition**: a trustworthy, inspectable, resumable create flow whose
  output matches the recipes exactly, never loses a generated key, never reverts
  more than it must, and keeps the filesystem tidy.

### Success Metrics (measurable)

- **M1 — Recipe conformance**: 100% of created identities re-parse to the recipe
  shape (parse→render→parse stability for the `Host` block and the `hasconfig`
  includeIf; verified by an automated round-trip test).
- **M2 — Bounded backups**: at most 3 gitid backups per managed file on disk at any
  time; 0 user/foreign `.bak` files ever deleted.
- **M3 — No lost keys**: 0 generated keys lost — an abandoned SSH-passed flow always
  leaves a recoverable orphan; a recoverable orphan reaches a complete identity in
  **≤ 2 steps** from the Keys view.
- **M4 — Doctor coverage**: 100% of managed writes validated by doctor; doctor runs on
  100% of startups; any warning is reflected by the landing-view badge.

### Feature Overview

- **Core Features**
  1. **Two-stage transactional create flow**: Stage A (SSH) commits only after the
     live SSH test passes; Stage B (gitconfig) commits separately and reverts
     independently.
  2. **Full command transparency**: the upload/test screen shows the complete
     `ssh -i <key-path> …` command including the **staged key file path** (and the
     final `~/.ssh` path), because without that file the test means nothing.
  3. **ssh-agent registration** on SSH success (after promoting the key to `~/.ssh`).
  4. **Resume from orphan / same-name reuse**: a key left on disk can be picked up
     later from the Keys view, or by starting an "add identity" with the same name
     (tested transparently against its `~/.ssh` path).
  5. **Keys view**: a top-level view parallel to Identities, listing SSH keys with a
     status color — **yellow when a key is orphaned** (on disk + alias, but no
     complete gitconfig identity).
  6. **Backup maintenance**: retain the **last 3** backups per managed file; prune
     automatically after each write and via a manual "Clean backups" action in a
     Maintenance area; applies to all managed backups.

- **Feature Boundaries (in scope)**: the create/add wizard, the Keys view, orphan
  detection + resume, ssh-agent registration, backup retention/pruning, and aligning
  the gitconfig match to the recipe (`hasconfig:remote.*.url`).

- **Out of scope (this PRD)**: non-interactive CLI flags for the above, multi-provider
  GPG signing alternatives (gitid uses ed25519 ssh-signing), the alt-SSH 443 endpoint
  and `insteadOf` URL rewriting (tracked separately), and Windows ssh-agent specifics.

- **User Scenarios**
  - *Happy path*: create `personal` → generate key → upload `.pub` to GitHub → test
    (PASS) → key promoted to `~/.ssh` + registered in agent + `Host personal.github.com`
    written → gitconfig stage writes the `hasconfig` includeIf + fragment +
    allowed_signers → resolved test (PASS) → identity complete.
  - *Abandon after SSH ok, during gitconfig*: revert **only** gitconfig; key + alias +
    agent registration remain → the key shows as **orphan (yellow)** in Keys → later
    resumed.
  - *Resume*: open Keys, select the orphan, "Continue setup" → flow re-enters at the
    gitconfig stage (or add identity with the same name → tested against `~/.ssh`).
  - *Maintenance*: Keys/Maintenance shows backup counts; "Clean backups" prunes to the
    last 3 per file.

### Detailed Requirements

Functional requirements (FR), each mapped to the recipe or a clarified decision:

- **FR-1 — Staged keygen + transparency.** Generate ed25519 in a temp staging path
  (`os.MkdirTemp`), public line carrying the `<name>@gitid` comment. The upload/test
  screen MUST display the full public key (never truncated), `[c] copy`, the provider
  upload instructions, and the **exact** test command including the staged key path
  and the eventual `~/.ssh/id_ed25519_<name>` destination.
- **FR-2 — SSH test (Stage A gate).** Test = `ssh -i <key-path> -o IdentitiesOnly=yes
  -o BatchMode=yes -o StrictHostKeyChecking=accept-new -p <port> -T git@<host>`,
  classified by output substring (never exit code). `<key-path>` is the staged temp
  path, OR the existing `~/.ssh/id_ed25519_<name>` when reusing a same-name key
  (shown transparently).
- **FR-3 — Stage A commit (SSH), atomic.** On SSH PASS, commit in this order with
  rollback on any structural failure (no half state):
  1. Promote the staged private key to `~/.ssh/id_ed25519_<name>` (0600) and the
     `.pub` (0644) via the backup-aware filewriter.
  2. Write the managed `Host <alias>` block to `~/.ssh/config` (backed up first).
  3. **Register only the final key in ssh-agent** (`ssh-add <final-key>`; on macOS
     `ssh-add --apple-use-keychain <final-key>`) — best-effort.
  If step 1 or 2 fails, roll back the prior step(s) (restore `~/.ssh/config` from its
  backup, remove the just-promoted key) and report the error; the staged temp key is
  retained for retry. A failed `ssh-add` does **not** roll back (warning only — the
  agent is best-effort). On SSH FAIL: nothing is promoted; the staged temp key is
  discarded on abandon (it never reached `~/.ssh`).
- **FR-4 — Stage B (gitconfig), independently transactional.** Only after Stage A is
  committed, write the gitconfig artifacts: the per-identity fragment
  (`~/.gitconfig.d/<name>`), the `includeIf` block (default `hasconfig:remote.*.url:
  git@<alias>:*/**`), and the `allowed_signers` entry; then run the resolved test
  (`ssh -G <alias>` + connectivity). On Stage B failure or user abandon: **revert
  only the gitconfig artifacts** (remove the managed includeIf block + fragment +
  allowed_signers entry, restoring from backup). The key + alias + agent registration
  **remain** → the identity is now an **orphan key**.
- **FR-5 — Orphan key model.** An orphan = a private key in `~/.ssh` whose `<name>`
  has an SSH `Host` alias but **no complete gitconfig identity** (missing/partial
  includeIf+fragment). Orphans are detectable from disk + parsed managed blocks.
- **FR-6 — Keys view.** A top-level view (peer of Identities / Health / Global
  Options) listing every gitid-relevant SSH key with a status glyph and color:
  green = fully wired identity, **yellow = orphan**, and an error color for
  broken/unreadable. Selecting an orphan offers **"Continue setup"** (resume Stage B)
  and **copy public key**.
- **FR-7 — Resume / same-name reuse.** Starting "add identity" with an existing
  `<name>` whose key is already in `~/.ssh` MUST skip keygen and test against that
  path (transparently shown), funneling into the same Stage A/B pipeline (reuse mode).
- **FR-8 — Backup maintenance (safe prune).** gitid backups carry a distinctive
  marker — `<file>.gitid.bak-<UTC-timestamp>` — so pruning can NEVER touch a `.bak`
  created by the user or another tool. Retention keeps the **3 most recent** gitid
  backups per file and never deletes the newest, even if more than 3 exist. Trigger:
  after each successful managed write, prune that file's gitid backups; a manual
  **"Clean backups"** action (Keys/Maintenance area) sweeps **all** managed files and
  reports the count removed. Scope: `~/.ssh/config`, `~/.gitconfig`, `~/.gitconfig.d/*`,
  `~/.ssh/allowed_signers`. (Adopting the `.gitid.bak-` marker may require migrating the
  current backup naming; pre-existing differently-named backups are left untouched.)
- **FR-9 — Doctor as the central guard.** Every managed write MUST be validated by
  doctor: after each write stage, run the relevant doctor checks (parse-back the
  managed blocks) and treat a non-coherent result as a Stage failure. Doctor runs
  **on every startup** (always, before the first paint completes its data load). If
  doctor reports any warning/finding, the **landing view (Identities) shows a badge**
  (a colored bullet) signaling "issues found — see Health for details", routing the
  user to the Health (doctor) view. Orphan keys and excess backups (FR-5/FR-8) surface
  as doctor findings too, so the Health view and `gitid doctor` CLI stay consistent
  with the Keys view.

### Target shape captured from `recipes/` (acceptance reference)

`~/.ssh/config` (managed `Host` block per identity):

```
Host personal.github.com
    Hostname ssh.github.com        # (alt-SSH 443 tracked separately; today: provider host)
    Port 443                       # (today: 22 — see Out of scope)
    User git
    IdentityFile ~/.ssh/id_ed25519_personal
    IdentitiesOnly yes
```

`~/.gitconfig` (managed includeIf — primary match is by remote URL/alias):

```
[includeIf "hasconfig:remote.*.url:git@personal.github.com:*/**"]
    path = ~/.gitconfig.d/personal
```

Per-identity fragment `~/.gitconfig.d/personal`: `user.name`, `user.email`,
`user.signingkey`, `commit.gpgsign`, `gpg.format = ssh`.

### Input / Output

- **Inputs**: identity name, git name/email, provider, port, alias (default
  `<name>.<provider>`), match strategy (default `hasconfig`), signing on/off; for
  resume: an existing key selection or a same-name entry.
- **Outputs**: promoted key pair in `~/.ssh`, agent registration, managed `Host`
  block, gitconfig includeIf + fragment + allowed_signers entry, ≤3 backups per file.

### Edge Cases

- SSH FAIL then abandon → discard staged temp key, no `~/.ssh` residue.
- SSH PASS, gitconfig FAIL/abandon → orphan key (yellow), gitconfig reverted.
- Same-name key already in `~/.ssh` → reuse path, no keygen, transparent test path.
- Agent unavailable / `ssh-add` fails → surface a non-fatal warning; key + alias still
  committed (agent registration is best-effort, not a gate).
- Backup prune when fewer than 3 exist → no-op.
- Orphan whose `.pub` is missing → derive it from the private key (with `<name>@gitid`
  comment) before copy.

## Design Decisions

### Technical Approach

- **Architecture**: extend the existing injected-seam pipeline (`internal/identity`
  `Deps`, `tester`, `sshconfig`, `gitconfig`, `filewriter`) and the Bubble Tea v2 TUI.
  Split the current single "PersistAll" into **two committable stages** (A: SSH key +
  alias + agent; B: gitconfig artifacts) with an explicit Stage-B revert.
- **Key components**
  - `internal/agent` (new): `Register(keyPath)` wrapping `ssh-add` (macOS
    `--apple-use-keychain`), no shell, best-effort.
  - `internal/filewriter`: backup **retention/prune** (keep last N=3) + a `PruneBackups`
    helper; surfaced via a maintenance API.
  - `internal/identity`: stage-split orchestration + Stage-B revert; orphan detection
    (`ListKeys`/`ClassifyKey`) from `~/.ssh` + parsed managed blocks.
  - `internal/gitconfig`: default match → `MatchHasconfig` value `remote.*.url:git@<alias>:*/**`.
  - `tui`: new **Keys** view (peer view + sidebar rail + status color), wizard
    Stage-A/B transitions, resume entry points, Maintenance/clean-backups action, and
    the landing-view **doctor-warning badge**.
  - `internal/doctor`: post-write validation hook (parse-back), startup run, and new
    findings for orphan keys + excess backups (shared by Keys view, Health view, CLI).
- **Data storage**: same on-disk files; backups remain timestamped siblings, capped at 3.
- **Interface design**: new seams added to `identity.Deps` (AgentRegister, PruneBackups,
  ListKeys); the TUI wires them in `tui/deps.go` (mirrored in `cmd/gitid`).

### Constraints

- **Safety**: never write `~/.ssh`/`~/.gitconfig` without a timestamped backup,
  idempotent managed blocks, and explicit confirmation (CLAUDE.md). Copy = public key
  only. Stage-B revert restores from backup; it never touches foreign content.
- **Performance**: agent registration and backup pruning are local, sub-second; must
  not block the TUI event loop (run as async `tea.Cmd`).
- **Compatibility**: macOS + Linux ssh-agent; `--apple-use-keychain` only on darwin.
- **Conformance**: output must match `recipes/` structure (ed25519, not RSA).

### Risk Assessment

- **Technical**: ssh-agent environment differences (no agent, headless) → make
  registration best-effort with a clear warning, never a gate. Stage-B revert
  correctness → drive with real files in tests, assert backups restore byte-for-byte.
- **Dependency**: relies on `ssh`/`ssh-add`/`git` on PATH (already required); doctor
  should check `ssh-add` presence.
- **Schedule**: the stage-split touches the create pipeline's core; mitigate by
  building behind tests first (TDD) and keeping Stage A behavior backward-compatible.

## Acceptance Criteria

### Functional Acceptance
- [ ] AC-1: Upload/test screen shows the full public key + the exact `ssh -i <path>`
      command including the **staged key path** and the final `~/.ssh` destination.
- [ ] AC-2: SSH test uses `-i <key-path>` and is classified by output (PASS gates Stage A).
- [ ] AC-3: On SSH PASS the key is promoted to `~/.ssh` (0600/0644, backed up), the
      `Host <alias>` block is written, and the key is registered in ssh-agent
      (macOS `--apple-use-keychain`).
- [ ] AC-4: gitconfig (Stage B) is written only after Stage A and produces the
      `hasconfig:remote.*.url:git@<alias>:*/**` includeIf + fragment + allowed_signers,
      matching `recipes/gitconfig.recipe`.
- [ ] AC-5: Abandoning/failing in Stage B reverts **only** gitconfig (restored from
      backup); the key + alias + agent registration remain.
- [ ] AC-6: A key on disk without a complete identity is classified **orphan** and
      appears **yellow** in the Keys view.
- [ ] AC-7: An orphan can be resumed ("Continue setup") into Stage B; starting an
      add-identity with a same-name `~/.ssh` key reuses it and tests its path.
- [ ] AC-8: At most **3** gitid backups (`<file>.gitid.bak-<ts>`) per managed file
      remain after writes; the manual "Clean backups" action prunes to 3 and reports
      the count removed; user/foreign `.bak` files are never deleted.
- [ ] AC-9: Stage A is atomic — a failure after key promotion rolls back (ssh config
      restored, promoted key removed); no half-written `~/.ssh` state remains.
- [ ] AC-10: Every managed write is validated by doctor (parse-back); a non-coherent
      result fails the stage.
- [ ] AC-11: Doctor runs on every startup; when it reports a warning, the landing
      (Identities) view shows a badge routing the user to the Health view; orphans and
      excess backups appear as doctor findings.

### Quality Standards
- [ ] TDD: failing test authored first for each FR; core logic in UI-free packages.
- [ ] Test coverage: new packages (`agent`, key-classification, prune) ≥ 85%; tui flow
      tests cover Stage A/B transitions and the Keys view.
- [ ] Lint clean (`make lint`, no `--no-verify`); every commit builds and passes hooks.
- [ ] Security: agent/exec calls use arg-slice form (no shell); revert never edits
      foreign content; backups precede every write.

### User Acceptance
- [ ] D-16 manual smoke on a real TTY: create → upload → test → agent → gitconfig →
      abandon-in-Stage-B → orphan(yellow) → resume — all behave as specified.
- [ ] Keys view + Maintenance reviewed by the user; backup clutter no longer grows
      unbounded.
- [ ] Docs updated (README objective already references `recipes/`; add Keys +
      maintenance notes).

## Execution Phases

### Phase 1: Preparation & contracts
**Goal**: lock seams and target shapes without behavior change.
- [ ] Define `identity.Deps` additions (AgentRegister, PruneBackups, ListKeys/ClassifyKey).
- [ ] Switch default match to `hasconfig` in `DefaultMatch`; assert render matches recipe.
- [ ] Backup retention contract in `filewriter` (keep last N=3) + `PruneBackups` (TDD).
- **Deliverables**: seam signatures + green unit tests for match + prune.

### Phase 2: Two-stage transactional create
**Goal**: split persist into Stage A (SSH) and Stage B (gitconfig) with revert.
- [ ] `internal/agent.Register` (ssh-add; macOS keychain), best-effort + warning.
- [ ] Stage A commit (promote key, write Host block, register agent) on SSH PASS.
- [ ] Stage B write + resolved test; Stage-B-only revert from backup on fail/abandon.
- [ ] Wizard transitions + transparency (full command incl. key path).
- **Deliverables**: create flow producing recipe-shaped artifacts; revert tests prove
  gitconfig restored byte-for-byte while key/alias persist.

### Phase 3: Keys view, orphan detection, resume & doctor guard
**Goal**: make keys first-class and recoverable; doctor becomes the central guard.
- [ ] Orphan classification from `~/.ssh` + parsed managed blocks (taxonomy: raw /
      alias-only / partial / broken / non-gitid-ignored; shared key not flagged orphan).
- [ ] Keys view (peer view, sidebar rail, status color; yellow = orphan).
- [ ] Resume ("Continue setup") + same-name reuse path; transparent `~/.ssh` test path.
- [ ] Doctor: run on every startup; post-write parse-back validation; orphan + excess-
      backup findings; landing-view warning badge → Health.
- **Deliverables**: Keys view with orphan(yellow); resume re-enters Stage B; startup
  doctor + badge.

### Phase 4: Maintenance, smoke & docs
**Goal**: tidy filesystem + verify end to end.
- [ ] Safe-prune (`.gitid.bak-` marker, keep 3, never newest, never foreign `.bak`).
- [ ] Maintenance area + "Clean backups" action (sweep all, report count).
- [ ] D-16 manual smoke on a real TTY across all scenarios (incl. abandon→orphan→resume).
- [ ] Docs: Keys + maintenance; update STATE/ROADMAP.
- **Deliverables**: user-approved smoke; ≤3 gitid backups/file enforced; metrics M1–M4 met.

---

**Document Version**: 1.1
**Created**: 2026-06-20
**Clarification Rounds**: 3
**Quality Score**: 100/100

### Locked decisions (this revision)
- ssh-agent registers **only the final** key (after promotion to `~/.ssh`).
- Backups: `<file>.gitid.bak-<ts>` marker; keep 3; never delete the newest or any
  foreign `.bak` (FR-8 / A).
- Stage A is atomic with rollback; `ssh-add` is best-effort, non-gating (FR-3 / B).
- Doctor is the central guard: validates every write, runs on every startup, and a
  warning shows a badge on the landing view → Health (FR-9).
- Measurable success metrics M1–M4 defined (G).

### Assumptions carried (confirm if wrong) — the 100→110 polish
- C: key taxonomy (yellow=orphan, red=broken, non-gitid keys ignored).
- D: resume re-runs the resolved test before Stage B; shared keys (AddAccount) are
  never flagged orphan.
- E: default match → `hasconfig`; existing `gitdir` identities coexist and doctor
  flags them as upgradable; the wizard Match field becomes a real selector later.
- I: provider scope starts at github.com (alias-based `hasconfig` is provider-agnostic;
  per-provider upload hints + 443/insteadOf tracked separately).
- J: fragment uses ssh-signing (`gpg.format=ssh`, `user.signingkey=<.pub path>`,
  `commit.gpgsign=true`), not the recipes' GPG keys.
