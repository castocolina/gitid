# Phase 4: Doctor - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-06-11
**Phase:** 4-doctor
**Areas discussed:** Auto-fix scope & mechanism, Report format & severity, Orphan detection scope, Check depth (baseline/agent/drift)

---

## Auto-fix scope & mechanism

### Q1 — Auto-fix structure vs read-only core

| Option | Description | Selected |
|--------|-------------|----------|
| Detection read-only + separate fixer | Keep internal/doctor pure; finding carries optional Fix func applied by cmd layer via filewriter | ✓ |
| `--fix` flag on the command | Detection read-only; --fix re-runs and applies | |
| Interactive per-finding prompt | Inline y/n per fix, no flag | |

**User's choice:** Detection read-only + separate fixer.

### Q2 — Which finding types get an auto-fix

| Option | Description | Selected |
|--------|-------------|----------|
| Permissions (chmod) | ~/.ssh 700, key 600, .pub 644, config 600 | ✓ |
| Orphaned managed blocks | Remove gitid-managed block whose counterpart is gone | ✓ |
| Missing wiring re-add | Re-add allowed_signers / IdentitiesOnly / re-point includeIf | ✓ |
| Report-only for everything else | Dep install, key deletion, agent loading, value drift | ✓ |

**User's choice:** All four (perms + orphaned blocks + wiring re-add are fixable; everything else report-only).

### Q3 — CLI trigger

| Option | Description | Selected |
|--------|-------------|----------|
| `--fix` flag | doctor reports; --fix applies | |
| Prompt after report | plain doctor reports then asks | |
| Both: flag skips the prompt | Default reports + offers prompt; --fix / --yes applies without prompting | ✓ |

**User's choice:** Both: flag skips the prompt.

### Q4 — Confirmation granularity

| Option | Description | Selected |
|--------|-------------|----------|
| Per-finding confirm | Each mutating fix confirmed individually (perms batch) | ✓ |
| Grouped single confirm | One preview, single confirm to apply all | |
| Tiered: batch safe, confirm risky | Perms batch; block/wiring confirm individually | |

**User's choice:** Per-finding confirm.

### Q5 — Reconcile `--fix skips prompt` with `per-finding confirm` under SAFE-03

| Option | Description | Selected |
|--------|-------------|----------|
| --fix enters per-finding confirm | --fix skips top-level gate, still confirms each mutating finding; --yes is the non-interactive confirmation | ✓ |
| --fix = apply all, --yes not needed | one upfront confirm, no per-finding prompts | |
| --fix interactive, --yes non-interactive | --fix always prompts; --fix --yes applies all | |

**User's choice:** `--fix` enters per-finding confirm; `--yes` IS the SAFE-03 confirmation for non-interactive runs.

**Notes:** Final model — `gitid doctor` reports + top-level "apply fixes?" gate → per-finding confirm; `--fix` skips the gate → per-finding confirm; `--fix --yes` applies all without prompts. Permissions may batch; orphaned-block removal and wiring re-add confirm individually (higher blast radius). Backup per mutated file always.

---

## Report format & severity

### Q1 — Severity model

| Option | Description | Selected |
|--------|-------------|----------|
| error / warning / info | Three levels | |
| error / warning / ok | Two problem levels + ok | |
| critical / error / warning / info | Four levels (critical = exposed key) | ✓ |

**User's choice:** critical / error / warning / info.

### Q2 — Layout & passing checks

| Option | Description | Selected |
|--------|-------------|----------|
| Grouped by family, problems + ✓ passes | Sections with ✓ for passes and findings for failures | ✓ |
| Grouped by family, problems only | Sections, findings only | |
| Flat list sorted by severity | One list, errors first | |

**User's choice:** Grouped by family, problems + ✓ passes.

### Q3 — Exit-code semantics

| Option | Description | Selected |
|--------|-------------|----------|
| Non-zero if any error | exit 1 on error, else 0 | |
| Tiered codes | 0/1/2(/3) by band | ✓ |
| Always 0 | informational only | |

**User's choice:** Tiered codes.

### Q4 — Tiered mapping

| Option | Description | Selected |
|--------|-------------|----------|
| 0 clean / 1 warn+info / 2 error / 3 critical | Highest severity present sets code | ✓ |
| 0 clean / 1 info+warn / 2 error+critical | critical collapses into error | |
| 0 / 1 warn / 2 error+crit, info→0 | info treated as clean | |

**User's choice:** 0 clean / 1 warn+info / 2 error / 3 critical.

### Q5 — Color / non-TTY

