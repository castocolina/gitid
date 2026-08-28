# FIELDS.md — create-flow (pilot surface)

Per-screen field-parity manifest for the create-identity flow (02-UX-DIRECTION.md
§4.1's 12 named states, lifted verbatim). This is the DLV-01 spec: it is authored
BEFORE the mockup/dummy screens and doubles as their contract. `agent-ui-ux-designer`
fills the **HTML present** / **TUI present** columns after both media exist and both
screenshot sets are captured (Task 3).

The machine-checkable gate is `.planning/design/create-flow/parity.json` (§3
dimensions + the `test-confirm-backup-boundary` row) — this document is its
human-readable companion.

---

## create-flow / algo-catalog

**Goal:** pick a key algorithm — top-5 catalog, ed25519 best/default (KEY-01/KEY-03).

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `algo_id` | "ed25519" | 1st, recommended | ✓ | ✓ | default/best, highlighted |
| 2 | `algo_id` | "ed25519-sk" | 2nd | ✓ | ✓ | hardware/FIDO2, needs libfido2 |
| 3 | `algo_id` | "rsa-4096" | 3rd | ✓ | ✓ | |
| 4 | `algo_id` | "ecdsa-p256" | 4th | ✓ | ✓ | |
| 5 | `algo_id` | "ecdsa-sk" | 5th | ✓ | ✓ | hardware/FIDO2, ECDSA |
| 6 | `security_note` | per-algorithm security copy | alongside each row | ✓ | ✓ | |
| 7 | `macos_availability` | macOS local-availability note | alongside each row | ✓ | ✓ | |
| 8 | `linux_availability` | Linux local-availability note | alongside each row | ✓ | ✓ | |

## create-flow / ssh-form-empty

**Goal:** the SSH form before any field is filled (SSHUI-01/02).

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `alias_prefix` | "Alias prefix" | 1st | ✓ | ✓ | empty |
| 2 | `ssh_host` | "SSH Host" | 2nd | ✓ | ✓ | empty, auto-joins once alias_prefix is set |
| 3 | `real_hostname` | "Real hostname" | 3rd | ✓ | ✓ | empty |
| 4 | `port` | "Port" | 4th, default 443 | ✓ | ✓ | pre-filled 443 even when other fields are empty |

## create-flow / ssh-form-filled

**Goal:** the SSH form filled, with a live `Host` block preview (SSHUI-03).

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `alias_prefix` | "Alias prefix" | 1st | ✓ | ✓ | "personal" |
| 2 | `ssh_host` | "SSH Host" | 2nd | ✓ | ✓ | "personal.github.com", auto-joined, editable |
| 3 | `real_hostname` | "Real hostname" | 3rd | ✓ | ✓ | "ssh.github.com" |
| 4 | `port` | "Port" | 4th, default 443 | ✓ | ✓ | 443 |
| 5 | `live_preview` | live `Host` block preview | right pane | ✓ | ✓ | exact recipe-accurate text, incl. `IdentitiesOnly yes` |
| 6 | `auto_upload_checkbox` | "Register with `<Provider>` automatically" | last row of the SSH form, after Port | n/a¹ | ✓ | D-08 amendment (09-UI-SPEC.md): (a) renders only when `Provider` matches `github`/`gitlab` (D-13 main-domain scope); (b) three visual states per D-01 (checked+enabled / unchecked+enabled / disabled); (c) a real Tab-ring focus slot with a `u` mnemonic (see 02-UX-DIRECTION.md §2's key-allocation table and 09-02's focus-order decision) — `u` is a shortcut, not the only affordance, the checkbox row is always Tab- and click-reachable; (d) one row, never wrapping — the disabled/hint reason is inline on the SAME row (09-UI-SPEC.md "Focal points", first bullet) |

¹ HTML present is `n/a` — the HTML mockup has no upload surface (Phase 9 has no Phase-2 HTML precedent), same convention `create-flow / reuse-key-vs-generate` uses for TUI-only states.

## create-flow / ssh-form-blank-prefix

**Goal:** blank-prefix WYSIWYG — `SSH Host` = the provider host verbatim, no invented
suffix (SSHUI-01, SSHUI-03).

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `alias_prefix` | "Alias prefix" | 1st | ✓ | ✓ | blank |
| 2 | `ssh_host` | "SSH Host" | 2nd | ✓ | ✓ | "github.com" — the provider host verbatim, not "<blank>.github.com" |
| 3 | `wysiwyg_note` | explanatory copy on the blank-prefix rule | below the field | ✓ | ✓ | |
| 4 | `auto_upload_checkbox` | "Register with `<Provider>` automatically" | last row of the SSH form, after Port | n/a¹ | ✓ | D-08 amendment (09-UI-SPEC.md): (a) renders only when `Provider` matches `github`/`gitlab` (D-13 main-domain scope); (b) three visual states per D-01 (checked+enabled / unchecked+enabled / disabled); (c) a real Tab-ring focus slot with a `u` mnemonic (see 02-UX-DIRECTION.md §2's key-allocation table and 09-02's focus-order decision) — `u` is a shortcut, not the only affordance, the checkbox row is always Tab- and click-reachable; (d) one row, never wrapping — the disabled/hint reason is inline on the SAME row (09-UI-SPEC.md "Focal points", first bullet) |

¹ HTML present is `n/a` — the HTML mockup has no upload surface (Phase 9 has no Phase-2 HTML precedent), same convention `create-flow / reuse-key-vs-generate` uses for TUI-only states.

## create-flow / reuse-key-vs-generate

**Goal:** choose between reusing an existing key or generating a new one (KEY-06).

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `key_choice` | "Generate a new key" | 1st | ✓ | ✓ | |
| 2 | `key_choice` | "Reuse an existing key" | 2nd | ✓ | ✓ | requires an existing key-file path |

## create-flow / macos-globals-block

**Goal:** show the `Host *` UseKeychain/AddKeysToAgent globals guarded by
`IgnoreUnknown` (SSHUI-05).

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `ignore_unknown` | `IgnoreUnknown UseKeychain` | 1st line | ✓ | ✓ | Linux no-op guard |
| 2 | `host_star` | `Host *` | 2nd | ✓ | ✓ | |
| 3 | `use_keychain` | `UseKeychain yes` | 3rd | ✓ | ✓ | macOS-only semantics explained |
| 4 | `add_keys_to_agent` | `AddKeysToAgent yes` | 4th | ✓ | ✓ | |

## create-flow / test-stage1-direct

**Goal:** stage 1 of the two-stage test — direct against the provider URL, no alias
(TEST-01).

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `test_command` | exact command run | 1st | ✓ | ✓ | `ssh -T -F <tmp> ...` against throwaway config |
| 2 | `test_output` | real command output | 2nd | ✓ | ✓ | GitHub auth-success banner |
| 3 | `tmp_file_note` | "runs against a throwaway temp file — live config untouched" | below output | ✓ | ✓ | SSHUI-04 |

## create-flow / upload-announcing

**Goal:** the `testUpload` sub-beat announces every command it is about to run,
exactly as it will run it (D-02 announce-and-do; UP-02/UP-03 shown==run).
Reached automatically between the existing `test-stage1-direct` state's
command staging and its own probe.

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `upload_running_line` × 1-2 | `"Running: <exact command>"` | one row per attempted registration | n/a | ✓ | gh shows up to 2 lines, glab shows 1; the literal string `uploader.CommandPreview` produced, never a hand-typed approximation |

## create-flow / upload-results

**Goal:** per-registration outcome rows, then the conditional manual-fallback
block, then auto-advance into the existing `test-stage1-direct` state
unchanged (D-02: no prompt, no cancel timer).

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `upload_result_row` × 1-2 | `"✓ <type> key registered"` / `"✓ <type> key already registered (skipped)"` / `"✗ <type> key registration failed: <reason>"` | one row per announced command | n/a | ✓ | mirrors the `upload_running_line` count |
| 2 | `upload_manual_fallback` | `UploadManualHeading` + `internal/upload.Instructions(provider)` | conditional, after result rows | n/a | ✓ | conditional — the byte-identical `internal/upload.Instructions(provider)` block under the frozen `UploadManualHeading` |

## create-flow / test-stage2-by-alias

**Goal:** stage 2 — targeted by alias, proving `IdentityFile` resolution via `ssh -G`
(TEST-01/TEST-02).

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `test_command` | exact command run | 1st | ✓ | ✓ | `ssh -G personal.github.com ... | grep identityfile` |
| 2 | `test_output` | real command output | 2nd | ✓ | ✓ | `identityfile ~/.ssh/id_ed25519_personal` |
| 3 | `tmp_file_note` | "runs against a throwaway temp file — live config untouched" | below output | ✓ | ✓ | SSHUI-04 |

## create-flow / test-fail

**Goal:** the test-failure error state (TEST-01).

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `test_command` | exact command run | 1st | ✓ | ✓ | same stage-1 command |
| 2 | `test_output` | real failure output | 2nd | ✓ | ✓ | "Permission denied (publickey)." |
| 3 | `error_affordance` | error glyph + word, retry hint | below output | ✓ | ✓ | red ✗ + word, never color alone |

## create-flow / confirm-write

**Goal:** beat 1+2 of the mutation ceremony — preview + confirm (§5).

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `preview_block` | exact resulting `Host` block, sentinels visible | 1st | ✓ | ✓ | `# BEGIN/END gitid managed: personal` |
| 2 | `target_file` | named target file path | 2nd | ✓ | ✓ | `~/.ssh/config` |
| 3 | `confirm_action` | explicit confirm keystroke, not default-focused destructive | 3rd | ✓ | ✓ | non-destructive create, but still an explicit confirm |
| 4 | `nothing_changed_note` | "nothing has changed yet" | below preview | ✓ | ✓ | |

## create-flow / backup-notice

**Goal:** beat 3 — the timestamped backup path (§5).

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `backup_path` | timestamped backup path | 1st | ✓ | ✓ | `~/.ssh/config.backup.2026-07-03T03-59-12Z` |
| 2 | `backup_explainer` | "the backup is the undo story" copy | below path | ✓ | ✓ | |

## create-flow / result-success

**Goal:** beat 4 — the success result (§5).

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `result_glyph` | green `✓` | 1st | ✓ | ✓ | glyph + word, never color alone |
| 2 | `result_message` | what changed + which file | 2nd | ✓ | ✓ | names `~/.ssh/config`, the alias, the IdentityFile |
| 3 | `restore_hint` | backup path again (how to restore) | 3rd | ✓ | ✓ | repeats the backup-notice path |

## create-flow / git-form (copy freeze — D6/D7 checkpoint-2 additions)

**Goal:** the wizard's Git-identity step-3 buttons and their adjacent hint
lines — the FROZEN single source of truth is
`02-STYLE-SPEC.md` §4; this row is the create-flow human-readable companion
(the machine-checkable proof is the repo-wide old-copy grep gate, `02-STYLE-SPEC.md` §6).
D6 (checkpoint-2) collapses all THREE buttons onto ONE row (Back/Skip/
Continue), never changing their copy; D7 adds the disabled-reason text
under `[ Continue ]`.

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `wizard_back_button` | "Back (Esc)" | 1st (same row as Skip/Continue, D6) | ✓ | ✓ | unchanged, not part of the freeze |
| 2 | `wizard_skip_button` | `[ Skip Git ]` | 2nd (same row, D6) | ✓ | ✓ | frozen; the explanation moved off the button onto its adjacent hint line |
| 3 | `wizard_skip_hint` | "Skip keeps this identity SSH-only and marks it incomplete." | below the button row, always visible | ✓ | ✓ | frozen hint line, `hint`/`Theme.Hint` role |
| 4 | `wizard_continue_button` | `[ Continue ]` | 3rd (same row, D6) | ✓ | ✓ | frozen; the explanation moved off the button onto its adjacent hint line |
| 5 | `wizard_continue_disabled_reason` | `— needs user.name + a valid email` | directly under the disabled Continue button | ✓ | ✓ | NEW (D7, checkpoint-2) — replaces the generic `— disabled` suffix everywhere; frozen, byte-exact |
| 6 | `wizard_continue_hint` | "Continue reviews the Git fragment, includeIf, and allowed_signers entries before writing." | below the button row, always visible | ✓ | ✓ | frozen hint line, `hint`/`Theme.Hint` role |

## 02-STYLE-SPEC.md emphasis-role parity dimensions (checkpoint-2 contract, D1–D9)

**Goal:** the checkable parity dimensions `02-STYLE-SPEC.md` §3 defines —
the content parity gate (the rows above) never modeled emphasis roles,
focus affordance, or keyboard-nav ergonomics; these rows are that missing
coverage's human-readable companion. Rows 2 and 9 below are REWRITTEN by
02-15/checkpoint-2 (D1/D4); rows 3, 6, and 8 are NEW (D2/D3/D8). Backed by
the Go test suite in `internal/dummytui` (see `02-STYLE-SPEC.md` §3 for the
exact test-name pattern per row) plus a fresh `agent-ui-ux-designer`
critique of the two live demos.

| # | Dimension | HTML present | TUI present | Notes |
|---|-----------|---------------|--------------|-------|
| 1 | `typography-emphasis-roles` | ✓ | ✓ | label bold, hint dim, warning/error/info carry their semantic colors on both sides |
| 2 | `field-contour` | ✓ | ✓ | **REWRITTEN (D1):** every field is ONE constant-height row in every state; focus = accent color + a `▸`/2px-outline redundant cue — NEVER a box (supersedes the 02-14 rounded-contour box) |
| 3 | `always-expanded-radios` | ✓ | ✓ | NEW (D2): both the match-strategy and algorithm groups render ALL options ALWAYS — no expand/collapse; the `(←/→ change)` hint sits on the group header, visible in both focus states |
| 4 | `hint-persistence` | ✓ | ✓ | the match-strategy hint is reserved and never disappears when the always-expanded group focuses |
| 5 | `arrow-nav` | ✓ | ✓ | the written precedence rule (02-STYLE-SPEC.md §2), identical in both media, incl. the Shift+←/→ focus-override chord (now reaching EVERY step incl. the review ceremony, D7) and the D4 top-level plain-arrow view switch |
| 6 | `glyph-checkbox-radio` | ✓ | ✓ | NEW (D3): the web's `MuiCheckbox`/`MuiRadio` render the SAME frozen glyphs (`☐/☑`, `○/●`) as the TUI via theme-level overrides |
| 7 | `preview-sizing` | ✓ | ✓ | bounded width, optional fixed height with a clip cue, title in the border/top edge |
| 8 | `click-to-focus` | ✓ | ✓ | NEW (D8): the entire rendered field/radio/checkbox row is the click target; disabled algorithm rows are inert |
| 9 | `dim-states` | ✓ | ✓ | **REWRITTEN (D4):** header nav reads `[N] Label` (bracket format moved from the wizard stepper, D5); disabled-nav dims INACTIVE tabs while a pane captures keys; the ACTIVE tab now dims to the NEW active-nav-dimmed role (accent foreground, no background) instead of keeping its accent background; the active pane still carries the active-area accent; a `←→ switch view` footer advertises the D4 top-level arrow switch |
