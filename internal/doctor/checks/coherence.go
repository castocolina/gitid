package checks

import (
	"fmt"
	"os"
	"strings"

	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/identity"
	"github.com/castocolina/gitid/internal/sshconfig"
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

	findings = append(findings, checkHandWrittenIdentitiesOnly(deps)...)

	return findings
}

// coherenceForAccount runs all coherence checks for a single identity.Account.
// All returned findings have IdentityName set to acct.Name so the TUI badge
// derivation can scope them to the correct sidebar row (D-08).
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
			IdentityName: acct.Name,
			// D-01: this finding can describe an SSH-side gap
			// ("ssh-host-block") or a Git-side gap ("gitconfig-includeif-
			// block"/"fragment-file"); incompleteTarget resolves which
			// domain the missing piece(s) belong to.
			Target: incompleteTarget(acct.Incomplete),
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
				IdentityName: acct.Name,
				Target:       "SSH",
			})
		}
	}

	// Check 2: includeIf fragment existence (DOC-03). Skipped when the
	// Incomplete branch above already reported this exact gap
	// ("fragment-file", set by identity.Reconstruct's own readFrag/Missing
	// check) — otherwise a single missing fragment file produces TWO
	// Coherence findings for the identical root cause (08-01-PLAN.md Task 1
	// "never zero, never two" tracer contract, caught by manual real-fixture
	// verification: internal/identity/loader.go's readFrag already proves
	// the fragment is missing before this function ever runs).
	if acct.FragmentPath != "" && !strings.Contains(acct.Incomplete, "fragment-file") {
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
				IdentityName: acct.Name,
				Target:       "Git",
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
				// Wire the real fixer: re-add the SSH Host block (with IdentitiesOnly yes)
				// via deps.AddWiring(SSHConfigPath, acct.Name, "ssh-host:<alias>:<hostname>:<port>:<keyPath>").
				// The AddWiring dispatcher in the cmd layer calls sshconfig.Write which
				// ReplaceBlocks the managed block — idempotent and preserves foreign content.
				hostname := hostInfo.Hostname
				if hostname == "" {
					hostname = acct.Hostname
				}
				port := hostInfo.Port
				if port == 0 {
					port = acct.Port
				}
				keyPath := hostInfo.IdentityFile
				if keyPath == "" {
					keyPath = acct.KeyPath
				}
				sshLine := fmt.Sprintf("ssh-host:%s:%s:%d:%s", alias, hostname, port, keyPath)
				sshConfigPath := deps.SSHConfigPath
				acctName := acct.Name
				addWiring := deps.AddWiring
				// Build Fix only when AddWiring is wired; otherwise report-only.
				var fix *doctor.FixDescriptor
				if addWiring != nil && sshConfigPath != "" {
					fix = &doctor.FixDescriptor{
						Summary: fmt.Sprintf("re-add IdentitiesOnly yes to Host block for %q", alias),
						Fn: func() error {
							return addWiring(sshConfigPath, acctName, sshLine)
						},
					}
				}
				findings = append(findings, doctor.Finding{
					Family:      doctor.FamilyCoherence,
					Severity:    doctor.SeverityError,
					Title:       fmt.Sprintf("Host %q: IdentitiesOnly yes missing", alias),
					Explanation: "Without IdentitiesOnly, SSH may use an unintended key for this host.",
					SuggestedFix: fmt.Sprintf(
						"re-run 'gitid identity add --name %s' (will repair the Host block)", acct.Name),
					Fix:          fix,
					IdentityName: acct.Name,
					Target:       "SSH",
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
				Fix:          nil, // locked-value override is report-only (D-17)
				IdentityName: acct.Name,
				Target:       "Git",
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
			findings = append(findings, allowedSignersMissingFindingWithFix(deps, acct))
		}
		// Other errors: skip — cannot determine state; the system check (SignerFile
		// existence) will report it if needed.
		return findings
	}

	// Scan lines for a matching entry (exact first-field == email, Pitfall 6).
	// WR-01: findSignerLine scans ALL candidates — an exact match anywhere in the
	// file wins over a case-fold match earlier in the file (see findSignerLine).
	foundLine, linePrincipal := findSignerLine(string(signerBytes), acct.GitEmail)
	switch {
	case !foundLine && linePrincipal == "":
		// No line at all with namespaces="git" whose first field equals email (byte-exact).
		findings = append(findings, allowedSignersMissingFindingWithFix(deps, acct))
	case foundLine && linePrincipal != acct.GitEmail:
		// A namespaces="git" line was found but the principal does not byte-match
		// (case-differing). Pitfall 6: byte-exact == required.
		// Wire the fixer: re-add the signers entry with the correct email.
		fix := buildSignersFix(deps, acct, fmt.Sprintf(
			"correct allowed_signers email to match '%s'", acct.GitEmail))
		findings = append(findings, doctor.Finding{
			Family:   doctor.FamilyCoherence,
			Severity: doctor.SeverityError,
			Title: fmt.Sprintf(
				"allowed_signers: email mismatch for identity %q", acct.Name),
			Explanation: "The signing line email does not byte-match user.email. Signature verification will fail.",
			SuggestedFix: fmt.Sprintf(
				"correct the email in ~/.ssh/allowed_signers to exactly match '%s'", acct.GitEmail),
			Fix:          fix,
			IdentityName: acct.Name,
			// D-01: allowed_signers is a ~/.ssh/-domain artifact even though
			// it enables git signing — routes to SSH per D-01's "tool-level
			// findings route per tool" rule (the file the fix touches lives
			// under ~/.ssh/, not ~/.gitconfig).
			Target: "SSH",
		})
	}

	return findings
}

