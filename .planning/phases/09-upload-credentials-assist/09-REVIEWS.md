---
phase: 9
reviewers: [xai-grok, codex, codex-sol]
reviewed_at: 2026-08-28T20:20:53Z
plans_reviewed: [09-01-PLAN.md, 09-02-PLAN.md, 09-03-PLAN.md, 09-04-PLAN.md, 09-05-PLAN.md, 09-06-PLAN.md, 09-07-PLAN.md, 09-08-PLAN.md]
models:
  xai-grok: "xai/grok-4.6 (reasoning=low)"
  codex: "gpt-5.6-sol (reasoning=low)"
  codex-sol: "openai/gpt-5.6-sol-fast (reasoning=low)"
model_sources:
  xai-grok: "pinned"
  codex: "banner"
  codex-sol: "pinned"
---

# Cross-AI Plan Review — Phase 9

> **Note on reviewer roster:** `review.default_reviewers` configures `codex-sol` (model
> `openai/gpt-5.6-sol-fast`) and `xai-grok` (opencode, model `xai/grok-4.6`). `codex-sol`
> failed at invocation — the pinned model is rejected by this Codex CLI's ChatGPT-account
> authentication (`"The 'openai/gpt-5.6-sol-fast' model is not supported when using Codex
> with a ChatGPT account."`, confirmed reproducible independent of the `openai/` prefix).
> Its section below is the diagnostic stub, not a review. To still deliver a genuine
> cross-AI review, the plain `codex` lane was invoked with its default banner-resolved
> model (`gpt-5.6-sol`) as a substitute for the broken instance; both `xai-grok` and
> `codex` completed full source-grounded reviews and are weighted at full consensus below.
> Recommend fixing `review.reviewer_instances.codex-sol.model` (drop `-fast`, and possibly
> the `openai/` prefix) for future runs.

## Consensus Summary

Both reviewers independently traced the actual e2e harness code (`e2e/harness_test.go`,
`e2e/create_flow_pty_e2e_test.go`, `internal/uploader/uploader.go`, `Makefile`,
`.planning/ONESHOT.md:128-141`) rather than reviewing plan text in isolation, and both
converged on the same root defect from different angles.

**Both reviewers agree the plan set is well-architected** — the wave ordering (contract →
tracer → engine → wizard → CLI → manager → UI gates → real-account validation), the
UI/backend separation, the deletion-safety defaults, and 09-08's overall protocol shape
(disposable key, prefixed titles, ID-scoped deletion, final sweep) all match the codebase's
existing patterns and ONESHOT.md's policy intent.

**Both reviewers also agree on the phase's central blocking risk: waves 2–7 do not
hermetically prevent unattended e2e/PTY tests from resolving the developer's real,
authenticated `gh` binary.** `newRealCreateFlowCmd` (`e2e/create_flow_pty_e2e_test.go:59-64`)
builds its test-process `PATH` as `<fakeSSHDir>:<original PATH>` — the real system `PATH`,
and therefore any real `gh`/`glab` on it, remains resolvable. 09-02's plan only prepends a
fake-`gh` directory in the *new* tracer test; existing PTY tests such as
`TestCreateFlow_TestStagePass` are not required to add the shim. Once 09-02 wires the real
`buildUploaderDeps` (`exec.LookPath` + `exec.Command`) into the create flow's autonomous
upload trigger, pressing Enter in any of those *existing, unmodified* tests can reach a real
GitHub/GitLab account during a routine, unattended `make test-e2e` run — exactly the failure
mode ONESHOT.md's External Account Policy confines to the opt-in 09-08 test. The same gap
recurs in 09-05 (CLI e2e) and 09-07 (new PTY gates), since neither requires the pre-existing
suite to adopt the shim.

### Agreed Strengths
- 09-08's protocol closely tracks ONESHOT.md's verbatim policy: blocking human checkpoint,
  pre-flight `gh auth status --hostname github.com` with skip-not-fail on missing
  auth/scope, disposable key generated via the project's own keygen into `t.TempDir()` (never
  `~/.ssh`), `gitid-e2e:<run-id>:<purpose>` title prefix, deletion strictly by recorded ID
  after a prefix re-confirm, a final prefix sweep, GitLab left untouched, and a dedicated
  build tag/Make target kept out of `test`/`test-e2e`/`lint` (modeled on the existing
  `smoke-network-test` precedent).
- A blocked/unauthenticated run records honest SKIP evidence rather than substituting a mock
  — both reviewers confirm this matches ONESHOT.md's explicit prohibition.
- Deletion-safety defaults across the phase (default-to-no, exact machine-title match,
  delete by confirmed ID without re-resolving) are consistently fail-closed.
- Argument-slice (never shell-string) subprocess construction is maintained throughout.

### Agreed Concerns

- **HIGH — Ambient-PATH leak lets waves 2–7's pre-existing/unmodified e2e tests reach a
  real, authenticated `gh`/`glab` once autonomous upload is wired (09-02 onward).**
  `e2e/create_flow_pty_e2e_test.go:59-64`'s `newRealCreateFlowCmd` appends the fake-SSH dir
  to the *original* `PATH` rather than replacing it; 09-02/09-05/09-07 only add shims to
  their own new tests, not to the existing suite that inherits the new upload trigger. This
  is the phase's dominant safety risk and both reviewers rate it HIGH / overall-phase risk
  HIGH until fixed. **Fix (both reviewers converge on this):** in 09-02, make the e2e harness
  construct a `PATH` containing only shim/isolated directories (fake `gh` + fake `glab` +
  fake `ssh`, or a `LookPath`-blocking wrapper) for every upload-capable PTY/CLI process,
  suite-wide — not opt-in per new test — plus a negative test asserting no real-CLI exec
  occurs when the shims are absent.
