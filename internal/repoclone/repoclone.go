// Package repoclone clones or pulls a remote repository using the matching
// gitid identity's SSH alias. All external effects (filesystem stat, git exec)
// are injected via Deps so this package is testable without real network or
// filesystem operations.
package repoclone

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
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

// ErrDestOutsideBase is returned when the computed destPath does not lie under
// <home>/git, indicating a path traversal attempt.
var ErrDestOutsideBase = errors.New("repoclone: destination is outside allowed base directory")

// ErrOptionLikeURL is returned when cloneURL begins with "-", which git would
// interpret as a command-line flag (argv flag smuggling) even in arg-slice form.
var ErrOptionLikeURL = errors.New("repoclone: refusing clone URL that begins with '-'")

// ProviderFromURL extracts the provider hostname from https://, git@host:, or
// ssh:// URLs. Returns ("", ErrUnknownProvider) when the URL cannot be parsed.
//
// Supported forms:
//   - https://github.com/org/repo.git
//   - git@github.com:org/repo.git   (SCP-like, no scheme)
//   - ssh://git@gitlab.example.com:443/org/repo
func ProviderFromURL(rawURL string) (string, error) {
	// SCP-like form: git@host:path (no scheme, colon separates host from path)
	if !strings.Contains(rawURL, "://") {
		// Must contain "@" and ":" to be a valid SCP-like URL
		atIdx := strings.Index(rawURL, "@")
		colonIdx := strings.Index(rawURL, ":")
		if atIdx >= 0 && colonIdx > atIdx {
			host := rawURL[atIdx+1 : colonIdx]
			if host != "" {
				return host, nil
			}
		}
		return "", ErrUnknownProvider
	}

	// Scheme-based URL (https://, ssh://, git://, etc.)
	u, err := url.Parse(rawURL)
	if err != nil || u.Hostname() == "" {
		return "", ErrUnknownProvider
	}
	return u.Hostname(), nil
}

// RewriteToAlias rewrites an HTTPS or SCP-form URL to the SSH alias form:
// git@<alias>:<org>/<repo>[.git]. The alias is the gitid SSH alias for the
// provider (e.g. "personal.github.com" from the SSH config Host block).
//
// Recipe form: git@personal.github.com:org/repo.git
func RewriteToAlias(rawURL, alias string) (string, error) {
	orgRepo, hasDotGit, err := extractOrgRepo(rawURL)
	if err != nil {
		return "", fmt.Errorf("repoclone: rewrite alias: %w", err)
	}
	suffix := ""
	if hasDotGit {
		suffix = ".git"
	}
	return fmt.Sprintf("git@%s:%s%s", alias, orgRepo, suffix), nil
}

// extractOrgRepo returns the "org/repo" path component (without leading slash,
// without .git suffix) and whether the original URL had a .git suffix.
func extractOrgRepo(rawURL string) (orgRepo string, hasDotGit bool, err error) {
	var pathPart string

	if !strings.Contains(rawURL, "://") {
		// SCP-like: git@host:org/repo.git
		colonIdx := strings.Index(rawURL, ":")
		if colonIdx < 0 {
			return "", false, ErrUnknownProvider
		}
		pathPart = rawURL[colonIdx+1:]
	} else {
		u, parseErr := url.Parse(rawURL)
		if parseErr != nil || u.Hostname() == "" {
			return "", false, ErrUnknownProvider
		}
		pathPart = strings.TrimPrefix(u.Path, "/")
	}

	hasDotGit = strings.HasSuffix(pathPart, ".git")
	orgRepo = strings.TrimSuffix(pathPart, ".git")
	if orgRepo == "" {
		return "", false, ErrUnknownProvider
	}
	return orgRepo, hasDotGit, nil
}

// DestPath computes the local clone destination under baseDir/client/reponame.
// baseDir is the expanded ~/git path (caller supplies it, typically
// filepath.Join(home, "git")). The function is pure: no exec, no filesystem.
//
// reponame = last path segment of rawURL, with .git suffix stripped.
func DestPath(baseDir, client, rawURL string) (string, error) {
	repoName, err := repoNameFromURL(rawURL)
	if err != nil {
		return "", fmt.Errorf("repoclone: dest path: %w", err)
	}
	return filepath.Join(baseDir, client, repoName), nil
}

// repoNameFromURL extracts the repository name (last path segment, .git stripped).
func repoNameFromURL(rawURL string) (string, error) {
	var pathPart string

	if !strings.Contains(rawURL, "://") {
		// SCP-like: git@host:org/repo.git
		colonIdx := strings.Index(rawURL, ":")
		if colonIdx < 0 {
			return "", ErrUnknownProvider
		}
		pathPart = rawURL[colonIdx+1:]
	} else {
		u, err := url.Parse(rawURL)
		if err != nil || u.Hostname() == "" {
			return "", ErrUnknownProvider
		}
		pathPart = u.Path
	}

	// Strip leading slash and .git suffix, take the last segment
	pathPart = strings.TrimPrefix(pathPart, "/")
	pathPart = strings.TrimSuffix(pathPart, ".git")
	segments := strings.Split(pathPart, "/")
	name := segments[len(segments)-1]
	if name == "" {
		return "", ErrUnknownProvider
	}
	return name, nil
}

// Clone clones cloneURL to destPath using deps.
//
// Guards (in order, before any exec):
//  1. ErrDestOutsideBase — destPath does not lie under <home>/git
//  2. ErrDestExists — destPath already exists (deps.Stat returns non-error)
//
// The allowed base is derived internally: deps.UserHomeDir() + "/git".
// Clone does NOT accept a base-dir parameter (REVIEWS.md #10).
func Clone(cloneURL, destPath string, deps Deps) ([]string, error) {
	// Derive allowed base internally from the injected UserHomeDir seam.
	home, err := deps.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("repoclone: resolving home dir: %w", err)
	}
	base := filepath.Join(home, "git")

	// Guard 0: option-like URL — reject a cloneURL that begins with "-" before any
	// exec. Even in arg-slice (no-shell) form, git treats a leading-"-" argument as
	// a flag, so a URL like "--upload-pack=..." would smuggle a flag (argv flag
	// smuggling). liveClone also inserts a "--" separator as defense in depth.
	if strings.HasPrefix(cloneURL, "-") {
		return nil, ErrOptionLikeURL
	}

	// Guard 1: dest-outside-base — reject path traversal before any I/O.
	cleanDest := filepath.Clean(destPath)
	rel, err := filepath.Rel(base, cleanDest)
	if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return nil, ErrDestOutsideBase
	}

	// Guard 2: dest-exists — reject if the destination already exists.
	if _, statErr := deps.Stat(destPath); statErr == nil {
		return nil, ErrDestExists
	} else if !os.IsNotExist(statErr) {
		return nil, fmt.Errorf("repoclone: stat dest: %w", statErr)
	}

	// Delegate to the injected clone function (liveClone in production).
	return deps.Clone(cloneURL, destPath)
}

// Pull runs git pull inside destPath via the injected deps.Pull function.
func Pull(destPath string, deps Deps) ([]string, error) {
	return deps.Pull(destPath)
}

// liveClone is the internal git clone implementation used by buildTUIRepoCloneDeps.
// Uses arg-slice form (no shell) — G204-clean.
func liveClone(cloneURL, destPath string) ([]string, error) {
	// "--" terminates option parsing so a leading-"-" URL/dest can never be read as
	// a git flag (argv flag smuggling), even though Clone already rejects such URLs.
	cmd := exec.Command("git", "clone", "--", cloneURL, destPath) //nolint:gosec // arg-slice; no shell; "--" guard; URL/dest validated (G204)
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
