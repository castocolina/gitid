---
phase: 06-global-ssh-options
plan: 06
subsystem: cli
tags: [ssh, cobra, json-schema, adaptive-depth, lifecycle]
requires:
  - phase: 06-global-ssh-options
    provides: [runGlobalSSHApply (06-04), runSSHStorageMigrate (06-05), GlobalSSHOptionStates, SSHStorageMigrationPlan]
provides:
  - "The real `gitid ssh` command group: options list/apply, storage show/migrate"
  - "Four frozen JSON envelopes: gitid.ssh.options/v1, gitid.ssh.storage/v1, gitid.ssh.apply/v1, gitid.ssh.migrate/v1"
  - "The frozen exit-status table (0/1/2/3) with process status equal to envelope exit_code"
  - "TUI fallback constructors NewAppOnGlobalSSH / ActiveTab / GlobalSSHUIState"
affects: [06-07 (visual gate + exit battery), scripts consuming gitid ssh --json]
actuals:
  tokens: 28000
  tasks: 2
  commits: 0
tech-stack:
  added: []
  patterns:
    - "CLI write verbs call the SAME UI-free ceremony the TUI uses (cliGlobalSSHApplyInto / cliSSHStorageMigrateInto), never a tea.Cmd and never a CLI-only write path"
    - "One sshWriteExitCode helper produces both process status and the envelope exit_code field"
    - "Adaptive depth through the existing depthResolver/confirmationPolicyFrom; incomplete + both TTYs opens NewAppOnGlobalSSH with nothing pre-selected"
key-files:
  created:
    - cmd/gitid/ssh.go
    - cmd/gitid/ssh_test.go
    - docs/gitid-ssh-json-schema.md
    - e2e/global_ssh_cli_e2e_test.go
  modified:
    - cmd/gitid/main.go
    - cmd/gitid/identity_test.go
    - docs/cli-parity-matrix.md
    - internal/tuikit/app.go
    - internal/tuikit/app_test.go
key-decisions:
  - "Command tree matches <frozen_contract> exactly: no extra verbs, no flat aliases"
  - "Zero-on-advisory is the default (D-04/D-14); --fail-on-advisory is the opt-in for exit 3"
  - "runSSHStorageMigrate is called with an empty plan token (CLI plans and commits inside one txMu hold)"
patterns-established:
  - "Write-result JSON envelopes on every path (success, refusal, rollback) so a consumer never has to distinguish no output from no advisories"
requirements-completed: [GSSH-01]
coverage:
  - id: D1
    description: Frozen four-command ssh tree wired to shared ceremonies
    requirement: GSSH-01
    verification:
      - kind: unit
        ref: cmd/gitid/ssh_test.go#TestSSHCmdTreeExactlyFourFrozenPaths
        status: pass
      - kind: unit
        ref: cmd/gitid/ssh_test.go#TestSSHWriteVerbsCallSharedCeremonyByConstruction
        status: pass
    human_judgment: false
  - id: D2
    description: Four JSON envelopes with exact key sets and enum members
    requirement: GSSH-01
    verification:
      - kind: unit
        ref: cmd/gitid/ssh_test.go#TestSSHJSONOptionsListExactKeySetAndEnums
        status: pass
      - kind: e2e
        ref: e2e/global_ssh_cli_e2e_test.go#TestGlobalSSHCLI_ListShowApplyIdempotent
        status: pass
    human_judgment: false
  - id: D3
    description: Exit-status table; process status equals envelope exit_code
    requirement: GSSH-01
    verification:
      - kind: unit
        ref: cmd/gitid/ssh_test.go#TestSSHExitCodeEqualsEnvelopeForEveryRow
        status: pass
      - kind: e2e
        ref: e2e/global_ssh_cli_e2e_test.go#TestGlobalSSHCLI_AdvisoryExitCodes
        status: pass
    human_judgment: false
  - id: D4
    description: Adaptive-depth table including empty TUI fallback
    requirement: GSSH-01
    verification:
      - kind: unit
        ref: cmd/gitid/ssh_test.go#TestSSHDepthResolverTable
        status: pass
      - kind: unit
        ref: cmd/gitid/ssh_test.go#TestSSHIncompleteBothTTYsOpensEmptyTUI
        status: pass
    human_judgment: false
duration: 90min
completed: 2026-08-27
status: complete
---

# Phase 06-06: gitid ssh CLI surface Summary

**Every Global SSH outcome is on the command line through the same ceremonies the screen uses, with a frozen command tree, four versioned JSON envelopes, and an exit-status table consistent with the phase's advisory posture.**

## Performance

- **Duration:** ~90 min
- **Tasks:** 2
- **Files modified:** 10

## Accomplishments

