package sshconfig

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/castocolina/gitid/internal/tester"
)

// TestMultiIdentityCoexistence proves SC-2: two same-provider identities written
// via the real sshconfig writer each resolve to their own distinct IdentityFile
// under hermetic `ssh -G -F <configPath>` — no real ~/.ssh/config is read.
func TestMultiIdentityCoexistence(t *testing.T) {
	// Guard: skip when ssh is not available in this environment.
	if _, err := exec.LookPath("ssh"); err != nil {
		t.Skip("ssh not found; skipping coexistence test")
	}

	// Hermetic HOME: no real ~/.ssh is touched (T-03-08).
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatalf("creating hermetic .ssh dir: %v", err)
	}
	configPath := filepath.Join(home, ".ssh", "config")

	// Write identity "personal" with its own key path.
	personalKey := filepath.Join(home, ".ssh", "id_ed25519_personal")
	if _, err := Write(configPath, "personal",
		RenderHostBlock("personal.github.com", "ssh.github.com", 443, personalKey),
		""); err != nil {
		t.Fatalf("writing personal identity: %v", err)
	}

	// Write identity "work" with a distinct key path, same provider.
	workKey := filepath.Join(home, ".ssh", "id_ed25519_work")
	if _, err := Write(configPath, "work",
		RenderHostBlock("work.github.com", "ssh.github.com", 443, workKey),
		""); err != nil {
		t.Fatalf("writing work identity: %v", err)
	}

	// resolveAlias runs ssh -G -F <configPath> <alias> hermetially and returns
	// the parsed ResolvedConfig. The -F flag pins the config file so the real
	// ~/.ssh/config is never consulted (T-03-08).
	resolveAlias := func(alias string) tester.ResolvedConfig {
		out, err := exec.Command("ssh", "-G", "-F", configPath, alias).Output() //nolint:gosec // arg-slice form, no shell; configPath is a hermetic test path (G204)
		if err != nil {
			t.Logf("ssh -G -F output for %s: %v", alias, err)
		}
		return tester.ParseResolved(string(out))
	}

	personalRC := resolveAlias("personal.github.com")
	workRC := resolveAlias("work.github.com")

	// SC-2: each alias must resolve to at least one IdentityFile.
	if len(personalRC.IdentityFiles) == 0 {
		t.Fatalf("personal alias resolved no IdentityFiles; ssh config may be malformed")
	}
	if len(workRC.IdentityFiles) == 0 {
		t.Fatalf("work alias resolved no IdentityFiles; ssh config may be malformed")
	}

	// SC-2: the two identities must resolve to distinct IdentityFiles.
	if personalRC.IdentityFiles[0] == workRC.IdentityFiles[0] {
		t.Errorf("SC-2 FAILED: personal and work resolved to the same IdentityFile %q; identities are not isolated",
			personalRC.IdentityFiles[0])
	}
}
