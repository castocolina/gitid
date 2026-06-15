package checks

import (
	"fmt"
	"os"
	"strings"

	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/identity"
)

// CheckCoherence checks that every managed identity's artifacts exist and resolve
// correctly (DOC-03, D-15). For each account in deps.Identities:
//
//  1. If KeyPath is set and Stat→ErrNotExist → error "IdentityFile <path> does not exist"
//     (report-only, no Fix — D-03).
//  2. If FragmentPath is set and Stat→ErrNotExist → error
//     "includeIf fragment <path> does not exist" (report-only, no Fix).
//  3. If the managed SSH Host block has IdentitiesOnly==false → error + Fix descriptor
//     (missing-wiring re-add class, D-02; fixer wired by Plan 05).
//  4. If RunGitConfigGet returns gpg.format != "ssh" → error finding (no Fix — D-17
//     locked-value carve-out; running 'git config --file <fragment> gpg.format ssh'
//     is the suggested action, but not auto-applied).
//  5. Read AllowedSignersPath via deps.ReadFile; search for a line whose first
//     whitespace-delimited field == account.GitEmail (byte-exact ==, Pitfall 6) AND
//     contains namespaces="git":
//     - Absent entry → error "no entry for <email>" + Fix descriptor (D-02).
//     - Found entry but first field != account.GitEmail (case-differing) → error
//     "email mismatch for identity <name>" + Fix descriptor (D-02).
//  6. If account.Incomplete != "" → Coherence finding describing the missing piece(s)
//     (Pitfall 5: Incomplete belongs in Coherence, NEVER Orphans, D-09).
//
// Only existence/resolution is checked — no content compare (D-15/D-19).
// The function never writes and does not import internal/filewriter (D-01).
func CheckCoherence(deps doctor.Deps) []doctor.Finding {
	var findings []doctor.Finding

	for _, acct := range deps.Identities {
		findings = append(findings, coherenceForAccount(deps, acct)...)
	}

	return findings
}

