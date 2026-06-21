package filewriter

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// backupMode is the restrictive mode applied to every backup file. Backups may
// hold private-key or config bytes, so they are never left world-readable
// (mitigates T-02-01).
const backupMode os.FileMode = 0o600

// Write atomically replaces targetPath with content at the exact requested
// mode, after first backing up any pre-existing target.
//
// The sequence is: copy the existing target to "<targetPath>.bak.<timestamp>"
// (mode 0600) if it exists, write content to a unique temp file in the same
// directory, fsync and close it, chmod it to the requested mode, then rename it
// over the target. The rename is atomic on a single filesystem, so a partial or
// crashed write never leaves a corrupted target (mitigates T-02-02). The mode is
// always set explicitly via os.Chmod and never relies on the process umask
// (mitigates T-02-03). On any error after temp creation the temp file is removed
// and the original target is left untouched.
//
// targetPath is supplied by other in-process gitid packages and always points
// at a gitid-managed path (e.g. ~/.ssh/config, the private key, a gitconfig
// fragment); it is therefore trusted and not attacker-controlled.
//
// backupPath is non-empty only when the target pre-existed.
func Write(targetPath string, content []byte, mode os.FileMode) (backupPath string, err error) {
	// 1. Back up an existing target before touching it.
	if _, statErr := os.Stat(targetPath); statErr == nil {
		backupPath = targetPath + ".bak." + time.Now().Format("20060102-150405")
		if copyErr := copyFile(targetPath, backupPath, backupMode); copyErr != nil {
			return "", fmt.Errorf("backing up %s: %w", targetPath, copyErr)
		}
	} else if !os.IsNotExist(statErr) {
		return "", fmt.Errorf("stat %s: %w", targetPath, statErr)
	}

	// 2. Create a UNIQUE temp file in the target's directory (mitigates
	//    T-02-04 predictable-temp races); never a fixed ".tmp" suffix.
	dir := filepath.Dir(targetPath)
	tmp, err := os.CreateTemp(dir, "gitid-*.tmp") //nolint:gosec // dir derived from a trusted gitid-managed targetPath
	if err != nil {
		return "", fmt.Errorf("creating temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()

	// From here on, any failure removes the temp and leaves the target intact.
	cleanup := func(cause error) (string, error) {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return "", cause
	}

	// 3. Write, fsync, close.
	if _, werr := tmp.Write(content); werr != nil {
		return cleanup(fmt.Errorf("writing temp file: %w", werr))
	}
	if serr := tmp.Sync(); serr != nil {
		return cleanup(fmt.Errorf("syncing temp file: %w", serr))
	}
	if cerr := tmp.Close(); cerr != nil {
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("closing temp file: %w", cerr)
	}

	// 4. Set the exact mode explicitly (never rely on umask).
	if cherr := os.Chmod(tmpName, mode); cherr != nil {
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("chmod temp file: %w", cherr)
	}

	// 5. Atomic rename into place.
	if rerr := os.Rename(tmpName, targetPath); rerr != nil {
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("renaming temp file over %s: %w", targetPath, rerr)
	}

	return backupPath, nil
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

// copyFile copies src to dst at the given mode, fsyncing dst before close so
// the backup is durable. src and dst are trusted gitid-managed paths.
func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src) //nolint:gosec // src is a trusted gitid-managed target path
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode) //nolint:gosec // dst is the trusted backup path for a gitid-managed target
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
