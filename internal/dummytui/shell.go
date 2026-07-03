package dummytui

import (
	"sort"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
)

// styleShellTitle renders the "gitid" app name in the header (D-02: no
// backend imports, so no dependency on tui/styles.go — a small local style).
var styleShellTitle = lipgloss.NewStyle().Bold(true)

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
// "<surface>/<screen>" breadcrumb screen-ID (review HIGH-3b).
func renderShellHeader(surfaceID, screenID string) string {
	appName := styleShellTitle.Render("gitid")
	breadcrumb := surfaceID + "/" + screenID
	return appName + "  " + breadcrumb
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
		hints = append(hints, k+" "+scr.Keys[k])
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
