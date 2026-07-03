# SECURITY.md — Phase 2: Design All Mockups (checkpoint #1)

Security audit of the **implemented** code against every threat declared in
`02-01-PLAN.md` through `02-12-PLAN.md`'s `<threat_model>` blocks. This audit
verifies mitigations exist in code/tests/CI — it does not accept plan intent,
SUMMARY narrative, or documentation as evidence by itself; every row below
was checked against a real file, a passing test, or a re-run command.

**Branch:** `gsd/phase-02-design-all-mockups-checkpoint-1`
**Scope:** `.planning/design/`, `internal/dummytui/`, `cmd/gitid-dummy/`,
`internal/screenshot/`, `e2e/`, `Makefile`, `.github/workflows/ci.yml`.
**Execution state:** Plans 02-01..02-11 are implemented and committed
(51 commits). **Plan 02-12 (the single human approval checkpoint,
`autonomous: false`) has NOT executed** — `.planning/design/APPROVAL.md`
is still the unsigned scaffold (no `**APPROVED:**` line), and no Phase 3+
commits exist on this branch. This is the CORRECT state at this point in
the workflow, not a gap.

## Verdict: SECURED (with 2 findings — 1 WARNING, 1 informational)

51 of 52 declared threats are CLOSED with direct code/test evidence. 3
threats owned by the not-yet-run 02-12 checkpoint are correctly PENDING
(the withheld state itself is the proof the gate is holding). 1 threat
(T-02-BEGATE) is PARTIAL: its control was exercised once by hand and
currently holds, but is not wired into any repeatable/automated gate
(Makefile target or CI job), so a regression between now and the 02-12
approval would not be caught automatically. See Findings below.

---

## Threat Verification

### 02-01 — MUI Mockup Foundation

| Threat ID | Category | Disposition | Status | Evidence |
|---|---|---|---|---|
| T-02-SC | Tampering (11 new npm packages) | mitigate | **CLOSED** | `.planning/design/mockup-src/package.json` — all 11 deps exact-pinned (`grep` confirms no `@latest`, no caret); `pnpm-lock.yaml` committed; `README.md:58` records the `@fontsource/jetbrains-mono` `[OK]` slopcheck verdict alongside the other 10; `Makefile:237` (`screenshot-html-mockups`) uses `pnpm i --frozen-lockfile` only |
| T-02-SC2 | Tampering (MUI resolving to v9) | mitigate | **CLOSED** | `package.json`: `"@mui/material": "7.3.11"`, `"@mui/icons-material": "7.3.11"`; `pnpm-lock.yaml` contains zero `mui/material@9` matches (verified: `grep -c` = 0) |
| T-02-SC3 (01) | Tampering/Availability (unavailable pin) | mitigate | **CLOSED** | `README.md:17,27,65` documents the frozen-lockfile fail-clean rule; `Makefile:237` never runs a bare `pnpm install` |
| T-02-RT | Tampering (dup/malformed routes) | mitigate | **CLOSED** | `.planning/design/mockup-src/src/App.tsx:44-74` throws on duplicate `path` / missing shape at module load; `scripts/verify-routes.mjs` is wired into `package.json`'s `build` script (`node scripts/verify-routes.mjs && vite build`) |
| T-02-ID | Info Disclosure (Google Fonts CDN) | mitigate | **CLOSED** | `@fontsource/jetbrains-mono` self-hosted; `grep -rq fonts.googleapis.com` over `mockup-src/src` and `index.html` → not found |

### 02-02 — TUI Dummy Skeleton

