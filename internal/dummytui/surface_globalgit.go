package dummytui

import (
	lipgloss "charm.land/lipgloss/v2"
)

// surface_globalgit.go registers the global-git surface (02-UX-DIRECTION.md
// §4.5, Phase 7's Global Git options screen) as a PRIMARY view — number key
// `3` (review HIGH-2): it REPLACES the 02-02 `data.go` placeholder that
// currently owns key `3` via RegisterOrReplace, so there is NO edit to
// data.go and NO duplicate-activation-key conflict. This file alone wires
// the replacement.
//
// The six screens below mirror, byte-for-byte on labels/copy/defaults, the
// /mui mockup built in Task 1
// (.planning/design/mockup-src/src/routes/global-git/*.route.tsx) and the
// literal recipe copy in src/data/recipeFixtures.ts's globalGit* exports —
// every recipe-critical value (the GGIT-01 11-option set, the managed-block
// text, the backup path) is kept as a byte-visible Go string constant here,
// not derived, so it stays a static, diff-able contract (matching
// surface_createflow.go/surface_gitscreen.go/surface_identitymanager.go/
// surface_globalssh.go's own precedent). NO backend import — only lipgloss
// (DLV-05 no-backend ALLOWLIST).
//
// Intra-surface ScreenDef.Keys allocate a linear ceremony chain `v`
// (→ option-detail), `f` (→ fix-preview), `w` (→ confirm-write), `y`
// (→ backup-notice), `z` (→ result-applied) — the SAME letters
// global-ssh's own chain uses (never a collision: each key is scoped to the
// surface that is currently active, and only one primary surface is active
// at a time). None of these collide with `n`/`g`, the only two globally
// reserved LaunchKeys in the 02-UX-DIRECTION.md §2 key-allocation table
// (the registry.go registration-time collision guard rejects any clash
// loudly).
//
// Highest-risk affordance (§4.5, §5): writes must preserve content OUTSIDE
// the managed block verbatim — confirm-write renders the
// `# BEGIN/END gitid managed:` sentinels. §5 also applies the "advisory,
// never blocking" rule to global-git (the SAME sentence that governs
// global-ssh: "Advisory <> blocking on the TWO Global-options surfaces"),
// so ggitAdvisoryNote/the yellow `!` glyph reuse global-ssh's own visual
// language (§2 "one color semantics table, applied everywhere").
//
// Each screen's Render also embeds its manifest.json "signature" — a
// screen-specific unique marker distinct from the "<surface>/<screen>"
// breadcrumb — so design_capture_test.go's TUI subtest and the PTY dummy-nav
// e2e can both assert a capture landed on the RIGHT screen, never a
// same-shaped-but-wrong-state false positive (review HIGH-3c, T-02-FP).

// ggitOption mirrors recipeFixtures.ts's GlobalGitOption shape — one entry
// per GGIT-01 baseline/recipe-default option.
type ggitOption struct {
	key, current, recommended, oneLiner string
	needsAction                         bool
	highlight                           bool // main-vs-master (GGIT-01's own dedicated highlight)
}

// ggitOptions is the Go mirror of recipeFixtures.ts's globalGitOptions —
// byte-identical keys/values/one-liners, not derived (a static, diff-able
// contract). Order matches 02-UX-DIRECTION.md §4.5's verbatim list.
var ggitOptions = []ggitOption{
	{key: "init.defaultBranch", current: "not set (git's built-in default: master)", recommended: "main", needsAction: true, highlight: true, oneLiner: "Distros still default new repos to \"master\"; main matches the modern GitHub/GitLab default without renaming existing repos."},
	{key: "core.ignorecase", current: "not set (OS-dependent: true on macOS/Windows, false on Linux)", recommended: "false", needsAction: true, oneLiner: "Keeps file-name case always significant, so a case-only rename is never silently ignored on a case-insensitive filesystem."},
	{key: "core.autocrlf / core.eol", current: "not set (line-ending handling varies by OS)", recommended: "input / lf", needsAction: true, oneLiner: "Normalizes line endings to LF in the repository and on checkout, avoiding CRLF diff noise across contributors on different platforms."},
	{key: "user.email (global)", current: "whatever `git config --global user.email` already holds, if anything", recommended: "left alone -- not written here", needsAction: false, oneLiner: "gitid never writes a global [user] section -- each identity's commits come from its own includeIf fragment (recipes/gitconfig.recipe); shown here for awareness only."},
	{key: "push.autoSetupRemote", current: "not set (git default: false)", recommended: "true", needsAction: true, oneLiner: "Lets `git push` on a new branch set its upstream automatically, instead of requiring --set-upstream every time."},
	{key: "pull.rebase", current: "not set (git default: false -- merge)", recommended: "true", needsAction: true, oneLiner: "Replays local commits on top of the fetched branch instead of creating a merge commit on every pull."},
	{key: "fetch.prune", current: "not set (git default: false)", recommended: "true", needsAction: true, oneLiner: "Removes local references to remote branches that were deleted upstream, every fetch."},
	{key: "alias (8 shortcuts)", current: "not set", recommended: "st, co, br, ci, df, lg, unstage, last", needsAction: true, oneLiner: "Short, common-workflow aliases (status, checkout, branch, commit, diff, a graph log, unstage, last commit)."},
	{key: "color (ui/branch/diff/status)", current: "not set (ui defaults to auto in modern git; the rest vary)", recommended: "auto for all four", needsAction: true, oneLiner: "Colorizes status, branch, diff, and general UI output consistently, even where a specific subcommand's own default might differ."},
	{key: "merge.conflictstyle", current: "not set (git default: merge)", recommended: "diff3", needsAction: true, oneLiner: "Shows the common ancestor alongside both sides of a conflict, making it easier to tell what each side actually changed."},
	{key: "diff.colorMoved", current: "not set", recommended: "zebra", needsAction: true, oneLiner: "Highlights moved blocks of code distinctly from genuine additions/deletions in colorized diffs, striping each moved block."},
}

