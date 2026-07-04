# Interactive demo redesign spec — checkpoint-1 feedback round 2 (2026-07-04)

Produced by the ui-ux-designer agent from the user's round-2 rejection + PRD anchors
(SHELL-01, SSHUI-01..05, TEST-01/02, STORE-01, GITUI-01..05, MGR-01..08, GSSH-01,
GGIT-01, HLTH-01..06, FIX-01/02). Implemented in `mockup-src/src/demo/`.

## Root defect

Navigation lived in the footer while the header was a passive breadcrumb — the exact
inversion of the k9s/lazygit/Textual frame. Everything else cascaded from that.

## 1. App frame

- Header, one row, three zones: brand `gitid` · nav tabs `1 Identities · 2 Global SSH
  · 3 Global Git · 4 Doctor` (flat square tabs, active = reverse video; number is part
  of the label) · right health chip `N ids · ! w · ✗ e` — clicking the chip jumps to
  Doctor. Fixer is NOT a tab (FIX-02: re-homed into Doctor).
- Thin dim sub-header line under the tabs = breadcrumb (`Identities › New identity ›
  Test connection`).
- Footer keybar: contextual actions ONLY + reserved `Enter · Esc · ? · Ctrl+P · q`.
  No navigation, no vim keys (j/k/v dropped; arrows + mouse).
- StatusLine stays its own transient-feedback region.

## 2. Identities (live master-detail)

- Two panes ~38/62. Sidebar: inline legend line `S ssh · G git  ✓ ok ! attn ✗ broken`;
  rows = tone glyph + name + S/G capability pips + one-line ellipsized note. Full
  MGR-02 state word appears ONLY in the detail header chip; full table in `?` help.
- 8-state mapping: tone = health (✓ green / ! yellow / ✗ red); pips = capability
  (`✓` wired / `–` none / `✗` broken) per S and G slots.
- Selection move (arrows/click) renders the detail IMMEDIATELY — no Enter, no view
  switch. Right pane also hosts create/edit/clone forms and ceremonies; sidebar stays
  visible.
- Detail: SSH section first; Git section or `[Configure now]`; read-only "Global
  baseline" strip with jump to Global Git (GITUI-01 kept intact); per-identity
  findings sub-panel with inline `Fix…` (compressed ceremony in-pane).

## 3. Create wizard (≤4 pane-states, slim `Step n/4` dots)

1. SSH: Provider (Autocomplete freeSolo: github.com/gitlab.com/bitbucket.org,
   editable) → alias prefix → computed Host alias (editable default `prefix.provider`)
   → real hostname → port(443) → Algorithm as a compact Select (ed25519 recommended
   default; others rarely change — keeps the pane one screen) + live Host-block
   preview.
2. Test: two stages, verbatim commands, consistent flag order
   (`ssh -T -F <tmp> -p <port> -i <key> git@<hostname>`, then
   `ssh -G -F <tmp> <alias> | grep identityfile` — stage 2 has no `-i` BY DESIGN: it
   proves the config supplies the key). Failure path with retry.
3. Git identity + match strategy MERGED: `[Configure now]`/`[Skip]` buttons; author
   fields + strategy Select + DUAL live preview (fragment file content AND the
   `~/.gitconfig` includeIf block).
4. Review + confirm inline = ceremony state A (backup path promised inline) → result
   state B.

## 4. Global SSH / Global Git

- Master-detail; option rows carry tone glyph + risk chip + apply checkbox; footer
  `Apply selected` → ceremony. Advisory, never blocking.
- Global SSH gets sub-tabs `[Options] [Storage & preview]` — Storage = STORE-01 dual
  strategy (sentinel block in ~/.ssh/config vs gitid-owned ~/.ssh/config.d/gitid.config
  via one `Include` line near the top) with a resulting-config preview per strategy;
  switching layouts walks the ceremony (STORE-03 migration is a write).
- Global Git keeps the main-vs-master highlight; sentinel-preserving preview.

## 5. Doctor (absorbs Fixer)

- Left list: SSH section → per-identity subgroups (+ `global`), then Git section.
  Severity contract locked: `~ info` cyan · `! warning` yellow · `✗ error/critical`
  red (word disambiguates; never ✗ for warning).
- Right detail: explanation + family + `[Fix this]` (only when suggestedFix exists) →
  compressed ceremony in-pane; success removes the finding live and decrements the
  header counts (state healing).
- `Fix all (n)` walks each fix through the same ceremony with a `k / n fixed` counter.

## 6. Ceremony compression (2 states)

- A: preview/diff + targets + inline backup PROMISE + confirm (typed word for
  destructive rewrites; affirmative never default-focused).
- B: result receipt — `✓ message`, `Wrote → file`, `Backed up → path`.

## 7. Keyboard model

`1..4` tabs · arrows move selection (live detail) · `←/→` sub-tabs · Enter activates
in-pane · Esc backs out one level (never destructive) · Tab/Shift+Tab fields ·
`?` help (key map + full legend) · `Ctrl+P` palette · `q` quit prompt · full mouse
support. Contextual accelerators (n/c/d/f) remain as footer hints only — every action
is also a real button.

## Requirements reconciliation

- GITUI-01 vs "show loose Git defaults in the git view": per-identity screen keeps
  writing per-identity values only; globals appear as a read-only inherited baseline
  strip with `Edit in Global Git → 3`.
- SHELL-02 said 5 views; superseded by the user's 4-view direction + FIX-02
  (fixer re-homed into Doctor). Divergence recorded here explicitly.

## includeIf `and` research (user question)

git includeIf supports ONE condition per section — there is no boolean operator.
Multiple `[includeIf]` sections pointing at the same path behave as OR. AND can be
approximated by nesting: an outer `gitdir:` includeIf includes an intermediate
fragment which itself contains a `hasconfig:` includeIf pointing at the real fragment
(conditional includes are processed recursively, depth-limited to 10). The mockup's
"both" strategy = two blocks = OR semantics; true AND would need the nesting trick.
