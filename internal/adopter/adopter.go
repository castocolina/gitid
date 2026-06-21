// Package adopter detects plain-style gitconfig fragments (~/.gitconfig_<name>)
// that are not yet managed by gitid and offers to migrate or reference them.
// All external effects (file reads, copies, writes) are injected via Deps so
// this package is testable without real filesystem operations.
package adopter

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
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
// It NEVER removes the source file; callers perform removal as a separate explicit
// step only after a successful Adopt and explicit user confirmation (D-05).
//
// For AdoptMigrate: copies the fragment to ~/.gitconfig.d/<identityName> then
// writes the managed includeIf block pointing at the new location.
// For AdoptReferenceInPlace: skips the copy and writes the includeIf pointing at
// the original sourcePath.
//
// gitconfigPath is passed to the live WriteIncludeIf seam closure in
// tui/deps.go and cmd/gitid/adopt.go; the seam captures it so Adopt stays
// dependency-free from gitconfigPath at call sites.
func Adopt(sourcePath, identityName, gitconfigPath string, method AdoptMethod, matches []gitconfig.Match, deps Deps) (AdoptResult, error) {
	// Validate identityName: reject newlines and invalid charset (T-05.7-02-01).
	if err := identity.ValidateName(identityName); err != nil {
		return AdoptResult{}, fmt.Errorf("adopter: invalid name: %w", err)
	}

	var result AdoptResult

	switch method {
	case AdoptMigrate:
		// Build destination path: ~/.gitconfig.d/<identityName>
		// (gitconfigPath is ~/.gitconfig; its directory is ~/ so ~/.gitconfig.d/ is a sibling)
		gitconfigDir := filepath.Dir(gitconfigPath)
		destPath := filepath.Join(gitconfigDir, ".gitconfig.d", identityName)

		// Copy the source fragment into the managed directory.
		if err := deps.CopyFile(sourcePath, destPath); err != nil {
			return AdoptResult{}, fmt.Errorf("adopter: copying fragment: %w", err)
		}
		result.MigratedPath = destPath

		// Write the includeIf block pointing at the new location.
		backupPath, err := deps.WriteIncludeIf(identityName, destPath, matches)
		if err != nil {
			return AdoptResult{}, fmt.Errorf("adopter: writing includeIf: %w", err)
		}
		if backupPath != "" {
			result.BackupPaths = append(result.BackupPaths, backupPath)
		}

	case AdoptReferenceInPlace:
		// Write the includeIf block pointing at the original path — no copy.
		backupPath, err := deps.WriteIncludeIf(identityName, sourcePath, matches)
		if err != nil {
			return AdoptResult{}, fmt.Errorf("adopter: writing includeIf: %w", err)
		}
		if backupPath != "" {
			result.BackupPaths = append(result.BackupPaths, backupPath)
		}

	default:
		return AdoptResult{}, fmt.Errorf("adopter: unknown adopt method %d", method)
	}

	return result, nil
}

// ListCandidates returns paths matching ~/.gitconfig_* that are not yet
// managed by gitid. It globs ~/.gitconfig_* from homeDir and filters out:
//   - any path under ~/.gitconfig.d/ (already-managed fragments)
//   - any path whose name suffix (after "gitconfig_") is in managedNames
//
// Returns an empty slice (not an error) when no candidates exist.
// Built from scratch via filepath.Glob — does NOT rely on CheckOrphans (premise
// correction REVIEWS.md #1: the orphan checker never globs ~/.gitconfig_*).
func ListCandidates(homeDir string, managedNames []string) ([]string, error) {
	pattern := filepath.Join(homeDir, ".gitconfig_*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("adopter: globbing candidates: %w", err)
	}

	// Build a set of already-managed name suffixes for O(1) lookup.
	managed := make(map[string]bool, len(managedNames))
	for _, n := range managedNames {
		managed[n] = true
	}

	gitconfigDDir := filepath.Join(homeDir, ".gitconfig.d") + string(filepath.Separator)

	var candidates []string
	for _, m := range matches {
		// Skip any path inside ~/.gitconfig.d/ (already managed fragments).
		if strings.HasPrefix(m, gitconfigDDir) {
			continue
		}
		// Derive the suffix after ".gitconfig_".
		base := filepath.Base(m)
		suffix, ok := strings.CutPrefix(base, ".gitconfig_")
		if !ok || suffix == "" {
			continue
		}
		// Skip names already under gitid management.
		if managed[suffix] {
			continue
		}
		candidates = append(candidates, m)
	}

	if candidates == nil {
		return []string{}, nil
	}
	return candidates, nil
}

// MatchIdentityName derives the best-match identity name for a fragment file.
//
// Resolution order:
//  1. Filename suffix: ~/.gitconfig_<suffix> where suffix is in knownNames → return suffix.
//  2. Email fallback: read the fragment via readFragment, look up the email in
//     accountEmails (email → identity name) → return the matched name.
//  3. Neither match → return ("", ErrAmbiguousIdentity).
//
// A symlink at sourcePath is rejected via os.Lstat guard (T-05.7-02-02): a
// symlink pointing outside the home directory is a security risk.
func MatchIdentityName(sourcePath string, knownNames []string, accountEmails map[string]string, readFragment func(string) (gitconfig.FragmentInfo, error)) (string, error) {
	// Symlink guard: reject symlinks to prevent path-traversal adoption (T-05.7-02-02).
	lstat, err := os.Lstat(sourcePath) //nolint:gosec // sourcePath is a gitid-derived candidate path returned by filepath.Glob (G304)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("adopter: stat candidate %s: %w", sourcePath, err)
	}
	if err == nil && lstat.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("adopter: candidate %s is a symlink — adoption of symlinks is not supported (T-05.7-02-02)", sourcePath)
	}

	// 1. Filename suffix match.
	base := filepath.Base(sourcePath)
	suffix, ok := strings.CutPrefix(base, ".gitconfig_")
	if ok && suffix != "" {
		for _, name := range knownNames {
			if name == suffix {
				return name, nil
			}
		}
	}

	// 2. Email fallback via fragment read.
	if readFragment != nil {
		info, fragErr := readFragment(sourcePath)
		if fragErr == nil && !info.Missing && info.GitEmail != "" {
			if name, found := accountEmails[info.GitEmail]; found {
				return name, nil
			}
		}
	}

	// 3. No match found.
	return "", ErrAmbiguousIdentity
}

// ListCandidatesFromHome is the os.Glob-based implementation suitable for wiring
// in tui/deps.go buildTUIAdopterDeps. It wraps ListCandidates with an empty
// managedNames set so the live closure in tui/deps.go stays a single call.
// The caller is responsible for passing managed names when needed; this form is
// used when the TUI wires candidates without a managed-names filter.
func ListCandidatesFromHome(homeDir string) ([]string, error) {
	return ListCandidates(homeDir, nil)
}
