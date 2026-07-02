package checks

import (
	"os"
	"testing"

	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
)

// ---------------------------------------------------------------------------
// Pure helper unit tests
// ---------------------------------------------------------------------------

func TestClassifyAgentState(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		exitCode int
		want     agentState
	}{
		{
			name:     "exit 0 with key lines means running-with-keys",
			output:   "256 SHA256:abc comment (ED25519)",
			exitCode: 0,
			want:     agentRunningWithKeys,
		},
		{
			name:     "exit 1 with no-identities text means running-empty",
			output:   "The agent has no identities.",
			exitCode: 1,
			want:     agentRunningEmpty,
		},
		{
			name:     "exit 1 without text still means running-empty",
			output:   "",
			exitCode: 1,
			want:     agentRunningEmpty,
		},
		{
			name:     "exit 2 means unreachable",
			output:   "Could not open a connection to your authentication agent.",
			exitCode: 2,
			want:     agentUnreachable,
		},
		{
			name:     "exit 3 (unknown) treated as unreachable",
			output:   "",
			exitCode: 3,
			want:     agentUnreachable,
		},
		{
			name:     "exit 0 empty output still running-with-keys (exit code wins)",
			output:   "",
			exitCode: 0,
			want:     agentRunningWithKeys,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyAgentState(tt.output, tt.exitCode)
			if got != tt.want {
				t.Errorf("classifyAgentState(%q, %d) = %d; want %d", tt.output, tt.exitCode, got, tt.want)
			}
		})
	}
}

