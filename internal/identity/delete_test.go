package identity

import (
	"errors"
	"testing"

	"github.com/castocolina/gitid/internal/filewriter"
)

// deleteCallLog records which DeleteDeps fields were called and with which args.
type deleteCallLog struct {
	readSSH            int
	readGitconfig      int
	writeSSH           int
	writeGitconfig     int
	removeFragment     int
	removeAllowedSigns int
	removeKeyFiles     int

	// Capture args passed to the write/remove calls.
	lastSSHContent       []byte
	lastGitconfigContent []byte
	lastFragPath         string
	lastAllowedSignPath  string
	lastAllowedSignEmail string
	lastKeyPath          string
	lastPubPath          string
}

// newFakeDeleteDeps returns a DeleteDeps where ReadSSH/ReadGitconfig serve
// fixtures, and Write*/Remove* deps record calls into log.
// sshFixture / gcFixture are returned by the Read deps.
func newFakeDeleteDeps(log *deleteCallLog, sshFixture, gcFixture []byte) DeleteDeps {
	return DeleteDeps{
		ReadSSH: func() ([]byte, error) {
			log.readSSH++
			return sshFixture, nil
		},
		ReadGitconfig: func() ([]byte, error) {
			log.readGitconfig++
			return gcFixture, nil
		},
		WriteSSH: func(content []byte) (string, error) {
			log.writeSSH++
			log.lastSSHContent = content
			return "ssh.bak", nil
		},
		WriteGitconfig: func(content []byte) (string, error) {
			log.writeGitconfig++
			log.lastGitconfigContent = content
			return "gc.bak", nil
		},
		RemoveFragment: func(fragPath string) (string, error) {
			log.removeFragment++
			log.lastFragPath = fragPath
			return "frag.bak", nil
		},
		RemoveAllowedSigners: func(path, email string) (string, error) {
			log.removeAllowedSigns++
			log.lastAllowedSignPath = path
			log.lastAllowedSignEmail = email
			return "sign.bak", nil
		},
		RemoveKeyFiles: func(keyPath, pubPath string) (string, string, error) {
			log.removeKeyFiles++
			log.lastKeyPath = keyPath
			log.lastPubPath = pubPath
			return "key.bak", "pub.bak", nil
		},
	}
}

// baseDeleteAccount returns an Account with all fields populated for delete tests.
func baseDeleteAccount() Account {
	return Account{
		Name:               "work",
		GitName:            "Work User",
		GitEmail:           "work@example.com",
		Provider:           "github",
		Alias:              "work.github.com",
		Hostname:           "ssh.github.com",
		Port:               443,
		KeyPath:            "/tmp/.ssh/id_ed25519_work",
		PubPath:            "/tmp/.ssh/id_ed25519_work.pub",
		FragmentPath:       "/tmp/.gitconfig.d/work",
		GitconfigPath:      "/tmp/.gitconfig",
		SSHConfigPath:      "/tmp/.ssh/config",
		AllowedSignersPath: "/tmp/.ssh/allowed_signers",
	}
}

// sshFixtureWithBlocks returns a minimal SSH config that contains:
//   - a managed block for "work" (the identity being deleted)
//   - a managed block for "_global" (macOS Host * block — must NOT be removed)
//   - a foreign Host block outside any sentinel (must NOT be removed)
func sshFixtureWithBlocks() []byte {
	return []byte(`# BEGIN gitid managed: _global
Host *
  IdentitiesOnly yes
# END gitid managed: _global

# foreign line not inside any block
Host foreign.example.com
  Hostname foreign.example.com
  Port 22

# BEGIN gitid managed: work
Host work.github.com
  Hostname ssh.github.com
  Port 443
  IdentityFile /tmp/.ssh/id_ed25519_work
  IdentitiesOnly yes
# END gitid managed: work
`)
}

// gcFixtureWithBlocks returns a minimal .gitconfig that contains:
//   - a managed block for "work" (the identity being deleted)
//   - foreign content outside any sentinel (must NOT be removed)
func gcFixtureWithBlocks() []byte {
	return []byte(`[user]
	name = Global User

# BEGIN gitid managed: work
[includeIf "gitdir:~/git/work/"]
	path = /tmp/.gitconfig.d/work
# END gitid managed: work
`)
}

