package main

import "testing"

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
