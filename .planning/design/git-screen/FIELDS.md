# FIELDS.md — git-screen (Phase 4, fan-out surface)

Per-screen field-parity manifest for the git-configuration screen
(02-UX-DIRECTION.md §4(2)'s 7 named states, lifted verbatim). This is the
DLV-01 spec: authored BEFORE the mockup/dummy screens, doubling as their
contract. `agent-ui-ux-designer` fills the **HTML present** / **TUI
present** columns after both media exist and both screenshot sets are
captured (Task 3) — same discipline as `.planning/design/create-flow/FIELDS.md`
(02-04, the pilot).

The machine-checkable gate is `.planning/design/git-screen/parity.json` (§3
dimensions + the `allowed-signers-byte-identity` and
`match-strategy-default-gitdir` rows) — this document is its human-readable
companion.

git-screen is a **keyless modal flow launched FROM Identities** via the
LaunchKey `g` (02-UX-DIRECTION.md §2 key-allocation table; `internal/dummytui/doc.go`
mirrors it). Every screen's `htmlRoute` is `/git-screen/<screen>`; every
route title and TUI breadcrumb is `git-screen/<screen>`.

---

## git-screen / git-form-empty

**Goal:** the per-identity Git fragment form before any field is filled
(GITUI-01/02). This screen appears AFTER the SSH screens (create-flow), for
the identity just created.

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `user_name` | "user.name" | 1st | ✓ | ✓ | empty |
| 2 | `user_email` | "user.email" | 2nd | ✓ | ✓ | empty; this value is the one that MUST end up byte-identical to `allowed_signers` (GITUI-04) |
| 3 | `gpg_format` | "gpg.format" | 3rd, fixed | ✓ | ✓ | always `ssh`, non-editable — gitid signs via SSH keys, no GPG |
| 4 | `user_signingkey` | "user.signingkey" | 4th | ✓ | ✓ | a PATH to the public key, never key material |
| 5 | `commit_gpgsign` | "commit.gpgsign" | 5th, default true | ✓ | ✓ | toggle |

## git-screen / git-form-filled

**Goal:** the fragment form filled, with a live fragment-text preview
(mirrors the guided-form + live-preview archetype, §2).

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `user_name` | "user.name" | 1st | ✓ | ✓ | "Personal Identity" |
| 2 | `user_email` | "user.email" | 2nd | ✓ | ✓ | "you@personal.example" |
| 3 | `gpg_format` | "gpg.format" | 3rd, fixed | ✓ | ✓ | "ssh" |
| 4 | `user_signingkey` | "user.signingkey" | 4th | ✓ | ✓ | "~/.ssh/id_ed25519_personal.pub" — path, not literal key material |
| 5 | `commit_gpgsign` | "commit.gpgsign" | 5th | ✓ | ✓ | "true" |
| 6 | `live_preview` | live fragment-text preview | right pane | ✓ | ✓ | exact recipe-accurate `[user]`/`[gpg]`/`[commit]` block text |

## git-screen / match-strategy-select

**Goal:** choose how the fragment is wired into `~/.gitconfig` (GITUI-03).

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `match_strategy` | "gitdir:" | 1st, DEFAULT | ✓ | ✓ | selected/highlighted by default |
| 2 | `match_strategy` | "hasconfig:remote.\*.url" | 2nd | ✓ | ✓ | combinable with gitdir |
| 3 | `match_strategy` | "both" | 3rd | ✓ | ✓ | applies both `includeIf` blocks |
| 4 | `includeif_preview` | live `includeIf` preview | right pane / below | ✓ | ✓ | updates to reflect the selected strategy; shown for `gitdir` (default) |

## git-screen / review-readonly

**Goal:** the read-only review before write — fragment + `includeIf` +
`allowed_signers` shown TOGETHER (GITUI-05), with the highest-risk
affordance: `allowed_signers`'s email is BYTE-IDENTICAL to `user.email`
(GITUI-04), shown side by side so a mismatch is visible.

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `fragment_block` | the Git fragment text | 1st | ✓ | ✓ | `[user]`/`[gpg]`/`[commit]`, sentinels visible |
| 2 | `includeif_block` | the selected-strategy `includeIf` block | 2nd | ✓ | ✓ | default `gitdir` |
| 3 | `allowed_signers_line` | the `~/.ssh/allowed_signers` line | 3rd | ✓ | ✓ | shown SIDE BY SIDE with `user_email_echo` |
| 4 | `user_email_echo` | `user.email` repeated for comparison | alongside #3 | ✓ | ✓ | byte-identical highlight/assertion — GITUI-04's highest-risk affordance |

## git-screen / confirm-write

**Goal:** mutation-ceremony beats 1+2 (§5) — preview + confirm, across all
THREE files this screen writes (GITUI-05).

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `fragment_preview` | exact fragment-file contents, sentinels visible | 1st | ✓ | ✓ | target `~/.gitconfig.d/personal` |
| 2 | `gitconfig_preview` | exact `includeIf` block appended to `~/.gitconfig`, sentinels visible | 2nd | ✓ | ✓ | target `~/.gitconfig` |
| 3 | `allowed_signers_preview` | exact `allowed_signers` line | 3rd | ✓ | ✓ | target `~/.ssh/allowed_signers` |
| 4 | `confirm_action` | explicit confirm keystroke | 4th | ✓ | ✓ | non-destructive create, still an explicit confirm |
| 5 | `nothing_changed_note` | "nothing has changed yet" | below previews | ✓ | ✓ | |

## git-screen / backup-notice

**Goal:** beat 3 — the timestamped backup path(s), for every existing file
this screen mutates (§5).

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `gitconfig_backup_path` | `~/.gitconfig` backup path | 1st | ✓ | ✓ | timestamped |
| 2 | `allowed_signers_backup_path` | `~/.ssh/allowed_signers` backup path | 2nd | ✓ | ✓ | timestamped |
| 3 | `backup_explainer` | "the backup is the undo story" copy | below paths | ✓ | ✓ | |

## git-screen / result-success

**Goal:** beat 4 — the success result (§5).

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `result_glyph` | green `✓` | 1st | ✓ | ✓ | glyph + word, never color alone |
| 2 | `result_message` | what changed + which files | 2nd | ✓ | ✓ | names the fragment file, `~/.gitconfig`, and the match strategy applied |
| 3 | `restore_hint` | both backup paths again (how to restore) | 3rd | ✓ | ✓ | repeats the backup-notice paths |
