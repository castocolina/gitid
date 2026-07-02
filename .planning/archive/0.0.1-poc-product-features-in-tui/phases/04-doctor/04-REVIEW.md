---
phase: 04-doctor
reviewed: 2026-06-12T00:00:00Z
depth: standard
files_reviewed: 21
files_reviewed_list:
  - cmd/gitid/doctor.go
  - cmd/gitid/doctor_test.go
  - cmd/gitid/main.go
  - internal/doctor/checks/baseline.go
  - internal/doctor/checks/baseline_test.go
  - internal/doctor/checks/coherence.go
  - internal/doctor/checks/coherence_test.go
  - internal/doctor/checks/deps.go
  - internal/doctor/checks/deps_test.go
  - internal/doctor/checks/orphans.go
  - internal/doctor/checks/orphans_test.go
  - internal/doctor/checks/perms.go
  - internal/doctor/checks/perms_test.go
  - internal/doctor/checks/signing.go
  - internal/doctor/checks/signing_test.go
  - internal/doctor/doctor.go
  - internal/doctor/doctor_test.go
  - internal/gitconfig/reader.go
  - internal/platform/platform.go
  - internal/platform/platform_test.go
  - internal/sshconfig/reader.go
findings:
  critical: 3
  warning: 6
  info: 3
  total: 12
status: issues_found
---

# Phase 4: Code Review Report

**Reviewed:** 2026-06-12
**Depth:** standard
**Files Reviewed:** 21
**Status:** issues_found

## Summary

This phase ships `gitid doctor`: seven read-only check families plus an opt-in
`--fix`/`--yes` auto-repair flow. The read-only checks and the `applyFixes`
consent flow (gate defaults to no, perms batched, orphans/coherence individually
confirmed, pre-fix exit code captured) are largely correct and well-tested.

However, the review found three BLOCKER-class defects that undermine the central
promise of the phase — that auto-fix mutations are safe and real:

1. **Every non-perms fix is a silent no-op.** The `Fn` closures for orphan,
   coherence, and baseline findings are `func() error { return nil }` stubs.
   `--fix --yes` prints `fixed:` and increments the applied tally for fixes that
   never touch any file. The wired `RemoveBlock`/`AddWiring` chokepoint closures
   in `buildDoctorDeps` are never plumbed into the finding `Fn` fields. This is
   acknowledged in 04-05-SUMMARY.md but ships anyway.
2. **The Agent family check is dead.** `buildDoctorDeps` never wires `RunSSHAdd`
   or `RunSSHKeygenFingerprint`, so `CheckAgent` always early-returns nil and the
   Agent section reports "all checks passed" unconditionally — masking a down
   agent or unloaded keys.
3. **`runDoctor` reads real `os.Stdin` even under `--yes`,** and the gate/exit
   semantics around an interactive-vs-piped session can block or misreport.

The narrative findings below detail these plus quality issues in the signer-line
scanner and the hardcoded mode in the `RemoveBlock` closure.

## Critical Issues

### CR-01: Orphan / coherence / baseline fixes are silent no-ops that report success

**File:** `internal/doctor/checks/orphans.go:69`, `internal/doctor/checks/orphans.go:92`, `internal/doctor/checks/coherence.go:118`, `internal/doctor/checks/coherence.go:192`, `internal/doctor/checks/coherence.go:212`, `internal/doctor/checks/baseline.go:63`, `internal/doctor/checks/baseline.go:120`

**Issue:** Every fixable finding outside `FamilyPerms` carries
`Fn: func() error { return nil }`. `applyFixes` (cmd/gitid/doctor.go:454,
:469, :492, :502) calls `f.Fix.Fn()`, and on a nil error prints
`  fixed: <summary>` and increments `applied`. So when a user runs
`gitid doctor --fix --yes` against a real environment with an orphaned managed
block or a missing `allowed_signers` entry, the tool reports the fix as applied
and exits as if the repair succeeded — but no file is mutated. The real
chokepoint closures `RemoveBlock` and `AddWiring` are wired in
`buildDoctorDeps` (cmd/gitid/doctor.go:208, :230) yet are never referenced by
any check, so they are unreachable in the production path. 04-05-SUMMARY.md
(lines 129+) documents this as a "known scope boundary," but a fix flow that
claims success while doing nothing is a data-integrity/trust defect, not a cosmetic
gap — a user who trusts the `fixed:` line will believe their config is repaired.

