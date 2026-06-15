---
phase: "05"
slug: cli-surface-tui
status: approved
shadcn_initialized: false
preset: none
created: 2026-06-12
reviewed_at: 2026-06-12
tool: none
---

# Phase 05 — Terminal TUI Interaction Contract

> This phase has no graphical or web UI. This document is a **terminal TUI and
> CLI interaction contract** produced for a Bubble Tea / lipgloss application —
> the visual and interaction equivalent of a UI design spec for a terminal.
>
> All six UI-SPEC dimensions are mapped to terminal idioms:
> - **Spacing/layout** → lipgloss padding/margin values, panel structure, viewport sizing
> - **Typography** → emphasis levels (bold/faint/underline), hierarchy conventions, truncation rules
> - **Color** → severity-driven palette inherited from Phase 4, with mandatory NO_COLOR/non-TTY downgrade
> - **Copywriting** → screen titles, help-bar text, form labels, empty/loading/error/confirm copy
> - **Design system** → reusable lipgloss styles + bubbles components, shared keymap
> - **Accessibility** → NO_COLOR/non-TTY plain mode is the accessibility contract; symbol+color pairing
>
> Phase 4 established the CLI color tokens, glyph contract, and layout patterns.
> This spec inherits all of them and extends them with lipgloss-native equivalents
> for the TUI dashboard, identity list, detail views, and in-app forms.
>
> **Source: decisions pre-populated from:**
> - `05-CONTEXT.md` D-01..D-14 (locked behavioral decisions)
> - `04-UI-SPEC.md` (color tokens, glyph contract, severity model — reused verbatim)
> - `CLAUDE.md` Technology Stack table (exact import paths and versions)

---

## Design System

| Property | Value | Terminal Interpretation |
|----------|-------|------------------------|
| Tool | none | No shadcn/web component library — terminal TUI only |
| Preset | not applicable | N/A |
| Component library | charm.land/bubbles/v2 v2.1.0 | `list.Model`, `viewport.Model`, `textinput.Model`, `spinner.Model`, `help.Model`, `key.Binding` — all from `charm.land/bubbles/v2` |
| TUI framework | charm.land/bubbletea/v2 v2.0.7 | Elm-architecture event loop; import `charm.land/bubbletea/v2` (NOT `github.com/charmbracelet/bubbletea`) |
| Styling library | charm.land/lipgloss/v2 v2.0.3 | Named style tokens (see Style Tokens section); import `charm.land/lipgloss/v2` |
| CLI framework | github.com/spf13/cobra v1.10.2 | Cobra command tree + auto-registered `completion` subcommand |
| Icon library | none | Terminal glyphs only: `✓` `✗` `!` `•` `›` with ASCII fallbacks |
| Font | terminal monospace | Monospace is given by the terminal emulator; no font selection |

**Import path mandate (hard rule — CLAUDE.md):**
- `charm.land/bubbletea/v2` NOT `github.com/charmbracelet/bubbletea`
- `charm.land/lipgloss/v2` NOT `github.com/charmbracelet/lipgloss`
- `charm.land/bubbles/v2` NOT `github.com/charmbracelet/bubbles`

---

## Spacing Scale

> Terminal equivalent: lipgloss padding/margin values and blank-line rhythm.
> All values are character cells (columns/rows), not pixels.
> The 8-point CSS grid maps to: 1 cell ≈ 4px equivalent in layout intent.

| Token | Lipgloss Value | Terminal Usage |
|-------|---------------|----------------|
| xs | 0 | Between items within a group (no gap) |
| sm | 1 | Inner padding of list items; horizontal gap between label and value |
| md | 2 | Panel internal padding (top/bottom); vertical gap between sections |
| lg | 3 | Outer panel margin; space above screen title |
| xl | 1 blank row | Between major panels (dashboard family cards) |

**Panel layout rules:**
- Dashboard family cards: `lipgloss.Style.Padding(1, 2)` (1 row top/bottom, 2 cols left/right)
- List items: `lipgloss.Style.PaddingLeft(2)` to match Phase 4 CLI 2-space indent
- Help/footer bar: `lipgloss.Style.PaddingTop(1)` to separate from last content line
- Form fields: `lipgloss.Style.Padding(0, 1)` with `lipgloss.Style.MarginBottom(1)` between fields

**Responsive minimum-size behavior:**
- Minimum supported terminal: 80 columns × 24 rows
- If terminal is narrower than 80 columns: show a single-line warning
  `Terminal too narrow — resize to at least 80 columns` (plain, no color) and halt rendering
- If terminal is shorter than 24 rows: show the help bar and a truncation message
  `[display truncated — resize terminal]` in place of overflowing content
- Width detection: `tea.WindowSizeMsg` on startup; re-check on each `tea.WindowSizeMsg`

---

## Typography

> Terminal equivalent: text emphasis and hierarchy conveyed by ANSI attributes via
> lipgloss — bold, faint, underline — not font weight or point size.
> There is one typeface: the terminal's monospace font.

