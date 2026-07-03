package dummytui

import (
	lipgloss "charm.land/lipgloss/v2"
)

// surface_globalssh.go registers the global-ssh surface (02-UX-DIRECTION.md
// §4.4, Phase 6's Global SSH options screen) as a PRIMARY view — number key
// `2` (review HIGH-2): it REPLACES the 02-02 `data.go` placeholder that
// currently owns key `2` via RegisterOrReplace, so there is NO edit to
// data.go and NO duplicate-activation-key conflict. This file alone wires
// the replacement.
//
// The six screens below mirror, byte-for-byte on labels/copy/defaults, the
// /mui mockup built in Task 1
// (.planning/design/mockup-src/src/routes/global-ssh/*.route.tsx) and the
// literal recipe copy in src/data/recipeFixtures.ts's globalSsh* exports —
// every recipe-critical value (the GSSH-01 6-option set, the Host * block
// text, the backup path) is kept as a byte-visible Go string constant here,
// not derived, so it stays a static, diff-able contract (matching
// surface_createflow.go/surface_gitscreen.go/surface_identitymanager.go's
// own precedent). NO backend import — only lipgloss (DLV-05 no-backend
// ALLOWLIST).
//
// Intra-surface ScreenDef.Keys allocate a linear ceremony chain `v`
// (→ option-detail), `f` (→ fix-preview), `w` (→ confirm-write), `y`
// (→ backup-notice), `z` (→ result-applied) — mirroring git-screen's own
// f/m/r/w/y/z chain. None of these collide with `n`/`g`, the only two
// globally reserved LaunchKeys in the 02-UX-DIRECTION.md §2 key-allocation
// table (the registry.go registration-time collision guard rejects any
// clash loudly).
//
// Highest-risk affordance (§4.4, §5): recommendations are ADVISORY, NEVER
// BLOCKING — a yellow `!`, never a red compliance gate, and the user may
// leave any option unchanged. gsshChosenToApply/gsshDeclinedOption
// concretely demonstrate this: the fixture applies 3 of the 4 "needs
// action" recommendations and deliberately leaves ForwardAgent unchanged,
// visible through fix-preview -> confirm-write -> backup-notice ->
// result-applied.
//
// Each screen's Render also embeds its manifest.json "signature" — a
// screen-specific unique marker distinct from the "<surface>/<screen>"
// breadcrumb — so design_capture_test.go's TUI subtest and the PTY dummy-nav
// e2e can both assert a capture landed on the RIGHT screen, never a
// same-shaped-but-wrong-state false positive (review HIGH-3c, T-02-FP).

// gsshOption mirrors recipeFixtures.ts's GlobalSSHOption shape — one entry
// per GSSH-01 dangerous-by-default option.
type gsshOption struct {
	key, current, risk, recommended, oneLiner string
	needsAction                               bool
}

// gsshOptions is the Go mirror of recipeFixtures.ts's globalSshOptions —
// byte-identical keys/values/one-liners, not derived (a static, diff-able
// contract). Order matches 02-UX-DIRECTION.md §4.4's verbatim list.
var gsshOptions = []gsshOption{
	{key: "StrictHostKeyChecking", current: "not set (OpenSSH default: ask)", risk: "Medium", recommended: "ask", needsAction: true, oneLiner: "Stating \"ask\" explicitly removes ambiguity about how an unknown host key is handled."},
	{key: "ForwardAgent", current: "not set (OpenSSH default: no)", risk: "Medium", recommended: "no", needsAction: true, oneLiner: "Globally forwarding your agent lets any host you connect to authenticate elsewhere as you."},
	{key: "HashKnownHosts", current: "not set", risk: "Low", recommended: "yes", needsAction: true, oneLiner: "Hashing known_hosts hides which hosts you connect to if the file ever leaks."},
	{key: "IdentitiesOnly", current: "not set globally (set per-Host by gitid)", risk: "High", recommended: "yes", needsAction: true, oneLiner: "Without it, ssh may offer every key it knows about to every host — leaking which OTHER keys you hold."},
	{key: "AddKeysToAgent", current: "yes", risk: "Low", recommended: "yes", needsAction: false, oneLiner: "Already set — keys stay available in the agent for the session (recipes/ssh-config.recipe Host * block)."},
	{key: "UseKeychain", current: "yes (macOS only)", risk: "Low", recommended: "yes", needsAction: false, oneLiner: "Already set — stores the key passphrase in the macOS Keychain (guarded by IgnoreUnknown on Linux)."},
}

