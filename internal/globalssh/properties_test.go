package globalssh

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

// TestAllDirectivesReturnsEveryResolvedKey is PROP-01's core behavior: given
// a fake RunSSHG returning a multi-line `ssh -G` payload carrying far more
// than the six curated Policy keys, AllDirectives returns one Directive per
// lowercase `<key> <value>` line, with values trimmed.
func TestAllDirectivesReturnsEveryResolvedKey(t *testing.T) {
	payload := "stricthostkeychecking ask\n" +
		"ciphers chacha20-poly1305@openssh.com\n" +
		"connecttimeout none\n" +
		"loglevel INFO\n" +
		"controlmaster false\n" +
		"hostbasedauthentication no\n" +
		"serveraliveinterval 0\n" +
		"streamlocalbindmask 0177\n"
	deps := Deps{
		RunSSHG: func(_ context.Context, _ ...string) (string, error) { return payload, nil },
	}
	got, err := AllDirectives(deps)
	if err != nil {
		t.Fatalf("AllDirectives: %v", err)
	}
	want := []Directive{
		{Key: "ciphers", Value: "chacha20-poly1305@openssh.com"},
		{Key: "connecttimeout", Value: "none"},
		{Key: "controlmaster", Value: "false"},
		{Key: "hostbasedauthentication", Value: "no"},
		{Key: "loglevel", Value: "INFO"},
		{Key: "serveraliveinterval", Value: "0"},
		{Key: "streamlocalbindmask", Value: "0177"},
		{Key: "stricthostkeychecking", Value: "ask"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("AllDirectives = %#v, want %#v", got, want)
	}
	if len(got) <= len(Policy) {
		t.Fatalf("AllDirectives returned %d rows, want strictly more than the %d-row Policy subset", len(got), len(Policy))
	}
}

// TestAllDirectivesIsSortedByKey pins the byte-identical-output guarantee:
// Go map iteration order is not stable, so an unsorted list would make two
// calls over the same payload diverge.
func TestAllDirectivesIsSortedByKey(t *testing.T) {
	payload := "zulu z\nalpha a\nmike m\nkilo k\n"
	deps := Deps{RunSSHG: func(_ context.Context, _ ...string) (string, error) { return payload, nil }}
	first, err := AllDirectives(deps)
	if err != nil {
		t.Fatalf("AllDirectives: %v", err)
	}
	second, err := AllDirectives(deps)
	if err != nil {
		t.Fatalf("AllDirectives: %v", err)
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

// TestAllDirectivesSkipsCamelCaseLines pins the verified OpenSSH property
// parseResolvedOptions already documents: a camelCase line never matches.
func TestAllDirectivesSkipsCamelCaseLines(t *testing.T) {
	payload := "StrictHostKeyChecking yes\nstricthostkeychecking ask\n"
	deps := Deps{RunSSHG: func(_ context.Context, _ ...string) (string, error) { return payload, nil }}
	got, err := AllDirectives(deps)
	if err != nil {
		t.Fatalf("AllDirectives: %v", err)
	}
	if len(got) != 1 || got[0].Key != "stricthostkeychecking" || got[0].Value != "ask" {
		t.Fatalf("AllDirectives = %#v, want exactly the one lowercase line", got)
	}
}

// TestAllDirectivesProbesTheWildcardSentinel is RESEARCH Pitfall 5's guard:
// the probe must run against exactly `-G <ProbeHost>` and never `-F`, so a
// per-alias host or an isolated `-F /dev/null` baseline can never silently
// answer a different question.
func TestAllDirectivesProbesTheWildcardSentinel(t *testing.T) {
	var gotArgs []string
	deps := Deps{RunSSHG: func(_ context.Context, args ...string) (string, error) {
		gotArgs = args
		return "stricthostkeychecking ask\n", nil
	}}
	if _, err := AllDirectives(deps); err != nil {
		t.Fatalf("AllDirectives: %v", err)
	}
	if len(gotArgs) != 2 || gotArgs[0] != "-G" || gotArgs[1] != ProbeHost {
		t.Fatalf("RunSSHG args = %v, want exactly [-G %s]", gotArgs, ProbeHost)
	}
}

// TestAllDirectivesPropagatesProbeError proves a probe failure yields
// (nil, err) rather than degrading to an empty list — the browse sub-tab
// needs the real error to render its fail-open warning.
func TestAllDirectivesPropagatesProbeError(t *testing.T) {
	wantErr := errors.New("boom")
	deps := Deps{RunSSHG: func(_ context.Context, _ ...string) (string, error) { return "", wantErr }}
	got, err := AllDirectives(deps)
	if got != nil {
		t.Fatalf("AllDirectives on probe error = %#v, want nil", got)
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("AllDirectives err = %v, want %v", err, wantErr)
	}
}
