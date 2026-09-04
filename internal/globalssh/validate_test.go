package globalssh

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

// fakeCombinedRunner returns a RunSSHGCombined seam that records every
// invocation's args and answers from a canned (output, error) queue-of-one —
// exactly the depsForOut style probe_test.go already uses for RunSSHG, mirrored
// here for the combined-output seam this file's classifier depends on.
type fakeCombinedRunner struct {
	out      string
	err      error
	lastArgs []string
	calls    int
}

func (f *fakeCombinedRunner) run(_ context.Context, args ...string) (string, error) {
	f.calls++
	f.lastArgs = append([]string(nil), args...)
	return f.out, f.err
}

func depsWithCombined(f *fakeCombinedRunner) Deps {
	return Deps{RunSSHGCombined: f.run, GOOS: "linux"}
}

const existingGlobalBodyFixture = "Host *\n  StrictHostKeyChecking accept-new\n  ForwardAgent no\n"

func TestProveCustomDirectiveAcceptsKnownDirective(t *testing.T) {
	f := &fakeCombinedRunner{out: "streamlocalbindmask 0177\n", err: nil}
	proof, err := ProveCustomDirective(depsWithCombined(f), existingGlobalBodyFixture, "StreamLocalBindMask", "0177")
	if err != nil {
		t.Fatalf("ProveCustomDirective returned error on a clean accept: %v", err)
	}
	if !proof.OK {
		t.Errorf("proof.OK = false, want true for a clean ssh -G exit")
	}
	if proof.UnknownName || proof.PreexistingError {
		t.Errorf("proof = %+v, want no name-related flags set on accept", proof)
	}
	if proof.Command == "" {
		t.Error("proof.Command is empty — want the exact staged-config command line")
	}
	if !strings.Contains(proof.Command, "-F") || !strings.Contains(proof.Command, "-G") {
		t.Errorf("proof.Command = %q, want -F <staged> -G %s", proof.Command, ProbeHost)
	}
	if proof.Output != f.out {
		t.Errorf("proof.Output = %q, want verbatim %q", proof.Output, f.out)
	}
}

func TestProveCustomDirectiveRejectsUnknownName(t *testing.T) {
	f := &fakeCombinedRunner{
		out: "/tmp/staged: line 2: Bad configuration option: thisisnotarealdirective\n" +
			"/tmp/staged: terminating, 1 bad configuration options\n",
		err: errors.New("exit status 255"),
	}
	proof, err := ProveCustomDirective(depsWithCombined(f), existingGlobalBodyFixture, "ThisIsNotARealDirective", "yes")
	if err != nil {
		t.Fatalf("ProveCustomDirective returned error classifying an unknown name: %v", err)
	}
	if proof.OK {
		t.Error("proof.OK = true, want false for an unrecognized directive name")
	}
	if !proof.UnknownName {
		t.Error("proof.UnknownName = false, want true — the offending name matches the candidate's own name")
	}
	if proof.PreexistingError {
		t.Error("proof.PreexistingError = true, want false — this is the candidate's own name, not a pre-existing problem")
	}
	if proof.OffendingName != "thisisnotarealdirective" {
		t.Errorf("proof.OffendingName = %q, want the lowercased candidate name", proof.OffendingName)
	}
}

func TestProveCustomDirectiveDistinguishesPreexistingConfigError(t *testing.T) {
	f := &fakeCombinedRunner{
		out: "/tmp/staged: line 1: Bad configuration option: someotherpreexistingdirective\n" +
			"/tmp/staged: terminating, 1 bad configuration options\n",
		err: errors.New("exit status 255"),
	}
	proof, err := ProveCustomDirective(depsWithCombined(f), existingGlobalBodyFixture, "StreamLocalBindMask", "0177")
	if err != nil {
		t.Fatalf("ProveCustomDirective returned error classifying a pre-existing config problem: %v", err)
	}
	if proof.OK {
		t.Error("proof.OK = true, want false")
	}
	if proof.UnknownName {
		t.Error("proof.UnknownName = true, want false — the offending name is NOT the candidate's own name")
	}
	if !proof.PreexistingError {
		t.Error("proof.PreexistingError = false, want true — a pre-existing problem must not be blamed on the new entry (D-I)")
	}
	if proof.OffendingName != "someotherpreexistingdirective" {
		t.Errorf("proof.OffendingName = %q, want the OTHER directive's lowercased name", proof.OffendingName)
	}
}

