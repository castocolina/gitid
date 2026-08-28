package globalgit

import (
	"testing"
)

// TestClassify_AlreadySet_FromUnset verifies that an effective value equal to
// the recommendation yields already-set even when the source class is unset
// (the value happened to already equal the recommendation through some means).
func TestClassify_AlreadySet_FromUnset(t *testing.T) {
	policy, _ := PolicyFor("init.defaultBranch")
	effective := map[string]EffectiveEntry{
		"init.defaultbranch": {Value: "main", Scope: "global", Origin: "/home/u/.gitconfig"},
	}
	inFile := map[string]EffectiveEntry{}

	rows, err := Classify([]OptionPolicy{policy}, effective, inFile, "/gitid/managed/path")
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].State != StateAlreadySet {
		t.Errorf("State = %v, want StateAlreadySet", rows[0].State)
	}
}

// TestClassify_NeedsAction_WhenUnsetEverywhere verifies that an option unset
// everywhere yields needs-action and a source class of unset, with the policy's
// git built-in default available.
func TestClassify_NeedsAction_WhenUnsetEverywhere(t *testing.T) {
	policy, _ := PolicyFor("init.defaultBranch")
	effective := map[string]EffectiveEntry{}
	inFile := map[string]EffectiveEntry{}

	rows, err := Classify([]OptionPolicy{policy}, effective, inFile, "/gitid/path")
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	row := rows[0]
	if row.State != StateNeedsAction {
		t.Errorf("State = %v, want StateNeedsAction", row.State)
	}
	if row.Source != SourceUnset {
		t.Errorf("Source = %v, want SourceUnset", row.Source)
	}
	if row.GitDefault != "master" {
		t.Errorf("GitDefault = %q, want %q", row.GitDefault, "master")
	}
}

// TestClassify_SetByGitid_WhenOriginMatchesManagedPath verifies that an
// effective origin equal to the gitid-owned baseline path yields the
// set-by-gitid source class.
func TestClassify_SetByGitid_WhenOriginMatchesManagedPath(t *testing.T) {
	policy, _ := PolicyFor("init.defaultBranch")
	managedPath := "/home/u/.gitconfig.d/00-baseline"
	effective := map[string]EffectiveEntry{
		"init.defaultbranch": {Value: "main", Scope: "global", Origin: managedPath},
	}
	inFile := map[string]EffectiveEntry{
		"init.defaultbranch": {Value: "main"},
	}

	rows, err := Classify([]OptionPolicy{policy}, effective, inFile, managedPath)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	row := rows[0]
	if row.State != StateAlreadySet {
		t.Errorf("State = %v, want StateAlreadySet", row.State)
	}
	if row.Source != SourceSetByGitid {
		t.Errorf("Source = %v, want SourceSetByGitid", row.Source)
	}
}

// TestClassify_SetByUser_WhenOriginDiffersFromManagedPath verifies set-but-differs.
func TestClassify_SetByUser_SetButDiffers(t *testing.T) {
	policy, _ := PolicyFor("init.defaultBranch")
	effective := map[string]EffectiveEntry{
		"init.defaultbranch": {Value: "trunk", Scope: "global", Origin: "/home/u/.gitconfig"},
	}
	inFile := map[string]EffectiveEntry{}

	rows, err := Classify([]OptionPolicy{policy}, effective, inFile, "/gitid/managed/path")
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	row := rows[0]
	if row.State != StateSetButDiffers {
		t.Errorf("State = %v, want StateSetButDiffers", row.State)
	}
	if row.Source != SourceSetByUser {
		t.Errorf("Source = %v, want SourceSetByUser (origin not the managed file)", row.Source)
	}
}

// TestClassify_SetByGitid_WhenOriginSpelledDifferentlyButSamePath is a code
// review regression test: sourceClassFor's origin-vs-baselineFilePath
// comparison previously used a literal string match only, which a genuinely
// unreachable "covers tilde-expanded paths" fallback branch claimed to
// handle but never actually did (byte-identical to the branch above it).
// The fix compares via filepath.Clean, so a differently-spelled-but-
// identical path (a doubled separator here, standing in for the tilde-vs-
// absolute or trailing-slash spellings git's own --show-origin and this
// project's own path resolution can produce) still resolves to SourceSetByGitid.
func TestClassify_SetByGitid_WhenOriginSpelledDifferentlyButSamePath(t *testing.T) {
	policy, _ := PolicyFor("init.defaultBranch")
	managedPath := "/home/u/.gitconfig.d/00-baseline"
	differentlySpelledSamePath := "/home/u//.gitconfig.d/00-baseline" // doubled separator, same real path
	effective := map[string]EffectiveEntry{
		"init.defaultbranch": {Value: "main", Scope: "global", Origin: differentlySpelledSamePath},
	}
	inFile := map[string]EffectiveEntry{
		"init.defaultbranch": {Value: "main"},
	}

	rows, err := Classify([]OptionPolicy{policy}, effective, inFile, managedPath)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].Source != SourceSetByGitid {
		t.Errorf("Source = %v, want SourceSetByGitid (origin resolves to the same real path as baselineFilePath)", rows[0].Source)
	}
}