| Role | Lipgloss Convention | Attribute | Usage |
|------|--------------------|-----------|-|
| Screen title | `StyleTitle` | Bold | Top line of each screen: `gitid — <Screen Name>` |
| Section header | `StyleHeader` | Bold | Family name in dashboard; `Identity List`, `Identity Detail`, form name |
| Selected item | `StyleSelected` | Bold + reverse video | Highlighted row in list components |
| Body / primary | `StyleBody` | Default foreground | Description text, field values, identity names |
| Secondary | `StyleFaint` | Faint (lipgloss `.Faint(true)`) | Suggested-fix text, secondary metadata (port, alias), key paths |
| Label | `StyleLabel` | Bold | Form field labels; metadata key in detail view (`Name:`, `Email:`, etc.) |
| Passing check | `StylePass` | ColorPass foreground | `✓` glyph + description in dashboard families |
| Finding title | `StyleFinding` | Severity color foreground | `✗` / `!` glyph + title in dashboard families |
| Input active | `StyleInputActive` | Underline + ColorAccent border | Text input field when focused |
| Input inactive | `StyleInputInactive` | Default foreground, faint border | Text input field when blurred |
| Help key | `StyleHelpKey` | Faint + Bold | Keymap keys in footer bar: `q` `Esc` `Enter` |
| Help desc | `StyleHelpDesc` | Faint | Keymap descriptions in footer bar: `quit` `back` `select` |

**Truncation rules:**
- List item text: truncate at `(terminalWidth - 6)` columns with `…` suffix; never wrap
- Screen title: never truncate (always fits within 80 cols by construction)
- Finding title in dashboard: truncate at `(panelWidth - 4)` cols with `…`; detail pane shows full text
- Form labels: fixed width of 16 chars, right-padded with spaces; values fill remaining width

---

## Color

> Severity-driven palette inherited directly from Phase 4 (`04-UI-SPEC.md`).
> The six named tokens from Phase 4 are mapped to lipgloss here.
>
> MANDATORY downgrade rules (in precedence order):
> 1. `NO_COLOR` env var set (any non-empty value) → all lipgloss color disabled; plain text only
> 2. Non-TTY / piped output → all lipgloss color disabled; plain text only
> 3. TTY with 256-color support → full palette below
> 4. TTY with 16-color only → lipgloss auto-downsampling handles it (use ANSI-index colors only)
>
> Implementation: check `NO_COLOR` first; then check `os.Stdout.Stat()` mode bits
> for `os.ModeCharDevice`; pass a `bool colorEnabled` to all renderer functions
> (same pattern as Phase 4 `ansi()` helper, now via `lipgloss.Style.Renderer`).
>
> Lipgloss v2 `lipgloss.NewRenderer(os.Stdout)` respects the `NO_COLOR` env var and
> TTY detection automatically when constructed with the output writer — use this
> over manual flag threading wherever possible in the TUI context.

### Severity Color Tokens (inherited from Phase 4, mapped to lipgloss)

| Token name | ANSI index | Lipgloss call | Usage |
|------------|-----------|---------------|-------|
| `ColorCritical` | 1 (red) | `lipgloss.Color("1")` | Critical finding glyph + title; error badge |
| `ColorError` | 1 (red) | `lipgloss.Color("1")` | Error finding glyph + title |
| `ColorWarning` | 3 (yellow) | `lipgloss.Color("3")` | Warning finding glyph + title |
| `ColorInfo` | 6 (cyan) | `lipgloss.Color("6")` | Info finding glyph + title; info badge |
| `ColorPass` | 2 (green) | `lipgloss.Color("2")` | Passing check glyph + text |
| `ColorFaint` | default | `.Faint(true)` | Suggested-fix text; secondary metadata |
| `ColorBold` | default | `.Bold(true)` | Family/section headers |
| `ColorAccent` | 4 (blue) | `lipgloss.Color("4")` | Active form input border; focused panel border; `›` nav indicator |
| `ColorMuted` | 8 (bright black) | `lipgloss.Color("8")` | Footer help bar text; placeholder text in text inputs |

**60/30/10 terminal split:**
- 60% dominant: default terminal foreground — body text, identity names, field values, explanation text
- 30% secondary: `ColorFaint` / `ColorMuted` — fix text, key paths, footer bar, form placeholders
- 10% accent: severity colors (`ColorCritical`/`ColorError`/`ColorWarning`/`ColorInfo`/`ColorPass`) + `ColorAccent`
  — reserved for: finding glyphs + titles, passing check glyphs, active input border, focused panel border, nav indicator `›`

**Accent reserved for (explicit list):**
1. `✓` glyph + finding title text in the doctor dashboard (severity colors)
2. `✗` / `!` glyph + finding title text in the doctor dashboard
3. Active/focused text input border
4. Focused panel border in multi-panel layouts
5. `›` navigation indicator on selected list item
6. Spinner animation (loading state) — `ColorAccent`

**Colorblind-safe rule (MANDATORY):**
Color is NEVER the sole carrier of meaning. Every severity level is also conveyed by:
- A glyph prefix (`✓` `✗` `!`) — or ASCII fallback (`OK` `FAIL` `!`) when `$TERM == "dumb"`
- A text label (`[critical]` `[warning]` `[info]`) inline in finding titles for non-error severities
- Panel or section header text that names the family

---

## Glyph Contract

> Inherited from Phase 4. Extended with TUI-specific navigation glyphs.

| Semantic | Glyph (UTF-8) | ASCII fallback | When used |
|----------|--------------|----------------|-----------|
| Passing check | `✓` | `OK` | Dashboard family — check passed |
| Failing check | `✗` | `FAIL` | Dashboard family — finding (critical/error/warning) |
| Advisory finding | `!` | `!` | Dashboard family — info-severity finding |
| Fixable marker | `[fix]` | `[fix]` | Same in both modes; shown next to auto-fixable findings |
| Navigation indicator | `›` | `>` | Selected list item; active nav stack level |
| Bullet | `•` | `*` | Identity list items; multi-value metadata |
| Loading | spinner chars | `...` | Family loading state in dashboard (bubbles/v2 spinner) |
| Incomplete identity | `~` | `~` | Identity list item when reconstruction is partial |

