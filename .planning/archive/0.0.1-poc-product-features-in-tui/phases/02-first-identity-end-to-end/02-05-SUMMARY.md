---
phase: 02-first-identity-end-to-end
plan: 05
subsystem: gitconfig + tester
tags: [gitconfig, includeIf, fragment, ssh-signing, ssh-tester, tdd]
requires:
  - "internal/filewriter (02-01): ReplaceBlock + atomic Write chokepoint"
provides:
  - "internal/gitconfig: RenderIncludeIf / WriteIncludeIf (managed includeIf block, gitdir+hasconfig)"
  - "internal/gitconfig: WriteFragment (git config --file fragment keys, signingkey-as-path, [remote] guard)"
  - "internal/gitconfig: SetAllowedSignersFile (global gpg.ssh.allowedSignersFile wiring)"
  - "internal/tester: ClassifyPreWrite / PreWrite / Resolved / ParseResolved (output-substring 3-way classifier + ssh -G parse)"
affects:
  - "Phase 02 create-new orchestration: consumes gitconfig writers + tester classifier to prove identity before/after write"
tech-stack:
  added: []
  patterns:
    - "gitconfig key/values via `git config --file` arg-slice os/exec (git is authoritative parser)"
    - "includeIf headers as filewriter sentinel-delimited managed blocks (no Go lib supports includeIf write-back)"
    - "SSH test classification by output substring only; exit code ignored (D-01)"
    - "injectable runner func seam for unit-testing exec-backed code without live network"
key-files:
  created:
    - internal/gitconfig/renderer.go
    - internal/gitconfig/fragment.go
    - internal/gitconfig/renderer_test.go
    - internal/gitconfig/fragment_test.go
    - internal/tester/tester.go
    - internal/tester/tester_test.go
  modified:
    - internal/gitconfig/doc.go
    - internal/tester/doc.go
decisions:
  - "RenderIncludeIf returns the FULL block (with sentinels) to match the plan artifact spec; WriteIncludeIf renders the body-only form internally for filewriter.ReplaceBlock to avoid double-wrapping."
  - "Newline/`[remote` injection rejected in two layers: validateValue (returns error, fragment path) and a panic in RenderIncludeIf (programming-error path, callers pass validated input)."
  - "Tester exposes an unexported `runner` seam + `preWriteWith` so the 3-way classifier and Result-shape are unit-tested with fixtures, no live SSH."
metrics:
  duration_min: 6
  completed: 2026-06-09
  tasks: 2
  files: 8
---

# Phase 02 Plan 05: gitconfig + tester Summary

Per-identity gitconfig fragment + includeIf managed block (gitdir trailing-slash and/or hasconfig) written via `git config --file` and the filewriter chokepoint, with `user.signingkey` as a `.pub` path and a hard `[remote]` guard; plus a two-phase SSH tester that classifies connectivity strictly by output substring (PASS / ReachableNotUploaded / Failure) and parses lowercase `ssh -G` keys, every Result carrying its input command and raw output.

## What Was Built

### internal/gitconfig (Task 1)
- `RenderIncludeIf(identity, fragmentPath, []Match) string` — emits `# BEGIN gitid managed: <id>` … `[includeIf "gitdir:~/git/<id>/"]` (trailing slash normalized — Pitfall 7/D-13) and/or `[includeIf "hasconfig:remote.*.url:…"]` with a `path = <fragment>` line … `# END gitid managed: <id>`. Both `Match` kinds combinable in one block (GIT-02).
- `WriteIncludeIf(...)` — idempotent install via `filewriter.ReplaceBlock` + atomic `filewriter.Write` (0644); foreign content preserved byte-for-byte; returns backup path.
- `WriteFragment(fragmentPath, name, email, signingKeyPath)` — sets `user.name`, `user.email`, `gpg.format=ssh`, `user.signingkey` (PATH, never inline — SIGN-02), `commit.gpgsign true` via `git config --file` arg-slice exec; rejects any value introducing a `[remote]` section (Pitfall 9) and malformed/newline-bearing input.
- `SetAllowedSignersFile(gitconfigPath, path)` — global `gpg.ssh.allowedSignersFile` wiring (SIGN-01).

