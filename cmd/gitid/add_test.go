package main

import (
	"bufio"
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
	"github.com/castocolina/gitid/internal/tester"
)

// fakeDeps returns a fully-faked identity.Deps so the add handler test never
// touches the network, the real keygen, or the filesystem.
func fakeDeps(_ io.Writer) identity.Deps {
	return identity.Deps{
		Generate: func(in identity.CreateInput) (identity.StagedKey, error) {
			return identity.StagedKey{
				TempPrivatePath:  "/tmp/stage/key",
				FinalPrivatePath: "/tmp/.ssh/id_ed25519_" + in.Name,
				FinalPubPath:     "/tmp/.ssh/id_ed25519_" + in.Name + ".pub",
				PubLine:          "ssh-ed25519 AAAAFAKE comment\n",
				PrivPEM:          []byte("FAKEPEM"),
			}, nil
		},
		PersistKey: func(s identity.StagedKey) (identity.KeyResult, error) {
			return identity.KeyResult{
				PrivatePath: s.FinalPrivatePath,
				PubPath:     s.FinalPubPath,
				PubLine:     s.PubLine,
			}, nil
		},
		Cleanup: func(_ identity.StagedKey) {},
		CopyPub: func(_ string) error { return nil },
		PreWrite: func(_, hostname string, _ int) tester.Result {
			return tester.Result{Command: "ssh -T git@" + hostname, Output: "Permission denied (publickey)", Outcome: tester.ReachableNotUploaded}
		},
		WriteSSH:            func(_, _, _ string) (string, error) { return "", nil },
		WriteGitconfig:      func(_, _, _ string, _ []gitconfig.Match) (string, error) { return "", nil },
		WriteFragment:       func(_, _, _, _ string, _ bool) error { return nil },
		WriteAllowedSigners: func(_, _, _ string) (string, error) { return "", nil },
		Resolved: func(alias string) (tester.Result, tester.ResolvedConfig) {
			return tester.Result{Command: "ssh -T git@" + alias, Output: "ok"}, tester.ResolvedConfig{User: "git"}
		},
		PubExists: func(_ string) bool { return true },
		DerivePub: func(_, _ string) (string, error) { return "ssh-ed25519 AAAADERIVED comment\n", nil },
		WritePub:  func(_, _ string) error { return nil },
	}
}

// fakeLoopDeps builds a Deps for runCreateLoop tests. preOutcome drives the
// PreWrite result; writeCount tracks how many times PersistAll-equivalent writes
// would be invoked (WriteSSH is the representative counter).
func fakeLoopDeps(preOutcome tester.Outcome) identity.Deps {
	d := fakeDeps(nil)
	d.PreWrite = func(_, _ string, _ int) tester.Result {
		return tester.Result{
			Command: "ssh -T git@ssh.github.com",
			Output:  "pre-write output",
			Outcome: preOutcome,
		}
	}
	return d
}

// fakeLoopDepsSeq builds a Deps whose PreWrite returns outcomes in sequence
// (first call returns seq[0], second seq[1], etc.; last element repeated after).
func fakeLoopDepsSeq(seq []tester.Outcome) (deps identity.Deps, callCount *int) {
	count := 0
	callCount = &count
	d := fakeDeps(nil)
	d.PreWrite = func(_, _ string, _ int) tester.Result {
		idx := count
		if idx >= len(seq) {
			idx = len(seq) - 1
		}
		count++
		return tester.Result{Outcome: seq[idx]}
	}
	return d, callCount
}

func sampleLoopInput() identity.CreateInput {
	return identity.CreateInput{
		Name:               "looptest",
		GitName:            "Loop User",
		GitEmail:           "loop@example.com",
		Provider:           "github",
		Alias:              "looptest.github",
		Hostname:           "ssh.github.com",
		Port:               443,
		AllowedSignersPath: "/tmp/.ssh/allowed_signers",
	}
}

func sampleLoopStaged() identity.StagedKey {
	return identity.StagedKey{
		TempPrivatePath:  "/tmp/.ssh/id_ed25519_looptest",
		FinalPrivatePath: "/tmp/.ssh/id_ed25519_looptest",
		FinalPubPath:     "/tmp/.ssh/id_ed25519_looptest.pub",
		PubLine:          "ssh-ed25519 AAAAFAKE comment\n",
		PrivPEM:          []byte("FAKEPEM"),
	}
}