**Glyph selection rule:** Use UTF-8 glyphs unless `$TERM == "dumb"` or `$TERM` is unset.
A pipe does NOT degrade glyphs (consistent with Phase 4). Color degrades independently.

---

## Screen and Component Inventory

> One focused screen at a time. Navigation uses a view-stack (D-12).

### Screen 1: Doctor Dashboard (home screen — TUI-01)

**Title:** `gitid — Doctor Dashboard`

**Layout:** Vertical list of family panels, each rendered as a bordered lipgloss block.
Full-screen viewport; families stream in as async `tea.Cmd` results complete.

**Family panels (fixed order — inherited from Phase 4):**
1. Dependencies
2. Permissions
3. Coherence
4. Orphans
5. Signing
6. Agent
7. Baseline

**Loading state per family:**
```
=== Dependencies ===
  [spinner] checking...
```
Spinner: `bubbles/v2 spinner.Model` with `spinner.Dot` style; `ColorAccent` foreground.
The spinner replaces finding rows until that family's `tea.Cmd` returns results.

**Loaded panel (pass):**
```
=== Dependencies ===
  ✓ ssh present (OpenSSH 9.8p1)
  ✓ ssh-keygen present
  ✓ git present (2.44.0)
```

**Loaded panel (with findings):**
```
=== Permissions ===
  ✓ ~/.ssh directory: 700
  ✗ ~/.ssh/gitid_work: 644 (expected 600) [critical]
    Private key has group or world read permission.
    fix: chmod 0600 ~/.ssh/gitid_work  [fix]
```

Finding display in the TUI dashboard:
- Title line rendered via `StyleFinding` with severity color
- Explanation rendered via `StyleBody` with 4-space indent (lipgloss `PaddingLeft(4)`)
- Fix line rendered via `StyleFaint` with 4-space indent
- `[fix]` badge rendered via `StyleBody` inline after the fix line

**Footer fix hint (when fixable findings exist):**
```
  N fix(es) available — run 'gitid doctor --fix' to apply
```
Rendered via `StyleFaint`. This replaces the interactive gate from the CLI (per D-11).

**Refresh:** `r` key triggers a new async run of all six families from scratch.
While refreshing: spinner shown in each family panel; previous results cleared.

**Navigation from dashboard:**
- `Enter` on the dashboard → Identity List (D-12)
- `i` shortcut → Identity List
- `q` → quit

---

### Screen 2: Identity List (TUI-02)

**Title:** `gitid — Identities`

**Component:** `bubbles/v2 list.Model` — one item per reconstructed identity.

**List item format (single line):**
```
  › personal    github.com:22    ~/.ssh/gitid_personal    ✓
```
Columns (tab-aligned within the terminal width):
1. `›` indicator (ColorAccent) + identity name (StyleBody bold on selected)
2. `provider:port` (StyleFaint)
3. key path (StyleFaint, truncated at available width - 6 with `…`)
4. Status glyph: `✓` (ColorPass) if coherent, `~` (StyleFaint) if incomplete, `✗` (ColorError) if doctor found findings for this identity

**Empty state:**
```
  No identities found.
  Run 'gitid identity add' or press 'a' to create your first identity.
```
Rendered in the center of the list area; `StyleFaint` for the second line.

**Actions from Identity List:**
- `Enter` → Identity Detail for selected item
- `a` → Create Identity form
- `Esc` → back to Dashboard
- `d` / `Delete` → show inline handoff message (see Handoff Copy below)
- `r` → rotate shortcut — show inline handoff message

---

### Screen 3: Identity Detail

**Title:** `gitid — Identity: <name>`

**Layout:** Two-column metadata display (label: value) via lipgloss table layout.
Label column width: 16 chars fixed. Value column fills remaining terminal width.

**Fields displayed:**
```
  Name:             personal
  Git Name:         Ramon Colina
  Git Email:        user@example.com
  Provider:         github.com
  Port:             22
  SSH Alias:        personal.github.com
  Key Path:         ~/.ssh/gitid_personal
  Match Strategy:   gitdir:~/git/personal/
  Signing:          enabled
```
Labels use `StyleLabel` (bold). Values use `StyleBody`.
Key path uses `StyleFaint` (secondary importance).

**Doctor summary for this identity:**
If the dashboard has findings for this identity's SSH alias or gitconfig fragment, show them
below the metadata block:
```
  Health:
    ✓ all checks passed
```
or
```
  Health:
    ✗ IdentityFile does not exist (error)
    ✗ key not loaded in agent (warning)
```

**Actions from Identity Detail:**
- `e` → Update Identity form
- `h` → Add Account (host alias) form
- `c` → Copy pubkey (show copy confirmation inline + upload instructions)
- `d` / `Delete` → show inline handoff copy (see Handoff Copy below)
- `r` → rotate handoff copy
- `Esc` → back to Identity List

---

### Screen 4: Create Identity Form

**Title:** `gitid — Create Identity`

**Component:** sequential `bubbles/v2 textinput.Model` fields, one focused at a time.
Advance with `Tab` / `Enter`; back with `Shift+Tab`.