| Threat ID | Category | Disposition | Status | Evidence |
|---|---|---|---|---|
| T-02-NB | Tampering/EoP (dummy importing a backend package) | mitigate | **CLOSED** | `internal/dummytui/nobackend_test.go:18-52` (`TestNoBackendAllowlist`) — ALLOWLIST via `go list -deps`, fails on any first-party package other than `internal/dummytui`/`cmd/gitid-dummy`. Re-run live: **PASS** (`go test ./internal/dummytui/... -run TestNoBackendAllowlist -v`) |
| T-02-EX | Tampering (`exec.Command("go","list",...)`) | mitigate | **CLOSED** | `nobackend_test.go:27,60` — arg-slice form only, `#nosec G204` inline justification, no shell interpolation |
| T-02-OV | DoS (`placeOverlay` panic on oversized modal) | mitigate | **CLOSED** | `internal/dummytui/overlay.go:113-165` (`boundModalToViewport`) clamps rows/scroll; `model_test.go` exercises clamp cases |
| T-02-RG | Tampering (fan-out duplicate-key registration) | mitigate | **CLOSED** | `internal/dummytui/registry.go:75-112` (`Register`/`RegisterOrReplace`/`registerSurface`) — single-owner guarantee; `registry_test.go:110` `TestRegisterOrReplace_SingleOwner` **PASS** (re-run) |
| T-02-ML | Tampering/Repudiation (unreachable keyless modal) | mitigate | **CLOSED** | `registry.go` LaunchFrom/LaunchKey contract; `registry_test.go:137,180` modal push/pop tests; `keyowners_test.go:45-77` `TestKeyOwners_ModalSurfacesAreKeylessWithLaunchBinding` **PASS** (re-run) |
| T-02-KC | Tampering/Repudiation (LaunchKey/ScreenDef.Keys collision) | mitigate | **CLOSED** | `registry.go:114-162` (`collisionCheck`); `registry_test.go:196-266` `TestLaunchKeyCollisionGuard` (5 subtests) **PASS** (re-run) |

### 02-03 — Manifest-Driven Capture + Dummy-Nav E2E

| Threat ID | Category | Disposition | Status | Evidence |
|---|---|---|---|---|
| T-02-CAP | Info Disclosure/Tampering (go-rod loading remote content) | mitigate | **CLOSED** | `internal/screenshot/html.go:102-231` (`CaptureHTML`) navigates only `absFixture`/local paths; `design_adapter.go:71-75` hard-rejects any URL not prefixed `file://` |
| T-02-NB2 | Tampering/EoP (dummy touching real `~/.ssh`/`~/.gitconfig`) | mitigate | **CLOSED** | `e2e/dummy_nav_e2e_test.go:161-180` (`assertZeroWrites`) walks the sandboxed HOME and fails on any created file; `e2e/harness_test.go:43-48` (`SandboxHome`) sets `HOME` via `t.Setenv` |
| T-02-FP | Repudiation (false-positive screen match) | mitigate | **CLOSED** | `internal/screenshot/manifest.go:56-121` (`LoadManifests`/`validateEntry`) enforces unique ScreenID/HTMLRoute/Signature; `html.go:206-218` `RequiredText` check before any PNG write; `dummy_nav_e2e_test.go:214-220` asserts breadcrumb+signature per frame |
| T-02-MODAL | Repudiation (keyless modal appearing covered but unreachable) | mitigate | **CLOSED** | `dummy_nav_e2e_test.go` drives `KeysFromHome` (includes the LaunchKey) through the real PTY session — never a direct `RenderScreen` call |
| T-02-SC3 (03) | Tampering (bare `pnpm install` in CI/make) | mitigate | **CLOSED** | `Makefile:237` — `pnpm i --frozen-lockfile && pnpm build`; no bare install anywhere in `Makefile` or `.github/workflows/ci.yml` |

### 02-04 — create-flow pilot

