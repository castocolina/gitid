package identity

import (
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/tester"
)

// modeLog records the mode-specific dep invocations so the orchestration tests
// can assert which effects each mode performed (without touching the network,
// the real keygen, or the filesystem). It embeds callLog (which includes the
// persistKey/cleanup counters) so no extra fields are needed here.
type modeLog struct {
	callLog
	derivePub    int
	pubExists    int
	writePub     int
	pubExistsRet bool
	lastPubLine  string
	lastPubPath  string
}

func newFakeModeDeps(log *modeLog, preOutcome tester.Outcome) Deps {
	d := newFakeDeps(&log.callLog, preOutcome)
	d.PubExists = func(_ string) bool {
		log.pubExists++
		return log.pubExistsRet
	}
	d.DerivePub = func(_ string) (string, error) {
		log.derivePub++
		return "ssh-ed25519 AAAADERIVED comment\n", nil
	}
	d.WritePub = func(pubPath, pubLine string) error {
		log.writePub++
		log.lastPubPath = pubPath
		log.lastPubLine = pubLine
		return nil
	}
	return d
}

func reuseInput() CreateInput {
	in := sampleInput()
	in.Name = "reuse"
	in.Alias = "reuse.github.com"
	in.Matches = []gitconfig.Match{DefaultMatch("reuse")}
	return in
}

// TestReuseSkipsKeygenAndUsesExistingKey asserts Reuse never calls Generate, sets
// the Account at the existing key path, and still drives all FOUR writers plus
// the resolved test through the shared pipeline (IDENT-02).
func TestReuseSkipsKeygenAndUsesExistingKey(t *testing.T) {
	var log modeLog
	log.pubExistsRet = true // .pub already present -> no derive
	deps := newFakeModeDeps(&log, tester.ReachableNotUploaded)

	existingKey := "/tmp/.ssh/id_ed25519_existing"
	res, err := Reuse(reuseInput(), existingKey, deps)
	if err != nil {
		t.Fatalf("Reuse returned error: %v", err)
	}
	if log.generate != 0 {
		t.Errorf("Reuse must NOT call Generate; called %d times", log.generate)
	}
	if res.Key.PrivatePath != existingKey {
		t.Errorf("Reuse Key.PrivatePath = %q, want %q", res.Key.PrivatePath, existingKey)
	}
	if res.Key.PubPath != existingKey+".pub" {
		t.Errorf("Reuse Key.PubPath = %q, want %q", res.Key.PubPath, existingKey+".pub")
	}
	if log.writePub != 0 {
		t.Errorf("Reuse must not write .pub when it already exists; wrote %d times", log.writePub)
	}
	if log.writeSSH != 1 || log.writeGitconfig != 1 || log.writeFragment != 1 || log.writeAllowedSigners != 1 {
		t.Errorf("Reuse must invoke all four writers once; got ssh=%d gitconfig=%d fragment=%d signers=%d",
			log.writeSSH, log.writeGitconfig, log.writeFragment, log.writeAllowedSigners)
	}
	if log.resolved != 1 {
		t.Errorf("Reuse must run the resolved test once; ran %d", log.resolved)
	}
}

// TestReuseDerivesMissingPub asserts that when the existing key's .pub is absent
// Reuse derives it and writes it (0644 is enforced by the WritePub dep), then
// proceeds through the four-writer pipeline (IDENT-02, RESEARCH Q3).
func TestReuseDerivesMissingPub(t *testing.T) {
	var log modeLog
	log.pubExistsRet = false // .pub missing -> derive + write
	deps := newFakeModeDeps(&log, tester.ReachableNotUploaded)

	existingKey := "/tmp/.ssh/id_ed25519_existing"
	if _, err := Reuse(reuseInput(), existingKey, deps); err != nil {
		t.Fatalf("Reuse returned error: %v", err)
	}
	if log.derivePub != 1 {
		t.Errorf("Reuse must derive the missing .pub once; derived %d times", log.derivePub)
	}
	if log.writePub != 1 {
		t.Errorf("Reuse must write the derived .pub once; wrote %d times", log.writePub)
	}
	if log.lastPubPath != existingKey+".pub" {
		t.Errorf("Reuse wrote .pub to %q, want %q", log.lastPubPath, existingKey+".pub")
	}
	if !strings.Contains(log.lastPubLine, "AAAADERIVED") {
		t.Errorf("Reuse wrote unexpected derived line %q", log.lastPubLine)
	}
	if log.writeAllowedSigners != 1 {
		t.Errorf("Reuse must still write allowed_signers; wrote %d times", log.writeAllowedSigners)
	}
}

