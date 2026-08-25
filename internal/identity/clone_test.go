// Package identity — clone derivation tests (plan 05-05).
//
// R-28 / 05-07: Account carries key paths and author values but no parsed
// fragment SigningKey value. That field only becomes available in plan 05-07
// (SigningKeyPath on Account). Until then, tests assert the DERIVED signing
// path only (ReuseKeyPath when reusing, or the empty generate sentinel) and
// MUST NOT claim equality against the source's configured fragment signing key.
package identity

import (
	"errors"
	"path"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/gitconfig"
)

func cloneSource() Account {
	return Account{
		Name:               "work",
		GitName:            "Work User",
		GitEmail:           "work@example.com",
		Provider:           "github.com",
		Alias:              "work.github.com",
		Hostname:           "ssh.github.com",
		Port:               443,
		KeyPath:            "/tmp/.ssh/id_ed25519_work",
		PubPath:            "/tmp/.ssh/id_ed25519_work.pub",
		Matches:            []gitconfig.Match{DefaultMatch("work")},
		FragmentPath:       "/tmp/.gitconfig.d/work",
		GitconfigPath:      "/tmp/.gitconfig",
		SSHConfigPath:      "/tmp/.ssh/config",
		AllowedSignersPath: "/tmp/.ssh/allowed_signers",
	}
}

func cloneTargets() CloneTargets {
	return CloneTargets{
		GitconfigPath:      "/tmp/.gitconfig",
		SSHConfigPath:      "/tmp/.ssh/config",
		AllowedSignersPath: "/tmp/.ssh/allowed_signers",
		FragmentDir:        "/tmp/.gitconfig.d",
	}
}

func TestSuggestCloneName_BaseAndBump(t *testing.T) {
	if got := SuggestCloneName("work", nil); got != "work-clone" {
		t.Errorf("SuggestCloneName(work, nil) = %q, want work-clone", got)
	}
	taken := []string{"work-clone", "work-clone-2", "work-clone-3"}
	if got := SuggestCloneName("work", taken); got != "work-clone-4" {
		t.Errorf("SuggestCloneName against taken base+2+3 = %q, want work-clone-4", got)
	}
}

func TestSuggestCloneName_CaseInsensitive(t *testing.T) {
	taken := []string{"Work-Clone", "WORK-CLONE-2"}
	if got := SuggestCloneName("work", taken); got != "work-clone-3" {
		t.Errorf("SuggestCloneName case-insensitive = %q, want work-clone-3", got)
	}
}

func TestSuggestCloneName_NeverReturnsTaken(t *testing.T) {
	taken := []string{"acme-clone", "acme-clone-2"}
	got := SuggestCloneName("acme", taken)
	for _, name := range taken {
		if strings.EqualFold(got, name) {
			t.Fatalf("SuggestCloneName returned taken name %q", got)
		}
	}
}

func TestDeriveCloneInput_CopiesAuthorAndSSHEndpoint(t *testing.T) {
	src := cloneSource()
	in, notices, err := DeriveCloneInput(src, "work-clone", true, cloneTargets())
	if err != nil {
		t.Fatalf("DeriveCloneInput: %v", err)
	}
	if in.Name != "work-clone" {
		t.Errorf("Name = %q, want work-clone", in.Name)
	}
	if in.GitName != src.GitName || in.GitEmail != src.GitEmail {
		t.Errorf("author not copied: name=%q email=%q", in.GitName, in.GitEmail)
	}
	if in.Provider != src.Provider {
		t.Errorf("Provider = %q, want %q", in.Provider, src.Provider)
	}
	if in.Hostname != src.Hostname || in.Port != src.Port {
		t.Errorf("endpoint = %s:%d, want %s:%d", in.Hostname, in.Port, src.Hostname, src.Port)
	}
	if want := DefaultAlias("work-clone", src.Provider); in.Alias != want {
		t.Errorf("Alias = %q, want DefaultAlias shape %q", in.Alias, want)
	}
	if len(notices.CopiedFields) != 2 {
		t.Fatalf("CopiedFields len = %d, want 2", len(notices.CopiedFields))
	}
	if notices.CopiedFields[0] != CopiedFieldGitName || notices.CopiedFields[1] != CopiedFieldGitEmail {
		t.Errorf("CopiedFields = %#v, want [%q %q]", notices.CopiedFields, CopiedFieldGitName, CopiedFieldGitEmail)
	}
}