// coherenceForAccount runs all coherence checks for a single identity.Account.
func coherenceForAccount(deps doctor.Deps, acct identity.Account) []doctor.Finding {
	var findings []doctor.Finding

	// D-09/Pitfall 5: an Incomplete account means a managed block exists but
	// artifacts are missing — this is Coherence, NOT Orphans.
	// Report the incomplete marker and skip further checks that require a full set.
	if acct.Incomplete != "" {
		findings = append(findings, doctor.Finding{
			Family:   doctor.FamilyCoherence,
			Severity: doctor.SeverityError,
			Title: fmt.Sprintf(
				"identity %q: incomplete — missing %s", acct.Name, acct.Incomplete),
			Explanation: fmt.Sprintf(
				"The managed block for %q exists but one or more artifacts are missing: %s.",
				acct.Name, acct.Incomplete),
			SuggestedFix: "run 'gitid identity add' to recreate the missing artifacts",
			Fix:          nil, // report-only; user must re-run create
		})
		// Continue with any checks that can still run (e.g. KeyPath existence if set).
	}

	// Check 1: IdentityFile existence (DOC-03).
	if acct.KeyPath != "" {
		_, err := deps.Stat(acct.KeyPath) //nolint:gosec // acct.KeyPath is a trusted gitid-managed path (G304)
		if err != nil && os.IsNotExist(err) {
			findings = append(findings, doctor.Finding{
				Family:   doctor.FamilyCoherence,
				Severity: doctor.SeverityError,
				Title:    fmt.Sprintf("IdentityFile %s does not exist", acct.KeyPath),
				Explanation: fmt.Sprintf(
					"The SSH Host block for %q references a key file that is missing.", acct.Name),
				SuggestedFix: "run 'gitid identity add' to recreate, or remove the orphaned SSH Host block",
				Fix:          nil, // report-only (D-03)
			})
		}
	}

	// Check 2: includeIf fragment existence (DOC-03).
	if acct.FragmentPath != "" {
		_, err := deps.Stat(acct.FragmentPath) //nolint:gosec // acct.FragmentPath is a trusted gitid-managed path (G304)
		if err != nil && os.IsNotExist(err) {
			findings = append(findings, doctor.Finding{
				Family:   doctor.FamilyCoherence,
				Severity: doctor.SeverityError,
				Title:    fmt.Sprintf("includeIf fragment %s does not exist", acct.FragmentPath),
				Explanation: fmt.Sprintf(
					"The gitconfig includeIf for %q points to a missing fragment file.", acct.Name),
				SuggestedFix: "run 'gitid identity add' to recreate the fragment",
				Fix:          nil, // report-only (D-03)
			})
		}
	}

	// Check 3: IdentitiesOnly yes (DOC-03, D-15).
	// Look up the managed SSH Host block for this account by alias.
	if acct.Alias != "" {
		if hostInfo, ok := deps.ManagedHosts[acct.Name]; ok {
			if !hostInfo.IdentitiesOnly {
				alias := acct.Alias
				if hostInfo.Alias != "" {
					alias = hostInfo.Alias
				}
				findings = append(findings, doctor.Finding{
					Family:      doctor.FamilyCoherence,
					Severity:    doctor.SeverityError,
					Title:       fmt.Sprintf("Host %q: IdentitiesOnly yes missing", alias),
					Explanation: "Without IdentitiesOnly, SSH may use an unintended key for this host.",
					SuggestedFix: fmt.Sprintf(
						"re-run 'gitid identity add --name %s' (will repair the Host block)", acct.Name),
					Fix: &doctor.FixDescriptor{
						Summary: fmt.Sprintf("re-add IdentitiesOnly yes to Host block for %q", alias),
						// Fn is nil here; Plan 05 wires the actual AddWiring fixer.
						Fn: func() error { return nil },
					},
				})
			}
		}
	}

	// Checks 4+5 only apply when the fragment path exists (fragExists check).
	// We can still check gpg.format via RunGitConfigGet if the fragment path is known.
	if acct.FragmentPath == "" || acct.GitEmail == "" {
		return findings
	}

	// Check 4: gpg.format locked-value carve-out (D-17).
	// Use RunGitConfigGet to check the fragment's gpg.format value.
	// When err != nil the fragment may not exist or the key may be absent; skip
	// silently (already reported above if fragment is missing). When gpg.format !=
	// "ssh", report and early-return — the allowed_signers check assumes ssh signing.
	if deps.RunGitConfigGet != nil {
		gpgFmt, err := deps.RunGitConfigGet(acct.FragmentPath, "gpg.format")
		if err == nil && gpgFmt != "ssh" {
			findings = append(findings, doctor.Finding{
				Family:   doctor.FamilyCoherence,
				Severity: doctor.SeverityError,
				Title: fmt.Sprintf(
					"identity %q: gpg.format is %q (expected \"ssh\")", acct.Name, gpgFmt),
				Explanation: "Commit signing is misconfigured. Signing with an SSH key requires gpg.format=ssh.",
				SuggestedFix: fmt.Sprintf(
					"git config --file %s gpg.format ssh", acct.FragmentPath),
				Fix: nil, // locked-value override is report-only (D-17)
			})
			// If gpg.format is wrong, skip allowed_signers check — it isn't a signing
			// identity in the expected configuration.
			return findings
		}
	}

	// Check 5: allowed_signers line present and email byte-matches (DOC-03, D-17, Pitfall 6).
	// Only check when this is a gpg.format=ssh signing identity (gpg.format = "ssh" confirmed above).
	if deps.AllowedSignersPath == "" || deps.ReadFile == nil {
		return findings
	}

	signerBytes, err := deps.ReadFile(deps.AllowedSignersPath) //nolint:gosec // AllowedSignersPath is a trusted gitid-managed path (G304)
	if err != nil {
		if os.IsNotExist(err) {
			// No allowed_signers file at all → missing entry for this identity.
			findings = append(findings, allowedSignersMissingFinding(acct))
		}
		// Other errors: skip — cannot determine state; the system check (SignerFile
		// existence) will report it if needed.
		return findings
	}

	// Scan lines for a matching entry (exact first-field == email, Pitfall 6).
	foundLine, linePrincipal := findSignerLine(string(signerBytes), acct.GitEmail)
	switch {
	case !foundLine && linePrincipal == "":
		// No line at all with namespaces="git" whose first field equals email (byte-exact).
		findings = append(findings, allowedSignersMissingFinding(acct))
	case foundLine && linePrincipal != acct.GitEmail:
		// A namespaces="git" line was found but the principal does not byte-match
		// (case-differing). Pitfall 6: byte-exact == required.
		findings = append(findings, doctor.Finding{
			Family:   doctor.FamilyCoherence,
			Severity: doctor.SeverityError,
			Title: fmt.Sprintf(
				"allowed_signers: email mismatch for identity %q", acct.Name),
			Explanation: "The signing line email does not byte-match user.email. Signature verification will fail.",
			SuggestedFix: fmt.Sprintf(
				"correct the email in ~/.ssh/allowed_signers to exactly match '%s'", acct.GitEmail),
			Fix: &doctor.FixDescriptor{
				Summary: fmt.Sprintf(
					"correct allowed_signers email to match '%s'", acct.GitEmail),
				Fn: func() error { return nil }, // Plan 05 wires actual fixer
			},
		})
	}

	return findings
}

// allowedSignersMissingFinding returns the "no entry for <email>" Coherence finding.
func allowedSignersMissingFinding(acct identity.Account) doctor.Finding {
	return doctor.Finding{
		Family:   doctor.FamilyCoherence,
		Severity: doctor.SeverityError,
		Title:    fmt.Sprintf("allowed_signers: no entry for %s", acct.GitEmail),
		Explanation: fmt.Sprintf(
			"Signing identity %q has no line in ~/.ssh/allowed_signers. Commit signature verification will fail.",
			acct.Name),
		SuggestedFix: "add the line manually or re-run 'gitid identity add'",
		Fix: &doctor.FixDescriptor{
			Summary: fmt.Sprintf("add allowed_signers entry for '%s'", acct.GitEmail),
			Fn:      func() error { return nil }, // Plan 05 wires actual fixer
		},
	}
}

// findSignerLine scans the allowed_signers file content for a line that contains
// namespaces="git" and whose first whitespace-delimited field is the identity
// email. Returns (found bool, firstField string).
//
//   - found=true, firstField==email → exact match (all OK).
//   - found=true, firstField!=email → case-differing mismatch (email mismatch
//     finding, Pitfall 6).
//   - found=false, firstField="" → no entry at all (missing-entry finding).
//
// Byte-exact == is used for the email comparison (Pitfall 6).
func findSignerLine(content, email string) (found bool, firstField string) {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.Contains(line, `namespaces="git"`) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 1 {
			continue
		}
		principal := fields[0]
		// Byte-exact check (Pitfall 6 — must not use EqualFold).
		if principal == email {
			return true, principal
		}
		// Case-insensitive match → email mismatch (different bytes). Return the
		// mismatched principal so the caller can report the exact value.
		if strings.EqualFold(principal, email) {
			return true, principal
		}
	}
	return false, ""
}