- **MEDIUM (codex: HIGH) — `UploadKeys`'s single `title string` parameter cannot satisfy
  09-08's per-purpose title requirement.** ONESHOT.md requires
  `gitid-e2e:<run-id>:<purpose>` with a distinct `<purpose>` per registration kind
  (authentication vs. signing), but 09-03's planned engine signature
  (`09-03-PLAN.md:140-146`) takes one title for the whole batch. codex rates this a hard
  blocker against 09-08's stated protocol; xai-grok flags the same mismatch as a MEDIUM
  policy-compliance gap and calls out that D-07's product-mode `KeyTitle` must not silently
  override the policy-mode custom prefix. Either way, 09-03's API needs a per-registration
  title before 09-08 can execute as specified.
- **MEDIUM — Resource-ID capture is by post-hoc inventory/title lookup, not IDs literally
  "returned" by the upload call.** `gh ssh-key add` does not emit a machine-readable ID in
  its stdout; both reviewers independently traced this to the same conclusion and agree the
  plan's mechanism (inventory lookup scoped to the exact run-prefix, then delete-by-ID) is
  an acceptable-but-inexact reading of ONESHOT.md's "record the exact returned resource ID"
  — recommend making the checkpoint/summary language describe the real mechanism rather than
  implying a direct returned-ID capture.

### Divergent Views

- **codex raises two additional HIGH findings xai-grok does not surface at HIGH:**
  1. **09-08 doesn't exercise the compiled autonomous product path.** The plan calls
     `uploader.UploadKeys` directly rather than driving the compiled `gitid` binary through
     `buildUploaderDeps`/CLI/TUI/eligibility/orchestration, so it validates the uploader
     engine in isolation, not the "autonomous upload path... proven once against a real
     GitHub account" claim in the plan's own success criteria. codex recommends running the
     compiled binary under the dedicated tag/target instead.
  2. **Scope verification in 09-08 is underspecified** ("a scope-revealing read" without a
     named command/parsing rule for confirming both `admin:public_key` and
     `admin:ssh_signing_key`). xai-grok raises the identical gap but at MEDIUM, recommending
     `gh api user` + `gh api user/ssh_signing_keys` or parsing `gh auth status -t`'s scope
     line.
  Given one reviewer (codex) treats these as execution-blocking and the other (xai-grok)
  treats the scope gap as a fixable MEDIUM, both should be resolved before 09-08 executes —
  the divergence is in severity, not in whether the gap exists.
- **codex additionally flags `.pub`-suffix-only validation as insufficient for the plan's
  ASVS V5 "public-key-only" invariant** (a private key renamed with a `.pub` suffix would
  pass `requirePubPath`), recommending content-based validation (reject private-key PEM
  markers). xai-grok does not raise this as a distinct finding.
- **codex flags a possible double-delete in 09-08** (`t.Cleanup` registered immediately after
  ID recording, then an explicit delete later in the same test — unless the cleanup closure
  tracks already-removed IDs, cleanup runs twice). xai-grok does not raise this.
- **codex flags that a blocked/skipped real-account run should not be allowed to fully close
  UP-03** — the plan's closure language permits marking UP-03 closed on a blocked run, which
  codex reads as contradicting the plan's own "required evidence" framing. xai-grok does not
  raise this as a distinct concern.
- **xai-grok raises a provider-hostname-matching gap not surfaced by codex as a named
  severity:** `ProviderForHostname`'s substring match (`strings.Contains(host, "github")`)
  would misclassify a host like `github.example.com` as GitHub, contradicting D-13's
  main-domain-only autonomous-behavior rule. (codex independently raises the same
  `ProviderForHostname` substring concern under 09-02 at MEDIUM, so this is really agreement
  with different framing rather than a true divergence — noted here because the two reviews
  discuss it under different headings.)

---

## OpenCode Review (xai-grok)

Verifying plan claims against the repo: ONESHOT policy, uploader/e2e shims, and Makefile gates.
# Phase 9 Cross-AI Plan Review

Verified against the live tree (`internal/uploader/uploader.go`, `e2e/harness_test.go`, `e2e/create_flow_pty_e2e_test.go`, `cmd/gitid/main_test.go`, `Makefile`, `.planning/ONESHOT.md:128-141`). Plans are generally well-grounded in research (archived `copy.go`, `Detect` first-found bug, `placeOverlay` gone, `identityVerb` grammar). The load-bearing risk is **real `gh` on PATH leaking into waves 2–7**, not 09-08's protocol text.

---

### SPECIAL FOCUS

#### 1. 09-08 vs ONESHOT Phase 9 External Account Policy

**Match (strong):** blocking human checkpoint; pre-flight `gh auth status --hostname github.com` + skip (not fail) if unauth/scope missing; disposable key in `t.TempDir()` via project keygen; titles `gitid-e2e:<run-id>:<purpose>`; record IDs; delete only recorded IDs after prefix re-confirm; final prefix sweep; GitLab untouched; distinct `realaccount` tag + `verify-upload-real-account` modeled on `smoke-network-test` (`Makefile:595-605`, `//go:build smoke` in `cmd/gitid/smoke_network_test.go:1-11`); blocked path still commits the test and records evidence without substituting a mock.

**Gaps vs the verbatim policy:**

| Policy clause | Plan | Verdict |
|---|---|---|
| Verify `gh auth status` **and required scopes** before every upload | Task 2 step 1: "scope-revealing read" — unspecified command | **MEDIUM** — pin e.g. `gh api user` + `gh api user/ssh_signing_keys` (or parse `Token scopes:` from `gh auth status -t`) and name the missing scope in `t.Skip` |
| Record **returned** resource IDs | IDs taken from **inventory title match** after add | **MEDIUM** — `gh ssh-key add` stdout is typically `Added SSH key.` (`e2e/harness_test.go:365`), not an ID. Inventory-by-title is the only practical capture; say so and constrain that lookup to the run prefix, not a full-account scan used as a delete selector |
| Cleanup never by broad inventory | Delete by recorded ID after prefix re-confirm; sweep asserts zero prefix leftovers | **OK** if sweep is read-only |
| No mock if blocked | Skip + REQUIREMENTS note | **OK** |

