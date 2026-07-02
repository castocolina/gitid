---
phase: "04"
slug: doctor
status: approved
reviewed_at: 2026-06-11
shadcn_initialized: false
preset: none
created: 2026-06-11
tool: none
---

# Phase 04 — Terminal CLI Interaction Contract

> This phase has no graphical or web UI. This document is a **terminal-output and
> interaction contract** — the CLI equivalent of a UI design spec. It answers:
> "What must the executor render, in what order, with what copy, so that
> `gitid doctor` is legible, safe, and consistent with the existing `gitid` command
> surface?"
>
> Phase 4 introduces **the first ANSI color** in the gitid CLI, locked by D-08:
> color on a TTY, auto-plain when piped/redirected, respect `NO_COLOR`. Every color
> decision is defined as a named token below. The tokens are designed to be adopted
> directly by the Phase 5 Bubble Tea / lipgloss TUI dashboard without redesign.
>
> Each standard UI-SPEC dimension is reinterpreted for a terminal CLI. Web/GUI-only
> sub-fields (CSS, hex-only palettes, font px, component libraries, breakpoints) are
> marked N/A with their terminal equivalent stated.

---

## Design System

| Property | Value | Terminal Interpretation |
|----------|-------|------------------------|
| Tool | none | N/A — no component library; plain `fmt.Fprintf` to `io.Writer` with ANSI escape sequences, consistent with existing `cmd/gitid/*.go` commands |
| Preset | not applicable | N/A |
| Component library | none | Existing `fp()` / `prompt()` / `confirm()` / `promptYN()` helpers in `cmd/gitid/` (add.go lines 260, 490-517; baseline.go lines 439-445); reuse without modification |
| Icon library | none | N/A — terminal output uses glyph prefix characters: `✓` `✗` `!` `•` with ASCII fallbacks `OK` `FAIL` `!` `*` |
| Font | terminal monospace | N/A — output is plaintext; monospace is given by the terminal |

**Source:** Detected from `cmd/gitid/add.go`, `list.go`, `baseline.go` (existing code).
Color is a **Phase 4 first** — not present in any prior phase command.

---

## Spacing Scale

> Terminal equivalent: blank lines and indentation levels, not CSS spacing tokens.
> Responsive breakpoints → terminal width handling: wrap long suggested-fix commands
> at 80 columns (see Wrapping Contract below).

| Token | Terminal Value | Usage |
|-------|---------------|-------|
| xs | 0 blank lines | Between finding parts within a single finding block |
| sm | 2-space indent | Field indent inside a finding (explanation and fix lines) |
| md | 4-space indent | Continuation lines for wrapped suggested-fix commands |
| lg | 1 blank line | Between families (before each family header) |
| xl | 1 blank line + `---` header | Between the report body and the apply-fixes gate |

**Indentation rule:** 2 spaces for finding body lines under the glyph+title line,
consistent with `printAccounts` in `list.go` (`"  key:      %s\n"`) and
`printConflictSection` in `baseline.go` (`"  ! message\n"`).

**No exceptions.**

---

## Typography

> Terminal equivalent: text weight and hierarchy are conveyed by ANSI bold attribute,
> UPPERCASE labels, `---` / `===` delimiters, and glyph prefix characters — not font
> weight or point size.

| Role | Terminal Convention | ANSI Attribute | Usage |
|------|---------------------|----------------|-------|
| Family header | `=== Family Name ===` | Bold (SGR 1) on TTY | Major section start for each of the 7 families |
| Passing check | `  ✓ description` (ASCII: `  OK description`) | Green (see Color section) on TTY | One line per passing check within a family |
| Finding title | `  ✗ title` (ASCII: `  FAIL title`) | Severity color (see Color section) on TTY | One line per failing check; severity determines color |
| Finding explanation | `    explanation text` | Default foreground | 2-space indent under finding title; plain text |
| Suggested fix | `    fix: command or action` | Dim/secondary (SGR 2) on TTY | 2-space indent under explanation |
| Fixable marker | `    [fixable]` | Default foreground | Shown when a FixDescriptor is non-nil, before the gate prompt |
| Prompt | `Label [default]:` | Default foreground | Interactive prompts — reuse `prompt()` helper |
| Confirm prompt (destructive) | `Label [y/N]:` | Default foreground | Per-finding confirm, default N (matches `confirm()` in add.go) |
| Top-level gate | `Apply N fix(es)? [y/N]:` | Default foreground | Top-level apply-fixes gate after report |
| Summary line | `doctor: N critical, N error, N warning, N info` | Default foreground | Final summary after report |
| Error message | `doctor: context: err` | Default foreground | Error format consistent with existing cmd pattern |

