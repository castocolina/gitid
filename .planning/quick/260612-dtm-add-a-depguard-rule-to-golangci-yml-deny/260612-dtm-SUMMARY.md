---
quick_id: 260612-dtm
status: complete
date: 2026-06-12
closes: WARNING-01 (Phase 04 SECURITY — T-04-03, T-04-21)
---

# Quick Task 260612-dtm — depguard D-01 gate

## Goal

Automate the D-01 "write-free core" invariant (`internal/doctor` must never import
`internal/filewriter`) that the Phase 4 security audit flagged as WARNING-01 — it held in
code but was only manually grep-verified.

## What changed

- **`.golangci.yml`** (committed `c9924bd`): added `depguard` to `linters.enable` and a
  `linters.settings.depguard.rules.doctor-no-filewriter` rule that denies
  `github.com/castocolina/gitid/internal/filewriter` for files matching `**/internal/doctor/**`
  (covers the package root and the `internal/doctor/checks/` sub-package). The cmd layer and
  other packages are unaffected and may still import filewriter.
- **`.planning/phases/04-doctor/04-SECURITY.md`**: flipped T-04-03 and T-04-21 from WARNING →
  CLOSED citing the new automated gate; rewrote the "Open Warnings" section as "Resolved
  Warnings" with the fire-test evidence; updated the Phase Verdict to "SECURED" (no open
  warnings); frontmatter `warnings: 1 → 0`, dropped `warning_followup`. `threats_closed`
  stays 21 (the 2 warning-threats were already in that count; resolving them improves quality,
  not the tally). The separate `x/term` "Unregistered Flags" disclosure was intentionally left
  as-is.

## Fire-test (proof the gate fires)

A *used* import (not blank, to isolate depguard from revive's blank-import rule) was added to a
temp file in the **sub-package** `internal/doctor/checks/`:

```
internal/doctor/checks/zz_depguard_firetest.go:3:8: import 'github.com/castocolina/gitid/internal/filewriter'
is not allowed from list 'doctor-no-filewriter': D-01: internal/doctor is the write-free core ... (depguard)
* depguard: 1
make: *** [lint] Error 1
```

`make lint` failed with `depguard: 1` (not a different linter), proving the rule fires and the
glob reaches the sub-package. The temp file was removed; the clean tree lints 0.

## Verification

- `make lint` → 0 issues on the real tree
- `go build ./...` → OK
- `go test ./...` → all packages pass
- No temp fire-test file left in the tree

## Self-Check: PASSED

## Notes

The first executor run hit a session limit mid-Task-2 (after committing `.golangci.yml` in
`c9924bd` but before finishing the 04-SECURITY.md edits). The orchestrator independently
re-ran the fire-test and completed the SECURITY.md updates (including correcting a
`threats_closed` miscount the partial run introduced).
