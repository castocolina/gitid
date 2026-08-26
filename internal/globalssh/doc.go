// Package globalssh is GSSH-01's effective-value and source-class engine: it
// reads the machine's real global SSH option values — through `ssh -G`
// probes and a directive scan of the files gitid reads — and classifies where
// each value provably comes from, so the Options sub-tab can render a
// provenance gitid can actually defend rather than one it infers.
//
// This package NEVER writes: writes belong to internal/sshconfig (the single
// owner of the gitid `Host *` managed block), and the ceremony that drives
// them belongs to cmd/gitid/lifecycle.go's runGlobalSSHApply. Data crosses
// the seam as OptionStatus values; internal/tuikit never imports this package
// (the view DTO conversion happens in cmd/gitid/wiring.go).
package globalssh
