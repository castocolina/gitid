package dummytui

// mouse_test.go pins the spec §7 mouse routing ("every action is also a
// real button"): header tab labels and the health chip switch views, and
// list rows / sub-tab labels are real click targets. Tests locate the
// needle in the RENDERED frame and click that cell, so hit-testing is
// verified against what is actually drawn, not against the zone math.

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// clickAt sends a left MouseClickMsg at frame cell (x, y) through Update.
func clickAt(t *testing.T, a App, x, y int) (App, tea.Cmd) {
	t.Helper()
	model, cmd := a.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	next, ok := model.(App)
	if !ok {
		t.Fatalf("Update returned %T, want App", model)
	}
	return next, cmd
}

// clickCell finds needle in the rendered frame — scanning from frame row
// fromY, restricted to the first maxCols display columns when maxCols > 0 —
// and left-clicks its first cell.
func clickCell(t *testing.T, a App, needle string, maxCols, fromY int) App {
	t.Helper()
	lines := strings.Split(appView(a), "\n")
	for y := fromY; y < len(lines); y++ {
		region := lines[y]
		if maxCols > 0 {
			region = ansi.Truncate(region, maxCols, "")
		}
		idx := strings.Index(region, needle)
		if idx < 0 {
			continue
		}
		x := ansi.StringWidth(region[:idx])
		next, _ := clickAt(t, a, x, y)
		return next
	}
	t.Fatalf("clickCell: %q not found in the rendered frame from row %d", needle, fromY)
	return a
}

func TestMouseHeaderTabLabelsSwitchTabs(t *testing.T) {
	a := NewApp()
	a = clickCell(t, a, "2 Global SSH", 0, 0)
	if a.tab != tabGlobalSSH {
		t.Fatalf("tab = %v after clicking the Global SSH label, want Global SSH", a.tab)
	}
	a = clickCell(t, a, "3 Global Git", 0, 0)
	if a.tab != tabGlobalGit {
		t.Fatalf("tab = %v after clicking the Global Git label, want Global Git", a.tab)
	}
	a = clickCell(t, a, "1 Identities", 0, 0)
	if a.tab != tabIdentities {
		t.Errorf("tab = %v after clicking the Identities label, want Identities", a.tab)
	}
}

func TestMouseHealthChipOpensDoctor(t *testing.T) {
	a := NewApp()
	a = clickCell(t, a, "8 ids", 0, 0)
	if a.tab != tabDoctor {
		t.Errorf("tab = %v after clicking the health chip, want Doctor", a.tab)
	}
}

func TestMouseClickBetweenHeaderTargetsIsInert(t *testing.T) {
	a := NewApp()
	// The gap between the last tab label and the right-aligned chip.
	a, _ = clickAt(t, a, a.width/2+20, 0)
	if a.tab != tabIdentities {
		t.Errorf("tab = %v after clicking header dead space, want Identities", a.tab)
	}
	// Breadcrumb and RESERVED-footer rows are inert (the contextual footer
	// line became clickable in batch 3 — covered by its own tests).
	a, _ = clickAt(t, a, 2, 1)
	a, _ = clickAt(t, a, 2, a.height-1)
	if a.tab != tabIdentities {
		t.Error("chrome rows outside the header must not switch tabs")
	}
}

func TestMouseSidebarRowSelectsIdentity(t *testing.T) {
	a := NewApp()
	a = clickCell(t, a, "opensource", sidebarWidth(a.width), frameBodyTop)
	if got := identModel(t, a).selected; got != "opensource" {
		t.Fatalf("selected = %q after clicking the opensource row, want opensource", got)
	}
	if !strings.Contains(appView(a), "› opensource") {
		t.Error("breadcrumb (and detail) must re-render live for the clicked row")
	}
}

