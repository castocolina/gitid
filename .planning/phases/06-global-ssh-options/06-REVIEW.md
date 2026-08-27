---
phase: 06-global-ssh-options
reviewed: 2026-08-27T00:00:00Z
depth: standard
files_reviewed: 81
files_reviewed_list:
  - Makefile
  - cmd/gitid/gate_visual_regression_test.go
  - cmd/gitid/identity_create.go
  - cmd/gitid/identity_test.go
  - cmd/gitid/lifecycle.go
  - cmd/gitid/lifecycle_test.go
  - cmd/gitid/main.go
  - cmd/gitid/ssh.go
  - cmd/gitid/ssh_test.go
  - cmd/gitid/wiring.go
  - cmd/gitid/wiring_storage_test.go
  - cmd/gitid/wiring_test.go
  - docs/cli-parity-matrix.md
  - docs/gitid-ssh-json-schema.md
  - e2e/global_ssh_cli_e2e_test.go
  - e2e/global_ssh_pty_e2e_test.go
  - e2e/global_ssh_storage_pty_e2e_test.go
  - e2e/harness_test.go
  - internal/doctor/checks/orphans_test.go
  - internal/doctor/checks/overlap.go
  - internal/doctor/checks/overlap_test.go
  - internal/doctor/checks/redundancy.go
  - internal/doctor/checks/redundancy_test.go
  - internal/doctor/checks/reserved_test.go
  - internal/dummytui/data_test.go
  - internal/dummytui/fixturebackend.go
  - internal/filewriter/filewriter.go
  - internal/filewriter/filewriter_test.go
  - internal/globalssh/classify.go
  - internal/globalssh/classify_test.go
  - internal/globalssh/doc.go
  - internal/globalssh/isolation_contract_test.go
  - internal/globalssh/peralias.go
  - internal/globalssh/peralias_test.go
  - internal/globalssh/policy.go
  - internal/globalssh/policy_test.go
  - internal/globalssh/probe.go
  - internal/globalssh/probe_test.go
  - internal/globalssh/shadow.go
  - internal/globalssh/shadow_test.go
  - internal/globalssh/version.go
  - internal/globalssh/version_test.go
  - internal/identity/delete.go
  - internal/identity/delete_test.go
  - internal/identity/identity.go
  - internal/identity/identity_test.go
  - internal/identity/modes.go
  - internal/identity/repair.go
  - internal/identity/rotate.go
  - internal/screenshot/createflow.go
  - internal/screenshot/createflow_packet.go
  - internal/screenshot/createflow_packet_test.go
  - internal/screenshot/createflow_regions.go
  - internal/screenshot/createflow_test.go
  - internal/sshconfig/directives.go
  - internal/sshconfig/directives_test.go
  - internal/sshconfig/globals.go
  - internal/sshconfig/globals_test.go
  - internal/sshconfig/include.go
  - internal/sshconfig/include_test.go
  - internal/sshconfig/migrate.go
  - internal/sshconfig/migrate_test.go
  - internal/sshconfig/parser_test.go
  - internal/sshconfig/reader.go
  - internal/sshconfig/reader_test.go
  - internal/sshconfig/renderer.go
  - internal/sshconfig/renderer_test.go
  - internal/sshconfig/validation.go
  - internal/sshconfig/validation_test.go
  - internal/sshconfig/writer.go
  - internal/tuikit/app.go
  - internal/tuikit/app_test.go
  - internal/tuikit/backend.go
  - internal/tuikit/backend_stub_test.go
  - internal/tuikit/batch3_test.go
  - internal/tuikit/ceremony.go
  - internal/tuikit/design.go
  - internal/tuikit/design_test.go
  - internal/tuikit/globalssh.go
  - internal/tuikit/globalssh_test.go
  - internal/tuikit/views.go
findings:
  critical: 5
  warning: 18
  info: 0
  total: 23
status: issues_found
---

# Phase 6: Code Review Report

**Reviewed:** 2026-08-27
**Depth:** standard
**Files Reviewed:** 81
**Status:** issues_found

## Summary

Phase 6 lands a large, heavily documented surface: a globals-block engine
(`internal/sshconfig/globals.go`), a name-registry consolidation, a six-option
classifier (`internal/globalssh/classify.go`), whole-graph shadow simulation
(`internal/globalssh/shadow.go`), a digest-locked migration engine
(`internal/sshconfig/migrate.go`), a frozen CLI verb surface (`cmd/gitid/ssh.go`),
and a visual-regression gate.

The lock contract asked about in the brief holds up: `txMu` and
`pendingMigrationMu` are acquired in the documented order, `takePendingMigration`
is genuinely atomic and never takes `txMu`, and the CLI branch of
`runSSHStorageMigrate` correctly calls `sshconfig.PlanMigration` directly rather
than re-entering `b.SSHStorageMigrationPlan`. The CLI verbs do route through
`runGlobalSSHApply` / `runSSHStorageMigrate` — no reimplemented write path was
found. `internal/filewriter` is clean.