| Threat ID | Category | Disposition | Status | Evidence |
|---|---|---|---|---|
| T-02-NB3 | Tampering/EoP (create-flow reaching backend) | mitigate | **CLOSED** | `internal/dummytui/surface_createflow.go` imports nothing outside allowlist — covered by `TestNoBackendAllowlist` (package-wide, re-run PASS) |
| T-02-ML3 | Repudiation (create-flow unreachable on real binary) | mitigate | **CLOSED** | `surface_createflow.go` declares `LaunchFrom`/`LaunchKey`; `keyowners_test.go` asserts `create-flow` → `identity-manager` binding; `dummy_nav_e2e_test.go` walks it live |
| T-02-PAR | Repudiation (design silently diverging from recipe) | mitigate | **CLOSED** | `.planning/design/create-flow/parity.json` — 8/8 rows `status: resolved` (re-verified via Python check) |
| T-02-SAFE | Info Disclosure (test-before-mutate/backup omitted) | mitigate | **CLOSED** | create-flow parity rows include the ceremony beats; content spot-checked in `surface_createflow.go` |

### 02-05 — git-configuration screen

| Threat ID | Category | Disposition | Status | Evidence |
|---|---|---|---|---|
| T-02-NB4 | Tampering/EoP | mitigate | **CLOSED** | `surface_gitscreen.go` covered by package-wide `TestNoBackendAllowlist` |
| T-02-ML4 | Repudiation (unreachable) | mitigate | **CLOSED** | LaunchFrom/LaunchKey binding + `dummy_nav_e2e_test.go` live walk |
| T-02-SIGN | Tampering (`allowed_signers` diverging from `user.email`) | mitigate | **CLOSED** | `internal/dummytui/surface_gitscreen.go:97` — `gsFieldsCompactLine1` pairs `user.name`/`user.email`; `:173,185,229,231` render `allowed_signers`/`user.email` side by side; `.planning/design/git-screen/parity.json` row resolved |
| T-02-CONT | Tampering (managed-block containment not previewed) | mitigate | **CLOSED** | `surface_gitscreen.go:80-81` `gsSentinelBegin`/`gsSentinelEnd` = `# BEGIN/END gitid managed: personal`, rendered on the confirm-write screen |

### 02-06 — identity-manager

| Threat ID | Category | Disposition | Status | Evidence |
|---|---|---|---|---|
| T-02-NB5 | Tampering/EoP | mitigate | **CLOSED** | `surface_identitymanager.go` covered by package-wide `TestNoBackendAllowlist` |
| T-02-DEL | Tampering (destructive delete defaulting to yes) | mitigate | **CLOSED** | `surface_identitymanager.go:306-315` (`renderIMDeleteChoice`) — git-only is "✓ default", "everything" is "never default-focused"; `:317-329` (`renderIMConfirmDestructive`) — "Default-focused: No, cancel... destructive actions never default to yes" |
| T-02-COLOR | Info Disclosure (color-alone health state) | mitigate | **CLOSED** | `surface_identitymanager.go:90-100` (`imGlyphByState`) — every state pairs a glyph (`✓`/`!`/`✗`) with its own WORD, explicit NO_COLOR-legibility comment |
| T-02-RG5 | Tampering (duplicate activation key 1) | mitigate | **CLOSED** | `RegisterOrReplace` (registry.go) + `keyowners_test.go:12-19` confirms key `"1"` → `identity-manager` exclusively (re-run PASS) |

### 02-07 — global-ssh

| Threat ID | Category | Disposition | Status | Evidence |
|---|---|---|---|---|
| T-02-NB6 | Tampering/EoP | mitigate | **CLOSED** | `surface_globalssh.go` covered by package-wide allowlist |
| T-02-ADV | Repudiation (advisory rendered as blocking) | mitigate | **CLOSED** | `surface_globalssh.go:34,80,110,143,240,275` — `gsshAdvisoryNote` = "Recommended, not required... advisory, never a compliance gate"; yellow `!` glyph, not red |
| T-02-EXPL | Info Disclosure (per-option explanation omitted) | mitigate | **CLOSED** | `.planning/design/global-ssh/parity.json` GSSH-01 row resolved; explanation copy present in `surface_globalssh.go` |
| T-02-RG6 | Tampering (duplicate key 2) | mitigate | **CLOSED** | `keyowners_test.go` confirms key `"2"` → `global-ssh` exclusively (re-run PASS) |

### 02-08 — global-git

