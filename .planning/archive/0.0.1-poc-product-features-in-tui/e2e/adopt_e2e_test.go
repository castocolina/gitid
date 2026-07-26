//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestAdopt_Migrate verifies that `gitid adopt <path> --method migrate --yes`
// copies the plain-style fragment to ~/.gitconfig.d/<name>, preserves the
// original, and writes an [includeIf] block in ~/.gitconfig.
//
// This test is RED — the `gitid adopt` subcommand does not yet exist.
// Wave 1 Plan 02 (05.7-02) turns this GREEN.
func TestAdopt_Migrate(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)

	// Seed ~/.gitconfig_work (plain-style fragment).
	fragContent := "[user]\n\tname = Work User\n\temail = work@corp.com\n"
	fragPath := filepath.Join(home, ".gitconfig_work")
	if err := os.WriteFile(fragPath, []byte(fragContent), 0o644); err != nil {
		t.Fatalf("seeding fragment: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, "adopt", fragPath,
		"--method", "migrate", "--yes")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), "HOME="+home)

	if err := cmd.Run(); err != nil {
		t.Fatalf("gitid adopt failed: %v\nstdout: %s\nstderr: %s",
			err, stdout.String(), stderr.String())
	}

	// Assert: original fragment preserved at its original path.
	assertFileExists(t, fragPath, "original fragment preserved")

	// Assert: migrated copy at ~/.gitconfig.d/work.
	assertFileExists(t, filepath.Join(home, ".gitconfig.d", "work"), "migrated fragment")

	// Assert: [includeIf] present in ~/.gitconfig.
	content, err := os.ReadFile(filepath.Join(home, ".gitconfig"))
	if err != nil {
		t.Fatalf("reading ~/.gitconfig: %v", err)
	}
	if !strings.Contains(string(content), "[includeIf") {
		t.Errorf("~/.gitconfig missing [includeIf] after adopt migrate;\ncontent:\n%s", content)
	}
}

// TestAdopt_Reference verifies that `gitid adopt <path> --method reference --yes`
// leaves the original fragment untouched in place and writes an [includeIf] that
// points directly at the original path.
//
// This test is RED — the `gitid adopt` subcommand does not yet exist.
// Wave 1 Plan 02 (05.7-02) turns this GREEN.
func TestAdopt_Reference(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)

	// Seed ~/.gitconfig_work (plain-style fragment).
	fragContent := "[user]\n\tname = Work User\n\temail = work@corp.com\n"
	fragPath := filepath.Join(home, ".gitconfig_work")
	if err := os.WriteFile(fragPath, []byte(fragContent), 0o644); err != nil {
		t.Fatalf("seeding fragment: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, "adopt", fragPath,
		"--method", "reference", "--yes")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), "HOME="+home)

	if err := cmd.Run(); err != nil {
		t.Fatalf("gitid adopt --method reference failed: %v\nstdout: %s\nstderr: %s",
			err, stdout.String(), stderr.String())
	}

	// Assert: original fragment is still at its original path and unchanged.
	assertFileExists(t, fragPath, "original fragment preserved in reference mode")
	gotContent, err := os.ReadFile(fragPath)
	if err != nil {
		t.Fatalf("reading original fragment: %v", err)
	}
	if string(gotContent) != fragContent {
		t.Errorf("original fragment content changed in reference mode;\ngot:\n%s\nwant:\n%s",
			gotContent, fragContent)
	}

	// Assert: [includeIf] pointing at the original path exists in ~/.gitconfig.
	gcContent, err := os.ReadFile(filepath.Join(home, ".gitconfig"))
	if err != nil {
		t.Fatalf("reading ~/.gitconfig: %v", err)
	}
	if !strings.Contains(string(gcContent), "[includeIf") {
		t.Errorf("~/.gitconfig missing [includeIf] after adopt reference;\ncontent:\n%s", gcContent)
	}
	if !strings.Contains(string(gcContent), filepath.Base(fragPath)) &&
		!strings.Contains(string(gcContent), fragPath) {
		t.Errorf("~/.gitconfig includeIf should reference the original fragment path;\ncontent:\n%s", gcContent)
	}
}
