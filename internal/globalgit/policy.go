package globalgit

import (
	"strings"
)

// GateKind distinguishes the two ways a git version gate affects a
// recommendation (D-08, 07-CONTEXT.md).
type GateKind int

const (
	// GateNone means there is no version gate for this option.
	GateNone GateKind = iota
	// GateInformational means old git silently ignores an unknown KEY, so
	// writing is harmless even on older versions — the gate is advisory only.
	GateInformational
	// GateHard means old git ERRORS on an unknown VALUE, so the written value
	// must change to a safe fallback when the git version is below the minimum
	// (merge.conflictstyle is the only such row in D-08).
	GateHard
)

// OptionPolicy is one row of the D-08 pinned recommendation table
// (07-CONTEXT.md D-08): the canonical key spelling gitid renders (note: git
// lower-cases all keys in --list output, so matching is done
// case-insensitively), gitid's recommended value, the git built-in default to
// name when the key is unset, an optional minimum git version, and the gate
// kind distinguishing informational from hard gates.
type OptionPolicy struct {
	// Key is the canonical display key spelling (e.g. "init.defaultBranch").
	// Git lower-cases keys in --list output, so all lookups are case-insensitive.
	Key string
	// Recommended is gitid's recommended value for this key.
	Recommended string
	// GitDefault is the git built-in default to name when the key is unset
	// ("unset" state copy: "not set (git's built-in default: <GitDefault>)").
	// Empty means git has no documented built-in default to name.
	GitDefault string
	// MinVersion is the minimum git version required for the recommended value
	// to be meaningful (e.g. "2.28"). Empty means no version gate.
	MinVersion string
	// Gate distinguishes informational gates (harmless to write on old git)
	// from hard gates (old git errors on an unknown VALUE).
	Gate GateKind
}

// Policy is the D-08 approved table. Its Key order MUST match
// tuikit.GlobalGitOptions' declaration order (the Options master-list renders
// rows in fixture order and the policy table backs each row); the alignment is
// pinned by a test. For plan 07-01 only init.defaultBranch has a live entry —
// the remaining rows are registered here so the table is the single source of
// truth and a half-table cannot quietly become a second authoritative list.
//
// Plan 07-03 fills the remaining entries to the full D-08 set.
var Policy = []OptionPolicy{
	{
		Key:         "init.defaultBranch",
		Recommended: "main",
		GitDefault:  "master",
		MinVersion:  "2.28",
		Gate:        GateInformational,
	},
}

// PolicyFor returns the approved row for key, matched case-insensitively
// (git lower-cases all keys in --list output), and whether it exists. Unknown
// keys are refused by name rather than silently dropped — this is the
// structural guard that prevents the render layer and the write authority from
// disagreeing about which keys exist (R-1, 07-01-PLAN.md).
func PolicyFor(key string) (OptionPolicy, bool) {
	for _, p := range Policy {
		if strings.EqualFold(p.Key, key) {
			return p, true
		}
	}
	return OptionPolicy{}, false
}
