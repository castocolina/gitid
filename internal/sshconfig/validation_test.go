package sshconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateHostBlockAcceptsValidValues(t *testing.T) {
	if err := ValidateHostBlock("acme.github.com", "ssh.github.com", "443", "~/.ssh/id_ed25519_acme"); err != nil {
		t.Errorf("valid Host block rejected: %v", err)
	}
}

func TestValidateHostBlockRejectsUnsafeAlias(t *testing.T) {
	cases := []struct {
		alias, wantField, wantSub string
	}{
		{"", "alias", "empty"},
		{"ac me", "alias", "whitespace"},
		{"acme*", "alias", "wildcard"},
		{"!acme", "alias", "negation"},
		{"ac,me", "alias", "comma"},
		{"acme\nHost evil", "alias", "whitespace"},
	}
	for _, tc := range cases {
		err := ValidateHostBlock(tc.alias, "ssh.github.com", "443", "~/.ssh/id_ed25519_acme")
		if err == nil {
			t.Errorf("alias %q: expected rejection", tc.alias)
			continue
		}
		ve, ok := err.(*ValidationError)
		if !ok {
			t.Errorf("alias %q: expected ValidationError, got %T", tc.alias, err)
			continue
		}
		if ve.Field != tc.wantField {
			t.Errorf("alias %q: field = %q, want %q", tc.alias, ve.Field, tc.wantField)
		}
		if !containsFold(ve.Message, tc.wantSub) {
			t.Errorf("alias %q: message = %q, want substring %q", tc.alias, ve.Message, tc.wantSub)
		}
	}
}

func TestValidateHostBlockRejectsUnsafePort(t *testing.T) {
	cases := []struct{ port, wantSub string }{
		{"0", "outside"},
		{"65536", "outside"},
		{"-1", "outside"},
		{"abc", "number"},
		{"22x", "number"},
	}
	for _, tc := range cases {
		err := ValidateHostBlock("acme.github.com", "ssh.github.com", tc.port, "~/.ssh/id_ed25519_acme")
		if err == nil {
			t.Errorf("port %q: expected rejection", tc.port)
			continue
		}
		ve := err.(*ValidationError)
		if ve.Field != "port" {
			t.Errorf("port %q: field = %q, want port", tc.port, ve.Field)
		}
		if !containsFold(ve.Message, tc.wantSub) {
			t.Errorf("port %q: message = %q, want %q", tc.port, ve.Message, tc.wantSub)
		}
	}
}

func TestHostMatchGlob(t *testing.T) {
	cases := []struct {
		pattern, candidate string
		want               bool
	}{
		{"acme.github.com", "acme.github.com", true},
		{"*.github.com", "personal.github.com", true},
		{"*.github.com", "org.github.com", true},
		{"?cme.github.com", "acme.github.com", true},
		{"*.github.com", "personal.gitlab.com", false},
		{"github.com", "personal.github.com", false},
		{"*", "anything", true},
	}
	for _, tc := range cases {
		if got := HostMatch(tc.pattern, tc.candidate); got != tc.want {
			t.Errorf("HostMatch(%q, %q) = %v, want %v", tc.pattern, tc.candidate, got, tc.want)
		}
	}
}

func TestAliasCollisionExact(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "config")
	writeFile(t, config, "Host acme.github.com\n    Hostname ssh.github.com\n")

	collides, err := AliasCollision(config, "acme.github.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !collides {
		t.Error("exact alias collision expected")
	}
}

func TestAliasCollisionWildcard(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "config")
	writeFile(t, config, "Host *.github.com\n    Hostname ssh.github.com\n")

	collides, err := AliasCollision(config, "personal.github.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !collides {
		t.Error("wildcard collision expected")
	}
}

func TestAliasCollisionNegated(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "config")
	writeFile(t, config, "Host *.github.com !acme.github.com\n    Hostname ssh.github.com\n")

	collides, err := AliasCollision(config, "acme.github.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if collides {
		t.Error("negated pattern must cancel the collision")
	}
}

func TestAliasCollisionIncludeAware(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "config")
	included := filepath.Join(dir, "included.config")
	writeFile(t, config, "Include "+included+"\n")
	writeFile(t, included, "Host included.github.com\n    Hostname ssh.github.com\n")

	collides, err := AliasCollision(config, "included.github.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !collides {
		t.Error("Include'd Host pattern must collide")
	}
}

