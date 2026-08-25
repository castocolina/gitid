package identity

import (
	"errors"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/tester"
)

// repairLog extends callLog with the two seams RepairKey may touch beyond
// the shared base (D-07 append, and the archive seam it must NEVER call).
type repairLog struct {
	callLog
	archiveKeyPair       int
	appendAllowedSigners int
	lastAppendIdentity   string
	lastAppendEmail      string
	lastAppendPubLine    string
	lastWriteSSHIdentity string
}

// newFakeRepairDeps builds a Deps with repair's two extra seams wired on top
// of the shared newFakeDeps base, recording every call. ArchiveKeyPair fails
// the test outright if invoked — repair must never archive (D-05).
func newFakeRepairDeps(t *testing.T, log *repairLog, preOutcome tester.Outcome) Deps {
	d := newFakeDeps(&log.callLog, preOutcome)
	d.ArchiveKeyPair = func(privPath, pubPath string) (string, string, error) {
		log.archiveKeyPair++
		t.Fatalf("RepairKey must NEVER call ArchiveKeyPair (D-05); called with priv=%q pub=%q", privPath, pubPath)
		return "", "", nil
	}
	d.AppendAllowedSigners = func(_, identity, email, pubLine string) (string, error) {
		log.appendAllowedSigners++
		log.lastAppendIdentity = identity
		log.lastAppendEmail = email
		log.lastAppendPubLine = pubLine
		return "", nil
	}
	origWriteSSH := d.WriteSSH
	d.WriteSSH = func(accountName, hostBlock, globalBlock string) (string, error) {
		log.lastWriteSSHIdentity = accountName
		return origWriteSSH(accountName, hostBlock, globalBlock)
	}
	return d
}

// repairAccount builds a fully-populated Account for the repair tests, with
// the gitid-managed target paths the command layer would supply.
func repairAccount(name, keyPath string) Account {
	return Account{
		Name:               name,
		GitName:            name + " User",
		GitEmail:           name + "@example.com",
		Provider:           "github",
		Alias:              name + ".github.com",
		Hostname:           "ssh.github.com",
		Port:               443,
		KeyPath:            keyPath,
		PubPath:            keyPath + ".pub",
		Matches:            []gitconfig.Match{DefaultMatch(name)},
		FragmentPath:       "/tmp/.gitconfig.d/" + name,
		GitconfigPath:      "/tmp/.gitconfig",
		SSHConfigPath:      "/tmp/.ssh/config",
		AllowedSignersPath: "/tmp/.ssh/allowed_signers",
	}
}

// repairGenerate returns a fake Generate that produces a StagedKey at
// RepairKeyPath's own-name-derived target — mirroring how the real
// composition root's Generate independently recomputes the same path from
// CreateInput.Algo ("ed25519", repairAlgo) and CreateInput.Name.
func repairGenerate(log *repairLog, acct Account, pubLine string) func(CreateInput) (StagedKey, error) {
	return func(_ CreateInput) (StagedKey, error) {
		log.generate++
		privPath, pubPath := RepairKeyPath(acct)
		return StagedKey{
			TempPrivatePath:  "/tmp/stage/repair",
			FinalPrivatePath: privPath,
			FinalPubPath:     pubPath,
			PubLine:          pubLine,
			PrivPEM:          []byte("REPAIRPEM"),
		}, nil
	}
}

// TestRepairKeyPathTargetsOwnNameNeverAccountKeyPath asserts RepairKeyPath
// returns the keygen.KeyPaths derivation of Account.Name, NOT Account.KeyPath
// — for an account whose KeyPath names a SIBLING (review R-02).
func TestRepairKeyPathTargetsOwnNameNeverAccountKeyPath(t *testing.T) {
	acct := repairAccount("identA", "/tmp/.ssh/id_ed25519_identB") // borrowing identB's key

	privPath, pubPath := RepairKeyPath(acct)

	if privPath == acct.KeyPath {
		t.Errorf("RepairKeyPath must NOT return Account.KeyPath (the sibling's path); got %q", privPath)
	}
	if privPath != "/tmp/.ssh/id_ed25519_identA" {
		t.Errorf("RepairKeyPath privPath = %q, want the own-name derivation %q", privPath, "/tmp/.ssh/id_ed25519_identA")
	}
	if pubPath != privPath+".pub" {
		t.Errorf("RepairKeyPath pubPath = %q, want %q", pubPath, privPath+".pub")
	}
}

