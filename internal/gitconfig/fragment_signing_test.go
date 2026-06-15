package gitconfig

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestWriteFragment_SigningTrue asserts that WriteFragment with signing=true writes
// all five keys including the signing trio (gpg.format, user.signingkey,
// commit.gpgsign) to the fragment file.
func TestWriteFragment_SigningTrue(t *testing.T) {
	home := t.TempDir()
	fragPath := filepath.Join(home, ".gitconfig.d", "work")
	signingKey := filepath.Join(home, ".ssh", "id_ed25519_work.pub")

	if err := WriteFragment(fragPath, "Work User", "work@example.com", signingKey, true); err != nil {
		t.Fatalf("WriteFragment(signing=true) error: %v", err)
	}

	out := gitConfigList(t, fragPath)
	assertContains(t, out, "user.name=Work User")
	assertContains(t, out, "user.email=work@example.com")
	assertContains(t, out, "gpg.format=ssh")
	assertContains(t, out, "user.signingkey="+signingKey)
	assertContains(t, out, "commit.gpgsign=true")
}

// TestWriteFragment_SigningFalse asserts that WriteFragment with signing=false
// writes only user.name and user.email; the three signing keys must NOT be present.
func TestWriteFragment_SigningFalse(t *testing.T) {
	home := t.TempDir()
	fragPath := filepath.Join(home, ".gitconfig.d", "work")
	signingKey := filepath.Join(home, ".ssh", "id_ed25519_work.pub")

	if err := WriteFragment(fragPath, "Work User", "work@example.com", signingKey, false); err != nil {
		t.Fatalf("WriteFragment(signing=false) error: %v", err)
	}

	out := gitConfigList(t, fragPath)
	assertContains(t, out, "user.name=Work User")
	assertContains(t, out, "user.email=work@example.com")
	assertNotContains(t, out, "gpg.format")
	assertNotContains(t, out, "user.signingkey")
	assertNotContains(t, out, "commit.gpgsign")
}

// TestWriteFragment_SigningToggleOnToOff asserts that when signing is first written
// as true and then overwritten with false, the three signing keys are gone
// (idempotent rewrite — Pitfall C: exit-5-safe unset).
func TestWriteFragment_SigningToggleOnToOff(t *testing.T) {
	home := t.TempDir()
	fragPath := filepath.Join(home, ".gitconfig.d", "work")
	signingKey := filepath.Join(home, ".ssh", "id_ed25519_work.pub")

	// First write with signing enabled.
	if err := WriteFragment(fragPath, "Work User", "work@example.com", signingKey, true); err != nil {
		t.Fatalf("WriteFragment(signing=true) error: %v", err)
	}
	// Verify signing keys are present.
	out := gitConfigList(t, fragPath)
	assertContains(t, out, "gpg.format=ssh")

	// Second write with signing disabled — must remove the signing trio.
	if err := WriteFragment(fragPath, "Work User", "work@example.com", signingKey, false); err != nil {
		t.Fatalf("WriteFragment(signing=false) after true error: %v", err)
	}
	out = gitConfigList(t, fragPath)
	assertNotContains(t, out, "gpg.format")
	assertNotContains(t, out, "user.signingkey")
	assertNotContains(t, out, "commit.gpgsign")
	// Identity keys remain.
	assertContains(t, out, "user.name=Work User")
	assertContains(t, out, "user.email=work@example.com")
}

// TestWriteFragment_SigningToggleOffToOn asserts that writing signing=false
// followed by signing=true produces the full five-key set.
func TestWriteFragment_SigningToggleOffToOn(t *testing.T) {
	home := t.TempDir()
	fragPath := filepath.Join(home, ".gitconfig.d", "work")
	signingKey := filepath.Join(home, ".ssh", "id_ed25519_work.pub")

	if err := WriteFragment(fragPath, "Work User", "work@example.com", signingKey, false); err != nil {
		t.Fatalf("WriteFragment(signing=false) error: %v", err)
	}
	if err := WriteFragment(fragPath, "Work User", "work@example.com", signingKey, true); err != nil {
		t.Fatalf("WriteFragment(signing=true) error: %v", err)
	}

	out := gitConfigList(t, fragPath)
	assertContains(t, out, "gpg.format=ssh")
	assertContains(t, out, "user.signingkey="+signingKey)
	assertContains(t, out, "commit.gpgsign=true")
}

// TestWriteFragment_ValidationStillApplied asserts that the validation guards
// (newline in name, malformed email) still apply regardless of signing flag.
func TestWriteFragment_ValidationStillApplied(t *testing.T) {
	home := t.TempDir()
	fragPath := filepath.Join(home, ".gitconfig.d", "work")

	if err := WriteFragment(fragPath, "Bad\nName", "work@example.com", "", false); err == nil {
		t.Error("WriteFragment must reject newline in user.name")
	}
	if err := WriteFragment(fragPath, "Work User", "notanemail", "", false); err == nil {
		t.Error("WriteFragment must reject malformed email")
	}
}

// gitConfigList runs `git config --file <path> --list` and returns the output string.
func gitConfigList(t *testing.T, fragPath string) string {
	t.Helper()
	out, err := exec.Command("git", "config", "--file", fragPath, "--list").Output() //nolint:gosec
	if err != nil {
		t.Fatalf("git config --file %s --list: %v", fragPath, err)
	}
	return string(out)
}

func assertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("expected output to contain %q\ngot:\n%s", needle, haystack)
	}
}

func assertNotContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if strings.Contains(haystack, needle) {
		t.Errorf("expected output NOT to contain %q\ngot:\n%s", needle, haystack)
	}
}
