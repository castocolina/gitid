package keygen

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"golang.org/x/crypto/ssh"
)

// keyCandidateGlob is the private-key filename convention scanned for reuse
// candidates, matching keygen.KeyPaths' `id_<algo>_<identity>` output and the
// OpenSSH default names (`id_ed25519`, `id_rsa`, …) — the same glob
// identity.listKeyFilesReal uses to enumerate managed keys. Keys stored under
// another name or another directory are reachable through the picker's
// manual-path row (D-10), not through this scan.
const keyCandidateGlob = "id_*"

// ReusableKey describes one private key offered by the key-reuse picker (D-10):
// its path, the algorithm and SHA256 fingerprint to display, and the flags the
// UI needs to label the row.
//
// It deliberately carries NO key material — no private-key bytes, no PEM, no
// passphrase (T-03-01). Only the path, display labels and flags leave this
// package, so nothing here can leak into a log, a preview or a command string.
type ReusableKey struct {
	// Path is the absolute-or-caller-relative path of the private key file.
	Path string
	// Algorithm is the OpenSSH key-type token ("ssh-ed25519", "ssh-rsa",
	// "ecdsa-sha2-nistp256", …) — the same vocabulary as AlgoInfo.QueryToken.
	// It is empty when the key is encrypted and no usable `.pub` sibling exists.
	Algorithm string
	// Fingerprint is the "SHA256:…" fingerprint of the PUBLIC key, as printed by
	// `ssh-keygen -lf`. Empty under the same condition as Algorithm.
	Fingerprint string
	// HasPub reports whether a `<key>.pub` sibling exists on disk.
	HasPub bool
	// Encrypted reports that the private key is passphrase-protected: it could
	// not be parsed without a passphrase, and gitid never prompts for one
	// (D-11). Such a key is still offered — reuse only needs its public half.
	Encrypted bool
	// ParseError records a non-fatal diagnostic for an entry that is still
	// offered (e.g. an encrypted key whose `.pub` sibling is unreadable or
	// unparseable). It is informational; the entry is usable regardless.
	ParseError error
}

// ScanReusableKeys enumerates the private keys in sshDir and returns one
// ReusableKey per candidate, ordered by path so the picker's rows never
// reshuffle between renders (D-10: "filename + algorithm + fingerprint").
//
// Parsing is pure and in-memory: files are read and handed to
// golang.org/x/crypto/ssh. Nothing is executed, no passphrase is ever requested,
// and no private-key bytes are retained or returned (T-03-01, D-11).
//
// Tolerance rules (D-13 — any parseable key is offered, nothing blocks the
// picker):
//
//   - unencrypted, parseable → Algorithm and Fingerprint come from the private
//     key's own public half (authoritative).
//   - encrypted WITH a `.pub` sibling → Encrypted=true and the metadata comes
//     FROM THE `.pub` (the private key cannot be parsed without a passphrase,
//     but its public half is right there — the same premise that lets
//     identity.ensurePub reuse an existing `.pub`).
//   - encrypted WITHOUT a usable `.pub` → Encrypted=true, metadata empty; the
//     row is still offered.
//   - anything else that fails to parse (garbage, certificates, truncated
//     files) → skipped, never returned, never fatal.
//
// A missing sshDir is "nothing to reuse": empty slice, nil error. Only a
// genuine enumeration failure returns an error.
func ScanReusableKeys(sshDir string) ([]ReusableKey, error) {
	matches, err := filepath.Glob(filepath.Join(sshDir, keyCandidateGlob))
	if err != nil {
		return nil, fmt.Errorf("keygen: globbing ssh key files in %s: %w", sshDir, err)
	}
	sort.Strings(matches)

	keys := make([]ReusableKey, 0, len(matches))
	for _, path := range matches {
		key, ok := scanKeyCandidate(path)
		if !ok {
			continue
		}
		keys = append(keys, key)
	}
	return keys, nil
}

