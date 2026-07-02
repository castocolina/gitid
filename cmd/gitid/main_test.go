package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// TestVersionNonEmpty verifies the version constant is populated,
// providing a minimal smoke-test that the package compiles and
// the basic constant is reachable.
func TestVersionNonEmpty(t *testing.T) {
	if version == "" {
		t.Fatal("version must be non-empty")
	}
}

// TestNewRootCmdDoesNotPanic confirms building the command tree completes
// without panicking and registers the expected subcommands.
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

	identity, _, err := root.Find([]string{"identity"})
	if err != nil || identity.Use != "identity" {
		t.Fatalf("expected 'identity' subcommand to be registered, err=%v", err)
	}
	if add, _, err := root.Find([]string{"identity", "add"}); err != nil || add.Use != "add" {
		t.Fatalf("expected 'identity add' subcommand, got %v (err=%v)", add, err)
	}
	if testCmd, _, err := root.Find([]string{"identity", "test"}); err != nil || testCmd.Name() != "test" {
		t.Fatalf("expected 'identity test' subcommand, got %v (err=%v)", testCmd, err)
	}
}

// TestNewRootCmdTopLevelAliases verifies that the three new top-level alias
// commands are registered: rotate, copy, and host (with host.add) (CLI-01 / D-05..D-07).
func TestNewRootCmdTopLevelAliases(t *testing.T) {
	root := newRootCmd()

	tests := []struct {
		path []string
		want string
	}{
		{[]string{"rotate"}, "rotate <name>"},
		{[]string{"copy"}, "copy <name>"},
		{[]string{"host"}, "host"},
		{[]string{"host", "add"}, "add"},
	}

	for _, tc := range tests {
		cmd, _, err := root.Find(tc.path)
		if err != nil {
			t.Errorf("root.Find(%v): unexpected error %v", tc.path, err)
			continue
		}
		if cmd.Use != tc.want {
			t.Errorf("cmd.Use = %q, want %q (path=%v)", cmd.Use, tc.want, tc.path)
		}
	}
}

// TestNewRootCmdIdentityCopyRegistered verifies that identity copy subcommand
// is registered (CLI-01 / D-06).
func TestNewRootCmdIdentityCopyRegistered(t *testing.T) {
	root := newRootCmd()
	cmd, _, err := root.Find([]string{"identity", "copy"})
	if err != nil {
		t.Fatalf("root.Find(['identity','copy']): %v", err)
	}
	if cmd.Use != "copy <name>" {
		t.Errorf("identity copy Use = %q, want %q", cmd.Use, "copy <name>")
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
