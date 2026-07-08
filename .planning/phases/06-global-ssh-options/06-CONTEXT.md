# Phase 6: Global SSH Options - Context

**Gathered:** 2026-07-07
**Status:** Ready for planning (plan AFTER Phase 5 executes — this phase needs
the app shell / view set and replaces the view-`2` demo screen; it also
refactors `internal/sshconfig` globals code the create ceremony exercises)

<domain>
## Phase Boundary

Wire the real backend behind the approved global-ssh design (view `2`, 6
frozen screens): review the 6 pinned dangerous-by-default options
(StrictHostKeyChecking, ForwardAgent, HashKnownHosts, IdentitiesOnly,
AddKeysToAgent, UseKeychain) with current value + provenance + risk +
recommended value + contractual verbatim explanation; apply user-selected
fixes through ONE combined backup + idempotent managed-block ceremony.
ADVISORY, never blocking — the frozen demo (apply 3 of 4, decline
ForwardAgent) is the affordance contract.

Requirement: GSSH-01 (+ DLV-04/DLV-06 UI-wave gates). Design FROZEN by
Phase 2 (`global-ssh/FIELDS.md`, 6 screens, keys `v/f/w/y/z`); the scoped
divergences decided below are documented + allowlisted per the established
precedent.

**Explicitly NOT in this phase:** global GIT options (Phase 7), health checks
on SSH artifacts beyond this screen's own re-verification (Phase 8 —
including "user UseKeychain line above the managed block" detection),
known_hosts rewriting (`ssh-keygen -H` is mentioned in copy, never run).

</domain>

<decisions>
## Implementation Decisions

### Effective-value detection & shadowing
- **D-01 — Three-probe diff for current values + provenance.** File parse
  (`internal/sshconfig`) + `ssh -G <probe>` (effective) + `ssh -G -F
  /dev/null <probe>` (compiled-defaults baseline — `-F` makes OpenSSH ignore
  `/etc/ssh/ssh_config`). Diffing the three classifies every option as
  user-set / system-set / OpenSSH default without reimplementing the
  first-obtained-value matcher and without `-vvv` scraping. Reuses
  `internal/tester`'s existing `-F` pattern (tester.go:162).
- **D-02 — UseKeychain is file-parse-only, by design.** VERIFIED: Apple's
  OpenSSH `ssh -G` never emits `usekeychain` even when set. The label mapper
  treats this as an explicit case with hedged provenance wording — not a
  bug found later.
- **D-03 — Provenance renders as three-tier labels** (frozen copy shape):
  "set by you at ~/.ssh/config line N" / "set in /etc/ssh/ssh_config —
  gitid cannot change this" / "not set (OpenSSH default: X)". When sources
  disagree, hedge ("set outside your config").
- **D-04 — Shadowing: pre-write simulation + post-write re-test.**
  Fix-preview writes the candidate config to a temp file (correct perms —
  ssh refuses group/world-writable) and runs `ssh -G -F <temp>`: if the
  intended value doesn't win, the preview says so honestly ("this fix will
  be shadowed and do nothing"), with a static parser scan used ONLY to name
  the shadowing line. Post-write, the existing chokepoint re-runs `ssh -G`
  and surfaces "applied but shadowed" as an ADVISORY finding. This is the
  project's prove-before-and-after loop, applied to globals.
- **D-05 — Probe host is a dummy `.invalid`-TLD name** (e.g.
  `gitid-probe.invalid`): matches only `Host *` + system + defaults, so the
  screen shows the honest GLOBAL value; `ssh -G` is offline so any name is
  safe. Per-alias resolution stays `internal/tester`'s job.

### Managed-block model (the create-ceremony collision)
- **D-06 — ONE gitid `Host *` block, key-union merge, shared owner.**
  CODE FINDING: the create ceremony whole-block-replaces the `_global`
  block on every create — a naive Phase 6 would have its fixes erased by
  the next identity created. Resolution: a single globals module owns the
  block; BOTH the create ceremony and the GSSH fix ceremony call the same
  `EnsureGlobals()`: parse the existing block body into an options map,
  overlay platform defaults only for ABSENT keys (existing values always
  win), render in canonical key order. GSSH fixes survive create
  re-normalization by construction; create's self-healing (deleted block
  regrows the trio) is retained. Golden-write tests guard parse→render
  round-trip stability.
