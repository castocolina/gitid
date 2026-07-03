package dummytui

import (
	lipgloss "charm.land/lipgloss/v2"
)

// surface_health.go registers the health surface (02-UX-DIRECTION.md §4.6,
// Phase 8's Health check screen) as a PRIMARY view — number key `4` (review
// HIGH-2): it REPLACES the 02-02 `data.go` placeholder that currently owns
// key `4` via RegisterOrReplace, so there is NO edit to data.go and NO
// duplicate-activation-key conflict. This file alone wires the replacement.
//
// The five screens below mirror, byte-for-byte on labels/copy/defaults, the
// /mui mockup built in Task 1
// (.planning/design/mockup-src/src/routes/health/*.route.tsx) and the
// literal recipe copy in src/data/recipeFixtures.ts's health* exports —
// every recipe-critical value (the 5 findings, the severity glyph
// contract, the parse-error snippet) is kept as a byte-visible Go string
// constant here, not derived, so it stays a static, diff-able contract
// (matching surface_createflow.go/surface_gitscreen.go/
// surface_identitymanager.go/surface_globalssh.go/surface_globalgit.go's
// own precedent). NO backend import — only lipgloss (DLV-05 no-backend
// ALLOWLIST).
//
// Intra-surface ScreenDef.Keys allocate `h` (→ health-all-green), `v` (→
// finding-detail), `i` (→ per-identity-health), `x` (→ parse-error) — all
// four reachable in ONE HOP from the entry screen (health-with-findings),
// never `n`/`g` (create-flow's/git-screen's own LaunchKeys, the only two
// globally reserved letters in the 02-UX-DIRECTION.md §2 key-allocation
// table — the registry.go registration-time collision guard rejects any
// clash loudly).
//
// Highest-risk affordance (§4.6, §5): READ-ONLY INTEGRITY — Health
// diagnoses, it never mutates. Unlike every other primary surface built so
// far (global-ssh/global-git's advisory fix chain, identity-manager's
// delete ceremony), health has NO write-ceremony screen at all: no
// confirm-write, no backup-notice, no result-applied. hlthReadOnlyNote is
// rendered on all 5 screens, and surface_health_test.go asserts (LOW-11,
// negative check) that no confirm/backup/apply write-ceremony marker
// string ever appears in any of health's rendered output.
//
// Each screen's Render also embeds its manifest.json "signature" — a
// screen-specific unique marker distinct from the "<surface>/<screen>"
// breadcrumb — so design_capture_test.go's TUI subtest and the PTY dummy-nav
// e2e can both assert a capture landed on the RIGHT screen, never a
// same-shaped-but-wrong-state false positive (review HIGH-3c, T-02-FP).

// hlthSeverity mirrors recipeFixtures.ts's HealthSeverity — the four
// internal/doctor/doctor.go Severity levels (info/warning/error/critical),
// byte-identical lowercase labels.
type hlthSeverity string

const (
	hlthInfo     hlthSeverity = "info"
	hlthWarning  hlthSeverity = "warning"
	hlthError    hlthSeverity = "error"
	hlthCritical hlthSeverity = "critical"
)

// hlthFinding mirrors recipeFixtures.ts's HealthFinding shape — one
// concrete health finding, scoped to either the SSH or Git section.
type hlthFinding struct {
	id, section, family, title, explanation, suggestedFix string
	severity                                              hlthSeverity
}

// hlthFindings is the Go mirror of recipeFixtures.ts's healthFindings —
// byte-identical ids/sections/severities/copy, not derived (a static,
// diff-able contract). Covers HLTH-03 (redundancy: duplicate Host *),
// HLTH-04 (contradictions: IdentitiesOnly no + an explicit IdentityFile;
// an includeIf targeting a missing fragment), and all four severity
// levels at once.
var hlthFindings = []hlthFinding{
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
	{
		id: "git-opensource-no-host-block", section: "Git", severity: hlthInfo, family: "Overlap",
		title:       "opensource has no dedicated SSH Host block",
		explanation: "The \"opensource\" Git identity resolves correctly via its includeIf, but relies entirely on the global SSH config -- there is no gitid-managed Host block scoping which key ssh offers for it. Informational only.",
	},
}

// hlthFindingByID looks up a fixture finding by id — used by the
// finding-detail and per-identity-health render funcs so the underlying
// list stays the single source of truth.
func hlthFindingByID(id string) hlthFinding {
	for _, f := range hlthFindings {
		if f.id == id {
			return f
		}
	}
	panic("dummytui: surface_health.go: no fixture finding with id " + id)
}

// hlthFindingDetailTarget mirrors recipeFixtures.ts's
// healthFindingDetailTarget — finding-detail's deep-dive target, the
// IdentitiesOnly/IdentityFile contradiction (mirrors global-ssh's own
// IdentitiesOnly deep-dive precedent).
var hlthFindingDetailTarget = hlthFindingByID("ssh-identitiesonly-contradiction")

