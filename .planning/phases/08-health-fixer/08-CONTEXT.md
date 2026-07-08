# Phase 8: Health + Fixer - Context

**Gathered:** 2026-07-07
**Status:** Ready for planning (plan AFTER Phases 5-7 execute — the checks
consume artifacts those phases write, and the screens replace the last two
demo views in the Phase 5 view set)

<domain>
## Phase Boundary

Re-home the doctor substrate into the two frozen surfaces: a READ-ONLY
Health screen (view `4`, 5 states) split into SSH and Git sections showing
severity-sorted findings with per-finding detail and a per-identity slice,
and the write-side Fixer (view `5`, 6 states) applying confirmed,
backed-up fixes in place through the `v→x→y→z` ceremony chain. Ships the
new checks queued by Phases 5-7, the HLTH-02 parse gates, and the
mandatory tolerances. Health never mutates (negative-asserted); the Fixer
lists the SAME findings Health diagnosed (traceable, not re-derived).

Requirements: HLTH-01..06, FIX-01, FIX-02 (+ DLV-04/DLV-06 UI-wave gates).
Design FROZEN by Phase 2 (`health/FIELDS.md` + `fixer/FIELDS.md`); this
phase carries one fixture-divergence RULING (typed confirm, D-11 below)
and one scoped CLAUDE.md amendment (D-09).

**Explicitly NOT in this phase:** known_hosts hashing (backlogged, D-08),
upload/credentials assist (Phase 9), the adopt-this-block offer after a
surgical fix (later phase — must not become a 7th fixer state).

</domain>

<decisions>
## Implementation Decisions

### Family → section mapping + CLI
- **D-01 — Per-finding `Target` field with family-default fallback.**
  `doctor.Finding` gains a Target/Section field; a central default map
  covers single-domain families; only the cross-file families (Coherence,
  Orphans, Signing, Redundancy) set it explicitly per finding. Computed
  ONCE, consumed by the health TUI, the fixer, `--json`, and the
  HLTH-05/MGR-07 per-identity slice. Guard test: every emitted finding
  resolves to a section. Tool-level findings route per tool (ssh/agent →
  SSH, git → Git, clipboard-info → SSH) as ordinary rows — NO new "System"
  sub-label (would diverge from the frozen field manifest).
- **D-02 — In-section grouping is flat severity-sorted rows**
  (critical→error→warning→info), family visible only as the finding-detail
  chip — settled by the frozen FIELDS.md manifest; family sub-headers would
  fail the parity gate.
- **D-03 — CLI: `gitid health [--json]` (read) + `gitid fix [--yes]
  [--dry-run]` (write).** `doctor` remains a PERMANENT HIDDEN Cobra alias
  of `health`, with a hidden `doctor --fix` forwarding shim to `fix`
  (docker `ps` precedent). The read/write command split enforces the frozen
  read-only affordance at the CLI layer. Both join the Phase 5 parity
  matrix.
- **D-04 — `gitid health --identity <name>`** filters via the existing
  `Finding.IdentityName` (+ identity-name shell completion) — the HLTH-05
  outcome as a CLI row. Whether global (empty-name) findings appear
  labeled or excluded in the scoped view: Claude's discretion.

### New checks (the Phases 5-7 queue) — full queue minus known_hosts
- **D-05 — Ship:** (each severity/fix pinned)
  | Check | Severity | Fix | Family/Section |
  |---|---|---|---|
  | `~/.ssh/config` exists + parses | parse fail = **critical** — renders the frozen parse-error screen; downstream checks PAUSED | No | NEW `Files` / SSH |
  | `~/.gitconfig` + every fragment parses (`git config --file <f> --list` exit code) | same ladder | No | `Files` / Git |
  | Global gitignore missing / dangling excludesfile (Phase 7 D-11 obligation) | missing pair = warning; key-set-but-file-missing = error | **YES** — ONE fix writing BOTH `core.excludesfile` AND the managed pattern file (never key-only); dormant `WriteGlobalGitignore` substrate | Baseline / Git |
  | SSH managed-block option applied-but-shadowed by an earlier user directive (Phase 6 D-04 hand-off) | warning | No — SuggestedFix names file:line (static-scan namer + `ssh -G` proof) | Coherence / SSH |
  | Git baseline key overridden by user's later value ("set, differs", Phase 7 D-02) | **info, HARD CAP** — test-asserted never escalates; excluded from any needs-action tally | No (provably a no-op) | Baseline / Git |
  | Author-resolution verification (Phase 7 D-06 re-run as a check): `git config --show-origin --get user.email/name` from matched-identity repo ctx → fragment; non-repo ctx → fallback | error when the invariant breaks | Report-only; SuggestedFix routes to the D9 ceremony / git-screen repair | Coherence / Git |
  | `IdentitiesOnly no` + explicit IdentityFile (frozen fixture) | **error** (fixture pins the chip) | YES — the fixer flagship; hand-written targets per D-09 | Coherence / SSH |
  | includeIf → missing fragment (frozen fixture; = `fragment-path-missing`) | error | Remove-orphaned-block when gitid-managed (existing RemoveBlock ceremony); **NEVER recreate an empty fragment** (silent broken author); hand-written → advisory | Coherence / Git |
  | User directive above the managed block (Phase 6 deferred item) | warning | No — user content untouchable outside D-09's ceremony | Coherence / SSH |