**Fields (in order):**
```
  Identity Name    [                    ]  e.g. personal
  Git Name         [                    ]  e.g. Ramon Colina
  Git Email        [                    ]  e.g. user@example.com
  Provider         [github.com          ]  hostname
  Port             [22                  ]  SSH port
  SSH Alias        [                    ]  leave blank to use provider
  Match Strategy   [gitdir:             ]  gitdir: or hasconfig:
  Signing          [y]  enable commit signing (y/n)
```

Label column: 16 chars, `StyleLabel`. Input field: lipgloss-bordered box, `StyleInputActive` when focused, `StyleInputInactive` when blurred. Hint text (after field): `StyleFaint`.

**Validation errors** (shown inline below the offending field, before proceeding):
```
  Identity Name    [work-invalid!       ]
  ! Name must match [a-z0-9][a-z0-9_-]* (no spaces or uppercase)
```
Error text: `StyleFinding` with `ColorWarning`. One line max.

**Prove-before-write screen (dedicated screen — D-04):**
After all fields are filled and user presses `Enter` on the last field, transition to the
Prove-Before-Write screen (Screen 6) before any write occurs.

---

### Screen 5: Update Identity Form

**Title:** `gitid — Update Identity: <name>`

Same layout as Create Identity form, but:
- Fields pre-populated with current values
- `Identity Name` field is read-only (shown with `StyleFaint`, no cursor, not focusable)
- Label shows `Name (immutable):` for the name field
- On submit → Prove-Before-Write screen (Screen 6)

---

### Screen 5b: Add Account (Host Alias) Form

**Title:** `gitid — Add Account: <identity-name>`

**Fields:**
```
  Identity         [personal            ]  (pre-filled, read-only)
  Provider         [github.com          ]  new host alias target
  SSH Alias        [                    ]  e.g. work.github.com
  Port             [22                  ]
  Match Strategy   [gitdir:             ]
```

Same styling rules as Create Identity form. On submit → Prove-Before-Write screen (Screen 6).

---

### Screen 6: Prove-Before-Write (shared — D-04)

**Title:** `gitid — Confirm: <action description>`

> This screen is the TUI equivalent of the CLI's two-phase test flow. It is
> NON-NEGOTIABLE per D-04: every in-app mutation must show the exact command run
> and its real output, note the timestamped backup, and require explicit confirm.

**Layout (top to bottom):**

```
  gitid — Confirm: Create identity "personal"

  Phase 1: Testing key authentication
  Command: ssh -i ~/.ssh/gitid_personal -o IdentitiesOnly=yes -T git@github.com

  [spinner] running...
```

After phase 1 completes:
```
  Phase 1: Testing key authentication
  Command: ssh -i ~/.ssh/gitid_personal -o IdentitiesOnly=yes -T git@github.com
  Output:  Hi username! You've successfully authenticated, but GitHub does not
           provide shell access.
  Status:  ✓ authenticated

  Phase 2: Testing resolved config
  Command: ssh -G personal.github.com | grep identityfile
  [spinner] running...
```

After both phases complete:
```
  Phase 1: ✓ authenticated
  Phase 2: ✓ resolves to ~/.ssh/gitid_personal

  Backup:  ~/.ssh/config.bak.20260612T143022
  Action:  Write 4 artifacts (SSH Host block, includeIf, fragment, allowed_signers)

  Write changes? [Enter to confirm / Esc to cancel]
```

**Styling:**
- Phase labels: `StyleHeader` (bold)
- `Command:` label: `StyleLabel` (bold); command text: `StyleBody` (monospace, no wrap — scroll with viewport if long)
- `Output:` label: `StyleLabel`; output text: `StyleFaint` (dim — tool output is secondary)
- `Status:` line: pass glyph `✓` in `ColorPass` + `StyleBody`; failure `✗` in `ColorCritical` + `StyleBody`
- `Backup:` label: `StyleLabel`; path: `StyleFaint`
- `Action:` label: `StyleLabel`; description: `StyleBody`
- Confirmation prompt line: `StyleBody` — shown only after both phases complete

**Failure path (test failed):**
```
  Phase 1: ✗ authentication failed [critical]
  Output:  git@github.com: Permission denied (publickey).

  Cannot proceed — SSH authentication failed.
  Press Esc to go back and review the identity configuration.
```
No confirm prompt shown. Only `Esc` is active.

**Loading state during test:**
- Spinner (bubbles/v2 `spinner.Dot`, `ColorAccent`) shown next to phase label
- Other key bindings disabled except `Esc` (cancel test in progress)

---

### Copy Pubkey Action (inline — not a full screen)

Triggered by `c` from Identity Detail. Does not replace the detail screen; renders
an overlay panel or inline section below the metadata.

**Inline confirmation block:**
```
  Public key copied to clipboard.
  Key: ssh-ed25519 AAAA...truncated...   [copy again: c]

  Upload instructions for github.com:
  Authentication:
    1. Go to github.com → Settings → SSH and GPG keys → New SSH key
    2. Title: "gitid: personal"
    3. Key type: Authentication Key
    4. Paste the public key

  Signing:
    1. Go to github.com → Settings → SSH and GPG keys → New SSH key
    2. Title: "gitid: personal (signing)"
    3. Key type: Signing Key
    4. Paste the public key

  Press any key to dismiss
```