**Fix:** Plumb the injected fixers through the finding `Fn` fields. Pass the
relevant `deps.RemoveBlock` / `deps.AddWiring` into the closures, e.g. in
orphans.go:

```go
Fix: &doctor.FixDescriptor{
    Summary: fmt.Sprintf("remove orphaned SSH Host block %q", n),
    Fn: func() error {
        return deps.RemoveBlock(deps.SSHConfigPath, n)
    },
},
```

and similarly route coherence IdentitiesOnly / allowed_signers re-adds through
`deps.AddWiring(...)` with the documented `ssh-host:` / `signers:` line encoding,
and baseline restores through `deps.AddWiring(deps.GitconfigPath, "baseline-include",
"baseline-include:"+deps.BaselineFilePath)`. Until plumbed, these findings must
NOT advertise `Fix != nil` — leave `Fix: nil` so the report does not render a
`[fix]` marker and `applyFixes` never claims a phantom repair.

### CR-02: Agent check is permanently dead — RunSSHAdd / RunSSHKeygenFingerprint never wired

**File:** `cmd/gitid/doctor.go:155-287` (buildDoctorDeps), `internal/doctor/checks/signing.go:83-86`

**Issue:** `CheckAgent` guards on `if deps.RunSSHAdd == nil { return nil }`
(signing.go:84). `buildDoctorDeps` wires `RunGitConfigGet`, `DetectTools`,
`GitVersionAtLeast`, etc., but never assigns `RunSSHAdd` or
`RunSSHKeygenFingerprint`. Both remain nil in production, so `CheckAgent` always
returns nil and the `=== Agent ===` section always prints
`✓ all checks passed`. A user whose ssh-agent is down, or whose managed key is
not loaded, gets a clean Agent report — the exact failure the family exists to
catch is invisible. The unit tests pass because they inject these fields directly
in `checks` tests and never exercise the `buildDoctorDeps` wiring for Agent.

**Fix:** Wire both seams in `buildDoctorDeps`, e.g.:

```go
RunSSHAdd: func() (string, int) {
    return deps.RunSSHAdd() // wrap the real ssh-add -l runner from internal/deps
},
RunSSHKeygenFingerprint: func(path string) (string, error) {
    return keygen.Fingerprint(path) // or the appropriate existing runner
},
```

Add a `runDoctor`-level integration test that asserts the Agent family produces a
finding when `ssh-add -l` reports an unreachable agent, so the wiring cannot
silently regress.

### CR-03: runDoctor reads real os.Stdin unconditionally, including under --fix --yes

**File:** `cmd/gitid/doctor.go:102-105`

**Issue:** `runDoctor` constructs `bufio.NewReader(os.Stdin)` and calls
`applyFixes` whenever `len(fixable) > 0`, regardless of `fix`/`yes`. In `--yes`
mode `applyFixes` never reads from `r`, so this is harmless there; but in the
bare `gitid doctor` path (fix=false) with output piped to a file or running in
CI with stdin not a TTY, the gate `confirm` call (`r.ReadString('\n')`) returns
immediately on EOF and is treated as "no" — acceptable. The real defect is the
combination with CR-01/the success reporting: there is no guard that stdin is
interactive before prompting, so `gitid doctor` invoked non-interactively (cron,
CI) will print an `Apply N fix(es)? [y/N]:` prompt to stdout that no one can
answer, polluting machine-parsed report output. Additionally, `applyFixes` is
invoked even when the only fixable findings are the CR-01 no-op stubs, so the
prompt offers "fixes" that cannot do anything.

**Fix:** Gate the prompt on an interactive stdin and on `fix`:

```go
if len(fixable) > 0 && (fix || isTerminalInput(os.Stdin)) {
    in := bufio.NewReader(os.Stdin)
    applyFixes(in, out, fixable, fix, yes)
}
```

