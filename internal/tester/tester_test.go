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
	res := preWriteWith(runner, "/home/u/.ssh/id_ed25519_work", "github.com")

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
