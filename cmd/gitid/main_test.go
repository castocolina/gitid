package main

import (
	"bytes"
	"errors"
	"runtime"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/version"
)

// TestVersionStringNonEmpty verifies versionString() is populated, providing
// a minimal smoke-test that the package compiles and the composed stamp is
// reachable. Replaces the retired TestVersionNonEmpty (the package-level
// `version` identifier moved into internal/version — Phase 10, D-09).
func TestVersionStringNonEmpty(t *testing.T) {
	if versionString() == "" {
		t.Fatal("versionString() must be non-empty")
	}
}

func TestComposeVersion(t *testing.T) {
	got := composeVersion(version.Info{Version: "1.2.3", Commit: "abc1234", BuildDate: "2026-08-30"}, "darwin", "arm64")
	want := "1.2.3 (abc1234, 2026-08-30, darwin/arm64)"
	if got != want {
		t.Fatalf("composeVersion() = %q, want %q", got, want)
	}
}

// TestVersionStringIncludesPlatformSuffix locks in D-11's platform suffix:
// the composed stamp must always contain "<goos>/<goarch>". Replaces the
// retired TestComposeVersionUsesTheDevDefaults, whose literal
// "0.0.0-dev (none, unknown)" assumed a static Makefile default that D-10
// retires in favor of a live git-describe value.
func TestVersionStringIncludesPlatformSuffix(t *testing.T) {
	got := versionString()
	want := runtime.GOOS + "/" + runtime.GOARCH
	if !strings.Contains(got, want) {
		t.Fatalf("versionString() = %q, want it to contain %q", got, want)
	}
}

func TestRootCommandVersionIsTheComposedStamp(t *testing.T) {
	got := newRootCmd().Version
	want := versionString()
	if got != want {
		t.Fatalf("newRootCmd().Version = %q, want %q", got, want)
	}
}

func TestVersionFlagOutput(t *testing.T) {
	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--version"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute(--version): %v", err)
	}
	want := "gitid version " + versionString() + "\n"
	if got := buf.String(); got != want {
		t.Fatalf("--version output = %q, want %q", got, want)
	}
}

// TestNewRootCmdDoesNotPanic confirms building the command tree completes
// without panicking and keeps the Phase-1 `debug` diagnostic surface.
func TestNewRootCmdDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("newRootCmd() panicked: %v", r)
		}
	}()

	root := newRootCmd()
	if root.Use != "gitid" {
		t.Fatalf("root.Use = %q, want gitid", root.Use)
	}

	debug, _, err := root.Find([]string{"debug"})
	if err != nil || debug.Name() != "debug" {
		t.Fatalf("expected 'debug' subcommand to be registered, err=%v", err)
	}
	if caps, _, err := root.Find([]string{"debug", "caps"}); err != nil || caps.Name() != "caps" {
		t.Fatalf("expected 'debug caps' subcommand, got %v (err=%v)", caps, err)
	}
}

// TestNewRootCmdArchivedPOCCommandsAreGone locks in D-14: the 0.0.1 POC Cobra
// surface stays archived except for the D-01 taxonomy Phase 5 deliberately
// rebuilds (SHELL-03) — "identity" is now a real, intentionally-registered
// noun group (see TestNewRootCmdSurfaceIsPhase5CLI), so it is removed from
// this archived list; every other POC command name must still be absent.
func TestNewRootCmdArchivedPOCCommandsAreGone(t *testing.T) {
	root := newRootCmd()
	// "rotate" was removed from this archived list in plan 05-08 exactly like
	// "identity" was removed in 05-01: the D-01 taxonomy deliberately rebuilds
	// the noun form (`gitid identity rotate`) and its flat alias, so it is no
	// longer an archived command but a real, intentionally-registered one.
	archived := [][]string{
		{"baseline"}, {"adopt"},
		{"copy"}, {"host"}, {"add"}, {"match"}, {"upload"},
	}
	for _, path := range archived {
		if _, _, err := root.Find(path); err == nil {
			t.Errorf("archived POC command %v is still registered (D-14)", path)
		}
	}
}

// TestNewRootCmdSurfaceIsPhase5CLI asserts the WHOLE top-level command
// surface Phase 5 (05-01-PLAN.md Task 3 + 05-08-PLAN.md Task 1, D-01) builds,
// so a future addition is a deliberate decision rather than an accident: the
// identity noun group and its flat aliases (including the 05-08 write verbs),
// the reserved ssh/git/health/fix noun groups, the Phase-1 debug readout, and
// Cobra's auto-registered completion/help.
func TestNewRootCmdSurfaceIsPhase5CLI(t *testing.T) {
	root := newRootCmd()
	root.InitDefaultCompletionCmd()
	root.InitDefaultHelpCmd()

	want := map[string]bool{
		"debug": true, "completion": true, "help": true,
		"identity": true, "ssh": true, "git": true, "health": true, "fix": true, "doctor": true,
		"list": true, "show": true, "delete": true,
		"create": true, "clone": true, "new-key": true, "rotate": true,
		// 09-05-PLAN.md (UP-02, UP-03): the manual re-run surface for the
		// autonomous provider-key registration plan 09-04 built — see
		// TestNewRootCmdArchivedPOCCommandsAreGone, deliberately untouched.
		"register-key": true,
		// Phase 10, D-11: `gitid version [--json]`.
		"version": true,
	}
	for _, cmd := range root.Commands() {
		if !want[cmd.Name()] {
			t.Errorf("unexpected subcommand %q registered; update this test deliberately if it is intentional", cmd.Name())
		}
		delete(want, cmd.Name())
	}
	for name := range want {
		t.Errorf("expected subcommand %q to be registered", name)
	}
}

// TestNoArgsActionNonTTY verifies that noArgsAction with isTTY=false writes
// the usage hint to errw and returns exit code 1 (TUI-01 non-TTY contract).
func TestNoArgsActionNonTTY(t *testing.T) {
	var out, errw bytes.Buffer
	code := noArgsAction(false, func() error { return nil }, &out, &errw)
	if code != 1 {
		t.Errorf("noArgsAction(isTTY=false) = %d, want 1", code)
	}
	hint := errw.String()
	if !strings.Contains(hint, "gitid: no subcommand given") {
		t.Errorf("noArgsAction(isTTY=false) hint = %q; want 'gitid: no subcommand given'", hint)
	}
}

// TestNoArgsActionTTYSuccess verifies that noArgsAction with isTTY=true and a
// no-error run function returns exit code 0 (TUI-01 TTY success path).
func TestNoArgsActionTTYSuccess(t *testing.T) {
	var out, errw bytes.Buffer
	code := noArgsAction(true, func() error { return nil }, &out, &errw)
	if code != 0 {
		t.Errorf("noArgsAction(isTTY=true, run=nil-err) = %d, want 0", code)
	}
}

// TestNoArgsActionTTYRunError verifies that noArgsAction with isTTY=true and
// a run function that returns an error writes the error to errw and returns
// exit code 1.
func TestNoArgsActionTTYRunError(t *testing.T) {
	var out, errw bytes.Buffer
	code := noArgsAction(true, func() error { return errors.New("tui crashed") }, &out, &errw)
	if code != 1 {
		t.Errorf("noArgsAction(isTTY=true, run=error) = %d, want 1", code)
	}
	if !strings.Contains(errw.String(), "tui crashed") {
		t.Errorf("errw = %q; want error message 'tui crashed'", errw.String())
	}
}
