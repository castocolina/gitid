package globalgit

import (
	"path/filepath"
	"runtime"
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
	// StateNeedsAction is the ZERO value: the option is unset (or a bundle has
	// an unset member), and gitid could help.
	StateNeedsAction OptionRowState = iota
	// StateAlreadySet means the effective value equals the recommendation.
	StateAlreadySet
	// StateSetButDiffers means the option is set somewhere to a
	// non-recommended value — a deliberate or external choice that is
	// flagged informational (D-02 word state, never blocking). Differs rows
	// are NOT selectable: gitid's block sits at the floor, so a write into a
	// key the user set later is provably a no-op (D-02's documented reason is
	// on the state so the renderer never has to re-derive it).
	StateSetButDiffers
	// StateNotApplicable means the option does not apply on this machine —
	// a probe failure (no state claim, ReasonProbeFailed) or a reason the
	// NotApplicableReason enum names. The four states are the whole
	// vocabulary, mirroring globalssh exactly.
	StateNotApplicable
)

// NotApplicableReason distinguishes the situations that can share the
// not-applicable visual state so each one keeps its own words (the git-side
// analog of globalssh.NotApplicableReason).
type NotApplicableReason int

const (
	// ReasonNone is the zero value: the row is applicable.
	ReasonNone NotApplicableReason = iota
	// ReasonProbeFailed means a probe this row depends on returned an error,
	// so no state claim is possible.
	ReasonProbeFailed
)

// OptionRow is the classified output for one option. Both State and Source
// are carried as separate fields so the renderer (plan 07-03) can derive
// independent label strings from them without re-deriving either.
type OptionRow struct {
	// Key is the canonical display key (e.g. "init.defaultBranch").
	Key string
	// CurrentValue is the effective value git reported, or empty if unset. For
	// a bundle row it holds the first present member's value (the aggregate
	// summary lives in Bundle*).
	CurrentValue string
	// Recommended is the policy's recommended value (row-level display summary).
	Recommended string
	// GitDefault is the policy's git built-in default to name when unset.
	GitDefault string
	// EffectiveOrigin is the origin git reported for the effective value.
	EffectiveOrigin string
	// State is the row's actionable state.
	State OptionRowState
	// Source is the provenance class.
	Source SourceClass
	// NotApplicableReason is populated only when State == StateNotApplicable.
	NotApplicableReason NotApplicableReason
	// ProbeError is non-empty when a probe failure prevented classification.
	// When non-empty, State == StateNotApplicable with ReasonProbeFailed.
	ProbeError string
	// BundleSet is how many of the bundle's member keys are set at all.
	BundleSet int
	// BundleDiffers is how many of the set members differ from the
	// recommendation. It is the re-derivation of the retired ScanConflicts
	// intersect-and-compare rule against strictly better evidence (the probes),
	// per 07-03-PLAN.md's <authority> block.
	BundleDiffers int
	// BundleTotal is how many member keys the bundle manages (for the
	// aggregate's denominator).
	BundleTotal int
	// BundleDiffersKeys names each member whose OWN value differs from the
	// recommendation and therefore wins under floor + last-wins (D-09) — the
	// detail pane's "yours differs — yours wins" notes. Empty for scalar rows.
	BundleDiffersKeys []string
}

