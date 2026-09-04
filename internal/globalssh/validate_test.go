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

// TestStageDirectiveConfigPlacesCandidateBeforeExistingBodySoItWins is the
// WR-02 regression: when the existing body already sets the SAME directive
// name the candidate is replacing, the candidate line must appear BEFORE
// the existing body in the staged text — ssh_config resolution is
// first-value-wins, so a candidate staged AFTER the existing value would
// resolve to the value being REPLACED, not the value being written, making
// the staged probe (and stage 2's "real result" render) lie about what the
// confirmed write will actually produce.
func TestStageDirectiveConfigPlacesCandidateBeforeExistingBodySoItWins(t *testing.T) {
	staged := stageDirectiveConfig(existingGlobalBodyFixture, "StrictHostKeyChecking", "yes")
	candidateIdx := strings.Index(staged, "StrictHostKeyChecking yes")
	existingIdx := strings.Index(staged, "StrictHostKeyChecking accept-new")
	if candidateIdx == -1 {
		t.Fatalf("staged config missing the candidate line, got:\n%s", staged)
	}
	if existingIdx == -1 {
		t.Fatalf("staged config missing the pre-existing value it is replacing, got:\n%s", staged)
	}
	if candidateIdx >= existingIdx {
		t.Errorf("candidate line at offset %d must come BEFORE the existing value at offset %d (first-match-wins) — got:\n%s", candidateIdx, existingIdx, staged)
	}
}

