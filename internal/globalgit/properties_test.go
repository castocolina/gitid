package globalgit

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

// TestAllSetKeysReturnsEverySetKeyWithProvenance is PROP-02's core behavior:
// given a fake RunGitConfig returning NUL-delimited records across global,
// system, and local scopes, AllSetKeys returns one SetKey per record
// carrying the value, the scope word git printed, and the origin with its
// "file:" prefix stripped.
func TestAllSetKeysReturnsEverySetKeyWithProvenance(t *testing.T) {
	golden := "global\x00file:/home/user/.gitconfig\x00core.editor\nvim\x00" +
		"system\x00file:/etc/gitconfig\x00core.autocrlf\ninput\x00" +
		"local\x00file:/repo/.git/config\x00user.name\nLocal User\x00"
	deps := Deps{
		RunGitConfig: func(_ context.Context, _ ...string) (string, error) { return golden, nil },
		NonRepoCwd:   tempNonRepoCwd,
	}
	got, err := AllSetKeys(deps)
	if err != nil {
		t.Fatalf("AllSetKeys: %v", err)
	}
	want := []SetKey{
		{Key: "core.autocrlf", Value: "input", Scope: "system", Origin: "/etc/gitconfig", ValueCount: 1},
		{Key: "core.editor", Value: "vim", Scope: "global", Origin: "/home/user/.gitconfig", ValueCount: 1},
		{Key: "user.name", Value: "Local User", Scope: "local", Origin: "/repo/.git/config", ValueCount: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("AllSetKeys = %#v, want %#v", got, want)
	}
}

// TestAllSetKeysMarksMultiValuedKeys is the WR-04 regression: git config is
// legitimately multi-valued (e.g. credential.helper stacked at system AND
// global scope) — parseNULRecords keys its map by name, so the LAST
// occurrence wins, but ValueCount must still report every physical
// occurrence git printed, so the screen never implies a stacked key is
// single-valued. A key that appears exactly once must report ValueCount 1
// (never 0, which would read as "not set").
func TestAllSetKeysMarksMultiValuedKeys(t *testing.T) {
	golden := "system\x00file:/usr/local/etc/gitconfig\x00credential.helper\nosxkeychain\x00" +
		"global\x00file:/home/user/.gitconfig\x00credential.helper\nosxkeychain\x00" +
		"global\x00file:/home/user/.gitconfig\x00credential.helper\ncache\x00" +
		"global\x00file:/home/user/.gitconfig\x00core.editor\nvim\x00"
	deps := Deps{
		RunGitConfig: func(_ context.Context, _ ...string) (string, error) { return golden, nil },
		NonRepoCwd:   tempNonRepoCwd,
	}
	got, err := AllSetKeys(deps)
	if err != nil {
		t.Fatalf("AllSetKeys: %v", err)
	}
	byKey := make(map[string]SetKey, len(got))
	for _, k := range got {
		byKey[k.Key] = k
	}
	cred, ok := byKey["credential.helper"]
	if !ok {
		t.Fatal("expected a credential.helper entry")
	}
	if cred.ValueCount != 3 {
		t.Errorf("credential.helper ValueCount = %d, want 3 (three physical occurrences across system+global scope)", cred.ValueCount)
	}
	if cred.Value != "cache" {
		t.Errorf("credential.helper Value = %q, want the LAST occurrence %q (git's own last-wins resolution)", cred.Value, "cache")
	}
	editor, ok := byKey["core.editor"]
	if !ok {
		t.Fatal("expected a core.editor entry")
	}
	if editor.ValueCount != 1 {
		t.Errorf("core.editor ValueCount = %d, want 1 for a single-occurrence key (never 0, which would read as 'not set')", editor.ValueCount)
	}
}

// TestAllSetKeysIsSortedByKey pins the byte-identical-output guarantee: Go
// map iteration order is not stable, so an unsorted list would make two
// calls over the same payload diverge.
func TestAllSetKeysIsSortedByKey(t *testing.T) {
	golden := "global\x00file:/a\x00zulu.key\nz\x00" +
		"global\x00file:/a\x00alpha.key\na\x00" +
		"global\x00file:/a\x00mike.key\nm\x00" +
		"global\x00file:/a\x00kilo.key\nk\x00"
	deps := Deps{
		RunGitConfig: func(_ context.Context, _ ...string) (string, error) { return golden, nil },
		NonRepoCwd:   tempNonRepoCwd,
	}
	first, err := AllSetKeys(deps)
	if err != nil {
		t.Fatalf("AllSetKeys: %v", err)
	}
	second, err := AllSetKeys(deps)
	if err != nil {
		t.Fatalf("AllSetKeys: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("two calls over the same payload diverged:\n%#v\nvs\n%#v", first, second)
	}
	for i := 1; i < len(first); i++ {
		if first[i-1].Key >= first[i].Key {
			t.Fatalf("not sorted ascending at index %d: %q >= %q", i, first[i-1].Key, first[i].Key)
		}
	}
}

// TestAllSetKeysUsesTheListForm asserts the EXACT argv AllSetKeys' underlying
// probe sends: `config --show-origin --show-scope --list -z`. A `--get` or a
// `--global`-narrowed form would answer a different question (D-E) — it
// would drop the system scope, exactly the "set at a scope gitid cannot
// change" provenance class this screen exists to expose.
func TestAllSetKeysUsesTheListForm(t *testing.T) {
	var gotArgs []string
	deps := Deps{
		RunGitConfig: func(_ context.Context, args ...string) (string, error) {
			gotArgs = args
			return "", nil
		},
		NonRepoCwd: tempNonRepoCwd,
	}
	if _, err := AllSetKeys(deps); err != nil {
		t.Fatalf("AllSetKeys: %v", err)
	}
	want := []string{"config", "--show-origin", "--show-scope", "--list", "-z"}
	if !reflect.DeepEqual(gotArgs, want) {
		t.Fatalf("RunGitConfig args = %v, want %v", gotArgs, want)
	}
}

// TestAllSetKeysPreservesValuesContainingSeparators proves a value
// containing a tab, an "=", and a newline survives intact — the whole
// reason the -z form is used rather than the ambiguous plain --show-origin
// format.
func TestAllSetKeysPreservesValuesContainingSeparators(t *testing.T) {
	value := "part1\tpart2=value\nembedded newline"
	golden := "global\x00file:/home/user/.gitconfig\x00alias.co\n" + value + "\x00"
	deps := Deps{
		RunGitConfig: func(_ context.Context, _ ...string) (string, error) { return golden, nil },
		NonRepoCwd:   tempNonRepoCwd,
	}
	got, err := AllSetKeys(deps)
	if err != nil {
		t.Fatalf("AllSetKeys: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("AllSetKeys returned %d rows, want 1", len(got))
	}
	if got[0].Value != value {
		t.Fatalf("Value = %q, want the exact round-tripped value %q", got[0].Value, value)
	}
}

// TestAllSetKeysPropagatesProbeError proves a probe error yields (nil, err);
// it does NOT degrade to an empty list, because the sub-tab must be able to
// tell "the probe failed" apart from "nothing is set" — two DIFFERENT states
// with two different frozen sentences.
func TestAllSetKeysPropagatesProbeError(t *testing.T) {
	wantErr := errors.New("boom")
	deps := Deps{
		RunGitConfig: func(_ context.Context, _ ...string) (string, error) { return "", wantErr },
		NonRepoCwd:   tempNonRepoCwd,
	}
	got, err := AllSetKeys(deps)
	if got != nil {
		t.Fatalf("AllSetKeys on probe error = %#v, want nil", got)
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("AllSetKeys err = %v, want wrapping %v", err, wantErr)
	}
}
