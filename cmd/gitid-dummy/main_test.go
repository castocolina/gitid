package main

import (
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/dummytui"
	"github.com/castocolina/gitid/internal/tuikit"
)

// TestDemoAppConstructsAndRenders is a smoke test over the SAME wiring
// main() uses — tuikit.NewApp injected with dummytui.NewFixtureBackend() —
// so a seam that is declared but never actually wired here cannot pass
// silently. Full behavior is tested inside internal/tuikit; the real binary
// is driven end-to-end by e2e/dummy_demo_e2e_test.go. This file also keeps
// `make test`'s -coverprofile run green: a buildable package with NO test
// files makes the coverage tooling reach for the `covdata` tool, which the
// auto-downloaded Go toolchain does not ship.
func TestDemoAppConstructsAndRenders(t *testing.T) {
	view := tuikit.NewApp(dummytui.NewFixtureBackend()).View()
	if !strings.Contains(view.Content, "gitid") {
		t.Fatalf("NewApp(FixtureBackend).View() did not render the frame; got %q", view.Content)
	}
	// The fixture seed must actually reach the frame: the header chip
	// renders the fixture identity count, not an empty state.
	if !strings.Contains(view.Content, "8 ids") {
		t.Errorf("the injected FixtureBackend must seed the 8 fixture identities; got %q", view.Content)
	}
}
