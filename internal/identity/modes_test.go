package identity

import (
	"errors"
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
	derivePub       int
	pubExists       int
	writePub        int
	readPub         int
	pubExistsRet    bool
	lastPubLine     string
	lastPubPath     string
	lastReadPubPath string
}

func newFakeModeDeps(log *modeLog, preOutcome tester.Outcome) Deps {
	d := newFakeDeps(&log.callLog, preOutcome)
	d.PubExists = func(_ string) bool {
		log.pubExists++
		return log.pubExistsRet
	}
	d.DerivePub = func(_, _ string) (string, error) {
		log.derivePub++
		return "ssh-ed25519 AAAADERIVED comment\n", nil
	}
	d.WritePub = func(pubPath, pubLine string) error {
		log.writePub++
		log.lastPubPath = pubPath
		log.lastPubLine = pubLine
		return nil
	}
	// ReadPub is deliberately left NIL here: this fake models a caller that has
	// not wired the new seam, so every test built on it also exercises the L2
	// nil-guard fallback. Tests that need the seam wire it explicitly (see
	// withReadPub).
	return d
}

// withReadPub wires the ReadPub seam on deps, recording the call and returning
// the supplied line/error. It models a composition root that HAS wired the new
// seam (the real wiring lands in cmd/gitid, plan 03-03).
func withReadPub(deps Deps, log *modeLog, line string, err error) Deps {
	deps.ReadPub = func(pubPath string) (string, error) {
		log.readPub++
		log.lastReadPubPath = pubPath
		return line, err
	}
	return deps
}

// errEncryptedKey is the shape ssh.ParsePrivateKey (via keygen.DerivePublicKey)
// returns for a passphrase-protected private key — the exact failure D-11 says
// must NOT block reuse when a matching .pub sits next to the key.
var errEncryptedKey = errors.New("keygen: parsing private key: ssh: this private key is passphrase protected")

