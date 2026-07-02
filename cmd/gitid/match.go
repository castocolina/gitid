package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/castocolina/gitid/internal/gitconfig"
)

// buildMatches constructs a []gitconfig.Match from the picker choice and
// supplied values. choice: "1"/"gitdir" → MatchGitdir; "2"/"url" → MatchHasconfig;
// "3"/"both" → [MatchGitdir, MatchHasconfig]; default → MatchGitdir.
//
// For hasconfig choices the stored Value is "remote.*.url:<urlPattern>" so it
// round-trips through conditionToMatch (strips "hasconfig:" → "remote.*.url:...").
// The caller passes the bare URL pattern (without the "remote.*.url:" prefix);
// buildMatches prepends it here (D-08, T-05.5-14 TOOL-04).
func buildMatches(choice, gitdirVal, urlVal string) []gitconfig.Match {
	switch strings.ToLower(strings.TrimSpace(choice)) {
	case "2", "url", "hasconfig":
		return []gitconfig.Match{
			{Kind: gitconfig.MatchHasconfig, Value: "remote.*.url:" + urlVal},
		}
	case "3", "both":
		return []gitconfig.Match{
			{Kind: gitconfig.MatchGitdir, Value: gitdirVal},
			{Kind: gitconfig.MatchHasconfig, Value: "remote.*.url:" + urlVal},
		}
	default: // "1", "gitdir", "" → gitdir (safe default, D-07)
		return []gitconfig.Match{
			{Kind: gitconfig.MatchGitdir, Value: gitdirVal},
		}
	}
}

// matchKinds returns the canonical strategy name for a []Match:
// "gitdir", "hasconfig", "both", or "gitdir" (default) when empty.
func matchKinds(matches []gitconfig.Match) string {
	var hasGitdir, hasHasconfig bool
	for _, m := range matches {
		switch m.Kind {
		case gitconfig.MatchGitdir:
			hasGitdir = true
		case gitconfig.MatchHasconfig:
			hasHasconfig = true
		}
	}
	switch {
	case hasGitdir && hasHasconfig:
		return "both"
	case hasHasconfig:
		return "hasconfig"
	default: // gitdir only or empty — default is gitdir (D-07)
		return "gitdir"
	}
}

// defaultURLPattern returns the suggested hasconfig URL pattern for a given
// hostname and identity name: git@<hostname>:<name>/** (D-08).
func defaultURLPattern(hostname, name string) string {
	return "git@" + hostname + ":" + name + "/**"
}

// matchFromFlag maps a --match flag value to the strategy number string used
// by buildMatches. Accepts "gitdir" or "" → "1", "hasconfig" → "2", "both" → "3".
// Returns an error for any other value (T-05.7-05-02: whitelist enforcement).
func matchFromFlag(s string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "gitdir", "":
		return "1", nil
	case "hasconfig":
		return "2", nil
	case "both":
		return "3", nil
	default:
		return "", fmt.Errorf("--match: unknown strategy %q (allowed: gitdir, hasconfig, both)", s)
	}
}

// strategyNumFromKind converts a matchKinds string ("gitdir"/"hasconfig"/"both")
// to the numeric picker default ("1"/"2"/"3") for update pre-fill (D-09).
func strategyNumFromKind(kind string) string {
	switch kind {
	case "hasconfig":
		return "2"
	case "both":
		return "3"
	default:
		return "1"
	}
}

// promptMatchStrategy prints the 3-option match-strategy menu, reads the user's
// choice (pre-filled with strategyDefault: "1"/"2"/"3"), prompts only for the
// needed value(s) with the supplied defaults, and returns
// buildMatches(choice, gitdirVal, urlVal).
//
// gitdirDefault is the pre-filled gitdir value (e.g. "~/git/<name>/").
// urlDefault is the pre-filled URL pattern (e.g. "git@ssh.github.com:<name>/**").
// strategyDefault is the pre-selected strategy number ("1"/"2"/"3"); pass ""
// for the interactive-add case which defaults to "1" (gitdir, D-07).
func promptMatchStrategy(r *bufio.Reader, out io.Writer, gitdirDefault, urlDefault string, strategyDefault ...string) []gitconfig.Match {
	fp(out, "Match strategy:\n")
	fp(out, "  1) folder (gitdir)      — activate identity for all repos under a directory (default)\n")
	fp(out, "  2) repo URL (hasconfig) — activate identity for repos matching a remote URL pattern\n")
	fp(out, "  3) both                 — activate by folder AND URL (OR-applied by git)\n")

	defStrategy := "1"
	if len(strategyDefault) > 0 && strategyDefault[0] != "" {
		defStrategy = strategyDefault[0]
	}
	choice := strings.TrimSpace(prompt(r, out, "Strategy", defStrategy))

	var gitdirVal, urlVal string
	switch strings.ToLower(choice) {
	case "2", "url", "hasconfig":
		urlVal = prompt(r, out, "URL pattern", urlDefault)
	case "3", "both":
		gitdirVal = prompt(r, out, "Match gitdir", gitdirDefault)
		urlVal = prompt(r, out, "URL pattern", urlDefault)
	default: // "1", "", gitdir
		choice = "1"
		gitdirVal = prompt(r, out, "Match gitdir", gitdirDefault)
	}

	return buildMatches(choice, gitdirVal, urlVal)
}
