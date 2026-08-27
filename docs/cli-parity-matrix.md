# gitid CLI parity matrix

This is the **requirement-keyed outcome-to-command matrix** D-04 (05-CONTEXT.md)
requires: every product *outcome* the TUI can produce has a CLI command, and
no ceremony *step* (preview, confirm, backup, re-test) is ever a command —
those are internal invariants of the one lifecycle chokepoint each verb calls.

The matrix is **machine-checked in both directions** by
`cmd/gitid/identity_test.go` (run by `make test`, decision D-04): every
`shipped` row's command path must resolve in the built command tree, every
`deferred Phase N` row's noun must exist and return its phase error, and every
command in the tree (outside the documented tooling exclusions) must be named
by at least one row. A future phase that adds an outcome **must** add a row to
this file; the checker turns an omission into a test failure, not
documentation drift.

**Tooling exclusions** (named in the checker): `gitid completion`, `gitid
help`, and `gitid debug` (Phase 1 diagnostic readout) are tooling, not product
outcomes, and are never required to appear in a row.

Flag names and shorthands chosen here are the **user-facing contract of the
CLI**: renaming one is a visible, deliberate edit because the matrix test pins
every path.

## Requirement-keyed outcome matrix

| Requirement | Outcome | Command | Selecting flags | Status | Notes |
|---|---|---|---|---|---|
| KEY-01, SSHUI-01, TEST-01, SHELL-03 | Create a new identity | `gitid identity create` / `gitid create` | `--name <n> --provider <p> --git-name <n> --git-email <e> [--ssh-host <alias> --hostname <h> --port <p> --algorithm <algo> --reuse-key <path> --strategy <gitdir|hasconfig|both> --git-dir <dir> --force-ssh] --yes` | shipped | D-02 adaptive depth: missing a required flag opens the pre-filled wizard on a terminal, or errors naming the missing flags off one. `--dry-run` runs both gate stages and prints the four artifact previews. `--force-ssh` (WR-06, default off) opts into the machine-global provider URL rewrite; the TUI exposes the same toggle. |
| MGR-01, SHELL-03 | List every identity | `gitid identity list` / `gitid list` | `[--json]` | shipped | Reads are always headless; `--dry-run` is a usage error. Frozen D-03 JSON document. |
| MGR-03, SHELL-03 | Show one identity | `gitid identity show <name>` / `gitid show <name>` | `[--json]` | shipped | Reads are always headless; `--dry-run` is a usage error. |
| MGR-04, SHELL-03 | Clone an identity reusing the source key | `gitid identity clone <source>` / `gitid clone <source>` | `--name <n> [--yes]` | shipped | D-15: clone has no second write pipeline — the re-derived values feed the create lifecycle. `--dry-run` runs both gate stages (D-16 full gate even for a same-key clone) and prints the previews. The clone's provider URL rewrite (WR-06) mirrors the SOURCE's own current setting — there is no `--force-ssh` flag on this verb. |
| MGR-04, SHELL-03 | Clone an identity with a fresh key | `gitid identity clone <source>` / `gitid clone <source>` | `--name <n> --new-key [--yes]` | shipped | `--new-key` generates at the new canonical path instead of reusing the source's key. |
| MGR-05, KEY-07, SHELL-03 | New key for an existing identity (repair) | `gitid identity new-key <name>` / `gitid new-key <name>` | `[--yes]` | shipped | Calls the SAME `runRepair` the TUI's CommitNewKey calls; repair never touches pre-existing key material. `--dry-run` tests the CURRENT key and creates nothing. |
| KEY-05, SHELL-03 | Rotate an identity's key | `gitid identity rotate <name>` / `gitid rotate <name>` | `[--yes]` | shipped | Calls the SAME `runRotate` the TUI's CommitRotate calls; archives the old pair, generates the new one at the same canonical paths. `--dry-run` labels the CURRENT key and carries the frozen post-rotation caveat. |
| MGR-06, SHELL-03 | Delete the Git side only | `gitid identity delete <name>` / `gitid delete <name>` | `--git-only [--yes]` | shipped | Exactly one scope flag required. SSH Host block, key pair, and allowed_signers line are left untouched (D-10). Calls the SAME `runDelete` the TUI's CommitDelete calls. |
| MGR-06, SHELL-03 | Delete everything (SSH + Git + key) | `gitid identity delete <name>` / `gitid delete <name>` | `--all [--yes]` | shipped | Key pair is archived before removal (D-11); shared-key downgrade note and the D-13 scan disclaimer are printed before acting. |
| GSSH-01, SHELL-03 | List the six global SSH options | `gitid ssh options list` | `[--json]` | shipped | Reads are always headless. Frozen `gitid.ssh.options/v1` envelope. |
| GSSH-01, SHELL-03 | Apply named global SSH options | `gitid ssh options apply` | `<key>... [--dry-run] [--yes] [--fail-on-advisory] [--json]` | shipped | Calls the SAME `runGlobalSSHApply` the TUI's CommitGlobalSSH calls. Headless when at least one key is named; incomplete on both TTYs opens the Global SSH Options sub-tab with an empty selection; otherwise exit 1 naming the missing positional. A successful backed-up write exits 0 even when post-write verification reports shadowing (D-04/D-14: advisory is never blocking); `--fail-on-advisory` is the opt-in for a stricter script; advisory detail is machine-readable in `gitid.ssh.apply/v1`. |
| GSSH-01, SHELL-03 | Show the SSH storage layout | `gitid ssh storage show` | `[--json]` | shipped | Reads are always headless. Frozen `gitid.ssh.storage/v1` envelope. Re-exercises STORE-01/STORE-03 rather than re-opening them. |
| GSSH-01, SHELL-03 | Migrate SSH storage layout | `gitid ssh storage migrate` | `--to <include or in-file> [--dry-run] [--yes] [--json]` | shipped | Calls the SAME `runSSHStorageMigrate` the TUI's CommitSSHStorage calls, with an empty plan token. Headless when `--to` is supplied; incomplete on both TTYs opens the Global SSH Storage sub-tab with the radio on the current layout; otherwise exit 1 naming `--to`. Frozen `gitid.ssh.migrate/v1`. Re-exercises STORE-01/STORE-03 rather than re-opening them. |
| GGIT-01, SHELL-03 | List the global Git options | `gitid git options list` | `[--json]` | shipped | Reads are always headless. Frozen `gitid.git.options/v1` envelope, one row per policy row in declaration order, including the fallback-author row (unreachable by apply). |
| GGIT-01, SHELL-03 | Apply named global Git options | `gitid git options apply` | `<key>... [--dry-run] [--yes] [--fail-on-advisory] [--json]` | shipped | Calls the SAME `runGlobalGitApply` the TUI's CommitGlobalGit calls. argv resolves against the policy's frozen row-token column ONLY (R-5: exact, case-sensitive) — a member config key is refused naming its owning token. Headless when at least one token is named; incomplete on both TTYs opens the Global Git screen with an empty selection; otherwise exit 1 naming the missing token. A successful backed-up write exits 0 even when advisories are reported; `--fail-on-advisory` exits 3 as the opt-in. Advisory detail is machine-readable in `gitid.git.apply/v1`. |
| GGIT-01, SHELL-03 | Show the global fallback author | `gitid git fallback show` | `[--json]` | shipped | Reads are always headless. Frozen `gitid.git.fallback/v1` envelope. A missing config file reports both halves unset — the first-run state, not a failure. |
| GGIT-01, SHELL-03 | Set or clear the global fallback author | `gitid git fallback set` | `--name <n> --email <e> --clear-name --clear-email [--dry-run] [--yes] [--json]` | shipped | Calls the SAME `runGitFallbackAuthorApply` the TUI's CommitGitFallbackAuthor calls. Setting and clearing are explicit and separate: an empty value, a set-plus-clear pair for the same half, or no flags at all are refusals, so a script can never erase a half by omission (D-04). Frozen `gitid.git.fallbackset/v1`. |
| — | Health | `gitid health …` | — | deferred Phase 8 | The noun is reserved (plan 05-01); Phase 8 adds rows here. |
| — | Fix | `gitid fix …` | — | deferred Phase 8 | The noun is reserved (plan 05-01); Phase 8 adds rows here. |

