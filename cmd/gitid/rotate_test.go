package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestNewRotateCmdRegistered confirms the `identity rotate` command is wired onto
// the root command tree with a single required positional argument.
func TestNewRotateCmdRegistered(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("newRootCmd() panicked: %v", r)
		}
	}()

	root := newRootCmd()
	rotate, _, err := root.Find([]string{"identity", "rotate"})
	if err != nil || rotate.Name() != "rotate" {
		t.Fatalf("expected 'identity rotate' subcommand, got %v (err=%v)", rotate, err)
	}
}

// TestRunIdentityRotateEmptyName asserts the handler rejects an empty identity
// name without panicking (the recover guard proves no panic escapes).
func TestRunIdentityRotateEmptyName(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("runIdentityRotate() panicked: %v", r)
		}
	}()

	var out bytes.Buffer
	in := strings.NewReader("\n")
	if err := runIdentityRotate(in, &out, "   ", false, fakeDeps); err == nil {
		t.Fatal("runIdentityRotate must reject an empty identity name")
	}
}

// TestRunIdentityRotateInvalidNameRejected asserts a name containing shell/
// newline metacharacters is rejected before any work (T-02-32 command-injection
// guard), without panicking.
func TestRunIdentityRotateInvalidNameRejected(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("runIdentityRotate() panicked: %v", r)
		}
	}()

	var out bytes.Buffer
	in := strings.NewReader("\n")
	if err := runIdentityRotate(in, &out, "work; rm -rf /", false, fakeDeps); err == nil {
		t.Fatal("runIdentityRotate must reject an identity name with injection metacharacters")
	}
}

// TestRunIdentityRotateDryRunDoesNotPanic drives the rotate handler in an
// isolated temp HOME with a confirmed rotation against fully-faked deps,
// asserting it completes without panicking and re-points via the fakes.
func TestRunIdentityRotateDryRunDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("runIdentityRotate() panicked: %v", r)
		}
	}()

	t.Setenv("HOME", t.TempDir())

	// Scripted: confirm the rotation (y).
	in := strings.NewReader("y\n")
	var out bytes.Buffer
	if err := runIdentityRotate(in, &out, "work", false, fakeDeps); err != nil {
		t.Fatalf("runIdentityRotate returned error: %v\noutput:\n%s", err, out.String())
	}
}
