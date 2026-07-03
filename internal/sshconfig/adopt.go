package sshconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/castocolina/gitid/internal/filewriter"
)

// IncludeDirective is one `Include` directive found in ~/.ssh/config, in file
// order. Raw is the path token exactly as written (quotes stripped); Expanded
// is the ~/absolute-expanded form used for filesystem comparison; Quoted
// records whether the source token was double-quoted.
type IncludeDirective struct {
	Raw      string
	Expanded string
	Quoted   bool
}

// AdoptMethod selects how Adopt evaluates DetectInclude's candidates.
type AdoptMethod int

const (
	// AdoptSentinelBearing auto-detects a candidate whose resolved path
	// already contains a gitid-managed sentinel block (a gitid-owned
	// target). The candidate must be unambiguous: its glob must resolve to
	// exactly one file.
	AdoptSentinelBearing AdoptMethod = iota
	// AdoptCallerChosen selects a candidate the caller has explicitly
	// confirmed (chosenPath), bypassing the sentinel-bearing/unambiguous
	// auto-detection requirement — used when a broad glob resolves to
	// multiple or non-gitid files and the caller unambiguously picks one.
	AdoptCallerChosen
	// AdoptCreateConfigD skips Include detection entirely and falls back to
	// creating the gitid-owned config.d layout (EnsureIncludeDir +
	// EnsureIncludeLine) instead of adopting any existing Include.
	AdoptCreateConfigD
)

// AdoptDeps holds all external effects Adopt needs, injectable for tests.
// Every function field must be non-nil in production wiring (RealAdoptDeps).
type AdoptDeps struct {
	// ReadFile reads a candidate file's content, used to check for a gitid
	// sentinel block. Wired to os.ReadFile in production.
	ReadFile func(path string) ([]byte, error)
	// Glob expands a path pattern to matching filesystem paths, in the same
	// glob semantics kevinburke/ssh_config's own Include resolution uses.
	// Wired to filepath.Glob in production.
	Glob func(pattern string) ([]string, error)
	// Lstat performs a symlink-aware stat (no dereference) — the guard that
	// rejects a symlinked adoption target (path-traversal mitigation,
	// mirroring the sibling gitconfig-fragment adopter's symlink guard).
	// Wired to os.Lstat in production.
	Lstat func(path string) (os.FileInfo, error)
}

// RealAdoptDeps returns production AdoptDeps wired to the real filesystem —
// the live constructor for cmd-layer callers.
func RealAdoptDeps() AdoptDeps {
	return AdoptDeps{
		ReadFile: os.ReadFile,
		Glob:     filepath.Glob,
		Lstat:    os.Lstat,
	}
}

// AdoptResult carries the outcome of an Adopt call.
type AdoptResult struct {
	// TargetPath is the resolved Include'd file selected as the write
	// target. Empty when Method is AdoptCreateConfigD — callers create
	// config.d/gitid.config via EnsureIncludeDir/EnsureIncludeLine instead.
	TargetPath string
	// Method records which selection path produced this result.
	Method AdoptMethod
}

// DetectInclude scans configPath's raw text for every `Include` directive
// line and returns each path token, in file order (first-match-wins order
// preserved across multiple Include lines).
//
// This is a deliberate pure text scan, NOT ssh_config.Decode — Decode
// performs real filesystem I/O (glob + read) as a side effect of resolving
// Include directives (Pitfall 5), which DetectInclude must not trigger merely
// to discover directive order/paths; Adopt performs its own controlled,
// injectable glob resolution instead.
//
// A missing configPath returns (nil, nil) — no directive is not an error
// (STORE-02's no-Include-directive case).
func DetectInclude(configPath string) ([]IncludeDirective, error) {
	content, err := os.ReadFile(configPath) //nolint:gosec // configPath is a trusted gitid-managed path supplied in-process
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("sshconfig: reading %s: %w", configPath, err)
	}

	var result []IncludeDirective
	for _, line := range strings.Split(string(content), "\n") {
		trimmed := strings.TrimSpace(line)
		rest, ok := includeDirectiveArgs(trimmed)
		if !ok {
			continue
		}
		for _, tok := range tokenizeIncludeArgs(rest) {
			result = append(result, IncludeDirective{
				Raw:      tok.raw,
				Expanded: expandIncludePath(tok.raw),
				Quoted:   tok.quoted,
			})
		}
	}
	return result, nil
}

// includeDirectiveArgs reports whether trimmed is an `Include` directive line
// (OpenSSH keyword matching is case-insensitive) and, if so, returns the
// remaining argument text (everything after the keyword and an optional `=`).
func includeDirectiveArgs(trimmed string) (string, bool) {
	const keyword = "include"
	if len(trimmed) < len(keyword) {
		return "", false
	}
	if !strings.EqualFold(trimmed[:len(keyword)], keyword) {
		return "", false
	}
	rest := trimmed[len(keyword):]
	if rest == "" {
		return "", false // "Include" with nothing following is not a directive
	}
	if rest[0] != ' ' && rest[0] != '\t' && rest[0] != '=' {
		return "", false // e.g. "IncludeSomethingElse" must not match
	}
	rest = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(rest), "="))
	return strings.TrimSpace(rest), true
}

// includeToken is one whitespace/quote-delimited argument token from an
// Include directive's argument list.
type includeToken struct {
	raw    string
	quoted bool
}