func TestProveCustomDirectiveTreatsOtherFailuresAsValueRejection(t *testing.T) {
	f := &fakeCombinedRunner{
		out: "/tmp/staged: line 3: Bad Port '99999'\n",
		err: errors.New("exit status 255"),
	}
	proof, err := ProveCustomDirective(depsWithCombined(f), existingGlobalBodyFixture, "Port", "99999")
	if err != nil {
		t.Fatalf("ProveCustomDirective returned error classifying a value rejection: %v", err)
	}
	if proof.OK {
		t.Error("proof.OK = true, want false")
	}
	if proof.UnknownName {
		t.Error("proof.UnknownName = true, want false — no 'Bad configuration option:' marker present")
	}
	if proof.PreexistingError {
		t.Error("proof.PreexistingError = true, want false — no 'Bad configuration option:' marker present")
	}
	if proof.Output != f.out {
		t.Errorf("proof.Output = %q, want the real verbatim output %q", proof.Output, f.out)
	}
}

func TestProveCustomDirectiveFailsClosedWhenTheProbeCannotRun(t *testing.T) {
	f := &fakeCombinedRunner{out: "", err: errors.New(`exec: "ssh": executable file not found in $PATH`)}
	proof, err := ProveCustomDirective(depsWithCombined(f), existingGlobalBodyFixture, "StreamLocalBindMask", "0177")
	if err == nil {
		t.Fatal("ProveCustomDirective returned a nil error for a transport-level failure — want fail-closed non-nil error")
	}
	if proof.OK {
		t.Error("proof.OK = true alongside a non-nil error — fail-closed property violated (no branch may return OK:true with err!=nil)")
	}
}

func TestProveCustomDirectiveStagesAThrowawayConfigAndNeverTouchesTheRealOne(t *testing.T) {
	realConfigPath := "/definitely/not/a/real/path/config"
	var capturedStagedPath string
	var capturedStagedBody []byte
	f := &fakeCombinedRunner{out: "streamlocalbindmask 0177\n", err: nil}
	// Wrap the fake so we can read the staged file's content DURING the call,
	// before ProveCustomDirective's defer removes it.
	wrapped := func(ctx context.Context, args ...string) (string, error) {
		for i, a := range args {
			if a == "-F" && i+1 < len(args) {
				capturedStagedPath = args[i+1]
				body, err := os.ReadFile(capturedStagedPath) //nolint:gosec // test reads its own staged temp file
				if err != nil {
					t.Fatalf("reading staged config during the call: %v", err)
				}
				capturedStagedBody = body
			}
		}
		return f.run(ctx, args...)
	}
	proof, err := ProveCustomDirective(Deps{RunSSHGCombined: wrapped}, existingGlobalBodyFixture, "StreamLocalBindMask", "0177")
	if err != nil {
		t.Fatalf("ProveCustomDirective: %v", err)
	}
	if !proof.OK {
		t.Fatal("expected an accepted proof for this test's fixture")
	}
	if capturedStagedPath == "" {
		t.Fatal("no -F argument was captured — the staged config path was never passed")
	}
	if capturedStagedPath == realConfigPath || strings.Contains(capturedStagedPath, "/definitely/not/a/real/path") {
		t.Fatalf("staged path = %q, must NEVER be the real config path", capturedStagedPath)
	}
	if !strings.Contains(capturedStagedPath, os.TempDir()) && !strings.HasPrefix(capturedStagedPath, "/tmp") && !strings.HasPrefix(capturedStagedPath, "/var") {
		t.Logf("staged path = %q (informational — temp dir location varies by platform)", capturedStagedPath)
	}
	bodyStr := string(capturedStagedBody)
	if !strings.Contains(bodyStr, "StrictHostKeyChecking accept-new") {
		t.Errorf("staged body = %q, want it to contain the CURRENT global block body", bodyStr)
	}
	if !strings.Contains(bodyStr, "StreamLocalBindMask 0177") {
		t.Errorf("staged body = %q, want it to contain the candidate name/value line", bodyStr)
	}
	// Post-condition: the staged file must be gone once ProveCustomDirective
	// returns — the throwaway config never survives the call.
	if _, statErr := os.Stat(capturedStagedPath); !os.IsNotExist(statErr) {
		t.Errorf("staged file %q still exists after ProveCustomDirective returned (stat err: %v) — the throwaway config must be removed", capturedStagedPath, statErr)
	}
}

// TestValidateDirectiveNameRejectsEmptyWhitespaceMultiTokenAndStructuralKeywords
// verifies CR-02's name-syntax guard: an empty name, a whitespace-only name,
// a multi-token name, and each structural keyword (Host/Match/Include/
// IgnoreUnknown, case-insensitively) must be rejected — ssh -G accepts all
// of these against a staged config (verified live against the real OpenSSH
// on this machine), so the staged probe alone cannot be trusted to reject
// them; they must be rejected BEFORE staging.
func TestValidateDirectiveNameRejectsEmptyWhitespaceMultiTokenAndStructuralKeywords(t *testing.T) {
	cases := []string{
		"", "   ", "Foo Bar", "Host", "host", "Match", "MATCH", "Include", "IgnoreUnknown",
	}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			if err := ValidateDirectiveName(name); err == nil {
				t.Errorf("ValidateDirectiveName(%q): expected error, got nil", name)
			}
		})
	}
}

