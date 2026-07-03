package identity

import (
	"sort"
	"testing"
)

// TestStateConstants verifies the State type declares exactly the 8 locked
// MGR-02 labels, with no duplicates and no empty values.
func TestStateConstants(t *testing.T) {
	all := []State{
		StateComplete, StateIncomplete, StateGitOnly, StateKeyUnused,
		StateKeyUsedSSHOnly, StateKeyUsedBoth, StateKeyMissing, StateFragmentPathMissing,
	}
	if len(all) != 8 {
		t.Fatalf("expected exactly 8 State constants, got %d", len(all))
	}

	seen := make(map[State]bool, len(all))
	gotStrings := make([]string, 0, len(all))
	for _, s := range all {
		if s == "" {
			t.Errorf("State constant must not be the empty string")
		}
		if seen[s] {
			t.Errorf("duplicate State constant value %q", s)
		}
		seen[s] = true
		gotStrings = append(gotStrings, s.String())
	}

	want := []string{
		"complete", "incomplete", "git-only", "key-unused",
		"key-used-ssh-only", "key-used-both", "key-missing", "fragment-path-missing",
	}
	sort.Strings(gotStrings)
	sort.Strings(want)
	for i := range want {
		if gotStrings[i] != want[i] {
			t.Fatalf("State vocabulary mismatch: got %v want %v", gotStrings, want)
		}
	}
}

// TestCrossReferenceUnusedKeys_None verifies that when every key path is
// referenced by some Host block, no keys are reported unused.
func TestCrossReferenceUnusedKeys_None(t *testing.T) {
	got := crossReferenceUnusedKeys(
		[]string{"/keys/a", "/keys/b"},
		[]string{"/keys/a", "/keys/b"},
	)
	if len(got) != 0 {
		t.Fatalf("expected no unused keys, got %v", got)
	}
}

// TestCrossReferenceUnusedKeys_SomeUnused verifies that keys present on disk
// but referenced by no Host block are returned in their original order, while
// referenced keys are excluded (mirrors orphans.go Class 3 set-difference).
func TestCrossReferenceUnusedKeys_SomeUnused(t *testing.T) {
	got := crossReferenceUnusedKeys(
		[]string{"/keys/a", "/keys/orphan1", "/keys/b", "/keys/orphan2"},
		[]string{"/keys/a", "/keys/b"},
	)
	want := []string{"/keys/orphan1", "/keys/orphan2"}
	if len(got) != len(want) {
		t.Fatalf("expected %d unused keys, got %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("unused key[%d]: got %q want %q", i, got[i], want[i])
		}
	}
}

// TestCrossReferenceUnusedKeys_Empty verifies empty inputs return an empty
// result with no panic.
func TestCrossReferenceUnusedKeys_Empty(t *testing.T) {
	got := crossReferenceUnusedKeys(nil, nil)
	if len(got) != 0 {
		t.Fatalf("expected empty result for empty inputs, got %v", got)
	}
}
