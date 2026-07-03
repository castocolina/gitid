package dummytui

import (
	lipgloss "charm.land/lipgloss/v2"
)

// surface_identitymanager.go registers the identity-manager surface
// (02-UX-DIRECTION.md §4(3), Phase 5's Identity Manager) as the app's HOME
// view — number key `1` (review HIGH-2): it REPLACES the 02-02 `data.go`
// placeholder that currently owns key `1` via RegisterOrReplace, so there is
// NO edit to data.go and NO duplicate-activation-key conflict. This file
// alone wires the replacement.
//
// The eight screens below mirror, byte-for-byte on labels/copy/defaults,
// the /mui mockup built in Task 1
// (.planning/design/mockup-src/src/routes/identity-manager/*.route.tsx) and
// the literal recipe copy in src/data/recipeFixtures.ts's
// identityManagerRows section — every recipe-critical value (the 8 MGR-02
// state labels, the delete-choice copy, the backup paths) is kept as a
// byte-visible Go string constant here, not derived, so it stays a static,
// diff-able contract (matching surface_createflow.go/surface_gitscreen.go's
// own precedent). NO backend import — only lipgloss (DLV-05 no-backend
// ALLOWLIST).
//
// Intra-surface ScreenDef.Keys allocate `a` (→ action-menu), `c` (→
// clone-name-prompt), and `d` (→ delete-choice) from the 02-UX-DIRECTION.md
// §2 key-allocation table (the single authority) — never `n`/`g`, which are
// create-flow's and git-screen's own LaunchKeys (registry.go's
// registration-time collision guard rejects any clash loudly). `v`
// (view detail), `e` (empty-state demo toggle), `w`/`x`/`y` (advance the
// clone/delete/confirm sub-flows) are additional intra-surface keys scoped
// to THIS surface only — the same "extra letters beyond the central table"
// precedent surface_createflow.go/surface_gitscreen.go already established
// (their own f/m/r/w/y/z, b/c/f/r/m/t/a/w/x/y/z letters never appear in the
// central table either).
//
// Five screens (action-menu, clone-name-prompt, delete-choice,
// confirm-destructive, backup-notice) are visually MODAL: rendered via
// placeOverlay (overlay.go) compositing a bordered modal box over a dimmed
// rendering of the identity list — the SAME compositing primitive
// model.go's live navigation uses for keyless modal surfaces, called
// directly here (rather than through the modalStack mechanism, which is
// reserved for cross-surface LaunchKey launches) because these are
// intra-surface screens of identity-manager itself, not a separate keyless
// surface.
//
// Each screen's Render also embeds its manifest.json "signature" — a
// screen-specific unique marker distinct from the "<surface>/<screen>"
// breadcrumb — so design_capture_test.go's TUI subtest and the PTY dummy-nav
// e2e can both assert a capture landed on the RIGHT screen, never a
// same-shaped-but-wrong-state false positive (review HIGH-3c, T-02-FP).

// imSurfaceTitle is the SurfaceDef.Title shown in the header.
const imSurfaceTitle = "Identities"

// imRow mirrors recipeFixtures.ts's IdentityManagerRow shape — one fixture
// identity per MGR-02 8-label state (internal/identity/state.go's locked
// vocabulary), so list-populated demonstrates every label at once.
type imRow struct {
	name, state, sshHost, keyPath, gitFragmentPath, note string
}