// TestRepairKeyRepairsIdentAWithoutTouchingIdentB seeds two identities where
// identity A points at identity B's key file, repairs A, and asserts A's
// rendered Host block now names A's OWN canonical path while no Deps seam
// is ever invoked with identity B's name or paths — repair does not receive
// B as an argument at all, so B is structurally untouched by construction;
// this test proves the ceremony targets A's own path rather than silently
// operating on the shared path.
func TestRepairKeyRepairsIdentAWithoutTouchingIdentB(t *testing.T) {
	var log repairLog
	deps := newFakeRepairDeps(t, &log, tester.ReachableNotUploaded)

	identBKeyPath := "/tmp/.ssh/id_ed25519_identB"
	acctA := repairAccount("identA", identBKeyPath) // A currently borrows B's key
	deps.Generate = repairGenerate(&log, acctA, "ssh-ed25519 AAAAREPAIRED c\n")

	res, err := RepairKey(acctA, nil, deps)
	if err != nil {
		t.Fatalf("RepairKey returned error: %v", err)
	}

	wantPriv, wantPub := RepairKeyPath(acctA)
	if res.Key.PrivatePath != wantPriv || res.Key.PubPath != wantPub {
		t.Errorf("RepairKey targeted priv=%q pub=%q, want own-name derivation priv=%q pub=%q",
			res.Key.PrivatePath, res.Key.PubPath, wantPriv, wantPub)
	}
	if strings.Contains(res.SSHPreview, "id_ed25519_identB") {
		t.Errorf("repaired SSH preview must NOT reference identB's key path:\n%s", res.SSHPreview)
	}
	if !strings.Contains(res.SSHPreview, "id_ed25519_identA") {
		t.Errorf("repaired SSH preview must reference identA's OWN canonical path:\n%s", res.SSHPreview)
	}
	if log.lastWriteSSHIdentity != "identA" {
		t.Errorf("WriteSSH was called for identity %q, want %q (identB must never be touched)", log.lastWriteSSHIdentity, "identA")
	}
}

// TestRepairKeySharedTargetRefuses asserts a non-empty owner list makes
// RepairKey return a wrapped ErrRepairTargetShared naming every owner, and
// that NO seam on Deps was invoked (all seams recorded, count zero).
func TestRepairKeySharedTargetRefuses(t *testing.T) {
	var log repairLog
	deps := newFakeRepairDeps(t, &log, tester.ReachableNotUploaded)
	acct := repairAccount("identA", "/tmp/.ssh/id_ed25519_identA")

	owners := []string{"identC", "identD"}
	_, err := RepairKey(acct, owners, deps)
	if err == nil {
		t.Fatal("RepairKey must return an error when the target is shared")
	}
	if !errors.Is(err, ErrRepairTargetShared) {
		t.Errorf("error = %v, want errors.Is(err, ErrRepairTargetShared)", err)
	}
	for _, owner := range owners {
		if !strings.Contains(err.Error(), owner) {
			t.Errorf("error message %q must name owner %q", err.Error(), owner)
		}
	}
	if log.generate != 0 || log.persistKey != 0 || log.writeSSH != 0 || log.writeGitconfig != 0 ||
		log.writeFragment != 0 || log.appendAllowedSigners != 0 || log.resolved != 0 || log.archiveKeyPair != 0 {
		t.Errorf("a shared-target refusal must invoke NO seam; got generate=%d persistKey=%d writeSSH=%d writeGitconfig=%d writeFragment=%d append=%d resolved=%d archive=%d",
			log.generate, log.persistKey, log.writeSSH, log.writeGitconfig, log.writeFragment, log.appendAllowedSigners, log.resolved, log.archiveKeyPair)
	}
}

