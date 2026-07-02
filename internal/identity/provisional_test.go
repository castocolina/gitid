package identity_test

import (
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
)

// -------------------------------------------------------------------------------
// EffectiveAlias tests
// -------------------------------------------------------------------------------

// TestEffectiveAlias covers the full table of inputs per the plan spec:
//   - blank alias + full provider host → provider host returned verbatim
//   - blank alias + short provider token → token + ".com"
//   - explicit alias (non-empty, possibly padded) → trimmed alias
func TestEffectiveAlias(t *testing.T) {
	cases := []struct {
		name     string
		alias    string
		provider string
		want     string
	}{
		{
			name:     "blank alias with dotted provider returns provider host",
			alias:    "",
			provider: "github.com",
			want:     "github.com",
		},
		{
			name:     "blank alias with short token returns token.com",
			alias:    "",
			provider: "github",
			want:     "github.com",
		},
		{
			name:     "explicit alias returned verbatim",
			alias:    "personal.github.com",
			provider: "github.com",
			want:     "personal.github.com",
		},
		{
			name:     "padded alias is trimmed",
			alias:    "  x  ",
			provider: "github.com",
			want:     "x",
		},
		{
			name:     "blank alias with gitlab.com returns gitlab.com",
			alias:    "",
			provider: "gitlab.com",
			want:     "gitlab.com",
		},
		{
			name:     "blank alias with bitbucket returns bitbucket.com",
			alias:    "",
			provider: "bitbucket",
			want:     "bitbucket.com",
		},
		{
			name:     "blank alias with custom dotted provider returned verbatim (lowercased)",
			alias:    "",
			provider: "corp.example.com",
			want:     "corp.example.com",
		},
		{
			name:     "blank alias + whitespace-only returns provider host",
			alias:    "   ",
			provider: "github.com",
			want:     "github.com",
		},
		{
			name:     "explicit alias not an invented name.provider form",
			alias:    "mywork",
			provider: "github.com",
			want:     "mywork",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := identity.EffectiveAlias(tc.alias, tc.provider)
			if got != tc.want {
				t.Errorf("EffectiveAlias(%q, %q) = %q, want %q", tc.alias, tc.provider, got, tc.want)
			}
		})
	}
}

// TestEffectiveAlias_NeverInventedNameProviderSuffix verifies that EffectiveAlias
// never returns the <name>.<provider> form — the WYSIWYG blank → provider host
// rule must not fall back to DefaultAlias.
func TestEffectiveAlias_NeverInventedNameProviderSuffix(t *testing.T) {
	got := identity.EffectiveAlias("", "github.com")
	if strings.Contains(got, ".") && !strings.HasSuffix(got, ".com") && !strings.EqualFold(got, "github.com") {
		t.Errorf("EffectiveAlias returned an invented name.provider form: %q", got)
	}
	// The most important check: blank alias must not produce something like
	// "<anything>.github.com" that looks like DefaultAlias("<identity>", "github.com").
	if got != "github.com" {
		t.Errorf("blank alias + 'github.com' must return 'github.com', got %q", got)
	}
}

// -------------------------------------------------------------------------------
// Provisional identity lifecycle tests (call-log fakes)
// -------------------------------------------------------------------------------

// callLog records which seam functions were called and with which args.
type callLog struct {
	writeProvisionalCalls []provisionalCallArgs
	promoteSSHCalls       []provisionalCallArgs
	dropProvisionalCalls  []string // just the name arg
	persistKeyCalls       int
	writeSSHCalls         int
	writeGitconfigCalls   int
}

type provisionalCallArgs struct {
	name      string
	hostBlock string
}

