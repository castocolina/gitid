package uploader

import (
	"testing"
)

// TestDetect_AuthToolNotFound is a RED unit stub asserting that when Detect
// is called with a Deps.LookPath that finds nothing, it returns AuthToolNotFound.
// The real Detect logic (scanning gh then glab) is built in Plan 04 (05.7-04).
// Currently Detect always returns AuthToolNotFound (stub body), so this test
// passes trivially — but the companion assertions below confirm the return shape.
//
// The real RED assertion is that Detect with a functional LookPath that DOES find
// gh returns AuthAuthenticated — that will fail until Plan 04.
func TestDetect_AuthToolNotFound(t *testing.T) {
	deps := Deps{
		LookPath: func(_ string) (string, error) {
			return "", &notFoundError{name: "gh"}
		},
		RunCmd: func(_ string, _ ...string) (string, int, error) {
			return "", 0, nil
		},
	}

	_, _, status := Detect(deps)
	if status != AuthToolNotFound {
		// RED: real Detect not yet built; stub always returns AuthToolNotFound.
		t.Errorf("Detect: got status %d want AuthToolNotFound(%d)", status, AuthToolNotFound)
	}
}

// TestDetect_GHAuthenticatedIsRED verifies that when a fake gh is present and
// returns exit 0 for "auth status", Detect returns AuthAuthenticated. This
// assertion is RED until Plan 04 implements the real Detect body.
func TestDetect_GHAuthenticatedIsRED(t *testing.T) {
	deps := Deps{
		LookPath: func(name string) (string, error) {
			if name == "gh" {
				return "/fake/gh", nil
			}
			return "", &notFoundError{name: name}
		},
		RunCmd: func(_ string, _ ...string) (string, int, error) {
			return "", 0, nil // exit 0 = authenticated
		},
	}

	_, _, status := Detect(deps)
	if status != AuthAuthenticated {
		// RED: stub returns AuthToolNotFound; Plan 04 makes this GREEN.
		t.Errorf("Detect with fake gh: got status %d want AuthAuthenticated(%d) — RED until Plan 04",
			status, AuthAuthenticated)
	}
}

// notFoundError mimics exec.ErrNotFound for LookPath stubs.
type notFoundError struct{ name string }

func (e *notFoundError) Error() string { return e.name + ": executable file not found in $PATH" }
