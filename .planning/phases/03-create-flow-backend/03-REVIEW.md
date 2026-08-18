---
phase: 03-create-flow-backend
reviewed: 2026-08-18T15:41:08Z
depth: deep
files_reviewed: 133
files_reviewed_list:
  - Makefile
  - cmd/gitid-dummy/main.go
  - cmd/gitid-dummy/main_test.go
  - cmd/gitid/add.go
  - cmd/gitid/add_test.go
  - cmd/gitid/addrepo.go
  - cmd/gitid/addrepo_test.go
  - cmd/gitid/adopt.go
  - cmd/gitid/adopt_test.go
  - cmd/gitid/baseline.go
  - cmd/gitid/baseline_test.go
  - cmd/gitid/copy.go
  - cmd/gitid/copy_test.go
  - cmd/gitid/debug.go
  - cmd/gitid/delete.go
  - cmd/gitid/delete_test.go
  - cmd/gitid/doctor.go
  - cmd/gitid/doctor_agent_test.go
  - cmd/gitid/doctor_realwiring_test.go
  - cmd/gitid/doctor_test.go
  - cmd/gitid/gate_visual_regression_test.go
  - cmd/gitid/list.go
  - cmd/gitid/list_test.go
  - cmd/gitid/main.go
  - cmd/gitid/main_test.go
  - cmd/gitid/match.go
  - cmd/gitid/match_test.go
  - cmd/gitid/rotate.go
  - cmd/gitid/rotate_test.go
  - cmd/gitid/smoke_network_test.go
  - cmd/gitid/test.go
  - cmd/gitid/test_test.go
  - cmd/gitid/update.go
  - cmd/gitid/update_test.go
  - cmd/gitid/upload.go
  - cmd/gitid/wiring.go
  - cmd/gitid/wiring_test.go
  - e2e/addrepo_e2e_test.go
  - e2e/adopt_e2e_test.go
  - e2e/create_e2e_test.go
  - e2e/create_flow_pty_e2e_test.go
  - e2e/harness_test.go
  - e2e/install_e2e_test.go
  - e2e/match_e2e_test.go
  - e2e/overlap_e2e_test.go
  - e2e/ui_pty_e2e_test.go
  - e2e/upload_e2e_test.go
  - internal/doctor/checks/reserved_test.go
  - internal/dummytui/data.go
  - internal/dummytui/data_test.go
  - internal/dummytui/fixturebackend.go
  - internal/dummytui/nobackend_test.go
  - internal/identity/identity.go
  - internal/identity/modes.go
  - internal/identity/modes_test.go
  - internal/keygen/keyscan.go
  - internal/keygen/keyscan_test.go
  - internal/repoclone/repoclone.go
  - internal/repoclone/repoclone_stub_test.go
  - internal/repoclone/repoclone_test.go
  - internal/screenshot/createflow.go
  - internal/sshconfig/include.go
  - internal/sshconfig/include_test.go
  - internal/sshconfig/reader.go
  - internal/sshconfig/reader_test.go
  - internal/tester/tester.go
  - internal/tester/tester_command_test.go
  - internal/tuikit/app.go
  - internal/tuikit/app_test.go
  - internal/tuikit/backend.go
  - internal/tuikit/backend_stub_test.go
  - internal/tuikit/batch3_test.go
  - internal/tuikit/ceremony.go
  - internal/tuikit/ceremony_test.go
  - internal/tuikit/design.go
  - internal/tuikit/doc.go
  - internal/tuikit/doctor.go
  - internal/tuikit/doctor_test.go
  - internal/tuikit/fixplans.go
  - internal/tuikit/fixplans_test.go
  - internal/tuikit/frame.go
  - internal/tuikit/frame_test.go
  - internal/tuikit/globalgit.go
  - internal/tuikit/globalgit_test.go
  - internal/tuikit/globalssh.go
  - internal/tuikit/globalssh_test.go
  - internal/tuikit/identities.go
  - internal/tuikit/identities_test.go
  - internal/tuikit/mouse_test.go
  - internal/tuikit/reviewfix_test.go
  - internal/tuikit/store.go
  - internal/tuikit/store_test.go
  - internal/tuikit/theme.go
  - internal/tuikit/theme_test.go
  - internal/tuikit/views.go
  - tui/addrepo.go
  - tui/addrepo_test.go
  - tui/adopt.go
  - tui/adopt_test.go
  - tui/confirm.go
  - tui/confirm_test.go
  - tui/copy.go
  - tui/copy_test.go
  - tui/deps.go
  - tui/deps_test.go
  - tui/detail.go
  - tui/detail_test.go
  - tui/doc.go
  - tui/globalopts.go
  - tui/globalopts_test.go
  - tui/health.go
  - tui/health_test.go
  - tui/help.go
  - tui/keymap.go
  - tui/messages.go
  - tui/model.go
  - tui/model_test.go
  - tui/overlay.go
  - tui/overlay_test.go
  - tui/palette.go
  - tui/prove_test.go
  - tui/render_bench_test.go
  - tui/scaffold_test.go
  - tui/sidebar.go
  - tui/sidebar_test.go
  - tui/styles.go
  - tui/styles_test.go
  - tui/tui.go
  - tui/tui_stub_test.go
  - tui/upload_test.go
  - tui/wiring_test.go
  - tui/wizard.go
  - tui/wizard_test.go