// TestProveCustomDirectiveResolvesTheCandidateValueNotTheReplacedOne is the
// WR-02 end-to-end regression against the real staged probe: when the
// existing body already sets StrictHostKeyChecking to accept-new and the
// candidate replaces it with yes, the staged proof's Output must resolve
// StrictHostKeyChecking to the CANDIDATE's canonicalised value (true, ssh
// -G's resolution of yes), not the value being replaced.
func TestProveCustomDirectiveResolvesTheCandidateValueNotTheReplacedOne(t *testing.T) {
	// The fake seam here plays the role of a real ssh -G: it parses the
	// staged file itself and resolves StrictHostKeyChecking by the SAME
	// first-value-wins rule real OpenSSH applies, so this test proves the
	// STAGING order is correct without requiring a real ssh binary.
	wrapped := func(_ context.Context, args ...string) (string, error) {
		for i, a := range args {
			if a == "-F" && i+1 < len(args) {
				staged, rerr := os.ReadFile(args[i+1]) //nolint:gosec // test reads its own staged temp file
				if rerr != nil {
					t.Fatalf("reading staged config: %v", rerr)
				}
				for _, line := range strings.Split(string(staged), "\n") {
					fields := strings.Fields(line)
					if len(fields) == 2 && strings.EqualFold(fields[0], "StrictHostKeyChecking") {
						resolved := fields[1]
						if resolved == "yes" {
							resolved = "true" // mimic ssh -G's own yes -> true canonicalisation
						}
						return "stricthostkeychecking " + resolved + "\n", nil
					}
				}
			}
		}
		return "", nil
	}
	proof, err := ProveCustomDirective(Deps{RunSSHGCombined: wrapped}, existingGlobalBodyFixture, "StrictHostKeyChecking", "yes")
	if err != nil {
		t.Fatalf("ProveCustomDirective: %v", err)
	}
	if !strings.Contains(proof.Output, "stricthostkeychecking true") {
		t.Errorf("proof.Output = %q, want it to resolve to the CANDIDATE value (true), not the pre-existing accept-new", proof.Output)
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

// TestIsStructuralDirectiveNameRecognizesTheSameSetValidateDirectiveNameRejects
// verifies WR-08's suppression seam: IsStructuralDirectiveName reports true
// for exactly the keywords ValidateDirectiveName rejects as structural
// (case-insensitively), and false for an ordinary settable directive.
func TestIsStructuralDirectiveNameRecognizesTheSameSetValidateDirectiveNameRejects(t *testing.T) {
	for _, name := range []string{"Host", "host", "Match", "MATCH", "Include", "IgnoreUnknown"} {
		if !IsStructuralDirectiveName(name) {
			t.Errorf("IsStructuralDirectiveName(%q) = false, want true", name)
		}
	}
	if IsStructuralDirectiveName("StreamLocalBindMask") {
		t.Error("IsStructuralDirectiveName(StreamLocalBindMask) = true, want false")
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

// TestValidateDirectiveValueRejectsWhitespaceDelimitedHash is the WR-03
// (09.5-REVIEW.md round 3) regression: OpenSSH treats a '#' preceded by
// whitespace (or at position 0) as a comment introducer and silently
// discards everything from it onward — proven against the real OpenSSH
// binary (CLAUDE.md's hypothesis -> test -> implementation method):
//
//	$ printf 'Host *\n  ServerAliveInterval 60 #note\n' > p
//	$ ssh -F p -G gitid-probe.invalid | grep serveraliveinterval
//	serveraliveinterval 60           <- "#note" silently gone
//
// Before this fix, ProveCustomDirective's staged probe (and the post-write
// resolved-vs-resolved advisory) compared the ALREADY-TRUNCATED resolved
// values, so the write was honestly proven but the RECEIPT (which shows the
// full raw "60 #note") was not. An inline '#' with no PRECEDING whitespace
// (e.g. "FOO=bar#baz") is safe — verified live above (setenv FOO=bar#baz
// resolves whole) — so the guard is narrower than a blanket '#' rejection.
func TestValidateDirectiveValueRejectsWhitespaceDelimitedHash(t *testing.T) {
	rejected := []string{"60 #note", "#note", "60\t#note", "60  #note"}
	for _, value := range rejected {
		t.Run(value, func(t *testing.T) {
			if err := ValidateDirectiveValue(value); err == nil {
				t.Errorf("ValidateDirectiveValue(%q): expected error (whitespace-delimited '#'), got nil", value)
			}
		})
	}
}

// TestValidateDirectiveValueAcceptsAnInlineHashWithNoPrecedingWhitespace
// verifies the guard is narrow: a '#' with no preceding whitespace is part
// of the value's own text (git-config-style values legitimately embed it,
// e.g. an env-var assignment), not a comment introducer, and must remain
// accepted.
func TestValidateDirectiveValueAcceptsAnInlineHashWithNoPrecedingWhitespace(t *testing.T) {
	if err := ValidateDirectiveValue("FOO=bar#baz"); err != nil {
		t.Errorf("ValidateDirectiveValue(FOO=bar#baz): unexpected error: %v", err)
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

// TestProveCustomDirectiveNeverMisclassifiesAMultiTokenNameAsAPreexistingError
// is the WR-03 regression: a directive name containing whitespace used to
// reach the staged ssh -G probe, which reports only the FIRST field of a
// multi-token candidate line (`Bad configuration option: foo` for a staged
// "Foo Bar <value>" line) — offendingDirectiveName's EqualFold comparison
// against the WHOLE submitted name then never matched, so the result was
// misclassified as PreexistingError (a false accusation about the user's
// own pre-existing config, D-I's whole purpose being to never do that).
// CR-02's ValidateDirectiveName now rejects a multi-token name BEFORE any
// staging happens, so this branch is provably unreachable: the fake runner
// (primed with exactly the misclassifying output a real ssh -G would
// produce) must never be invoked, and the result must be a plain error, not
// a PreexistingError classification.
func TestProveCustomDirectiveNeverMisclassifiesAMultiTokenNameAsAPreexistingError(t *testing.T) {
	f := &fakeCombinedRunner{
		out: "/tmp/staged: line 2: Bad configuration option: foo\n" +
			"/tmp/staged: terminating, 1 bad configuration options\n",
		err: errors.New("exit status 255"),
	}
	proof, err := ProveCustomDirective(depsWithCombined(f), existingGlobalBodyFixture, "Foo Bar", "somevalue")
	if err == nil {
		t.Fatal("ProveCustomDirective with a multi-token name: expected error, got nil")
	}
	if proof.PreexistingError {
		t.Error("proof.PreexistingError = true — WR-03 regressed: a multi-token name must never reach the misclassification branch")
	}
	if f.calls != 0 {
		t.Errorf("the staged ssh -G probe ran %d time(s) for a multi-token name — must fail closed BEFORE staging (WR-03)", f.calls)
	}
}

// TestProveCustomDirectiveFailsClosedWhenRunSSHGCombinedIsNil is the WR-10
// regression: Deps is a plain struct of function fields, and the doc on Deps
// says "every field is non-nil in the real BuildProbeDeps wiring" — but that
// is a convention, not an enforcement (this project carries a documented
// RECURRING injected-seam wiring blindspot). A nil RunSSHGCombined must
// produce the documented fail-closed (DirectiveProof{OK:false}, err), never
// a panic in a tea.Cmd goroutine.
func TestProveCustomDirectiveFailsClosedWhenRunSSHGCombinedIsNil(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ProveCustomDirective panicked with a nil RunSSHGCombined seam: %v — must fail closed instead", r)
		}
	}()
	proof, err := ProveCustomDirective(Deps{GOOS: "linux"}, existingGlobalBodyFixture, "StreamLocalBindMask", "0177")
	if err == nil {
		t.Fatal("ProveCustomDirective with a nil RunSSHGCombined seam: expected a non-nil error, got nil")
	}
	if proof.OK {
		t.Error("proof.OK = true, want false for an unwired seam")
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

// TestProveCustomDirectiveStagesReplacementNotAccumulationForAMultiValuedName
// is the WR-08 (09.5-REVIEW.md round 3) end-to-end regression: OpenSSH
// ACCUMULATES some directive names (IdentityFile, CertificateFile,
// LocalForward, RemoteForward, DynamicForward, PermitRemoteOpen, …), so the
// OLD hand-assembled staging ("candidate line, then the existing body
// verbatim") resolved the UNION of the candidate and the existing value for
// those names — over-reporting what the write will actually produce. The
// real write goes through sshconfig.EnsureGlobals's explicit overlay ->
// merged.set(name, value), which REPLACES the existing value in place for
// EVERY directive name uniformly. This proves ProveCustomDirective's OWN
// staged file (captured mid-call, before its defer removes it) carries
// exactly ONE "IdentityFile" line — the candidate, not the union.
func TestProveCustomDirectiveStagesReplacementNotAccumulationForAMultiValuedName(t *testing.T) {
	existingBody := "Host *\n  IdentityFile /tmp/a\n"
	f := &fakeCombinedRunner{out: "identityfile /tmp/b\n", err: nil}
	var capturedStagedBody string
	wrapped := func(ctx context.Context, args ...string) (string, error) {
		for i, a := range args {
			if a == "-F" && i+1 < len(args) {
				body, rerr := os.ReadFile(args[i+1]) //nolint:gosec // test reads its own staged temp file
				if rerr != nil {
					t.Fatalf("reading staged config during the call: %v", rerr)
				}
				capturedStagedBody = string(body)
			}
		}
		return f.run(ctx, args...)
	}
	proof, err := ProveCustomDirective(Deps{RunSSHGCombined: wrapped, GOOS: "linux"}, existingBody, "IdentityFile", "/tmp/b")
	if err != nil {
		t.Fatalf("ProveCustomDirective: %v", err)
	}
	if !proof.OK {
		t.Fatalf("expected an accepted proof, got: %+v", proof)
	}
	if n := strings.Count(strings.ToLower(capturedStagedBody), "identityfile"); n != 1 {
		t.Errorf("staged config contains %d IdentityFile lines, want exactly 1 (candidate REPLACES the existing value, matching the real write):\n%s", n, capturedStagedBody)
	}
	if strings.Contains(capturedStagedBody, "/tmp/a") {
		t.Errorf("staged config must not carry the REPLACED value /tmp/a, got:\n%s", capturedStagedBody)
	}
}

// TestStageDirectiveConfigForWriteReplacesRatherThanAccumulates is
// WR-08's direct, white-box proof: the NEW staging helper
// ProveCustomDirective uses for its "what will the write actually produce"
// text renders exactly ONE line for an accumulating directive name that
// already has an existing value — the candidate REPLACES it, matching
// sshconfig.EnsureGlobals's own merged.set semantics exactly (proven via
// TestRenderGlobalBodyWithOverlayMatchesEnsureGlobalsExactly in the
// sshconfig package; this test proves the globalssh-side staging call
// reaches that same renderer).
func TestStageDirectiveConfigForWriteReplacesRatherThanAccumulates(t *testing.T) {
	existingBody := "Host *\n  IdentityFile /tmp/a\n"
	staged := stageDirectiveConfigForWrite(Deps{GOOS: "linux"}, existingBody, "IdentityFile", "/tmp/b")
	if n := strings.Count(strings.ToLower(staged), "identityfile"); n != 1 {
		t.Errorf("staged config contains %d IdentityFile lines, want exactly 1 (the candidate REPLACES the existing value, matching the real write):\n%s", n, staged)
	}
	if !strings.Contains(staged, "IdentityFile /tmp/b") {
		t.Errorf("staged config missing the candidate value, got:\n%s", staged)
	}
	if strings.Contains(staged, "/tmp/a") {
		t.Errorf("staged config must not carry the REPLACED value /tmp/a, got:\n%s", staged)
	}
}

// TestStageDirectiveConfigForWritePlacesIgnoreUnknownGuardFirst is the WR-09
// (09.5-REVIEW.md round 3) regression: gitid's rendered block always begins
// with "IgnoreUnknown UseKeychain" (sshconfig.renderGlobalBody), and
// ssh_config(5) documents that IgnoreUnknown must be listed EARLY — "it
// will not be applied to unknown options that appear before it." The OLD
// hand-assembled staging put the candidate line at file scope AHEAD of the
// whole existing body (including its guard line), so a candidate like
// "UseKeychain yes" staged on Linux was reported UnknownName even though the
// file the write would actually produce places it AFTER the guard. The new
// staging helper must place the guard line (part of the canonical render)
// BEFORE the candidate.
func TestStageDirectiveConfigForWritePlacesIgnoreUnknownGuardFirst(t *testing.T) {
	staged := stageDirectiveConfigForWrite(Deps{GOOS: "linux"}, existingGlobalBodyFixture, "UseKeychain", "yes")
	guardIdx := strings.Index(staged, "IgnoreUnknown UseKeychain")
	candidateIdx := strings.Index(staged, "UseKeychain yes")
	if guardIdx == -1 {
		t.Fatalf("staged config missing the IgnoreUnknown guard line, got:\n%s", staged)
	}
	if candidateIdx == -1 {
		t.Fatalf("staged config missing the candidate line, got:\n%s", staged)
	}
	if guardIdx >= candidateIdx {
		t.Errorf("guard line at offset %d must come BEFORE the candidate at offset %d, got:\n%s", guardIdx, candidateIdx, staged)
	}
}

// fakeStdoutRunner is the RunSSHG (stdout-only) sibling of fakeCombinedRunner,
// for ResolveDirectiveValue's tests.
type fakeStdoutRunner struct {
	out      string
	err      error
	lastArgs []string
}

func (f *fakeStdoutRunner) run(_ context.Context, args ...string) (string, error) {
	f.lastArgs = append([]string(nil), args...)
	return f.out, f.err
}

// TestResolveDirectiveValueReturnsTheCanonicalisedResolution is the WR-01
// regression: ResolveDirectiveValue must return what the staged ssh -G probe
// resolved the candidate value to — the canonicalised form, not the raw
// input — so a caller comparing it against another resolved value never
// compares a typed representation against a resolved one.
func TestResolveDirectiveValueReturnsTheCanonicalisedResolution(t *testing.T) {
	f := &fakeStdoutRunner{out: "stricthostkeychecking true\n"}
	got, err := ResolveDirectiveValue(Deps{RunSSHG: f.run}, "StrictHostKeyChecking", "yes")
	if err != nil {
		t.Fatalf("ResolveDirectiveValue: %v", err)
	}
	if got != "true" {
		t.Errorf("got %q, want the canonicalised %q (ssh -G resolves yes -> true)", got, "true")
	}
	if len(f.lastArgs) < 2 || f.lastArgs[len(f.lastArgs)-2] != "-G" || f.lastArgs[len(f.lastArgs)-1] != ProbeHost {
		t.Errorf("recorded argv %v does not end with -G %s", f.lastArgs, ProbeHost)
	}
}

// TestResolveDirectiveValueStagesInIsolation asserts the staged config
// carries ONLY the candidate directive — no pre-existing body — so an
// unrelated directive can never influence this directive's resolution.
func TestResolveDirectiveValueStagesInIsolation(t *testing.T) {
	var capturedContent string
	wrapped := func(_ context.Context, args ...string) (string, error) {
		// Read the staged file HERE, synchronously, inside the fake seam
		// invocation — ResolveDirectiveValue removes tmpDir via defer right
		// after this call returns, so reading it back afterwards would race
		// the cleanup.
		for i, a := range args {
			if a == "-F" && i+1 < len(args) {
				staged, rerr := os.ReadFile(args[i+1]) //nolint:gosec // path is this test's own fake seam invocation
				if rerr != nil {
					t.Fatalf("reading staged config inside the fake seam: %v", rerr)
				}
				capturedContent = string(staged)
			}
		}
		return "streamlocalbindmask 0177\n", nil
	}
	if _, err := ResolveDirectiveValue(Deps{RunSSHG: wrapped}, "StreamLocalBindMask", "0177"); err != nil {
		t.Fatalf("ResolveDirectiveValue: %v", err)
	}
	if capturedContent == "" {
		t.Fatal("staged config content was never captured — ResolveDirectiveValue did not pass -F")
	}
	if strings.Contains(capturedContent, "ForwardAgent") || strings.Contains(capturedContent, "IdentitiesOnly") {
		t.Errorf("staged config carries unrelated directives, want ONLY the candidate — staged:\n%s", capturedContent)
	}
	if !strings.Contains(capturedContent, "StreamLocalBindMask 0177") {
		t.Errorf("staged config missing the candidate line, got:\n%s", capturedContent)
	}
}

// TestResolveDirectiveValueFailsClosedWhenRunSSHGIsNil mirrors
// TestProveCustomDirectiveFailsClosedWhenRunSSHGCombinedIsNil for the
// stdout-only seam.
func TestResolveDirectiveValueFailsClosedWhenRunSSHGIsNil(t *testing.T) {
	if _, err := ResolveDirectiveValue(Deps{}, "StrictHostKeyChecking", "yes"); err == nil {
		t.Fatal("ResolveDirectiveValue with a nil RunSSHG seam: expected a non-nil error, got nil")
	}
}

// TestResolveDirectiveValueErrorsWhenTheKeyIsAbsentFromTheResolution covers
// a structural/rejected candidate whose staged probe succeeds (exit 0) but
// whose resolved-options output never echoes the candidate's own key back —
// the caller must see an error, not a silently empty "" value that could be
// mistaken for a genuine resolved empty string.
func TestResolveDirectiveValueErrorsWhenTheKeyIsAbsentFromTheResolution(t *testing.T) {
	f := &fakeStdoutRunner{out: "hashknownhosts no\n"}
	if _, err := ResolveDirectiveValue(Deps{RunSSHG: f.run}, "StreamLocalBindMask", "0177"); err == nil {
		t.Fatal("ResolveDirectiveValue: expected an error when the resolved set has no entry for the candidate key")
	}
}
