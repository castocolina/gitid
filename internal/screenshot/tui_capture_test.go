//go:build screenshot

package screenshot

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
)

// fixtureModel is a trivial Bubble Tea model used ONLY to prove the
// screenshot-tui capture path end-to-end (Task 2, TOOL-05/DLV-03 spike). It
// is NOT product UI — Phase 2's design-approved TUI mockups are what later
// `screenshot-tui` invocations will actually capture. Its View() is read
// directly (no live Update loop, no real PTY — D-01 "teatest-style" capture)
// at the fixed geometry recorded below (D-04).
type fixtureModel struct {
	width, height int
}

func (m fixtureModel) Init() tea.Cmd { return nil }

func (m fixtureModel) Update(tea.Msg) (tea.Model, tea.Cmd) { return m, nil }

func (m fixtureModel) View() tea.View {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42")).Render("gitid screenshot-tui spike")
	body := lipgloss.NewStyle().Foreground(lipgloss.Color("63")).
		Render(fmt.Sprintf("fixed %dx%d fixture golden -- TOOL-05/DLV-03", m.width, m.height))
	return tea.NewView(title + "\n" + body)
}

// Fixed capture parameters (D-04). These must never change without also
// re-recording the golden hash in .planning/design/_spike/GOLDENS.md.
const (
	tuiFixtureWidth  = 100
	tuiFixtureHeight = 30
	tuiFixtureTheme  = "dracula"

	// tuiGoldenSHA256 is the recorded golden hash from
	// .planning/design/_spike/GOLDENS.md. A re-run of TestCaptureTUI on the
	// same OS/font/theme/content must reproduce this exact value.
	tuiGoldenSHA256 = "32c8b8992c84e59e188460c9ee8bb0d9059c9f10a6355057aed63181ebc12c64"
)

// TestCaptureTUI is the runnable entry point `make screenshot-tui` invokes
// (via `go test -tags screenshot -run TestCaptureTUI ./internal/screenshot/...`).
// It captures a trivial fixture View() dump, renders it through the real
// CaptureTUI -> freeze path, writes the PNG under
// .planning/design/_spike/tui/, and asserts the golden hash reproduces on
// re-run (recorded in .planning/design/_spike/GOLDENS.md). Paths are
// relative to this package directory (go test's working directory is always
// the package's source directory, regardless of invocation cwd).
func TestCaptureTUI(t *testing.T) {
	fontFile := filepath.Join("..", "..", ".planning", "design", "fonts", "JetBrainsMono-Regular.ttf")
	if _, err := os.Stat(fontFile); err != nil {
		t.Fatalf("TestCaptureTUI: vendored font missing at %s: %v", fontFile, err)
	}
	outDir := filepath.Join("..", "..", ".planning", "design", "_spike", "tui")

	model := fixtureModel{width: tuiFixtureWidth, height: tuiFixtureHeight}
	golden := model.View().Content
	if golden == "" {
		t.Fatal("TestCaptureTUI: fixture View() produced an empty golden")
	}

	result, err := CaptureTUI(golden, TUIOptions{
		FontFile: fontFile,
		Theme:    tuiFixtureTheme,
		Width:    tuiFixtureWidth,
		Height:   tuiFixtureHeight,
		OutDir:   outDir,
		Name:     "spike",
	})
	if err != nil {
		t.Fatalf("CaptureTUI: %v", err)
	}

	info, statErr := os.Stat(result.PNGPath)
	if statErr != nil {
		t.Fatalf("CaptureTUI: rendered PNG missing at %s: %v", result.PNGPath, statErr)
	}
	if info.Size() == 0 {
		t.Fatalf("CaptureTUI: rendered PNG at %s is empty", result.PNGPath)
	}

	if result.SHA256 != tuiGoldenSHA256 {
		t.Errorf("CaptureTUI: golden hash mismatch -- got %s, want %s (recorded in "+
			".planning/design/_spike/GOLDENS.md); re-run is not reproducing the recorded golden",
			result.SHA256, tuiGoldenSHA256)
	}
}