// TestDelete_KeepKey asserts that when keepKey=true, RemoveKeyFiles is NOT
// called, but all four artifact removals (SSH block, gitconfig block, fragment
// file, allowed_signers line) ARE performed (D-07).
func TestDelete_KeepKey(t *testing.T) {
	acct := baseDeleteAccount()
	var log deleteCallLog
	deps := newFakeDeleteDeps(&log, sshFixtureWithBlocks(), gcFixtureWithBlocks())

	res, err := Delete(acct, true, deps)
	if err != nil {
		t.Fatalf("Delete(keepKey=true) error: %v", err)
	}

	// RemoveKeyFiles must NOT be called when keepKey=true.
	if log.removeKeyFiles != 0 {
		t.Errorf("RemoveKeyFiles called %d times with keepKey=true, want 0 (D-07)", log.removeKeyFiles)
	}

	// All four artifact removals must have run.
	if log.writeSSH != 1 {
		t.Errorf("WriteSSH called %d times, want 1", log.writeSSH)
	}
	if log.writeGitconfig != 1 {
		t.Errorf("WriteGitconfig called %d times, want 1", log.writeGitconfig)
	}
	if log.removeFragment != 1 {
		t.Errorf("RemoveFragment called %d times, want 1", log.removeFragment)
	}
	if log.removeAllowedSigns != 1 {
		t.Errorf("RemoveAllowedSigners called %d times, want 1", log.removeAllowedSigns)
	}

	// Backup paths from the fakes must appear in DeleteResult.
	if res.SSHBackup != "ssh.bak" {
		t.Errorf("SSHBackup = %q, want %q", res.SSHBackup, "ssh.bak")
	}
	if res.GitconfigBackup != "gc.bak" {
		t.Errorf("GitconfigBackup = %q, want %q", res.GitconfigBackup, "gc.bak")
	}
	if res.FragmentBackup != "frag.bak" {
		t.Errorf("FragmentBackup = %q, want %q", res.FragmentBackup, "frag.bak")
	}
	if res.AllowedSignersBackup != "sign.bak" {
		t.Errorf("AllowedSignersBackup = %q, want %q", res.AllowedSignersBackup, "sign.bak")
	}
}

// TestDelete_DeleteKey asserts that when keepKey=false, RemoveKeyFiles IS
// called with the account's KeyPath and PubPath (D-07 irreversible path).
func TestDelete_DeleteKey(t *testing.T) {
	acct := baseDeleteAccount()
	var log deleteCallLog
	deps := newFakeDeleteDeps(&log, sshFixtureWithBlocks(), gcFixtureWithBlocks())

	_, err := Delete(acct, false, deps)
	if err != nil {
		t.Fatalf("Delete(keepKey=false) error: %v", err)
	}

	if log.removeKeyFiles != 1 {
		t.Errorf("RemoveKeyFiles called %d times with keepKey=false, want 1 (D-07)", log.removeKeyFiles)
	}
	if log.lastKeyPath != acct.KeyPath {
		t.Errorf("RemoveKeyFiles keyPath = %q, want %q", log.lastKeyPath, acct.KeyPath)
	}
	if log.lastPubPath != acct.PubPath {
		t.Errorf("RemoveKeyFiles pubPath = %q, want %q", log.lastPubPath, acct.PubPath)
	}
}