// Statuses is the package-level entry point: it runs both probes through deps
// and classifies every policy option. This is the exported function
// cmd/gitid/wiring.go calls to populate GlobalGitOptionStates.
//
// baselineFilePath is the resolved path of the include'd baseline file gitid
// owns — the classifier compares effective origins against it to decide
// set-by-gitid provenance. It must not know how the path was resolved.
func Statuses(deps Deps, baselineFilePath string) ([]OptionRow, error) {
	// The Options classifier cares only about the single effective value per
	// key (git's own last-wins resolution) — multi-value occurrence counts
	// are WR-04's Set-keys-screen concern (properties.go's AllSetKeys), not
	// this classifier's, so the second return value is discarded here.
	effective, _, effectiveErr := effectiveProbe(deps)
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
// failure as ProbeError and makes no state claim (StateNotApplicable with
// ReasonProbeFailed), while rows whose evidence came from the other probe are
// unaffected.
func ClassifyWithErrors(
	policies []OptionPolicy,
	effective map[string]EffectiveEntry,
	_ map[string]EffectiveEntry, // inFile: retained for API/caller stability; classifyOne no longer needs the map itself now that sourceClassFor compares paths directly (code review fix) — only inFileProbeErr's error signal still matters.
	baselineFilePath string,
	effectiveProbeErr string,
	inFileProbeErr string,
) ([]OptionRow, error) {
	rows := make([]OptionRow, 0, len(policies))
	for _, policy := range policies {
		row := classifyOne(policy, effective, baselineFilePath, effectiveProbeErr, inFileProbeErr)
		rows = append(rows, row)
	}
	return rows, nil
}

// classifyOne classifies a single option policy following globalssh's branch
// order: VALUE first, source second. This ensures that a value equal to the
// recommendation is already-set whatever set it — the source class only
// decides the WORDS in the rendered label.
//
// A row with exactly one member classifies exactly as the tracer did. A BUNDLE
// row (several members — the line-endings pair and the two sections) is
// needs-action when ANY member is unset (D-09/D-10: a write to that member is
// a real offer, whatever the others do), already-set only when every member is
// set to its recommendation, and set-but-differs otherwise. A row with NO
// members (the fallback-author pair, owned by plan 07-02's separate verb) is
// needs-action with no state claim — the view never offers it through the
// baseline apply.
func classifyOne(
	policy OptionPolicy,
	effective map[string]EffectiveEntry,
	baselineFilePath string,
	effectiveProbeErr string,
	inFileProbeErr string,
) OptionRow {
	row := OptionRow{
		Key:         policy.Key,
		Recommended: policy.Recommended,
	}

	// Both probes failed — cannot classify.
	if effectiveProbeErr != "" && inFileProbeErr != "" {
		row.State = StateNotApplicable
		row.NotApplicableReason = ReasonProbeFailed
		row.ProbeError = "effective: " + effectiveProbeErr + "; in-file: " + inFileProbeErr
		return row
	}
	// Individual probe failure: the row carries the error from whichever
	// probe failed (we need BOTH to classify confidently).
	if effectiveProbeErr != "" {
		row.State = StateNotApplicable
		row.NotApplicableReason = ReasonProbeFailed
		row.ProbeError = "effective probe: " + effectiveProbeErr
		return row
	}
	if inFileProbeErr != "" {
		row.State = StateNotApplicable
		row.NotApplicableReason = ReasonProbeFailed
		row.ProbeError = "in-file probe: " + inFileProbeErr
		return row
	}

	if len(policy.Members) == 0 {
		// Unset IS the recipes default for the fallback-author row
		// ("left unset unless explicitly opted in") — not a warning.
		row.State = StateAlreadySet
		row.Source = SourceUnset
		return row
	}

	// Single-member (scalar) row — the tracer's exact path.
	if len(policy.Members) == 1 {
		member := policy.Members[0]
		row.GitDefault = member.GitDefault
		lk := strings.ToLower(member.Key)
		effEntry, effPresent := effective[lk]

		if !effPresent {
			row.Source = SourceUnset
			row.GitDefault = effectiveGitDefault(member)
			if unsetMatchesRecommendation(member) {
				row.State = StateAlreadySet
				return row
			}
			row.State = StateNeedsAction
			return row
		}

		row.CurrentValue = effEntry.Value
		row.EffectiveOrigin = effEntry.Origin
		row.Source = sourceClassFor(effEntry, baselineFilePath)

		if strings.EqualFold(effEntry.Value, member.Recommended) {
			row.State = StateAlreadySet
			return row
		}
		if row.Source == SourceUnchangeable || row.Source == SourceSetByUser {
			row.State = StateSetButDiffers
			return row
		}
		// Source is gitid or unset but value differs from recommendation.
		row.State = StateNeedsAction
		return row
	}

	// Bundle row (several members): aggregate across members. Any member
	// unset makes the row needs-action — a write there is a real offer. A
	// member set to a different value is STILL written (the block contains
	// every member key per D-09), but it never alone lifts the row above
	// needs-action. Only when every member is already its recommendation is
	// the row already-set.
	bundle := BundleFor(policy, effective)
	row.BundleTotal, row.BundleSet, row.BundleDiffers = bundle.Total, bundle.Set, bundle.Differs
	row.BundleDiffersKeys = bundle.DiffersKeys
	anyUnset := bundle.Set < bundle.Total
	anyDiffers := bundle.Differs > 0
	allSetEqual := !anyUnset && !anyDiffers
	hasPresent := bundle.Set > 0
	source := SourceUnset
	for _, member := range policy.Members {
		lk := strings.ToLower(member.Key)
		effEntry, effPresent := effective[lk]
		if !effPresent {
			continue
		}
		if row.CurrentValue == "" {
			row.CurrentValue = effEntry.Value
			row.EffectiveOrigin = effEntry.Origin
		}
		if row.GitDefault == "" {
			row.GitDefault = member.GitDefault
		}
		memberSource := sourceClassFor(effEntry, baselineFilePath)
		switch memberSource {
		case SourceUnchangeable:
			source = SourceUnchangeable
		case SourceSetByUser:
			if source != SourceUnchangeable {
				source = SourceSetByUser
			}
		case SourceSetByGitid:
			if source == SourceUnset {
				source = SourceSetByGitid
			}
		}
	}
	if !hasPresent {
		row.Source = SourceUnset
		allDefault := true
		for _, member := range policy.Members {
			if !unsetMatchesRecommendation(member) {
				allDefault = false
				break
			}
		}
		if allDefault && len(policy.Members) > 0 {
			row.State = StateAlreadySet
			return row
		}
		row.State = StateNeedsAction
		return row
	}
	row.Source = source
	switch {
	case anyUnset:
		row.State = StateNeedsAction
	case allSetEqual:
		row.State = StateAlreadySet
	case anyDiffers:
		row.State = StateSetButDiffers
	default:
		row.State = StateNeedsAction
	}
	return row
}

func ignorecaseGitDefault() string {
	switch runtime.GOOS {
	case "windows", "darwin":
		return "true"
	default:
		return "false"
	}
}

func effectiveGitDefault(m MemberPolicy) string {
	if strings.EqualFold(m.Key, "core.ignorecase") {
		return ignorecaseGitDefault()
	}
	return m.GitDefault
}

func unsetMatchesRecommendation(m MemberPolicy) bool {
	def := effectiveGitDefault(m)
	return def != "" && strings.EqualFold(def, m.Recommended)
}

// sourceClassFor determines the SourceClass from the effective entry, the
// managed baseline path, and whether the key is physically in the baseline file.
//
// The non-file: origin check must come first: "command line", "blob:<sha>",
// "standard input" are all scopes gitid cannot change, and the caller must
// never be told they are "set by gitid" because the path comparison would be
// meaningless.
func sourceClassFor(entry EffectiveEntry, baselineFilePath string) SourceClass {
	// Non-file origin: gitid cannot change this scope.
	if !looksLikeFilePath(entry.Origin) {
		return SourceUnchangeable
	}
	// System scope: gitid cannot change the system config.
	if entry.Scope == "system" {
		return SourceUnchangeable
	}
	// Origin matches the gitid-managed baseline file. Compared via
	// filepath.Clean so a tilde-expanded/symlink-differing spelling of the
	// SAME path (e.g. baselineFilePath resolved through a different $HOME
	// representation than git's own --show-origin report) still matches —
	// code review found the original literal-only comparison made a SECOND
	// "covers tilde-expanded paths" branch below it byte-identical to this
	// one and therefore dead code, never actually implementing what its own
	// comment claimed. That second branch is deliberately NOT replaced with
	// an inFilePresent-only fallback: inFilePresent means the key exists
	// SOMEWHERE in the baseline file's own content, which is also true in
	// the ordinary set-but-differs case (gitid's floor value is present in
	// baseline, but the user's OWN separate file wins per git's last-wins
	// include order) — falling back to inFilePresent alone would misclassify
	// that differs case as SourceSetByGitid instead of SourceSetByUser.
	if filepath.Clean(entry.Origin) == filepath.Clean(baselineFilePath) {
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
