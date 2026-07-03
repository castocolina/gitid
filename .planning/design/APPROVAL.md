# APPROVAL.md — DLV-08 approval record (Phase 2: Design All Mockups)

**Status: SCAFFOLD — awaiting the single human checkpoint (02-12). The closing
sign-off line below (bold `APPROVED`, a colon, a date, and "by" the approver — see
"No backend logic before approval" for the exact format) is deliberately absent and
the approver name is NOT inferred here; it is added by the human at the 02-12
checkpoint.**

This record is the DLV-08 approval mechanics artifact (`02-RESEARCH.md` § "Approval
Mechanics (DLV-08)"): written after every surface's `CRITIQUE.md` shows 0 open
findings and the final cross-surface consistency pass (below) is complete, listing
the complete reference set and the resolved parity findings, ready for the single
human sign-off that gates Phases 3-9.

## Final cross-surface consistency pass

Performed against the complete `.planning/design/mockup-src` (`/mui`) build and the
`cmd/gitid-dummy` dummy screenshot set, applying `agent-ui-ux-designer`'s documented
methodology (evidence-based critique against this project's own design contract —
`02-UX-DIRECTION.md` — rather than generic web-SaaS research, since gitid's mockups
are a deliberate terminal skin, not a marketing site).

**Reviewer note (same limitation recorded in every prior 02-04..02-10 `CRITIQUE.md`
and `02-01`/`02-02`-SUMMARY.md):** the executor's toolset in this session was limited
to `Read`/`Write`/`Edit`/`Bash` — no `Task`/subagent-dispatch tool was available to
spawn a fresh-context `agent-ui-ux-designer` subagent for this final pass. In its
place, this pass was performed directly by the plan executor, viewing all 100 frozen
reference PNGs (50 HTML + 50 TUI, `.planning/design/REFERENCE-INDEX.md`) — sampling
across all 7 surfaces (entry list/options/health-summary screens, the shared
four-beat mutation ceremony on every mutating surface, and the strongest-confirm
destructive-delete screen) — side by side against `02-UX-DIRECTION.md` §§2/5/6, and
cross-checking every surface's `parity.json` (63 rows, 0 unresolved) and `CRITIQUE.md`.
**This does not substitute for a fresh-context `agent-ui-ux-designer` pass** —
flagging explicitly so the phase-level `/gsd-code-review` and the external
cross-vendor review can re-run one if the orchestrator has that capability this
session lacked.

**Findings:** none new. The pass confirms:
- **One shared shell** across all 7 surfaces in both media: `gitid  <surface>/<screen>`
  breadcrumb header, surface title, body content, `[SIG-...]` signature label, and an
  always-on keybar with reserved keys (`1`-`5`, `Esc`, `q`, `?`) present identically —
  verified on `identity-manager/list-populated`, `global-ssh/options-list`, and
  `health/health-with-findings`.
- **One color-semantics table**: green `check` = complete/resolved/success, yellow
  `!` = warning/advisory (never blocking), red `x`/cross = error/critical/destructive,
  cyan `~` = info — identical glyphs and semantics across `identity-manager`,
  `global-ssh`, and `health`.
- **One four-beat mutation ceremony** (preview -> confirm -> backup notice -> result)
  verified end to end on `global-ssh` (`confirm-write` shows the literal
  `# BEGIN/END gitid managed:` sentinels and "Nothing has changed yet"; `backup-notice`
  shows the timestamped backup path as the undo story) and on `identity-manager`'s
  irreversible delete (`confirm-destructive`: default-focused "No", the strongest
  confirm — typed identity-name confirmation — reserved for the one genuinely
  irreversible action, per §6.D).
- **Health visibly read-only, advisory surfaces visibly non-blocking**:
  `health/health-with-findings` states "Health only diagnoses -- nothing here writes
  to your files. Open the Fixer (key 5) to change anything shown." and carries no
  ceremony beats; `global-ssh/options-list` states "Recommended, not required -- you
  can leave any option unchanged. This is advisory, never a compliance gate."
- **HTML mockup and TUI dummy read as one product**: `identity-manager/list-populated`
  in both media share identical breadcrumb, copy, status labels, and structure (monospace
  type, dark terminal palette in the HTML skin) — confirming §6.A's "terminal skin
  approved" criterion.
- **Every surface `parity.json` is 0-unresolved** (63 rows across 7 surfaces) — no new
  divergence found by this pass; no new rows added.

No `CRITIQUE.md` or `parity.json` file was modified by this pass (0 new findings to
resolve).

## A. Shell & IA

- [ ] Global frame approved: header/context bar, body archetypes, status line,
      always-on keybar.
- [ ] Navigation model approved: five primary views on number keys `1`-`5` + palette;
      reserved keys (`Esc`/`q`/`?`/`/`/`Enter`) consistent across all surfaces.
- [ ] Terminal skin approved: the MUI mockup reads as a terminal, and it and the TUI
      dummy read as **one product**.

## B. Per-surface completeness (all seven)

- [ ] Every named state in each surface's manifest is present in **both** media
      (HTML + TUI), frame-for-frame. See `REFERENCE-INDEX.md` — 50/50/50 (manifest
      count / HTML PNGs / TUI PNGs) across create-flow, git-screen, identity-manager,
      global-ssh, global-git, health, fixer.
- [ ] Empty / first-run states are designed (not just the happy path) — especially
      the Identity Manager `list-empty` landing and the Fixer `nothing-to-fix`.

## C. Copy, fields, options, defaults FREEZE

- [ ] Field order and labels final on every form.
- [ ] Helper/explanation copy final (Global SSH & Git per-option explanations
      especially).
- [ ] Option sets final: algorithm catalog; match strategy (gitdir/hasconfig/both,
      default gitdir); delete choices; reuse-vs-generate key.
- [ ] Defaults recipe-accurate: `Port 443`, `IdentitiesOnly yes`, `gpg.format=ssh`,
      `init.defaultBranch=main`, `core.ignorecase=false`, blank-prefix WYSIWYG.
- [ ] Recipe fidelity confirmed: alias-per-identity `Host` block, `insteadOf` URL
      rewrite, `includeIf hasconfig:`/`gitdir:`, `allowed_signers` line byte-identical
      to `user.email` — all visible in the relevant previews.

## D. Safety affordances

- [ ] Every mutating surface shows the full four-beat ceremony (preview -> confirm ->
      backup path -> result).
- [ ] Destructive actions do not default to "yes"; the irreversible full-delete
      carries the strongest confirm.
- [ ] Health is visibly read-only; advisory options are visibly non-blocking.

## E. Parity & accessibility

- [ ] The HTML<->TUI semantic parity critique is run and all divergence findings are
      resolved (`REFERENCE-INDEX.md` — 63/63 parity rows resolved across 7 surfaces).
- [ ] Legible under `NO_COLOR`/monochrome; no meaning by color alone; keyboard-only
      operability demonstrated.

## F. Explicit acknowledgment

- [ ] The user understands and accepts that **the approved screenshots become the
      frozen reference set** that every later phase (3-9) is visually regression-tested
      against, and that **no backend logic is written for any surface before this
      approval** (DLV-05).

## Reference set

- Complete index: `.planning/design/REFERENCE-INDEX.md` (all 50 screens x 2 media =
  100 PNGs, per-surface FIELDS.md/parity.json/CRITIQUE.md links, computed counts,
  cross-surface gate summary).
- Per-surface detail:
  - `.planning/design/create-flow/{FIELDS.md,parity.json,CRITIQUE.md}`
  - `.planning/design/git-screen/{FIELDS.md,parity.json,CRITIQUE.md}`
  - `.planning/design/identity-manager/{FIELDS.md,parity.json,CRITIQUE.md}`
  - `.planning/design/global-ssh/{FIELDS.md,parity.json,CRITIQUE.md}`
  - `.planning/design/global-git/{FIELDS.md,parity.json,CRITIQUE.md}`
  - `.planning/design/health/{FIELDS.md,parity.json,CRITIQUE.md}`
  - `.planning/design/fixer/{FIELDS.md,parity.json,CRITIQUE.md}`

## No backend logic before approval

No backend logic is written for any surface until a sign-off line is added below by
the human checkpoint (02-12), in the exact format: bold `APPROVED`, immediately
followed by a colon, then a space, the date, the word "by", and the approver — i.e.
`**APPROVED` + `:` + ` <date> by <user-supplied approver>`. This line is
intentionally absent from this scaffold, and the approver name is NOT inferred by
the executor — it must be a user-supplied string at the 02-12 checkpoint.
Phase 3+'s plans `depends_on` this phase's completion (a process/plan-ordering
guarantee); the phase also carries a runtime-checked complement for the dummy itself
(DLV-05's no-backend ALLOWLIST, `internal/dummytui/nobackend_test.go`, reconfirmed by
02-11 Task 1).
