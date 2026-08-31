# Deferred Items

Out-of-scope findings discovered during code review, logged per the fixer's
scope boundary (fix only mechanical bugs; leave frozen-copy/design decisions
for a deliberate follow-up).

## From 09-REVIEW.md / 09-REVIEW-FIX.md, iteration 3 fixer pass (2026-08-31)

Three code-review Warnings survived three independent review rounds
(iterations 3, 4, and 5), each re-verified as still-open and each classified
consistently: they require a deliberate design/product decision, not a code
fix. All three are non-blocking to Phase 9's own goal (upload/credentials
assist working correctly) — the code paths they describe already behave
correctly, only the copy/rendering shape is at issue.

### 1. D-08 register-key pane mutates the provider account with no confirm step

**File:** `internal/tuikit/identities.go:2669-2677`,
`.planning/design/identity-manager/FIELDS.md:87`

`FIELDS.md:87` records "opening the modal IS the explicit opt-in" as an
intentional contract, restated in `TestIdentityManager_RegisterKeyModalRuns`'s
doc comment. Does not violate CLAUDE.md's write-confirmation rule (that rule
is scoped to `~/.ssh/config`/`~/.gitconfig` mutations; this is a provider API
call). Changing it means amending `FIELDS.md`, `design.go`'s frozen copy, the
visual-divergence allowlist, and at least three real-binary e2e tests.

**Recommendation:** route through `/gsd-discuss-phase` for a future phase if
the product decision changes, or explicitly ratify the existing "modal-open
is opt-in" contract as final.

### 2. The multi-line `ManualCommand` is interpolated into a single-line sentence

**File:** `cmd/gitid/upload_run.go:552-558`, `internal/tuikit/design.go:695`,
`internal/tuikit/identities.go:2812`, `:2821`

The fix is either an R22 frozen-copy amendment in `design.go` (render the
commands as an indented block) or a producer change to join with `" && "` —
both deliberate copy/rendering decisions requiring dedicated review.

**Recommendation:** file as a tracked design/backlog item; a natural fit for
a future TUI-polish phase.

### 3. The upload checkbox's actionable copy is truncated at the only width production uses

**File:** `internal/tuikit/identities.go:4820-4828`, `:4835`,
`internal/tuikit/design.go:582`, `:586`

Shortening the frozen unauth/disabled labels is an R22 copywriting decision;
widening the row breaks the wizard's one-line row budget.

**Recommendation:** file as a ROADMAP/backlog item alongside item 2 above —
both are `design.go` R22 copy amendments and could be resolved together.
