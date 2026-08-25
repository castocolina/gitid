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

// newFakeEverythingDeps returns a full fake DeleteDeps for the everything
// scope: managed accounts, provider-ref-count, and key-archive seams are all
// wired to log/track calls. sshFixture/gcFixture are the current bytes;
// accounts is what deps.Accounts() reports (used for D-09 ref-counting).
func newFakeEverythingDeps(log *deleteCallLog, sshFixture, gcFixture []byte, accounts []Account) DeleteDeps {
	deps := newFakeDeleteDeps(log, sshFixture, gcFixture)
	deps.Accounts = func() ([]Account, error) { return accounts, nil }
	deps.ForeignProviderRefs = func(string) (int, error) { return 0, nil }
	deps.RemoveProviderRewrite = func(string) (string, error) { return "pr.bak", nil }
	deps.CopyKeyPairToArchive = func(privPath, pubPath string) (string, string, error) {
		return privPath + ".archived", pubPath + ".archived", nil
	}
	return deps
}

// TestDelete_Everything_NoLongerRefuses asserts that DeleteScopeEverything is
// a real implementation as of this plan: Delete does NOT return
// ErrScopeNotAvailable for it anymore.
func TestDelete_Everything_NoLongerRefuses(t *testing.T) {
	acct := baseDeleteAccount()
	var log deleteCallLog
	deps := newFakeEverythingDeps(&log, sshFixtureWithBlocks(), gcFixtureWithBlocks(), []Account{acct})

	_, err := Delete(acct, DeleteScopeEverything, deps)
	if err != nil {
		t.Fatalf("Delete(everything) error: %v", err)
	}
	if errors.Is(err, ErrScopeNotAvailable) {
		t.Error("Delete(everything) still returns ErrScopeNotAvailable — the scope must be implemented in this plan")
	}
}

// TestDelete_Everything_SoleProviderRemovesRewrite is the D-09 removal
// direction: deleting the ONLY identity for a provider (no other managed
// account, no foreign reference) removes the provider rewrite block and
// names it via ProviderRewriteRemoved/ProviderRewriteBackup.
func TestDelete_Everything_SoleProviderRemovesRewrite(t *testing.T) {
	acct := baseDeleteAccount()
	var log deleteCallLog
	deps := newFakeEverythingDeps(&log, sshFixtureWithBlocks(), gcFixtureWithBlocks(), []Account{acct})

	res, err := Delete(acct, DeleteScopeEverything, deps)
	if err != nil {
		t.Fatalf("Delete(everything) error: %v", err)
	}
	if !res.ProviderRewriteRemoved {
		t.Error("ProviderRewriteRemoved = false, want true (sole provider reference)")
	}
	if res.ProviderRewriteBackup != "pr.bak" {
		t.Errorf("ProviderRewriteBackup = %q, want %q", res.ProviderRewriteBackup, "pr.bak")
	}
}

// TestDelete_Everything_SharedProviderKeepsRewrite is the D-09 survival
// direction: a sibling managed account on the SAME provider keeps the
// rewrite block, and ProviderRewriteRemoved stays false.
func TestDelete_Everything_SharedProviderKeepsRewrite(t *testing.T) {
	acct := baseDeleteAccount()
	sibling := acct
	sibling.Name = "personal"
	sibling.Alias = "personal.github.com"
	sibling.KeyPath = "/tmp/.ssh/id_ed25519_personal"

	var log deleteCallLog
	deps := newFakeEverythingDeps(&log, sshFixtureWithBlocks(), gcFixtureWithBlocks(), []Account{acct, sibling})

	res, err := Delete(acct, DeleteScopeEverything, deps)
	if err != nil {
		t.Fatalf("Delete(everything) error: %v", err)
	}
	if res.ProviderRewriteRemoved {
		t.Error("ProviderRewriteRemoved = true, want false (a sibling identity still uses the provider)")
	}
	if res.ProviderRewriteBackup != "" {
		t.Errorf("ProviderRewriteBackup = %q, want empty (block was not removed)", res.ProviderRewriteBackup)
	}
}

// TestDelete_Everything_ForeignReferenceKeepsRewrite is D-09's hand-written-
// alias half: even with NO other managed account, a non-zero
// ForeignProviderRefs count keeps the rewrite block.
func TestDelete_Everything_ForeignReferenceKeepsRewrite(t *testing.T) {
	acct := baseDeleteAccount()
	var log deleteCallLog
	deps := newFakeEverythingDeps(&log, sshFixtureWithBlocks(), gcFixtureWithBlocks(), []Account{acct})
	deps.ForeignProviderRefs = func(providerKey string) (int, error) {
		if providerKey != "github.com" {
			t.Errorf("ForeignProviderRefs called with %q, want %q", providerKey, "github.com")
		}
		return 1, nil
	}

	res, err := Delete(acct, DeleteScopeEverything, deps)
	if err != nil {
		t.Fatalf("Delete(everything) error: %v", err)
	}
	if res.ProviderRewriteRemoved {
		t.Error("ProviderRewriteRemoved = true, want false (a hand-written alias still targets the provider)")
	}
}

