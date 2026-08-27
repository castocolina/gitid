package globalssh

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/castocolina/gitid/internal/filewriter"
	"github.com/castocolina/gitid/internal/sshconfig"
)

// maxIncludeDepth is the bounded cap for recursive Include discovery.
// A config graph deeper than this returns Inconclusive rather than recursing
// unboundedly — a hostile or merely careless config can otherwise loop forever
// if the cycle detection has a gap, so this cap is the belt-and-suspenders
// bound even when no cycle is present. 8 is several times deeper than any
// real-world gitid config graph.
var maxIncludeDepth = 8

// GraphFile is one file in a SimulationGraph: its absolute real path (used in
// naming findings) and its current on-disk content.
type GraphFile struct {
	Path    string
	Content []byte
}

// SimulationGraph is the faithful representation of the whole configuration
// graph that ssh reads when starting from EntryPointPath. It is the structure
// BuildGraph produces and Simulate materialises into a private mirror.
//
// Files is in first-discovery (resolution) order: the entry point first, then
// every file reachable through Include directives, recursively, cycle-free.
// Each unique path appears exactly once; a diamond graph's shared file is
// included once at its first-encountered position.
type SimulationGraph struct {
	EntryPointPath    string      // the file ssh actually starts from (~/.ssh/config)
	ManagedTargetPath string      // equal to EntryPointPath under the in-file layout
	CandidateContent  []byte      // exactly what EnsureGlobals produced for this apply
	Files             []GraphFile // entry point first, then all reachable files
}

// ShadowFinding is one key whose effective value does not match the candidate's
// value. ShadowedByFile and ShadowedByLine are non-empty only when the scanner
// located a specific directive; they are left empty rather than guessed.
type ShadowFinding struct {
	Key            string
	WantValue      string
	GotValue       string
	ShadowedBy     string
	ShadowedByFile string
	ShadowedByLine int
}

// ShadowResult is returned by both Simulate and Verify.
type ShadowResult struct {
	Findings     []ShadowFinding
	Inconclusive bool
	Reason       string
}

