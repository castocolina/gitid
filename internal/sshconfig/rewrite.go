package sshconfig

// rewrite.go implements the phase's single highest-risk write affordance
// (08-CONTEXT.md D-09/D-10): the Fixer — and ONLY the Fixer — may rewrite
// exactly ONE existing directive's VALUE on a hand-written (sentinel-less,
// non-gitid-managed) Host stanza, exclusively through the full ceremony
// (true diff preview, typed confirm, timestamped backup, mandatory
// verification). Every other write path in this codebase remains
// managed-blocks-only.
//
// RewriteHostDirective is the surgical single-directive primitive itself;
// ApplyVerifiedHostDirective is the D-10 verification loop wrapped around it
// (rewrite -> parse->render->re-parse stability -> real `ssh -G`
// re-verification -> automatic restore from the just-taken backup on any
// mismatch). DiffHostDirective renders the true before/after diff the Fixer
// ceremony and `gitid fix` preview.

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/castocolina/gitid/internal/filewriter"
)

// verifyProbeTimeout bounds every real `ssh -G` invocation ApplyVerifiedHostDirective
// makes, mirroring internal/globalssh's probeTimeout (T-06-02 class: a
// pathological config must never block gitid indefinitely). A var, not a const,
// so tests can shrink it.
var verifyProbeTimeout = 3 * time.Second

// postRewriteHook is a test-only injection point: when non-nil,
// ApplyVerifiedHostDirective calls it with configPath immediately after the
// rewrite and before the re-parse stability check, letting a test corrupt the
// just-written file to prove the auto-restore path (D-10). Nil in production.
var postRewriteHook func(configPath string)

// RewriteHostDirective performs the surgical single-directive rewrite: it
// reads configPath, RE-LOCATES the first Host stanza whose pattern matches
// hostPattern (never trusting a line number carried from an earlier scan — a
// stale scan must not silently rewrite the wrong line), replaces ONLY that
// stanza's named directive's VALUE line (preserving the line's original
// indentation, key casing, separator style, and trailing comment), and writes
// through the filewriter chokepoint (atomic temp -> rename, timestamped
// backup, 0600). Every other byte of the file — comments, blank lines,
// unrelated Host stanzas — is preserved verbatim.
//
// The host pattern and the new value are validated before the write (Security
// Domain V5: reuse the existing hostname validation; CR-18's control-byte
// discipline for the value — never a raw string splice). If the stanza or its
// directive cannot be located, a descriptive error is returned — never a
// guess, never a newly appended line.
//
// configPath is a gitid-resolved, trusted path supplied in-process.
func RewriteHostDirective(configPath, hostPattern, directive, newValue string) (backupPath string, err error) {
	if err := validateRewriteValue(directive, newValue); err != nil {
		return "", err
	}
	if err := validateToken(hostPattern); err != nil {
		return "", fmt.Errorf("rewriting %s on Host %q: invalid pattern: %w", directive, hostPattern, err)
	}

	content, err := os.ReadFile(configPath) //nolint:gosec // configPath is a trusted gitid-managed path supplied in-process (G304)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("rewriting %s on Host %q: reading %s: %w", directive, hostPattern, configPath, err)
	}

	composed, ok := rewriteDirectiveLine(content, hostPattern, directive, newValue)
	if !ok {
		return "", fmt.Errorf(
			"rewriting %s on Host %q in %s: Host stanza or %s directive not found (the config changed since the finding was computed)",
			directive, hostPattern, configPath, directive)
	}

	// Round-trip safety: the composed config must parse cleanly before we
	// commit it to disk (parse -> compose -> parse stability, CLAUDE.md's
	// second-Decode pass).
	if _, perr := Parse(composed); perr != nil {
		return "", fmt.Errorf("rewriting %s on Host %q in %s: composed config is not parseable, refusing to write: %w", directive, hostPattern, configPath, perr)
	}

	backupPath, err = filewriter.Write(configPath, composed, configMode)
	if err != nil {
		return "", fmt.Errorf("rewriting %s on Host %q in %s: %w", directive, hostPattern, configPath, err)
	}
	return backupPath, nil
}

