// Package platform detects the operating system (darwin or linux) and provides
// platform-specific hints for gitid: UseKeychain guard for SSH config on macOS,
// clipboard command selection, and permission-fix command suggestions.
// It has no third-party dependencies.
//
// Implementation lands in a later phase (Phase 2+).
package platform
