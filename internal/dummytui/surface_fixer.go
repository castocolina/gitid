package dummytui

import (
	"strconv"

	lipgloss "charm.land/lipgloss/v2"
)

// surface_fixer.go registers the fixer surface (02-UX-DIRECTION.md §4.7,
// Phase 8's Fixer screen) as a PRIMARY view — number key `5` (review
// HIGH-2): it REPLACES the 02-02 `data.go` placeholder that currently owns
// key `5` via RegisterOrReplace, so there is NO edit to data.go and NO
// duplicate-activation-key conflict. This file alone wires the replacement.
//
// The six screens below mirror, byte-for-byte on labels/copy/defaults, the
// /mui mockup built in Task 1
// (.planning/design/mockup-src/src/routes/fixer/*.route.tsx) and the
// literal recipe copy in src/data/recipeFixtures.ts's fixer* exports —
// every recipe-critical value (the fixable findings list, the flagship
// rewrite diff, the backup path) is kept as a byte-visible Go string
// constant here, not derived, so it stays a static, diff-able contract
// (matching surface_createflow.go/surface_gitscreen.go/
// surface_identitymanager.go/surface_globalssh.go/surface_globalgit.go/
// surface_health.go's own precedent). NO backend import — only lipgloss
// (DLV-05 no-backend ALLOWLIST).
//
// Intra-surface ScreenDef.Keys allocate a linear ceremony chain `v` (→
// fix-preview), `x` (→ confirm-destructive), `y` (→ backup-notice), `z`
// (→ result-applied) from the entry screen, plus `e` (→ nothing-to-fix,
// mirroring surface_identitymanager.go's own `e` -> list-empty
// allocation and surface_health.go's alternate-state key pattern) — never
// `n`/`g` (create-flow's/git-screen's own LaunchKeys, the only two
// globally reserved letters in the 02-UX-DIRECTION.md §2 key-allocation
// table — the registry.go registration-time collision guard rejects any
// clash loudly).
//
// Highest-risk affordance (§4.7, §5): FIX-IN-PLACE REWRITES OF EXISTING
// DIRECTIVES. The flagship walk-through target (fixTarget) is the SAME
// "ssh-identitiesonly-contradiction" finding surface_health.go's
// hlthFindingDetailTarget deep-dives (traceable HLTH-04 hand-off):
// rewriting IdentitiesOnly no -> yes on an EXISTING Host clientb.github.com
// block. fix-preview renders a true before/after `-`/`+` diff (not an
// additions-only `+` list, unlike global-ssh's/global-git's fix-preview);
// confirm-destructive uses the strongest confirm this medium allows short
// of a typed confirmation (mirrors surface_identitymanager.go's "delete
// everything" precedent) — destructive actions never default-focus "yes"
// (§5). backup-notice names the timestamped backup path BEFORE applying.
//
// Each screen's Render also embeds its manifest.json "signature" — a
// screen-specific unique marker distinct from the "<surface>/<screen>"
// breadcrumb — so design_capture_test.go's TUI subtest and the PTY dummy-nav
// e2e can both assert a capture landed on the RIGHT screen, never a
// same-shaped-but-wrong-state false positive (review HIGH-3c, T-02-FP).

// fixFinding mirrors recipeFixtures.ts's HealthFinding shape (reused
// byte-identically from surface_health.go's hlthFinding, kept as a
// separate package-local type so this file has no cross-surface-file
// dependency, matching every prior fan-out surface's own isolation).
type fixFinding struct {
	id, section, family, title, explanation, suggestedFix string
	severity                                              hlthSeverity
}