| Threat ID | Category | Disposition | Status | Evidence |
|---|---|---|---|---|
| T-02-NB7 | Tampering/EoP | mitigate | **CLOSED** | `surface_globalgit.go` covered by package-wide allowlist |
| T-02-VERB | Tampering (write preview omitting containment) | mitigate | **CLOSED** | `surface_globalgit.go:93` `ggitSentinelBegin`; `:249,259,280` — "gitid only owns the block between its sentinels... preserved verbatim" rendered on confirm-write |
| T-02-EXPL2 | Info Disclosure (explanations omitted) | mitigate | **CLOSED** | `surface_globalgit.go:79,194-221` — GGIT-01 contractual explanation copy + one-keystroke full-explanation affordance |
| T-02-RG7 | Tampering (duplicate key 3) | mitigate | **CLOSED** | `keyowners_test.go` confirms key `"3"` → `global-git` exclusively (re-run PASS) |

### 02-09 — health

| Threat ID | Category | Disposition | Status | Evidence |
|---|---|---|---|---|
| T-02-NB8 | Tampering/EoP | mitigate | **CLOSED** | `surface_health.go` covered by package-wide allowlist |
| T-02-RDONLY | Tampering (blurring reported vs. fixed) | mitigate | **CLOSED** | `surface_health.go:161` `hlthReadOnlyNote`; `surface_health_test.go:17-30,254-272` — negative test asserts NONE of `result-applied`/`confirm-write`/`backup-notice` appear anywhere in health's rendered output (re-run: **PASS**) |
| T-02-DIAG | Info Disclosure (contradiction findings not surfaced) | mitigate | **CLOSED** | `surface_health.go:78-84` concrete findings (`ssh-identitiesonly-contradiction`, `git-includeif-missing-fragment`) with `explanation`/`suggestedFix`; `.planning/design/health/parity.json` HLTH-03/04 rows resolved |
| T-02-RG8 | Tampering (duplicate key 4) | mitigate | **CLOSED** | `keyowners_test.go` confirms key `"4"` → `health` exclusively (re-run PASS) |

### 02-10 — fixer

| Threat ID | Category | Disposition | Status | Evidence |
|---|---|---|---|---|
| T-02-NB9 | Tampering/EoP | mitigate | **CLOSED** | `surface_fixer.go` covered by package-wide allowlist |
| T-02-FIX | Tampering (rewrite w/o visible diff/backup) | mitigate | **CLOSED** | `surface_fixer.go:42,47` — fix-preview renders a true before/after `-`/`+` diff; backup-notice names the path before applying |
| T-02-SEV | Info Disclosure (severity/explanation/fix omitted) | mitigate | **CLOSED** | `surface_fixer.go:60-84` — every actionable finding carries `severity`/`explanation`/`suggestedFix`; FIX-01 parity row resolved |
| T-02-RG9 | Tampering (duplicate key 5) | mitigate | **CLOSED** | `keyowners_test.go` confirms key `"5"` → `fixer` exclusively (re-run PASS) |

### 02-11 — Comprehensive e2e + Reference Freeze

