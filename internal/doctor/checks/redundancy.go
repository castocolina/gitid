package checks

import (
	"fmt"
	"strings"

	"github.com/castocolina/gitid/internal/doctor"
)

// globalDirectives is the set of SSH directives that appear at global scope
// (root-level or under a "Host *" stanza) and are tracked for duplicate
// detection. Detection is case-insensitive.
var globalDirectives = []string{"usekeychain", "addkeystoagent", "ignoreunknown"}

// globalScan holds the results of scanning the full ~/.ssh/config content for
// redundancy indicators.
type globalScan struct {
	// hostStarCount is the number of "Host *" stanzas found in the file.
	hostStarCount int
	// directiveCounts maps a lowercased directive name to the number of times
	// it appears at global scope (root-level OR under any "Host *" stanza).
	// Only the three global directives (UseKeychain, AddKeysToAgent,
	// IgnoreUnknown) are counted.
	directiveCounts map[string]int
}

// scanGlobalDirectives parses content line-by-line to count "Host *" stanzas
// and track occurrences of the three global directives at global scope (either
// root-level or nested under a "Host *" stanza). Directives under named-host
// stanzas (e.g. "Host github.com") are NOT counted as global scope.
//
// Sentinel comments (# BEGIN / # END gitid managed: ...) are skipped
// transparently — the scan works across the whole file including managed blocks.
//
// Returns a globalScan with the aggregated counts.
func scanGlobalDirectives(content []byte) globalScan {
	result := globalScan{
		directiveCounts: make(map[string]int),
	}
	if len(content) == 0 {
		return result
	}

	// inGlobalHostStar tracks whether the current line is inside a "Host *"
	// stanza (so nested directives count as global scope).
	inGlobalHostStar := false

	for _, rawLine := range strings.Split(string(content), "\n") {
		line := strings.TrimSpace(rawLine)

		// Skip blank lines and comment lines (including sentinel markers).
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		upper := strings.ToUpper(line)

		// Detect the start of a new Host stanza.
		if strings.HasPrefix(upper, "HOST ") {
			rest := strings.TrimSpace(line[len("Host "):])
			if rest == "*" {
				// "Host *" — a global wildcard stanza.
				result.hostStarCount++
				inGlobalHostStar = true
			} else {
				// Named-host stanza (e.g. "Host github.com") — not global scope.
				inGlobalHostStar = false
			}
			continue
		}

		// Check for root-level directive (no leading whitespace) or a directive
		// nested under "Host *" (leading whitespace is expected).
		//
		// Root-level means the raw line does NOT start with whitespace. The
		// rawLine (pre-TrimSpace) determines indent level.
		isRootLevel := len(rawLine) > 0 && rawLine[0] != ' ' && rawLine[0] != '\t'

		if isRootLevel || inGlobalHostStar {
			// Check whether this line starts with one of the tracked directives.
			for _, dir := range globalDirectives {
				// Directive lines may optionally carry a value after whitespace.
				// Match the directive name at the start of the trimmed line,
				// followed by whitespace or end-of-line.
				lineUp := strings.ToUpper(line)
				if strings.HasPrefix(lineUp, strings.ToUpper(dir)) {
					// Ensure the match is a whole-word prefix (not a substring).
					rest := line[len(dir):]
					if rest == "" || rest[0] == ' ' || rest[0] == '\t' {
						result.directiveCounts[dir]++
					}
				}
			}
		}
	}

	return result
}