// BuildGraph performs recursive Include discovery starting at entryPointPath,
// returning a SimulationGraph whose Files contains every file reachable from
// the entry point. managedTargetPath is the file the candidate content will
// replace. candidate is the bytes EnsureGlobals produced.
//
// Cycle detection uses TWO sets (06-REVIEWS.md cycle-3 MEDIUM finding):
//   - active: the RECURSION STACK, paths on the CURRENT descent. Pushed before
//     recursing into a file, POPPED ON RETURN. A path already in active is a
//     true cycle: the file is including itself transitively.
//   - expanded: a MEMO of every path already discovered anywhere in the walk,
//     NEVER POPPED. A path in expanded but NOT in active is a DIAMOND (the same
//     file legitimately reached through two Include branches). Skip the
//     re-descent — its subtree is already in the graph — and continue without
//     error.
//
// A single global visited set cannot tell those two apart and would report a
// valid diamond as a cycle, returning an unearned INCONCLUSIVE. This is the
// exact shape aliasCollides' own seen map has (matcher_extraction: pre-existing,
// out of scope, explicitly not to be copied here).
//
// SimulationGraph.Files carries each unique path exactly ONCE, in first-discovery
// order. That is safe for the naming step because shadowSourceFor returns the
// FIRST hit in resolution order, and a doubly-spliced file's earlier occurrence
// is the deciding one — the later re-visit would not change the answer.
//
// Recursion is required: sshconfig.ManagedBlockNames and DetectInclude are both
// deliberately single-hop, and 06-REVIEWS.md cycle-2 found that mirroring only
// the entry point's DIRECT includes makes the mirror unfaithful — a nested
// include would either silently vanish (a false clean result) or, worse, resolve
// to the user's REAL file from inside the mirror. Both consequences are named
// here so nobody "simplifies" the recursion away.
func BuildGraph(entryPointPath, managedTargetPath string, candidate []byte) (SimulationGraph, error) {
	absEntry, err := filepath.Abs(entryPointPath)
	if err != nil {
		return SimulationGraph{}, fmt.Errorf("globalssh: resolving entry point %s: %w", entryPointPath, err)
	}
	absTarget, err := filepath.Abs(managedTargetPath)
	if err != nil {
		return SimulationGraph{}, fmt.Errorf("globalssh: resolving managed target %s: %w", managedTargetPath, err)
	}

	active := map[string]bool{}   // recursion stack: popped on return
	expanded := map[string]bool{} // memo: never popped
	var files []GraphFile

	var discover func(path string, depth int) error
	discover = func(path string, depth int) error {
		if depth > maxIncludeDepth {
			return fmt.Errorf("globalssh: Include nesting depth exceeds cap (%d) at %s", maxIncludeDepth, path)
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("globalssh: resolving %s: %w", path, err)
		}

		// Diamond check: already discovered via another branch, skip.
		if expanded[abs] {
			if active[abs] {
				// This path is on the current descent stack — true cycle.
				return fmt.Errorf("globalssh: Include cycle detected at %s", abs)
			}
			// Diamond: legitimate re-reach. Already in graph. Skip.
			return nil
		}

		// Read the file (missing files contribute nothing, like ManagedBlockNames).
		content, err := os.ReadFile(abs) //nolint:gosec // abs is derived from a gitid-managed path supplied in-process
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return fmt.Errorf("globalssh: reading %s: %w", abs, err)
		}

		expanded[abs] = true
		files = append(files, GraphFile{Path: abs, Content: content})

		active[abs] = true
		defer func() { delete(active, abs) }()

		// Detect and recurse into Include targets.
		directives, err := sshconfig.DetectInclude(abs)
		if err != nil {
			return fmt.Errorf("globalssh: detecting Includes in %s: %w", abs, err)
		}
		for _, d := range directives {
			matches, gerr := filepath.Glob(d.Expanded)
			if gerr != nil {
				return fmt.Errorf("globalssh: globbing %s: %w", d.Expanded, gerr)
			}
			for _, m := range matches {
				if err := discover(m, depth+1); err != nil {
					return err
				}
			}
		}
		return nil
	}

	if err := discover(absEntry, 0); err != nil {
		return SimulationGraph{}, err
	}
	if !expanded[absTarget] {
		content, readErr := os.ReadFile(absTarget) //nolint:gosec // absTarget is a trusted gitid-managed path supplied in-process
		if readErr != nil && !os.IsNotExist(readErr) {
			return SimulationGraph{}, fmt.Errorf("globalssh: reading managed target %s: %w", absTarget, readErr)
		}
		files = append(files, GraphFile{Path: absTarget, Content: content})
	}

	return SimulationGraph{
		EntryPointPath:    absEntry,
		ManagedTargetPath: absTarget,
		CandidateContent:  candidate,
		Files:             files,
	}, nil
}

