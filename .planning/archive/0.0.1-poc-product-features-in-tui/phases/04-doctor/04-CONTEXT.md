# Phase 4: Doctor - Context

**Gathered:** 2026-06-11
**Status:** Ready for planning

<domain>
## Phase Boundary

Phase 4 delivers `gitid doctor` — a **read-only health report** over a
gitid-managed environment, spanning six check families:

- **Dependencies (DOC-01):** `ssh`, `ssh-keygen`, `ssh-add`, `git`, clipboard
  tool present, each with a per-OS install hint (brew / apt / dnf / pacman).
- **Permissions (DOC-02):** `~/.ssh` 700, private keys 600, `.pub` 644,
  config files 600.
- **Coherence / drift (DOC-03):** every `IdentityFile` resolves, every
  `includeIf` points to an existing fragment, `IdentitiesOnly yes` is present,
  signing identities have an `allowed_signers` line.
- **Orphans (DOC-04):** artifacts on disk with no owning managed block —
  reported **distinctly** from coherence gaps.
- **Signing wiring + ssh-agent (DOC-05):** `gpg.format=ssh`, `allowed_signers`
  path, agent reachable + managed keys loaded; `git < 2.36` warning when
  `hasconfig:` is used.
- **Findings model (DOC-06):** every finding has severity + plain-English
  explanation + suggested fix; auto-fix offered with confirmation where
  applicable.

