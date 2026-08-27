# gitid git JSON schema

This is the frozen, versioned machine-readable output contract for
`gitid git` (Phase 7, GGIT-01 / 07-CONTEXT.md D-08). A consumer may rely on
every field named here being present on every successful parse. Adding,
renaming, or removing a field, or changing an enum string, is a breaking
change: bump the `schema` identifier and retire the old value in the same
commit as this document.

Rules that apply to all four envelopes:

- A field is emitted even when empty, so a consumer can rely on its presence.
  Empty strings are `""`; empty arrays are `[]`, never `null`.
- Every enum string is lower-case kebab-case and is part of the contract.
- Every path is a display path (`~/…`), never a raw absolute sandbox path.
- The `options` array is ordered by `globalgit.Policy` declaration order,
  never sorted or filtered.
- A `schema` value changes only on a breaking change.
- A write verb emits its envelope on success, on refusal, and on a rolled-back
  failure alike — a consumer never has to distinguish "no output" from "no
  advisories".

## `gitid git options list --json`

Identifier: `gitid.git.options/v1`

```
{
  "schema": "gitid.git.options/v1",
  "options": [ { … } ]
}
```

Top-level keys (exact set): `schema`, `options`.

The array carries one row per `globalgit.Policy` entry in declaration order,
including the fallback-author row (which has no `token` and is not an apply
target).

### Option object

| Field | Type | Meaning |
|---|---|---|
| `key` | string | The policy's canonical display key, in Policy declaration order. |
| `token` | string | The frozen CLI row token (R-5); empty for the fallback-author row, which has no apply token. |
| `current_value` | string | Effective value; a bundle row carries the aggregate ("3 of 8 set, 1 differs"). |
| `provenance` | string | The provenance label ("set by you in …", "not set", …). |
| `recommended_value` | string | gitid's recommended value for the row. |
| `state` | string | `needs-action` \| `already-set` \| `differs` \| `not-applicable` \| `probe-error`. |
| `probe_error` | string | Probe failure note; empty when none. |

Option-object keys (exact set): `key`, `token`, `current_value`, `provenance`,
`recommended_value`, `state`, `probe_error`.

The `state` enum is exactly the render layer's own taxonomy: the four
`GlobalGitOptionState` values plus the probe-error word, produced by the same
mapper the Options pane and the apply refusals use.

## `gitid git options apply --json`

Identifier: `gitid.git.apply/v1`

The WRITE-RESULT envelope. Emitted on success, on refusal, and on a rolled-back
failure alike.

```
{
  "schema": "gitid.git.apply/v1",
  "dry_run": false,
  "applied": ["<row token named on the command line>"],
  "declined": ["<row token named on the command line but refused>"],
  "target_path": "<display path>",
  "backups": ["<display path>"],
  "restored": ["<display path>"],
  "advisories": ["<advisory>"],
  "error": "<string, empty on success>",
  "exit_code": 0
}
```

Top-level keys (exact set): `schema`, `dry_run`, `applied`, `declined`,
`target_path`, `backups`, `restored`, `advisories`, `error`, `exit_code`.

- `applied`/`declined` carry the ROW TOKENS the command line named (R-5's argv
  vocabulary), not the member config keys.
- `target_path` is the resolved baseline file that receives the managed block
  (`~/.gitconfig.d/00-baseline`), the same file the ceremony's own preview
  names.
- `backups` carries the unconditional timestamped backups the ceremony took
  (the floor `~/.gitconfig` write and the baseline write, for files that
  already existed).
- `advisories` is present-but-empty (`[]`) when the write reported nothing. It
  carries the version-gate substitution advisory when the conflict-style row
  was written below the hard gate, and the post-write verification strings
  when a key gitid wrote does not resolve to the value it wrote.
- `exit_code` is produced by the same mapping helper as the process status.
  They cannot disagree.

### Captured example — a below-gate conflict-style apply