// Simulate is the pre-write half of the D-04 prove-before-and-after loop.
// It materialises the SimulationGraph into a private mirror, runs the
// resolution probe against the mirrored ENTRY POINT (not the candidate file
// alone — the correction is stated here, with its reason: running the probe
// against the candidate file alone cannot observe a directive in the main
// config, and under the Include layout — which is the DEFAULT on a fresh
// machine — the main config is exactly where a user's own Host * block lives),
// and returns a ShadowResult for each key whose effective value does not match
// the candidate's recommended value.
//
// A BuildGraph error, an Include cycle, a depth-cap hit, or a probe failure
// all return Inconclusive with a reason — never a false "no shadowing".
func Simulate(deps Deps, graph SimulationGraph, keys []string) ShadowResult {
	// materialise a private mirror
	mirrorRoot, err := os.MkdirTemp("", "gitid-sim-*")
	if err != nil {
		return ShadowResult{Inconclusive: true, Reason: fmt.Sprintf("globalssh: creating mirror root: %v", err)}
	}
	defer func() { _ = os.RemoveAll(mirrorRoot) }()

	// Set mirror root permissions to 0700 — OpenSSH refuses group- or
	// world-writable config directories (T-06-12). The MkdirTemp default mode
	// may be more permissive on some platforms, so we set it explicitly.
	if err := os.Chmod(mirrorRoot, 0o700); err != nil { //nolint:gosec // explicitly setting restrictive mode 0700 for OpenSSH compatibility
		return ShadowResult{Inconclusive: true, Reason: fmt.Sprintf("globalssh: setting mirror root mode: %v", err)}
	}

	// Build a map from real absolute path → mirrored absolute path.
	mirrorPath := func(realPath string) string {
		rel := strings.TrimPrefix(realPath, "/")
		return filepath.Join(mirrorRoot, rel)
	}

	// Write each file into the mirror. The managed target gets the candidate
	// content instead of its real bytes.
	mirroredEntryPoint := mirrorPath(graph.EntryPointPath)
	for _, gf := range graph.Files {
		mp := mirrorPath(gf.Path)
		if err := os.MkdirAll(filepath.Dir(mp), 0o700); err != nil {
			return ShadowResult{Inconclusive: true, Reason: fmt.Sprintf("globalssh: creating mirror dir for %s: %v", mp, err)}
		}
		content := gf.Content
		if gf.Path == graph.ManagedTargetPath {
			content = graph.CandidateContent
		}
		// Rewrite Include directives to point inside the mirror.
		content = rewriteIncludes(content, graph.Files, mirrorPath, mirrorRoot)
		if err := os.WriteFile(mp, content, 0o600); err != nil {
			return ShadowResult{Inconclusive: true, Reason: fmt.Sprintf("globalssh: writing mirror file %s: %v", mp, err)}
		}
	}

	// Run the resolution probe against the mirrored ENTRY POINT with the
	// isolated-config flag, so no real user files participate.
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	out, probeErr := deps.RunSSHG(ctx, "-G", "-F", mirroredEntryPoint, ProbeHost)
	if probeErr != nil {
		return ShadowResult{Inconclusive: true, Reason: fmt.Sprintf("globalssh: simulation probe failed: %v", probeErr)}
	}

	resolved := parseResolvedOptions(out)

	var findings []ShadowFinding
	for _, k := range keys {
		policy, ok := PolicyFor(k)
		if !ok {
			continue
		}
		want := policy.Recommended
		got := resolved[strings.ToLower(k)]
		if strings.EqualFold(got, want) {
			continue
		}
		// The probe reports a different value — the fix would be shadowed.
		// Ask the scanner to NAME the shadowing directive.
		culpritText, culpritFile, culpritLine := shadowSourceFor(graph, k)
		findings = append(findings, ShadowFinding{
			Key:            k,
			WantValue:      want,
			GotValue:       got,
			ShadowedBy:     culpritText,
			ShadowedByFile: culpritFile,
			ShadowedByLine: culpritLine,
		})
	}
	return ShadowResult{Findings: findings}
}

// Verify is the post-write half of the D-04 prove-before-and-after loop.
// It runs the same comparison as Simulate, but against the LIVE machine with
// NO isolated-config flag, so it reflects the real resolution order including
// files gitid does not manage. This is the same probe Statuses already uses;
// the invocation is shared rather than duplicated.
//
// A probe failure returns Inconclusive with a reason — never a false "no
// shadowing". The static scan is NOT used here because Verify does not have
// access to the graph; it is a live-machine check only.
func Verify(deps Deps, keys []string) ShadowResult {
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	out, err := deps.RunSSHG(ctx, "-G", ProbeHost)
	if err != nil {
		return ShadowResult{Inconclusive: true, Reason: fmt.Sprintf("globalssh: post-write verify probe failed: %v", err)}
	}
	resolved := parseResolvedOptions(out)

	var findings []ShadowFinding
	for _, k := range keys {
		policy, ok := PolicyFor(k)
		if !ok {
			continue
		}
		want := policy.Recommended
		got := resolved[strings.ToLower(k)]
		if strings.EqualFold(got, want) {
			continue
		}
		findings = append(findings, ShadowFinding{
			Key:       k,
			WantValue: want,
			GotValue:  got,
		})
	}
	return ShadowResult{Findings: findings}
}

