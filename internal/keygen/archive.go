package keygen

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// archiveDirMode is the mode enforced on the D-06 archive directory: private
// material, never world- or group-accessible.
const archiveDirMode os.FileMode = 0o700

// archivedPrivateKeyMode / archivedPublicKeyMode mirror the project's
// existing private/public key mode convention (0600/0644), reused here
// rather than imported — this package's other files (signers.go) already
// duplicate mode constants locally instead of importing a shared one from
// cmd/gitid, so this follows the established pattern.
const (
	archivedPrivateKeyMode os.FileMode = 0o600
	archivedPublicKeyMode  os.FileMode = 0o644
)

// ArchivedPair names the two archive-side paths CopyKeyPairToArchive or
// MoveKeyPairToArchive produced for one key pair. PublicPath is empty when
// the source pair had no public half (the caller passed an empty pubPath).
type ArchivedPair struct {
	PrivatePath string
	PublicPath  string
}

// CreatedFunc is announced once per archive copy, the instant that copy
// exists on disk and always BEFORE any source removal is attempted. A
// caller with a rollback journal registers the path here so a later failure
// can still undo it (review R3-01). The composition root's binding of this
// observer is owned by plan 05-03 Task 2.
type CreatedFunc func(path string) error

// ErrNoCreatedObserver is returned when a nil CreatedFunc is supplied to
// either archiver. Archiving without a way to undo it is the defect: the
// unsafe path must be asked for by name via IgnoreCreated, never silently
// accepted as a nil default.
var ErrNoCreatedObserver = errors.New("keygen: archiving requires a created-file observer")

// ErrArchiveIncomplete wraps a MoveKeyPairToArchive error produced when the
// archive copies were created successfully (and announced to onCreated) but
// removing one or more source paths afterwards failed. The archive copies
// are valid either way; the caller decides what to do about the surviving
// source(s) — RemoveArchivedPair is the rollback primitive for undoing them.
var ErrArchiveIncomplete = errors.New("keygen: archive copies created but the source pair was not fully removed")

// IgnoreCreated is a CreatedFunc that does nothing. It is the documented
// escape hatch for a caller with NO rollback obligation. A transactional
// caller — anything that can partially fail and needs to undo an archive
// entry — must NEVER use this; pass a real observer that records the path
// somewhere durable instead.
func IgnoreCreated(string) error { return nil }

// CopyKeyPairToArchive copies privPath (and pubPath, when non-empty) into
// archiveDir, NEVER touching the sources. This is the primitive
// delete-everything uses (review R-03) — rotation uses MoveKeyPairToArchive
// instead, because the two callers need different removal semantics from
// one shared copy step.
//
// The archive directory is created at archiveDirMode (0700) if absent, and
// tightened back to 0700 if its current mode is not already 0700 — an
// existing wider-than-0700 directory is TIGHTENED rather than left wide
// (review R-20). A symlinked archive directory is rejected — via os.Lstat,
// which does NOT follow the link — before any chmod or copy is attempted,
// so a symlink's target is never touched (review R-20).
//
// Each destination is named "<basename>.<stamp>" for the private half; the
// public half's destination is named the same way from pubPath's own
// basename (which already carries the ".pub" suffix), so ".pub" lands
// before the stamp automatically. stamp is supplied by the caller — the
// production caller uses the same UnixNano decimal convention
// filewriter.backupExistingTarget uses. This function owns only the
// PRIMITIVE half of the stamp-collision contract (review R2-05): an
// existing destination is a hard error here, and the composition-root retry
// that consumes that error is plan 05-03 Task 2's responsibility, where the
// retrying closure is written.
//
// onCreated is called once per archive copy, the instant that copy exists
// on disk, and always before any later step that could remove a source
// (review R3-01). A nil onCreated is refused with ErrNoCreatedObserver
// before any copy is attempted. An onCreated that returns an error aborts
// the archive: the copies made so far are removed, and the error surfaces
// wrapped with context.
//
// When a destination archive path already exists, the call fails without
// overwriting it and without touching any source (copyExclusive's
// exclusive-create idiom, mirrored from internal/filewriter — review R2-07).
func CopyKeyPairToArchive(archiveDir, privPath, pubPath, stamp string, onCreated CreatedFunc) (ArchivedPair, error) {
	if onCreated == nil {
		return ArchivedPair{}, ErrNoCreatedObserver
	}
	if err := prepareArchiveDir(archiveDir); err != nil {
		return ArchivedPair{}, err
	}

	privDst := archiveDestPath(archiveDir, privPath, stamp)
	if err := copyExclusive(privPath, privDst, archivedPrivateKeyMode); err != nil {
		return ArchivedPair{}, fmt.Errorf("keygen: archiving %s: %w", privPath, err)
	}
	if cerr := onCreated(privDst); cerr != nil {
		_ = os.Remove(privDst)
		return ArchivedPair{}, fmt.Errorf("keygen: archive observer rejected %s: %w", privDst, cerr)
	}

	var pubDst string
	if pubPath != "" {
		pubDst = archiveDestPath(archiveDir, pubPath, stamp)
		if err := copyExclusive(pubPath, pubDst, archivedPublicKeyMode); err != nil {
			_ = os.Remove(privDst)
			return ArchivedPair{}, fmt.Errorf("keygen: archiving %s: %w", pubPath, err)
		}
		if cerr := onCreated(pubDst); cerr != nil {
			_ = os.Remove(privDst)
			_ = os.Remove(pubDst)
			return ArchivedPair{}, fmt.Errorf("keygen: archive observer rejected %s: %w", pubDst, cerr)
		}
	}

	return ArchivedPair{PrivatePath: privDst, PublicPath: pubDst}, nil
}

