---
phase: quick-260612-dtm
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - .golangci.yml
  - .planning/phases/04-doctor/04-SECURITY.md
autonomous: true
requirements: [WARNING-01, T-04-03, T-04-21]

must_haves:
  truths:
    - "A depguard rule in .golangci.yml fails lint when any file under internal/doctor/ imports github.com/castocolina/gitid/internal/filewriter"
    - "make lint exits 0 on the real (clean) tree — the cmd layer and other packages are NOT restricted from importing filewriter"
    - "The gate is proven to fire: a temporary forbidden import is flagged by depguard, then removed, leaving the tree clean"
    - "04-SECURITY.md T-04-03 and T-04-21 are flipped from WARNING to CLOSED, citing the depguard rule as the automated gate"
  artifacts:
    - path: ".golangci.yml"
      provides: "depguard linter enabled + a doctor-scoped deny rule for internal/filewriter"
      contains: "depguard"
    - path: ".planning/phases/04-doctor/04-SECURITY.md"
      provides: "Updated threat table + frontmatter reflecting WARNING-01 closure"
      contains: "depguard"
  key_links:
    - from: ".golangci.yml depguard rule"
      to: "github.com/castocolina/gitid/internal/filewriter"
      via: "deny list scoped to internal/doctor/** files glob"
      pattern: "internal/filewriter"
---

<objective>
Automate the D-01 "write-free core" invariant by adding a golangci-lint v2 `depguard`
rule that forbids importing `github.com/castocolina/gitid/internal/filewriter` from any
file under `internal/doctor/`. This closes Phase 4 SECURITY WARNING-01 (threats T-04-03,
T-04-21), which today is enforced only by a manual grep.

Purpose: A future contributor could import filewriter into internal/doctor and the
violation would slip through unnoticed. depguard turns the manual grep gate into an
enforced lint failure.

Output: An updated `.golangci.yml` with `depguard` enabled and a doctor-scoped deny
rule, proven to fire via a temporary forbidden import; and an updated
`.planning/phases/04-doctor/04-SECURITY.md` reflecting the closure.
</objective>

<execution_context>
@$HOME/.claude/gsd-core/workflows/execute-plan.md
@$HOME/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@.golangci.yml
@go.mod
@.planning/phases/04-doctor/04-SECURITY.md
@CLAUDE.md

# Verified facts (do NOT re-derive — they were confirmed during planning):
#   - Module path is `github.com/castocolina/gitid` (go.mod line 1).
#   - golangci-lint is v2.12.2; `.golangci.yml` already uses `version: "2"`,
#     `linters.default: none`, and a curated `linters.enable:` list.
#   - `make lint` runs `$(GOPATH_BIN)/golangci-lint run ./...` (Makefile).
#     The binary is NOT on PATH; always invoke via `make lint`.
#   - The internal/doctor tree is CLEAN today: the only matches for
#     "internal/filewriter" under internal/doctor/ are comment text in
#     checks/orphans.go and checks/coherence.go, NOT real imports.
#   - SECURITY.md WARNING rows: T-04-03 (the Tampering row) and T-04-21 (the EoP row).
#     Frontmatter today: threats_closed: 21, warnings: 1, plus a warning_followup string.
</context>

<tasks>