// shadowSourceFor names the first directive in the resolution order that
// shadows key for ProbeHost. It uses sshconfig.ScanDirectivesMulti on a
// linearised representation of the graph and returns the FIRST surviving hit —
// first-obtained-value, not last; getting this backwards was 06-REVIEWS.md's
// cycle-2 HIGH finding.
//
// Steps:
//  1. Linearise the graph into an ordered []sshconfig.DirectiveSource that
//     mirrors how ssh actually reads it (each file's bytes above an Include are
//     obtained before the included files; bytes below are obtained after).
//  2. Scan and filter: keep only hits whose HostPattern matches ProbeHost under
//     OpenSSH's negation-aware rules (sshconfig.HostLineMatches, the function
//     exported from aliasCollides' loop — do NOT reimplement negation here),
//     and discard hits inside gitid's own managed block.
//  3. Return the FIRST survivor (file and 1-based true line). Return empty when
//     nothing survives — the warning then reports shadowing without a culprit
//     rather than inventing one.
//
// This function has NO say in whether shadowing occurred; a static scan cannot
// model Match blocks or every OpenSSH pattern form and would produce false
// negatives. The probe decides; this only labels.
func shadowSourceFor(graph SimulationGraph, key string) (text, file string, line int) {
	sources := linearise(graph)
	hits := sshconfig.ScanDirectivesMulti(sources, []string{key})

	// The sentinel markers for gitid's managed block.
	beginSentinel := filewriter.BeginPrefix + sshconfig.GlobalBlockName
	endSentinel := filewriter.EndPrefix + sshconfig.GlobalBlockName

	for _, h := range hits {
		// Filter by Host pattern matching ProbeHost.
		// sshconfig.HostLineMatches is the exported negation-aware matcher
		// extracted from aliasCollides. Do NOT call aliasCollides (unexported
		// and file-path-shaped), and do NOT reimplement the negation rule here
		// (06-REVIEWS.md cycle-3 HIGH: one implementation, two callers).
		//
		// An empty HostPattern means the directive is at the top level (not
		// inside any Host stanza). Top-level directives apply to all hosts,
		// so they always match.
		if h.HostPattern != "" && !sshconfig.HostLineMatches(h.HostPattern, ProbeHost) {
			continue
		}

		// Discard hits inside gitid's own managed block.
		if isInsideManagedBlock(h, graph.ManagedTargetPath, beginSentinel, endSentinel) {
			continue
		}

		// First surviving hit — this is the one ssh reported.
		return fmt.Sprintf("%s %s (line %d in %s)", h.Key, h.Value, h.Line, h.SourcePath),
			h.SourcePath, h.Line
	}
	return "", "", 0
}

// linearise converts a SimulationGraph into an ordered []sshconfig.DirectiveSource
// that mirrors how ssh reads the config: for each file, emit the bytes ABOVE
// the first Include directive, then recursively linearise each included file,
// then emit the bytes BELOW (with the appropriate LineOffset).
//
// A diamond's shared file IS linearised at both splice points — ssh reads it
// at both, so both occurrences are part of the resolved order and the first
// one wins. The guard is a POPPED-ON-RETURN recursion stack, never a global
// deduplication set, so the diamond's second occurrence is NOT suppressed.
// BuildGraph has already rejected true cycles as Inconclusive, so this guard
// is a backstop.
func linearise(graph SimulationGraph) []sshconfig.DirectiveSource {
	fileMap := map[string][]byte{}
	for _, f := range graph.Files {
		fileMap[f.Path] = f.Content
	}

	active := map[string]bool{}
	var result []sshconfig.DirectiveSource

	var walk func(path string)
	walk = func(path string) {
		if active[path] {
			return // cycle backstop (BuildGraph should have caught this)
		}
		active[path] = true
		defer func() { delete(active, path) }()

		content, ok := fileMap[path]
		if !ok {
			return
		}

		directives, err := sshconfig.DetectInclude(path)
		if err != nil || len(directives) == 0 {
			// No Includes: emit the whole file as one source.
			result = append(result, sshconfig.DirectiveSource{
				Path:       path,
				Content:    content,
				LineOffset: 0,
			})
			return
		}

		// For each Include directive, emit the slice ABOVE it, then recurse
		// into the included files, then continue with the slice BELOW.
		lines := strings.Split(string(content), "\n")
		emittedUpTo := 0 // in LINES (0-based)

		for _, d := range directives {
			// Find the line number of this Include directive in the file.
			includeLine := findIncludeLine(lines, d.Expanded, d.Raw)
			if includeLine < 0 {
				continue
			}

			// Emit everything from emittedUpTo to just before this Include.
			if includeLine > emittedUpTo {
				slice := strings.Join(lines[emittedUpTo:includeLine], "\n")
				if slice != "" {
					result = append(result, sshconfig.DirectiveSource{
						Path:       path,
						Content:    []byte(slice + "\n"),
						LineOffset: emittedUpTo,
					})
				}
			}

			// Recurse into each included file.
			matches, _ := filepath.Glob(d.Expanded)
			for _, m := range matches {
				walk(m)
			}

			emittedUpTo = includeLine + 1
		}

		// Emit remainder after the last Include.
		if emittedUpTo < len(lines) {
			slice := strings.Join(lines[emittedUpTo:], "\n")
			if slice != "" {
				result = append(result, sshconfig.DirectiveSource{
					Path:       path,
					Content:    []byte(slice),
					LineOffset: emittedUpTo,
				})
			}
		}
	}

	walk(graph.EntryPointPath)
	return result
}