// fixFindings is the Go mirror of recipeFixtures.ts's fixerFindings — the
// subset of hlthFindings that carries a suggestedFix (the fixer only
// lists ACTIONABLE problems, §4.7's "each problem: severity + plain
// explanation + suggested fix"). Byte-identical ids/sections/severities/
// copy to surface_health.go's hlthFindings — traceable, not re-derived
// (HLTH-04's own "available on the Fixer screen" hand-off, honored here).
var fixFindings = []fixFinding{
	{
		id: "ssh-key-perms-archived", section: "SSH", severity: hlthCritical, family: "Permissions",
		title:        "Private key is world-readable",
		explanation:  "~/.ssh/id_ed25519_archived is mode 0644 -- gitid-managed keys must be 0600. Any other account on this machine can read the key material.",
		suggestedFix: "chmod 0600 ~/.ssh/id_ed25519_archived -- available on the Fixer screen.",
	},
	{
		id: "ssh-identitiesonly-contradiction", section: "SSH", severity: hlthError, family: "Coherence",
		title:        "IdentitiesOnly no contradicts an explicit IdentityFile",
		explanation:  "Host clientb.github.com sets IdentitiesOnly no while also naming IdentityFile ~/.ssh/id_ed25519_clientB -- ssh may still offer every other key it knows before falling back to the one explicitly configured (HLTH-04).",
		suggestedFix: "Set IdentitiesOnly yes on the clientb.github.com Host block -- available on the Fixer screen.",
	},
	{
		id: "git-includeif-missing-fragment", section: "Git", severity: hlthError, family: "Orphans",
		title:        "includeIf targets a missing fragment",
		explanation:  "[includeIf \"gitdir:~/legacy/\"] in ~/.gitconfig points at ~/.gitconfig.d/legacy, which does not exist on disk -- commits made under ~/legacy/ silently fall back to your global git identity instead of \"legacy\" (HLTH-04).",
		suggestedFix: "Restore ~/.gitconfig.d/legacy, or repoint the includeIf -- available on the Fixer screen.",
	},
	{
		id: "ssh-duplicate-host-star", section: "SSH", severity: hlthWarning, family: "Redundancy",
		title:        "Duplicate Host * stanza",
		explanation:  "~/.ssh/config defines Host * twice -- line 4 and line 41. The second stanza silently overrides directives set by the first (HLTH-03).",
		suggestedFix: "Merge the two Host * stanzas into one -- available on the Fixer screen.",
	},
}

// fixFindingByID looks up a fixture finding by id -- used by fix-preview,
// confirm-destructive, and fixer-list's own highlight so the underlying
// list stays the single source of truth.
func fixFindingByID(id string) fixFinding {
	for _, f := range fixFindings {
		if f.id == id {
			return f
		}
	}
	panic("dummytui: surface_fixer.go: no fixture finding with id " + id)
}

// fixTarget mirrors recipeFixtures.ts's fixerTarget -- fix-preview's/
// confirm-destructive's/backup-notice's/result-applied's single
// walk-through target, the flagship §4.7 highest-risk affordance: a
// fix-in-place REWRITE of an EXISTING directive's value, not merely an
// addition.
var fixTarget = fixFindingByID("ssh-identitiesonly-contradiction")

const (
	fixTargetFile = "~/.ssh/config"
	fixTargetHost = "clientb.github.com"

	fixBackupPath = "~/.ssh/config.backup.2026-07-03T03-59-12Z"

	fixResultMessage = "IdentitiesOnly set to yes on Host " + fixTargetHost + " in " + fixTargetFile + "."

	fixConfirmDestructiveNote = "This rewrites a directive already present in your SSH config. Review the diff above before confirming -- this cannot be undone without restoring the backup."

	// fixSafetyNote mirrors recipeFixtures.ts's fixerSafetyNote -- shown on
	// every one of the 6 fixer screens (§4.7, §5): fixes are always
	// previewed, confirmed, and backed up before anything is written.
	fixSafetyNote = "Every fix is previewed, confirmed, and backed up before anything is written -- never a blind write."
)

// fixFixPreviewLines mirrors recipeFixtures.ts's fixerFixPreviewLines -- a
// true `-`/`+` rewrite diff (not additions-only), because this fix
// REWRITES an existing directive's value rather than adding a new one
// (T-02-FIX). Two-space context lines show the rest of the existing Host
// block is untouched.
var fixFixPreviewLines = []string{
	"  Host " + fixTargetHost,
	"      Hostname ssh.github.com",
	"      Port 443",
	"      User git",
	"      IdentityFile ~/.ssh/id_ed25519_clientB",
	"-     IdentitiesOnly no",
	"+     IdentitiesOnly yes",
}

// fixNothingToFixSummary mirrors recipeFixtures.ts's
// fixerNothingToFixSummary -- nothing-to-fix's zero-findings summary for
// both sections (§4.7's healthy empty state).
const (
	fixNothingToFixSSH = "SSH -- 0 fixable problems. Every Host block is coherent, every key is 0600."
	fixNothingToFixGit = "Git -- 0 fixable problems. Every includeIf target exists, every allowed_signers email matches."
)

