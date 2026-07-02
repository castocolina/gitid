# Phase 2: First Identity End-to-End - Context

**Gathered:** 2026-06-09
**Status:** Ready for planning

<domain>
## Phase Boundary

Phase 2 delivers the **first vertical slice** of real `gitid` domain logic: a user
creates **one** identity and the tool produces the four coordinated artifacts —
a managed SSH `Host` block, a `~/.gitconfig` `includeIf` block, a per-identity
fragment in `~/.gitconfig.d/`, and an `~/.ssh/allowed_signers` line — written
**safely** (timestamped backup → atomic write → idempotent whole-block rewrite →
explicit confirmation → correct permissions). Correctness is proven by the
two-phase test flow (pre-write `ssh -i`, post-write `ssh -T <alias>` + `ssh -G`),
each printing the command and its real output. The public key is copied to the
clipboard, and GitHub/GitLab upload steps (auth + signing) are shown.

**Phase is in MVP mode** (`**Mode:** mvp` in ROADMAP.md): plans are vertical
slices, not horizontal layers. The first slice proves **create-new identity
end-to-end**; reuse-existing-key and account/alias modes are fast-follow plans
within this same phase.

**In scope (Phase 2 requirements):** IDENT-01, IDENT-02, IDENT-06, KEY-01, KEY-02,
SSH-01, SSH-02, SSH-03, GIT-01, GIT-02, GIT-03, SIGN-01, SIGN-02, TEST-01, TEST-02,
TEST-03, SAFE-01, SAFE-02, SAFE-03, CLIP-01, CLIP-02, UP-01, UP-02.

**Out of scope:** full CLI surface + shell completion and the TUI (Phase 5 —
Phase 2 wires only a minimal real `gitid identity add`); list/update/delete and
managed-block reconstruction on startup (IDENT-03/04/05/07, Phase 3); the doctor
health checks (Phase 4 — except the narrow key-generation install hint noted in
D-14); `insteadOf` URL rewriting and `add repo` (v2).

</domain>

<decisions>
## Implementation Decisions

### Two-phase test flow & new-key gate
- **D-01:** The pre-write test (`ssh -i <key> -o IdentitiesOnly=yes -T git@<host>`)
  is classified primarily by its **output string**, with the **exit code as a
  corroborating secondary signal** (ssh -T exits non-zero even on success, so exit
  code alone is unreliable):
  - output contains `successfully authenticated` → **PASS** (key already authorized,
    e.g. reused/already-uploaded key)
  - output contains `Permission denied (publickey)` → **REACHABLE-BUT-NOT-UPLOADED**
    (expected for a brand-new key; the key is valid and the host reachable)
  - connection refused / DNS / timeout (and corroborating exit code) → **FAILURE**
- **D-02:** On the `REACHABLE-BUT-NOT-UPLOADED` (new-key) result, the create flow
  **proceeds to write** the four artifacts (after backup + confirmation), copies the
  `.pub` to the clipboard, shows the upload steps, and then runs the resolved
  **phase-2 test** (`ssh -T git@<alias>` + `ssh -G <alias>`) for the user to confirm
  after uploading. This honors "prove the key is valid and the host is reachable
  before writing" without an impossible pre-authorization requirement for a new key.
- **D-03:** The post-write resolved test parses `ssh -G <alias>` output to assert the
  config actually resolved the expected `identityfile`, `identitiesonly yes`,
  `user git`, `hostname`, and `port` (Success Criterion 1). Both phases print input
  (command) and real output (TEST-03).

### Phase 2 user entry point
- **D-04:** The user drives create via a **real, minimal `gitid identity add` Cobra
  command** (plus an `identity test` path as needed). It is the foundation the
  Phase 5 CLI builds on — **not** throwaway code or a temporary harness.
- **D-05:** Inputs (identity name, git name, git email, provider, host binding,
  match dir) are gathered via **interactive prompts** with sensible defaults shown.

### Key generation, naming, passphrase, algorithm
- **D-06:** Key filename encodes the algorithm actually used: `~/.ssh/id_<algo>_<identity>`
  (normally `~/.ssh/id_ed25519_<identity>`), `.pub` alongside.
- **D-07:** Passphrase policy is **optional — prompt, allow empty**. With the macOS
  `UseKeychain yes` block the passphrase is stored in Keychain, keeping this
  low-friction while letting security-conscious users set one.
- **D-08:** On generate, the key is **loaded into the agent / Keychain immediately**
  via `ssh-add` (macOS: `ssh-add --apple-use-keychain`), matching the `Host *`
  `AddKeysToAgent yes` / `UseKeychain yes` block.
- **D-09:** **Algorithm capability probe.** gitid probes what the local `ssh-keygen`
  supports (e.g. `ssh-keygen -Q key` / known-types) before generating. **ed25519
  remains the default and only normal path.** If ed25519 is unavailable, gitid
  **warns** and offers the **best available fallback** in priority order
  **ed25519 → rsa-4096 → ecdsa** (single algorithm offered, not a free picker).
  This is a **narrow, deliberate refinement** of the locked `PROJECT.md` "ed25519-only"
  decision — ed25519 stays the standard; the probe just prevents a hard failure on
  a toolchain without it. (Not the full per-identity key-type field — that stays future.)