// TestRepairKeyNeverCallsArchiveSeam asserts RepairKey never invokes
// deps.ArchiveKeyPair (D-05) — enforced by newFakeRepairDeps' fail-the-test
// wiring; a successful RepairKey run here is itself the proof.
func TestRepairKeyNeverCallsArchiveSeam(t *testing.T) {
	var log repairLog
	deps := newFakeRepairDeps(t, &log, tester.ReachableNotUploaded)
	acct := repairAccount("identA", "/tmp/.ssh/id_ed25519_identA")
	deps.Generate = repairGenerate(&log, acct, "ssh-ed25519 AAAAREPAIRED c\n")

	if _, err := RepairKey(acct, nil, deps); err != nil {
		t.Fatalf("RepairKey returned error: %v", err)
	}
	if log.archiveKeyPair != 0 {
		t.Errorf("ArchiveKeyPair invoked %d times, want 0", log.archiveKeyPair)
	}
}

// TestRepairKeyPersistsExactlyOnce asserts a recording Deps.PersistKey seam
// counts exactly 1 invocation after one RepairKey run.
func TestRepairKeyPersistsExactlyOnce(t *testing.T) {
	var log repairLog
	deps := newFakeRepairDeps(t, &log, tester.ReachableNotUploaded)
	acct := repairAccount("identA", "/tmp/.ssh/id_ed25519_identA")
	deps.Generate = repairGenerate(&log, acct, "ssh-ed25519 AAAAREPAIRED c\n")

	if _, err := RepairKey(acct, nil, deps); err != nil {
		t.Fatalf("RepairKey returned error: %v", err)
	}
	if log.persistKey != 1 {
		t.Errorf("PersistKey invoked %d times, want exactly 1", log.persistKey)
	}
}

// TestRepairKeyUsesAppendWriterNotReplaceWriter asserts RepairKey selects
// the APPENDING signer writer (deps.AppendAllowedSigners), never the
// replacing deps.WriteAllowedSigners — this plan's signer-semantics
// resolution (APPEND) applied to repair.
func TestRepairKeyUsesAppendWriterNotReplaceWriter(t *testing.T) {
	var log repairLog
	deps := newFakeRepairDeps(t, &log, tester.ReachableNotUploaded)
	acct := repairAccount("identA", "/tmp/.ssh/id_ed25519_identA")
	deps.Generate = repairGenerate(&log, acct, "ssh-ed25519 AAAAREPAIRED c\n")

	if _, err := RepairKey(acct, nil, deps); err != nil {
		t.Fatalf("RepairKey returned error: %v", err)
	}
	if log.appendAllowedSigners != 1 {
		t.Errorf("AppendAllowedSigners invoked %d times, want exactly 1", log.appendAllowedSigners)
	}
	if log.writeAllowedSigners != 0 {
		t.Errorf("WriteAllowedSigners (the replacing writer) invoked %d times, want 0", log.writeAllowedSigners)
	}
	if log.lastAppendIdentity != "identA" {
		t.Errorf("AppendAllowedSigners identity = %q, want %q", log.lastAppendIdentity, "identA")
	}
	if log.lastAppendEmail != "identA@example.com" {
		t.Errorf("AppendAllowedSigners email = %q, want the account's GitEmail", log.lastAppendEmail)
	}
	if !strings.Contains(log.lastAppendPubLine, "AAAAREPAIRED") {
		t.Errorf("AppendAllowedSigners pubLine = %q, want the NEW staged public line", log.lastAppendPubLine)
	}
}