findings:
  critical: 14
  warning: 4
  info: 0
  total: 18
status: issues_found
---

# Phase 3: Code Review Report

**Reviewed:** 2026-08-18T15:41:08Z
**Depth:** deep
**Files Reviewed:** 133
**Status:** issues_found — **BLOCK**

## Summary

The Phase 3 implementation must not ship. The create ceremony can report success before persistence, mutate user files before confirmation, accept unsafe SSH input, and leave coordinated configuration partially written. Test-stage behavior does not prove the alias resolves through the intended key. The visual gate exempts every captured screen and does not compare against approved screenshot evidence.

Verification completed during review:

- `go test -race ./...` — 838 tests passed across 19 packages.
- `go test -tags e2e -race -timeout 180s ./e2e/...` — 13 tests passed.
- `go test -tags screenshot -run TestGateVisualRegression -v ./cmd/gitid/...` — passed.
- `go vet ./...` — passed.

These results do not clear the review because multiple tests encode the incorrect behavior described below.

## Narrative Findings (AI reviewer)

## Critical Issues

### CR-01 [BLOCKER]: Success is displayed before persistence occurs

**Files:** `internal/tuikit/ceremony.go:137-164`, `internal/tuikit/identities.go:2055-2073`, `cmd/gitid/wiring.go:332-369`

**Issue:** Confirmation only marks the ceremony done. Persistence is delayed until the user dismisses a receipt that already claims files were written. A later persistence failure is stored in `PersistError()` but is not surfaced by the UI.

**Fix:** Dispatch persistence on confirmation, wait for an explicit result message, and show the receipt only after success. Render failure details and keep the ceremony recoverable.

### CR-02 [BLOCKER]: Testing mutates user files before confirmation

**Files:** `cmd/gitid/wiring.go:151-180`, `cmd/gitid/wiring.go:540-551`, `internal/identity/modes.go:84-90`, `internal/tester/tester.go:75-84`

**Issue:** Stage 1 generates keys at final paths; reuse can create a missing `.pub`; and `StrictHostKeyChecking=accept-new` can update the user's `known_hosts`. Canceling the wizard can therefore leave persistent state or replace key material without consent.

**Fix:** Generate and derive keys in a mode-0700 temporary directory, use a temporary `UserKnownHostsFile`, and move validated artifacts to final paths only inside the confirmed ceremony.

### CR-03 [BLOCKER]: Algorithm selection, preview, and persistence disagree

**Files:** `internal/tuikit/identities.go:764-772`, `internal/tuikit/identities.go:924-953`, `internal/tuikit/identities.go:2683-2707`, `cmd/gitid/wiring.go:379-397`, `cmd/gitid/wiring.go:702-711`, `internal/keygen/catalog.go:53-60`, `internal/keygen/registry.go:37-45`

**Issue:** Rendering uses a static catalog while selection indexes the backend-probed catalog. Availability metadata is dropped, disabled detection is note-specific, and the preview path always uses ed25519. Displayed stub algorithms remain selectable and fail later.

**Fix:** Use one backend catalog for rendering and selection, preserve explicit availability/implementation state, disable unavailable entries, and derive the key path from the selected algorithm.