// fixBatchFixNote mirrors recipeFixtures.ts's fixerBatchFixNote (§4.7:
// "Batch-fix (if offered) must still preview every change; no silent
// multi-file mutation.").
var fixBatchFixNote = "Apply all " + strconv.Itoa(len(fixFindings)) + " fixes -- each one still previews its own diff and backup path before writing; nothing is applied silently."

// Screen-specific signatures -- MUST stay byte-identical to
// .planning/design/fixer/manifest.json's "signature" field per screen
// (review HIGH-3c: a screen-specific marker, never a generic reused
// string).
const (
	sigFIXList               = "SIG-FIX-LIST-SSH-GIT-SPLIT"
	sigFIXFixPreview         = "SIG-FIX-PREVIEW-IDENTITIESONLY-DIFF"
	sigFIXConfirmDestructive = "SIG-FIX-CONFIRM-DESTRUCTIVE-REWRITE"
	sigFIXBackupNotice       = "SIG-FIX-BACKUP-NOTICE"
	sigFIXResultApplied      = "SIG-FIX-RESULT-APPLIED"
	sigFIXNothingToFix       = "SIG-FIX-NOTHING-TO-FIX-EMPTY"
)

// Local styles (D-02: no backend imports, so no dependency on
// tui/styles.go -- a small self-contained palette mirroring
// 02-UX-DIRECTION.md §2's color semantics table, reusing the SAME
// severity-glyph contract surface_health.go pins: warning=! yellow,
// error/critical=✗ red (word-distinguished), info=~ cyan. Mirrors
// surface_createflow.go's styleCF* / surface_globalgit.go's styleGGIT* /
// surface_health.go's styleHLTH* sets, kept package-local under a fix*
// prefix so this file has no cross-surface-file dependency.
var (
	styleFIXHeading = lipgloss.NewStyle().Bold(true)
	styleFIXDim     = lipgloss.NewStyle().Faint(true)
	styleFIXSuccess = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	styleFIXWarning = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)
	styleFIXError   = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	styleFIXInfo    = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
)

func init() {
	RegisterOrReplace(SurfaceDef{
		ID:            "fixer",
		Title:         "Fixer",
		ActivationKey: "5",
		Screens: []ScreenDef{
			{ID: "fixer-list", Keys: map[string]string{"v": "fix-preview", "e": "nothing-to-fix"}, Render: renderFIXList},
			{ID: "fix-preview", Keys: map[string]string{"x": "confirm-destructive"}, Render: renderFIXFixPreview},
			{ID: "confirm-destructive", Keys: map[string]string{"y": "backup-notice"}, Render: renderFIXConfirmDestructive},
			{ID: "backup-notice", Keys: map[string]string{"z": "result-applied"}, Render: renderFIXBackupNotice},
			{ID: "result-applied", Render: renderFIXResultApplied},
			{ID: "nothing-to-fix", Render: renderFIXNothingToFix},
		},
	})
}

// fixBody joins the heading, the safety banner, the body lines, and the
// trailing signature marker into one screen body string -- every render
// func below funnels through this so fixSafetyNote and the signature are
// ALWAYS present, in the same place, deterministically, on EVERY fixer
// screen. Mirrors cfBody/gsBody/imBody/gsshBody/ggitBody/hlthBody.
func fixBody(heading, sig string, lines ...string) string {
	all := make([]string, 0, len(lines)+4)
	all = append(all, styleFIXHeading.Render(heading))
	all = append(all, styleFIXDim.Render(fixSafetyNote), "")
	all = append(all, lines...)
	all = append(all, "", styleFIXDim.Render("["+sig+"]"))
	return lipgloss.JoinVertical(lipgloss.Left, all...)
}

// fixSeverityGlyph pairs a finding's severity with the LOCKED
// glyph+color contract (reused byte-identically from surface_health.go's
// hlthSeverityGlyph), always rendered together with the severity's own
// WORD (never color alone).
func fixSeverityGlyph(sev hlthSeverity) string {
	switch sev {
	case hlthCritical, hlthError:
		return styleFIXError.Render("✗ " + string(sev))
	case hlthWarning:
		return styleFIXWarning.Render("! " + string(sev))
	case hlthInfo:
		return styleFIXInfo.Render("~ " + string(sev))
	default:
		return string(sev)
	}
}