// TestRunCreateLoop_PASSAutoPermits asserts that on tester.PASS, runCreateLoop
// returns (true, false, nil) immediately with NO loop prompt (D-03).
func TestRunCreateLoop_PASSAutoPermits(t *testing.T) {
	deps := fakeLoopDeps(tester.PASS)
	in := strings.NewReader("") // no stdin consumed when first PreWrite is PASS
	var out bytes.Buffer

	persist, skip, err := runCreateLoop(bufio.NewReader(in), &out, sampleLoopInput(), sampleLoopStaged(), deps)
	if err != nil {
		t.Fatalf("runCreateLoop(PASS) returned error: %v", err)
	}
	if !persist {
		t.Error("runCreateLoop(PASS): persist must be true (D-03 auto-persist)")
	}
	if skip {
		t.Error("runCreateLoop(PASS): skipConfirmed must be false (PASS is authenticated, not skipped)")
	}
	// No loop menu shown when first check is PASS.
	if strings.Contains(out.String(), "[r] retry") {
		t.Errorf("runCreateLoop(PASS): must NOT show retry menu when first result is PASS; got:\n%s", out.String())
	}
}

// TestRunCreateLoop_QuitKeepsKeyNoConfig asserts quit returns (false, false) and
// prints the key-kept message; PersistAll is NOT called (D-04).
func TestRunCreateLoop_QuitKeepsKeyNoConfig(t *testing.T) {
	deps := fakeLoopDeps(tester.ReachableNotUploaded)
	var writeCount int
	origWrite := deps.WriteSSH
	deps.WriteSSH = func(_, _, _ string) (string, error) {
		writeCount++
		return origWrite("", "", "")
	}

	stdin := strings.NewReader("q\n")
	var out bytes.Buffer

	persist, skip, err := runCreateLoop(bufio.NewReader(stdin), &out, sampleLoopInput(), sampleLoopStaged(), deps)
	if err != nil {
		t.Fatalf("runCreateLoop(quit) returned error: %v", err)
	}
	if persist {
		t.Error("runCreateLoop(quit): persist must be false (D-04: quit writes no config)")
	}
	if skip {
		t.Error("runCreateLoop(quit): skipConfirmed must be false")
	}
	if writeCount != 0 {
		t.Errorf("runCreateLoop(quit): WriteSSH must not be called; called %d times", writeCount)
	}
	// The key-kept message must mention the key path.
	if !strings.Contains(out.String(), sampleLoopStaged().FinalPrivatePath) {
		t.Errorf("runCreateLoop(quit): output must mention the key path %q; got:\n%s",
			sampleLoopStaged().FinalPrivatePath, out.String())
	}
}

// TestRunCreateLoop_SkipWithConfirmPersists asserts that stdin "s" then "y"
// returns (true, true) after printing the not-yet-authenticated warning (D-05).
func TestRunCreateLoop_SkipWithConfirmPersists(t *testing.T) {
	deps := fakeLoopDeps(tester.ReachableNotUploaded)
	stdin := strings.NewReader("s\ny\n")
	var out bytes.Buffer

	persist, skipConfirmed, err := runCreateLoop(bufio.NewReader(stdin), &out, sampleLoopInput(), sampleLoopStaged(), deps)
	if err != nil {
		t.Fatalf("runCreateLoop(skip+y) returned error: %v", err)
	}
	if !persist {
		t.Error("runCreateLoop(skip+y): persist must be true (D-05 skip+confirm)")
	}
	if !skipConfirmed {
		t.Error("runCreateLoop(skip+y): skipConfirmed must be true")
	}
	// The warning about unauthenticated key must be shown.
	outStr := out.String()
	if !strings.Contains(strings.ToLower(outStr), "not yet authenticated") &&
		!strings.Contains(strings.ToLower(outStr), "not authenticated") {
		t.Errorf("runCreateLoop(skip+y): must print authentication warning; got:\n%s", outStr)
	}
}

// TestRunCreateLoop_SkipDeclinedLoops asserts that stdin "s" then "n" does NOT
// persist and the loop continues (no early return, no error).
func TestRunCreateLoop_SkipDeclinedLoops(t *testing.T) {
	// After skip+n, the loop must continue. We give it 'q' to quit.
	deps := fakeLoopDeps(tester.ReachableNotUploaded)
	stdin := strings.NewReader("s\nn\nq\n")
	var out bytes.Buffer

	persist, _, err := runCreateLoop(bufio.NewReader(stdin), &out, sampleLoopInput(), sampleLoopStaged(), deps)
	if err != nil {
		t.Fatalf("runCreateLoop(skip+n then quit) returned error: %v", err)
	}
	if persist {
		t.Error("runCreateLoop(skip+n then quit): persist must be false (skip declined, then quit)")
	}
}