where `isTerminalInput` mirrors `isTerminalOutput` (checks `ModeCharDevice`).
Non-interactive bare `doctor` should skip the gate entirely and rely on the exit
code, not emit an unanswerable prompt.

## Warnings

### WR-01: findSignerLine returns on first case-insensitive hit, can false-flag a mismatch

**File:** `internal/doctor/checks/coherence.go:227-252`

**Issue:** `findSignerLine` iterates lines and returns on the first line whose
principal `EqualFold`s the email (coherence.go:247-248). If `allowed_signers`
contains a case-differing line *before* an exact byte-match line for the same
email, the function returns the case-differing principal and the caller
(coherence.go:178) emits a spurious "email mismatch" error even though a correct
entry exists later in the file. The byte-exact branch only wins if it appears
first.

**Fix:** Scan all lines, prefer an exact match over any case-fold match:

```go
func findSignerLine(content, email string) (found bool, firstField string) {
    var caseFold string
    for _, line := range strings.Split(content, "\n") {
        // ... skip blank/comment, require namespaces="git" ...
        if principal == email {
            return true, principal // exact wins immediately
        }
        if caseFold == "" && strings.EqualFold(principal, email) {
            caseFold = principal // remember but keep scanning for an exact match
        }
    }
    if caseFold != "" {
        return true, caseFold
    }
    return false, ""
}
```

### WR-02: RemoveBlock closure hardcodes mode 0600 — wrong for allowed_signers

**File:** `cmd/gitid/doctor.go:208-219`

**Issue:** The `RemoveBlock` closure hardcodes `mode := os.FileMode(0o600)` for
every file it rewrites, with the comment "config files are always 0600." That is
correct for `~/.ssh/config` and `~/.gitconfig`, but `~/.ssh/allowed_signers` is
mode 0644 (it is public, not secret — see `keygen.allowedSignersMode = 0o644`).
Once CR-01 is fixed and orphaned signer blocks are removed via this closure (or
if any future caller passes the allowed_signers path), this will tighten a
public file to 0600. `filewriter.Write` sets the mode explicitly, so the file's
mode would be silently changed on every removal.

**Fix:** Derive the mode from the target path, or preserve the existing file
mode by stat-ing before rewrite:

```go
mode := os.FileMode(0o600)
if path == allowedSignersPath { // or: stat existing file and reuse its perm
    mode = 0o644
}
```

Preferably pass the intended mode in from the caller rather than inferring it.

### WR-03: gitconfig perms check expects 0600 but standard ~/.gitconfig is 0644

**File:** `internal/doctor/checks/perms.go:50-51`, `internal/doctor/checks/perms.go:18`

**Issue:** `CheckPermissions` checks `~/.gitconfig` against `modeSSHConfig =
0o600` at SeverityError. A `~/.gitconfig` created by `git` itself is typically
0644, and it contains no secret material (it is not a key). Flagging every
default `~/.gitconfig` as an error and offering `chmod 0600` as a fix will
produce a noisy false positive on virtually every machine, and tightening it to
0600 has no security benefit while diverging from the mode git writes. This will
also make the bare-home test path emit an error finding for an unrelated reason.

**Fix:** Either drop the `~/.gitconfig` mode check entirely, or check it against
0644 at warning severity only when it is group/world-*writable* (the actual risk
is write, not read). At minimum, do not classify a world-readable gitconfig as a
SeverityError.

### WR-04: Port parse failure in AddWiring silently defaults to 22, masking malformed input

**File:** `cmd/gitid/doctor.go:241-246`

**Issue:** In the `ssh-host:` AddWiring branch, a `portStr` that fails
`fmt.Sscanf("%d")` is silently coerced to `port = 22`. For a gitid alt-ssh
endpoint that legitimately uses 443 (noted in sshconfig/reader.go:99-101, WR-06),
a malformed or truncated payload would rewrite the Host block with Port 22,
breaking connectivity to that host. The error is swallowed.

**Fix:** Return an error on an unparseable non-empty port instead of defaulting:

```go
if portStr != "" {
    n, err := strconv.Atoi(portStr)
    if err != nil {
        return fmt.Errorf("doctor: AddWiring ssh-host: bad port %q: %w", portStr, err)
    }
    port = n
}
```