// ApplyVerifiedHostDirective is the D-10 verification loop around
// RewriteHostDirective:
//
//  1. rewrite (backup taken first, atomically);
//  2. parse -> render -> re-parse the rewritten file and assert byte
//     stability — the round-trip check every prior phase's write ceremony
//     already implements (CLAUDE.md "second-Decode pass");
//  3. when the `ssh` binary is present, run `ssh -G -F <configPath>
//     <hostPattern>` (arg-slice exec.CommandContext, bounded timeout, no
//     shell) and assert the resolved <directive> value now matches newValue;
//  4. on ANY verification failure, restore the file from the backup this same
//     apply just took and return a descriptive error — the file is never left
//     in the failed intermediate state.
//
// When `ssh` is absent (or the probe itself errors), the loop degrades to the
// parse->render->re-parse check only — D-10's documented headless fallback,
// never a hard failure purely because the probe binary is missing.
func ApplyVerifiedHostDirective(configPath, hostPattern, directive, newValue string) (backupPath string, err error) {
	backupPath, err = RewriteHostDirective(configPath, hostPattern, directive, newValue)
	if err != nil {
		return "", err
	}

	if postRewriteHook != nil {
		postRewriteHook(configPath)
	}

	// Step 2: parse -> render -> re-parse byte-stability on the rewritten file.
	after, rerr := os.ReadFile(configPath) //nolint:gosec // configPath is a trusted gitid-managed path supplied in-process (G304)
	if rerr != nil {
		return backupPath, fmt.Errorf("verifying rewrite of %s: re-reading %s: %w", directive, configPath, rerr)
	}
	if !roundTripStable(after) {
		return restoreFromBackup(backupPath, configPath, fmt.Errorf(
			"rewriting %s on Host %q in %s: rewritten config is not parse->render->re-parse stable, restored from backup",
			directive, hostPattern, configPath))
	}

	// Step 3: real `ssh -G` re-verification when the probe binary is present.
	if _, lookErr := exec.LookPath("ssh"); lookErr == nil {
		ok, probeErr := sshGResolves(configPath, hostPattern, directive, newValue)
		if probeErr == nil && !ok {
			return restoreFromBackup(backupPath, configPath, fmt.Errorf(
				"rewriting %s on Host %q in %s: `ssh -G` still does not resolve %s to %q, restored from backup",
				directive, hostPattern, configPath, directive, newValue))
		}
		// A probe error (timeout, ssh present but failing) degrades to the
		// round-trip check only — the D-10 headless fallback. Never a hard
		// failure purely because the probe could not complete.
	}

	return backupPath, nil
}

// roundTripStable reports whether content survives a parse -> render ->
// re-parse -> re-render cycle byte-identically (the CLAUDE.md second-Decode
// stability invariant): a config gitid's own write path just produced must not
// be silently mangled by gitid's own parser/renderer on the next pass.
func roundTripStable(content []byte) bool {
	cfg, err := Parse(content)
	if err != nil {
		return false
	}
	render1 := cfg.String()
	cfg2, err := Parse([]byte(render1))
	if err != nil {
		return false
	}
	return cfg2.String() == render1
}

// sshGResolves runs `ssh -G -F <configPath> <hostPattern>` through the
// bounded, arg-slice, group-killed probe pattern (mirrors
// internal/globalssh.BuildProbeDeps) and reports whether the resolved value
// of directive now equals newValue. A non-nil error means the probe could not
// complete (ssh missing at run time, timeout, non-zero exit) — the caller
// degrades rather than restores on it.
func sshGResolves(configPath, hostPattern, directive, newValue string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), verifyProbeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ssh", "-G", "-F", configPath, hostPattern) //nolint:gosec // arg-slice form, no shell; configPath is a trusted gitid-managed path (G204)
	// Run ssh in its own process group and SIGKILL the whole group on timeout,
	// exactly as internal/globalssh's probe does: a pathological config can
	// fork a grandchild that holds the stdout pipe after the direct child is
	// killed, which would block Output() past the deadline (T-06-02 class).
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) // best-effort group kill
		}
		return nil
	}
	cmd.WaitDelay = 500 * time.Millisecond
	out, err := cmd.Output()
	if err != nil {
		return false, err
	}
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	resolved := ""
	for _, line := range strings.Split(string(out), "\n") {
		idx := strings.IndexByte(line, ' ')
		if idx <= 0 {
			continue
		}
		key := line[:idx]
		if key != strings.ToLower(key) {
			continue // camelCase line: never a directive match (tester.ParseResolved property)
		}
		if strings.EqualFold(key, directive) {
			resolved = strings.TrimSpace(line[idx+1:])
			break
		}
	}
	return strings.EqualFold(strings.TrimSpace(resolved), newValue), nil
}

