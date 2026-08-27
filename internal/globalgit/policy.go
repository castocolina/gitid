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

// MemberPolicy is one config key a row manages. Three rows manage more than
// one key (the line-endings pair and the two bundle sections) and one row
// manages none (the fallback author, owned by plan 07-02's separate verb), so
// the member SET — not one key — is the record's unit of composition.
type MemberPolicy struct {
	// Key is the config key as git reports it in --list output (lower-cased).
	// Lookups are case-insensitive so the canonical display spelling can be
	// stored here (e.g. "init.defaultBranch").
	Key string
	// Recommended is gitid's recommended value for this key, sourced verbatim
	// from recipes/gitconfig.recipe's ~/.gitconfig_default example.
	Recommended string
	// GitDefault is git's own built-in default when the key is unset. Empty
	// means git has no documented default worth naming (an alias or a key
	// whose unset behavior equals the recommendation).
	GitDefault string
	// Note records a researched quirk the render layer's copy re-states. For
	// core.eol it documents that git ignores the key while core.autocrlf is
	// "input" — and that gitid writes it anyway because it documents intent.
	Note string
}

// OptionPolicy is one row of the D-08 pinned recommendation table
// (07-CONTEXT.md D-08): the canonical display key gitid renders, the frozen
// CLI token (R-5), the row-level display recommendation, the member config
// keys the row manages with their values, an optional minimum git version,
// the gate kind distinguishing informational from hard gates, and the fallback
// value written when the hard gate is not met.
type OptionPolicy struct {
	// Key is the canonical DISPLAY key spelling. For three rows it is prose
	// containing spaces and slashes ("core.autocrlf / core.eol") and therefore
	// can never be an argv token — that is what Token is for (R-5). Git
	// lower-cases keys in --list output, so all lookups are case-insensitive.
	Key string
	// Token is the frozen CLI token plan 07-05's `options apply` accepts and
	// NOTHING else. It is a third identifier alongside the display key and the
	// member keys, carried here so the policy table stays the single authority.
	// The fallback-author row carries none: it is not an options-apply target.
	Token string
	// Recommended is the row-level display summary of the recommendation. For
	// a scalar row it equals the single member's value; for a bundle row it
	// summarises the whole section ("auto for all four").
	Recommended string
	// Members is the set of config keys this row manages. A bundle row's
	// selection applies every member as a unit; the fallback-author row has
	// none.
	Members []MemberPolicy
	// MinVersion is the minimum git version required for the recommended value
	// to be meaningful or, for the hard gate, to be writable (e.g. "2.35").
	// Empty means no version gate.
	MinVersion string
	// Gate distinguishes informational gates (harmless to write on old git,
	// which silently ignores an unknown KEY) from hard gates (old git ERRORS
	// at merge time on an unknown VALUE).
	Gate GateKind
	// Fallback is the value written when a hard gate is not met. merge.
	// conflictstyle declares "diff3" — accepted by every git that accepts
	// "zdiff3".
	Fallback string
}

// IsFallbackAuthor reports whether this row is the D-04 fallback-author pair —
// owned by plan 07-02's separate verb, never an options-apply target, never a
// baseline-block member.
func (p OptionPolicy) IsFallbackAuthor() bool {
	return len(p.Members) == 0
}

// IsBundle reports whether this row manages a whole section as a unit (D-09).
func (p OptionPolicy) IsBundle() bool {
	return len(p.Members) > 1
}

// memberFor resolves a member by its config key, case-insensitively, and
// whether it exists.
func (p OptionPolicy) memberFor(key string) (MemberPolicy, bool) {
	for _, m := range p.Members {
		if strings.EqualFold(m.Key, key) {
			return m, true
		}
	}
	return MemberPolicy{}, false
}

// lgFormatString is the alias.lg format string, byte-identical to
// recipes/gitconfig.recipe's ~/.gitconfig_default example. It lives in one
// place so the policy, the composer test, and the frozen block text cannot
// drift from the recipe that is their North Star (07-CONTEXT.md D-08).
const lgFormatString = "log --graph --pretty=format:'%Cred%h%Creset -%C(yellow)%d%Creset %s %Cgreen(%cr) %C(bold blue)<%an>%Creset' --abbrev-commit"

