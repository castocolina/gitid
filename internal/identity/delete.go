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
	// DeleteScopeEverything removes every artifact for the identity: the
	// gitconfig includeIf block, the fragment file, the SSH Host block, the
	// allowed_signers block, the provider rewrite (when no reference of any
	// kind survives — D-09), and the key pair (archived, then removed —
	// D-11), unless the key is shared with a sibling identity, in which
	// case it is kept and the sibling is named (D-12, see delete.go's
	// keySurvives handling).
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
	// RemoveKeyFiles removes the LIVE private and public key files. Under
	// DeleteScopeEverything it is the LAST seam Delete calls, after the D-11
	// archive copy (CopyKeyPairToArchive, below) has already landed — the
	// archive copy IS the recoverable backup, so this seam's own returned
	// backup paths are not consulted by Delete; only that the live files are
	// gone afterward matters. Under DeleteScopeGitOnly this is NEVER called
	// — the key pair is kept untouched (D-10). A missing key file is a no-op
	// (no error).
	RemoveKeyFiles func(keyPath, pubPath string) (keyBackup, pubBackup string, err error)

	// Accounts returns every reconstructed account (including the one being
	// deleted), letting Delete compute ProviderRefCount and SharedKeyOwners
	// without a second reconstruction pass. The composition root binds this
	// to the SAME account list the identity list/CommitDelete/CLI delete
	// already read — never a second, divergent lookup. Only consulted under
	// DeleteScopeEverything.
	Accounts func() ([]Account, error)

	// ForeignProviderRefs counts every Host stanza OUTSIDE any gitid-managed
	// block (D-09: "hand-written aliases targeting the same provider") whose
	// resolved provider key (ProviderKeyForHost) equals providerKey. The
	// composition root fills this by walking the real ~/.ssh/config. Only
	// consulted under DeleteScopeEverything, and only when a provider key is
	// resolvable for the identity being deleted.
	ForeignProviderRefs func(providerKey string) (int, error)

	// RemoveProviderRewrite removes the shared "provider-rewrite:<host>"
	// managed block for providerKey (D-09), called ONLY when both
	// ProviderRefCount and ForeignProviderRefs report zero for providerKey —
	// i.e. no reference of any kind survives. Never called under
	// DeleteScopeGitOnly: the identity's own SSH alias survives that scope,
	// so it still counts as a user of the provider.
	RemoveProviderRewrite func(providerKey string) (backupPath string, err error)

	// CopyKeyPairToArchive copies the identity's key pair into the D-11
	// archive directory WITHOUT touching the sources — the recoverable-
	// removal primitive delete-everything uses (review R-03; rotation uses
	// the MOVE primitive instead, through identity.Deps.ArchiveKeyPair — the
	// two must never be collapsed into one). Called FIRST, before any other
	// write, so a failure at any later step leaves both the archive copy and
	// the live key intact. The composition root binds this to a
	// transaction's journal exactly as identity.Deps.ArchiveKeyPair is bound
	// (review R3-01): a backend-wide binding refuses; only a
	// transaction-scoped seam actually archives. Skipped entirely when the
	// D-12 shared-key downgrade applies (the key survives).
	CopyKeyPairToArchive func(privPath, pubPath string) (archivedPriv, archivedPub string, err error)
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
	// KeyBackup / PubBackup are the D-11 archive copy paths under
	// DeleteScopeEverything (never a filewriter timestamped sibling backup
	// — the archive copy itself IS the recoverable backup). Empty when the
	// key was never touched (git-only) or kept (D-12 shared-key downgrade).
	KeyBackup string
	PubBackup string

	// ProviderRewriteRemoved is true when the D-09 ref-count found no
	// surviving reference of any kind and the provider rewrite block was
	// removed. ProviderRewriteBackup carries the backup path from that
	// removal (empty when the block did not pre-exist).
	ProviderRewriteRemoved bool
	ProviderRewriteBackup  string

	// KeyKeptFor names every sibling identity that still references this
	// identity's key path (D-12) — non-empty exactly when the everything
	// scope's key archive-and-remove step was skipped because the key
	// survives.
	KeyKeptFor []string

	// ArchivedKeyPaths carries every archive-directory path this call
	// created (private, and public when present) — review R-10: so a
	// caller's rollback journal can discover and undo them even when a
	// later step in the same transaction fails.
	ArchivedKeyPaths []string
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
// DeleteScopeEverything removes every artifact (see the constant's doc
// comment). It is a full implementation as of this plan (05-04) — the prior
// ErrScopeNotAvailable refusal for this scope has been removed.
//
// An unrecognized scope string still returns an error satisfying
// errors.Is(err, ErrScopeNotAvailable).
func Delete(acct Account, scope DeleteScope, deps DeleteDeps) (DeleteResult, error) {
	var res DeleteResult

	switch scope {
	case DeleteScopeGitOnly:
		res.SSHUntouched = true
	case DeleteScopeEverything:
		// handled below
	default:
		return res, fmt.Errorf("identity: delete scope %q: %w", scope, ErrScopeNotAvailable)
	}

	// Remove ONLY the per-identity includeIf block from the gitconfig bytes.
	// Equality-guarded (review R-14): when RemoveBlock produces bytes
	// identical to what was read (e.g. the block is already absent — an
	// idempotent re-run), the writer is skipped entirely so no second backup
	// is created and GitconfigBackup stays empty. Common to both scopes.
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

	if scope == DeleteScopeGitOnly {
		// Remove the whole fragment file (whole-file backup+remove). This is
		// already idempotent at the deps level: a real RemoveFragment (backed
		// by filewriter.BackupAndRemove) returns an empty backup path with no
		// error when the file is already absent.
		fragBak, ferr := deps.RemoveFragment(acct.FragmentPath)
		if ferr != nil {
			return res, fmt.Errorf("identity: removing fragment file: %w", ferr)
		}
		res.FragmentBackup = fragBak

		// D-10: allowed_signers, the SSH Host block, the provider rewrite,
		// and the key pair are NEVER touched under DeleteScopeGitOnly — the
		// identity's SSH alias survives this scope, so it still counts as a
		// user of the provider (the ref-count question only arises on the
		// everything path, below).
		return res, nil
	}

	return deleteEverything(acct, deps, res)
}