**Source:** Hierarchy derived from `list.go` `printAccounts` (label-first, 2-space
indent) and `baseline.go` `printConflictSection` (`!` prefix convention). Bold and
color are new in Phase 4 (D-08).

---

## Color

> D-08 is locked: color on TTY, auto-plain when piped/redirected, respect `NO_COLOR`.
> TTY detection: `(file.Stat().Mode() & os.ModeCharDevice) != 0` — pure stdlib,
> zero new imports (RESEARCH.md Pattern 5).
>
> These are NAMED TOKENS, not hard-wired ANSI codes. The Phase 5 lipgloss TUI
> should import these logical roles and map them to `lipgloss.Color()` calls.
> For the Phase 4 CLI, they map to bare ANSI SGR codes below.
>
> Color degradation order: full color (TTY) → no color (piped/NO_COLOR).
> All information is also conveyed by the glyph and text prefix — color is never
> the sole carrier of meaning.

### Severity Color Tokens

| Token name | ANSI SGR code | Lipgloss equivalent (Phase 5) | Usage |
|------------|--------------|-------------------------------|-------|
| `ColorCritical` | `\033[31m` (red) | `lipgloss.Color("1")` | critical severity finding title + glyph |
| `ColorError` | `\033[31m` (red) | `lipgloss.Color("1")` | error severity finding title + glyph |
| `ColorWarning` | `\033[33m` (yellow) | `lipgloss.Color("3")` | warning severity finding title + glyph |
| `ColorInfo` | `\033[36m` (cyan) | `lipgloss.Color("6")` | info severity finding title + glyph |
| `ColorPass` | `\033[32m` (green) | `lipgloss.Color("2")` | passing check glyph + text |
| `ColorDim` | `\033[2m` (dim) | `lipgloss.NewStyle().Faint(true)` | suggested-fix line (secondary text) |
| `ColorBold` | `\033[1m` (bold) | `lipgloss.NewStyle().Bold(true)` | family headers |
| `ColorReset` | `\033[0m` | `lipgloss.NewStyle()` (default) | after any colored token |

**Critical vs error:** Both use red. Distinction is carried by the severity label
text (`critical` vs `error`), not by color. This is intentional — adding a second
red shade risks poor visibility on light terminals. The glyph `✗` and the label
text disambiguate.

**60/30/10 terminal analog:**
- 60% dominant surface: default terminal foreground (explanation text, summary lines)
- 30% secondary: `ColorDim` (suggested-fix commands — recede to let title/explanation lead)
- 10% accent: severity colors + `ColorPass` — reserved for the glyph and finding title only

**Accent reserved for:** finding title line (glyph + first line only), family header
(bold only, no color). Never applied to explanation text, fix lines, or the summary count.

**Plain-text fallback (piped / NO_COLOR):** All ANSI codes are omitted. Glyphs degrade
from `✓`/`✗` to `OK`/`FAIL` only when the `TERM` env var is `dumb` or unset. On
modern terminals piped output retains UTF-8 glyphs; only ANSI SGR codes are stripped.

**Implementation:**

```go
// in cmd/gitid/doctor.go — color helper (zero new imports)
func ansi(code, text string, colorEnabled bool) string {
    if !colorEnabled {
        return text
    }
    return "\033[" + code + "m" + text + "\033[0m"
}

// severity → ANSI code mapping
func severityCode(s doctor.Severity) string {
    switch s {
    case doctor.SeverityCritical, doctor.SeverityError:
        return "31" // red
    case doctor.SeverityWarning:
        return "33" // yellow
    default: // info
        return "36" // cyan
    }
}
```

---

## Glyph Contract

> Glyphs form part of the visual hierarchy. Both the colored form (TTY) and the
> ASCII fallback (dumb terminal or explicit `--no-color`) must be declared.

| Semantic | Glyph (UTF-8) | ASCII fallback | When used |
|----------|--------------|----------------|-----------|
| Passing check | `✓` | `OK` | Each check that found no issue within a family |
| Failing check | `✗` | `FAIL` | Each finding (any severity) |
| Advisory finding | `!` | `!` | Info-only findings (same glyph in both modes) |
| Fixable marker | `[fix]` | `[fix]` | Same in both modes — appears after the fix: line |

