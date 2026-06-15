// Package clipboard copies public key text to the system clipboard (CLIP-01).
// It is a thin wrapper over github.com/atotto/clipboard, which dispatches to
// pbcopy on macOS and xclip, xsel, or wl-copy on Linux based on runtime
// availability; gitid does not hand-roll per-OS exec.
//
// When no clipboard tool is available the Copy call fails gracefully: it returns
// an error wrapping ErrNoClipboard so the caller can detect the no-tool case and
// print the key for manual copy instead of crashing (CLIP-02).
//
// Implemented in Phase 2 (pulled forward from a later phase because the
// create-new identity flow copies the .pub on generate).
package clipboard