// incompleteTarget resolves the D-01 Target for an Incomplete finding from
// acct.Incomplete's comma-joined missing-piece markers (Reconstruct's
// "ssh-host-block" / "gitconfig-includeif-block" / "fragment-file"
// vocabulary, identity/loader.go). A missing SSH Host block is an SSH-domain
// gap; a missing gitconfig includeIf block or fragment file is a Git-domain
// gap. When both sides are missing (a genuinely incomplete identity with
// neither artifact yet), Git wins — the Coherence Git section is where a
// brand-new identity's setup gap is diagnosed first in the frozen field
// manifest's ordering (SSH section lists structural block presence; the
// combined "incomplete" state is fundamentally about the Git side never
// having been created).
func incompleteTarget(incomplete string) string {
	hasSSHGap := strings.Contains(incomplete, "ssh-host-block")
	hasGitGap := strings.Contains(incomplete, "gitconfig-includeif-block") || strings.Contains(incomplete, "fragment-file")
	if hasGitGap {
		return "Git"
	}
	if hasSSHGap {
		return "SSH"
	}
	return "Git" // unrecognized marker: default to Git rather than leave empty
}

// allowedSignersMissingFindingWithFix returns the "no entry for <email>" Coherence
// finding with a real Fix.Fn that calls deps.AddWiring to re-add the signer line.
// If the public key cannot be read, Fix is nil (cannot safely reconstruct the line).
func allowedSignersMissingFindingWithFix(deps doctor.Deps, acct identity.Account) doctor.Finding {
	fix := buildSignersFix(deps, acct, fmt.Sprintf("add allowed_signers entry for '%s'", acct.GitEmail))
	return doctor.Finding{
		IdentityName: acct.Name,
		Family:       doctor.FamilyCoherence,
		Severity:     doctor.SeverityError,
		Title:        fmt.Sprintf("allowed_signers: no entry for %s", acct.GitEmail),
		Explanation: fmt.Sprintf(
			"Signing identity %q has no line in ~/.ssh/allowed_signers. Commit signature verification will fail.",
			acct.Name),
		SuggestedFix: "add the line manually or re-run 'gitid identity add'",
		Fix:          fix,
		// D-01: allowed_signers lives under ~/.ssh/ — SSH per the "tool-level
		// findings route per tool" rule, matching the mismatch variant above.
		Target: "SSH",
	}
}