// TestValidateDirectiveNameAcceptsAKnownShapedToken verifies a normal
// single-token directive name is accepted.
func TestValidateDirectiveNameAcceptsAKnownShapedToken(t *testing.T) {
	if err := ValidateDirectiveName("StreamLocalBindMask"); err != nil {
		t.Errorf("ValidateDirectiveName(StreamLocalBindMask): unexpected error: %v", err)
	}
}

// TestValidateDirectiveValueRejectsEmptyAndControlBytes verifies the value
// guard: empty/whitespace-only, and embedded newline/CR/NUL, are rejected.
func TestValidateDirectiveValueRejectsEmptyAndControlBytes(t *testing.T) {
	cases := []string{"", "   ", "bad\nvalue", "bad\rvalue", "bad\x00value"}
	for _, value := range cases {
		t.Run(value, func(t *testing.T) {
			if err := ValidateDirectiveValue(value); err == nil {
				t.Errorf("ValidateDirectiveValue(%q): expected error, got nil", value)
			}
		})
	}
}

// TestValidateDirectiveValueAcceptsAnOrdinaryValue verifies a normal value
// is accepted.
func TestValidateDirectiveValueAcceptsAnOrdinaryValue(t *testing.T) {
	if err := ValidateDirectiveValue("accept-new"); err != nil {
		t.Errorf("ValidateDirectiveValue(accept-new): unexpected error: %v", err)
	}
}

// TestProveCustomDirectiveFailsClosedOnAnEmptyNameWithoutStagingAProbe
// verifies CR-02's central fix: ProveCustomDirective rejects an empty name
// BEFORE the staged ssh -G probe ever runs — proven by asserting the fake
// runner is never invoked. Before the fix, this candidate reached ssh -G,
// which (verified live against the real OpenSSH on this machine) exits 0 on
// a whitespace-only directive line, so proof.OK would incorrectly be true.
func TestProveCustomDirectiveFailsClosedOnAnEmptyNameWithoutStagingAProbe(t *testing.T) {
	f := &fakeCombinedRunner{out: "streamlocalbindmask 0177\n", err: nil}
	proof, err := ProveCustomDirective(depsWithCombined(f), existingGlobalBodyFixture, "", "somevalue")
	if err == nil {
		t.Fatal("ProveCustomDirective with an empty name: expected error, got nil")
	}
	if proof.OK {
		t.Error("proof.OK = true, want false for an empty name")
	}
	if f.calls != 0 {
		t.Errorf("the staged ssh -G probe ran %d time(s) for an empty name — must fail closed BEFORE staging", f.calls)
	}
}

// TestProveCustomDirectiveFailsClosedOnAStructuralKeywordNameWithoutStagingAProbe
// mirrors the above for each structural keyword — verified live against the
// real OpenSSH on this machine to exit 0 (Host/Match/Include all open a
// nested stanza or file reference inside gitid's own Host * block, which
// EnsureGlobals silently drops on its next write).
func TestProveCustomDirectiveFailsClosedOnAStructuralKeywordNameWithoutStagingAProbe(t *testing.T) {
	for _, name := range []string{"Host", "Match", "Include", "IgnoreUnknown"} {
		t.Run(name, func(t *testing.T) {
			f := &fakeCombinedRunner{out: "ok\n", err: nil}
			proof, err := ProveCustomDirective(depsWithCombined(f), existingGlobalBodyFixture, name, "evil.example")
			if err == nil {
				t.Fatalf("ProveCustomDirective with structural name %q: expected error, got nil", name)
			}
			if proof.OK {
				t.Errorf("proof.OK = true, want false for structural name %q", name)
			}
			if f.calls != 0 {
				t.Errorf("the staged ssh -G probe ran %d time(s) for structural name %q — must fail closed BEFORE staging", f.calls, name)
			}
		})
	}
}

func TestProveCustomDirectiveProbesTheWildcardSentinel(t *testing.T) {
	f := &fakeCombinedRunner{out: "streamlocalbindmask 0177\n", err: nil}
	_, err := ProveCustomDirective(depsWithCombined(f), existingGlobalBodyFixture, "StreamLocalBindMask", "0177")
	if err != nil {
		t.Fatalf("ProveCustomDirective: %v", err)
	}
	if len(f.lastArgs) < 2 {
		t.Fatalf("recorded argv %v is too short", f.lastArgs)
	}
	last, secondLast := f.lastArgs[len(f.lastArgs)-1], f.lastArgs[len(f.lastArgs)-2]
	if secondLast != "-G" || last != ProbeHost {
		t.Errorf("recorded argv %v does not end with -G %s (the same wildcard-only sentinel the browse path uses)", f.lastArgs, ProbeHost)
	}
}