<task type="auto">
  <name>Task 1: Add and prove the depguard deny rule in .golangci.yml</name>
  <files>.golangci.yml</files>
  <action>
    Extend `.golangci.yml` to enforce D-01 with `depguard`, keeping the change minimal and
    consistent with the file's existing v2 style and comment density.

    Step A — Confirm the v2 schema before editing (do NOT assume v1 syntax). golangci-lint
    v2 places depguard config under `linters.settings.depguard.rules.<rulename>` where each
    rule has a `files:` list of glob patterns and a `deny:` list of `{pkg, desc}` entries.
    Confirm this against the installed binary or docs: run `make lint` once first to ensure
    the binary is installed, then consult `golangci-lint linters` or the depguard docs. v1
    used a flat `denied`/`allow` shape — that form is wrong here.

    Step B — Add `depguard` to the existing `linters.enable:` list, with a short inline
    comment in the same style as the neighbours (e.g. "depguard # D-01: forbid filewriter
    import inside internal/doctor (write-free core)").

    Step C — Under `linters.settings:` (alongside the existing `gosec:` block) add a
    `depguard:` block with a single named rule scoped ONLY to the doctor tree. The rule must:
      - match files under `internal/doctor/` — cover BOTH `internal/doctor/*.go` and the
        `internal/doctor/checks/` subdirectory. Use a glob such as `internal/doctor/**` or
        equivalent that depguard's v2 `files:` matcher accepts; the fire-test in Task 2 will
        confirm the glob actually catches `checks/*.go`, so widen it there if needed.
      - deny `github.com/castocolina/gitid/internal/filewriter` with a `desc:` that explains
        D-01 (e.g. "D-01: internal/doctor is the write-free core; all mutation is injected
        via Deps closures owned by the cmd layer — never import filewriter here").
    Do NOT add any other deny entries, allow-lists, or broad restrictions. The cmd layer and
    every other package MUST remain free to import filewriter — the rule's `files:` glob is
    the only thing that scopes it.

    Step D — Keep all existing linters and the gosec settings untouched. Do not weaken any
    linter or add exclusions. English-only content.
  </action>
  <verify>
    <automated>grep -v '^[[:space:]]*#' .golangci.yml | grep -q 'depguard' && grep -q 'internal/filewriter' .golangci.yml && grep -q 'internal/doctor' .golangci.yml && echo RULE_PRESENT</automated>
  </verify>
  <done>
    `.golangci.yml` contains a `depguard` entry under `linters.enable` and a
    `linters.settings.depguard` block whose single rule denies
    `github.com/castocolina/gitid/internal/filewriter` for files under `internal/doctor/`,
    with a D-01 `desc`. No existing linter or gosec setting was changed. The grep verify
    prints RULE_PRESENT.
  </done>
</task>

<task type="auto">
  <name>Task 2: Fire-test the gate, confirm clean tree, then close WARNING-01 in 04-SECURITY.md</name>
  <files>.planning/phases/04-doctor/04-SECURITY.md</files>
  <action>
    Prove the gate works, then record the closure. Order matters: prove first, document second.

    Step A — Baseline green. Run `make lint`. It MUST exit 0 on the real, clean tree (the
    cmd layer imports filewriter legitimately and must NOT be flagged). Also run
    `go build ./...` and `go test ./...`; both MUST stay green. Capture the commands and
    their real output (input + output) per the project's hypothesis to test loop.

    Step B — Fire-test (the user explicitly required this). Create a throwaway file
    `internal/doctor/depguard_firetest_tmp.go` in package `doctor` whose ONLY purpose is a
    blank import of the filewriter package (`import _ "github.com/castocolina/gitid/internal/filewriter"`).
    Run `make lint`. CONFIRM depguard reports the forbidden import (the finding names
    depguard and the filewriter package). Record the exact lint output line as evidence. If
    depguard does NOT flag it, the glob in Task 1 is wrong (likely the
    `internal/doctor/checks/` subdirectory or the file pattern) — go back and widen the
    `files:` glob until the fire-test fails lint, then re-verify Step A still passes 0 on the
    clean tree. To prove the scope is correct, also confirm the same blank import inside a
    cmd-layer file is NOT what is being flagged — only the doctor-scoped temp file should
    trip the rule (the cmd layer already imports filewriter and lint stays 0 in Step A,
    which is sufficient evidence the deny is not global).

    Step C — Restore clean tree. Delete `internal/doctor/depguard_firetest_tmp.go`. Run
    `make lint` again — it MUST exit 0. Confirm
    `grep -rn '"github.com/castocolina/gitid/internal/filewriter"' internal/doctor/` returns
    no real import lines (only the pre-existing comment matches in orphans.go/coherence.go).
    Do NOT leave the forbidden import or the temp file anywhere in the tree.

    Step D — Update `.planning/phases/04-doctor/04-SECURITY.md` to record the closure,
    citing the new depguard rule as the now-automated gate:
      - Threat table: flip the T-04-03 row Status from WARNING to CLOSED and the T-04-21 row
        Status from WARNING to CLOSED. In each Evidence cell, replace the "no automated gate
        exists / manual-only" language with a citation of the depguard rule, e.g.
        ".golangci.yml depguard rule denies internal/filewriter under internal/doctor/** —
        fire-tested: a temp forbidden import was flagged by `make lint`, then removed; clean
        tree lints 0".
      - Frontmatter: `warnings: 1` -> `warnings: 0`; `threats_closed: 21` ->
        `threats_closed: 23` (both former WARNING mitigate threats are now CLOSED — total 23
        closed of 24, the remaining 1 disposition being the accept group; threats_total stays
        24; threats_accepted stays 3; threats_open stays 0). Remove the `warning_followup:`
        line. Verify the arithmetic against the table before saving (count CLOSED rows vs
        accepted rows so the numbers reconcile honestly — note T-04-04/14/24 are accepts that
        are also shown CLOSED in the table; do not double-count).
      - Audit Summary section: update "Threats Closed: 21/24" -> "Threats Closed: 23/24" and
        change the prose that says the gate "does not exist" / "is absent as automation" to
        state the depguard gate now exists and is fire-tested. Do NOT touch the
        "Unregistered Flags: 1" line — that count refers to the golang.org/x/term direct
        dependency, a separate concern from WARNING-01.
      - Open Warnings section (WARNING-01): mark it RESOLVED, citing the depguard rule and
        the fire-test, rather than deleting the section's history.
      - Phase Verdict: update the "SECURED with warnings" wording to reflect that WARNING-01
        is now closed (e.g. "SECURED — all mitigate threats CLOSED; WARNING-01 remediated via
        depguard").
    English-only throughout. Make only the edits needed to reflect the closure honestly; do
    not rewrite unrelated rows.
  </action>
  <verify>
    <automated>test ! -f internal/doctor/depguard_firetest_tmp.go && make lint && go build ./... && grep -nE 'T-04-03.*CLOSED' .planning/phases/04-doctor/04-SECURITY.md && grep -nE 'T-04-21.*CLOSED' .planning/phases/04-doctor/04-SECURITY.md && grep -q 'warnings: 0' .planning/phases/04-doctor/04-SECURITY.md && grep -q 'depguard' .planning/phases/04-doctor/04-SECURITY.md && echo CLOSURE_OK</automated>
  </verify>
  <done>
    The fire-test demonstrated depguard flags a forbidden internal/doctor import of
    filewriter; the temp file is deleted; `make lint` exits 0 and `go build ./...` +
    `go test ./...` are green on the clean tree. `04-SECURITY.md` shows T-04-03 and T-04-21
    as CLOSED with depguard evidence, frontmatter `warnings: 0` and `threats_closed: 23`,
    `warning_followup` removed, WARNING-01 marked RESOLVED, and the verdict updated. The
    verify prints CLOSURE_OK.
  </done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| internal/doctor -> internal/filewriter | The D-01 boundary: the doctor (read-only core) must never reach across into the write layer. depguard enforces this at lint time. |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-04-03 | Tampering | internal/doctor import isolation | mitigate | depguard rule in .golangci.yml denies internal/filewriter under internal/doctor/**; fire-tested in Task 2 |
| T-04-21 | EoP | doctor fix closures (no direct write path) | mitigate | Same depguard rule closes the manual-gate gap; lint now fails on any filewriter import in the doctor tree |
| T-DTM-SC | Tampering | no package installs | accept | This task adds no npm/pip/cargo/go dependencies — only a config rule for an already-pinned linter; no supply-chain surface |
</threat_model>

<verification>
- `make lint` exits 0 on the clean tree (real code, cmd layer still imports filewriter freely).
- A temporary `import _ ".../internal/filewriter"` under internal/doctor is flagged by depguard, then removed.
- `go build ./...` and `go test ./...` stay green.
- `.golangci.yml` contains the depguard enable entry and a doctor-scoped deny rule; no existing linter weakened.
- 04-SECURITY.md: T-04-03 and T-04-21 are CLOSED with depguard evidence; frontmatter and summary counts reconciled (warnings: 0, threats_closed: 23).
</verification>

<success_criteria>
- depguard fails lint on a forbidden filewriter import anywhere under internal/doctor/ (proven), and passes on the clean tree.
- The deny is scoped to internal/doctor/** only — no other package is restricted.
- WARNING-01 is closed in 04-SECURITY.md with honest, reconciled threat counts and a citation of the depguard gate.
- No forbidden import or temp file remains in the tree.
</success_criteria>

<output>
Create `.planning/quick/260612-dtm-add-a-depguard-rule-to-golangci-yml-deny/260612-dtm-SUMMARY.md` when done.
</output>
