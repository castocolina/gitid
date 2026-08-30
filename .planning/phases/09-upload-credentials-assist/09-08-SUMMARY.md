---
phase: 09-upload-credentials-assist
plan: 08
subsystem: upload-credentials-assist
status: complete
requirements: [UP-01, UP-02, UP-03]
real_account_outcome: EXECUTED
account: castocolina
run_id: 20260830t035232-94f43a07
---

# 09-08 Summary — real-account upload validation and Phase 9 closure

## Task 1 — authorization checkpoint

The developer's verbatim reply was:

`authorized: castocolina`

The checkpoint described four disposable keys across two phases, the Phase P
product-title naming deviation, and resource-ID capture by a run-scoped
inventory lookup after registration. It also stated that no existing key,
real `~/.ssh`, real `~/.gitconfig`, or GitLab resource would be touched.

## Task 2 — executed real-account validation

Outcome: **EXECUTED** against GitHub account `castocolina`.

Observed, redacted `gh auth status --hostname github.com` scope line:

`Token scopes: 'admin:public_key', 'admin:ssh_signing_key', 'gist', 'read:org', 'repo'`

The test parsed that line into an exact-member set and required both
`admin:public_key` and `admin:ssh_signing_key`; no token-revealing flag was
used. `GH_CONFIG_DIR` resolution is unit-tested in this order:
`GH_CONFIG_DIR`, then `$XDG_CONFIG_HOME/gh`, then `$HOME/.config/gh`.

The run used ID `20260830t035232-94f43a07`. Phase P created a disposable
identity in a temporary HOME and ran the compiled `gitid` binary's
`register-key` verb. The D-07 product title carried the identity scope
`gitid-e2e-20260830t035232-94f43a07`. Phase N used a second disposable key
pair and the exact policy titles:

- `gitid-e2e:20260830t035232-94f43a07:authentication`
- `gitid-e2e:20260830t035232-94f43a07:signing`

`gh ssh-key add` does not return a machine-readable resource ID. Each ID below
was therefore resolved by a run-scoped inventory lookup after registration,
never claimed as a value returned by the add call:

| Phase | Registration | Resource ID | Recorded title | Capture and cleanup |
|---|---|---:|---|---|
| P | authentication | 161717246 | `gitid: gitid-e2e-20260830t035232-94f43a07 @ MacBookPro` | Resolved by run-scoped inventory lookup; deleted by recorded ID after title scope re-confirmation. |
| P | signing | 1144250 | `gitid: gitid-e2e-20260830t035232-94f43a07 @ MacBookPro` | Resolved by run-scoped inventory lookup; deleted by recorded ID after title scope re-confirmation. |
| N | authentication | 161717247 | `gitid-e2e:20260830t035232-94f43a07:authentication` | Resolved by exact run-scoped inventory lookup; deleted by recorded ID after title scope re-confirmation. |
| N | signing | 1144251 | `gitid-e2e:20260830t035232-94f43a07:signing` | Resolved by exact run-scoped inventory lookup; deleted by recorded ID after title scope re-confirmation. |

All four registrations succeeded. The test verified both registration kinds in
inventory with `uploader.HasRegistration`, removed each resource exactly once
from the outstanding-ID map after its explicit deletion, and ran the final
read-only sweep last through LIFO `t.Cleanup` ordering. Final sweep output:

`remaining=0 product-scope="gitid-e2e-20260830t035232-94f43a07" policy-prefix="gitid-e2e:20260830t035232-94f43a07:"`

The baseline and final counts of inventory entries carrying neither run scope
were equal. No private key path appeared in any recorded argv; only the public
key paths were passed to the uploader. No GitLab command ran.

The permanent implementation is `e2e/upload_real_account_e2e_test.go`, behind
its separate `realaccount` build tag. `verify-upload-real-account` is opt-in,
listed only in `.PHONY` and its own Makefile definition, and is not a
prerequisite of routine tests, lint, E2E, or CI. `make test-e2e` selects only
the `e2e` tag; the hermetic-environment source guard explicitly excludes this
file because it does not select that tag.

## Task 3 — requirement and roadmap closure

UP-01 and UP-02 are marked complete with wired evidence. UP-03 is marked
complete because the real-account run **EXECUTED**, not skipped or blocked. The
UP-03 read-back has a closed checkbox and no skipped/blocked marker.

