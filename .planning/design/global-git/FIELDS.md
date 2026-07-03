# FIELDS.md — global-git (Phase 7, fan-out surface)

Per-screen field-parity manifest for the Global Git options screen (view `3`,
02-UX-DIRECTION.md §4.5's 6 named states, lifted verbatim). This is the
DLV-01 spec: authored BEFORE the mockup/dummy screens, doubling as their
contract. `agent-ui-ux-designer` fills the **HTML present** / **TUI present**
columns after both media exist and both screenshot sets are captured
(Task 3) — same discipline as `.planning/design/create-flow/FIELDS.md`
(02-04, the pilot), `.planning/design/git-screen/FIELDS.md` (02-05),
`.planning/design/identity-manager/FIELDS.md` (02-06), and
`.planning/design/global-ssh/FIELDS.md` (02-07, the closest sibling — same
6-state shape and master-detail archetype).

The machine-checkable gate is `.planning/design/global-git/parity.json`
(§3 dimensions + the `main-vs-master-highlight` and
`managed-block-containment` rows) — this document is its human-readable
companion.

global-git is a **master-detail** surface (§2 body archetype) on number key
`3` (`SurfaceDef.ActivationKey`), registered via `RegisterOrReplace` (review
HIGH-2), replacing the 02-02 placeholder. Every screen's `htmlRoute` is
`/global-git/<screen>`; every route title and TUI breadcrumb is
`global-git/<screen>`. Intra-surface `ScreenDef.Keys` allocate `v` (→
option-detail), `f` (→ fix-preview), `w` (→ confirm-write), `y` (→
backup-notice), `z` (→ result-applied) — the SAME linear ceremony chain as
global-ssh's own `v`/`f`/`w`/`y`/`z` allocation (both surfaces reuse the
identical letters because they are never the active surface simultaneously
— each key is scoped to the surface's own `ScreenDef.Keys`, not global).
None of these collide with `n`/`g` (create-flow's/git-screen's own
LaunchKeys, the only globally reserved letters in the 02-UX-DIRECTION.md §2
key-allocation table).

**GGIT-01 pinned here:** the exact 11-option baseline + recipe-defaults set,
in §4.5's verbatim order — `init.defaultBranch` (highlighting **main vs
master**), `core.ignorecase`, `core.autocrlf`/`core.eol`, global
`user.email`, `push.autoSetupRemote`, `pull.rebase`, `fetch.prune`,
`alias` (8 shortcuts), `color` (ui/branch/diff/status), `merge.conflictstyle`,
`diff.colorMoved`. Values are drawn directly from the shared
`globalGitDefaults`/`globalGitDefaultsBlockText` fixture (02-01/02-02,
unmodified) plus `recipes/gitconfig.recipe`'s own `~/.gitconfig_default`
example block for the fields the shared fixture does not cover (autocrlf,
eol, aliases, color, user.email).