**Glyph selection rule:** Use UTF-8 glyphs unless `$TERM == "dumb"` or `$TERM` is
unset. The TTY detection check handles color; the dumb-terminal check is separate and
handles glyphs only. A pipe does NOT degrade glyphs — only color.

---

## Report Layout Contract

### Overall structure

```
=== Dependencies ===
  ✓ ssh present (OpenSSH 9.8p1)
  ✓ ssh-keygen present
  ✗ clipboard tool missing
    No clipboard tool found (pbcopy/xclip/wl-copy). Public-key copy will not work.
    fix: brew install pbcopy  (macOS)
         apt install xclip    (Debian/Ubuntu)
         dnf install xclip    (Fedora)

=== Permissions ===
  ✓ ~/.ssh directory: 700
  ✗ ~/.ssh/gitid_work: 644 (expected 600)
    Private key has group/world read permission — key may be exposed.
    fix: chmod 0600 ~/.ssh/gitid_work
    [fix]

=== Coherence ===
  ✓ IdentityFile ~/.ssh/gitid_personal resolves
  ✗ IdentityFile ~/.ssh/gitid_old does not exist
    The SSH Host block for "old" references a key file that is missing.
    fix: run 'gitid identity add' to recreate, or delete the orphaned block

=== Orphans ===
  ✓ no orphaned managed blocks
  ✗ ~/.gitconfig.d/stale has no owning includeIf block
    A gitconfig fragment exists but no includeIf in ~/.gitconfig claims it.
    fix: remove ~/.gitconfig.d/stale  (gitid will confirm before removing)
    [fix]

=== Signing ===
  ✓ gpg.format=ssh for identity "personal"
  ✓ allowed_signers line present for personal@example.com

=== Agent ===
  ✓ ssh-agent reachable
  ✗ identity "work" key not loaded in agent
    The key ~/.ssh/gitid_work is not in the running agent. Commits may fail.
    fix: ssh-add ~/.ssh/gitid_work

=== Baseline ===
  ✓ baseline [include] resolves
  ✓ core.excludesfile wired to ~/.gitignore_global
  ✗ core.ignorecase is true (expected false)
    The baseline sets ignorecase=false; an override has set it to true.
    fix: run 'git config --global core.ignorecase false'  or re-run 'gitid baseline setup'

---
doctor: 0 critical, 1 error, 3 warning, 1 info
exit code: 2

Apply 2 fix(es)? [y/N]:
```

### Family ordering (fixed, not alphabetical)

The families render in this fixed order in every run:

1. Dependencies
2. Permissions
3. Coherence
4. Orphans
5. Signing
6. Agent
7. Baseline

**Rationale:** Failure urgency descends roughly top-to-bottom. Dependencies must be
present before permissions can be checked; coherence before orphans; signing/agent
last because they depend on coherence passing. Baseline is last because it is a
separate concern from SSH identity health.

**Baseline as its own family (not folded into Coherence/Orphans):** The four baseline
checks (excludesfile, include resolves, ignorecase drift, curated excludes) are
semantically distinct from SSH identity coherence. A separate Baseline section maps
cleanly to `gitid baseline show` and seeds the Phase 5 TUI's distinct Baseline panel.
This is a recommendation, not a hard lock — the planner may fold individual baseline
findings into Orphans (for an orphaned include block) if that produces cleaner code,
as long as the rendered section header reads `Baseline` in the output.

### Family header format

```
=== Dependencies ===
```

- `===` delimiters on both sides, single space between `===` and name.
- Name is the Family constant string value exactly: `Dependencies`, `Permissions`,
  `Coherence`, `Orphans`, `Signing`, `Agent`, `Baseline`.
- Bold ANSI (SGR 1) on TTY, plain text when piped/NO_COLOR.
- 1 blank line before each family header (no blank line before the first family).

### Passing check format

```
  ✓ description of what was checked (result)
```

- 2-space indent, `✓` glyph (or `OK` in ASCII mode), 1 space, description.
- Color: `ColorPass` (green) applied to `✓` glyph only; description is default foreground.
- One line per passing check — no multi-line pass blocks.
- Description convention: short, starts with the artifact or tool name.
  Examples: `ssh present (OpenSSH 9.8p1)`, `~/.ssh directory: 700`,
  `IdentityFile ~/.ssh/gitid_personal resolves`, `gpg.format=ssh for identity "work"`.

### Finding (failure) format

```
  ✗ title [severity label if not error]
    explanation text, plain English.
    fix: exact command or action text
    [fix]   ← only when FixDescriptor is non-nil
```

