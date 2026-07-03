// Package identity — state.go provides the MGR-02 8-label state taxonomy as a
// PURE classifier over Reconstruct's []Account output plus injected
// key-existence/usage facts. No filesystem access, no sidecar DB lives here
// (DLV-07): the impure fact-gathering aggregator lives in inventory.go.
package identity

// State is the shared 8-label MGR-02 vocabulary. The SAME State type is used
// for both the IdentityState and KeyState axes of IdentityHealth (see
// Classify) and for the single-label ClassifyState precedence result — the
// eight labels are locked and MUST NOT be renamed or extended: complete,
// incomplete, git-only, key-unused, key-used-ssh-only, key-used-both,
// key-missing, fragment-path-missing.
type State string

// The 8 locked MGR-02 state labels (the shared vocabulary).
const (
	// StateComplete: the Host block and gitconfig fragment are both present
	// (structural completeness).
	StateComplete State = "complete"
	// StateIncomplete: the SSH side is present but there is no gitconfig
	// includeIf block for this identity.
	StateIncomplete State = "incomplete"
	// StateGitOnly: a git identity relies on the global SSH config — it has
	// no own Host block.
	StateGitOnly State = "git-only"
	// StateKeyUnused: the key file exists on disk but no identity references
	// it (key axis).
	StateKeyUnused State = "key-unused"
	// StateKeyUsedSSHOnly: the key is referenced by a Host block but is not
	// wired for git commit signing.
	StateKeyUsedSSHOnly State = "key-used-ssh-only"
	// StateKeyUsedBoth: the key is wired for both SSH auth and git commit
	// signing.
	StateKeyUsedBoth State = "key-used-both"
	// StateKeyMissing: the identity references a key file that is absent
	// from disk.
	StateKeyMissing State = "key-missing"
	// StateFragmentPathMissing: a gitconfig includeIf block points at a
	// gitconfig fragment file that does not exist.
	StateFragmentPathMissing State = "fragment-path-missing"
)

// String implements fmt.Stringer so State values render directly in debug/
// list output and test failure messages (D-08).
func (s State) String() string { return string(s) }

// crossReferenceUnusedKeys returns the subset of keyPaths that do NOT appear
// in referencedIdentityFiles, preserving keyPaths' original order — a pure
// set-difference. It mirrors the doctor Orphans check's Class 3 unused-key
// logic so that logic lives in exactly one place (RESEARCH.md Open Question
// 2: the doctor package must never import the filewriter package, so the
// shared helper lives here in identity, which the doctor package already
// depends on for identity-shaped data).
//
// Pure: no filesystem access, no sidecar DB. Callers resolve "keyPaths" (the
// keys that actually exist on disk) and "referencedIdentityFiles" (every
// IdentityFile value from every Host block) via their own injected seams
// before calling this function.
func crossReferenceUnusedKeys(keyPaths []string, referencedIdentityFiles []string) []string {
	referenced := make(map[string]bool, len(referencedIdentityFiles))
	for _, p := range referencedIdentityFiles {
		referenced[p] = true
	}

	var unused []string
	for _, kp := range keyPaths {
		if !referenced[kp] {
			unused = append(unused, kp)
		}
	}
	return unused
}