### CR-04 [BLOCKER]: Collision checking validates the identity name instead of the SSH alias

**Files:** `internal/tuikit/identities.go:1075-1088`, `cmd/gitid/wiring.go:452-483`, `cmd/gitid/wiring.go:879-897`

**Issue:** The UI checks a value such as `acme` while persistence writes `acme.github.com`. The backend also limits host discovery to managed blocks, missing hand-written and wildcard Host patterns. This permits first-match-wins ambiguity.

**Fix:** Check the complete alias against all effective Host patterns from the main file and included files using OpenSSH pattern semantics. Fail closed when configuration cannot be parsed.

### CR-05 [BLOCKER]: SSH form values permit malformed or injected configuration

**Files:** `internal/tuikit/identities.go:321-332`, `internal/tuikit/identities.go:1075-1080`, `internal/sshconfig/renderer.go:31-44`

**Issue:** Host and hostname checks only require nonempty values, while the port only requires digits. Control characters, whitespace, wildcard patterns, directive-like input, port 0, and ports above 65535 reach direct string interpolation into SSH config.

**Fix:** Reject control characters and whitespace in host tokens, validate hostnames and aliases against a strict grammar, constrain ports to 1-65535, validate paths, and make rendering return an error for unsafe values.

### CR-06 [BLOCKER]: Stage 2 does not prove the configured alias resolves through the expected key

**Files:** `internal/tester/tester.go:87-105`, `internal/tester/tester.go:168-192`, `cmd/gitid/wiring.go:558-580`, `internal/tuikit/identities.go:2895-2907`

**Issue:** Connectivity pins `-i` despite UI copy saying otherwise. The additional `ssh -G` command is not shown, its error is discarded, and incorrect or empty `IdentityFile`, `IdentitiesOnly`, user, hostname, or port values do not fail the stage.

**Fix:** Display both exact commands. Require a successful `ssh -G` result matching every expected field, and perform alias connectivity without bypassing configuration through `-i`.

### CR-07 [BLOCKER]: Required automatic stage chaining is absent

**Files:** `internal/tuikit/identities.go:1336-1356`, `internal/tuikit/identities.go:1953-1962`, `e2e/create_flow_pty_e2e_test.go:192-218`

**Issue:** A passing/reachable stage 1 stops and requires another Enter, contrary to D-04. The E2E test redefines chaining as a second user action.

**Fix:** Batch or dispatch stage 2 immediately after the accepted stage-1 outcomes and update tests to assert that no intervening input is needed.

### CR-08 [BLOCKER]: Existing-key reuse does not verify key pairing or normalize permissions

**Files:** `internal/keygen/keyscan.go:112-134`, `internal/identity/modes.go:54-91`, `cmd/gitid/wiring.go:182-197`

**Issue:** For encrypted private keys, any adjacent `.pub` is trusted and presented for upload without proving it matches. Reused files bypass permission normalization, violating the matching-key and 0600/0644 requirements.

**Fix:** Parse and canonicalize the public key, verify pairing through a passphrase-aware mechanism or reject unverifiable pairs, and apply previewed permission changes only during confirmed persistence.

### CR-09 [BLOCKER]: Coordinated writes are not transactional

**Files:** `cmd/gitid/wiring.go:813-832`, `cmd/gitid/wiring.go:151-197`, `internal/identity/identity.go:323-349`

**Issue:** Private key, public key, Include line, target config, and Host block writes are separate operations without rollback. A later failure can leave dangling includes, partial identities, or unmatched key files.

**Fix:** Stage all outputs, capture pre-write state, apply a coordinated transaction, and roll back every earlier mutation when any later step fails.

### CR-10 [BLOCKER]: The visual regression gate exempts every captured screen

**Files:** `cmd/gitid/gate_visual_regression_test.go:105-127`, `.planning/design/create-flow/visual-divergence-allowlist.txt:26-27,45,61-65`, `internal/screenshot/createflow.go:42-57`

**Issue:** All eight enumerated screens are allowlisted at whole-screen granularity. Any current or future difference on those screens therefore passes, including unrelated sidebar and header divergence.

**Fix:** Allowlist exact normalized regions or predicates rather than screen IDs. Require meaningful non-allowlisted coverage and reject stale, duplicate, unknown, or unexplained exemptions.