- **D-07 — Placement: layout-follows-identities.** Last block of
  `~/.ssh/config.d/gitid.config` (Include'd layout, the default) or last
  gitid block of `~/.ssh/config` (in-file). Because Phase 3 floors the
  Include line at the TOP of `~/.ssh/config`, gitid's `Host *` precedes the
  user's own main-file directives — fixes actually take effect. The
  confirm-write screen names the RESOLVED target file (small allowlisted
  copy divergence; D-05-Phase-3 precedent). Residual in-file-layout
  shadowing is covered by D-04's re-verify, advisorily.
- **D-08 — Sentinel rename `_global` → `global-ssh` + registry
  consolidation.** The frozen confirm-write sentinel (`# BEGIN/END gitid
  managed: global-ssh`) is contractual. First write adopts/renames a legacy
  `_global` block; BOTH names are registered in
  `sshconfig.IsReservedBlockName`; the 4+ scattered `== "_global"` string
  literals (reader.go, overlap.go, migrate.go, identity/delete.go) are
  replaced with registry calls — closing the doctor reserved-block
  false-positive class the Phase-4-D-04 way.
- **D-09 — Ordering: identities-first / `Host *`-last** inside the gitid
  region (existing T-02-15 invariant, zero code change). RECIPE DIVERGENCE,
  documented explicitly: the recipe shows `Host *` at the top, but its top
  placement is inert (no key collides with a host block); gitid's block
  ordering is the only one that keeps per-alias values winning under
  first-match-wins.

### Platform set + pinned recommended values
- **D-10 — Pinned table (APPROVED):**

  | Option | Recommended | Risk | Core why |
  |---|---|---|---|
  | StrictHostKeyChecking | `accept-new` | Medium | pins first-seen keys, hard-fails on CHANGED key (the MITM signal); requires OpenSSH ≥ 7.6 |
  | ForwardAgent | `no` | High | remote root can authenticate as you anywhere; flags only an explicit `yes` (default is already no) |
  | HashKnownHosts | `yes` | Low | plaintext known_hosts leaks the host graph; copy notes hashing is NOT retroactive (`ssh-keygen -H` mentioned, never run) |
  | IdentitiesOnly | `yes` **per alias, never `Host *`** | High | first agent-accepted key wins = gitid's core failure mode; global scope breaks non-gitid hosts — recipe scopes per-alias |
  | AddKeysToAgent | `yes` | Low | default re-prompts passphrases, pressuring toward passphrase-less keys |
  | UseKeychain | `yes` (macOS, guarded) | Low | Apple-only; hard-errors unguarded on Linux |

  IdentitiesOnly's "fix" verifies per-alias conformance (create already
  writes it) — the row is verify/informational, never a `Host *` write.
  StrictHostKeyChecking/ForwardAgent/HashKnownHosts are BEYOND-recipe
  advisory hardening (consistent with CLAUDE.md's "and more") — documented,
  not recipe-conformance gaps.
- **D-11 — UseKeychain on Linux: show-disabled row + always-guard.** The
  row stays visible with a "macOS-only" note (preserves the frozen 6-row
  contract and shared visual baselines across OSes; needs a not-applicable
  row state). Whenever UseKeychain is written, `IgnoreUnknown UseKeychain`
  is written lexically BEFORE it inside the managed block — the recipe's
  own first-directive pattern; block ordering test required.
- **D-12 — Explicitly-set non-recommended values get a distinct WORD
  state:** same yellow `!` glyph, word "set, differs from recommendation" —
  danger stays visible (GSSH-01 is danger-aware) while deliberate choices
  aren't misrepresented as omissions. This extends the frozen §4.4
  glyph-word vocabulary and MUST be pinned before the copy freeze; the
  fix-preview "N of M" counting must define whether "differs" counts as
  needs-action (Claude's discretion, documented in the UI wave).
- **D-13 — Version-aware copy, split contractual/dynamic.** The frozen
  verbatim explanations embed only STATIC version facts ("accept-new
  requires OpenSSH 7.6+, 2017"); a separate NON-contractual field renders
  "your OpenSSH: X.Y" via the existing `internal/platform.ProbeSSHVersion()`
  and gates the accept-new fix on ≥ 7.6. The copy-freeze grep must not
  match the dynamic line.

### Advisory posture: persistence + selection
- **D-14 — Fully stateless, with derived states.** NO decline record of any
  kind (matches brew/flutter doctor precedent and MGR-08's files-are-the-
  state norm; the marker-comment alternative would force a write ceremony
  for a non-change and re-open the doctor false-positive wound). Any
  explicit value — recommended or not — flips the row to "already set" /
  "set, differs — your choice"; only truly-unset options stay yellow. The
  frozen copy "You can revisit ForwardAgent here any time" narrates exactly
  this.
- **D-15 — Selection starts EMPTY, opt-in.** Space toggles rows
  (established Bubble Tea multi-select pattern); `f` previews the selected
  set and is guarded when the selection is empty. The most honestly
  advisory default.
- **D-16 — ONE combined ceremony; toward-recommendations only.** The frozen
  fix-preview ("Applying 3 of 4", one combined diff) + linear `f→w→y→z`
  chain fix the model: one backup notice, one write, one result. No
  free-form value editor — the screen only moves options TOWARD
  recommendations; deliberate non-recommended values are hand-edited and
  then represented by D-12's state.

### Claude's Discretion
- Whether "set, differs" counts in the "N of M need action" tally (pin
  during the UI wave, before copy freeze).
- Temp-file location/perms for the D-04 simulation (must satisfy ssh's
  config-perm checks; use the scratchpad-style private dir).
- Exact copy for: the not-applicable UseKeychain row, "set, differs — your
  choice", the shadowed-fix warning, provenance labels, and the six
  contractual explanations (draft in the UI wave; freeze via 02-STYLE-SPEC
  §6; static version facts only).
- `gitid ssh` CLI subcommand shape (follows Phase 5 D-01..D-04: e.g.
  `gitid ssh options list --json` / `gitid ssh options fix --yes
  --dry-run`), recorded in the Phase 5 parity matrix.
- Legacy `_global` adoption/rename mechanics and migration test shape.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### North Star — canonical config end state
- `recipes/ssh-config.recipe` — the `IgnoreUnknown UseKeychain` guard
  (literal first directive), per-alias IdentitiesOnly scoping, `Host *`
  content (UseKeychain + AddKeysToAgent only). D-09's divergence and D-10's
  scoping derive from it.
- `recipes/README.md` — wiring overview.

### Requirements / roadmap (authoritative)
- `.planning/ROADMAP.md` §"Phase 6" — goal + 3 success criteria.
- `.planning/REQUIREMENTS.md` §I (GSSH-01), §H (MGR-08 files-are-the-state
  norm D-14 leans on), §A (DLV-04/06).

### Approved design (BINDING)
- `.planning/design/global-ssh/FIELDS.md` — the 6 frozen screens
  (options-list/option-detail/fix-preview/confirm-write/backup-notice/
  result-applied), keys `v/f/w/y/z`, the pinned 6-option set, the
  advisory-not-blocking affordance (3-of-4 demo), TUI viewport compactions
  (CRITIQUE.md finding #2), parity rows
  `per-option-explanation-verbatim` + `advisory-not-blocking`.
- `.planning/design/global-ssh/CRITIQUE.md` — accepted TUI compaction
  rationale.
- `.planning/phases/02-design-all-mockups-checkpoint-1/02-STYLE-SPEC.md` —
  theme roles, copy freeze §6 (D-12's new word state joins it), glyph+word
  never-color-alone rule.
- `.planning/phases/02-design-all-mockups-checkpoint-1/02-UX-DIRECTION.md`
  — §4.4 global-ssh states, §5 mutation ceremony beats, §2 key allocation.

### Upstream phase contracts
- `.planning/phases/03-create-flow-backend/03-CONTEXT.md` — Include'd
  layout default + floored Include line (D-07 relies on it); Host *
  re-normalization decision D-06 reconciles with; D-24/D-25 visual-gate +
  Codex review mechanics.
- `.planning/phases/05-identity-manager/05-CONTEXT.md` — D-01..D-04 CLI
  conventions this phase's `gitid ssh` commands must follow + parity-matrix
  obligation.
- `.planning/phases/04-git-configuration-screen/04-CONTEXT.md` — D-04
  reserved-registration precedent D-08 extends.

### Substrate (code findings that ground D-06/D-08)
- `internal/sshconfig/writer.go` — `_global` sentinel key + ordering
  invariant (identities-first, T-02-15).
- `internal/sshconfig/renderer.go:48-73` — `RenderGlobalBlock` (the
  darwin-only trio D-06's merge generalizes).
- `internal/sshconfig/include.go:37-93` — `IsReservedBlockName` (D-08's
  registry) + `EnsureIncludeLine` flooring.
- `internal/doctor/checks/redundancy.go:101-179` — consolidate-into-one-
  block advice (why two blocks is untenable);
  `internal/doctor/checks/orphans.go:52-61` — reserved-skip guard the new
  name joins.
- `internal/tester/tester.go:162` — the existing `-F` isolated-config
  pattern D-01/D-04 reuse; `ParseResolved` for `ssh -G` output.
- `internal/platform` — OS detection + `ProbeSSHVersion()` (D-11/D-13).
- `internal/filewriter/block.go` — idempotent block write (+ prepend,
  unused here by D-07).

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/tester` — `ssh -G` shelling + `-F` isolation + resolved-output
  parsing: D-01's probes and D-04's simulation are compositions of what
  exists.
- `internal/sshconfig` — parse/render/managed-block machinery; D-06 is a
  refactor of `RenderGlobalBlock`+`writer.go` into the shared globals
  module, not new parsing.
- `internal/platform.ProbeSSHVersion()` — D-13's dynamic line + accept-new
  gate (distro-suffix caveat already handled per project memory).
- `internal/tuikit` (Phase 3) + master-detail archetype — the options-list/
  option-detail screens compose existing frame/rows/ceremony components.
- Phase 5's CLI conventions — `gitid ssh options …` slots into the D-01
  noun-verb tree; `--json` list output reuses the D-03-Phase-5 pattern.

### Established Patterns
- Mutation ceremony beats (§5) — one combined ceremony (D-16) is the
  git-screen D-10-Phase-4 shape applied to globals.
- Copy-freeze §6 grep gates — new strings: provenance labels, "set,
  differs…", macOS-only note, shadowed-fix warning, six explanations
  (static facts only, D-13).
- Scoped-divergence discipline — D-07's resolved-file naming, D-12's word
  state, D-09's recipe-divergence note: all documented + allowlisted.
- Reserved-name registration for gitid-owned artifacts (D-08; Phase 4 D-04
  and Phase 5 D-06 precedents).

### Integration Points
- View `2` replaces its Phase 3 D-16 demo screen; Health/Fixer (views 4/5)
  keep banners until Phase 8.
- Create ceremony's globals write switches to the shared `EnsureGlobals()`
  (D-06) — create-flow call site changes; its e2e must prove GSSH fixes
  survive a subsequent create.
- Doctor: `global-ssh` + legacy `_global` join the reserved registry; the
  redundancy check's advice text updates to the new name.
- Phase 8 later consumes D-04's "applied but shadowed" advisory finding
  class and the "user UseKeychain above managed block" case as health
  checks.
- UI wave: `/gsd-ui-phase 6` for the divergence set + PTY e2e per screen +
  golden-text gate + agent-ui-ux-designer + Codex review (Phase 3
  D-24/D-25 conventions).

</code_context>

<specifics>
## Specific Ideas

- The phase's identity is the prove-before-and-after loop applied to global
  options: simulation in the preview, re-verification after the write —
  "advisory" includes being honest when a fix would be (or ended up)
  shadowed.
- Two verified platform facts must be designed in, not discovered:
  Apple `ssh -G` omits `usekeychain` (D-02); `IgnoreUnknown` must precede
  `UseKeychain` lexically (D-11).
- Advisory tone carries the stateless model (D-14): re-showing a declined
  recommendation is acceptable BECAUSE the copy never nags and "set,
  differs — your choice" respects deliberate values.

</specifics>

<deferred>
## Deferred Ideas

- **known_hosts hashing execution** (`ssh-keygen -H` run for the user) —
  Phase 8 fixer candidate; Phase 6 copy only mentions it.
- **"User UseKeychain/directive above the managed block" health check** —
  Phase 8 (HLTH-04 contradictions family), fed by D-04's finding class.
- **Per-alias drill-down of global options** (N×6 matrix) — only if a real
  need emerges; `internal/tester` covers per-alias resolution today.
- **Free-form value editor for SSH options** — out of scope for the
  advisory screen; would need a design re-freeze.
- **Preselected opt-out selection default** — revisit at UAT if the
  empty-start selection proves too many keystrokes (identical code path).

</deferred>

---

*Phase: 6-Global SSH Options*
*Context gathered: 2026-07-07*