// TestRunCreateLoop_RetryThenPASSPersists asserts that "r" (retry) loops back and
// on the second call (PASS) auto-persists (D-02 retry loop).
func TestRunCreateLoop_RetryThenPASSPersists(t *testing.T) {
	deps, _ := fakeLoopDepsSeq([]tester.Outcome{tester.ReachableNotUploaded, tester.PASS})
	stdin := strings.NewReader("r\n") // first response is retry; second PreWrite is PASS → auto-persist
	var out bytes.Buffer

	persist, skipConfirmed, err := runCreateLoop(bufio.NewReader(stdin), &out, sampleLoopInput(), sampleLoopStaged(), deps)
	if err != nil {
		t.Fatalf("runCreateLoop(retry→PASS) returned error: %v", err)
	}
	if !persist {
		t.Error("runCreateLoop(retry→PASS): persist must be true after PASS on retry")
	}
	if skipConfirmed {
		t.Error("runCreateLoop(retry→PASS): skipConfirmed must be false (was auto-persist via PASS)")
	}
}

// TestRunCreateLoop_ReachableNotUploadedNeverAutoPersists is the D-06 regression
// guard: ReachableNotUploaded alone must NEVER cause auto-persist.
func TestRunCreateLoop_ReachableNotUploadedNeverAutoPersists(t *testing.T) {
	deps := fakeLoopDeps(tester.ReachableNotUploaded)
	// Send quit so we don't block forever; the assert is that we reach the quit
	// branch without having auto-persisted.
	stdin := strings.NewReader("q\n")
	var out bytes.Buffer

	persist, _, err := runCreateLoop(bufio.NewReader(stdin), &out, sampleLoopInput(), sampleLoopStaged(), deps)
	if err != nil {
		t.Fatalf("runCreateLoop(ReachableNotUploaded→quit) returned error: %v", err)
	}
	if persist {
		t.Errorf("runCreateLoop: ReachableNotUploaded must NEVER auto-persist (D-06); persist=%v", persist)
	}
}

// TestCreateNoWriteAllFourPromptInOutput asserts the old "Write all four
// artifacts now?" prompt is gone from the create-new output (D-02 removed).
func TestCreateNoWriteAllFourPromptInOutput(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// PASS on first PreWrite → auto-persist, no extra prompt.
	fakeDepsPass := func(_ io.Writer) identity.Deps {
		d := fakeDeps(nil)
		d.PreWrite = func(_, _ string, _ int) tester.Result {
			return tester.Result{Outcome: tester.PASS}
		}
		return d
	}

	in := strings.NewReader(strings.Join([]string{
		"1", // create-new
		"testid",
		"Test User",
		"t@example.com",
		"github",
		"", "", "", // alias/hostname/port defaults
		"", "", // strategy (default=1=gitdir) + gitdir value defaults
		"", // passphrase
	}, "\n") + "\n")

	var out bytes.Buffer
	// dryRun=false; PASS → should auto-persist, definitely no "Write all four" prompt
	if err := runIdentityAdd(in, &out, false, addFlags{}, fakeDepsPass); err != nil {
		t.Logf("runIdentityAdd returned error (acceptable during RED): %v\noutput:\n%s", err, out.String())
	}
	if strings.Contains(out.String(), "Write all four artifacts now?") {
		t.Errorf("output must not contain removed 'Write all four artifacts now?' prompt;\n%s", out.String())
	}
}

// TestUploadInstructionsGitHubBothKeys asserts the GitHub copy covers BOTH the
// authentication key and the signing key registrations and the settings URL
// (UP-01/UP-02).
func TestUploadInstructionsGitHubBothKeys(t *testing.T) {
	got := uploadInstructions("github")
	for _, want := range []string{
		"https://github.com/settings/ssh/new",
		"Authentication key",
		"Signing key",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("uploadInstructions(github) missing %q\n--- got ---\n%s", want, got)
		}
	}
}

// TestUploadInstructionsGitLabOneKey asserts the GitLab copy points at the
// GitLab key settings page and notes a single key serves both roles.
func TestUploadInstructionsGitLabOneKey(t *testing.T) {
	got := uploadInstructions("gitlab")
	if !strings.Contains(got, "https://gitlab.com/-/user_settings/ssh_keys") {
		t.Errorf("uploadInstructions(gitlab) missing settings URL\n%s", got)
	}
	if !strings.Contains(strings.ToLower(got), "authentication & signing") {
		t.Errorf("uploadInstructions(gitlab) should note the key is for both auth and signing\n%s", got)
	}
}