**Styling:**
- `Public key copied to clipboard.` → `StylePass` (ColorPass foreground)
- `Key:` truncated pubkey → `StyleFaint`
- Section headers `Authentication:` / `Signing:` → `StyleHeader` (bold)
- Numbered steps → `StyleBody`
- `Press any key to dismiss` → `StyleFaint`

---

### Handoff Messages (CLI hand-off — D-03, D-11)

When the user presses `d` (delete) or `r` (rotate) from Identity List or Identity Detail,
show a non-blocking inline message (does NOT navigate away):

**Delete handoff:**
```
  Delete and Rotate run from the CLI to preserve the full safe-write flow.
  To delete:  gitid identity delete <name>
  Press Esc or any key to dismiss
```

**Rotate handoff:**
```
  Delete and Rotate run from the CLI to preserve the full safe-write flow.
  To rotate:  gitid rotate <name>
  Press Esc or any key to dismiss
```

**Doctor fix handoff** (dashboard footer):
```
  N fix(es) available — run 'gitid doctor --fix' to apply
```

All handoff messages use `StyleFaint` for the explanatory line; `StyleBody` for the command.

---

## Keymap Contract (D-13 — LOCKED)

> Declared as `key.Binding` values in a shared `keymap.go` file in `tui/`.
> All screens share the global bindings; screen-local bindings are additive.

### Global Bindings (all screens)

| Key | Binding name | Action | Help text |
|-----|-------------|--------|-----------|
| `q` / `ctrl+c` | `KeyQuit` | Quit application | `quit` |
| `Esc` | `KeyBack` | Pop navigation stack / cancel / dismiss | `back` |
| `?` | `KeyHelp` | Toggle expanded help | `help` |
| `r` | `KeyRefresh` | Refresh current screen | `refresh` |

### Navigation Bindings (list/selection screens)

| Key | Binding name | Action | Help text |
|-----|-------------|--------|-----------|
| `↑` / `k` | `KeyUp` | Move selection up | `up` |
| `↓` / `j` | `KeyDown` | Move selection down | `down` |
| `←` / `h` | `KeyLeft` | (reserved / scroll left) | `left` |
| `→` / `l` | `KeyRight` | (reserved / scroll right) | `right` |
| `Enter` | `KeySelect` | Select / confirm | `select` |
| `g` | `KeyTop` | Jump to top of list | `top` |
| `G` | `KeyBottom` | Jump to bottom of list | `bottom` |

### Identity List Bindings (Screen 2)

| Key | Binding name | Action | Help text |
|-----|-------------|--------|-----------|
| `a` | `KeyAdd` | Open Create Identity form | `add` |
| `d` / `Delete` | `KeyDelete` | Show delete handoff message | `delete` |
| `r` | `KeyRotate` | Show rotate handoff message | `rotate (CLI)` |

### Identity Detail Bindings (Screen 3)

| Key | Binding name | Action | Help text |
|-----|-------------|--------|-----------|
| `e` | `KeyEdit` | Open Update Identity form | `edit` |
| `h` | `KeyAddHost` | Open Add Account form | `add host` |
| `c` | `KeyCopy` | Copy pubkey + show upload instructions | `copy pubkey` |
| `d` / `Delete` | `KeyDelete` | Show delete handoff | `delete (CLI)` |
| `r` | `KeyRotate` | Show rotate handoff | `rotate (CLI)` |

### Form Bindings (Screens 4, 5, 5b)

| Key | Binding name | Action | Help text |
|-----|-------------|--------|-----------|
| `Tab` | `KeyNext` | Focus next field | `next field` |
| `Shift+Tab` | `KeyPrev` | Focus previous field | `prev field` |
| `Enter` | `KeySubmit` | Submit (on last field) | `submit` |
| `Esc` | `KeyBack` | Cancel form, go back | `cancel` |

### Prove-Before-Write Bindings (Screen 6)

| Key | Binding name | Action | Help text |
|-----|-------------|--------|-----------|
| `Enter` | `KeyConfirm` | Write all artifacts (only active after both tests pass) | `confirm write` |
| `Esc` | `KeyBack` | Cancel, discard, return to form | `cancel` |

---

## Help / Footer Bar Contract

Every screen shows a persistent help/footer bar at the bottom, rendered via
`bubbles/v2 help.Model`.

**Normal state (compact — D-13):**
```
q quit  Esc back  ↑↓/jk move  Enter select  ? help
```

**Expanded state (after `?`):**
Full keymap for the current screen, one binding per line:
```
q / ctrl+c   quit
Esc          back
↑↓ / j k    move up/down
Enter        select
a            add identity
d            delete (CLI)
r            refresh / rotate (CLI)
e            edit
c            copy pubkey
?            close help
```

**Styling:**
- All help bar text: `StyleHelpKey` for keys (bold + faint), `StyleHelpDesc` for descriptions (faint)
- Separator between key and description: 2 spaces
- Footer bar separated from content by `lipgloss.Style.PaddingTop(1)` and a `─` divider line
  using `lipgloss.Style.Border(lipgloss.NormalBorder(), true, false, false, false)`
- Divider border color: `ColorMuted`

---

## State / Transition Copy

> All copy is English-only per CLAUDE.md. No emoji. Format: `context: description`.

### Screen Titles

| Screen | Title |
|--------|-------|
| Dashboard | `gitid — Doctor Dashboard` |
| Identity List | `gitid — Identities` |
| Identity Detail | `gitid — Identity: <name>` |
| Create Form | `gitid — Create Identity` |
| Update Form | `gitid — Update Identity: <name>` |
| Add Account Form | `gitid — Add Account: <identity-name>` |
| Prove-Before-Write | `gitid — Confirm: <action>` |

