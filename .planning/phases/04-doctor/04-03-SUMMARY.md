---
phase: "04-doctor"
plan: "03"
subsystem: doctor
tags: [doctor, coherence, orphans, tdd, identity, allowed_signers, gpg-format]
dependency_graph:
  requires: ["04-01", "04-02"]
  provides: [CheckCoherence, CheckOrphans]
  affects: [plans-04-04-04-05, gitid-doctor-output]
tech_stack:
  added: []
  patterns:
    - injected-deps
    - TDD-RED-GREEN
    - stub-overwrite-in-place
    - byte-exact-email-comparison
    - orphan-vs-incomplete-distinction
key_files:
  created:
    - internal/doctor/checks/coherence_test.go
    - internal/doctor/checks/orphans_test.go
  modified:
    - internal/doctor/checks/coherence.go
    - internal/doctor/checks/orphans.go
    - internal/doctor/doctor.go
    - internal/gitconfig/reader.go
    - internal/sshconfig/reader.go
    - cmd/gitid/doctor.go
decisions:
  - "CheckCoherence receives identity.Account slice via deps.Identities (Plan 02 additive pattern); checks package imports identity directly (no cycle: checks→doctor→identity, checks→identity, no reverse)"
  - "Incomplete account (managed block exists, artifact missing) classified under Coherence, never Orphans (Pitfall 5 / D-09)"
  - "gpg.format check skips to allowedSigners early-return on err!=nil to avoid empty-block lint (revive); err==nil && gpgFmt!='ssh' is the trigger"
  - "Orphan Class 1: SSH block without gitconfig counterpart; Class 2: gitconfig block without SSH counterpart; Class 3: key file not in AllSSHHostIdentityFiles"
  - "findSignerLine uses strings.EqualFold to detect case-differing principals (Pitfall 6) and returns them as email-mismatch findings; byte-exact == used for 'found' case"
  - "gitconfig.RunGitConfigGet added as standalone injectable seam (arg-slice exec.Command G204); wired in buildDoctorDeps"
  - "sshconfig.ParseAllHostIdentityFiles added to parse ALL Host IdentityFile paths (managed + hand-written) for D-12 cross-reference"
  - "SSHManagedBlockNames derived from ParseManagedHosts result keys in buildDoctorDeps; GitconfigManagedBlockNames from filewriter.ListBlocks on gcBytes"
metrics:
  duration: "~10min"
  completed: "2026-06-11"
  tasks: 2
  files: 8
---

# Phase 04 Plan 03: Coherence + Orphans Families Summary

Coherence (DOC-03) checks existence/resolution of managed identity artifacts — IdentityFile,
includeIf fragment, IdentitiesOnly yes, allowed_signers line, gpg.format=ssh, and email
byte-match. Orphans (DOC-04) cross-references managed block names vs disk presence to detect
unowned artifacts. Both stub files from Plan 01 were overwritten in place — no redeclaration.

## Commits

| Commit | Message |
|--------|---------|
| `6cce435` | test(04-03): add failing tests for CheckCoherence (RED) |
| `4f881e4` | feat(04-03): implement CheckCoherence — existence/resolution + locked-value carve-outs (GREEN) |
| `6fbcb20` | test(04-03): add failing tests for CheckOrphans (RED) |
| `efd01ad` | feat(04-03): implement CheckOrphans + wire new Deps fields in cmd layer (GREEN) |

## TDD Gate Compliance

- RED gate (Task 1): `6cce435` — test commit precedes GREEN `4f881e4`.
- RED gate (Task 2): `6fbcb20` — test commit precedes GREEN `efd01ad`.
- GREEN gate (Task 1): `4f881e4` — implements after RED is confirmed (7 failures → 8 passes).
- GREEN gate (Task 2): `efd01ad` — implements after RED is confirmed (3 failures → 6 passes).
- No REFACTOR commits needed (lint-clean on first pass after goimports fix).

## Artifacts Produced

### internal/doctor/checks/coherence.go (OVERWRITES Plan 01 stub)

- `CheckCoherence(deps doctor.Deps) []doctor.Finding` — real implementation replacing stub.
- Per-account coherenceForAccount iterates `deps.Identities []identity.Account`.
- Check 1: KeyPath Stat→ErrNotExist → error, no Fix (D-03 report-only).
- Check 2: FragmentPath Stat→ErrNotExist → error, no Fix.
- Check 3: ManagedHosts[name].IdentitiesOnly==false → error + Fix descriptor (D-02; Plan 05 wires Fn).
- Check 4: RunGitConfigGet(fragmentPath, "gpg.format") != "ssh" → error, no Fix (D-17 locked-value).
- Check 5a: No allowed_signers line with namespaces="git" for this email → error + Fix (D-02).
- Check 5b: Line found but principal != email (byte-exact Pitfall 6) → email mismatch error + Fix.
- Check 6: account.Incomplete != "" → Coherence finding (Pitfall 5: never Orphans, D-09).