// TestUploadInstructionsUnknownNonEmpty ensures an unknown provider still gets
// non-empty guidance.
func TestUploadInstructionsUnknownNonEmpty(t *testing.T) {
	if strings.TrimSpace(uploadInstructions("bitbucket")) == "" {
		t.Fatal("uploadInstructions(unknown) must not be empty")
	}
}

// TestRunIdentityAddDryRunDoesNotPanic drives the add handler in --dry-run mode
// with scripted prompt input and an isolated temp HOME, asserting it completes
// without panicking and writes nothing (the dry-run path performs no file
// mutations). It uses the recover panic-guard convention.
func TestRunIdentityAddDryRunDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("runIdentityAdd() panicked: %v", r)
		}
	}()

	// Isolate HOME so no real ~/.ssh or ~/.gitconfig is touched.
	t.Setenv("HOME", t.TempDir())

	// Scripted answers: mode(1=create-new), name, git name, git email, provider,
	// alias(default), hostname(default), port(default), strategy(default=1),
	// match-gitdir(default), passphrase(empty).
	in := strings.NewReader(strings.Join([]string{
		"1", // create-new mode (D-10)
		"gitidtest",
		"Test User",
		"test@example.com",
		"github",
		"", // alias default
		"", // hostname default
		"", // port default
		"", // strategy default (1=gitdir)
		"", // match gitdir default
		"", // passphrase empty
	}, "\n") + "\n")

	var out bytes.Buffer
	if err := runIdentityAdd(in, &out, true, addFlags{}, fakeDeps); err != nil {
		t.Fatalf("runIdentityAdd(dry-run) returned error: %v\noutput:\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "--dry-run: no files were written.") {
		t.Errorf("expected dry-run notice in output, got:\n%s", out.String())
	}
	// The unified preview must show the allowed_signers artifact.
	if !strings.Contains(out.String(), "allowed_signers") {
		t.Errorf("dry-run preview must include the allowed_signers artifact, got:\n%s", out.String())
	}
}

// TestRunIdentityAddReuseDryRun drives the reuse-existing-key mode (D-10 mode 2)
// in --dry-run with scripted input, asserting it dispatches to the reuse path
// (prompting for the existing key) and completes without panic or write.
func TestRunIdentityAddReuseDryRun(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("runIdentityAdd(reuse) panicked: %v", r)
		}
	}()

	t.Setenv("HOME", t.TempDir())

	// mode(2=reuse), name, git name, git email, provider, alias, hostname, port,
	// strategy(default), match-gitdir(default), passphrase, existing-key-path.
	in := strings.NewReader(strings.Join([]string{
		"2", // reuse-existing-key mode
		"reused",
		"Reuse User",
		"reuse@example.com",
		"github",
		"", "", "", // alias/hostname/port defaults
		"", "", // strategy (default=1) + gitdir value defaults
		"",                            // passphrase
		"/tmp/.ssh/id_ed25519_reused", // existing key path
	}, "\n") + "\n")

	var out bytes.Buffer
	if err := runIdentityAdd(in, &out, true, addFlags{}, fakeDeps); err != nil {
		t.Fatalf("runIdentityAdd(reuse dry-run) error: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "--dry-run: no files were written.") {
		t.Errorf("expected dry-run notice, got:\n%s", out.String())
	}
}