// tokenizeIncludeArgs splits an Include directive's argument text into
// tokens, honouring double-quoted paths (which may contain no unescaped
// whitespace splitting) — OpenSSH permits multiple space-separated globs per
// Include line.
func tokenizeIncludeArgs(s string) []includeToken {
	runes := []rune(s)
	n := len(runes)
	var tokens []includeToken
	i := 0
	for i < n {
		for i < n && (runes[i] == ' ' || runes[i] == '\t') {
			i++
		}
		if i >= n {
			break
		}
		if runes[i] == '"' {
			i++
			start := i
			for i < n && runes[i] != '"' {
				i++
			}
			tokens = append(tokens, includeToken{raw: string(runes[start:i]), quoted: true})
			if i < n {
				i++ // skip closing quote
			}
			continue
		}
		start := i
		for i < n && runes[i] != ' ' && runes[i] != '\t' {
			i++
		}
		tokens = append(tokens, includeToken{raw: string(runes[start:i])})
	}
	return tokens
}

// isAcceptablePathForm reports whether raw is a form gitid will consider
// adopting: absolute, or explicitly `~/.ssh`-relative. Any other relative
// path (bare-relative, or `~`-relative outside `~/.ssh`) is rejected at the
// boundary — a defensive gitid-specific safety rule, stricter than what real
// OpenSSH itself would resolve (OpenSSH treats a bare-relative Include path
// as implicitly `~/.ssh`-relative too, but gitid does not auto-adopt on that
// assumption).
func isAcceptablePathForm(raw string) bool {
	return filepath.IsAbs(raw) || strings.HasPrefix(raw, "~/.ssh/")
}

// expandIncludePath expands raw into an absolute filesystem path for
// comparison/globbing, consistently for absolute, `~/`-relative, and
// bare-relative forms. Bare-relative expansion mirrors OpenSSH/kevinburke's
// own convention (relative to `~/.ssh/`) purely for a non-nil, informative
// Expanded value — isAcceptablePathForm is what actually gates adoption.
//
// os.UserHomeDir (not os/user.Current, which ignores $HOME on darwin) is used
// so tests can pin expansion to a hermetic t.TempDir() HOME.
func expandIncludePath(raw string) string {
	home, _ := os.UserHomeDir()
	switch {
	case filepath.IsAbs(raw):
		return raw
	case strings.HasPrefix(raw, "~/"):
		return filepath.Join(home, raw[2:])
	default:
		return filepath.Join(home, ".ssh", raw)
	}
}

// Adopt selects a write target for STORE-02 adoption from configPath's
// detected Include directives, applying the selection rules: a candidate is
// adoptable only if (a) its path is absolute or `~/.ssh`-relative, (b) it is
// not a symlink, and (c) either it already carries a gitid sentinel and its
// glob resolves unambiguously to exactly one file (AdoptSentinelBearing), or
// the caller explicitly confirmed it via chosenPath (AdoptCallerChosen).
// Include order is preserved: the first qualifying directive wins.
//
// When method is AdoptCreateConfigD, or when no directive qualifies, Adopt
// returns AdoptResult{Method: AdoptCreateConfigD} with an empty TargetPath —
// callers then create the gitid-owned config.d layout instead of adopting.
func Adopt(configPath string, method AdoptMethod, chosenPath string, deps AdoptDeps) (AdoptResult, error) {
	if method == AdoptCreateConfigD {
		return AdoptResult{Method: AdoptCreateConfigD}, nil
	}

	directives, err := DetectInclude(configPath)
	if err != nil {
		return AdoptResult{}, fmt.Errorf("sshconfig: adopt: %w", err)
	}

	for _, d := range directives {
		target, ok, cerr := candidateTarget(d, method, chosenPath, deps)
		if cerr != nil {
			return AdoptResult{}, cerr
		}
		if ok {
			return AdoptResult{TargetPath: target, Method: method}, nil
		}
	}

	// No directive qualifies for adoption — fall back to creating the
	// gitid-owned config.d layout.
	return AdoptResult{Method: AdoptCreateConfigD}, nil
}

// candidateTarget evaluates one IncludeDirective against the selection rules
// for method, returning the resolved single-file target when it qualifies.
func candidateTarget(d IncludeDirective, method AdoptMethod, chosenPath string, deps AdoptDeps) (string, bool, error) {
	if !isAcceptablePathForm(d.Raw) {
		return "", false, nil // bare-relative / non-~/.ssh path — rejected at the boundary
	}

	matches, err := deps.Glob(d.Expanded)
	if err != nil {
		return "", false, fmt.Errorf("sshconfig: adopt: globbing %s: %w", d.Expanded, err)
	}

	var target string
	switch method {
	case AdoptCallerChosen:
		if chosenPath == "" || !containsPath(matches, chosenPath) {
			return "", false, nil // caller's choice absent, or not among this directive's matches
		}
		target = chosenPath
	default: // AdoptSentinelBearing
		if len(matches) != 1 {
			return "", false, nil // ambiguous (or empty) — reject unless caller-chosen
		}
		target = matches[0]
		content, rerr := deps.ReadFile(target)
		if rerr != nil {
			return "", false, nil //nolint:nilerr // unreadable candidate cannot confirm gitid ownership; try the next directive
		}
		if len(filewriter.ListBlocks(content)) == 0 {
			return "", false, nil // no gitid sentinel — not gitid-owned, do not auto-adopt
		}
	}

	// Symlink guard (path-traversal mitigation, mirrors the sibling
	// gitconfig-fragment adopter's os.Lstat guard).
	lst, lerr := deps.Lstat(target)
	if lerr != nil && !os.IsNotExist(lerr) {
		return "", false, fmt.Errorf("sshconfig: adopt: stat %s: %w", target, lerr)
	}
	if lerr == nil && lst.Mode()&os.ModeSymlink != 0 {
		return "", false, nil // symlink target — rejected
	}

	return target, true, nil
}

// containsPath reports whether target is present in paths.
func containsPath(paths []string, target string) bool {
	for _, p := range paths {
		if p == target {
			return true
		}
	}
	return false
}
