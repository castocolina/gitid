// Package identity — state.go provides the MGR-02 8-label state taxonomy as a
// PURE classifier over Reconstruct's []Account output plus injected
// key-existence/usage facts. No filesystem access, no sidecar DB lives here
// (DLV-07): the impure fact-gathering aggregator lives in inventory.go.
package identity

import "strings"

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

// Problem is a fine-grained, individually-detectable issue surfaced inside an
// IdentityHealth report. Unlike the single-label State, MULTIPLE Problems can
// co-occur on one identity (e.g. a missing fragment AND a missing key) — this
// is what lets IdentityHealth report orthogonal facts without collapsing them
// into one label.
type Problem string

// The detectable Problem values, one per branch Classify can independently
// trigger.
const (
	// ProblemNoSSHHostBlock: the identity has no SSH Host block of its own.
	ProblemNoSSHHostBlock Problem = "no-ssh-host-block"
	// ProblemNoGitconfigBlock: the identity has no gitconfig includeIf block.
	ProblemNoGitconfigBlock Problem = "no-gitconfig-includeif-block"
	// ProblemFragmentMissing: the gitconfig includeIf block's fragment path
	// does not exist on disk.
	ProblemFragmentMissing Problem = "fragment-file-missing"
	// ProblemKeyFileMissing: the identity's key file does not exist on disk.
	ProblemKeyFileMissing Problem = "key-file-missing"
	// ProblemKeyUnreferenced: the identity's key file exists but is not
	// referenced by any Host block or wired for git commit signing.
	ProblemKeyUnreferenced Problem = "key-unreferenced"
)

// IdentityHealth is the per-identity health report: an orthogonal
// IdentityState axis (structural — is the Host block / gitconfig fragment
// present and coherent) and KeyState axis (is the key file present and how is
// it used), PLUS a Problems list so co-occurring facts on the two axes are
// never collapsed into a single label — e.g. an identity that is BOTH
// fragment-path-missing AND references a missing key surfaces both:
// IdentityState=fragment-path-missing, KeyState=key-missing, and Problems
// containing both ProblemFragmentMissing and ProblemKeyFileMissing.
//
// The name is the plan-locked artifact name (01-04-PLAN.md must_haves/
// artifacts) consumed by 01-06's debug/list command; the revive "stutters"
// suggestion (identity.Health) is intentionally not applied so the name stays
// grep-stable across the plan chain.
//
//nolint:revive // plan-locked artifact name (see comment above), not renamed to avoid stutter
type IdentityHealth struct {
	Name          string
	IdentityState State
	KeyState      State
	Problems      []Problem
}

// missingSet parses Account.Incomplete — a comma-joined list of missing-piece
// markers set by Reconstruct ("ssh-host-block", "gitconfig-includeif-block",
// "fragment-file") — into a membership set for Classify to branch on.
func missingSet(incomplete string) map[string]bool {
	set := make(map[string]bool)
	if incomplete == "" {
		return set
	}
	for _, part := range strings.Split(incomplete, ",") {
		set[part] = true
	}
	return set
}

// Classify is a PURE function over Reconstruct's Account output plus injected
// key-fact booleans (keyExists, keyUsedInSSH, keyUsedInGit) — no filesystem
// access, no sidecar DB (DLV-07). It derives the IdentityState axis from
// acct.Incomplete/acct.Alias/acct.FragmentPath — already resolved by
// Reconstruct, which appends "fragment-file" to Incomplete when its readFrag
// seam reports the fragment missing, so a non-existent fragment path is
// already visible here without a separate filesystem check — and derives the
// KeyState axis from the three injected booleans. A Problem is appended for
// EVERY individually-detected issue so overlapping facts on the two axes are
// never collapsed into one label.
//
// IdentityState branch order (most specific first): a gitconfig includeIf
// block that points at a non-existent fragment (fragment-path-missing) is
// reported even when the SSH side is also incomplete, because it is the more
// actionable diagnosis; a missing SSH Host block with a present gitconfig
// side is git-only; a missing gitconfig side is incomplete; anything else is
// complete.
//
// KeyState branch order: a missing key file is reported first (key-missing);
// otherwise a key used for BOTH SSH auth and git signing is key-used-both,
// SSH-only use is key-used-ssh-only, and — because the locked 8-label
// vocabulary has no dedicated "git-signing-only" key label — a key used only
// for git signing (without an SSH Host block reference) is also bucketed
// key-used-both, since it IS actively used and must not be mislabeled
// key-unused; a key that is neither is key-unused.
func Classify(acct Account, keyExists, keyUsedInSSH, keyUsedInGit bool) IdentityHealth {
	missing := missingSet(acct.Incomplete)
	var problems []Problem

	var idState State
	switch {
	case missing["fragment-file"]:
		idState = StateFragmentPathMissing
		problems = append(problems, ProblemFragmentMissing)
	case missing["ssh-host-block"] && acct.FragmentPath != "":
		idState = StateGitOnly
		problems = append(problems, ProblemNoSSHHostBlock)
	case missing["gitconfig-includeif-block"]:
		idState = StateIncomplete
		problems = append(problems, ProblemNoGitconfigBlock)
	case missing["ssh-host-block"]:
		// Degenerate case: neither side is fully populated (no dedicated 9th
		// label exists in the locked MGR-02 vocabulary for this).
		idState = StateIncomplete
		problems = append(problems, ProblemNoSSHHostBlock)
	default:
		idState = StateComplete
	}

	var keyState State
	switch {
	case !keyExists:
		keyState = StateKeyMissing
		problems = append(problems, ProblemKeyFileMissing)
	case keyUsedInSSH && keyUsedInGit:
		keyState = StateKeyUsedBoth
	case keyUsedInSSH:
		keyState = StateKeyUsedSSHOnly
	case keyUsedInGit:
		keyState = StateKeyUsedBoth
	default:
		keyState = StateKeyUnused
		problems = append(problems, ProblemKeyUnreferenced)
	}

	return IdentityHealth{
		Name:          acct.Name,
		IdentityState: idState,
		KeyState:      keyState,
		Problems:      problems,
	}
}

// ClassifyState collapses Classify's two-axis IdentityHealth into a single
// State by a DOCUMENTED precedence order, for callers that want one label per
// identity (the original single-state MGR-02 API). Structural IdentityState
// blockers are reported before key-axis problems, and key-axis problems
// before the fully-healthy "complete" label. Precedence (most severe first):
//
//  1. fragment-path-missing (IdentityState)
//  2. git-only              (IdentityState)
//  3. incomplete            (IdentityState)
//  4. key-missing           (KeyState)
//  5. key-unused            (KeyState)
//  6. key-used-ssh-only     (KeyState)
//  7. complete              (both axes healthy)
func ClassifyState(acct Account, keyExists, keyUsedInSSH, keyUsedInGit bool) State {
	h := Classify(acct, keyExists, keyUsedInSSH, keyUsedInGit)

	switch h.IdentityState {
	case StateFragmentPathMissing, StateGitOnly, StateIncomplete:
		return h.IdentityState
	}

	switch h.KeyState {
	case StateKeyMissing, StateKeyUnused, StateKeyUsedSSHOnly:
		return h.KeyState
	}

	return StateComplete
}
