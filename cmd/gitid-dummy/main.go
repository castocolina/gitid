// Command gitid-dummy is the Phase 2 navigation-only TUI dummy (DLV-05).
// It is a physically separate binary from the shipped cmd/gitid product:
// it imports ONLY internal/dummytui (which in turn imports only
// charm.land/bubbletea/v2, charm.land/lipgloss/v2, and their transitive
// deps) — proven by internal/dummytui's nobackend_test.go ALLOWLIST check.
//
// Unlike cmd/gitid/main.go, this binary has no Cobra command tree: it always
// launches the TUI when run from a terminal (the isTTY-gate idiom from
// cmd/gitid/main.go, minus the Cobra Execute() branch — a dummy binary has
// no non-TUI subcommands).
package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"golang.org/x/term"

	"github.com/castocolina/gitid/internal/dummytui"
)

func main() {
	os.Exit(run(os.Stdout, os.Stderr))
}

// run is extracted from main() so tests can drive both branches without
// invoking the real TUI or os.Exit (mirrors cmd/gitid/main.go's
// noArgsAction pattern).
func run(_, errw *os.File) int {
	isTTY := term.IsTerminal(int(os.Stdout.Fd()))
	if !isTTY {
		_, _ = fmt.Fprintln(errw, "gitid-dummy: not a terminal; this binary only renders an interactive nav-only TUI")
		return 1
	}
	p := tea.NewProgram(dummytui.NewModel())
	if _, err := p.Run(); err != nil {
		_, _ = fmt.Fprintf(errw, "gitid-dummy: program error: %v\n", err)
		return 1
	}
	return 0
}
