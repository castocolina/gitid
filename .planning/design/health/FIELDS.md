# FIELDS.md — health (Phase 8, fan-out surface)

Per-screen field-parity manifest for the Health check screen (view `4`,
02-UX-DIRECTION.md §4.6's 5 named states, lifted verbatim). This is the
DLV-01 spec: authored BEFORE the mockup/dummy screens, doubling as their
contract. `agent-ui-ux-designer` fills the **HTML present** / **TUI present**
columns after both media exist and both screenshot sets are captured
(Task 3) — same discipline as `.planning/design/create-flow/FIELDS.md`
(02-04, the pilot), `.planning/design/git-screen/FIELDS.md` (02-05),
`.planning/design/identity-manager/FIELDS.md` (02-06),
`.planning/design/global-ssh/FIELDS.md` (02-07), and
`.planning/design/global-git/FIELDS.md` (02-08).

The machine-checkable gate is `.planning/design/health/parity.json` (§3
dimensions + the `ssh-git-two-section` and `read-only-integrity` rows) —
this document is its human-readable companion.

health is a **master-detail** surface (§2 body archetype) on number key
`4` (`SurfaceDef.ActivationKey`), registered via `RegisterOrReplace`
(review HIGH-2), replacing the 02-02 placeholder. Every screen's
`htmlRoute` is `/health/<screen>`; every route title and TUI breadcrumb is
`health/<screen>`. Intra-surface `ScreenDef.Keys` allocate `h` (→
health-all-green), `v` (→ finding-detail), `i` (→ per-identity-health), `x`
(→ parse-error) — all four reachable in one hop from the entry screen
(`health-with-findings`), never `n`/`g` (create-flow's/git-screen's own
LaunchKeys, the only globally reserved letters in the
02-UX-DIRECTION.md §2 key-allocation table).

**HLTH-01 pinned here:** every screen shows an **SSH section and a Git
section**, never merged. **HLTH-03/HLTH-04 pinned here:** the concrete
example findings are a duplicate `Host *` stanza (redundancy), `Host
clientb.github.com` setting `IdentitiesOnly no` alongside an explicit
`IdentityFile` (contradiction), and an `includeIf` in `~/.gitconfig`
targeting a missing `~/.gitconfig.d/legacy` fragment (contradiction).
**HLTH-05 pinned here:** `per-identity-health` computes the SAME slice
MGR-07's Identity Manager row badges derive from (the `legacy` identity,
reused byte-identically from `identityManagerRows`).

**Highest-risk affordance (§4.6): read-only integrity.** Health must
clearly *diagnose, not mutate* — it hands off to the Fixer (view `5`). NO
write ceremony (no confirm/backup/apply beat) appears anywhere on this
surface. Every one of the 5 screens carries the SAME explicit read-only
statement (`healthReadOnlyNote` / `hlthReadOnlyNote`), and this is
negatively asserted (review LOW-11): no confirm/backup/apply
write-ceremony marker string appears anywhere in the route files or the
dummy's rendered output for any of the 5 screens.

**Severity model (HLTH-06 substrate):** every finding carries one of
`internal/doctor/doctor.go`'s four `Severity` levels — `info` / `warning`
/ `error` / `critical` — under the LOCKED glyph contract: healthy = `✓`
green, info = `~` cyan, warning = `!` yellow, error/critical = `✗` red
(NEVER `✗` for warning; error and critical share the glyph and are
distinguished by the WORD). `healthFindings` (recipeFixtures.ts) /
`hlthFindings` (surface_health.go) demonstrate all four levels at once,
split across the SSH and Git sections.

---

## health / health-with-findings (entry screen)

**Goal:** the default health view — SSH and Git sections, each listing
its findings severity-sorted (critical → error → warning → info), with a
right-pane preview of the highlighted finding.

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `read_only_banner` | "Health only diagnoses…" | top of body | ✓ | ✓ | the read-only-integrity affordance, stated explicitly on every screen |
| 2 | `ssh_section` | "SSH" heading + findings | left pane, first | ✓ | ✓ | HLTH-01 |
| 3 | `git_section` | "Git" heading + findings | left pane, second | ✓ | ✓ | HLTH-01 |
| 4 | `finding_row` × 5 | severity glyph + word + title | severity-sorted within each section | ✓ | ✓ | critical → error → warning → info |
| 5 | `finding_severity_glyph` | `~`/`!`/`✗` + word | leading each row | ✓ | ✓ | never color alone; `✗` shared by error/critical, distinguished by word |
| 6 | `detail_preview` | highlighted finding's explanation + suggested fix | right pane (master-detail) | ✓ | ✗ (compact keybar hint instead) | HTML shows a full right-pane preview (master-detail archetype, §2); TUI's live-PTY viewport budget replaces it with a one-line "v full detail (IdentitiesOnly contradiction)" hint — an accepted §3 "widget mechanics may differ" compaction, the target finding and the next-step key are still identical |

## health / health-all-green

**Goal:** the healthy empty state — both sections report zero findings.

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `read_only_banner` | "Health only diagnoses…" | top of body | ✓ | ✓ | |
| 2 | `ssh_all_green` | "✓ SSH — …checked. All present…" | 1st | ✓ | ✓ | green ✓ + word |
| 3 | `git_all_green` | "✓ Git — …checked. Every fragment…" | 2nd | ✓ | ✓ | green ✓ + word |

## health / finding-detail

**Goal:** the full detail of one finding — the IdentitiesOnly/IdentityFile
contradiction (HLTH-04), the deep-dive target reached from
health-with-findings.

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `read_only_banner` | "Health only diagnoses…" | top of body | ✓ | ✓ | |
| 2 | `detail_severity_family` | severity + family chip | 1st | ✓ | ✓ | error, Coherence |
| 3 | `detail_explanation` | full contradiction explanation | 2nd | ✓ | ✓ | contractual verbatim copy (HLTH-04) |
| 4 | `detail_suggested_fix` | "available on the Fixer screen" | 3rd | ✓ | ✓ | names the hand-off, never itself a fix action |

## health / per-identity-health

**Goal:** the per-identity slice (HLTH-05) that feeds a Manager row —
targets the `legacy` identity (`fragment-path-missing`): SSH healthy, Git
broken.

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `read_only_banner` | "Health only diagnoses…" | top of body | ✓ | ✓ | |
| 2 | `identity_name` | "legacy" | 1st | ✓ | ✓ | |
| 3 | `identity_ssh_note` | "✓ Host block present…" | 2nd | ✓ | ✓ | green ✓ + word |
| 4 | `identity_git_finding` | the includeIf-missing-fragment finding, scoped | 3rd | ✓ | ✓ | SAME finding as health-with-findings' Git row — traceable, not re-derived |
| 5 | `identity_mgr_handoff_note` | "feeds the Identity Manager row for legacy" | 4th | ✓ | ✓ | HLTH-05 ↔ MGR-07 traceability, stated explicitly |

## health / parse-error

**Goal:** a config file that will not parse (HLTH-02) — the one condition
Health can only report, reinforcing read-only integrity concretely.

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `read_only_banner` | "Health only diagnoses…" | top of body | ✓ | ✓ | |
| 2 | `parse_error_file` | `~/.gitconfig.d/work` | 1st | ✓ | ✓ | |
| 3 | `parse_error_raw` | raw git parse error text | 2nd | ✓ | ✓ | |
| 4 | `parse_error_snippet` | the offending line | 3rd | ✓ | ✓ | |
| 5 | `parse_error_explanation` | "checks paused until it parses again" | 4th | ✓ | ✓ | |