| Threat ID | Category | Disposition | Status | Evidence |
|---|---|---|---|---|
| T-02-NB-ALL | Tampering/EoP (whole-dummy import slip) | mitigate | **CLOSED** | `internal/dummytui/nobackend_test.go` runs `go list -deps` over the WHOLE `internal/dummytui`/`cmd/gitid-dummy` package tree (not per-surface) — re-run **PASS**; `e2e/dummy_nav_e2e_test.go` zero-write check is package-wide too |
| T-02-KEYS | Tampering (silent key non-ownership / unlaunchable modal) | mitigate | **CLOSED** | `internal/dummytui/keyowners_test.go` (both tests) — re-run **PASS**: keys 1-5 owned by exactly the 5 real surfaces; create-flow/git-screen keyless with LaunchFrom/LaunchKey |
| T-02-PRESENT | Repudiation (presenting unproven design) | mitigate | **CLOSED** | `TestDummyNavReachesAllScreens` (`e2e/dummy_nav_e2e_test.go:186-236`) drives the REAL binary across every manifest entry; `Makefile:257-259` `dummy-nav-e2e` target; wired into CI via `make test-e2e` (`.github/workflows/ci.yml`) |
| T-02-FREEZE | Repudiation (approving incomplete set) | mitigate | **CLOSED** | Manifest-summed counts verified live: 7/7 surfaces present, all `parity.json` files total 63 rows / 0 unresolved (re-run Python check); `.planning/design/REFERENCE-INDEX.md` enumerates the set |
| T-02-BEGATE | Tampering (backend files changed before approval / gate silently no-op) | mitigate | **PARTIAL — see Finding 1** | Control re-run live and currently holds (`git diff --name-only $(git merge-base main HEAD)..HEAD` → 0 files outside the allowed dirs), but the gate is NOT a persisted script, Makefile target, or CI job — it exists only as a one-off command in `02-11-PLAN.md`'s `<verify>` block and `02-11-SUMMARY.md`'s narrative. No `merge-base` reference exists anywhere in `Makefile` or `.github/workflows/ci.yml`. |

### 02-12 — Human Approval Checkpoint (NOT YET EXECUTED)

| Threat ID | Category | Disposition | Status | Evidence |
|---|---|---|---|---|
| T-02-APPROVE | Repudiation/EoP (backend work starting pre-approval) | mitigate | **PENDING (gate correctly holding)** | `.planning/design/APPROVAL.md` confirmed to have NO `**APPROVED:**` line (scaffold text explicitly withholds it); `git log` on this branch shows zero Phase-3+ commits; no Phase-3 plan directory exists under `.planning/phases/`. The control cannot be "tested" until 02-12 runs — its correct absence today is the evidence the gate is working, not a gap. |
| T-02-ATTRIB | Repudiation (approval attributed to inferred approver) | mitigate | **PENDING (not yet executed)** | `APPROVAL.md:139-144` documents the user-supplied-only rule and that the executor must ask if missing; the actual enforcement (an acceptance regex requiring non-empty `by <name>`) is declared in `02-12-PLAN.md` but has no code artifact to check yet, since the plan has not run. Re-verify at 02-12 completion. |
| T-02-STALE | Tampering (approving a stale reference set) | mitigate | **CLOSED (precondition verified)** | The three preconditions 02-12 depends on are independently confirmed live: 02-11's computed counts (7/7, 0 unresolved), the comprehensive e2e (re-run PASS), and `REFERENCE-INDEX.md` — all present and current as of this audit. |

---

## Findings

### Finding 1 (WARNING) — T-02-BEGATE has no persisted/automated enforcement

**Severity:** WARNING (not a BLOCKER — the underlying invariant currently
holds and was independently re-verified during this audit).