// TestRepairKeyAllowedSignersEndsWithTwoLines pins the APPEND resolution at
// the block-accumulation level: repairing an identity whose allowed_signers
// block already carries one line (simulated via a stateful fake) ends with
// two lines, not one.
func TestRepairKeyAllowedSignersEndsWithTwoLines(t *testing.T) {
	var log repairLog
	deps := newFakeRepairDeps(t, &log, tester.ReachableNotUploaded)
	acct := repairAccount("identA", "/tmp/.ssh/id_ed25519_identA")
	deps.Generate = repairGenerate(&log, acct, "ssh-ed25519 AAAAREPAIRED c\n")

	block := []string{"identA@example.com namespaces=\"git\" ssh-ed25519 AAAAOLD"} // pre-existing line
	deps.AppendAllowedSigners = func(_, identity, email, pubLine string) (string, error) {
		log.appendAllowedSigners++
		newLine, lerr := mustLine(email, pubLine)
		if lerr != nil {
			return "", lerr
		}
		block = append(block, newLine)
		_ = identity
		return "", nil
	}

	if _, err := RepairKey(acct, nil, deps); err != nil {
		t.Fatalf("RepairKey returned error: %v", err)
	}
	if len(block) != 2 {
		t.Errorf("allowed_signers block has %d lines after repair, want 2 (D-07 APPEND): %v", len(block), block)
	}
}

// mustLine mirrors keygen.AllowedSignersLine's shape closely enough for the
// two-lines test above, without importing internal/keygen into this
// domain-level test's fake — the identity package's own tests already avoid
// depending on keygen for anything beyond what Deps already injects.
func mustLine(email, pubLine string) (string, error) {
	if strings.Contains(email, ",") {
		return "", errors.New("comma in email")
	}
	return email + " namespaces=\"git\" " + strings.TrimRight(pubLine, "\n"), nil
}

// TestRepairKeyAbortsOnPreWriteFailure asserts repair honors the pre-write
// gate: a Failure aborts before any artifact is touched.
func TestRepairKeyAbortsOnPreWriteFailure(t *testing.T) {
	var log repairLog
	deps := newFakeRepairDeps(t, &log, tester.Failure)
	acct := repairAccount("identA", "/tmp/.ssh/id_ed25519_identA")
	deps.Generate = repairGenerate(&log, acct, "ssh-ed25519 AAAAREPAIRED c\n")

	if _, err := RepairKey(acct, nil, deps); err == nil {
		t.Fatal("RepairKey must error when the pre-write test fails")
	}
	if log.persistKey != 0 {
		t.Errorf("RepairKey gate-failure: PersistKey called %d times, want 0", log.persistKey)
	}
	if log.writeSSH != 0 || log.writeGitconfig != 0 || log.writeFragment != 0 || log.appendAllowedSigners != 0 {
		t.Fatalf("RepairKey must perform NO writes on pre-write Failure; got ssh=%d gitconfig=%d fragment=%d signers=%d",
			log.writeSSH, log.writeGitconfig, log.writeFragment, log.appendAllowedSigners)
	}
}

// TestRepairKeyMissingIdentityGeneratesFreshKey asserts repairing a
// key-missing identity (no key ever existed at its own canonical path)
// generates a fresh key at that target and re-points the four artifacts.
func TestRepairKeyMissingIdentityGeneratesFreshKey(t *testing.T) {
	var log repairLog
	deps := newFakeRepairDeps(t, &log, tester.ReachableNotUploaded)
	// The account's own canonical path — nothing exists there yet, but
	// RepairKeyPath derives the SAME path regardless of whether the file is
	// present (KeyPath still names where the missing key WOULD be).
	acct := repairAccount("identA", "/tmp/.ssh/id_ed25519_identA")
	deps.Generate = repairGenerate(&log, acct, "ssh-ed25519 AAAAFRESH c\n")

	res, err := RepairKey(acct, nil, deps)
	if err != nil {
		t.Fatalf("RepairKey returned error: %v", err)
	}
	if log.writeSSH != 1 || log.writeGitconfig != 1 || log.writeFragment != 1 || log.appendAllowedSigners != 1 {
		t.Errorf("RepairKey must re-point all four artifacts once; got ssh=%d gitconfig=%d fragment=%d signers=%d",
			log.writeSSH, log.writeGitconfig, log.writeFragment, log.appendAllowedSigners)
	}
	if !strings.Contains(res.AllowedSignersLine, "AAAAFRESH") {
		t.Errorf("repair allowed_signers line must carry the fresh public key\n%s", res.AllowedSignersLine)
	}
}
