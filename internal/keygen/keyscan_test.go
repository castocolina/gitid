package keygen

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

// writeFixture writes a fixture file into dir with test-only permissions.
func writeFixture(t *testing.T, dir, name string, data []byte, perm os.FileMode) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, perm); err != nil { //nolint:gosec // test-controlled temp path
		t.Fatalf("writing fixture %s: %v", name, err)
	}
	return path
}

// seedEd25519 writes an ed25519 private key (optionally passphrase-encrypted)
// named privName into dir, and its `.pub` sibling when withPub is true. It
// returns the generated authorized-key line.
func seedEd25519(t *testing.T, dir, privName, passphrase string, withPub bool) string {
	t.Helper()
	mat, err := GenerateMaterial(Params{
		Algo:       "ed25519",
		Identity:   privName,
		Comment:    privName + "@gitid",
		Passphrase: passphrase,
	})
	if err != nil {
		t.Fatalf("GenerateMaterial(%s): %v", privName, err)
	}
	writeFixture(t, dir, privName, mat.PrivPEM, 0o600)
	if withPub {
		writeFixture(t, dir, privName+".pub", []byte(mat.PubLine), 0o644)
	}
	return mat.PubLine
}

// seedRSA writes a 2048-bit RSA private key (no `.pub` sibling) named privName
// into dir. 2048 keeps the test fast; gitid's own generate path is 4096-only —
// this fixture exists to prove the scan is algorithm-agnostic (D-13), not to
// endorse a key size.
func seedRSA(t *testing.T, dir, privName string) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating rsa fixture: %v", err)
	}
	block, err := ssh.MarshalPrivateKey(priv, privName+"@gitid")
	if err != nil {
		t.Fatalf("serializing rsa fixture: %v", err)
	}
	writeFixture(t, dir, privName, pem.EncodeToMemory(block), 0o600)
}

// fingerprintOfPubLine independently computes the SHA256 fingerprint of an
// authorized-key line, so the scan's fingerprint is compared against a value
// derived OUTSIDE the code under test.
func fingerprintOfPubLine(t *testing.T, pubLine string) (algorithm, fingerprint string) {
	t.Helper()
	pub, _, _, _, err := ssh.ParseAuthorizedKey([]byte(pubLine))
	if err != nil {
		t.Fatalf("parsing fixture pub line: %v", err)
	}
	return pub.Type(), ssh.FingerprintSHA256(pub)
}

// fakeSSHKeygenRunner returns a runner that emits the pre-computed SHA256
// fingerprint for known paths, as if `ssh-keygen -E sha256 -lf <path>` had run.
// Paths not in the map error, which lets tests assert unverifiable-key handling.
func fakeSSHKeygenRunner(fps map[string]string) sshKeygenRunner {
	return func(args []string) (string, error) {
		if len(args) == 0 || args[len(args)-1] == "" {
			return "", fmt.Errorf("fake runner: missing path")
		}
		path := args[len(args)-1]
		fp, ok := fps[path]
		if !ok {
			return "", fmt.Errorf("fake runner: no fingerprint for %q (known: %v)", path, fps)
		}
		return fmt.Sprintf("256 %s comment (ED25519)\n", fp), nil
	}
}

// byPath indexes a scan result by key path for assertion convenience.
func byPath(keys []ReusableKey) map[string]ReusableKey {
	m := make(map[string]ReusableKey, len(keys))
	for _, k := range keys {
		m[filepath.Base(k.Path)] = k
	}
	return m
}

