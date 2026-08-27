package globalgit

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// AuthorKeyResolution is one key's resolved value and origin from one
// working directory.
type AuthorKeyResolution struct {
	Value  string
	Origin string
}

// DirectoryResolution is the pair of author keys git resolved from one
// working directory.
type DirectoryResolution struct {
	Name  AuthorKeyResolution
	Email AuthorKeyResolution
}

// MatchedOutcome names how the matched-directory half of VerifyAuthorResolution
// concluded. An unverifiable check must never be indistinguishable from a
// passing one (D-06).
type MatchedOutcome int

const (
	// MatchedNotVerifiable means matchedDir was empty: this machine carries
	// no managed identity whose gitdir: pattern the ceremony could test.
	MatchedNotVerifiable MatchedOutcome = iota
	// MatchedVerified means the matched-directory read completed.
	MatchedVerified
)

// AuthorResolution is the D-06 post-write invariant probe: from a directory
// matching a managed identity the author should resolve to that identity's
// fragment, and from a directory matching no identity it should resolve to
// the fallback block. Read-only.
type AuthorResolution struct {
	MatchedOutcome MatchedOutcome
	Matched        DirectoryResolution
	Unmatched      DirectoryResolution
}

// VerifyAuthorResolution runs `git config --show-origin --get user.email`
// and `… --get user.name` with the working directory set to each of the two
// supplied directories, through the SAME injected RunGitConfig seam
// BuildProbeDeps established — `git -C <dir>` so no second process seam
// is opened. matchedDir empty means the matched half is not verifiable on
// this machine; the result records that as its own outcome rather than as
// a pass or a failure.
func VerifyAuthorResolution(deps Deps, matchedDir, unmatchedDir string) (AuthorResolution, error) {
	var out AuthorResolution
	if deps.RunGitConfig == nil {
		return out, fmt.Errorf("globalgit: VerifyAuthorResolution: RunGitConfig is nil")
	}
	if unmatchedDir == "" {
		return out, fmt.Errorf("globalgit: VerifyAuthorResolution: unmatchedDir is required")
	}
	unmatched, err := resolveAuthorFrom(deps, unmatchedDir)
	if err != nil {
		return out, err
	}
	out.Unmatched = unmatched
	if matchedDir == "" {
		out.MatchedOutcome = MatchedNotVerifiable
		return out, nil
	}
	matched, err := resolveAuthorFrom(deps, matchedDir)
	if err != nil {
		return out, err
	}
	out.Matched = matched
	out.MatchedOutcome = MatchedVerified
	return out, nil
}

func resolveAuthorFrom(deps Deps, dir string) (DirectoryResolution, error) {
	name, err := getAuthorKey(deps, dir, "user.name")
	if err != nil {
		return DirectoryResolution{}, err
	}
	email, err := getAuthorKey(deps, dir, "user.email")
	if err != nil {
		return DirectoryResolution{}, err
	}
	return DirectoryResolution{Name: name, Email: email}, nil
}

func getAuthorKey(deps Deps, dir, key string) (AuthorKeyResolution, error) {
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	out, err := deps.RunGitConfig(ctx, "-C", dir, "config", "--show-origin", "--get", key)
	if err != nil {
		if isGitConfigUnset(err) {
			return AuthorKeyResolution{}, nil
		}
		return AuthorKeyResolution{}, fmt.Errorf("globalgit: %s from %s: %w", key, dir, err)
	}
	return parseShowOriginGet(out), nil
}

// isGitConfigUnset reports git config --get's "key not found" exit (code 1).
func isGitConfigUnset(err error) bool {
	var exitErr *exec.ExitError
	return isExitErr(err, &exitErr) && exitErr.ExitCode() == 1
}

// parseShowOriginGet parses `git config --show-origin --get` stdout:
//
//	file:/path/to/config<TAB>value
//
// The file: prefix is stripped via the same helper the -z list parser uses.
func parseShowOriginGet(out string) AuthorKeyResolution {
	out = strings.TrimSpace(out)
	if out == "" {
		return AuthorKeyResolution{}
	}
	origin, value, ok := strings.Cut(out, "\t")
	if !ok {
		return AuthorKeyResolution{Value: out}
	}
	return AuthorKeyResolution{Value: value, Origin: stripFileOrigin(origin)}
}
