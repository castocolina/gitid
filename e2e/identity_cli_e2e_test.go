//go:build e2e

package e2e

// identity_cli_e2e_test.go: drives the REAL gitid binary's `identity list`
// D-03 read surface end-to-end via a plain exec.Command (no PTY — this is
// the PTY-free assertion channel D-03 exists to provide), the same pattern
// debug_e2e_test.go/adopt_e2e_test.go use for a non-interactive command.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// seedSSHOnlyIdentity writes ONLY an SSH Host block for name — no gitconfig
// includeIf block, no fragment file — so the reconstructed Account carries
// empty Git fields (MGR-03).
func seedSSHOnlyIdentity(t *testing.T, home, name string) {
	t.Helper()
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("seedSSHOnlyIdentity: MkdirAll .ssh: %v", err)
	}
	privKey := filepath.Join(sshDir, "id_ed25519_"+name)
	pubKey := privKey + ".pub"
	if err := os.WriteFile(privKey, []byte("-----BEGIN OPENSSH PRIVATE KEY-----\nSTUB\n-----END OPENSSH PRIVATE KEY-----\n"), 0o600); err != nil {
		t.Fatalf("seedSSHOnlyIdentity: WriteFile privKey: %v", err)
	}
	pubContent := fmt.Sprintf("ssh-ed25519 AAAAC3NzaC1lZDI1NTE5STUB %s@gitid-test\n", name)
	if err := os.WriteFile(pubKey, []byte(pubContent), 0o644); err != nil {
		t.Fatalf("seedSSHOnlyIdentity: WriteFile pubKey: %v", err)
	}

	sshConfig := filepath.Join(sshDir, "config")
	existing, _ := os.ReadFile(sshConfig) //nolint:gosec // fixed test sandbox path (G304)
	sshConfigContent := fmt.Sprintf(
		"%s# BEGIN gitid managed: %s\nHost %s.github.com\n  HostName github.com\n  User git\n  IdentityFile ~/.ssh/id_ed25519_%s\n  IdentitiesOnly yes\n# END gitid managed: %s\n",
		existing, name, name, name, name,
	)
	if err := os.WriteFile(sshConfig, []byte(sshConfigContent), 0o600); err != nil {
		t.Fatalf("seedSSHOnlyIdentity: WriteFile ssh/config: %v", err)
	}
}

// identityListDoc mirrors cmd/gitid's identityListDocument shape for
// unmarshaling — a separate, independently-typed struct so this test asserts
// against the WIRE format, not an imported cmd/gitid type.
type identityListDoc struct {
	Identities []identityRecordDoc `json:"identities"`
	UnusedKeys []string            `json:"unused_keys"`
}

type identityRecordDoc struct {
	Name          string   `json:"name"`
	IdentityState string   `json:"identity_state"`
	KeyState      string   `json:"key_state"`
	State         string   `json:"state"`
	Problems      []string `json:"problems"`
	Alias         string   `json:"alias"`
	Hostname      string   `json:"hostname"`
	Port          int      `json:"port"`
	KeyPath       string   `json:"key_path"`
	PubPath       string   `json:"pub_path"`
	FragmentPath  string   `json:"fragment_path"`
	Provider      string   `json:"provider"`
	ForceSSH      bool     `json:"force_ssh"`
	GitName       string   `json:"git_name"`
	GitEmail      string   `json:"git_email"`
	MatchStrategy string   `json:"match_strategy"`
	Complete      bool     `json:"complete"`
}

// runIdentityListJSON runs `gitid identity list --json` against home and
// returns the raw stdout bytes.
func runIdentityListJSON(t *testing.T, ctx context.Context, bin, home string) []byte {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, "identity", "list", "--json") //nolint:gosec // bin from BuildBinary; fixed args
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), "HOME="+home)
	if err := cmd.Run(); err != nil {
		t.Fatalf("gitid identity list --json failed: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}
	return stdout.Bytes()
}

// TestIdentityCLI_ListShow seeds a sandbox HOME with one complete identity
// and one SSH-only identity, runs `gitid identity list --json` via a plain
// exec.Command, unmarshals the output, and asserts the SSH-only record's Git
// fields are empty strings while the complete record's are populated (D-03).
func TestIdentityCLI_ListShow(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)
	seedMinimalIdentity(t, home, "complete")
	seedSSHOnlyIdentity(t, home, "sshonly")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	raw := runIdentityListJSON(t, ctx, bin, home)

	var doc identityListDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("json.Unmarshal: %v (raw: %s)", err, raw)
	}
	if len(doc.Identities) != 2 {
		t.Fatalf("identities = %d, want 2; raw: %s", len(doc.Identities), raw)
	}

	byName := map[string]identityRecordDoc{}
	for _, r := range doc.Identities {
		byName[r.Name] = r
	}

	complete, ok := byName["complete"]
	if !ok {
		t.Fatalf("missing the 'complete' identity record: %+v", doc.Identities)
	}
	if complete.GitName == "" || complete.GitEmail == "" || complete.FragmentPath == "" {
		t.Errorf("the complete identity's Git fields must be populated: %+v", complete)
	}

	sshOnly, ok := byName["sshonly"]
	if !ok {
		t.Fatalf("missing the 'sshonly' identity record: %+v", doc.Identities)
	}
	if sshOnly.GitName != "" || sshOnly.GitEmail != "" || sshOnly.MatchStrategy != "" || sshOnly.FragmentPath != "" {
		t.Errorf("the SSH-only identity's Git fields must all be empty strings, got: %+v", sshOnly)
	}

	if doc.UnusedKeys == nil {
		t.Error("unused_keys must be present (even if empty) in the decoded document")
	}

	// Run again and assert byte-identical stdout — MGR-08: reconstructed per
	// invocation, nothing cached or persisted between runs.
	second := runIdentityListJSON(t, ctx, bin, home)
	if string(raw) != string(second) {
		t.Errorf("two consecutive `identity list --json` invocations diverged:\nfirst:  %s\nsecond: %s", raw, second)
	}
}
