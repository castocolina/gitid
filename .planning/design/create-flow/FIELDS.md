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

## create-flow / ssh-form-blank-prefix

**Goal:** blank-prefix WYSIWYG — `SSH Host` = the provider host verbatim, no invented
suffix (SSHUI-01, SSHUI-03).

| # | Field | Label | Order | HTML present | TUI present | Notes |
|---|-------|-------|-------|---------------|--------------|-------|
| 1 | `alias_prefix` | "Alias prefix" | 1st | ✓ | ✓ | blank |
| 2 | `ssh_host` | "SSH Host" | 2nd | ✓ | ✓ | "github.com" — the provider host verbatim, not "<blank>.github.com" |
| 3 | `wysiwyg_note` | explanatory copy on the blank-prefix rule | below the field | ✓ | ✓ | |

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
