package repoclone

import (
	"testing"
)

// TestProviderFromURL_Github is a RED unit stub asserting that
// ProviderFromURL("https://github.com/o/r") returns "github.com". The real
// implementation is built in Plan 03 (05.7-03). This test fails now because
// ProviderFromURL returns an error sentinel.
func TestProviderFromURL_Github(t *testing.T) {
	provider, err := ProviderFromURL("https://github.com/o/r")
	if err != nil {
		// RED: real implementation not yet built. This FAIL is expected until Plan 03.
		t.Errorf("ProviderFromURL returned error (RED — not yet implemented): %v", err)
		return
	}
	if provider != "github.com" {
		t.Errorf("ProviderFromURL: got %q want %q", provider, "github.com")
	}
}
