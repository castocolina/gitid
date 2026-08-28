package checks

import (
	"fmt"
	"os"

	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/sshconfig"
)

// CheckOrphans detects artifacts on disk that no owning managed block claims —
// the inverse of the Coherence family's "incomplete" marker (D-09). Orphans are
// reported under their own Orphans family, strictly distinct from Coherence (D-10).
//
// Three classes of orphans are detected:
//
//  1. SSH Host block name in SSHManagedBlockNames with no matching name in
//     GitconfigManagedBlockNames → SSH-only block → info, report-only. The
//     block may have been created SSH-only or may intentionally retain SSH
//     after its Git identity was removed.
//
//  2. Gitconfig managed block name in GitconfigManagedBlockNames with no matching
//     name in SSHManagedBlockNames → orphaned gitconfig fragment block → warning + Fix
//     (managed-block orphan removal, D-11). Fix.Fn calls deps.RemoveBlock with
//     deps.GitconfigPath and the block name.
//
//  3. A key file in KeyPaths that exists on disk (Stat→OK) but whose path does NOT
//     appear in AllSSHHostIdentityFiles (the union of every IdentityFile from every Host
//     block — gitid-managed AND hand-written, D-12) → unused-key warning, NO Fix
//     (D-03/D-13 report-only, honest wording). Guarded against missing pub files
//     (Pitfall 7).
//
// Note: Class 1 intentionally reports a one-sided SSH block without inferring its
// history. An on-disk block cannot reveal whether it was created SSH-only or its Git
// counterpart was deliberately removed, so it is always informational and never offered
// for removal. Class 2 remains a removable orphan when its SSH counterpart is absent.
//
// The function never reads known_hosts (D-14) and never imports internal/filewriter (D-01).
func CheckOrphans(deps doctor.Deps) []doctor.Finding {
	var findings []doctor.Finding

	// --- Class 1 + 2: cross-reference SSH managed block names vs gitconfig block names.
	// An SSH block with no gitconfig counterpart, or vice-versa, is an orphaned block.

	gcNames := sliceToSet(deps.GitconfigManagedBlockNames)
	sshNames := sliceToSet(deps.SSHManagedBlockNames)

	// Class 1: SSH block names that have no matching gitconfig managed block.
	// This state is informational only because a healthy SSH-only block must not
	// be removed based on an unknowable history.
	for _, name := range deps.SSHManagedBlockNames {
		// Reserved non-identity wiring (the gitid-owned Include line) has no
		// gitconfig counterpart by design — it is NOT an orphan. Skip it, or
		// its removal fix would delete the legitimate Include line and fight
		// EnsureIncludeLine's restore in an endless loop (Pitfall 4 / project
		// memory "Doctor reserved-block false-positive loop" — the SSH-side
		// mirror of the Class 2 gitconfig guard below).
		if sshconfig.IsReservedBlockName(name) {
			continue
		}
		if !gcNames[name] {
			findings = append(findings, doctor.Finding{
				Family:       doctor.FamilyOrphans,
				Severity:     doctor.SeverityInfo,
				Title:        fmt.Sprintf("SSH Host block %q: no gitconfig includeIf", name),
				Explanation:  fmt.Sprintf("This SSH block %q has no matching Git identity — created SSH-only, or its Git side was removed intentionally.", name),
				SuggestedFix: "Informational only, no action offered.",
				Fix:          nil,
				Target:       "SSH",
			})
		}
	}

	// Class 2: gitconfig block names that have no matching SSH Host block.
	// Fix.Fn calls deps.RemoveBlock on GitconfigPath with the block name (when wired).
	for _, name := range deps.GitconfigManagedBlockNames {
		// Reserved non-identity wiring (e.g. baseline-include) has no SSH Host
		// block by design — it is NOT an orphan. Skip it, or its removal fix
		// would delete the legitimate baseline include and fight the Baseline
		// check's restore in an endless loop.
		if gitconfig.IsReservedBlockName(name) {
			continue
		}
		if !sshNames[name] {
			// This gitconfig block has no SSH Host partner — orphaned gitconfig block.
			n := name // capture for closure
			gitconfigPath := deps.GitconfigPath
			removeBlock := deps.RemoveBlock
			// Build Fix only when RemoveBlock is wired; otherwise report-only.
			var fix *doctor.FixDescriptor
			if removeBlock != nil && gitconfigPath != "" {
				fix = &doctor.FixDescriptor{
					Summary: fmt.Sprintf("remove orphaned gitconfig block %q", n),
					Fn: func() error {
						return removeBlock(gitconfigPath, n)
					},
				}
			}
			findings = append(findings, doctor.Finding{
				Family:      doctor.FamilyOrphans,
				Severity:    doctor.SeverityWarning,
				Title:       fmt.Sprintf("gitconfig block %q: no SSH Host block", n),
				Explanation: fmt.Sprintf("A gitconfig managed block %q exists but no SSH Host block claims it.", n),
				SuggestedFix: fmt.Sprintf(
					"remove the orphaned gitconfig block %q  (gitid will confirm before removing)", n),
				Fix:    fix,
				Target: "Git",
			})
		}
	}

	// --- Class 3: unused key files (D-12, D-13).
	// Build a set of all IdentityFile paths from every Host block (managed + hand-written).
	referencedKeys := sliceToSet(deps.AllSSHHostIdentityFiles)

	for _, keyPath := range deps.KeyPaths {
		// Guard: only flag keys that actually exist on disk (Pitfall 7).
		_, err := deps.Stat(keyPath) //nolint:gosec // keyPath is a trusted gitid-managed path (G304)
		if err != nil {
			if os.IsNotExist(err) {
				continue // key file missing; coherence will handle this
			}
			continue // other stat errors — skip
		}

		// D-12: cross-reference against ALL Host blocks (managed + hand-written).
		if !referencedKeys[keyPath] {
			// Not referenced by any Host block — report as unused key warning.
			// D-13: warning only, NO Fix; wording must admit gitid cannot confirm unused.
			kp := keyPath // capture for closures
			findings = append(findings, doctor.Finding{
				Family:   doctor.FamilyOrphans,
				Severity: doctor.SeverityWarning,
				Title:    fmt.Sprintf("%s: not referenced in ~/.ssh/config", kp),
				Explanation: "This key is not referenced by any SSH Host block (gitid-managed or hand-written). " +
					"It may be used for direct server SSH or 'ssh -i' — review before deleting.",
				SuggestedFix: fmt.Sprintf(
					"inspect usage manually; delete with 'rm %s' if confirmed unused", kp),
				Fix:    nil, // key deletion is NEVER auto-fixed (D-03/D-13)
				Target: "SSH",
			})
		}
	}

	return findings
}

// sliceToSet converts a string slice to a boolean presence map.
func sliceToSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}
