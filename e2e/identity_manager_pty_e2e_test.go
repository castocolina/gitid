//go:build e2e

package e2e

// identity_manager_pty_e2e_test.go — raw-keystroke PTY proof that the
// Identity Manager's Git-only delete really writes (05-01-PLAN.md Task 1,
// the Phase 5 tracer). Drives the REAL `gitid` binary (never gitid-dummy)
// via raw keystrokes over a pseudo-terminal at 100x30, seeded with ONE
// complete identity's four artifacts plus a key pair.
//
// This is the DIRECT countermeasure to 05-RESEARCH.md Pitfall 1
// (realBackend.Persist silently falling through to the in-memory
// tuikit.Reduce for every mutation except AddIdentity/Reset): the test
// restarts the binary in a SECOND PTY session after the delete and asserts
// the deleted Git side is STILL absent from the freshly re-read list — a
// Reduce-only illusion would show the identity healed back to "complete" on
// restart, since nothing would have actually left disk.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// untouchedGitOnlyDeletePaths are the artifacts D-10 says a Git-only delete
// must NEVER touch: the SSH config, both key halves, and allowed_signers.
func untouchedGitOnlyDeletePaths(home, name string) []string {
	sshDir := filepath.Join(home, ".ssh")
	return []string{
		filepath.Join(sshDir, "config"),
		filepath.Join(sshDir, "id_ed25519_"+name),
		filepath.Join(sshDir, "id_ed25519_"+name+".pub"),
		filepath.Join(sshDir, "allowed_signers"),
	}
}

// seedRecipeShapeSSHConfig overwrites the fixture's ~/.ssh/config with a
// recipe-shape (recipes/ssh-config.recipe) managed block for name — the
// alt-SSH `Hostname ssh.github.com` / `Port 443` pair review R-17 asks the
// surviving block to still carry after a git-only delete. seedGitPTYIdentity
// (ui_pty_e2e_test.go's seedMinimalIdentity) writes a plainer fixture
// (`HostName github.com`, no explicit Port) sufficient for its own tests,
// but not for this file's recipe-shape regression proof.
func seedRecipeShapeSSHConfig(t *testing.T, home, name string) {
	t.Helper()
	content := "# BEGIN gitid managed: " + name + "\n" +
		"Host " + name + ".github.com\n" +
		"  Hostname ssh.github.com\n" +
		"  Port 443\n" +
		"  User git\n" +
		"  IdentityFile ~/.ssh/id_ed25519_" + name + "\n" +
		"  IdentitiesOnly yes\n" +
		"# END gitid managed: " + name + "\n\n" +
		"Host *\n" +
		"  IdentitiesOnly yes\n"
	if err := os.WriteFile(filepath.Join(home, ".ssh", "config"), []byte(content), 0o600); err != nil {
		t.Fatalf("seedRecipeShapeSSHConfig: WriteFile: %v", err)
	}
}