- **D-14:** **No acceptable algorithm available.** If the probe finds the local
  toolchain cannot produce *any* of the fallback-chain algorithms, gitid **stops**
  and shows **per-OS install/upgrade guidance** — a link to the OpenSSH project plus
  concrete steps for the detected platform (macOS: `brew install openssh`; Linux:
  `apt` / `dnf` / `pacman`) — instead of failing opaquely. This is a **Phase-2
  mini-version of DOC-01** (the doctor's per-OS dependency hints, formally Phase 4);
  build it on the shared `platform` / `deps` package seam so Phase 4 generalizes it
  without duplicate logic.

### Create modes (IDENT-01/02/06)
- **D-10:** The create flow offers **three modes**, user-chosen at start:
  1. **Create new identity + new key** (IDENT-01) — generate a fresh key. *Most
     secure; recommended default.*
  2. **Reuse an existing key** (IDENT-02) — point a new identity at an existing key
     file instead of generating one.
  3. **Add an account/alias for an existing identity** (IDENT-06) — map an already-
     created identity to a (new) provider via a host alias so several identities can
     share one provider.
- **D-11:** **MVP sequencing.** The first slice proves **mode #1 (create-new)
  end-to-end first**; modes #2 and #3 are **fast-follow plans within Phase 2**, not a
  separate phase.

### SSH host binding, alias form, match strategy
- **D-12:** **Host binding is user-chosen** at create time, offering both an **alias**
  and the **real provider host**, with **alias pre-selected as the recommended
  default** (uniform behavior, explicit `IdentityFile` + `IdentitiesOnly yes`, scales
  to multiple identities per provider). User may override to claim the real host.
  Alias form: **`<identity>.<provider>`** (e.g. `work.github.com`).
- **D-13:** **Match strategy** default is **`gitdir:~/git/<identity>/`** (with trailing
  slash, matching the `~/git/<client>/` layout). **`hasconfig:remote.*.url:...`** is
  also **selectable in Phase 2** (GIT-02 fully covered — both strategies renderable
  and combinable per account).

### Claude's Discretion (defaults applied unless planning surfaces a conflict)
- **Confirmation / preview UX:** show a **unified preview of all four artifacts**
  (diff against current state for files that exist) followed by a **single explicit
  confirmation**; support a **dry-run** that previews without writing (SAFE-03).
- **`allowed_signers` wiring:** file at `~/.ssh/allowed_signers`; wire
  `gpg.ssh.allowedSignersFile` **globally** in `~/.gitconfig` (managed block); the
  per-identity fragment sets `gpg.format=ssh`, `user.signingkey` (path, not inline —
  SIGN-02), `commit.gpgsign true` (GIT-03). `allowed_signers` line email is
  byte-identical to `user.email` (SIGN-01).
- **Backup format:** `<file>.bak.<timestamp>` (e.g. `~/.ssh/config.bak.<ts>`)
  per SAFE-01.
- **Managed-block sentinels:** `# BEGIN gitid managed: <identity-name>` …
  `# END gitid managed: <identity-name>` per CLAUDE.md; idempotent whole-block
  rewrite, content outside blocks preserved verbatim (SAFE-02).
- **gitconfig writes:** `git config`-via-`os/exec` for key/value sets; raw
  sentinel-delimited text for `includeIf` headers (git cannot write `includeIf`
  natively) per CLAUDE.md.
- **SSH config parse/render:** `kevinburke/ssh_config`; validate post-write with a
  second decode pass (parse → render → parse stability).
- Upload-instruction copy detail (exact GitHub/GitLab steps for auth vs signing keys,
  UP-01/UP-02) — planner's wording.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Target artifact structure (the end-state shape to produce)
- `.planning/references/target-sshconfig.md` — target `~/.ssh/config` structure:
  global keychain/agent block, per-account host aliases, Port 443 + `ssh.`/`altssh.`
  hostnames, explicit `IdentityFile` + `IdentitiesOnly yes`. (Reference uses RSA; PRD
  supersedes with ed25519.)
- `.planning/references/target-gitconfig.md` — target `~/.gitconfig` structure:
  `include` + `includeIf` with both `gitdir:` and `hasconfig:remote.*.url:` strategies,
  per-identity fragment referencing. (`insteadOf` shown is v2.)
- `.planning/references/legacy-ssh-keygen.md` — the legacy Bash script `gitid` replaces
  (what NOT to do: blind append, RSA-only, no coherence).

### Project intent, scope & locked decisions
- `.planning/PROJECT.md` — Core Value, Constraints (Safety, Engineering, Signing,
  Platform), Key Decisions table. Note D-09/D-14 here are a **narrow refinement** of
  PROJECT.md's "ed25519-only" decision (ed25519 stays default; probe+fallback added).
- `.planning/REQUIREMENTS.md` §IDENT/KEY/SSH/GIT/SIGN/TEST/SAFE/CLIP/UP — the Phase 2
  requirement IDs, Acceptance Criteria, and Definition of Done.
- `.planning/ROADMAP.md` §"Phase 2: First Identity End-to-End" — goal + 5 success
  criteria + MVP mode flag.
- `CLAUDE.md` — working method (hypothesis → test → implement; test→confirm+backup→
  re-test for config), safe-write rules, managed-block sentinel format, `git config`-
  via-exec strategy, English-only, TDD.

### Architecture & pitfalls (for package layout and safe-write correctness)
- `.planning/research/ARCHITECTURE.md` — `cmd/` / `internal/` / `tui/` layout; the
  `identity`, `sshconfig`, `gitconfig`, `keygen`, `tester`, `clipboard`, `filewriter`,
  `platform`, `deps` package seams Phase 2 fills in.
- `.planning/research/PITFALLS.md` — atomic-write + permission pitfalls the
  `filewriter` package must honor (`~/.ssh` 700, key 600, `.pub` 644, `config` 600).
- `.planning/research/STACK.md` — pinned versions / import paths: `golang.org/x/crypto/ssh`
  (ed25519 gen + OpenSSH serialization), `kevinburke/ssh_config`, `atotto/clipboard`,
  `spf13/cobra`.
- `.planning/research/FEATURES.md` — feature decomposition relevant to the slice.

### Prior phase (carry-forward)
- `.planning/phases/01-bootstrap/01-CONTEXT.md` — package skeleton (D-09 there),
  TDD harness, lint/security gates Phase 2 code must pass.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- Phase 1 scaffolded empty `internal/` packages with `doc.go` + passing stub tests:
  `identity`, `sshconfig`, `gitconfig`, `keygen`, `tester`, `clipboard`, `filewriter`,
  `platform`, `deps`, `doctor`, plus `cmd/gitid/main.go` and `tui/`. Phase 2 fills the
  real logic into these seams (no new top-level packages needed).
- `internal/filewriter` is the safe-write seam (backup → temp → rename → chmod);
  build the SAFE-01/02/03 + KEY-02 mechanics there so SSH and gitconfig writers reuse it.
- `internal/platform` / `internal/deps` is the seam for OS detection and per-OS install
  hints (D-14's mini-DOC-01 lives here; Phase 4 doctor generalizes it).

### Established Patterns
- Core is built **test-first (TDD)**; config parse → render → parse must be round-trip
  stable (TOOL-04). `make test` / `make lint` (golangci-lint + gosec, hard-fail) gate
  every commit via pre-commit; pre-push runs `go test -race` + coverage.
- All generated content (code, comments, UI/log/error messages, commits, docs) in English.
- Commits go to `main` (no remote); history squashed/compacted at each plan close + review.
- `gsd-tools.cjs` requires Volta's bin on PATH in non-interactive shells (scripted GSD
  calls only — not the Go build).

### Integration Points
- `gitid identity add` (Cobra, `cmd/gitid`) orchestrates: `keygen` (probe + generate) →
  `clipboard` (copy .pub) → `tester` (pre-write `ssh -i`) → preview/confirm →
  `sshconfig` + `gitconfig` + `filewriter` (four artifacts) → `tester` (resolved
  `ssh -T` + `ssh -G`) → upload instructions.

</code_context>

<specifics>
## Specific Ideas

- Pre-write test command form: `ssh -i <key> -o IdentitiesOnly=yes -T git@<host>`,
  classified by output string (D-01).
- Resolved test: `ssh -T git@<alias>` + `ssh -G <alias>`, asserting resolved
  `identityfile` / `identitiesonly yes` / `user git` / `hostname` / `port` (D-03).
- Algorithm fallback chain: **ed25519 → rsa-4096 → ecdsa**; probe via
  `ssh-keygen -Q key` (or equivalent known-types check) (D-09).
- Alias form `<identity>.<provider>` (e.g. `work.github.com`); match
  `gitdir:~/git/<identity>/` with trailing slash (D-12, D-13).
- macOS `Host *` block: `IgnoreUnknown UseKeychain` → `UseKeychain yes` +
  `AddKeysToAgent yes`, ordered after specific hosts (SSH-03).

</specifics>

<deferred>
## Deferred Ideas

- **Full algorithm picker / per-identity key-type field** — D-09 deliberately keeps
  Phase 2 to ed25519-default + single-fallback. A free multi-algorithm picker (and the
  per-identity key-type field anticipated in PROJECT.md's Azure DevOps note) is a later
  consideration, not Phase 2.
- **Full doctor health checks (DOC-01..07)** — Phase 4. D-14 ships only the narrow
  key-generation install hint now; the general per-OS dependency/permission/drift/orphan
  checks stay in Phase 4 and reuse the same `platform`/`deps` seam.
- **List / update / delete / startup reconstruction (IDENT-03/04/05/07)** — Phase 3.
- **`insteadOf` URL rewriting, `add repo`, global config toggles, adopt fragments** — v2.

None of these expand Phase 2 scope — discussion stayed within the phase boundary.

</deferred>

---

*Phase: 2-first-identity-end-to-end*
*Context gathered: 2026-06-09*