- **D-06 — Binding tolerance obligations** (code that fights upstream
  decisions gets fixed in this phase):
  1. **CheckOrphans Class-1 downgrade:** an SSH block whose gitconfig
     partner was removed by a deliberate git-only delete (Phase 5 D-10's
     state) currently trips an orphan finding WITH a destructive
     RemoveBlock fix — the false-positive-loop class. Downgrade to info,
     NO fix, for that detected state.
  2. **Reserved-PATH registry:** does not exist (block names only).
     Phase 8 consumes/builds it so `~/.ssh/gitid-archive/` (Phase 5 D-06)
     is never swept into unused-key findings — with a populated-archive
     regression test.
  3. **Tolerances specced + test-asserted:** two allowed_signers lines per
     email (rotate-append, Phase 5 D-07); fragment-less principals after
     git-only delete (info at most); "set, differs" info cap.
- **D-07 — New checks join existing families** (Coherence/Baseline) except
  the parse gates, which get the new `Files` family appended after
  Redundancy (safe: parse failure renders the dedicated screen, not a
  family-ordered list row).
- **D-08 — known_hosts hashing is BACKLOGGED with a named home**
  (post-v1.0, or a Phase 8 stretch task if the wave runs light). Whenever
  it ships: info severity, gated on `HashKnownHosts yes` already being set
  (completion of the user's own choice — never a nag), fix runs
  `ssh-keygen -H` under the ceremony, and it deliberately amends the
  "doctor never reads known_hosts" invariant.

### Fix-in-place policy (the highest-risk affordance)
- **D-09 — Surgical single-directive edit, via a SCOPED CLAUDE.md
  amendment.** FIXTURE-VERIFIED: the frozen flagship walk-through rewrites
  `IdentitiesOnly no → yes` on a sentinel-LESS, hand-written
  `Host clientb.github.com` block, and the frozen result copy pins "only
  the rewritten directive changed" — foreclosing both managed-blocks-only
  and adopt-then-fix. Policy: the Fixer — and ONLY the Fixer — may rewrite
  exactly ONE existing directive outside managed blocks, exclusively
  through the full ceremony (true diff preview, typed confirm per D-11,
  timestamped backup named before apply). CLAUDE.md's managed-block rule
  gets a documented scoped amendment; "hand-written config is never
  corrupted" becomes an enforced verification property (D-10), extending
  Phase 5 D-13's warn-honestly precedent to the write side.
- **D-10 — Full verification loop, MANDATORY, around every fixer edit:**
  parse→modify→render→re-parse stability check (the CLAUDE.md second-
  Decode pass) + post-apply resolved-config re-verification (`ssh -G` /
  `git config`) + AUTOMATIC restore from backup on mismatch. Degrades to
  render+re-parse only when ssh/git are absent (headless fallback; known
  CI portability class).
- **D-11 — Fixture-divergence RULING: typed hostname confirm.** The
  dummytui `ConfirmWord: "clientb.github.com"` depiction (already
  test-asserted) wins over FIELDS.md's "short of a typed confirmation"
  sentence — amend the FIELDS.md sentence (documented divergence). Typing
  the target hostname is the confirm for hand-written-config fixes.

### Batch application + re-evaluation
- **D-12 — Batch = an auto-advancing QUEUE of per-finding ceremonies.**
  The frozen `fixerBatchFixNote` ("Apply all N fixes — each one still
  previews") + the six-state cap foreclose a combined ceremony. Each
  selected fix walks its own `v→x→y→z` with its own backup (= its own
  undo). Heterogeneous fixes (different files, Fn vs Interactive) are why
  the Phase 6 D-16 combined model does not transfer.
- **D-13 — Re-run ALL checks after EVERY applied fix**; the queue is
  rebuilt from fresh findings (matched by signature) so no fix closure
  ever runs against stale state — the doctor bug's root cause. Findings
  incidentally resolved simply vanish from the re-rendered list.
- **D-14 — Explicit convergence alarm.** If a finding reappears after its
  OWN fix reported success, emit an error-severity "fix did not converge"
  finding, permanently exclude that fix from re-offer, and surface it on
  the result path — upgrading `convergeFixes`' current silent stop
  (`cmd/gitid/doctor.go:185`, maxPasses=10 stays as the hard backstop;
  ESLint's circular-fix evolution is the precedent).
- **D-15 — Result screen stays frozen (3 fields).** Dismissing
  result-applied returns to the RE-EVALUATED fixer list, which IS the
  remaining-findings report; `nothing-to-fix` becomes the earned end
  state. No new result fields (no design amendment).
- **D-16 — Single-fix rollback scope.** A fix that fails mid-apply
  restores its OWN backup; earlier completed (and re-verified) fixes
  stand; the queue halts naming the failure AND listing what already
  succeeded. Phase 4 D-11's all-or-nothing protected one logical artifact;
  fixer fixes are independent repairs — rolling back verified repairs
  would re-open diagnosed problems.

### Claude's Discretion
- Global-findings presentation in the `--identity` scoped view (D-04).
- The `Files` family display name; finding-signature shape (family|title
  today — disambiguate if two findings can share it).
- Exact copy for: the convergence alarm, the shadowed-fix SuggestedFix
  file:line format, parse-error screen wiring, the queue-halt failure
  message (draft in the UI wave; freeze via §6).
- Temp/probe mechanics reuse from Phases 6-7 (shadow scan namer, author
  probes) — share one implementation where the detector is the same.
- The `doctor --fix` shim's exact flag mapping.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### North Star
- `recipes/ssh-config.recipe` + `recipes/gitconfig.recipe` — conformance
  targets several checks verify (IdentitiesOnly per-alias, excludesfile,
  includeIf wiring).

### Requirements / roadmap (authoritative)
- `.planning/ROADMAP.md` §"Phase 8" — goal + 5 success criteria.
- `.planning/REQUIREMENTS.md` §K (HLTH-01..06), §L (FIX-01/02), §A
  (DLV-04/06).

### Approved design (BINDING; one ruling + one sentence amendment)
- `.planning/design/health/FIELDS.md` — 5 frozen screens, pinned example
  findings (the two HLTH-04 contradictions, the parse-error state, the
  `legacy` per-identity slice), 4-severity glyph contract, read-only
  negative assertion.
- `.planning/design/fixer/FIELDS.md` — 6 frozen screens, the flagship
  sentinel-less diff, `fixerBatchFixNote`, `result_preserved_note`
  ("only the rewritten directive changed"), the confirm sentence D-11
  amends.
- `.planning/phases/02-design-all-mockups-checkpoint-1/02-STYLE-SPEC.md` +
  `02-UX-DIRECTION.md` §4.6/§4.7/§5 — glyphs, ceremony beats, key
  allocation.

### Upstream phase contracts (the queue's sources)
- `.planning/phases/05-identity-manager/05-CONTEXT.md` — D-06 (archive
  dir), D-07 (signers append), D-10 (fragment-less principal), D-13
  (warn-never-block precedent), CLI conventions.
- `.planning/phases/06-global-ssh-options/06-CONTEXT.md` — D-04 (shadow
  simulation this phase re-detects), deferred list (directive-above-block,
  known_hosts).
- `.planning/phases/07-global-git-options/07-CONTEXT.md` — D-02 ("set,
  differs"), D-06 (author-resolution loop), D-11 (gitignore obligation).

### Substrate (code findings that ground the decisions)
- `internal/doctor/doctor.go` — 9 families, Severity, Finding
  (IdentityName), FixDescriptor (Fn/Interactive) — D-01's Target field
  lands here.
- `internal/doctor/checks/orphans.go` — Class-1 orphan fix (the D-06.1
  downgrade target; reserved-name guards + false-positive-loop comments).
- `internal/doctor/checks/coherence.go` — uniform SeverityError posture;
  new contradiction checks join here.
- `cmd/gitid/doctor.go` — `convergeFixes` (:185, silent stop → D-14
  alarm), `applyFixes` D-04 batching (:607), the file D-03's new commands
  replace.
- `internal/dummytui/fixplans.go` (:43 ConfirmWord) + `data.go` (:790-833
  sentinel-less flagship diff) — the D-09/D-11 fixture evidence.
- `internal/gitconfig/baseline.go` — `WriteGlobalGitignore` /
  `DefaultGitignorePatterns` (dormant; the D-05 gitignore fix).
- `internal/sshconfig/include.go` — `IsReservedBlockName` (block names
  only; D-06.2 adds the path registry).
- `internal/identity/inventory.go` — `listKeyFilesReal` glob (the archive
  regression test's subject).

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- The doctor engine (families, Deps injection, fix descriptors,
  convergeFixes) — Phase 8 re-homes and extends; it does not rewrite.
- Phase 6's shadow-detection static-scan namer + `ssh -G` probes and
  Phase 7's author-resolution probes — the same detectors re-run as
  checks; share one implementation.
- `internal/tuikit` master-detail + ceremony components (Phases 3-7) —
  both new surfaces compose existing machinery.
- `internal/adopter` / `internal/sshconfig.Adopt` — NOT used by the
  fixer's flagship (D-09 forecloses adopt-then-fix) but available for the
  deferred adopt-offer.

### Established Patterns
- Prove-before-and-after: D-10 applies the Phases 6/7 loops to every
  fixer edit, with auto-restore.
- Reserved-name/path registration before any new gitid-owned artifact
  class (D-06.2).
- Copy-freeze §6 + scoped-divergence discipline (D-11's sentence
  amendment, the CLAUDE.md scoped amendment note).
- Severity ladder discipline: critical = parse gate only; info = user
  choices, hard-capped.

### Integration Points
- Views 4/5 replace the last demo screens — the Phase 3 D-16 banner era
  ends with this phase.
- MGR-07/HLTH-05: the manager's row badges consume the same
  Finding.IdentityName slice `gitid health --identity` exposes.
- Phase 9 (upload) may add its own checks later; the Files family +
  Target field give it a slot without redesign.
- CLI: `health`/`fix`/`--identity` rows join the Phase 5 parity matrix;
  `doctor` alias preserves POC muscle memory.
- UI wave: `/gsd-ui-phase 8` (divergence set: typed-confirm sentence
  amendment, any new copy) + PTY e2e per screen + golden-text gate +
  agent-ui-ux-designer + Codex review (Phase 3 D-24/D-25 conventions).

</code_context>

<specifics>
## Specific Ideas

- The false-positive loop is this phase's governing precedent: D-06.1
  removes the last live instance of the class, D-13/D-14 make the class
  structurally impossible to ship again silently.
- The fixer's scoped power (D-09) is the deliberate counterpart of the
  advisory posture everywhere else: Health never writes, options screens
  never fight the user, and exactly one surface — with typed confirmation,
  backup, and mandatory re-verification — may touch a hand-written line.
- Frozen fixtures are evidence, not just pictures: the sentinel-less
  flagship diff decided D-09; the ConfirmWord fixture decided D-11; the
  batch note decided D-12.

</specifics>

<deferred>
## Deferred Ideas

- **known_hosts hashing fix** — post-v1.0 or Phase 8 stretch (D-08's
  gating rules pinned now).
- **Adopt-this-block offer after a surgical fix** — later phase, lives
  outside the 6 fixer states (Manager/Health affordance).
- **Signers-line prune fixer** (stale rotate-append lines, orphaned
  principals) — deliberate, opt-in, and must exclude git-only-deleted
  identities' principals (Phase 5 deferred item; tolerances pinned in
  D-06.3).
- **`result-applied` "N remaining · also resolved: M" field** — only if
  the UI-wave designer wants it; the D-13 bookkeeping makes the data free.
- **Health checks for Phase 9 upload artifacts** — Phase 9.

</deferred>

---

*Phase: 8-Health + Fixer*
*Context gathered: 2026-07-07*
