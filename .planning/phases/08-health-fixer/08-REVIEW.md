---
phase: 08-health-fixer
reviewed: 2026-08-28T00:00:00Z
depth: standard
files_reviewed: 64
files_reviewed_list:
  - CLAUDE.md
  - Makefile
  - cmd/gitid/doctor_alias.go
  - cmd/gitid/doctor_alias_test.go
  - cmd/gitid/fix.go
  - cmd/gitid/fix_test.go
  - cmd/gitid/gate_visual_regression_test.go
  - cmd/gitid/health.go
  - cmd/gitid/health_test.go
  - cmd/gitid/identity.go
  - cmd/gitid/identity_test.go
  - cmd/gitid/main.go
  - cmd/gitid/main_test.go
  - cmd/gitid/wiring.go
  - cmd/gitid/wiring_storage_test.go
  - cmd/gitid/wiring_test.go
  - docs/cli-parity-matrix.md
  - e2e/dummy_demo_e2e_test.go
  - e2e/git_configuration_pty_e2e_test.go
  - e2e/global_git_pty_e2e_test.go
  - e2e/health_fix_cli_e2e_test.go
  - e2e/health_fixer_pty_e2e_test.go
  - e2e/identity_manager_pty_e2e_test.go
  - e2e/ui_pty_e2e_test.go
  - internal/doctor/checks/baseline.go
  - internal/doctor/checks/baseline_test.go
  - internal/doctor/checks/coherence.go
  - internal/doctor/checks/coherence_test.go
  - internal/doctor/checks/deps.go
  - internal/doctor/checks/files.go
  - internal/doctor/checks/files_test.go
  - internal/doctor/checks/orphans.go
  - internal/doctor/checks/orphans_test.go
  - internal/doctor/checks/perms.go
  - internal/doctor/checks/redundancy.go
  - internal/doctor/checks/signing.go
  - internal/doctor/doctor.go
  - internal/doctor/doctor_test.go
  - internal/dummytui/fixturebackend.go
  - internal/screenshot/createflow.go
  - internal/screenshot/createflow_regions.go
  - internal/sshconfig/reader.go
  - internal/sshconfig/rewrite.go
  - internal/sshconfig/rewrite_test.go
  - internal/tuikit/app.go
  - internal/tuikit/app_test.go
  - internal/tuikit/backend.go
  - internal/tuikit/backend_stub_test.go
  - internal/tuikit/batch3_test.go
  - internal/tuikit/ceremony.go
  - internal/tuikit/doctor.go
  - internal/tuikit/doctor_test.go
  - internal/tuikit/fixer_screen.go
  - internal/tuikit/fixer_screen_test.go
  - internal/tuikit/frame.go
  - internal/tuikit/frame_test.go
  - internal/tuikit/globalgit_test.go
  - internal/tuikit/globalssh.go
  - internal/tuikit/health_screen.go
  - internal/tuikit/health_screen_test.go
  - internal/tuikit/identities.go
  - internal/tuikit/identities_test.go
  - internal/tuikit/mouse_test.go
  - internal/tuikit/store.go
findings:
  critical: 1
  warning: 4
  info: 3
  total: 8
status: issues_found
---

# Phase 8: Code Review Report — Health + Fixer

**Reviewed:** 2026-08-28T00:00:00Z
**Depth:** standard
**Files Reviewed:** 64
**Status:** issues_found

## Summary

This phase splits the combined Doctor screen into Health (read-only) and Fixer
(apply) tabs, adds new check families (Files/parse-gate, several new Coherence
checks), wires `doctor.Run()` as the single findings source, and adds CLI
parity (`gitid health`, `gitid fix`, hidden `gitid doctor`). The D-09/D-10
surgical single-directive rewrite ceremony (`internal/sshconfig/rewrite.go`) —
the single highest-risk write path in this phase — is careful and well
constructed: value/control-byte validation, re-locate-don't-trust-a-line-
number, parse→render→re-parse stability check, a real `ssh -G` post-write
re-verification with a bounded/killed subprocess, and automatic restore from
backup on any verification failure. `gitid fix`'s CLI batch walk also
correctly gates on `doctor.Finding.Fix != nil`.