// MoveKeyPairToArchive copies both halves of a key pair into archiveDir via
// CopyKeyPairToArchive, then removes the sources through the injected
// remove seam. This is the primitive rotation uses (review R-03). It is
// implemented as literally that composition — copy, then remove — so the
// two functions can never drift, and so onCreated's calls (fired inside the
// copy) always land BEFORE the first source removal (review R3-01): a
// second-source-removal failure can never leave an archive copy the
// caller's journal never heard about, because the journal already heard
// about BOTH copies before removal was attempted at all.
//
// remove is an explicit parameter (review R2-06, revised cycle 3): the
// production composition root passes os.Remove; a test can inject a remove
// that fails deterministically on a chosen source, independent of platform
// and unaffected by running as root. It is never a package-level mutable
// variable, which would break parallel tests.
//
// A nil onCreated is refused with ErrNoCreatedObserver before any copy — and
// therefore before remove is ever called. An onCreated that returns an
// error aborts the archive before any source is removed: the copies made so
// far are removed, remove is never invoked, and the error surfaces.
//
// On a source-removal failure, the archive copies remain (they are valid),
// and the returned error wraps ErrArchiveIncomplete and names which
// source(s) survived. The returned ArchivedPair carries both archive paths
// regardless of whether removal succeeded — every one of which onCreated
// has already announced.
func MoveKeyPairToArchive(archiveDir, privPath, pubPath, stamp string, remove func(path string) error, onCreated CreatedFunc) (ArchivedPair, error) {
	pair, err := CopyKeyPairToArchive(archiveDir, privPath, pubPath, stamp, onCreated)
	if err != nil {
		return ArchivedPair{}, err
	}

	var survivors []string
	if rerr := remove(privPath); rerr != nil {
		survivors = append(survivors, privPath)
	}
	if pubPath != "" {
		if rerr := remove(pubPath); rerr != nil {
			survivors = append(survivors, pubPath)
		}
	}
	if len(survivors) > 0 {
		return pair, fmt.Errorf("keygen: source(s) not removed after archiving (%s): %w", strings.Join(survivors, ", "), ErrArchiveIncomplete)
	}
	return pair, nil
}

// RemoveArchivedPair removes exactly the paths carried in pair, tolerating
// an already-absent path, and refusing any path that is not contained in
// archiveDir. This is the rollback primitive review R-10 requires: a
// transaction that created archive entries mid-flight must be able to undo
// exactly its own entries, and nothing outside the archive directory.
func RemoveArchivedPair(archiveDir string, pair ArchivedPair) error {
	dir := filepath.Clean(archiveDir)
	for _, p := range []string{pair.PrivatePath, pair.PublicPath} {
		if p == "" {
			continue
		}
		clean := filepath.Clean(p)
		if clean != dir && !strings.HasPrefix(clean, dir+string(filepath.Separator)) {
			return fmt.Errorf("keygen: refusing to remove %s: not inside archive directory %s", p, archiveDir)
		}
		if err := os.Remove(clean); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("keygen: removing archived %s: %w", p, err)
		}
	}
	return nil
}

// archiveDestPath builds the archive destination for src: the source's own
// basename plus a dot plus stamp. For the public half, src (pubPath) already
// carries the ".pub" suffix in its basename, so ".pub" lands before the
// stamp automatically — no separate public-path branch is needed.
func archiveDestPath(archiveDir, src, stamp string) string {
	return filepath.Join(archiveDir, filepath.Base(src)+"."+stamp)
}

// prepareArchiveDir ensures archiveDir exists at archiveDirMode (0700),
// tightening an existing directory back to 0700 whenever its current mode
// is not already 0700 (review R-20). A symlinked archiveDir is rejected via
// os.Lstat — which does NOT follow the link — before any mkdir or chmod is
// attempted, so a symlink's target is never touched.
func prepareArchiveDir(archiveDir string) error {
	if info, err := os.Lstat(archiveDir); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("keygen: archive directory %s is a symlink, refusing to use it", archiveDir)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("keygen: stat archive directory %s: %w", archiveDir, err)
	}

	if err := os.MkdirAll(archiveDir, archiveDirMode); err != nil {
		return fmt.Errorf("keygen: creating archive directory %s: %w", archiveDir, err)
	}
	info, err := os.Stat(archiveDir)
	if err != nil {
		return fmt.Errorf("keygen: stat archive directory %s: %w", archiveDir, err)
	}
	if info.Mode().Perm() != archiveDirMode {
		if cherr := os.Chmod(archiveDir, archiveDirMode); cherr != nil {
			return fmt.Errorf("keygen: chmod archive directory %s: %w", archiveDir, cherr)
		}
	}
	return nil
}

// copyExclusive copies src to dst at mode via an exclusive create
// (O_CREATE|O_EXCL) — mirroring internal/filewriter's own unexported
// copyFileExclusive idiom rather than importing it (review R2-07):
// filewriter is the project's write chokepoint and its copy helper is bound
// to backup semantics this caller does not want, so widening that
// package's public surface for one consumer would weaken the chokepoint's
// meaning. The one property that must not drift between the two
// implementations — an existing destination is a hard error, never an
// overwrite — is pinned by a test in both packages.
func copyExclusive(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src) //nolint:gosec // src is a trusted gitid-managed path
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode) //nolint:gosec // dst is a collision-checked archive path
	if err != nil {
		return err
	}
	if _, cerr := io.Copy(out, in); cerr != nil {
		_ = out.Close()
		_ = os.Remove(dst)
		return cerr
	}
	if cerr := out.Close(); cerr != nil {
		_ = os.Remove(dst)
		return cerr
	}
	// O_CREATE applies the umask, so set the mode explicitly.
	if cherr := os.Chmod(dst, mode); cherr != nil {
		_ = os.Remove(dst)
		return cherr
	}
	return nil
}
