// Package checks implements the per-family health check functions for
// internal/doctor. Each exported function takes a doctor.Deps and returns
// []doctor.Finding — pure data, no writes (D-01).
package checks

import (
	"fmt"
	"strings"

	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/gitconfig"
)

// agentState classifies the result of an ssh-add -l probe.
type agentState int

const (
	agentUnreachable     agentState = iota // exit 2 or exec error
	agentRunningEmpty                      // exit 1 / "no identities"
	agentRunningWithKeys                   // exit 0
)

// classifyAgentState interprets (output, exitCode) from ssh-add -l.
// Uses both exit code AND output text for portability across OpenSSH versions
// (RESEARCH Pitfall 1): some older versions may return exit 2 for "no keys"
// instead of exit 1, so text is the reliable secondary signal.
func classifyAgentState(output string, exitCode int) agentState {
	switch exitCode {
	case 0:
		return agentRunningWithKeys
	case 1:
		// Both portability variants: "The agent has no identities." (OpenSSH 8+)
		// and the older "no identities" form are treated as running-empty.
		return agentRunningEmpty
	default:
		// Exit 2 (or any unknown code) means unreachable. Check text as a
		// secondary guard for old OpenSSH versions that may return exit 1 for
		// unreachable instead of the empty case.
		if strings.Contains(output, "no identities") || strings.Contains(output, "has no identities") {
			// Exit 2 text says "no identities" — old SSH quirk; treat as empty.
			return agentRunningEmpty
		}
		return agentUnreachable
	}
}

// extractFingerprint parses the SHA256:... token from a ssh-keygen -lf output
// line.
// Input:  "256 SHA256:vRBdzHY... comment (ED25519)"
// Output: "SHA256:vRBdzHY..."
// Returns "" when no SHA256: token is present.
func extractFingerprint(keygenLine string) string {
	for _, field := range strings.Fields(keygenLine) {
		if strings.HasPrefix(field, "SHA256:") {
			return field
		}
	}
	return ""
}

// isKeyLoaded reports whether the key at pubKeyPath is currently loaded in the
// agent. It calls the injected runFp to obtain the fingerprint line, then
// searches agentOutput for the SHA256 token.
// Returns false if runFp errors (missing pub file — Pitfall 7 caller guard).
func isKeyLoaded(agentOutput, pubKeyPath string, runFp func(string) (string, error)) bool {
	fpLine, err := runFp(pubKeyPath)
	if err != nil {
		return false
	}
	fp := extractFingerprint(fpLine)
	return fp != "" && strings.Contains(agentOutput, fp)
}

// CheckAgent checks whether the ssh-agent is reachable and each gitid-managed
// key is loaded in the running agent.
//
//   - If unreachable: one Agent warning ("ssh-agent: not reachable"), no per-key
//     findings — there is no point checking keys when the agent is down.
//   - If running: for each gitid-managed identity, guard for a missing pub file
//     (Pitfall 7 — coherence already reports it) then probe the fingerprint match.
//     A key absent from the agent yields a per-identity Agent warning.
//   - No finding carries a FixDescriptor — agent loading is report-only (D-03).
func CheckAgent(deps doctor.Deps) []doctor.Finding {
	if deps.RunSSHAdd == nil {
		return nil
	}

	agentOut, exitCode := deps.RunSSHAdd()
	state := classifyAgentState(agentOut, exitCode)

	if state == agentUnreachable {
		return []doctor.Finding{
			{
				Family:   doctor.FamilyAgent,
				Severity: doctor.SeverityWarning,
				Title:    "ssh-agent: not reachable",
				Explanation: "Cannot connect to the SSH agent. " +
					"Passphrase-protected keys will prompt for passphrase on each use.",
				SuggestedFix: `start the agent with 'eval "$(ssh-agent -s)"' and re-add your keys`,
				Fix:          nil, // report-only per D-03
			},
		}
	}

	// Agent is running (with or without keys). Check each managed identity.
	var findings []doctor.Finding
	for _, acct := range deps.Identities {
		if acct.PubPath == "" {
			// No pub path — skip (coherence covers it).
			continue
		}
		// Guard: skip if the pub file is absent (Pitfall 7).
		if deps.Stat != nil {
			if _, err := deps.Stat(acct.PubPath); err != nil {
				continue
			}
		}

		if !isKeyLoaded(agentOut, acct.PubPath, deps.RunSSHKeygenFingerprint) {
			findings = append(findings, doctor.Finding{
				Family:   doctor.FamilyAgent,
				Severity: doctor.SeverityWarning,
				// UI-SPEC DOC-05: identity "<name>": key not loaded in agent
				Title: fmt.Sprintf(`identity %q: key not loaded in agent`, acct.Name),
				Explanation: fmt.Sprintf(
					"The key %s is not in the running ssh-agent. "+
						"Operations may prompt for passphrase.",
					acct.KeyPath,
				),
				SuggestedFix: fmt.Sprintf("ssh-add %s", acct.KeyPath),
				Fix:          nil, // report-only per D-03
			})
		}
	}
	return findings
}

// CheckSigning checks the git-version gate (D-20, DOC-05): warns when any
// gitid-managed identity uses the hasconfig: match strategy and the local git
// is older than 2.36. The gpg.format=ssh and allowed_signers email checks are
// owned by Plan 03 CheckCoherence (D-17 carve-outs) — not duplicated here.
func CheckSigning(deps doctor.Deps) []doctor.Finding {
	if deps.GitVersionAtLeast == nil {
		return nil
	}

	// Only check the version gate once — if any identity uses hasconfig: and
	// git is old, emit one warning (not one per identity, to avoid noise).
	hasHashconfig := false
	for _, acct := range deps.Identities {
		for _, m := range acct.Matches {
			if m.Kind == gitconfig.MatchHasconfig {
				hasHashconfig = true
				break
			}
		}
		if hasHashconfig {
			break
		}
	}

	if !hasHashconfig {
		return nil
	}

	if deps.GitVersionAtLeast(2, 36) {
		return nil
	}

	// Determine the actual git version string for the copy (best-effort).
	// The gate already told us it's < 2.36; we surface a generic label.
	return []doctor.Finding{
		{
			Family:   doctor.FamilySigning,
			Severity: doctor.SeverityWarning,
			// UI-SPEC DOC-05: git <actual_version>: hasconfig: not supported
			Title: "git: hasconfig: not supported",
			Explanation: "One or more identities use 'hasconfig:remote.*.url:' match strategy, " +
				"which requires git >= 2.36.",
			SuggestedFix: "upgrade git (required: >= 2.36)\n" +
				"         brew upgrade git  (macOS)\n" +
				"         apt install git   (Debian/Ubuntu — may need backports)",
			Fix: nil, // report-only per D-03
		},
	}
}
