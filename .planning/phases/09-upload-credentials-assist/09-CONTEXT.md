# Phase 9: Upload / Credentials Assist - Context

**Gathered:** 2026-07-08
**Status:** Ready for planning

<domain>
## Phase Boundary

After a valid identity exists, gitid registers the `.pub` with the hosting provider
for **authentication and signing** (GitHub = two registrations of the same key;
GitLab = one with `--usage-type auth_and_signing`) — **autonomously** when the
provider's CLI (`gh`/`glab`) is authenticated (UP-03), falling back to clear manual
instructions otherwise. Upload never gates create/copy (D-11 carried). UP-01/UP-02
substrate (`internal/upload`, `internal/uploader`) is built; this phase delivers
UP-03 autonomy, corrects the substrate's routing/value defects, and gives the
upload experience a designed home in the TUI (in-wizard step) and CLI.

</domain>

<decisions>
## Implementation Decisions

### Autonomy model & triggers
- **D-01 (Checkbox model — USER-SPECIFIED):** The identity wizard (and rotate/
  clone/add-account flows) exposes an **auto-upload option** whose state is
  DERIVED from `(hostname, CLI presence, auth status)`:
  1. Hostname matches a supported CLI's main domain (`github.com` → gh,
     `gitlab.com` → glab) + tool present + **authenticated** → checkbox
     **enabled and pre-checked** → upload runs autonomously at the upload step.
  2. Hostname matches + tool present + **not logged in** → checkbox **enabled
     but unchecked** — the user may pre-check it anticipating authentication;
     if still unauthenticated when it runs, degrade to manual instructions
     (never gates) with a "run `gh auth login`" hint.
  3. No matching CLI / self-hosted / unknown host → checkbox **disabled**;
     manual instructions path.
  Headless CLI mirrors the checkbox with flags; defaults derived identically.
  `gitid copy` stays opt-in (`--upload-keys`) — it is a read/export command and
  doubles as the manual re-run surface.
- **D-02 (Announce-and-do):** Autonomous upload prints each command **exactly as
  executed** (gh = two lines: authentication + signing) via `CommandPreview`
  (shared `buildArgs` keeps shown==run structural), then per-key results. No
  prompt, no cancel timer (a timed window is a stop and makes PTY e2e flaky).
  The POC per-key Enter/skip queue in `tui/copy.go` is replaced.
- **D-03 (Failure semantics):** Upload failure never affects the primary
  operation's exit code (exit 0 + warning block + full manual instructions,
  D-11). Duplicate-key responses are classified as idempotent success (see
  D-14 for the glab nuance). USER REQUIREMENT: the upload experience is
  **integrated into the TUI identity wizard as its own step/section** —
  announced commands, per-key results, manual-fallback block — and
  rotate/clone/copy contexts reuse the same upload-section component.
- **D-04 (Rotate old key — USER OVERRIDE):** After rotate, gitid presents an
  **interactive delete offer** for the old remote key: a confirmed ceremony
  (never autonomous), using the provider key inventory (D-15) to resolve the
  old key's ID. Research recommended leave-and-report; the user chose the
  offer. Autonomous behavior remains additive-only — gitid never deletes
  remotely without this explicit per-key confirmation.
- **D-05 (Timing):** In create, upload runs **after the key exists and before
  the `ssh -T` test loop**, so the gate can genuinely PASS on first attempt
  (gh auth login does the same; registration is effectively immediate). The
  existing PASS-or-ReachableNotUploaded gate (Phase 3 D-01) absorbs rare
  propagation lag — no new retry machinery.
- **D-06 (Controls):** `--no-upload` opt-out flag on create/rotate/clone/
  add-account (gh `--skip-ssh-key` precedent). `--dry-run` prints the exact
  `CommandPreview` output and never executes (dry-run stays strictly
  read-only). No env-var opt-out in v1.0.
