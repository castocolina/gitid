# FIELDS.md — fixer (Phase 8, fan-out surface)

Per-screen field-parity manifest for the Fixer screen (view `5`,
02-UX-DIRECTION.md §4.7's 6 named states, lifted verbatim). This is the
DLV-01 spec: authored BEFORE the mockup/dummy screens, doubling as their
contract. `agent-ui-ux-designer` fills the **HTML present** / **TUI present**
columns after both media exist and both screenshot sets are captured
(Task 3) — same discipline as `.planning/design/create-flow/FIELDS.md`
(02-04, the pilot), `.planning/design/git-screen/FIELDS.md` (02-05),
`.planning/design/identity-manager/FIELDS.md` (02-06),
`.planning/design/global-ssh/FIELDS.md` (02-07),
`.planning/design/global-git/FIELDS.md` (02-08), and
`.planning/design/health/FIELDS.md` (02-09, the closest sibling — the
fixer is health's write-side counterpart).

The machine-checkable gate is `.planning/design/fixer/parity.json` (§3
dimensions + the `fix-in-place-diff-and-backup` and
`nothing-to-fix-empty-state` rows) — this document is its human-readable
companion.

fixer is a **master-detail** surface (§2 body archetype) on number key
`5` (`SurfaceDef.ActivationKey`), registered via `RegisterOrReplace`
(review HIGH-2), replacing the 02-02 placeholder. Every screen's
`htmlRoute` is `/fixer/<screen>`; every route title and TUI breadcrumb is
`fixer/<screen>`. Intra-surface `ScreenDef.Keys` allocate a linear
ceremony chain `v` (→ fix-preview), `x` (→ confirm-destructive), `y` (→
backup-notice), `z` (→ result-applied) from the entry screen, plus `e`
(→ nothing-to-fix, mirroring identity-manager's `list-empty` "e"
allocation and health's own alternate-state key) — never `n`/`g`
(create-flow's/git-screen's own LaunchKeys, the only two globally
reserved letters in the 02-UX-DIRECTION.md §2 key-allocation table — the
registry.go registration-time collision guard rejects any clash loudly).

**FIX-01/FIX-02 pinned here:** the fixer presents SSH and Git problems in
two sections (FIX-02), each with severity + plain explanation + suggested
fix (FIX-01), and applies fixes only with confirm + backup (FIX-01). It
lists the SAME `healthFindings`/`hlthFindings` Health diagnosed
(`fixerFindings`, the subset carrying a `suggestedFix`) — traceable, not
re-derived, honoring HLTH-04's own "available on the Fixer screen"
hand-off text.

**Highest-risk affordance (§4.7): fix-in-place rewrites of EXISTING
directives.** The flagship walk-through target
(`fixerTarget`/`fixerTarget` in Go) is the SAME
`ssh-identitiesonly-contradiction` finding health/finding-detail
deep-dives: rewriting `IdentitiesOnly no` to `IdentitiesOnly yes` on an
EXISTING `Host clientb.github.com` block — a true before/after diff
(`-`/`+` lines), not an additions-only `+` list (unlike global-ssh/
global-git's fix-preview). `confirm-destructive` uses the strongest
confirm this medium allows short of a typed confirmation (mirrors
identity-manager's "delete everything" precedent) — destructive actions
never default-focus "yes" (§5). `backup-notice` names the timestamped
backup path BEFORE applying. A batch-fix is offered (`fixerBatchFixNote`)
but explicitly still previews every change — no 7th named state is added
for it, per §4.7's exact 6-state list.

---

## fixer / fixer-list (entry screen)

**Goal:** the default fixer view — SSH and Git sections, each listing its
fixable problems (severity + plain explanation + suggested fix), with a
right-pane preview of the highlighted problem and a batch-fix note.

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `safety_banner` | "Every fix is previewed, confirmed, and backed up…" | top of body | ✓ | ✓ | the fix-in-place safety affordance, stated explicitly on every screen |
| 2 | `ssh_section` | "SSH" heading + problems | left pane, first | ✓ | ✓ | FIX-02 |
| 3 | `git_section` | "Git" heading + problems | left pane, second | ✓ | ✓ | FIX-02 |
| 4 | `problem_row` | severity glyph + word + title + suggested fix | severity order, per section | ✓ | ✓ | reuses `healthFindings`' severity/title/family |
| 5 | `problem_severity_glyph` | `~`/`!`/`✗` + word | leading each row | ✓ | ✓ | never color alone; LOCKED contract reused from health |
| 6 | `detail_preview` | highlighted problem's full explanation + suggested fix | right pane (master-detail) | ✓ | ✗ (compact keybar hint instead) | HTML shows a full right-pane preview (master-detail archetype, §2); TUI's live-PTY viewport budget replaces it with a one-line "v preview fix" hint — an accepted §3 "widget mechanics may differ" compaction, the same class health/global-git already established |
| 7 | `batch_fix_note` | "Apply all N fixes — each one still previews…" | bottom of right pane | ✓ | ✓ | §4.7 batch-fix-still-previews rule, stated explicitly |

## fixer / fix-preview

**Goal:** mutation-ceremony beat 1 (§5) — the read-only before/after diff
of the exact change to an EXISTING directive (the flagship highest-risk
affordance).

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `rewrite_banner` | "This fix REWRITES a directive already present…" | 1st | ✓ | ✓ | names the target file |
| 2 | `fix_diff` | `-`/`+` before/after diff lines | 2nd | ✓ | ✓ | true rewrite diff, not additions-only |
| 3 | `context_note` | "Only the highlighted line changes…" | 3rd | ✓ | ✓ | |

## fixer / confirm-destructive

**Goal:** mutation-ceremony beat 2 (§5), specific to fix-in-place
rewrites of existing directives — the strongest confirm this medium
allows.

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `rewrite_warning` | "This rewrites a directive already present…cannot be undone…" | 1st | ✓ | ✓ | error-severity alert |
| 2 | `target_summary` | file + Host block + directive rewritten | 2nd | ✓ | ✓ | |
| 3 | `default_focus_note` | "Default-focused: No, cancel…" | 3rd | ✓ | ✓ | destructive actions never default to "yes" (§5) |

## fixer / backup-notice

**Goal:** mutation-ceremony beat 3 (§5) — the timestamped backup path,
named before applying.

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `ssh_config_backup_path` | `~/.ssh/config` backup path | 1st | ✓ | ✓ | timestamped |
| 2 | `backup_explainer` | "the backup is the undo story" copy | below path | ✓ | ✓ | |

## fixer / result-applied

**Goal:** mutation-ceremony beat 4 (§5) — what changed, in which file,
and how to restore.

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `result_success_message` | "✓ IdentitiesOnly set to yes…" | 1st | ✓ | ✓ | green ✓, never color alone |
| 2 | `result_preserved_note` | "Only the rewritten directive changed…preserved verbatim" | 2nd | ✓ | ✓ | |
| 3 | `result_backup_restore_path` | backup path again | 3rd | ✓ | ✓ | |

## fixer / nothing-to-fix

**Goal:** the healthy empty state (§4.7) — both sections report zero
fixable problems.

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `safety_banner` | "Every fix is previewed, confirmed, and backed up…" | top of body | ✓ | ✓ | same banner as fixer-list |
| 2 | `ssh_all_fixed` | "✓ SSH — 0 fixable problems…" | 1st | ✓ | ✓ | green ✓ + word |
| 3 | `git_all_fixed` | "✓ Git — 0 fixable problems…" | 2nd | ✓ | ✓ | green ✓ + word |
