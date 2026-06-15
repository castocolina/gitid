package identity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/sshconfig"
)

// buildSSHBlock constructs the raw managed-block bytes for a single SSH identity.
// It wraps sshconfig.RenderHostBlock in the sentinel markers.
func buildSSHBlock(name, alias, hostname string, port int, identityFile string) string {
	body := sshconfig.RenderHostBlock(alias, hostname, port, identityFile)
	return "# BEGIN gitid managed: " + name + "\n" +
		body + "\n" +
		"# END gitid managed: " + name + "\n"
}

// buildGCBlock constructs the raw managed-block bytes for a single gitconfig
// includeIf identity.
func buildGCBlock(name, fragPath, gitdir string) string {
	return "# BEGIN gitid managed: " + name + "\n" +
		"[includeIf \"gitdir:" + gitdir + "\"]\n" +
		"\tpath = " + fragPath + "\n" +
		"# END gitid managed: " + name + "\n"
}

// TestReconstruct_Empty verifies that empty sshBytes and gcBytes return an
// empty (nil) slice with no error.
func TestReconstruct_Empty(t *testing.T) {
	got, err := Reconstruct([]byte(""), []byte(""), func(_ string) (gitconfig.FragmentInfo, error) {
		return gitconfig.FragmentInfo{Missing: true}, nil
	})
	if err != nil {
		t.Fatalf("Reconstruct on empty inputs returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty slice for empty inputs, got %d accounts", len(got))
	}
}

// TestReconstruct_Complete verifies that two complete identities are returned
// as two Accounts with correct fields and empty Incomplete markers.
func TestReconstruct_Complete(t *testing.T) {
	home := t.TempDir()

	sshContent := buildSSHBlock("personal", "personal.github.com", "ssh.github.com", 443,
		filepath.Join(home, ".ssh", "id_ed25519_personal"),
	) + buildSSHBlock("work", "work.github.com", "ssh.github.com", 22,
		filepath.Join(home, ".ssh", "id_ed25519_work"),
	)

	personalFrag := filepath.Join(home, ".gitconfig.d", "personal")
	workFrag := filepath.Join(home, ".gitconfig.d", "work")
	gcContent := buildGCBlock("personal", personalFrag, "~/git/personal/") +
		buildGCBlock("work", workFrag, "~/git/work/")

	readFrag := func(fragPath string) (gitconfig.FragmentInfo, error) {
		switch fragPath {
		case personalFrag:
			return gitconfig.FragmentInfo{
				GitName: "Personal User", GitEmail: "personal@example.com",
			}, nil
		case workFrag:
			return gitconfig.FragmentInfo{
				GitName: "Work User", GitEmail: "work@example.com",
			}, nil
		default:
			return gitconfig.FragmentInfo{Missing: true}, nil
		}
	}

	accounts, err := Reconstruct([]byte(sshContent), []byte(gcContent), readFrag)
	if err != nil {
		t.Fatalf("Reconstruct returned error: %v", err)
	}
	if len(accounts) != 2 {
		t.Fatalf("expected 2 accounts, got %d: %v", len(accounts), accounts)
	}

	byName := make(map[string]Account)
	for _, a := range accounts {
		byName[a.Name] = a
	}

	personal, ok := byName["personal"]
	if !ok {
		t.Fatal("missing 'personal' account")
	}
	if personal.Incomplete != "" {
		t.Errorf("personal Incomplete should be empty, got %q", personal.Incomplete)
	}
	if personal.Alias != "personal.github.com" {
		t.Errorf("personal Alias: got %q", personal.Alias)
	}
	if personal.GitName != "Personal User" {
		t.Errorf("personal GitName: got %q", personal.GitName)
	}
	if personal.GitEmail != "personal@example.com" {
		t.Errorf("personal GitEmail: got %q", personal.GitEmail)
	}

	work, ok := byName["work"]
	if !ok {
		t.Fatal("missing 'work' account")
	}
	if work.Incomplete != "" {
		t.Errorf("work Incomplete should be empty, got %q", work.Incomplete)
	}
}

