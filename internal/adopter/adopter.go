// Package adopter detects plain-style gitconfig fragments (~/.gitconfig_<name>)
// that are not yet managed by gitid and offers to migrate or reference them.
// All external effects (file reads, copies, writes) are injected via Deps so
// this package is testable without real filesystem operations.
package adopter

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/castocolina/gitid/internal/gitconfig"
)

// Deps holds all external effects. Build live in tui/deps.go and cmd/gitid/adopt.go;
// pass fakes in tests. Every function field must be non-nil (wiring guard in
// tui/wiring_test.go TestBuildTUIDepsNilGuard_Phase57).
type Deps struct {
	ReadFile func(path string) ([]byte, error)
	// WriteFile backs up, writes atomically at mode; returns backupPath.
	// Wire to: filewriter.Write(path, content, 0o644)
	WriteFile func(path string, content []byte, mode os.FileMode) (backupPath string, err error)
	CopyFile  func(src, dst string) error
	// BackupAndRemove backs up src to a timestamped path and removes the original.
	// Wire to: filewriter.BackupAndRemove(path)
	BackupAndRemove func(path string) (backupPath string, err error)
	// WriteIncludeIf writes the managed includeIf block for an identity.
	// The gitconfigPath is captured by the live closure in tui/deps.go and cmd/gitid/adopt.go.
	// Wire to: gitconfig.WriteIncludeIf(gitconfigPath, id, fragPath, matches)
	WriteIncludeIf func(id, fragPath string, matches []gitconfig.Match) (backupPath string, err error)
	ReadFragment   func(path string) (gitconfig.FragmentInfo, error)
	// ListCandidates returns paths matching ~/.gitconfig_* that are not yet managed.
	ListCandidates func(homeDir string) ([]string, error)
}

// AdoptMethod selects migrate (copy+repoint) or reference-in-place.
type AdoptMethod int

const (
	// AdoptMigrate copies the fragment to ~/.gitconfig.d/ and writes an includeIf.
	AdoptMigrate AdoptMethod = iota
	// AdoptReferenceInPlace writes an includeIf pointing at the original file path.
	AdoptReferenceInPlace
)

// AdoptResult carries the outcome of a successful Adopt call.
type AdoptResult struct {
	// BackupPaths holds paths of any timestamped backups created during adoption.
	BackupPaths []string
	// MigratedPath is the destination path used for AdoptMigrate; empty for AdoptReferenceInPlace.
	MigratedPath string
	// IncludeIfBody is the rendered includeIf block written to ~/.gitconfig.
	IncludeIfBody string
}

// ErrAmbiguousIdentity is returned when a fragment path maps to more than one
// candidate identity name under gitid management.
var ErrAmbiguousIdentity = errors.New("adopter: ambiguous identity name")

// Adopt adopts sourcePath under identityName using method.
// It NEVER removes the source file; callers pass removeOriginal=true explicitly
// only after a successful Adopt and explicit user confirmation (D-05).
//
// RED stub: returns zero + sentinel. Plan 02 (05.7-02) implements the real body.
func Adopt(_, _, _ string, _ AdoptMethod, _ []gitconfig.Match, _ Deps) (AdoptResult, error) {
	return AdoptResult{}, errors.New("adopter: not implemented")
}

// ListCandidates returns paths matching ~/.gitconfig_* that are not yet
// managed by gitid (i.e. not in ~/.gitconfig.d/).
//
// RED stub: returns nil + sentinel. Plan 02 (05.7-02) implements the real body.
func ListCandidates(_ string) ([]string, error) {
	return nil, errors.New("adopter: not implemented")
}

// MatchIdentityName derives the candidate identity name from a fragment path.
// Convention: ~/.gitconfig_<name> → <name>.
//
// RED stub: returns empty + sentinel. Plan 02 (05.7-02) implements the real body.
func MatchIdentityName(_ string) (string, error) {
	return "", errors.New("adopter: not implemented")
}

// listCandidatesGlob is the internal implementation used by ListCandidates once
// it is wired; it globs ~/.gitconfig_* and filters already-managed names.
// Exported for use by tui/deps.go's ListCandidates closure.
func listCandidatesGlob(homeDir string) ([]string, error) {
	pattern := filepath.Join(homeDir, ".gitconfig_*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	if matches == nil {
		return []string{}, nil
	}
	return matches, nil
}

// ListCandidatesFromHome is the os.Glob-based implementation suitable for wiring
// in tui/deps.go buildTUIAdopterDeps. It wraps listCandidatesGlob so the live
// closure in tui/deps.go stays a single call.
func ListCandidatesFromHome(homeDir string) ([]string, error) {
	return listCandidatesGlob(homeDir)
}