### internal/tester (Task 2)
- `ClassifyPreWrite(output) Outcome` — pure substring switch: `successfully authenticated`→PASS, `Permission denied (publickey)`→ReachableNotUploaded, else→Failure. Exit code never consulted (D-01/Pitfall 2).
- `PreWrite(keyPath, host) Result` — builds `ssh -i <key> -o IdentitiesOnly=yes -o BatchMode=yes -o ConnectTimeout=10 -T git@<host>`, captures combined output, stores `cmd.String()` (input) + output (TEST-03).
- `Resolved(alias)` — live `ssh -T git@<alias>` + `ssh -G <alias>`.
- `ParseResolved(sshGOutput) ResolvedConfig` — case-sensitive lowercase `^key ` prefix match; `identityfile` repeats collected (D-03); camelCase lines ignored.

## TDD Gate Compliance
Both tasks followed RED → GREEN:
- gitconfig: `3d2826d` (test/RED) → `016e253` (feat/GREEN)
- tester: `22b2f88` (test/RED) → `4afdf88` (feat/GREEN)

RED commits ship minimal compiling stubs (zero-value / `errNotImplemented` sentinel, never `panic`) so the lint-gated pre-commit hook passes while tests fail genuinely at runtime. No REFACTOR commit needed (implementations were clean on first GREEN; gofmt/goimports normalization applied by the hook).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] gosec annotations on test-file file I/O**
- **Found during:** Task 1 RED commit (pre-commit lint hook blocked)
- **Issue:** `make lint` (gosec) flagged G204/G304/G306 on `exec.Command`, `os.ReadFile`, and `os.WriteFile(0644)` in the new test files.
- **Fix:** Added per-line `//nolint:gosec` annotations matching the established convention in `internal/filewriter/filewriter_test.go` (TempDir-derived fixture paths; 0644 is the gitconfig contract).
- **Files modified:** internal/gitconfig/renderer_test.go, internal/gitconfig/fragment_test.go
- **Commit:** folded into RED `3d2826d`

**2. [Rule 3 - Blocking] revive unused-parameter in test runner closure**
- **Found during:** Task 2 RED commit
- **Issue:** revive flagged the unused `args` parameter of the injected runner closure.
- **Fix:** renamed to `_`.
- **Commit:** folded into RED `22b2f88`

No architectural deviations (no Rule 4). No new dependencies (`go get` not run).

## Verification Evidence

```
$ go test ./internal/gitconfig/... ./internal/tester/... -race
ok  github.com/castocolina/gitid/internal/gitconfig  1.742s  coverage: 82.1%
ok  github.com/castocolina/gitid/internal/tester     1.834s  coverage: 66.7%

$ make lint
0 issues.

$ grep -rEc '(sh|bash) -c' internal/gitconfig/ internal/tester/
# 0 matches in every file — no shell-string exec
```

## Success Criteria Status
- GIT-01/02: includeIf managed block (gitdir trailing slash + hasconfig, combinable) → asserted
- GIT-03/SIGN-02: fragment sets the four keys; `user.signingkey` is a path, no inline literal → asserted
- SIGN-01: `SetAllowedSignersFile` wires global `gpg.ssh.allowedSignersFile` → done
- TEST-01/02/03, D-01/D-03: 3-way substring classification (exit code ignored) + lowercase `ssh -G` parse with repeated identityfile + Result carries input command and raw output → asserted

## Known Stubs
None. All artifacts are fully wired; `Resolved` and `PreWrite` shell out to the real `ssh` binary (unit tests exercise the pure classifier/parser and the injectable seam, not live network — by design, read-only).

## Notes for Downstream
- The create-new orchestration plan should: (1) `tester.PreWrite` before any write to confirm host reachability and ReachableNotUploaded for a new key (D-02 proceed), (2) `WriteFragment` + `WriteIncludeIf` + `SetAllowedSignersFile`, (3) `tester.Resolved` to prove the alias resolves to the expected identity afterward.
- `Match` values for hasconfig must already include the `remote.*.url:` prefix (caller supplies the full condition tail); gitdir values may omit the trailing slash (normalized for you).

## Self-Check: PASSED
All 6 source/test files and the SUMMARY exist on disk; all 4 task commits (RED+GREEN for both tasks) are present in git history.
