package identity

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/tester"
)

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

// rotateLog extends callLog with the two rotate-only seams (D-06 archive,
// D-07 append) no other mode uses.
type rotateLog struct {
	callLog
	archiveKeyPair       int
	appendAllowedSigners int
	lastArchivedPriv     string
	lastArchivedPub      string
	lastAppendEmail      string
	lastAppendPubLine    string
	// archiveReturn/archiveErr let a test script exactly what ArchiveKeyPair
	// returns, including the R3-01 shape (both paths PLUS an error).
	archiveReturnPriv string
	archiveReturnPub  string
	archiveErr        error
}

// newFakeRotateDeps builds a Deps with rotate's two extra seams wired on top
// of the shared newFakeDeps base, recording every call.
func newFakeRotateDeps(log *rotateLog, preOutcome tester.Outcome) Deps {
	d := newFakeDeps(&log.callLog, preOutcome)
	d.ArchiveKeyPair = func(privPath, pubPath string) (string, string, error) {
		log.archiveKeyPair++
		log.lastArchivedPriv = privPath
		log.lastArchivedPub = pubPath
		if log.archiveErr != nil {
			return log.archiveReturnPriv, log.archiveReturnPub, log.archiveErr
		}
		return privPath + ".archived", pubPath + ".archived", nil
	}
	d.AppendAllowedSigners = func(_, identity, email, pubLine string) (string, error) {
		log.appendAllowedSigners++
		log.lastAppendEmail = email
		log.lastAppendPubLine = pubLine
		_ = identity
		return "", nil
	}
	return d
}

// rotateGenerate returns a fake Generate that returns newKey/newKey+".pub" as
// the FINAL paths (simulating the real composition root reconstructing the
// SAME canonical path from a correctly-derived Algo — see
// TestRotateInputPreservesAlgorithm for the unit proof of that derivation).
func rotateGenerate(log *rotateLog, tempPath, newKey, pubLine string) func(CreateInput) (StagedKey, error) {
	return func(_ CreateInput) (StagedKey, error) {
		log.generate++
		return StagedKey{
			TempPrivatePath:  tempPath,
			FinalPrivatePath: newKey,
			FinalPubPath:     newKey + ".pub",
			PubLine:          pubLine,
			PrivPEM:          []byte("NEWPEM"),
		}, nil
	}
}

// TestRotateGeneratesNewKeyAndRepointsAllFour asserts Rotate generates a
// fresh key, archives the old pair, persists the new pair exactly once, and
// re-points ALL FOUR managed artifacts via the APPEND signer writer, then
// re-runs the resolved test (KEY-01/KEY-05).
func TestRotateGeneratesNewKeyAndRepointsAllFour(t *testing.T) {
	var log rotateLog
	deps := newFakeRotateDeps(&log, tester.ReachableNotUploaded)

	newKey := "/tmp/.ssh/id_ed25519_work" // same canonical path, D-06/D-08
	deps.Generate = rotateGenerate(&log, "/tmp/stage/newkey", newKey, "ssh-ed25519 AAAANEWKEY comment\n")

	res, err := Rotate(rotateAccount(), deps)
	if err != nil {
		t.Fatalf("Rotate returned error: %v", err)
	}
	if log.generate != 1 {
		t.Errorf("Rotate must generate a fresh key once; generated %d", log.generate)
	}
	if res.Key.PrivatePath != newKey {
		t.Errorf("Rotate Key.PrivatePath = %q, want %q", res.Key.PrivatePath, newKey)
	}
	if log.archiveKeyPair != 1 {
		t.Errorf("Rotate must archive the previous key pair once; archived %d times", log.archiveKeyPair)
	}
	if log.lastArchivedPriv != "/tmp/.ssh/id_ed25519_work" || log.lastArchivedPub != "/tmp/.ssh/id_ed25519_work.pub" {
		t.Errorf("Rotate archived the wrong paths: priv=%q pub=%q", log.lastArchivedPriv, log.lastArchivedPub)
	}
	if res.ArchivedPrivatePath == "" || res.ArchivedPublicPath == "" {
		t.Error("Rotate result must carry the archived paths")
	}
	if res.ProviderHost != "github" {
		t.Errorf("Rotate result ProviderHost = %q, want the account's Provider %q", res.ProviderHost, "github")
	}
	// All four artifacts re-pointed, via the APPEND writer, never the
	// replacing deps.WriteAllowedSigners.
	if log.writeSSH != 1 || log.writeGitconfig != 1 || log.writeFragment != 1 {
		t.Errorf("Rotate must re-point ssh/gitconfig/fragment once; got ssh=%d gitconfig=%d fragment=%d",
			log.writeSSH, log.writeGitconfig, log.writeFragment)
	}
	if log.appendAllowedSigners != 1 {
		t.Errorf("Rotate must APPEND the new signer line once; appended %d times", log.appendAllowedSigners)
	}
	if log.writeAllowedSigners != 0 {
		t.Errorf("Rotate must NOT use the replacing WriteAllowedSigners writer; called %d times", log.writeAllowedSigners)
	}
	if !strings.Contains(log.lastAppendPubLine, "AAAANEWKEY") {
		t.Errorf("Rotate appended the wrong public line: %q", log.lastAppendPubLine)
	}
	if log.lastAppendEmail != "work@example.com" {
		t.Errorf("Rotate appended with email %q, want the account's GitEmail", log.lastAppendEmail)
	}
	if log.resolved != 1 {
		t.Errorf("Rotate must re-run the resolved test once; ran %d", log.resolved)
	}
}

