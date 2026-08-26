---
phase: 06-global-ssh-options
plan: 03
subsystem: ui
tags: [globalssh, provenance, version-gate, copy-freeze]
requires:
  - phase: 06-global-ssh-options
    provides: [globalssh engine, D-10 policy table, Host * write ceremony, tuikit GlobalSSHPlanner seam (06-01); reserved-name registry (06-02)]
provides:
  - "Four-state classifier (needs-action / already-set / differs / not-applicable) keyed on value first, source class second"
  - "Per-source probe-error isolation so a resolution failure does not degrade UseKeychain or IdentitiesOnly"
  - "IdentitiesOnly per-alias conformance (verify-only, never a Host * write; empty inventory is nothing-to-verify)"
  - "D-13 VersionGate with VersionUnverified withholding accept-new until ssh -V is readable"
  - "Fixture-versus-policy parity plus four visual states, four not-applicable sentences, and truthful attribution"
affects: [06-04 apply ceremony N-of-M, 06-06 CLI verb, 06-07 visual gate]
actuals:
  tokens: 5783
  tasks: 3
  commits: 4
tech-stack:
  added: []
  patterns: [value-before-source classification, dual-enum numeric pin across the backend boundary, Selectable as the one toggle predicate]
key-files:
  created:
    - internal/globalssh/peralias.go
    - internal/globalssh/peralias_test.go
    - internal/globalssh/version.go
    - internal/globalssh/version_test.go
    - internal/tuikit/design_test.go
  modified:
    - internal/globalssh/classify.go
    - internal/globalssh/classify_test.go
    - internal/globalssh/policy.go
    - internal/globalssh/policy_test.go
    - internal/tuikit/design.go
    - internal/tuikit/globalssh.go
    - internal/tuikit/globalssh_test.go
    - internal/tuikit/views.go
    - internal/dummytui/fixturebackend.go
    - cmd/gitid/wiring.go
    - cmd/gitid/wiring_test.go
    - cmd/gitid/lifecycle.go
    - Makefile
key-decisions:
  - "State is decided by VALUE equality before any source-class branch, so ForwardAgent no (OpenSSH default) is already-set, not a false alarm."
  - "Attribution is a separate boolean (AttributedToUser), never a fifth visual state."
  - "VersionUnverified withholds the write; the row still explains itself."
  - "Status tally = needs-action + set-but-differs; not-applicable is excluded. Pinned on needsAttention so 06-04 cannot re-derive it."
patterns-established:
  - "Pattern: tuikit mirrors backend enums by VALUE ONLY; cmd/gitid pins the numeric pairing so a silent renumbering cannot drift the copy."
  - "Pattern: one Selectable() predicate owns the toggle key, the checkbox click, and the checkbox glyph."
requirements-completed: [GSSH-01]
coverage:
  - id: D1
    description: "Four-state classifier with value-before-source branch order, UseKeychain file-parse-only, IdentitiesOnly per-alias, and per-source probe errors"
    requirement: GSSH-01
    verification:
      - kind: unit
        ref: internal/globalssh/classify_test.go#TestStateFor / TestPerAlias / TestProbeError / TestStatusesConcurrentLatency
        status: pass
    human_judgment: false
  - id: D2
    description: "D-10 fixture parity, numeric NotApplicableReason pin, and D-13 VersionGate with unverified write refusal"
    requirement: GSSH-01
    verification:
      - kind: unit
        ref: cmd/gitid/wiring_test.go#TestGlobalSSHFixturePolicyParity / TestGlobalSSHOptionStatesVersionUnverified
        status: pass
      - kind: other
        ref: make gate-copy-freeze (D-13 exclusion)
        status: pass
    human_judgment: false
  - id: D3
    description: "Four visual states, four distinct not-applicable sentences, truthful attribution, IdentitiesOnly not selectable"
    requirement: GSSH-01
    verification:
      - kind: unit
        ref: internal/tuikit/globalssh_test.go#TestOptionRow* / TestGlobalSSHSelectablePredicate / TestGlobalSSHToggleAndClickRespectSelectability
        status: pass
      - kind: automated_ui
        ref: internal/tuikit/globalssh_test.go#TestGlobalSSHNoColorStatesDistinguishable / TestGlobalSSHGeometryWithEveryStateAndBanner
        status: pass
    human_judgment: false
duration: 90min
completed: 2026-08-26
status: complete
---

# Phase 06-03: Six Honest Global SSH Option Rows Summary

**All six D-10 options now render a real value, a real provenance, and exactly one of four visual states — with attribution that never credits a value gitid did not parse, and with accept-new withheld until OpenSSH is identified.**

## Performance
- **Duration:** 90min
- **Tasks:** 3
- **Files modified:** 21 unique (5 created, 16 modified across the three implementation commits)

