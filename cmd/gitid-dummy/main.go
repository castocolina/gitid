// Command gitid-dummy is the LIVE interactive design demo of the gitid
// TUI (DLV-05): a fully navigable Bubble Tea v2 app over dummy, in-memory
// state seeded from internal/dummytui/data.go (recipe-faithful per
// recipes/, the North Star). It imports NO backend package and never
// touches any file — every "write" is a reducer transition previewed and
// confirmed through the demo's own ceremony.
//
// Run it in a terminal of at least 100x30. Alt-screen and mouse cell
// motion are enabled by the app model's View (Bubble Tea v2 has no
// tea.WithAltScreen/WithMouse* options).
package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/castocolina/gitid/internal/dummytui"
)

func main() {
	p := tea.NewProgram(dummytui.NewApp())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "gitid-dummy:", err)
		os.Exit(1)
	}
}