- 2-space indent, `✗` glyph (or `FAIL` in ASCII mode), 1 space, title.
- Severity is shown inline only when it is NOT `error` (since `error` is the
  expected failure level and would add noise). For `critical` show `[critical]`
  after title; for `warning` show `[warning]`; for `info` use `!` glyph instead
  of `✗`. For `error`, no severity label — the `✗` glyph implies broken.
- Color: severity color token applied to `✗` glyph + title text; explanation and
  fix lines use default foreground and `ColorDim` respectively.
- Explanation: 4-space indent (2 more than glyph line), plain English sentence(s).
  Max 2 lines before the fix: line.
- `fix:` label: 4-space indent, `fix:` in dim, then the suggested command(s) or
  human-readable action.
- `[fix]`: 4-space indent, shown on its own line when the finding is auto-fixable.
  Signals to the user that `--fix` can handle it.

**Finding title conventions:**
- Short noun phrase or fragment identifier: `~/.ssh/gitid_work: 644 (expected 600)`,
  `IdentityFile ~/.ssh/gitid_old does not exist`, `ssh-add missing`.
- Avoid "Error:" or "Warning:" prefix in the title — the glyph and severity label
  carry that information.

**Info findings use `!` glyph (not `✗`):**

```
  ! pbcopy not found [info]
    Optional clipboard tool not installed. Key copy-to-clipboard will not work.
    fix: brew install pbcopy
```

---

## Severity Label Contract

| Severity | Glyph | Color token | Inline severity label | Exit code contribution |
|----------|-------|-------------|----------------------|----------------------|
| critical | `✗` | `ColorCritical` (red) | `[critical]` shown after title | 3 |
| error | `✗` | `ColorError` (red) | (none — implied by `✗`) | 2 |
| warning | `✗` | `ColorWarning` (yellow) | `[warning]` shown after title | 1 |
| info | `!` | `ColorInfo` (cyan) | `[info]` shown after title | 1 |
| pass | `✓` | `ColorPass` (green) | N/A — no label | 0 |

**Exit code summary line** is always printed, even on a clean run:

```
doctor: 0 critical, 0 error, 0 warning, 0 info
exit code: 0
```

The words `critical`, `error`, `warning`, `info` in the summary line are plain — no
color applied to the summary line itself.

---

## Wrapping Contract

> Terminal equivalent of "responsive breakpoints" — how long lines are handled.

- **Family headers:** never wrap — titles are short by design.
- **Passing check lines:** never wrap — one line, description kept under 70 chars.
- **Finding title lines:** never wrap — kept under 70 chars by construction.
- **Explanation text:** wrap at 78 columns (2-space indent + 76 chars of text).
  Continuation lines indent an additional 2 spaces (4 total) to visually subordinate.
- **Suggested-fix commands:** wrap at 78 columns; continuation lines use 9-space
  indent (`         `) to align with the text after `fix: `:

```
    fix: brew install openssh  (macOS)
         apt install openssh-client  (Debian/Ubuntu)
         dnf install openssh  (Fedora)
         pacman -S openssh  (Arch)
```

  Each per-OS variant is on its own line with the OS name in parentheses. No line
  continuation backslash — each variant is a complete command.

- **Max terminal width assumption:** 80 columns. Lines are constructed to fit within
  80 columns before wrapping. Do not attempt to detect the actual terminal width
  via `tput cols` — that adds a subprocess dependency.

---

## Apply-Fixes Gate Contract

This section defines the exact UX flow for D-04 (CLI trigger + confirm semantics).

### `gitid doctor` (no flags) — report + optional gate

1. Run all checks. Print the full grouped report.
2. Print the summary line.
3. If zero fixable findings: print nothing additional and exit.
4. If one or more fixable findings exist: print the gate:

```
---
Apply N fix(es)? [y/N]:
```

   - Default is `N` (uppercase) — consistent with `confirm()` helper.
   - On `N` or Enter: print `No fixes applied.` and exit with the severity exit code.
   - On `y`: proceed to per-finding confirm (see below).

### `gitid doctor --fix` — skip gate, go directly to per-finding confirm

Same as above but step 4 is replaced: skip the top-level gate, proceed directly to
per-finding confirm for each fixable finding in report order.

### `gitid doctor --fix --yes` — non-interactive apply

Apply all fixable findings without any prompts. Print a one-line confirmation per
fix applied:

```
  fixed: chmod 0600 ~/.ssh/gitid_work
  fixed: removed orphaned block "stale" from ~/.gitconfig
```