// scanKeyCandidate inspects one candidate path and reports whether it is a key
// the picker should offer. It returns ok=false for `.pub` siblings, non-regular
// files, unreadable files, and private keys that fail to parse for any reason
// other than a missing passphrase — a single bad file never aborts the scan.
func scanKeyCandidate(path string) (ReusableKey, bool) {
	if filepath.Ext(path) == ".pub" {
		return ReusableKey{}, false
	}
	// Stat (not Lstat): a symlink to a real key file is a legitimate candidate,
	// but directories, sockets and devices are not.
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return ReusableKey{}, false
	}

	privBytes, err := os.ReadFile(path) //nolint:gosec // candidate path enumerated from the caller-supplied ssh dir; contents are parsed in memory and never returned
	if err != nil {
		return ReusableKey{}, false
	}

	pubPath := path + ".pub"
	key := ReusableKey{Path: path, HasPub: fileExists(pubPath)}

	signer, err := ssh.ParsePrivateKey(privBytes)
	switch {
	case err == nil:
		pub := signer.PublicKey()
		key.Algorithm = pub.Type()
		key.Fingerprint = ssh.FingerprintSHA256(pub)
		return key, true

	case isPassphraseMissing(err):
		// D-11: encrypted keys are accepted, never passphrase-prompted. Take the
		// display metadata from the public half when it is available on disk.
		key.Encrypted = true
		if key.HasPub {
			algo, fp, pubErr := pubMetadata(pubPath)
			if pubErr != nil {
				key.ParseError = pubErr
			}
			key.Algorithm, key.Fingerprint = algo, fp
		}
		return key, true

	default:
		// Not a usable private key (garbage, certificate, truncated, …): record
		// the reason and skip the entry without aborting the scan (D-13).
		key.ParseError = fmt.Errorf("keygen: parsing private key %s: %w", path, err)
		return key, false
	}
}

// pubMetadata reads an authorized-key line from pubPath and returns its
// algorithm token and SHA256 fingerprint. An unreadable or unparseable `.pub`
// yields empty metadata plus a wrapped error — never a fatal scan failure.
func pubMetadata(pubPath string) (algorithm, fingerprint string, err error) {
	pubBytes, err := os.ReadFile(pubPath) //nolint:gosec // public-key sibling of an enumerated candidate; public material only
	if err != nil {
		return "", "", fmt.Errorf("keygen: reading public key %s: %w", pubPath, err)
	}
	pub, _, _, _, err := ssh.ParseAuthorizedKey(pubBytes)
	if err != nil {
		return "", "", fmt.Errorf("keygen: parsing public key %s: %w", pubPath, err)
	}
	return pub.Type(), ssh.FingerprintSHA256(pub), nil
}

// isPassphraseMissing reports whether err is x/crypto/ssh's
// "key is passphrase protected" signal — the one parse failure that means the
// key is valid but locked, rather than unusable.
func isPassphraseMissing(err error) bool {
	var missing *ssh.PassphraseMissingError
	return errors.As(err, &missing)
}

// fileExists reports whether path names an existing file, following symlinks.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ScanManualKey inspects EXACTLY the path a user names on the reuse picker's
// manual-path row (D-10). Unlike ScanReusableKeys' directory scan — which
// legitimately follows a symlink sitting inside the user's own ~/.ssh — a
// manually-typed path is untrusted input that can point anywhere on disk, so
// a symlinked candidate is REJECTED before parsing (os.Lstat, never
// followed) rather than silently resolved (T-03-13, RESEARCH Security
// Domain). A non-symlink candidate is otherwise scanned with the same D-11/
// D-13 tolerance rules ScanReusableKeys applies.
func ScanManualKey(path string) (ReusableKey, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return ReusableKey{}, fmt.Errorf("keygen: %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return ReusableKey{}, fmt.Errorf("keygen: %s is a symlink — the manual-path row rejects symlinked keys", path)
	}
	if !info.Mode().IsRegular() {
		return ReusableKey{}, fmt.Errorf("keygen: %s is not a regular file", path)
	}
	key, ok := scanKeyCandidate(path)
	if !ok {
		if key.ParseError != nil {
			return ReusableKey{}, key.ParseError
		}
		return ReusableKey{}, fmt.Errorf("keygen: %s is not a usable private key", path)
	}
	return key, nil
}