The `deferred Phase N` status is what keeps this matrix a complete map of the
product surface on day one: a later phase that forgets to add its real rows
gets a checker failure, while the reserved noun still matches plan 05-01's
"arrives in a later phase" error instead of contradicting it.

## The per-verb `--dry-run` contract (R12-DR)

Every write verb accepts `--dry-run`; it always exits zero having written
nothing, and it never demands `--yes` or a terminal (a dry run stops before
the confirmation gate). What "dry run" means is **per verb**, because the
verbs do not all have the same stages:

| Verb | Dry run does | Dry run does NOT |
|---|---|---|
| create, clone | run stage 1 and stage 2 against the staged key, print outcomes and the four artifact previews | write to `~/.ssh` or `~/.gitconfig`; the staging directory is cleaned up |
| rotate, new-key | print the ceremony plan (targets, archive destination shape, provider host); run the connectivity test against the identity's CURRENT key when it exists | generate a key, stage anything, archive, or write |
| delete (both scopes) | print the full `DeletePlan` — targets, shared-key note, scan hits, disclaimer | run any connectivity test (delete's row in `lifecycleStages` has no test stage), back up, or write |
| ssh options apply | run the plan, the simulation and the shadow report and print them | write or back up |
| ssh storage migrate | run the migration plan and print the resulting configuration for both files | back up, write, or trim |
| list, show | — | accept the flag at all; `--dry-run` is a usage error on reads |

The rotate/new-key dry-run row has a deliberate scope limit stated in the
output itself, not only here (review R2-13): the connectivity test it runs
proves the identity's **CURRENT** key still reaches the provider. It proves
NOTHING about the post-rotation state — the new key does not exist yet, has
not been uploaded, and its `ssh -G` resolution against the replacement is
exactly the unproven artifact D-16 says must be re-tested for real. The frozen
caveat sentence is: *"This dry run tests only the current key's reachability —
the new key has not been generated, uploaded, or resolved, so nothing about
the post-rotation state is proven."*

The delete row's "no connectivity test" is not a local choice of this plan —
it is delete's row in plan 05-07's `lifecycleStages` table, and a test reads
that table and asserts the recording tester seam count is zero for a delete
dry run AND a full delete, so the two plans' contracts cannot drift apart.