// failingDerivePub replaces DerivePub with one that always errors, simulating an
// ENCRYPTED private key (ssh.ParsePrivateKey cannot parse it without a
// passphrase). Any code path that reaches DerivePub therefore fails loudly.
func failingDerivePub(deps Deps, log *modeLog) Deps {
	deps.DerivePub = func(_, _ string) (string, error) {
		log.derivePub++
		return "", errEncryptedKey
	}
	return deps
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

// TestReuseEncryptedKeyWithExistingPub asserts the D-11 / KEY-06 contract: an
// ENCRYPTED private key that already has a matching `.pub` sibling is reusable
// WITHOUT any passphrase prompt. DerivePub is stubbed to fail the way
// ssh.ParsePrivateKey fails on a passphrase-protected key, so the test passes
// only when ensurePub reads the existing `.pub` via the ReadPub seam and never
// touches the private key.
func TestReuseEncryptedKeyWithExistingPub(t *testing.T) {
	var log modeLog
	log.pubExistsRet = true // the .pub sits next to the encrypted private key
	const existingLine = "ssh-ed25519 AAAAEXISTINGPUB encrypted@gitid\n"

	deps := failingDerivePub(newFakeModeDeps(&log, tester.ReachableNotUploaded), &log)
	deps = withReadPub(deps, &log, existingLine, nil)

	existingKey := "/tmp/.ssh/id_ed25519_encrypted"
	res, err := Reuse(reuseInput(), existingKey, deps)
	if err != nil {
		t.Fatalf("Reuse of an encrypted key with an existing .pub must succeed; got error: %v", err)
	}
	if log.derivePub != 0 {
		t.Errorf("Reuse must NOT parse the encrypted private key; DerivePub called %d times", log.derivePub)
	}
	if log.readPub != 1 {
		t.Errorf("Reuse must read the existing .pub once; ReadPub called %d times", log.readPub)
	}
	if log.lastReadPubPath != existingKey+".pub" {
		t.Errorf("ReadPub called with %q, want %q", log.lastReadPubPath, existingKey+".pub")
	}
	if log.writePub != 0 {
		t.Errorf("Reuse must not rewrite an existing .pub; wrote %d times", log.writePub)
	}
	if res.Key.PubLine != existingLine {
		t.Errorf("Reuse PubLine = %q, want the existing .pub line %q", res.Key.PubLine, existingLine)
	}
	// The allowed_signers line is built from the SAME existing public line, so
	// the signing artifact stays consistent with the key actually on disk.
	if !strings.Contains(res.AllowedSignersLine, "AAAAEXISTINGPUB") {
		t.Errorf("allowed_signers line must carry the existing public key; got %q", res.AllowedSignersLine)
	}
}

// TestEnsurePubNilReadPubFallsBackToDerivePub is the L2 nil-guard obligation: a
// caller that has NOT wired the new ReadPub seam must keep today's behavior
// (derive from the private key) instead of panicking on a nil function value.
func TestEnsurePubNilReadPubFallsBackToDerivePub(t *testing.T) {
	var log modeLog
	log.pubExistsRet = true
	deps := newFakeModeDeps(&log, tester.ReachableNotUploaded)
	if deps.ReadPub != nil {
		t.Fatal("test setup: this fake must leave ReadPub nil to exercise the nil-guard")
	}

	line, err := ensurePubReadOnly("/tmp/.ssh/id_ed25519_x", "/tmp/.ssh/id_ed25519_x.pub", "x@gitid", deps)
	if err != nil {
		t.Fatalf("ensurePubReadOnly with a nil ReadPub must fall back to DerivePub; got error: %v", err)
	}
	if log.derivePub != 1 {
		t.Errorf("nil ReadPub must fall back to DerivePub once; called %d times", log.derivePub)
	}
	if log.readPub != 0 {
		t.Errorf("a nil ReadPub must never be invoked; called %d times", log.readPub)
	}
	if log.writePub != 0 {
		t.Errorf("ensurePubReadOnly must not write an already-present .pub; wrote %d times", log.writePub)
	}
	if !strings.Contains(line, "AAAADERIVED") {
		t.Errorf("nil-ReadPub fallback returned %q, want the DerivePub line", line)
	}
}

// TestEnsurePubReadPubErrorIsWrapped asserts a ReadPub failure propagates with
// the package-prefixed wrapping convention used throughout modes.go, naming the
// path that could not be read.
func TestEnsurePubReadPubErrorIsWrapped(t *testing.T) {
	var log modeLog
	log.pubExistsRet = true
	readErr := errors.New("permission denied")
	deps := withReadPub(newFakeModeDeps(&log, tester.ReachableNotUploaded), &log, "", readErr)

	pubPath := "/tmp/.ssh/id_ed25519_x.pub"
	_, err := ensurePubReadOnly("/tmp/.ssh/id_ed25519_x", pubPath, "x@gitid", deps)
	if err == nil {
		t.Fatal("ensurePubReadOnly must return an error when ReadPub fails")
	}
	if !errors.Is(err, readErr) {
		t.Errorf("ensurePubReadOnly must wrap the ReadPub error with %%w; got %v", err)
	}
	if !strings.Contains(err.Error(), "identity: reading existing public key") ||
		!strings.Contains(err.Error(), pubPath) {
		t.Errorf("ensurePubReadOnly error must follow the identity: <verb> <path> convention; got %q", err.Error())
	}
	if log.derivePub != 0 {
		t.Errorf("a ReadPub failure must not silently fall back to DerivePub; called %d times", log.derivePub)
	}
}

// TestEnsurePubMissingPubStillDerives asserts the .pub-ABSENT branch is
// read-only in staging: ReadPub is never consulted, DerivePub produces the
// line, but WritePub is NOT called. The confirmed transaction writes the .pub
// later (CR-02).
func TestEnsurePubMissingPubStillDerives(t *testing.T) {
	var log modeLog
	log.pubExistsRet = false // .pub absent
	deps := withReadPub(newFakeModeDeps(&log, tester.ReachableNotUploaded), &log, "ssh-ed25519 AAAASTALE x\n", nil)

	pubPath := "/tmp/.ssh/id_ed25519_x.pub"
	line, err := ensurePubReadOnly("/tmp/.ssh/id_ed25519_x", pubPath, "x@gitid", deps)
	if err != nil {
		t.Fatalf("ensurePubReadOnly returned error: %v", err)
	}
	if log.readPub != 0 {
		t.Errorf("ReadPub must not be called when the .pub is absent; called %d times", log.readPub)
	}
	if log.derivePub != 1 {
		t.Errorf("absent .pub must derive once; derive=%d", log.derivePub)
	}
	if log.writePub != 0 {
		t.Errorf("staging must not write the derived .pub; wrote %d times", log.writePub)
	}
	if !strings.Contains(line, "AAAADERIVED") {
		t.Errorf("absent .pub derivation returned unexpected line: %q", line)
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

// TestCallOrderReuse pins Reuse's exact seam invocation sequence (review
// R-01): the staging half (PubExists, then the ReadPub-nil-guard fallback to
// DerivePub, then a second PubExists check inside writeReusePub — the .pub
// already exists so WritePub is never reached), followed by runPipeline's
// four phases (PersistKey never appears: PrivPEM is nil for a reuse).
func TestCallOrderReuse(t *testing.T) {
	var rec orderRecorder
	deps := newOrderRecordingDeps(&rec)

	if _, err := Reuse(reuseInput(), "/tmp/.ssh/id_ed25519_existing", deps); err != nil {
		t.Fatalf("Reuse returned error: %v", err)
	}

	assertOrder(t, "Reuse", rec.order, []string{
		"PubExists", "DerivePub", "PubExists",
		"CopyPub", "PreWrite",
		"WriteSSH", "WriteGitconfig", "WriteFragment", "WriteAllowedSigners",
		"Resolved",
	})
}

// TestCallOrderAddAccount pins AddAccount's exact seam invocation sequence
// (review R-01): DerivePub once (deriving the shared key's public line, no
// Generate call), then runPipeline's four phases (PersistKey never appears:
// PrivPEM is nil — no new key material for a shared-key second account).
func TestCallOrderAddAccount(t *testing.T) {
	var rec orderRecorder
	deps := newOrderRecordingDeps(&rec)

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

	if _, err := AddAccount(existing, "gitlab", "work.gitlab.com", deps); err != nil {
		t.Fatalf("AddAccount returned error: %v", err)
	}

	assertOrder(t, "AddAccount", rec.order, []string{
		"DerivePub",
		"CopyPub", "PreWrite",
		"WriteSSH", "WriteGitconfig", "WriteFragment", "WriteAllowedSigners",
		"Resolved",
	})
}
