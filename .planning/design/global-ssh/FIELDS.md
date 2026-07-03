# FIELDS.md — global-ssh (Phase 6, fan-out surface)

Per-screen field-parity manifest for the Global SSH options screen (view `2`,
02-UX-DIRECTION.md §4.4's 6 named states, lifted verbatim). This is the
DLV-01 spec: authored BEFORE the mockup/dummy screens, doubling as their
contract. `agent-ui-ux-designer` fills the **HTML present** / **TUI present**
columns after both media exist and both screenshot sets are captured
(Task 3) — same discipline as `.planning/design/create-flow/FIELDS.md`
(02-04, the pilot), `.planning/design/git-screen/FIELDS.md` (02-05), and
`.planning/design/identity-manager/FIELDS.md` (02-06).

The machine-checkable gate is `.planning/design/global-ssh/parity.json`
(§3 dimensions + the `per-option-explanation-verbatim` and
`advisory-not-blocking` rows) — this document is its human-readable
companion.

global-ssh is a **master-detail** surface (§2 body archetype) on number key
`2` (`SurfaceDef.ActivationKey`), registered via `RegisterOrReplace` (review
HIGH-2), replacing the 02-02 placeholder. Every screen's `htmlRoute` is
`/global-ssh/<screen>`; every route title and TUI breadcrumb is
`global-ssh/<screen>`. Intra-surface `ScreenDef.Keys` allocate `v` (→
option-detail), `f` (→ fix-preview), `w` (→ confirm-write), `y` (→
backup-notice), `z` (→ result-applied) — a linear ceremony chain, mirroring
git-screen's `f`/`m`/`r`/`w`/`y`/`z` precedent. None of these collide with
`n`/`g` (create-flow's/git-screen's own LaunchKeys, the only globally
reserved letters in the 02-UX-DIRECTION.md §2 key-allocation table).

**GSSH-01 pinned here:** the exact dangerous-by-default option set is
**StrictHostKeyChecking, ForwardAgent, HashKnownHosts, IdentitiesOnly,
AddKeysToAgent, UseKeychain** — closing REQUIREMENTS.md's previously-open
"GSSH-01 option list" item during this design phase (acceptable per
REQUIREMENTS.md "Still Open" note).

**Highest-risk affordance (§4.4):** recommendations are **ADVISORY, never
blocking** — a yellow `!`, never a red compliance gate, and the user may
leave any option unchanged. `options-list`/`fix-preview`/`confirm-write`/
`result-applied` demonstrate this concretely: the user applies 3 of the 4
"needs action" recommendations and deliberately leaves `ForwardAgent`
unchanged, visible through to the final result screen.

---

## global-ssh / options-list (entry screen)

**Goal:** review all 6 dangerous-by-default SSH options at a glance —
each with current value, risk, recommended value, and a one-line
explanation (GSSH-01).

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `option_row` × 6 | one row per GSSH-01 option | top-to-bottom, §4.4 verbatim order | ✓ | ✓ | StrictHostKeyChecking, ForwardAgent, HashKnownHosts, IdentitiesOnly, AddKeysToAgent, UseKeychain |
| 2 | `row_glyph` | ! / ✓ | leading each row | ✓ | ✓ | glyph + WORD ("recommended" / "already set"), never color alone — yellow `!` is ADVISORY, never red |
| 3 | `row_current_value` | "current: …" | within row | ✓ | ✓ | e.g. "not set (OpenSSH default: ask)" |
| 4 | `row_risk` | Low / Medium / High risk | within row | ✓ | ✓ | severity word, not a blocking color |
| 5 | `row_recommended_value` | "recommended: …" | within row | ✓ | ✓ | |
| 6 | `row_one_liner` | per-option explanation | below current/recommended | ✓ | ✗ (options-list only) | GSSH-01 "explains every option" — TUI omits this on options-list ONLY due to the fixed 80x24 live-PTY viewport (6 rows × 4 lines overflowed the terminal, see CRITIQUE.md finding #2); the SAME explanation is fully present, verbatim, on option-detail in both media, and every option's key/current/recommended/risk still appears on the TUI's options-list row |
| 7 | `advisory_banner` | "Recommended, not required…" | top of body | ✓ | ✓ | the advisory-not-blocking affordance, stated explicitly |
| 8 | `detail_preview` | highlighted option's current/risk/recommended | right pane (master-detail) | ✓ | ✗ (compact keybar hint instead) | HTML shows a full right-pane preview (master-detail archetype, §2); TUI's live-PTY viewport budget replaces it with a one-line "v full explanation (IdentitiesOnly)  f preview fix" hint naming the same target — an accepted §3 "widget mechanics may differ" compaction (CRITIQUE.md finding #2), the target option and the next-step keys are still identical |

## global-ssh / option-detail

**Goal:** the full, contractual (verbatim, §3) risk explanation for one
option — IdentitiesOnly, the highest-risk entry.

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `detail_current_risk_recommended` | current / risk chip / recommended | 1st | ✓ | ✓ | |
| 2 | `detail_explanation` | full multi-paragraph explanation | 2nd | ✓ | ✓ | contractual verbatim copy (GSSH-01) |
| 3 | `advisory_note` | "Recommended, not required…" | 3rd | ✓ | ✓ | restated per-screen, not just on options-list |

## global-ssh / fix-preview

**Goal:** mutation-ceremony beat 1 (§5) — the read-only diff of the exact
change, demonstrating the advisory affordance concretely (3 of 4 applied,
1 declined).

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `fix_summary_banner` | "Applying N of 4…" + declined option named | 1st | ✓ | ✓ | |
| 2 | `fix_diff` | `+`/unchanged/declined diff lines | 2nd | ✓ | ✓ | `Host *` block in `~/.ssh/config` |
| 3 | `managed_block_note` | "gitid only owns the block…" | 3rd | ✓ | ✓ | managed-block containment shown, not just asserted (§5) |

## global-ssh / confirm-write

**Goal:** mutation-ceremony beats 1+2 (§5) — the exact resulting text, with
target file named and sentinels visible; nothing has changed yet.

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `confirm_nothing_changed_banner` | "Nothing has changed yet…" | 1st | ✓ | ✓ | |
| 2 | `confirm_target_file` | `~/.ssh/config` named | within banner/body | ✓ | ✓ | |
| 3 | `confirm_managed_block_text` | sentinel-visible exact text | 2nd | ✓ | ✓ | `# BEGIN/END gitid managed: global-ssh` |
| 4 | `confirm_declined_note` | ForwardAgent absence explained | within banner | ✓ | ✓ | |

## global-ssh / backup-notice

**Goal:** mutation-ceremony beat 3 (§5) — the timestamped backup path.

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `ssh_config_backup_path` | `~/.ssh/config` backup path | 1st | ✓ | ✓ | timestamped |
| 2 | `backup_explainer` | "the backup is the undo story" copy | below path | ✓ | ✓ | |

## global-ssh / result-applied

**Goal:** mutation-ceremony beat 4 (§5) — what changed, in which file, and
how to restore; restates the declined option explicitly.

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `result_success_message` | "✓ 3 of 4 recommended options applied…" | 1st | ✓ | ✓ | green ✓, never color alone |
| 2 | `result_declined_restated` | "You can revisit ForwardAgent here any time…" | 2nd | ✓ | ✓ | advisory reaffirmed at the end of the ceremony |
| 3 | `result_backup_restore_path` | backup path again | 3rd | ✓ | ✓ | |