// fakeDepsForProvisional builds an identity.Deps with call-log fakes wired into
// the three provisional seams and the key write seams.
func fakeDepsForProvisional(log *callLog, stagedKey identity.StagedKey) identity.Deps {
	return identity.Deps{
		PersistKey: func(_ identity.StagedKey) (identity.KeyResult, error) {
			log.persistKeyCalls++
			return identity.KeyResult{
				PrivatePath: stagedKey.FinalPrivatePath,
				PubPath:     stagedKey.FinalPubPath,
				PubLine:     stagedKey.PubLine,
			}, nil
		},
		WriteSSH: func(_, _, _ string) (string, error) {
			log.writeSSHCalls++
			return "/backup/ssh.bak", nil
		},
		WriteGitconfig: func(_, _, _ string, _ []gitconfig.Match) (string, error) {
			log.writeGitconfigCalls++
			return "/backup/gitconfig.bak", nil
		},
		WriteProvisionalSSH: func(name, hostBlock string) (string, error) {
			log.writeProvisionalCalls = append(log.writeProvisionalCalls, provisionalCallArgs{name: name, hostBlock: hostBlock})
			return "/backup/prov.bak", nil
		},
		PromoteSSH: func(name, hostBlock string) (string, error) {
			log.promoteSSHCalls = append(log.promoteSSHCalls, provisionalCallArgs{name: name, hostBlock: hostBlock})
			return "/backup/promote.bak", nil
		},
		DropProvisionalSSH: func(name string) (string, error) {
			log.dropProvisionalCalls = append(log.dropProvisionalCalls, name)
			return "/backup/drop.bak", nil
		},
	}
}

// makeCreateInput returns a minimal CreateInput for testing the provisional lifecycle.
func makeCreateInput() identity.CreateInput {
	return identity.CreateInput{
		Name:     "alice",
		Provider: "github",
		Alias:    "personal.github.com",
		Hostname: "ssh.github.com",
		Port:     443,
	}
}

// makeStagedKey returns a StagedKey with distinct staged and final paths, and
// non-nil PrivPEM so PersistKey fires (the new-key path).
func makeStagedKey() identity.StagedKey {
	return identity.StagedKey{
		TempPrivatePath:  "/tmp/gitid-staging/id_ed25519_alice",
		FinalPrivatePath: "~/.ssh/id_ed25519_alice",
		FinalPubPath:     "~/.ssh/id_ed25519_alice.pub",
		PubLine:          "ssh-ed25519 AAAA... alice@example.com",
		PrivPEM:          []byte("FAKE-PEM"),
	}
}

// TestPersistSSHProvisional_WritesProvisionalWithStagedKey verifies that
// PersistSSHProvisional:
//   - calls WriteProvisionalSSH with the identity name and a host block whose
//     IdentityFile is the STAGED (temp) key path
//   - does NOT call PersistKey, WriteSSH, PromoteSSH, or WriteGitconfig
func TestPersistSSHProvisional_WritesProvisionalWithStagedKey(t *testing.T) {
	log := &callLog{}
	staged := makeStagedKey()
	in := makeCreateInput()
	deps := fakeDepsForProvisional(log, staged)

	res, err := identity.PersistSSHProvisional(in, staged, deps)
	if err != nil {
		t.Fatalf("PersistSSHProvisional error: %v", err)
	}

	// WriteProvisionalSSH must have been called exactly once.
	if len(log.writeProvisionalCalls) != 1 {
		t.Fatalf("expected 1 WriteProvisionalSSH call, got %d", len(log.writeProvisionalCalls))
	}
	call := log.writeProvisionalCalls[0]
	if call.name != "alice" {
		t.Errorf("WriteProvisionalSSH called with name %q, want %q", call.name, "alice")
	}
	// Host block must reference the STAGED key path, not the final.
	if !strings.Contains(call.hostBlock, staged.TempPrivatePath) {
		t.Errorf("host block does not contain staged key path %q:\n%q", staged.TempPrivatePath, call.hostBlock)
	}
	if strings.Contains(call.hostBlock, staged.FinalPrivatePath) {
		t.Errorf("host block unexpectedly contains FINAL key path %q:\n%q", staged.FinalPrivatePath, call.hostBlock)
	}

	// PersistKey must NOT have been called.
	if log.persistKeyCalls != 0 {
		t.Errorf("PersistKey was called %d times, expected 0", log.persistKeyCalls)
	}
	// WriteSSH (managed) must NOT have been called.
	if log.writeSSHCalls != 0 {
		t.Errorf("WriteSSH was called %d times, expected 0", log.writeSSHCalls)
	}
	// PromoteSSH must NOT have been called.
	if len(log.promoteSSHCalls) != 0 {
		t.Errorf("PromoteSSH was called %d times, expected 0", len(log.promoteSSHCalls))
	}
	// WriteGitconfig must NOT have been called.
	if log.writeGitconfigCalls != 0 {
		t.Errorf("WriteGitconfig was called %d times, expected 0", log.writeGitconfigCalls)
	}

	// SSHPreview must contain the staged key (it is the provisional body).
	if !strings.Contains(res.SSHPreview, staged.TempPrivatePath) {
		t.Errorf("SSHPreview does not contain staged key path:\n%q", res.SSHPreview)
	}
	// SSHBackup must be non-empty (from the fake).
	if res.SSHBackup == "" {
		t.Errorf("expected non-empty SSHBackup")
	}
}