// imRows is the Go mirror of recipeFixtures.ts's identityManagerRows —
// byte-identical names/states/notes, not derived (a static, diff-able
// contract). The `personal` row reuses the SAME alias/paths
// surface_createflow.go's cf* and surface_gitscreen.go's gs* constants use,
// so "personal" stays canonical across all three surfaces.
var imRows = []imRow{
	{name: "personal", state: "complete", sshHost: "personal.github.com", keyPath: "~/.ssh/id_ed25519_personal", gitFragmentPath: "~/.gitconfig.d/personal", note: "SSH Host block and Git fragment both present."},
	{name: "work", state: "incomplete", sshHost: "work.github.com", keyPath: "~/.ssh/id_ed25519_work", note: "SSH Host block present; no Git identity configured for this alias."},
	{name: "opensource", state: "git-only", gitFragmentPath: "~/.gitconfig.d/opensource", note: "Git identity relies on the global SSH config; no own Host block."},
	{name: "archived", state: "key-unused", keyPath: "~/.ssh/id_ed25519_archived", note: "Key file exists on disk but no identity references it."},
	{name: "staging", state: "key-used-ssh-only", sshHost: "staging.github.com", keyPath: "~/.ssh/id_ed25519_staging", note: "Key referenced by a Host block; not wired for Git commit signing."},
	{name: "clientA", state: "key-used-both", sshHost: "clienta.github.com", keyPath: "~/.ssh/id_ed25519_clientA", gitFragmentPath: "~/.gitconfig.d/clientA", note: "Key wired for both SSH auth and Git commit signing."},
	{name: "clientB", state: "key-missing", sshHost: "clientb.github.com", keyPath: "~/.ssh/id_ed25519_clientB", note: "Host block references a key file that is absent from disk."},
	{name: "legacy", state: "fragment-path-missing", sshHost: "legacy.github.com", gitFragmentPath: "~/.gitconfig.d/legacy", note: "includeIf points at a Git fragment file that does not exist."},
}

// imActionTarget/imDetailTarget mirror recipeFixtures.ts's
// identityManagerActionTarget/identityManagerDetailTarget: action-menu,
// clone-name-prompt, delete-choice, and confirm-destructive all target the
// fully-populated `personal` row; detail-ssh-first deliberately targets the
// SSH-only `work` row to prove MGR-03/MGR-07 (never fabricate Git
// attributes for an SSH-only identity).
var (
	imActionTarget = imRows[0] // "personal"
	imDetailTarget = imRows[1] // "work"
)

// imGlyphByState pairs each MGR-02 label with its color-semantics glyph
// (02-UX-DIRECTION.md §2: healthy=✓, needs-action/advisory=!,
// error/destructive/missing=✗) — always rendered together with the state's
// own WORD (never color alone, the NO_COLOR-legibility requirement).
var imGlyphByState = map[string]string{
	"complete":              "✓",
	"incomplete":            "!",
	"git-only":              "!",
	"key-unused":            "!",
	"key-used-ssh-only":     "✓",
	"key-used-both":         "✓",
	"key-missing":           "✗",
	"fragment-path-missing": "✗",
}

// imToneByState pairs each MGR-02 label with its semantic style, mirroring
// imGlyphByState's tone. References styleIM* (declared further below in
// this file) — safe because Go resolves package-level var initialization
// order by dependency, not by textual position.
var imToneByState = map[string]lipgloss.Style{
	"complete":              styleIMSuccess,
	"incomplete":            styleIMWarning,
	"git-only":              styleIMWarning,
	"key-unused":            styleIMWarning,
	"key-used-ssh-only":     styleIMSuccess,
	"key-used-both":         styleIMSuccess,
	"key-missing":           styleIMError,
	"fragment-path-missing": styleIMError,
}

// MGR-04/MGR-06 literal copy — byte-identical to recipeFixtures.ts's
// identityManagerCloneSuggestedName/identityManagerDeleteChoices.
const (
	imCloneSuggestedName     = "personal-clone"
	imDeleteChoiceGitOnly    = "Delete Git identity only"
	imDeleteChoiceEverything = "Delete everything (SSH + Git + key)"

	// §5 beat-3 timestamped backup paths — the SAME timestamp convention
	// surface_createflow.go's cfBackupPath / surface_gitscreen.go's
	// gsGitconfigBackupPath use (declared independently here, not
	// cross-file-referenced, to keep each surface file self-contained per
	// review MEDIUM-10's fan-out isolation).
	imSSHConfigBackupPath = "~/.ssh/config.backup.2026-07-03T03-59-12Z"
	imGitconfigBackupPath = "~/.gitconfig.backup.2026-07-03T03-59-12Z"
)