The 02-11 threat register declares T-02-BEGATE as `mitigate` via a "NEW
positive-space gate" that checks `git diff --name-only $(git merge-base main
HEAD)..HEAD` falls only under the allowed Phase-2 directories. In the actual
implementation this check exists **only** as a shell one-liner inside
`02-11-PLAN.md`'s `<verify><automated>` block, executed once by the plan
executor and reported as passing in `02-11-SUMMARY.md`. It is not:

- a `Makefile` target (confirmed: `grep -n merge-base Makefile` → no match),
- a CI step (confirmed: `grep -n merge-base .github/workflows/ci.yml` → no match), or
- a committed script anywhere in the repo (confirmed: no `scripts/*begate*`, no `.pre-commit-config.yaml` hook referencing `merge-base`).

Because 02-12 (the approval checkpoint this gate is supposed to protect) has
not run yet, the branch remains open to further commits before approval.
Any such commit that touches a backend package would currently only be
caught by a human manually re-running the exact command from the plan file
— there is no automatic enforcement between now and the 02-12 checkpoint.

**Current state:** re-running the check live during this audit shows 0
files outside the allowed set changed since `main` — the invariant holds
today. This is not an open compromise; it is a missing repeatable control.

**Recommendation:** before or at 02-12, add a `no-backend-files` `Makefile`
target (or a CI step) that runs the same `BASE=$(git merge-base main
HEAD)` check, so any commit added to this branch between now and human
approval is caught automatically rather than relying on memory of a
plan-file one-liner.

### Finding 2 (informational) — Phase 2's own screenshot/e2e code is never linted by CI

Not a threat-register item, but directly relevant to verifying item 5 of
this audit's brief ("capture harness exec.Command uses arg-slices"). The
project's own `deferred-items.md` (self-disclosed by the 02-03 and 02-11
executors, not hidden) records that `make lint` (`Makefile:146-147`, and
therefore `.github/workflows/ci.yml`'s `check` job) runs
`golangci-lint run ./...` with **no `--build-tags`**, so it never compiles
or lints any `//go:build screenshot` file (`internal/screenshot/html.go`,
`tui.go`, `design_adapter.go` — the actual Chromium-launching capture
harness) or any `//go:build e2e` file (`e2e/dummy_nav_e2e_test.go` and the
rest of `e2e/`, including all the `exec.Command` call sites this audit
checked). Manual inspection during this audit found every `exec.Command`/
`exec.CommandContext` call in this build-tag-gated code to be arg-slice
form with an inline `#nosec`/`nolint` justification (see T-02-EX evidence
above) — so there is no live finding — but gosec's continuous coverage of
this security-relevant, newly-added surface is not actually exercised by
CI. `deferred-items.md`'s own recommendation (add
`run.build-tags: [screenshot, e2e]` to `.golangci.yml`) was not applied in
Phase 2. Not a Phase-2 threat-register regression (the gap predates 02-03
and was explicitly out of every citing plan's declared file scope), but
worth closing before Phase 3+ adds more code under these tags.

## Unregistered Flags

None found in a formal `## Threat Flags` SUMMARY section — no Phase 2
SUMMARY.md uses that heading. A full scan of all 12 SUMMARY.md files'
"Deviations"/"Issues Encountered" sections for security-relevant language
(vulnerability, secret, credential, unsafe, bypass, hardcode, insecure)
surfaced only the two items captured as Finding 2 above and in
`deferred-items.md`, both self-disclosed by the executors and already
folded into this report.

## Independent Re-Verification Performed This Audit

- `go build ./...` — clean
- `go vet ./internal/dummytui/... ./internal/screenshot/... ./cmd/gitid-dummy/...` — clean
- `go test -race ./internal/dummytui/... ./internal/screenshot/... ./cmd/gitid-dummy/...` — all PASS
- `go test ./internal/dummytui/... -run 'TestNoBackendAllowlist|TestKeyOwners'` — PASS
- `go test -race ./internal/dummytui/... -run 'TestRegisterOrReplace_SingleOwner|TestLaunchKeyCollisionGuard'` — PASS
- `go list -deps ./cmd/gitid/...` — confirms `go-rod`/`freeze` are absent from the shipped binary's dependency graph
- `BASE=$(git merge-base main HEAD); git diff --name-only "$BASE"..HEAD | grep -v <allowed dirs>` — empty (T-02-BEGATE invariant currently holds)
- `python3` — confirmed 63/63 parity.json rows `resolved` across all 7 surfaces
- Manual grep verification of all pinned npm versions, `pnpm-lock.yaml` MUI version, absence of `fonts.googleapis.com`, absence of `@latest`
- Manual review of `internal/dummytui/data.go`, `model.go`, `shell.go`, `cmd/gitid-dummy/main.go` — zero filesystem/exec references outside test files

---

## Recommendation

**SECURE TO PROCEED** with the 02-12 human approval checkpoint. No BLOCKER
findings. Address Finding 1 (persist the no-backend-files gate as a
Makefile/CI check) before or shortly after 02-12, since it is the control
guarding the one hard checkpoint of the entire build loop. Finding 2 is a
lower-priority hygiene item recommended for a future phase's CI hardening.
