# 02-12 Checkpoint Feedback — Round 1 (2026-07-03)

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