// hlthPerIdentityGitFinding mirrors recipeFixtures.ts's
// healthPerIdentityGitFinding — per-identity-health's Git finding, scoped
// to the "legacy" identity, the SAME finding as
// git-includeif-missing-fragment above (traceable, not re-derived).
var hlthPerIdentityGitFinding = hlthFindingByID("git-includeif-missing-fragment")

// hlthAllGreenSummary mirrors recipeFixtures.ts's healthAllGreenSummary.
const (
	hlthAllGreenSSH = "SSH -- 3 identities, 3 Host blocks, 3 keys checked. All present, all mode 0600, no redundant Host * stanzas, no contradictions."
	hlthAllGreenGit = "Git -- 3 includeIf blocks checked. Every fragment file exists, every allowed_signers email matches its identity's user.email."
)

// hlthPerIdentity* mirror recipeFixtures.ts's healthPerIdentityTarget /
// healthPerIdentitySSHNote — the "legacy" identity (identityManagerRows,
// state fragment-path-missing), reused byte-identically so this surface's
// per-identity slice (HLTH-05) is traceably the SAME data MGR-07's
// Identity Manager row badge derives from.
const (
	hlthPerIdentityName       = "legacy"
	hlthPerIdentityState      = "fragment-path-missing"
	hlthPerIdentityNote       = "includeIf points at a Git fragment file that does not exist."
	hlthPerIdentitySSHNote    = "Host block present (legacy.github.com), IdentityFile present, IdentitiesOnly yes. No SSH findings for this identity."
	hlthPerIdentityMgrHandoff = "This slice feeds the Identity Manager row for " + hlthPerIdentityName + " (MGR-07): " + hlthPerIdentityNote
)

// hlthParseError* mirror recipeFixtures.ts's healthParseErrorTarget —
// HLTH-02's parse-error example: the one condition Health can only
// report, reinforcing read-only integrity concretely.
const (
	hlthParseErrorFile        = "~/.gitconfig.d/work"
	hlthParseErrorRaw         = "error: bad config line 4 in file ~/.gitconfig.d/work"
	hlthParseErrorSnippet     = "line 4:     signingkey = \"~/.ssh/id_ed25519_work.pub"
	hlthParseErrorExplanation = "A signingkey value is missing its closing quote -- git cannot parse this file at all, so no Git identity check can run for \"work\" until it parses again."
)

// hlthReadOnlyNote — byte-identical to recipeFixtures.ts's
// healthReadOnlyNote. Shown on EVERY one of the 5 health screens (review
// LOW-11): the explicit, negatively-checkable read-only statement.
const hlthReadOnlyNote = "Health only diagnoses -- nothing here writes to your files. Open the Fixer (key 5) to change anything shown."

// Screen-specific signatures — MUST stay byte-identical to
// .planning/design/health/manifest.json's "signature" field per screen
// (review HIGH-3c: a screen-specific marker, never a generic reused
// string).
const (
	sigHLTHWithFindings  = "SIG-HLTH-WITH-FINDINGS-SSH-GIT-SPLIT"
	sigHLTHAllGreen      = "SIG-HLTH-ALL-GREEN"
	sigHLTHFindingDetail = "SIG-HLTH-FINDING-DETAIL-IDENTITIESONLY-CONTRADICTION"
	sigHLTHPerIdentity   = "SIG-HLTH-PER-IDENTITY-LEGACY-FRAGMENT-MISSING"
	sigHLTHParseError    = "SIG-HLTH-PARSE-ERROR-GITCONFIG-FRAGMENT"
)

// Local styles (D-02: no backend imports, so no dependency on
// tui/styles.go — a small self-contained palette mirroring
// 02-UX-DIRECTION.md §2's color semantics table, EXTENDED with the
// LOCKED 4-level severity glyph contract this surface's own §4.6 addendum
// pins: warning is ALWAYS yellow `!`, error/critical are ALWAYS red `✗`
// (distinguished by the WORD, never the glyph/color), info is cyan `~`.
// Mirrors surface_createflow.go's styleCF* / surface_globalssh.go's
// styleGSSH* sets, kept package-local under an hlth* prefix so this file
// has no cross-surface-file dependency.
var (
	styleHLTHHeading = lipgloss.NewStyle().Bold(true)
	styleHLTHDim     = lipgloss.NewStyle().Faint(true)
	styleHLTHSuccess = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	styleHLTHWarning = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)
	styleHLTHError   = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	styleHLTHInfo    = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
)

func init() {
	RegisterOrReplace(SurfaceDef{
		ID:            "health",
		Title:         "Health",
		ActivationKey: "4",
		Screens: []ScreenDef{
			{ID: "health-with-findings", Keys: map[string]string{"h": "health-all-green", "v": "finding-detail", "i": "per-identity-health", "x": "parse-error"}, Render: renderHLTHWithFindings},
			{ID: "health-all-green", Render: renderHLTHAllGreen},
			{ID: "finding-detail", Render: renderHLTHFindingDetail},
			{ID: "per-identity-health", Render: renderHLTHPerIdentityHealth},
			{ID: "parse-error", Render: renderHLTHParseError},
		},
	})
}