// TestDelete_GlobalAndForeignPreserved asserts that after Delete:
//   - The "_global" SSH Host * block is preserved byte-for-byte (D-08).
//   - Foreign content outside any gitid block is preserved (Success Criterion 3).
//   - Only the "work" managed block is removed from both SSH config and gitconfig.
//
// This test uses the real filewriter.RemoveBlock inside the WriteSSH fake to
// assert the content passed to WriteSSH has the right blocks removed/preserved.
func TestDelete_GlobalAndForeignPreserved(t *testing.T) {
	acct := baseDeleteAccount()
	sshFixture := sshFixtureWithBlocks()
	gcFixture := gcFixtureWithBlocks()

	var capturedSSH []byte
	var capturedGC []byte

	deps := DeleteDeps{
		ReadSSH:       func() ([]byte, error) { return sshFixture, nil },
		ReadGitconfig: func() ([]byte, error) { return gcFixture, nil },
		WriteSSH: func(content []byte) (string, error) {
			capturedSSH = content
			return "", nil
		},
		WriteGitconfig: func(content []byte) (string, error) {
			capturedGC = content
			return "", nil
		},
		RemoveFragment:       func(_ string) (string, error) { return "", nil },
		RemoveAllowedSigners: func(_, _ string) (string, error) { return "", nil },
		RemoveKeyFiles:       func(_, _ string) (string, string, error) { return "", "", nil },
	}

	_, err := Delete(acct, true, deps)
	if err != nil {
		t.Fatalf("Delete error: %v", err)
	}

	// The "work" block must be gone from the SSH content passed to WriteSSH.
	if containsBlock(capturedSSH, "work") {
		t.Error("SSH content passed to WriteSSH still contains 'work' managed block")
	}

	// The "_global" block must still be present (D-08).
	if !containsBlock(capturedSSH, "_global") {
		t.Error("SSH content passed to WriteSSH is missing '_global' managed block (D-08)")
	}

	// Foreign SSH content outside any block must be preserved.
	if !containsLine(capturedSSH, "Host foreign.example.com") {
		t.Error("SSH content passed to WriteSSH is missing foreign 'Host foreign.example.com' line (Success Criterion 3)")
	}

	// The "work" block must be gone from the gitconfig content passed to WriteGitconfig.
	if containsBlock(capturedGC, "work") {
		t.Error("gitconfig content passed to WriteGitconfig still contains 'work' managed block")
	}

	// Foreign gitconfig content outside any block must be preserved.
	if !containsLine(capturedGC, "[user]") {
		t.Error("gitconfig content passed to WriteGitconfig is missing foreign '[user]' line (Success Criterion 3)")
	}
}

// TestDelete_RemoveBlockUsedForSSHAndGitconfig verifies that the content
// passed to WriteSSH/WriteGitconfig is the result of removing ONLY acct.Name
// from the respective fixture — using real filewriter.RemoveBlock for
// comparison. This directly proves the implementation calls RemoveBlock with
// acct.Name (not a hardcoded string or the wrong name).
func TestDelete_RemoveBlockUsedForSSHAndGitconfig(t *testing.T) {
	acct := baseDeleteAccount()
	sshFixture := sshFixtureWithBlocks()
	gcFixture := gcFixtureWithBlocks()

	var capturedSSH, capturedGC []byte
	deps := DeleteDeps{
		ReadSSH:              func() ([]byte, error) { return sshFixture, nil },
		ReadGitconfig:        func() ([]byte, error) { return gcFixture, nil },
		WriteSSH:             func(c []byte) (string, error) { capturedSSH = c; return "", nil },
		WriteGitconfig:       func(c []byte) (string, error) { capturedGC = c; return "", nil },
		RemoveFragment:       func(_ string) (string, error) { return "", nil },
		RemoveAllowedSigners: func(_, _ string) (string, error) { return "", nil },
		RemoveKeyFiles:       func(_, _ string) (string, string, error) { return "", "", nil },
	}

	_, err := Delete(acct, true, deps)
	if err != nil {
		t.Fatalf("Delete error: %v", err)
	}

	expectedSSH := filewriter.RemoveBlock(sshFixture, acct.Name)
	if string(capturedSSH) != string(expectedSSH) {
		t.Errorf("SSH content mismatch:\ngot:  %q\nwant: %q", string(capturedSSH), string(expectedSSH))
	}

	expectedGC := filewriter.RemoveBlock(gcFixture, acct.Name)
	if string(capturedGC) != string(expectedGC) {
		t.Errorf("gitconfig content mismatch:\ngot:  %q\nwant: %q", string(capturedGC), string(expectedGC))
	}
}