### internal/doctor/checks/orphans.go (OVERWRITES Plan 01 stub)

- `CheckOrphans(deps doctor.Deps) []doctor.Finding` — real implementation replacing stub.
- Incomplete accounts guarded: their names excluded from orphan checks (Pitfall 5).
- Class 1: SSH managed block name not in GitconfigManagedBlockNames → warning + Fix (D-11).
- Class 2: gitconfig managed block name not in SSHManagedBlockNames → warning + Fix (D-11).
- Class 3: key in KeyPaths, Stat→OK, path not in AllSSHHostIdentityFiles → warning, NO Fix.
  D-13 honest wording: "may be used for direct server SSH or 'ssh -i' — review before deleting".
- Never reads known_hosts (D-14). No filewriter import (D-01).

### internal/doctor/doctor.go (extended)

Added to `Deps` struct (additive, non-breaking):
- `Identities []identity.Account` — pre-reconstructed accounts (coherence + orphan data source).
- `ManagedHosts map[string]sshconfig.SSHHostInfo` — SSH Host block info for IdentitiesOnly check.
- `GitconfigManagedBlockNames []string` — names of gitconfig managed blocks (orphan detection).
- `SSHManagedBlockNames []string` — names of SSH managed blocks (orphan detection).
- `AllSSHHostIdentityFiles []string` — every IdentityFile from every Host block (D-12 cross-ref).

### internal/gitconfig/reader.go (extended)

- `RunGitConfigGet(file, key string) (string, error)` — runs `git config --file <file> <key>`;
  injectable seam for the gpg.format locked-value check (D-17); G204-annotated.

### internal/sshconfig/reader.go (extended)

- `ParseAllHostIdentityFiles(content []byte) []string` — parses ALL Host blocks (not just managed
  ones) and returns every unique IdentityFile path. Used for D-12 unused-key cross-reference.

### cmd/gitid/doctor.go (wiring additions)

`buildDoctorDeps` now wires:
- `RunGitConfigGet: func(file, key) { return gitconfig.RunGitConfigGet(file, key) }`
- `Identities: accounts` (already reconstructed for perms check)
- `ManagedHosts: managedHosts` (from sshconfig.ParseManagedHosts)
- `SSHManagedBlockNames: sshBlockNames` (keys of managedHosts map)
- `GitconfigManagedBlockNames: gcBlockNames` (from filewriter.ListBlocks on gcBytes)
- `AllSSHHostIdentityFiles: allSSHHostIDFiles` (from sshconfig.ParseAllHostIdentityFiles)

---

## Deps Fields Used (Wave-2 Contract Confirmation)

| Field | Type | Purpose in Plan 03 |
|-------|------|--------------------|
| `Identities` | `[]identity.Account` | Account list for Coherence/Orphans iteration |
| `ManagedHosts` | `map[string]sshconfig.SSHHostInfo` | IdentitiesOnly check per SSH block |
| `GitconfigManagedBlockNames` | `[]string` | Orphan Class 2 detection |
| `SSHManagedBlockNames` | `[]string` | Orphan Class 1 detection |
| `AllSSHHostIdentityFiles` | `[]string` | D-12 unused-key cross-reference |
| `Stat` | `func(path string) (os.FileInfo, error)` | Existence checks (Checks 1, 2; Orphan Class 3) |
| `ReadFile` | `func(path string) ([]byte, error)` | Read allowed_signers bytes |
| `RunGitConfigGet` | `func(file, key string) (string, error)` | gpg.format locked-value check |
| `AllowedSignersPath` | `string` | Path passed to ReadFile for signer lookup |
| `KeyPaths` | `[]string` | Orphan Class 3 key cross-reference (already existed) |

---

## Fix Descriptors for Plan 05

| Finding | Fix non-nil? | Plan 05 action |
|---------|-------------|----------------|
| IdentityFile missing | NO (report-only) | User must re-run `gitid identity add` |
| Fragment missing | NO (report-only) | User must re-run `gitid identity add` |
| IdentitiesOnly missing | YES | Wire Fn to `sshconfig.AddIdentitiesOnly` via cmd layer |
| allowed_signers entry missing | YES | Wire Fn to `gitconfig.WriteAllowedSigners` via cmd layer |
| email mismatch | YES | Wire Fn to repair the signer line |
| gpg.format mismatch | NO (report-only) | User must run `git config --file ...` |
| Orphan SSH block | YES | Wire Fn to `filewriter.RemoveBlock` on ssh config |
| Orphan gitconfig block | YES | Wire Fn to `filewriter.RemoveBlock` on gitconfig |
| Unused key | NO (D-03/D-13) | Report-only; user must review and decide |

---

## Deviations from Plan

### Rule 2: Added RunGitConfigGet to gitconfig package

**Found during:** Task 1 GREEN

