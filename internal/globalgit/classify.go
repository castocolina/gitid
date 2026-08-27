package globalgit

import (
	"strings"
)

// SourceClass describes where an option's effective value came from. It is
// carried as a SEPARATE field from the state (D-03 decision), because plan
// 07-03 renders them into different labels and must not have to re-derive
// either from the other. Following globalssh/classify.go's approach exactly:
// source class is separate from state.
type SourceClass int

const (
	// SourceUnset means the option was not set anywhere — git is using its
	// own compiled-in default.
	SourceUnset SourceClass = iota
	// SourceSetByGitid means the effective value's origin resolves to the
	// include'd baseline file gitid manages.
	SourceSetByGitid
	// SourceSetByUser means the effective value was set by the user in a
	// file gitid does not own (origin is a different file path).
	SourceSetByUser
	// SourceUnchangeable means the effective value came from a scope gitid
	// cannot change: a non-file: origin (e.g. "command line", "blob:<sha>",
	// "standard input") or the system-wide config.
	SourceUnchangeable
)

// OptionRowState describes the actionable state of one option row. Following
// globalssh/classify.go's branch order: decide by VALUE first, by source
// second.
type OptionRowState int

const (
	// StateNeedsAction is the ZERO value: the option is unset or differs from
	// the recommendation, and gitid could help.
	StateNeedsAction OptionRowState = iota
	// StateAlreadySet means the effective value equals the recommendation.
	StateAlreadySet
	// StateSetButDiffers means the option is set somewhere to a
	// non-recommended value — a deliberate or external choice that is
	// flagged informational (D-02 word state, never blocking).
	StateSetButDiffers
	// StateUnclaimed means a probe failure prevented classification — no
	// state claim can be made. ProbeError on the row carries the details.
	StateUnclaimed
)

// OptionRow is the classified output for one option. Both State and Source
// are carried as separate fields so the renderer (plan 07-03) can derive
// independent label strings from them without re-deriving either.
type OptionRow struct {
	// Key is the canonical display key (e.g. "init.defaultBranch").
	Key string
	// CurrentValue is the effective value git reported, or empty if unset.
	CurrentValue string
	// Recommended is the policy's recommended value.
	Recommended string
	// GitDefault is the policy's git built-in default to name when unset.
	GitDefault string
	// EffectiveOrigin is the origin git reported for the effective value.
	EffectiveOrigin string
	// State is the row's actionable state.
	State OptionRowState
	// Source is the provenance class.
	Source SourceClass
	// ProbeError is non-empty when a probe failure prevented classification.
	// When non-empty, State == StateUnclaimed.
	ProbeError string
}

// Statuses is the package-level entry point: it runs both probes through deps
// and classifies every policy option. This is the exported function
// cmd/gitid/wiring.go calls to populate GlobalGitOptionStates.
//
// baselineFilePath is the resolved path of the include'd baseline file gitid
// owns — the classifier compares effective origins against it to decide
// set-by-gitid provenance. It must not know how the path was resolved.
func Statuses(deps Deps, baselineFilePath string) ([]OptionRow, error) {
	effective, effectiveErr := effectiveProbe(deps)
	var effectiveErrStr string
	if effectiveErr != nil {
		effectiveErrStr = effectiveErr.Error()
		effective = nil
	}

	inFile, inFileErr := inFileProbe(deps, baselineFilePath)
	var inFileErrStr string
	if inFileErr != nil {
		inFileErrStr = inFileErr.Error()
		inFile = nil
	}

	return ClassifyWithErrors(Policy, effective, inFile, baselineFilePath, effectiveErrStr, inFileErrStr)
}

// Classify classifies each policy option from the two probe results. It is
// exported so cmd/gitid/wiring.go can call it directly when it already has
// the probe results.
//
// Following globalssh/classify.go's branch order: decide by VALUE first, by
// source second.
func Classify(
	policies []OptionPolicy,
	effective map[string]EffectiveEntry,
	inFile map[string]EffectiveEntry,
	baselineFilePath string,
) ([]OptionRow, error) {
	return ClassifyWithErrors(policies, effective, inFile, baselineFilePath, "", "")
}

// ClassifyWithErrors is the internal classifier. effectiveProbeErr and
// inFileProbeErr carry any probe failures; a non-empty string means that probe
// failed and its evidence is unavailable for this row. A probe failure
// degrades only the rows that depended on that probe — the row carries the
// failure as ProbeError and makes no state claim (StateUnclaimed), while rows
// whose evidence came from the other probe are unaffected.
func ClassifyWithErrors(
	policies []OptionPolicy,
	effective map[string]EffectiveEntry,
	inFile map[string]EffectiveEntry,
	baselineFilePath string,
	effectiveProbeErr string,
	inFileProbeErr string,
) ([]OptionRow, error) {
	rows := make([]OptionRow, 0, len(policies))
	for _, policy := range policies {
		row := classifyOne(policy, effective, inFile, baselineFilePath, effectiveProbeErr, inFileProbeErr)
		rows = append(rows, row)
	}
	return rows, nil
}

