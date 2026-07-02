package platform

import (
	"os"
	"path/filepath"
	"strings"
)

// BinaryInstallInfo returns the resolved path of the running gitid binary and
// whether the directory containing it appears on the current PATH.
// It uses os.Executable() + filepath.EvalSymlinks so symlinked installs resolve
// to the real binary path. The PATH check is delegated to binaryOnPath so it
// can be unit-tested without the real running binary.
func BinaryInstallInfo() (path string, onPATH bool, err error) {
	exe, err := os.Executable()
	if err != nil {
		return "", false, err
	}
	// Resolve symlinks to find the real binary path (A1: tolerate EvalSymlinks
	// error by keeping the unresolved path).
	if resolved, evalErr := filepath.EvalSymlinks(exe); evalErr == nil {
		exe = resolved
	}
	onPATH = binaryOnPath(exe, os.Getenv("PATH"))
	return exe, onPATH, nil
}

// binaryOnPath reports whether the directory containing exePath appears in
// pathEnv (a colon-separated PATH string). It is a pure helper so tests can
// exercise the PATH check without depending on the real running binary.
func binaryOnPath(exePath string, pathEnv string) bool {
	if pathEnv == "" {
		return false
	}
	exeDir := filepath.Clean(filepath.Dir(exePath))
	for _, entry := range strings.Split(pathEnv, string(os.PathListSeparator)) {
		if entry == "" {
			continue
		}
		// Clean both sides so a trailing slash (e.g. "/usr/local/bin/") or other
		// non-canonical form still matches a genuinely reachable directory.
		if filepath.Clean(entry) == exeDir {
			return true
		}
	}
	return false
}