// gsshDetailTarget mirrors recipeFixtures.ts's globalSshDetailTarget —
// option-detail's single target (IdentitiesOnly, the highest-risk entry).
var gsshDetailTarget = gsshOptions[3] // IdentitiesOnly

// GSSH-01 contractual (verbatim, §3) explanation copy — byte-identical to
// recipeFixtures.ts's globalSshDetailExplanation.
const gsshDetailExplanation = `When IdentitiesOnly is not set (or set to "no"), ssh may try EVERY key it can find -- every file in ~/.ssh matching the default names, plus every key already loaded in your ssh-agent -- against any host you connect to. On a machine with multiple identities (personal, work, client keys), this means:

  - the wrong key can be offered first, revealing to a server which OTHER keys you hold;
  - a host you don't fully trust can trigger authentication attempts meant for a completely different identity.

Setting "IdentitiesOnly yes" on a Host block restricts ssh to ONLY the IdentityFile(s) listed for that host -- this is why every gitid-managed Host block (recipes/ssh-config.recipe) already sets it per-identity. This screen recommends also stating it explicitly in the global Host * block, as a safety net for any Host entries gitid does not manage.`

// gsshAdvisoryNote — byte-identical to recipeFixtures.ts's globalSshAdvisoryNote.
const gsshAdvisoryNote = "Recommended, not required -- you can leave any option unchanged. This is advisory, never a compliance gate."

// §4.4/§5 highest-risk affordance: 3 of 4 "needs action" options applied,
// ForwardAgent deliberately declined — byte-identical to recipeFixtures.ts's
// globalSshChosenToApply/globalSshDeclinedOption.
const (
	gsshChosenSummary  = "3 of 4"
	gsshDeclinedOption = "ForwardAgent"

	gsshTargetFile = "~/.ssh/config"

	gsshSentinelBegin = "# BEGIN gitid managed: global-ssh"
	gsshSentinelEnd   = "# END gitid managed: global-ssh"

	// gsshHostStarBlockText mirrors recipeFixtures.ts's
	// globalSshHostStarBlockText — extends the recipe's own `Host *` shape
	// (recipes/ssh-config.recipe) with the 3 chosen recommendations plus
	// the 2 already-recommended options. ForwardAgent is intentionally
	// absent — declined by the user.
	gsshHostStarBlockText = `IgnoreUnknown UseKeychain

Host *
    StrictHostKeyChecking ask
    HashKnownHosts yes
    IdentitiesOnly yes
    UseKeychain yes
    AddKeysToAgent yes`

	gsshBackupPath = "~/.ssh/config.backup.2026-07-03T03-59-12Z"

	gsshResultMessage = "3 of 4 recommended options applied to Host * in ~/.ssh/config. ForwardAgent was left unchanged, as chosen -- advisory, never required."
)

var (
	gsshManagedBlockText = gsshSentinelBegin + "\n" + gsshHostStarBlockText + "\n" + gsshSentinelEnd

	// gsshFixPreviewLines mirrors recipeFixtures.ts's
	// globalSshFixPreviewLines — the diff-style lines fix-preview shows.
	gsshFixPreviewLines = []string{
		"+ StrictHostKeyChecking ask",
		"+ HashKnownHosts yes",
		"+ IdentitiesOnly yes",
		"  UseKeychain yes (already set)",
		"  AddKeysToAgent yes (already set)",
		"  ForwardAgent -- left unchanged (declined; advisory, not required)",
	}
)