Product titles are `gitid: <name> @ <host>` (`09-03` `KeyTitle`); the e2e path must pass **custom** run-prefixed titles into `UploadKeys`, not `KeyTitle`. Plan 09-08 step 4 says that; keep it as an acceptance criterion so D-07 does not overwrite the policy prefix.

#### 2. Waves 1–7: stub vs real account

| Wave | Guard | Real-account risk |
|---|---|---|
| 09-01 | Design/copy only | None |
| 09-02 unit | Fake `Deps` | None |
| 09-02 PTY | `FakeGHDir` prepended **only in new tests** | **HIGH** — see below |
| 09-03 | Fake `Deps` only | None |
| 09-04 | Fake `Deps`; PTY "tracer still holds" | Inherits 09-02 PATH leak |
| 09-05 CLI e2e | Explicit shims + sandbox HOME | OK **if** PATH prepend is exclusive |
| 09-06 | TUI unit + fake backend | Low |
| 09-07 PTY | Shims "inside sandboxed HOME" | **HIGH** unless PATH **replaces** system PATH |

**HIGH — existing create-flow PTY will hit real `gh` after 09-02.**

`newRealCreateFlowCmd` today:

```59:64:e2e/create_flow_pty_e2e_test.go
func newRealCreateFlowCmd(...) {
	env := append(os.Environ(), "HOME="+home, "TERM=xterm-256color")
	if fakeSSHDir != "" {
		env = append(env, "PATH="+fakeSSHDir+":"+os.Getenv("PATH"))
	}
```

System `PATH` still contains real `gh`. After 09-02, `UploadEligibility` + checked checkbox + Enter on step 1 calls `RunUpload` → `exec.LookPath("gh")` → real account. Existing tests (`TestCreateFlow_TestStagePass`, etc.) are **not** required to prepend `FakeGHDir` or strip `gh`. 09-02 Task 3 only extends the helper for **new** tests.

`FakeGHDir`'s unknown-subcommand branch is `exit 0` (`e2e/harness_test.go:386-388`). Until Task 2 lands, a live `gh api` from a half-wired binary is not shimmed.

**Concrete guard the plans do not require:** for every `e2e` / `make test-e2e` process after 09-02: `PATH=<shim dirs only>` **or** `LookPath` that cannot see the developer's `gh`, plus a negative test that `ReadFakeCLILog` is empty / no network when shims are absent and `gh` is renamed off PATH. Sandboxed `HOME` does **not** isolate `gh` OAuth (`hosts.yml` lives under the **real** user config when the binary on PATH is real `gh`).

09-02 "Done" text ("Running the real binary with an authenticated `gh` … performs a real `gh ssh-key add`") must not be an executor instruction; only 09-08 may do that.

#### 3. "Safe by default" without a mechanism

| Claim | Mechanism in plan? | Evidence |
|---|---|---|
| Waves 1–7 never touch a real account | PATH shim in **some** tests | Incomplete — see PATH prepend |
| 09-08 never in CI | Distinct tag + not a Make prereq | Matches `smoke-network-test` — **OK** if grep/acceptance hold |
| Upload never gates | `UploadRunMsg` on all errors | Planned tests — **OK** |
| `.pub` only | `requirePubPath` + PTY argv ends `.pub` | **OK**; 09-02 tracer still uses `(string, error)` `UploadKey` with convention-only (`uploader.go:131-132`) until 09-03 |
| Dry-run read-only | Zero `ssh-key add` | Inventory/auth reads still run — plan admits this; matrix must say so |
| Delete only confirmed ID | D-04 no re-resolve | **OK** |
| No env opt-out | D-06 | Fine for product; **not** a test isolation mechanism |

---

### 09-01 — design contract + copy freeze

**Summary.** Right first wave: FIELDS/APPROVAL/`u` claim/`design.go`/`gate-copy-freeze` before render. Grounded in `design.go` freeze pattern and WR-13 (`Makefile` comments outside recipes).

**Strengths**
- Glyphs stay in `glyphCheckOff`/`glyphCheckOn` (`identities.go` ~4211), not labels.
- `UploadKeyTitleFmt` excluded from freeze — matches dynamic-prefix pattern.
- Does not resurrect archived `copy.go`.

**Concerns**
- **MEDIUM:** Task 1 says checkbox is a **Tab-ring focus slot**; 09-02 later resolves A1 the same way. 09-01 Notes must not freeze "`u` only" if 09-02 rebases focus.
- **LOW:** `grep -vc '^#' internal/dummytui/doc.go` is a weak verify if `doc.go` is comment-only.

**Risk:** **LOW**

---

### 09-02 — tracer

**Summary.** Correct tracer: `DetectFor`, hostname `AuthCheck`, Backend methods, `testUpload`, nil-guard, PATH shims, one PTY. Matches live `Detect` first-found (`uploader.go:89-105`) and bare `AuthCheck` (`uploader.go:111-112`).

**Strengths**
- Deletes `Detect` instead of leaving a footgun.
- Checkbox at `wizardFocusManualPath + 1` avoids shifting `wizardFocusKeySource` (`identities.go:149-151`).
- `stagedKeyFor` share is the right anti-pattern.
- Shim argv log is the process-boundary half of shown==run.