// TestScanReusableKeys covers the D-10 picker contract: every parseable private
// key under the scanned dir is offered with its algorithm and SHA256
// fingerprint; encrypted keys are flagged (never passphrase-prompted, D-11) and
// are ONLY offered when their public half can be verified on disk and matches
// the private key non-interactively; unparseable and non-key files are skipped
// without aborting the scan (D-13).
func TestScanReusableKeys(t *testing.T) {
	// Swap in a fake ssh-keygen runner so the encrypted-key proof runs without
	// requiring the real binary in every test environment.
	oldRunner := currentSSHKeygenRunner
	defer func() { currentSSHKeygenRunner = oldRunner }()

	dir := t.TempDir()

	plainPub := seedEd25519(t, dir, "id_ed25519_plain", "", true)
	seedRSA(t, dir, "id_rsa_nopub")
	encWithPub := seedEd25519(t, dir, "id_ed25519_encpub", "s3cret", true)
	seedEd25519(t, dir, "id_ed25519_encnopub", "s3cret", false)

	// Noise the scan must tolerate: garbage that looks like a key candidate, an
	// orphan `.pub`, a non-candidate filename, and a directory.
	writeFixture(t, dir, "id_garbage", []byte("this is not a private key\n"), 0o600)
	writeFixture(t, dir, "id_ed25519_orphan.pub", []byte(plainPub), 0o644)
	writeFixture(t, dir, "known_hosts", []byte("github.com ssh-ed25519 AAAA\n"), 0o644)
	if err := os.Mkdir(filepath.Join(dir, "id_directory"), 0o700); err != nil {
		t.Fatalf("creating directory fixture: %v", err)
	}

	_, wantEncFP := fingerprintOfPubLine(t, encWithPub)
	currentSSHKeygenRunner = fakeSSHKeygenRunner(map[string]string{
		filepath.Join(dir, "id_ed25519_encpub"): wantEncFP,
	})

	keys, err := ScanReusableKeys(dir)
	if err != nil {
		t.Fatalf("ScanReusableKeys returned error: %v", err)
	}
	got := byPath(keys)

	// encrypted-nopub is now skipped (unverifiable pair), so 3 entries remain.
	if len(keys) != 3 {
		for _, k := range keys {
			t.Logf("entry: path=%s algo=%q fp=%q enc=%v haspub=%v err=%v", filepath.Base(k.Path), k.Algorithm, k.Fingerprint, k.Encrypted, k.HasPub, k.ParseError)
		}
		t.Fatalf("ScanReusableKeys returned %d entries, want 3 (plain, rsa, enc+pub); got %v",
			len(keys), keyNames(keys))
	}
	for _, skipped := range []string{"id_garbage", "id_ed25519_orphan", "id_ed25519_orphan.pub", "id_ed25519_encnopub", "known_hosts", "id_directory"} {
		if _, ok := got[skipped]; ok {
			t.Errorf("ScanReusableKeys must skip %q", skipped)
		}
	}

	t.Run("plain key carries algorithm + fingerprint", func(t *testing.T) {
		k := got["id_ed25519_plain"]
		wantAlgo, wantFP := fingerprintOfPubLine(t, plainPub)
		if k.Algorithm != wantAlgo {
			t.Errorf("Algorithm = %q, want %q", k.Algorithm, wantAlgo)
		}
		if k.Fingerprint != wantFP {
			t.Errorf("Fingerprint = %q, want %q", k.Fingerprint, wantFP)
		}
		if !k.HasPub || k.Encrypted || k.ParseError != nil {
			t.Errorf("plain key flags: HasPub=%v Encrypted=%v ParseError=%v; want true/false/nil",
				k.HasPub, k.Encrypted, k.ParseError)
		}
	})

	t.Run("non-ed25519 key without .pub is still offered", func(t *testing.T) {
		k := got["id_rsa_nopub"]
		if k.Algorithm != ssh.KeyAlgoRSA {
			t.Errorf("Algorithm = %q, want %q", k.Algorithm, ssh.KeyAlgoRSA)
		}
		if !strings.HasPrefix(k.Fingerprint, "SHA256:") {
			t.Errorf("Fingerprint = %q, want a SHA256: fingerprint", k.Fingerprint)
		}
		if k.HasPub || k.Encrypted {
			t.Errorf("rsa fixture flags: HasPub=%v Encrypted=%v; want false/false", k.HasPub, k.Encrypted)
		}
	})

	t.Run("encrypted key with .pub takes metadata from the .pub after fingerprint match", func(t *testing.T) {
		k := got["id_ed25519_encpub"]
		wantAlgo, wantFP := fingerprintOfPubLine(t, encWithPub)
		if !k.Encrypted || !k.HasPub {
			t.Fatalf("encrypted+pub flags: Encrypted=%v HasPub=%v; want true/true", k.Encrypted, k.HasPub)
		}
		if k.Algorithm != wantAlgo {
			t.Errorf("Algorithm = %q, want %q (derived from the .pub sibling)", k.Algorithm, wantAlgo)
		}
		if k.Fingerprint != wantFP {
			t.Errorf("Fingerprint = %q, want %q (derived from the .pub sibling)", k.Fingerprint, wantFP)
		}
	})
}