- **D-07 (Key title — USER-RAISED):** Keys are registered under
  **`gitid: <name> @ <hostname>`** (e.g. `gitid: personal @ ramons-mbp`) so the
  same identity purpose from different machines is distinguishable on the
  provider. Title matching (rotate leftovers, delete offer) is **machine-scoped**
  — it must only ever match THIS machine's titles; key-content comparison
  remains the primary truth for dedupe.

### Design surface (no frozen contract exists for upload)
- **D-08 (Hybrid amendment):** Upload is a **section, not a navigable surface**.
  ONE shared upload-section component contract covering all states: detecting →
  checkbox row (per D-01 model) → announced-running with literal command echo →
  per-key results (GitHub ×2 rows, GitLab ×1) → manual-fallback instructions
  block (multi-line, provider-templated from `internal/upload.Instructions`) →
  section-omitted when no matching CLI. Amend `create-flow/FIELDS.md` (new
  in-wizard upload beat + result rows) and `identity-manager/FIELDS.md` (copy
  modal) via the exercised design-amendment path (scoped commits + APPROVAL
  addendum). Rotate/clone results reuse the same component contract.
- **D-09 (Visual gate satisfiability):** The 100 static reference PNGs were
  removed repo-wide (`REFERENCE-INDEX.md`); the live demos are the approved
  reference. Phase 9's UI wave MUST mint fresh approved captures for exactly
  the amended screens — these become the success-criterion-4 visual-regression
  baseline. Approved sequence: (1) `/gsd-ui-phase` authors the shared component
  spec; (2) FIELDS.md amendments; (3) dummytui + mockup demo states +
  `agent-ui-ux-designer` parity critique; (4) capture + approve screenshots;
  (5) rework `tui/copy.go` to the contract; (6) backend + PTY e2e + visual
  diff vs the new approved set.
- **D-10 (POC contradiction):** `tui/copy.go`'s per-key Enter/skip prompt queue
  contradicts UP-03 autonomy — it is redesigned in the contract (D-08) before
  backend work, not patched in code review.

### Tool selection & provider matching
- **D-11 (DetectFor):** Replace first-found `Detect` with **`DetectFor(provider)`**:
  `github` → gh only, `gitlab` → glab only (lowercase substring match, same
  convention as `upload.Instructions`), unknown → manual-only. **Never
  cross-route**; when the matching tool is absent or unauthenticated the path
  is manual fallback even if the other tool is authenticated. (Current
  first-found routing sends GitLab identities to gh and lets an
  unauthenticated gh shadow an authenticated glab — a correctness bug.)
- **D-12 (glab value — Open Question A2 CLOSED):** Use
  `--usage-type auth_and_signing` (verified: glab's own default; accepted
  values `auth`/`signing`/`auth_and_signing`; flag since glab v1.54.0,
  2025-03-19). The pinned "conservative" `auth` actively downgrades from the
  default and silently loses signing registration — it must be replaced.
  glab <1.54.0 fails loudly ("unknown flag") into the manual fallback —
  acceptable. Rename `GLabKeyTypeForAuth` to reflect combined usage.
- **D-13 (Self-hosted scope):** v1.0 autonomous upload is gated to
  **github.com / gitlab.com** hosts only (consistent with D-01's main-domain
  checkbox gate); self-hosted GHE / self-managed GitLab identities get the
  manual-instructions path. `GH_HOST`/`GITLAB_HOST` env plumbing is deferred
  (it changes the `Deps.RunCmd` API and the CommandPreview shown==run parity).
  Verified: `gh ssh-key add` has NO `--hostname` flag.
- **D-14 (Auth probe + scopes):** Probe with `auth status --hostname <host>`
  (bare `gh auth status` checks ALL hosts — wrong in both directions). Do not
  parse scopes pre-flight; attempt the upload and **classify scope errors**
  into remediation copy (`gh auth refresh -h <host> -s admin:public_key`;
  signing uses `-s admin:ssh_signing_key`), with distinct handling for gh
  partial success (auth uploaded, signing scope-blocked → D-16).

