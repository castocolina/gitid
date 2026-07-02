# Phase 2 — UX Direction (planning engagement, `agent-ui-ux-designer`)

**Role of this document.** This is design DIRECTION, not the screens. It gives the
`gsd-planner` the guardrails, the shared shell, the parity rubric, the per-surface
state manifests, and the approval checklist that Phase 2's mockup / dummy / critique /
approval plans must be built around. Everything here is verified against `recipes/`
(the North Star config end-state) and the Phase 3–9 success criteria in `ROADMAP.md`.

Where I push back on the phase framing, it is called out as **⚠ RISK** with a concrete
mitigation the planner should adopt.

---

## 0. Three risks in the current framing — read first

These change how the plans should be structured, so they lead.

**⚠ RISK 1 — MUI v7 is the wrong *visual* idiom for a terminal tool, and naive use makes
the parity gate meaningless.** Material Design means cards, elevation/shadow, ripple,
FABs, rounded corners, a 8px-grid web layout. A Bubble Tea TUI is flat, monospace,
fixed-cell, box-drawing, keyboard-first. If the HTML mockup is authored as a generic
Material dashboard, then (a) the HTML↔TUI diff (DLV-04) is ~100% pixel noise forever,
and (b) you will have designed a web app and then discovered it doesn't fit 80 columns.
**Mitigation (adopt both):**
  1. **Theme MUI into a terminal skin.** Use `/mui`, but constrain the theme: monospace
     font (JetBrains Mono / IBM Plex Mono), dark surface, `shadows: none`,
     `shape.borderRadius: 0`, disabled ripple, a fixed max-width "terminal" container,
     ANSI-like semantic palette, and a simulated bottom keybar. The HTML mockup should
     read as *a screenshot of a terminal*, not a SaaS dashboard.
  2. **Set the parity bar to SEMANTIC, not pixel** (see §3). The HTML mockup's job is to
     lock **content and flow** — fields, labels, copy, option sets, order, states — not
     to be a pixel target for the TUI.

**⚠ RISK 2 — "every screen" of a multi-step flow is under-specified; without a canonical
named-state list per surface, the HTML author and the TUI author will capture different
frames and parity cannot be assessed.** A create flow is not one screen; it is a
sequence with empty/normal/error/confirm variants. **Mitigation:** the per-surface
manifests in §4 enumerate the exact named states both media MUST capture. The planner
should lift these verbatim into the mockup + dummy plans so the two screenshot sets are
frame-for-frame comparable.

**⚠ RISK 3 — this single approval freezes the reference for ALL of Phases 3–9, so
anything not truly frozen at approval cascades rework through six later
visual-regression gates.** In particular field order, labels, helper copy, option sets,
and defaults become contractual at approval. **Mitigation:** (a) mock with **real,
recipe-accurate copy now — no lorem, no placeholder option lists**; (b) the approval
checklist (§6) includes an explicit **copy + field-order + defaults freeze**; (c)
sequence the work as a **pilot surface first** (see §1) so the design language and the
parity rubric are validated on one surface before the other six are fanned out against
an unproven shell.

---

## 1. Design ethos & anti-slop stance

**What this product must feel like:** a *credible, terminal-native developer safety
tool* — in the lineage of `gh`, `lazygit`, `k9s`, `tig`, `htop`. Dense but legible,
keyboard-first, honest about what it is about to do to your dotfiles, and calm. It edits
`~/.ssh/config` and `~/.gitconfig`; the aesthetic must earn trust, not decorate.

**Terminal "AI slop" — what to AVOID** (the TUI equivalent of purple-gradient SaaS):
- **Box-everything.** A border around every label, nested panels three deep. Borders are
  a structural tool (separate master/detail, frame a modal), not a texture. lazygit/k9s
  use a handful of framed panels, not fifty.
- **Emoji and glyph confetti.** Sprinkled ✨🚀🔥 for "delight". A security tool that
  shows emoji where it should show a file path reads as unserious. Glyphs are allowed
  only as **semantic status markers** (✓ healthy, ! needs-action, ✗ error/destructive)
  and even then must be paired with a text label (NO_COLOR + colorblind — see below).
- **Rainbow color.** Every field a different hue. Color must be **semantic and
  restricted** (§2), never decorative.
