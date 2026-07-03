package dummytui

import (
	lipgloss "charm.land/lipgloss/v2"
)

// surface_gitscreen.go registers the git-screen surface (02-UX-DIRECTION.md
// §4(2), Phase 4's git-configuration screen) as a KEYLESS modal flow
// launched FROM identity-manager via the target-owned LaunchFrom/LaunchKey
// binding (review C3): this file alone wires the launch point — no edit to
// data.go or model.go is needed. LaunchKey "g" is allocated to git-screen in
// doc.go/02-UX-DIRECTION.md §2's key-allocation table (the single
// authority); the registration-time collision guard in registry.go rejects
// any future surface that tries to claim it. Register (not RegisterOrReplace)
// is used, mirroring surface_createflow.go — git-screen has no data.go
// placeholder to replace; the empty ActivationKey is exempt from the
// duplicate-key check (review H2), so create-flow and git-screen both
// register keyless without colliding.
//
// The seven screens below mirror, byte-for-byte on labels/copy/defaults,
// the /mui mockup built in Task 1
// (.planning/design/mockup-src/src/routes/git-screen/*.route.tsx) and the
// literal recipe copy in src/data/recipeFixtures.ts — every recipe-critical
// value (gpg.format=ssh, the allowed_signers email, the default gitdir
// match strategy, the ~/.gitconfig.d/<identity> fragment path per the
// already-built GITUI-02) is kept as a byte-visible Go string constant
// here, not derived, so it stays a static, diff-able contract. NO backend
// import — only lipgloss (DLV-05 no-backend ALLOWLIST).
//
// Each screen's Render also embeds its manifest.json "signature" — a
// screen-specific unique marker distinct from the "<surface>/<screen>"
// breadcrumb — so design_capture_test.go's TUI subtest and the PTY dummy-nav
// e2e can both assert a capture landed on the RIGHT screen, never a
// same-shaped-but-wrong-state false positive (review HIGH-3c, T-02-FP).

// Recipe-accurate literal copy (recipes/gitconfig.recipe via
// src/data/recipeFixtures.ts — the North Star; structure matches, algorithm
// is ed25519 not the gists' RSA per the recipes' own "structure, not key
// type" caveat). Identity alias "personal" / host "personal.github.com"
// matches recipeFixtures.ts and surface_createflow.go's cf* constants.
//
// gsFragmentFile uses the ALREADY-BUILT GITUI-02 convention
// (~/.gitconfig.d/<identity>) — distinct from the create-flow pilot's own
// recipe-literal ~/.gitconfig_<identity> naming (cfSSHHostBlock etc. use a
// different, unrelated fixture set); both are internally consistent with
// their own recipeFixtures.ts exports.
const (
	gsUserName      = "Personal Identity"
	gsUserEmail     = "you@personal.example"
	gsGpgFormat     = "ssh"
	gsSigningKey    = "~/.ssh/id_ed25519_personal.pub"
	gsCommitGpgSign = "true"

	gsFragmentFile       = "~/.gitconfig.d/personal"
	gsGitconfigFile      = "~/.gitconfig"
	gsAllowedSignersFile = "~/.ssh/allowed_signers"

	gsFragmentText = `[user]
    name = ` + gsUserName + `
    email = ` + gsUserEmail + `
    signingkey = ` + gsSigningKey + `

[gpg]
    format = ` + gsGpgFormat + `

[commit]
    gpgsign = ` + gsCommitGpgSign

	gsIncludeIfGitdirLine = `[includeIf "gitdir:~/personal/"]
    path = ` + gsFragmentFile

	gsIncludeIfHasconfigLine = `[includeIf "hasconfig:remote.*.url:git@personal.github.com:*/**"]
    path = ` + gsFragmentFile

	gsMatchStrategyDefault = "gitdir"

	gsAllowedSignersKeyMaterial = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIDesignMockupFixtureKeyNotReal0"
	gsAllowedSignersLine        = gsUserEmail + " " + gsAllowedSignersKeyMaterial

	gsSentinelBegin = "# BEGIN gitid managed: personal"
	gsSentinelEnd   = "# END gitid managed: personal"

	gsGitconfigBackupPath      = "~/.gitconfig.backup.2026-07-03T03-59-12Z"
	gsAllowedSignersBackupPath = "~/.ssh/allowed_signers.backup.2026-07-03T03-59-12Z"

	gsResultMessage = `Git identity "personal" configured — ` + gsFragmentFile + ` now applies via the ` + gsMatchStrategyDefault + ` match strategy.`

	// gsFieldsCompactLine{1,2,3} are a TUI-only condensed field=value
	// rendering of the fragment (label + value pairs, no INI blank-line
	// spacers) — used on review-readonly/confirm-write where the fixed
	// 80x24 PTY viewport (20 available rows, model.go verticalMargin=4)
	// leaves no room for the full multi-line INI block on top of THREE
	// targets' previews. §3's parity rubric explicitly allows this ("MAY
	// differ: exact spacing, pixel layout... provided the terminal skin
	// keeps them close") — field set/order/labels/values still match the
	// /mui mockup exactly, only the line-wrapping differs.
	gsFieldsCompactLine1 = "  user.name=" + gsUserName + "   user.email=" + gsUserEmail
	gsFieldsCompactLine2 = "  gpg.format=" + gsGpgFormat + "   commit.gpgsign=" + gsCommitGpgSign
	gsFieldsCompactLine3 = "  user.signingkey=" + gsSigningKey
)