// TestRotateAbortsOnPreWriteFailure asserts rotation honors the pre-write
// gate: a Failure aborts BEFORE the archive step and before any artifact is
// touched (no half-rotated state).
func TestRotateAbortsOnPreWriteFailure(t *testing.T) {
	var log rotateLog
	deps := newFakeRotateDeps(&log, tester.Failure)
	deps.Generate = rotateGenerate(&log, "/tmp/stage/new", "/tmp/.ssh/new", "ssh-ed25519 AAAANEW c\n")

	if _, err := Rotate(rotateAccount(), deps); err == nil {
		t.Fatal("Rotate must error when the pre-write test fails")
	}
	if log.archiveKeyPair != 0 {
		t.Errorf("Rotate must NOT archive on a gate failure; archived %d times", log.archiveKeyPair)
	}
	if log.persistKey != 0 {
		t.Errorf("Rotate must NOT persist on a gate failure; persisted %d times", log.persistKey)
	}
	if log.writeSSH != 0 || log.writeGitconfig != 0 || log.writeFragment != 0 || log.appendAllowedSigners != 0 {
		t.Fatalf("Rotate must perform NO writes on pre-write Failure; got ssh=%d gitconfig=%d fragment=%d signers=%d",
			log.writeSSH, log.writeGitconfig, log.writeFragment, log.appendAllowedSigners)
	}
}

// TestRotatePersistKeyOnConfirm asserts Rotate records PersistKey count 1 on
// the confirmed (gate-passed) path and 0 on a gate-failure path.
func TestRotatePersistKeyOnConfirm(t *testing.T) {
	t.Run("confirmed", func(t *testing.T) {
		var log rotateLog
		deps := newFakeRotateDeps(&log, tester.ReachableNotUploaded)
		deps.Generate = rotateGenerate(&log, "/tmp/stage/rot", "/tmp/.ssh/id_ed25519_work", "ssh-ed25519 AAAAROTED c\n")

		if _, err := Rotate(rotateAccount(), deps); err != nil {
			t.Fatalf("Rotate returned error: %v", err)
		}
		if log.persistKey != 1 {
			t.Errorf("Rotate confirmed: PersistKey called %d times, want 1", log.persistKey)
		}
	})

	t.Run("gate-failure", func(t *testing.T) {
		var log rotateLog
		deps := newFakeRotateDeps(&log, tester.Failure)
		deps.Generate = rotateGenerate(&log, "/tmp/stage/rot", "/tmp/.ssh/id_ed25519_work", "ssh-ed25519 AAAAROTED c\n")

		if _, err := Rotate(rotateAccount(), deps); err == nil {
			t.Fatal("Rotate gate-failure must return an error")
		}
		if log.persistKey != 0 {
			t.Errorf("Rotate gate-failure: PersistKey called %d times, want 0", log.persistKey)
		}
	})
}

