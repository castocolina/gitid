---
phase: 02-first-identity-end-to-end
plan: 03
subsystem: crypto
tags: [ed25519, openssh, allowed_signers, clipboard, x-crypto, atotto, tdd, filewriter]

# Dependency graph
requires:
  - phase: 02-01
    provides: filewriter.Write (backup + atomic temp→rename + chmod) and filewriter.ReplaceBlock (idempotent sentinel managed block)
provides:
  - "keygen.Generate: ed25519 keypair → OpenSSH PEM private key (0600) + authorized .pub line (0644) at D-06 path id_<algo>_<identity>"
  - "keygen.AllowedSignersLine: `<email> namespaces=\"git\" ssh-ed25519 …` with byte-identical email (SIGN-01)"
  - "keygen.WriteAllowedSigners: persists the signers line into ~/.ssh/allowed_signers (0644) inside an idempotent per-identity managed block (SAFE-02, the recovered 4th artifact)"
  - "clipboard.Copy: atotto wrapper that wraps ErrNoClipboard for graceful no-tool failure (CLIP-01/CLIP-02)"
affects: [02-04, 02-05, 02-06, 02-07]

# Tech tracking
tech-stack:
  added:
    - golang.org/x/crypto v0.53.0 (ed25519 OpenSSH serialization)
    - github.com/atotto/clipboard v0.1.4 (cross-platform clipboard dispatch)
  patterns:
    - "All keygen/signers file writes delegate to the filewriter chokepoint; zero os.WriteFile"
    - "Package-level function-variable seam (writeAll) for injecting/simulating unavailable backends in tests without touching the OS"
    - "RED stubs return zero values + a sentinel error (not panic) so RED compiles, lint passes, and behavior tests fail on assertions"

key-files:
  created:
    - internal/keygen/keygen.go
    - internal/keygen/keygen_test.go
    - internal/keygen/signers.go
    - internal/keygen/signers_test.go
    - internal/clipboard/clipboard.go
    - internal/clipboard/clipboard_test.go
  modified:
    - internal/keygen/doc.go
    - internal/clipboard/doc.go
    - go.mod
    - go.sum

key-decisions:
  - "RED stubs return zero values + sentinel errNotImplemented rather than panic, so staticcheck/gosec see reachable code and RED still genuinely fails (panic stubs tripped SA4006 'value never used')"
  - "Clipboard no-tool detection keys on atotto's exported clipboard.Unsupported bool (v0.1.4 has no exported sentinel error; WriteAll's missingCommands is unexported); tests set clipboard.Unsupported to simulate the no-tool path"
  - "Generate rejects any algo != ed25519 (fail fast) rather than silently defaulting"
  - "Pass the value from ed25519.GenerateKey directly to ssh.MarshalPrivateKey (value works at v0.53.0 per RESEARCH Pitfall 10)"

patterns-established:
  - "filewriter chokepoint: keygen private key 0600, .pub 0644, allowed_signers 0644 all via filewriter.Write; allowed_signers per-identity block via filewriter.ReplaceBlock"
  - "writeAll function-variable seam for clipboard backend injection in unit tests"

requirements-completed: [IDENT-01, KEY-02, SIGN-01, CLIP-01, CLIP-02]

# Metrics
duration: 7min
completed: 2026-06-09
---

# Phase 2 Plan 3: keygen + clipboard Summary

**ed25519 OpenSSH keygen (0600 key / 0644 .pub via filewriter), the allowed_signers line + idempotent per-identity file write as the 4th coordinated artifact, and an atotto clipboard wrapper with graceful no-tool failure — all TDD RED→GREEN.**

## Performance

- **Duration:** 7 min
- **Started:** 2026-06-09T18:03:30Z
- **Completed:** 2026-06-09T18:10:01Z
- **Tasks:** 2 (3 if counting the pre-approved dependency checkpoint)
- **Files modified:** 10