| Option | Description | Selected |
|--------|-------------|----------|
| Color on TTY, auto-plain when piped | TTY detect, NO_COLOR respected | ✓ |
| Color + explicit --no-color flag | TTY detect + override flag | |
| Planner's call | leave to planner | |

**User's choice:** Color on TTY, auto-plain when piped (respect NO_COLOR).

---

## Orphan detection scope

### Q1 — How to bound "unused key" detection

| Option | Description | Selected |
|--------|-------------|----------|
| Only gitid-named keys | Flag only id_<algo>_<name> keys with no managed Host block | |
| Any ~/.ssh key unreferenced by managed blocks (as info) | Broader, advisory | |
| Cross-ref against all IdentityFile lines | Orphan only if no Host (managed or hand-written) references it | (basis) |
| **Other (free text)** | "Maybe only mark as warn the keys not used by git — but that doesn't mean they're not used, they may be used against SSH servers" | ✓ |

**User's choice:** Free-text — unreferenced keys are at most a **warning**, not orphans, because they may serve non-git SSH. Resolved rule: cross-ref against ALL ~/.ssh/config Host blocks (managed + hand-written); unreferenced ⇒ warning only, with honest wording; no key-file auto-fix.

### Q2 — Orphans vs coherence/incomplete reporting

| Option | Description | Selected |
|--------|-------------|----------|
| Distinct families, distinct findings | Orphans family separate from Coherence | ✓ |
| One 'drift' family, sub-typed | Single family, tagged | |

**User's choice:** Distinct families, distinct findings.

**Notes:** Follow-up — user asked whether `known_hosts` could check if a key is referenced from a non-git server host. Answered: no — known_hosts stores server host keys, not a client-key→host usage map; SSH persists no such association. Recorded as investigated-and-rejected (CONTEXT D-14). User requested a future enhancement: "map non-git references as SSH server keys" (CONTEXT Deferred Ideas).

---

## Check depth (baseline / agent / drift)

### Q1 — Which Phase 3.1 baseline checks fold in

| Option | Description | Selected |
|--------|-------------|----------|
| excludesfile wiring | core.excludesfile set + points to existing ~/.gitignore_global | ✓ |
| Baseline include resolves | managed [include] points to existing 00-baseline | ✓ |
| ignorecase drift | core.ignorecase=false; warn if flipped | ✓ |
| Curated excludes present | gitignore block still has curated entries | ✓ |

**User's choice:** All four.

### Q2 — ssh-agent depth

| Option | Description | Selected |
|--------|-------------|----------|
| Running + managed keys loaded | ssh-add -l + warn per managed identity not loaded | ✓ |
| Running only | reachable + count | |
| Running + all-keys inventory | reachable + list every loaded key | |

**User's choice:** Running + managed keys loaded.

### Q3 — Drift meaning

| Option | Description | Selected |
|--------|-------------|----------|
| Existence + resolution only | files resolve, includeIf points to fragment, IdentitiesOnly present, allowed_signers present | ✓ |
| Plus targeted value drift | + a few known-footgun value checks | (carve-out) |
| Full content compare | re-render and diff | |

**User's choice:** Existence + resolution only — with a bounded carve-out for locked invariants (ignorecase=false, gpg.format=ssh, allowed_signers email == user.email), which are fixed-value checks, not open-ended drift. Full content-compare explicitly rejected.

---

## Claude's Discretion

- Exact `gitid doctor` command/flag naming (`--fix` / `--yes` or equivalents) and help text — consistent with Phase 2/3 minimal-real-Cobra pattern.
- Report family ordering, exact line formatting, and whether Baseline is its own section vs folded into Coherence/Orphans — dashboard-shaped for Phase 5.
- Per-OS install-hint text for `git` and the clipboard tool — extend the `platform.InstallHint` pattern (brew/apt/dnf/pacman).
- Finding/severity type shape and how the optional fix descriptor is modeled — TDD-first; core stays write-free.
- Where the cmd-layer fixer lives and how it reuses filewriter/gitconfig/sshconfig writers.

## Deferred Ideas

- **Map non-git keys to SSH server hosts** (user future request) — upgrade the unused-key warning to a confident classification.
- **`known_hosts` correlation** — investigated and rejected; recorded so it is not retried.
- **TUI doctor dashboard** (Phase 5) — DOC-07's "runs first in the TUI" half.
- **Full Cobra surface + shell completion** (Phase 5).
- **`url-rewrites` block health checks** — not folded into the four baseline checks; possible future doctor pass.