// TestGatherCreateInputRejectsUnsafeName is a table-driven RED test asserting that
// gatherCreateInput rejects unsafe identity names before the name flows into
// filepath.Join or key paths (T-02-23 mitigation).
func TestGatherCreateInputRejectsUnsafeName(t *testing.T) {
	cases := []struct {
		name    string
		wantErr bool
	}{
		{name: "../evil", wantErr: true},
		{name: "a/b", wantErr: true},
		{name: "foo bar", wantErr: true},
		{name: "name;rm", wantErr: true},
		{name: "work", wantErr: false},
		{name: "personal.gh", wantErr: false},
		{name: "my-id_2", wantErr: false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			// Scripted answers: identity name, then valid defaults for the
			// remaining prompts (gitName, gitEmail, provider, alias, hostname,
			// port, strategy, match-gitdir, passphrase). addFlags{} → no flags.
			answers := strings.Join([]string{
				tc.name,         // Identity name
				"Test User",     // Git user.name
				"t@example.com", // Git user.email
				"github",        // Provider
				"",              // alias (default)
				"",              // hostname (default)
				"",              // port (default)
				"",              // strategy (default=1=gitdir)
				"",              // match gitdir (default)
				"",              // passphrase (empty)
			}, "\n") + "\n"
			r := strings.NewReader(answers)
			var out bytes.Buffer
			_, err := gatherCreateInput(bufio.NewReader(r), &out, "ed25519", addFlags{})
			if tc.wantErr && err == nil {
				t.Errorf("gatherCreateInput(%q): expected error, got nil", tc.name)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("gatherCreateInput(%q): unexpected error: %v", tc.name, err)
			}
			if tc.wantErr && err != nil && !strings.Contains(err.Error(), "invalid identity name") {
				t.Errorf("gatherCreateInput(%q): error %q does not mention \"invalid identity name\"", tc.name, err.Error())
			}
		})
	}
}

// TestGatherAddAccountRejectsUnsafeName is a table-driven RED test asserting that
// gatherAddAccount rejects unsafe existing identity names before the name flows
// into filepath.Join (T-02-23 mitigation).
func TestGatherAddAccountRejectsUnsafeName(t *testing.T) {
	cases := []struct {
		name    string
		wantErr bool
	}{
		{name: "../evil", wantErr: true},
		{name: "a/b", wantErr: true},
		{name: "foo bar", wantErr: true},
		{name: "name;rm", wantErr: true},
		{name: "work", wantErr: false},
		{name: "personal.gh", wantErr: false},
		{name: "my-id_2", wantErr: false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			// For accept cases: supply all required answers so the function
			// runs to completion. For reject cases only the first answer
			// (name) matters; the function returns early on name rejection.
			answers := strings.Join([]string{
				tc.name,                     // Existing identity name
				"Work User",                 // Git user.name
				"work@example.com",          // Git user.email
				"/tmp/.ssh/id_ed25519_work", // Existing private key path
				"gitlab",                    // New provider
				"",                          // New host alias (default)
				"",                          // Hostname (default)
				"",                          // Port (default)
				"",                          // Match gitdir (default)
			}, "\n") + "\n"
			r := strings.NewReader(answers)
			var out bytes.Buffer
			_, _, _, err := gatherAddAccount(bufio.NewReader(r), &out)
			if tc.wantErr && err == nil {
				t.Errorf("gatherAddAccount(%q): expected error, got nil", tc.name)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("gatherAddAccount(%q): unexpected error: %v", tc.name, err)
			}
			if tc.wantErr && err != nil && !strings.Contains(err.Error(), "invalid identity name") {
				t.Errorf("gatherAddAccount(%q): error %q does not mention \"invalid identity name\"", tc.name, err.Error())
			}
		})
	}
}

// TestRunIdentityAddAddAccountDryRun drives the add-account mode (D-10 mode 3) in
// --dry-run, asserting the rendered alias preview reuses the existing key path
// (IDENT-06) and no write occurs.
func TestRunIdentityAddAddAccountDryRun(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("runIdentityAdd(add-account) panicked: %v", r)
		}
	}()

	t.Setenv("HOME", t.TempDir())

	keyPath := "/tmp/.ssh/id_ed25519_work"
	// mode(3=add-account), existing name, git name, git email, existing key path,
	// new provider, new alias, hostname, port, match.
	in := strings.NewReader(strings.Join([]string{
		"3", // add-account mode
		"work",
		"Work User",
		"work@example.com",
		keyPath,
		"gitlab",
		"work.gitlab.com",
		"", "", "", // hostname/port/match defaults
	}, "\n") + "\n")

	var out bytes.Buffer
	if err := runIdentityAdd(in, &out, true, addFlags{}, fakeDeps); err != nil {
		t.Fatalf("runIdentityAdd(add-account dry-run) error: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), keyPath) {
		t.Errorf("add-account preview must reuse the existing key path %q, got:\n%s", keyPath, out.String())
	}
	if !strings.Contains(out.String(), "Host work.gitlab.com") {
		t.Errorf("add-account preview must declare the new alias, got:\n%s", out.String())
	}
}