DOC-07 ("doctor runs first when the TUI launches, and is available as
`gitid doctor` on the CLI") is split: **Phase 4 ships the CLI command and the
read-only check core**; the TUI dashboard that runs doctor first is **Phase 5**.
The grouped-by-family report layout chosen here is deliberately the seed of that
Phase 5 dashboard.

**Phase is in MVP mode** (`**Mode:** mvp` in ROADMAP.md): plans are vertical
slices that *compose* existing proven primitives (`deps.Detect`,
`platform.InstallHint`, `identity.Reconstruct`, `filewriter.ListBlocks`, the
sshconfig/gitconfig readers) into checks — no new low-level machinery beyond a
finding/severity model and a thin fixer in the cmd layer.

**In scope (Phase 4 requirements):** DOC-01, DOC-02, DOC-03, DOC-04, DOC-05,
DOC-06, DOC-07 (CLI half only).

**Out of scope:**
- **TUI doctor dashboard** (the "runs first in the TUI" half of DOC-07) — Phase 5.
- **Full Cobra surface + shell completion** — Phase 5. Phase 4 wires a minimal
  real `gitid doctor` subcommand following the Phase 2/3 pattern.
- **General content-diff drift** — re-rendering managed blocks and diffing
  against disk is explicitly rejected (D-19); coherence is existence/resolution
  plus a small set of locked-value checks.
- **Confident classification of non-git keys** — gitid cannot know if an
  unreferenced key is used for plain SSH; it stays a `warning` (D-13). Building a
  real non-git key→server map is deferred.
- **New mutation primitives** — all fixes route through the existing
  `internal/filewriter` chokepoint (backup → atomic → idempotent); doctor adds
  no new write path.

</domain>

<decisions>
## Implementation Decisions

### Auto-fix architecture (DOC-06)
- **D-01 (read-only core + decoupled fixer):** `internal/doctor` stays **pure** —
  it returns structured findings and **never writes** (preserving the package
  doc's current contract and full fake-testability). A finding may carry an
  **optional fix descriptor** (e.g. a `Fix` func / fix metadata); the **cmd
  layer** applies it, routing every mutation through `internal/filewriter`
  (backup → atomic → idempotent). Detection and mutation are separate concerns.
- **D-02 (auto-fixable classes):** only three finding types get an auto-fix:
  - **Permissions** — `chmod` to the KEY-02 targets (`~/.ssh` 700, key 600,
    `.pub` 644, config 600). The canonical safe, reversible fix.
  - **Orphaned managed blocks** — remove a gitid-managed block whose counterpart
    is gone (e.g. a fragment with no `includeIf`, an alias Host block with no
    `includeIf`). Routed through `filewriter` block removal.
  - **Missing-wiring re-add** — re-add a missing `allowed_signers` line / missing
    `IdentitiesOnly yes`, reconstructed from other managed blocks.
- **D-03 (everything else report-only):** dependency installs, **key-file
  deletion**, ssh-agent loading, and any user-edited value drift are
  **report-only** — doctor shows the exact suggested command but never runs it.
- **D-04 (CLI trigger + confirm semantics, reconciled with SAFE-03):**
  - `gitid doctor` → pure report; when fixable findings exist, offer a
    **top-level "apply fixes?" gate**; on yes → **per-finding confirm**.
  - `gitid doctor --fix` → **skips the top-level gate**, goes straight to
    **per-finding confirm** for each fixable finding.
  - `gitid doctor --fix --yes` → applies all fixable findings **without prompts**
    (for scripts/CI). **`--yes` IS the explicit SAFE-03 confirmation** — there is
    no silent write path.
  - **Granularity:** permissions may batch under one confirm; **orphaned-block
    removal and wiring re-add confirm individually** (higher blast radius, they
    touch config files). A timestamped backup is taken per mutated file as always.

### Report format & severity (DOC-06 / DOC-07-CLI)
- **D-05 (severity model — 4 levels):** `critical` / `error` / `warning` /
  `info`.
  - `critical` = key/secret exposure (e.g. private key world-readable).
  - `error` = broken (missing required dep, `IdentityFile` that won't resolve,
    auth/config will fail).
  - `warning` = degraded/risky (agent not running, `git < 2.36` with
    `hasconfig:`, locked-value drift like `ignorecase` flipped, an unreferenced
    key).
  - `info` = advisory (optional tool missing).
- **D-06 (layout — grouped by family, show passes):** render in sections —
  Dependencies, Permissions, Coherence, Orphans, Signing, Agent (+ Baseline, see
  D-16). Within each, show **`✓` for passing checks** and the finding for
  failures. This full health-dashboard view is intentionally the seed of the
  Phase 5 TUI dashboard.
- **D-07 (tiered exit codes):** the **highest severity present** sets the code —
  `0` clean / `1` warning+info / `2` error / `3` critical. Anything not-clean is
  non-zero (info alone ⇒ 1), so CI/pre-flight can gate on it.
- **D-08 (color):** color on a TTY (red/yellow/green per severity), auto-plain
  when piped/redirected, and respect the `NO_COLOR` env var.

### Orphan detection (DOC-04)
- **D-09 (orphan = artifact with no owning block):** orphans are the **inverse**
  of Phase 3's "incomplete" marker. *Incomplete* (Phase 3 D-02) = a managed block
  exists but a piece is missing → **Coherence**. *Orphan* = an artifact exists on
  disk but **no managed block claims it** → **Orphans**.
- **D-10 (distinct families):** orphans report under their **own `Orphans`
  family**, distinct from `Coherence` (SC-4 explicitly: "distinct from coherence
  failures"). Not a shared sub-typed "drift" family.
- **D-11 (managed-block orphans are the fixable ones):** a fragment file with no
  `includeIf`, or an alias `Host` block with no matching `includeIf`, is a
  managed-block orphan → auto-fixable removal per D-02.
- **D-12 ("unused key" scope — cross-ref ALL Host blocks):** the unused-key check
  cross-references a private key against **every `Host` block in `~/.ssh/config`,
  gitid-managed AND hand-written**. If any Host's `IdentityFile` references it,
  it is **not** flagged. This respects the user's own SSH config.
- **D-13 (unused key ⇒ `warning` only, honest wording):** a key referenced
  nowhere in `~/.ssh/config` is surfaced at **`warning`** severity — never
  `error`/`critical` — with wording that **admits gitid cannot know it is
  unused**: it may serve direct server SSH or ad-hoc `ssh -i`. Suggested action
  is *review*, not deletion. **No auto-fix for key files** (D-03).
- **D-14 (`known_hosts` correlation — investigated and REJECTED):** reading
  `~/.ssh/known_hosts` to decide whether a key serves a non-git host **does not
  work** and must not be attempted. `known_hosts` stores **server host public
  keys** (for verifying the *server's* identity), not a client-key→host usage
  map; SSH persists no record of which identity key authenticated to which host.
  The only durable signal is `~/.ssh/config` `IdentityFile` per `Host` (already
  used in D-12). Recorded so downstream agents do not retry this dead end.

### Check depth (DOC-03 / DOC-05) + baseline fold-in (Phase 3.1 deferred)
- **D-15 (coherence = existence/resolution only):** the general coherence checks
  verify **existence and resolution**, not values: every `IdentityFile` resolves
  to a real file, every `includeIf` points to an existing fragment,
  `IdentitiesOnly yes` is present, signing identities have an `allowed_signers`
  line. **No full content compare.**
- **D-16 (fold in ALL four Phase 3.1 baseline checks):**
  - **excludesfile wiring** — `core.excludesfile` is set and points to an
    existing `~/.gitignore_global`.
  - **baseline `[include]` resolves** — the managed `[include]` block in
    `~/.gitconfig` points to an existing `~/.gitconfig.d/00-baseline` (orphaned
    include if deleted).
  - **ignorecase drift** — `core.ignorecase=false` as the baseline set it; warn
    if flipped to `true`.
  - **curated excludes present** — the `gitignore` managed block still contains
    its curated entries (`.DS_Store`, `*.log`, …).
  These report under a **Baseline** family (or fold into Coherence/Orphans as the
  planner sees fit, keeping the orphaned-include as an orphan-class finding).
- **D-17 (bounded locked-value carve-outs):** D-15 ("existence only") and the
  value checks (`ignorecase` drift in D-16, plus `gpg.format=ssh` and the
  `allowed_signers` email == `user.email` match) reconcile cleanly: these are a
  **small, fixed set of locked invariants** (Phase 3.1 D-03; Phase 2 SIGN-01),
  not open-ended content drift. They are the deliberate exception, not a slide
  toward full content-compare.
- **D-18 (ssh-agent = reachable + managed keys loaded):** check the agent is
  reachable (`ssh-add -l`), then **warn for each gitid-managed identity whose key
  is not currently loaded**. Scoped to gitid's own keys — no noise about the
  user's other loaded keys.
- **D-19 (full content-compare rejected):** re-rendering each managed block and
  diffing against disk is explicitly out — heavy, brittle against legitimate user
  edits, and prone to false "drift." D-15 + D-17 cover real breakage without it.
- **D-20 (git<2.36 + `hasconfig:` warning):** keep the DOC-05 warning when a
  managed `includeIf` uses `hasconfig:` and the local git is older than 2.36;
  `deps.GitVersionAtLeast(2, 36)` already exists.

### Claude's Discretion
- **Command/flag naming & help text** — `gitid doctor` plus `--fix` / `--yes`
  (or equivalents like `--no-prompt`); exact flag names and help copy are the
  planner's call, consistent with the Phase 2/3 minimal-real-Cobra pattern.
- **Family ordering & exact line formatting** of the grouped report (and whether
  Baseline is its own section vs folded into Coherence/Orphans) — planner's call,
  keeping it dashboard-shaped for Phase 5 reuse.
- **Per-OS install-hint text for `git` and the clipboard tool** — extend the
  `platform.InstallHint` pattern (currently OpenSSH-only) to git/clipboard; exact
  wording is planner discretion (brew / apt / dnf / pacman per DOC-01).
- **Finding/severity type shape** (struct fields, how the optional fix descriptor
  is modeled) — planner's call, TDD-first; must keep the core write-free.
- **Where the fixer lives** in `cmd/gitid` and how it re-uses `filewriter`
  removal / `gitconfig`/`sshconfig` writers — planner's call.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project intent, scope & locked decisions
- `.planning/ROADMAP.md` §"Phase 4: Doctor" — goal + 5 success criteria + MVP
  mode + `UI hint: yes`. Also §"Phase 5" for the TUI-dashboard boundary that
  DOC-07's TUI half belongs to.
- `.planning/REQUIREMENTS.md` §DOC-01…DOC-07 — the seven doctor requirement
  statements and their acceptance phrasing; plus §KEY-02 (target permissions for
  the chmod fix) and §SIGN-01/SIGN-02 (allowed_signers form, signingkey-as-path).
- `.planning/PROJECT.md` §Constraints + §"Key Decisions" — Safety (backup +
  idempotent whole-block + permissions + confirm), Architecture (thin CLI/TUI
  over a tested UI-free core; `doctor`/`deps`/`platform` are named seams),
  English-only, `git config`-via-exec read strategy.
- `CLAUDE.md` — working method (hypothesis → test → implement; tests show
  input+output), safe-write rules, managed-block sentinel format, the
  `kevinburke/ssh_config` round-trip and `git config`-via-`os/exec` strategies,
  TDD, English-only artifacts.

### Prior phases (the layers doctor inspects)
- `.planning/phases/03.1-baseline-global-git-config-global-gitignore/03.1-CONTEXT.md`
  — the baseline model doctor now health-checks: `~/.gitconfig.d/00-baseline` via
  a managed `[include]` (D-01/D-10 floor placement), the three managed blocks
  (`baseline`, `url-rewrites`, `gitignore`), the locked `ignorecase=false` /
  `excludesfile` / curated-excludes invariants (D-03/D-08), and the original
  "Deferred: Doctor checks the baseline (Phase 4)" note this phase fulfills.
- `.planning/phases/03-full-identity-crud-multi-identity/03-CONTEXT.md` — the
  IDENT-07 reconstruction model (D-01: sentinel identity name as correlation key;
  D-02: best-effort with a light "incomplete" marker, deep diagnosis deferred to
  doctor here). Doctor's coherence/orphan checks build directly on `Reconstruct`.
- `.planning/phases/02-first-identity-end-to-end/02-CONTEXT.md` — the
  four-artifact write model, sentinel format, fragment placement
  (`~/.gitconfig.d/<identity>`), and the two-phase test/`ssh -G` resolution model
  doctor's coherence checks lean on.

### Architecture, pitfalls & target structure
- `.planning/research/ARCHITECTURE.md` — `internal/` package seams; doctor
  composes `platform`, `deps`, `sshconfig`, `gitconfig`, `identity`, `filewriter`.
- `.planning/research/PITFALLS.md` — permission, atomic-write, and round-trip
  pitfalls the permission check + any fixer must honor.
- `.planning/research/STACK.md` — `git config` via `os/exec` for reads
  (`--get`/`--list`/`--get-regexp`), `ssh -G`/`ssh-add -l` invocation patterns.

### Existing code doctor extends
- `internal/doctor/doc.go` — the current package contract ("never writes — returns
  structured findings only"); D-01 preserves this.
- `internal/deps/deps.go` — `Detect()` (DOC-01 tool probe), `MissingRequired()`,
  `GitVersionAtLeast(major, minor)` (DOC-05 / D-20 git-version gate).
- `internal/platform/platform.go` — `CurrentOS()`, `InstallHint(os)` (per-OS hint
  pattern to extend to git/clipboard, D-discretion).
- `internal/identity/loader.go` — `Reconstruct(...)` and the `Incomplete` marker
  (coherence/orphan foundation).
- `internal/filewriter/block.go` — `ListBlocks` / `RemoveBlock` (orphan
  enumeration + fixable removal), `ReplaceBlock` (wiring re-add).
- `internal/gitconfig/reader.go` + `internal/sshconfig/reader.go` —
  `ParseManagedIncludeIf`, `ReadFragment`, `ParseManagedHosts` (coherence reads).

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`deps.Detect()` / `deps.Report.MissingRequired()` / `deps.GitVersionAtLeast`**
  (`internal/deps/deps.go`) — DOC-01 dependency family and the DOC-05 git-version
  warning are direct compositions of these; no new probing needed.
- **`platform.InstallHint(os)` + `platform.CurrentOS()`**
  (`internal/platform/platform.go:35,109`) — per-OS install-hint pattern (brew /
  apt / dnf / pacman). Currently OpenSSH-only; extend the same switch to git and
  the clipboard tool for DOC-01.
- **`identity.Reconstruct(...)`** (`internal/identity/loader.go:17`) — rebuilds
  the identity/account set from managed blocks with an `Incomplete` field; the
  coherence (DOC-03) and orphan (DOC-04) checks key off this. The `Account`
  struct (`identity.go:25`) already carries `KeyPath`, `PubPath`, `FragmentPath`,
  `AllowedSignersPath`, `Alias`, etc. — exactly the fields the checks resolve.
- **`filewriter.ListBlocks` / `RemoveBlock` / `ReplaceBlock`**
  (`internal/filewriter/block.go:18,59,175`) — orphan enumeration, fixable
  orphaned-block removal, and wiring re-add all route through these (CRLF-tolerant,
  test-proven).
- **`gitconfig.ParseManagedIncludeIf` / `ReadFragment`** and
  **`sshconfig.ParseManagedHosts`** (the readers) — the coherence checks
  (`includeIf` → fragment, Host → `IdentityFile`/`IdentitiesOnly`) parse via these.
- **`identity.Deps` injected-function pattern** (`modes.go`/`update.go`/
  `delete.go`) — every external effect is a function field, so logic is
  fake-testable and TUI-reusable. The doctor checks **and** the cmd-layer fixer
  should follow this shape so the Phase 5 TUI can reuse them.

### Established Patterns
- Core is **test-first (TDD)**; the read-only doctor core is highly testable with
  fakes (no real filesystem/agent in unit tests). Findings are pure data.
- **Safe-write rule applies to the fixer only** — never the detection core. Every
  mutating fix: timestamped backup → atomic temp→rename→chmod → idempotent block
  rewrite → explicit confirmation (per-finding for block/wiring edits; `--yes` is
  the non-interactive confirmation).
- `internal/doctor` is **currently a stub** (`doctor_stub_test.go`,
  `doc.go` only) — Phase 4 is the first real implementation; greenfield within
  the package but composing proven siblings.
- `make test` / `make lint` (golangci-lint + gosec, hard-fail) gate every commit;
  pre-push runs `go test -race` + coverage. All content English. Commits squashed
  at plan close + review.
- `gsd-tools.cjs` needs Volta's bin on PATH in non-interactive shells (GSD scripts
  only — not the Go build): `export PATH="$HOME/.volta/bin:$PATH"`.

### Integration Points
- New minimal `gitid doctor` Cobra command in `cmd/gitid` (mirrors
  `add`/`list`/`update`/`baseline`): builds `doctor.Deps` from the real internal
  packages, runs the read-only checks, renders the grouped report, and — on
  `--fix`/prompt — drives the cmd-layer fixer through `filewriter`.
- The report's family grouping + `✓`/severity model is the data shape the Phase 5
  TUI dashboard will consume — keep the finding type UI-agnostic.
- Coherence/orphan/baseline checks read `~/.ssh/config`, `~/.gitconfig`,
  `~/.gitconfig.d/*`, `~/.gitignore_global`, key files, and `ssh-add -l` output —
  all reads, no sidecar DB, consistent with the reconstruction model.

</code_context>

<specifics>
## Specific Ideas

- Command surface: `gitid doctor` (report) / `gitid doctor --fix` (per-finding
  confirm) / `gitid doctor --fix --yes` (non-interactive apply).
- Six (or seven with Baseline) report families, grouped, each showing `✓` passes
  and severity-tagged findings — dashboard-shaped for Phase 5.
- Exit codes: `0` clean / `1` warn+info / `2` error / `3` critical (highest wins).
- Severity bands: `critical` (key exposed) / `error` (broken) / `warning`
  (degraded, incl. unreferenced key + locked-value drift) / `info` (advisory).
- Auto-fix only for: permissions (chmod to KEY-02 perms), orphaned managed-block
  removal, missing-wiring re-add. Key-file deletion and dep installs are
  report-only.
- Unused-key wording must explicitly say "may be used for non-git SSH / `ssh -i`
  — review, don't assume safe to delete."
- Locked-value checks (the only value comparisons): `ignorecase=false`,
  `gpg.format=ssh`, `allowed_signers` email == `user.email`.

</specifics>

<deferred>
## Deferred Ideas

- **Map non-git keys to SSH server hosts (FUTURE REQUEST).** Build a real map of
  which `~/.ssh` keys serve non-git SSH (servers) so the "unused key" `warning`
  (D-13) can be upgraded to a confident classification instead of a hedge. No
  reliable persistent source exists today (D-14 rules out `known_hosts`); would
  need heuristics (e.g. parsing all `Host` blocks — already done — plus optional
  user annotation). User-requested during Phase 4 discussion.
- **`known_hosts`-based correlation — investigated and rejected (D-14).** Recorded
  here too so it is not re-proposed: `known_hosts` holds server host keys, not a
  client-key→host usage map.
- **TUI doctor dashboard (Phase 5).** DOC-07's "runs first in the TUI" half; the
  grouped-by-family report here is its intended data source.
- **Full Cobra surface + shell completion (Phase 5).** Phase 4 ships only the
  minimal real `gitid doctor` command.
- **`url-rewrites` block health checks.** Phase 3.1's `url-rewrites` managed block
  was not folded into the Phase 4 baseline checks (the four folded checks cover
  excludesfile/include/ignorecase/excludes). A future doctor pass could validate
  the `insteadOf` rewrites (e.g. dangling/duplicate mappings) if needed.

### Reviewed Todos (not folded)
None — `todo.match-phase 4` returned no matches.

</deferred>

---

*Phase: 4-Doctor*
*Context gathered: 2026-06-11*