// TestDelete_Everything_ArchivesThenRemovesKeyLast proves the D-11 ordering:
// CopyKeyPairToArchive is invoked, RemoveKeyFiles is invoked LAST, and both
// archive paths are reported.
func TestDelete_Everything_ArchivesThenRemovesKeyLast(t *testing.T) {
	acct := baseDeleteAccount()
	var log deleteCallLog
	var order []string
	deps := newFakeEverythingDeps(&log, sshFixtureWithBlocks(), gcFixtureWithBlocks(), []Account{acct})
	deps.CopyKeyPairToArchive = func(privPath, pubPath string) (string, string, error) {
		order = append(order, "archive")
		return privPath + ".archived", pubPath + ".archived", nil
	}
	origRemoveKeyFiles := deps.RemoveKeyFiles
	deps.RemoveKeyFiles = func(keyPath, pubPath string) (string, string, error) {
		order = append(order, "remove-key")
		return origRemoveKeyFiles(keyPath, pubPath)
	}
	deps.RemoveProviderRewrite = func(string) (string, error) {
		order = append(order, "remove-provider-rewrite")
		return "", nil
	}

	res, err := Delete(acct, DeleteScopeEverything, deps)
	if err != nil {
		t.Fatalf("Delete(everything) error: %v", err)
	}
	if len(order) == 0 || order[len(order)-1] != "remove-key" {
		t.Fatalf("call order = %v, want RemoveKeyFiles ('remove-key') LAST", order)
	}
	if order[0] != "archive" {
		t.Fatalf("call order = %v, want CopyKeyPairToArchive ('archive') FIRST", order)
	}
	wantArchived := []string{acct.KeyPath + ".archived", acct.PubPath + ".archived"}
	if len(res.ArchivedKeyPaths) != 2 || res.ArchivedKeyPaths[0] != wantArchived[0] || res.ArchivedKeyPaths[1] != wantArchived[1] {
		t.Errorf("ArchivedKeyPaths = %v, want %v", res.ArchivedKeyPaths, wantArchived)
	}
	if res.KeyBackup != wantArchived[0] || res.PubBackup != wantArchived[1] {
		t.Errorf("KeyBackup/PubBackup = %q/%q, want %q/%q", res.KeyBackup, res.PubBackup, wantArchived[0], wantArchived[1])
	}
	if log.removeKeyFiles != 1 {
		t.Errorf("RemoveKeyFiles called %d times, want 1", log.removeKeyFiles)
	}
}

// TestDelete_Everything_ArchiveFailureLeavesLiveKeyIntact proves that a
// failure at any step AFTER the key archive and BEFORE the final live-key
// removal never calls RemoveKeyFiles — the live key survives.
func TestDelete_Everything_ArchiveFailureLeavesLiveKeyIntact(t *testing.T) {
	acct := baseDeleteAccount()
	var log deleteCallLog
	deps := newFakeEverythingDeps(&log, sshFixtureWithBlocks(), gcFixtureWithBlocks(), []Account{acct})
	sentinel := errors.New("injected ssh write failure")
	deps.WriteSSH = func([]byte) (string, error) { return "", sentinel }

	res, err := Delete(acct, DeleteScopeEverything, deps)
	if err == nil {
		t.Fatal("Delete(everything) must return the injected error")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error = %v, want it to wrap the injected sentinel", err)
	}
	if log.removeKeyFiles != 0 {
		t.Errorf("RemoveKeyFiles called %d times after an earlier-step failure, want 0", log.removeKeyFiles)
	}
	if len(res.ArchivedKeyPaths) != 2 {
		t.Errorf("ArchivedKeyPaths = %v, want both paths reported despite the later failure", res.ArchivedKeyPaths)
	}
}

// TestDelete_Everything_MissingKeySucceeds asserts that deleting an identity
// whose key file is already absent succeeds without invoking the archive or
// key-removal seams (KeyPath == "").
func TestDelete_Everything_MissingKeySucceeds(t *testing.T) {
	acct := baseDeleteAccount()
	acct.KeyPath = ""
	acct.PubPath = ""
	var log deleteCallLog
	deps := newFakeEverythingDeps(&log, sshFixtureWithBlocks(), gcFixtureWithBlocks(), []Account{acct})
	deps.CopyKeyPairToArchive = func(string, string) (string, string, error) {
		t.Fatal("CopyKeyPairToArchive invoked for an identity with no key path")
		return "", "", nil
	}

	res, err := Delete(acct, DeleteScopeEverything, deps)
	if err != nil {
		t.Fatalf("Delete(everything) error: %v", err)
	}
	if log.removeKeyFiles != 0 {
		t.Errorf("RemoveKeyFiles called %d times for an identity with no key path, want 0", log.removeKeyFiles)
	}
	if len(res.ArchivedKeyPaths) != 0 {
		t.Errorf("ArchivedKeyPaths = %v, want empty", res.ArchivedKeyPaths)
	}
}

