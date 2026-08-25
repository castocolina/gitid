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
	body := sshconfig.RenderHostBlock(alias, hostname, port, identityFile, "")
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

// TestReconstruct_FragmentPathTildeExpansion proves WR-02: gitid always
// writes includeIf `path =` as "~/.gitconfig.d/<name>" (IncludeIfPreview,
// WriteIncludeIf), and git itself expands "~" when resolving includeIf at
// runtime — but Reconstruct's own readFrag callback receives the path
// directly via os.Stat/exec, which never expands "~". Without expansion,
// every real identity's own fragment read-back fails and GitName/GitEmail
// never populate, permanently misclassifying every complete identity as
// SSH-only (surfaced by the D-07 collision-message branch in Phase 4).
func TestReconstruct_FragmentPathTildeExpansion(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	sshContent := buildSSHBlock("acme", "acme.github.com", "ssh.github.com", 443,
		filepath.Join(home, ".ssh", "id_ed25519_acme"),
	)
	gcContent := buildGCBlock("acme", "~/.gitconfig.d/acme", "~/git/acme/")

	expandedFrag := filepath.Join(home, ".gitconfig.d", "acme")
	readFrag := func(fragPath string) (gitconfig.FragmentInfo, error) {
		if fragPath != expandedFrag {
			// Prove the callback never receives the literal, unexpanded
			// "~/..." form.
			return gitconfig.FragmentInfo{Missing: true}, nil
		}
		return gitconfig.FragmentInfo{GitName: "Acme User", GitEmail: "acme@example.com"}, nil
	}

	accounts, err := Reconstruct([]byte(sshContent), []byte(gcContent), readFrag)
	if err != nil {
		t.Fatalf("Reconstruct returned error: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account, got %d: %v", len(accounts), accounts)
	}
	acme := accounts[0]
	if acme.Incomplete != "" {
		t.Errorf("Incomplete should be empty, got %q", acme.Incomplete)
	}
	if acme.GitName != "Acme User" {
		t.Errorf("GitName: got %q, want %q", acme.GitName, "Acme User")
	}
	if acme.GitEmail != "acme@example.com" {
		t.Errorf("GitEmail: got %q, want %q", acme.GitEmail, "acme@example.com")
	}
	// FragmentPath itself stays verbatim ("~/...") — callers (display,
	// re-derived write targets) depend on the original parsed form.
	if acme.FragmentPath != "~/.gitconfig.d/acme" {
		t.Errorf("FragmentPath should stay verbatim, got %q", acme.FragmentPath)
	}
}