// classifyOne classifies a single option policy following globalssh's branch
// order: VALUE first, source second. This ensures that a value equal to the
// recommendation is already-set whatever set it — the source class only
// decides the WORDS in the rendered label.
func classifyOne(
	policy OptionPolicy,
	effective map[string]EffectiveEntry,
	inFile map[string]EffectiveEntry,
	baselineFilePath string,
	effectiveProbeErr string,
	inFileProbeErr string,
) OptionRow {
	row := OptionRow{
		Key:         policy.Key,
		Recommended: policy.Recommended,
		GitDefault:  policy.GitDefault,
	}

	// Both probes failed — cannot classify.
	if effectiveProbeErr != "" && inFileProbeErr != "" {
		row.State = StateUnclaimed
		row.ProbeError = "effective: " + effectiveProbeErr + "; in-file: " + inFileProbeErr
		return row
	}

	// Individual probe failure: the row carries the error from whichever
	// probe failed (we need BOTH to classify confidently).
	if effectiveProbeErr != "" {
		row.State = StateUnclaimed
		row.ProbeError = "effective probe: " + effectiveProbeErr
		return row
	}
	if inFileProbeErr != "" {
		row.State = StateUnclaimed
		row.ProbeError = "in-file probe: " + inFileProbeErr
		return row
	}

	// Both probes succeeded. Look up the effective value.
	lk := strings.ToLower(policy.Key)
	effEntry, effPresent := effective[lk]
	_, inFilePresent := inFile[lk]

	if !effPresent {
		// Not set anywhere — needs action, source unset.
		row.State = StateNeedsAction
		row.Source = SourceUnset
		return row
	}

	row.CurrentValue = effEntry.Value
	row.EffectiveOrigin = effEntry.Origin

	// Decide source class.
	row.Source = sourceClassFor(effEntry, baselineFilePath, inFilePresent)

	// Branch by VALUE first (globalssh/classify.go's order: decide by value
	// first, by source second — stated in the doc comment there).
	if strings.EqualFold(effEntry.Value, policy.Recommended) {
		row.State = StateAlreadySet
		return row
	}

	// Value is not the recommendation. Source determines the state.
	// A value at a scope gitid cannot change → set-but-differs.
	// A value set by the user in their own config → set-but-differs.
	// A value set by gitid itself but not matching → treat as needs-action
	// (this means gitid wrote an old value; the write will correct it).
	if row.Source == SourceUnchangeable {
		row.State = StateSetButDiffers
		return row
	}
	if row.Source == SourceSetByUser {
		row.State = StateSetButDiffers
		return row
	}

	// Source is gitid or unset but value differs from recommendation.
	row.State = StateNeedsAction
	return row
}

// sourceClassFor determines the SourceClass from the effective entry, the
// managed baseline path, and whether the key is physically in the baseline file.
//
// The non-file: origin check must come first: "command line", "blob:<sha>",
// "standard input" are all scopes gitid cannot change, and the caller must
// never be told they are "set by gitid" because the path comparison would be
// meaningless.
func sourceClassFor(entry EffectiveEntry, baselineFilePath string, inFilePresent bool) SourceClass {
	// Non-file origin: gitid cannot change this scope.
	if !looksLikeFilePath(entry.Origin) {
		return SourceUnchangeable
	}
	// System scope: gitid cannot change the system config.
	if entry.Scope == "system" {
		return SourceUnchangeable
	}
	// Origin matches the gitid-managed baseline file.
	if entry.Origin == baselineFilePath {
		return SourceSetByGitid
	}
	// Also treat as set-by-gitid when the key is in the baseline file AND
	// the effective origin resolves there (covers tilde-expanded paths).
	if inFilePresent && entry.Origin == baselineFilePath {
		return SourceSetByGitid
	}
	return SourceSetByUser
}

// looksLikeFilePath returns true when the origin looks like a file path rather
// than a non-file source like "command line", "standard input", or "blob:<sha>".
// Git's documented non-file origins are all multi-word strings or start with
// "blob:" — a simple "/" prefix check is sufficient for Unix paths; we also
// accept any origin that does not contain a space and is not empty, which
// handles absolute paths on all platforms.
func looksLikeFilePath(origin string) bool {
	if origin == "" {
		return false
	}
	// Known non-file origins all contain a space or start with "blob:".
	if strings.HasPrefix(origin, "blob:") {
		return false
	}
	if strings.ContainsRune(origin, ' ') {
		return false
	}
	return true
}