// TestRotateArchivesBeforePersist asserts the archive seam fires BEFORE the
// persist seam, using the order-recording Deps builder from Task 1
// (identity_test.go's newOrderRecordingDeps, extended with ArchiveKeyPair).
func TestRotateArchivesBeforePersist(t *testing.T) {
	var rec orderRecorder
	deps := newOrderRecordingDeps(&rec)

	if _, err := Rotate(rotateAccount(), deps); err != nil {
		t.Fatalf("Rotate returned error: %v", err)
	}

	assertOrder(t, "Rotate", rec.order, []string{
		"Generate", "CopyPub", "PreWrite",
		"ArchiveKeyPair", "PersistKey",
		"WriteSSH", "WriteGitconfig", "WriteFragment", "AppendAllowedSigners",
		"Resolved", "Cleanup",
	})
}

// TestRotateFailureAtPersistRestoresArchivePaths asserts a failure injected
// at the key-persist step returns a RotateResult carrying both archive
// paths (so the caller's journal can remove them) and that allowed_signers
// is left completely untouched.
func TestRotateFailureAtPersistRestoresArchivePaths(t *testing.T) {
	var log rotateLog
	deps := newFakeRotateDeps(&log, tester.ReachableNotUploaded)
	deps.Generate = rotateGenerate(&log, "/tmp/stage/rot", "/tmp/.ssh/id_ed25519_work", "ssh-ed25519 AAAAROTED c\n")
	deps.PersistKey = func(_ StagedKey) (KeyResult, error) {
		log.persistKey++
		return KeyResult{}, errors.New("disk full")
	}

	res, err := Rotate(rotateAccount(), deps)
	if err == nil {
		t.Fatal("Rotate must return an error when persist fails")
	}
	if res.ArchivedPrivatePath == "" || res.ArchivedPublicPath == "" {
		t.Errorf("RotateResult must carry both archive paths on a persist failure; got priv=%q pub=%q",
			res.ArchivedPrivatePath, res.ArchivedPublicPath)
	}
	if log.appendAllowedSigners != 0 || log.writeAllowedSigners != 0 {
		t.Errorf("allowed_signers must be untouched on a persist failure; append=%d write=%d",
			log.appendAllowedSigners, log.writeAllowedSigners)
	}
	if log.writeSSH != 0 || log.writeGitconfig != 0 || log.writeFragment != 0 {
		t.Errorf("no artifact writer may run on a persist failure; ssh=%d gitconfig=%d fragment=%d",
			log.writeSSH, log.writeGitconfig, log.writeFragment)
	}
}

// TestRotateArchiveStepFailureCarriesBothPaths is the step-3 half of review
// R3-01: an ArchiveKeyPair seam that returns BOTH paths TOGETHER WITH an
// error (the exact shape keygen.MoveKeyPairToArchive produces on a
// source-removal failure) must still surface both paths on the returned
// RotateResult — the case cycle 2's "step 4 or later" wording left
// uncovered.
func TestRotateArchiveStepFailureCarriesBothPaths(t *testing.T) {
	var log rotateLog
	deps := newFakeRotateDeps(&log, tester.ReachableNotUploaded)
	deps.Generate = rotateGenerate(&log, "/tmp/stage/rot", "/tmp/.ssh/id_ed25519_work", "ssh-ed25519 AAAAROTED c\n")
	log.archiveReturnPriv = "/tmp/.ssh/gitid-archive/id_ed25519_work.123"
	log.archiveReturnPub = "/tmp/.ssh/gitid-archive/id_ed25519_work.pub.123"
	log.archiveErr = fmt.Errorf("keygen: source(s) not removed after archiving (%s): %w",
		"/tmp/.ssh/id_ed25519_work.pub", errArchiveIncompleteForTest)

	res, err := Rotate(rotateAccount(), deps)
	if err == nil {
		t.Fatal("Rotate must return an error when the archive step itself fails")
	}
	if res.ArchivedPrivatePath != log.archiveReturnPriv || res.ArchivedPublicPath != log.archiveReturnPub {
		t.Errorf("RotateResult must carry the archive step's OWN returned paths even on its own error; got priv=%q pub=%q, want priv=%q pub=%q",
			res.ArchivedPrivatePath, res.ArchivedPublicPath, log.archiveReturnPriv, log.archiveReturnPub)
	}
	if log.persistKey != 0 {
		t.Errorf("Rotate must not persist after an archive-step failure; persisted %d times", log.persistKey)
	}
}