func TestDeriveCloneInput_MatchesContainCloneNotSource(t *testing.T) {
	src := cloneSource()
	in, _, err := DeriveCloneInput(src, "work-clone", false, cloneTargets())
	if err != nil {
		t.Fatalf("DeriveCloneInput: %v", err)
	}
	if len(in.Matches) != 1 {
		t.Fatalf("Matches len = %d, want 1", len(in.Matches))
	}
	want := GitdirMatch("work-clone")
	if in.Matches[0] != want {
		t.Errorf("Matches[0] = %#v, want %#v", in.Matches[0], want)
	}
	if strings.Contains(in.Matches[0].Value, "work/") && !strings.Contains(in.Matches[0].Value, "work-clone") {
		t.Errorf("match value still names source directory: %q", in.Matches[0].Value)
	}
	if !strings.Contains(in.Matches[0].Value, "work-clone") {
		t.Errorf("match value missing clone name: %q", in.Matches[0].Value)
	}
	if strings.Contains(in.Matches[0].Value, "/work/") {
		t.Errorf("match value still contains source-only path segment: %q", in.Matches[0].Value)
	}
}

func TestDeriveCloneInput_HasconfigPreservesKindAndRebuildsValue(t *testing.T) {
	src := cloneSource()
	src.Matches = []gitconfig.Match{{
		Kind:  gitconfig.MatchHasconfig,
		Value: "remote.*.url:git@work.github.com:*/**",
	}}
	in, _, err := DeriveCloneInput(src, "work-clone", false, cloneTargets())
	if err != nil {
		t.Fatalf("DeriveCloneInput: %v", err)
	}
	want := HasconfigMatch(in.Alias)
	if len(in.Matches) != 1 || in.Matches[0] != want {
		t.Fatalf("hasconfig derived = %#v, want %#v", in.Matches, want)
	}
	block, err := gitconfig.RenderCheckedIncludeIf(in.Name, in.FragmentPath, in.Matches)
	if err != nil {
		t.Fatalf("RenderCheckedIncludeIf: %v", err)
	}
	parsed := gitconfig.ParseManagedIncludeIf([]byte(block))
	info, ok := parsed[in.Name]
	if !ok {
		t.Fatalf("parsed includeIf missing identity %q; block:\n%s", in.Name, block)
	}
	if len(info.Matches) != 1 || info.Matches[0] != want {
		t.Errorf("round-trip match = %#v, want %#v", info.Matches, want)
	}
}

func TestDeriveCloneInput_GitdirWithoutSourceNameStillDerives(t *testing.T) {
	src := cloneSource()
	src.Matches = []gitconfig.Match{{
		Kind:  gitconfig.MatchGitdir,
		Value: "~/projects/clients/",
	}}
	in, _, err := DeriveCloneInput(src, "work-clone", false, cloneTargets())
	if err != nil {
		t.Fatalf("DeriveCloneInput: %v", err)
	}
	want := GitdirMatch("work-clone")
	if len(in.Matches) != 1 || in.Matches[0] != want {
		t.Errorf("derived = %#v, want %#v (source value had no source name)", in.Matches, want)
	}
}

func TestDeriveCloneInput_GitdirWithSourceNameTwiceIsNotTextuallyRewritten(t *testing.T) {
	src := cloneSource()
	// Source name appears as the directory AND inside a longer component.
	src.Matches = []gitconfig.Match{{
		Kind:  gitconfig.MatchGitdir,
		Value: "~/git/work/work-notes/",
	}}
	in, _, err := DeriveCloneInput(src, "work-clone", false, cloneTargets())
	if err != nil {
		t.Fatalf("DeriveCloneInput: %v", err)
	}
	want := GitdirMatch("work-clone")
	if len(in.Matches) != 1 || in.Matches[0] != want {
		t.Errorf("derived = %#v, want constructor output %#v (proves no textual rewrite)", in.Matches, want)
	}
	if strings.Contains(in.Matches[0].Value, "work-notes") {
		t.Errorf("textual rewrite leaked source component into clone match: %q", in.Matches[0].Value)
	}
}

func TestDeriveCloneInput_MultipleMatchesReDerivedInOrder(t *testing.T) {
	src := cloneSource()
	src.Matches = []gitconfig.Match{
		{Kind: gitconfig.MatchGitdir, Value: "~/git/work/"},
		{Kind: gitconfig.MatchHasconfig, Value: "remote.*.url:git@work.github.com:*/**"},
	}
	in, _, err := DeriveCloneInput(src, "work-clone", false, cloneTargets())
	if err != nil {
		t.Fatalf("DeriveCloneInput: %v", err)
	}
	if len(in.Matches) != 2 {
		t.Fatalf("Matches len = %d, want 2 (none dropped)", len(in.Matches))
	}
	if in.Matches[0] != GitdirMatch("work-clone") {
		t.Errorf("Matches[0] = %#v, want gitdir constructor", in.Matches[0])
	}
	if in.Matches[1] != HasconfigMatch(in.Alias) {
		t.Errorf("Matches[1] = %#v, want hasconfig constructor", in.Matches[1])
	}
}