// CheckRedundancy detects SSH-config structural redundancy across the ENTIRE
// ~/.ssh/config (pre-existing unmanaged content AND gitid's managed _global
// block) and returns advisory findings for:
//
//  1. Multiple "Host *" stanzas (count > 1) — one finding.
//  2. Duplicate global directives among UseKeychain, AddKeysToAgent,
//     IgnoreUnknown — where the same directive appears more than once at global
//     scope (root-level OR under any "Host *"). Each duplicated directive
//     produces one finding.
//
// All findings: Family=FamilyRedundancy, Severity=SeverityWarning, Fix=nil.
// This is strictly advisory — it NEVER blocks the doctor or any write flow
// (T-05.7-11-01 / UAT G-4 / SSH-03). No auto-fix (T-05.7-11-02).
//
// An empty or missing config returns nil (best-effort, no error surface).
func CheckRedundancy(deps doctor.Deps) []doctor.Finding {
	if deps.ReadFile == nil {
		return nil
	}
	content, err := deps.ReadFile(deps.SSHConfigPath)
	if err != nil || len(content) == 0 {
		// Best-effort: treat a missing or unreadable config as clean.
		return nil
	}

	scan := scanGlobalDirectives(content)

	var findings []doctor.Finding

	// Finding 1: multiple "Host *" stanzas.
	if scan.hostStarCount > 1 {
		findings = append(findings, doctor.Finding{
			Family:   doctor.FamilyRedundancy,
			Severity: doctor.SeverityWarning,
			Title: fmt.Sprintf(
				"multiple \"Host *\" stanzas found (%d) in ~/.ssh/config",
				scan.hostStarCount,
			),
			Explanation: fmt.Sprintf(
				"~/.ssh/config contains %d \"Host *\" stanzas. "+
					"SSH evaluates Host patterns in file order; multiple global stanzas "+
					"can cause unexpected directive resolution. One \"Host *\" stanza "+
					"(gitid's managed _global block) is sufficient. "+
					"The extra stanzas likely include directives that duplicate those "+
					"already set in gitid's managed _global block.",
				scan.hostStarCount,
			),
			SuggestedFix: "Consolidate all global SSH directives (UseKeychain, AddKeysToAgent, " +
				"IgnoreUnknown) into a single \"Host *\" stanza — gitid's managed _global block " +
				"already contains the correct set. Remove any hand-written \"Host *\" blocks " +
				"that duplicate it.",
			Fix: nil, // advisory only — no destructive auto-fix (T-05.7-11-02)
		})
	}

	// Findings 2+: duplicate individual global directives.
	// Emit findings in a stable order matching the globalDirectives slice.
	for _, dir := range globalDirectives {
		count := scan.directiveCounts[dir]
		if count > 1 {
			// Reconstruct the canonical casing for the directive name from the
			// globalDirectives slice (UseKeychain, AddKeysToAgent, IgnoreUnknown).
			canonical := canonicalDirectiveName(dir)
			findings = append(findings, doctor.Finding{
				Family:   doctor.FamilyRedundancy,
				Severity: doctor.SeverityWarning,
				Title: fmt.Sprintf(
					"duplicate global directive %q appears %d times in ~/.ssh/config",
					canonical, count,
				),
				Explanation: fmt.Sprintf(
					"The global SSH directive %q is set %d times at global scope "+
						"(root-level or under \"Host *\") across ~/.ssh/config. "+
						"This spans both hand-written content and gitid's managed _global block. "+
						"Only the last occurrence takes effect, which may hide "+
						"intent or cause confusion.",
					canonical, count,
				),
				SuggestedFix: fmt.Sprintf(
					"Keep %q only in gitid's managed _global \"Host *\" block and "+
						"remove all other occurrences from hand-written sections of ~/.ssh/config.",
					canonical,
				),
				Fix: nil, // advisory only — no destructive auto-fix (T-05.7-11-02)
			})
		}
	}

	return findings
}

// canonicalDirectiveName returns the standard mixed-case spelling of a
// directive name given its lowercased form. Falls back to the input if unknown.
func canonicalDirectiveName(lower string) string {
	switch lower {
	case "usekeychain":
		return "UseKeychain"
	case "addkeystoagent":
		return "AddKeysToAgent"
	case "ignoreunknown":
		return "IgnoreUnknown"
	default:
		return lower
	}
}