The Phase 9 ROADMAP section now records all eight delivered waves, and its
Goal and success criteria remain unchanged.

## Per-Phase Checklist

1. Discuss — `09-CONTEXT.md` exists.
2. UI-phase — `09-UI-SPEC.md` exists for this TUI phase.
3. Plan — `09-01-PLAN.md` through `09-08-PLAN.md` exist and carry `cross_ai: true`.
4. Plan review convergence — `09-REVIEWS.md` records the cross-AI corrections incorporated by this plan.
5. Execute — `09-01-SUMMARY.md` through this summary record the eight waves and independent gate results.
6. Code review — phase review artifacts remain the orchestrator's closeout responsibility.
7. Verify-work — phase verification remains the orchestrator's closeout responsibility.
8. UI review — `09-07-SUMMARY.md` and its UI-frame review packet carry the Phase 9 visual evidence.
9. Audit UAT — remains the orchestrator's required phase-close action.

## Deviations recorded during Phase 9

1. **D-09 step-3/step-5 collapse (09-01):** the two design steps were collapsed because the upload control is a sub-beat of the existing wizard flow; its approved copy and field evidence preserve the intended interaction.
2. **Frozen-label checkbox glyph (09-01):** the checkbox glyph stayed outside frozen label constants so visual state can change without altering locked copy; the frozen text is still asserted by `gate-copy-freeze`.
3. **Tab-ring focus slot (09-02):** the checkbox is a real ordered focus-slice slot rather than an `iota` rebase, preserving existing focus indices and keyboard traversal.
4. **`register-key` verb (09-05):** the shipped `register-key` name replaces archived `gitid copy --upload-keys` wording because the copy command no longer exists; the same shared upload orchestration and documented exit contract preserve the intended manual re-run surface.
5. **Phase P real-account title (09-08):** Phase P scopes its compiled product path through the disposable identity name instead of the literal policy title, because D-07's shipped title is frozen and a test-only override would violate D-06. The run scope remains present in every product title, and Phase N separately preserves the policy's literal title format.

## Decision-level resolutions from review

- **D-11 substring versus D-13 main-domain routing:** host-boundary matching wins, so only the main domain or a real subdomain is eligible; strings that merely contain a provider name do not cross-route.
- **`register-key` exit code:** D-03 applies rather than being excepted. Registration is the verb's primary operation, so all attempted registrations failing produces non-zero; success, partial success, and already-complete outcomes exit zero.

## Gate results (independently re-run at this plan's close)

- `TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./...` — 2353 passed, 22 packages.
- `GOTOOLCHAIN=go1.26.4 make test` (includes `gate-copy-freeze`) — pass; every frozen-copy string found.
- `GOTOOLCHAIN=go1.26.4 make lint` — `lint-tagged` (go vet across screenshot/smoke/e2e/realaccount tags) pass; `golangci-lint run --build-tags screenshot ./internal/screenshot/...` 0 issues; `golangci-lint run ./...` 0 issues.
- `GOTOOLCHAIN=go1.26.4 make test-e2e` — pass, 814.5s (full `-tags e2e -race` suite; the realaccount file is provably excluded).
- `GOTOOLCHAIN=go1.26.4 make gate-visual-regression` — pass, 51 RequiredScreenSpecs frames checked, all Phase 9 negative controls pass.
- `GOTOOLCHAIN=go1.26.4 make gate-copy-freeze` — pass (run standalone in addition to being a `make test` prerequisite).
- `make verify-upload-real-account` — PASS on two independent runs during this dispatch:
  - Run `20260830t035232-94f43a07`: all four registrations succeeded; IDs 161717246/1144250 (Phase P auth/signing) and 161717247/1144251 (Phase N auth/signing) resolved by run-scoped inventory lookup, deleted after scope re-confirmation; final sweep remaining=0.
  - Run `20260830t041912-91d9689b` (final independent re-verification): all four registrations succeeded; IDs 161718510/1144268 (Phase P auth/signing) and 161718512/1144269 (Phase N auth/signing) resolved by run-scoped inventory lookup, deleted after scope re-confirmation; final sweep remaining=0.
  - Both runs confirm the pre-existing inventory count (entries carrying neither run scope) was unchanged before and after.
