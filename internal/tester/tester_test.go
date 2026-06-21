package tester

import (
	"strings"
	"testing"
)

func TestClassifyPreWrite_Pass(t *testing.T) {
	out := "Hi user! You've successfully authenticated, but GitHub does not provide shell access."
	if got := ClassifyPreWrite(out); got != PASS {
		t.Errorf("ClassifyPreWrite(success) = %v, want PASS", got)
	}
}

func TestClassifyPreWrite_ReachableNotUploaded(t *testing.T) {
	out := "git@github.com: Permission denied (publickey)."
	if got := ClassifyPreWrite(out); got != ReachableNotUploaded {
		t.Errorf("ClassifyPreWrite(permission denied) = %v, want ReachableNotUploaded", got)
	}
}

func TestClassifyPreWrite_FailureConnectionRefused(t *testing.T) {
	out := "ssh: connect to host github.com port 22: Connection refused"
	if got := ClassifyPreWrite(out); got != Failure {
		t.Errorf("ClassifyPreWrite(connection refused) = %v, want Failure", got)
	}
}

func TestClassifyPreWrite_FailureDNSAndTimeout(t *testing.T) {
	cases := []string{
		"ssh: Could not resolve hostname nope.invalid: nodename nor servname provided",
		"ssh: connect to host github.com port 22: Operation timed out",
	}
	for _, out := range cases {
		if got := ClassifyPreWrite(out); got != Failure {
			t.Errorf("ClassifyPreWrite(%q) = %v, want Failure", out, got)
		}
	}
}

func TestClassifyPreWrite_IgnoresExitCode(t *testing.T) {
	// D-01: classification is by output substring only. Even output that would
	// accompany exit code 0 (ssh -T exits 0 on denial) must classify by content.
	out := "git@github.com: Permission denied (publickey)."
	if got := ClassifyPreWrite(out); got != ReachableNotUploaded {
		t.Errorf("classification must ignore exit code: got %v, want ReachableNotUploaded", got)
	}
}

func TestPreWrite_CapturesCommandAndOutput(t *testing.T) {
	// TEST-03: Result carries the exact input command and the raw output.
	fakeOut := "git@github.com: Permission denied (publickey)."
	runner := func(_ []string) (string, error) {
		return fakeOut, nil
	}
	res := preWriteWith(runner, "/home/u/.ssh/id_ed25519_work", "github.com", 22)

	if res.Command == "" {
		t.Errorf("Result.Command is empty; expected the ssh invocation string")
	}
	if !strings.Contains(res.Command, "ssh") ||
		!strings.Contains(res.Command, "-i") ||
		!strings.Contains(res.Command, "IdentitiesOnly=yes") ||
		!strings.Contains(res.Command, "BatchMode=yes") ||
		!strings.Contains(res.Command, "ConnectTimeout=10") ||
		!strings.Contains(res.Command, "git@github.com") {
		t.Errorf("Result.Command missing expected ssh args: %q", res.Command)
	}
	if res.Output != fakeOut {
		t.Errorf("Result.Output = %q, want %q", res.Output, fakeOut)
	}
	if res.Outcome != ReachableNotUploaded {
		t.Errorf("Result.Outcome = %v, want ReachableNotUploaded", res.Outcome)
	}
}

func TestPreWriteArgs_ContainsRequiredFlags(t *testing.T) {
	// Table-driven: verify preWriteArgs produces the correct arg slice with port
	// and StrictHostKeyChecking=accept-new, while NOT containing any alias-shaped
	// value (a value with dots but no real TLD like "ramon.github").
	tests := []struct {
		name     string
		keyPath  string
		hostname string
		port     int
		// wantContains is a list of strings that must appear somewhere in the joined args.
		wantContains []string
		// wantAbsent is a list of strings that must NOT appear in any element.
		wantAbsent []string
	}{
		{
			name:     "github port 443",
			keyPath:  "/tmp/.ssh/id_ed25519_work",
			hostname: "ssh.github.com",
			port:     443,
			wantContains: []string{
				"-i", "/tmp/.ssh/id_ed25519_work",
				"IdentitiesOnly=yes",
				"BatchMode=yes",
				"ConnectTimeout=10",
				"StrictHostKeyChecking=accept-new",
				"-p", "443",
				"git@ssh.github.com",
			},
			wantAbsent: []string{"ramon.github"},
		},
		{
			name:     "gitlab port 443",
			keyPath:  "/tmp/.ssh/id_ed25519_gitlab",
			hostname: "altssh.gitlab.com",
			port:     443,
			wantContains: []string{
				"IdentitiesOnly=yes",
				"StrictHostKeyChecking=accept-new",
				"-p", "443",
				"git@altssh.gitlab.com",
			},
			wantAbsent: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := preWriteArgs(tt.keyPath, tt.hostname, tt.port)
			joined := strings.Join(args, " ")
			for _, want := range tt.wantContains {
				if !strings.Contains(joined, want) {
					t.Errorf("preWriteArgs args missing %q\nfull args: %v", want, args)
				}
			}
			for _, absent := range tt.wantAbsent {
				for _, arg := range args {
					if arg == absent {
						t.Errorf("preWriteArgs args must not contain alias value %q\nfull args: %v", absent, args)
					}
				}
			}
		})
	}
}

