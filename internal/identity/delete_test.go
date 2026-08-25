package identity

import (
	"errors"
	"strconv"
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
	lastAllowedSignName  string
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
		RemoveAllowedSigners: func(path, name string) (string, error) {
			log.removeAllowedSigns++
			log.lastAllowedSignPath = path
			log.lastAllowedSignName = name
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

// fatalOnInvokeSSHDeps returns a DeleteDeps whose ReadSSH/WriteSSH call
// t.Fatal if invoked — the fail-on-invoke seam used to prove the SSH branch
// is STRUCTURALLY skipped under DeleteScopeGitOnly (review R-13), not merely
// equality-guarded. Every other field is a benign fake so a Git-only Delete
// completes without hitting the SSH seams.
func fatalOnInvokeSSHDeps(t *testing.T, gcFixture []byte) DeleteDeps {
	t.Helper()
	return DeleteDeps{
		ReadSSH: func() ([]byte, error) {
			t.Fatal("ReadSSH invoked under DeleteScopeGitOnly — the SSH branch must be structurally skipped (R-13)")
			return nil, nil
		},
		WriteSSH: func([]byte) (string, error) {
			t.Fatal("WriteSSH invoked under DeleteScopeGitOnly — the SSH branch must be structurally skipped (R-13)")
			return "", nil
		},
		ReadGitconfig:        func() ([]byte, error) { return gcFixture, nil },
		WriteGitconfig:       func([]byte) (string, error) { return "gc.bak", nil },
		RemoveFragment:       func(string) (string, error) { return "frag.bak", nil },
		RemoveAllowedSigners: func(string, string) (string, error) { return "sign.bak", nil },
		RemoveKeyFiles:       func(string, string) (string, string, error) { return "key.bak", "pub.bak", nil },
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

// TestDelete_GitOnly_RemovesGitconfigAndFragmentOnly asserts that
// DeleteScopeGitOnly removes the gitconfig includeIf block and the fragment
// file, and calls neither RemoveAllowedSigners nor RemoveKeyFiles (D-10).
func TestDelete_GitOnly_RemovesGitconfigAndFragmentOnly(t *testing.T) {
	acct := baseDeleteAccount()
	var log deleteCallLog
	deps := newFakeDeleteDeps(&log, sshFixtureWithBlocks(), gcFixtureWithBlocks())

	res, err := Delete(acct, DeleteScopeGitOnly, deps)
	if err != nil {
		t.Fatalf("Delete(git-only) error: %v", err)
	}

	// D-10: neither RemoveAllowedSigners nor RemoveKeyFiles is called.
	if log.removeAllowedSigns != 0 {
		t.Errorf("RemoveAllowedSigners called %d times under git-only, want 0 (D-10)", log.removeAllowedSigns)
	}
	if log.removeKeyFiles != 0 {
		t.Errorf("RemoveKeyFiles called %d times under git-only, want 0 (D-10)", log.removeKeyFiles)
	}

	// The gitconfig and fragment removals DID happen.
	if log.writeGitconfig != 1 {
		t.Errorf("WriteGitconfig called %d times, want 1", log.writeGitconfig)
	}
	if log.removeFragment != 1 {
		t.Errorf("RemoveFragment called %d times, want 1", log.removeFragment)
	}

	// The neither-invoked SSH seams (log.readSSH/writeSSH untouched) plus the
	// explicit SSHUntouched flag are the observable proof.
	if log.readSSH != 0 || log.writeSSH != 0 {
		t.Errorf("ReadSSH/WriteSSH called (%d/%d) under git-only, want 0/0 (R-13)", log.readSSH, log.writeSSH)
	}
	if !res.SSHUntouched {
		t.Error("DeleteResult.SSHUntouched = false under git-only, want true")
	}
	if res.SSHBackup != "" {
		t.Errorf("SSHBackup = %q under git-only, want empty", res.SSHBackup)
	}

	if res.GitconfigBackup != "gc.bak" {
		t.Errorf("GitconfigBackup = %q, want %q", res.GitconfigBackup, "gc.bak")
	}
	if res.FragmentBackup != "frag.bak" {
		t.Errorf("FragmentBackup = %q, want %q", res.FragmentBackup, "frag.bak")
	}
	if res.AllowedSignersBackup != "" {
		t.Errorf("AllowedSignersBackup = %q under git-only, want empty", res.AllowedSignersBackup)
	}
	if res.KeyBackup != "" || res.PubBackup != "" {
		t.Errorf("Key/Pub backups = %q/%q under git-only, want empty/empty", res.KeyBackup, res.PubBackup)
	}
}

// TestDelete_GitOnly_SSHBranchStructurallySkipped is the review R-13 proof:
// injecting DeleteDeps.ReadSSH/WriteSSH seams that call t.Fatal when invoked,
// Delete under DeleteScopeGitOnly must still complete with a nil error — the
// SSH branch is never even reached, not merely equality-guarded into a no-op.
func TestDelete_GitOnly_SSHBranchStructurallySkipped(t *testing.T) {
	acct := baseDeleteAccount()
	deps := fatalOnInvokeSSHDeps(t, gcFixtureWithBlocks())

	if _, err := Delete(acct, DeleteScopeGitOnly, deps); err != nil {
		t.Fatalf("Delete(git-only) error: %v", err)
	}
}

// TestDelete_GitOnly_AllowedSignersAndSSHSurvive is the D-10 proof from the
// caller's point of view: the allowed_signers block and the raw SSH bytes are
// simply never touched — asserted by the fact that the equivalent deps are
// never invoked (see TestDelete_GitOnly_SSHBranchStructurallySkipped) and
// RemoveAllowedSigners is never called (see
// TestDelete_GitOnly_RemovesGitconfigAndFragmentOnly). This test names the
// requirement explicitly for readability of the acceptance criteria.
func TestDelete_GitOnly_AllowedSignersAndSSHSurvive(t *testing.T) {
	acct := baseDeleteAccount()
	var log deleteCallLog
	deps := newFakeDeleteDeps(&log, sshFixtureWithBlocks(), gcFixtureWithBlocks())

	if _, err := Delete(acct, DeleteScopeGitOnly, deps); err != nil {
		t.Fatalf("Delete(git-only) error: %v", err)
	}
	if log.removeAllowedSigns != 0 {
		t.Error("allowed_signers block was touched under DeleteScopeGitOnly (D-10 violation)")
	}
	if log.readSSH != 0 || log.writeSSH != 0 {
		t.Error("SSH config was touched under DeleteScopeGitOnly (D-10 violation)")
	}
}

// TestDelete_Everything_NotYetAvailable asserts that DeleteScopeEverything
// returns an error satisfying errors.Is(err, ErrScopeNotAvailable) in this
// plan, and that NO dep is invoked before that error is returned.
func TestDelete_Everything_NotYetAvailable(t *testing.T) {
	acct := baseDeleteAccount()
	deps := fatalOnInvokeSSHDeps(t, gcFixtureWithBlocks())
	deps.ReadGitconfig = func() ([]byte, error) {
		t.Fatal("ReadGitconfig invoked before the DeleteScopeEverything not-yet-available error was returned")
		return nil, nil
	}
	deps.RemoveFragment = func(string) (string, error) {
		t.Fatal("RemoveFragment invoked before the DeleteScopeEverything not-yet-available error was returned")
		return "", nil
	}

	_, err := Delete(acct, DeleteScopeEverything, deps)
	if err == nil {
		t.Fatal("Delete(everything) must return an error in this plan")
	}
	if !errors.Is(err, ErrScopeNotAvailable) {
		t.Errorf("Delete(everything) error = %v, want errors.Is(err, ErrScopeNotAvailable)", err)
	}
}

// TestDelete_UnknownScope_NotAvailable asserts that an unrecognized scope
// string also surfaces ErrScopeNotAvailable, not a distinct error type — one
// sentinel covers both "unimplemented" and "unrecognized" so both skins can
// still refuse identically regardless of which case fired.
func TestDelete_UnknownScope_NotAvailable(t *testing.T) {
	acct := baseDeleteAccount()
	deps := fatalOnInvokeSSHDeps(t, gcFixtureWithBlocks())

	_, err := Delete(acct, DeleteScope("bogus"), deps)
	if !errors.Is(err, ErrScopeNotAvailable) {
		t.Errorf("Delete(bogus scope) error = %v, want errors.Is(err, ErrScopeNotAvailable)", err)
	}
}

// TestDeleteScopeFrom_KnownAndUnknown proves DeleteScopeFrom recognizes both
// constants and rejects anything else with a typed, non-nil error.
func TestDeleteScopeFrom_KnownAndUnknown(t *testing.T) {
	cases := []struct {
		in      string
		want    DeleteScope
		wantErr bool
	}{
		{"git-only", DeleteScopeGitOnly, false},
		{"everything", DeleteScopeEverything, false},
		{"", "", true},
		{"all", "", true},
		{"GIT-ONLY", "", true}, // case-sensitive: not a silent normalize
	}
	for _, tc := range cases {
		got, err := DeleteScopeFrom(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("DeleteScopeFrom(%q) error = nil, want non-nil", tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("DeleteScopeFrom(%q) error = %v, want nil", tc.in, err)
		}
		if got != tc.want {
			t.Errorf("DeleteScopeFrom(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestDelete_GitOnly_GlobalAndForeignGitconfigPreserved asserts that after a
// Git-only Delete: the "work" includeIf block is gone from the gitconfig
// bytes passed to WriteGitconfig, and foreign content outside any gitid block
// survives untouched.
func TestDelete_GitOnly_GlobalAndForeignGitconfigPreserved(t *testing.T) {
	acct := baseDeleteAccount()
	gcFixture := gcFixtureWithBlocks()

	var capturedGC []byte
	deps := DeleteDeps{
		ReadGitconfig:        func() ([]byte, error) { return gcFixture, nil },
		WriteGitconfig:       func(content []byte) (string, error) { capturedGC = content; return "", nil },
		RemoveFragment:       func(string) (string, error) { return "", nil },
		RemoveAllowedSigners: func(string, string) (string, error) { return "", nil },
		RemoveKeyFiles:       func(string, string) (string, string, error) { return "", "", nil },
	}

	if _, err := Delete(acct, DeleteScopeGitOnly, deps); err != nil {
		t.Fatalf("Delete error: %v", err)
	}

	if containsBlock(capturedGC, "work") {
		t.Error("gitconfig content passed to WriteGitconfig still contains 'work' managed block")
	}
	if !containsLine(capturedGC, "[user]") {
		t.Error("gitconfig content passed to WriteGitconfig is missing foreign '[user]' line")
	}
}

// TestDelete_GitOnly_RemoveBlockUsedForGitconfig verifies that the content
// passed to WriteGitconfig is the result of removing ONLY acct.Name from the
// fixture — using real filewriter.RemoveBlock for comparison.
func TestDelete_GitOnly_RemoveBlockUsedForGitconfig(t *testing.T) {
	acct := baseDeleteAccount()
	gcFixture := gcFixtureWithBlocks()

	var capturedGC []byte
	deps := DeleteDeps{
		ReadGitconfig:        func() ([]byte, error) { return gcFixture, nil },
		WriteGitconfig:       func(c []byte) (string, error) { capturedGC = c; return "", nil },
		RemoveFragment:       func(string) (string, error) { return "", nil },
		RemoveAllowedSigners: func(string, string) (string, error) { return "", nil },
		RemoveKeyFiles:       func(string, string) (string, string, error) { return "", "", nil },
	}

	if _, err := Delete(acct, DeleteScopeGitOnly, deps); err != nil {
		t.Fatalf("Delete error: %v", err)
	}

	expectedGC := filewriter.RemoveBlock(gcFixture, acct.Name)
	if string(capturedGC) != string(expectedGC) {
		t.Errorf("gitconfig content mismatch:\ngot:  %q\nwant: %q", string(capturedGC), string(expectedGC))
	}
}

// TestDelete_GitOnly_FragmentArgs verifies that RemoveFragment receives the
// correct path.
func TestDelete_GitOnly_FragmentArgs(t *testing.T) {
	acct := baseDeleteAccount()
	var log deleteCallLog
	deps := newFakeDeleteDeps(&log, sshFixtureWithBlocks(), gcFixtureWithBlocks())

	if _, err := Delete(acct, DeleteScopeGitOnly, deps); err != nil {
		t.Fatalf("Delete error: %v", err)
	}

	if log.lastFragPath != acct.FragmentPath {
		t.Errorf("RemoveFragment path = %q, want %q", log.lastFragPath, acct.FragmentPath)
	}
}

// TestDelete_GitOnly_ReadGitconfigError verifies that a ReadGitconfig
// failure is propagated and wrapped with the expected "identity: ..." prefix.
func TestDelete_GitOnly_ReadGitconfigError(t *testing.T) {
	acct := baseDeleteAccount()
	sentinel := errors.New("read gitconfig error")
	deps := DeleteDeps{
		ReadGitconfig:        func() ([]byte, error) { return nil, sentinel },
		WriteGitconfig:       func([]byte) (string, error) { return "", nil },
		RemoveFragment:       func(string) (string, error) { return "", nil },
		RemoveAllowedSigners: func(string, string) (string, error) { return "", nil },
		RemoveKeyFiles:       func(string, string) (string, string, error) { return "", "", nil },
	}

	_, err := Delete(acct, DeleteScopeGitOnly, deps)
	if err == nil {
		t.Fatal("Delete must return error when ReadGitconfig fails")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error should wrap sentinel, got: %v", err)
	}
}

// TestDelete_GitOnly_Idempotent verifies that deleting an identity whose
// gitconfig block is already absent (already deleted) is a true no-op: no
// error, no WriteGitconfig call (RemoveBlock produced identical bytes), and
// an empty GitconfigBackup — the review R-14 equality guard in action.
func TestDelete_GitOnly_Idempotent(t *testing.T) {
	acct := baseDeleteAccount()
	acct.Name = "nonexistent" // no block for this name in the fixture

	var log deleteCallLog
	deps := newFakeDeleteDeps(&log, sshFixtureWithBlocks(), gcFixtureWithBlocks())

	res, err := Delete(acct, DeleteScopeGitOnly, deps)
	if err != nil {
		t.Fatalf("Delete(idempotent) error: %v", err)
	}
	if log.writeGitconfig != 0 {
		t.Errorf("WriteGitconfig called %d times on an already-absent block, want 0 (R-14 equality guard)", log.writeGitconfig)
	}
	if res.GitconfigBackup != "" {
		t.Errorf("GitconfigBackup = %q on a no-op delete, want empty", res.GitconfigBackup)
	}
}

// TestDelete_GitOnly_SecondRunProducesNoSecondBackup runs the SAME Git-only
// delete twice over one fixture (using the real filewriter.RemoveBlock
// result to feed the second run's ReadGitconfig, simulating what the on-disk
// bytes would actually look like after run 1) and asserts the second run
// returns a nil error and an empty GitconfigBackup — no second backup file.
func TestDelete_GitOnly_SecondRunProducesNoSecondBackup(t *testing.T) {
	acct := baseDeleteAccount()
	gcFixture := gcFixtureWithBlocks()

	writeCount := 0
	current := gcFixture
	deps := DeleteDeps{
		ReadGitconfig: func() ([]byte, error) { return current, nil },
		WriteGitconfig: func(content []byte) (string, error) {
			writeCount++
			current = content
			return "gc.bak." + strconv.Itoa(writeCount), nil
		},
		RemoveFragment:       func(string) (string, error) { return "", nil },
		RemoveAllowedSigners: func(string, string) (string, error) { return "", nil },
		RemoveKeyFiles:       func(string, string) (string, string, error) { return "", "", nil },
	}

	first, err := Delete(acct, DeleteScopeGitOnly, deps)
	if err != nil {
		t.Fatalf("first Delete error: %v", err)
	}
	if first.GitconfigBackup == "" {
		t.Fatal("first Delete should have produced a gitconfig backup (the block was present)")
	}
	if writeCount != 1 {
		t.Fatalf("writeCount after first Delete = %d, want 1", writeCount)
	}

	second, err := Delete(acct, DeleteScopeGitOnly, deps)
	if err != nil {
		t.Fatalf("second Delete error: %v", err)
	}
	if writeCount != 1 {
		t.Errorf("writeCount after second (idempotent) Delete = %d, want 1 (no second backup)", writeCount)
	}
	if second.GitconfigBackup != "" {
		t.Errorf("second Delete GitconfigBackup = %q, want empty", second.GitconfigBackup)
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