func TestExtractFingerprint(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{
			name: "standard ssh-keygen output",
			line: "256 SHA256:vRBdzHYKWKt131j4W3gBbBwqid2tALp3weJk9eZz1hE castocolina@gmail.com (ED25519)",
			want: "SHA256:vRBdzHYKWKt131j4W3gBbBwqid2tALp3weJk9eZz1hE",
		},
		{
			name: "line without SHA256 returns empty",
			line: "256 MD5:ab:cd:ef:12 comment (RSA)",
			want: "",
		},
		{
			name: "empty line returns empty",
			line: "",
			want: "",
		},
		{
			name: "SHA256 token with short hash",
			line: "256 SHA256:abc comment (ED25519)",
			want: "SHA256:abc",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractFingerprint(tt.line)
			if got != tt.want {
				t.Errorf("extractFingerprint(%q) = %q; want %q", tt.line, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// CheckAgent tests via fake deps
// ---------------------------------------------------------------------------

// fakeStatSucceeds returns an injected Stat function that always reports the
// file as existing (used so the pub-path guard passes in agent tests).
func fakeStatSucceeds() func(string) (os.FileInfo, error) {
	return func(_ string) (os.FileInfo, error) {
		// Return a non-nil FileInfo with enough to look like a readable file.
		// We use os.Stat on a real path (/dev/null) as a portable stand-in.
		return os.Stat("/dev/null") //nolint:gosec // test helper – /dev/null is known path
	}
}

// fakeStatFails returns an injected Stat function that always reports the file
// as missing (used to test the PubPath-missing guard, Pitfall 7).
func fakeStatFails() func(string) (os.FileInfo, error) {
	return func(_ string) (os.FileInfo, error) {
		return nil, os.ErrNotExist
	}
}

func TestAgentUnreachable(t *testing.T) {
	deps := doctor.Deps{
		RunSSHAdd: func() (string, int) { return "", 2 },
		// RunSSHKeygenFingerprint must NOT be called when unreachable.
		RunSSHKeygenFingerprint: func(_ string) (string, error) {
			t.Error("RunSSHKeygenFingerprint should not be called when agent is unreachable")
			return "", nil
		},
		GitVersionAtLeast: func(_, _ int) bool { return true },
		Identities: []identity.Account{
			{Name: "work", KeyPath: "/home/u/.ssh/gitid_work", PubPath: "/home/u/.ssh/gitid_work.pub"},
		},
		Stat: fakeStatSucceeds(),
	}

	findings := CheckAgent(deps)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for unreachable agent; got %d: %v", len(findings), findings)
	}
	f := findings[0]
	if f.Family != doctor.FamilyAgent {
		t.Errorf("finding family = %q; want %q", f.Family, doctor.FamilyAgent)
	}
	if f.Severity != doctor.SeverityWarning {
		t.Errorf("finding severity = %v; want warning", f.Severity)
	}
	if f.Fix != nil {
		t.Errorf("agent unreachable finding must be report-only (Fix must be nil); got %+v", f.Fix)
	}
	// Exact copy from UI-SPEC DOC-05
	wantTitle := "ssh-agent: not reachable"
	if f.Title != wantTitle {
		t.Errorf("finding title = %q; want %q", f.Title, wantTitle)
	}
}

func TestAgentKeyNotLoaded(t *testing.T) {
	// Agent is running with some keys, but not the one for "work" identity.
	agentOutput := "256 SHA256:OTHER comment (ED25519)" // not the work key

	deps := doctor.Deps{
		RunSSHAdd: func() (string, int) { return agentOutput, 0 },
		RunSSHKeygenFingerprint: func(_ string) (string, error) {
			return "256 SHA256:WORKKEY work@example.com (ED25519)", nil
		},
		GitVersionAtLeast: func(_, _ int) bool { return true },
		Identities: []identity.Account{
			{Name: "work", KeyPath: "/home/u/.ssh/gitid_work", PubPath: "/home/u/.ssh/gitid_work.pub"},
		},
		Stat: fakeStatSucceeds(),
	}

	findings := CheckAgent(deps)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for key-not-loaded; got %d: %v", len(findings), findings)
	}
	f := findings[0]
	if f.Family != doctor.FamilyAgent {
		t.Errorf("finding family = %q; want %q", f.Family, doctor.FamilyAgent)
	}
	if f.Severity != doctor.SeverityWarning {
		t.Errorf("finding severity = %v; want warning", f.Severity)
	}
	if f.Fix != nil {
		t.Errorf("key-not-loaded finding must be report-only (Fix must be nil, D-03); got %+v", f.Fix)
	}
	// Exact copy from UI-SPEC DOC-05
	wantTitle := `identity "work": key not loaded in agent`
	if f.Title != wantTitle {
		t.Errorf("finding title = %q; want %q", f.Title, wantTitle)
	}
}

func TestAgentKeyLoaded(t *testing.T) {
	const fp = "SHA256:LOADEDKEY"
	// Agent output already contains the fingerprint for "personal".
	agentOutput := "256 " + fp + " personal@example.com (ED25519)"

	deps := doctor.Deps{
		RunSSHAdd: func() (string, int) { return agentOutput, 0 },
		RunSSHKeygenFingerprint: func(_ string) (string, error) {
			return "256 " + fp + " personal@example.com (ED25519)", nil
		},
		GitVersionAtLeast: func(_, _ int) bool { return true },
		Identities: []identity.Account{
			{Name: "personal", KeyPath: "/home/u/.ssh/gitid_personal", PubPath: "/home/u/.ssh/gitid_personal.pub"},
		},
		Stat: fakeStatSucceeds(),
	}

	findings := CheckAgent(deps)

	if len(findings) != 0 {
		t.Errorf("expected 0 findings when key is loaded; got %d: %v", len(findings), findings)
	}
}

func TestAgentMissingPubSkipped(t *testing.T) {
	// PubPath Stat fails → agent check must be skipped for that identity (Pitfall 7).
	agentOutput := "256 SHA256:OTHER other@example.com (ED25519)"

	deps := doctor.Deps{
		RunSSHAdd: func() (string, int) { return agentOutput, 0 },
		RunSSHKeygenFingerprint: func(_ string) (string, error) {
			t.Error("RunSSHKeygenFingerprint should not be called for identity with missing pub")
			return "", nil
		},
		GitVersionAtLeast: func(_, _ int) bool { return true },
		Identities: []identity.Account{
			{Name: "broken", KeyPath: "/home/u/.ssh/gitid_broken", PubPath: "/home/u/.ssh/gitid_broken.pub"},
		},
		// Stat always fails — pub file is absent.
		Stat: fakeStatFails(),
	}

	findings := CheckAgent(deps)

	if len(findings) != 0 {
		t.Errorf("expected 0 findings when pub is missing (skip per Pitfall 7); got %d: %v", len(findings), findings)
	}
}

// ---------------------------------------------------------------------------
// CheckSigning / git version gate tests
// ---------------------------------------------------------------------------

func TestGitVersionGate_HasconfigOldGit(t *testing.T) {
	// An identity with a hasconfig: match + git < 2.36 → warning finding.
	deps := doctor.Deps{
		GitVersionAtLeast: func(_, _ int) bool {
			// Simulate git 2.35 — less than 2.36.
			return false
		},
		RunSSHAdd: func() (string, int) { return "", 0 },
		Stat:      fakeStatSucceeds(),
		Identities: []identity.Account{
			{
				Name:    "work",
				KeyPath: "/home/u/.ssh/gitid_work",
				PubPath: "/home/u/.ssh/gitid_work.pub",
				Matches: []gitconfig.Match{
					{Kind: gitconfig.MatchHasconfig, Value: "remote.*.url:https://github.com/work/*"},
				},
			},
		},
		RunSSHKeygenFingerprint: func(_ string) (string, error) { return "", nil },
	}

	findings := CheckSigning(deps)

	found := false
	for _, f := range findings {
		if f.Family == doctor.FamilySigning && f.Severity == doctor.SeverityWarning {
			found = true
			if f.Fix != nil {
				t.Errorf("git-version warning must be report-only (Fix must be nil); got %+v", f.Fix)
			}
		}
	}
	if !found {
		t.Errorf("expected a Signing warning for git<2.36 + hasconfig:; got findings: %v", findings)
	}
}

func TestGitVersionGate_HasconfigNewGit(t *testing.T) {
	// An identity with a hasconfig: match + git >= 2.36 → no finding.
	deps := doctor.Deps{
		GitVersionAtLeast: func(_, _ int) bool { return true },
		RunSSHAdd:         func() (string, int) { return "", 0 },
		Stat:              fakeStatSucceeds(),
		Identities: []identity.Account{
			{
				Name:    "work",
				KeyPath: "/home/u/.ssh/gitid_work",
				PubPath: "/home/u/.ssh/gitid_work.pub",
				Matches: []gitconfig.Match{
					{Kind: gitconfig.MatchHasconfig, Value: "remote.*.url:https://github.com/work/*"},
				},
			},
		},
		RunSSHKeygenFingerprint: func(_ string) (string, error) { return "", nil },
	}

	findings := CheckSigning(deps)

	for _, f := range findings {
		if f.Family == doctor.FamilySigning && f.Severity == doctor.SeverityWarning {
			t.Errorf("expected no Signing warning for git>=2.36 with hasconfig:; got: %+v", f)
		}
	}
}

func TestGitVersionGate_OnlyGitdirNoWarning(t *testing.T) {
	// An identity with only gitdir: match — no hasconfig: — no warning even on old git.
	deps := doctor.Deps{
		GitVersionAtLeast: func(_, _ int) bool { return false },
		RunSSHAdd:         func() (string, int) { return "", 0 },
		Stat:              fakeStatSucceeds(),
		Identities: []identity.Account{
			{
				Name:    "personal",
				KeyPath: "/home/u/.ssh/gitid_personal",
				PubPath: "/home/u/.ssh/gitid_personal.pub",
				Matches: []gitconfig.Match{
					{Kind: gitconfig.MatchGitdir, Value: "~/git/personal/"},
				},
			},
		},
		RunSSHKeygenFingerprint: func(_ string) (string, error) { return "", nil },
	}

	findings := CheckSigning(deps)

	for _, f := range findings {
		if f.Family == doctor.FamilySigning && f.Severity == doctor.SeverityWarning {
			t.Errorf("expected no Signing warning for gitdir-only match on old git; got: %+v", f)
		}
	}
}

// ---------------------------------------------------------------------------
// isKeyLoaded helper tests
// ---------------------------------------------------------------------------

func TestIsKeyLoaded_Present(t *testing.T) {
	const fp = "SHA256:TESTFP"
	agentOut := "256 " + fp + " user@example.com (ED25519)"

	loaded := isKeyLoaded(agentOut, "/home/u/.ssh/id_ed25519.pub", func(_ string) (string, error) {
		return "256 " + fp + " user@example.com (ED25519)", nil
	})
	if !loaded {
		t.Error("expected isKeyLoaded=true when fingerprint is in agent output")
	}
}

func TestIsKeyLoaded_Absent(t *testing.T) {
	agentOut := "256 SHA256:OTHER other@example.com (ED25519)"

	loaded := isKeyLoaded(agentOut, "/home/u/.ssh/id_ed25519.pub", func(_ string) (string, error) {
		return "256 SHA256:MINE user@example.com (ED25519)", nil
	})
	if loaded {
		t.Error("expected isKeyLoaded=false when fingerprint is NOT in agent output")
	}
}

func TestIsKeyLoaded_FingerprintError(t *testing.T) {
	// When ssh-keygen fails, treat as not-loaded.
	loaded := isKeyLoaded("anything", "/missing.pub", func(_ string) (string, error) {
		return "", os.ErrNotExist
	})
	if loaded {
		t.Error("expected isKeyLoaded=false when RunSSHKeygenFingerprint errors")
	}
}