## Accomplishments
- `stateFor` classifies by value first: a recommendation that already holds — including OpenSSH's own `ForwardAgent no` — is already-set, whatever its source.
- The dummy fixture and the live policy agree field-for-field, pinned so they cannot drift again.
- The Options sub-tab renders four states with existing glyphs and existing theme roles; IdentitiesOnly is verify-only; an unreadable `ssh -V` withholds the accept-new write.

## Task Commits
1. **Task 1: Four-state classifier, UseKeychain and IdentitiesOnly special cases, per-source probe errors** - `797f2f4` (feat)
2. **Task 2: D-10 fixture correction, VersionGate, copy-freeze exclusion** - `566df27` (feat)
3. **Task 3: Four visual states, four not-applicable sentences, truthful attribution** - `a575c4a` (feat)

**Plan metadata:** `[this commit]` (docs: add plan summary)

## Files Created/Modified
- `internal/globalssh/classify.go` — four-state `stateFor` (platform → empty → value-equals-recommended → baseline-unequal → differs), concurrent probes, per-source ProbeError
- `internal/globalssh/peralias.go` — IdentitiesOnly conformance (`total==0` → `ReasonNothingToVerify`; any offender → needs-action)
- `internal/globalssh/version.go` — `VersionGate` with numeric component compare; empty version → `VersionUnverified`
- `internal/tuikit/design.go` — StrictHostKeyChecking `accept-new` + rewritten one-liner; ForwardAgent Risk `High`; frozen D-12/D-11/D-13 sentences
- `internal/tuikit/views.go` — `GlobalSSHNotApplicableReason`, `Selectable()`, attribution/writable fields
- `internal/tuikit/globalssh.go` — `optionRow` takes the view; `needsAttention` is the pinned tally; VersionNote in the detail pane
- `cmd/gitid/wiring.go` — one `ssh -V` probe per read; version outcomes mapped onto not-applicable; D-03 provenance labels
- `cmd/gitid/lifecycle.go` — `runGlobalSSHApply` refuses unverified/too-old `accept-new` with an error naming `ssh -V`
- `Makefile` — freeze the new sentences; D-13 negative assertion assembled at runtime so the check line cannot trip itself

## Authoritative resolutions of 06-UI-SPEC.md unresolved items

These are this plan's resolutions of the UI Considerations table rows marked unresolved.

### State model (partial / D-12 three-or-four-state row)
Go type: `tuikit.GlobalSSHOptionState` (`int`, iota) with four values — `GlobalSSHNeedsAction`, `GlobalSSHAlreadySet`, `GlobalSSHDiffers`, `GlobalSSHNotApplicable`. Attribution is **not** a fifth state: `AttributedToUser bool` is the render-layer projection of `SourceGitidParsed`. Not-applicability is **not** collapsed into one sentence: `GlobalSSHNotApplicableReason` mirrors `globalssh.NotApplicableReason` by value, pinned numeric-for-numeric in `TestGlobalSSHFixturePolicyParity`.

### Probe-failure render (error / `ssh -G` failure)
No fifth visual state and no doctor finding. A probe error degrades only the options that depended on that source (resolution failure → StrictHostKeyChecking/ForwardAgent/HashKnownHosts/AddKeysToAgent; config-read failure → UseKeychain/IdentitiesOnly). The row stays in its classifier state, is **not selectable**, and the detail pane shows the probe-error note. Advisory posture extends to detection: the pane still explains the option.

### Not-applicable render per reason (n/a / D-11)
The row stays visible (frozen 6-row contract). Line 2 replaces the `now: … → …` text with the reason sentence and omits the arrow and both checkbox glyphs:

| Reason | Sentence |
|---|---|
| Platform | `not applicable (macOS-only setting)` |
| Version too old | `not applicable (OpenSSH too old for accept-new)` |
| Version unverified | `not applicable (OpenSSH version could not be verified)` |
| Nothing to verify | `not applicable (nothing on this machine to verify)` |

The platform sentence is asserted never to appear on a version-gated row.

### Concurrent probe latency (loading)
Measured under `-race` with each of the three probes sleeping 90ms (budget 270ms):

- typical: 90.3–91.3ms
- worst of five runs: 91.3ms

That is one probe duration, not three, so the concurrent bound holds with a full budget of slack. **No loading affordance.** The Options sub-tab continues to render after `activate()` returns, the same pattern as every other tab. A spinner would be a new UI element this plan is forbidden to introduce.

### Pinned tally rule (Claude's discretion in 06-CONTEXT.md)
`needsAttention` counts `GlobalSSHNeedsAction` **and** `GlobalSSHDiffers` together and skips `GlobalSSHNotApplicable`. The status line reads `"%d of %d options need action"` from that count. Comment on the predicate names 06-CONTEXT.md so plan 06-04's ceremony N-of-M must reuse it rather than re-deriving one.

## VersionUnverified observation (evidence for the D-13 refusal)