Then exit with the severity exit code (which reflects the pre-fix state, since
unfixable findings may still exist).

### Per-finding confirm (interactive fix)

For each fixable finding (in report order), print:

```
Fix: chmod 0600 ~/.ssh/gitid_work
Apply? [y/N]:
```

- The `Fix:` label echoes `FixDescriptor.Summary` exactly.
- Default `N` — the user must opt into each fix.
- On `y`: apply the fix, then print `  fixed: <summary>`.
- On `N` or Enter: print `  skipped: <summary>`.
- After all per-finding confirms: print the total tally:

```
doctor: 2 fix(es) applied, 1 skipped.
```

### Batching rule for permissions (D-04)

When multiple permission findings exist (e.g. three key files with wrong mode),
they MAY be presented as a single batched confirm:

```
Fix 3 permission(s):
  chmod 0600 ~/.ssh/gitid_work
  chmod 0600 ~/.ssh/gitid_old
  chmod 0700 ~/.ssh
Apply all? [y/N]:
```

Orphaned-block removal and wiring re-add findings are NEVER batched — each presents
its own confirm. This is a hard rule (higher blast radius, D-04).

### `--yes` without `--fix` error

```
doctor: --yes requires --fix
```

Exits with code 1. This matches the Cobra convention of printing to stderr via
`cmd.PrintErr` (or equivalent).

---

## Copywriting Contract

All copy is in English. No emoji. No Spanish. Error messages follow
`doctor: <context>: <err>` format (matches `"identity add: resolving home dir: %w"`
in add.go).

### Finding explanations — voice and rules

- Plain English, present tense, active voice where possible.
- State the observed fact first, then the consequence.
- Do not use "Error:", "Warning:", or severity words in the explanation body —
  the glyph/label carries that.
- Max 2 sentences per explanation.

### Copywriting table

| Element | Copy |
|---------|------|
| Empty/all-clear state | `doctor: all checks passed\nexit code: 0` |
| Summary line | `doctor: N critical, N error, N warning, N info\nexit code: N` |
| Top-level gate | `Apply N fix(es)? [y/N]:` |
| Gate declined | `No fixes applied.` |
| Per-finding confirm | `Fix: <FixDescriptor.Summary>\nApply? [y/N]:` |
| Fix applied | `  fixed: <FixDescriptor.Summary>` |
| Fix skipped | `  skipped: <FixDescriptor.Summary>` |
| Post-fix tally | `doctor: N fix(es) applied, N skipped.` |
| --fix --yes fix applied | `  fixed: <FixDescriptor.Summary>` |
| --yes without --fix | `doctor: --yes requires --fix` |
| Error: home dir | `doctor: resolving home dir: <err>` |
| Error: reading ssh config | `doctor: reading ~/.ssh/config: <err>` |
| Error: reading gitconfig | `doctor: reading ~/.gitconfig: <err>` |
| Error: fix failed | `doctor: fix failed: <FixDescriptor.Summary>: <err>` |
| Batch perms confirm header | `Fix N permission(s):` |
| Backup notice (before fix) | `  backup: <path>.bak.<timestamp>` |

### Finding-specific copy examples (prescriptive)

These are the exact Explanation and fix: lines for each finding class. Executors
MUST follow this copy; variation is not permitted (consistent with the Phase 3.1
UI-SPEC copy contract for `baseline`).

**DOC-01 — missing required dependency**

```
  ✗ <tool> missing
    Required tool not found in PATH. gitid cannot function without it.
    fix: <platform.InstallHint output for tool>
```

Required tools: `ssh`, `ssh-keygen`, `ssh-add`, `git`.

**DOC-01 — missing optional dependency (info)**

```
  ! <tool> not found [info]
    Optional tool not installed. <specific capability> will not work.
    fix: <platform.InstallHint output for tool>
```

Optional tools: clipboard tool (`pbcopy`/`xclip`/`wl-copy`). Capability text:
`Public-key copy to clipboard`.

**DOC-02 — private key wrong mode (critical)**

```
  ✗ <path>: <actual_mode> (expected 0600) [critical]
    Private key has group or world read permission. The key may be exposed to other users.
    fix: chmod 0600 <path>
    [fix]
```

**DOC-02 — ssh directory wrong mode (error)**

```
  ✗ ~/.ssh directory: <actual_mode> (expected 0700)
    SSH directory allows enumeration by other users. SSH may refuse to use this configuration.
    fix: chmod 0700 ~/.ssh
    [fix]
```