A real apply of `merge.conflictstyle` on a machine whose git is below the
2.35 hard gate writes the fallback value (`diff3`) instead of the
recommendation (`zdiff3`) and reports the substitution as an advisory. Captured
from a real run of the ceremony on a sandbox home with pre-existing files
(backup timestamps vary per run):

```json
{
  "schema": "gitid.git.apply/v1",
  "dry_run": false,
  "applied": [
    "merge.conflictstyle"
  ],
  "declined": [],
  "target_path": "~/.gitconfig.d/00-baseline",
  "backups": [
    "~/.gitconfig.bak.1787870736064013000",
    "~/.gitconfig.d/00-baseline.bak.1787870736114080000"
  ],
  "restored": [],
  "advisories": [
    "advisory: merge.conflictstyle is below the git version gate — wrote \"diff3\" instead of \"zdiff3\"",
    "advisory: merge.conflictstyle was applied but the effective value is \"diff3\" — a later setting in your config may override it"
  ],
  "error": "",
  "exit_code": 0
}
```

This example is the clearest statement that the advisory channel exists: a
script that applied `merge.conflictstyle` and read only `applied` would believe
it got `zdiff3`; the `advisories` array names the value actually written.

## `gitid git fallback show --json`

Identifier: `gitid.git.fallback/v1`

```
{
  "schema": "gitid.git.fallback/v1",
  "name": "<display value>",
  "email": "<display value>",
  "name_status": "set|unset",
  "email_status": "set|unset"
}
```

Top-level keys (exact set): `schema`, `name`, `email`, `name_status`,
`email_status`.

`name`/`email` carry the raw values (`""` when unset); `name_status`/
`email_status` carry `set` or `unset`. A missing config file is the first-run
state and reports both halves `unset` with exit 0 — it is not a failure.

## `gitid git fallback set --json`

Identifier: `gitid.git.fallbackset/v1`

The WRITE-RESULT envelope for the dedicated fallback-author ceremony (D-05:
never the baseline managed block). Emitted on success, on refusal, and on a
rolled-back failure alike.

```
{
  "schema": "gitid.git.fallbackset/v1",
  "dry_run": false,
  "set_name": false,
  "set_email": false,
  "cleared_name": false,
  "cleared_email": false,
  "target_path": "<display path>",
  "backups": ["<display path>"],
  "restored": ["<display path>"],
  "advisories": ["<advisory>"],
  "error": "<string, empty on success>",
  "exit_code": 0
}
```

Top-level keys (exact set): `schema`, `dry_run`, `set_name`, `set_email`,
`cleared_name`, `cleared_email`, `target_path`, `backups`, `restored`,
`advisories`, `error`, `exit_code`.

- `set_name`/`set_email`/`cleared_name`/`cleared_email` record which halves the
  invocation set or cleared (D-04's explicit set-versus-clear contract) — a
  consumer can tell a clear from an omission.
- `target_path` is the main config (`~/.gitconfig`), where the fallback-author
  pair is written.
- `backups` carries the unconditional timestamped backup (when the file already
  existed); `restored` names the paths a rollback restored.

## Exit-status contract

| Code | Meaning |
|---|---|
| 0 | The command succeeded. For a write, this includes a successful backed-up write whose ceremony reported an advisory. |
| 0 | A dry run, even with `--fail-on-advisory` supplied — a dry run never writes, so it can never report a post-write advisory. |
| 1 | Usage, validation or refusal — an unknown or member-key token, a non-selectable row, a malformed or contradictory flag combination, or a required confirmation not given. Nothing was written. |
| 2 | The write was attempted and rolled back. The envelope's `restored` names the restored paths. |
| 3 | Reserved for `--fail-on-advisory` on `options apply` only: the write succeeded AND the ceremony reported an advisory. Never returned without that flag. |

The zero-on-advisory default follows D-04 and D-14: an advisory is information,
never blocking. `--fail-on-advisory` is the opt-in for a stricter script.
`exit_code` is produced by the same helper as the process status — a script
that trusts the exit status and a consumer that reads the envelope can never
disagree.
