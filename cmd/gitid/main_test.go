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
func TestRunDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("run() panicked: %v", r)
		}
	}()
	run()
}