// TestReconstruct_MissingSSH verifies that when the SSH block is absent for
// an identity present in gitconfig, the Account is returned with Incomplete
// containing "ssh-host-block".
func TestReconstruct_LoadProviderRewrite(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	sshContent := buildSSHBlock("work", "work.github.com", "ssh.github.com", 443, "~/.ssh/id_ed25519_work")
	workFrag := "~/.gitconfig.d/work"
	// Reconstruct expands "~" before calling readFrag (WR-02) — the callback
	// never sees the literal, unexpanded form. See loader.go and
	// TestReconstruct_FragmentPathTildeExpansion.
	wantReadPath := filepath.Join(home, ".gitconfig.d", "work")
	rewriteName, err := gitconfig.ProviderRewriteBlockName("github.com")
	if err != nil {
		t.Fatalf("ProviderRewriteBlockName: %v", err)
	}
	rewriteBody, err := gitconfig.RenderProviderRewrite("github.com")
	if err != nil {
		t.Fatalf("RenderProviderRewrite: %v", err)
	}
	gcContent := buildGCBlock("work", workFrag, "~/git/work/") +
		"# BEGIN gitid managed: " + rewriteName + "\n" + rewriteBody + "\n# END gitid managed: " + rewriteName + "\n"

	accounts, err := Reconstruct([]byte(sshContent), []byte(gcContent), func(path string) (gitconfig.FragmentInfo, error) {
		if path != wantReadPath {
			t.Fatalf("ReadFragment path = %q, want %q", path, wantReadPath)
		}
		return gitconfig.FragmentInfo{GitName: "Work User", GitEmail: "work@example.com"}, nil
	})
	if err != nil {
		t.Fatalf("Reconstruct: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("account count = %d, want 1: %v", len(accounts), accounts)
	}
	if accounts[0].Name != "work" {
		t.Errorf("account name = %q, want work", accounts[0].Name)
	}
	// CR-09: the provider-rewrite block IS present in gcContent -- ForceSSH
	// must reflect that real on-disk state, not a default.
	if !accounts[0].ForceSSH {
		t.Error("ForceSSH = false, want true — the provider-rewrite block is present in the parsed gitconfig bytes")
	}
}

// TestReconstruct_ForceSSHFalseWithoutRewriteBlock is CR-09's negative
// counterpart to TestReconstruct_LoadProviderRewrite: the SAME identity,
// same provider, but with NO provider-rewrite block in ~/.gitconfig — proves
// ForceSSH is read from the real bytes each time, not defaulted true
// whenever a Provider happens to be set.
func TestReconstruct_ForceSSHFalseWithoutRewriteBlock(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	sshContent := buildSSHBlock("work", "work.github.com", "ssh.github.com", 443, "~/.ssh/id_ed25519_work")
	workFrag := "~/.gitconfig.d/work"
	gcContent := buildGCBlock("work", workFrag, "~/git/work/") // no provider-rewrite block

	accounts, err := Reconstruct([]byte(sshContent), []byte(gcContent), func(string) (gitconfig.FragmentInfo, error) {
		return gitconfig.FragmentInfo{GitName: "Work User", GitEmail: "work@example.com"}, nil
	})
	if err != nil {
		t.Fatalf("Reconstruct: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("account count = %d, want 1: %v", len(accounts), accounts)
	}
	if accounts[0].ForceSSH {
		t.Error("ForceSSH = true, want false — no provider-rewrite block exists in the parsed gitconfig bytes")
	}
}

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

// TestReconstruct_ProviderFromMarker verifies D-11/D-12: when the SSH block
// contains a "# gitid: provider=github" marker, Provider comes from the marker
// regardless of the alias shape (F-3 regression guard).
func TestReconstruct_ProviderFromMarker(t *testing.T) {
	// Alias shape does NOT match <name>.<provider> — but marker is present.
	body := sshconfig.RenderHostBlock("userz3r0.personal.github", "ssh.github.com", 443, "~/.ssh/id_ed25519_userz3r0", "github")
	sshContent := "# BEGIN gitid managed: userz3r0\n" + body + "\n# END gitid managed: userz3r0\n"
	workFrag := "~/.gitconfig.d/userz3r0"
	gcContent := buildGCBlock("userz3r0", workFrag, "~/git/personal/")
	readFrag := func(_ string) (gitconfig.FragmentInfo, error) {
		return gitconfig.FragmentInfo{GitName: "User Z3r0", GitEmail: "user@example.com"}, nil
	}

	accounts, err := Reconstruct([]byte(sshContent), []byte(gcContent), readFrag)
	if err != nil {
		t.Fatalf("Reconstruct returned error: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accounts))
	}
	if accounts[0].Provider != "github" {
		t.Errorf("Provider from marker: got %q want %q", accounts[0].Provider, "github")
	}
}

// TestReconstruct_ProviderFromHostnameMap verifies D-12 (the F-3 regression fix):
// a markerless block whose Hostname is "ssh.github.com" resolves to provider
// "github" via the hostname map — NOT "ssh.github.com" (the old TrimPrefix bug).
func TestReconstruct_ProviderFromHostnameMap(t *testing.T) {
	// Markerless block — uses legacy buildSSHBlock which passes "" as provider.
	sshContent := buildSSHBlock("userz3r0_gh", "userz3r0.personal.github", "ssh.github.com", 443,
		"~/.ssh/id_ed25519_userz3r0",
	)
	workFrag := "~/.gitconfig.d/userz3r0_gh"
	gcContent := buildGCBlock("userz3r0_gh", workFrag, "~/git/personal/")
	readFrag := func(_ string) (gitconfig.FragmentInfo, error) {
		return gitconfig.FragmentInfo{GitName: "User Z3r0", GitEmail: "user@example.com"}, nil
	}

	accounts, err := Reconstruct([]byte(sshContent), []byte(gcContent), readFrag)
	if err != nil {
		t.Fatalf("Reconstruct returned error: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accounts))
	}
	// Must be "github" (from hostname map), NOT "ssh.github.com" (F-3 bug)
	// and NOT "personal.github" (from TrimPrefix of "userz3r0.personal.github").
	if accounts[0].Provider != "github" {
		t.Errorf("Provider from hostname map: got %q want %q", accounts[0].Provider, "github")
	}
}

// TestReconstruct_ProviderMarkerWinsOverHostnameMap verifies D-12: when both a
// marker AND a known hostname are present, the marker takes precedence.
func TestReconstruct_ProviderMarkerWinsOverHostnameMap(t *testing.T) {
	// Marker says "gitlab" but hostname is ssh.github.com → marker wins.
	body := sshconfig.RenderHostBlock("work.github.com", "ssh.github.com", 443, "~/.ssh/id_ed25519_work", "gitlab")
	sshContent := "# BEGIN gitid managed: work\n" + body + "\n# END gitid managed: work\n"
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
	// Marker "gitlab" must win over hostname map "github".
	if accounts[0].Provider != "gitlab" {
		t.Errorf("Provider: marker should win over hostname map; got %q want %q", accounts[0].Provider, "gitlab")
	}
}

// TestReconstruct_ProviderUnknownHostname verifies D-13: a markerless block
// with an unknown hostname leaves Provider empty — no crash, honest unknown.
func TestReconstruct_ProviderUnknownHostname(t *testing.T) {
	sshContent := buildSSHBlock("mywork", "mywork.git.example.com", "git.example.com", 22,
		"~/.ssh/id_ed25519_mywork",
	)
	workFrag := "~/.gitconfig.d/mywork"
	gcContent := buildGCBlock("mywork", workFrag, "~/git/mywork/")
	readFrag := func(_ string) (gitconfig.FragmentInfo, error) {
		return gitconfig.FragmentInfo{GitName: "My Work", GitEmail: "mywork@example.com"}, nil
	}

	accounts, err := Reconstruct([]byte(sshContent), []byte(gcContent), readFrag)
	if err != nil {
		t.Fatalf("Reconstruct returned error: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accounts))
	}
	if accounts[0].Provider != "" {
		t.Errorf("Provider for unknown hostname: got %q want empty string", accounts[0].Provider)
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
		"personal.github.com", "ssh.github.com", 443, personalKeyPath, "",
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
	if err := gitconfig.WriteFragment(personalFrag, "Personal User", "personal@example.com", personalPub, true); err != nil {
		t.Fatalf("gitconfig.WriteFragment personal: %v", err)
	}

	// Write work identity via Phase 2 pipeline.
	workKeyPath := filepath.Join(sshDir, "id_ed25519_work")
	workHostBlock := sshconfig.RenderHostBlock(
		"work.github.com", "ssh.github.com", 22, workKeyPath, "",
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
	if err := gitconfig.WriteFragment(workFrag, "Work User", "work@example.com", workPub, true); err != nil {
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

// ---------------------------------------------------------------------------
// 05-04 — provider normalization (RewriteProviderKey, ProviderHostForSSHHostname,
// ProviderKeyForHost, ProviderRefCount)
// ---------------------------------------------------------------------------

// TestRewriteProviderKey covers the three-branch precedence, including the
// review R2-11 short-provider/short-alias correction.
func TestRewriteProviderKey(t *testing.T) {
	cases := []struct {
		name     string
		provider string
		alias    string
		want     string
	}{
		{"already dotted provider used as-is", "github.com", "work.github.com", "github.com"},
		{"short provider resolves via table (R2-11)", "github", "work.github.com", "github.com"},
		{"short provider + short alias still resolves (R2-11 regression)", "github", "mygh", "github.com"},
		{"gitlab short form", "gitlab", "x", "gitlab.com"},
		{"bitbucket short form", "bitbucket", "x", "bitbucket.org"},
		{"empty provider falls to alias suffix", "", "work.github.com", "github.com"},
		{"empty provider, two-label alias returns alias verbatim", "", "github.com", "github.com"},
		{"empty provider, empty alias returns empty", "", "", ""},
		{"unknown short provider falls through to alias suffix", "customcorp", "work.git.example.com", "git.example.com"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RewriteProviderKey(tc.provider, tc.alias)
			if got != tc.want {
				t.Errorf("RewriteProviderKey(%q, %q) = %q, want %q", tc.provider, tc.alias, got, tc.want)
			}
		})
	}
}

// TestProviderHostForSSHHostname pins every recognized alt-SSH/bare hostname
// (including the Bitbucket pair review R2-04 adds) and the honest-unknown
// empty-string case.
func TestProviderHostForSSHHostname(t *testing.T) {
	cases := []struct {
		hostname string
		want     string
	}{
		{"ssh.github.com", "github.com"},
		{"github.com", "github.com"},
		{"altssh.gitlab.com", "gitlab.com"},
		{"gitlab.com", "gitlab.com"},
		{"altssh.bitbucket.org", "bitbucket.org"},
		{"bitbucket.org", "bitbucket.org"},
		{"git.example.internal", ""},
		{"", ""},
	}
	for _, tc := range cases {
		got := ProviderHostForSSHHostname(tc.hostname)
		if got != tc.want {
			t.Errorf("ProviderHostForSSHHostname(%q) = %q, want %q", tc.hostname, got, tc.want)
		}
	}
}

// TestProviderKeyForHost pins the documented precedence: marker, then
// hostname, then alias-suffix fallback — including the exact recipe-
// canonical regression cases review R-05 and its Bitbucket twin (R2-04) name.
func TestProviderKeyForHost(t *testing.T) {
	cases := []struct {
		name     string
		alias    string
		hostname string
		marker   string
		want     string
	}{
		{"explicit marker wins", "foo", "ssh.github.com", "gitlab.com", "gitlab.com"},
		{"ssh.github.com + dotted alias (R-05 regression)", "foo.github.com", "ssh.github.com", "", "github.com"},
		{"ssh.github.com + dotless alias (R2-11 regression)", "mygh", "ssh.github.com", "", "github.com"},
		{"altssh.gitlab.com", "x.gitlab.com", "altssh.gitlab.com", "", "gitlab.com"},
		{"altssh.bitbucket.org (R2-04 Bitbucket twin)", "foo.bitbucket.org", "altssh.bitbucket.org", "", "bitbucket.org"},
		{"bare bitbucket.org hostname", "foo.bitbucket.org", "bitbucket.org", "", "bitbucket.org"},
		{"unrecognized hostname falls to alias suffix", "work.custom.example.com", "git.example.internal", "", "custom.example.com"},
		{"unrecognized hostname, no usable alias", "custom", "git.example.internal", "", "custom"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ProviderKeyForHost(tc.alias, tc.hostname, tc.marker)
			if got != tc.want {
				t.Errorf("ProviderKeyForHost(%q, %q, %q) = %q, want %q", tc.alias, tc.hostname, tc.marker, got, tc.want)
			}
		})
	}
}

// TestProviderRefCount_ShortFormProviderCounted asserts that ProviderRefCount
// counts an account whose reconstructed Provider is the SHORT form ("github")
// as a reference to the "github.com" key — both sides of the comparison are
// normalized.
func TestProviderRefCount_ShortFormProviderCounted(t *testing.T) {
	accounts := []Account{
		{Name: "work", Provider: "github", Alias: "work.github.com"},
		{Name: "personal", Provider: "github.com", Alias: "personal.github.com"},
	}
	got := ProviderRefCount(accounts, "github.com", "personal")
	if got != 1 {
		t.Errorf("ProviderRefCount = %d, want 1 (the short-form 'work' account)", got)
	}
}

// TestProviderRefCount_ShortProviderShortAlias is the R2-11 sibling proof:
// an account whose Provider is short AND whose Alias is also dotless still
// counts as a "github.com" reference (not silently dropped).
func TestProviderRefCount_ShortProviderShortAlias(t *testing.T) {
	accounts := []Account{
		{Name: "mygh-account", Provider: "github", Alias: "mygh"},
	}
	got := ProviderRefCount(accounts, "github.com", "someone-else")
	if got != 1 {
		t.Errorf("ProviderRefCount = %d, want 1 (short-provider/short-alias account)", got)
	}
}

// TestProviderRefCount_ExcludesSelf asserts the excludingName account is
// never counted, even when it matches providerKey.
func TestProviderRefCount_ExcludesSelf(t *testing.T) {
	accounts := []Account{
		{Name: "work", Provider: "github.com", Alias: "work.github.com"},
	}
	got := ProviderRefCount(accounts, "github.com", "work")
	if got != 0 {
		t.Errorf("ProviderRefCount = %d, want 0 (self excluded)", got)
	}
}

// wizardProviderShortForms is the domain-side mirror of
// tuikit.wizardProviders' three FQDN entries, expressed as (short, FQDN)
// pairs matching DefaultHostname's own provider switch (identity.go) — the
// "providerHostname" table review R2-04's acceptance criterion names.
// internal/identity must not import internal/tuikit (layering), so this
// table is declared here, independently, and TestProviderTableRoundTripsWizardProviders
// asserts it covers every case DefaultHostname's switch handles, so a fourth
// provider added to DefaultHostname without a matching entry here fails this
// test.
var wizardProviderShortForms = []struct {
	short  string
	altSSH string
	fqdn   string
}{
	{"github", "ssh.github.com", "github.com"},
	{"gitlab", "altssh.gitlab.com", "gitlab.com"},
	{"bitbucket", "altssh.bitbucket.org", "bitbucket.org"},
}

// TestProviderTableRoundTripsWizardProviders is the review R2-04 coverage
// gate: for every provider DefaultHostname's own switch handles (github,
// gitlab, bitbucket), BOTH that function's returned alt-SSH endpoint and the
// bare FQDN round-trip through ProviderHostForSSHHostname to the same FQDN
// key — so a provider the product can create can never be silently absent
// from the D-09 count path.
func TestProviderTableRoundTripsWizardProviders(t *testing.T) {
	if len(wizardProviderShortForms) != 3 {
		t.Fatalf("wizardProviderShortForms has %d entries, want 3 (github, gitlab, bitbucket)", len(wizardProviderShortForms))
	}
	for _, tc := range wizardProviderShortForms {
		t.Run(tc.short, func(t *testing.T) {
			// DefaultHostname's own answer for this short provider must be the
			// alt-SSH endpoint this table expects.
			if got := DefaultHostname(tc.short); got != tc.altSSH {
				t.Fatalf("DefaultHostname(%q) = %q, want %q (table/switch drift)", tc.short, got, tc.altSSH)
			}
			// The alt-SSH endpoint round-trips to the FQDN key.
			if got := ProviderHostForSSHHostname(tc.altSSH); got != tc.fqdn {
				t.Errorf("ProviderHostForSSHHostname(%q) = %q, want %q", tc.altSSH, got, tc.fqdn)
			}
			// The bare FQDN hostname round-trips to itself.
			if got := ProviderHostForSSHHostname(tc.fqdn); got != tc.fqdn {
				t.Errorf("ProviderHostForSSHHostname(%q) = %q, want %q", tc.fqdn, got, tc.fqdn)
			}
		})
	}
}