**Concerns**
- **HIGH:** PATH leak into existing PTY (above). Fix in this wave: default create-flow PATH must not include real `gh`/`glab`.
- **MEDIUM:** `ProviderForHostname` duplicates `upload.Instructions` substring match (`upload.go:31-34`). `github.example.com` would look like GitHub — D-13 wants main-domain only. Prefer suffix/host equality (`github.com`, `ssh.github.com`) not `strings.Contains`.
- **MEDIUM:** Tracer uploads **authentication only**; autonomy "done" claim overstates vs UP-01 two registrations (deferred to 09-04 — OK if wording stays "one type").
- **LOW:** Fixture `Ready` for any hostname containing `github` will diverge from live eligibility; 09-07 must classify it.

**Risk:** **HIGH** until PATH isolation is a must-have for **all** e2e, not only new tests.

---

### 09-03 — engine

**Summary.** Right package-local expansion: D-16/D-12/D-15/D-14/V5. `GLabKeyTypeForAuth = "auth"` (`uploader.go:80`) is a real defect to close.

**Strengths**
- `UploadKeys` continue-on-failure at the engine.
- `TitleMatchesThisMachine` exact match (D-07).
- `TestNoSecondSSHKeyAddArgvBuilder`.
- Inventory errors → nil slice + error (cannot look like empty account).
- Classifiers return kinds, not copy.

**Concerns**
- **MEDIUM:** `DeleteKey` ID from provider JSON into argv — arg slice is fine; still validate ID is numeric/`[0-9]+` so a title cannot be passed as ID.
- **LOW:** Confirm `gh ssh-key delete --yes` / `glab -y` on the machine as planned; flags drift.

**Risk:** **LOW–MEDIUM**

---

### 09-04 — wizard section complete

**Summary.** Delivers ROADMAP criterion 3 and UP-01 fallback; D-17 confirmation without a new `tester` probe is correct (`internal/tester` Outcome at `tester.go`).

**Strengths**
- One `toggleUploadCheckbox` for four affordances.
- `AlreadyComplete` / inventory degrade / never-error `UploadRunMsg`.
- `TestNoPersistedUploadState` for D-18.

**Concerns**
- **MEDIUM:** D-17 "unconfirmed" as uploaded + Reason — easy to miss in CLI printer (09-05).
- **LOW:** Worst-case row budget vs 5-line GitHub `Instructions` (`upload.go:36-41`) — overflow viewport is necessary, not optional.

**Risk:** **MEDIUM** (control-flow + frame), not account-safety if 09-02 PATH is fixed.

---

### 09-05 — CLI

**Summary.** `register-key` vs archived `copy` is the right reading of `TestNewRootCmdArchivedPOCCommandsAreGone` (`main_test.go:53-56`). Shared `runUploadFor` is the only way TUI/CLI stay aligned.

**Strengths**
- Does not edit the archived-command test.
- `--no-upload` help as one constant on four verbs.
- Exit code isolated from upload (D-03).
- Headless e2e against shims.

**Concerns**
- **MEDIUM:** `register` alias vs `{"add"}` archived — `register-key`/`register` is fine; do not add root `copy`.
- **LOW:** Dry-run still running inventory/auth — document as reads.

**Risk:** **LOW** if CLI e2e PATH is shim-only.

---

### 09-06 — manager + D-04

**Summary.** `identPane` + `actionMenuRows = 4` (`identities.go:58`, `% actionMenuRows` at 2314/2318) is the real desync. Lazy inventory for delete ID matches D-18.

**Strengths**
- Default-leave + Enter-from-default must not delete.
- Exact title match vs other machines.
- Commit does not re-resolve ID.
- Repair omits offer.

**Concerns**
- **HIGH (product, not CI):** D-04 is remotely destructive; interaction tests are the guard — keep them.
- **MEDIUM:** Same-key clone omit depends on `ReuseKeyPath` still meaning "no new key".

**Risk:** **MEDIUM**

---

### 09-07 — UI gates

**Summary.** Right close for DLV-04/06 and D-09; shared-renderer CR-15 named with FIELDS backstop.

**Strengths**
- Four negative controls + allowlist↔code sync.
- `uxRegionDifferenceScoped` (CR-04).
- Distinct `-run` prefix `UploadVisual`.

**Concerns**
- **HIGH:** Same PATH leak as 09-02 for every new PTY unless PATH is replaced.
- **MEDIUM:** `agent-ui-ux-designer` in an autonomous wave is an orchestrator/human step; do not fake REVIEW.md.

**Risk:** **MEDIUM–HIGH** (account leak), **LOW** for gate design.

---

### 09-08 — real account + closure

**Summary.** Protocol and opt-in isolation match ONESHOT and the `smoke` precedent. `autonomous: false` + blocking checkpoint is required (ONESHOT rule 9).

**Strengths**
- Tag ≠ `e2e`; target not a prereq of `test`/`test-e2e`/`lint`.
- `t.Cleanup` after IDs recorded.
- Before/after count of **non-prefixed** keys.
- Honest SKIP; REQUIREMENTS must not claim the run if blocked.
- ROADMAP Goal/criteria byte-unchanged.

**Concerns**
- **MEDIUM:** ID capture via inventory (above).
- **MEDIUM:** Scope probe underspecified.
- **LOW:** `make verify-upload-real-account` must `t.Skip` → exit 0 when unauth (Go skip is exit 0 — good).

**Risk:** **MEDIUM** (policy nits), **LOW** for CI mutation **if** tag isolation is implemented as specified.

---

### Suggestions (do before execute)

1. **09-02 (blocking):** All e2e after upload wiring: `PATH` = fake-ssh + fake-gh/glab + busybox-minimal **only**, or `LookPath` wrapper. Add `TestCreateFlow_ExistingPTYDoesNotInvokeRealGH` (empty argv log / no `gh` exec) on current `TestCreateFlow_TestStagePass` PATH.
2. **09-08:** Specify scope probe; constrain ID lookup to `gitid-e2e:<run-id>:`; assert `UploadKeys` titles are that prefix, not `KeyTitle`.
3. **09-02 `ProviderForHostname`:** Match hosts, not `contains("github")`.
4. Strike "real `gh ssh-key add`" from 09-02 Done; reserve for 09-08.
5. Tighten `FakeGHDir` default `*)` from `exit 0` to fail closed for unknown verbs until modes exist.