The defects are elsewhere, and several of them are in exactly the safety
machinery the phase exists to provide:

- The "pure, non-mutating" planning path **writes to `~/.ssh/`** — merely
  previewing an Include-layout migration (arrow keys in the TUI, or
  `--dry-run` on the CLI) creates and chmods `~/.ssh/config.d`, with no
  confirmation, in violation of the project's own write rule.
- `EnsureGlobals` **truncates multi-token directive values**, silently
  destroying hand-added directives inside the block it claims never to drop.
- The shadow simulation's Include rewriter handles only single-token
  `Include <path>` lines, so it either **drops reachable files (false
  "no shadowing")** or **leaves the real absolute path in the mirror**, defeating
  the isolation guarantee its own doc-comment asserts.
- The migrate verb can never emit the exit code and `restored` array its frozen
  schema documents for a rolled-back migration; the only test that "proves"
  exit 2 replaces the real ceremony with a fake.
- The Storage & preview sub-tab activates into an error state on every entry,
  and mouse-driven layout selection can never reach the migration ceremony.

The doc-comment-to-code ratio in this phase is unusually high, and several
comments assert properties the code does not have (`PlanMigration` "never
writes"; `planTokenFor` "without leaking any file content"; `extractGSSApplyHeading`
"absorbs a wrapped continuation row"). Comments that state guarantees are only
useful if a test enforces them.

## Critical Issues

### CR-01: `PlanMigration` mutates `~/.ssh/` — previews and `--dry-run` create and chmod `config.d`

**File:** `internal/sshconfig/migrate.go:289-293`, `cmd/gitid/wiring.go:1645-1650`, `cmd/gitid/ssh.go:384`, `cmd/gitid/lifecycle.go:1053`
**Issue:**
`PlanMigration`'s doc-comment (lines 276-279) states: *"It never writes, backs up,
or shells out beyond the resolution snapshot."* Its first action for
`MigrateToInclude` is:

```go
if direction == MigrateToInclude {
    if derr := EnsureIncludeDir(filepath.Dir(destPath)); derr != nil {
```

`EnsureIncludeDir` → `filewriter.EnsureDir(dir, 0o700)` → `os.MkdirAll` **plus**
`os.Chmod(dirPath, 0o700)`. Both are real filesystem mutations of the user's
`~/.ssh` tree, and the chmod also rewrites the permissions of a `config.d` the
user already owns.

Every non-write path reaches this:

1. `internal/tuikit/globalssh.go:551` — pressing `↓` on the Storage sub-tab to
   *look at* the Include layout calls `SSHStorageMigrationPlan` → `PlanMigration`
   → directory created. No ceremony, no confirmation, no backup.
2. `cmd/gitid/ssh.go:384` — `gitid ssh storage migrate --to include --dry-run`
   calls `PlanMigration` directly. The flag help says *"print the migration plan
   … and exit 0 without writing."*
3. `cmd/gitid/lifecycle.go:1053` — the CLI branch, before the `p.DryRun` early
   return at line 1059.

This violates CLAUDE.md's binding rule (*"Never write to a user's `~/.ssh/config`
or `~/.gitconfig` without a timestamped backup, idempotent managed blocks, and
explicit confirmation"*) and the phase's own preview/confirm/backup ceremony.

**Fix:** Move directory creation out of the planning half and into the
committing half only. `MigrateWithPlan` already calls `EnsureIncludeDir` at
lines 394-398, so the planning call is redundant for the commit path.

```go
func PlanMigration(direction MigrateDirection, deps MigrateDeps) (MigrationPlan, error) {
	sourcePath, destPath := migratePaths(direction, deps)

	// NO EnsureIncludeDir here: planning is read-only. MigrateWithPlan
	// creates the directory after the confirm gate. A missing directory is
	// not an error at plan time — readOrEmpty already tolerates a missing
	// destination file.

	sourceContent, err := readOrEmpty(deps, sourcePath)
	...
```

Add a regression test that snapshots `os.Stat(~/.ssh)` before and after
`PlanMigration`/`--dry-run` and fails on any difference (existence, mode, mtime).

---

### CR-02: `EnsureGlobals` truncates multi-token directive values, corrupting hand-added directives

**File:** `internal/sshconfig/globals.go:216-233` (specifically line 230)
**Issue:**
`parseGlobalBody` stores only `fields[1]`:

```go
fields := strings.Fields(trimmed)
if len(fields) < 2 { continue }
...
m.set(fields[0], fields[1])   // <-- everything after the first value token is lost
```

`renderGlobalBody` then re-emits `key + " " + value`. Any directive inside
gitid's managed `Host *` block whose value is more than one token is silently
rewritten to its first token and the remainder is deleted from the user's
`~/.ssh/config`:

| Before (user's block)                                    | After `EnsureGlobals` |
|----------------------------------------------------------|-----------------------|
| `IdentityAgent "~/Library/Group Containers/…/Listeners"`   | `IdentityAgent "~/Library/Group` |
| `SendEnv LANG LC_*`                                        | `SendEnv LANG` |
| `ProxyCommand ssh -W %h:%p bastion`                        | `ProxyCommand ssh` |
| `CanonicalDomains example.com internal.example.com`        | `CanonicalDomains example.com` |

The `IdentityAgent` case is the common 1Password/Secretive macOS setup. The
function's own doc-comment (line 57 and lines 236-239) promises the opposite:
*"a hand-added directive inside the block that is NOT in this list is appended
after the ordered keys so it is never dropped."*

This fires on every create (`sshconfig.Write` with a non-empty `globalsGOOS`) and
on every `gitid ssh options apply`. Comment lines inside the block are also
dropped (line 220 skips them and nothing re-emits them).

**Fix:** Preserve the whole value, and preserve comments.

```go
// globalKV gains no new field; the value simply carries every token.
fields := strings.Fields(trimmed)
if len(fields) < 2 { continue }
if strings.EqualFold(fields[0], "Host") || strings.EqualFold(fields[0], "IgnoreUnknown") {
    continue
}
m.set(fields[0], strings.Join(fields[1:], " "))
```

Table-test `EnsureGlobals` round-tripping each of the four rows above, and add a
case for a `#` comment line inside the managed block.

---

### CR-03: `rewriteIncludes` mirrors only one path token and misses `Include=` — the simulation reads the user's real config or silently drops files

**File:** `internal/globalssh/shadow.go:517-564`
**Issue:**
`BuildGraph` discovers Includes through `sshconfig.DetectInclude`, which
correctly handles OpenSSH's full grammar: multiple space-separated globs per
line, the `Include=path` equals form, and double-quoted paths containing spaces
(`internal/sshconfig/adopt.go:100-160`). `rewriteIncludes` does not:

```go
fields := strings.Fields(trimmed)
if len(fields) < 2 || !strings.EqualFold(fields[0], "Include") {
    continue
}
raw := strings.Trim(fields[1], `"`)
...
lines[i] = "Include " + mirrored   // whole line replaced by ONE path
```

Three concrete failures:

1. **`Include ~/.ssh/a ~/.ssh/b`** — only `~/.ssh/a` is mirrored; `~/.ssh/b` is
   deleted from the mirrored line. Both files are present in `graph.Files` and
   are written into the mirror, but `b` is unreachable from the mirrored entry
   point. A `Host *` / `StrictHostKeyChecking` directive in `b` is invisible, so
   `Simulate` reports **no shadowing** for a fix that will in fact be shadowed —
   the exact false negative D-04 exists to prevent.
2. **`Include=~/.ssh/other`** — `strings.Fields` yields one token, so
   `len(fields) < 2` and the line is left **verbatim** in the mirror. `ssh -G -F
   <mirror>` then reads the user's **real** `~/.ssh/other`. The function's
   doc-comment (lines 511-516) explicitly promises this cannot happen:
   *"an unresolved Include must never leak the real machine into the simulation
   (T-06-37, T-06-49)."*
3. **`Include "~/.ssh/my config"`** — `fields[1]` is `"~/.ssh/my`; the glob fails,
   `known` is false, and the line is rewritten to a `nonexistent-` path,
   silently removing a reachable file.

The mirrored path is also emitted unquoted (`"Include " + mirrored`), so any home
directory containing a space produces an unparseable Include line in the mirror.

**Fix:** Rewrite the line from `DetectInclude`'s own tokens rather than
re-tokenising with `strings.Fields`, and quote every emitted path.

```go
// Pass the per-file []IncludeDirective into rewriteIncludes instead of
// re-parsing, and rebuild the whole argument list:
func rewriteIncludeLine(directives []sshconfig.IncludeDirective, mirrorPath func(string) string, mirrorRoot string) string {
	parts := make([]string, 0, len(directives))
	for _, d := range directives {
		target := mirrorPath(d.Expanded)
		if !resolvable(d) {
			target = filepath.Join(mirrorRoot, "nonexistent-"+filepath.Base(d.Raw))
		}
		parts = append(parts, strconv.Quote(target)) // always quoted
	}
	return "Include " + strings.Join(parts, " ")
}
```

Add `shadow_test.go` cases for: multi-glob Include, `Include=` form, quoted path
with a space, and a home directory containing a space — each asserting the
mirror contains **no** path outside `mirrorRoot`.

---

### CR-04: `gitid ssh storage migrate` can never report a rollback — exit code 2 and `restored` are unreachable

**File:** `cmd/gitid/lifecycle.go:1080-1083`, `cmd/gitid/ssh.go:107-121`, `docs/gitid-ssh-json-schema.md:124,140`
**Issue:**
`runSSHStorageMigrate` discards every rollback detail:

```go
result, merr := sshconfig.MigrateWithPlan(plan, deps)
if merr != nil {
    return res, fmt.Errorf("gitid: storage migration: %w", merr)  // res.Restored is nil
}
```

`res` is the zero `lifecycleResult`; `Restored` is never populated on any path in
this function. The engine's `rollbackTracked` (`internal/sshconfig/migrate.go:549`)
*does* restore files and names the backups, but only inside the error **string**.

Downstream consequences:

- `sshWriteExitCode` (`cmd/gitid/ssh.go:107`) returns 2 only when
  `len(res.Restored) > 0`, so a real rolled-back migration always exits **1**.
  `docs/gitid-ssh-json-schema.md:140` documents `2 | The write **or the
  migration** failed and was rolled back.` — unreachable for migrate.
- `fillMigrateFromResult` always writes `"restored": []` into the frozen
  `gitid.ssh.migrate/v1` envelope, even when files were restored.
- `printMigrateHuman`'s `restored: …` stderr line at `cmd/gitid/ssh.go:564-566`
  is dead code.

The test that claims to cover this (`cmd/gitid/ssh_test.go:215-227`) replaces
`cliSSHStorageMigrateInto` with a stub that hand-returns
`lifecycleResult{Restored: [...]}`, so it exercises the envelope mapping, not the
ceremony. `runGlobalSSHApply` gets this right (its `fail` closure sets
`res.Restored = outcomes`); migrate is the asymmetric one.

**Fix:** Have `MigrateWithPlan` return the restored paths as data, and propagate
them.

```go
// internal/sshconfig/migrate.go — MigrateResult gains the field:
type MigrateResult struct {
	SourceBackup string
	TargetBackup string
	Restored     []string // paths this transaction rolled back
	Recovery     string
}
// rollbackTracked records each restored path into the returned MigrateResult
// (returned alongside the error, not discarded).

// cmd/gitid/lifecycle.go:
result, merr := sshconfig.MigrateWithPlan(plan, deps)
res.Restored = result.Restored
if merr != nil {
	return res, fmt.Errorf("gitid: storage migration: %w", merr)
}
```

Replace the seam-stubbing test with one that drives a real failure (e.g. a
`newMigrateDeps` override whose `WriteFile` fails on the source trim) and asserts
`exitStatusOf(err) == 2` and a non-empty `restored` array.

---

### CR-05: Storage sub-tab always activates into an error state; the mouse path can never open the migration ceremony

**File:** `internal/tuikit/globalssh.go:136-154`, `694-701`, `581-590`, `939`
**Issue:**
`activate` sets `m.storageChoice = s.SSHStorage` and then immediately plans for
that same layout:

```go
m.storageChoice = s.SSHStorage
...
view, verr := m.backend.SSHStorageMigrationPlan(m.storageChoice)
if verr != nil { m.storageViewErr = verr.Error() }
```

`realBackend.SSHStorageMigrationPlan` (`cmd/gitid/wiring.go:1631-1636`) refuses a
no-op:

```go
if currentLayout == layout {
	return ..., fmt.Errorf("gitid: layout is already %s — nothing to plan", layout)
}
```

`s.SSHStorage` is derived from the same `b.storage()` call as `currentLayout`
(`InitialState`, `wiring.go:640-646`), so the two are **always** equal on entry.
Every activation of the Global SSH tab therefore stores
`storageViewErr = "gitid: layout is already sentinel — nothing to plan"`, and
`renderStorage` (line 945-950) renders the pane as:

```
! gitid: layout is already sentinel — nothing to plan
Re-enter the screen to retry.
```

instead of the STORE-01 resulting-config preview the sub-tab exists to show.

The keyboard `↑`/`↓` handler (lines 541-556) clears and refetches, which is why
the committed frame `ui-frames/storage-browse.txt` looks correct — it is captured
*after* a `↓`. The e2e test (`e2e/global_ssh_storage_pty_e2e_test.go:255-272`)
only asserts that the substrings `"Sentinel"` and `"Include"` appear, and both
live in the left radio pane, so the error state is not covered.

The mouse path is worse. `handleStorageClick` sets the radio without refetching:

```go
case strings.Contains(line, "gitid-owned ~/.ssh/config.d"):
	m.storageChoice = StorageInclude
	return keyResult{model: m, handled: true}
```

`storageViewErr` still holds the activation error, so (a) the
`" Migrate layout… (Enter) "` button is never rendered (line 939 requires
`m.storageViewErr == ""`), and (b) the `enter` handler bails at lines 582-585.
A mouse-only user can never reach the storage migration ceremony.

**Fix:** Treat "already on this layout" as a normal state, not an error, and make
the mouse path share the keyboard path's refetch.

```go
// wiring.go — return a view describing the current layout instead of an error,
// or have activate() plan for the OTHER layout. Then:

// globalssh.go — extract the refetch and call it from BOTH paths:
func (m globalSSHModel) refetchStoragePlan() globalSSHModel {
	m.storageView = SSHStorageMigrationView{}
	m.storageViewErr = ""
	view, verr := m.backend.SSHStorageMigrationPlan(m.storageChoice)
	if verr != nil {
		m.storageViewErr = verr.Error()
	} else {
		m.storageView = view
	}
	return m
}

case strings.Contains(line, "gitid-owned ~/.ssh/config.d"):
	m.storageChoice = StorageInclude
	return keyResult{model: m.refetchStoragePlan(), handled: true}
```

Add a PTY assertion on the **first** storage frame (before any keystroke) that
the right pane contains `"Resulting config"` and not `"nothing to plan"`, plus a
mouse-click test that reaches `gssStorageCeremony`.

Note: fixing this by refetching on mouse click makes CR-01 fire on a click too —
CR-01 must be fixed first or together with this one.

## Warnings

### WR-01: Post-write verify advisories have a dead branch and swallow `Inconclusive`

**File:** `cmd/gitid/lifecycle.go:956-964`
**Issue:** `globalssh.Verify` (`internal/globalssh/shadow.go:296-304`) constructs
`ShadowFinding` with only `Key`, `WantValue` and `GotValue` — it never sets
`ShadowedByFile`/`ShadowedByLine` (its own doc says the static scan is not used
there). So `if f.ShadowedByFile != ""` at line 959 is unreachable and the
file/line advisory string it guards can never be produced. Separately, when
`verResult.Inconclusive` is true (probe failure after the write) nothing is
recorded at all — a failed post-write verification silently reports success.
**Fix:** Delete the dead branch, and surface the inconclusive case:

```go
verResult := globalssh.Verify(globalssh.BuildProbeDeps(b.sshConfigPath), keys)
if verResult.Inconclusive {
	res.Advisories = append(res.Advisories,
		"advisory: post-write verification could not run ("+verResult.Reason+") — the fix was written but not re-verified")
}
for _, f := range verResult.Findings {
	res.Advisories = append(res.Advisories, fmt.Sprintf(
		"advisory: %s was applied but is still shadowed by an external directive — the fix may not take effect", f.Key))
}
```

### WR-02: `txMu` is acquired on the Bubble Tea update goroutine, freezing the UI

**File:** `cmd/gitid/wiring.go:1626-1628`, `internal/tuikit/globalssh.go:141-152`, `551`
**Issue:** `SSHStorageMigrationPlan` takes `b.txMu` and is called synchronously
from `activate` and from the `↑`/`↓` handler — both run on the Bubble Tea update
goroutine. `activate` additionally runs `GlobalSSHOptionStates()` (two bounded
`ssh -G` probes plus `ssh -V`) inline. If a `CommitGlobalSSH`/`CommitSSHStorage`
`tea.Cmd` goroutine currently holds `txMu` (its transaction can run one
`ssh -G` per managed alias at up to 3s each, plus writes), the entire TUI stops
responding until it finishes. The `<lock_contract>` documents *"txMu must NOT
cross the tea.Cmd boundary"* but says nothing about taking it on the update
goroutine, which is the same hazard from the other side.
**Fix:** Move both the option probe and the storage plan into `tea.Cmd`s that
deliver their result as a message (the phase already has `GlobalSSHCommitMsg` /
`SSHStorageCommitMsg` precedent), and render a "reading your configuration…"
state until they arrive. Extend the lock contract with a rule 6: *no lifecycle
mutex is taken on the update goroutine.*

### WR-03: A dry run consumes the held migration plan

**File:** `cmd/gitid/lifecycle.go:1040-1061`
**Issue:** `takePendingMigration(planToken)` clears the slot, and only afterwards
does `if p.DryRun { return res, nil }` run. A dry run driven with a token
therefore destroys the user's previewed plan and forces a re-open, with no
diagnostic. No production caller does this today (`cliSSHStorageMigrateInto`
passes `""`), but the ordering is a trap for the next caller.
**Fix:** Hoist the dry-run return above the plan retrieval, or peek without
consuming for dry runs:

```go
if p.DryRun && planToken != "" {
	return res, nil // a dry run never consumes the held plan
}
```

### WR-04: `planTokenFor` is a reversible hex encoding, not a hash, and its doc-comment claims otherwise

**File:** `cmd/gitid/wiring.go:1536-1550`
**Issue:**

```go
h := fmt.Sprintf("%d|%s|%s|%x|%x", plan.Direction, plan.SourcePath, plan.DestPath,
	plan.Digests[plan.SourcePath], plan.Digests[plan.DestPath])
return fmt.Sprintf("%x", []byte(h))
```

`%x` on a `[]byte` is hex encoding, not hashing. The token is a trivially
reversible encoding of both absolute file paths (and therefore the user's home
directory) and the two digests, double-hex-encoded and ~350 bytes long. It
crosses into `tuikit.SSHStorageMigrationView.PlanToken`, i.e. into the view
layer, PTY frame captures and the screenshot/visual-regression evidence packets.
The comment claims it is *"enough to uniquely identify the plan without leaking
any file content across the boundary"* and *"a short but collision-resistant
representation"* — neither is true of a hex dump.
**Fix:**

```go
sum := sha256.Sum256([]byte(h))
return hex.EncodeToString(sum[:])
```

### WR-05: Unresolved author commentary and dead assignments shipped in `SSHStorageMigrationPlan`

**File:** `cmd/gitid/wiring.go:1673-1698`
**Issue:** The view struct literal assigns all three preview fields to
`string(plan.DestAfter)` (lines 1676-1678) and the `if toInclude { … } else { … }`
block immediately below overwrites every one of them — three dead assignments.
Line 1687-1688 ships an unfinished thought:

```go
// MainPreview is the Include-line-bearing ~/.ssh/config (SourceAfter for toInclude
// means the config file after the Include line and block removal — wait, let me check)
```

**Fix:** Delete the three dead assignments (leave the fields unset in the literal
and set them once in the branch), and replace the commentary with the resolved
statement of which plan side maps to which preview field.

### WR-06: `fillApplyFromResult`'s dry-run branch is identical to the success branch; `printApplyDryRun`'s `jsonOut` parameter is always false

**File:** `cmd/gitid/ssh.go:498-505`, `521-535`, `295`
**Issue:** Lines 498-505 are two identical blocks:

```go
if dryRun {
	env.Applied = append([]string{}, keys...)
	env.Declined = []string{}
	return
}
env.Applied = append([]string{}, keys...)
env.Declined = []string{}
```

The `dryRun` parameter has no effect. Separately, `printApplyDryRun`'s first
statement is `if jsonOut { return }`, but its only call site
(`cmd/gitid/ssh.go:295`) passes the literal `false` and is already nested inside
`if !flags.JSON`. Both are dead.
**Fix:** Drop the `dryRun` parameter from `fillApplyFromResult` and the `jsonOut`
parameter from `printApplyDryRun`.

### WR-07: Test-only fixtures live in the production `cmd/gitid/ssh.go`

**File:** `cmd/gitid/ssh.go:796-831`
**Issue:** `jsonObjectKeys` and twelve package-level vars
(`sshOptionRecordKeys`, `sshOptionsDocKeys`, `sshStorageDocKeys`,
`sshApplyDocKeys`, `sshMigrateDocKeys`, `sshStateEnum`, `sshSourceEnum`,
`sshRiskEnum`, `sshScopeEnum`, `sshNAReasonEnum`, `sshLayoutEnum`) are referenced
**only** from `cmd/gitid/ssh_test.go`. They ship in the binary and the `unused`
linter cannot flag them because test references count as uses.
**Fix:** Move all thirteen declarations into `ssh_test.go` (or a
`ssh_schema_test.go`) so the schema contract lives with the test that enforces it.

### WR-08: The CLI confirmation prompt discards the ceremony preview

**File:** `cmd/gitid/ssh.go:301-306`, `407-412`, `cmd/gitid/identity.go:211`
**Issue:** `confirmationPolicyFrom` wires the lifecycle prompt as
`func(string) (bool, error) { return prompt() }` — the `preview` argument that
`runGlobalSSHApply` builds (target file + option list, `lifecycle.go:873-874`)
and `runSSHStorageMigrate` builds (`lifecycle.go:1065`) is thrown away. The
interactive CLI user is asked
`Apply global SSH option(s) HashKnownHosts? Type "yes" to confirm:` and confirms a
write to `~/.ssh/config` having been shown neither the resolved target path nor
the diff nor the shadow warnings. The TUI ceremony shows all three; `--dry-run`
shows them but is a separate invocation. This is the *confirm* half of CLAUDE.md's
test → confirm + backup → re-test loop.
**Fix:** Print the preview before the question:

```go
policy, perr := confirmationPolicyFrom(cmd, "apply global SSH options", stdinTTY, stdoutTTY, flags.Yes,
	func(preview string) (bool, error) {
		if plan, err := b.GlobalSSHApplyPlan(keys); err == nil {
			printApplyDryRun(cmd.OutOrStdout(), plan)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s\nType \"yes\" to confirm: ", preview)
		...
	})
```

This requires widening `confirmationPolicyFrom`'s `prompt` parameter to
`func(string) (bool, error)`.

### WR-09: `VersionGate` hardcodes `accept-new` in a note it renders for any gated policy

**File:** `internal/globalssh/version.go:49,51`
**Issue:**

```go
return VersionTooOld, fmt.Sprintf("%s %s — accept-new needs OpenSSH %s+, upgrade to use it", ...)
return VersionAvailable, fmt.Sprintf("%s %s — accept-new is available", ...)
```

`VersionGate` is generic over `OptionPolicy`, and `Policy` is explicitly declared
as "DATA, not behavior" that future rows extend. The moment a second row gains a
`MinOpenSSH`, both surfaces (`cmd/gitid/ssh.go:680` and
`cmd/gitid/wiring.go:1408`) render "accept-new" for the wrong option.
**Fix:** Use `p.Recommended` (or `p.Key + " " + p.Recommended`) in place of the
literal, and update the copy-freeze entries in `Makefile:314-315` accordingly.

### WR-10: `PerAliasConformance` and `resolutionDependentKeys` are production-unused, and the IdentitiesOnly offender list is never surfaced

**File:** `internal/globalssh/peralias.go:12`, `internal/globalssh/classify.go:162`
**Issue:** `PerAliasConformance` is exported but referenced only from
`peralias_test.go`; `Statuses` calls the unexported `perAliasFromContent`
directly. `resolutionDependentKeys` is likewise test-only. The `offenders` slice
that `perAliasFromContent` carefully sorts is discarded by `Statuses`
(`classify.go:225` uses `_` for it), so the IdentitiesOnly row can only ever say
`no` — it can never name *which* aliases are non-conforming, which is the one
actionable fact for a verify-only row.
**Fix:** Either unexport/remove `PerAliasConformance` and delete
`resolutionDependentKeys`, or carry `offenders` onto `OptionStatus` and render
them in the detail pane.

### WR-11: `Simulate` is always inconclusive when `~/.ssh/config` does not exist

**File:** `internal/globalssh/shadow.go:128-136`, `162-171`, `216`
**Issue:** `discover` returns early for a missing file **without** adding it to
`files` or `expanded`:

```go
content, err := os.ReadFile(abs)
if err != nil {
	if os.IsNotExist(err) { return nil }   // not appended to files
	...
}
```

The `!expanded[absTarget]` fallback at line 165 covers the managed *target* but
not the *entry point*. When the entry point is missing and differs from the
target (the fresh-machine Include layout), `graph.Files` never contains it, so
`Simulate` never writes `mirroredEntryPoint`, and `ssh -G -F <missing>` fails →
`Inconclusive` with a probe error. The D-04 pre-write shadow check is therefore
permanently unavailable in exactly the first-run case the phase targets.
**Fix:** Always seed `files` with the entry point, using empty content when the
file is absent:

```go
if err := discover(absEntry, 0); err != nil { return SimulationGraph{}, err }
if !expanded[absEntry] {
	files = append([]GraphFile{{Path: absEntry, Content: nil}}, files...)
	expanded[absEntry] = true
}
```

### WR-12: `extractGSSApplyHeading` can never absorb the wrapped continuation row its comment promises

**File:** `internal/screenshot/createflow_regions.go:955-971`
**Issue:**

```go
out = append(out, line)
if i+1 < len(lines) && strings.Contains(stripANSI(lines[i+1]), "Touches") {
	break
}
return strings.Join(out, "\n")
```

`lines[i+1]` is never appended, and both the `break` and the `return` produce the
same single-line result. The comment states the region *"absorbs an
immediately-following wrapped continuation row if the resolved target wraps past
the ceremony's width"* — it does not. A resolved target long enough to wrap
(the common case for a sandbox HOME like `/tmp/h4042748673/.ssh/config.d/gitid.config`)
silently drops its tail from the comparison, weakening the
`T-06-CEREMONYTARGET` disposition.
**Fix:**

```go
for i, line := range lines {
	if !strings.Contains(stripANSI(line), "Write Host * managed block to") { continue }
	out = append(out, line)
	for j := i + 1; j < len(lines) && !strings.Contains(stripANSI(lines[j]), "Touches"); j++ {
		out = append(out, lines[j])
	}
	break
}
```

### WR-13: `#` comment lines inside a backslash-continued Make recipe

**File:** `Makefile:336-338`
**Issue:**

```make
	fi; \
	# D-13: the dynamic version line's prefix (VersionNotePrefix) must never be
	# frozen — it changes with the user's OpenSSH build. Assembled at runtime so
	# this check line cannot satisfy (or trip) the grep itself.
	dyn_prefix="Your OpenSSH"; dyn_prefix="$$dyn_prefix:"; \
```

The `fi; \` continues into the first `#` line, which comments out the remainder of
that logical shell line. The next two `#` lines and the `dyn_prefix=` line become
separate recipe lines with **no `@` prefix**, so `make gate-copy-freeze` echoes
raw recipe text to stdout. More importantly, adding a trailing `\` to any of those
comment lines (a one-character edit) would silently swallow the entire D-13
exclusion check, and the gate would still report success.
**Fix:** Move the explanation above the target as a `##` doc-comment and keep the
recipe a single continued line:

```make
	fi; \
	dyn_prefix="Your OpenSSH"; dyn_prefix="$$dyn_prefix:"; \
	if grep -qF -- "$$dyn_prefix" Makefile; then \
```

### WR-14: A probe failure is classified as `needs-action`, inviting the user to "fix" something gitid could not read

**File:** `internal/globalssh/classify.go:242-244`, `274-276`, `302`
**Issue:** When `configErr`/`resolutionErr` is non-nil the row gets
`Source = SourceInconclusive` but `CurrentValue` stays `""`, and `stateFor`
(line 129-131) maps an empty value to `StateNeedsAction`. A machine with no
`ssh` on `PATH` therefore renders all six rows as "needs action" with a checkbox,
and `runGlobalSSHApply` will happily write them. The advisory posture is
preserved for the *explanation* but not for the *state*.
**Fix:** Short-circuit in `Statuses` — when `st.Source == SourceInconclusive`,
set `st.State = StateNotApplicable` with a new `ReasonProbeFailed`, so the row
renders its `ProbeError` and is not selectable. Mirror the existing
`ReasonVersionUnverified` precedent, which already withholds the write.

### WR-15: `Statuses` reads the user config twice concurrently and discards the second error

**File:** `internal/globalssh/classify.go:209-210`
**Issue:**

```go
go func() { defer wg.Done(); hits, configErr = fileHits(deps) }()   // calls deps.ReadConfig()
go func() { defer wg.Done(); _, config, _ = deps.ReadConfig() }()   // calls it again, error dropped
```

Two concurrent reads of the same file, and the second discards its error while
the code below (line 224) gates on `configErr` from the *first*. If the two
disagree (a write landing between them) `config` may be stale or nil while
`configErr` is nil, and `perAliasFromContent(nil)` silently reports
`total == 0` → `ReasonNothingToVerify`, hiding every non-conforming alias.
**Fix:** Read once and derive both:

```go
var path string
go func() {
	defer wg.Done()
	path, config, configErr = deps.ReadConfig()
	if configErr == nil {
		hits = indexHits(sshconfig.ScanDirectives(config, path, policyKeys()))
	}
}()
```

### WR-16: Two of the seven new visual-regression checkpoints are exempt from comparison on both surfaces

**File:** `cmd/gitid/gate_visual_regression_test.go:1171-1172`, `1298`, `1325`, `Makefile:485-508`
**Issue:** `gss-apply-receipt` and `gss-storage-migrate-receipt` are registered
as non-applicable on *both* the real and the dummy surface, so the byte
comparison the gate exists to perform never runs for them; the evidence is
deferred to two committed PTY frame files whose *existence* is asserted. A
committed frame is not a regression detector — it will not change when the
receipt rendering changes. 2 of 7 Phase 6 checkpoints therefore contribute no
regression coverage.
**Fix:** Either drive the receipt states in-process against a seeded sandbox HOME
(the storage PTY tests already prove a real journal-backed write is reachable in
a sandbox), or make the PTY frames themselves a golden comparison in
`gate-visual-regression` rather than an existence check.

### WR-17: `renderOptions` indexes an unguarded slice — panic on an empty option list

**File:** `internal/tuikit/globalssh.go:884`
**Issue:** `detail := options[selIdx]` runs with no length check. `handleKey`
guards `len(options) == 0` (line 516) and `view` guards `m.optionsErr != ""`
(line 839), but a backend that returns `(nil, nil)` — zero rows, no error —
reaches `renderOptions` and panics with an index-out-of-range on the first render.
`globalssh.Statuses` always returns `len(Policy)` rows today, but
`GlobalSSHOptionStates` is a `Backend` interface method that any implementation
may satisfy, and `NoopGlobalSSHPlanner` exists precisely to be substituted.
**Fix:**

```go
if len(options) == 0 {
	return m.subTabStrip() + "\n " + styleFaint.Render("No global SSH options to show.")
}
```

### WR-18: Post-migration refetch plans the reverse migration and stores it as the pending plan

**File:** `internal/tuikit/globalssh.go:203-209`
**Issue:**

```go
// Refetch the storage view so the current-layout marker moves to the new layout.
view, verr := m.backend.SSHStorageMigrationPlan(s.SSHStorage)
```

`s` is the state *before* the `SetSSHStorage` reducer runs, so `s.SSHStorage` is
the **old** layout. On disk the layout is now the new one, so
`SSHStorageMigrationPlan` succeeds and returns a plan for migrating **back** —
and `putPendingMigration` stores it as the live pending plan with a fresh token.
The stated purpose ("so the current-layout marker moves") is not achieved either:
`renderStorage`'s marker comes from `s.SSHStorage` (line 923), not from the view.
**Fix:** Drop the refetch here and let the next `activate` re-plan after the
reducer has updated `DemoState`, or pass the confirmed `layout` and treat the
resulting view as informational only without storing a pending plan.

## Info

_None. All findings above are classified BLOCKER or WARNING._

---

_Reviewed: 2026-08-27_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