// ggitDetailTarget mirrors recipeFixtures.ts's globalGitDetailTarget —
// option-detail's single target (init.defaultBranch, the option carrying
// the main-vs-master highlight).
var ggitDetailTarget = ggitOptions[0] // init.defaultBranch

// GGIT-01 contractual (verbatim, §3) explanation copy — byte-identical to
// recipeFixtures.ts's globalGitDetailExplanation.
const ggitDetailExplanation = `Until Git 2.28 (July 2020), every new repository's default branch was named "master" -- a name inherited from Git's early conventions. GitHub, GitLab, and Bitbucket now all default new repositories to "main" instead, and many teams have followed suit for their own local defaults.

Setting init.defaultBranch = main only affects repositories created AFTER this is set -- it never renames an existing "master" branch in a repository you already have. If you clone or work in a repository whose default branch is still "master" (many older projects have not renamed it), that repository's branch is completely unaffected; this setting only decides what "git init" names the FIRST branch of a brand-new repository.

This is a naming convention, not a security or correctness fix -- it is included here because it is one of the most visible defaults a new gitid user will notice, and stating it explicitly (rather than relying on git's own compiled-in default, or a value some other tool set) keeps the choice intentional and self-documenting.`

// ggitAdvisoryNote -- byte-identical to recipeFixtures.ts's globalGitAdvisoryNote.
const ggitAdvisoryNote = "Recommended, not required -- you can leave any option unchanged. This is advisory, never a compliance gate."

const (
	ggitTargetFile = "~/.gitconfig"

	ggitSentinelBegin = "# BEGIN gitid managed: global-git"
	ggitSentinelEnd   = "# END gitid managed: global-git"

	ggitBackupPath = "~/.gitconfig.backup.2026-07-03T03-59-12Z"

	ggitResultMessage = "10 of 10 baseline options applied to ~/.gitconfig. Global user.email was left alone, as always -- each identity's commits use their own includeIf fragment."

	// ggitFullManagedBlockText mirrors recipeFixtures.ts's
	// globalGitFullManagedBlockText -- the exact managed-block text gitid
	// writes to ~/.gitconfig. Section order matches the shared
	// globalGitDefaultsBlockText fixture's own established order
	// ([init]/[core]/[push]/[pull]/[fetch]/[merge]/[diff]), extended with
	// core.autocrlf/eol, [color], and [alias]. user.email is intentionally
	// ABSENT -- gitid never writes a [user] section here.
	ggitFullManagedBlockText = ggitSentinelBegin + `
[init]
    defaultBranch = main

[core]
    ignorecase = false
    autocrlf = input
    eol = lf

[push]
    autoSetupRemote = true

[pull]
    rebase = true

[fetch]
    prune = true

[color]
    ui = auto
    branch = auto
    diff = auto
    status = auto

[merge]
    conflictstyle = diff3

[diff]
    colorMoved = zebra

[alias]
    st = status
    co = checkout
    br = branch
    ci = commit
    df = diff
    lg = log --graph --pretty=format:'%Cred%h%Creset -%C(yellow)%d%Creset %s %Cgreen(%cr) %C(bold blue)<%an>%Creset' --abbrev-commit
    unstage = reset HEAD --
    last = log -1 HEAD
` + ggitSentinelEnd
)

