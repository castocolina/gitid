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
	if row.State != StateUnclaimed {
		t.Errorf("State should be StateUnclaimed when probe errored, got %v", row.State)
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
	if row.State != StateUnclaimed {
		t.Errorf("State should be StateUnclaimed when probe errored, got %v", row.State)
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
