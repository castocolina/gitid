package globalgit

import (
	"strings"
)

// BundleResult is the D-09 bundle aggregate for one bundle row (the line-
// endings pair and the two sections: alias and color). It is re-derived from
// the two probes globalgit already takes — the effective value map and the
// physically-in-file map — replacing the retired conflict-scan contract (the
// intersect-and-compare rule is the same, the input is the probe results
// instead of a temp file plus a second `git config --file --list` call; the
// <authority> block documents the mechanism change).
type BundleResult struct {
	// Total is how many member keys the bundle manages.
	Total int
	// Set is how many members have an effective value at all.
	Set int
	// Differs is how many SET members differ from their recommendation.
	Differs int
	// DiffersKeys names each member whose own value differs from the
	// recommendation and therefore wins under floor + last-wins after an
	// apply (D-09). Presentation only — the block still contains every member
	// key, and these are the keys the detail pane's "your value wins" notes
	// name.
	DiffersKeys []string
}

// BundleFor computes BundleResult for a bundle policy row from the effective
// probe alone — a member is "set" exactly when it has an effective value, and
// "differs" exactly when that value does not equal the member's
// recommendation (case-insensitive, matching git's resolution).
func BundleFor(policy OptionPolicy, effective map[string]EffectiveEntry) BundleResult {
	res := BundleResult{Total: len(policy.Members)}
	for _, member := range policy.Members {
		lk := strings.ToLower(member.Key)
		entry, ok := effective[lk]
		if !ok {
			continue
		}
		res.Set++
		if !strings.EqualFold(entry.Value, member.Recommended) {
			res.Differs++
			res.DiffersKeys = append(res.DiffersKeys, member.Key)
		}
	}
	return res
}
