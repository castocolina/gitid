package filewriter

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// backupMode is the restrictive mode applied to every backup file. Backups may
// hold private-key or config bytes, so they are never left world-readable
// (mitigates T-02-01).
const backupMode os.FileMode = 0o600

// maxBackupCollisionAttempts bounds the nanosecond-collision retry loop in
// backupExistingTarget (Codex HIGH #1). Two backups of the SAME target
// landing on the exact same nanosecond is vanishingly rare; the bound exists
// only to guarantee termination, never to silently give up on a real
// collision streak.
const maxBackupCollisionAttempts = 100

// Write atomically replaces targetPath with content at the exact requested
// mode, after first backing up any pre-existing target.
//
// The sequence is: copy the existing target to a collision-proof
// "<targetPath>.bak.<unix-nanoseconds>" path (mode 0600) if it exists, write
// content to a unique temp file in the same directory, fsync and close it,
// chmod it to the requested mode, then rename it over the target. The
// backup copy is created via an EXCLUSIVE create (O_EXCL): a same-instant
// collision on the backup path is retried with a fresh timestamp rather than
// ever silently overwriting a still-live recovery snapshot (Codex HIGH #1 —
// a migration rollback's own backup-of-the-failed-file step must never
// clobber the pristine backup it is restoring FROM). The rename is atomic on
// a single filesystem, so a partial or crashed write never leaves a
// corrupted target (mitigates T-02-02). The mode is always set explicitly
// via os.Chmod and never relies on the process umask (mitigates T-02-03). On
// any error after temp creation the temp file is removed and the original
// target is left untouched.
//
// targetPath is supplied by other in-process gitid packages and always points
// at a gitid-managed path (e.g. ~/.ssh/config, the private key, a gitconfig
// fragment); it is therefore trusted and not attacker-controlled.
//
// backupPath is non-empty only when the target pre-existed.
func Write(targetPath string, content []byte, mode os.FileMode) (backupPath string, err error) {
	// 1. Back up an existing target before touching it.
	if _, statErr := os.Stat(targetPath); statErr == nil {
		backupPath, err = backupExistingTarget(targetPath)
		if err != nil {
			return "", fmt.Errorf("backing up %s: %w", targetPath, err)
		}
	} else if !os.IsNotExist(statErr) {
		return "", fmt.Errorf("stat %s: %w", targetPath, statErr)
	}

	if rerr := atomicReplace(targetPath, content, mode); rerr != nil {
		return "", rerr
	}
	return backupPath, nil
}

// WriteNoBackup atomically replaces targetPath with content at the exact
// requested mode WITHOUT backing up any pre-existing target — it shares
// Write's temp-file -> fsync -> close -> chmod -> rename sequence (steps 2-5)
// but skips Write's own backup-creation step entirely.
//
// This is the dedicated ROLLBACK/RESTORE seam (Codex HIGH #1):
// crash-recovery code that already holds the pristine bytes to restore
// in memory (e.g. sshconfig.Migrate's rollback) must restore through this
// function, never through Write — re-entering Write's backup step during a
// restore would create a NEW backup of the file being replaced, and that new
// backup could, on a same-instant collision, jeopardize the very recovery
// snapshot the caller is restoring FROM. Callers that need a backup of the
// pre-restore state should capture it themselves BEFORE calling
// WriteNoBackup; Migrate's rollback deliberately does not, because the
// step-2 pristine backup already documents the true pre-migration state.
func WriteNoBackup(targetPath string, content []byte, mode os.FileMode) error {
	return atomicReplace(targetPath, content, mode)
}

// atomicReplace performs the shared temp-file -> fsync -> close -> chmod ->
// rename sequence used by both Write (after backing up any pre-existing
// target) and WriteNoBackup (skipping the backup step entirely).
func atomicReplace(targetPath string, content []byte, mode os.FileMode) error {
	// Create a UNIQUE temp file in the target's directory (mitigates
	// T-02-04 predictable-temp races); never a fixed ".tmp" suffix.
	dir := filepath.Dir(targetPath)
	tmp, err := os.CreateTemp(dir, "gitid-*.tmp") //nolint:gosec // dir derived from a trusted gitid-managed targetPath
	if err != nil {
		return fmt.Errorf("creating temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()

	// From here on, any failure removes the temp and leaves the target intact.
	cleanup := func(cause error) error {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return cause
	}

	// Write, fsync, close.
	if _, werr := tmp.Write(content); werr != nil {
		return cleanup(fmt.Errorf("writing temp file: %w", werr))
	}
	if serr := tmp.Sync(); serr != nil {
		return cleanup(fmt.Errorf("syncing temp file: %w", serr))
	}
	if cerr := tmp.Close(); cerr != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("closing temp file: %w", cerr)
	}

	// Set the exact mode explicitly (never rely on umask).
	if cherr := os.Chmod(tmpName, mode); cherr != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("chmod temp file: %w", cherr)
	}

	// Atomic rename into place.
	if rerr := os.Rename(tmpName, targetPath); rerr != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("renaming temp file over %s: %w", targetPath, rerr)
	}

	return nil
}