// ggitFixPreviewLines mirrors recipeFixtures.ts's globalGitFixPreviewLines
// -- the diff-style lines fix-preview shows.
var ggitFixPreviewLines = []string{
	"+ [init]",
	"+     defaultBranch = main",
	"+ [core]",
	"+     ignorecase = false",
	"+     autocrlf = input",
	"+     eol = lf",
	"+ [push]",
	"+     autoSetupRemote = true",
	"+ [pull]",
	"+     rebase = true",
	"+ [fetch]",
	"+     prune = true",
	"+ [color]",
	"+     ui = auto, branch = auto, diff = auto, status = auto",
	"+ [merge]",
	"+     conflictstyle = diff3",
	"+ [diff]",
	"+     colorMoved = zebra",
	"+ [alias]",
	"+     st, co, br, ci, df, lg, unstage, last (8 shortcuts)",
	"  user.email -- left alone; gitid never writes [user] here (each identity uses its own includeIf fragment)",
}

// Screen-specific signatures -- MUST stay byte-identical to
// .planning/design/global-git/manifest.json's "signature" field per screen
// (review HIGH-3c: a screen-specific marker, never a generic reused string).
const (
	sigGGITOptionsList   = "SIG-GGIT-OPTIONS-LIST-11-BASELINE"
	sigGGITOptionDetail  = "SIG-GGIT-OPTION-DETAIL-DEFAULTBRANCH"
	sigGGITFixPreview    = "SIG-GGIT-FIX-PREVIEW-BASELINE-APPLY"
	sigGGITConfirmWrite  = "SIG-GGIT-CONFIRM-WRITE"
	sigGGITBackupNotice  = "SIG-GGIT-BACKUP-NOTICE"
	sigGGITResultApplied = "SIG-GGIT-RESULT-APPLIED"
)

// Local styles (D-02: no backend imports, so no dependency on
// tui/styles.go -- a small self-contained palette mirroring
// 02-UX-DIRECTION.md §2's color semantics table: healthy=green+word,
// warning/advisory=yellow+word, never color alone). Mirrors
// surface_createflow.go's styleCF* / surface_gitscreen.go's styleGS* /
// surface_identitymanager.go's styleIM* / surface_globalssh.go's
// styleGSSH* sets, kept package-local under a ggit* prefix so this file
// has no cross-surface-file dependency.
var (
	styleGGITHeading = lipgloss.NewStyle().Bold(true)
	styleGGITDim     = lipgloss.NewStyle().Faint(true)
	styleGGITSuccess = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	styleGGITWarning = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)
)

func init() {
	RegisterOrReplace(SurfaceDef{
		ID:            "global-git",
		Title:         "Global Git",
		ActivationKey: "3",
		Screens: []ScreenDef{
			{ID: "options-list", Keys: map[string]string{"v": "option-detail"}, Render: renderGGITOptionsList},
			{ID: "option-detail", Keys: map[string]string{"f": "fix-preview"}, Render: renderGGITOptionDetail},
			{ID: "fix-preview", Keys: map[string]string{"w": "confirm-write"}, Render: renderGGITFixPreview},
			{ID: "confirm-write", Keys: map[string]string{"y": "backup-notice"}, Render: renderGGITConfirmWrite},
			{ID: "backup-notice", Keys: map[string]string{"z": "result-applied"}, Render: renderGGITBackupNotice},
			{ID: "result-applied", Render: renderGGITResultApplied},
		},
	})
}

// ggitBody joins the heading, body lines, and the trailing signature marker
// into one screen body string -- every render func below funnels through
// this so the signature is always present, in the same place,
// deterministically. Mirrors cfBody/gsBody/imBody/gsshBody.
func ggitBody(heading, sig string, lines ...string) string {
	all := make([]string, 0, len(lines)+2)
	all = append(all, styleGGITHeading.Render(heading))
	all = append(all, lines...)
	all = append(all, "", styleGGITDim.Render("["+sig+"]"))
	return lipgloss.JoinVertical(lipgloss.Left, all...)
}

// ggitGlyph pairs an option's needsAction state with the color-semantics
// glyph (02-UX-DIRECTION.md §2: healthy=✓, needs-action/advisory=!) --
// always rendered together with a WORD (never color alone).
func ggitGlyph(needsAction bool) string {
	if needsAction {
		return styleGGITWarning.Render("!")
	}
	return styleGGITSuccess.Render("✓")
}