### WR-05: --yes failure path still increments nothing but reports incomplete tally honestly only by luck

**File:** `cmd/gitid/doctor.go:451-460`, `cmd/gitid/doctor.go:490-497`

**Issue:** In `--yes` mode, a `Fn` error prints `doctor: fix failed: ...` and
does NOT increment `applied`, which is correct. But combined with CR-01, the
no-op stubs always return nil, so the tally will report orphan/coherence/baseline
fixes as `applied` even though nothing happened — the tally is only "honest" for
perms today. This is a corollary of CR-01 but worth tracking separately: the
`applied`/`skipped` accounting must reflect actual file mutations, which requires
the real fixers to be wired and to return real errors (e.g. a failed
`filewriter.Write` on a read-only filesystem). Until then the tally
overstates success.

**Fix:** Resolve CR-01; then add a test that injects a `RemoveBlock` returning an
error and asserts the orphan finding is counted as failed (not applied).

### WR-06: Pre-fix exit code can mislead after a successful full repair (no re-check)

**File:** `cmd/gitid/doctor.go:88-108`

**Issue:** `runDoctor` returns the pre-fix severity unconditionally (D-07, by
design) — even when `--fix --yes` repairs everything. The doc comment frames this
as intentional ("CI is never misled into thinking the env was already healthy").
That is defensible, but the user-facing effect is that `gitid doctor --fix --yes`
on a fully-repairable environment prints `fixed:` for everything and then exits
3, with no second-pass re-check to confirm the repairs actually cleared the
findings. Given CR-01 (fixes may be no-ops), there is no feedback loop that would
expose a fix that claimed success but changed nothing. The exit code alone cannot
distinguish "I fixed it but report pre-fix state" from "I pretended to fix it."

**Fix:** After applying fixes, re-run the affected checks (or the full suite) and
print a post-fix summary line (e.g. `doctor: 2 finding(s) remain after fixes`)
while still returning the pre-fix code for CI. This makes a phantom no-op fix
observable.

## Info

### IN-01: parseIncludeIfBody assumes single-space `path = ` prefix

**File:** `internal/gitconfig/reader.go:52-54`

**Issue:** `strings.HasPrefix(line, "path = ")` requires exactly one space on
each side of `=`. A fragment written with `path=~/...` or `path  =  ~/...` (git
permits both) would not be detected and `FragmentPath` would be empty, cascading
into a false coherence finding. Since gitid controls the write format this is
low-risk today, but it is brittle against hand edits.

**Fix:** Trim around `=`: split on the first `=`, compare the trimmed key to
`path`, and trim the value.

### IN-02: Unused/dead struct fields and `_ = calls` test scaffolding

**File:** `cmd/gitid/doctor_test.go:244-247`

**Issue:** `TestDoctorPermsBatched` declares `calls := 0` then immediately
`_ = calls`, dead scaffolding left in the committed test. Minor, but it signals
incomplete cleanup.

**Fix:** Remove `calls` and the `_ = calls` line.

### IN-03: Sentinel "exit code %d" error string is brittle

**File:** `cmd/gitid/doctor.go:41`

**Issue:** `runDoctor`'s non-zero return is converted to
`fmt.Errorf("exit code %d", code)` and propagated to Cobra, but `main()`
(main.go:13) maps any non-nil error to `os.Exit(1)` — so the tiered 0/1/2/3 exit
code computed by `doctor.ExitCode` is collapsed to 1 at the process boundary. The
report prints `exit code: 3` but the process actually exits 1. CI that keys on
`$?` will see 1 for every non-clean run, losing the critical/error/warning
distinction the D-07 tiering was built to provide.

**Fix:** Have the doctor command set the real process exit code (e.g. via a
package-level `exitCode` captured by `main`, or `cmd.Root().SetVersionTemplate`-
style propagation), so `gitid doctor` exits with 3 on critical, 2 on error, 1 on
warning/info. As written, the entire tiered-exit feature is defeated at the
`main()` boundary — this is arguably a BLOCKER for any CI consumer; classified
Info only because the in-report number is still correct and the workflow may
treat process-exit plumbing as a separate follow-up.

---

_Reviewed: 2026-06-12_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
