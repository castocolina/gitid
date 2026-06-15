// Package doctor performs health checks on a gitid-managed environment:
// key permissions, SSH config coherence, gitconfig coherence, orphaned managed
// blocks, signing key wiring, ssh-agent presence, and required tool availability.
// It composes the platform, deps, sshconfig, and gitconfig packages and never
// writes to any file — it returns structured findings only.
//
// Implementation lands in a later phase (Phase 4+).
package doctor
