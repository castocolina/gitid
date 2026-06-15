// Package clipboard copies public key text to the system clipboard.
// On macOS it uses pbcopy; on Linux it dispatches to xclip, xsel, or
// wl-copy based on runtime availability detected via the deps package.
// It is a thin wrapper over os/exec.
//
// Implementation lands in a later phase (Phase 5+).
package clipboard