// deleteEverything performs the DeleteScopeEverything-only steps, ordered so
// a partial failure is never destructive in a new way: copy the key pair
// into the archive FIRST, then remove the managed SSH Host block and the
// allowed_signers block, then unlink the fragment, then remove the provider
// rewrite (D-09, only when no reference of any kind survives), then remove
// the LIVE key files LAST. Any failure before that last step therefore
// leaves both a valid archive copy and a working live key.
func deleteEverything(acct Account, deps DeleteDeps, res DeleteResult) (DeleteResult, error) {
	// 1. Archive the key pair FIRST — before any other write — so a failure
	// anywhere below still leaves a recoverable copy (D-11, review R-03).
	if acct.KeyPath != "" {
		archivedPriv, archivedPub, aerr := deps.CopyKeyPairToArchive(acct.KeyPath, acct.PubPath)
		// Record whatever archive paths were created BEFORE inspecting the
		// error (review R3-01/R-10): a partial archive is still reportable.
		if archivedPriv != "" {
			res.ArchivedKeyPaths = append(res.ArchivedKeyPaths, archivedPriv)
		}
		if archivedPub != "" {
			res.ArchivedKeyPaths = append(res.ArchivedKeyPaths, archivedPub)
		}
		if aerr != nil {
			return res, fmt.Errorf("identity: archiving key pair: %w", aerr)
		}
		res.KeyBackup = archivedPriv
		res.PubBackup = archivedPub
	}

	// 2. Remove the managed SSH Host block. Equality-guarded exactly like
	// the gitconfig block above.
	sshBytes, rerr := deps.ReadSSH()
	if rerr != nil {
		return res, fmt.Errorf("identity: reading ssh config: %w", rerr)
	}
	updatedSSH := filewriter.RemoveBlock(sshBytes, acct.Name)
	if !bytes.Equal(updatedSSH, sshBytes) {
		sshBak, werr := deps.WriteSSH(updatedSSH)
		if werr != nil {
			return res, fmt.Errorf("identity: removing ssh host block: %w", werr)
		}
		res.SSHBackup = sshBak
	}

	// 3. Remove the allowed_signers block, keyed by identity NAME (symmetric
	// with the block-keyed writer keygen.WriteAllowedSigners) — so an
	// Incomplete identity with a missing fragment (GitEmail == "") still has
	// its signing block removed.
	asBak, aserr := deps.RemoveAllowedSigners(acct.AllowedSignersPath, acct.Name)
	if aserr != nil {
		return res, fmt.Errorf("identity: removing allowed_signers block: %w", aserr)
	}
	res.AllowedSignersBackup = asBak

	// 4. Unlink the fragment file.
	fragBak, ferr := deps.RemoveFragment(acct.FragmentPath)
	if ferr != nil {
		return res, fmt.Errorf("identity: removing fragment file: %w", ferr)
	}
	res.FragmentBackup = fragBak

	// 5. Remove the provider rewrite block, ONLY when no reference of any
	// kind survives (D-09): neither another gitid-managed identity
	// (ProviderRefCount over NORMALIZED keys) nor a hand-written Host stanza
	// (ForeignProviderRefs).
	providerKey := RewriteProviderKey(acct.Provider, acct.Alias)
	if providerKey != "" {
		accounts, aerr := deps.Accounts()
		if aerr != nil {
			return res, fmt.Errorf("identity: listing accounts for provider ref-count: %w", aerr)
		}
		managedRefs := ProviderRefCount(accounts, providerKey, acct.Name)
		foreignRefs := 0
		if deps.ForeignProviderRefs != nil {
			foreignRefs, aerr = deps.ForeignProviderRefs(providerKey)
			if aerr != nil {
				return res, fmt.Errorf("identity: counting foreign provider refs: %w", aerr)
			}
		}
		if managedRefs == 0 && foreignRefs == 0 && deps.RemoveProviderRewrite != nil {
			prBak, perr := deps.RemoveProviderRewrite(providerKey)
			if perr != nil {
				return res, fmt.Errorf("identity: removing provider rewrite: %w", perr)
			}
			res.ProviderRewriteBackup = prBak
			res.ProviderRewriteRemoved = true
		}
	}

	// 6. Remove the LIVE key files LAST. Any failure above this point has
	// already returned, leaving both the archive copy (step 1) and the live
	// key intact.
	if acct.KeyPath != "" {
		if _, _, kerr := deps.RemoveKeyFiles(acct.KeyPath, acct.PubPath); kerr != nil {
			return res, fmt.Errorf("identity: removing live key files: %w", kerr)
		}
	}

	return res, nil
}