- **Fake dashboards.** Sparklines, gauges, "cards" of vanity metrics. gitid has no
  metrics; it has identities, config blocks, and health states.
- **Centered everything.** Centered body text/nav fights the terminal. Reading and
  scanning in a terminal is top-left-anchored and left-aligned — this is both convention
  (Jakob's Law) and consistent with NN/g's left-side attention bias. Master list goes
  **left**, detail **right** (lazygit, k9s, mutt, tig all do this).

**Heuristics this design is accountable to** (Nielsen's 10, mapped to gitid):
- **Visibility of system status** → persistent context header + status line; and the
  signature pattern from Phase 3: *show the exact command run and its real output*.
- **Match system ↔ real world** → speak git/ssh vocabulary; show the **actual config
  text** being authored (WYSIWYG live preview), not an abstraction of it.
- **User control & freedom** → `Esc` cancels/returns everywhere; nothing mutates before
  an explicit confirm; **backups are the undo**.
- **Error prevention** → test against throwaway temp files before touching live config;
  read-only preview precedes every write; destructive actions never default to "yes".
- **Recognition over recall** → keybar always visible; live preview so the user never
  holds config syntax in their head.
- **Consistency & standards** → follow the gh/lazygit/k9s key conventions rather than
  inventing (list-nav, `/` filter, `?` help, `:`/palette, number-keys for top views).
- **Help users recover from errors** → the Fixer: severity + plain-English explanation +
  suggested fix, applied only with confirm + backup.

**Planner action:** sequence Phase 2 as **shell-first, then a pilot surface, then
fan-out.** Concretely: (P0) design + approve the global shell/IA (§2) and the terminal
skin; (P1) take **the create-identity flow as the pilot** — it is the most complex
surface and exercises nearly every pattern (multi-step form, live preview, two-stage
test with real command output, confirm, backup) — build it in *both* media, run the
parity rubric, and get a lightweight go/no-go on the design language; (P2) fan out the
remaining six surfaces against the now-proven shell; (P3) full parity critique + the
single user approval. This avoids mocking seven surfaces on an unvalidated shell.

---

## 2. Cross-surface information architecture (the shared shell)

Every one of the seven surfaces renders inside **one app frame** so they read as a
single product. Both the MUI mockup and the TUI dummy must implement this identically.

**Layout regions (top → bottom):**
1. **Header / context bar** — app name (`gitid`), the current view name, and a global
   context chip (e.g. identity count, global health `✓/!/✗`). One line, left-aligned.
2. **Body** — per-surface. Two canonical body archetypes only, for consistency:
   - **Master–detail** (list left ~⅓, detail right ~⅔): Identity Manager, Health, Fixer,
     Global SSH options, Global Git options.
   - **Guided form + live preview** (form left, live config-text preview right): the
     create flow and the git-config screen. The preview pane shows the **real resulting
     block** (`Host …` / `[includeIf …]` / `allowed_signers` line) as it is typed.
3. **Status / message line** — one line for transient feedback: what is happening, the
   file being written, the **backup path**, validation errors.
4. **Keybar / footer** — context-sensitive keybindings, **always visible** (lazygit/k9s
   convention; direct Recognition-over-recall payoff). Shows only keys valid in the
   current context.

**Global navigation model (must be consistent across all surfaces):**
- **Five primary views reachable by number keys `1`–`5` and a command palette** — this
  is a hard requirement from MGR-06/SHELL-01. The five: **1 Identities · 2 Global SSH ·
  3 Global Git · 4 Health · 5 Fixer.** (Create flow and the git-config screen are
  *modal flows launched from Identities*, not top-level numbers.)
- **Palette**: `:` or `Ctrl+P` opens it; every action reachable there (parity with the
  Cobra CLI surface, SHELL-02/03).
- **Reserved keys, identical everywhere:** `Esc` back/cancel · `q` quit · `?` help
  overlay · `/` filter/search in any list · `Enter` activate/confirm · arrows/`j`/`k`
  move. Do **not** reassign these per surface.

**Key-allocation table (SINGLE SOURCE OF TRUTH — every surface MUST allocate its
`ActivationKey`, its keyless `LaunchKey`, and its intra-surface `ScreenDef.Keys`
against THIS table, never independently).** All of `1`–`5`, `n`, `g`, `a`, `c`, `d`
below are pressed while **identity-manager** is the active surface, so they MUST be
mutually distinct and distinct from the reserved keys.

| Key | Owner | Kind | Meaning |
|-----|-------|------|---------|
| `1` | identity-manager | ActivationKey (number-key view) | Identities / home |
| `2` | global-ssh | ActivationKey | Global SSH options |
| `3` | global-git | ActivationKey | Global Git options |
| `4` | health | ActivationKey | Health |
| `5` | fixer | ActivationKey | Fixer |
| `n` | create-flow | LaunchKey (LaunchFrom `identity-manager`) | launch the new-identity modal from Identities |
| `g` | git-screen | LaunchKey (LaunchFrom `identity-manager`) | launch the git-config modal from Identities |
| `a` | identity-manager | intra-surface `ScreenDef.Keys` | → `action-menu` |
| `c` | identity-manager | intra-surface `ScreenDef.Keys` | → `clone-name-prompt` |
| `d` | identity-manager | intra-surface `ScreenDef.Keys` | → `delete-choice` |
| `Enter` | (all) | reserved | activate / open detail / confirm |
| `Esc` | (all) | reserved | back / cancel / pop modal |
| `q` `?` `/` `j` `k` arrows | (all) | reserved | quit / help / filter / move |

- **route() precedence (deterministic):** on a given active surface a key resolves in the
  order **intra-surface `ScreenDef.Keys` → keyless `LaunchKey` (`LaunchFrom` == active
  view) → number-key `ActivationKey` view-switch** (a launch key fires only if the active
  screen's `ScreenDef.Keys` does not claim it). This ordering is a determinism backstop
  only: the 02-02 **registration guard** rejects (a test-detectable error) any
  registration that would let two of these claim the SAME key on the SAME source surface,
  so a collision fails at registration — never silently as an 02-11 e2e failure.
- **Adding a surface/transition:** claim a free key HERE first, then mirror this table in
  `internal/dummytui/doc.go`. The three fan-out plans (create-flow 02-04, git-screen
  02-05, identity-manager 02-06) pick keys against THIS table, not against each other.

**Consistency rules every surface must obey (this is the parity contract):**
- Same four regions, same order, same keybar grammar.
- **One color semantics table, applied everywhere** (see below).
- **One mutation ceremony** (the four-beat in §5) for every write, no exceptions.
- Live preview shows **real config text with the `# BEGIN/END gitid managed:` sentinels
  visible**, so the user sees gitid only owns its block.
- Left-anchored lists/forms; detail/preview on the right.

**Color semantics (restricted, ANSI-safe, adaptive):**
| Role | Cue | Notes |
|------|-----|-------|
| Healthy / success / applied | green + `✓` + word | never color alone |
| Needs action / warning / advisory | yellow + `!` + word | Global SSH/Git recommendations are advisory, never blocking |
| Error / destructive / missing | red + `✗` + word | delete, "key-missing", parse failure |
| Inactive / secondary / hint copy | dim/gray | helper text, disabled keys |
| Focus / selection | reverse or bold, **not** a new hue | selection is structural, not decorative |

Non-negotiables carried from accessibility practice: **never encode meaning in color
alone** (pair every colored state with a glyph *and* a word); the design must remain
fully legible under **`NO_COLOR` / monochrome**; the TUI must be **fully operable by
keyboard alone** (mouse is additive per SSHUI-02, keyboard is the floor). The
terminal-skinned MUI mockup must honor the same — no meaning conveyed by hue only.

---

## 3. The HTML ↔ TUI parity rubric (semantic, not pixel)

Because Material HTML and a terminal are different media, the later critique enforces
**semantic parity**. DLV-04's word "appearance" is scoped here to avoid a permanently
red pixel-diff.

**MUST match — a difference here is a divergence FINDING:**
- **Field set and field order** (e.g. create-SSH order: `Alias prefix` → `SSH Host` →
  `Real hostname` → `Port`).
- **Labels and helper/explanation copy** — **verbatim**. (Global SSH/Git screens must
  *explain each option*; that explanatory copy is contractual.)
- **Option sets / enumerations** — the algorithm catalog; match-strategy options
  (`gitdir:` / `hasconfig:remote.*.url` / both, **default `gitdir`**); delete choices
  ("everything (SSH+Git+key)" vs "Git identity only"); reuse-key vs generate-key.
- **Defaults** — `Port 443`, `IdentitiesOnly yes`, `gitdir` default match, `gpg.format=
  ssh`, `init.defaultBranch = main`, `core.ignorecase = false`. Defaults must be
  recipe-accurate in both media.
- **Flow order and the named-state list** (§4) — same screens, same states, same
  sequence.
- **Presence of the safety affordances** — live preview, confirm step, backup notice,
  error state, and the destructive-confirm — must exist on the same steps in both media.
- **Keybindings surfaced** for equivalent actions.

**MAY differ — acceptable medium difference, NOT a finding:**
- Exact spacing, pixel layout, box-drawing vs CSS borders.
- Material elevation/shadow/ripple/rounded (HTML) vs flat cells (TUI) — provided the
  terminal skin (§0, Risk 1) keeps them close.
- Widget mechanics: an MUI `Select` dropdown vs a TUI inline list/rotary — as long as
  **the option set and default match**.
- Exact color hues — as long as the **semantic role** matches (destructive is red-family
  in both; healthy is green-family in both).
- Scroll (HTML) vs paginate (TUI); mouse-first (HTML) vs keyboard-first (TUI).

**Rule of thumb for the critique:** if a blind user reading a transcript of both screens
(labels, copy, options, order, states) could not tell them apart, parity holds. If the
*words, fields, options, defaults, or safety steps* differ, it is a finding.

---

## 4. Per-surface design notes + state manifests

For each surface: **goal**, the **named states to capture in BOTH media** (this is the
screenshot manifest — lift verbatim into plans), and the **highest-risk affordance**.

### (1) Create-identity flow  — *pilot surface* (Phase 3)
- **Goal:** create a working SSH identity end-to-end: pick algorithm → fill SSH block →
  test against throwaway config → store.
- **States:** `algo-catalog` · `ssh-form-empty` · `ssh-form-filled` (with live `Host`
  block preview) · `ssh-form-blank-prefix` (WYSIWYG: provider host verbatim, SSHUI-03) ·
  `reuse-key-vs-generate` · `macos-globals-block` (`Host *` UseKeychain/AddKeysToAgent
  under `IgnoreUnknown`, SSHUI-05) · `test-stage1-direct` (exact command + real output) ·
  `test-stage2-by-alias` (`ssh -G` proving which `IdentityFile` resolves) · `test-fail`
  (error state) · `confirm-write` (preview of the resulting block + target file) ·
  `backup-notice` (timestamped path) · `result-success`.
- **Highest-risk affordance:** the **test-then-confirm-then-backup** boundary — the
  design must make unmistakable that nothing touched live config until confirm, and that
  the test ran against throwaway temp files. This is the pattern the whole tool's
  credibility rests on; get it right here, reuse everywhere.

### (2) Git-configuration screen (Phase 4)
- **Goal:** author the per-identity Git fragment and wire it up, after the SSH screens.
- **States:** `git-form-empty` · `git-form-filled` (`user.name`/`user.email`,
  `gpg.format=ssh`, `user.signingkey` as **path not literal**, `commit.gpgsign`) ·
  `match-strategy-select` (gitdir / hasconfig / both, live `includeIf` preview) ·
  `review-readonly` (fragment + `includeIf` + `allowed_signers` together) ·
  `confirm-write` · `backup-notice` · `result-success`.
- **Highest-risk affordance:** the **`allowed_signers` email must be byte-identical to
  `user.email`** (GITUI-04). The design should show these two side by side in the review
  state so a mismatch is visible, not buried.

### (3) Identity Manager  — the app's home view (Phase 5)
- **Goal:** see every identity's health at a glance; open SSH-first detail; clone / add
  key / rotate / delete.
- **States:** `list-empty` (first-run, no identities — the true landing state, must be
  designed, not an afterthought) · `list-populated` (per-row health: complete /
  incomplete / git-only / key-unused / key-missing / fragment-path-missing) ·
  `detail-ssh-first` (SSH details first, then Git; **never render git attributes for an
  SSH-only identity**, MGR-03/07) · `action-menu` · `clone-name-prompt` (must be a
  *distinct* new name) · `delete-choice` (**"delete everything (SSH+Git+key)"** vs
  **"delete Git identity only"**, MGR-06) · `confirm-destructive` · `backup-notice`.
- **Highest-risk affordance:** the **delete choice**. Two destructive options; the safer
  one is default-focused; the irreversible "everything" path gets the strongest confirm
  (see §5). Row health states must be legible under NO_COLOR.

### (4) Global SSH options (Phase 6)
- **Goal:** review and safely fix dangerous-when-unset SSH globals, each explained.
- **States:** `options-list` (StrictHostKeyChecking, ForwardAgent, HashKnownHosts,
  IdentitiesOnly, AddKeysToAgent, UseKeychain — each with **current value + risk +
  recommended value**) · `option-detail` (the explanation) · `fix-preview` ·
  `confirm-write` · `backup-notice` · `result-applied`.
- **Highest-risk affordance:** recommendations are **advisory, never blocking**
  (GSSH-01). The design must not look like a compliance gate; "recommended" ≠ "required".
  A yellow `!` advisory, never a red block, and the user can leave any option unchanged.

### (5) Global Git options (Phase 7)
- **Goal:** manage shared git config, each option explained.
- **States:** `options-list` (`init.defaultBranch` **highlighting main vs master**,
  `core.ignorecase=false`, `autocrlf`/eol, global `user.email`, recipe defaults:
  `push.autoSetupRemote`, `pull.rebase`, `fetch.prune`, aliases, color,
  `merge.conflictstyle`, `diff.colorMoved`) · `option-detail` · `fix-preview` ·
  `confirm-write` · `backup-notice` · `result-applied`.
- **Highest-risk affordance:** writes must **preserve content outside managed blocks
  verbatim** (GGIT-01). The preview must show the managed-block sentinels so the user
  sees their hand-written git config is untouched.

### (6) Health check (Phase 8)
- **Goal:** an **SSH section and a Git section**; files exist + parse; detect
  redundant/contradictory config; per-identity + global health.
- **States:** `health-all-green` · `health-with-findings` (severity-sorted, split SSH /
  Git) · `finding-detail` (e.g. multiple `Host *`; `IdentitiesOnly no` with a specific
  `IdentityFile`; an `includeIf` targeting a missing fragment) · `per-identity-health`
  (the slice that feeds a Manager row) · `parse-error` (a config file that won't parse).
- **Highest-risk affordance:** **read-only integrity.** Health must clearly *diagnose,
  not mutate* — it hands off to the Fixer. The design must not blur "reported" with
  "fixed". No write ceremony appears on this surface at all.

### (7) Fixer (Phase 8)
- **Goal:** present SSH and Git problems with severity + explanation + suggested fix;
  apply in place, only with confirm + backup.
- **States:** `fixer-list` (two sections, each problem: severity + plain explanation +
  suggested fix) · `fix-preview` (diff of the exact change) · `confirm-destructive` (for
  anything that rewrites existing directives) · `backup-notice` · `result-applied` ·
  `nothing-to-fix` (the healthy empty state).
- **Highest-risk affordance:** **fix-in-place rewrites of existing user directives.**
  Each fix shows a before/after diff and names the backup path before applying. Batch-fix
  (if offered) must still preview every change; no silent multi-file mutation.

---

## 5. Safety-affordance requirements (the mutation ceremony)

gitid mutates sensitive dotfiles. Backups and confirmations cannot be a wiring-time
afterthought — **they must be first-class screens in the approved set**, or the Phase
3–9 visual-regression gate will never cover them. Every write, on every surface, uses
the **same four-beat ceremony** (this is a consistency rule and a parity checkpoint):

1. **Preview (read-only).** Show the **exact resulting config text / diff**, with the
   **target file path(s) named** and the `# BEGIN/END gitid managed:` sentinels visible.
   Nothing has changed yet — say so.
2. **Confirm.** An explicit, deliberate keystroke. **Destructive actions never
   default-focus "yes".** For the irreversible "delete everything (SSH+Git+key)", require
   the strongest confirm the medium allows (default-focused "No", or a typed
   confirmation) — reserve that weight for genuinely irreversible actions so it doesn't
   become banner-blind noise.
3. **Backup notice.** Show the **timestamped backup path** at (or immediately after)
   confirm — the backup *is* the undo story, so it must be visible, not silent.
4. **Result.** State what changed, in which file(s), and how to restore (the backup
   path again). Success is green `✓`; partial/failed is explicit, never swallowed.

Additional rules the mockups must encode:
- **Test-before-mutate is visible** (create flow): the two-stage test runs against
  throwaway temp files; the design must say the live config is untouched during testing.
- **Advisory ≠ blocking** on the two Global-options surfaces: recommendations are yellow
  `!`, dismissible, and the user may apply none.
- **Health never writes.** Diagnosis and mutation are separated across surfaces (Health →
  Fixer). No ceremony beats appear on Health.
- **Managed-block containment** is shown, not just asserted: previews render the
  sentinels so users see gitid edits only its own block and preserves the rest verbatim.

---

## 6. Approval checklist (DLV-08 — the single human checkpoint)

When the full screenshot set is presented, the user is asked to sign off on **exactly
these**. This is the one hard stop gating Phases 3–9, so the checklist is the contract.

**A. Shell & IA**
- [ ] Global frame approved: header/context bar, body archetypes, status line, always-on
      keybar.
- [ ] Navigation model approved: five primary views on number keys `1`–`5` + palette;
      reserved keys (`Esc`/`q`/`?`/`/`/`Enter`) consistent across all surfaces.
- [ ] Terminal skin approved: the MUI mockup reads as a terminal, and it and the TUI
      dummy read as **one product**.

**B. Per-surface completeness (all seven)**
- [ ] Every named state in each §4 manifest is present in **both** media (HTML + TUI),
      frame-for-frame.
- [ ] Empty / first-run states are designed (not just the happy path) — especially the
      Identity Manager `list-empty` landing and the Fixer `nothing-to-fix`.

**C. Copy, fields, options, defaults FREEZE** (⚠ this is what cascades if skipped)
- [ ] Field order and labels final on every form.
- [ ] Helper/explanation copy final (Global SSH & Git per-option explanations especially).
- [ ] Option sets final: algorithm catalog; match strategy (gitdir/hasconfig/both,
      **default gitdir**); delete choices; reuse-vs-generate key.
- [ ] Defaults recipe-accurate: `Port 443`, `IdentitiesOnly yes`, `gpg.format=ssh`,
      `init.defaultBranch=main`, `core.ignorecase=false`, blank-prefix WYSIWYG.
- [ ] Recipe fidelity confirmed: alias-per-identity `Host` block, `insteadOf` URL
      rewrite, `includeIf hasconfig:`/`gitdir:`, `allowed_signers` line **byte-identical
      to `user.email`** — all visible in the relevant previews.

**D. Safety affordances**
- [ ] Every mutating surface shows the full four-beat ceremony (preview → confirm →
      backup path → result).
- [ ] Destructive actions do not default to "yes"; the irreversible full-delete carries
      the strongest confirm.
- [ ] Health is visibly read-only; advisory options are visibly non-blocking.

**E. Parity & accessibility**
- [ ] The HTML↔TUI **semantic** parity critique (§3) is run and all divergence findings
      are resolved.
- [ ] Legible under `NO_COLOR`/monochrome; no meaning by color alone; keyboard-only
      operability demonstrated.

**F. Explicit acknowledgment**
- [ ] The user understands and accepts that **the approved screenshots become the frozen
      reference set** that every later phase (3–9) is visually regression-tested against,
      and that **no backend logic is written for any surface before this approval**
      (DLV-05).

---

## 7. Handoff to the planner — concrete asks

1. **Structure the phase shell-first → pilot → fan-out → parity → approval** (§1), not
   seven parallel surfaces from a cold start.
2. **Lift the §4 state manifests verbatim** into the mockup and dummy plans so both
   screenshot sets are frame-comparable (mitigates Risk 2).
3. **Constrain the `/mui` theme to the terminal skin** in the mockup plan's task list
   (mitigates Risk 1); name `agent-ui-ux-designer` + `/mui` in every UI task (DLV-02).
4. **Mock with real, recipe-accurate copy and option lists now** — the approval is a
   copy/field/defaults freeze (mitigates Risk 3).
5. **Make the mutation ceremony (§5) a required set of states**, not an implied one, so
   backups/confirms are in the approved reference and covered by the Phase 3–9 gate.
6. **Scope the DLV-04 parity gate to §3's semantic rubric** in the review plan, so the
   live-TUI regression check does not drown in Material-vs-terminal pixel noise.