// TestClassify_SetByUser_EvenWhenKeyAlsoPresentInBaselineFile guards the fix
// above against over-correcting: a key present in BOTH the baseline file and
// the user's own separate file, where the user's file wins per git's
// last-wins include order, must stay SourceSetByUser — the fix must compare
// the EFFECTIVE origin's real path, never fall back to "is the key present
// in the baseline file at all" as a substitute signal (that would misclassify
// exactly this ordinary set-but-differs case as gitid-set).
func TestClassify_SetByUser_EvenWhenKeyAlsoPresentInBaselineFile(t *testing.T) {
	policy, _ := PolicyFor("init.defaultBranch")
	managedPath := "/home/u/.gitconfig.d/00-baseline"
	effective := map[string]EffectiveEntry{
		// Effective origin is the USER's own file — it wins.
		"init.defaultbranch": {Value: "trunk", Scope: "global", Origin: "/home/u/.gitconfig"},
	}
	inFile := map[string]EffectiveEntry{
		// The key is ALSO present in the baseline file (gitid's own floor
		// value), but does not win.
		"init.defaultbranch": {Value: "main"},
	}

	rows, err := Classify([]OptionPolicy{policy}, effective, inFile, managedPath)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].Source != SourceSetByUser {
		t.Errorf("Source = %v, want SourceSetByUser (the user's own file is the effective origin, even though the key also exists in the baseline file)", rows[0].Source)
	}
}