- Replaced the reserved `ssh` noun with exactly the four frozen commands.
- Both write verbs reach disk only through `runGlobalSSHApply` / `runSSHStorageMigrate` (empty plan token on migrate).
- Process status and envelope `exit_code` come from one helper; zero-on-advisory is the default.

## Command shape (exercise of 06-CONTEXT.md discretion)

Matches `<frozen_contract>` exactly:

```
gitid ssh options list    [--json]
gitid ssh options apply   <key>... [--dry-run] [--yes] [--fail-on-advisory] [--json]
gitid ssh storage show    [--json]
gitid ssh storage migrate --to <include|in-file> [--dry-run] [--yes] [--json]
```

No extra verbs. No flat root-level aliases.

## Schema identifiers

- `gitid.ssh.options/v1`
- `gitid.ssh.storage/v1`
- `gitid.ssh.apply/v1`
- `gitid.ssh.migrate/v1`

## Exit-status table (observed; process status == envelope `exit_code`)

| Code | Case | Observed |
|---|---|---|
| 0 | success, including advisory | `TestSSHExitCodeEqualsEnvelopeForEveryRow` / e2e advisory case |
| 1 | unknown key, per-alias, version-unverified, missing `--yes` | unit refusals |
| 2 | rolled-back write | recording double returning `Restored` |
| 3 | `--fail-on-advisory` on a shadowed success | e2e `TestGlobalSSHCLI_AdvisoryExitCodes` |
| 0 | dry-run even with `--fail-on-advisory` | unit table row `dry-run-advisory-stays-zero` |

## Captured `gitid ssh options apply --json` (successful-but-shadowed)

Command: `gitid ssh options apply StrictHostKeyChecking --yes --json` against a sandbox HOME whose `~/.ssh/config` has `Host * / StrictHostKeyChecking no` **above** the Include line.

```
{
  "schema": "gitid.ssh.apply/v1",
  "dry_run": false,
  "applied": ["StrictHostKeyChecking"],
  "declined": [],
  "target_path": "~/.ssh/config.d/gitid.config",
  "backups": [
    "~/.ssh/config.bak.1787808898582396000",
    "~/.ssh/config.d/gitid.config.bak.1787808898634685000"
  ],
  "restored": [],
  "advisories": [
    "shadow warning: StrictHostKeyChecking will be shadowed by ~/.ssh/config (line 2)",
    "advisory: StrictHostKeyChecking was applied but is still shadowed by an external directive — the fix may not take effect"
  ],
  "simulation_inconclusive": false,
  "simulation_note": "",
  "error": "",
  "exit_code": 0
}
```

Process status: `EXIT:0`. The advisory channel the threat model promises exists.

## Adaptive-depth (observed)

| Verb | Complete | Incomplete + both TTYs | Incomplete otherwise |
|---|---|---|---|
| `options apply` | ≥1 key → headless | TUI on Global SSH, Options sub-tab, selection empty | exit 1 naming `<key>` |
| `storage migrate` | `--to` supplied → headless | TUI on Global SSH, Storage sub-tab, radio on current layout | exit 1 naming `--to` |
| `options list`, `storage show` | always headless | n/a | n/a |

`ssh.go` contains `depthResolver{` and `confirmationPolicyFrom(` and zero `confirmationAlreadyObtained`. `cliSSHStorageMigrateInto` calls `b.runSSHStorageMigrate(target, "", p)`.

## Deviations from Plan

### 1. TUI launch constructors in `internal/tuikit` (not in `files_modified`)

- **Found during:** Task 1 (CLI TUI fallback)
- **Issue:** The plan's incomplete-plus-both-TTYs branch must open Global SSH on a named sub-tab with nothing pre-selected. `tuikit` had no constructor for that.
- **Fix:** Added `NewAppOnGlobalSSH`, `ActiveTab`, `GlobalSSHUIState` plus `TestNewAppOnGlobalSSHOpensEmptyOptionsAndStorage`.
- **Impact:** Genuine cross-cutting fix required by the frozen adaptive-depth contract; no CLI-only write path.

**Total deviations:** 1 auto-fixed. No scope creep.

## Issues Encountered

- Advisory e2e initially used `placement=below`, which OpenSSH does not treat as shadowing. Switched to `above` (same fixture the PTY shadow test uses).
- Source-level "no inline TTY test" assertion was too broad (`term.IsTerminal` is used by the list verb for table vs TSV). Scoped it to the two write-verb function bodies.

## User Setup Required

None.

## Next Phase Readiness

06-07 can consume the real CLI surface, the four envelopes, and the TUI fallback constructors. Commits for this plan are not yet created.

---
*Phase: 06-global-ssh-options*
*Completed: 2026-08-27*