**DOC-02 — .pub file wrong mode (warning)**

```
  ✗ <path>: <actual_mode> (expected 0644) [warning]
    Public key has incorrect permissions. Some tools may refuse to read it.
    fix: chmod 0644 <path>
    [fix]
```

**DOC-02 — ssh config wrong mode (error)**

```
  ✗ ~/.ssh/config: <actual_mode> (expected 0600)
    SSH config file has incorrect permissions. SSH will refuse to use it.
    fix: chmod 0600 ~/.ssh/config
    [fix]
```

**DOC-03 — IdentityFile does not resolve (error)**

```
  ✗ IdentityFile <path> does not exist
    The SSH Host block for "<identity>" references a key file that is missing.
    fix: run 'gitid identity add' to recreate, or remove the orphaned SSH Host block
```

**DOC-03 — includeIf fragment missing (error)**

```
  ✗ includeIf fragment <path> does not exist
    The gitconfig includeIf for "<identity>" points to a missing fragment file.
    fix: run 'gitid identity add' to recreate the fragment
```

**DOC-03 — IdentitiesOnly missing (error)**

```
  ✗ Host "<alias>": IdentitiesOnly yes missing
    Without IdentitiesOnly, SSH may use an unintended key for this host.
    fix: re-run 'gitid identity add --name <identity>' (will repair the Host block)
    [fix]
```

**DOC-03 — allowed_signers line missing (error)**

```
  ✗ allowed_signers: no entry for <email>
    Signing identity "<identity>" has no line in ~/.ssh/allowed_signers. Commit signature verification will fail.
    fix: add the line manually or re-run 'gitid identity add'
    [fix]
```

**DOC-04 — orphaned managed block (warning)**

```
  ✗ <path>: orphaned gitconfig fragment [warning]
    Fragment exists on disk but no includeIf block in ~/.gitconfig claims it.
    fix: remove <path>  (gitid will confirm before removing)
    [fix]
```

**DOC-04 — unused key file (warning)**

```
  ✗ <path>: not referenced in ~/.ssh/config [warning]
    This key is not referenced by any SSH Host block (gitid-managed or hand-written).
    It may be used for direct server SSH or 'ssh -i' — review before deleting.
    fix: inspect usage manually; delete with 'rm <path>' if confirmed unused
```

Note: no `[fix]` marker — key deletion is report-only per D-03 and D-13.

**DOC-05 — gpg.format not ssh (error)**

```
  ✗ identity "<name>": gpg.format is "<actual>" (expected "ssh")
    Commit signing is misconfigured. Signing with an SSH key requires gpg.format=ssh.
    fix: git config --file ~/.gitconfig.d/<name> gpg.format ssh
```

**DOC-05 — allowed_signers email mismatch (error)**

```
  ✗ allowed_signers: email mismatch for identity "<name>"
    The signing line email does not byte-match user.email. Signature verification will fail.
    fix: correct the email in ~/.ssh/allowed_signers to exactly match '<email>'
    [fix]
```

**DOC-05 — ssh-agent unreachable (warning)**

```
  ✗ ssh-agent: not reachable [warning]
    Cannot connect to the SSH agent. Passphrase-protected keys will prompt for passphrase on each use.
    fix: start the agent with 'eval "$(ssh-agent -s)"' and re-add your keys
```

**DOC-05 — managed key not loaded in agent (warning)**

```
  ✗ identity "<name>": key not loaded in agent [warning]
    The key ~/.ssh/gitid_<name> is not in the running ssh-agent. Operations may prompt for passphrase.
    fix: ssh-add ~/.ssh/gitid_<name>
```

**DOC-05 — git < 2.36 with hasconfig: (warning)**

```
  ✗ git <actual_version>: hasconfig: not supported [warning]
    One or more identities use 'hasconfig:remote.*.url:' match strategy, which requires git >= 2.36.
    fix: upgrade git (current: <actual_version>, required: >= 2.36)
         brew upgrade git  (macOS)
         apt install git   (Debian/Ubuntu — may need backports)
```

**D-16 — excludesfile not set or missing (error)**

```
  ✗ core.excludesfile: not set or file missing
    The global gitignore is not configured. OS/editor artifacts will not be excluded.
    fix: run 'gitid baseline setup'
```

**D-16 — baseline include missing/orphaned (error)**

```
  ✗ baseline [include] block missing from ~/.gitconfig
    The managed baseline include block is gone. Baseline settings have no effect.
    fix: run 'gitid baseline setup'
    [fix]
```