## Accomplishments
- `keygen.Generate` produces a valid OpenSSH private key (BEGIN OPENSSH PRIVATE KEY, 0600) and authorized `.pub` line (0644) at the D-06 path `id_<algo>_<identity>`, with an encrypted-key path when a passphrase is supplied (IDENT-01, KEY-02).
- `keygen.AllowedSignersLine` emits the SIGN-01 line with a byte-identical email, mandatory `namespaces="git"`, and exactly one trailing newline.
- `keygen.WriteAllowedSigners` persists that line into `~/.ssh/allowed_signers` (0644) inside an idempotent per-identity managed block — re-runs produce an empty diff and a second identity appends a distinct block while preserving foreign content (SAFE-02). This is the recovered 4th coordinated artifact.
- `clipboard.Copy` wraps atotto/clipboard and wraps `ErrNoClipboard` when no clipboard tool is available, so the create-new flow can print the key for manual copy instead of crashing (CLIP-01/CLIP-02).
- Added and pinned `golang.org/x/crypto v0.53.0` and `github.com/atotto/clipboard v0.1.4` (both now direct deps; go.sum checksummed).

## Task Commits

Each task was committed atomically (TDD RED→GREEN):

1. **Task 1 RED: failing keygen + allowed_signers tests** - `5ec438c` (test)
2. **Task 1 GREEN: ed25519 keygen + allowed_signers file write** - `26635f9` (feat)
3. **Task 2 RED: failing clipboard Copy tests** - `8a89fb0` (test)
4. **Task 2 GREEN: clipboard Copy with graceful no-tool failure** - `fb39642` (feat)

**Plan metadata:** committed separately (docs: complete plan)

_No standalone REFACTOR commits were needed; GREEN implementations were already clean._

## Files Created/Modified
- `internal/keygen/keygen.go` - ed25519 Generate + OpenSSH serialize; writes via filewriter (0600/0644)
- `internal/keygen/signers.go` - AllowedSignersLine + WriteAllowedSigners (idempotent managed-block file write, 0644)
- `internal/keygen/keygen_test.go` - PEM header, .pub prefix, modes 0600/0644, passphrase form
- `internal/keygen/signers_test.go` - signers-line byte-match + idempotent/multi-identity/foreign-preserving file-write + backup-on-preexist
- `internal/keygen/doc.go` - updated to reflect the implemented contract (Generate, AllowedSignersLine, WriteAllowedSigners; filewriter delegation)
- `internal/clipboard/clipboard.go` - Copy via atotto with ErrNoClipboard graceful failure; writeAll injection seam
- `internal/clipboard/clipboard_test.go` - available/unavailable/other-error paths via writeAll + clipboard.Unsupported
- `internal/clipboard/doc.go` - clipboard pulled forward to Phase 2 (Phase 5+ marker removed)
- `go.mod` / `go.sum` - x/crypto v0.53.0 + atotto/clipboard v0.1.4 (direct, checksummed)

## Decisions Made
- **RED stubs return zero values + sentinel `errNotImplemented` instead of `panic`.** Panic stubs made staticcheck flag `SA4006: value never used` (downstream code unreachable) and blocked the lint-gated RED commit. Zero-value returns keep code reachable so RED fails on assertions, lint passes, and the hook (no `--no-verify`) is satisfied.
- **Clipboard no-tool detection keys on `clipboard.Unsupported` (exported bool).** atotto v0.1.4 has no exported sentinel error; its `missingCommands` is unexported. Tests set `clipboard.Unsupported = true` and inject `writeAll` to simulate the no-tool path without uninstalling OS tools.
- **`Generate` rejects any algo other than ed25519** (fail fast) rather than silently defaulting.
- **Value from `ed25519.GenerateKey` passed directly to `ssh.MarshalPrivateKey`** (value works at v0.53.0; RESEARCH Pitfall 10).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] RED stubs changed from panic to zero-value+sentinel-error**
- **Found during:** Task 1 (first RED commit attempt)
- **Issue:** The runtime-note RED convention uses `panic("not implemented")`, but the pre-commit `make lint` hook (staticcheck) rejected it with `SA4006: this value of path is never used` because the panicking call made subsequent statements unreachable, blocking the lint-gated RED commit (no `--no-verify` allowed).
- **Fix:** RED stubs return zero values plus a sentinel `errNotImplemented`; behavior tests still fail genuinely (on assertions / sentinel error) while lint passes and the code stays reachable.
- **Files modified:** internal/keygen/keygen.go, internal/keygen/signers.go (RED commit `5ec438c`)
- **Verification:** RED `go test` fails on assertions; `make lint` reports 0 issues.
- **Committed in:** `5ec438c` (Task 1 RED commit)

