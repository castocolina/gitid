package globalgit

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// probeTimeout bounds every real `git config` invocation so a pathological
// configuration can never block gitid indefinitely (T-07-07). It is a var,
// not a const, so tests can shrink it to exercise timeout behavior.
var probeTimeout = 5 * time.Second

// EffectiveEntry is one key→value pair from the effective-configuration probe,
// together with the scope and origin git reported.
type EffectiveEntry struct {
	// Value is the effective value for this key.
	Value string
	// Scope is the scope word git printed (e.g. "global", "local", "system",
	// "command").
	Scope string
	// Origin is the file path (with the "file:" prefix stripped), or a
	// verbatim non-file origin string such as "command line" or "standard
	// input" (git also emits "blob:<sha>" for worktree configs).
	Origin string
}

// Deps is every external effect the probe set needs, injected as function
// fields so the probes are fully mockable in tests. Every field is non-nil in
// the real BuildProbeDeps wiring and in test fakes, closing the project's
// documented injected-seam wiring blindspot.
type Deps struct {
	// RunGitConfig executes one `git config` probe with the given argument
	// SLICE and returns its stdout. It is bounded by probeTimeout and never
	// runs through a shell (T-07-07).
	RunGitConfig func(ctx context.Context, args ...string) (string, error)
	// NonRepoCwd is the directory the effective-values probe runs from. It
	// MUST NOT be inside any git repository — running from inside a repo
	// folds repo-local and worktree values into the same output and gitid
	// would report a repo's setting as the machine's global one (T-07-06,
	// the git-side analog of globalssh's Phase 6 D-05 isolated-probe
	// discipline). The caller (cmd/gitid/wiring.go) supplies a directory
	// it controls; tests use t.TempDir().
	NonRepoCwd string
}

// BuildProbeDeps returns production Deps wired to the REAL `git` binary and a
// caller-supplied non-repository working directory. It is exported and
// real-wired because this project has a documented injected-seam wiring
// blindspot where a nil or fixture seam reaches production; it mirrors
// globalssh.BuildProbeDeps's shape.
func BuildProbeDeps(nonRepoCwd string) Deps {
	return Deps{
		RunGitConfig: func(ctx context.Context, args ...string) (string, error) {
			if ctx == nil {
				ctx = context.Background()
			}
			cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec // arg-slice form, no shell; args are fixed probe flags (G204)
			cmd.Dir = nonRepoCwd
			out, err := cmd.Output()
			return string(out), err
		},
		NonRepoCwd: nonRepoCwd,
	}
}

// effectiveProbe runs `git config --show-origin --show-scope --list -z` from
// deps.NonRepoCwd (which MUST NOT be inside any git repository — see Deps
// doc comment and T-07-06) and returns a map of lowercase key → EffectiveEntry.
//
// The -z flag is used because it is the only unambiguous layout for values
// that may contain tabs, newlines, or equals signs (empirically verified at
// planning time against git 2.55). The record layout is:
//
//	<scope> NUL <origin> NUL <key> LF <value> NUL
//
// This layout is pinned by a golden-literal test in probe_test.go.
//
// When a key appears more than once, the LAST record wins, matching git's own
// last-wins resolution. The "file:" prefix is stripped from the origin when
// present; other origins (e.g. "command line", "blob:<sha>", "standard
// input") are preserved verbatim and treated as scopes gitid cannot change.
//
// GIT_CONFIG_NOSYSTEM is deliberately NOT set: the system scope is real and
// must be visible, because a system-set value is exactly the "set at a scope
// gitid cannot change" provenance class.
func effectiveProbe(deps Deps) (map[string]EffectiveEntry, error) {
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	out, err := deps.RunGitConfig(ctx,
		"config", "--show-origin", "--show-scope", "--list", "-z")
	if err != nil {
		return nil, fmt.Errorf("globalgit: effective probe: git config --show-origin --show-scope --list -z: %w", err)
	}
	return parseNULRecords(out), nil
}