// TestReuseAbortsOnPreWriteFailure asserts the reuse path honors the same
// pre-write gate as Create: a Failure aborts before any write (D-01).
func TestReuseAbortsOnPreWriteFailure(t *testing.T) {
	var log modeLog
	log.pubExistsRet = true
	deps := newFakeModeDeps(&log, tester.Failure)

	if _, err := Reuse(reuseInput(), "/tmp/.ssh/id_ed25519_existing", deps); err == nil {
		t.Fatal("Reuse must error when the pre-write test fails")
	}
	if log.writeSSH != 0 || log.writeGitconfig != 0 || log.writeFragment != 0 || log.writeAllowedSigners != 0 {
		t.Fatalf("Reuse must perform NO writes on pre-write Failure; got ssh=%d gitconfig=%d fragment=%d signers=%d",
			log.writeSSH, log.writeGitconfig, log.writeFragment, log.writeAllowedSigners)
	}
}

// TestAddAccountSharesKeyPath asserts AddAccount renders a second Host block and
// includeIf for a distinct alias that reuse the existing identity's key path, so
// several identities can share one provider key (IDENT-06).
func TestAddAccountSharesKeyPath(t *testing.T) {
	var log modeLog
	log.pubExistsRet = true
	deps := newFakeModeDeps(&log, tester.ReachableNotUploaded)

	existing := Account{
		Name:     "work",
		GitName:  "Work User",
		GitEmail: "work@example.com",
		Provider: "github",
		Alias:    "work.github.com",
		Hostname: "ssh.github.com",
		Port:     443,
		KeyPath:  "/tmp/.ssh/id_ed25519_work",
		PubPath:  "/tmp/.ssh/id_ed25519_work.pub",
		Matches:  []gitconfig.Match{DefaultMatch("work")},
	}

	res, err := AddAccount(existing, "gitlab", "work.gitlab.com", deps)
	if err != nil {
		t.Fatalf("AddAccount returned error: %v", err)
	}
	// AddAccount must NOT generate a new key.
	if log.generate != 0 {
		t.Errorf("AddAccount must not call Generate; called %d", log.generate)
	}
	// The new SSH host block references the SAME key path as the existing account.
	if !strings.Contains(res.SSHPreview, existing.KeyPath) {
		t.Errorf("AddAccount SSH block must reuse existing key path %q\n%s", existing.KeyPath, res.SSHPreview)
	}
	if !strings.Contains(res.SSHPreview, "Host work.gitlab.com") {
		t.Errorf("AddAccount SSH block must declare the new alias\n%s", res.SSHPreview)
	}
	if res.Key.PrivatePath != existing.KeyPath {
		t.Errorf("AddAccount Key.PrivatePath = %q, want shared %q", res.Key.PrivatePath, existing.KeyPath)
	}
	// A confirmed AddAccount writes the SSH host block + includeIf for the alias.
	if log.writeSSH != 1 {
		t.Errorf("AddAccount must write the SSH host block once; wrote %d", log.writeSSH)
	}
	if log.writeGitconfig != 1 {
		t.Errorf("AddAccount must write the includeIf once; wrote %d", log.writeGitconfig)
	}
}

// rotateAccount is a fully-populated Account for the rotation tests, with the
// gitid-managed target paths the command layer would supply.
func rotateAccount() Account {
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
		Matches:            []gitconfig.Match{DefaultMatch("work")},
		FragmentPath:       "/tmp/.gitconfig.d/work",
		GitconfigPath:      "/tmp/.gitconfig",
		SSHConfigPath:      "/tmp/.ssh/config",
		AllowedSignersPath: "/tmp/.ssh/allowed_signers",
	}
}

// TestRotateGeneratesNewKeyAndRepointsAllFour asserts Rotate generates a fresh
// key and re-points ALL FOUR managed artifacts (SSH host block, includeIf,
// fragment, allowed_signers) to the new key via the shared pipeline — keyed by
// the SAME identity name so ReplaceBlock replaces old references rather than
// duplicating them — then re-runs the resolved two-phase test (KEY-01).
func TestRotateGeneratesNewKeyAndRepointsAllFour(t *testing.T) {
	var log modeLog
	deps := newFakeModeDeps(&log, tester.ReachableNotUploaded)

	// Generate returns a NEW key path distinct from the existing one.
	newKey := "/tmp/.ssh/id_ed25519_work_rotated"
	deps.Generate = func(_ CreateInput) (StagedKey, error) {
		log.generate++
		return StagedKey{
			TempPrivatePath:  "/tmp/stage/newkey",
			FinalPrivatePath: newKey,
			FinalPubPath:     newKey + ".pub",
			PubLine:          "ssh-ed25519 AAAANEWKEY comment\n",
			PrivPEM:          []byte("NEWPEM"),
		}, nil
	}

	res, err := Rotate(rotateAccount(), deps)
	if err != nil {
		t.Fatalf("Rotate returned error: %v", err)
	}
	if log.generate != 1 {
		t.Errorf("Rotate must generate a fresh key once; generated %d", log.generate)
	}
	if res.Key.PrivatePath != newKey {
		t.Errorf("Rotate Key.PrivatePath = %q, want the new key %q", res.Key.PrivatePath, newKey)
	}
	// All four artifacts re-pointed.
	if log.writeSSH != 1 || log.writeGitconfig != 1 || log.writeFragment != 1 || log.writeAllowedSigners != 1 {
		t.Errorf("Rotate must re-point all four artifacts once; got ssh=%d gitconfig=%d fragment=%d signers=%d",
			log.writeSSH, log.writeGitconfig, log.writeFragment, log.writeAllowedSigners)
	}
	// SSH preview references the NEW key, not the old one.
	if !strings.Contains(res.SSHPreview, newKey) {
		t.Errorf("Rotate SSH block must reference the NEW key %q\n%s", newKey, res.SSHPreview)
	}
	if strings.Contains(res.SSHPreview, "id_ed25519_work\n") || strings.Contains(res.SSHPreview, "id_ed25519_work ") {
		t.Errorf("Rotate SSH block must NOT still reference the old key path\n%s", res.SSHPreview)
	}
	// Re-runs the resolved test (KEY-01 re-test).
	if log.resolved != 1 {
		t.Errorf("Rotate must re-run the resolved test once; ran %d", log.resolved)
	}
	// allowed_signers line re-points to the NEW public key.
	if !strings.Contains(res.AllowedSignersLine, "AAAANEWKEY") {
		t.Errorf("Rotate allowed_signers line must carry the NEW public key\n%s", res.AllowedSignersLine)
	}
}