### CR-11 [BLOCKER]: DLV-04 does not compare implementation output with approved screenshots

**Files:** `.planning/REQUIREMENTS.md:45-52`, `.planning/phases/03-create-flow-backend/03-06-PLAN.md:70-76,180-205`, `.planning/phases/03-create-flow-backend/03-06-review-packet/MANIFEST.md:24-45`, `.planning/phases/03-create-flow-backend/03-06-SUMMARY.md:103-123`

**Issue:** The gate compares live real-backend text with freshly generated dummy-backend text, not approved HTML/TUI screenshots. No PNG pairs exist and the required independent Codex review remains pending.

**Fix:** Capture deterministic real-binary PNGs, compare them against the approved reference set, run both required independent reviews, and resolve all blocking visual findings.

### CR-12 [BLOCKER]: The unapproved Provider field remains in the form

**Files:** `.planning/REQUIREMENTS.md:132-137`, `.planning/phases/03-create-flow-backend/03-UI-SPEC.md:148-154`, `internal/tuikit/identities.go:111-118`, `internal/tuikit/identities.go:453-460`

**Issue:** SSHUI-01 and D-20 require provider inference without a second editable field, but Provider is visible and first in tab order. The shared dummy/real implementation prevents the current visual gate from detecting this divergence.

**Fix:** Remove the editable Provider field or obtain explicit scoped design approval and update the requirement before implementation.

### CR-13 [BLOCKER]: Configuration read failures silently become empty healthy state

**Files:** `cmd/gitid/wiring.go:467-483`, `cmd/gitid/wiring.go:838-855`, `cmd/gitid/wiring.go:879-897`

**Issue:** Inventory, managed-block, and Host parse errors are discarded. The UI may show no identities or collisions and then write into a configuration it could not understand.

**Fix:** Propagate initialization and parsing failures into a blocking UI state. Collision and persistence checks must fail closed.

### CR-14 [BLOCKER]: Include detection uses an unsafe path-prefix test

**File:** `cmd/gitid/wiring.go:797-810`

**Issue:** A raw `strings.HasPrefix` treats unrelated paths such as `~/.ssh/config.different/...` as children of `~/.ssh/config.d`. The tool can then omit the real Include line and write an unreachable Host block.

**Fix:** Use exact glob equivalence or path-boundary-aware containment rather than a string prefix.

## Warnings

### WR-01 [WARNING]: The visual gate performs real network operations

**Files:** `internal/screenshot/createflow.go:59-72,197-204`, `cmd/gitid/gate_visual_regression_test.go:92-103`

**Issue:** Capture drains real backend commands without injecting deterministic SSH behavior, making the gate provider/network dependent.

**Fix:** Inject fake SSH effects or a controlled executable through `PATH` for screenshot capture.

### WR-02 [WARNING]: The persistence boundary fails open when no test outcome exists

**File:** `cmd/gitid/wiring.go:1003-1012`

**Issue:** `storeUnlocked()` returns true when `outcomeKnown` is false. Current UI ordering masks this, but the backend does not enforce TEST-03 itself.

**Fix:** Require successful stage-1 and stage-2 records tied to the current create specification.

### WR-03 [WARNING]: Provider extraction assumes a two-label public suffix

**File:** `cmd/gitid/wiring.go:1078-1087`

**Issue:** Enterprise and multi-label domains can produce incorrect provider metadata and same-provider warnings.

**Fix:** Preserve the provider selected/inferred earlier rather than reconstructing it from the final two labels.

### WR-04 [WARNING]: Phase governance artifacts contradict completion claims

**Files:** `.planning/phases/03-create-flow-backend/03-UI-SPEC.md:315-324`, `.planning/phases/03-create-flow-backend/03-VALIDATION.md:1-8,37-41`, `.planning/ROADMAP.md:178-180`, `.planning/REQUIREMENTS.md:447-476`

**Issue:** The UI spec and validation ledger remain draft/pending and non-Nyquist while requirement rows are already marked complete.

**Fix:** Correct traceability only after implementation fixes and independent verification are complete.

---

_Reviewed: 2026-08-18T15:41:08Z_
_Reviewer: the agent (gsd-code-reviewer)_
_Depth: deep_
_Verdict: BLOCK_