### Overall risk: **HIGH**

Justification: 09-08's policy write-up is close to exact; waves 1–7 **intend** shims but **do not enforce** them on the existing PTY helper that **keeps system PATH**. Once 09-02 wires live `buildUploaderDeps` (`exec.LookPath` + `exec.Command`), unattended `make test-e2e` on a machine with authenticated `gh` can mutate a real GitHub account — the failure mode ONESHOT confines to 09-08. Fix PATH isolation in 09-02 and the rest of the phase is **MEDIUM** (D-04, ID capture, hostname matching) rather than an account-safety incident.

---

## Codex Review

### Overall assessment

The eight-plan sequence is thoughtfully decomposed and unusually strong on UI contracts, dependency ordering, provider-command seams, and deletion safety. However, it is not ready to execute. Two HIGH-risk gaps remain:

1. Waves 2–7 do not hermetically prevent pre-existing PTY tests from resolving the developer's real `gh`/`glab`.
2. Plan 09-08 does not actually validate the compiled autonomous product path, and its per-purpose title requirement is incompatible with the planned `UploadKeys` API.

Overall risk: **HIGH** until those issues are corrected.

---

### 09-01 — Design contract and frozen copy

**Summary.** The plan establishes the design and copy contracts before runtime implementation, matching the repository's design-first process. Its scope is appropriate and it does not introduce external-account risk.

**Strengths**
- It correctly updates the current shared-renderer architecture rather than relying on the obsolete overlay model. The live manager uses an `identPane` enum and explicit dispatch/render switches at `internal/tuikit/identities.go:41-56`, not an overlay helper.
- Freezing upload copy in `internal/tuikit/design.go` respects the existing UI/backend separation: `internal/tuikit/views.go:6-20` explicitly prohibits imports from `uploader` and requires DTO conversion in the composition root.
- The planned action-menu amendment anticipates a real source defect: the current menu count is independently hard-coded as four at `internal/tuikit/identities.go:58`.
- Keeping checkbox glyphs outside label constants follows the existing composed-glyph rendering approach and avoids two sources of checkbox truth.

**Concerns**
- **LOW** — The Makefile literal-presence gate is weaker than the byte-exact Go test. A `grep -F` gate only proves that a string appears somewhere in the scanned roots; a comment or dead declaration could satisfy it. The byte-exact table test is the real contract. The plan should not describe both controls as equally strong.
- **LOW** — Copy-frozen operational error text may become expensive to improve. Scope classifiers and provider diagnostics are not yet implemented, but their complete user-facing sentences are frozen in Wave 1. Later discoveries may require another documented design amendment.

**Suggestions**
- Treat `TestFrozenUploadCopy` as the authoritative copy contract and the Makefile check as a secondary source-presence guard.
- Add an explicit rule allowing a reviewed amendment if real `gh`/`glab` behavior discovered in Waves 3 or 8 makes remediation text inaccurate.

**Risk assessment:** **LOW.** It is design-only, well ordered, and does not touch external accounts.

---

### 09-02 — GitHub tracer and PTY wiring

**Summary.** The tracer-first architecture is strong, but the plan's "every automated test uses shims" safety claim is not enforced across the existing PTY suite. This is a blocking safety defect.

**Strengths**
- Replacing first-found detection with provider-specific lookup addresses the current source behavior. `internal/uploader/uploader.go:89-105` currently searches tools in priority order rather than routing by provider.
- Adding a real-constructor nil guard follows an established mechanism: `cmd/gitid/wiring_test.go:98-115` reflects over every injected function field.
- The proposed TUI DTO boundary matches `internal/tuikit/views.go:6-20`.
- The state-machine insertion is correctly grounded in the existing async test methods at `internal/tuikit/backend.go:416-424`.
- The new tracer test itself explicitly prepends fake `gh` and records argv, which is the right process-boundary proof.

**Concerns**
- **HIGH** — Pre-existing create-flow PTY tests can reach the real `gh`. The current helper preserves the ambient system path:
  - `newRealCreateFlowCmd` appends `PATH=<fakeSSH>:<original PATH>` at `e2e/create_flow_pty_e2e_test.go:59-64`.
  - Existing tests such as `TestCreateFlow_TestStagePass` use only `FakeSSHDir` at `e2e/create_flow_pty_e2e_test.go:253-268`.
  - Those tests press Enter on the test step, which this plan changes into autonomous upload.
  - The default provider is GitHub-like, while the fake directory contains only `ssh`, so a real authenticated `gh` later in the original `PATH` remains resolvable.

  Updating the helper to accept variadic prefixes, as 09-02 proposes at `.planning/phases/09-upload-credentials-assist/09-02-PLAN.md:461-464`, does not make every existing caller pass a fake provider CLI. This directly contradicts the plan's claim that all automated tests are offline-shimmed at `09-02-PLAN.md:575`.
- **MEDIUM** — `ProviderForHostname` uses an over-broad substring predicate. The plan specifies "contains `github`/`gitlab`," but D-13 limits autonomous behavior to the main domains. A hostname such as `notgithub.example` would qualify. SSH aliases such as `personal.github.com` require suffix-aware matching, not unrestricted substring matching.
- **MEDIUM** — Eligibility performs synchronous subprocess calls from the render path. The plan says `UploadEligibility` is sync and may be evaluated during rendering. `gh auth status` can be slow or hung; Bubble Tea rendering must not block on an external process. There is also no timeout in the planned `Deps.RunCmd` signature.

