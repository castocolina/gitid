package globalgit

import (
	"testing"
)

func TestPolicyFor_KnownKey(t *testing.T) {
	p, ok := PolicyFor("init.defaultBranch")
	if !ok {
		t.Fatal("PolicyFor(init.defaultBranch) returned ok=false")
	}
	if p.Recommended != "main" {
		t.Errorf("Recommended = %q, want %q", p.Recommended, "main")
	}
	if p.GitDefault != "master" {
		t.Errorf("GitDefault = %q, want %q", p.GitDefault, "master")
	}
	if p.MinVersion != "2.28" {
		t.Errorf("MinVersion = %q, want %q", p.MinVersion, "2.28")
	}
	if p.Gate != GateInformational {
		t.Errorf("Gate = %v, want GateInformational", p.Gate)
	}
}

func TestPolicyFor_CaseInsensitive(t *testing.T) {
	_, ok := PolicyFor("INIT.DEFAULTBRANCH")
	if !ok {
		t.Error("PolicyFor should match case-insensitively")
	}
	_, ok = PolicyFor("Init.DefaultBranch")
	if !ok {
		t.Error("PolicyFor should match case-insensitively (mixed case)")
	}
}

func TestPolicyFor_UnknownKey(t *testing.T) {
	_, ok := PolicyFor("unknown.key.that.does.not.exist")
	if ok {
		t.Error("PolicyFor should return ok=false for unknown key")
	}
}

func TestPolicy_HasAtLeastOneEntry(t *testing.T) {
	if len(Policy) == 0 {
		t.Error("Policy table must have at least one entry")
	}
}