// hlthBody joins the heading, the read-only banner, the body lines, and
// the trailing signature marker into one screen body string — every
// render func below funnels through this so hlthReadOnlyNote and the
// signature are ALWAYS present, in the same place, deterministically, on
// EVERY health screen (review LOW-11). Mirrors cfBody/gsBody/imBody/
// gsshBody.
func hlthBody(heading, sig string, lines ...string) string {
	all := make([]string, 0, len(lines)+4)
	all = append(all, styleHLTHHeading.Render(heading))
	all = append(all, styleHLTHDim.Render(hlthReadOnlyNote), "")
	all = append(all, lines...)
	all = append(all, "", styleHLTHDim.Render("["+sig+"]"))
	return lipgloss.JoinVertical(lipgloss.Left, all...)
}

// hlthSeverityGlyph pairs a finding's severity with the LOCKED
// glyph+color contract, always rendered together with the severity's own
// WORD (never color alone, the NO_COLOR-legibility requirement).
func hlthSeverityGlyph(sev hlthSeverity) string {
	switch sev {
	case hlthCritical, hlthError:
		return styleHLTHError.Render("✗ " + string(sev))
	case hlthWarning:
		return styleHLTHWarning.Render("! " + string(sev))
	case hlthInfo:
		return styleHLTHInfo.Render("~ " + string(sev))
	default:
		return string(sev)
	}
}

// hlthFindingLine renders ONE finding as a SINGLE compact line: severity
// glyph+word + title + family — the same one-line-per-item TUI
// compaction precedent surface_globalssh.go's gsshOptionLine and
// surface_globalgit.go's compact-line renderers already established for
// the fixed 80x24 live-PTY viewport (no scroll region), so a 5-finding
// list plus the header/status/keybar chrome never overflows.
func hlthFindingLine(f hlthFinding) string {
	return hlthSeverityGlyph(f.severity) + "  " + f.title + "  [" + f.family + "]"
}

func renderHLTHWithFindings() string {
	lines := []string{styleHLTHHeading.Render("SSH")}
	for _, f := range hlthFindings {
		if f.section == "SSH" {
			lines = append(lines, hlthFindingLine(f))
		}
	}
	lines = append(lines, "", styleHLTHHeading.Render("Git"))
	for _, f := range hlthFindings {
		if f.section == "Git" {
			lines = append(lines, hlthFindingLine(f))
		}
	}
	lines = append(lines,
		"",
		styleHLTHDim.Render("h all-green example   v full detail ("+hlthFindingDetailTarget.title+")   i per-identity   x parse-error example"),
	)
	return hlthBody("Health", sigHLTHWithFindings, lines...)
}

func renderHLTHAllGreen() string {
	return hlthBody("Health", sigHLTHAllGreen,
		styleHLTHHeading.Render("SSH"),
		styleHLTHSuccess.Render("✓ "+hlthAllGreenSSH),
		"",
		styleHLTHHeading.Render("Git"),
		styleHLTHSuccess.Render("✓ "+hlthAllGreenGit),
	)
}

func renderHLTHFindingDetail() string {
	f := hlthFindingDetailTarget
	lines := []string{
		hlthSeverityGlyph(f.severity) + "   " + f.section + " / " + f.family,
		"",
		f.explanation,
	}
	if f.suggestedFix != "" {
		lines = append(lines, "", styleHLTHWarning.Render("! "+f.suggestedFix))
	}
	return hlthBody(f.title, sigHLTHFindingDetail, lines...)
}

func renderHLTHPerIdentityHealth() string {
	g := hlthPerIdentityGitFinding
	return hlthBody("Per-identity health -- "+hlthPerIdentityName+" ("+hlthPerIdentityState+")", sigHLTHPerIdentity,
		styleHLTHHeading.Render("SSH"),
		styleHLTHSuccess.Render("✓ "+hlthPerIdentitySSHNote),
		"",
		styleHLTHHeading.Render("Git"),
		hlthSeverityGlyph(g.severity)+"  "+g.title,
		g.explanation,
		"",
		styleHLTHDim.Render(hlthPerIdentityMgrHandoff),
	)
}

func renderHLTHParseError() string {
	return hlthBody("Parse error", sigHLTHParseError,
		styleHLTHDim.Render("Git -- "+hlthParseErrorFile),
		styleHLTHError.Render("✗ "+hlthParseErrorRaw),
		hlthParseErrorSnippet,
		"",
		hlthParseErrorExplanation,
	)
}

// hlthSignatureByScreen is a lookup table screen-ID -> signature,
// mirroring manifest.json — used by surface_health_test.go.
var hlthSignatureByScreen = map[string]string{
	"health-with-findings": sigHLTHWithFindings,
	"health-all-green":     sigHLTHAllGreen,
	"finding-detail":       sigHLTHFindingDetail,
	"per-identity-health":  sigHLTHPerIdentity,
	"parse-error":          sigHLTHParseError,
}
