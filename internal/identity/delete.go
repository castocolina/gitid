package identity

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/castocolina/gitid/internal/filewriter"
)

// DeleteScope identifies how much of an identity's artifacts a delete
// removes. The two constants are the whole vocabulary MGR-06 needs; a third
// value is never expected and DeleteScopeFrom rejects it.
type DeleteScope string

const (
	// DeleteScopeGitOnly removes only the Git-side artifacts (the gitconfig
	// includeIf block and the fragment file) and keeps the SSH Host block,
	// the key pair, and the allowed_signers line untouched (05-CONTEXT.md
	// D-10 of THIS phase — not to be confused with the archived Phase-3
	// keepKey D-07 this file's doc comments used to cite. Phase 5's own D-07
	// is the unrelated allowed_signers APPEND decision).
	DeleteScopeGitOnly DeleteScope = "git-only"
	// DeleteScopeEverything removes every artifact for the identity (SSH
	// Host block, gitconfig includeIf block, fragment file, allowed_signers
	// line, and the key pair). Recognized as a valid scope but NOT YET
	// IMPLEMENTED in this plan — Delete returns ErrScopeNotAvailable when
	// called with it. Plan 05-04 replaces that branch with the real
	// implementation; nothing else about Delete's signature changes.
	DeleteScopeEverything DeleteScope = "everything"
)

// ErrScopeNotAvailable is returned by Delete when scope names a real, known
// DeleteScope this build does not yet implement (DeleteScopeEverything —
// landing in plan 05-04), or an unrecognized scope string. Both the CLI
// `--all` path (cmd/gitid/identity_delete.go) and the TUI's delete-choice
// everything option (internal/tuikit/identities.go handleDeleteKey) surface
// this ONE sentinel from this ONE domain entry point, so neither skin can
// drift from the other's refusal message (review R-16).
var ErrScopeNotAvailable = errors.New("identity: delete scope not yet available")

// DeleteScopeFrom parses a scope string (e.g. a CLI flag value) into a
// DeleteScope, returning a typed error for anything that is not a known
// scope constant. It does not itself decide availability — Delete does that
// — so a caller can always distinguish "not a real scope" from "a real scope
// this build has not implemented yet".
func DeleteScopeFrom(s string) (DeleteScope, error) {
	switch DeleteScope(s) {
	case DeleteScopeGitOnly:
		return DeleteScopeGitOnly, nil
	case DeleteScopeEverything:
		return DeleteScopeEverything, nil
	default:
		return "", fmt.Errorf("identity: unknown delete scope %q", s)
	}
}

// DeleteDeps holds every external effect Delete performs, injected as function
// fields so Delete is testable with fakes and reusable by the TUI. It mirrors
// the Deps convention from identity.go.
type DeleteDeps struct {
	// ReadSSH reads the raw bytes of ~/.ssh/config. Under DeleteScopeGitOnly
	// this is NEVER called — the SSH branch is structurally skipped (D-10,
	// review R-13).
	ReadSSH func() ([]byte, error)
	// ReadGitconfig reads the raw bytes of ~/.gitconfig.
	ReadGitconfig func() ([]byte, error)
	// WriteSSH writes the updated ~/.ssh/config bytes and returns a backup
	// path. Under DeleteScopeGitOnly this is NEVER called (see ReadSSH).
	WriteSSH func(content []byte) (backupPath string, err error)
	// WriteGitconfig writes the updated ~/.gitconfig bytes and returns a backup path.
	WriteGitconfig func(content []byte) (backupPath string, err error)
	// RemoveFragment removes the whole per-identity fragment file with backup.
	RemoveFragment func(fragPath string) (backupPath string, err error)
	// RemoveAllowedSigners removes the identity's managed block from the
	// allowed_signers file, keyed by identity NAME (symmetric with the
	// block-keyed writer keygen.WriteAllowedSigners). Keying by name — not email
	// — ensures an Incomplete identity with a missing fragment (GitEmail == "")
	// still has its signing block removed. Under DeleteScopeGitOnly this is
	// NEVER called — D-10 keeps the signing line intact so historic
	// `git log --show-signature` verification keeps working.
	RemoveAllowedSigners func(path, name string) (backupPath string, err error)
	// RemoveKeyFiles removes the private and public key files via a recoverable
	// backup-then-remove, returning the private and public key backup paths.
	// Under DeleteScopeGitOnly this is NEVER called — the key pair is kept
	// untouched (D-10). A missing key file is a no-op (empty backup path, no
	// error).
	RemoveKeyFiles func(keyPath, pubPath string) (keyBackup, pubBackup string, err error)
}