**Issue:** The plan assumed `RunGitConfigGet` already existed as an injectable seam in
`internal/gitconfig`. It did not — the `ReadFragment` function used `exec.Command` internally
but there was no standalone `RunGitConfigGet` function for the check layer to call via injection.

**Fix:** Added `RunGitConfigGet(file, key string) (string, error)` to `internal/gitconfig/reader.go`.
Wired as a closure in `buildDoctorDeps`. The `doctor.Deps.RunGitConfigGet` field already existed
in the Deps contract from Plan 01; it just needed a real backing function.

**Files modified:** `internal/gitconfig/reader.go`, `cmd/gitid/doctor.go`
**Commit:** `efd01ad`

### Rule 2: Added ParseAllHostIdentityFiles to sshconfig package

**Found during:** Task 1 RED design (D-12 cross-reference requires all Host blocks)

**Issue:** `sshconfig.ParseManagedHosts` only returns gitid-managed blocks. D-12 requires
ALL Host blocks (managed + hand-written) to be scanned for IdentityFile paths.

**Fix:** Added `ParseAllHostIdentityFiles(content []byte) []string` to `internal/sshconfig/reader.go`.
Parses the full ssh_config (using the kevinburke parser) and returns unique IdentityFile paths.

**Files modified:** `internal/sshconfig/reader.go`
**Commit:** `6cce435` (in the RED commit as infrastructure alongside test)

### Architecture: Identities, ManagedHosts, block name fields added to doctor.Deps

**Found during:** Task 1 RED design

**Issue:** Plan 01's frozen Deps contract did not include `Identities`, `ManagedHosts`,
`GitconfigManagedBlockNames`, `SSHManagedBlockNames`, or `AllSSHHostIdentityFiles`. These are
required for fake-testable coherence and orphan checks.

**Fix:** Added five fields additively to `doctor.Deps`. Follows the established Plan 02 precedent
(`DetectTools`, `ReadBaselineState`, `BaselineFilePath`, `GitignorePath` were similarly added).
No existing fields renamed or removed. All Wave-2 plans wire against exact existing field names.

**Files modified:** `internal/doctor/doctor.go`
**Commit:** `6cce435`

---

## Known Stubs

| File | Function | Reason |
|------|----------|--------|
| `internal/doctor/checks/coherence.go` | `Fix.Fn` closures | Return nil — Plan 05 wires the actual AddWiring/signer-write fixer |
| `internal/doctor/checks/orphans.go` | `Fix.Fn` closures | Return nil — Plan 05 wires the actual RemoveBlock fixer |
| `internal/doctor/checks/signing.go` | `CheckSigning`, `CheckAgent` | Returns nil — real implementation in Plan 04 |

These stubs are intentional by plan design. The `[fix]` marker renders correctly in
`gitid doctor` output, but the actual mutation will only run after Plan 05 wires the Fn closures.

---

## Threat Surface Scan

No new network endpoints, auth paths, or schema changes beyond what is documented in the
plan's threat model (T-04-08 through T-04-SC).

T-04-10 (Spoofing via allowed_signers email case-mismatch): mitigated. `findSignerLine` uses
byte-exact `==` comparison as the "found" check; `strings.EqualFold` is used only to DETECT
case-differing mismatches for the email-mismatch finding. The result: case-differing principals
are never silently accepted as valid — they surface as errors.

T-04-09 (Tampering — mis-classifying user's in-use key as deletable): mitigated. `CheckOrphans`
cross-references `AllSSHHostIdentityFiles` (populated from ALL Host blocks via
`ParseAllHostIdentityFiles`) before flagging any key. No auto-fix for key files (D-03/D-13).

T-04-11 (Tampering — doctor gaining write capability): mitigated. No `filewriter` import in
`internal/doctor`. Fix.Fn closures return `nil` (no-ops); Plan 05 wires real removals.

## Self-Check: PASSED

Files exist:
- `internal/doctor/checks/coherence.go` ✓
- `internal/doctor/checks/coherence_test.go` ✓
- `internal/doctor/checks/orphans.go` ✓
- `internal/doctor/checks/orphans_test.go` ✓
- `internal/doctor/doctor.go` ✓
- `internal/gitconfig/reader.go` ✓
- `internal/sshconfig/reader.go` ✓
- `cmd/gitid/doctor.go` ✓

Commits verified:
- `6cce435` (RED 1) ✓
- `4f881e4` (GREEN 1) ✓
- `6fbcb20` (RED 2) ✓
- `efd01ad` (GREEN 2) ✓

Single-definition verification:
- `grep -rn 'func CheckCoherence' internal/doctor/checks/` → exactly one match ✓
- `grep -rn 'func CheckOrphans' internal/doctor/checks/` → exactly one match ✓

No filewriter import in internal/doctor: ✓
No known_hosts reads in internal/doctor: ✓
`go build ./...` green: ✓
`go test ./internal/doctor/...` green: ✓
`go test ./...` green: ✓
`make lint` green: ✓
