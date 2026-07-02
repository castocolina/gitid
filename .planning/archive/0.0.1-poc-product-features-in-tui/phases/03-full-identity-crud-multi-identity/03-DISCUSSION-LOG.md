# Phase 3: Full Identity CRUD + Multi-Identity - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-06-10
**Phase:** 3-full-identity-crud-multi-identity
**Areas discussed:** Reconstruction & partial blocks, List output & health boundary, Update mechanics & immutability, Delete depth & key default
**Roadmap side-effect:** A scope expansion surfaced mid-discussion (baseline/global git config + global gitignore) was routed OUT of Phase 3 into a new **Phase 3.1** before continuing.

---

## Scope routing (pre-discussion)

The user's freeform note ("query/check/mutate any identity SSH key, global git config or per-site config") plus two reference gists revealed a capability gitid lacks: a managed **baseline/global git config + global gitignore**. This is not a "how to implement Phase 3" clarification — it's a roadmap-level change.

| Option | Description | Selected |
|--------|-------------|----------|
| New dedicated phase, promote to v1 | Keep Phase 3 = identity CRUD; add a new phase, promote GLOBAL-01/URLRW-01 v2→v1 + add GITIGNORE-01 | ✓ |
| Expand Phase 3 to include it | Fold baseline-config into Phase 3 | |
| Capture as deferred, keep P3 focused | Note as backlog only | |

**Timing follow-up:** "Pause and add the phase to roadmap now" (vs finish Phase 3 first) — selected. Phase 3.1 inserted via `/gsd-phase --insert 3` before Doctor; ROADMAP.md + REQUIREMENTS.md + STATE.md updated; gists saved to `samples/`. Then Phase 3 discussion resumed.

---

## Reconstruction & partial blocks (IDENT-07)

### Correlation key

| Option | Description | Selected |
|--------|-------------|----------|
| Identity name (sentinel block name) | Canonical key present in all four artifacts; enumerate sentinel block names, gather pieces | ✓ |
| SSH Host alias | Key off alias; weaker (absent from fragment + allowed_signers) | |
| You decide | Claude picks during planning | |

**User's choice:** Identity name (sentinel block name).

### Partial / inconsistent block sets

| Option | Description | Selected |
|--------|-------------|----------|
| Best-effort reconstruct + light flag | Build Account from what exists, show light "incomplete" marker; deep diagnosis → doctor | ✓ |
| Skip incomplete, show only coherent | Hide partial identities from list | |
| Reconstruct silently, no marker | No health signal in list at all | |

**User's choice:** Best-effort reconstruct + light flag.

---

## List output & health boundary (IDENT-03)

### List shape for multi-account identities (IDENT-06)

| Option | Description | Selected |
|--------|-------------|----------|
| Grouped by identity, accounts nested | One block per identity, accounts/aliases nested beneath | |
| Flat table, one row per account | Compact, greppable; identity repeats | |
| You decide | Claude picks layout during planning/UI design | ✓ |

**User's choice:** You decide (Claude's discretion).
**Notes:** Default intent recorded — grouped-by-identity for the human view, optional flat/parseable flag. Health boundary resolved by the Reconstruction "light flag" choice above: list carries a light incompleteness marker only; deep diagnosis stays in doctor (Phase 4).

---

## Update mechanics & immutability (IDENT-04)

### Is the identity name editable?

| Option | Description | Selected |
|--------|-------------|----------|
| Immutable in P3 (rename = delete+recreate) | Edit all fields except name; avoids fragile multi-file rename cascade | ✓ |
| Support rename as a coordinated operation | Move key + .pub, rename fragment, rewrite all blocks, re-point, re-test | |
| You decide | Claude picks during planning | |

**User's choice:** Immutable in P3 (rename = delete+recreate).

### Re-test after update?

| Option | Description | Selected |
|--------|-------------|----------|
| Re-test only on structural changes | Run `ssh -T`/`ssh -G` when alias/provider/port/match changed; skip for pure fragment edits | ✓ |
| Always re-test after any update | Verify after every update regardless | |
| Never auto-test (offer manual) | User runs `gitid identity test` themselves | |

**User's choice:** Re-test only on structural changes.

---

## Delete depth & key default (IDENT-05)

### Key default on delete

| Option | Description | Selected |
|--------|-------------|----------|
| Keep key by default, offer to delete | Remove blocks + fragment file + signers line; separate explicit prompt (default no) to delete key | ✓ |
| Prompt with no default (force a choice) | Always ask, no pre-selected answer | |
| Delete key by default | Cleanest teardown, but irreversible action is the easy path | |

**User's choice:** Keep key by default, offer to delete.

### Removal scope + shared-block handling

| Option | Description | Selected |
|--------|-------------|----------|
| Per-identity only; never touch shared; show manifest | Remove the 4 per-identity artifacts; leave Host * + global signers path always; manifest + confirm + backup | ✓ |
| Also clean global wiring on last delete | Remove shared wiring when deleting last identity | |
| You decide | Claude picks during planning | |

**User's choice:** Per-identity only; never touch shared; show manifest.

---

## Claude's Discretion

- Exact `list` layout (grouped-by-identity default + optional flat/parseable flag).
- Incompleteness-marker wording/glyph (keep it light — marker + what's missing).
- Minimal `gitid identity list/update/delete` Cobra subcommand shapes (Phase 2 pattern).
- Package placement + signatures of the new read-side primitives (managed-block lister/reader and `RemoveBlock`); all mutation stays on `internal/filewriter`.

## Deferred Ideas

- Baseline/global git config + global gitignore → **Phase 3.1** (GLOBAL-01, URLRW-01 promoted v2→v1; GITIGNORE-01 added). Refs: `samples/gist-60f2f1d-gitconfig`, `samples/gist-2c98cff-ssh-config`.
- Doctor checks the baseline (ignorecase off, missing excludesfile/excludes) → Phase 4.
- Identity rename as an in-place operation → later enhancement.
- TUI view/edit of identities and baseline → Phase 5.
- `add repo`, adopt-fragments (ADOPT-01), automatic key upload (AUTOUP-01) → v2.