**2. [Rule 1 - Bug] Test fixtures + comment adjusted to satisfy gosec/revive**
- **Found during:** Tasks 1 and 2 (RED commits)
- **Issue:** Test seed writes used 0644 (gosec G306), test ReadFile flagged G304, a test error string was capitalized (revive), and a code comment containing the literal `os.WriteFile` would have tripped the plan's `grep` guard / read as a direct-write.
- **Fix:** Seed writes set to 0600 with `//nolint:gosec` fixture annotations, ReadFile annotated, error string lowercased, and the keygen comment reworded to not contain `os.WriteFile`.
- **Files modified:** internal/keygen/keygen_test.go, internal/keygen/signers_test.go, internal/keygen/keygen.go, internal/clipboard/clipboard_test.go
- **Verification:** `make lint` 0 issues; `grep -v '^//' … | grep -c 'os.WriteFile'` returns 0.
- **Committed in:** `5ec438c`, `8a89fb0` (RED commits)

---

**Total deviations:** 2 auto-fixed (1 blocking, 1 bug). Both are test-harness/lint-gate adjustments; no production-logic scope creep.
**Impact on plan:** None to the delivered behavior. All planned behaviors implemented exactly; deviations only reconciled the RED convention with the lint-gated hook.

## Issues Encountered
- `go mod tidy` removes a dependency that is added but not yet imported. After the keygen RED commit, `tidy` dropped `atotto/clipboard` (clipboard.go was still a stub); it was re-added with `go get` at the start of Task 2 and promoted to a direct dependency once `Copy` imported it. Expected greenfield-Go behavior, not a defect.

## User Setup Required
None - no external service configuration required. (The dependency-legitimacy checkpoint was pre-approved by the orchestrator; both deps are pinned in CLAUDE.md and verified against the Go proxy in 02-RESEARCH.md.)

## Threat Mitigations Applied
- **T-02-09** (key world-readable): private key written 0600, `.pub` 0644 via filewriter; no os.WriteFile.
- **T-02-10** (hand-rolled serialization): uses `ssh.MarshalPrivateKey`/`MarshalAuthorizedKey`; never hand-rolled.
- **T-02-11** (cross-protocol signing reuse): `AllowedSignersLine` mandates `namespaces="git"` + byte-identical email.
- **T-02-33** (duplicate/stale allowed_signers): `WriteAllowedSigners` idempotent per-identity ReplaceBlock + backup; foreign lines preserved.
- **T-02-12 / T-02-SC** (supply chain): both deps pinned + Approved in RESEARCH; go.sum checksums; Go modules run no install scripts.

## Next Phase Readiness
- 02-06 (create-new orchestration) can call `keygen.Generate`, `keygen.AllowedSignersLine` + `keygen.WriteAllowedSigners`, and `clipboard.Copy` directly.
- 02-05 (global gitconfig) will point `gpg.ssh.allowedSignersFile` at the same `~/.ssh/allowed_signers` path this plan writes.
- No blockers.

## Self-Check: PASSED

All created files exist on disk; all four task commits (`5ec438c`, `26635f9`, `8a89fb0`, `fb39642`) are present in git history.

## TDD Gate Compliance

Plan type is `tdd`. Both features show the mandatory RED→GREEN sequence in git log:
- keygen: `test(02-03)` RED `5ec438c` → `feat(02-03)` GREEN `26635f9`
- clipboard: `test(02-03)` RED `8a89fb0` → `feat(02-03)` GREEN `fb39642`

No unexpected RED-phase passes; both RED commits failed on assertions/sentinel error before implementation.

---
*Phase: 02-first-identity-end-to-end*
*Completed: 2026-06-09*
