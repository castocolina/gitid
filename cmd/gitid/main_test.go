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

// TestRunDoesNotPanic confirms the run function completes without panicking.
func TestRunDoesNotPanic(_ *testing.T) {
	// run() writes to stdout and returns; no panic expected.
	run()
}