// DiffHostDirective renders the D-09 true before/after diff for the surgical
// rewrite: every line of the matching Host stanza with a two-space context
// prefix, the target directive line shown as `- <original>` / `+ <rewritten>`.
// It mirrors the frozen FixerFixPreviewLines shape (internal/tuikit/design.go)
// so the real diff is structurally byte-compatible with the fixture copy. A
// missing stanza or directive returns a descriptive error — the caller renders
// a fallback rather than an empty diff.
func DiffHostDirective(content []byte, hostPattern, directive, newValue string) (string, error) {
	if err := validateRewriteValue(directive, newValue); err != nil {
		return "", err
	}
	if err := validateToken(hostPattern); err != nil {
		return "", fmt.Errorf("diffing %s on Host %q: invalid pattern: %w", directive, hostPattern, err)
	}
	lines := strings.SplitAfter(string(content), "\n")
	directiveIdx := locateDirectiveLine(lines, hostPattern, directive)
	if directiveIdx < 0 {
		return "", fmt.Errorf("diffing %s on Host %q: stanza or directive not found", directive, hostPattern)
	}
	// Walk back to the stanza header, then forward to the next stanza
	// boundary to collect the stanza's lines.
	headerIdx := directiveIdx
	for headerIdx > 0 && !isStanzaHeader(strings.TrimRight(lines[headerIdx-1], "\n\r")) {
		headerIdx--
	}
	var b strings.Builder
	oldValueLine := strings.TrimRight(lines[directiveIdx], "\n\r")
	newLine, ok := rewriteDirectiveLineValue(oldValueLine, newValue)
	if !ok {
		return "", fmt.Errorf("diffing %s on Host %q: cannot render the rewritten directive line", directive, hostPattern)
	}
	for i := headerIdx; i < len(lines); i++ {
		trimmed := strings.TrimRight(lines[i], "\n\r")
		if i > directiveIdx && isStanzaHeader(trimmed) {
			break // next stanza boundary
		}
		if i == directiveIdx {
			b.WriteString("- " + oldValueLine + "\n")
			b.WriteString("+ " + newLine + "\n")
			continue
		}
		b.WriteString("  " + trimmed + "\n")
	}
	return b.String(), nil
}

// rewriteDirectiveLine is the byte-level splice: it returns content with the
// matched stanza's directive value line rewritten, or (nil, false) when the
// stanza or directive cannot be located. Only the value portion of ONE line
// changes; every other line is byte-identical.
func rewriteDirectiveLine(content []byte, hostPattern, directive, newValue string) ([]byte, bool) {
	lines := strings.SplitAfter(string(content), "\n")
	idx := locateDirectiveLine(lines, hostPattern, directive)
	if idx < 0 {
		return nil, false
	}
	old := strings.TrimRight(lines[idx], "\n\r")
	rebuilt, ok := rewriteDirectiveLineValue(old, newValue)
	if !ok {
		return nil, false
	}
	lines[idx] = rebuilt + lines[idx][len(old):]
	return []byte(strings.Join(lines, "")), true
}

// locateDirectiveLine scans lines for the first Host stanza whose pattern
// matches hostPattern and returns the index of its <directive> line (key
// matched case-insensitively), or -1 when the stanza or directive is absent.
// Stanza boundaries are lines whose trimmed text begins with Host or Match
// (case-insensitive) — the same boundaries kevinburke/ssh_config uses.
func locateDirectiveLine(lines []string, hostPattern, directive string) int {
	inStanza := false
	matching := false
	for i, line := range lines {
		trimmed := strings.TrimRight(line, "\n\r")
		if isStanzaHeader(trimmed) {
			inStanza = true
			matching = stanzaPatternMatches(trimmed, hostPattern)
			continue
		}
		if !inStanza || !matching {
			continue
		}
		if key, _, ok := parseDirectiveLine(trimmed); ok && strings.EqualFold(key, directive) {
			return i
		}
	}
	return -1
}