// TestPromoteSSH_PersistsKeyBeforePromote verifies that PromoteSSH:
//   - calls PersistKey FIRST (before PromoteSSH seam)
//   - then calls PromoteSSH with a host block whose IdentityFile is the FINAL key
//   - does NOT call WriteSSH or WriteGitconfig or WriteProvisionalSSH
func TestPromoteSSH_PersistsKeyBeforePromote(t *testing.T) {
	log := &callLog{}
	staged := makeStagedKey()
	in := makeCreateInput()

	// Capture call order by noting that PersistKey must complete before PromoteSSH.
	var callOrder []string
	deps := fakeDepsForProvisional(log, staged)
	deps.PersistKey = func(_ identity.StagedKey) (identity.KeyResult, error) {
		callOrder = append(callOrder, "PersistKey")
		log.persistKeyCalls++
		return identity.KeyResult{
			PrivatePath: staged.FinalPrivatePath,
			PubPath:     staged.FinalPubPath,
			PubLine:     staged.PubLine,
		}, nil
	}
	deps.PromoteSSH = func(name, hostBlock string) (string, error) {
		callOrder = append(callOrder, "PromoteSSH")
		log.promoteSSHCalls = append(log.promoteSSHCalls, provisionalCallArgs{name: name, hostBlock: hostBlock})
		return "/backup/promote.bak", nil
	}

	res, err := identity.PromoteSSH(in, staged, deps)
	if err != nil {
		t.Fatalf("PromoteSSH error: %v", err)
	}

	// PersistKey must have been called.
	if log.persistKeyCalls != 1 {
		t.Fatalf("PersistKey called %d times, expected 1", log.persistKeyCalls)
	}
	// PromoteSSH seam must have been called.
	if len(log.promoteSSHCalls) != 1 {
		t.Fatalf("PromoteSSH called %d times, expected 1", len(log.promoteSSHCalls))
	}

	// PersistKey must come BEFORE PromoteSSH (T-05.7-09-04 ordering).
	if len(callOrder) < 2 || callOrder[0] != "PersistKey" || callOrder[1] != "PromoteSSH" {
		t.Errorf("call order mismatch: got %v, want [PersistKey, PromoteSSH]", callOrder)
	}

	// PromoteSSH host block must reference the FINAL key path, not the staged.
	call := log.promoteSSHCalls[0]
	if !strings.Contains(call.hostBlock, staged.FinalPrivatePath) {
		t.Errorf("promoted host block does not contain FINAL key path %q:\n%q", staged.FinalPrivatePath, call.hostBlock)
	}
	if strings.Contains(call.hostBlock, staged.TempPrivatePath) {
		t.Errorf("promoted host block unexpectedly contains STAGED key path %q:\n%q", staged.TempPrivatePath, call.hostBlock)
	}

	// WriteSSH (managed) must NOT have been called.
	if log.writeSSHCalls != 0 {
		t.Errorf("WriteSSH was called %d times, expected 0", log.writeSSHCalls)
	}
	// WriteProvisionalSSH must NOT have been called.
	if len(log.writeProvisionalCalls) != 0 {
		t.Errorf("WriteProvisionalSSH was called %d times, expected 0", len(log.writeProvisionalCalls))
	}
	// WriteGitconfig must NOT have been called.
	if log.writeGitconfigCalls != 0 {
		t.Errorf("WriteGitconfig was called %d times, expected 0", log.writeGitconfigCalls)
	}

	// SSHPreview must contain the final key.
	if !strings.Contains(res.SSHPreview, staged.FinalPrivatePath) {
		t.Errorf("SSHPreview does not contain FINAL key path:\n%q", res.SSHPreview)
	}
	// Key result must carry the final paths.
	if res.Key.PrivatePath != staged.FinalPrivatePath {
		t.Errorf("Key.PrivatePath = %q, want %q", res.Key.PrivatePath, staged.FinalPrivatePath)
	}
}