// DeleteResult holds the backup paths produced by Delete. An EMPTY backup
// path field means "this artifact was already absent or already correct"
// (matching filewriter's own empty-backupPath = did-not-pre-exist
// convention) — never "not attempted"; SSHUntouched is the explicit signal
// for "this scope never even looked at this artifact".
type DeleteResult struct {
	// SSHUntouched is true when the SSH branch was structurally skipped for
	// the requested scope (DeleteScopeGitOnly, D-10) — deps.ReadSSH and
	// deps.WriteSSH were never invoked, so SSHBackup is meaningfully empty
	// rather than "nothing changed".
	SSHUntouched         bool
	SSHBackup            string
	GitconfigBackup      string
	FragmentBackup       string
	AllowedSignersBackup string
	KeyBackup            string
	PubBackup            string
}

// Delete removes an identity's artifacts according to scope, with backup via
// the injected DeleteDeps. Shared/global blocks (e.g. the macOS "_global" SSH
// block and the global signing wiring) are NEVER touched — only acct.Name is
// passed to RemoveBlock. RemoveBlock is idempotent: if a block is already
// absent the file is returned unchanged (no error), and — for the writer
// branches that live directly inside Delete (review R-14) — an unchanged
// RemoveBlock result skips its writer entirely, so an idempotent re-run never
// mints a second backup.
//
// DeleteScopeGitOnly (05-CONTEXT.md D-10 of THIS phase — not the archived
// Phase-3 keepKey D-07, and not Phase 5's own D-07, which is the unrelated
// allowed_signers APPEND decision): removes the gitconfig includeIf block and
// the fragment file. The SSH branch is STRUCTURALLY skipped — deps.ReadSSH
// and deps.WriteSSH are never invoked, so an unchanged ~/.ssh/config can
// never acquire a timestamped backup or a receipt line (review R-13) — and
// deps.RemoveAllowedSigners / deps.RemoveKeyFiles are never called, so the
// allowed_signers line and the key pair survive untouched.
//
// DeleteScopeEverything is a recognized scope NOT YET IMPLEMENTED in this
// plan: Delete returns an error satisfying errors.Is(err,
// ErrScopeNotAvailable) before touching any dep, so the CLI `--all` path and
// the TUI everything option refuse identically from this one place (review
// R-16). Plan 05-04 replaces this branch with the real implementation.
//
// An unrecognized scope string also returns an error satisfying errors.Is(err,
// ErrScopeNotAvailable).
func Delete(acct Account, scope DeleteScope, deps DeleteDeps) (DeleteResult, error) {
	var res DeleteResult

	switch scope {
	case DeleteScopeGitOnly:
		// fall through below
	case DeleteScopeEverything:
		return res, fmt.Errorf("identity: delete scope %q: %w", scope, ErrScopeNotAvailable)
	default:
		return res, fmt.Errorf("identity: delete scope %q: %w", scope, ErrScopeNotAvailable)
	}

	// D-10 / review R-13: the SSH branch is STRUCTURALLY skipped for
	// DeleteScopeGitOnly — deps.ReadSSH and deps.WriteSSH are never called,
	// so an unchanged ~/.ssh/config can never acquire a timestamped backup.
	res.SSHUntouched = true

	// Remove ONLY the per-identity includeIf block from the gitconfig bytes.
	// Equality-guarded (review R-14): when RemoveBlock produces bytes
	// identical to what was read (e.g. the block is already absent — an
	// idempotent re-run), the writer is skipped entirely so no second backup
	// is created and GitconfigBackup stays empty.
	gcBytes, err := deps.ReadGitconfig()
	if err != nil {
		return res, fmt.Errorf("identity: reading gitconfig: %w", err)
	}
	updatedGC := filewriter.RemoveBlock(gcBytes, acct.Name)
	if !bytes.Equal(updatedGC, gcBytes) {
		gcBak, werr := deps.WriteGitconfig(updatedGC)
		if werr != nil {
			return res, fmt.Errorf("identity: removing gitconfig block: %w", werr)
		}
		res.GitconfigBackup = gcBak
	}

	// Remove the whole fragment file (whole-file backup+remove). This is
	// already idempotent at the deps level: a real RemoveFragment (backed by
	// filewriter.BackupAndRemove) returns an empty backup path with no error
	// when the file is already absent.
	fragBak, err := deps.RemoveFragment(acct.FragmentPath)
	if err != nil {
		return res, fmt.Errorf("identity: removing fragment file: %w", err)
	}
	res.FragmentBackup = fragBak

	// D-10: allowed_signers and the key pair are NEVER touched under
	// DeleteScopeGitOnly — deps.RemoveAllowedSigners and deps.RemoveKeyFiles
	// are deliberately not called here.
	return res, nil
}
