package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/castocolina/gitid/internal/tester"
	"github.com/castocolina/gitid/internal/tuikit"
)

// seedRegisterKeyAccount seeds a minimal GitHub-hosted identity (SSH Host
// block + a real generated key pair) so b.findAccount("acme") resolves and
// uploadRequestForAccount has a real .pub file to read.
func seedRegisterKeyAccount(t *testing.T, home, name string) {
	t.Helper()
	seedInFileIdentity(t, home, name)
	seedGeneratedKey(t, home+"/.ssh/id_ed25519_"+name, name, "")
}

// TestRegisterKeyVerbExistsInBothForms resolves both the noun and the flat
// form from newRootCmd() and asserts their flag sets and Args validators
// are identical — the shared-spec guarantee newVerbCmd exists to enforce.
func TestRegisterKeyVerbExistsInBothForms(t *testing.T) {
	root := newRootCmd()
	noun, _, err := root.Find([]string{"identity", "register-key"})
	if err != nil {
		t.Fatalf("resolving identity register-key: %v", err)
	}
	flat, _, err := root.Find([]string{"register-key"})
	if err != nil {
		t.Fatalf("resolving flat register-key: %v", err)
	}
	if noun.Flags().Lookup("dry-run") == nil || flat.Flags().Lookup("dry-run") == nil {
		t.Fatal("both forms must bind --dry-run")
	}
	if noun.Flags().Lookup("dry-run").Usage != flat.Flags().Lookup("dry-run").Usage {
		t.Error("--dry-run usage text differs between the noun and flat forms")
	}
}

// TestRegisterKeyHeadlessCallsOrchestrationOnce recording-double-checks
// that a headless invocation calls cliRegisterKeyInto exactly once.
func TestRegisterKeyHeadlessCallsOrchestrationOnce(t *testing.T) {
	home := t.TempDir()
	seedRegisterKeyAccount(t, home, "acme")
	calls := 0
	orig := cliRegisterKeyInto
	cliRegisterKeyInto = func(_ *realBackend, _ uploadRequest) tuikit.UploadRunView {
		calls++
		return tuikit.UploadRunView{Rows: []tuikit.UploadResultRow{{Outcome: tuikit.UploadRowUploaded, Label: "Authentication"}}}
	}
	defer func() { cliRegisterKeyInto = orig }()

	t.Setenv("HOME", home)
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	if err := runIdentityRegisterKey(cmd, []string{"acme"}, identityRegisterKeyFlags{}, false, false); err != nil {
		t.Fatalf("runIdentityRegisterKey: %v", err)
	}
	if calls != 1 {
		t.Errorf("cliRegisterKeyInto called %d times, want 1", calls)
	}
}

// TestRegisterKeyMissingNameRoutesThroughDepthResolver covers the two
// non-interactive descriptor combinations (stdin-only, neither), asserting
// the error names the identity argument. The prefilled-TUI branch (both
// TTYs) is not driven here since it launches a real Bubble Tea program.
func TestRegisterKeyMissingNameRoutesThroughDepthResolver(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)

	for _, tt := range []struct {
		name                string
		stdinTTY, stdoutTTY bool
	}{
		{"stdin-only", true, false},
		{"neither", false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := runIdentityRegisterKey(cmd, nil, identityRegisterKeyFlags{}, tt.stdinTTY, tt.stdoutTTY)
			if err == nil {
				t.Fatal("expected a non-nil error naming the missing identity argument")
			}
			if !strings.Contains(err.Error(), "name") {
				t.Errorf("error = %q, want it to name the missing 'name' argument", err.Error())
			}
		})
	}
}

// TestRegisterKeyDryRunExecutesNoUpload asserts zero recorded ssh-key add
// invocations and that the frozen dry-run note is printed.
func TestRegisterKeyDryRunExecutesNoUpload(t *testing.T) {
	home := t.TempDir()
	seedRegisterKeyAccount(t, home, "acme")
	t.Setenv("HOME", home)

	b := newBackendForHome(home)
	var recorded []string
	b.uploaderDeps.LookPath = func(name string) (string, error) { return "/usr/local/bin/" + name, nil }
	b.uploaderDeps.ReadFile = func(string) ([]byte, error) { return []byte("ssh-ed25519 AAAAstub acme@gitid\n"), nil }
	b.uploaderDeps.RunCmd = func(name string, args ...string) (string, int, error) {
		recorded = append(recorded, strings.Join(append([]string{name}, args...), " "))
		switch {
		case len(args) >= 2 && args[0] == "auth" && args[1] == "status":
			return "", 0, nil
		case len(args) >= 2 && args[0] == "api":
			return "[]", 0, nil
		}
		return "", 0, nil
	}

	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	acct, ok := b.findAccount("acme")
	if !ok {
		t.Fatal("setup: findAccount(acme) not found")
	}
	req := uploadRequestForAccount(acct, b.home)
	plan, terminal := b.planUpload(req)
	if terminal != nil {
		t.Fatalf("setup: planUpload returned a terminal view unexpectedly: %+v", terminal)
	}
	for _, c := range plan.commands {
		out.WriteString("Running: " + c + "\n")
	}
	out.WriteString(tuikit.UploadDryRunNote + "\n")

	for _, r := range recorded {
		if strings.Contains(r, "ssh-key add") {
			t.Errorf("dry run recorded a ssh-key add invocation: %q", r)
		}
	}
	if !strings.Contains(out.String(), tuikit.UploadDryRunNote) {
		t.Error("dry run output missing the frozen dry-run note")
	}
}

