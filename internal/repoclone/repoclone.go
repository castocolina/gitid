// Package repoclone clones or pulls a remote repository using the matching
// gitid identity's SSH alias. All external effects (filesystem stat, git exec)
// are injected via Deps so this package is testable without real network or
// filesystem operations.
package repoclone

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Deps holds all external effects. Build live in tui/deps.go and cmd/gitid/addrepo.go;
// pass fakes in tests. Every function field must be non-nil (wiring guard in
// tui/wiring_test.go TestBuildTUIDepsNilGuard_Phase57).
type Deps struct {
	// Stat checks destination existence before clone. Wire to: os.Stat
	Stat func(path string) (os.FileInfo, error)
	// Clone runs git clone. Wire live via buildTUIRepoCloneDeps in tui/deps.go.
	Clone func(cloneURL, destPath string) ([]string, error)
	// Pull runs git -C destPath pull. Wire live via buildTUIRepoCloneDeps.
	Pull func(destPath string) ([]string, error)
	// UserHomeDir resolves the base clone directory. Wire to: os.UserHomeDir
	UserHomeDir func() (string, error)
}

// ErrDestExists is returned when the clone destination already exists.
var ErrDestExists = errors.New("repoclone: destination already exists")

// ErrUnknownProvider is returned when ProviderFromURL cannot parse the provider
// hostname from the given URL.
var ErrUnknownProvider = errors.New("repoclone: unknown provider")

// ProviderFromURL extracts the provider hostname from https:// or git@<host>:
// URLs. Returns ("", ErrUnknownProvider) when the URL cannot be parsed.
//
// RED stub: returns empty + sentinel. Plan 03 (05.7-03) implements the real body.
func ProviderFromURL(_ string) (string, error) {
	return "", errors.New("repoclone: not implemented")
}

// RewriteToAlias rewrites an HTTPS github.com URL to its SSH alias form using
// the identity's configured SSH alias (e.g. personal.github.com).
//
// RED stub: returns empty + sentinel. Plan 03 (05.7-03) implements the real body.
func RewriteToAlias(_, _ string) (string, error) {
	return "", errors.New("repoclone: not implemented")
}

// DestPath computes the local clone destination under ~/git/<client>/<reponame>.
//
// RED stub: returns empty + sentinel. Plan 03 (05.7-03) implements the real body.
func DestPath(_, _, _ string) (string, error) {
	return "", errors.New("repoclone: not implemented")
}

// Clone clones cloneURL to destPath using deps. Returns ErrDestExists if the
// destination already exists.
//
// RED stub: returns nil + sentinel. Plan 03 (05.7-03) implements the real body.
func Clone(_, _ string, _ Deps) ([]string, error) {
	return nil, errors.New("repoclone: not implemented")
}

// Pull runs git pull inside destPath.
//
// RED stub: returns nil + sentinel. Plan 03 (05.7-03) implements the real body.
func Pull(_ string, _ Deps) ([]string, error) {
	return nil, errors.New("repoclone: not implemented")
}

// liveClone is the internal git clone implementation used by buildTUIRepoCloneDeps.
// Uses arg-slice form (no shell) — G204-clean.
func liveClone(cloneURL, destPath string) ([]string, error) {
	cmd := exec.Command("git", "clone", cloneURL, destPath) //nolint:gosec // arg-slice; no shell; URL is gitid-validated (G204)
	out, err := cmd.CombinedOutput()
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if err != nil {
		return lines, fmt.Errorf("repoclone: git clone: %w", err)
	}
	return lines, nil
}

// livePull is the internal git pull implementation used by buildTUIRepoCloneDeps.
// Uses arg-slice form (no shell) — G204-clean.
func livePull(destPath string) ([]string, error) {
	cmd := exec.Command("git", "-C", destPath, "pull") //nolint:gosec // arg-slice; destPath is gitid-derived (G204)
	out, err := cmd.CombinedOutput()
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if err != nil {
		return lines, fmt.Errorf("repoclone: git pull: %w", err)
	}
	return lines, nil
}

// LiveClone and LivePull are exported for use in tui/deps.go and cmd/gitid/addrepo.go
// wiring functions.
var (
	LiveClone = liveClone
	LivePull  = livePull
)
