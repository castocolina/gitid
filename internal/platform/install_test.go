package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBinaryOnPath(t *testing.T) {
	// Create a temp dir to simulate a binary location.
	dir := t.TempDir()
	exePath := filepath.Join(dir, "gitid")

	origPATH := os.Getenv("PATH")

	t.Run("binary dir on PATH reports true", func(t *testing.T) {
		pathEnv := dir + string(os.PathListSeparator) + origPATH
		if !binaryOnPath(exePath, pathEnv) {
			t.Errorf("binaryOnPath(%q, %q) = false, want true", exePath, pathEnv)
		}
	})

	t.Run("binary dir NOT on PATH reports false", func(t *testing.T) {
		pathEnv := "/usr/bin:/bin"
		if binaryOnPath(exePath, pathEnv) {
			t.Errorf("binaryOnPath(%q, %q) = true, want false", exePath, pathEnv)
		}
	})

	t.Run("empty PATH reports false", func(t *testing.T) {
		if binaryOnPath(exePath, "") {
			t.Errorf("binaryOnPath(%q, \"\") = true, want false", exePath)
		}
	})
}

func TestBinaryInstallInfo(t *testing.T) {
	path, _, err := BinaryInstallInfo()
	if err != nil {
		t.Fatalf("BinaryInstallInfo() unexpected error: %v", err)
	}
	if path == "" {
		t.Error("BinaryInstallInfo() returned empty path, want non-empty")
	}
}
