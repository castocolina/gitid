package main

import (
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/dummytui"
)

// TestDemoAppConstructsAndRenders is a smoke test: the app model behind
// this entry point builds and renders its frame. Full behavior is tested
// inside internal/dummytui; the real binary is driven end-to-end by
// e2e/dummy_demo_e2e_test.go. This file also keeps `make test`'s
// -coverprofile run green: a buildable package with NO test files makes
// the coverage tooling reach for the `covdata` tool, which the
// auto-downloaded Go toolchain does not ship.
func TestDemoAppConstructsAndRenders(t *testing.T) {
	view := dummytui.NewApp().View()
	if !strings.Contains(view.Content, "gitid") {
		t.Fatalf("NewApp().View() did not render the frame; got %q", view.Content)
	}
}