// Policy is the D-08 approved table. Its Key order MUST match
// tuikit.GlobalGitOptions' declaration order (the Options master-list renders
// rows in fixture order and the policy table backs each row); the alignment is
// pinned by a test. The order here is the frozen §4.5 display order, and the
// user.useConfigOnly row sits immediately after the fallback-author row per
// the 07-03-PLAN.md <authority> block (D-07 placement, binding for every
// row-index assertion in later plans).
//
// All alias and color values are VERBATIM from recipes/gitconfig.recipe's
// ~/.gitconfig_default example (the 8 aliases including the full lg format
// string, and the 4 color keys) — never paraphrased.
var Policy = []OptionPolicy{
	{
		Key:         "init.defaultBranch",
		Token:       "init.defaultBranch",
		Recommended: "main",
		Members: []MemberPolicy{
			{Key: "init.defaultBranch", Recommended: "main", GitDefault: "master"},
		},
		MinVersion: "2.28",
		Gate:       GateInformational,
	},
	{
		Key:         "core.ignorecase",
		Token:       "core.ignorecase",
		Recommended: "false",
		Members: []MemberPolicy{
			// git's built-in default on a case-insensitive filesystem is true —
			// the exact probe gitid's "false" recommendation is fighting.
			{Key: "core.ignorecase", Recommended: "false", GitDefault: "true"},
		},
	},
	{
		Key:         "core.autocrlf / core.eol",
		Token:       "core.lineEndings",
		Recommended: "input / lf",
		Members: []MemberPolicy{
			{Key: "core.autocrlf", Recommended: "input", GitDefault: "false"},
			{Key: "core.eol", Recommended: "lf", GitDefault: "native", Note: "core.eol is ignored by git while core.autocrlf is input; it is written anyway because it documents intent."},
		},
	},
	{
		// D-04/D-05: the fallback-author pair has its own dedicated ceremony,
		// never the baseline managed block. No members, no token (R-5).
		Key:         "user.email (global fallback)",
		Recommended: "left unset unless explicitly opted in",
	},
	//nolint:gosec // G101: the CLI token and recommended values are config identifiers, never credentials
	{
		Key:         "user.useConfigOnly",
		Token:       "user.useConfigOnly",
		Recommended: "true",
		Members: []MemberPolicy{
			{Key: "user.useConfigOnly", Recommended: "true", GitDefault: "false"},
		},
	},
	//nolint:gosec // G101: the CLI token and recommended values are config identifiers, never credentials
	{
		Key:         "push.autoSetupRemote",
		Token:       "push.autoSetupRemote",
		Recommended: "true",
		Members: []MemberPolicy{
			{Key: "push.autoSetupRemote", Recommended: "true", GitDefault: "false"},
		},
		MinVersion: "2.37",
		Gate:       GateInformational,
	},
	{
		Key:         "pull.rebase",
		Token:       "pull.rebase",
		Recommended: "true",
		Members: []MemberPolicy{
			{Key: "pull.rebase", Recommended: "true", GitDefault: "false"},
		},
	},
	{
		Key:         "fetch.prune",
		Token:       "fetch.prune",
		Recommended: "true",
		Members: []MemberPolicy{
			{Key: "fetch.prune", Recommended: "true", GitDefault: "false"},
		},
	},
	{
		Key:         "alias (8 shortcuts)",
		Token:       "alias",
		Recommended: "st, co, br, ci, df, lg, unstage, last",
		Members: []MemberPolicy{
			{Key: "alias.st", Recommended: "status"},
			{Key: "alias.co", Recommended: "checkout"},
			{Key: "alias.br", Recommended: "branch"},
			{Key: "alias.ci", Recommended: "commit"},
			{Key: "alias.df", Recommended: "diff"},
			{Key: "alias.lg", Recommended: lgFormatString},
			{Key: "alias.unstage", Recommended: "reset HEAD --"},
			{Key: "alias.last", Recommended: "log -1 HEAD"},
		},
	},
	{
		Key:         "color (ui/branch/diff/status)",
		Token:       "color",
		Recommended: "auto for all four",
		Members: []MemberPolicy{
			{Key: "color.ui", Recommended: "auto"},
			{Key: "color.branch", Recommended: "auto"},
			{Key: "color.diff", Recommended: "auto"},
			{Key: "color.status", Recommended: "auto"},
		},
	},
	{
		Key:         "merge.conflictstyle",
		Token:       "merge.conflictstyle",
		Recommended: "zdiff3",
		Members: []MemberPolicy{
			{Key: "merge.conflictstyle", Recommended: "zdiff3", GitDefault: "merge"},
		},
		MinVersion: "2.35",
		Gate:       GateHard,
		Fallback:   "diff3",
	},
	{
		Key:         "diff.colorMoved",
		Token:       "diff.colorMoved",
		Recommended: "zebra",
		Members: []MemberPolicy{
			{Key: "diff.colorMoved", Recommended: "zebra", GitDefault: "no"},
		},
		MinVersion: "2.15",
		Gate:       GateInformational,
	},
}

// PolicyFor returns the approved row for the DISPLAY key, matched
// case-insensitively (git lower-cases all keys in --list output), and whether
// it exists. Unknown keys are refused by name rather than silently dropped —
// this is the structural guard that prevents the render layer and the write
// authority from disagreeing about which keys exist (R-1, 07-01-PLAN.md).
func PolicyFor(key string) (OptionPolicy, bool) {
	for _, p := range Policy {
		if strings.EqualFold(p.Key, key) {
			return p, true
		}
	}
	return OptionPolicy{}, false
}

// PolicyForToken returns the approved row whose frozen CLI token equals
// token exactly (R-5: case-sensitive, never a fuzzy or member-key match).
// Rows with an empty Token (the fallback-author row) are not apply targets.
func PolicyForToken(token string) (OptionPolicy, bool) {
	for _, p := range Policy {
		if p.Token != "" && p.Token == token {
			return p, true
		}
	}
	return OptionPolicy{}, false
}

// TokenOwningMember returns the frozen CLI token of the row that manages
// member as a config key, matched case-insensitively. Used only to name the
// token a script should have typed when it passed a member key instead.
func TokenOwningMember(member string) (string, bool) {
	for _, p := range Policy {
		if p.Token == "" {
			continue
		}
		if _, ok := p.memberFor(member); ok {
			return p.Token, true
		}
	}
	return "", false
}
