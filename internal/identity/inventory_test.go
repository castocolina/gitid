package identity

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/sshconfig"
)

// buildInventoryFixture assembles the fake-deps multi-identity fixture used by
// TestBuildInventory: five identities exercising every corner BuildInventory
// must resolve (a fully-wired identity, an SSH-only-signing identity, an
// identity missing its gitconfig side, a git-only identity with no Host
// block, and a fragment-path-missing identity), plus one on-disk key file
// that is referenced by no Host block anywhere (the global unused-key case).
func buildInventoryFixture() InventoryDeps {
	const (
		completeKey         = "/keys/complete"
		sshonlyKey          = "/keys/sshonly"
		incompleteKey       = "/keys/incomplete"
		fragmentmissingKey  = "/keys/fragmentmissing"
		orphanKey           = "/keys/orphan"
		completeFrag        = "/frags/complete"
		sshonlyFrag         = "/frags/sshonly"
		gitonlyFrag         = "/frags/gitonly"
		fragmentmissingFrag = "/frags/fragmentmissing"
	)

	sshContent := buildSSHBlock("complete", "complete.github.com", "ssh.github.com", 443, completeKey) +
		buildSSHBlock("sshonly", "sshonly.github.com", "ssh.github.com", 443, sshonlyKey) +
		buildSSHBlock("incomplete", "incomplete.github.com", "ssh.github.com", 443, incompleteKey) +
		buildSSHBlock("fragmentmissing", "fragmentmissing.github.com", "ssh.github.com", 443, fragmentmissingKey)

	gcContent := buildGCBlock("complete", completeFrag, "~/git/complete/") +
		buildGCBlock("sshonly", sshonlyFrag, "~/git/sshonly/") +
		buildGCBlock("gitonly", gitonlyFrag, "~/git/gitonly/") +
		buildGCBlock("fragmentmissing", fragmentmissingFrag, "~/git/fragmentmissing/")

	readFrag := func(fragPath string) (gitconfig.FragmentInfo, error) {
		switch fragPath {
		case completeFrag:
			return gitconfig.FragmentInfo{
				GitName: "Complete User", GitEmail: "complete@example.com",
				SigningKey: completeKey + ".pub", GPGFormat: "ssh", CommitSign: true,
			}, nil
		case sshonlyFrag:
			// Present, but signing is not enabled — SSH-only key usage.
			return gitconfig.FragmentInfo{GitName: "SSH Only User", GitEmail: "sshonly@example.com"}, nil
		case gitonlyFrag:
			return gitconfig.FragmentInfo{GitName: "Git Only User", GitEmail: "gitonly@example.com"}, nil
		case fragmentmissingFrag:
			return gitconfig.FragmentInfo{Missing: true}, nil
		default:
			return gitconfig.FragmentInfo{Missing: true}, nil
		}
	}

	existing := map[string]bool{
		completeKey: true, sshonlyKey: true, incompleteKey: true, fragmentmissingKey: true,
	}
	statFn := func(path string) (os.FileInfo, error) {
		if existing[path] {
			return nil, nil
		}
		return nil, os.ErrNotExist
	}

	return InventoryDeps{
		ReadSSHConfig: func() ([]byte, error) { return []byte(sshContent), nil },
		ReadGitconfig: func() ([]byte, error) { return []byte(gcContent), nil },
		ReadFragment:  readFrag,
		Stat:          statFn,
		ListKeyFiles: func() ([]string, error) {
			return []string{completeKey, sshonlyKey, incompleteKey, fragmentmissingKey, orphanKey}, nil
		},
	}
}

// TestBuildInventory drives the builder with all-fake deps over the
// multi-identity fixture and asserts per-identity IdentityHealth (both axes)
// plus the global UnusedKeys set — including an unused-key case (the orphan
// key referenced by no Host block) and a fragment-path-missing case.
func TestBuildInventory(t *testing.T) {
	inv, err := BuildInventory(buildInventoryFixture())
	if err != nil {
		t.Fatalf("BuildInventory returned error: %v", err)
	}
	if len(inv.Identities) != 5 {
		t.Fatalf("expected 5 identities, got %d: %+v", len(inv.Identities), inv.Identities)
	}

	byName := make(map[string]IdentityHealth, len(inv.Identities))
	for _, h := range inv.Identities {
		byName[h.Name] = h
	}

	cases := []struct {
		name         string
		wantIdentity State
		wantKey      State
	}{
		{"complete", StateComplete, StateKeyUsedBoth},
		{"sshonly", StateComplete, StateKeyUsedSSHOnly},
		{"incomplete", StateIncomplete, StateKeyUsedSSHOnly},
		{"gitonly", StateGitOnly, StateKeyMissing},
		{"fragmentmissing", StateFragmentPathMissing, StateKeyUsedSSHOnly},
	}
	for _, c := range cases {
		h, ok := byName[c.name]
		if !ok {
			t.Errorf("identity %q missing from inventory", c.name)
			continue
		}
		if h.IdentityState != c.wantIdentity {
			t.Errorf("%s IdentityState: got %q want %q", c.name, h.IdentityState, c.wantIdentity)
		}
		if h.KeyState != c.wantKey {
			t.Errorf("%s KeyState: got %q want %q", c.name, h.KeyState, c.wantKey)
		}
	}

	// Global unused-key case: /keys/orphan is on disk (per ListKeyFiles) but
	// referenced by no Host block anywhere.
	if len(inv.UnusedKeys) != 1 || inv.UnusedKeys[0] != "/keys/orphan" {
		t.Errorf("UnusedKeys: got %v want [/keys/orphan]", inv.UnusedKeys)
	}
}

