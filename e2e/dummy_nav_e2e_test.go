//go:build e2e

package e2e

// dummy_nav_e2e_test.go — manifest-driven PTY navigation proof for the REAL
// cmd/gitid-dummy binary (02-03-PLAN.md Task 2).
//
// Reuses e2e/ui_pty_e2e_test.go's harness verbatim (ptySession, startPTY,
// sendKey, waitFor, snapshot, close — single-owner emu goroutine design; not
// re-derived here) plus e2e/harness_test.go's SandboxHome and this file's
// own BuildDummyBinary (harness_test.go).
//
// For EACH .planning/design/*/manifest.json entry (loaded via
// internal/screenshot.LoadManifests — the SAME manifest source
// design_capture_test.go uses, so a screen's TUI reachability and its HTML
// mockup capture are always driven off one shared, hardened schema):
//
//  1. RE-HOME (review HIGH-3a) — a bounded Esc-loop pops any open modal
//     frame (or is a harmless no-op at the top level per route()'s Esc
//     handling, registry.go), then "1" (identity-manager's ActivationKey)
//     switches to the Identities view; every entry is therefore
//     order-independent.
//  2. Drive the entry's ABSOLUTE keysFromHome (for a keyless modal surface
//     this includes the 02-02 LaunchKey that opens the modal from
//     Identities — review C3: the modal is reached through the RUNNING
//     binary the way a real user reaches it, never via a direct
//     standalone-screen-render call into internal/dummytui).
//  3. Assert the decoded frame contains BOTH the active-screen breadcrumb
//     ("<surface>/<screen>", review HIGH-3b) AND the entry's
//     screen-specific Signature.
//
// After every entry, assert ZERO files were created under the sandboxed
// HOME (T-02-NB2) — the runtime complement to internal/dummytui's
// import-graph no-backend allowlist (nobackend_test.go).
//
// With no manifest.json files checked in yet (this plan ships the loader
// and this walker; surfaces add their own manifest.json in later plans),
// the per-surface loop body never runs — the test still boots the real
// binary, asserts it renders, and asserts zero writes: an intentional
// partial no-op, not a skipped test.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/castocolina/gitid/internal/screenshot"
)

// dummyNavKeystrokeDelay is the inter-keystroke pause for the dummy nav
// walk, matching ui_pty_e2e_test.go's keystrokeDelay convention.
const dummyNavKeystrokeDelay = 80 * time.Millisecond

// reHomeMaxEscPops bounds the Esc-loop that pops any open modal frame
// before returning to the Identities home (review HIGH-3a). The current
// modal-launch contract (internal/dummytui/doc.go) supports at most a
// handful of nested modal launches, so this is a generous bound.
const reHomeMaxEscPops = 5

// newDummyPTYCmd constructs an exec.Cmd for the gitid-dummy binary (always
// launches its TUI — no args, no Cobra subcommands) with the given
// sandboxed HOME. The binary path comes from BuildDummyBinary — never from
// user input (G204 clean).
func newDummyPTYCmd(ctx context.Context, bin, home string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, bin) //nolint:gosec // arg-slice; bin from BuildDummyBinary, no user input
	cmd.Env = append(os.Environ(), "HOME="+home, "TERM=xterm-256color")
	return cmd
}

// dummyReady waits for the dummy TUI to render its initial frame (the
// "gitid" app name plus the seeded identity-manager/<entry-screen>
// breadcrumb — a prefix check, since the entry screen's ID is surface-owned,
// see reHome below).
func dummyReady(t *testing.T, s *ptySession) {
	t.Helper()
	last, ok := s.waitFor(8*time.Second, func(text string) bool {
		return strings.Contains(text, "gitid") && strings.Contains(text, "identity-manager/")
	})
	if !ok {
		t.Fatalf("dummyReady: dummy TUI did not render within 8 seconds. Last frame:\n%s", last)
	}
}

// reHome drives the dummy back to the Identities home BEFORE each manifest
// entry (review HIGH-3a), making every entry order-independent: a bounded
// number of Esc presses pops any open modal frame (or is a harmless no-op
// once no modal is open — route()'s top-level Esc handler just resets the
// active screen to entry), then "1" switches to identity-manager, then a
// final Esc guarantees its entry screen. Asserts the header breadcrumb
// shows "identity-manager/" before returning — a PREFIX check (not the
// literal "identity-manager/entry"), because the entry screen's ID is
// surface-owned and not "entry" once a fan-out plan replaces the 02-02
// placeholder via RegisterOrReplace (identity-manager's own entry screen is
// "list-populated" as of 02-06) — this deliberately stays agnostic to
// whichever screen ID the currently-registered identity-manager surface
// treats as its entry, matching dummyReady's own prefix check above.
func reHome(t *testing.T, s *ptySession) {
	t.Helper()
	for i := 0; i < reHomeMaxEscPops; i++ {
		s.sendKey([]byte{0x1b}, dummyNavKeystrokeDelay) // Esc
	}
	s.sendKey([]byte("1"), dummyNavKeystrokeDelay)  // identity-manager's ActivationKey
	s.sendKey([]byte{0x1b}, dummyNavKeystrokeDelay) // Esc: force the entry screen

	last, ok := s.waitFor(3*time.Second, func(text string) bool {
		return strings.Contains(text, "identity-manager/")
	})
	if !ok {
		t.Fatalf("reHome: did not land on identity-manager/<entry>. Last frame:\n%s", last)
	}
}