// buildSignersFix builds a FixDescriptor for re-adding or correcting an allowed_signers
// entry for the given account. It reads the public key via deps.ReadFile(acct.PubPath).
// Returns nil if no public key path is available or the read fails — in that case the
// finding is report-only (cannot safely reconstruct the signers line).
func buildSignersFix(deps doctor.Deps, acct identity.Account, summary string) *doctor.FixDescriptor {
	if acct.PubPath == "" || deps.ReadFile == nil || deps.AddWiring == nil || deps.AllowedSignersPath == "" {
		return nil // cannot build a signers line without the public key (D-03)
	}
	pubBytes, err := deps.ReadFile(acct.PubPath) //nolint:gosec // acct.PubPath is a trusted gitid-managed path (G304)
	if err != nil {
		return nil // public key unreadable — report-only (D-03)
	}
	pubLine := strings.TrimRight(string(pubBytes), "\n")
	email := acct.GitEmail
	allowedSignersPath := deps.AllowedSignersPath
	acctName := acct.Name
	addWiring := deps.AddWiring
	signerLine := fmt.Sprintf("signers:%s:%s", email, pubLine)
	return &doctor.FixDescriptor{
		Summary: summary,
		Fn: func() error {
			return addWiring(allowedSignersPath, acctName, signerLine)
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
// WR-01 all-candidate scan: the function scans ALL qualifying lines before
// returning. An exact byte-match anywhere in the file wins over a case-fold
// match earlier in the file. This prevents a case-differing line before the
// correct line from masking the exact match and emitting a spurious mismatch.
//
// Algorithm:
//  1. On exact match (principal == email) → return (true, principal) immediately.
//  2. On case-fold match → remember the first case-fold principal but CONTINUE
//     scanning (do not return yet — an exact match may follow).
//  3. After exhausting all lines → if an exact match was found, it was already
//     returned; if only a case-fold match was found, return (true, caseFoldPrincipal);
//     if neither → return (false, "").
//
// Byte-exact == is used for the final email comparison (Pitfall 6).
func findSignerLine(content, email string) (found bool, firstField string) {
	var caseFoldPrincipal string // first case-fold match; empty if none seen yet
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
			// Exact match found — return immediately; this is the authoritative result.
			return true, principal
		}
		// Case-insensitive match → remember first occurrence and keep scanning.
		// An exact match later in the file must win (WR-01).
		if caseFoldPrincipal == "" && strings.EqualFold(principal, email) {
			caseFoldPrincipal = principal
		}
	}
	if caseFoldPrincipal != "" {
		return true, caseFoldPrincipal
	}
	return false, ""
}

// checkHandWrittenIdentitiesOnly detects the D-09 flagship contradiction on a
// HAND-WRITTEN (non-gitid-managed) Host stanza: an explicit `IdentitiesOnly
// no` alongside a non-empty `IdentityFile`. This is a genuinely NEW check,
// distinct from coherenceForAccount's Check 3 above (which only covers
// gitid-MANAGED Host blocks via deps.ManagedHosts, keyed by reconstructed
// identity name) — a hand-written stanza has no identity.Account and no
// ManagedHosts entry, so it is otherwise invisible to Coherence.
//
// Every stanza whose ManagedBlockName is non-empty (gitid-managed) is
// skipped here — coherenceForAccount's Check 3 already handles that case,
// and double-reporting the same managed block would violate the "never
// zero, never two" contract (08-01's Task 1 tracer precedent).
//
// "Unset" IdentitiesOnly (the *bool is nil) is never conflated with
// "explicitly no": only IdentitiesOnly != nil && !*IdentitiesOnly trips this
// check, matching the plan's explicit instruction.
func checkHandWrittenIdentitiesOnly(deps doctor.Deps) []doctor.Finding {
	var findings []doctor.Finding
	for _, hb := range deps.AllHostBlocks {
		if hb.ManagedBlockName != "" {
			continue // gitid-managed — coherenceForAccount's Check 3 already covers it
		}
		if hb.IdentitiesOnly == nil || *hb.IdentitiesOnly || hb.IdentityFile == "" {
			continue
		}
		var fix *doctor.FixDescriptor
		if deps.SSHConfigPath != "" {
			hostPattern := hb.Pattern
			sshConfigPath := deps.SSHConfigPath
			fix = &doctor.FixDescriptor{
				Summary: fmt.Sprintf("set IdentitiesOnly yes on hand-written Host %q", hostPattern),
				Fn: func() error {
					_, err := sshconfig.ApplyVerifiedHostDirective(sshConfigPath, hostPattern, "IdentitiesOnly", "yes")
					return err
				},
			}
		}
		findings = append(findings, doctor.Finding{
			Family:   doctor.FamilyCoherence,
			Severity: doctor.SeverityError,
			Title:    fmt.Sprintf("IdentitiesOnly no contradicts an explicit IdentityFile on Host %q", hb.Pattern),
			Explanation: fmt.Sprintf(
				"Host %s sets IdentitiesOnly no while also naming IdentityFile %s -- ssh may still offer every other key it knows before falling back to the one explicitly configured.",
				hb.Pattern, hb.IdentityFile),
			SuggestedFix: fmt.Sprintf("Set IdentitiesOnly yes on the %s Host block -- available on the Fixer screen.", hb.Pattern),
			Fix:          fix,
			Target:       "SSH",
			Rewrite: &doctor.FixRewrite{
				HostPattern: hb.Pattern,
				Directive:   "IdentitiesOnly",
				NewValue:    "yes",
			},
		})
	}
	return findings
}