// findIncludeLine finds the 0-based line index of the Include directive in lines
// that matches this IncludeDirective. Returns -1 if not found.
func findIncludeLine(lines []string, expanded, raw string) int {
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		fields := strings.Fields(trimmed)
		if len(fields) < 2 || !strings.EqualFold(fields[0], "Include") {
			continue
		}
		token := fields[1]
		token = strings.Trim(token, `"`)
		if token == raw || token == expanded {
			return i
		}
		// Also match the glob pattern directly.
		if strings.Contains(expanded, "*") || strings.Contains(expanded, "?") {
			if token == raw {
				return i
			}
		}
	}
	return -1
}

// isInsideManagedBlock reports whether hit falls inside gitid's own managed
// block in the managed target file. Directives inside the managed block must
// not be named as culprits — they are the gitid content being applied.
func isInsideManagedBlock(hit sshconfig.DirectiveHit, managedTargetPath, beginSentinel, endSentinel string) bool {
	if hit.SourcePath != managedTargetPath {
		return false
	}
	// Read the managed target to find the sentinel line numbers.
	content, err := os.ReadFile(managedTargetPath) //nolint:gosec
	if err != nil {
		return false
	}
	lines := strings.Split(string(content), "\n")
	inBlock := false
	for i, line := range lines {
		lineNo := i + 1 // 1-based
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, beginSentinel) {
			inBlock = true
		}
		if inBlock && lineNo == hit.Line {
			return true
		}
		if strings.Contains(trimmed, endSentinel) {
			inBlock = false
		}
	}
	return false
}

// rewriteIncludes rewrites Include directives in content to point to mirrored
// paths. An Include that cannot be resolved to a path in the graph is rewritten
// to a non-existent mirror path so the simulation degrades to "that file
// contributes nothing" rather than silently reading the user's real file —
// an unresolved Include must never leak the real machine into the simulation
// (T-06-37, T-06-49).
func rewriteIncludes(content []byte, graphFiles []GraphFile, mirrorPath func(string) string, mirrorRoot string) []byte {
	fileSet := map[string]bool{}
	for _, gf := range graphFiles {
		fileSet[gf.Path] = true
	}

	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		fields := strings.Fields(trimmed)
		if len(fields) < 2 || !strings.EqualFold(fields[0], "Include") {
			continue
		}
		// Rewrite this Include line.
		raw := strings.Trim(fields[1], `"`)
		// Try to find the matching mirrored file via glob. A candidate managed
		// target may not exist on the real filesystem yet, so also match the
		// graph's known files before deciding an Include is unresolved.
		expanded := expandPathForMirror(raw)
		matches, _ := filepath.Glob(expanded)
		known := len(matches) > 0
		if !known {
			for _, gf := range graphFiles {
				if ok, _ := filepath.Match(expanded, gf.Path); ok {
					known = true
					break
				}
			}
		}
		if !known {
			// No matches — rewrite to a non-existent path inside the mirror.
			lines[i] = "Include " + filepath.Join(mirrorRoot, "nonexistent-"+filepath.Base(raw))
			continue
		}
		// Check if the match is a glob pattern; if so rewrite to the mirrored glob.
		if strings.Contains(raw, "*") || strings.Contains(raw, "?") {
			// Rewrite the glob path to the mirror.
			// Find the directory portion and rewrite it.
			mirrored := mirrorPath(expandPathForMirror(raw))
			lines[i] = "Include " + mirrored
		} else {
			// Single file: rewrite to mirrored path.
			mirrored := mirrorPath(expandPathForMirror(raw))
			lines[i] = "Include " + mirrored
		}
	}
	return []byte(strings.Join(lines, "\n"))
}

// expandPathForMirror expands a raw Include path token to an absolute path
// for mirror path construction. Mirrors expandIncludePath in adopt.go.
func expandPathForMirror(raw string) string {
	home, _ := os.UserHomeDir()
	switch {
	case filepath.IsAbs(raw):
		return raw
	case len(raw) >= 2 && raw[0] == '~' && raw[1] == '/':
		return filepath.Join(home, raw[2:])
	default:
		return filepath.Join(home, ".ssh", raw)
	}
}