**Suggestions**
- Make provider isolation global and fail-closed: build a provider-shim directory containing both `gh` and `glab` for every unattended PTY test; default both shims to "tool absent" or unauthenticated; ensure `newRealCreateFlowCmd` constructs a controlled `PATH` where real provider CLIs are unreachable; add a suite-level source or runtime guard asserting every real-binary upload-capable PTY command receives the provider shim directory.
- Match `github.com`/`gitlab.com` on a DNS-label boundary, such as exact match or `strings.HasSuffix(host, ".github.com")`.
- Probe eligibility asynchronously and cache it in model state. Add command timeouts to all provider subprocess calls.

**Risk assessment:** **HIGH.** Without a hermetic PATH guard, routine `make test-e2e` can mutate a real account.

---

### 09-03 — Uploader engine, inventory, classification, deletion

**Summary.** The engine decomposition is generally sound and testable, but two security claims are stronger than the proposed mechanisms, and the API shape conflicts with the real-account policy planned for Wave 8.

**Strengths**
- Independent per-registration outcomes properly model GitHub partial success.
- Correcting GitLab to `auth_and_signing` aligns with the project's one-key authentication-and-signing recipe at `recipes/README.md:24-29`.
- Full JSON parsing and normalized key comparison centralize provider behavior rather than duplicating it across TUI and CLI.
- Exact machine-scoped title matching is a good prerequisite for the rotate delete offer.
- Argument arrays preserve the current no-shell design in `internal/uploader/uploader.go:164-185`.

**Concerns**
- **HIGH** — `UploadKeys` accepts only one title for all registrations. The planned signature is `UploadKeys(tool, toolPath, pubPath, title string, regs, deps)` at `.planning/phases/09-upload-credentials-assist/09-03-PLAN.md:140-146`. Plan 09-08 requires distinct exact titles `gitid-e2e:<run-id>:<purpose>` where purpose is the registration kind. A single title parameter cannot produce separate authentication and signing titles.
- **MEDIUM** — `.pub` suffix validation does not establish the "public-key-only" invariant. `requirePubPath` merely checks `strings.HasSuffix(path, ".pub")` at `09-03-PLAN.md:147-150`. A private key copied or symlinked under a `.pub` name would pass. This is insufficient for the plan's critical ASVS V5 claim.
- **MEDIUM** — `DeleteKey` is a destructive primitive with no intrinsic ID/title binding. The function accepts an arbitrary ID. Call-site confirmation is necessary, but the engine API makes accidental unsafe reuse easy.
- **LOW** — Source-scanning for a second argv builder is brittle. Lexical adjacency tests can miss a dynamically constructed duplicate or fail on harmless formatting.

**Suggestions**
- Change the upload batch API to accept per-registration requests (`{Registration, Title}` pairs).
- Parse and validate public-key content before execution, using an authorized-key parser; reject private-key PEM markers and malformed public keys.
- Expose a safer deletion operation accepting the recorded `ExistingKey` or a confirmed `{ID, expectedTitlePrefix}` tuple.
- Keep shown-vs-run tests based on recorded argv rather than relying mainly on source scans.

**Risk assessment:** **MEDIUM-HIGH.** Core architecture is good, but the title API blocks exact policy compliance and the public-key guard is too weak.

---

### 09-04 — Complete wizard upload behavior

**Summary.** This plan covers most functional branches and correctly centralizes dedupe and failure handling. Its main risks are blocking I/O, unclear upload timing, and overcomplicated error recovery.

**Strengths**
- Inventory failure explicitly degrades to upload rather than gating creation.
- Missing-registration calculation is centralized in the uploader engine, preventing CLI/TUI drift.
- Manual fallback reuses `internal/upload.Instructions`, whose current GitHub and GitLab branches are at `internal/upload/upload.go:31-54`.
- Reusing the existing tester classification is correct: `PASS`, `ReachableNotUploaded`, and `Failure` already exist at `internal/tester/tester.go:10-23`.
- The no-persisted-upload-state snapshot directly tests D-18.

**Concerns**
- **MEDIUM** — Provider I/O still has no timeout. Inventory and uploads occur inside a `tea.Cmd`, which avoids blocking Update, but a hung `gh`/`glab` can leave the wizard stuck indefinitely.
- **MEDIUM** — The plan says "announce before run," but a single `UploadRunMsg` returned after execution cannot visibly render an announcing state before the subprocess completes. The tracer stores commands in the final result. A true announcing state requires an initial message/update followed by the execution command.
- **MEDIUM** — Confirmation semantics are underspecified. Post-upload inventory includes the initial dedupe read plus one or two confirmation reads. Tests that assert "exactly one confirmation read" need explicit phase-aware recording to avoid counting the pre-upload inventory.
- **LOW** — Catching panics to keep creation moving may conceal programming defects. Expected provider errors should be converted to result rows; arbitrary panics should not silently become operational warnings.

**Suggestions**
- Add `context.Context` or an injected timeout-aware runner to every provider operation.
- Model announce and completion as two messages: `UploadStartedMsg` first, then `UploadRunMsg`.
- Tag or separately record inventory phases in tests.
- Recover only at the outer application boundary if the repository has an established panic policy; do not normalize programmer panics as provider failures.

**Risk assessment:** **MEDIUM.** Behavior coverage is strong, but the async UX contract and timeout handling need refinement.

---

### 09-05 — CLI parity and autonomous write-verb upload

**Summary.** The shared orchestration and shared verb specification fit the source architecture well. The same ambient-PATH safety issue from 09-02 also affects this wave and must be fixed before enabling default autonomous upload in all write verbs.