var (
	gsGitconfigIncludeBlockText = gsSentinelBegin + "\n" + gsIncludeIfGitdirLine + "\n" + gsSentinelEnd

	// gsAllowedSignersLineDisplay word-wraps gsAllowedSignersLine onto two
	// rows (email, then the ssh-ed25519 key material) so it never exceeds
	// modalWidth (model.go: min(termWidth-8, 72) = 72 on the 80-col PTY) —
	// the raw single-line gsAllowedSignersLine is 90+ columns wide. The
	// WRITTEN value is still the single-line gsAllowedSignersLine
	// (TestGitScreen_AllowedSignersEmailMatchesUserEmail asserts against
	// that); this is a display-only wrap.
	gsAllowedSignersLineDisplay = gsUserEmail + "\n  " + gsAllowedSignersKeyMaterial
)

// Screen-specific signatures — MUST stay byte-identical to
// .planning/design/git-screen/manifest.json's "signature" field per screen
// (review HIGH-3c: a screen-specific marker, never a generic reused string).
const (
	sigGitFormEmpty        = "SIG-GIT-FORM-EMPTY"
	sigGitFormFilled       = "SIG-GIT-FORM-FILLED"
	sigMatchStrategySelect = "SIG-MATCH-STRATEGY-SELECT-DEFAULT-GITDIR"
	sigReviewReadonly      = "SIG-REVIEW-READONLY-ALLOWED-SIGNERS"
	sigGitConfirmWrite     = "SIG-GIT-CONFIRM-WRITE"
	sigGitBackupNotice     = "SIG-GIT-BACKUP-NOTICE"
	sigGitResultSuccess    = "SIG-GIT-RESULT-SUCCESS"
)

// Local styles (D-02: no backend imports, so no dependency on tui/styles.go
// — a small self-contained palette mirroring 02-UX-DIRECTION.md §2's color
// semantics table: healthy=green+word, warning=yellow+word,
// error=red+word, never color alone). Mirrors surface_createflow.go's
// styleCF* set, kept package-local under a gs* prefix so this file has no
// cross-surface-file dependency.
var (
	styleGSHeading = lipgloss.NewStyle().Bold(true)
	styleGSDim     = lipgloss.NewStyle().Faint(true)
	styleGSSuccess = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	styleGSWarning = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)
)

func init() {
	Register(SurfaceDef{
		ID:         "git-screen",
		Title:      "Git Configuration",
		LaunchFrom: "identity-manager",
		LaunchKey:  "g",
		Screens: []ScreenDef{
			{ID: "git-form-empty", Keys: map[string]string{"f": "git-form-filled"}, Render: renderGitFormEmpty},
			{ID: "git-form-filled", Keys: map[string]string{"m": "match-strategy-select"}, Render: renderGitFormFilled},
			{ID: "match-strategy-select", Keys: map[string]string{"r": "review-readonly"}, Render: renderMatchStrategySelect},
			{ID: "review-readonly", Keys: map[string]string{"w": "confirm-write"}, Render: renderReviewReadonly},
			{ID: "confirm-write", Keys: map[string]string{"y": "backup-notice"}, Render: renderGitConfirmWrite},
			{ID: "backup-notice", Keys: map[string]string{"z": "result-success"}, Render: renderGitBackupNotice},
			{ID: "result-success", Render: renderGitResultSuccess},
		},
	})
}

// gsBody joins the heading, body lines, and the trailing signature marker
// into one screen body string — every render func below funnels through
// this so the signature is always present, in the same place, deterministically.
func gsBody(heading, sig string, lines ...string) string {
	all := make([]string, 0, len(lines)+2)
	all = append(all, styleGSHeading.Render(heading))
	all = append(all, lines...)
	all = append(all, "", styleGSDim.Render("["+sig+"]"))
	return lipgloss.JoinVertical(lipgloss.Left, all...)
}

func renderGitFormEmpty() string {
	return gsBody("Git identity (per-identity)", sigGitFormEmpty,
		"user.name:         (empty)",
		"user.email:        (empty — must byte-match allowed_signers later, GITUI-04)",
		"gpg.format:        "+gsGpgFormat+" (fixed)",
		"user.signingkey:   (empty — a PATH to the public key, never the key material itself)",
		"commit.gpgsign:    true (default)",
		"",
		styleGSDim.Render("Live fragment preview: (fill in the fields to see the resulting fragment)"),
	)
}

