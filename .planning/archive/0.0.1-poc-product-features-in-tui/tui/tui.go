package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

// Run launches the Bubble Tea TUI. It builds the doctor and identity deps,
// creates the root model, and runs the tea.Program.
// Alt-screen mode is enabled via the View.AltScreen field in rootModel.View()
// (RESEARCH.md "State of the Art" — tea.WithAltScreen() does not exist in v2).
// It returns an error on program failure; the caller (cmd/gitid/main.go)
// owns the os.Exit call (RESEARCH.md Pattern 7).
//
// Run is the single exported entry point for the tui package. The one-directional
// dependency arrow is preserved: tui imports internal packages; cmd/gitid imports
// tui; internal packages import nothing from tui or cmd/gitid.
func Run() error {
	doctorDeps, identityDeps, updateDeps, deleteDeps, adoptDeps, repoCloneDeps, uploaderDeps, err := buildTUIDeps()
	if err != nil {
		return fmt.Errorf("tui: building deps: %w", err)
	}
	m := newRootModelFull(doctorDeps, identityDeps, updateDeps, deleteDeps, adoptDeps, repoCloneDeps, uploaderDeps)
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("tui: program error: %w", err)
	}
	return nil
}