### Loading States

| Context | Copy |
|---------|------|
| Dashboard family loading | `[spinner] checking...` (spinner from bubbles/v2) |
| Dashboard refreshing | `[spinner] refreshing...` (all families reset to loading) |
| Prove-Before-Write: phase 1 running | `[spinner] testing SSH authentication...` |
| Prove-Before-Write: phase 2 running | `[spinner] testing resolved config...` |
| Clipboard copy in progress | `[spinner] copying...` |

### Empty States

| Context | Copy |
|---------|------|
| Identity List — no identities | `No identities found.\nPress 'a' to create your first identity.` |
| Dashboard — all families pass | `✓ all checks passed` (below last family panel, `StylePass`) |
| Dashboard — no identities to check | `No identities configured — doctor checks limited to dependencies and baseline.` (`StyleFaint`) |

### Error States

| Context | Copy |
|---------|------|
| Dashboard family error | `  ✗ <family> check failed: <err>` (inline in family panel, `StyleFinding`/`ColorError`) |
| Prove-Before-Write: phase 1 fail | `✗ authentication failed [critical]\nOutput: <actual output>\nCannot proceed — SSH authentication failed.\nPress Esc to go back.` |
| Prove-Before-Write: phase 2 fail | `✗ resolved config check failed [error]\nOutput: <actual output>\nCannot proceed — config resolution failed.\nPress Esc to go back.` |
| Clipboard copy failed | `! clipboard copy failed [info]\n<error from atotto>\nKey is printed above — copy manually.` (`StyleFinding`/`ColorInfo`) |
| Write error | `✗ write failed: <err> [critical]\nNo changes were written. Press Esc to go back.` |
| Terminal too narrow | `Terminal too narrow — resize to at least 80 columns` (plain, no lipgloss) |

### Destructive / Mutation Actions

> Per D-03 and D-11: Delete and Rotate are CLI-only. The TUI only shows a handoff.
> In-app mutations (Create, Update, Add Account) MUST go through Screen 6.

| Action | Confirmation approach |
|--------|-----------------------|
| Create identity | Prove-Before-Write screen (Screen 6) with two-phase SSH test + explicit Enter |
| Update identity | Prove-Before-Write screen (Screen 6) |
| Add account | Prove-Before-Write screen (Screen 6) |
| Delete identity | Handoff message: `gitid identity delete <name>` — no in-app write |
| Rotate key | Handoff message: `gitid rotate <name>` — no in-app write |
| Doctor fixes | Footer note: `gitid doctor --fix` — no in-app apply |

### CLI-only Command Copywriting

These are the help strings and usage text for the new Cobra commands (CLI-01 / D-05, D-06, D-07):

| Command | `Use` | `Short` |
|---------|-------|---------|
| `gitid copy <name>` | `copy <name>` | `Copy the public key to the clipboard and print upload instructions` |
| `gitid host add` | `add` (under `host` group) | `Add a host alias (SSH account) to an existing identity` |
| `gitid rotate <name>` | `rotate <name>` | `Rotate the SSH key for an identity and re-test all artifacts` |
| `gitid completion bash` | (auto-registered by Cobra) | (auto-generated) |
| `gitid identity copy <name>` | `copy <name>` | `Copy the public key to the clipboard and print upload instructions` |

**`gitid copy` output format:**
```
Copied public key for "<name>" to clipboard.

Key: ssh-ed25519 AAAA...

Upload instructions for <provider>:
Authentication:
  1. Go to <provider> → Settings → SSH keys → New SSH key
  2. Title: "gitid: <name>"
  3. Key type: Authentication Key
  4. Paste the public key

Signing:
  1. Go to <provider> → Settings → SSH keys → New SSH key
  2. Title: "gitid: <name> (signing)"
  3. Key type: Signing Key
  4. Paste the public key
```
Styled: `Copied public key` line uses `ColorPass` on TTY; remainder is default foreground.
Follows Phase 4 TTY/NO_COLOR/piped rules (no color when piped).

**`gitid` (no-args) TUI launch:**
If stdout is not a TTY (piped), print:
```
gitid: no subcommand given. Run 'gitid --help' for usage.
```
and exit 1. Do NOT launch the TUI when output is piped.

---

## Copywriting Contract

> Consolidated reference for the planner and executor.

| Element | Copy |
|---------|------|
| Primary CTA (Create form) | `Create Identity` (confirm button label, also screen title verb) |
| Primary CTA (Update form) | `Update Identity` |
| Primary CTA (Add Account form) | `Add Account` |
| Primary CTA (Prove-Before-Write) | `Write changes? [Enter to confirm / Esc to cancel]` |
| Empty state — no identities | `No identities found.\nPress 'a' to create your first identity.` |
| Empty state — all doctor checks pass | `✓ all checks passed` |
| Loading — dashboard family | `[spinner] checking...` |
| Loading — prove-before-write | `[spinner] testing SSH authentication...` |
| Error — write failed | `✗ write failed: <err> [critical]\nNo changes were written. Press Esc to go back.` |
| Error — test phase 1 fail | `✗ authentication failed [critical]\nCannot proceed — SSH authentication failed.` |
| Error — clipboard fail | `! clipboard copy failed [info]\nKey is printed above — copy manually.` |
| Handoff — delete | `Delete and Rotate run from the CLI.\nTo delete: gitid identity delete <name>` |
| Handoff — rotate | `Delete and Rotate run from the CLI.\nTo rotate: gitid rotate <name>` |
| Handoff — doctor fix | `N fix(es) available — run 'gitid doctor --fix' to apply` |
| Non-TTY no-args message | `gitid: no subcommand given. Run 'gitid --help' for usage.` |
| Validation error — name | `Name must match [a-z0-9][a-z0-9_-]* (no spaces or uppercase)` |
| Validation error — email | `Enter a valid email address` |
| Backup notice | `Backup: <path>.bak.<timestamp>` |
| Clipboard copy success | `Copied public key for "<name>" to clipboard.` |