// fixFindingLine renders ONE problem as a SINGLE compact line: severity
// glyph+word + title + suggested fix -- the same one-line-per-item TUI
// compaction precedent surface_health.go's hlthFindingLine and
// surface_globalgit.go's ggitOptionLine already established for the
// fixed 80x24 live-PTY viewport (no scroll region), so a multi-problem
// list plus the header/status/keybar chrome never overflows.
func fixFindingLine(f fixFinding) string {
	return fixSeverityGlyph(f.severity) + "  " + f.title + "  -- " + f.suggestedFix
}

func renderFIXList() string {
	sshFindings := make([]fixFinding, 0, len(fixFindings))
	gitFindings := make([]fixFinding, 0, len(fixFindings))
	for _, f := range fixFindings {
		if f.section == "SSH" {
			sshFindings = append(sshFindings, f)
		} else {
			gitFindings = append(gitFindings, f)
		}
	}

	lines := []string{styleFIXHeading.Render("SSH")}
	for _, f := range sshFindings {
		lines = append(lines, fixFindingLine(f))
	}
	lines = append(lines, "", styleFIXHeading.Render("Git"))
	for _, f := range gitFindings {
		lines = append(lines, fixFindingLine(f))
	}
	lines = append(lines,
		"",
		styleFIXDim.Render("v preview fix ("+fixTarget.title+")   e nothing-to-fix example"),
		styleFIXDim.Render(fixBatchFixNote),
	)
	return fixBody("Fixer", sigFIXList, lines...)
}

func renderFIXFixPreview() string {
	lines := []string{
		styleFIXWarning.Render("! This fix REWRITES a directive already present in " + fixTargetFile + " -- it is not a new addition."),
		"",
		styleFIXDim.Render("Diff -- " + fixTargetFile + ":"),
	}
	lines = append(lines, fixFixPreviewLines...)
	lines = append(lines,
		"",
		"Only the highlighted line changes -- the rest of the Host block is untouched.",
	)
	return fixBody("Preview fix -- "+fixTarget.title, sigFIXFixPreview, lines...)
}

func renderFIXConfirmDestructive() string {
	return fixBody("Confirm: "+fixTarget.title, sigFIXConfirmDestructive,
		styleFIXError.Render("✗ "+fixConfirmDestructiveNote),
		"File: "+fixTargetFile,
		"Host block: "+fixTargetHost,
		"Directive rewritten: IdentitiesOnly no -> yes",
		"",
		"Default-focused: No, cancel. Destructive actions never default to yes (§5) --",
		"a backup is taken before anything is written.",
	)
}

func renderFIXBackupNotice() string {
	return fixBody("Backup created", sigFIXBackupNotice,
		styleFIXSuccess.Render("✓ "+fixTargetFile+" backup: "+fixBackupPath),
		"",
		styleFIXDim.Render("A full copy of "+fixTargetFile+" was saved BEFORE any change is applied --"),
		styleFIXDim.Render("this backup path is the undo story."),
	)
}

func renderFIXResultApplied() string {
	return fixBody("✓ Fix applied", sigFIXResultApplied,
		styleFIXSuccess.Render("✓ "+fixResultMessage),
		"Only the rewritten directive changed -- the rest of "+fixTargetFile+" was preserved verbatim.",
		"",
		styleFIXDim.Render("To restore by hand, the backup is at "+fixBackupPath+"."),
	)
}

func renderFIXNothingToFix() string {
	return fixBody("Fixer", sigFIXNothingToFix,
		styleFIXHeading.Render("SSH"),
		styleFIXSuccess.Render("✓ "+fixNothingToFixSSH),
		"",
		styleFIXHeading.Render("Git"),
		styleFIXSuccess.Render("✓ "+fixNothingToFixGit),
	)
}

// fixSignatureByScreen is a lookup table screen-ID -> signature,
// mirroring manifest.json -- used by surface_fixer_test.go.
var fixSignatureByScreen = map[string]string{
	"fixer-list":          sigFIXList,
	"fix-preview":         sigFIXFixPreview,
	"confirm-destructive": sigFIXConfirmDestructive,
	"backup-notice":       sigFIXBackupNotice,
	"result-applied":      sigFIXResultApplied,
	"nothing-to-fix":      sigFIXNothingToFix,
}