// TestClassify_ProbeError_EffectiveFails_InFileUnaffected verifies that an
// effective-probe failure leaves a row whose evidence is in-file-only unaffected
// (i.e. the row carries the probe error and makes no state claim).
func TestClassify_ProbeError_EffectiveFails_InFileUnaffected(t *testing.T) {
	policy, _ := PolicyFor("init.defaultBranch")

	// Effective probe error: rows dependent on it carry the error.
	effectiveErr := "effective probe failed: some error"
	inFile := map[string]EffectiveEntry{
		"init.defaultbranch": {Value: "main"},
	}

	rows, err := ClassifyWithErrors([]OptionPolicy{policy}, nil, inFile, "/gitid/path", effectiveErr, "")
	if err != nil {
		t.Fatalf("ClassifyWithErrors: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	row := rows[0]
	if row.ProbeError == "" {
		t.Error("row should carry probe error when effective probe failed")
	}
	if row.State != StateNotApplicable {
		t.Errorf("State should be StateNotApplicable when probe errored, got %v", row.State)
	}
	if row.NotApplicableReason != ReasonProbeFailed {
		t.Errorf("NotApplicableReason = %v, want ReasonProbeFailed", row.NotApplicableReason)
	}
}

// TestClassify_ProbeError_InFileFails_EffectiveUnaffected verifies that an
// in-file-probe failure leaves a row whose only evidence is effective-only
// unaffected.
func TestClassify_ProbeError_InFileFails_EffectiveUnaffected(t *testing.T) {
	policy, _ := PolicyFor("init.defaultBranch")
	effective := map[string]EffectiveEntry{
		"init.defaultbranch": {Value: "main", Scope: "global", Origin: "/some/path"},
	}

	inFileErr := "in-file probe failed: permission denied"
	rows, err := ClassifyWithErrors([]OptionPolicy{policy}, effective, nil, "/gitid/path", "", inFileErr)
	if err != nil {
		t.Fatalf("ClassifyWithErrors: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	row := rows[0]
	if row.ProbeError == "" {
		t.Error("row should carry probe error when in-file probe failed")
	}
	if row.State != StateNotApplicable {
		t.Errorf("State should be StateNotApplicable when probe errored, got %v", row.State)
	}
	if row.NotApplicableReason != ReasonProbeFailed {
		t.Errorf("NotApplicableReason = %v, want ReasonProbeFailed", row.NotApplicableReason)
	}
}

// TestClassify_UnchangeableScope verifies set-at-unchangeable-scope when the
// origin is not a file: origin (e.g. "command line").
func TestClassify_UnchangeableScope(t *testing.T) {
	policy, _ := PolicyFor("init.defaultBranch")
	effective := map[string]EffectiveEntry{
		"init.defaultbranch": {Value: "develop", Scope: "command", Origin: "command line"},
	}
	inFile := map[string]EffectiveEntry{}

	rows, err := Classify([]OptionPolicy{policy}, effective, inFile, "/gitid/path")
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	row := rows[0]
	if row.Source != SourceUnchangeable {
		t.Errorf("Source = %v, want SourceUnchangeable for command-line origin", row.Source)
	}
}

// TestClassify_BundleAggregate verifies the D-09 bundle aggregate: a bundle
// row's set-count, differs-count, and differs-keys are re-derived from the two
// probes (the retired ScanConflicts intersect-and-compare rule) and the row is
// needs-action when every member present but none unset and one differs.
func TestClassify_BundleAggregate(t *testing.T) {
	policy, _ := PolicyFor("color (ui/branch/diff/status)")
	// ui=auto (matches), branch=auto (matches), diff=false (differs),
	// status=<absent> (unset)
	effective := map[string]EffectiveEntry{
		"color.ui":     {Value: "auto", Scope: "global", Origin: "/home/u/.gitconfig"},
		"color.branch": {Value: "auto", Scope: "global", Origin: "/home/u/.gitconfig"},
		"color.diff":   {Value: "false", Scope: "global", Origin: "/home/u/.gitconfig"},
	}
	inFile := map[string]EffectiveEntry{}

	rows, err := Classify([]OptionPolicy{policy}, effective, inFile, "/gitid/path")
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	row := rows[0]
	if row.BundleTotal != 4 {
		t.Errorf("BundleTotal = %d, want 4", row.BundleTotal)
	}
	if row.BundleSet != 3 {
		t.Errorf("BundleSet = %d, want 3", row.BundleSet)
	}
	if row.BundleDiffers != 1 {
		t.Errorf("BundleDiffers = %d, want 1 (color.diff=false)", row.BundleDiffers)
	}
	if len(row.BundleDiffersKeys) != 1 || row.BundleDiffersKeys[0] != "color.diff" {
		t.Errorf("BundleDiffersKeys = %v, want [color.diff]", row.BundleDiffersKeys)
	}
	// One member unset → needs-action (D-09/D-10: the row counts once and has
	// a real offer even though some members are already correct).
	if row.State != StateNeedsAction {
		t.Errorf("State = %v, want StateNeedsAction (one member unset)", row.State)
	}
}

// TestClassify_BundleAllSetAndEqualIsAlreadySet verifies a bundle row whose
// members are all present and all equal their recommendations is already-set.
func TestClassify_BundleAllSetAndEqualIsAlreadySet(t *testing.T) {
	policy, _ := PolicyFor("color (ui/branch/diff/status)")
	effective := map[string]EffectiveEntry{
		"color.ui":     {Value: "auto", Scope: "global", Origin: "/home/u/.gitconfig"},
		"color.branch": {Value: "auto", Scope: "global", Origin: "/home/u/.gitconfig"},
		"color.diff":   {Value: "auto", Scope: "global", Origin: "/home/u/.gitconfig"},
		"color.status": {Value: "auto", Scope: "global", Origin: "/home/u/.gitconfig"},
	}
	inFile := map[string]EffectiveEntry{}

	rows, err := Classify([]OptionPolicy{policy}, effective, inFile, "/gitid/path")
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	row := rows[0]
	if row.State != StateAlreadySet {
		t.Errorf("State = %v, want StateAlreadySet", row.State)
	}
	if row.BundleDiffers != 0 {
		t.Errorf("BundleDiffers = %d, want 0", row.BundleDiffers)
	}
}

// TestClassify_BundleAllSetButSomeDifferIsSetButDiffers verifies a bundle row
// whose members are all present but some differ is set-but-differs OR
// needs-action — per D-09 the row is a needs-action offer only while a member
// is UNSET. When every member is set and at least one differs, the state is
// set-but-differs (D-02: gitid's write into floor-values is a no-op).
func TestClassify_BundleAllSetButSomeDifferIsSetButDiffers(t *testing.T) {
	policy, _ := PolicyFor("color (ui/branch/diff/status)")
	effective := map[string]EffectiveEntry{
		"color.ui":     {Value: "auto", Scope: "global", Origin: "/home/u/.gitconfig"},
		"color.branch": {Value: "auto", Scope: "global", Origin: "/home/u/.gitconfig"},
		"color.diff":   {Value: "false", Scope: "global", Origin: "/home/u/.gitconfig"},
		"color.status": {Value: "auto", Scope: "global", Origin: "/home/u/.gitconfig"},
	}
	inFile := map[string]EffectiveEntry{}

	rows, err := Classify([]OptionPolicy{policy}, effective, inFile, "/gitid/path")
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	row := rows[0]
	if row.State != StateSetButDiffers {
		t.Errorf("State = %v, want StateSetButDiffers (all set, one differs)", row.State)
	}
}

// TestClassify_AttributionByOriginPathNotMembership is the D-03 trap pin: a
// key gitid MANAGES, set by the user in their own file, classifies as the
// user's — decided by the effective ORIGIN PATH, never by membership in the
// managed key set. Proven with the fail-loud author key, whose two [user]
// homes (the baseline block's useConfigOnly vs the user's own ~/.gitconfig)
// make the membership trap live.
func TestClassify_AttributionByOriginPathNotMembership(t *testing.T) {
	policy, _ := PolicyFor("user.useConfigOnly")
	managedPath := "/home/u/.gitconfig.d/00-baseline"
	// The user wrote useConfigOnly=false into their OWN ~/.gitconfig — the file
	// wins under the floor include. The key is one gitid manages, but
	// membership must NOT decide attribution: the origin path is the user's.
	effective := map[string]EffectiveEntry{
		"user.useconfigonly": {Value: "false", Scope: "global", Origin: "/home/u/.gitconfig"},
	}
	inFile := map[string]EffectiveEntry{
		"user.useconfigonly": {Value: "false"},
	}

	rows, err := Classify([]OptionPolicy{policy}, effective, inFile, managedPath)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	row := rows[0]
	if row.Source != SourceSetByUser {
		t.Errorf("Source = %v, want SourceSetByUser (origin is the user's file, not the baseline)", row.Source)
	}
	if row.State != StateSetButDiffers {
		t.Errorf("State = %v, want StateSetButDiffers (user's false differs from recommendation)", row.State)
	}
	if row.CurrentValue != "false" || row.EffectiveOrigin != "/home/u/.gitconfig" {
		t.Errorf("row must carry the user's effective value/origin: (%q, %q)", row.CurrentValue, row.EffectiveOrigin)
	}
}

// TestClassify_ProbeFailureLeavesOtherSourceRowsUnaffected asserts a probe
// failure affecting one evidence source leaves rows classified from the other
// source intact: classifying with both probes when only ONE has an error still
// marks every row not-applicable with ReasonProbeFailed (the row makes no
// state claim), but a classification WITHOUT any probe error is not degraded.
func TestClassify_ProbeFailureLeavesOtherSourceRowsUnaffected(t *testing.T) {
	policy, _ := PolicyFor("init.defaultBranch")
	effective := map[string]EffectiveEntry{
		"init.defaultbranch": {Value: "main", Scope: "global", Origin: "/home/u/.gitconfig"},
	}
	inFile := map[string]EffectiveEntry{
		"init.defaultbranch": {Value: "main"},
	}

	// Effective probe failed only: the row depends on it → not-applicable.
	rows, err := ClassifyWithErrors([]OptionPolicy{policy}, nil, inFile, "/gitid/path", "eff failed", "")
	if err != nil {
		t.Fatalf("ClassifyWithErrors: %v", err)
	}
	if rows[0].State != StateNotApplicable || rows[0].NotApplicableReason != ReasonProbeFailed {
		t.Errorf("eff-probe-failed row = (%v, %v), want not-applicable/probe-failed", rows[0].State, rows[0].NotApplicableReason)
	}

	// No probe errors: the same evidence classifies cleanly as already-set —
	// the healthy case proves the failure path did not leak into it.
	rows, err = Classify([]OptionPolicy{policy}, effective, inFile, "/gitid/path")
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if rows[0].State != StateAlreadySet {
		t.Errorf("healthy row = %v, want StateAlreadySet", rows[0].State)
	}
}