// ggitOptionLine renders ONE option as a SINGLE compact line: glyph + key +
// current -> recommended (or the informational value) + an optional
// "[main vs master]" bracket for the one highlighted option. This is a
// TUI-only compaction (§3 "MAY differ: exact spacing, pixel layout...
// provided the terminal skin keeps them close") -- the live gitid-dummy
// binary renders inside a REAL, fixed 80x24 PTY (e2e/ui_pty_e2e_test.go's
// ptyTermWidth/ptyTermHeight) with NO scroll region, unlike the static
// RenderScreen()->freeze capture path, which has no height limit. The
// per-option one-liner explanation (present on the /mui mockup's
// options-list row) is intentionally NOT repeated here to stay inside that
// budget across all 11 options plus the header/status/keybar chrome -- the
// SAME field set/order/values (key, current, recommended) still match; the
// full contractual explanation is one keystroke away on option-detail.
// Mirrors surface_gitscreen.go's gsFieldsCompactLine*/
// surface_globalssh.go's gsshOptionLine precedent (same viewport
// constraint, same resolution).
func ggitOptionLine(opt ggitOption) string {
	glyph := ggitGlyph(opt.needsAction)
	valueText := opt.current + " -> " + opt.recommended
	if !opt.needsAction {
		valueText = opt.current + " (" + opt.recommended + ")"
	}
	line := glyph + " " + opt.key + "   " + valueText
	if opt.highlight {
		line += "   [main vs master]"
	}
	return line
}

func renderGGITOptionsList() string {
	lines := []string{styleGGITWarning.Render(ggitAdvisoryNote)}
	for _, opt := range ggitOptions {
		lines = append(lines, ggitOptionLine(opt))
	}
	lines = append(lines,
		styleGGITDim.Render("v full explanation ("+ggitDetailTarget.key+")   f preview fix"),
	)
	return ggitBody("Global Git options", sigGGITOptionsList, lines...)
}

func renderGGITOptionDetail() string {
	t := ggitDetailTarget
	return ggitBody(t.key, sigGGITOptionDetail,
		"current: "+t.current+"   recommended: "+t.recommended+"   [main vs master]",
		"",
		ggitDetailExplanation,
		"",
		styleGGITWarning.Render("! "+ggitAdvisoryNote),
	)
}

func renderGGITFixPreview() string {
	lines := []string{
		styleGGITWarning.Render("! Applying 10 of 10 baseline options to the managed block in " + ggitTargetFile + "."),
		styleGGITWarning.Render("! user.email is intentionally absent -- gitid never writes a global [user] section."),
		"",
		styleGGITDim.Render("Diff -- managed block in " + ggitTargetFile + ":"),
	}
	lines = append(lines, ggitFixPreviewLines...)
	lines = append(lines,
		"",
		"gitid only owns the block between its sentinels -- everything else in "+ggitTargetFile+" is preserved verbatim.",
	)
	return ggitBody("Preview fix", sigGGITFixPreview, lines...)
}

func renderGGITConfirmWrite() string {
	return ggitBody("Confirm write", sigGGITConfirmWrite,
		styleGGITWarning.Render("! Nothing has changed yet — review below, then confirm."),
		styleGGITWarning.Render("! user.email is intentionally absent -- gitid never writes a global [user] section here."),
		"",
		ggitTargetFile+" (append, sentinels visible):",
		ggitFullManagedBlockText,
	)
}

func renderGGITBackupNotice() string {
	return ggitBody("Backup created", sigGGITBackupNotice,
		styleGGITSuccess.Render("✓ "+ggitTargetFile+" backup: "+ggitBackupPath),
		"",
		styleGGITDim.Render("A full copy of the previous file was saved before any change was applied —"),
		styleGGITDim.Render("this backup path is the undo story."),
	)
}

func renderGGITResultApplied() string {
	return ggitBody("✓ Options applied", sigGGITResultApplied,
		styleGGITSuccess.Render("✓ "+ggitResultMessage),
		"Written to the managed block in "+ggitTargetFile+".",
		"Everything outside the sentinels -- including any hand-written [user]/[includeIf]/[url] sections -- was preserved verbatim.",
		"",
		styleGGITDim.Render("To restore by hand, the backup is at "+ggitBackupPath+"."),
	)
}

// ggitSignatureByScreen is a lookup table screen-ID -> signature, mirroring
// manifest.json -- used by surface_globalgit_test.go.
var ggitSignatureByScreen = map[string]string{
	"options-list":   sigGGITOptionsList,
	"option-detail":  sigGGITOptionDetail,
	"fix-preview":    sigGGITFixPreview,
	"confirm-write":  sigGGITConfirmWrite,
	"backup-notice":  sigGGITBackupNotice,
	"result-applied": sigGGITResultApplied,
}