// errArchiveIncompleteForTest stands in for keygen.ErrArchiveIncomplete
// without importing internal/keygen into this domain-level test — the
// identity package must stay UI/infra-free and never import keygen's
// archive primitives directly; Rotate only ever sees them through Deps.
var errArchiveIncompleteForTest = errors.New("archive copies created but the source pair was not fully removed")

// TestRotateTwiceAccumulatesLinesAndArchives asserts rotating an identity
// TWICE yields three signer-line append calls (the pre-existing account
// already carries one, per this test's simulated state) and two DISTINCT
// archive-generation calls — the domain-level half of "no filename
// collision"; the composition root's real stamp-collision retry is tested
// at the wiring layer (cmd/gitid/wiring_test.go).
func TestRotateTwiceAccumulatesLinesAndArchives(t *testing.T) {
	var log rotateLog
	deps := newFakeRotateDeps(&log, tester.ReachableNotUploaded)

	var appendedLines []string
	deps.AppendAllowedSigners = func(_, _, _, pubLine string) (string, error) {
		log.appendAllowedSigners++
		appendedLines = append(appendedLines, pubLine)
		return "", nil
	}
	var archivedPaths []string
	gen := 0
	deps.ArchiveKeyPair = func(privPath, pubPath string) (string, string, error) {
		log.archiveKeyPair++
		gen++
		archivedPriv := fmt.Sprintf("%s.gen%d", privPath, gen)
		archivedPaths = append(archivedPaths, archivedPriv)
		return archivedPriv, pubPath + fmt.Sprintf(".gen%d", gen), nil
	}

	acct := rotateAccount()
	for i := 0; i < 2; i++ {
		deps.Generate = rotateGenerate(&log, "/tmp/stage/rot", acct.KeyPath, fmt.Sprintf("ssh-ed25519 AAAAGEN%d c\n", i))
		if _, err := Rotate(acct, deps); err != nil {
			t.Fatalf("Rotate #%d returned error: %v", i+1, err)
		}
	}

	if len(appendedLines) != 2 {
		t.Errorf("two rotations must append exactly 2 new lines; appended %d: %v", len(appendedLines), appendedLines)
	}
	if len(archivedPaths) != 2 || archivedPaths[0] == archivedPaths[1] {
		t.Errorf("two rotations must produce 2 DISTINCT archived paths; got %v", archivedPaths)
	}
}

// TestRotateInputPreservesAlgorithm asserts rotateInput derives Algo from
// the account's existing KeyPath (via algoFromKeyPath) so the real
// composition root's deps.Generate recomputes the IDENTICAL canonical path
// rather than an empty/wrong one (D-08).
func TestRotateInputPreservesAlgorithm(t *testing.T) {
	acct := rotateAccount() // KeyPath = /tmp/.ssh/id_ed25519_work
	in := rotateInput(acct)
	if in.Algo != "ed25519" {
		t.Errorf("rotateInput(acct).Algo = %q, want %q", in.Algo, "ed25519")
	}
}

// TestAlgoFromKeyPath is a table test for the naming-convention parser,
// including an identity name containing an underscore and an algorithm
// segment containing a hyphen (rsa-4096) — both of which a naive
// strings.Split on "_" would mis-parse.
func TestAlgoFromKeyPath(t *testing.T) {
	cases := []struct {
		keyPath, name, want string
	}{
		{"/tmp/.ssh/id_ed25519_work", "work", "ed25519"},
		{"/tmp/.ssh/id_rsa-4096_work", "work", "rsa-4096"},
		{"/tmp/.ssh/id_ed25519_my_client_name", "my_client_name", "ed25519"},
	}
	for _, tc := range cases {
		if got := algoFromKeyPath(tc.keyPath, tc.name); got != tc.want {
			t.Errorf("algoFromKeyPath(%q, %q) = %q, want %q", tc.keyPath, tc.name, got, tc.want)
		}
	}
}