// TestPromoteSSH_SkipsPersistKeyWhenPrivPEMNil verifies that PromoteSSH does NOT
// call PersistKey when staged.PrivPEM is nil (existing-key reuse/add-account path).
func TestPromoteSSH_SkipsPersistKeyWhenPrivPEMNil(t *testing.T) {
	log := &callLog{}
	staged := makeStagedKey()
	staged.PrivPEM = nil // existing-key path
	staged.TempPrivatePath = staged.FinalPrivatePath
	in := makeCreateInput()
	deps := fakeDepsForProvisional(log, staged)

	_, err := identity.PromoteSSH(in, staged, deps)
	if err != nil {
		t.Fatalf("PromoteSSH (nil PrivPEM) error: %v", err)
	}

	if log.persistKeyCalls != 0 {
		t.Errorf("PersistKey called %d times for nil PrivPEM, expected 0", log.persistKeyCalls)
	}
	if len(log.promoteSSHCalls) != 1 {
		t.Errorf("PromoteSSH seam called %d times, expected 1", len(log.promoteSSHCalls))
	}
}

// TestDropProvisionalSSH_CallsOnlyDropSeam verifies that DropProvisionalSSH
// calls only the DropProvisionalSSH dep (not PersistKey, WriteSSH, PromoteSSH,
// WriteGitconfig, or WriteProvisionalSSH).
func TestDropProvisionalSSH_CallsOnlyDropSeam(t *testing.T) {
	log := &callLog{}
	staged := makeStagedKey()
	in := makeCreateInput()
	deps := fakeDepsForProvisional(log, staged)

	backupPath, err := identity.DropProvisionalSSH(in, deps)
	if err != nil {
		t.Fatalf("DropProvisionalSSH error: %v", err)
	}

	// Only DropProvisionalSSH must have been called.
	if len(log.dropProvisionalCalls) != 1 {
		t.Fatalf("DropProvisionalSSH seam called %d times, expected 1", len(log.dropProvisionalCalls))
	}
	if log.dropProvisionalCalls[0] != "alice" {
		t.Errorf("DropProvisionalSSH called with name %q, want %q", log.dropProvisionalCalls[0], "alice")
	}

	// No other seams must fire.
	if log.persistKeyCalls != 0 {
		t.Errorf("PersistKey called %d times, expected 0", log.persistKeyCalls)
	}
	if log.writeSSHCalls != 0 {
		t.Errorf("WriteSSH called %d times, expected 0", log.writeSSHCalls)
	}
	if len(log.writeProvisionalCalls) != 0 {
		t.Errorf("WriteProvisionalSSH called %d times, expected 0", len(log.writeProvisionalCalls))
	}
	if len(log.promoteSSHCalls) != 0 {
		t.Errorf("PromoteSSH called %d times, expected 0", len(log.promoteSSHCalls))
	}
	if log.writeGitconfigCalls != 0 {
		t.Errorf("WriteGitconfig called %d times, expected 0", log.writeGitconfigCalls)
	}

	// Backup path from the fake must be returned.
	if backupPath == "" {
		t.Errorf("expected non-empty backupPath, got empty string")
	}
}
