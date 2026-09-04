package globalssh

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// badConfigOptionMarker is the exact OpenSSH stderr substring a staged-config
// `ssh -G` prints when it encounters a directive NAME it does not recognize
// (RESEARCH Pattern 3, verified live against OpenSSH_9.9p2/LibreSSL 3.3.6):
//
//	<path>: line N: Bad configuration option: <lowercased-name>
//	<path>: terminating, 1 bad configuration options
//
// This is the ONLY signal ProveCustomDirective trusts to answer "is this
// directive name real" — never a static keyword list (D-H).
const badConfigOptionMarker = "Bad configuration option: "

// DirectiveProof is the staged-config classification result for one
// candidate SSH directive name=value pair (PROP-04, D-H/D-I). Exactly one of
// OK, UnknownName, or PreexistingError is true on a call that returns a nil
// error; all three are false only for a "known name, rejected value" result
// (D-I's fourth classification, surfaced as a value/syntax rejection rather
// than a name error).
//
// Command and Output carry the RAW command line and the VERBATIM combined
// output — never a paraphrase — because 09.5-UI-SPEC.md's stage-2 render
// requires showing the exact command and its real result (TEST-01's
// "show the exact command, then its real output" contract).
type DirectiveProof struct {
	// OK is true when the staged probe accepted BOTH the candidate's name
	// and its value: ssh -G exited cleanly against the staged config.
	OK bool
	// UnknownName is true when the staged probe's `Bad configuration
	// option: ` diagnostic names THIS candidate's own directive — the name
	// itself is not recognized by the locally installed OpenSSH.
	UnknownName bool
	// PreexistingError is true when the staged probe's `Bad configuration
	// option: ` diagnostic names a DIFFERENT directive than the candidate —
	// a problem that already existed in the user's current global block,
	// which must not be blamed on the entry just submitted (D-I).
	PreexistingError bool
	// OffendingName is the lowercased directive name the `Bad configuration
	// option: ` diagnostic named, populated whenever UnknownName or
	// PreexistingError is true. Empty otherwise.
	OffendingName string
	// Command is the exact `ssh -F <staged> -G <ProbeHost>` command line
	// that was run — the staged path included, never redacted, so the
	// rendered command is byte-identical to what actually executed.
	Command string
	// Output is the verbatim combined stdout+stderr the staged probe
	// produced. Never paraphrased.
	Output string
}

// structuralDirectives are keywords that change the SHAPE of the config
// rather than set a value. ssh -G accepts them as a directive line (verified
// live against the real OpenSSH on this machine), so the staged probe alone
// cannot reject them — but writing one into gitid's own `Host *` managed
// block breaks the block's single-wildcard-stanza invariant AND is silently
// dropped by parseGlobalBody (internal/sshconfig/globals.go) on the very
// next EnsureGlobals write (CR-02).
var structuralDirectives = map[string]bool{
	"host": true, "match": true, "include": true, "ignoreunknown": true,
}

// IsStructuralDirectiveName reports whether name (case-insensitively) is one
// of the structural keywords ValidateDirectiveName rejects. Exported
// (09.5-REVIEW.md WR-08) so a caller that already knows a name is
// structural — today unreachable through the normal write path since
// ValidateDirectiveName rejects it earlier, but kept as the SAME
// enumeration for any future caller reached without that gate — can
// suppress a "not found in the resolved directive set" advisory that would
// otherwise fire falsely: `ssh -G` never echoes a structural directive back
// in its resolved-options output, so its ABSENCE from that set proves
// nothing about whether the write landed.
func IsStructuralDirectiveName(name string) bool {
	return structuralDirectives[strings.ToLower(name)]
}

// ValidateDirectiveName rejects any candidate name that is not a single,
// safe, unquoted OpenSSH directive token (CR-02). This is the ONLY guard
// standing between a free-form directive name and gitid's own managed
// `Host *` block — the staged ssh -G probe in ProveCustomDirective accepts
// an empty name, a whitespace-only name, and every structural keyword
// (Host/Match/Include/IgnoreUnknown), so none of those can be caught by the
// probe alone.
func ValidateDirectiveName(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("globalssh: directive name cannot be empty")
	}
	if len(strings.Fields(name)) != 1 {
		return fmt.Errorf("globalssh: directive name %q must be a single token with no whitespace", name)
	}
	for _, r := range name {
		if r <= ' ' || r == 0x7f || r == '#' || r == '=' || r == '"' || r == '\'' || r == '\\' {
			return fmt.Errorf("globalssh: directive name %q contains a character that is unsafe in an unquoted SSH config token", name)
		}
	}
	if structuralDirectives[strings.ToLower(name)] {
		return fmt.Errorf("globalssh: %q is a structural directive, not a settable option — gitid's managed Host * block cannot contain it", name)
	}
	return nil
}

