package dummytui

import (
	"fmt"
	"sort"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
)

// styleShellTitle renders the "gitid" app name in the header (D-02: no
// backend imports, so no dependency on tui/styles.go — a small local style).
var styleShellTitle = lipgloss.NewStyle().Bold(true)

// styleShellChipWarning renders the header's global-health context chip in
// its "needs action" tone — the semantic-color yellow "!" (02-UX-DIRECTION.md
// §2: never color alone, the glyph+word pairing below carries the meaning).
var styleShellChipWarning = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)

// shellHeaderContext* are the static fixture values for the header's global
// context chip (review MED-HIGH A2: 02-UX-DIRECTION.md §2 region 1 requires
// "app name, the current view name, and a global context chip (e.g. identity
// count, global health ✓/!/✗)" — this chip was previously entirely absent
// from the TUI shell). Mirrors the /mui mockup's Header.tsx chip content —
// specifically the value identity-manager/list-populated and
// /detail-ssh-first (the app's actual HOME/entry screens,
// .planning/design/mockup-src/src/routes/identity-manager/list-populated.route.tsx)
// pass as headerContext: {identityCount: identityManagerRows.length (8),
// health: 'warning'}. That is the correct semantic-parity target for a
// SINGLE static package-global TUI header (shown on every surface, not just
// identity-manager) — the smaller ad hoc {1, 'healthy'} / {0, 'healthy'}
// props some OTHER HTML modal routes (action-menu, clone-name-prompt,
// delete-choice, confirm-destructive, backup-notice, list-empty) pass are
// screen-local demo simplifications for those individual story states, not
// the shell's own "global" rollup — the home screen's value is the one
// value a single persistent chip can honestly mirror across every surface.
const (
	shellHeaderIdentityCount = 8
	shellHeaderHealthGlyph   = "!"
	shellHeaderHealthWord    = "needs action"
)

// styleShellDimmed dims the persistent shell render behind an open modal,
// mirroring tui/model.go's StyleDimmed dim-then-composite dispatch shape.
var styleShellDimmed = lipgloss.NewStyle().Faint(true)

// renderShell composes the four-region shell (header/body/status/keybar) for
// the given active surface/screen, matching the MUI mockup's shared shell
// (02-UX-DIRECTION.md section 2; 02-01's shell parity source) and
// tui/model.go's renderPersistentLayout region-composition shape (backend-free
// re-derivation, not a shared implementation). The header carries the
// "<surface>/<screen>" breadcrumb screen-ID. Shared by RenderScreen
// (registry.go, static capture) and model.go's View() (live navigation).
func renderShell(sd SurfaceDef, scr ScreenDef) string {
	header := renderShellHeader(sd.ID, scr.ID)
	body := scr.Render()
	status := renderShellStatus()
	keybar := renderShellKeybar(sd, scr)
	return lipgloss.JoinVertical(lipgloss.Left, header, body, status, keybar)
}

// renderShellHeader renders the 1-row header: app name + the
// "<surface>/<screen>" breadcrumb screen-ID (review HIGH-3b) + the global
// context chip (review A2: identity count + global health glyph/word, per
// 02-UX-DIRECTION.md §2's header region 1 spec — "app name, the current view
// name, and a global context chip"). The chip is a single static fixture
// value shown identically on every surface (see shellHeaderContext* doc
// comment above), mirroring the /mui mockup's Header.tsx chip semantic
// content for the app's actual home screen.
func renderShellHeader(surfaceID, screenID string) string {
	appName := styleShellTitle.Render("gitid")
	breadcrumb := surfaceID + "/" + screenID
	chip := fmt.Sprintf("%d identities · %s", shellHeaderIdentityCount,
		styleShellChipWarning.Render(shellHeaderHealthGlyph+" "+shellHeaderHealthWord))
	return appName + "  " + breadcrumb + "  " + chip
}

// renderShellStatus renders the 1-row status/message line. The dummy has no
// backend, so there is never a real transient message — the region is
// present (shell parity with the MUI mockup and the real tui/) but always
// empty static text.
func renderShellStatus() string {
	return ""
}

// renderShellKeybar renders the always-visible keybar, showing only the keys
// valid in the CURRENT context (02-UX-DIRECTION.md section 2): the active
// screen's intra-surface ScreenDef.Keys transitions, any keyless surfaces
// launchable FROM this surface, and the reserved keys. Sorted by key for
// deterministic output (RenderScreen byte-identical determinism contract).
func renderShellKeybar(sd SurfaceDef, scr ScreenDef) string {
	var hints []string

	intraKeys := make([]string, 0, len(scr.Keys))
	for k := range scr.Keys {
		intraKeys = append(intraKeys, k)
	}
	sort.Strings(intraKeys)
	for _, k := range intraKeys {
		label := scr.Keys[k]
		if override, ok := scr.KeyLabels[k]; ok {
			label = override
		}
		hints = append(hints, k+" "+label)
	}

	for _, other := range Surfaces() {
		if other.ActivationKey == "" && other.LaunchFrom == sd.ID && other.LaunchKey != "" {
			hints = append(hints, other.LaunchKey+" "+other.Title)
		}
	}

	if sd.ActivationKey != "" {
		hints = append(hints, "1-5 switch view")
	}

	hints = append(hints, "Esc back", "q quit", "? help")
	return strings.Join(hints, "  ")
}
