// Package filewriter provides the shared safe-write concern for gitid:
// timestamped backup, render-to-temp, atomic rename via os.Rename,
// correct file-permission setting, and optional restore on error.
// It is used by both the sshconfig and gitconfig packages to keep
// backup and atomicity logic in one place.
//
// Implementation lands in a later phase (Phase 2+).
package filewriter