// ValidateDirectiveValue rejects a value that would not survive rendering
// into a single unquoted directive line, or that is empty (CR-02: an empty
// submission must not silently write a whitespace-only line and take a
// backup for nothing).
func ValidateDirectiveValue(value string) error {
	if strings.ContainsAny(value, "\n\r\x00") {
		return fmt.Errorf("globalssh: directive value %q must not contain a line break or NUL", value)
	}
	if strings.TrimSpace(value) == "" {
		return errors.New("globalssh: directive value cannot be empty")
	}
	return nil
}

// ProveCustomDirective is the ENTIRE mechanism behind PROP-04's "known SSH
// directive" requirement (D-H): it stages currentGlobalBody plus the
// candidate `name value` line into a THROWAWAY temp config, under a fresh
// 0700 directory (OpenSSH refuses a group- or world-writable config
// directory, the same constraint Simulate already handles at
// shadow.go:218), and runs `ssh -F <staged> -G ProbeHost` through the
// injected deps.RunSSHGCombined seam — never deps.RunSSHG, which returns
// stdout only and cannot see the `Bad configuration option: ` diagnostic.
//
// The real user config is never opened for writing here: the staged file is
// written under os.MkdirTemp and removed (via defer) before this function
// returns, and the ONLY path handed to ssh is that staged path (T-09.5-21).
//
// Classification (D-I): the exit outcome is the FIRST-PASS gate — a nil
// error means the candidate's name AND value were both accepted. A non-nil
// error with NO combined output at all is treated as a transport-level
// failure (the probe could not run: a missing binary, a timed-out process
// that produced nothing) and returns fail-closed: (proof with OK false,
// a non-nil error). A non-nil error WITH output is classified by the
// `Bad configuration option: ` substring — present and naming the
// candidate's own (lowercased) name means UnknownName; present and naming a
// DIFFERENT name means PreexistingError (a problem that predates this
// entry); absent entirely means the name is recognized but the VALUE was
// rejected, reported with the real output and no name-related flag set.
// There is no code path that returns OK: true alongside a non-nil error.
func ProveCustomDirective(deps Deps, currentGlobalBody, name, value string) (DirectiveProof, error) {
	// CR-02: fail closed on the name/value shape BEFORE any staged probe
	// runs. ssh -G accepts an empty name, a whitespace-only name, and every
	// structural keyword (Host/Match/Include/IgnoreUnknown) — none of those
	// can be caught by the probe itself, so they must never reach staging.
	if err := ValidateDirectiveName(name); err != nil {
		return DirectiveProof{}, err
	}
	if err := ValidateDirectiveValue(value); err != nil {
		return DirectiveProof{}, err
	}
	// WR-10: Deps is a plain struct of function fields; "every field is
	// non-nil in the real BuildProbeDeps wiring" is a convention, not an
	// enforcement, and this project carries a documented RECURRING
	// injected-seam wiring blindspot. Fail closed here rather than let a
	// nil deps.RunSSHGCombined panic in what is normally a tea.Cmd
	// goroutine.
	if deps.RunSSHGCombined == nil {
		return DirectiveProof{}, errors.New("globalssh: RunSSHGCombined seam is not wired — refusing to report an unproven directive as accepted")
	}

	tmpDir, err := os.MkdirTemp("", "gitid-directive-proof-*")
	if err != nil {
		return DirectiveProof{}, fmt.Errorf("globalssh: creating staged directive-proof directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// OpenSSH refuses a group- or world-writable config directory — the same
	// constraint Simulate already enforces on its mirror root (shadow.go:218).
	if err := os.Chmod(tmpDir, 0o700); err != nil { //nolint:gosec // explicitly setting restrictive mode 0700 for OpenSSH compatibility
		return DirectiveProof{}, fmt.Errorf("globalssh: setting staged directive-proof directory mode: %w", err)
	}

	stagedPath := filepath.Join(tmpDir, "staged_ssh_config")
	stagedContent := stageDirectiveConfig(currentGlobalBody, name, value)
	if err := os.WriteFile(stagedPath, []byte(stagedContent), 0o600); err != nil {
		return DirectiveProof{}, fmt.Errorf("globalssh: writing staged directive-proof config: %w", err)
	}

	args := []string{"-F", stagedPath, "-G", ProbeHost}
	cmdString := exec.Command("ssh", args...).String() //nolint:gosec // arg-slice form for display only; not executed here (G204)

	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	out, runErr := deps.RunSSHGCombined(ctx, args...)

	proof := DirectiveProof{Command: cmdString, Output: out}

	if runErr == nil {
		proof.OK = true
		return proof, nil
	}

	if strings.TrimSpace(out) == "" {
		// The probe produced no output at all: the binary could not run, a
		// timeout fired before anything was captured, or some other
		// transport-level failure. There is nothing here to classify, so
		// fail CLOSED rather than guess (an unproven directive must never be
		// treated as accepted, and must never be silently blamed on a name
		// or value it never got to evaluate).
		return proof, fmt.Errorf("globalssh: staged directive proof could not run: %w", runErr)
	}

	offending, hasMarker := offendingDirectiveName(out)
	if hasMarker {
		proof.OffendingName = offending
		if strings.EqualFold(offending, name) {
			proof.UnknownName = true
		} else {
			proof.PreexistingError = true
		}
		return proof, nil
	}

	// The name is recognized (no "Bad configuration option: " diagnostic),
	// but the staged probe still failed — a value/syntax rejection, reported
	// with the real output and no name-related flag set (D-I's fourth
	// outcome).
	return proof, nil
}

// ResolveDirectiveValue stages name/value ALONE — an isolated `Host *\n
// <name> <value>\n` config, deliberately NOT mixed with any existing
// directive body — and returns the CANONICALISED value the locally
// installed OpenSSH resolves it to (WR-01, 09.5-REVIEW.md round 2). This is
// the exact canonicalisation `ssh -G` applies to the machine's live
// configuration: `yes` becomes `true`, a leading zero on a numeric value is
// stripped, quotes around a path are stripped, and a `+`/`-`/`^`
// list-modifier expands into the full resolved list. Comparing a caller's
// raw TYPED value against a machine's RESOLVED value (as
// customDirectiveVerifyAdvisories used to) means EVERY one of those
// canonicalisations makes an honest, successful write look like a broken,
// shadowed one. Resolving BOTH sides of the comparison through the SAME ssh
// -G pass — the candidate value here, and the machine's actual resolved
// value via AllDirectives — makes the comparison apples-to-apples.
//
// The isolated staging (no existing body) is deliberate: this function
// answers "what does OpenSSH resolve THIS value to", not "what does the
// whole machine resolve to" — mixing in unrelated directives could let an
// unrelated shadowing rule change the answer for a key that is not actually
// in question here.
func ResolveDirectiveValue(deps Deps, name, value string) (string, error) {
	if deps.RunSSHG == nil {
		return "", errors.New("globalssh: RunSSHG seam is not wired — refusing to resolve a directive value")
	}

	tmpDir, err := os.MkdirTemp("", "gitid-directive-resolve-*")
	if err != nil {
		return "", fmt.Errorf("globalssh: creating staged directive-resolve directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := os.Chmod(tmpDir, 0o700); err != nil { //nolint:gosec // explicitly setting restrictive mode 0700 for OpenSSH compatibility
		return "", fmt.Errorf("globalssh: setting staged directive-resolve directory mode: %w", err)
	}

	stagedPath := filepath.Join(tmpDir, "staged_ssh_config")
	stagedContent := stageDirectiveConfig("", name, value)
	if err := os.WriteFile(stagedPath, []byte(stagedContent), 0o600); err != nil {
		return "", fmt.Errorf("globalssh: writing staged directive-resolve config: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	out, runErr := deps.RunSSHG(ctx, "-F", stagedPath, "-G", ProbeHost)
	if runErr != nil {
		return "", fmt.Errorf("globalssh: staged directive resolve could not run: %w", runErr)
	}

	resolved := parseResolvedOptions(out)
	v, ok := resolved[strings.ToLower(name)]
	if !ok {
		return "", fmt.Errorf("globalssh: resolved directive set has no key %q", name)
	}
	return v, nil
}

// stageDirectiveConfig builds the throwaway config's full text: the current
// global block body (which already contains its own `Host *` line whenever
// a block exists on disk) followed by the candidate `name value` line,
// indented like every other directive inside the wildcard stanza. When
// currentGlobalBody is empty (no block written yet) or does not already
// carry a `Host *` line, one is prepended so the candidate line still lands
// inside a wildcard stanza rather than at file scope.
func stageDirectiveConfig(currentGlobalBody, name, value string) string {
	body := strings.TrimRight(currentGlobalBody, "\n")
	var b strings.Builder
	if !hasHostStarLine(body) {
		b.WriteString("Host *\n")
	}
	if body != "" {
		b.WriteString(body)
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "  %s %s\n", name, value)
	return b.String()
}

// hasHostStarLine reports whether body already contains a `Host *` line —
// case-insensitively, tolerating leading whitespace, exactly as OpenSSH
// itself would recognize the directive.
func hasHostStarLine(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.EqualFold(fields[0], "Host") && fields[1] == "*" {
			return true
		}
	}
	return false
}

// offendingDirectiveName extracts the directive name from a
// `Bad configuration option: <name>` diagnostic line inside out. The second
// return value is false when the marker is absent, so a caller never
// confuses an empty extraction with "no marker present".
func offendingDirectiveName(out string) (string, bool) {
	idx := strings.Index(out, badConfigOptionMarker)
	if idx < 0 {
		return "", false
	}
	rest := out[idx+len(badConfigOptionMarker):]
	if nl := strings.IndexAny(rest, "\r\n"); nl >= 0 {
		rest = rest[:nl]
	}
	return strings.TrimSpace(rest), true
}
