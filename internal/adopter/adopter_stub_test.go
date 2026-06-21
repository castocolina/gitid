package adopter

import (
	"testing"
)

// TestMatchIdentityName_ExtractsNameFromPath is a RED unit stub asserting that
// MatchIdentityName("~/.gitconfig_work") returns "work". The real implementation
// is built in Plan 02 (05.7-02). This test fails now because MatchIdentityName
// returns an error sentinel.
func TestMatchIdentityName_ExtractsNameFromPath(t *testing.T) {
	name, err := MatchIdentityName("/home/user/.gitconfig_work")
	if err != nil {
		// RED: real implementation not yet built. This FAIL is expected until Plan 02.
		t.Errorf("MatchIdentityName returned error (RED — not yet implemented): %v", err)
		return
	}
	if name != "work" {
		t.Errorf("MatchIdentityName: got %q want %q", name, "work")
	}
}
