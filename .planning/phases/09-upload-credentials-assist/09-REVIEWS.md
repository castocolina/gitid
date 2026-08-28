---
phase: 9
reviewers: [xai-grok, codex, codex-sol]
reviewed_at: 2026-08-28T21:06:35Z
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

# Cross-AI Plan Review — Phase 9 (Cycle 2)

> **Note on reviewer roster:** as in cycle 1, `review.default_reviewers` configures
> `codex-sol` (model `openai/gpt-5.6-sol-fast`) and `xai-grok` (opencode, model
> `xai/grok-4.6`). `codex-sol` again failed at invocation — the pinned model is rejected
> by this Codex CLI's ChatGPT-account authentication (`"The 'openai/gpt-5.6-sol-fast'
> model is not supported when using Codex with a ChatGPT account."`). Its section below
> is the diagnostic stub, not a review — this is the same known, expected/acceptable
> substitution as cycle 1 (`review.reviewer_instances.codex-sol.model` still needs
> fixing — drop `-fast` and/or the `openai/` prefix — but that is out of scope for this
> review cycle). The plain `codex` lane was invoked as substitute, using its
> default banner-resolved model (`gpt-5.6-sol`). Both `xai-grok` and `codex` completed
> full source-grounded reviews of the CURRENT plan set (post-revision commit `a28ccbb`)
> and are weighted at full consensus below.

## Consensus Summary

Both reviewers independently re-verified the eight cycle-1 findings against the CURRENT
`09-0X-PLAN.md` files and the live codebase (not against the revision's commit-message
prose), and **both confirm all 8 cycle-1 findings — including the ambient-PATH-leak
HIGH — are now backed by concrete, executor-followable plan content**: named
constructors (`e2eEnv`, `ProviderDenyDir`), named tests (`TestE2EEnvHidesRealProviderCLIs`,
`TestEveryE2EChildEnvIsHermetic`, `TestUploadKeysHonoursPerRegistrationTitles`,
`TestUploadKeyRefusesAPrivateKeyRenamedAsPub`, `TestRegisterKeyExitContract`, etc.),
explicit file:line locations in the plans, and (where relevant) confirmation that the
underlying code defect the finding described still exists pre-execution — i.e. the plan
now specifies the fix rather than merely asserting it happened. `xai-grok` additionally
verified the live code still has the leak (`e2e/create_flow_pty_e2e_test.go:59-64` still
prepends fake SSH onto ambient PATH), confirming the guard test's target defect is real,
not hypothetical, and that the plan-level fix is not yet applied to code (expected —
execution has not started).

**Both reviewers independently converge on the same fresh, cycle-2-introduced concern:**
Plan 09-08's real-account "product" validation phase (Phase P) creates an identity with
upload suppressed (`create --no-upload`) and then explicitly runs `gitid register-key` —
this proves the compiled-binary CLI path and the manual re-registration verb, but it does
**not** exercise the default, autonomous upload-on-create trigger that UP-03 is meant to
validate. `codex` rates this a fresh HIGH (closure claims coverage it does not have);
`xai-grok` rates it MEDIUM (the design gap is real but correctable with an additional
phase, not a redesign). This is the single most consequential disagreement between the
two reviewers and should be resolved before this phase is considered execution-ready.

`codex` additionally raised two new HIGH concerns not caught by `xai-grok`: (1) Phase P's
use of `SandboxHome` (temporary `HOME`) risks hiding the developer's real, already
authenticated `gh` CLI session, since `gh`'s config normally lives under the real `HOME`
and the plan does not preserve/pass through `GH_CONFIG_DIR` or another explicit
read-only auth source; and (2) `UploadEligibility` passes SSH-alias hostnames (e.g.
`ssh.github.com`, `personal.github.com`) directly to `gh auth status --hostname <input>`,
where a real GitHub session is normally keyed to the canonical `github.com`, so a
correctly configured identity could be misreported as unauthenticated. `xai-grok` did not
independently flag either of these — treat both as open until a third pass confirms or
refutes them against the plan text and the `SandboxHome`/`ProviderForHostname`
implementations.

### Agreed Strengths

- The eight-wave ordering (contract → tracer → engine → full wizard → CLI → manager → UI
  gates → real-account validation) remains coherent and unchanged by the revision.