### Idempotency, verification & health
- **D-15 (Provider key inventory):** Add ONE inventory function to
  `internal/uploader`: `gh api user/keys` + `gh api user/ssh_signing_keys`
  (JSON via `--jq`; `gh ssh-key list` has no `--json`), and
  `glab ssh-key list -F json`; compare the normalized key blob (fields 1–2 of
  the `.pub`). It powers: pre-upload dedupe (skip types already present),
  GitHub per-type gap detection (D-16), signing verification (D-17), the
  rotate title-match leftover report, and the D-04 delete-offer key-ID lookup.
  Inventory/list failure degrades to plain upload — never gates (D-11).
  Duplicate errors that slip through anyway are classified: gh → benign (gh
  ≥2.27.0 dedupes client-side and exits 0; any gh new enough for
  `--type signing` also dedupes); glab `"has already been taken"` → treated as
  a **cross-account conflict finding** with manual instructions, NEVER silent
  success (GitLab fingerprints are globally unique across accounts).
- **D-16 (Per-type ensure):** GitHub's two registrations are attempted
  **independently**, driven by the inventory's missing-type diff; the uploader
  returns a **per-registration result struct** (uploaded / already-present /
  failed) instead of `(string, error)`. Partial failure reports per-type with
  manual instructions scoped to the failed type; the next run converges the gap.
- **D-17 (Verification):** Post-upload, re-run the existing `internal/tester`
  `ssh -T` against the alias expecting **ReachableNotUploaded → PASS** (proves
  auth end-to-end AND catches GitLab cross-account conflicts, since PASS
  proves the account matched), plus one post-upload inventory read to confirm
  the signing registration `ssh -T` cannot see. One bounded retry as
  propagation-lag insurance.
- **D-18 (Health):** **No persisted upload state.** The live tester's
  `ReachableNotUploaded` IS the health signal; upload outcomes stay transient
  UI messages. (Derived-state files are this repo's known doctor
  false-positive-loop footgun.) An inventory-backed signing-registration
  finding in the health screen is post-v1.0.

### Claude's Discretion
- Exact copy: announced-running lines, per-key result rows, manual-fallback
  block wording, scope-remediation messages, `gh auth login` hint, checkbox
  labels/disabled-state note, rotate delete-offer ceremony copy.
- Flag naming/wiring details (`--no-upload` vs per-flow variants) within the
  Phase 5 adaptive-CLI + outcome-parity contract.
- Inventory function shape (single call returning per-type presence vs
  separate calls), JSON parsing details, normalized-blob comparison.
- Hostname source for the D-07 title (`os.Hostname()` trimming/normalization).
- Where the upload step sits in the wizard beat sequence (after key persist,
  before/around the test loop per D-05) — pinned during the UI wave.
- Error-string classifiers' looseness (match on scope name / fingerprint
  keyword, not full sentences).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### North Star & requirements
- `recipes/README.md` + `recipes/` — canonical config model (per-provider alias,
  one ed25519 key doing auth + signing — WHY GitHub needs the same `.pub` twice)
- `.planning/REQUIREMENTS.md` §M — UP-01 (built), UP-02 (built), UP-03 (this phase)
- `.planning/ROADMAP.md` Phase 9 — goal, success criteria (esp. criterion 4
  visual gate), dependencies (Phases 3, 5)

### Frozen design system (amendment targets)
- `.planning/design/create-flow/FIELDS.md` — result-success (3 fields today,
  no upload row) + wizard beats; D-08 amendment target
- `.planning/design/identity-manager/FIELDS.md` — copy modal; D-08 amendment target
- `.planning/design/REFERENCE-INDEX.md` — records the reference-PNG removal;
  live demos are the approved reference (grounds D-09)
- `.planning/design/APPROVAL.md` — amendment/approval discipline

### Built substrate (this phase modifies)
- `internal/uploader/uploader.go` — Detect (→ DetectFor, D-11), buildArgs /
  CommandPreview (shown==run), `GLabKeyTypeForAuth` (D-12), UploadKey
  (→ per-type result struct, D-16), new inventory function (D-15)
- `internal/upload/upload.go` — `Instructions(provider)` manual-fallback text +
  the provider substring-match convention D-11 reuses