**Strengths**
- The proposed `identityVerb` integration uses the real one-spec/two-command mechanism at `cmd/gitid/identity.go:17-27` and `cmd/gitid/identity.go:61-95`.
- Missing-name behavior correctly reuses `depthResolver`, whose three headless/TTY outcomes are already implemented at `cmd/gitid/identity.go:101-155`.
- Not resurrecting archived `copy` is consistent with the current command surface.
- One orchestration and one printer is the right way to maintain CLI/TUI parity.
- The planned e2e tests explicitly use provider shims and check `.pub` argv.

**Concerns**
- **HIGH** — Existing CLI tests are not comprehensively isolated from real provider CLIs. Adding autonomous upload to create, clone, rotate, and new-key means any existing test that constructs the real backend with a GitHub/GitLab account can probe or upload through ambient `PATH`. The plan only promises shims for its newly added e2e cases.
- **MEDIUM** — `register-key` always exits zero even when its sole requested operation failed. "Upload never gates the primary operation" is sensible for create/rotate, but `register-key` has no other primary operation. Always returning zero makes scripting unable to distinguish success from failure.
- **MEDIUM** — `--dry-run` may still contact the provider. The plan permits auth and inventory reads at `09-05-PLAN.md:242-249`. That is read-only, but the help text "executes nothing" can be read as no subprocess or network activity.

**Suggestions**
- Install fail-closed `gh` and `glab` shims at the test harness level, not test by test.
- Give `register-key` a nonzero exit for failed registration, or explicitly define a distinct "primary operation succeeded but upload degraded" exit contract. The "never gates" rule should remain for create/rotate/clone/new-key.
- Make dry-run documentation explicit: local/provider reads may occur, but no remote mutation occurs.

**Risk assessment:** **HIGH** until ambient provider access is eliminated; otherwise **MEDIUM**.

---

### 09-06 — Identity Manager, ceremonies, remote delete offer

**Summary.** The UI integration follows the current source architecture and the remote deletion ceremony is carefully fail-closed. The main remaining issue is synchronous network work on the UI thread.

**Strengths**
- Adding `paneRegisterKey` after existing enum values avoids shifting current pane IDs; the current enum ends at `paneKeyCeremony` at `internal/tuikit/identities.go:43-56`.
- Deriving menu length fixes the real independent-count defect at `internal/tuikit/identities.go:58`.
- Async message guards mirror the established pane/pending pattern rather than allowing stale results to land in another pane.
- The delete offer defaults to leave, requires an explicit selection move, exact-matches the machine-scoped title, and deletes the stored ID without re-resolving it.
- Falling back to the existing grace hint on inventory error is appropriately fail-closed.

**Concerns**
- **MEDIUM** — `RotateDeleteOffer` is specified as synchronous while it performs a fresh provider inventory read. Calling it as the result pane is entered can freeze Bubble Tea on network or CLI delay.
- **MEDIUM** — Delete failure cleanup/retry behavior is incomplete. The plan says render failure text, but does not specify whether the confirmed ID remains available for retry or whether leaving the pane loses the only safe target reference.
- **LOW** — The modal opens and immediately mutates remotely on `u`. This is user-authorized because opening is explicit opt-in, but the action-menu label should clearly communicate immediate execution.

**Suggestions**
- Make offer resolution a `tea.Cmd` with loading, success, and unavailable messages.
- Preserve the confirmed `{ID, title, prefix}` tuple after a failed deletion so a retry deletes the same reviewed target.
- Use copy such as "Register key now" if immediate execution is intended.

**Risk assessment:** **MEDIUM.** Deletion safety is strong; UI responsiveness and retry semantics need tightening.

---

### 09-07 — PTY and visual-regression gates

**Summary.** The visual and PTY coverage is comprehensive and correctly acknowledges the shared-renderer blind spot. It does not, however, repair the missing suite-wide provider isolation.

**Strengths**
- It adds raw-keystroke coverage for ready, unauthenticated, disabled, omitted, partial-scope, already-complete, modal, and delete-offer states.
- It explicitly backstops the shared-renderer limitation with contract-derived assertions.
- Scoped allowlist predicates and four negative controls make the visual gate meaningfully falsifiable.
- It requires both real and dummy compiled PTY sessions, consistent with the existing `make test-e2e` contract at `Makefile:443-463`.

**Concerns**
- **HIGH** — New tests use shims, but the full e2e suite still contains old upload-capable tests using only fake SSH and ambient provider CLIs. The plan's threat claim that every PTY test prepends provider shims is therefore not established.
- **MEDIUM** — Approved frames are copied manually from a gitignored directory. Without a provenance hash or deterministic promotion tool, it is possible to commit a frame from a different run than the passing test.
- **LOW** — Eight registered screens versus twelve new PTY tests needs an explicit mapping. The plan asks the summary to provide it, but a machine-checked registry-to-test mapping would be stronger.

**Suggestions**
- Add a mandatory suite-wide provider-shim guard in this wave if it was not fixed in 09-02.
- Generate frame metadata containing source commit, test name, shim mode, and content hash; promote frames via one reproducible command.
- Add a test that every `uploadVisualSpecs()` screen ID maps to at least one committed PTY frame.

**Risk assessment:** **HIGH** while the ambient-PATH leak remains; otherwise **LOW-MEDIUM**.

---

### 09-08 — Real-account validation and closure

**Summary.** The plan captures most of the external-account policy — human authorization, opt-in tag, disposable key, prefixed titles, recorded IDs, ID-scoped deletion, and final sweep — but it does not match the policy exactly and does not validate the product path it claims to validate.