// TestGatherCreateInput_URLStrategyWritesHasconfig asserts that choosing strategy
// "2" in gatherCreateInput (URL/hasconfig) produces a MatchHasconfig in the
// returned CreateInput.Matches (D-07, T-05.5-14).
func TestGatherCreateInput_URLStrategyWritesHasconfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	// Prompts: name, git.name, git.email, provider, alias, hostname, port,
	// strategy="2", url-pattern, passphrase.
	answers := strings.Join([]string{
		"myid",
		"My Name",
		"my@example.com",
		"github",
		"",                             // alias default
		"",                             // hostname default
		"",                             // port default
		"2",                            // strategy: URL (hasconfig)
		"git@ssh.github.com:myuser/**", // URL pattern
		"",                             // passphrase
	}, "\n") + "\n"

	r := strings.NewReader(answers)
	var out bytes.Buffer
	input, err := gatherCreateInput(bufio.NewReader(r), &out, "ed25519", addFlags{})
	if err != nil {
		t.Fatalf("gatherCreateInput(strategy=2) error: %v\noutput: %s", err, out.String())
	}
	if len(input.Matches) != 1 || input.Matches[0].Kind != gitconfig.MatchHasconfig {
		t.Errorf("strategy=2: want single MatchHasconfig, got %+v", input.Matches)
	}
	want := "remote.*.url:git@ssh.github.com:myuser/**"
	if input.Matches[0].Value != want {
		t.Errorf("strategy=2: want Value %q, got %q", want, input.Matches[0].Value)
	}
}

// TestGatherCreateInput_BothStrategyWritesTwoMatches asserts that strategy "3"
// (both) produces two Matches: MatchGitdir first, MatchHasconfig second (D-07).
func TestGatherCreateInput_BothStrategyWritesTwoMatches(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	answers := strings.Join([]string{
		"myid",
		"My Name",
		"my@example.com",
		"github",
		"",                           // alias default
		"",                           // hostname default
		"",                           // port default
		"3",                          // strategy: both
		"~/git/myid/",                // gitdir value
		"git@ssh.github.com:myid/**", // URL value
		"",                           // passphrase
	}, "\n") + "\n"

	r := strings.NewReader(answers)
	var out bytes.Buffer
	input, err := gatherCreateInput(bufio.NewReader(r), &out, "ed25519", addFlags{})
	if err != nil {
		t.Fatalf("gatherCreateInput(strategy=3) error: %v\noutput: %s", err, out.String())
	}
	if len(input.Matches) != 2 {
		t.Fatalf("strategy=3: want 2 matches, got %d: %+v", len(input.Matches), input.Matches)
	}
	if input.Matches[0].Kind != gitconfig.MatchGitdir {
		t.Errorf("strategy=3: match[0] want MatchGitdir, got %v", input.Matches[0].Kind)
	}
	if input.Matches[1].Kind != gitconfig.MatchHasconfig {
		t.Errorf("strategy=3: match[1] want MatchHasconfig, got %v", input.Matches[1].Kind)
	}
}

// TestGatherCreateInput_NameFlag asserts that --name skips the name prompt (D-09).
func TestGatherCreateInput_NameFlag(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	// With --name flag, name prompt is skipped.
	// Remaining prompts: git.name, git.email, provider, alias, hostname, port,
	// strategy (default=1), gitdir, passphrase = 9 prompts.
	answers := strings.Repeat("\n", 9)
	r := strings.NewReader(answers)
	var out bytes.Buffer
	input, err := gatherCreateInput(bufio.NewReader(r), &out, "ed25519", addFlags{name: "flagname"})
	if err != nil {
		t.Fatalf("gatherCreateInput(--name) error: %v\noutput: %s", err, out.String())
	}
	if input.Name != "flagname" {
		t.Errorf("--name flag: want Name %q, got %q", "flagname", input.Name)
	}
	// Verify name prompt label is NOT in output when flag is used.
	if strings.Contains(out.String(), "Identity name") {
		t.Errorf("--name flag: 'Identity name' prompt must be skipped; got output:\n%s", out.String())
	}
}

// TestGatherCreateInput_ProviderFlag asserts that --provider skips the provider
// prompt and uses the flag value (D-09).
func TestGatherCreateInput_ProviderFlag(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	// With --provider flag, provider prompt is skipped.
	// Prompts: name, git.name, git.email, alias, hostname, port,
	// strategy (default), gitdir, passphrase = 9 prompts.
	answers := strings.Join([]string{
		"testprov",  // name
		"Test User", // git.name
		"t@t.com",   // git.email
		// provider SKIPPED by flag
		"", // alias default
		"", // hostname default
		"", // port default
		"", // strategy default
		"", // gitdir default
		"", // passphrase
	}, "\n") + "\n"

	r := strings.NewReader(answers)
	var out bytes.Buffer
	input, err := gatherCreateInput(bufio.NewReader(r), &out, "ed25519", addFlags{provider: "gitlab"})
	if err != nil {
		t.Fatalf("gatherCreateInput(--provider) error: %v\noutput: %s", err, out.String())
	}
	if input.Provider != "gitlab" {
		t.Errorf("--provider flag: want Provider %q, got %q", "gitlab", input.Provider)
	}
}

