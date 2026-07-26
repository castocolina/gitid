// Package tui provides the Bubble Tea v2 TUI for gitid. It wires a Bubble Tea
// program to the internal packages for identity management, displaying a
// doctor dashboard on startup and supporting form-based identity creation
// and editing.
//
// The dependency arrow is strictly one-directional: tui imports internal
// packages (doctor, identity, filewriter, etc.) but is never imported by them.
// cmd/gitid imports tui for the no-args TUI launch path. Internal packages
// import nothing from tui or cmd/gitid.
//
// Entry point: [Run] builds deps, creates the root model, and runs the
// tea.Program with alt-screen mode (View.AltScreen = true).
package tui
