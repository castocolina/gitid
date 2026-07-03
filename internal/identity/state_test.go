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

// classifyCase is one table row for TestClassify: an Account + injected key
// facts, plus the expected IdentityHealth (both axes + Problems) and the
// expected single-label ClassifyState precedence result.
type classifyCase struct {
	name         string
	acct         Account
	keyExists    bool
	keyUsedInSSH bool
	keyUsedInGit bool

	wantIdentity   State
	wantKey        State
	wantProblems   []Problem
	wantClassified State
}

// problemsEqual compares two Problem slices for exact order+content equality,
// treating nil and empty as equivalent (Classify returns nil when no problem
// applies).
func problemsEqual(got, want []Problem) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// TestClassify is the table-driven proof for MGR-02: every one of the 8
// locked labels is asserted by at least one row (rows 1-7 isolate each label
// on a healthy opposite axis), PLUS at least 2 dedicated overlap rows (8-9)
// proving that co-occurring facts on both axes are never collapsed — both
// IdentityState and KeyState are wrong AND both corresponding Problems are
// present simultaneously.
func TestClassify(t *testing.T) {
	cases := []classifyCase{
		// --- Single-focus rows: one label per axis, the other axis healthy. ---
		{
			name:      "complete identity, key used both",
			acct:      Account{Name: "complete", Alias: "complete.github.com", FragmentPath: "/frags/complete", Incomplete: ""},
			keyExists: true, keyUsedInSSH: true, keyUsedInGit: true,
			wantIdentity: StateComplete, wantKey: StateKeyUsedBoth,
			wantProblems: nil, wantClassified: StateComplete,
		},
		{
			name:      "incomplete identity (no gitconfig side), key used both",
			acct:      Account{Name: "incomplete", Alias: "incomplete.github.com", FragmentPath: "", Incomplete: "gitconfig-includeif-block"},
			keyExists: true, keyUsedInSSH: true, keyUsedInGit: true,
			wantIdentity: StateIncomplete, wantKey: StateKeyUsedBoth,
			wantProblems: []Problem{ProblemNoGitconfigBlock}, wantClassified: StateIncomplete,
		},
		{
			name:      "git-only identity (no ssh host block), key used both",
			acct:      Account{Name: "gitonly", Alias: "", FragmentPath: "/frags/gitonly", Incomplete: "ssh-host-block"},
			keyExists: true, keyUsedInSSH: true, keyUsedInGit: true,
			wantIdentity: StateGitOnly, wantKey: StateKeyUsedBoth,
			wantProblems: []Problem{ProblemNoSSHHostBlock}, wantClassified: StateGitOnly,
		},
		{
			name:      "fragment-path-missing identity, key used both",
			acct:      Account{Name: "fragmissing", Alias: "fragmissing.github.com", FragmentPath: "/frags/fragmissing", Incomplete: "fragment-file"},
			keyExists: true, keyUsedInSSH: true, keyUsedInGit: true,
			wantIdentity: StateFragmentPathMissing, wantKey: StateKeyUsedBoth,
			wantProblems: []Problem{ProblemFragmentMissing}, wantClassified: StateFragmentPathMissing,
		},
		{
			name:      "complete identity, key unused",
			acct:      Account{Name: "keyunused", Alias: "keyunused.github.com", FragmentPath: "/frags/keyunused", Incomplete: ""},
			keyExists: true, keyUsedInSSH: false, keyUsedInGit: false,
			wantIdentity: StateComplete, wantKey: StateKeyUnused,
			wantProblems: []Problem{ProblemKeyUnreferenced}, wantClassified: StateKeyUnused,
		},
		{
			name:      "complete identity, key used ssh only",
			acct:      Account{Name: "sshonly", Alias: "sshonly.github.com", FragmentPath: "/frags/sshonly", Incomplete: ""},
			keyExists: true, keyUsedInSSH: true, keyUsedInGit: false,
			wantIdentity: StateComplete, wantKey: StateKeyUsedSSHOnly,
			wantProblems: nil, wantClassified: StateKeyUsedSSHOnly,
		},
		{
			name:      "complete identity, key missing",
			acct:      Account{Name: "keymissing", Alias: "keymissing.github.com", FragmentPath: "/frags/keymissing", Incomplete: ""},
			keyExists: false, keyUsedInSSH: false, keyUsedInGit: false,
			wantIdentity: StateComplete, wantKey: StateKeyMissing,
			wantProblems: []Problem{ProblemKeyFileMissing}, wantClassified: StateKeyMissing,
		},
		// --- Overlap rows: both axes unhealthy simultaneously (D-11 style
		// proof that IdentityHealth never collapses co-occurring facts). ---
		{
			name:      "overlap: fragment-path-missing AND key-missing",
			acct:      Account{Name: "overlap1", Alias: "overlap1.github.com", FragmentPath: "/frags/overlap1", Incomplete: "fragment-file"},
			keyExists: false, keyUsedInSSH: false, keyUsedInGit: false,
			wantIdentity: StateFragmentPathMissing, wantKey: StateKeyMissing,
			wantProblems:   []Problem{ProblemFragmentMissing, ProblemKeyFileMissing},
			wantClassified: StateFragmentPathMissing, // structural axis wins precedence
		},
		{
			name:      "overlap: git-only AND key-unused",
			acct:      Account{Name: "overlap2", Alias: "", FragmentPath: "/frags/overlap2", Incomplete: "ssh-host-block"},
			keyExists: true, keyUsedInSSH: false, keyUsedInGit: false,
			wantIdentity: StateGitOnly, wantKey: StateKeyUnused,
			wantProblems:   []Problem{ProblemNoSSHHostBlock, ProblemKeyUnreferenced},
			wantClassified: StateGitOnly, // structural axis wins precedence
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			health := Classify(c.acct, c.keyExists, c.keyUsedInSSH, c.keyUsedInGit)
			if health.Name != c.acct.Name {
				t.Errorf("Name: got %q want %q", health.Name, c.acct.Name)
			}
			if health.IdentityState != c.wantIdentity {
				t.Errorf("IdentityState: got %q want %q", health.IdentityState, c.wantIdentity)
			}
			if health.KeyState != c.wantKey {
				t.Errorf("KeyState: got %q want %q", health.KeyState, c.wantKey)
			}
			if !problemsEqual(health.Problems, c.wantProblems) {
				t.Errorf("Problems: got %v want %v", health.Problems, c.wantProblems)
			}

			got := ClassifyState(c.acct, c.keyExists, c.keyUsedInSSH, c.keyUsedInGit)
			if got != c.wantClassified {
				t.Errorf("ClassifyState: got %q want %q", got, c.wantClassified)
			}
		})
	}
}

// TestClassifyState_PrecedenceStructuralBeforeKey directly proves the
// documented precedence: a structural IdentityState blocker (git-only) wins
// over a simultaneous key-axis blocker (key-missing).
func TestClassifyState_PrecedenceStructuralBeforeKey(t *testing.T) {
	acct := Account{Name: "precedence", Alias: "", FragmentPath: "/frags/precedence", Incomplete: "ssh-host-block"}
	got := ClassifyState(acct, false, false, false) // key-missing AND git-only both apply
	if got != StateGitOnly {
		t.Errorf("ClassifyState precedence: got %q want %q (structural axis must win over key axis)", got, StateGitOnly)
	}
}