func TestPreWriteWith_ClassifiesAndCapturesPortAndAcceptNew(t *testing.T) {
	// Verify that preWriteWith with a fake runner returning "Permission denied"
	// produces ReachableNotUploaded and that the Result.Command contains -p, the
	// port value, StrictHostKeyChecking=accept-new, and the correct target host.
	fakeOut := "git@ssh.github.com: Permission denied (publickey)."
	fakeRunner := func(_ []string) (string, error) {
		return fakeOut, nil
	}
	res := preWriteWith(fakeRunner, "/tmp/.ssh/id_ed25519_work", "ssh.github.com", 443)

	if res.Outcome != ReachableNotUploaded {
		t.Errorf("Outcome = %v, want ReachableNotUploaded", res.Outcome)
	}
	for _, want := range []string{"-p", "443", "StrictHostKeyChecking=accept-new", "git@ssh.github.com"} {
		if !strings.Contains(res.Command, want) {
			t.Errorf("Result.Command missing %q\ncommand: %s", want, res.Command)
		}
	}
}

const sshGFixture = `user git
hostname github.com
port 22
identitiesonly yes
identityfile ~/.ssh/id_ed25519_work
identityfile ~/.ssh/id_ed25519_other
addkeystoagent yes
`

func TestParseResolved_LowercaseKeys(t *testing.T) {
	rc := ParseResolved(sshGFixture)

	if rc.User != "git" {
		t.Errorf("User = %q, want %q", rc.User, "git")
	}
	if rc.Hostname != "github.com" {
		t.Errorf("Hostname = %q, want %q", rc.Hostname, "github.com")
	}
	if rc.Port != "22" {
		t.Errorf("Port = %q, want %q", rc.Port, "22")
	}
	if rc.IdentitiesOnly != "yes" {
		t.Errorf("IdentitiesOnly = %q, want %q", rc.IdentitiesOnly, "yes")
	}
	if len(rc.IdentityFiles) != 2 {
		t.Fatalf("IdentityFiles len = %d, want 2: %v", len(rc.IdentityFiles), rc.IdentityFiles)
	}
	found := false
	for _, f := range rc.IdentityFiles {
		if f == "~/.ssh/id_ed25519_work" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected ~/.ssh/id_ed25519_work among identityfiles, got %v", rc.IdentityFiles)
	}
}

func TestParseResolved_IgnoresCamelCase(t *testing.T) {
	// Pitfall 3: ssh -G emits lowercase keys. A camelCase line must NOT match.
	rc := ParseResolved("IdentityFile ~/.ssh/should_not_match\nuser git\n")
	if len(rc.IdentityFiles) != 0 {
		t.Errorf("camelCase IdentityFile should not parse; got %v", rc.IdentityFiles)
	}
	if rc.User != "git" {
		t.Errorf("User = %q, want %q", rc.User, "git")
	}
}

// TestPreWriteCommand_MatchesPreWriteArgShape verifies that PreWriteCommand returns
// a string that contains the same key flags as preWriteArgs and is byte-identical in
// arg shape to what PreWrite would run. The helper is read-only (no exec).
func TestPreWriteCommand_MatchesPreWriteArgShape(t *testing.T) {
	got := PreWriteCommand("/tmp/id_ed25519_personal", "ssh.github.com", 443)

	// Must contain "ssh" as the program name (exec.Command("ssh", ...).String() may
	// expand to the full path, e.g. /usr/bin/ssh on macOS).
	if !strings.Contains(got, "ssh") {
		t.Errorf("PreWriteCommand must reference ssh command; got %q", got)
	}
	// Must contain the same required flags as preWriteArgs.
	for _, want := range []string{
		"-i", "/tmp/id_ed25519_personal",
		"IdentitiesOnly=yes",
		"BatchMode=yes",
		"ConnectTimeout=10",
		"StrictHostKeyChecking=accept-new",
		"-p", "443",
		"git@ssh.github.com",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("PreWriteCommand missing %q; full string: %q", want, got)
		}
	}
}

// TestPreWriteCommand_MatchesPreWriteResultCommand verifies that PreWriteCommand
// produces the same string as the Command field on a Result returned by preWriteWith
// for the same arguments — ensuring pre-run display is byte-identical to the run.
func TestPreWriteCommand_MatchesPreWriteResultCommand(t *testing.T) {
	keyPath := "/home/u/.ssh/staging/id_ed25519_work"
	hostname := "ssh.github.com"
	port := 443

	// Produce a result via preWriteWith using a fake no-op runner.
	fakeRunner := func(_ []string) (string, error) {
		return "hi", nil
	}
	res := preWriteWith(fakeRunner, keyPath, hostname, port)

	got := PreWriteCommand(keyPath, hostname, port)
	if got != res.Command {
		t.Errorf("PreWriteCommand(%q, %q, %d) = %q\n  preWriteWith.Command = %q\n  strings must be equal",
			keyPath, hostname, port, got, res.Command)
	}
}