// backupExistingTarget copies targetPath's CURRENT on-disk content to a
// collision-proof "<targetPath>.bak.<unix-nanoseconds>" path, never
// overwriting an existing file at the backup path (Codex HIGH #1). The copy
// is created via an exclusive create (O_EXCL); on the vanishingly rare
// same-nanosecond collision (os.IsExist), a fresh timestamp is drawn and the
// copy retried, up to maxBackupCollisionAttempts.
func backupExistingTarget(targetPath string) (backupPath string, err error) {
	for attempt := 0; attempt < maxBackupCollisionAttempts; attempt++ {
		candidate := targetPath + ".bak." + strconv.FormatInt(time.Now().UnixNano(), 10)
		copyErr := copyFileExclusive(targetPath, candidate, backupMode)
		if copyErr == nil {
			return candidate, nil
		}
		if os.IsExist(copyErr) {
			continue
		}
		return "", copyErr
	}
	return "", fmt.Errorf("could not create a unique backup for %s after %d attempts", targetPath, maxBackupCollisionAttempts)
}

// BackupAndRemove creates a timestamped backup of path (same naming convention
// as Write) and removes the original via atomic rename. Used for whole-file
// deletion where content replacement does not apply (fragment file delete,
// IDENT-05 D-08). If path does not exist, returns ("", nil) — idempotent.
func BackupAndRemove(path string) (backupPath string, err error) {
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		return "", nil
	} else if statErr != nil {
		return "", fmt.Errorf("filewriter: stat %s: %w", path, statErr)
	}
	backupPath = path + ".bak." + time.Now().Format("20060102-150405")
	if renErr := os.Rename(path, backupPath); renErr != nil {
		return "", fmt.Errorf("filewriter: backing up %s before remove: %w", path, renErr)
	}
	return backupPath, nil
}

// EnsureDir creates dirPath (and any missing parents) and sets its mode
// explicitly, then chmods an already-existing directory to the same mode. It is
// used to enforce the ~/.ssh 0700 contract without relying on the umask.
//
// dirPath is a trusted gitid-managed path supplied in-process.
func EnsureDir(dirPath string, mode os.FileMode) error {
	if err := os.MkdirAll(dirPath, mode); err != nil {
		return fmt.Errorf("creating directory %s: %w", dirPath, err)
	}
	if err := os.Chmod(dirPath, mode); err != nil {
		return fmt.Errorf("setting mode on directory %s: %w", dirPath, err)
	}
	return nil
}

// CopyFile copies src to dst via the safe-write chokepoint: it reads src,
// then calls Write(dst, content, 0o644) to back up any pre-existing dst,
// write atomically, and set the exact mode. This inherits the full
// backup+atomic+chmod guarantees of Write for fragment adoption (ADOPT-01).
// The returned backupPath is non-empty when dst pre-existed (same semantics
// as Write). src and dst are trusted gitid-managed paths.
func CopyFile(src, dst string) (backupPath string, err error) {
	content, err := os.ReadFile(src) //nolint:gosec // src is a trusted gitid-managed path (G304)
	if err != nil {
		return "", fmt.Errorf("filewriter: reading source %s: %w", src, err)
	}
	return Write(dst, content, 0o644)
}

// copyFileExclusive copies src to dst at the given mode via an EXCLUSIVE
// create (O_CREATE|O_EXCL), fsyncing dst before close so the backup is
// durable. Unlike a truncating copy, this NEVER silently overwrites a
// pre-existing dst: if dst already exists, OpenFile fails with an
// os.IsExist-detectable error and no bytes are touched (Codex HIGH #1 — a
// backup must never clobber a file already occupying its path). src and dst
// are trusted gitid-managed paths.
func copyFileExclusive(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src) //nolint:gosec // src is a trusted gitid-managed target path
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode) //nolint:gosec // dst is the trusted, collision-checked backup path for a gitid-managed target
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(dst)
		return err
	}
	if err := out.Sync(); err != nil {
		_ = out.Close()
		_ = os.Remove(dst)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(dst)
		return err
	}

	// O_CREATE applies the umask, so set the mode explicitly to guarantee 0600.
	if err := os.Chmod(dst, mode); err != nil {
		_ = os.Remove(dst)
		return err
	}
	return nil
}