// TestRotateAbortsOnPreWriteFailure asserts rotation honors the pre-write gate:
// a Failure aborts before any artifact is touched (no half-rotated state).
func TestRotateAbortsOnPreWriteFailure(t *testing.T) {
	var log modeLog
	deps := newFakeModeDeps(&log, tester.Failure)
	deps.Generate = func(_ CreateInput) (StagedKey, error) {
		log.generate++
		return StagedKey{
			TempPrivatePath:  "/tmp/stage/new",
			FinalPrivatePath: "/tmp/.ssh/new",
			FinalPubPath:     "/tmp/.ssh/new.pub",
			PubLine:          "ssh-ed25519 AAAANEW c\n",
			PrivPEM:          []byte("NEWPEM"),
		}, nil
	}

	if _, err := Rotate(rotateAccount(), deps); err == nil {
		t.Fatal("Rotate must error when the pre-write test fails")
	}
	if log.writeSSH != 0 || log.writeGitconfig != 0 || log.writeFragment != 0 || log.writeAllowedSigners != 0 {
		t.Fatalf("Rotate must perform NO writes on pre-write Failure; got ssh=%d gitconfig=%d fragment=%d signers=%d",
			log.writeSSH, log.writeGitconfig, log.writeFragment, log.writeAllowedSigners)
	}
}

// TestRotatePersistKeyOnConfirm asserts Rotate records PersistKey count 1 on
// the confirmed (gate-passed) path and 0 on a Failure path.
func TestRotatePersistKeyOnConfirm(t *testing.T) {
	t.Run("confirmed", func(t *testing.T) {
		var log modeLog
		deps := newFakeModeDeps(&log, tester.ReachableNotUploaded)
		deps.Generate = func(_ CreateInput) (StagedKey, error) {
			log.generate++
			return StagedKey{
				TempPrivatePath:  "/tmp/stage/rot",
				FinalPrivatePath: "/tmp/.ssh/id_ed25519_work_rotated",
				FinalPubPath:     "/tmp/.ssh/id_ed25519_work_rotated.pub",
				PubLine:          "ssh-ed25519 AAAAROTED c\n",
				PrivPEM:          []byte("ROTPEM"),
			}, nil
		}
		if _, err := Rotate(rotateAccount(), deps); err != nil {
			t.Fatalf("Rotate returned error: %v", err)
		}
		if log.persistKey != 1 {
			t.Errorf("Rotate confirmed: PersistKey called %d times, want 1", log.persistKey)
		}
	})

	t.Run("gate-failure", func(t *testing.T) {
		var log modeLog
		deps := newFakeModeDeps(&log, tester.Failure)
		deps.Generate = func(_ CreateInput) (StagedKey, error) {
			log.generate++
			return StagedKey{
				TempPrivatePath:  "/tmp/stage/rot",
				FinalPrivatePath: "/tmp/.ssh/id_ed25519_work_rotated",
				FinalPubPath:     "/tmp/.ssh/id_ed25519_work_rotated.pub",
				PubLine:          "ssh-ed25519 AAAAROTED c\n",
				PrivPEM:          []byte("ROTPEM"),
			}, nil
		}
		if _, err := Rotate(rotateAccount(), deps); err == nil {
			t.Fatal("Rotate gate-failure must return an error")
		}
		if log.persistKey != 0 {
			t.Errorf("Rotate gate-failure: PersistKey called %d times, want 0", log.persistKey)
		}
	})
}
