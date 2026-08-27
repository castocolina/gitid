# gitid ssh JSON schema

This is the frozen, versioned machine-readable output contract for
`gitid ssh` (Phase 6, GSSH-01 / 05-CONTEXT.md D-03). A consumer may rely on
every field named here being present on every successful parse. Adding,
renaming, or removing a field, or changing an enum string, is a breaking
change: bump the `schema` identifier and retire the old value in the same
commit as this document.

Rules that apply to all four envelopes:

- A field is emitted even when empty, so a consumer can rely on its presence.
  Empty strings are `""`; empty arrays are `[]`, never `null`.
- Every enum string is lower-case kebab-case and is part of the contract.
- Every path is a display path (`~/…`), never a raw absolute sandbox path.
- The `options` array is ordered by `globalssh.Policy` declaration order,
  never sorted or filtered.
- A `schema` value changes only on a breaking change.

## `gitid ssh options list --json`

Identifier: `gitid.ssh.options/v1`

```
{
  "schema": "gitid.ssh.options/v1",
  "options": [ { … } ]
}
```

Top-level keys (exact set): `schema`, `options`.

### Option object

| Field | Type | Meaning |
|---|---|---|
| `key` | string | Policy key, in Policy declaration order. |
| `current_value` | string | Effective value; empty when unset. |
| `recommended_value` | string | gitid's recommended value. |
| `risk` | string | `low` \| `medium` \| `high`. |
| `scope` | string | `global` \| `per-alias`. |
| `state` | string | `needs-action` \| `already-set` \| `differs` \| `not-applicable`. |
| `source` | string | `gitid-parsed` \| `outside-gitid` \| `system-file` \| `baseline` \| `inconclusive`. |
| `source_file` | string | Display path of the named source; empty when unknown. |
| `source_line` | integer | Line number; `0` when unknown. |
| `not_applicable_reason` | string | `none` \| `platform` \| `version-too-old` \| `version-unverified` \| `nothing-to-verify` \| `probe-failed`. |
| `version_note` | string | Dynamic OpenSSH note; empty when none. |
| `probe_error` | string | Probe failure note; empty when none. |

Option-object keys (exact set): `key`, `current_value`, `recommended_value`,
`risk`, `scope`, `state`, `source`, `source_file`, `source_line`,
`not_applicable_reason`, `version_note`, `probe_error`.

## `gitid ssh storage show --json`

Identifier: `gitid.ssh.storage/v1`

```
{
  "schema": "gitid.ssh.storage/v1",
  "layout": "include|in-file",
  "target_path": "<display path>",
  "main_config_path": "<display path>",
  "include_line_present": true
}
```

Top-level keys (exact set): `schema`, `layout`, `target_path`,
`main_config_path`, `include_line_present`.

`layout` is `include` (gitid-owned `~/.ssh/config.d/gitid.config` via an
Include line) or `in-file` (sentinel-delimited blocks in `~/.ssh/config`).

## `gitid ssh options apply --json`

Identifier: `gitid.ssh.apply/v1`

The WRITE-RESULT envelope. Emitted on success, on refusal, and on a
rolled-back failure alike — a consumer never has to distinguish "no output"
from "no advisories".

```
{
  "schema": "gitid.ssh.apply/v1",
  "dry_run": false,
  "applied": ["<policy key>"],
  "declined": ["<policy key named on the command line but refused>"],
  "target_path": "<display path>",
  "backups": ["<display path>"],
  "restored": ["<display path>"],
  "advisories": ["<post-write verification advisory>"],
  "simulation_inconclusive": false,
  "simulation_note": "<string, empty when none>",
  "error": "<string, empty on success>",
  "exit_code": 0
}
```

Top-level keys (exact set): `schema`, `dry_run`, `applied`, `declined`,
`target_path`, `backups`, `restored`, `advisories`,
`simulation_inconclusive`, `simulation_note`, `error`, `exit_code`.

`exit_code` is produced by the same mapping helper as the process status.
They cannot disagree.

`advisories` is present-but-empty (`[]`) when the write succeeded with no
shadowing; it carries the post-write verification strings when the fix
landed but is shadowed. Pre-write simulation warnings also appear here on
a dry run.

## `gitid ssh storage migrate --json`

Identifier: `gitid.ssh.migrate/v1`

```
{
  "schema": "gitid.ssh.migrate/v1",
  "dry_run": false,
  "from_layout": "include|in-file",
  "to_layout": "include|in-file",
  "moved_identities": ["<alias>"],
  "moved_globals": true,
  "backups": ["<display path>"],
  "restored": ["<display path>"],
  "error": "<string, empty on success>",
  "exit_code": 0
}
```

Top-level keys (exact set): `schema`, `dry_run`, `from_layout`, `to_layout`,
`moved_identities`, `moved_globals`, `backups`, `restored`, `error`,
`exit_code`.

## Exit-status contract

| Code | Meaning |
|---|---|
| 0 | The command succeeded. For a write, this includes a successful backed-up write whose post-write verification reported an advisory. |
| 1 | Usage, validation or refusal. Nothing was written. |
| 2 | The write or the migration failed and was rolled back. |
| 3 | Reserved for `--fail-on-advisory` only: the write succeeded AND an advisory was reported. Never returned without that flag. |

The zero-on-advisory default follows D-04 and D-14: shadowing is advisory
and never blocking. `--fail-on-advisory` is the opt-in for a stricter script.
