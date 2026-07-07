# Phase 4: Git Configuration Screen - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-07
**Phase:** 4-git-configuration-screen
**Areas discussed:** insteadOf home (W1), SSH-only completion path, gitdir path derivation, Write ceremony shape

---

## insteadOf home (W1)

| Option | Description | Selected |
|--------|-------------|----------|
| Phase 7, Global Git (Recommended) | Global per-provider config per the recipe; GITUI-01 restricts Phase 4 to per-identity options | |
| Phase 4, per-identity ceremony | Git ceremony also writes the insteadOf block; W1 closes in Phase 4 | ✓ (user override) |
| Drop it for v1.0 | Permanent divergence — rejected, it's North Star wiring | |

| Option | Description | Selected |
|--------|-------------|----------|
| Recipe-literal, per provider | `[url "git@<provider>:"] insteadOf = https://<provider>/`, one managed block per PROVIDER, idempotent across identities | ✓ |
| Alias-targeted per identity | Rewrites https to THIS identity's alias — collides with a second identity on the same provider | |
| Let Claude decide | | |

| Option | Description | Selected |
|--------|-------------|----------|
| Toggle, default ON | Checkbox row "Force SSH over HTTPS for <provider>", default checked; scoped divergence | ✓ |
| Always write it | Recipe-literal, zero choice; opt-out = hand-editing | |
| Only preview, ask nothing | Always write with a prominent preview row | |

**Notes:** Orchestrator recorded a mandatory constraint: the new insteadOf managed block must be registered as a RESERVED gitconfig block (doctor false-positive loop precedent).

---

## SSH-only completion path

| Option | Description | Selected |
|--------|-------------|----------|
| Wizard resume affordance (Recommended) | Alias collision with an existing gitid identity becomes an offer to jump to the git step | ✓ |
| CLI bridge command | `gitid git <identity>` — pulls SHELL-03 forward | |
| Minimal real Identities list + g | Pulls Phase 5 manager UI forward | |
| Wait for Phase 5 | Usability hole for 1-2 phases | |

| Option | Description | Selected |
|--------|-------------|----------|
| Complete-only (Recommended) | Offer only for SSH-only identities; complete ones keep the plain block | |
| Also edit complete identities | One flow, create + edit modes | ✓ (user: "same workflow for both … DRY principle") |

**Notes:** User direction: build the git-form flow ONCE with create/edit modes in Phase 4 so Phase 5's manager reuses it. Edit invariants recorded: idempotent rewrite of all three targets; changed email REPLACES the allowed_signers line.

---

## gitdir path derivation

| Option | Description | Selected |
|--------|-------------|----------|
| Auto-derive + editable row (Recommended) | Default `~/git/<identity>/`, editable field under the radio group, live in the includeIf preview | ✓ |
| Auto-derive, fixed | Zero divergence; wrong layouts silently never match | |
| Let Claude decide | | |

| Option | Description | Selected |
|--------|-------------|----------|
| Create it, shown in preview (Recommended) | mkdir listed as a previewed change | ✓ |
| Don't create, hint only | Identity looks wired while matching nothing | |
| Block until it exists | Hostile — dir legitimately absent before first clone | |

| Option | Description | Selected |
|--------|-------------|----------|
| SSH pattern only (Recommended) | `hasconfig:remote.*.url:git@<ssh-host>:*/**` single block | ✓ |
| SSH + https pair | Recipe-complete; dead weight with insteadOf default-ON | |
| Pair only when insteadOf is OFF | Couples two decisions | |

---

## Write ceremony shape

| Option | Description | Selected |
|--------|-------------|----------|
| One combined ceremony (Recommended) | Final review stacks all previews; ONE confirm writes everything; one backup notice; one result | ✓ |
| Two sequential ceremonies | SSH writes first, git ceremony after; mid-wizard abort leaves side effects | |

| Option | Description | Selected |
|--------|-------------|----------|
| All-or-nothing rollback (Recommended) | Any failure restores every written file from ceremony-start backups; clean failure report | ✓ |
| Stop + report partial state | Leaves half-state for user/Fixer | |
| Best effort, doctor cleans up | Worst first-run experience | |

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, -/+ diff in edit mode (Recommended) | Fixer's before/after diff pattern for changed lines; create mode keeps plain previews | ✓ |
| Plain previews in both modes | Hides what's being replaced | |

---

## Claude's Discretion

- Exact frozen copy for the toggle row, gitdir row label, collision-offer variants, combined-review headers (freeze via 02-STYLE-SPEC §6).
- allowed_signers managed-line mechanics (keyed by identity).
- Home of the reusable git-form flow component within/alongside `internal/tuikit`.
- Combined-review row-budget fitting at 100×30.

## Deferred Ideas

- Manager list/detail + g-launch → Phase 5 (reuses the D-06 flow).
- CLI `gitid git <identity>` → Phase 5 (SHELL-03).
- Global recipe defaults → Phase 7 (insteadOf removed from its scope).
- hasconfig https variants → dropped (D-09).
- Git-artifact health checks → Phase 8.