// TestGatherCreateInput_GitdirFlag asserts that --gitdir skips the picker and
// uses the flag value directly, building a MatchGitdir (D-09).
func TestGatherCreateInput_GitdirFlag(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	// With --gitdir flag, picker and gitdir value prompt are skipped.
	// Prompts: name, git.name, git.email, provider, alias, hostname, port, passphrase = 8.
	answers := strings.Join([]string{
		"gitdirid",
		"Test User",
		"t@t.com",
		"github",
		"", "", "", // alias/hostname/port defaults
		"", // passphrase
	}, "\n") + "\n"

	r := strings.NewReader(answers)
	var out bytes.Buffer
	input, err := gatherCreateInput(bufio.NewReader(r), &out, "ed25519", addFlags{gitdir: "~/work/specific/"})
	if err != nil {
		t.Fatalf("gatherCreateInput(--gitdir) error: %v\noutput: %s", err, out.String())
	}
	if len(input.Matches) != 1 || input.Matches[0].Kind != gitconfig.MatchGitdir {
		t.Errorf("--gitdir: want single MatchGitdir, got %+v", input.Matches)
	}
	if input.Matches[0].Value != "~/work/specific/" {
		t.Errorf("--gitdir: want Value %q, got %q", "~/work/specific/", input.Matches[0].Value)
	}
}

// TestGatherCreateInput_URLFlag asserts that --url skips the picker and
// uses the flag value, building a MatchHasconfig (D-09).
func TestGatherCreateInput_URLFlag(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	// With --url flag, picker and URL value prompt are skipped.
	// Prompts: name, git.name, git.email, provider, alias, hostname, port, passphrase = 8.
	answers := strings.Join([]string{
		"urlid",
		"Test User",
		"t@t.com",
		"github",
		"", "", "", // alias/hostname/port defaults
		"", // passphrase
	}, "\n") + "\n"

	r := strings.NewReader(answers)
	var out bytes.Buffer
	input, err := gatherCreateInput(bufio.NewReader(r), &out, "ed25519", addFlags{url: "git@ssh.github.com:myorg/**"})
	if err != nil {
		t.Fatalf("gatherCreateInput(--url) error: %v\noutput: %s", err, out.String())
	}
	if len(input.Matches) != 1 || input.Matches[0].Kind != gitconfig.MatchHasconfig {
		t.Errorf("--url: want single MatchHasconfig, got %+v", input.Matches)
	}
	want := "remote.*.url:git@ssh.github.com:myorg/**"
	if input.Matches[0].Value != want {
		t.Errorf("--url: want Value %q, got %q", want, input.Matches[0].Value)
	}
}

// TestGatherCreateInput_HostAliasLabel asserts the host-alias prompt is labeled
// exactly "Host alias" (D-10/F-5).
func TestGatherCreateInput_HostAliasLabel(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	// Drive with all defaults; capture output.
	answers := strings.Join([]string{
		"hostaliasid",
		"Test User",
		"t@t.com",
		"github",
		"", "", "", // alias/hostname/port defaults
		"", "", // strategy + gitdir defaults
		"", // passphrase
	}, "\n") + "\n"

	r := strings.NewReader(answers)
	var out bytes.Buffer
	_, err := gatherCreateInput(bufio.NewReader(r), &out, "ed25519", addFlags{})
	if err != nil {
		t.Fatalf("gatherCreateInput error: %v", err)
	}
	if !strings.Contains(out.String(), "Host alias") {
		t.Errorf("gatherCreateInput: prompt must read 'Host alias' (D-10); output:\n%s", out.String())
	}
}