**D-16 — ignorecase drift (warning)**

```
  ✗ core.ignorecase: true (expected false) [warning]
    An override has enabled case-insensitive matching. This can hide filename case conflicts on macOS.
    fix: git config --global core.ignorecase false  or re-run 'gitid baseline setup'
```

**D-16 — curated excludes missing from gitignore (warning)**

```
  ✗ ~/.gitignore_global: curated entries missing [warning]
    One or more gitid-managed gitignore patterns are absent. OS/editor artifacts may be committed.
    fix: run 'gitid baseline setup' to restore the managed gitignore block
    [fix]
```

---

## Install Hint Format

Per-OS install hints follow this format for all dependency findings (DOC-01, DOC-05
git version). Lines after the first are indented to align with the text after `fix: `:

```
    fix: brew install <package>  (macOS)
         apt install <package>   (Debian/Ubuntu)
         dnf install <package>   (Fedora)
         pacman -S <package>     (Arch)
```

Platform detection uses `platform.CurrentOS()` (already in `internal/platform`).
When running on a known platform, only that platform's hint is shown:

```
    fix: brew install openssh  (macOS)
```

When the platform is unknown (`platform.OSUnknown` or equivalent), show all four.

---

## Information Architecture

### `gitid doctor` sub-tree

```
gitid doctor [--fix] [--yes]
```

No sub-subcommands. Two flags:
- `--fix`: skip top-level gate, go to per-finding confirm.
- `--yes`: apply all fixable findings without prompts (requires `--fix`).

### Cobra wiring

- `newDoctorCmd()` returns a `*cobra.Command` with `Use: "doctor"` and
  `Short: "Run a health check on the gitid-managed environment"`.
- Registered directly on the root command (not under a sub-group like `identity`).
- `--fix` and `--yes` are `BoolVar` flags on the doctor command only.

---

## Accessibility Contract (Terminal)

1. **No information conveyed by color alone.** Glyphs (`✓`, `✗`, `!`) carry
   the same information as color. A monochrome terminal or piped output is fully legible.
2. **Piped output is legible.** `gitid doctor 2>&1 | grep FAIL` must produce
   useful results. The `FAIL` ASCII glyph replaces `✗` in this context (dumb TERM only).
3. **All prompts show the default visually** (`[y/N]` — uppercase N = default No).
   Users who press Enter receive the safe default (no fix applied).
4. **All destructive actions state consequences before confirm.** The orphaned-block
   removal confirm shows the exact file path before asking. The `[fix]` marker in
   the report tells the user which findings will be addressed before the gate is reached.
5. **`NO_COLOR` is respected.** Setting `NO_COLOR=1` (any non-empty value) strips all
   ANSI SGR codes. This is checked before the TTY detection (D-08, no-color.org spec).
6. **English only.** All output, prompts, warnings, and error messages.
7. **Non-TTY environments (CI):** `gitid doctor` in a CI pipeline with no TTY produces
   plain text, exits with a non-zero code when findings exist, and never blocks on prompts
   (because the gate only prompts when `--fix` is passed or the user answers `y` interactively).

---

## Phase 5 Portability Notes

These design tokens and data shapes are designed to be adopted by the Phase 5
Bubble Tea / lipgloss TUI dashboard without redesign.

| CLI contract | Phase 5 TUI analog |
|-------------|-------------------|
| `ColorCritical` → `\033[31m` | `lipgloss.Color("1")` (ANSI 256-color index 1) |
| `ColorWarning` → `\033[33m` | `lipgloss.Color("3")` |
| `ColorPass` → `\033[32m` | `lipgloss.Color("2")` |
| `ColorInfo` → `\033[36m` | `lipgloss.Color("6")` |
| `ColorDim` → `\033[2m` | `lipgloss.NewStyle().Faint(true)` |
| `ColorBold` → `\033[1m` | `lipgloss.NewStyle().Bold(true)` |
| `Family` grouping | One panel/card per Family in the TUI dashboard |
| `Finding.Severity` | Maps to the same color token in the TUI item renderer |
| `Finding.Title` | TUI list item primary line |
| `Finding.Explanation` | TUI list item secondary/description line |
| `Finding.SuggestedFix` | TUI detail pane |
| `Finding.Fix != nil` | TUI "fixable" badge / action button enabled |

The `Finding` struct returned by `doctor.Run()` is already UI-agnostic (RESEARCH.md
Pattern 1). The CLI renderer and the TUI renderer both consume the same `[]Finding`
slice — no translation layer is needed.