func renderGitFormFilled() string {
	return gsBody("Git identity (per-identity)", sigGitFormFilled,
		"user.name:         "+gsUserName,
		"user.email:        "+gsUserEmail,
		"gpg.format:        "+gsGpgFormat+" (fixed)",
		"user.signingkey:   "+gsSigningKey+" (a PATH to the public key — never the key material itself)",
		"commit.gpgsign:    "+gsCommitGpgSign,
		"",
		styleGSDim.Render("Live fragment preview:"),
		gsFragmentText,
	)
}

func renderMatchStrategySelect() string {
	return gsBody("Match strategy", sigMatchStrategySelect,
		styleGSSuccess.Render("gitdir: ✓ default")+" — applies by repository directory path.",
		"hasconfig:remote.*.url — applies by matching remote URL, combinable with gitdir.",
		"both — applies both includeIf blocks together.",
		"",
		styleGSDim.Render("Live includeIf preview ("+gsMatchStrategyDefault+"):"),
		gsIncludeIfGitdirLine,
		"",
		styleGSDim.Render("hasconfig alternative:"),
		gsIncludeIfHasconfigLine,
	)
}

// renderReviewReadonly and renderGitConfirmWrite use the compact
// gsFieldsCompact* / gsAllowedSignersLineDisplay forms (not the raw
// multi-line INI blocks renderGitFormFilled uses) — reviewing/confirming
// THREE files' content on one screen, on the fixed 80x24 PTY viewport
// (20 available rows, model.go verticalMargin=4), leaves no budget for the
// full sentineled block per target. §3's parity rubric explicitly allows
// this ("MAY differ: exact spacing, pixel layout... provided the terminal
// skin keeps them close") — field set/order/labels/values still match the
// /mui mockup exactly.

func renderReviewReadonly() string {
	return gsBody("Review (read-only)", sigReviewReadonly,
		styleGSDim.Render("Fragment ("+gsFragmentFile+"):"),
		gsFieldsCompactLine1,
		gsFieldsCompactLine2,
		gsFieldsCompactLine3,
		"",
		styleGSDim.Render("includeIf ("+gsMatchStrategyDefault+", default):"),
		gsIncludeIfGitdirLine,
		"",
		styleGSDim.Render("~/.ssh/allowed_signers:"),
		gsAllowedSignersLineDisplay,
		"user.email:        "+gsUserEmail,
		styleGSSuccess.Render("✓ Byte-identical — trusted for this identity's signatures."),
	)
}

func renderGitConfirmWrite() string {
	return gsBody("Confirm write", sigGitConfirmWrite,
		styleGSWarning.Render("! Nothing has changed yet — review below, then confirm."),
		gsFragmentFile+" (new, sentinels visible):",
		gsSentinelBegin,
		gsFieldsCompactLine1,
		gsFieldsCompactLine2,
		gsFieldsCompactLine3,
		gsSentinelEnd,
		"",
		gsGitconfigFile+" (append, sentinels visible):",
		gsGitconfigIncludeBlockText,
		"",
		gsAllowedSignersFile+" (append):",
		gsAllowedSignersLineDisplay,
	)
}

func renderGitBackupNotice() string {
	return gsBody("Backups created", sigGitBackupNotice,
		styleGSSuccess.Render("✓ "+gsGitconfigFile+" backup: "+gsGitconfigBackupPath),
		styleGSSuccess.Render("✓ "+gsAllowedSignersFile+" backup: "+gsAllowedSignersBackupPath),
		"",
		styleGSDim.Render("A full copy of each previous file was saved before any change was applied —"),
		styleGSDim.Render("these backup paths are the undo story."),
	)
}

func renderGitResultSuccess() string {
	return gsBody("✓ Git identity configured", sigGitResultSuccess,
		styleGSSuccess.Render("✓ "+gsResultMessage),
		"Written to "+gsFragmentFile+", appended to "+gsGitconfigFile+" and "+gsAllowedSignersFile+".",
		"",
		styleGSDim.Render("To restore by hand, the backups are at "+gsGitconfigBackupPath+" and "+gsAllowedSignersBackupPath+"."),
	)
}

// gsSignatureByScreen is a lookup table screen-ID -> signature, mirroring
// manifest.json — used by surface_gitscreen_test.go.
var gsSignatureByScreen = map[string]string{
	"git-form-empty":        sigGitFormEmpty,
	"git-form-filled":       sigGitFormFilled,
	"match-strategy-select": sigMatchStrategySelect,
	"review-readonly":       sigReviewReadonly,
	"confirm-write":         sigGitConfirmWrite,
	"backup-notice":         sigGitBackupNotice,
	"result-success":        sigGitResultSuccess,
}
