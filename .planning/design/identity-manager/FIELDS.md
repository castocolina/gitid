# FIELDS.md — identity-manager (Phase 5, fan-out surface)

Per-screen field-parity manifest for the Identity Manager — the app's HOME
view (02-UX-DIRECTION.md §4(3)'s 8 named states, lifted verbatim). This is
the DLV-01 spec: authored BEFORE the mockup/dummy screens, doubling as their
contract. `agent-ui-ux-designer` fills the **HTML present** / **TUI present**
columns after both media exist and both screenshot sets are captured
(Task 3) — same discipline as `.planning/design/create-flow/FIELDS.md`
(02-04, the pilot) and `.planning/design/git-screen/FIELDS.md` (02-05).

The machine-checkable gate is `.planning/design/identity-manager/parity.json`
(§3 dimensions + the `delete-choice-safe-default`, `no_color-row-health`,
and `ssh-first-detail` rows) — this document is its human-readable
companion.

identity-manager is the **nav root** — the primary surface on number key `1`
(`SurfaceDef.ActivationKey`), registered via `RegisterOrReplace` (review
HIGH-2), replacing the 02-02 placeholder. Every screen's `htmlRoute` is
`/identity-manager/<screen>`; every route title and TUI breadcrumb is
`identity-manager/<screen>`. Intra-surface `ScreenDef.Keys` allocate `a`
(→ action-menu), `c` (→ clone-name-prompt), `d` (→ delete-choice), and `u`
(→ register-key-modal, D-08/D-09 Phase 9 amendment) from the
02-UX-DIRECTION.md §2 key-allocation table (the single authority) —
never `n`/`g`, which are create-flow's and git-screen's own LaunchKeys.

---

## identity-manager / list-populated (entry screen)

**Goal:** show every identity's health at a glance — the MGR-01/MGR-02
master list, per-row 8-label state taxonomy legible under `NO_COLOR`.

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `identity_row` × 8 | one row per MGR-02 label | top-to-bottom | ✓ | ✓ | `personal` (complete), `work` (incomplete), `opensource` (git-only), `archived` (key-unused), `staging` (key-used-ssh-only), `clientA` (key-used-both), `clientB` (key-missing), `legacy` (fragment-path-missing) |
| 2 | `row_glyph` | ✓ / ! / ✗ | leading each row | ✓ | ✓ | glyph + WORD, never color alone (NO_COLOR row) |
| 3 | `row_state_word` | the MGR-02 label itself | trailing the glyph | ✓ | ✓ | e.g. "key-missing", "fragment-path-missing" — verbatim vocabulary |
| 4 | `row_note` | one-line explanation | below/beside each row | ✓ | ✓ | why this identity is in this state |
| 5 | `header_context_chip` | identity count + global health | header, region 1 | ✓ | ✓ | 8 identities; global health rolls up to the worst row present |

## identity-manager / list-empty (first-run landing)

**Goal:** the true first-run landing state — designed, not an afterthought
(§4(3), §6 checklist item B).

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `empty_state_copy` | "No identities yet" + guidance | 1st | ✓ | ✓ | explicit empty-state copy, not a blank list |
| 2 | `empty_state_cta` | "Press n to create your first identity" | 2nd | ✓ | ✓ | points at create-flow's `n` LaunchKey |

## identity-manager / detail-ssh-first

**Goal:** SSH details first, then Git — **never** render Git attributes for
an SSH-only identity (MGR-03/MGR-07). Targets the `work` identity
(state `incomplete`, SSH-only) precisely to prove this rule.

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `ssh_section` | SSH details (Host, Hostname, Port, IdentityFile) | 1st | ✓ | ✓ | shown FIRST, per MGR-03 |
| 2 | `git_section_absent_note` | "No Git identity configured for this alias" | 2nd | ✓ | ✓ | explicit absence, never fabricated Git fields — the MGR-03/07 highest-value proof |
| 3 | `per_identity_health` | this identity's own health slice | alongside | ✓ | ✓ | MGR-07 |

## identity-manager / action-menu

**Goal:** the hub of per-identity actions, opened via `a` from the list or
detail screen.

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `action_view_detail` | "View SSH-first detail" | 1st | ✓ | ✓ | |
| 2 | `action_clone` | "Clone (c)" | 2nd | ✓ | ✓ | → clone-name-prompt |
| 3 | `action_new_key` | "Generate new key" | 3rd | ✓ | ✓ | MGR-05 (referenced, not a separate named state in §4(3)) |
| 4 | `action_delete` | "Delete (d)" | 4th | ✓ | ✓ | → delete-choice |
| 5 | `action_register_key` | "Register key (u)" | 5th | n/a | ✓ | → register-key-modal; new key `u` claimed in `02-UX-DIRECTION.md` §2 (D-08/D-09 Phase 9 amendment) |

## identity-manager / register-key-modal

**Goal:** the manual re-trigger surface for an identity that landed on
`ReachableNotUploaded` and was never auto-uploaded (D-08's "copy modal").
Reached from the action-menu's `action_register_key` row.

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `modal_heading` | `"Register <name>'s key with <Provider>"` | 1st | n/a | ✓ | the frozen `RegisterKeyModalHeadingFmt` |
| 2 | `upload_running_line` × 1-2 | `"Running: <exact command>"` | 2nd | n/a | ✓ | same shared upload-section render as create-flow's `upload-announcing` |
| 3 | `upload_result_row` × 1-2 | result rows | 3rd | n/a | ✓ | same shared upload-section render as create-flow's `upload-results` |
| 4 | `upload_manual_fallback` | `UploadManualHeading` + `internal/upload.Instructions(provider)` | conditional, after result rows | n/a | ✓ | conditional; the D-01 checkbox is OMITTED here — opening the modal IS the explicit opt-in, so it announces-and-runs immediately when the provider matches and the tool is authenticated, and jumps straight to the manual-fallback state otherwise |

## identity-manager / clone-name-prompt

**Goal:** clone the targeted identity into a **DISTINCT** new name (MGR-04)
— never a bare duplicate of the source name.

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `clone_source_name` | source identity name | 1st | ✓ | ✓ | "personal" |
| 2 | `clone_suggested_name` | suggested new name | 2nd | ✓ | ✓ | "personal-clone" — distinct from the source |
| 3 | `clone_distinct_note` | "must differ from the source name" | 3rd | ✓ | ✓ | MGR-04's explicit constraint |

## identity-manager / delete-choice

**Goal:** the surface's highest-risk affordance (§4(3), MGR-06) — two
destructive options; the **safer one is default-focused**.

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `delete_choice_git_only` | "Delete Git identity only" | 1st, **default-focused** | ✓ | ✓ | the safer option |
| 2 | `delete_choice_everything` | "Delete everything (SSH + Git + key)" | 2nd | ✓ | ✓ | irreversible — never default-focused |
| 3 | `delete_choice_target` | the identity being deleted | above the choices | ✓ | ✓ | "personal" |

## identity-manager / confirm-destructive

**Goal:** beat 2 of the mutation ceremony (§5), specific to the irreversible
"everything" path — the **strongest confirm the medium allows**.

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `confirm_warning` | "This cannot be undone" + what will be removed | 1st | ✓ | ✓ | names SSH Host block + Git fragment + key file |
| 2 | `confirm_default_no` | default-focused "No" / explicit typed confirm | 2nd | ✓ | ✓ | destructive actions never default to "yes" (§5) |

## identity-manager / backup-notice

**Goal:** beat 3 of the mutation ceremony (§5) — the timestamped backup
path(s) for every file this delete touches.

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `ssh_config_backup_path` | `~/.ssh/config` backup path | 1st | ✓ | ✓ | timestamped |
| 2 | `gitconfig_backup_path` | `~/.gitconfig` backup path | 2nd | ✓ | ✓ | timestamped |
| 3 | `backup_explainer` | "the backup is the undo story" copy | below paths | ✓ | ✓ | |