func TestDeriveCloneInput_NoMatchesFallsBackToProjectGitdirDefault(t *testing.T) {
	src := cloneSource()
	src.Matches = nil
	in, _, err := DeriveCloneInput(src, "work-clone", false, cloneTargets())
	if err != nil {
		t.Fatalf("DeriveCloneInput: %v", err)
	}
	want := GitdirMatch("work-clone")
	if len(in.Matches) != 1 || in.Matches[0] != want {
		t.Errorf("empty-source fallback = %#v, want project gitdir default %#v", in.Matches, want)
	}
}

func TestDeriveCloneInput_FragmentAndTargets(t *testing.T) {
	src := cloneSource()
	targets := cloneTargets()
	in, _, err := DeriveCloneInput(src, "work-clone", true, targets)
	if err != nil {
		t.Fatalf("DeriveCloneInput: %v", err)
	}
	wantFrag := path.Join(targets.FragmentDir, "work-clone")
	if in.FragmentPath != wantFrag {
		t.Errorf("FragmentPath = %q, want %q", in.FragmentPath, wantFrag)
	}
	if in.GitconfigPath != targets.GitconfigPath || in.SSHConfigPath != targets.SSHConfigPath {
		t.Errorf("managed paths not taken from targets: git=%q ssh=%q", in.GitconfigPath, in.SSHConfigPath)
	}
	if in.AllowedSignersPath != targets.AllowedSignersPath {
		t.Errorf("AllowedSignersPath = %q, want %q", in.AllowedSignersPath, targets.AllowedSignersPath)
	}
}

func TestDeriveCloneInput_ReuseKeyPathIsCloneResolvedKey(t *testing.T) {
	src := cloneSource()
	in, _, err := DeriveCloneInput(src, "work-clone", true, cloneTargets())
	if err != nil {
		t.Fatalf("DeriveCloneInput: %v", err)
	}
	// Derived signing/auth key path == the clone's resolved key (source key on reuse).
	if in.ReuseKeyPath != src.KeyPath {
		t.Errorf("ReuseKeyPath = %q, want clone's resolved key %q", in.ReuseKeyPath, src.KeyPath)
	}
	inGen, _, err := DeriveCloneInput(src, "work-clone", false, cloneTargets())
	if err != nil {
		t.Fatalf("DeriveCloneInput generate: %v", err)
	}
	if inGen.ReuseKeyPath != "" {
		t.Errorf("generate ReuseKeyPath = %q, want empty (new canonical path at write time)", inGen.ReuseKeyPath)
	}
}

func TestDeriveCloneInput_SameNameIsFieldError(t *testing.T) {
	src := cloneSource()
	_, _, err := DeriveCloneInput(src, "work", false, cloneTargets())
	var fe *FieldError
	if !errors.As(err, &fe) {
		t.Fatalf("error = %v (%T), want *FieldError", err, err)
	}
	if fe.Field != "name" {
		t.Errorf("Field = %q, want name", fe.Field)
	}
}

func TestDeriveCloneInput_EmptyNameIsFieldError(t *testing.T) {
	src := cloneSource()
	_, _, err := DeriveCloneInput(src, "  ", false, cloneTargets())
	var fe *FieldError
	if !errors.As(err, &fe) {
		t.Fatalf("error = %v (%T), want *FieldError", err, err)
	}
	if fe.Field != "name" {
		t.Errorf("Field = %q, want name", fe.Field)
	}
}

func TestGitdirMatch_UsesDefaultMatch(t *testing.T) {
	if got, want := GitdirMatch("acme"), DefaultMatch("acme"); got != want {
		t.Errorf("GitdirMatch = %#v, want DefaultMatch %#v", got, want)
	}
}

func TestHasconfigMatch_RecipeCanonical(t *testing.T) {
	got := HasconfigMatch("work-clone.github.com")
	if got.Kind != gitconfig.MatchHasconfig {
		t.Errorf("Kind = %v, want MatchHasconfig", got.Kind)
	}
	want := "remote.*.url:git@work-clone.github.com:*/**"
	if got.Value != want {
		t.Errorf("Value = %q, want %q", got.Value, want)
	}
}