// TestSharedKeyOwners_MultipleSiblingsSortedOrder proves SharedKeyOwners
// returns EVERY other identity referencing the target key path, sorted.
func TestSharedKeyOwners_MultipleSiblingsSortedOrder(t *testing.T) {
	shared := "/tmp/.ssh/id_ed25519_shared"
	accounts := []Account{
		{Name: "zeta", KeyPath: shared},
		{Name: "alpha", KeyPath: shared},
		{Name: "target", KeyPath: shared},
		{Name: "unrelated", KeyPath: "/tmp/.ssh/id_ed25519_other"},
	}
	got := SharedKeyOwners(accounts, shared, "target")
	want := []string{"alpha", "zeta"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("SharedKeyOwners = %v, want %v", got, want)
	}
}

// TestSharedKeyOwners_NoOwnersEmpty asserts a sole-owner key returns nil/empty.
func TestSharedKeyOwners_NoOwnersEmpty(t *testing.T) {
	accounts := []Account{{Name: "target", KeyPath: "/tmp/.ssh/id_ed25519_target"}}
	got := SharedKeyOwners(accounts, "/tmp/.ssh/id_ed25519_target", "target")
	if len(got) != 0 {
		t.Errorf("SharedKeyOwners = %v, want empty", got)
	}
}

// TestDelete_Everything_SharedKeyDowngrade_SkipsArchiveKeepsFiles is the D-12
// downgrade proof: when a sibling shares the key, the archive-and-remove
// step is skipped entirely, the live key files survive, and KeyKeptFor names
// the sibling.
func TestDelete_Everything_SharedKeyDowngrade_SkipsArchiveKeepsFiles(t *testing.T) {
	acct := baseDeleteAccount()
	sibling := Account{Name: "sibling", KeyPath: acct.KeyPath, Alias: "sibling.github.com"}

	var log deleteCallLog
	deps := newFakeEverythingDeps(&log, sshFixtureWithBlocks(), gcFixtureWithBlocks(), []Account{acct, sibling})
	deps.CopyKeyPairToArchive = func(string, string) (string, string, error) {
		t.Fatal("CopyKeyPairToArchive invoked despite a surviving sibling (D-12 violation)")
		return "", "", nil
	}

	res, err := Delete(acct, DeleteScopeEverything, deps)
	if err != nil {
		t.Fatalf("Delete(everything) error: %v", err)
	}
	if log.removeKeyFiles != 0 {
		t.Errorf("RemoveKeyFiles called %d times despite a surviving sibling, want 0", log.removeKeyFiles)
	}
	if len(res.ArchivedKeyPaths) != 0 {
		t.Errorf("ArchivedKeyPaths = %v, want empty (key was kept, not archived)", res.ArchivedKeyPaths)
	}
	if len(res.KeyKeptFor) != 1 || res.KeyKeptFor[0] != "sibling" {
		t.Errorf("KeyKeptFor = %v, want [\"sibling\"]", res.KeyKeptFor)
	}
}

// TestDelete_Everything_NoSharedKey_ArchivesAndRemoves is the D-12 contrast:
// with no sibling, the key is archived and removed normally, and KeyKeptFor
// stays empty.
func TestDelete_Everything_NoSharedKey_ArchivesAndRemoves(t *testing.T) {
	acct := baseDeleteAccount()
	var log deleteCallLog
	deps := newFakeEverythingDeps(&log, sshFixtureWithBlocks(), gcFixtureWithBlocks(), []Account{acct})

	res, err := Delete(acct, DeleteScopeEverything, deps)
	if err != nil {
		t.Fatalf("Delete(everything) error: %v", err)
	}
	if log.removeKeyFiles != 1 {
		t.Errorf("RemoveKeyFiles called %d times, want 1", log.removeKeyFiles)
	}
	if len(res.ArchivedKeyPaths) != 2 {
		t.Errorf("ArchivedKeyPaths = %v, want 2 entries", res.ArchivedKeyPaths)
	}
	if len(res.KeyKeptFor) != 0 {
		t.Errorf("KeyKeptFor = %v, want empty", res.KeyKeptFor)
	}
}

// TestDelete_Everything_AllowedSignersKeyedByName proves RemoveAllowedSigners
// is invoked with acct.AllowedSignersPath/acct.Name under everything scope
// (D-10 contrast: git-only never calls it at all).
func TestDelete_Everything_AllowedSignersKeyedByName(t *testing.T) {
	acct := baseDeleteAccount()
	var log deleteCallLog
	deps := newFakeEverythingDeps(&log, sshFixtureWithBlocks(), gcFixtureWithBlocks(), []Account{acct})

	if _, err := Delete(acct, DeleteScopeEverything, deps); err != nil {
		t.Fatalf("Delete(everything) error: %v", err)
	}
	if log.removeAllowedSigns != 1 {
		t.Errorf("RemoveAllowedSigners called %d times, want 1", log.removeAllowedSigns)
	}
	if log.lastAllowedSignPath != acct.AllowedSignersPath {
		t.Errorf("RemoveAllowedSigners path = %q, want %q", log.lastAllowedSignPath, acct.AllowedSignersPath)
	}
	if log.lastAllowedSignName != acct.Name {
		t.Errorf("RemoveAllowedSigners name = %q, want %q", log.lastAllowedSignName, acct.Name)
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
