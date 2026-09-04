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
		{Key: "core.autocrlf", Value: "input", Scope: "system", Origin: "/etc/gitconfig"},
		{Key: "core.editor", Value: "vim", Scope: "global", Origin: "/home/user/.gitconfig"},
		{Key: "user.name", Value: "Local User", Scope: "local", Origin: "/repo/.git/config"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("AllSetKeys = %#v, want %#v", got, want)
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