---

## Registry Safety

| Registry | Blocks Used | Safety Gate |
|----------|-------------|-------------|
| N/A — CLI phase only | none | Not applicable |

No shadcn, no third-party component registry. Phase 4 introduces no new external
dependencies (RESEARCH.md §Standard Stack: "No new `go get` needed").

---

## Checker Sign-Off

> Adapted for CLI phase: each dimension maps to a terminal-output quality check.

- [ ] Dimension 1 Copywriting: all CTAs, empty-state, per-finding copy, per-OS hints, gate prompts, and error copy declared above; English-only; no emoji; unused-key wording explicitly admits gitid cannot confirm it is unused
- [ ] Dimension 2 Visuals (terminal): family headers, glyph prefix markers, indent levels, delimiter characters, ASCII fallbacks, and fix/[fix] formatting all specified; consistent with existing cmd/gitid/ patterns
- [ ] Dimension 3 Color: 6 named severity tokens declared with ANSI codes and Phase 5 lipgloss equivalents; TTY detection method specified; NO_COLOR compliance confirmed; color never sole carrier of meaning
- [ ] Dimension 4 Typography: terminal monospace; hierarchy via bold + glyphs + UPPERCASE + indentation; dim for suggested-fix lines; all roles declared
- [ ] Dimension 5 Spacing: blank-line rhythm declared; 2-space indent rule; 4-space continuation; wrap at 78 columns; per-OS hint alignment specified
- [ ] Dimension 6 Registry Safety: N/A (no component registry); no new dependencies

**Approval:** pending

---

## Source Traceability

| Decision | Source |
|----------|--------|
| 4-level severity model (critical/error/warning/info) | CONTEXT.md D-05 (LOCKED) |
| Grouped-by-family report with ✓ for passes | CONTEXT.md D-06 (LOCKED) |
| 7 families: Dependencies, Permissions, Coherence, Orphans, Signing, Agent, Baseline | CONTEXT.md D-06 + D-16 (LOCKED) |
| Baseline as own family (not folded) | RESEARCH.md Open Question 2 — recommendation adopted |
| Color on TTY, plain when piped, NO_COLOR | CONTEXT.md D-08 (LOCKED) |
| Tiered exit codes 0/1/2/3, highest severity wins | CONTEXT.md D-07 (LOCKED) |
| Auto-fix gate: doctor = gate + per-confirm; --fix = per-confirm; --fix --yes = silent | CONTEXT.md D-04 (LOCKED) |
| Permissions may batch under one confirm; orphans/wiring confirm individually | CONTEXT.md D-04 (LOCKED) |
| Unused-key → warning only, honest wording (cannot confirm unused) | CONTEXT.md D-13 (LOCKED) |
| Key-file deletion is report-only, no [fix] marker | CONTEXT.md D-03 (LOCKED) |
| Per-OS install hints format | REQUIREMENTS.md DOC-01; platform.InstallHint() pattern in internal/platform |
| Platform-specific single hint (when known), all-four (when unknown) | CONTEXT.md §Claude's Discretion |
| Family header format `=== Name ===` | baseline.go `printBaselinePreview` (`"=== Preview: baseline setup ==="`) pattern |
| 2-space indent, `!` prefix | list.go `printAccounts` lines 107 + baseline.go `printConflictSection` |
| `fp()` helper for output | add.go line 260 |
| `confirm()` with default N | add.go lines 512-517 |
| `promptYN()` with default Y | baseline.go line 439 |
| ANSI color codes (bare SGR, no lipgloss in CLI) | RESEARCH.md Pattern 5 / State of the Art table |
| Severity token names for Phase 5 lipgloss portability | CLAUDE.md §charm.land/lipgloss/v2 v2.0.3; ROADMAP.md Phase 5 |
| TTY detection via `file.Stat().Mode() & os.ModeCharDevice` | RESEARCH.md Pattern 5 (pure stdlib, zero imports) |
| Glyph UTF-8 vs ASCII fallback rule | RESEARCH.md Pattern 5 anti-pattern "Hardcoding color codes when piped" |
| Error message format `doctor: context: err` | add.go line 57 (`"identity add: resolving home dir: %w"`) |
| --yes requires --fix enforcement | RESEARCH.md Open Question 3 — `fmt.Errorf("--yes requires --fix")` in RunE |
| English-only artifacts | CLAUDE.md §Language |
| No web design tokens (no rem, no hex-only, no breakpoints) | Objective brief |