---

## Non-TTY / Piped Behavior Contract

> When stdout is not a TTY (CI, pipes, scripts), the TUI does NOT launch.
> All `gitid` subcommands remain fully functional in non-TTY mode.

| Condition | Behavior |
|-----------|----------|
| `gitid` (no args), non-TTY | Print `gitid: no subcommand given. Run 'gitid --help' for usage.` + exit 1 |
| Any subcommand, non-TTY | Normal Cobra execution; plain text output (no lipgloss color) |
| `gitid doctor`, non-TTY | Full Phase 4 CLI report; plain text; no interactive gate (existing behavior) |
| `NO_COLOR` set | All lipgloss color disabled, both in TUI and CLI output |
| `$TERM == "dumb"` | ASCII glyph fallbacks (`OK`/`FAIL`/`>`/`*`); no ANSI codes |

---

## Accessibility Contract

1. **No information conveyed by color alone.** Every severity level uses both a color AND a glyph prefix (`✓` `✗` `!`). All list states additionally carry text labels. A monochrome terminal is fully legible.
2. **Piped output is plain text.** `gitid doctor 2>&1 | grep FAIL` works. `gitid identity list 2>&1 | grep email` works.
3. **NO_COLOR is respected.** Any non-empty `NO_COLOR` value strips all ANSI codes and lipgloss color. Checked before TTY detection.
4. **All prompts show the default visually.** `[Enter to confirm / Esc to cancel]` — Esc is always the safe default (no write). The confirm key is Enter, making "do nothing" (Esc) safe by default.
5. **All destructive actions state consequences before confirm.** The Prove-Before-Write screen shows exactly which files will be written and that a backup will be created — before the user presses Enter.
6. **Visible help/footer bar at all times.** The compact keymap bar is always rendered at the bottom of every screen (D-13). `?` expands it.
7. **Minimum terminal size check.** If terminal is < 80×24, render a diagnostic message and halt TUI rendering rather than producing garbled layout.
8. **English only.** All output, labels, prompts, error messages (CLAUDE.md).

---

## Lipgloss Style Token Reference

> Concrete lipgloss v2 style declarations for the executor.
> All styles defined in a single `tui/styles.go` file.
> `renderer` is a `*lipgloss.Renderer` constructed with `lipgloss.NewRenderer(os.Stdout)`.

```go
// tui/styles.go — canonical style token declarations

var (
    StyleTitle = renderer.NewStyle().Bold(true)

    StyleHeader = renderer.NewStyle().Bold(true)

    StyleSelected = renderer.NewStyle().Bold(true).Reverse(true)

    StyleBody = renderer.NewStyle()  // default foreground

    StyleFaint = renderer.NewStyle().Faint(true)

    StyleLabel = renderer.NewStyle().Bold(true).Width(16)

    StylePass = renderer.NewStyle().Foreground(lipgloss.Color("2"))

    StyleFinding = renderer.NewStyle()  // color applied per-severity at call site

    StyleInputActive = renderer.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(lipgloss.Color("4"))

    StyleInputInactive = renderer.NewStyle().
        Border(lipgloss.NormalBorder()).
        BorderForeground(lipgloss.Color("8"))

    StyleHelpKey  = renderer.NewStyle().Faint(true).Bold(true)
    StyleHelpDesc = renderer.NewStyle().Faint(true)

    StylePanel = renderer.NewStyle().
        Padding(1, 2).
        Border(lipgloss.RoundedBorder()).
        BorderForeground(lipgloss.Color("8"))

    StylePanelFocused = renderer.NewStyle().
        Padding(1, 2).
        Border(lipgloss.RoundedBorder()).
        BorderForeground(lipgloss.Color("4"))
)

// Severity color helper — maps doctor.Severity to a lipgloss foreground color
func SeverityStyle(s doctor.Severity) lipgloss.Style {
    switch s {
    case doctor.SeverityCritical, doctor.SeverityError:
        return renderer.NewStyle().Foreground(lipgloss.Color("1"))
    case doctor.SeverityWarning:
        return renderer.NewStyle().Foreground(lipgloss.Color("3"))
    default: // info
        return renderer.NewStyle().Foreground(lipgloss.Color("6"))
    }
}
```

---

## Information Architecture

### Full Command Surface (CLI-01, D-05)

```
gitid
├── doctor [--fix] [--yes]
├── identity
│   ├── add [--name] [--email] [--provider] ...
│   ├── list
│   ├── test <name>
│   ├── update <name>
│   ├── delete <name>
│   ├── copy <name>          ← new in Phase 5 (D-06)
│   └── rotate <name>
├── baseline
│   ├── setup [--dry-run]
│   └── show
├── host
│   └── add                  ← alias to identity add (add-account mode) (D-07)
├── copy <name>              ← top-level alias (D-05/D-06)
├── rotate <name>            ← top-level alias (D-05)
└── completion
    ├── bash
    ├── zsh
    └── fish
```

