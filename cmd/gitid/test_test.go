package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestRunIdentityTestDoesNotPanic exercises the `identity test` handler with a
// non-resolving alias and asserts it completes without panicking and prints the
// resolved-config labels. It is read-only (ssh -T / ssh -G never mutate files)
// and uses the recover panic-guard convention.
func TestRunIdentityTestDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("runIdentityTest() panicked: %v", r)
		}
	}()

	var out bytes.Buffer
	// A clearly non-existent alias: ssh resolves nothing; the handler must still
	// render its labels without panicking.
	if err := runIdentityTest(&out, "gitid-nonexistent.invalid"); err != nil {
		t.Fatalf("runIdentityTest() returned error: %v", err)
	}
	if !strings.Contains(out.String(), "identitiesonly") {
		t.Errorf("expected resolved-config labels in output, got:\n%s", out.String())
	}
}

// TestRunIdentityTestEmptyAlias asserts an empty alias is rejected.
func TestRunIdentityTestEmptyAlias(t *testing.T) {
	var out bytes.Buffer
	if err := runIdentityTest(&out, "   "); err == nil {
		t.Fatal("runIdentityTest() expected error on empty alias")
	}
}
