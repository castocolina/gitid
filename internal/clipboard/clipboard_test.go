package clipboard

import (
	"errors"
	"testing"

	"github.com/atotto/clipboard"
)

// TestCopyUnavailable asserts that when the clipboard backend reports it is
// unsupported (atotto sets clipboard.Unsupported and WriteAll returns an error),
// Copy returns an error the caller can detect via ErrNoClipboard so it can fall
// back to printing the key manually (CLIP-02).
func TestCopyUnavailable(t *testing.T) {
	origWrite := writeAll
	origUnsupported := clipboard.Unsupported
	t.Cleanup(func() {
		writeAll = origWrite
		clipboard.Unsupported = origUnsupported
	})

	clipboard.Unsupported = true
	writeAll = func(string) error {
		return errors.New("no clipboard utilities available")
	}

	err := Copy("ssh-ed25519 AAAA... work@gitid")
	if err == nil {
		t.Fatal("Copy returned nil when the clipboard backend is unavailable; want an error")
	}
	if !errors.Is(err, ErrNoClipboard) {
		t.Errorf("Copy error = %v, want it to wrap ErrNoClipboard", err)
	}
}

// TestCopyAvailable asserts that when a clipboard backend is available, Copy
// returns nil and forwards the exact text (CLIP-01).
func TestCopyAvailable(t *testing.T) {
	origWrite := writeAll
	origUnsupported := clipboard.Unsupported
	t.Cleanup(func() {
		writeAll = origWrite
		clipboard.Unsupported = origUnsupported
	})

	clipboard.Unsupported = false
	const want = "ssh-ed25519 AAAA... work@gitid"
	var got string
	writeAll = func(text string) error {
		got = text
		return nil
	}

	if err := Copy(want); err != nil {
		t.Fatalf("Copy returned error with an available backend: %v", err)
	}
	if got != want {
		t.Errorf("Copy forwarded %q, want %q", got, want)
	}
}

// TestCopyPropagatesOtherErrors asserts that a backend error while the backend
// is reportedly supported is returned as-is, not masked as ErrNoClipboard.
func TestCopyPropagatesOtherErrors(t *testing.T) {
	origWrite := writeAll
	origUnsupported := clipboard.Unsupported
	t.Cleanup(func() {
		writeAll = origWrite
		clipboard.Unsupported = origUnsupported
	})

	clipboard.Unsupported = false
	sentinel := errors.New("clipboard backend exploded")
	writeAll = func(string) error { return sentinel }

	err := Copy("data")
	if !errors.Is(err, sentinel) {
		t.Errorf("Copy error = %v, want it to wrap %v", err, sentinel)
	}
	if errors.Is(err, ErrNoClipboard) {
		t.Errorf("a generic backend error must not be reported as ErrNoClipboard")
	}
}