// TestWarnOverlapAndConfirm_DeclineAbortsWrite asserts that declining the overlap
// prompt prevents any write (D-16). The helper is tested directly with a prospective
// account that overlaps an existing one; stdin "n" must cause it to return false.
func TestWarnOverlapAndConfirm_DeclineAbortsWrite(t *testing.T) {
	existing := []identity.Account{
		{
			Name:    "existing-id",
			Matches: []gitconfig.Match{{Kind: gitconfig.MatchGitdir, Value: "~/git/"}},
		},
	}
	prospective := identity.Account{
		Name:    "new-id",
		Matches: []gitconfig.Match{{Kind: gitconfig.MatchGitdir, Value: "~/git/personal/"}},
	}

	// Overlap detected (parent/child gitdir); user declines with "n".
	stdin := strings.NewReader("n\n")
	var out bytes.Buffer
	proceed := warnOverlapAndConfirm(bufio.NewReader(stdin), &out, prospective, existing)
	if proceed {
		t.Error("warnOverlapAndConfirm: declining 'n' must return false (abort write)")
	}
	outStr := out.String()
	if !strings.Contains(strings.ToLower(outStr), "overlap") {
		t.Errorf("warnOverlapAndConfirm: must mention overlap in output; got:\n%s", outStr)
	}
}

// TestWarnOverlapAndConfirm_AcceptContinues asserts that confirming "y" returns
// true, allowing the write to proceed (D-16 deliberate overlaps stay possible).
func TestWarnOverlapAndConfirm_AcceptContinues(t *testing.T) {
	existing := []identity.Account{
		{
			Name:    "root-id",
			Matches: []gitconfig.Match{{Kind: gitconfig.MatchGitdir, Value: "~/git/"}},
		},
	}
	prospective := identity.Account{
		Name:    "child-id",
		Matches: []gitconfig.Match{{Kind: gitconfig.MatchGitdir, Value: "~/git/work/"}},
	}

	stdin := strings.NewReader("y\n")
	var out bytes.Buffer
	proceed := warnOverlapAndConfirm(bufio.NewReader(stdin), &out, prospective, existing)
	if !proceed {
		t.Error("warnOverlapAndConfirm: accepting 'y' must return true (proceed)")
	}
}

// TestWarnOverlapAndConfirm_NoOverlapNoPrompt asserts that non-overlapping accounts
// return true immediately without showing any warning or prompt.
func TestWarnOverlapAndConfirm_NoOverlapNoPrompt(t *testing.T) {
	existing := []identity.Account{
		{
			Name:    "awork",
			Matches: []gitconfig.Match{{Kind: gitconfig.MatchGitdir, Value: "~/git/a/"}},
		},
	}
	prospective := identity.Account{
		Name:    "bwork",
		Matches: []gitconfig.Match{{Kind: gitconfig.MatchGitdir, Value: "~/git/b/"}},
	}

	// No stdin expected (no prompt should appear).
	stdin := strings.NewReader("")
	var out bytes.Buffer
	proceed := warnOverlapAndConfirm(bufio.NewReader(stdin), &out, prospective, existing)
	if !proceed {
		t.Error("warnOverlapAndConfirm: no overlap must return true without prompt")
	}
	if strings.Contains(strings.ToLower(out.String()), "overlap") {
		t.Errorf("warnOverlapAndConfirm: must NOT show overlap warning for non-overlapping accounts; got:\n%s", out.String())
	}
}

// TestWarnOverlapAndConfirm_SameNameReCreateNoSelfOverlap guards the same-name
// re-create case: an on-disk account sharing the prospective's name (and gitdir)
// must NOT be reported as an overlap with itself. Re-running `add --name work ...`
// over an existing "work" identity is a rewrite, not an ambiguity.
func TestWarnOverlapAndConfirm_SameNameReCreateNoSelfOverlap(t *testing.T) {
	existing := []identity.Account{
		{
			Name:    "work",
			Matches: []gitconfig.Match{{Kind: gitconfig.MatchGitdir, Value: "~/git/work/"}},
		},
	}
	prospective := identity.Account{
		Name:    "work",
		Matches: []gitconfig.Match{{Kind: gitconfig.MatchGitdir, Value: "~/git/work/"}},
	}

	// No stdin expected: the self-match must be excluded, so no prompt appears.
	stdin := strings.NewReader("")
	var out bytes.Buffer
	proceed := warnOverlapAndConfirm(bufio.NewReader(stdin), &out, prospective, existing)
	if !proceed {
		t.Error("warnOverlapAndConfirm: same-name re-create must return true without prompt")
	}
	if strings.Contains(strings.ToLower(out.String()), "overlap") {
		t.Errorf("warnOverlapAndConfirm: must NOT report a self-overlap on same-name re-create; got:\n%s", out.String())
	}
}
