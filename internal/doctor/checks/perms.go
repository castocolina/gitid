// Package checks implements the per-family health check functions for
// gitid doctor. Each family lives in its own file and is overwritten in
// place by Wave 2 plans without redeclaration.
package checks

import (
	"fmt"
	"os"

	"github.com/castocolina/gitid/internal/doctor"
)

// KEY-02 target modes for each path class.
const (
	modeSSHDir    os.FileMode = 0o700 // ~/.ssh directory
	modePrivKey   os.FileMode = 0o600 // private key
	modePubKey    os.FileMode = 0o644 // .pub file
	modeSSHConfig os.FileMode = 0o600 // ~/.ssh/config (must be 0600)
	modeGitconfig os.FileMode = 0o644 // ~/.gitconfig (git writes 0644 by default; WR-03)
)

// CheckPermissions checks that gitid-managed paths have the KEY-02 target
// modes: ~/.ssh directory 0700, private keys 0600, .pub files 0644, SSH
// config 0600. It uses deps.Stat (injected) so the core remains write-free
// and fully fake-testable. The fixer closes over deps.FixPerm — internal/doctor
// never calls os.Chmod directly (D-01, T-04-02).
func CheckPermissions(deps doctor.Deps) []doctor.Finding {
	var findings []doctor.Finding

	// Check ~/.ssh directory (error severity if wrong — broken env).
	if deps.SSHDir != "" {
		findings = append(findings, checkPath(deps, deps.SSHDir, modeSSHDir, doctor.SeverityError)...)
	}

	// Check private key files (critical severity — key exposure).
	for _, path := range deps.KeyPaths {
		findings = append(findings, checkPath(deps, path, modePrivKey, doctor.SeverityCritical)...)
	}

	// Check .pub files (warning severity — restrictive is inconvenient, not dangerous).
	for _, path := range deps.PubKeyPaths {
		findings = append(findings, checkPath(deps, path, modePubKey, doctor.SeverityWarning)...)
	}

	// Check SSH config (error severity if wrong — could affect SSH behavior).
	if deps.SSHConfigPath != "" {
		findings = append(findings, checkPath(deps, deps.SSHConfigPath, modeSSHConfig, doctor.SeverityError)...)
	}

	// Check gitconfig: only flag when group- or world-writable (WR-03).
	// git itself writes ~/.gitconfig with mode 0644 — flagging that as an error
	// produces a false positive on virtually every machine. We only warn when
	// the file is writable by group or world (real risk is write, not read).
	if deps.GitconfigPath != "" {
		findings = append(findings, checkGitconfigPath(deps, deps.GitconfigPath)...)
	}

	return findings
}

// checkPath stats a single path, compares its mode to want, and returns a
// Finding slice (zero findings = OK, one finding = mode mismatch). Absent
// files (os.ErrNotExist) are skipped — coherence's concern, not perms.
func checkPath(deps doctor.Deps, path string, want os.FileMode, sev doctor.Severity) []doctor.Finding {
	info, err := deps.Stat(path) //nolint:gosec // path is a trusted gitid-managed path (G304)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // missing paths are not a perms problem
		}
		// Other stat errors are also skipped — the coherence check handles resolution.
		return nil
	}

	got := info.Mode().Perm()
	if got == want {
		return nil // mode is correct
	}

	explanation := permExplanation(path, got, want, sev)
	fix := fmt.Sprintf("chmod %04o %s", want, path)

	// Capture path and want for the closure without loop-variable aliasing.
	p, m := path, want
	return []doctor.Finding{
		{
			Family:       doctor.FamilyPerms,
			Severity:     sev,
			Title:        fmt.Sprintf("%s: %04o (expected %04o)", path, got, want),
			Explanation:  explanation,
			SuggestedFix: fix,
			Fix: &doctor.FixDescriptor{
				Summary: fix,
				Fn: func() error {
					return deps.FixPerm(p, m) // D-01: injected, never os.Chmod directly
				},
			},
		},
	}
}

// checkGitconfigPath checks ~/.gitconfig for the write-access risk only (WR-03).
// git writes ~/.gitconfig with 0644 by default; that is acceptable. We only
// warn when the file is writable by group or world — the real exposure risk.
func checkGitconfigPath(deps doctor.Deps, path string) []doctor.Finding {
	info, err := deps.Stat(path) //nolint:gosec // path is a trusted gitid-managed path (G304)
	if err != nil {
		return nil // missing or unreadable — not a perms concern
	}

	got := info.Mode().Perm()
	// Only flag group-write (0o020) or world-write (0o002) — not the read bits.
	const writeMask = os.FileMode(0o022)
	if (got & writeMask) == 0 {
		return nil // no writable bits by group/world — acceptable
	}

	explanation := fmt.Sprintf(
		"~/.gitconfig has mode %04o — group or world write access may allow tampering with git identity,\n"+
			"signing key, or URL rewrite settings. Remove the write bits: chmod %04o %s",
		got, modeGitconfig, path,
	)
	fix := fmt.Sprintf("chmod %04o %s", modeGitconfig, path)
	p := path
	return []doctor.Finding{
		{
			Family:       doctor.FamilyPerms,
			Severity:     doctor.SeverityWarning, // write risk is warning, not error
			Title:        fmt.Sprintf("%s: %04o (expected %04o)", path, got, modeGitconfig),
			Explanation:  explanation,
			SuggestedFix: fix,
			Fix: &doctor.FixDescriptor{
				Summary: fix,
				Fn: func() error {
					return deps.FixPerm(p, modeGitconfig) //nolint:gosec // D-01: injected, never os.Chmod directly
				},
			},
		},
	}
}

// permExplanation returns the human-readable explanation for a mode mismatch,
// matching the UI-SPEC Copywriting Contract for each path class.
func permExplanation(path string, got, want os.FileMode, sev doctor.Severity) string {
	switch {
	case sev == doctor.SeverityCritical:
		return fmt.Sprintf(
			"Private key %s has mode %04o — group or world read permission may expose the key material.\n"+
				"SSH ignores keys that are too permissive; authentication will fail on some servers.",
			path, got,
		)
	case want == modeSSHDir:
		return fmt.Sprintf(
			"~/.ssh directory has mode %04o (expected 0700). Group or world read/execute access\n"+
				"allows other users to enumerate your SSH keys.",
			got,
		)
	case want == modePubKey:
		return fmt.Sprintf(
			"Public key %s has mode %04o (expected 0644). Public keys should be world-readable\n"+
				"so SSH and signing tools can read them.",
			path, got,
		)
	default:
		return fmt.Sprintf(
			"Config file %s has mode %04o (expected %04o). Loose permissions may expose credentials.",
			path, got, want,
		)
	}
}