However, the **TUI's own notion of "fixable" does not use that same signal**.
Both the Fixer tab (`fixableFindings`) and the Health tab's "fixable" label use
`SuggestedFix != ""` as their sole criterion, while a large fraction of the
new check families' findings are explicitly **report-only** (`Fix: nil`) but
still carry non-empty `SuggestedFix` text (by design — the text should always
tell the user what to do by hand). The conversion in `cmd/gitid/wiring.go`
(`runDoctorAndConvert`) copies `SuggestedFix` unconditionally and never
threads whether a real `Fix` descriptor exists. The net effect: the Fixer tab
offers an "f · Fix this…" ceremony for report-only findings, walks the user
through a preview/confirm/backup ceremony with a **fabricated** backup path,
and on confirm shows a fake "✓ … applied" success receipt — while
`persistFixFinding` silently no-ops because there is no `Fix.Fn` to call. See
CR-01 below; this is the standout finding of the review.

Three further Warning-level issues and three Info-level issues are listed
below. No SQL/command-injection, hardcoded-secret, or unsafe-deserialization
issues were found; every `exec.Command` invocation in the reviewed files uses
the arg-slice form with fixed/trusted paths.

## Structural Findings (fallow)

None provided for this review.

## Narrative Findings (AI reviewer)

## Critical Issues

### CR-01: Health/Fixer treat report-only findings as auto-fixable, producing a fake "success" write+backup receipt

**File:** `internal/tuikit/doctor.go:104-113` (`fixableFindings`), `internal/tuikit/health_screen.go:186-189,205-207`, `cmd/gitid/wiring.go:4033-4067` (`runDoctorAndConvert`)

**Issue:** `doctor.Finding.Fix` (a `*FixDescriptor`, nil for report-only
findings) is the authoritative "is this really auto-fixable" signal —
`cmd/gitid/fix.go`'s CLI walk gets this right (`firstFixable` filters on
`f.Fix != nil`, and the dry-run loop does `if f.Fix == nil { continue }`).
The TUI does not use this signal at all. `internal/tuikit.DemoFinding` /
`HealthFinding` never carries a `Fixable` bit; `fixableFindings` and
`health_screen.go`'s `fixNote`/`"Switch to Fixer to apply this."` hint both
key off `SuggestedFix != ""` instead:

```go
// internal/tuikit/doctor.go
func fixableFindings(ordered []DemoFinding) []DemoFinding {
	var out []DemoFinding
	for _, f := range ordered {
		if f.SuggestedFix != "" {   // WRONG signal — should be "has a real Fix"
			out = append(out, f)
		}
	}
	return out
}
```

But `runDoctorAndConvert` (`cmd/gitid/wiring.go:4051-4064`) copies
`SuggestedFix: f.SuggestedFix` unconditionally, regardless of whether
`f.Fix` is nil. Multiple checks introduced in this very phase set report-only
findings (`Fix: nil`) with **non-empty** `SuggestedFix` text — several of them
literally say "not offered as a fix" in that same text:

- `internal/doctor/checks/coherence.go:465-500` `checkShadowedGlobalOptions` — `Fix: nil`, `SuggestedFix: "… -- advisory only; not offered as a fix."`
- `internal/doctor/checks/coherence.go:526-550` `checkDirectiveAboveManagedBlock` — `Fix: nil`, `SuggestedFix: "user content above a managed block is left untouched by design -- not offered as a fix."`
- `internal/doctor/checks/coherence.go:565-597` `checkAuthorResolution` — `Fix: nil`, `SuggestedFix: "repair via the Global Git screen or re-run 'gitid identity add' -- not offered as a fix."`
- `internal/doctor/checks/orphans.go:64-73` (Class 1 SSH-only block) — `Fix: nil`, `SuggestedFix: "Informational only, no action offered."`
- `internal/doctor/checks/orphans.go:152-163` (Class 3 unused key) — `Fix: nil`, `SuggestedFix: "inspect usage manually; delete with 'rm %s' if confirmed unused"`
- `internal/doctor/checks/signing.go:91-102,119-133` `CheckAgent` — `Fix: nil` in both branches, non-empty `SuggestedFix`
- `internal/doctor/checks/signing.go:172-188` `CheckSigning` — `Fix: nil`, `SuggestedFix: "upgrade git (required: >= 2.36) …"`
- `internal/doctor/checks/deps.go:57-66,75-85` `CheckDeps` — `Fix: nil` for every missing-tool/clipboard finding, non-empty `SuggestedFix`
- `internal/doctor/checks/coherence.go:82-97,100-123,190-209` (KeyPath/fragment existence, gpg.format mismatch) — all `Fix: nil`, non-empty `SuggestedFix`

Because of this, essentially every check family this phase adds is exposed on
the Fixer tab as "fixable." Walking through the ceremony for one of these:

1. `fixerModel.handleKey`'s `f`/`F` path builds `m.ceremony = fixCeremonyFor(m.backend, sel)`.
2. `fixCeremonyFor` (`internal/tuikit/identities.go:2494-2506`) always sets `Backups: []string{NewBackupPath(plan.File)}` — a **fabricated** timestamp-based path (`internal/tuikit/store.go:533-536`), not a real backup that was ever taken.
3. `FixPlanFor` (`cmd/gitid/wiring.go:1297-1321`) falls back to `tuikit.PlanFor(finding)` for any finding without a `Rewrite` descriptor; its `default` case (`internal/tuikit/fixplans.go:60-65`) returns `Result: "Fix applied."` for **any** unrecognized finding ID — including every one of the report-only findings above.
4. On confirm, the ceremony sets `done = true` optimistically and renders the receipt: `"✓ Fix applied."`, `"Wrote → ~/.ssh/config"`, `"Backed up → ~/.ssh/config.backup.<fabricated timestamp>"`.
5. `persistFixFinding` (`cmd/gitid/wiring.go:4078-4106`) locates the raw finding, finds `target.Fix == nil`, and does **nothing** — `b.setPersistErr(nil); return b.stateWithFreshFindings(converted)`. No error is ever surfaced.

The user is shown a complete, convincing success receipt (message, "Wrote →",
"Backed up →" with a plausible path) for a write that never happened and a
backup that was never taken. For a tool whose entire purpose is disciplined,
auditable config mutation with confirm+backup guarantees (see this
repo's own `CLAUDE.md`), this is a serious correctness/trust defect: a user
who "fixes" e.g. a `gpg.format != ssh` finding or a missing `allowed_signers`
entry via the Fixer tab will believe the problem is resolved when their
config is unchanged.

This gap is corroborated by the phase's own unit test fixture:
`internal/tuikit/fixer_screen_test.go`'s `wave2to5FixableFindings()` hand-
crafts `ssh-shadowed-option`, `git-author-resolution`, and
`ssh-directive-above-block` with **empty** `SuggestedFix` so that
`TestFixerCompleteFixableSet` proves they're excluded — but the *real* checks
that produce these exact findings (`checkShadowedGlobalOptions`,
`checkAuthorResolution`, `checkDirectiveAboveManagedBlock`, cited above) set
non-empty `SuggestedFix` text. The test fixture does not match production
data, so it gives false confidence that report-only Wave 3/5 findings are
correctly excluded from the Fixer when in fact they are not.

**Fix:** Thread the real fixability signal through the pipeline instead of
inferring it from `SuggestedFix` text. Concretely:
- Add `Fixable bool` (or equivalent) to `doctor.Finding`'s conversion output — e.g. add a field to `tuikit.HealthFinding`/`DemoFinding` set from `f.Fix != nil` in `runDoctorAndConvert`:
  ```go
  converted = append(converted, tuikit.DemoFinding{
      HealthFinding: tuikit.HealthFinding{
          ...
          SuggestedFix: f.SuggestedFix,
          Fixable:      f.Fix != nil,
      },
      ...
  })
  ```
- Change `fixableFindings` (`internal/tuikit/doctor.go`) and `health_screen.go`'s `fixNote`/hint logic to key off `Fixable`, not `SuggestedFix != ""` — mirroring `cmd/gitid/fix.go`'s already-correct `f.Fix != nil` gate.
- Update `internal/tuikit/fixer_screen_test.go`'s fixture to match the real checks' `SuggestedFix` text (non-empty) for the report-only findings, so the test exercises the actual production shape instead of a shape engineered to pass.

## Warnings

### WR-01: D-16 batch-halt message is nonsensical for a single (non-batch) fix failure

**File:** `internal/tuikit/app.go:313-331` (`checkFixBatchHalt`), `internal/tuikit/fixer_screen.go:88-101` (`haltBatch`)

**Issue:** `App.checkFixBatchHalt` calls `preFixer.haltBatch(...)` on ANY
`PersistError()` after a `FixFinding` dispatch, regardless of whether the
just-attempted fix was part of an `F` (fix-all) batch walk or a single `f`
fix:

```go
func (a *App) checkFixBatchHalt(prevScreen screenModel) {
	postFixer, ok := a.screens[a.tab].(fixerModel)
	if !ok || postFixer.pendingFixID == "" {
		return
	}
	preFixer, wasFixer := prevScreen.(fixerModel)
	if err := a.backend.PersistError(); err != nil {
		if wasFixer {
			a.screens[a.tab] = preFixer.haltBatch(postFixer.pendingFixName, err.Error())
		}
		return
	}
	...
}
```

`haltBatch` computes `total := 0; if m.batch != nil { total = m.batch.total }`
and `n := len(m.batchSucceeded) + 1`. For a single, non-batch `f` fix,
`preFixer.batch` is nil, so `total == 0` and `n == 1`, producing:

> "Fix 1 of 0 failed and was rolled back from its own backup -- the first 0
> fixes already applied stand. Nothing else in this batch was attempted."

This is shown for a fix that was never part of a batch — it mentions "this
batch" and "0 of 0" nonsensically. This path is reachable in production
whenever a real single-fix write fails (e.g. permission denied, disk full,
the `ApplyVerifiedHostDirective` verify-then-restore path itself failing) and
is not covered by any existing test — `fixer_screen_test.go`'s only failure
test drives the `F` batch-walk path (`TestFixerBatchWalk`-style, asserting
"Fix 2 of 3 failed…").

**Fix:** Only route through `haltBatch`'s batch-shaped message when
`preFixer.batch != nil`; add a distinct single-fix failure message (or reuse
`ceremonyModel.commitFailed` alone, without the `batchHalt` banner) when
there was no batch:

```go
if err := a.backend.PersistError(); err != nil {
	if wasFixer {
		if preFixer.batch != nil {
			a.screens[a.tab] = preFixer.haltBatch(postFixer.pendingFixName, err.Error())
		} else {
			a.screens[a.tab] = preFixer.singleFixFailed(err.Error())
		}
	}
	return
}
```

### WR-02: The D-09 surgical rewrite has no internal guard against operating on a gitid-managed block

**File:** `internal/sshconfig/rewrite.go:59-91` (`RewriteHostDirective`), `:110-145` (`ApplyVerifiedHostDirective`)

**Issue:** This project's own `CLAUDE.md` calls this exact code path "a
narrow, explicit carve-out" that "does not relax the managed-blocks-only rule
above for any other write path," and the file's own header comment calls it
"the phase's single highest-risk write affordance." Today the invariant
("only ever rewrites a hand-written, non-gitid-managed stanza") is enforced
entirely by the one caller that builds a `Fix` from it —
`checkHandWrittenIdentitiesOnly` (`internal/doctor/checks/coherence.go:404-
443`), which filters `deps.AllHostBlocks` on `hb.ManagedBlockName == ""`
before ever constructing the `Fix.Fn` closure. `RewriteHostDirective` /
`ApplyVerifiedHostDirective` / `locateDirectiveLine` themselves never check
whether the located stanza sits inside a `# BEGIN gitid managed: …` /
`# END gitid managed: …` sentinel pair (the same check `managedBlockLineMap`
in `internal/sshconfig/reader.go:174-197` already knows how to perform).

For code this sensitive and this explicitly called out as the one exception
to a hard architectural rule, the primitive itself should refuse to rewrite a
stanza that lives inside a managed block, rather than relying solely on
caller discipline — a future caller (a new check, a CLI flag, a refactor of
`checkHandWrittenIdentitiesOnly`) could pass in a pattern that resolves
inside a managed block and the function would happily perform the surgical
rewrite there, silently defeating the sentinel-delimited-block invariant that
governs every other write path in this codebase.

**Fix:** Add a self-check inside `RewriteHostDirective` (or
`locateDirectiveLine`) that rejects a match whose line falls inside a
`managedBlockLineMap` entry, returning a descriptive error — the same defense
`ParseAllHostBlocks` already computes for `HostBlockFacts.ManagedBlockName`.

### WR-03: `deps.Stat` is nil-guarded inconsistently across `internal/doctor/checks`

**File:** `internal/doctor/checks/coherence.go:84,109`, `internal/doctor/checks/orphans.go:139`, `internal/doctor/checks/perms.go:65,112`

**Issue:** `internal/doctor/checks/files.go`'s `checkGitConfig`, `baseline.go`'s
`excludesFileExists`, and `signing.go`'s `CheckAgent` all guard with
`if deps.Stat != nil { … }` (or `if d.Stat == nil { return false }`) before
calling the injected seam. `coherenceForAccount` (KeyPath/FragmentPath
existence checks), `CheckOrphans` (Class 3 unused-key check), and
`checkPath`/`checkGitconfigPath` in `perms.go` call `deps.Stat(path)`
directly with no nil check:

```go
// internal/doctor/checks/coherence.go:84
_, err := deps.Stat(acct.KeyPath) //nolint:gosec // ...
```

In production `cmd/gitid/wiring.go` always wires a non-nil `Stat`, so this is
not exploitable today, but it is a real inconsistency against a pattern this
same package already establishes for exactly this seam — a future test or a
new check added without wiring `Stat` will panic instead of degrading
gracefully the way the rest of the package does.

**Fix:** Add the same `if deps.Stat == nil { … }` guard to `coherenceForAccount`,
`CheckOrphans`, and `perms.go`'s `checkPath`/`checkGitconfigPath` for
consistency with `files.go`/`baseline.go`/`signing.go`.

### WR-04: `CheckAgent`'s external process calls have no timeout, unlike this phase's own established pattern for the same risk class

**File:** `cmd/gitid/wiring.go:3901-3926` (`runDoctorSSHAdd`, `runDoctorSSHKeygenFingerprint`)

**Issue:** `internal/sshconfig/rewrite.go`'s `sshGResolves` explicitly documents
and defends against "T-06-02 class: a pathological config must never block
gitid indefinitely" using `context.WithTimeout`, `Setpgid: true` +
`syscall.Kill(-pid, SIGKILL)` on cancel, and `WaitDelay`. `runDoctorSSHAdd`
(`ssh-add -l`) and `runDoctorSSHKeygenFingerprint` (`ssh-keygen -lf <path>`),
which `CheckAgent` depends on for every Health/Fixer scan and every
`gitid health`/`gitid fix` CLI invocation, use plain `exec.Command` with no
context, no timeout, and no bounded kill:

```go
func runDoctorSSHAdd() (string, int) {
	cmd := exec.Command("ssh-add", "-l")
	out, err := cmd.CombinedOutput()
	...
}
```

A hung or unresponsive `ssh-agent` (e.g. agent-forwarded to an unreachable
remote — a documented real-world SSH failure mode) blocks this call
indefinitely, hanging the doctor scan (and therefore the TUI and the CLI)
with no bound, exactly the class of risk this same phase already treats as
worth defending against elsewhere.

**Fix:** Apply the same bounded-timeout, process-group-kill pattern
`sshGResolves` already uses (or factor it into a shared helper) for
`runDoctorSSHAdd` and `runDoctorSSHKeygenFingerprint`.

## Info

### IN-01: Dead-in-production duplicate filtering helpers in `cmd/gitid/health.go`

**File:** `cmd/gitid/health.go:116-124` (`findingsForIdentity`), `:145-160` (`suppressParseErrorFindings`)

**Issue:** `runHealth` (the only production caller path) uses
`findingsForIdentityPairs` and `suppressParseErrorFindingPairs` exclusively.
The non-"Pairs" `findingsForIdentity` and `suppressParseErrorFindings`
functions are never called from any command path — they exist solely to be
exercised directly by `health_test.go`. This is duplicated logic (two
implementations of the same filtering idea) that must be kept in sync by
hand with no functional benefit, since only the `Pairs` variants are
reachable in production.

**Fix:** Delete the unused variants, or have `health_test.go` exercise the
`Pairs` versions (with a raw-findings fixture) instead of maintaining a
second, unused implementation just to keep test coverage on it.

### IN-02: `rewriteDirectiveLineValue` treats `#` in a new value as a comment start

**File:** `internal/sshconfig/rewrite.go:301-342`

**Issue:** `validateRewriteValue` only rejects `\n`, `\r`, and `\x00` in the
new directive value; it does not reject `#`. `rewriteDirectiveLineValue`
scans the line's tail for the first `#` to preserve a trailing comment, so a
`newValue` containing `#` would be silently truncated at that character when
the line is rebuilt. Not reachable today — the only real caller passes the
literal `"yes"` — but this is a general-purpose primitive (also used by
`DiffHostDirective`) whose contract quietly assumes no legitimate SSH
directive value ever contains `#`, which is only true for the one directive
it currently rewrites.

**Fix:** Either reject `#` in `validateRewriteValue` (matching the existing
control-byte discipline) or quote/escape it appropriately, so a future caller
of this primitive with a different directive/value cannot silently corrupt
the rewritten line.

### IN-03: Stale doc comment on `doctor.Run`

**File:** `internal/doctor/doctor.go:322-324`

**Issue:** The comment "Order: Dependencies, Permissions, Coherence, Orphans,
Signing, Agent, Baseline, Overlap, Redundancy." omits `Files`, even though the
function's own dispatch loop calls `deps.CheckFiles` last and `Families()`
(same file, `:441-454`) correctly lists `FamilyFiles` at the end. No
behavioral impact — purely a stale comment from before `CheckFiles` was
added.

**Fix:** Append `, Files` to the doc comment's order list.

---

_Reviewed: 2026-08-28T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
