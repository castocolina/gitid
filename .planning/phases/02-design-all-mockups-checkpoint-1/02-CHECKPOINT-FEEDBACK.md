# 02-12 Checkpoint Feedback

## Round 3+ / TUI replan (2026-07-04) — user directives between rounds

After the round-3 web-demo fixes, the user redirected the checkpoint tail: remove the
stale static reference set (100 PNGs, GALLERY, dummy-nav frame dumps, parity/manifest
files, the static-screen Go dummy — commit 7453561), update the specs (REFERENCE-INDEX
now names the interactive demo authoritative), and replan the Go deliverable as an
EXECUTABLE live TUI demo mirroring the web demo (02-13-PLAN.md, commit badee17).
Framework re-confirmed with the user: Bubble Tea v2 retained; github.com/grindlemire/go-tui
evaluated and rejected as too immature.

02-13 executed (6ea89a5, 5fb05f8, 3a63249, 53a3514), then hardened through a
three-reviewer convergence loop with the user-set bar "every interaction and screen
must match the HTML live demo": superpowers fresh-context code review,
agent-ui-ux-designer frame inspection at 100x30, and Codex cross-vendor source diff.
Three fix batches landed (f8f962c mouse routing + backup-path + purity; 250c1b6
divider/section structure + truncation cues + inline edit preview + focusable wizard
buttons, Ctrl+S removed; 0169ae7 full click-target + focus-ring parity, checkbox/radio
click semantics, footer honesty, PTY mouse+apply e2e). Final verdicts: Codex MATCHED
(zero residuals), UX designer MATCHED, code reviewer ready-to-present (3 cosmetic
minors logged in deferred-items.md). All gates observed green at 0169ae7: unit -race,
lint 0 issues, e2e suite incl. TestDummyDemo_LiveWalk + TestDummyDemo_MouseAndGitApply,
gate-no-backend-files, import allowlist. Checkpoint re-presented with BOTH live demos;
APPROVAL.md remains unsigned.

---

## Round 2 (2026-07-04) — verdict: changes requested (structural)

User feedback: the round-1 demo missed the frame concept — no common header with
NAVIGATION (Identities / Global SSH / Global Git / Doctor; Fixer is a consequence,
not a view), footer mixing navigation with screen actions, vim affordances (j/k),
long state labels instead of icons/badges + legend, detail requiring Enter instead
of live master-detail, a 7-card create wizard instead of a compact provider-first
form, sparse split git screens missing the loose-default Git properties, the PRD's
STORE-01 SSH storage strategy missing entirely, and health visibility not pervasive.

Response: full redesign per 02-REDESIGN-SPEC.md (produced with the ui-ux-designer
agent + the mui skill, anchored to SHELL-01/SSHUI-01/STORE-01/GITUI-01/FIX-02):
numbered header nav tabs + clickable per-severity health chip, thin breadcrumb line,
contextual-only footer (no vim keys), live sidebar↔detail on the Identities view
(tone glyph + S/G capability pips + inline legend; full legend in ?-help), forms and
ceremonies render in the detail pane with the sidebar visible, create wizard
compressed to 4 pane-states (provider autocomplete → joined Host alias → hostname →
port → algorithm Select + live preview; two-stage test with consistent flag order
and an explicit no--i-in-stage-2 rationale; merged Git+strategy step with DUAL
preview; review+confirm inline), Global SSH gains the STORE-01 Storage & preview
sub-tab with a migratable sentinel↔Include layout, Global Git master-detail with
apply checkboxes, Doctor absorbs the Fixer (per-identity grouping, inline fix
ceremony, Fix all with counter and live healing), ceremony compressed to 2 states
(backup promised in confirm, receipted in result).

Playwright-driven verification fixed 3 more interaction bugs before re-presenting:
Enter swallowed by focused text fields (now falls through as the pane's primary
action unless a component consumed it), the `F` fix-all key wired only as a click
target, and the Doctor status line reusing the frozen fixture copy that still said
"Fixer (key 5)". Gates: pnpm typecheck clean, pnpm build OK (verify-routes 52
routes), make test EXIT=0, make lint 0 issues; the 50 static reference routes are
untouched.