// inFileProbe runs `git config --file <path> --list -z` WITHOUT --includes,
// so it reports only keys physically present in that file — the physical
// presence signal that decides set-by-gitid attribution (plan 07-01) and the
// bundle aggregate's set/differs counts (plan 07-03, bundle.go). It replaces
// the retired conflict-scan contract in internal/gitconfig, which previously
// shelled a second `git config --file --list` on a temp file; the two probes
// already taken here carry strictly more information (the effective value, its
// origin, AND physical presence).
// A missing file is not an error — it returns an empty result (the first-run
// case, when the baseline file has not yet been written).
func inFileProbe(deps Deps, filePath string) (map[string]EffectiveEntry, error) {
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	out, err := deps.RunGitConfig(ctx, "config", "--file", filePath, "--list", "-z")
	if err != nil {
		// git config --file <missing> exits 128 ("fatal: …") or 1.
		// We treat any exit as "missing file" when the output is empty and
		// err is a missing-file indicator. A missingFileError (test fake)
		// or an exec.ExitError with code 128 both signal "file not found".
		if isMissingFileErr(err, out) {
			return map[string]EffectiveEntry{}, nil
		}
		return nil, fmt.Errorf("globalgit: in-file probe: git config --file %s --list -z: %w", filePath, err)
	}
	return parseNULRecords(out), nil
}

// isMissingFileErr returns true when the error looks like "git config --file
// <missing>" — git exits 128 when the file does not exist.
func isMissingFileErr(err error, output string) bool {
	if err == nil {
		return false
	}
	// Check if the error type implements a testable IsMissingFile method.
	type missingFileSentinel interface {
		IsMissingFile() bool
	}
	if mf, ok := err.(missingFileSentinel); ok && mf.IsMissingFile() {
		return true
	}
	// Real git: exit status 128 with empty output means file not found.
	var exitErr *exec.ExitError
	if isExitErr(err, &exitErr) {
		if exitErr.ExitCode() == 128 && strings.TrimSpace(output) == "" {
			return true
		}
	}
	return false
}

// isExitErr checks if err is an *exec.ExitError and populates target.
func isExitErr(err error, target **exec.ExitError) bool {
	ee, ok := err.(*exec.ExitError)
	if ok {
		*target = ee
	}
	return ok
}

// parseNULRecords parses `git config --show-origin --show-scope --list -z`
// output into a map of lowercase key → EffectiveEntry. The -z record layout
// is:
//
//	<scope> NUL <origin> NUL <key> LF <value> NUL
//
// When a key appears more than once, the LAST record wins (git's own
// last-wins resolution). The "file:" prefix is stripped from the origin when
// present.
func parseNULRecords(out string) map[string]EffectiveEntry {
	result := make(map[string]EffectiveEntry)
	if out == "" {
		return result
	}

	// Records are NUL-terminated.
	// Each record: <scope> NUL <origin> NUL <key> LF <value> NUL
	// We split on the record separator (final NUL of each record) first.
	// Since the entire output is a sequence of such records, we can split
	// by NUL and process tuples.
	parts := strings.Split(out, "\x00")

	i := 0
	for i < len(parts) {
		// Each record occupies 4 NUL-separated slots when split:
		// [scope, origin, "key\nvalue", ""]
		// But since "key LF value" is itself a NUL-terminated token, the
		// actual split gives us:
		// parts[i]   = scope
		// parts[i+1] = origin
		// parts[i+2] = "key\nvalue"  (key LF value — key contains no NUL)
		// After the last record there may be a trailing empty string.
		if i+2 >= len(parts) {
			break
		}

		scope := parts[i]
		origin := parts[i+1]
		keyValue := parts[i+2]
		i += 3

		// Skip empty records.
		if scope == "" && origin == "" && keyValue == "" {
			continue
		}

		// Split "key\nvalue" on the FIRST newline only (value may contain
		// further newlines, which is exactly why we use -z).
		nlIdx := strings.IndexByte(keyValue, '\n')
		if nlIdx < 0 {
			// Malformed record — skip.
			continue
		}
		key := strings.ToLower(keyValue[:nlIdx])
		value := keyValue[nlIdx+1:]

		origin = stripFileOrigin(origin)

		// Last-wins: overwrite any earlier entry for the same key.
		result[key] = EffectiveEntry{
			Value:  value,
			Scope:  scope,
			Origin: origin,
		}
	}

	return result
}

// stripFileOrigin strips git's "file:" origin prefix when present, leaving
// other origins (command line, blob:, standard input) verbatim. Shared by
// the -z list parser and VerifyAuthorResolution's --show-origin --get parser
// so the prefix rule cannot drift (D-06).
func stripFileOrigin(origin string) string {
	return strings.TrimPrefix(origin, "file:")
}