**Highest-risk affordance (§4.5):** writes must **preserve content outside
managed blocks verbatim** (GGIT-01) — `confirm-write` renders the
`# BEGIN/END gitid managed:` sentinels so the user sees their hand-written
`[user]`/`[includeIf]`/`[url]` sections are untouched. §5 also applies the
"advisory, never blocking" rule to global-git (the SAME two-surfaces
sentence that governs global-ssh): recommendations are a yellow `!`,
dismissible, and the user may apply none — `options-list`/`option-detail`/
`fix-preview`/`confirm-write` reuse global-ssh's own advisory visual
language for consistency (§2 "one color semantics table, applied
everywhere").

---

## global-git / options-list (entry screen)

**Goal:** review all 11 baseline + recipe-default git options at a glance —
each with current value, recommended value, and a one-line explanation
(GGIT-01).

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `option_row` × 11 | one row per GGIT-01 option | top-to-bottom, §4.5 verbatim order | ✓ | ✓ | init.defaultBranch, core.ignorecase, core.autocrlf/eol, user.email (global), push.autoSetupRemote, pull.rebase, fetch.prune, alias, color, merge.conflictstyle, diff.colorMoved |
| 2 | `row_glyph` | ! / ✓ | leading each row | ✓ | ✓ | glyph + WORD ("recommended" / "informational"), never color alone — yellow `!` is ADVISORY, never red |
| 3 | `row_current_value` | "current: …" | within row | ✓ | ✓ | e.g. "not set (git's built-in default: master)" |
| 4 | `row_recommended_value` | "recommended: …" | within row | ✓ | ✓ | |
| 5 | `row_one_liner` | per-option explanation | below current/recommended | ✓ | ✗ (options-list only) | GGIT-01 "each option explained" — TUI omits this on options-list ONLY due to the fixed 80×24 live-PTY viewport (mirrors global-ssh CRITIQUE.md finding #2, the SAME accepted class of divergence — see CRITIQUE.md finding #2 below); the SAME explanation is fully present, verbatim, on option-detail in both media, and every option's key/current/recommended still appears on the TUI's options-list row |
| 6 | `advisory_banner` | "Recommended, not required…" | top of body | ✓ | ✓ | the advisory-not-blocking affordance, stated explicitly |
| 7 | `main_vs_master_highlight` | "main vs master" chip | on the `init.defaultBranch` row | ✓ | ✓ | GGIT-01's dedicated highlight — a distinct `Chip`/bracket marker, not just the shared glyph+word treatment every other row gets |
| 8 | `detail_preview` | highlighted option's current/recommended | right pane (master-detail) | ✓ | ✗ (compact keybar hint instead) | HTML shows a full right-pane preview (master-detail archetype, §2); TUI's live-PTY viewport budget replaces it with a one-line "v full explanation (init.defaultBranch)  f preview fix" hint naming the same target — an accepted §3 "widget mechanics may differ" compaction, the same class global-ssh's options-list already established |

## global-git / option-detail

**Goal:** the full, contractual (verbatim, §3) explanation for one option —
`init.defaultBranch`, the option carrying the main-vs-master highlight.

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `detail_current_recommended` | current / main-vs-master chip / recommended | 1st | ✓ | ✓ | |
| 2 | `detail_explanation` | full multi-paragraph explanation | 2nd | ✓ | ✓ | contractual verbatim copy (GGIT-01) |
| 3 | `advisory_note` | "Recommended, not required…" | 3rd | ✓ | ✓ | restated per-screen, not just on options-list |

## global-git / fix-preview

**Goal:** mutation-ceremony beat 1 (§5) — the read-only diff of the exact
change to the managed block in `~/.gitconfig`, with global `user.email`
explicitly called out as intentionally absent (a structural gitid rule, not
a user decline).

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `fix_summary_banner` | "Applying 10 of 10…" + `user.email` absence explained | 1st | ✓ | ✓ | |
| 2 | `fix_diff` | `+` diff lines, section by section | 2nd | ✓ | ✓ | the managed block in `~/.gitconfig` |
| 3 | `managed_block_note` | "gitid only owns the block…" | 3rd | ✓ | ✓ | managed-block containment shown, not just asserted (§5) |

## global-git / confirm-write

**Goal:** mutation-ceremony beats 1+2 (§5) — the exact resulting text, with
target file named and sentinels visible; nothing has changed yet. The
surface's own highest-risk affordance (GGIT-01).

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `confirm_nothing_changed_banner` | "Nothing has changed yet…" | 1st | ✓ | ✓ | |
| 2 | `confirm_target_file` | `~/.gitconfig` named | within banner/body | ✓ | ✓ | |
| 3 | `confirm_managed_block_text` | sentinel-visible exact text | 2nd | ✓ | ✓ | `# BEGIN/END gitid managed: global-git` |
| 4 | `confirm_outside_block_preserved_note` | "everything else…is preserved verbatim" | within banner/body | ✓ | ✓ | names `[user]`/`[includeIf]`/`[url]` explicitly as examples of what is untouched |

## global-git / backup-notice

**Goal:** mutation-ceremony beat 3 (§5) — the timestamped backup path.

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `gitconfig_backup_path` | `~/.gitconfig` backup path | 1st | ✓ | ✓ | timestamped |
| 2 | `backup_explainer` | "the backup is the undo story" copy | below path | ✓ | ✓ | |

## global-git / result-applied

**Goal:** mutation-ceremony beat 4 (§5) — what changed, in which file, and
how to restore; restates that global `user.email` was left alone.

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `result_success_message` | "✓ 10 of 10 baseline options applied…" | 1st | ✓ | ✓ | green ✓, never color alone |
| 2 | `result_user_email_restated` | "Global user.email was left alone…" | 2nd | ✓ | ✓ | the structural-rule affordance reaffirmed at the end of the ceremony |
| 3 | `result_backup_restore_path` | backup path again | 3rd | ✓ | ✓ | |