// TestScanReusableKeysEncryptedWithUnparseablePub asserts an encrypted key whose
// `.pub` sibling is garbage is skipped — the pair is unverifiable, so it is
// blocked rather than offered with untrusted metadata.
func TestScanReusableKeysEncryptedWithUnparseablePub(t *testing.T) {
	dir := t.TempDir()
	seedEd25519(t, dir, "id_ed25519_badpub", "s3cret", false)
	writeFixture(t, dir, "id_ed25519_badpub.pub", []byte("not an authorized key line\n"), 0o644)

	keys, err := ScanReusableKeys(dir)
	if err != nil {
		t.Fatalf("ScanReusableKeys returned error: %v", err)
	}
	if len(keys) != 0 {
		t.Fatalf("want 0 entries, got %d (%v)", len(keys), keyNames(keys))
	}
}

// TestScanReusableKeysEncryptedMismatchBlocksReuse asserts that an encrypted
// private key whose `.pub` sibling belongs to a DIFFERENT key is skipped, so a
// mismatched sibling cannot mask itself (CR-08).
func TestScanReusableKeysEncryptedMismatchBlocksReuse(t *testing.T) {
	oldRunner := currentSSHKeygenRunner
	defer func() { currentSSHKeygenRunner = oldRunner }()

	dir := t.TempDir()
	// encPriv has its own .pub, but we will also create a second .pub that does
	// NOT match and overwrite the sibling so the pair is mismatched.
	encPubLine := seedEd25519(t, dir, "id_ed25519_mismatch", "s3cret", true)
	otherPubLine := seedEd25519(t, dir, "id_ed25519_other", "", true)
	if encPubLine == otherPubLine {
		t.Fatal("setup: expected two distinct pub lines")
	}

	// Replace the sibling with the OTHER key's .pub.
	writeFixture(t, dir, "id_ed25519_mismatch.pub", []byte(otherPubLine), 0o644)

	// The fake runner reports the encrypted private key's REAL fingerprint.
	_, privFP := fingerprintOfPubLine(t, encPubLine)
	currentSSHKeygenRunner = fakeSSHKeygenRunner(map[string]string{
		filepath.Join(dir, "id_ed25519_mismatch"): privFP,
	})

	keys, err := ScanReusableKeys(dir)
	if err != nil {
		t.Fatalf("ScanReusableKeys returned error: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("want 1 entry (the correctly matched other key), got %d (%v)", len(keys), keyNames(keys))
	}
	if filepath.Base(keys[0].Path) != "id_ed25519_other" {
		t.Errorf("expected id_ed25519_other, got %q", filepath.Base(keys[0].Path))
	}
}

// TestScanReusableKeysPlainMismatchBlocksReuse asserts that an unencrypted
// private key whose `.pub` sibling belongs to a DIFFERENT key is skipped.
func TestScanReusableKeysPlainMismatchBlocksReuse(t *testing.T) {
	dir := t.TempDir()
	seedEd25519(t, dir, "id_ed25519_plain", "", true)
	otherPubLine := seedEd25519(t, dir, "id_ed25519_other", "", true)
	// Replace the sibling with the OTHER key's .pub.
	writeFixture(t, dir, "id_ed25519_plain.pub", []byte(otherPubLine), 0o644)

	keys, err := ScanReusableKeys(dir)
	if err != nil {
		t.Fatalf("ScanReusableKeys returned error: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("want 1 entry (the correctly matched other key), got %d (%v)", len(keys), keyNames(keys))
	}
	if filepath.Base(keys[0].Path) != "id_ed25519_other" {
		t.Errorf("expected id_ed25519_other, got %q", filepath.Base(keys[0].Path))
	}
}

// TestScanReusableKeysMissingDir asserts a non-existent ssh dir is "nothing to
// reuse" (empty slice, nil error) — the first-run case, not a failure.
func TestScanReusableKeysMissingDir(t *testing.T) {
	keys, err := ScanReusableKeys(filepath.Join(t.TempDir(), "no-such-dir"))
	if err != nil {
		t.Fatalf("a missing ssh dir must not error; got %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("a missing ssh dir must yield no keys; got %v", keyNames(keys))
	}
}

// TestScanReusableKeysIsDeterministic asserts the scan order is stable so the
// picker never reshuffles rows between renders.
func TestScanReusableKeysIsDeterministic(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"id_ed25519_c", "id_ed25519_a", "id_ed25519_b"} {
		seedEd25519(t, dir, name, "", true)
	}
	first, err := ScanReusableKeys(dir)
	if err != nil {
		t.Fatalf("ScanReusableKeys returned error: %v", err)
	}
	second, err := ScanReusableKeys(dir)
	if err != nil {
		t.Fatalf("ScanReusableKeys returned error: %v", err)
	}
	if !reflect.DeepEqual(keyNames(first), keyNames(second)) {
		t.Fatalf("scan order is not stable: %v vs %v", keyNames(first), keyNames(second))
	}
	want := []string{"id_ed25519_a", "id_ed25519_b", "id_ed25519_c"}
	if !reflect.DeepEqual(keyNames(first), want) {
		t.Errorf("scan order = %v, want sorted %v", keyNames(first), want)
	}
}

// TestReusableKeyCarriesNoPrivateMaterial is the T-03-01 guard: the picker DTO
// exposes paths, labels and flags only. A []byte field (or any new field) would
// be a channel for private-key bytes to reach the UI.
func TestReusableKeyCarriesNoPrivateMaterial(t *testing.T) {
	want := map[string]string{
		"Path":        "string",
		"Algorithm":   "string",
		"Fingerprint": "string",
		"HasPub":      "bool",
		"Encrypted":   "bool",
		"ParseError":  "error",
	}
	rt := reflect.TypeOf(ReusableKey{})
	if rt.NumField() != len(want) {
		t.Fatalf("ReusableKey has %d fields, want exactly %d — new fields must be reviewed for key-material leakage",
			rt.NumField(), len(want))
	}
	for i := range rt.NumField() {
		f := rt.Field(i)
		wantKind, ok := want[f.Name]
		if !ok {
			t.Errorf("unexpected ReusableKey field %q — review it for key-material leakage", f.Name)
			continue
		}
		if got := f.Type.String(); got != wantKind {
			t.Errorf("ReusableKey.%s is %s, want %s", f.Name, got, wantKind)
		}
	}
}

// keyNames renders the base filenames of a scan result for assertion messages.
func keyNames(keys []ReusableKey) []string {
	names := make([]string, 0, len(keys))
	for _, k := range keys {
		names = append(names, filepath.Base(k.Path))
	}
	return names
}

// TestScanManualKeyRejectsSymlink is the T-03-13 guard: the reuse picker's
// manual-path row must reject a symlinked candidate BEFORE parsing it,
// os.Lstat rather than os.Stat, unlike the directory scan (which legitimately
// follows symlinks inside the user's own ~/.ssh).
func TestScanManualKeyRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	seedEd25519(t, dir, "id_ed25519_real", "", true)
	link := filepath.Join(dir, "id_ed25519_link")
	if err := os.Symlink(filepath.Join(dir, "id_ed25519_real"), link); err != nil {
		t.Fatalf("seeding symlink fixture: %v", err)
	}

	_, err := ScanManualKey(link)
	if err == nil {
		t.Fatal("ScanManualKey on a symlinked candidate = nil error, want a rejection (T-03-13)")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("error = %v, want it to name the symlink rejection", err)
	}
}

// TestScanManualKeyAcceptsRegularFile proves a non-symlinked manual candidate
// is scanned with the SAME D-11/D-13 tolerance as the directory scan.
func TestScanManualKeyAcceptsRegularFile(t *testing.T) {
	dir := t.TempDir()
	pubLine := seedEd25519(t, dir, "id_ed25519_manual", "", true)
	wantAlgo, wantFP := fingerprintOfPubLine(t, pubLine)

	key, err := ScanManualKey(filepath.Join(dir, "id_ed25519_manual"))
	if err != nil {
		t.Fatalf("ScanManualKey: %v", err)
	}
	if key.Algorithm != wantAlgo || key.Fingerprint != wantFP {
		t.Errorf("ScanManualKey = {Algorithm:%q Fingerprint:%q}, want {%q %q}",
			key.Algorithm, key.Fingerprint, wantAlgo, wantFP)
	}
}

// TestScanManualKeyMissingPath proves a non-existent manual path errors
// (never silently offered) instead of the empty-slice "nothing to reuse"
// convention ScanReusableKeys uses for a missing directory — a manual pick is
// a single explicit candidate, so its absence must be reported.
func TestScanManualKeyMissingPath(t *testing.T) {
	_, err := ScanManualKey(filepath.Join(t.TempDir(), "does-not-exist"))
	if err == nil {
		t.Fatal("ScanManualKey on a missing path = nil error, want an error")
	}
}

// TestScanManualKeyRejectsUnparseable proves a garbage manual candidate errors
// rather than being silently dropped (D-13 applies to what is OFFERED; a
// manual pick that fails to parse must tell the user why).
func TestScanManualKeyRejectsUnparseable(t *testing.T) {
	dir := t.TempDir()
	path := writeFixture(t, dir, "not-a-key", []byte("definitely not a key\n"), 0o600)

	_, err := ScanManualKey(path)
	if err == nil {
		t.Fatal("ScanManualKey on an unparseable candidate = nil error, want an error")
	}
}