**Strengths**
- The human checkpoint explicitly describes the two temporary registrations and cleanup at `.planning/phases/09-upload-credentials-assist/09-08-PLAN.md:90-134`.
- The proposed test uses a distinct build tag and isolated Make target at `09-08-PLAN.md:151-152` and `09-08-PLAN.md:200-205`.
- Cleanup re-reads inventory, locates the recorded ID, confirms its prefix, then deletes by that ID at `09-08-PLAN.md:176-193`. That matches the safe interpretation of `.planning/ONESHOT.md:130-140`.
- A missing auth/scope condition is explicitly recorded as a skip/block with no mock substitute.
- The final sweep checks the exact current-run prefix, not all `gitid-e2e` resources.

**Concerns**
- **HIGH** — It does not test the compiled autonomous gitid path. The plan says this proves the autonomous product path, but registration is performed by calling `uploader.UploadKeys` directly at `09-08-PLAN.md:171-175`. It bypasses compiled CLI/TUI wiring, `buildUploaderDeps`, provider eligibility, autonomous trigger logic, shared orchestration, CLI output, and post-upload product control flow. This is an uploader integration test, not a real-account e2e of gitid.
- **HIGH** — The exact per-purpose title policy is incompatible with the planned engine API. The policy requires `gitid-e2e:<run-id>:<purpose>` at `.planning/ONESHOT.md:130-133`, and 09-08 promises different purposes at `09-08-PLAN.md:162-166`. But 09-03 defines one `title string` for all registrations at `09-03-PLAN.md:140-146`.
- **HIGH** — Scope verification is vague and not exact. "Attempting a scope-revealing read" at `09-08-PLAN.md:156-161` does not define which command proves both `admin:public_key` and `admin:ssh_signing_key`. A successful read endpoint may need only read scope and does not prove upload authorization.
- **MEDIUM** — `t.Cleanup` plus explicit cleanup can double-delete. Cleanup is registered at `09-08-PLAN.md:176-180`, then deletion is performed explicitly at `09-08-PLAN.md:185-191`. Unless the cleanup closure tracks which IDs were already removed, it will run again after the test and attempt a second deletion.
- **MEDIUM** — The authorization checkpoint claims IDs "GitHub returns," but `gh ssh-key add` output is not specified to return them. The actual mechanism is a subsequent inventory lookup. The checkpoint and summary should describe that accurately.
- **MEDIUM** — A blocked real-account run can still close UP-03. Plan 09-08 permits marking UP-03 closed while noting that required real-account validation was blocked at `09-08-PLAN.md:248-262`. That conflicts with the plan's own objective that this validation is required evidence. It should remain implemented-but-unvalidated or explicitly waived, not fully closed.

**Suggestions**
- Run the compiled product under the dedicated tag/target: generate a disposable identity/key in a temporary HOME, invoke `gitid register-key <identity>` (or the relevant compiled headless create flow), capture stdout and verify the announced commands, record IDs through inventory only after the compiled binary returns, clean up through a dedicated test cleanup helper.
- Modify the engine to accept a distinct title per registration.
- Define exact scope preflight — for example, parsing the authenticated token's reported scope list and requiring both exact names before upload. Record the command and raw redacted evidence.
- Implement idempotent cleanup state: keep a set of outstanding recorded IDs, remove an ID from the set after successful explicit deletion, have `t.Cleanup` process only outstanding IDs, always run the final prefix sweep.
- If auth/scope is unavailable, record `BLOCKED/SKIP`, execute no mock, and leave real-account validation and final UP-03 verification open.

**Risk assessment:** **HIGH.** The safety structure is promising, but the test does not validate the claimed product path and cannot currently satisfy the exact title policy.

---

### Cross-plan strengths
- The wave ordering is coherent: contract → tracer → engine → full wizard → CLI → manager → UI gates → real-account validation.
- The source architecture is generally respected across shared CLI verbs (`cmd/gitid/identity.go:17-95`), UI/backend separation (`internal/tuikit/views.go:6-20`), injected backend seams (`internal/tuikit/backend.go:416-424`), current pane architecture (`internal/tuikit/identities.go:41-56`), and existing provider instruction copy (`internal/upload/upload.go:31-54`).
- The plans consistently use argument slices rather than a shell.
- Remote deletion is designed to default to no, exact-match the machine title, and use a confirmed ID.
- The dedicated real-account tag/target is the correct isolation strategy for Wave 8.

### Required corrections before execution
1. Make all Waves 2–7 tests hermetic with respect to both `gh` and `glab`; never append an uncontrolled original PATH that can expose real provider CLIs.
2. Add provider subprocess timeouts.
3. Make provider matching suffix/boundary-aware.
4. Replace suffix-only `.pub` checking with public-key content validation.
5. Change batch upload to accept per-registration titles.
6. Make 09-08 run the compiled gitid path.
7. Define exact scope verification and recorded blocked evidence.
8. Make real-account cleanup idempotent and preserve the final prefix sweep.
9. Do not close required real-account validation as complete when it was blocked.

### Final risk assessment
**HIGH.** The implementation architecture is mostly strong, but the current plans could allow routine e2e tests to touch a real account, and the dedicated real-account plan does not yet prove the product behavior or satisfy the exact naming protocol. These are plan-review blockers, not implementation polish.

---

## Codex Review (codex-sol) — FAILED, diagnostic stub only

This instance's configured model (`openai/gpt-5.6-sol-fast`) was rejected by the Codex CLI's
ChatGPT-account authentication and produced no review:

```
ERROR: {"type":"error","status":400,"error":{"type":"invalid_request_error","message":"The
'openai/gpt-5.6-sol-fast' model is not supported when using Codex with a ChatGPT account."}}
```

Reproduced independently of the `openai/` prefix (bare `gpt-5.6-sol-fast` fails identically).
The plain `codex` lane's default banner-resolved model (`gpt-5.6-sol`, no `-fast` suffix)
succeeded — see the "Codex Review" section above, which was run as a substitute to still
deliver a genuine second cross-AI opinion. No findings from this failed instance are included
in the consensus above.

---