// Screen-specific signatures — MUST stay byte-identical to
// .planning/design/identity-manager/manifest.json's "signature" field per
// screen (review HIGH-3c: a screen-specific marker, never a generic reused
// string).
const (
	sigIMListPopulated      = "SIG-IM-LIST-POPULATED-8-LABEL"
	sigIMListEmpty          = "SIG-IM-LIST-EMPTY-FIRST-RUN"
	sigIMDetailSSHFirst     = "SIG-IM-DETAIL-SSH-FIRST-NO-GIT-ATTRS"
	sigIMActionMenu         = "SIG-IM-ACTION-MENU"
	sigIMCloneNamePrompt    = "SIG-IM-CLONE-NAME-PROMPT-DISTINCT-NAME"
	sigIMDeleteChoice       = "SIG-IM-DELETE-CHOICE-SAFE-DEFAULT"
	sigIMConfirmDestructive = "SIG-IM-CONFIRM-DESTRUCTIVE-STRONGEST-CONFIRM"
	sigIMBackupNotice       = "SIG-IM-BACKUP-NOTICE"
)

// Local styles (D-02: no backend imports, so no dependency on
// tui/styles.go — a small self-contained palette mirroring
// 02-UX-DIRECTION.md §2's color semantics table). Mirrors
// surface_createflow.go's styleCF* / surface_gitscreen.go's styleGS* sets,
// kept package-local under an im* prefix so this file has no cross-surface-
// file dependency.
var (
	styleIMHeading = lipgloss.NewStyle().Bold(true)
	styleIMDim     = lipgloss.NewStyle().Faint(true)
	styleIMSuccess = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	styleIMWarning = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)
	styleIMError   = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	// styleIMInfo is the LOCKED severity-glyph contract's info tone (cyan
	// "~", pinned by surface_health.go's styleHLTHInfo / surface_fixer.go's
	// styleFIXInfo: warning=! yellow, error/critical=✗ red, info=~ cyan) —
	// used here (not styleIMWarning) for detail-ssh-first's SSH-only note,
	// which is informational (MGR-03/MGR-07: SSH-only is an expected,
	// healthy state for some identities, not something needing action).
	styleIMInfo  = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	styleIMModal = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Padding(0, 1)
)

func init() {
	RegisterOrReplace(SurfaceDef{
		ID:            "identity-manager",
		Title:         imSurfaceTitle,
		ActivationKey: "1",
		Screens: []ScreenDef{
			{ID: "list-populated", Keys: map[string]string{"a": "action-menu", "v": "detail-ssh-first", "e": "list-empty"}, Render: renderIMListPopulated},
			{ID: "list-empty", Render: renderIMListEmpty},
			{ID: "detail-ssh-first", Keys: map[string]string{"a": "action-menu"}, Render: renderIMDetailSSHFirst},
			{ID: "action-menu", Keys: map[string]string{"c": "clone-name-prompt", "d": "delete-choice"}, Render: renderIMActionMenu},
			{ID: "clone-name-prompt", Keys: map[string]string{"w": "backup-notice"}, Render: renderIMCloneNamePrompt},
			{ID: "delete-choice", Keys: map[string]string{"x": "confirm-destructive"}, Render: renderIMDeleteChoice},
			{ID: "confirm-destructive", Keys: map[string]string{"y": "backup-notice"}, Render: renderIMConfirmDestructive},
			{ID: "backup-notice", Render: renderIMBackupNotice},
		},
	})
}

// imBody joins the heading, body lines, and the trailing signature marker
// into one screen body string — every render func below funnels through
// this so the signature is always present, in the same place,
// deterministically. Mirrors cfBody/gsBody.
func imBody(heading, sig string, lines ...string) string {
	all := make([]string, 0, len(lines)+2)
	all = append(all, styleIMHeading.Render(heading))
	all = append(all, lines...)
	all = append(all, "", styleIMDim.Render("["+sig+"]"))
	return lipgloss.JoinVertical(lipgloss.Left, all...)
}