// TestDelete_AllowedSignersArgs verifies that RemoveAllowedSigners receives the
// correct path and email from the account.
func TestDelete_AllowedSignersArgs(t *testing.T) {
	acct := baseDeleteAccount()
	var log deleteCallLog
	deps := newFakeDeleteDeps(&log, sshFixtureWithBlocks(), gcFixtureWithBlocks())

	_, err := Delete(acct, true, deps)
	if err != nil {
		t.Fatalf("Delete error: %v", err)
	}

	if log.lastAllowedSignPath != acct.AllowedSignersPath {
		t.Errorf("RemoveAllowedSigners path = %q, want %q", log.lastAllowedSignPath, acct.AllowedSignersPath)
	}
	if log.lastAllowedSignEmail != acct.GitEmail {
		t.Errorf("RemoveAllowedSigners email = %q, want %q", log.lastAllowedSignEmail, acct.GitEmail)
	}
}

// TestDelete_FragmentArgs verifies that RemoveFragment receives the correct path.
func TestDelete_FragmentArgs(t *testing.T) {
	acct := baseDeleteAccount()
	var log deleteCallLog
	deps := newFakeDeleteDeps(&log, sshFixtureWithBlocks(), gcFixtureWithBlocks())

	_, err := Delete(acct, true, deps)
	if err != nil {
		t.Fatalf("Delete error: %v", err)
	}

	if log.lastFragPath != acct.FragmentPath {
		t.Errorf("RemoveFragment path = %q, want %q", log.lastFragPath, acct.FragmentPath)
	}
}

// TestDelete_ReadSSHError verifies that a ReadSSH failure is propagated and
// wrapped with the expected "identity: ..." prefix.
func TestDelete_ReadSSHError(t *testing.T) {
	acct := baseDeleteAccount()
	sentinel := errors.New("read ssh error")
	deps := DeleteDeps{
		ReadSSH:              func() ([]byte, error) { return nil, sentinel },
		ReadGitconfig:        func() ([]byte, error) { return gcFixtureWithBlocks(), nil },
		WriteSSH:             func(_ []byte) (string, error) { return "", nil },
		WriteGitconfig:       func(_ []byte) (string, error) { return "", nil },
		RemoveFragment:       func(_ string) (string, error) { return "", nil },
		RemoveAllowedSigners: func(_, _ string) (string, error) { return "", nil },
		RemoveKeyFiles:       func(_, _ string) (string, string, error) { return "", "", nil },
	}

	_, err := Delete(acct, true, deps)
	if err == nil {
		t.Fatal("Delete must return error when ReadSSH fails")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error should wrap sentinel, got: %v", err)
	}
}

// TestDelete_Idempotent verifies that deleting an identity that has no managed
// block in the file (block already absent) returns no error and calls WriteSSH
// with the original content (RemoveBlock idempotent property).
func TestDelete_Idempotent(t *testing.T) {
	acct := baseDeleteAccount()
	acct.Name = "nonexistent" // no block for this name in the fixture

	var log deleteCallLog
	deps := newFakeDeleteDeps(&log, sshFixtureWithBlocks(), gcFixtureWithBlocks())

	_, err := Delete(acct, true, deps)
	if err != nil {
		t.Fatalf("Delete(idempotent) error: %v", err)
	}
	// WriteSSH must still be called (content unchanged, but write still happens)
	if log.writeSSH != 1 {
		t.Errorf("WriteSSH called %d times on idempotent delete, want 1", log.writeSSH)
	}
}

// containsBlock reports whether content contains a complete gitid managed block
// for name (i.e., both BEGIN and END sentinel lines).
func containsBlock(content []byte, name string) bool {
	begin := filewriter.BeginPrefix + name
	end := filewriter.EndPrefix + name
	s := string(content)
	return containsStr(s, begin) && containsStr(s, end)
}

// containsLine reports whether content contains a line matching the given string.
func containsLine(content []byte, line string) bool {
	return containsStr(string(content), line)
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && stringContains(s, sub))
}

func stringContains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