// TestReconstruct_MissingSSH verifies that when the SSH block is absent for
// an identity present in gitconfig, the Account is returned with Incomplete
// containing "ssh-host-block".
func TestReconstruct_MissingSSH(t *testing.T) {
	workFrag := "~/.gitconfig.d/work"
	gcContent := buildGCBlock("work", workFrag, "~/git/work/")

	readFrag := func(_ string) (gitconfig.FragmentInfo, error) {
		return gitconfig.FragmentInfo{GitName: "Work User", GitEmail: "work@example.com"}, nil
	}

	accounts, err := Reconstruct([]byte(""), []byte(gcContent), readFrag)
	if err != nil {
		t.Fatalf("Reconstruct returned error: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accounts))
	}
	if accounts[0].Name != "work" {
		t.Errorf("expected name 'work', got %q", accounts[0].Name)
	}
	if !strings.Contains(accounts[0].Incomplete, "ssh-host-block") {
		t.Errorf("Incomplete should contain 'ssh-host-block', got %q", accounts[0].Incomplete)
	}
}

// TestReconstruct_MissingIncludeIf verifies that when the includeIf block is
// absent for an identity present in ssh config, the Account is returned with
// Incomplete containing "gitconfig-includeif-block".
func TestReconstruct_MissingIncludeIf(t *testing.T) {
	sshContent := buildSSHBlock("work", "work.github.com", "ssh.github.com", 22,
		"~/.ssh/id_ed25519_work",
	)

	readFrag := func(_ string) (gitconfig.FragmentInfo, error) {
		return gitconfig.FragmentInfo{Missing: true}, nil
	}

	accounts, err := Reconstruct([]byte(sshContent), []byte(""), readFrag)
	if err != nil {
		t.Fatalf("Reconstruct returned error: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accounts))
	}
	if !strings.Contains(accounts[0].Incomplete, "gitconfig-includeif-block") {
		t.Errorf("Incomplete should contain 'gitconfig-includeif-block', got %q", accounts[0].Incomplete)
	}
}

// TestReconstruct_MissingFragment verifies that when readFrag returns
// Missing=true the Account is returned with Incomplete containing "fragment-file".
func TestReconstruct_MissingFragment(t *testing.T) {
	sshContent := buildSSHBlock("work", "work.github.com", "ssh.github.com", 22,
		"~/.ssh/id_ed25519_work",
	)
	workFrag := "~/.gitconfig.d/work"
	gcContent := buildGCBlock("work", workFrag, "~/git/work/")

	readFrag := func(_ string) (gitconfig.FragmentInfo, error) {
		return gitconfig.FragmentInfo{Missing: true}, nil
	}

	accounts, err := Reconstruct([]byte(sshContent), []byte(gcContent), readFrag)
	if err != nil {
		t.Fatalf("Reconstruct returned error: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accounts))
	}
	if !strings.Contains(accounts[0].Incomplete, "fragment-file") {
		t.Errorf("Incomplete should contain 'fragment-file', got %q", accounts[0].Incomplete)
	}
}

// TestReconstruct_ProviderDerived verifies that when the alias is in the
// <name>.<provider> form, Provider is derived from the alias (A1).
func TestReconstruct_ProviderDerived(t *testing.T) {
	// Alias "work.github.com" for identity "work" → provider "github.com".
	sshContent := buildSSHBlock("work", "work.github.com", "ssh.github.com", 22,
		"~/.ssh/id_ed25519_work",
	)
	workFrag := "~/.gitconfig.d/work"
	gcContent := buildGCBlock("work", workFrag, "~/git/work/")
	readFrag := func(_ string) (gitconfig.FragmentInfo, error) {
		return gitconfig.FragmentInfo{GitName: "Work User", GitEmail: "work@example.com"}, nil
	}

	accounts, err := Reconstruct([]byte(sshContent), []byte(gcContent), readFrag)
	if err != nil {
		t.Fatalf("Reconstruct returned error: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accounts))
	}
	// TrimPrefix("work.github.com", "work.") = "github.com"
	if accounts[0].Provider != "github.com" {
		t.Errorf("Provider: got %q want %q", accounts[0].Provider, "github.com")
	}
}

// TestReconstruct_RoundTrip is the definitive IDENT-07 + TOOL-04 proof:
// writes two identities via the Phase 2 pipeline then reconstructs and asserts
// the []Account set matches the original inputs.
func TestReconstruct_RoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")

	sshDir := filepath.Join(home, ".ssh")
	gcDir := filepath.Join(home, ".gitconfig.d")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("mkdir sshDir: %v", err)
	}
	if err := os.MkdirAll(gcDir, 0o700); err != nil {
		t.Fatalf("mkdir gcDir: %v", err)
	}

	// Create fake .pub key files.
	personalPub := filepath.Join(sshDir, "id_ed25519_personal.pub")
	workPub := filepath.Join(sshDir, "id_ed25519_work.pub")
	if err := os.WriteFile(personalPub, []byte("ssh-ed25519 AAAAPERSONAL personal\n"), 0o600); err != nil {
		t.Fatalf("write personal pub: %v", err)
	}
	if err := os.WriteFile(workPub, []byte("ssh-ed25519 AAAAWORK work\n"), 0o600); err != nil {
		t.Fatalf("write work pub: %v", err)
	}

	sshConfigPath := filepath.Join(sshDir, "config")
	gitconfigPath := filepath.Join(home, ".gitconfig")
	personalFrag := filepath.Join(gcDir, "personal")
	workFrag := filepath.Join(gcDir, "work")

	// Write personal identity via Phase 2 pipeline.
	personalKeyPath := filepath.Join(sshDir, "id_ed25519_personal")
	personalHostBlock := sshconfig.RenderHostBlock(
		"personal.github.com", "ssh.github.com", 443, personalKeyPath,
	)
	if _, err := sshconfig.Write(sshConfigPath, "personal", personalHostBlock, ""); err != nil {
		t.Fatalf("sshconfig.Write personal: %v", err)
	}
	personalMatches := []gitconfig.Match{
		{Kind: gitconfig.MatchGitdir, Value: "~/git/personal/"},
	}
	if _, err := gitconfig.WriteIncludeIf(gitconfigPath, "personal", personalFrag, personalMatches); err != nil {
		t.Fatalf("gitconfig.WriteIncludeIf personal: %v", err)
	}
	if err := gitconfig.WriteFragment(personalFrag, "Personal User", "personal@example.com", personalPub); err != nil {
		t.Fatalf("gitconfig.WriteFragment personal: %v", err)
	}

	// Write work identity via Phase 2 pipeline.
	workKeyPath := filepath.Join(sshDir, "id_ed25519_work")
	workHostBlock := sshconfig.RenderHostBlock(
		"work.github.com", "ssh.github.com", 22, workKeyPath,
	)
	if _, err := sshconfig.Write(sshConfigPath, "work", workHostBlock, ""); err != nil {
		t.Fatalf("sshconfig.Write work: %v", err)
	}
	workMatches := []gitconfig.Match{
		{Kind: gitconfig.MatchGitdir, Value: "~/git/work/"},
	}
	if _, err := gitconfig.WriteIncludeIf(gitconfigPath, "work", workFrag, workMatches); err != nil {
		t.Fatalf("gitconfig.WriteIncludeIf work: %v", err)
	}
	if err := gitconfig.WriteFragment(workFrag, "Work User", "work@example.com", workPub); err != nil {
		t.Fatalf("gitconfig.WriteFragment work: %v", err)
	}

	// Read back the written files.
	sshBytes, err := os.ReadFile(sshConfigPath) //nolint:gosec // test reads back a controlled fixture path
	if err != nil {
		t.Fatalf("reading ssh config: %v", err)
	}
	gcBytes, err := os.ReadFile(gitconfigPath) //nolint:gosec // test reads back a controlled fixture path
	if err != nil {
		t.Fatalf("reading gitconfig: %v", err)
	}

	// Reconstruct using the real ReadFragment.
	accounts, err := Reconstruct(sshBytes, gcBytes, gitconfig.ReadFragment)
	if err != nil {
		t.Fatalf("Reconstruct returned error: %v", err)
	}
	if len(accounts) != 2 {
		t.Fatalf("expected 2 accounts after round-trip, got %d: %v", len(accounts), accounts)
	}

	byName := make(map[string]Account)
	for _, a := range accounts {
		byName[a.Name] = a
	}

	for _, name := range []string{"personal", "work"} {
		acct, ok := byName[name]
		if !ok {
			t.Errorf("account %q missing from reconstruction", name)
			continue
		}
		if acct.Incomplete != "" {
			t.Errorf("account %q has Incomplete=%q, expected empty", name, acct.Incomplete)
		}
		if acct.Alias == "" {
			t.Errorf("account %q has empty Alias", name)
		}
		if acct.FragmentPath == "" {
			t.Errorf("account %q has empty FragmentPath", name)
		}
		if acct.GitName == "" {
			t.Errorf("account %q has empty GitName", name)
		}
		if acct.GitEmail == "" {
			t.Errorf("account %q has empty GitEmail", name)
		}
	}

	personal := byName["personal"]
	if personal.Alias != "personal.github.com" {
		t.Errorf("personal Alias: got %q want personal.github.com", personal.Alias)
	}
	if personal.Port != 443 {
		t.Errorf("personal Port: got %d want 443", personal.Port)
	}
	if personal.GitName != "Personal User" {
		t.Errorf("personal GitName: got %q want 'Personal User'", personal.GitName)
	}
	if personal.GitEmail != "personal@example.com" {
		t.Errorf("personal GitEmail: got %q want 'personal@example.com'", personal.GitEmail)
	}
	if personal.KeyPath != personalKeyPath {
		t.Errorf("personal KeyPath: got %q want %q", personal.KeyPath, personalKeyPath)
	}
	if personal.PubPath != personalKeyPath+".pub" {
		t.Errorf("personal PubPath: got %q want %q", personal.PubPath, personalKeyPath+".pub")
	}

	work := byName["work"]
	if work.Alias != "work.github.com" {
		t.Errorf("work Alias: got %q want work.github.com", work.Alias)
	}
	if work.GitName != "Work User" {
		t.Errorf("work GitName: got %q want 'Work User'", work.GitName)
	}
}