// Screen-specific signatures — MUST stay byte-identical to
// .planning/design/global-ssh/manifest.json's "signature" field per screen
// (review HIGH-3c: a screen-specific marker, never a generic reused string).
const (
	sigGSSHOptionsList   = "SIG-GSSH-OPTIONS-LIST-6-DANGEROUS"
	sigGSSHOptionDetail  = "SIG-GSSH-OPTION-DETAIL-IDENTITIESONLY"
	sigGSSHFixPreview    = "SIG-GSSH-FIX-PREVIEW-PARTIAL-APPLY"
	sigGSSHConfirmWrite  = "SIG-GSSH-CONFIRM-WRITE"
	sigGSSHBackupNotice  = "SIG-GSSH-BACKUP-NOTICE"
	sigGSSHResultApplied = "SIG-GSSH-RESULT-APPLIED"
)

// Local styles (D-02: no backend imports, so no dependency on
// tui/styles.go — a small self-contained palette mirroring
// 02-UX-DIRECTION.md §2's color semantics table: healthy=green+word,
// warning/advisory=yellow+word, never color alone). Mirrors
// surface_createflow.go's styleCF* / surface_gitscreen.go's styleGS* /
// surface_identitymanager.go's styleIM* sets, kept package-local under a
// gssh* prefix so this file has no cross-surface-file dependency.
var (
	styleGSSHHeading = lipgloss.NewStyle().Bold(true)
	styleGSSHDim     = lipgloss.NewStyle().Faint(true)
	styleGSSHSuccess = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	styleGSSHWarning = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)
)

func init() {
	RegisterOrReplace(SurfaceDef{
		ID:            "global-ssh",
		Title:         "Global SSH",
		ActivationKey: "2",
		Screens: []ScreenDef{
			{ID: "options-list", Keys: map[string]string{"v": "option-detail"}, Render: renderGSSHOptionsList},
			{ID: "option-detail", Keys: map[string]string{"f": "fix-preview"}, Render: renderGSSHOptionDetail},
			{ID: "fix-preview", Keys: map[string]string{"w": "confirm-write"}, Render: renderGSSHFixPreview},
			{ID: "confirm-write", Keys: map[string]string{"y": "backup-notice"}, Render: renderGSSHConfirmWrite},
			{ID: "backup-notice", Keys: map[string]string{"z": "result-applied"}, Render: renderGSSHBackupNotice},
			{ID: "result-applied", Render: renderGSSHResultApplied},
		},
	})
}

// gsshBody joins the heading, body lines, and the trailing signature marker
// into one screen body string — every render func below funnels through
// this so the signature is always present, in the same place,
// deterministically. Mirrors cfBody/gsBody/imBody.
func gsshBody(heading, sig string, lines ...string) string {
	all := make([]string, 0, len(lines)+2)
	all = append(all, styleGSSHHeading.Render(heading))
	all = append(all, lines...)
	all = append(all, "", styleGSSHDim.Render("["+sig+"]"))
	return lipgloss.JoinVertical(lipgloss.Left, all...)
}

// gsshGlyph pairs an option's needsAction state with the color-semantics
// glyph (02-UX-DIRECTION.md §2: healthy=✓, needs-action/advisory=!) —
// always rendered together with a WORD (never color alone).
func gsshGlyph(needsAction bool) string {
	if needsAction {
		return styleGSSHWarning.Render("!")
	}
	return styleGSSHSuccess.Render("✓")
}

// gsshOptionLine renders ONE option as a SINGLE compact line: glyph + key +
// current -> recommended (or "(already set)") + a bracketed risk word. This
// is a TUI-only compaction (§3 "MAY differ: exact spacing, pixel layout...
// provided the terminal skin keeps them close") — the live gitid-dummy
// binary renders inside a REAL, fixed 80x24 PTY (e2e/ui_pty_e2e_test.go's
// ptyTermWidth/ptyTermHeight) with NO scroll region, unlike the static
// RenderScreen()->freeze capture path, which has no height limit. The
// per-option one-liner explanation (present on the /mui mockup's
// options-list row) is intentionally NOT repeated here to stay inside that
// budget across all 6 options plus the header/status/keybar chrome — the
// SAME field set/order/values (key, current, recommended, risk) still
// match; the full contractual explanation is one keystroke away on
// option-detail. Mirrors surface_gitscreen.go's gsFieldsCompactLine*
// precedent (same viewport constraint, same resolution).
func gsshOptionLine(opt gsshOption) string {
	glyph := gsshGlyph(opt.needsAction)
	valueText := opt.current + " -> " + opt.recommended
	if !opt.needsAction {
		valueText = opt.current + " (already set)"
	}
	return glyph + " " + opt.key + "   " + valueText + "   [" + opt.risk + "]"
}