// TestIdentityManager_DeleteGitOnly drives the real binary's delete-choice
// ceremony end-to-end: "d" opens the scope chooser (git-only default-
// focused, the safer option), Enter opens the ceremony, Enter confirms
// (async CommitDelete), the receipt renders, and a SECOND binary launch
// proves the write survived a restart.
func TestIdentityManager_DeleteGitOnly(t *testing.T) {
	home := SandboxHome(t)
	seedGitPTYIdentity(t, home, "acme") // ui_pty_e2e_test.go + git_configuration_pty_e2e_test.go fixture: complete SSH+Git+signers+key
	seedRecipeShapeSSHConfig(t, home, "acme")
	bin := BuildBinary(t)

	untouched := untouchedGitOnlyDeletePaths(home, "acme")
	before := snapshotGitBytes(t, untouched)
	if len(before) != len(untouched) {
		t.Fatalf("fixture invalid: expected all %d untouched-path fixtures to pre-exist, got %d: %v", len(untouched), len(before), before)
	}
	sshConfigBefore := before[filepath.Join(home, ".ssh", "config")]
	if !strings.Contains(string(sshConfigBefore), "IdentitiesOnly yes") ||
		!strings.Contains(string(sshConfigBefore), "Hostname ssh.github.com") ||
		!strings.Contains(string(sshConfigBefore), "Port 443") {
		t.Fatalf("fixture invalid: seeded SSH config missing expected recipe-shape lines:\n%s", sshConfigBefore)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	s := startPTYAt(t, newRealCreateFlowCmd(ctx, bin, home, ""), dummyTermWidth, dummyTermHeight)
	closed := false
	defer func() {
		if !closed {
			s.close(t)
		}
	}()

	uiReady(t, s)
	mustSee(t, s, "Identities", "sidebar renders with the seeded identity")
	mustSee(t, s, "acme", "the seeded identity appears in the sidebar")

	s.sendKey([]byte("d"), keystrokeDelay)
	mustSee(t, s, "Delete Git identity only (safer — SSH stays)", "delete-choice: the scope chooser renders")
	mustSee(t, s, "Delete everything (SSH + Git + key) — irreversible", "delete-choice: both scope options render")

	s.sendKey(dummyKeyEnter, keystrokeDelay) // git-only is default-focused — Enter opens the ceremony on it
	mustSee(t, s, `Delete the Git identity of "acme" (SSH stays)`, "ceremony: the git-only heading renders")
	mustSee(t, s, "Nothing has changed yet", "ceremony: the pre-confirm assurance renders before any write")

	// Nothing on disk yet — still just showing the ceremony preview.
	assertGitBytesUnchanged(t, before, snapshotGitBytes(t, untouched))
	fragmentPath := filepath.Join(home, ".gitconfig.d", "acme")
	if _, err := os.Stat(fragmentPath); err != nil {
		t.Fatalf("fragment must still exist before confirm: %v", err)
	}

	s.sendKey(dummyKeyEnter, keystrokeDelay) // confirm — async CommitDelete
	mustSee(t, s, `Git identity of "acme" deleted`, "result-success: the receipt renders after the real write completes")
	mustSee(t, s, "the SSH side is untouched", "result-success: the receipt names the git-only scope's promise")

	saveFrame(t, "identity-manager-delete-git-only-result", s)

	// Filesystem assertions — the D-10 proof: SSH config, both key halves,
	// and allowed_signers are byte-identical to their pre-delete state.
	assertGitBytesUnchanged(t, before, snapshotGitBytes(t, untouched))

	// R-17: the surviving SSH Host block keeps its recipe-shape lines.
	sshConfigAfter, err := os.ReadFile(filepath.Join(home, ".ssh", "config")) //nolint:gosec // fixed sandbox path (G304)
	if err != nil {
		t.Fatalf("reading post-delete ssh config: %v", err)
	}
	for _, want := range []string{"IdentitiesOnly yes", "Hostname ssh.github.com", "Port 443"} {
		if !strings.Contains(string(sshConfigAfter), want) {
			t.Errorf("post-delete SSH config missing recipe-shape line %q:\n%s", want, sshConfigAfter)
		}
	}

	// The Git side is actually gone: includeIf block removed from
	// ~/.gitconfig, fragment file removed from disk.
	gcAfter, err := os.ReadFile(filepath.Join(home, ".gitconfig")) //nolint:gosec // fixed sandbox path (G304)
	if err != nil {
		t.Fatalf("reading post-delete gitconfig: %v", err)
	}
	if strings.Contains(string(gcAfter), "BEGIN gitid managed: acme") {
		t.Errorf("post-delete gitconfig still carries the acme includeIf block:\n%s", gcAfter)
	}
	if _, err := os.Stat(fragmentPath); !os.IsNotExist(err) {
		t.Errorf("fragment file survived a git-only delete: statErr=%v", err)
	}

	s.close(t)
	closed = true

	// Restart the binary in a SECOND PTY session — the direct countermeasure
	// to RESEARCH.md Pitfall 1: a Reduce-only illusion would show "acme"
	// healed back to complete on restart, since nothing would have actually
	// left disk. A real write survives the restart.
	ctx2, cancel2 := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel2()
	s2 := startPTYAt(t, newRealCreateFlowCmd(ctx2, bin, home, ""), dummyTermWidth, dummyTermHeight)
	defer s2.close(t)

	uiReady(t, s2)
	mustSee(t, s2, "acme", "the identity row survives a restart (git-only heals to incomplete, never disappears)")
	mustNotSee(t, s2, `Delete the Git identity of "acme"`, "no stale ceremony carries over into the fresh session")

	saveFrame(t, "identity-manager-delete-git-only-post-restart", s2)
}

// TestIdentityManager_CLIAndTUIProduceByteIdenticalGitconfig runs the
// git-only delete over TWO IDENTICAL sandbox HOMEs — once through the REAL
// binary's TUI ceremony (raw PTY keystrokes) and once through the REAL
// binary's CLI verb (`identity delete --git-only --yes`) — and asserts the
// resulting ~/.gitconfig bytes are byte-equal: D-02's "CLI and TUI MUST call
// the same chokepoint — no behavioral fork" claim, proven end-to-end through
// the compiled binary (cmd/gitid/wiring_test.go proves the in-process half).
func TestIdentityManager_CLIAndTUIProduceByteIdenticalGitconfig(t *testing.T) {
	bin := BuildBinary(t)

	homeTUI := SandboxHome(t)
	seedGitPTYIdentity(t, homeTUI, "acme")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	s := startPTYAt(t, newRealCreateFlowCmd(ctx, bin, homeTUI, ""), dummyTermWidth, dummyTermHeight)
	uiReady(t, s)
	mustSee(t, s, "acme", "TUI path: seeded identity renders")
	s.sendKey([]byte("d"), keystrokeDelay)
	s.sendKey(dummyKeyEnter, keystrokeDelay) // open the git-only ceremony
	mustSee(t, s, `Delete the Git identity of "acme" (SSH stays)`, "TUI path: ceremony opened")
	s.sendKey(dummyKeyEnter, keystrokeDelay) // confirm
	mustSee(t, s, `Git identity of "acme" deleted`, "TUI path: receipt rendered")
	s.close(t)

	// A fresh HOME for the CLI path — SandboxHome sets $HOME for the CURRENT
	// test's own os/exec calls only, but the CLI runs as a real subprocess
	// with its own explicit HOME env entry, so a second t.TempDir() suffices
	// without disturbing the TUI path's already-completed sandbox above.
	homeCLI := t.TempDir()
	seedGitPTYIdentity(t, homeCLI, "acme")

	cliCtx, cliCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cliCancel()
	cliCmd := exec.CommandContext(cliCtx, bin, "identity", "delete", "acme", "--git-only", "--yes") //nolint:gosec // bin from BuildBinary; fixed args
	cliCmd.Env = append(os.Environ(), "HOME="+homeCLI)
	if out, err := cliCmd.CombinedOutput(); err != nil {
		t.Fatalf("CLI identity delete failed: %v\noutput: %s", err, out)
	}

	gcTUI, err := os.ReadFile(filepath.Join(homeTUI, ".gitconfig")) //nolint:gosec // fixed sandbox path (G304)
	if err != nil {
		t.Fatalf("reading TUI-path gitconfig: %v", err)
	}
	gcCLI, err := os.ReadFile(filepath.Join(homeCLI, ".gitconfig")) //nolint:gosec // fixed sandbox path (G304)
	if err != nil {
		t.Fatalf("reading CLI-path gitconfig: %v", err)
	}
	if string(gcTUI) != string(gcCLI) {
		t.Errorf("TUI and CLI delete paths produced different ~/.gitconfig bytes:\nTUI:\n%s\n--- CLI ---\n%s", gcTUI, gcCLI)
	}
}
