package clipboard

import (
	"errors"
	"fmt"

	"github.com/atotto/clipboard"
)

// ErrNoClipboard is wrapped by Copy when no clipboard backend is available, so
// callers can detect this case via errors.Is and fall back to printing the key
// for manual copy (CLIP-02).
var ErrNoClipboard = errors.New("clipboard: no clipboard tool available")

// writeAll is the seam used to reach the system clipboard. It defaults to
// atotto/clipboard.WriteAll and is swapped in tests to simulate an unavailable
// backend without uninstalling the OS clipboard tools.
var writeAll = clipboard.WriteAll

// Copy places text on the system clipboard (CLIP-01). It dispatches to
// atotto/clipboard, which selects pbcopy/wl-copy/xclip/xsel per platform; gitid
// never hand-rolls per-OS exec.
//
// When no clipboard tool is available (atotto reports clipboard.Unsupported),
// the underlying error is wrapped with ErrNoClipboard so the caller can detect
// the no-tool case and print the key for manual copy (CLIP-02). Any other
// backend failure is returned unchanged.
func Copy(text string) error {
	if err := writeAll(text); err != nil {
		if clipboard.Unsupported {
			return fmt.Errorf("%w: %v", ErrNoClipboard, err)
		}
		return err
	}
	return nil
}
