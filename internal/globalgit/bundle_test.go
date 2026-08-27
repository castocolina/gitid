package globalgit

import (
	"reflect"
	"testing"
)

// TestBundleForAggregate verifies the D-09 aggregate counts a section with some
// members set, some unset, and some set to a different value correctly.
func TestBundleForAggregate(t *testing.T) {
	policy, _ := PolicyFor("alias (8 shortcuts)")
	effective := map[string]EffectiveEntry{
		"alias.st": {Value: "status", Scope: "global", Origin: "/home/u/.gitconfig"},
		"alias.co": {Value: "pull", Scope: "global", Origin: "/home/u/.gitconfig"}, // differs
		"alias.lg": {Value: lgFormatString, Scope: "global", Origin: "/home/u/.gitconfig"},
		// alias.br, alias.ci, alias.df, alias.unstage, alias.last: unset
	}
	res := BundleFor(policy, effective)
	if res.Total != 8 {
		t.Errorf("Total = %d, want 8", res.Total)
	}
	if res.Set != 3 {
		t.Errorf("Set = %d, want 3", res.Set)
	}
	if res.Differs != 1 {
		t.Errorf("Differs = %d, want 1 (alias.co=pull)", res.Differs)
	}
	if !reflect.DeepEqual(res.DiffersKeys, []string{"alias.co"}) {
		t.Errorf("DiffersKeys = %v, want [alias.co]", res.DiffersKeys)
	}
}

// TestBundleForNamesOnlyDiffersMembers asserts the detail pane names exactly
// the members whose values differ, and no others.
func TestBundleForNamesOnlyDiffersMembers(t *testing.T) {
	policy, _ := PolicyFor("color (ui/branch/diff/status)")
	effective := map[string]EffectiveEntry{
		"color.ui":     {Value: "auto"},
		"color.branch": {Value: "always"}, // differs
		"color.diff":   {Value: "auto"},
		"color.status": {Value: "always"}, // differs
	}
	res := BundleFor(policy, effective)
	want := []string{"color.branch", "color.status"}
	if !reflect.DeepEqual(res.DiffersKeys, want) {
		t.Errorf("DiffersKeys = %v, want %v (only the differing members)", res.DiffersKeys, want)
	}
}

// TestBundleForEmptyEffective verifies a fully-unset bundle produces a zero
// set-count and an empty differs list — the first-run case.
func TestBundleForEmptyEffective(t *testing.T) {
	policy, _ := PolicyFor("alias (8 shortcuts)")
	res := BundleFor(policy, map[string]EffectiveEntry{})
	if res.Set != 0 || res.Differs != 0 || len(res.DiffersKeys) != 0 {
		t.Errorf("empty bundle = %+v, want set=0 differs=0 no keys", res)
	}
}

// TestClassify_BundleNeedsActionWhenOneUnset verifies a bundle row is
// needs-action when at least one member key is unset, even when every other
// member is already correct (D-10's "counts once if ANY member is unset").
func TestClassify_BundleNeedsActionWhenOneUnset(t *testing.T) {
	policy, _ := PolicyFor("alias (8 shortcuts)")
	effective := map[string]EffectiveEntry{
		"alias.st":      {Value: "status", Scope: "global", Origin: "/home/u/.gitconfig"},
		"alias.co":      {Value: "checkout", Scope: "global", Origin: "/home/u/.gitconfig"},
		"alias.br":      {Value: "branch", Scope: "global", Origin: "/home/u/.gitconfig"},
		"alias.ci":      {Value: "commit", Scope: "global", Origin: "/home/u/.gitconfig"},
		"alias.df":      {Value: "diff", Scope: "global", Origin: "/home/u/.gitconfig"},
		"alias.lg":      {Value: lgFormatString, Scope: "global", Origin: "/home/u/.gitconfig"},
		"alias.unstage": {Value: "reset HEAD --", Scope: "global", Origin: "/home/u/.gitconfig"},
		// alias.last unset
	}
	inFile := map[string]EffectiveEntry{}
	rows, err := Classify([]OptionPolicy{policy}, effective, inFile, "/gitid/path")
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if rows[0].State != StateNeedsAction {
		t.Errorf("State = %v, want StateNeedsAction (one member unset)", rows[0].State)
	}
}