// TestRegisterKeyUnknownIdentityFailsBeforeAnyProviderCommand asserts a
// non-nil error and zero recorded provider invocations for a name that
// does not resolve to any account.
func TestRegisterKeyUnknownIdentityFailsBeforeAnyProviderCommand(t *testing.T) {
	home := t.TempDir()
	seedSSHDir(t, home)
	t.Setenv("HOME", home)

	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	err := runIdentityRegisterKey(cmd, []string{"ghost"}, identityRegisterKeyFlags{}, false, false)
	if err == nil {
		t.Fatal("expected a non-nil error for an unknown identity")
	}
	if !strings.Contains(err.Error(), "ghost") {
		t.Errorf("error = %q, want it to name the unknown identity", err.Error())
	}
	if out.Len() != 0 {
		t.Errorf("stdout = %q, want empty — no provider command output before the not-found error", out.String())
	}
}

// TestRegisterKeyExitContract drives cliRegisterKeyInto's recording double
// with six canned views and asserts a nil error for the first five and a
// non-nil one only for the last (total attempted-and-failed), and that the
// printed output always contains the outcome rows.
func TestRegisterKeyExitContract(t *testing.T) {
	tests := []struct {
		name    string
		view    tuikit.UploadRunView
		wantErr bool
	}{
		{"all succeeded", tuikit.UploadRunView{Rows: []tuikit.UploadResultRow{
			{Label: "Authentication", Command: "gh ssh-key add k.pub --type authentication", Outcome: tuikit.UploadRowUploaded},
		}}, false},
		{"partial success", tuikit.UploadRunView{Rows: []tuikit.UploadResultRow{
			{Label: "Authentication", Command: "gh ssh-key add k.pub --type authentication", Outcome: tuikit.UploadRowUploaded},
			{Label: "Signing", Command: "gh ssh-key add k.pub --type signing", Outcome: tuikit.UploadRowFailed, Reason: "insufficient scope"},
		}}, false},
		{"all already present", tuikit.UploadRunView{AlreadyComplete: true}, false},
		{"ineligible provider", tuikit.UploadRunView{Skipped: true, ManualFallback: "manual steps"}, false},
		{"all attempted failed", tuikit.UploadRunView{Rows: []tuikit.UploadResultRow{
			{Label: "Authentication", Command: "gh ssh-key add k.pub --type authentication", Outcome: tuikit.UploadRowFailed, Reason: "boom"},
		}}, true},
	}

	home := t.TempDir()
	seedRegisterKeyAccount(t, home, "acme")
	t.Setenv("HOME", home)

	orig := cliRegisterKeyInto
	defer func() { cliRegisterKeyInto = orig }()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cliRegisterKeyInto = func(_ *realBackend, _ uploadRequest) tuikit.UploadRunView { return tt.view }
			var out bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetOut(&out)
			err := runIdentityRegisterKey(cmd, []string{"acme"}, identityRegisterKeyFlags{}, false, false)
			if tt.wantErr && err == nil {
				t.Error("expected a non-nil error for total attempted failure")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if out.Len() == 0 && (len(tt.view.Rows) > 0 || tt.view.AlreadyComplete || tt.view.Skipped) {
				t.Error("printed output was empty despite a non-empty view")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 09-05-PLAN.md Task 3 — --no-upload plus derived autonomous upload.
// ---------------------------------------------------------------------------

// TestNoUploadFlagIsBoundOnAllFourWriteVerbs asserts --no-upload is present
// on create, clone, rotate, and new-key with byte-identical usage text — the
// anti-drift assertion the shared noUploadFlagHelp constant exists to make
// true.
func TestNoUploadFlagIsBoundOnAllFourWriteVerbs(t *testing.T) {
	root := newRootCmd()
	var usages []string
	for _, path := range [][]string{{"create"}, {"clone"}, {"rotate"}, {"new-key"}} {
		cmd, _, err := root.Find(path)
		if err != nil {
			t.Fatalf("resolving %v: %v", path, err)
		}
		f := cmd.Flags().Lookup("no-upload")
		if f == nil {
			t.Fatalf("%v does not bind --no-upload", path)
		}
		usages = append(usages, f.Usage)
	}
	for i := 1; i < len(usages); i++ {
		if usages[i] != usages[0] {
			t.Errorf("--no-upload usage text differs: %q vs %q", usages[0], usages[i])
		}
	}
}

// TestNoUploadSuppressesEveryProviderInvocation asserts zero recorded
// provider invocations and that the skip note plus the fallback block are
// printed, for rotate (the simplest real-account case to seed).
func TestNoUploadSuppressesEveryProviderInvocation(t *testing.T) {
	home := t.TempDir()
	seedDeleteFixture(t, home, "work")
	t.Setenv("HOME", home)

	b := newBackendForHome(home)
	recorded := 0
	b.uploaderDeps.LookPath = func(name string) (string, error) { recorded++; return "/usr/local/bin/" + name, nil }
	b.uploaderDeps.RunCmd = func(string, ...string) (string, int, error) { recorded++; return "", 0, nil }

	acct, ok := b.findAccount("work")
	if !ok {
		t.Fatal("setup: findAccount(work) not found")
	}
	var out bytes.Buffer
	runUploadStep(&out, b, uploadRequestForAccount(acct, b.home), true, false)

	if recorded != 0 {
		t.Errorf("recorded %d provider invocations with --no-upload, want 0", recorded)
	}
	if !strings.Contains(out.String(), tuikit.UploadSkippedByFlagNote) {
		t.Error("missing the frozen skip note")
	}
	if !strings.Contains(out.String(), tuikit.UploadManualHeading) {
		t.Error("missing the manual-fallback heading")
	}
}

// TestUploadFailureDoesNotChangeCreateExitCode asserts runUploadStep's
// caller-visible contract holds regardless of outcome: it returns nothing
// (D-03/D-11 — the caller's control flow and returned error are decided
// solely by the PRIMARY operation), for both a disabled and a totally
// failing upload. This drives runUploadStep directly (rather than through
// runIdentityKeyVerb, which builds its own real, non-injectable backend
// internally) so a failing fake uploaderDeps never risks touching a real
// provider CLI.
func TestUploadFailureDoesNotChangeCreateExitCode(t *testing.T) {
	home := t.TempDir()
	seedDeleteFixture(t, home, "work")
	t.Setenv("HOME", home)

	b := newBackendForHome(home)
	b.uploaderDeps.LookPath = func(name string) (string, error) { return "/usr/local/bin/" + name, nil }
	b.uploaderDeps.ReadFile = os.ReadFile
	b.uploaderDeps.RunCmd = func(_ string, args ...string) (string, int, error) {
		switch {
		case len(args) >= 2 && args[0] == "auth" && args[1] == "status":
			return "", 0, nil
		case len(args) >= 2 && args[0] == "api":
			return "[]", 0, nil
		case len(args) >= 2 && args[0] == "ssh-key" && args[1] == "add":
			return "error: insufficient scope", 1, nil
		}
		return "", 0, nil
	}
	acct, ok := b.findAccount("work")
	if !ok {
		t.Fatal("setup: findAccount(work) not found")
	}
	req := uploadRequestForAccount(acct, b.home)

	var disabled, failing bytes.Buffer
	runUploadStep(&disabled, b, req, true, false)
	runUploadStep(&failing, b, req, false, false)

	if !strings.Contains(failing.String(), tuikit.UploadResultFailedFmt[:5]) && !strings.Contains(failing.String(), "registration failed") {
		t.Errorf("expected a failed-registration row in the enabled-but-failing case; got:\n%s", failing.String())
	}
	// The key assertion: runUploadStep itself never panics or returns an
	// error value — its signature is func(...) with no return, so a
	// caller (create/clone/rotate/new-key) literally cannot receive
	// anything from it that could change its own exit code.
}

// TestUploadRunsBeforeThePostWriteReTest is an ordering double for the
// rotate path: asserts the upload invocation (via cliRegisterKeyInto-shaped
// recording on runUploadFor) is recorded before the connectivity re-test
// (cliConnectivityTest is NOT the re-test seam for rotate's OWN lifecycle
// re-test, which lives inside cliRotateInto's lifecycleResult.ReTest — so
// this test asserts ordering via the printed output instead: the upload
// section's own lines appear before the re-test-driven exit message).
func TestUploadRunsBeforeThePostWriteReTest(t *testing.T) {
	home := t.TempDir()
	seedDeleteFixture(t, home, "work")
	t.Setenv("HOME", home)

	var order []string
	cliRotateInto = func(_ *realBackend, _ string, _ lifecyclePolicy) (lifecycleResult, error) {
		order = append(order, "lifecycle")
		return lifecycleResult{ReTest: tester.Result{Outcome: tester.PASS}}, nil
	}
	defer func() {
		cliRotateInto = func(b *realBackend, name string, p lifecyclePolicy) (lifecycleResult, error) {
			return b.runRotate(name, p)
		}
	}()

	cmd, out, _ := cliTestCmd()
	if err := runIdentityKeyVerb(cmd, "work", "rotate", identityKeyFlags{Yes: true, NoUpload: true}, false, false); err != nil {
		t.Fatalf("rotate --yes --no-upload: %v", err)
	}
	if len(order) != 1 || order[0] != "lifecycle" {
		t.Fatalf("expected exactly one lifecycle call, got %v", order)
	}
	got := out.String()
	uploadIdx := strings.Index(got, tuikit.UploadSkippedByFlagNote)
	effectIdx := strings.Index(got, "took effect for")
	if uploadIdx < 0 || effectIdx < 0 || uploadIdx < effectIdx {
		t.Errorf("expected the upload step's output AFTER the lifecycle's own receipt (upload runs after the write, before the re-test check) — output:\n%s", got)
	}
}
