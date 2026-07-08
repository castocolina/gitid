# Phase 7: Global Git Options - Context

**Gathered:** 2026-07-07
**Status:** Ready for planning (plan AFTER Phase 6 executes — this phase reuses
Phase 6's advisory screen machinery, D-12/D-14 derived states, and the D-13
version-gated copy pattern; it depends on Phase 5's view set)

<domain>
## Phase Boundary

Wire the real backend behind the approved global-git design (view `3`, 6
frozen screens + the D9 dedicated ceremony): review the pinned option set
with current value + provenance + recommended value + contractual verbatim
explanation; apply user-selected fixes through ONE combined backup +
idempotent managed-block ceremony; the D9 global-fallback author gets its
own dedicated ceremony. ADVISORY, never blocking; content outside managed
blocks preserved verbatim (the surface's highest-risk affordance).

Requirement: GGIT-01 (+ DLV-04/DLV-06 UI-wave gates). Design FROZEN by
Phase 2 (`global-git/FIELDS.md`, 6 screens + D9 section, keys `v/f/w/y/z`);
this phase carries TWO documented design amendments decided below (D9
two-field no-checkbox pair — user override; `user.useConfigOnly` 12th row)
plus the established scoped-divergence set.

**Explicitly NOT in this phase:** insteadOf URL rewriting (moved to Phase 4's
ceremony — W1 closed there; `RemoveURLRewritesBlock`/`DefaultURLRewrites`
are Phase 4 territory), the global-gitignore surface (deferred to Phase 8's
fixer — D-11 below), health checks (Phase 8).

**Carried forward from Phase 6 (not re-decided):** the advisory posture
bundle — stateless + derived states (6/D-14), "set, differs" word state
(6/D-12), empty opt-in selection (6/D-15), one combined ceremony +
toward-recommendations only (6/D-16), static-facts + dynamic-line copy
split (6/D-13), reserved-name registration discipline (6/D-08).

</domain>

<decisions>
## Implementation Decisions

### Precedence, block home, and detection
- **D-01 — Keep the POC include'd-baseline-file architecture.** A floor
  `[include]` sentinel block in `~/.gitconfig` (existing
  `WriteBaselineInclude` + `PrependBlockIfNotFound`) points at the include'd
  baseline file holding the options. EMPIRICALLY VERIFIED (git 2.47 probes):
  floor include + user's later value → user wins; an APPENDED block silently
  overrides the user (rejected outright). This is also the recipe's own
  idiom (`[include] path = ~/.gitconfig_default`). The baseline sentinel
  adopts the frozen name **`global-git`** (rename from the POC name, first
  write adopts/renames — Phase 6 D-08 precedent) and is registered
  doctor-reserved; confirm-write names the RESOLVED target file (allowlisted
  copy divergence — Phase 6 D-07 precedent).
- **D-02 — Conflict policy: informational, no fix.** Keys the user already
  set later in the file render as "set, differs — your choice" (6/D-12),
  are NOT selectable, and are EXCLUDED from the "N of M" tally — under
  last-wins a write into the floor block is provably a no-op.
  `ScanConflicts` (Winner="user" hard-coded) already computes the list.
- **D-03 — Detection via native git probes** (stack-doc mandate: git is the
  authoritative parser). Effective values + provenance:
  `git config --show-origin --show-scope --list` run from a NON-repo cwd
  (the Phase 6 D-05 dummy-probe analog; `--show-scope` needs git ≥ 2.26 —
  safe floor). Physically-in-file: `git config --file ~/.gitconfig --list`
  without `--includes` (the existing ScanConflicts pattern). Diffing the two
  classifies rows gitid-managed / user-set / unset. Text manipulation stays
  confined to sentinel-block writes in filewriter.

### D9 global-fallback author (design amendment — USER OVERRIDE)
- **D-04 — D9 becomes a TWO-FIELD, NO-CHECKBOX pair:** editable
  `user.name` and `user.email` fields. Semantics (user's words: "we need
  two fields, if empty we set empty config, if set we set the variables"):
  an EMPTY field leaves that key UNSET — omitted from the fallback block,
  or removed if previously set; NEVER written as an empty-string value. A
  non-empty field is written. Independent per-field apply; email format
  validation reuses the create-form validator when non-empty. Rationale
  (verified): with only an email set, git silently guesses the author name
  from the OS account (GECOS/username) — the frozen email-only D9
  half-works by construction. This amends the frozen D9 field list + copy
  (documented design amendment; the 7 frozen D9 strings are updated via the
  copy-freeze mechanism). The includeIf-precedence invariant and helper
  language stay.
- **D-05 — The fallback pair lives in its OWN small sentinel block,
  positioned EARLY** (immediately after the floor include; new
  insert-after-floor filewriter primitive; block registered
  doctor-reserved). VERIFIED: plain `git config --global user.email` lands
  AFTER includeIf blocks on a recipe-shaped file and hijacks every
  identity's author — rejected. The frozen "own dedicated ceremony, never
  folded into the baseline block" is honored literally.
- **D-06 — The invariant is PROVEN post-write, not asserted:**
  `git config --show-origin --get user.email` (and user.name) runs from (a)
  a matched-identity repo context — must resolve to the fragment path — and
  (b) a non-repo cwd — must resolve to the fallback block. The prove-
  before-and-after loop applied to author identity.
- **D-07 — `user.useConfigOnly = true` ships as a 12th advisory row**
  (documented divergence amending the frozen row count): own opt-in row,
  default unchecked (recipes leave it unset — awareness-first). It converts
  git's silent guessed-junk-author path into a clear error — the
  fail-loud companion to includeIf setups. MANDATORY cross-warning copy when
  it is selected while the fallback pair is partial (name-only/email-only):
  unmatched-repo commits will hard-fail — that is the point, but say it.
  Also render the guessed-name warning on the name-empty + email-set state.

### Pinned values + version gates
- **D-08 — The 11-row pinned table (APPROVED):**

  | Option | Recommended | Gate | Note |
  |---|---|---|---|
  | init.defaultBranch | `main` | ≥ 2.28 informational | main-vs-master highlight row; current shows "not set (git's built-in default: master)" |
  | core.ignorecase | `false` | — | frozen copy MUST state the APFS caveat: `git init`/`clone` probe the filesystem and write repo-local `true`, which beats global — "per-repo detection overrides this" |
  | core.autocrlf / core.eol | `input` + `lf` | — | eol is ignored while autocrlf=input but documents intent; current-when-unset: "no conversion; eol=native" |
  | user.email (fallback) | unset | — | D-04/D-05 dedicated ceremony; never in the baseline block |
  | push.autoSetupRemote | `true` | ≥ 2.37 informational | old git silently ignores unknown KEYS — harmless to write |
  | pull.rebase | `true` | — | |
  | fetch.prune | `true` | — | |
  | alias | 8 recipe shortcuts (st co br ci df lg unstage last) | — | bundle row, D-09 |
  | color | ui/branch/diff/status = auto | — | bundle row, D-09 |
  | merge.conflictstyle | `zdiff3`, **fallback `diff3` below 2.35** | ≥ 2.35 HARD gate | the ONLY row where the gate changes the WRITTEN value — old git ERRORS at merge time on unknown VALUES |
  | diff.colorMoved | `zebra` | ≥ 2.15 informational | |

  Version notes reuse Phase 6 D-13: static version facts inside frozen
  copy + a NON-contractual "your git: X.Y" line via the EXISTING
  `deps.GitVersionAtLeast` (do not add a new probe).
  DISCREPANCY RESOLVED: the POC's zdiff3 (with its C4 gate work) wins over
  the frozen fixture's diff3 — amend BOTH fixture texts
  (`GlobalGitFullManagedBlockText` + the `merge=diff3` strip text in
  `internal/dummytui/data.go` / `recipeFixtures.ts`) in ONE commit, before
  the copy freeze.
- **D-09 — Bundle rows (alias, color): unit-apply + honest notes.** One
  checkbox per bundle row toggles the whole canonical section (POC
  `IncludeAliases`/`ExtraColors` model — byte-stable block, SC-1
  idempotency preserved). The block includes ALL keys regardless of user
  collisions (last-wins + floor = user's own `alias.co` etc. always wins);
  option-detail shows per-key "yours differs — yours wins" notes (reuses
  ScanConflicts, pure presentation, stateless). The bundle row's current
  cell is an aggregate summary ("3 of 8 set, 1 differs") with glyph/word
  derived from the 6/D-12 vocabulary. Per-key sub-selection deferred to
  post-v1 feedback.
- **D-10 — Whether "set, differs" rows/bundle members count in the "N of M
  need action" tally is ONE rule pinned during the UI wave** (before copy
  freeze), shared by scalar and bundle rows and consistent with Phase 6's
  same open discretion.

### GITIGNORE-01 scope call
- **D-11 — Global gitignore DEFERS to Phase 8's fixer.** Missing/unmanaged
  global gitignore becomes a FIX-01 finding (detect → explain → confirmed,
  backed-up write of BOTH `core.excludesfile` AND the managed pattern
  file — the dormant `WriteGlobalGitignore`/`DefaultGitignorePatterns`
  substrate maps 1:1 onto the fix-engine shape). Key-only fold-in is
  REJECTED (git silently tolerates a dangling excludesfile — invisibly
  broken half-state). TWO obligations land NOW in Phase 7:
  1. **Amend REQUIREMENTS §J** — the "(GLOBAL-01/GITIGNORE-01/URLRW-01
     fold in)" note is stale (URLRW-01 already re-homed to Phase 4) and
     must be corrected to name Phase 8 for GITIGNORE-01, preventing a
     false-complete GGIT-01 at phase close.
  2. **Demote `core.excludesfile` out of baseline Tier-1** (latent contract
     break found: `baseline.go:424` emits it unconditionally while the
     frozen fixture block omits it) — gate it so Phase 7's real write
     matches the frozen block byte-for-byte.

### Claude's Discretion
- The D-10 tally rule; exact aggregate-cell phrasing under the 80-col
  budget.
- Exact copy for: the APFS ignorecase caveat, guessed-name warning,
  useConfigOnly cross-warning, per-key "yours differs" notes, provenance
  labels, the 11 contractual explanations + amended D9 strings (draft in
  the UI wave; freeze via §6; static version facts only).
- The insert-after-floor filewriter primitive's shape; legacy POC sentinel
  adoption/migration mechanics.
- `gitid git options …` CLI shape per Phase 5 D-01..D-04 + parity-matrix
  entries (including the D9 pair and useConfigOnly as CLI-reachable
  outcomes).
- cwd discipline + tab-separated parsing for the D-03 probes.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### North Star — canonical config end state
- `recipes/gitconfig.recipe` — the `[include] path` idiom D-01 keeps, the
  `~/.gitconfig_default` example block (alias/color/values), lines 14-17
  `core.excludesfile` (the D-11 recipe line Phase 8 pays), fragments owning
  the author pair (D-04's baseline).
- `recipes/README.md` — wiring overview.

### Requirements / roadmap (authoritative)
- `.planning/ROADMAP.md` §"Phase 7" — goal + 3 success criteria.
- `.planning/REQUIREMENTS.md` §J (GGIT-01 — carries the D-11 amendment
  obligation), §H (MGR-08 norm).
- `.planning/archive/0.0.1-poc-product-features-in-tui/REQUIREMENTS.md`
  line 100 — original GITIGNORE-01 definition (key + pattern file are ONE
  requirement).

### Approved design (BINDING, with two documented amendments)
- `.planning/design/global-git/FIELDS.md` — the 6 frozen screens + the D9
  section (amended by D-04/D-07), parity rows `main-vs-master-highlight` +
  `managed-block-containment`, TUI compaction notes (ggitCompactValueLines).
- `.planning/design/global-git/CRITIQUE.md` — accepted compaction rationale.
- `.planning/phases/02-design-all-mockups-checkpoint-1/02-STYLE-SPEC.md` —
  §4 D9 frozen strings (amended via copy-freeze mechanism), §6 grep gates.
- `.planning/phases/02-design-all-mockups-checkpoint-1/02-UX-DIRECTION.md`
  — §4.5 global-git states, §5 ceremony beats, §2 key allocation.

### Upstream phase contracts
- `.planning/phases/06-global-ssh-options/06-CONTEXT.md` — the carried
  advisory bundle (D-04 prove-loop, D-07/D-08 precedents, D-12/D-13/D-14/
  D-15/D-16) this phase mirrors on the git side.
- `.planning/phases/05-identity-manager/05-CONTEXT.md` — CLI conventions +
  parity-matrix obligation.
- `.planning/phases/04-git-configuration-screen/04-CONTEXT.md` — insteadOf
  is Phase 4's (scope note), fragment writes the author pair D-05 must
  never override, reserved-registration precedent.

### Substrate (code findings that ground the decisions)
- `internal/gitconfig/baseline.go` — `WriteBaselineInclude` (:610, floor
  include), `PrependBlockIfNotFound` usage (:622), `ScanConflicts` (:70,
  Winner="user"), `BaselineConfig`/`DefaultBaselineConfig` (:316/:337,
  zdiff3 + C4 gate comment), `RenderBaselineBlock` (:401; :424 emits
  excludesfile Tier-1 — the D-11 demotion target),
  `WriteGlobalGitignore`/`DefaultGitignorePatterns` (dormant, zero live
  callers — Phase 8's substrate).
- `internal/dummytui/data.go` — `GlobalGitFullManagedBlockText` (~:604
  `conflictstyle = diff3`) + strip text (~:572) — the D-08 fixture
  amendment targets.
- `internal/deps` — `GitVersionAtLeast(major, minor)` (deps.go:47; already
  gates signing at (2,36)) — D-08's probe, reuse as-is.
- `internal/filewriter/block.go` — `PrependBlockIfNotFound` (:200-230);
  D-05 adds an insert-after-floor sibling primitive.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- The whole Phase 6 advisory screen machinery (options-list/option-detail/
  ceremony chain in `internal/tuikit`) — global-git is the same archetype
  with git-flavored rows; the frozen design says both surfaces share one
  visual language.
- `internal/gitconfig/baseline.go` — render/scan/write substrate largely
  exists; the work is the global-git sentinel adoption, the Tier-1
  excludesfile demotion, the zdiff3 fixture sync, and wiring live callers.
- `git config --show-origin --show-scope` — native provenance; no parser
  to build.
- Create-form email validator (D-04); Phase 6's D-13 version-gated copy
  mechanism (D-08).

### Established Patterns
- Prove-before-and-after applied to precedence: D-06's matched/unmatched
  post-write author verification is Phase 6 D-04's loop with git-native
  probes.
- Copy-freeze §6 gates: new strings include the amended D9 set, the
  useConfigOnly row, the APFS caveat, warnings and notes (static facts
  only; dynamic "your git: X.Y" excluded).
- Scoped-divergence + design-amendment discipline: D-04/D-07 are documented
  amendments (the D9 precedent itself shows the mechanism); resolved-file
  naming and fixture changes are allowlisted.
- Reserved-name registration: `global-git` baseline sentinel + the D-05
  fallback block + (Phase 8 later) the gitignore block.

### Integration Points
- View `3` replaces its demo screen; Health/Fixer keep banners until
  Phase 8.
- Phase 8 consumes: the D-11 gitignore fix (substrate ready), the
  "set, differs" finding class, and D-06's author-resolution check as a
  health family.
- Phase 4's fragment writes + includeIf blocks are the precedence context
  D-05/D-06 protect; Phase 4's insteadOf blocks live in the same file —
  the doctor's reserved registry is the shared guard.
- CLI: `gitid git options list --json` / `fix --yes --dry-run` +
  fallback-pair and useConfigOnly outcomes join the Phase 5 parity matrix.
- UI wave: `/gsd-ui-phase 7` covers the amendment set (D9 pair fields,
  12th row, aggregate cells) + PTY e2e + golden-text gate +
  agent-ui-ux-designer + Codex review (Phase 3 D-24/D-25 conventions).

</code_context>

<specifics>
## Specific Ideas

- USER OVERRIDE (verbatim intent): "git user.email and user.name are
  separate config, why checkbox — we need two fields; if empty we set
  empty config, if set we set the variables" → D-04's two-field,
  no-checkbox, empty-means-unset model with independent per-field apply.
- The phase's identity mirrors Phase 6 with the precedence arrow flipped:
  git is last-wins, so gitid's defaults go at the FLOOR and the user's own
  config is never fought — "applied but overridden by you" is a state to
  display, not a bug to fix.
- Empirical probes are the evidence standard: the block-home and D9
  placement decisions cite verified git behavior, and D-06 makes the
  invariant a permanent post-write test.

</specifics>

<deferred>
## Deferred Ideas

- **Global gitignore surface** (core.excludesfile + managed pattern file)
  — Phase 8 FIX-01 (D-11; substrate dormant and ready; §J amendment lands
  now).
- **Per-key alias/color sub-selection** — post-v1.0, only on real user
  feedback (D-09).
- **core.pager (`less -FRX`)** — in the POC baseline block but not in the
  frozen 11-row set; keep emitting in the block if already present, no UI
  row in v1.0 (revisit with the gitignore call in Phase 8 if desired).
- **Selectable fix + simulate-and-warn for user-set keys** — only if UAT
  shows the informational-only D-02 posture confuses users.
- **Windows line-ending guidance (autocrlf=true)** — out of scope until a
  Windows target exists.

</deferred>

---

*Phase: 7-Global Git Options*
*Context gathered: 2026-07-07*
