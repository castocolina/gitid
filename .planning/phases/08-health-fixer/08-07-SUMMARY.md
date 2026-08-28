# 08-07 SUMMARY — CLI parity: hidden doctor alias, health envelope, tiered exits, and headless e2e

## Outcome

Phase 8's CLI surface is complete. `gitid doctor` is a hidden compatibility alias for Health, and `gitid doctor --fix` routes to the same fixer command body with `--yes` and `--dry-run` forwarded unchanged. `gitid health --json` now emits the frozen `gitid.health/v1` envelope, and Health/Fix command-process status follows the doctor severity tiers.

## Task 1 — Hidden doctor alias

- Added `cmd/gitid/doctor_alias.go` and registered it in `newRootCmd`.
- `doctor` delegates to `runHealth`, the helper used by `health`; its default output and exit tier are identical.
- `doctor --fix` delegates to `runFixCommand`, the helper used by `fix`.
- The constructor documents the pinned behavior: `--fix` alone remains interactive; only `--yes` removes per-finding confirmation; neither path bypasses the existing verification loop.
- Alias regression coverage proves hidden help, Health output parity, unattended forwarding, and interactive prompting.

## Task 2 — JSON contract and exit tiers

- Replaced the provisional flat finding array with `{"schema":"gitid.health/v1","findings":[...]}`, matching the project’s established schema-first versioned-envelope pattern.
- `runHealth` keeps raw doctor findings separate from its render-suppressed projection, so the exit tier reflects the complete scan while JSON/TUI-equivalent output continues suppressing ordinary rows under a critical Files parse failure.
- Reused `exitCodeError`/`exitStatusOf`; warning/info, error, and critical resolve to 1, 2, and 3 respectively. Critical Files and Permissions findings intentionally share tier 3.
- `fix` applies the tier after its existing shared fix loop completes; its direct test seam remains behavior-only and does not acquire a process-status side effect.

## Task 3 — Matrix and e2e

- Expanded `docs/cli-parity-matrix.md` with separate requirement-keyed rows for Health view, identity scope, single fix, batch fix, dry-run preview, and the hidden doctor alias.
- Added `e2e/health_fix_cli_e2e_test.go`, which builds and drives the real binary against a hermetic home through `health --json`, `health --identity`, `fix --dry-run`, `doctor`, and `doctor --fix --yes`.

## Verification

- `go build ./...` — PASS.
- `go vet -tags e2e ./...` — PASS.
- `TERM=dumb SSH_AUTH_SOCK= go test -count=1 -race ./...` — PASS (2155 tests).
- `make lint` — PASS.
- `make test-e2e` — PASS (`ok github.com/castocolina/gitid/e2e 603.162s`).
