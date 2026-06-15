package checks

import (
	"fmt"
	"strings"

	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
)

// OverlapPair names two identities whose match conditions overlap.
type OverlapPair struct {
	A, B   string // identity names (A is the earlier account in the input slice)
	Kind   string // "identical-gitdir", "nested-gitdir", "hasconfig-url"
	Detail string // human-readable description of the overlap condition
}

// DetectOverlaps returns raw overlap pairs across all three overlap kinds (D-14).
// Exported so cmd/gitid/add.go and update.go can call it at write time (D-16).
// Reserved blocks (IsReservedBlockName) and "_global" are filtered automatically
// to avoid spurious findings and to prevent the baseline-include infinite-fix loop
// bug class (Pitfall 4 / T-05.5-15).
func DetectOverlaps(accounts []identity.Account) []OverlapPair {
	// Filter out reserved and _global accounts — they have no active match
	// conditions and must never produce overlap findings.
	filtered := make([]identity.Account, 0, len(accounts))
	for _, a := range accounts {
		if gitconfig.IsReservedBlockName(a.Name) || a.Name == "_global" {
			continue
		}
		filtered = append(filtered, a)
	}

	var pairs []OverlapPair
	for i := 0; i < len(filtered); i++ {
		for j := i + 1; j < len(filtered); j++ {
			a, b := filtered[i], filtered[j]
			// Compare each combination of matches from a and b.
			for _, ma := range a.Matches {
				for _, mb := range b.Matches {
					if ma.Kind == gitconfig.MatchGitdir && mb.Kind == gitconfig.MatchGitdir {
						kind, detail := classifyGitdirOverlap(ma.Value, mb.Value)
						if kind != "" {
							pairs = append(pairs, OverlapPair{
								A:      a.Name,
								B:      b.Name,
								Kind:   kind,
								Detail: detail,
							})
						}
					}
					if ma.Kind == gitconfig.MatchHasconfig && mb.Kind == gitconfig.MatchHasconfig {
						if hasconfigOverlaps(ma.Value, mb.Value) {
							pairs = append(pairs, OverlapPair{
								A:    a.Name,
								B:    b.Name,
								Kind: "hasconfig-url",
								Detail: fmt.Sprintf(
									"hasconfig patterns %q and %q share a common URL prefix or one subsumes the other",
									ma.Value, mb.Value,
								),
							})
						}
					}
				}
			}
		}
	}
	return pairs
}

// CheckOverlap wraps DetectOverlaps as a doctor.Finding slice (D-15, SeverityWarning).
// Fix is always nil — overlaps are advisory and not auto-fixable (T-05.5-15).
// The Explanation names both identities and notes the last-written-wins semantics.
func CheckOverlap(deps doctor.Deps) []doctor.Finding {
	pairs := DetectOverlaps(deps.Identities)
	if len(pairs) == 0 {
		return nil
	}
	findings := make([]doctor.Finding, 0, len(pairs))
	for _, p := range pairs {
		findings = append(findings, doctor.Finding{
			Family:   doctor.FamilyOverlap,
			Severity: doctor.SeverityWarning,
			Title: fmt.Sprintf(
				"identities %q and %q have overlapping match conditions (%s)",
				p.A, p.B, p.Kind,
			),
			Explanation: fmt.Sprintf(
				"Identities %q and %q both match the same repository path or URL (%s). "+
					"Git evaluates [includeIf] blocks in file order and the last matching block wins. "+
					"The identity written most recently to ~/.gitconfig (%q) will take precedence. "+
					"Detail: %s",
				p.A, p.B, p.Kind, p.B, p.Detail,
			),
			SuggestedFix: fmt.Sprintf(
				"Narrow the match condition of %q or %q so they do not overlap "+
					"(e.g. use a more specific gitdir path or URL pattern).",
				p.A, p.B,
			),
			Fix: nil, // overlaps are advisory — no auto-fix (T-05.5-15)
		})
	}
	return findings
}

// classifyGitdirOverlap returns the kind and a human-readable detail string if
// two gitdir values overlap, or ("", "") if they do not.
func classifyGitdirOverlap(a, b string) (kind, detail string) {
	an := normGitdir(a)
	bn := normGitdir(b)
	if an == bn {
		return "identical-gitdir", fmt.Sprintf("both use gitdir %q", an)
	}
	if strings.HasPrefix(an, bn) || strings.HasPrefix(bn, an) {
		if strings.HasPrefix(an, bn) {
			return "nested-gitdir", fmt.Sprintf("gitdir %q is nested inside %q", an, bn)
		}
		return "nested-gitdir", fmt.Sprintf("gitdir %q is nested inside %q", bn, an)
	}
	return "", ""
}

// normGitdir normalises a gitdir value to always end with "/" for consistent
// prefix comparison (RESEARCH.md heuristic).
func normGitdir(s string) string {
	return strings.TrimSuffix(s, "/") + "/"
}

// hasconfigOverlaps reports whether two hasconfig URL patterns overlap using a
// conservative heuristic: equal patterns, one is a prefix of the other, or they
// share the same "git@<host>:" prefix with at least one using a wildcard that
// subsumes the other (D-14 — over-approximation is explicitly acceptable).
// Both values are expected in the internal format "remote.*.url:<pattern>".
func hasconfigOverlaps(a, b string) bool {
	pa := strings.TrimPrefix(a, "remote.*.url:")
	pb := strings.TrimPrefix(b, "remote.*.url:")

	if pa == pb {
		return true
	}
	// One is a prefix of the other.
	if strings.HasPrefix(pa, pb) || strings.HasPrefix(pb, pa) {
		return true
	}
	// Same git@<host>: prefix — conservative: treat same host as potentially overlapping.
	hostA := extractGitHost(pa)
	hostB := extractGitHost(pb)
	if hostA != "" && hostA == hostB {
		return true
	}
	return false
}

// extractGitHost returns the "git@<host>" prefix of a URL pattern, or "" if the
// pattern does not match that form.
func extractGitHost(pattern string) string {
	if !strings.HasPrefix(pattern, "git@") {
		return ""
	}
	rest := strings.TrimPrefix(pattern, "git@")
	colonIdx := strings.Index(rest, ":")
	if colonIdx < 0 {
		return ""
	}
	return "git@" + rest[:colonIdx]
}