- `cmd/gitid/copy.go` — `--upload-keys`/`--yes` CLI path, `buildUploaderDeps`
- `tui/copy.go` — copyPubkeyModel POC prompt queue (D-10 redesign target)
- `internal/tester/` — `ssh -T` classification incl. ReachableNotUploaded
  (D-17 verification substrate; Phase 3 D-01 gate)

### Prior phase contracts that bind here
- `.planning/phases/05-identity-manager/05-CONTEXT.md` — CLI grammar
  (noun-verb, gh-adaptive, `--yes`/`--dry-run` semantics), rotate=retirement,
  clone=copy+re-derive (all upload trigger flows)
- `.planning/phases/08-health-fixer/08-CONTEXT.md` — health/finding model
  (D-18 boundary; no persisted upload state)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/uploader` — Detect/AuthCheck/UploadKey/CommandPreview with the
  Deps seam (LookPath, RunCmd) already testable with fakes; shown==run is
  structural via shared `buildArgs`.
- `internal/upload.Instructions(provider)` — provider-templated manual
  instructions, already used by CLI + TUI; the manual-fallback block of the
  D-08 component renders this.
- `internal/tester` — ssh -T PASS / ReachableNotUploaded classification is the
  post-upload verification (D-17) and the health signal (D-18) for free.
- `internal/dummytui/` fixtures + mockup demo — extend with the D-08 states at
  existing granularity (states inside existing fixtures, no new surface file).

### Established Patterns
- Deps-injection seams with nil-guard wiring tests (`tui/wiring_test.go`) —
  the inventory function and DetectFor must join the guard.
- Upload never gates (D-11) is enforced at every current call site — preserve
  under all new paths (inventory failure, scope errors, unauth).
- Design-amendment discipline (Phases 6–8): scoped FIELDS.md commits +
  APPROVAL addendum + parity critique before code.

### Integration Points
- Create flow (`cmd/gitid/add.go` runCreateNew + TUI wizard): upload step
  inserts after key persist, before the test loop (D-05); manual-instructions
  print point already sits there.
- Rotate/clone/add-account flows (Phase 5 surfaces): same component + derived
  checkbox; rotate additionally gets the D-04 delete-offer ceremony.
- `gitid copy --upload-keys`: unchanged trigger semantics, upgraded internals
  (DetectFor, inventory, per-type results).

</code_context>

<specifics>
## Specific Ideas

- USER: the wizard checkbox with derived enabled/checked state (D-01) — the
  user explicitly framed the two scenarios (logged / not logged) and the
  main-domain-suffix match as the enabling condition.
- USER: "CLI is fine but I need it integrated in the UI in the identity
  wizard" — upload is a first-class wizard step, not CLI-only output (D-03).
- USER: key titles must distinguish machines — "personal ramon@gmail.com at
  linuxmachine is not the same as at machostname" → `gitid: <name> @ <hostname>`
  (D-07).
- USER: interactive delete offer for the old key after rotate (D-04),
  overriding the leave-and-report recommendation.
- Announce-and-do precedent: gcloud compute ssh / gh auth login; idempotency
  precedent: ssh-copy-id, Ansible github_key (observe→diff→apply→verify).

</specifics>

<deferred>
## Deferred Ideas

- `GH_HOST`/`GITLAB_HOST` env plumbing for self-hosted autonomous upload →
  post-v1.0 (Deps.RunCmd API change + preview parity work; needs a GHE/
  self-managed test target).
- `GITID_NO_UPLOAD` env-var opt-out → only if a fleet/CI use case appears.
- Inventory-backed signing-registration finding in the health screen →
  post-v1.0 increment (D-18 keeps v1.0 health stateless).
- Tool-inventory display ("glab installed but this is a GitHub identity") →
  only if the TUI wants a richer status panel.
- Distinct exit code for "primary OK, upload degraded" → only if a CI consumer
  materializes.

</deferred>

---

*Phase: 9-upload-credentials-assist*
*Context gathered: 2026-07-08*