// imRenderRows renders one line per imRows entry: a leading `>` marker on
// the highlighted row, the glyph+WORD state pair (styled, NO_COLOR-legible
// because the word is always present), the identity name, and its note.
func imRenderRows(highlight string) []string {
	lines := make([]string, 0, len(imRows))
	for _, r := range imRows {
		marker := "  "
		if r.name == highlight {
			marker = "> "
		}
		state := imToneByState[r.state].Render(imGlyphByState[r.state] + " " + r.state)
		lines = append(lines, marker+state+"  "+r.name+" — "+r.note)
	}
	return lines
}

// imListBody renders the bare identity list (heading + rows, no
// signature/trailing chrome) — reused as the DIMMED background every
// placeOverlay-composited modal screen below dims and composites over.
func imListBody() string {
	lines := append([]string{styleIMHeading.Render("Identities")}, imRenderRows(imActionTarget.name)...)
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// imOverlay composites a bordered modal box (title + sig + lines) over a
// DIMMED rendering of the identity list via placeOverlay (overlay.go) —
// the dummy's SAME modal-compositing primitive model.go's live navigation
// uses for cross-surface keyless-modal launches, called directly here
// because these are intra-surface screens of identity-manager itself.
//
// Two independent fixes for review HIGH-1 / HI-01:
//
//  1. Centers/bounds against model.go's currentViewport — NOT the fixed
//     defaultWidth/defaultHeight constants directly — so a LIVE terminal
//     narrower/shorter than the 100x30 default (e.g. the documented 80x24
//     minimum) never has its modal centered against the wrong, larger
//     canvas and shifted past the real right edge. currentViewport
//     defaults to (defaultWidth, defaultHeight) until the first
//     tea.WindowSizeMsg, so every STATIC capture caller (RenderScreen,
//     screenshot-tui-mockups, design_capture_test.go, manifest_test.go —
//     none of which ever sends a WindowSizeMsg) keeps today's exact
//     deterministic 100x30 capture geometry (D-04) unchanged.
//  2. Applies styleIMModal.Width(mw) at render time — mirroring the REAL
//     product's own StyleModal.Width(mw).Render(...) convention
//     (tui/model.go, tui/confirm.go, tui/addrepo.go, etc.) — so lipgloss
//     actually WRAPS content to the clamped modalWidth budget instead of
//     silently auto-sizing the border box to whichever content line
//     happens to be longest. Without this, several of this surface's own
//     hand-authored lines (e.g. confirm-destructive's "Default-focused..."
//     sentence) are wider than modalWidth's 72-column cap on ANY terminal
//     size, so correcting the origin/centering alone (fix 1) is
//     insufficient — the box itself must also be constrained to wrap.
func imOverlay(title, sig string, lines ...string) string {
	mw := modalWidth(currentViewport.w)
	modal := styleIMModal.Width(mw).Render(imBody(title, sig, lines...))
	bg := padToHeight(styleIMDim.Render(imListBody()), currentViewport.h)

	const verticalMargin = 4
	available := currentViewport.h - verticalMargin
	if available < 1 {
		available = 1
	}
	bounded := boundModalToViewport(modal, available, 0)

	mh := lipgloss.Height(bounded)
	x, y := modalOrigin(currentViewport.w, currentViewport.h, mw, mh)
	return placeOverlay(x, y, bounded, bg)
}

func renderIMListPopulated() string {
	lines := imRenderRows(imActionTarget.name)
	lines = append(lines,
		"",
		styleIMDim.Render("Preview — "+imActionTarget.name+":"),
		"  SSH Host: "+imActionTarget.sshHost,
		"  Key: "+imActionTarget.keyPath,
		"  Git fragment: "+imActionTarget.gitFragmentPath,
	)
	return imBody("Identities", sigIMListPopulated, lines...)
}

func renderIMListEmpty() string {
	return imBody("Identities", sigIMListEmpty,
		"No identities yet.",
		"",
		"gitid manages ~/.ssh/config and ~/.gitconfig per identity — nothing has been",
		"configured on this machine yet.",
		"",
		"Press n to create your first identity (SSH connection + Git configuration, end to end).",
	)
}

func renderIMDetailSSHFirst() string {
	t := imDetailTarget
	return imBody("Identity detail — "+t.name, sigIMDetailSSHFirst,
		"1. SSH (shown first)",
		"  Host: "+t.sshHost,
		"  IdentityFile: "+t.keyPath,
		"  IdentitiesOnly: yes",
		"",
		"2. Git",
		styleIMInfo.Render("~ No Git identity configured for this alias — SSH-only."),
		"  gitid never renders fabricated Git attributes here (MGR-03/MGR-07).",
		"",
		"Per-identity health (MGR-07): "+imToneByState[t.state].Render(imGlyphByState[t.state]+" "+t.state),
		"  "+t.note,
	)
}

func renderIMActionMenu() string {
	t := imActionTarget
	return imOverlay("Action menu — "+t.name, sigIMActionMenu,
		"View SSH-first detail",
		"Clone (c) — create a new identity from this one, under a distinct name (MGR-04).",
		"Generate new key — rotate this identity's key (MGR-05).",
		"Delete (d) — choose Git-identity-only, or delete everything (MGR-06).",
	)
}

func renderIMCloneNamePrompt() string {
	s := imActionTarget
	return imOverlay("Clone identity", sigIMCloneNamePrompt,
		"Source identity: "+s.name,
		"New identity name: "+imCloneSuggestedName+" (must differ from the source name — MGR-04)",
		"",
		styleIMDim.Render("Cloning copies the SSH Host block and Git fragment shape under the new name — the key"),
		styleIMDim.Render("material itself is not copied; a new key is generated for the clone."),
	)
}

func renderIMDeleteChoice() string {
	t := imActionTarget
	return imOverlay("Delete "+t.name, sigIMDeleteChoice,
		styleIMSuccess.Render("> "+imDeleteChoiceGitOnly+" ✓ default"),
		"  Removes the Git includeIf block and fragment; the SSH Host block and key are kept.",
		"",
		styleIMError.Render("  "+imDeleteChoiceEverything),
		"  Irreversible — never default-focused. Continuing requires the strongest confirm (§5).",
	)
}

func renderIMConfirmDestructive() string {
	t := imActionTarget
	return imOverlay("Confirm: "+imDeleteChoiceEverything, sigIMConfirmDestructive,
		styleIMError.Render("✗ This action is irreversible."),
		"Removes for "+t.name+":",
		"  - SSH Host block ("+t.sshHost+")",
		"  - Git configuration ("+t.gitFragmentPath+")",
		"  - Key file ("+t.keyPath+")",
		"",
		"Default-focused: No, cancel. Type the identity name to confirm — destructive actions",
		"never default to yes (§5).",
	)
}

func renderIMBackupNotice() string {
	return imOverlay("Backups created", sigIMBackupNotice,
		styleIMSuccess.Render("✓ ~/.ssh/config backup: "+imSSHConfigBackupPath),
		styleIMSuccess.Render("✓ ~/.gitconfig backup: "+imGitconfigBackupPath),
		"",
		styleIMDim.Render("A full copy of each previous file was saved before any change was applied —"),
		styleIMDim.Render("these backup paths are the undo story."),
	)
}

// imSignatureByScreen is a lookup table screen-ID -> signature, mirroring
// manifest.json — used by surface_identitymanager_test.go.
var imSignatureByScreen = map[string]string{
	"list-populated":      sigIMListPopulated,
	"list-empty":          sigIMListEmpty,
	"detail-ssh-first":    sigIMDetailSSHFirst,
	"action-menu":         sigIMActionMenu,
	"clone-name-prompt":   sigIMCloneNamePrompt,
	"delete-choice":       sigIMDeleteChoice,
	"confirm-destructive": sigIMConfirmDestructive,
	"backup-notice":       sigIMBackupNotice,
}