// sendKeysFromHome drives an entry's ABSOLUTE keystroke sequence from the
// Identities home. "esc" and "enter" map to their raw control bytes; every
// other element is sent as its literal bytes — the same key-string
// vocabulary route()/registry.go uses (tea.KeyMsg.String()), so a manifest
// entry's KeysFromHome is copy-pasteable from the key-allocation table in
// internal/dummytui/doc.go.
func sendKeysFromHome(s *ptySession, keys []string) {
	for _, k := range keys {
		switch k {
		case "esc":
			s.sendKey([]byte{0x1b}, dummyNavKeystrokeDelay)
		case "enter":
			s.sendKey([]byte{'\r'}, dummyNavKeystrokeDelay)
		default:
			s.sendKey([]byte(k), dummyNavKeystrokeDelay)
		}
	}
}

// saveDummyFrame writes the current emulator snapshot under
// .planning/design/dummy-nav-frames/ — a Phase-2-appropriate location
// (deviation from ui_pty_e2e_test.go's saveFrame, which targets
// .planning/phases/05.7-.../ui-frames/; see 02-PATTERNS.md). Non-fatal:
// frame saving is evidence collection only.
func saveDummyFrame(t *testing.T, name string, s *ptySession) {
	t.Helper()
	dir := filepath.Join(repoRoot(t), ".planning", "design", "dummy-nav-frames")
	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // test-only evidence dir (G306)
		t.Logf("saveDummyFrame: MkdirAll: %v (non-fatal)", err)
		return
	}
	path := filepath.Join(dir, name+".txt")
	if err := os.WriteFile(path, []byte(s.snapshot()), 0o644); err != nil { //nolint:gosec // test-only snapshot (G306)
		t.Logf("saveDummyFrame: WriteFile: %v (non-fatal)", err)
		return
	}
	t.Logf("saved dummy-nav PTY frame: %s", path)
}

// assertZeroWrites walks home and fails if the dummy created ANY file or
// directory under the sandboxed HOME (T-02-NB2) — the runtime proof
// complementing internal/dummytui/nobackend_test.go's import-graph
// allowlist. home itself (the walk root, created empty by SandboxHome) is
// excluded from the check.
func assertZeroWrites(t *testing.T, home string) {
	t.Helper()
	var found []string
	err := filepath.WalkDir(home, func(path string, _ os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == home {
			return nil
		}
		found = append(found, path)
		return nil
	})
	if err != nil {
		t.Fatalf("assertZeroWrites: WalkDir(%q): %v", home, err)
	}
	if len(found) > 0 {
		t.Errorf("assertZeroWrites: dummy created %d file(s)/dir(s) under sandboxed HOME %s (DLV-05 zero-write proof failed): %v", len(found), home, found)
	}
}

// TestDummyNavReachesAllScreens is the runnable entry point `make
// dummy-nav-e2e` invokes (via `go test -tags e2e -race -timeout 60s -run
// TestDummyNav ./e2e/...`). See the file header comment for the full
// re-home / keysFromHome / breadcrumb+signature / zero-write contract.
func TestDummyNavReachesAllScreens(t *testing.T) {
	bin := BuildDummyBinary(t)
	home := SandboxHome(t)

	designDir := filepath.Join(repoRoot(t), ".planning", "design")
	entries, err := screenshot.LoadManifests(designDir)
	if err != nil {
		t.Fatalf("LoadManifests(%q): %v", designDir, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	s := startPTY(t, newDummyPTYCmd(ctx, bin, home))

	dummyReady(t, s)
	saveDummyFrame(t, "identity-manager-entry", s)

	bySurface := screenshot.SurfacesByEntries(entries)
	for _, surface := range screenshot.SortedSurfaceNames(bySurface) {
		screens := bySurface[surface]
		t.Run(surface, func(t *testing.T) {
			for _, e := range screens {
				e := e
				t.Run(e.Screen, func(t *testing.T) {
					reHome(t, s)
					sendKeysFromHome(s, e.KeysFromHome)

					screenID := screenshot.ScreenID(e)
					last, ok := s.waitFor(5*time.Second, func(text string) bool {
						return strings.Contains(text, screenID) && strings.Contains(text, e.Signature)
					})
					if !ok {
						t.Fatalf("%s: breadcrumb+signature not reached (keysFromHome=%v). Last frame:\n%s", screenID, e.KeysFromHome, last)
					}
					saveDummyFrame(t, "dummy-nav-"+strings.ReplaceAll(screenID, "/", "-"), s)
				})
			}
		})
	}

	if len(entries) == 0 {
		t.Logf("TestDummyNavReachesAllScreens: no manifest.json files found under %s -- 0 screens driven (expected until later Phase 2 plans add manifests)", designDir)
	}

	// Close the PTY session (terminates the dummy process) BEFORE the
	// zero-write check, so any write the process might issue at shutdown
	// is captured too -- not just writes made while it was running.
	s.close(t)
	assertZeroWrites(t, home)
}