Open research answer recorded in 02-REDESIGN-SPEC.md: git includeIf has no boolean
AND — same-path multiple sections behave as OR; AND is approximated by nesting
conditional includes (recursion depth 10).

---

## Round 1 (2026-07-03)

## Verdict: changes requested (not approved)

User feedback (translated/summarized from the checkpoint review):

1. `GALLERY.html` is useful to see the mockups but is **non-interactive** — many
   screens collapsed into one page, not a demo.
2. `http://localhost:8747` landed on a flat, prose-only page with nothing actionable.
3. As requested since the PRD phase, the checkpoint needs an **interactive but dummy
   end-to-end demonstration**: navigate all screens live (pressing `1,2,3,4…`,
   `Ctrl+P`), emulate adding an SSH key, testing it, advancing to Git details,
   reviewing, saving or cancelling; list identities with flag indicators
   (complete/incomplete, git-associated, health ok/findings); select an identity and
   see what exactly is wrong or fine; doctor + fix options; delete with feedback;
   clone/duplicate; global SSH and global Git screens with edit options and warnings
   that redirect to doctor/fixer; help — all with dummy data and actions that allow
   navigating every option.

## Response (this fix-pass)

Built an **interactive demo** into the mockup SPA at the index route (`/`):
`mockup-src/src/demo/` — a TUI-style navigation stack + reducer over dummy state
seeded from `recipeFixtures.ts` (the same single source the static mockups and the Go
TUI dummy render), with the TUI's exact key map and a shared `MutationCeremony`
component so every "write" walks the same preview → confirm → backup → result
ceremony. Coverage: create wizard (incl. simulated two-stage test with a failure
toggle), live identity list with state/git/findings flags, per-identity detail with
action menu / clone / new key / delete (scope choice + typed destructive confirm),
global-ssh and global-git advisory apply flows with doctor-redirect banners, health
scan → finding detail → fixer hand-off, fixer with per-fix diffs (the flagship
IdentitiesOnly rewrite requires typing the Host name) and state healing (fixing the
legacy fragment flips that identity back to `complete`), `?` help overlay, and a
`Ctrl+P` palette that also opens each of the 50 static reference screens.

Bugs found and fixed **by driving the demo with Playwright** before re-presenting:

- Global hotkeys leaked the pressed character into the newly-focused field
  (`n` typed an "n" into the alias input) — global map now `preventDefault()`s.
- Radios/checkboxes are `<input>`s: after clicking one, Enter/y were swallowed by the
  text-field guard and wizards silently stopped advancing — toggles now fall through
  (space/arrows stay native).
- `g` on an identity detail / on the home selection targeted the FIRST identity
  instead of the contextual one — both screens now claim `g` locally.
- Fixer batch note used the static fixture count ("4 fixes") after fixes were
  applied — now computed from live state.

## Verification (observed)

- `pnpm typecheck` clean; `pnpm build` OK (includes `verify-routes: OK`, 52 routes).
- Playwright walkthrough of every flow above — 20+ screenshots under
  `.playwright-mcp/walkthrough/` (sent to the user at re-present).
- Static reference route spot-check: `#/identity-manager/list-populated` still renders
  breadcrumb + `[SIG-IM-LIST-POPULATED-8-LABEL]` (the 50 capture-gated routes are
  untouched; only `/` changed owner from the internal shell-demo page, now at
  `/_shell/shell-demo`, which no manifest references).
- `make test` EXIT=0 · `make lint` 0 issues · `make dummy-nav-e2e` EXIT=0
  (incl. gate-no-backend-files OK against main 321884c).

## Status

Re-presenting checkpoint 02-12 with the interactive demo. APPROVAL.md remains
unsigned pending the user's verdict.
