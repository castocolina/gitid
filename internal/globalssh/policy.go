package globalssh

import "strings"

// OptionPolicy is one row of the D-10 pinned recommendation table
// (06-CONTEXT.md D-10): the canonical key spelling gitid renders, gitid's
// recommended value, the risk the option carries when it is
// dangerous-by-default, the scope the recommendation may be written at
// ("global" = the gitid `Host *` block; "per-alias" = NEVER the `Host *`
// block — the recipe scopes IdentitiesOnly per-alias), the platform that may
// carry it ("all" or "darwin"), and the minimum OpenSSH version its
// recommended value requires (empty when there is no version gate).
type OptionPolicy struct {
	Key         string
	Recommended string
	Risk        string
	Scope       string
	Platform    string
	MinOpenSSH  string
}

// Policy is the D-10 approved table. Its Key order MUST match
// tuikit.GlobalSSHOptions' declaration order (the Options sub-tab renders rows
// in fixture order and the policy table backs each row); the alignment is
// pinned by a dedicated test.
//
// The table is DATA, not behavior: all six rows are declared now even though
// plan 06-01 only computes a correct option STATE for HashKnownHosts, because
// a half-table would quietly become a second source of truth about what gitid
// recommends (06-REVIEWS.md LOW finding, accepted as-is with this rationale).
var Policy = []OptionPolicy{
	{Key: "StrictHostKeyChecking", Recommended: "accept-new", Risk: "Medium", Scope: "global", Platform: "all", MinOpenSSH: "7.6"},
	{Key: "ForwardAgent", Recommended: "no", Risk: "High", Scope: "global", Platform: "all"},
	{Key: "HashKnownHosts", Recommended: "yes", Risk: "Low", Scope: "global", Platform: "all"},
	{Key: "IdentitiesOnly", Recommended: "yes", Risk: "High", Scope: "per-alias", Platform: "all"},
	{Key: "AddKeysToAgent", Recommended: "yes", Risk: "Low", Scope: "global", Platform: "all"},
	{Key: "UseKeychain", Recommended: "yes", Risk: "Low", Scope: "global", Platform: "darwin"},
}

// PolicyFor returns the approved row for key, case-insensitively, and whether
// it exists. Unknown keys are refused by name rather than silently dropped.
func PolicyFor(key string) (OptionPolicy, bool) {
	for _, p := range Policy {
		if strings.EqualFold(p.Key, key) {
			return p, true
		}
	}
	return OptionPolicy{}, false
}

// WritableToHostStar reports whether this recommendation may be written to
// gitid's Host * block. IdentitiesOnly is per-alias and must never be.
func (p OptionPolicy) WritableToHostStar() bool {
	return p.Scope != "per-alias"
}