// TestBuildInventory_ReadSSHConfigError verifies a ReadSSHConfig failure is
// wrapped and surfaced rather than silently swallowed.
func TestBuildInventory_ReadSSHConfigError(t *testing.T) {
	deps := buildInventoryFixture()
	wantErr := errors.New("boom")
	deps.ReadSSHConfig = func() ([]byte, error) { return nil, wantErr }

	_, err := BuildInventory(deps)
	if err == nil {
		t.Fatal("expected an error when ReadSSHConfig fails, got nil")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("expected wrapped error to unwrap to %v, got %v", wantErr, err)
	}
}

// TestBuildInventory_ListKeyFilesError verifies a ListKeyFiles failure is
// wrapped and surfaced rather than silently swallowed.
func TestBuildInventory_ListKeyFilesError(t *testing.T) {
	deps := buildInventoryFixture()
	wantErr := errors.New("boom")
	deps.ListKeyFiles = func() ([]string, error) { return nil, wantErr }

	_, err := BuildInventory(deps)
	if err == nil {
		t.Fatal("expected an error when ListKeyFiles fails, got nil")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("expected wrapped error to unwrap to %v, got %v", wantErr, err)
	}
}

// TestBuildInventoryDeps asserts every function field of the real
// BuildInventoryDeps() constructor is non-nil, closing the project's
// documented injected-seam wiring blindspot (a broken/nil real seam must not
// silently pass on green fakes).
func TestBuildInventoryDeps(t *testing.T) {
	deps := BuildInventoryDeps()
	if deps.ReadSSHConfig == nil {
		t.Error("BuildInventoryDeps().ReadSSHConfig is nil")
	}
	if deps.ReadGitconfig == nil {
		t.Error("BuildInventoryDeps().ReadGitconfig is nil")
	}
	if deps.ReadFragment == nil {
		t.Error("BuildInventoryDeps().ReadFragment is nil")
	}
	if deps.Stat == nil {
		t.Error("BuildInventoryDeps().Stat is nil")
	}
	if deps.ListKeyFiles == nil {
		t.Error("BuildInventoryDeps().ListKeyFiles is nil")
	}
}

// TestBuildInventoryIncludeLayout is the D-11 proof: it seeds an identity
// whose managed SSH block lives ONLY in ~/.ssh/config.d/gitid.config (the
// main ~/.ssh/config carries just the gitid-owned Include line — the
// STORE-01 layout), under a t.TempDir() HOME sandbox, then drives the REAL
// BuildInventoryDeps() + BuildInventory and asserts the identity is
// classified — proving the Include'd layout is not carved out.
func TestBuildInventoryIncludeLayout(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")

	sshDir := filepath.Join(home, ".ssh")
	configDir := filepath.Join(sshDir, "config.d")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("mkdir config.d: %v", err)
	}
	gcDir := filepath.Join(home, ".gitconfig.d")
	if err := os.MkdirAll(gcDir, 0o700); err != nil {
		t.Fatalf("mkdir gitconfig.d: %v", err)
	}

	// Main ~/.ssh/config carries ONLY the Include line — the managed
	// identity block lives exclusively in config.d/gitid.config.
	if err := os.WriteFile(filepath.Join(sshDir, "config"), []byte("Include ~/.ssh/config.d/*.config\n"), 0o600); err != nil {
		t.Fatalf("write ssh config: %v", err)
	}

	keyPath := filepath.Join(sshDir, "id_ed25519_included")
	if err := os.WriteFile(keyPath, []byte("fake-private-key-material\n"), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	pubPath := keyPath + ".pub"
	if err := os.WriteFile(pubPath, []byte("ssh-ed25519 AAAAINCLUDED included@gitid\n"), 0o600); err != nil {
		t.Fatalf("write pub: %v", err)
	}

	hostBody := sshconfig.RenderHostBlock("included.github.com", "ssh.github.com", 443, keyPath, "")
	sshBlock := "# BEGIN gitid managed: included\n" + hostBody + "\n# END gitid managed: included\n"
	if err := os.WriteFile(filepath.Join(configDir, "gitid.config"), []byte(sshBlock), 0o600); err != nil {
		t.Fatalf("write config.d block: %v", err)
	}

	gitconfigPath := filepath.Join(home, ".gitconfig")
	fragPath := filepath.Join(gcDir, "included")
	matches := []gitconfig.Match{{Kind: gitconfig.MatchGitdir, Value: "~/git/included/"}}
	if _, err := gitconfig.WriteIncludeIf(gitconfigPath, "included", fragPath, matches); err != nil {
		t.Fatalf("WriteIncludeIf: %v", err)
	}
	if err := gitconfig.WriteFragment(fragPath, "Included User", "included@example.com", pubPath, true); err != nil {
		t.Fatalf("WriteFragment: %v", err)
	}

	deps := BuildInventoryDeps()
	inv, err := BuildInventory(deps)
	if err != nil {
		t.Fatalf("BuildInventory returned error: %v", err)
	}

	var found *IdentityHealth
	for i := range inv.Identities {
		if inv.Identities[i].Name == "included" {
			found = &inv.Identities[i]
		}
	}
	if found == nil {
		t.Fatalf("identity 'included' not found in inventory (Include'd config.d layout not classified — D-11 violation): %+v", inv.Identities)
	}
	if found.IdentityState != StateComplete {
		t.Errorf("included IdentityState: got %q want %q", found.IdentityState, StateComplete)
	}
	if found.KeyState != StateKeyUsedBoth {
		t.Errorf("included KeyState: got %q want %q", found.KeyState, StateKeyUsedBoth)
	}
}