**Top-level `gitid` (no args) → launch TUI** (when stdout is a TTY).

### TUI Navigation Stack (D-12)

```
Dashboard (home)
└── Identity List       [Enter from Dashboard, or 'i']
    └── Identity Detail [Enter on list item]
        ├── Create Form [not from detail; 'a' from list]
        ├── Update Form ['e' from detail]
        └── Add Account ['h' from detail]
            └── Prove-Before-Write [after form submit]
                └── (write) → back to detail / list
```

Stack behavior:
- Each `Enter` / form-open pushes a new view onto the stack
- `Esc` always pops one level (returns to parent)
- Quitting (`q`) from any level exits the application

---

## Registry Safety

| Registry | Blocks Used | Safety Gate |
|----------|-------------|-------------|
| N/A — terminal TUI only | none | Not applicable |

No shadcn. No third-party component registry. All TUI components come from
`charm.land/bubbles/v2` (official Charm library, same trust level as the framework).

---

## Checker Sign-Off

> Adapted for terminal TUI phase: each dimension maps to a terminal-output and interaction quality check.

- [ ] Dimension 1 Copywriting: all screen titles, CTA labels, empty/loading/error states, handoff messages, form validation copy, and CLI help strings declared; English-only; no emoji; all destructive actions covered
- [ ] Dimension 2 Visuals: screen inventory complete (6 screens + 1 inline action); layout rules per screen declared; panel padding/border style tokens provided; list item format specified; help bar format specified
- [ ] Dimension 3 Color: 9 named tokens declared with ANSI index and lipgloss calls; TTY/NO_COLOR/piped downgrade rules in precedence order; color never sole carrier of meaning; 60/30/10 split documented; accent reserved-for list explicit
- [ ] Dimension 4 Typography: 12 named lipgloss style roles declared; concrete lipgloss v2 declarations provided in `tui/styles.go` reference; truncation rules per context; label column width fixed
- [ ] Dimension 5 Spacing: 5-token scale declared in character cells; per-panel padding values specified; minimum terminal size and failure behavior declared; viewport sizing rules stated
- [ ] Dimension 6 Registry Safety: N/A (bubbles/v2 is official Charm library, same provenance as framework); no third-party registry

**Approval:** pending

---

## Source Traceability

| Decision | Source |
|----------|--------|
| Drill-down stack navigation (Dashboard → List → Detail → Form; Esc pops) | 05-CONTEXT.md D-12 (LOCKED) |
| Keymap: arrows + vim j/k/h/l + q/Esc/Enter/?/r + visible help bar | 05-CONTEXT.md D-13 (LOCKED) |
| Async/progressive dashboard: six families stream in as tea.Cmd msgs | 05-CONTEXT.md D-09 (LOCKED) |
| TUI-native lipgloss view over structured doctor.Run findings | 05-CONTEXT.md D-10 (LOCKED) |
| In-app forms: Create, Update, Add-account, Copy; Delete+Rotate CLI-only | 05-CONTEXT.md D-02, D-03 (LOCKED) |
| Doctor fixes shown in dashboard, applied via CLI only | 05-CONTEXT.md D-11 (LOCKED) |
| Prove-before-write: exact cmd+output + explicit pre-write confirm | 05-CONTEXT.md D-04 (LOCKED) |
| Prove-before-write presentation: dedicated screen (Screen 6) | 05-CONTEXT.md D-04 — Claude's Discretion |
| 6 severity color tokens with lipgloss ANSI-index mapping | 04-UI-SPEC.md Color section (adopted verbatim) |
| Glyph contract (✓/✗/!/•) with ASCII fallbacks | 04-UI-SPEC.md Glyph Contract (adopted verbatim) |
| 7 doctor family order: Dependencies, Permissions, Coherence, Orphans, Signing, Agent, Baseline | 04-UI-SPEC.md Family ordering (adopted verbatim) |
| Family header format `=== Name ===` → mapped to StyleHeader in TUI panels | 04-UI-SPEC.md Family header format |
| Phase 4 CLI color tokens mapped to lipgloss lipgloss.Color("N") calls | 04-UI-SPEC.md Phase 5 Portability Notes (direct mapping) |
| charm.land/bubbletea/v2 v2.0.7, lipgloss/v2 v2.0.3, bubbles/v2 v2.1.0 | CLAUDE.md Technology Stack table (LOCKED) |
| Vanity import paths — NOT github.com/charmbracelet/* | CLAUDE.md Technology Stack table (LOCKED) |
| NO_COLOR / non-TTY plain-mode rules | 04-UI-SPEC.md D-08 (LOCKED); accessibility contract |
| English-only all output | CLAUDE.md Language section (LOCKED) |
| TUI does not launch when stdout is non-TTY | 05-CONTEXT.md Integration Points |
| Minimum terminal 80×24 + diagnostic message | Claude's Discretion (standard TUI practice) |
| `tui/styles.go` as canonical style token file | Claude's Discretion (consolidation pattern) |
| Top-level aliases: copy, rotate, host add | 05-CONTEXT.md D-05, D-06, D-07 (LOCKED) |
| Cobra auto-registered completion subcommand | CLAUDE.md §Cobra + shell completion; 05-CONTEXT.md D-08 |