func TestAliasCollisionMissingFile(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "config")
	collides, err := AliasCollision(config, "acme.github.com")
	if err != nil {
		t.Fatalf("missing config must not error: %v", err)
	}
	if collides {
		t.Error("missing config cannot collide")
	}
}

func TestAliasCollisionMalformedConfig(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "config")
	writeFile(t, config, "Match\n")

	_, err := AliasCollision(config, "acme.github.com")
	if err == nil {
		t.Error("malformed config must return a blocking error")
	}
}

func TestAliasCollisionCyclicInclude(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "config")
	included := filepath.Join(dir, "included.config")
	writeFile(t, config, "Include "+included+"\n")
	writeFile(t, included, "Include "+config+"\n")

	_, err := AliasCollision(config, "acme.github.com")
	if err == nil {
		t.Error("cyclic Include must return a blocking error")
	}
}

// TestHostPatternsMatch drives the exported stanza matcher over the full
// vocabulary of cases the plan pinned: plain, wildcard, negated-only,
// positive-plus-negated, multi-token, and empty.
func TestHostPatternsMatch(t *testing.T) {
	const probe = "gitid-probe.invalid"
	cases := []struct {
		name     string
		patterns []string
		want     bool
	}{
		{"plain match", []string{"gitid-probe.invalid"}, true},
		{"plain no-match", []string{"other.invalid"}, false},
		{"wildcard star", []string{"*"}, true},
		{"wildcard glob", []string{"*.invalid"}, true},
		{"negated only", []string{"!gitid-probe.invalid"}, false},
		{"negated only non-matching", []string{"!other.invalid"}, false},
		{"positive plus negated — excluded", []string{"*.invalid", "!gitid-probe.invalid"}, false},
		{"positive plus negated — not excluded", []string{"*.invalid", "!other.invalid"}, true},
		{"multi-token match", []string{"first.invalid", "gitid-probe.invalid"}, true},
		{"empty patterns", []string{}, false},
		// The projection gate: the wildcard guard stays at aliasCollides' call site,
		// so HostPatternsMatch({"*"}, probe) MUST return true — a matcher that
		// skips "*" would blind the dominant row-4 shadowing case.
		{"single-star must match", []string{"*"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := HostPatternsMatch(tc.patterns, probe)
			if got != tc.want {
				t.Errorf("HostPatternsMatch(%v, %q) = %v, want %v", tc.patterns, probe, got, tc.want)
			}
		})
	}
}

// TestHostLineMatchesEquivalence asserts that HostLineMatches returns identical
// results for the same lists supplied as one whitespace-separated string.
func TestHostLineMatchesEquivalence(t *testing.T) {
	const probe = "gitid-probe.invalid"
	cases := []struct {
		line string
		want bool
	}{
		{"*", true},
		{"*.invalid", true},
		{"gitid-probe.invalid", true},
		{"other.invalid", false},
		{"*.invalid !gitid-probe.invalid", false},
		{"*.invalid !other.invalid", true},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.line, func(t *testing.T) {
			got := HostLineMatches(tc.line, probe)
			if got != tc.want {
				t.Errorf("HostLineMatches(%q, %q) = %v, want %v", tc.line, probe, got, tc.want)
			}
			// Verify equivalence with HostPatternsMatch over the same tokens.
			tokens := strings.Fields(tc.line)
			want2 := HostPatternsMatch(tokens, probe)
			if got != want2 {
				t.Errorf("HostLineMatches(%q) = %v but HostPatternsMatch(%v) = %v — must agree", tc.line, got, tokens, want2)
			}
		})
	}
}

// TestHostPatternsMatchPreservesNegationPrefix is the projection gate from
// matcher_extraction cycle-4 LOW: tokens must arrive with their "!" prefix
// intact so HostPatternsMatch can interpret them. A negation-carrying list
// must still exclude.
func TestHostPatternsMatchPreservesNegationPrefix(t *testing.T) {
	const probe = "gitid-probe.invalid"
	// "!gitid-probe.invalid" should exclude; if the prefix were pre-trimmed the
	// token would become "gitid-probe.invalid" and match positively.
	if HostPatternsMatch([]string{"*.invalid", "!gitid-probe.invalid"}, probe) {
		t.Error("negation-carrying token excluded the host but HostPatternsMatch returned true — prefix was not preserved")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func containsFold(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}