// rewriteDirectiveLineValue rebuilds one directive line with a new value,
// preserving the line's leading whitespace, key text, separator style
// (space, ` = `, or `key=value`), and any trailing comment. It returns the
// rebuilt line without its line ending — the caller re-attaches the original
// newline so untouched line endings are preserved byte-for-byte.
func rewriteDirectiveLineValue(line, newValue string) (string, bool) {
	trimmed := strings.TrimRight(line, "\n\r")
	leading := trimmed[:len(trimmed)-len(strings.TrimLeft(trimmed, " \t"))]
	body := strings.TrimLeft(trimmed, " \t")
	i := 0
	for i < len(body) && body[i] != ' ' && body[i] != '\t' && body[i] != '=' {
		i++
	}
	if i == 0 {
		return "", false
	}
	key := body[:i]
	sep := body[i:]
	j := 0
	for j < len(sep) && (sep[j] == ' ' || sep[j] == '\t') {
		j++
	}
	if j < len(sep) && sep[j] == '=' {
		j++
		for j < len(sep) && (sep[j] == ' ' || sep[j] == '\t') {
			j++
		}
	}
	valStart := j
	commentAt := len(sep)
	for k := j; k < len(sep); k++ {
		if sep[k] == '#' {
			commentAt = k
			break
		}
	}
	valEnd := commentAt
	for valEnd > valStart && (sep[valEnd-1] == ' ' || sep[valEnd-1] == '\t') {
		valEnd--
	}
	return leading + key + sep[:valStart] + newValue + sep[valEnd:], true
}

// parseDirectiveLine splits a directive line into its key and the remainder
// after the key (separator + value + comment). ok is false for comment-only
// and blank lines.
func parseDirectiveLine(trimmed string) (key, rest string, ok bool) {
	body := strings.TrimLeft(trimmed, " \t")
	i := 0
	for i < len(body) && body[i] != ' ' && body[i] != '\t' && body[i] != '=' {
		i++
	}
	if i == 0 {
		return "", "", false
	}
	return body[:i], body[i:], true
}

// isStanzaHeader reports whether trimmed is a Host or Match stanza header
// line (case-insensitive), the same stanza boundary kevinburke/ssh_config
// uses.
func isStanzaHeader(trimmed string) bool {
	t := strings.ToLower(strings.TrimSpace(trimmed))
	return strings.HasPrefix(t, "host ") || t == "host" ||
		strings.HasPrefix(t, "match ") || t == "match"
}

// stanzaPatternMatches reports whether a Host stanza header line's pattern
// tokens contain hostPattern (case-insensitive — SSH host patterns match
// case-insensitively). Match stanzas never match a literal Host-pattern
// rewrite target by design.
func stanzaPatternMatches(header, hostPattern string) bool {
	fields := strings.Fields(header)
	if len(fields) < 2 {
		return false
	}
	if !strings.EqualFold(fields[0], "host") {
		return false // "Match host …" — not a Host stanza
	}
	for _, p := range fields[1:] {
		if strings.EqualFold(p, hostPattern) {
			return true
		}
	}
	return false
}

// validateRewriteValue applies the CR-18 control-byte discipline to the
// surgical rewrite: a value or directive containing a newline, carriage
// return, or NUL byte would break the config's structure, so it is rejected
// before any write rather than written verbatim.
func validateRewriteValue(directive, newValue string) error {
	if strings.TrimSpace(directive) == "" || strings.TrimSpace(newValue) == "" {
		return fmt.Errorf("rewrite directive and value must both be non-empty")
	}
	for _, s := range []string{directive, newValue} {
		if strings.ContainsAny(s, "\n\r\x00") {
			return fmt.Errorf("rewrite directive/value contains a newline or control byte — refusing to write it verbatim")
		}
	}
	return nil
}

// restoreFromBackup restores configPath from the backup this same apply took
// and returns err wrapped with the restore's outcome. The restore goes through
// filewriter.WriteNoBackup — the dedicated rollback seam — never through
// Write, which would create a NEW backup of the very file being restored
// (Codex HIGH #1 class).
func restoreFromBackup(backupPath, configPath string, cause error) (string, error) {
	if backupPath == "" {
		return "", fmt.Errorf("%w (and no backup existed to restore from)", cause)
	}
	content, rerr := os.ReadFile(backupPath) //nolint:gosec // backupPath is a filewriter-created trusted backup path (G304)
	if rerr != nil {
		return "", fmt.Errorf("%w (and restoring from %s failed: %v)", cause, backupPath, rerr)
	}
	if werr := filewriter.WriteNoBackup(configPath, content, configMode); werr != nil {
		return "", fmt.Errorf("%w (and restoring %s from %s failed: %v)", cause, configPath, backupPath, werr)
	}
	return "", fmt.Errorf("%w (restored %s from %s)", cause, configPath, backupPath)
}