- The suite-wide `e2eEnv` hermetic-boundary fix is grounded in the actual current
  environment-construction sites in the e2e package, not scoped only to new tests.
- Content-based `.pub` validation (via `Deps.ReadFile` + `ssh.ParseAuthorizedKey` +
  private-key-PEM rejection) closes the convention-only gap cleanly, with a regression
  test using a real private key renamed with a `.pub` suffix.
- `register-key`'s exit-code contract is now unambiguous (zero for any success/partial
  success/already-complete/ineligible/dry-run; non-zero only when every attempted
  registration failed), resolving the cycle-1 D-03 ambiguity.
- UP-03 closure is now correctly three-valued (EXECUTED closes it; SKIPPED/BLOCKED leave
  it open), with a mechanical read-back preventing a closed checkbox from coexisting with
  a blocked/skipped marker.

### Agreed Concerns

- **MEDIUM/HIGH (split) — Plan 09-08's real-account Phase P does not exercise
  create-time autonomous upload**, only the compiled binary's manual `register-key`
  verb after upload was explicitly suppressed at creation. Both reviewers agree this is
  a real coverage gap relative to what UP-03 claims to validate; they diverge only on
  severity (`codex`: HIGH — "overstates coverage"; `xai-grok`: MEDIUM — "design gap,
  correctable with an optional Phase P2 running `gitid create` without `--no-upload`").
- **MEDIUM — 09-02's `files_modified` list for the e2e hermetic migration may be
  incomplete** relative to the live tree. `xai-grok` names two specific gaps:
  `e2e/global_ssh_storage_pty_e2e_test.go` (via `newRealCreateFlowCmd`) and
  `e2e/install_e2e_test.go` (`cmd.Env = os.Environ()`). Both reviewers agree the
  source-scan test (`TestEveryE2EChildEnvIsHermetic`) is the real backstop and must be
  implemented as genuine assignment analysis (e.g. go/ast), not a hardcoded filename
  list or a naive `grep e2eEnv` — a naive string-presence check would not catch
  `create_flow_pty_e2e_test.go`'s existing `os.Environ()`-based construction.

### Divergent Views

- **Overall risk rating:** `codex` rates the revised plan set overall **HIGH** risk
  (driven by the three fresh concerns above, especially the SandboxHome/auth-hostname
  pair it alone identified). `xai-grok` rates it overall **MEDIUM** ("blocking design is
  in the plans; remaining risk is executor miss + 09-08 product-path vs create
  autonomy"). Reconcile by weight of evidence: the Phase-P-autonomy gap is corroborated
  by both and should be treated as a real, unresolved concern; the `SandboxHome`/
  auth-hostname findings are `codex`-only and need independent verification (they were
  not contradicted by `xai-grok`, but `xai-grok`'s review does not mention `HOME`
  sandboxing or the `AuthHostname` distinction at all, so absence is not confirmation).
- `codex` frames the ambient-PATH fix as fully "concrete enough for execution"; `xai-grok`
  frames the same fix as "specified" but flags execution-quality risk in the source-scan
  implementation (assignment analysis vs string grep) more explicitly, plus two
  additional e2e files it believes 09-02's file list misses. Both are actionable
  refinements to 09-02 rather than a disagreement about whether the design is sound.

---

## Codex Review (codex-sol)

codex review failed or returned empty output. stderr:
OpenAI Codex v0.149.1
--------
workdir: /Users/ramon/git/personal/ssh-git-config
model: openai/gpt-5.6-sol-fast
provider: openai
approval: never
sandbox: workspace-write [workdir, /tmp, $TMPDIR]
reasoning effort: low
reasoning summaries: none
session id: 01a04a2d-f47b-7b72-87a1-5ff153261f4c
--------
[full prompt echo omitted — see cycle-1 09-REVIEWS.md for the identical stub shape]
--------
ERROR: {"type":"error","status":400,"error":{"type":"invalid_request_error","message":"The 'openai/gpt-5.6-sol-fast' model is not supported when using Codex with a ChatGPT account."}}
ERROR: {"type":"error","status":400,"error":{"type":"invalid_request_error","message":"The 'openai/gpt-5.6-sol-fast' model is not supported when using Codex with a ChatGPT account."}}

---

## Codex Review (codex, substitute for codex-sol)

# Cross-AI Plan Review — Cycle 2

## Summary

The revision materially resolves the cycle-1 findings. The ambient-PATH fix, provider routing, bounded async execution, per-registration titles, content-based public-key validation, delete-ID validation, exact scope checking, idempotent cleanup bookkeeping, and conditional UP-03 closure are all expressed as concrete tasks with named tests and acceptance criteria.

However, the fresh pass found three significant issues in Plan 09-08 and one provider-auth bug in Plan 09-02. Most importantly, the real-account "product" phase runs `register-key` after suppressing upload during creation, so it does not actually prove UP-03's autonomous trigger. Additionally, sandboxing `HOME` can hide the real `gh` authentication configuration, and provider auth is probed using SSH aliases such as `ssh.github.com` rather than the canonical CLI auth host `github.com`.

Overall risk remains **HIGH** until those issues are corrected.

## Cycle-1 Finding Verification

### 1. HIGH — ambient PATH could reach real `gh`/`glab`

**Status: RESOLVED in plan**

The existing code confirms the original vulnerability: `newRealCreateFlowCmd` prepends fake SSH to the ambient PATH, leaving real provider CLIs reachable ([create_flow_pty_e2e_test.go](/Users/ramon/git/personal/ssh-git-config/e2e/create_flow_pty_e2e_test.go:52)). There are numerous other hand-built child environments across the e2e package, including CLI, PTY, dummy, and debug tests.

The revised plan now specifies:

- Fail-closed `gh` and `glab` deny shims with logging ([09-02-PLAN.md](/Users/ramon/git/personal/ssh-git-config/.planning/phases/09-upload-credentials-assist/09-02-PLAN.md:550)).
- One `e2eEnv` constructor ordering caller shims, deny shims, then ambient PATH ([09-02-PLAN.md](/Users/ramon/git/personal/ssh-git-config/.planning/phases/09-upload-credentials-assist/09-02-PLAN.md:561)).
- Migration of every enumerated child-environment construction site ([09-02-PLAN.md](/Users/ramon/git/personal/ssh-git-config/.planning/phases/09-upload-credentials-assist/09-02-PLAN.md:581)).
- A runtime resolution test, `TestE2EEnvHidesRealProviderCLIs` ([09-02-PLAN.md](/Users/ramon/git/personal/ssh-git-config/.planning/phases/09-upload-credentials-assist/09-02-PLAN.md:594)).
- A build-tag-aware source scan, `TestEveryE2EChildEnvIsHermetic`, which must fail with the offending file and line for an unmigrated `.Env` assignment ([09-02-PLAN.md](/Users/ramon/git/personal/ssh-git-config/.planning/phases/09-upload-credentials-assist/09-02-PLAN.md:604)).
- A negative demonstration requiring a temporary hand-written environment assignment to make the guard fail ([09-02-PLAN.md](/Users/ramon/git/personal/ssh-git-config/.planning/phases/09-upload-credentials-assist/09-02-PLAN.md:683)).
- A pre-existing-flow regression control proving a test with no fake provider CLI still resolves only the deny shim.

This is concrete enough for execution and directly covers the current source's many `.Env` construction sites.

### 2. Per-registration titles

**Status: RESOLVED**

The revised engine API now has:

- `RegistrationRequest{Registration, Title}` ([09-03-PLAN.md](/Users/ramon/git/personal/ssh-git-config/.planning/phases/09-upload-credentials-assist/09-03-PLAN.md:170)).
- `UploadKey` taking one request ([09-03-PLAN.md](/Users/ramon/git/personal/ssh-git-config/.planning/phases/09-upload-credentials-assist/09-03-PLAN.md:175)).
- `UploadKeys` taking `[]RegistrationRequest`, with no batch-wide title and no title derivation or override ([09-03-PLAN.md](/Users/ramon/git/personal/ssh-git-config/.planning/phases/09-upload-credentials-assist/09-03-PLAN.md:183)).
- Tests for distinct titles and for preventing product-title substitution.
- A signature-level reflection assertion, avoiding comment-sensitive source scanning.

Plan 09-08 Phase N then supplies exact, distinct policy titles for authentication and signing and verifies them in real provider inventory ([09-08-PLAN.md](/Users/ramon/git/personal/ssh-git-config/.planning/phases/09-upload-credentials-assist/09-08-PLAN.md:334)).

### 3. Real-account validation bypassed the compiled product

**Status: PARTIALLY RESOLVED**

The revision introduces two concrete phases:

- Phase P builds and invokes the compiled `gitid` binary and asserts exit code, announced commands, and result rows ([09-08-PLAN.md](/Users/ramon/git/personal/ssh-git-config/.planning/phases/09-upload-credentials-assist/09-08-PLAN.md:306)).
- Phase N directly exercises the engine with exact per-registration policy titles ([09-08-PLAN.md](/Users/ramon/git/personal/ssh-git-config/.planning/phases/09-upload-credentials-assist/09-08-PLAN.md:334)).
- A scoped source check must prove Phase P invokes the built binary and neither constructs `uploader.Deps` nor calls the engine directly ([09-08-PLAN.md](/Users/ramon/git/personal/ssh-git-config/.planning/phases/09-upload-credentials-assist/09-08-PLAN.md:401)).

That resolves the engine-only problem, but Phase P runs `create --no-upload` followed by explicit `register-key`. It therefore proves the compiled manual re-registration path, not the default autonomous create trigger. This remains a new HIGH concern below.

### 4. Scope verification was underspecified

**Status: RESOLVED**

Plan 09-08 now names the exact command:

```text
gh auth status --hostname github.com
```

It requires parsing the reported `Token scopes:` line into a set and checking exact membership of both required scopes ([09-08-PLAN.md](/Users/ramon/git/personal/ssh-git-config/.planning/phases/09-upload-credentials-assist/09-08-PLAN.md:270)).

Acceptance additionally requires account-free table tests proving:

- Substring-containing fake scope names are rejected.
- Either single required scope is insufficient.
- The exact pair succeeds.
- No token-revealing flag appears in the preflight argv ([09-08-PLAN.md](/Users/ramon/git/personal/ssh-git-config/.planning/phases/09-upload-credentials-assist/09-08-PLAN.md:403)).

### 5. `.pub` suffix did not prove public-key content

**Status: RESOLVED**

The current uploader indeed enforces this only by convention ([uploader.go](/Users/ramon/git/personal/ssh-git-config/internal/uploader/uploader.go:119)).

The revised plan requires:

- Reading through injected `Deps.ReadFile`.
- Rejecting private-key PEM markers.
- Parsing as an OpenSSH authorized-key line.
- Running all validation before `RunCmd` ([09-03-PLAN.md](/Users/ramon/git/personal/ssh-git-config/.planning/phases/09-upload-credentials-assist/09-03-PLAN.md:198)).
- A regression using a real generated private key renamed with a `.pub` suffix.
- A positive valid-public-key control.
- Zero subprocess invocations for every invalid-content case.

This is concrete and closes the ASVS V5 gap.

### 6. Delete accepted an arbitrary provider ID

**Status: RESOLVED**

Plan 09-03 now requires numeric-only resource IDs, rejecting empty, whitespace-bearing, dash-leading, and non-numeric values before building argv or invoking a subprocess. It also introduces `DeleteRecordedKey`, which retains the inventory entry's ID/title binding.

The tests explicitly require zero `RunCmd` calls for malformed IDs and verify that `DeleteRecordedKey` passes the recorded ID. This is a meaningful strengthening over argument-slice safety alone.

### 7. Provider routing, hostname matching, and blocking I/O

**Status: RESOLVED in plan, with one new functional flaw**

The current source still has first-found routing and a bare auth probe ([uploader.go](/Users/ramon/git/personal/ssh-git-config/internal/uploader/uploader.go:83), [uploader.go](/Users/ramon/git/personal/ssh-git-config/internal/uploader/uploader.go:107)).

The revision concretely replaces it with:

- Exact-or-dot-suffix main-domain matching, including normalization and negative cases such as `github.example.com` and `github.com.evil.net` ([09-02-PLAN.md](/Users/ramon/git/personal/ssh-git-config/.planning/phases/09-upload-credentials-assist/09-02-PLAN.md:229)).
- Provider-specific `DetectFor`, never cross-routing to the other CLI ([09-02-PLAN.md](/Users/ramon/git/personal/ssh-git-config/.planning/phases/09-upload-credentials-assist/09-02-PLAN.md:245)).
- `AuthCheck` with an explicit hostname ([09-02-PLAN.md](/Users/ramon/git/personal/ssh-git-config/.planning/phases/09-upload-credentials-assist/09-02-PLAN.md:251)).
- `exec.CommandContext` with a configurable timeout ([09-02-PLAN.md](/Users/ramon/git/personal/ssh-git-config/.planning/phases/09-upload-credentials-assist/09-02-PLAN.md:372)).
- Async `tea.Cmd` eligibility, never provider I/O in `View` ([09-02-PLAN.md](/Users/ramon/git/personal/ssh-git-config/.planning/phases/09-upload-credentials-assist/09-02-PLAN.md:283)).
- Memoization, stale-message rejection, timeout tests, and a source check preventing render-path probing.

The general finding is resolved, but the canonical auth-host issue described below must be corrected.

### 8. Double cleanup and false UP-03 closure

**Status: RESOLVED**

Plan 09-08 now specifies:

- An outstanding-ID map as the single cleanup source of truth.
- Immediate insertion once an ID is learned.
- Removal after successful explicit deletion.
- `t.Cleanup` operating only on IDs still outstanding.
- Account-free bookkeeping tests for both full explicit cleanup and early failure ([09-08-PLAN.md](/Users/ramon/git/personal/ssh-git-config/.planning/phases/09-upload-credentials-assist/09-08-PLAN.md:352), [09-08-PLAN.md](/Users/ramon/git/personal/ssh-git-config/.planning/phases/09-upload-credentials-assist/09-08-PLAN.md:404)).

UP-03 closure is now explicitly three-valued:

- EXECUTED → close.
- SKIPPED → remain open.
- BLOCKED → remain open ([09-08-PLAN.md](/Users/ramon/git/personal/ssh-git-config/.planning/phases/09-upload-credentials-assist/09-08-PLAN.md:449)).

Acceptance requires a mechanical read-back preventing a closed checkbox from coexisting with a blocked/skipped marker ([09-08-PLAN.md](/Users/ramon/git/personal/ssh-git-config/.planning/phases/09-upload-credentials-assist/09-08-PLAN.md:513)).

## Strengths

- The eight-wave ordering is coherent: contract → tracer → engine → full wizard → CLI → manager → UI gates → real-account validation.
- The e2e isolation correction is suite-wide and grounded in the actual current environment assignments, rather than limited to new tests.
- The plan preserves `internal/tuikit`'s backend-free boundary and uses DTO conversion at the composition root.
- Shown-command-equals-run-command remains structural through one argv builder.
- Inventory failures consistently degrade to upload rather than becoming gates.
- GitLab's duplicate/cross-account behavior is intentionally distinguished from GitHub's benign duplicate behavior.
- Remote deletion defaults to leave, resolves a fresh machine-scoped target, and retains the confirmed ID across a failed retry.
- `register-key` now has an unambiguous exit contract: zero for any success, partial success, already complete, ineligible/manual fallback, or dry run; non-zero only when work was attempted and every attempt failed ([09-05-PLAN.md](/Users/ramon/git/personal/ssh-git-config/.planning/phases/09-upload-credentials-assist/09-05-PLAN.md:230), [09-05-PLAN.md](/Users/ramon/git/personal/ssh-git-config/.planning/phases/09-upload-credentials-assist/09-05-PLAN.md:289)).
- Frame provenance, registry-to-frame correspondence, negative controls, and honest handling of an unavailable specialist UI critique are unusually strong review mechanisms.

## Concerns

- **HIGH — Phase P does not prove autonomous upload.** It deliberately creates with upload suppressed, then invokes `gitid register-key` ([09-08-PLAN.md](/Users/ramon/git/personal/ssh-git-config/.planning/phases/09-upload-credentials-assist/09-08-PLAN.md:306)). That proves compiled wiring and the explicit rerun verb, but bypasses the default autonomous trigger whose real-provider validation is supposed to close UP-03. Calling this authoritative UP-03 evidence overstates coverage.

- **HIGH — sandbox `HOME` can hide the real `gh` authentication session.** `SandboxHome` sets the process-wide `HOME` to a temporary directory ([harness_test.go](/Users/ramon/git/personal/ssh-git-config/e2e/harness_test.go:39)). Plan 09-08 then runs the compiled binary under that temporary HOME while expecting its child `gh` to use the developer's real authenticated configuration. The plan does not preserve `GH_CONFIG_DIR`, the original config directory, or another explicit read-only auth source. This may cause the required run to skip as unauthenticated even after the user authorized it.

- **HIGH — auth probing uses the SSH/alias hostname instead of the canonical provider auth host.** `ProviderForHostname` deliberately accepts `ssh.github.com` and other subdomains, but `UploadEligibility` passes that same input to `gh auth status --hostname <input>` and memoizes the result per provider ([09-02-PLAN.md](/Users/ramon/git/personal/ssh-git-config/.planning/phases/09-upload-credentials-assist/09-02-PLAN.md:390)). A GitHub session normally belongs to `github.com`, not `ssh.github.com` or `personal.github.com`. Valid identities can therefore appear unauthenticated. The provider mapper should return both provider and canonical auth host.

- **MEDIUM — `e2eEnv` does not validate its caller prefixes.** The constructor puts arbitrary caller-provided directories before the deny shims ([09-02-PLAN.md](/Users/ramon/git/personal/ssh-git-config/.planning/phases/09-upload-credentials-assist/09-02-PLAN.md:561)). The source guard proves callers use `e2eEnv`, but not that they avoid passing an ambient/system directory as a prefix. A future call such as `e2eEnv(t, home, os.Getenv("PATH"))` could bypass the deny layer while satisfying the source scan.

- **MEDIUM — "final sweep always runs" is not fully established for early failures.** The plan registers `t.Cleanup`, but the described final sweep remains in the main test body. A fatal assertion before that point invokes cleanup later but may bypass the read-only final sweep. Cleanup should itself end with the sweep, or both explicit and deferred paths should call one idempotent cleanup-and-sweep coordinator.

- **LOW — Plan 09-02 is very large.** It combines the tracer, a suite-wide environment migration, full shim expansion, and PTY proof. The two-commit boundary helps, but execution risk remains high because the plan estimates 145k tokens and touches more than twenty files.

## Suggestions

- Change real-account Phase P to run a headless compiled create/write verb with autonomous upload enabled. Use a disposable identity and temporary HOME, then assert both registrations and output. Keep `register-key` as a separate compiled smoke test if desired, but do not use it as UP-03's only product evidence.
- Capture the original GitHub CLI config location before sandboxing HOME and pass it read-only to Phase P, for example via an explicitly resolved `GH_CONFIG_DIR`. Add a preflight proving the child `gh` invoked by the compiled binary sees the same authenticated account.
- Make provider resolution return a structured value such as `{Provider, Tool, AuthHostname}`. Map every GitHub host/alias to `AuthHostname: "github.com"` and every GitLab host/alias to `"gitlab.com"`.
- Have `e2eEnv` resolve `gh` and `glab` after composing PATH and fail immediately unless each resolved binary lies inside the deny directory or an explicitly registered fake-provider directory.
- Move the final inventory sweep into the shared idempotent cleanup coordinator so it runs from both the normal path and `t.Cleanup`.
- Consider splitting 09-02 into a tracer plan and a hermetic-harness migration plan, with the harness migration executed first.

## Risk Assessment

**Overall risk: HIGH.**

The cycle-1 findings are substantially and concretely repaired. The remaining HIGH rating comes from fresh execution-path issues: the required real-account test does not exercise the autonomous trigger it is meant to validate, its temporary HOME may make real `gh` authentication unavailable, and alias hostnames can be passed incorrectly to `gh auth status --hostname`. These are correctable without redesigning the phase, but they should be fixed before execution or convergence is not yet achieved.

---

## OpenCode Review (xai-grok)

Verifying cycle-1 fixes against current PLAN.md files and the live codebase.
# Phase 9 Plan Review — Cycle 2

Verified against live tree: `e2e/create_flow_pty_e2e_test.go:59-64` still prepends fake SSH onto ambient PATH; `os.Environ()` env construction remains at the sites 09-02 lists. Cycle-1 HIGH is **not yet code**, but the **revised plans specify a buildable suite-wide fix**. Residual risk is execution completeness (missed env sites, source-scan quality), not missing design.

---

## 1. Summary

The revision made cycle-1 findings **executable**: 09-02 Task 2 specifies `ProviderDenyDir` + `e2eEnv` PATH order, two guard tests, and `TestCreateFlow_ExistingPTYCannotReachRealProviderCLI`; 09-03 specifies `RegistrationRequest`, content-based `requirePublicKey`, and `DeleteRecordedKey`; 09-05 pins `register-key` exit codes; 09-08 specifies compiled-binary Phase P, policy-title Phase N, exact scope parse, outstanding-ID cleanup, and UP-03 gated on EXECUTED. Live code still leaks PATH (`create_flow_pty_e2e_test.go:61-64`). New issues: 09-02 `files_modified` omits some e2e files that assign `cmd.Env`; 09-08 Phase P proves `register-key`, not create-time autonomy; 09-05 must_haves still claim dry-run "executes no provider command." Overall risk **MEDIUM** (was HIGH): blocking design is in the plans; remaining risk is executor miss + 09-08 product-path vs create autonomy.

---

## 2. Cycle-1 Finding Verification

| # | Finding | Status | Evidence |
|---|---------|--------|----------|
| 1 | **HIGH — ambient PATH leak** | **RESOLVED in plan** (not in code yet) | `09-02-PLAN.md:548-619` — `ProviderDenyDir`, `e2eEnv` PATH = prefixes + deny + ambient; migrate listed sites; `TestE2EEnvHidesRealProviderCLIs` (`:595-603`); `TestEveryE2EChildEnvIsHermetic` source scan with file:line fail (`:604-618`); `TestCreateFlow_ExistingPTYCannotReachRealProviderCLI` (`09-02-PLAN.md:761-775`). Guard would catch a missed `cmd.Env = append(os.Environ()…)` **if** the scan treats that assignment as env construction. Live leak still `e2e/create_flow_pty_e2e_test.go:59-64`. |
| 2 | **MEDIUM — `UploadKeys` single title** | **RESOLVED** | `09-03-PLAN.md:138-139,170-191` — `RegistrationRequest{Registration, Title}`; `UploadKeys(..., []RegistrationRequest, ...)`; `TestUploadKeysHonoursPerRegistrationTitles`; reflect signature check (`09-03-PLAN.md` acceptance, R4). |
| 3 | **MEDIUM — ID not returned by `ssh-key add`** | **RESOLVED** | `09-08-PLAN.md:323-332` — inventory lookup after add; checkpoint text (`09-08-PLAN.md` Task 1) forbids "ID GitHub returns." |
| 4 | **HIGH — 09-08 not compiled product path** | **RESOLVED** (see residual) | `09-08-PLAN.md:306-321` Phase P: `BuildBinary` + `gitid register-key`; source check scoped to Phase P helper; Phase N separate (`:334-345`). |
| 5 | **Scope verification underspecified** | **RESOLVED** | `09-08-PLAN.md:272-285` — `gh auth status --hostname github.com`, `Token scopes:` → set, exact `admin:public_key` and `admin:ssh_signing_key`; table test (`:403`). |
| 6 | **`.pub` suffix ≠ public key** | **RESOLVED** | `09-03-PLAN.md:140,198-208` — suffix filter + `Deps.ReadFile` + PEM reject + `ssh.ParseAuthorizedKey`; `TestUploadKeyRefusesAPrivateKeyRenamedAsPub`. |
| 7 | **`t.Cleanup` double-delete** | **RESOLVED** | `09-08-PLAN.md:352-368` outstanding map; fake-deleter unit test (`:404`). |
| 8 | **Blocked run must not close UP-03** | **RESOLVED** | `09-08-PLAN.md` Task 3 EXECUTED/SKIPPED/BLOCKED; mechanical read-back of checkbox vs blocker. |

**Also requested:**

- **`register-key` vs D-03:** **RESOLVED** — `09-05-PLAN.md:36` and Task 2 behavior: write verbs stay exit 0; `register-key` non-zero only if every *attempted* registration failed; `TestRegisterKeyExitContract`.
- **`ProviderForHostname` / async eligibility:** **RESOLVED** — `09-02-PLAN.md` Task 1: DNS-label equality + `.<domain>` suffix; `UploadEligibility` → `tea.Cmd`; memo per provider key; `TestUploaderDepsRunCmdIsTimeBounded`. Live `internal/uploader` still has first-found `Detect` until 09-02 runs.

---

## 3. Strengths

- Deny-shim ahead of ambient PATH (not wholesale PATH wipe) matches `FakeGitShimDir` + real `git` (`e2e/harness_test.go` pattern).
- Three-layer hermetic proof: runtime resolve, source scan, pre-existing PTY control.
- Engine vs product split in 09-08 is honest about D-07 vs ONESHOT naming.
- Content-based pub-key check matches `internal/keygen` authorized-key parse.
- Shared `runUploadFor` + copy-freeze + never-gates auto-advance stay coherent across waves.

---

## 4. Concerns

- **MEDIUM — 09-02 file list incomplete vs live env sites.** Task 2 `files_modified` (`09-02-PLAN.md:520`) omits `e2e/global_ssh_storage_pty_e2e_test.go` (uses `newRealCreateFlowCmd`) and `e2e/install_e2e_test.go` (`cmd.Env = os.Environ()` at `:40`). Install is allowlisted; storage PTY is covered **only if** `newRealCreateFlowCmd` is migrated. Executor grepping only `files_modified` could miss `global_ssh_pty_e2e_test.go`. The source-scan test is the backstop — it must not be implemented as a hardcoded filename list.

- **MEDIUM — Phase P ≠ create-time autonomy.** `09-08-PLAN.md:306-311` creates with upload **suppressed**, then `register-key`. That is the compiled CLI path, not "Enter on create wizard / `gitid create` auto-upload." UP-03 wording is create-time autonomy. Residual coverage gap: real-account proof of **default** upload on create is still shim-only (09-02/09-05 e2e).

- **LOW — 09-05 must_haves vs R11.** `09-05-PLAN.md:34` still says dry-run "executes no provider command at all"; Task 2/3 correctly allow auth/inventory reads (`:91-93` R11). Executor could implement the must_have and fail R11 tests.

- **LOW — Deny shim vs "disabled".** `09-02-PLAN.md:576-579` says deny ⇒ "tool present, not authenticated." That is `Unauth`, not `Disabled`. 09-07 documents this (`TestCreateFlow_UploadCheckboxDisabledState`); keep 09-02 SUMMARY from claiming Disabled coverage via deny.

- **LOW — Source-scan fragility.** `TestEveryE2EChildEnvIsHermetic` must treat `cmd.Env = env` where `env` is not an `e2eEnv(...)` call as a fail. A naive `grep e2eEnv` would not catch `create_flow_pty_e2e_test.go:61`. Plan says fail with file:line (`:607-608`); executor must implement assignment analysis, not string presence.

---

## 5. Suggestions

- Add `e2e/global_ssh_storage_pty_e2e_test.go` and `e2e/global_ssh_pty_e2e_test.go` to 09-02 Task 2 files (or state "all `newRealCreateFlowCmd` callers inherit via the helper").
- Align 09-05 must_haves dry-run sentence with R11.
- In 09-08 SUMMARY, state UP-03 real-account evidence is **register-key product path**, not create-default autonomy; optional extra Phase P2 `gitid create` without `--no-upload` if that is required to close UP-03 strictly.
- Implement hermetic scan on `Cmd.Env` assignments (go/ast), not comment/string filtered grep only.

---

## 6. Risk Assessment

**MEDIUM**

Cycle-1 HIGH is specified with constructors, tests, and a failure mode that would catch a missed `cmd.Env` **if implemented as specified**. Code still leaks PATH today. Remaining MEDIUM items are list completeness, create-vs-register-key evidence, and scan implementation quality — not missing threat models. Do not raise overall HIGH unless the executor ships 09-02 without the source-scan test actually failing on `append(os.Environ(), … PATH=…)`.

---