func TestMouseSidebarInertWhileFormPaneOpen(t *testing.T) {
	a := NewApp()
	a = clickCell(t, a, "opensource", sidebarWidth(a.width), frameBodyTop)
	a, _ = press(t, a, "n") // open the create wizard
	a = clickCell(t, a, "personal", sidebarWidth(a.width), frameBodyTop)
	m := identModel(t, a)
	if m.selected != "opensource" {
		t.Errorf("selected = %q, want opensource — the dimmed sidebar must be inert", m.selected)
	}
	if m.pane != paneCreate {
		t.Errorf("pane = %v, want paneCreate — a sidebar click must not close the wizard", m.pane)
	}
}

func TestMouseDoctorFindingRowSelects(t *testing.T) {
	a := doctorApp(t)
	a = clickCell(t, a, "includeIf targets a missing", masterListWidth(a.width), frameBodyTop)
	if got := docModel(t, a).selectedID; got != "git-includeif-missing-fragment" {
		t.Fatalf("selectedID = %q after clicking the finding row, want git-includeif-missing-fragment", got)
	}
	// A group subheader is not a finding — clicking it keeps the selection.
	a = clickCell(t, a, "SSH · archived", masterListWidth(a.width), frameBodyTop)
	if got := docModel(t, a).selectedID; got != "git-includeif-missing-fragment" {
		t.Errorf("selectedID = %q after clicking a group label, want unchanged", got)
	}
}

func TestMouseGlobalSSHSubTabsAndOptionRows(t *testing.T) {
	a, _ := press(t, NewApp(), "2")

	// Sub-tab labels switch sub-tabs (searched from the body, because the
	// breadcrumb row repeats the sub-tab name and must stay inert).
	a = clickCell(t, a, "Storage & preview", 0, frameBodyTop)
	if gssModelOf(t, a).subTab != gssStorage {
		t.Fatal("clicking the Storage & preview label must switch the sub-tab")
	}
	a = clickCell(t, a, "Options", 0, frameBodyTop)
	if gssModelOf(t, a).subTab != gssOptions {
		t.Fatal("clicking the Options label must switch back")
	}

	// Option rows select — the detail pane follows.
	a = clickCell(t, a, "HashKnownHosts", masterListWidth(a.width), frameBodyTop)
	if got := gssModelOf(t, a).detailKey; got != "HashKnownHosts" {
		t.Errorf("detailKey = %q after clicking the HashKnownHosts row, want HashKnownHosts", got)
	}
}

func TestMouseGlobalGitOptionRowSelects(t *testing.T) {
	a, _ := press(t, NewApp(), "3")
	a = clickCell(t, a, "pull.rebase", masterListWidth(a.width), frameBodyTop)
	if got := gitModelOf(t, a).detailKey; got != "pull.rebase" {
		t.Errorf("detailKey = %q after clicking the pull.rebase row, want pull.rebase", got)
	}
}

func TestMouseRightClickAndOverlayClicksAreIgnored(t *testing.T) {
	a := NewApp()
	model, _ := a.Update(tea.MouseClickMsg{X: 10, Y: 0, Button: tea.MouseRight})
	a = model.(App)
	if a.tab != tabIdentities {
		t.Error("right clicks must be ignored")
	}
	a, _ = press(t, a, "?")
	a = clickCell(t, a, "4 Doctor", 0, 0)
	if a.tab != tabIdentities || a.overlay != overlayHelp {
		t.Error("clicks while an overlay is open must be ignored (overlays are keyboard-driven)")
	}
}

// gssModelOf extracts the Global SSH child model.
func gssModelOf(t *testing.T, a App) globalSSHModel {
	t.Helper()
	m, ok := a.screens[tabGlobalSSH].(globalSSHModel)
	if !ok {
		t.Fatalf("screens[1] is %T, want globalSSHModel", a.screens[tabGlobalSSH])
	}
	return m
}

// gitModelOf extracts the Global Git child model.
func gitModelOf(t *testing.T, a App) globalGitModel {
	t.Helper()
	m, ok := a.screens[tabGlobalGit].(globalGitModel)
	if !ok {
		t.Fatalf("screens[2] is %T, want globalGitModel", a.screens[tabGlobalGit])
	}
	return m
}