func renderGSSHOptionsList() string {
	lines := []string{styleGSSHWarning.Render(gsshAdvisoryNote)}
	for _, opt := range gsshOptions {
		lines = append(lines, gsshOptionLine(opt))
	}
	lines = append(lines,
		styleGSSHDim.Render("v full explanation ("+gsshDetailTarget.key+")   f preview fix"),
	)
	return gsshBody("Global SSH options", sigGSSHOptionsList, lines...)
}

func renderGSSHOptionDetail() string {
	t := gsshDetailTarget
	return gsshBody(t.key, sigGSSHOptionDetail,
		"current: "+t.current+"   risk: "+t.risk+"   recommended: "+t.recommended,
		"",
		gsshDetailExplanation,
		"",
		styleGSSHWarning.Render("! "+gsshAdvisoryNote),
	)
}

func renderGSSHFixPreview() string {
	lines := []string{
		styleGSSHWarning.Render("! Applying " + gsshChosenSummary + " recommended options to Host * in " + gsshTargetFile + "."),
		styleGSSHWarning.Render("! " + gsshDeclinedOption + " was left unchanged — advisory, not required."),
		"",
		styleGSSHDim.Render("Diff — Host * in " + gsshTargetFile + ":"),
	}
	lines = append(lines, gsshFixPreviewLines...)
	lines = append(lines,
		"",
		"gitid only owns the block between its sentinels — everything else in "+gsshTargetFile+" is preserved verbatim.",
	)
	return gsshBody("Preview fix", sigGSSHFixPreview, lines...)
}

func renderGSSHConfirmWrite() string {
	return gsshBody("Confirm write", sigGSSHConfirmWrite,
		styleGSSHWarning.Render("! Nothing has changed yet — review below, then confirm."),
		styleGSSHWarning.Render("! "+gsshDeclinedOption+" is intentionally absent — left unchanged."),
		"",
		gsshTargetFile+" (append, sentinels visible):",
		gsshManagedBlockText,
	)
}

func renderGSSHBackupNotice() string {
	return gsshBody("Backup created", sigGSSHBackupNotice,
		styleGSSHSuccess.Render("✓ "+gsshTargetFile+" backup: "+gsshBackupPath),
		"",
		styleGSSHDim.Render("A full copy of the previous file was saved before any change was applied —"),
		styleGSSHDim.Render("this backup path is the undo story."),
	)
}

func renderGSSHResultApplied() string {
	return gsshBody("✓ Options applied", sigGSSHResultApplied,
		styleGSSHSuccess.Render("✓ "+gsshResultMessage),
		"Written to the Host * block in "+gsshTargetFile+".",
		"You can revisit "+gsshDeclinedOption+" here any time — nothing was ever required.",
		"",
		styleGSSHDim.Render("To restore by hand, the backup is at "+gsshBackupPath+"."),
	)
}

// gsshSignatureByScreen is a lookup table screen-ID -> signature, mirroring
// manifest.json — used by surface_globalssh_test.go.
var gsshSignatureByScreen = map[string]string{
	"options-list":   sigGSSHOptionsList,
	"option-detail":  sigGSSHOptionDetail,
	"fix-preview":    sigGSSHFixPreview,
	"confirm-write":  sigGSSHConfirmWrite,
	"backup-notice":  sigGSSHBackupNotice,
	"result-applied": sigGSSHResultApplied,
}