On this machine `ssh -V` reports `OpenSSH_9.9p2, LibreSSL 3.3.6` — readable, well above 7.6. **Observed unreadable-version frequency during this plan: 0.** The refusal path was exercised only by injecting an empty `platform.SSHVersion` in tests. Keep the conservative withholding: reversing it is one `VersionGate` branch, and this session produced no counter-evidence that real machines are blocked.

## Decisions Made
- Value equality runs before any source-class test, so a safe OpenSSH default is already-set and worded `safe by default`, never as something the user configured.
- `Selectable()` is the single definition of "can be chosen": needs-action or differs, writable to Host *, no probe error. IdentitiesOnly fails `WritableToHostStar()` even when it needs action.
- Provenance labels match D-03's frozen shapes: `set by you at … line N` / `set in … — gitid cannot change this` / `not set (OpenSSH default: X)` / `set outside your config`.

## Deviations from Plan

### Auto-fixed Issues

**1. `cmd/gitid/lifecycle.go` is the write authority that must refuse unverified accept-new**
- **Found during:** Task 2
- **Issue:** The plan's `files_modified` omitted `lifecycle.go`, but an acceptance criterion requires `runGlobalSSHApply` to refuse a `VersionUnverified` key rather than writing.
- **Fix:** After the per-alias guard, a policy with `MinOpenSSH` runs `VersionGate`; anything other than `VersionAvailable` returns an error naming `ssh -V`.
- **Files modified:** `cmd/gitid/lifecycle.go`
- **Verification:** `TestGlobalSSHOptionStatesVersionUnverified`
- **Committed in:** `566df27`

**2. Task 2 needed the reason enum and VersionNote render to satisfy its own ACs**
- **Found during:** Task 2
- **Issue:** The parity pin and the "VersionNote appears in StrictHostKeyChecking's detail pane" criterion live in Task 2, but `GlobalSSHNotApplicableReason` and the detail-pane line were listed under Task 3.
- **Fix:** Landed the enum, the VersionNote line, and the render test in the Task 2 commit so the ACs could pass without a half-wired row.
- **Files modified:** `internal/tuikit/views.go`, `internal/tuikit/globalssh.go`, `internal/tuikit/globalssh_test.go`
- **Verification:** Task 2 `go test -run 'Fixture|Parity|Version|GlobalSSHOption'`
- **Committed in:** `566df27`

**3. `optionRow` signature change and IdentitiesOnly selectability forced test/fixture follow-ons**
- **Found during:** Task 3
- **Issue:** `optionRow` now takes the view; IdentitiesOnly is not selectable, so the fixture apply ceremony is 2 of 3 writable pending keys (StrictHostKeyChecking + HashKnownHosts), not 3 of 4 including IdentitiesOnly.
- **Fix:** Updated `batch3_test.go` (`TestOptionRowNowValueClipsWithEllipsis`), the stub/dummy fixture projections (`WritableToHostStar: key != "IdentitiesOnly"`), and the apply-ceremony assertions.
- **Files modified:** `internal/tuikit/batch3_test.go`, `internal/tuikit/backend_stub_test.go`, `internal/dummytui/fixturebackend.go`, `internal/tuikit/globalssh_test.go`
- **Verification:** Task 3 test filter + `TestNoBackendAllowlist`
- **Committed in:** `a575c4a`

**4. D-13 exclusion check must not match its own grep pattern**
- **Found during:** Task 2
- **Issue:** `grep -qF -- 'Your OpenSSH:' Makefile` matches the check line itself.
- **Fix:** Assemble the prefix at runtime (`dyn_prefix="Your OpenSSH"; dyn_prefix="$dyn_prefix:"`). Verified by adding the prefix to the frozen list (gate failed), then reverting.
- **Files modified:** `Makefile`
- **Committed in:** `566df27`

**5. revive `redefines-builtin-id: min`**
- **Found during:** Task 2 commit hook
- **Issue:** `versionLess(got, min string)` tripped revive.
- **Fix:** Renamed to `minimum`.
- **Files modified:** `internal/globalssh/version.go`
- **Committed in:** `566df27`

---

**Total deviations:** 5 auto-fixed (all required for the stated ACs or the lint hook).
**Impact on plan:** No scope creep. Extra files are the write-refusal site, the reason enum the parity test pins, and the tests the new `optionRow` signature broke.

## Issues Encountered
- First `gate-copy-freeze` D-13 check failed because it grepped its own pattern; assembling the prefix at runtime is the durable form.
- Colour-disabled distinguishability at 100×44 master-list width clips the full differs sentence; the no-color test asserts the surviving word (`differs`) rather than the whole frozen string. The unclipped `optionRow` tests still pin the full sentences at width 80/100.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
06-04 can reuse `needsAttention` for the ceremony N-of-M and `runGlobalSSHApply` as the one write site (already refuses unverified accept-new). D-04 shadowing simulation is still empty on `ShadowWarnings` / `ShadowAdvisories`, as this plan left it.

---
*Phase: 06-global-ssh-options*
*Completed: 2026-08-26*
