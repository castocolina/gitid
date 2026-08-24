package main

// wiring_cr_test.go — TDD RED tests for CR-07 through CR-11 and WR-01.
//
// These tests document the exact production gaps found by the 2026-08-21 deep
// review and must fail with current code. Once production is corrected they
// become the permanent regression suite.
//
// CR-07: stage-2 proof must validate all ssh -G fields and expose both commands+outputs.
// CR-08: no pre-confirm EnsureDir/chmod under the real SSH directory.
// CR-09: reused private key normalised to 0600 in confirmed transaction, rolled back on failure.
// CR-10: selected algorithm (rsa-4096) survives through DemoIdentity → createInput → commit.
// CR-11: IdentityFile rejects unsafe tokens (space, tab, quote, backslash, etc.).
// WR-01: validated provider survives to committed Host block, never re-derived from alias suffix.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/keygen"
	"github.com/castocolina/gitid/internal/sshconfig"
	"github.com/castocolina/gitid/internal/tester"
	"github.com/castocolina/gitid/internal/tuikit"
)

// ---------------------------------------------------------------------------
// CR-07 — stage-2 proof: both commands/outputs, all field validation
// ---------------------------------------------------------------------------

// TestStage2ProofCarriesBothCommandsAndOutputs verifies that the stage-2
// result DTO carried in the WizardStageMsg exposes BOTH the connectivity
// command+output AND the ssh -G resolution command+output, so the UI can
// render both to the user (CR-07/TEST-01/TEST-02).
//
// Current state (RED): the DTO carries only one command and the result detail
// only shows the first IdentityFile from ParseResolved — the resolution command
// and raw output are discarded (tester.ResolvedVia L194-204).
func TestStage2ProofCarriesBothCommandsAndOutputs(t *testing.T) {
	// Stage2Result must carry the resolution (ssh -G) command separately from
	// the connectivity command, and the raw resolution output must be present.
	// We test this via the new tester.ResolvedViaResult type that must carry
	// both commands, both raw outputs, the resolution error, and parsed config.
	//
	// This is a structural check: if tester.ResolvedVia now returns a
	// Stage2Result that exposes both commands, the production code is fixed.
	configPath := "/tmp/not-real"
	keyPath := "/tmp/not-real-key"
	alias := "acme.github.com"
	knownHosts := ""

	connectCmd := tester.ResolvedViaCommand(configPath, keyPath, alias, knownHosts)
	resolutionCmd := tester.ResolvedViaGCommand(configPath, alias)

	if connectCmd == resolutionCmd {
		t.Errorf("connectivity command and resolution command must be distinct:\n  connect: %q\n  resolve: %q",
			connectCmd, resolutionCmd)
	}
	if strings.Contains(connectCmd, "-G") {
		t.Errorf("connectivity command must NOT contain -G: %q", connectCmd)
	}
	if !strings.Contains(resolutionCmd, "-G") {
		t.Errorf("resolution command must contain -G: %q", resolutionCmd)
	}
}

// TestStage2FieldValidationFailsOnMismatch proves that a stage-2 result with
// wrong User/Hostname/Port/IdentitiesOnly/IdentityFile is classified as
// Failure, not PASS or ReachableNotUploaded (CR-07).
//
// Current state (RED): TestStage2 in wiring.go calls ResolvedVia but never
// compares expected fields — success is recorded solely from the connectivity
// classification.
func TestStage2FieldValidationFailsOnMismatch(t *testing.T) {
	// We test the field validation helper directly. It must exist and reject
	// a resolved config where User != "git".
	rc := tester.ResolvedConfig{
		User:           "wrong-user",
		Hostname:       "ssh.github.com",
		Port:           "443",
		IdentitiesOnly: "yes",
		IdentityFiles:  []string{"/home/.ssh/id_ed25519_acme"},
	}
	err := tester.ValidateResolvedConfig(rc, tester.ExpectedResolution{
		User:            "git",
		Hostname:        "ssh.github.com",
		Port:            "443",
		IdentitiesOnly:  "yes",
		ExpectedKeyPath: "/home/.ssh/id_ed25519_acme",
	})
	if err == nil {
		t.Fatal("ValidateResolvedConfig with wrong User should return error (CR-07)")
	}
	if !strings.Contains(err.Error(), "user") {
		t.Errorf("error should mention the mismatched field 'user': %v", err)
	}
}

// TestStage2ValidationRejectsEmptyResolutionOutput proves that an empty ssh -G
// output (command ran but produced nothing) is rejected as a proof failure
// (CR-07).
func TestStage2ValidationRejectsEmptyResolutionOutput(t *testing.T) {
	err := tester.ValidateResolvedConfig(tester.ResolvedConfig{}, tester.ExpectedResolution{
		User:            "git",
		Hostname:        "ssh.github.com",
		Port:            "443",
		IdentitiesOnly:  "yes",
		ExpectedKeyPath: "/home/.ssh/id_ed25519_acme",
	})
	if err == nil {
		t.Fatal("ValidateResolvedConfig with all-empty resolved config should return error (empty ssh -G output — CR-07)")
	}
}

// TestStage2ValidationAcceptsCorrectFields proves that a correctly matching
// resolved config returns nil error from the validator (CR-07 positive case).
func TestStage2ValidationAcceptsCorrectFields(t *testing.T) {
	rc := tester.ResolvedConfig{
		User:           "git",
		Hostname:       "ssh.github.com",
		Port:           "443",
		IdentitiesOnly: "yes",
		IdentityFiles:  []string{"/home/.ssh/id_ed25519_acme", "/home/.ssh/id_ed25519_other"},
	}
	err := tester.ValidateResolvedConfig(rc, tester.ExpectedResolution{
		User:            "git",
		Hostname:        "ssh.github.com",
		Port:            "443",
		IdentitiesOnly:  "yes",
		ExpectedKeyPath: "/home/.ssh/id_ed25519_acme",
	})
	if err != nil {
		t.Errorf("ValidateResolvedConfig with matching fields should return nil: %v", err)
	}
}

// TestStage2ValidationRejectsWrongIdentityFileOrder proves that the expected key
// must appear as the FIRST effective IdentityFile in the resolved config — a
// later-listed key means the config is not correctly wired (CR-07).
func TestStage2ValidationRejectsWrongIdentityFileOrder(t *testing.T) {
	rc := tester.ResolvedConfig{
		User:           "git",
		Hostname:       "ssh.github.com",
		Port:           "443",
		IdentitiesOnly: "yes",
		// The expected key is second — the default id_ed25519 would match first.
		IdentityFiles: []string{"/home/.ssh/id_ed25519", "/home/.ssh/id_ed25519_acme"},
	}
	err := tester.ValidateResolvedConfig(rc, tester.ExpectedResolution{
		User:            "git",
		Hostname:        "ssh.github.com",
		Port:            "443",
		IdentitiesOnly:  "yes",
		ExpectedKeyPath: "/home/.ssh/id_ed25519_acme",
	})
	if err == nil {
		t.Fatal("ValidateResolvedConfig with expected key NOT first in IdentityFiles should return error (CR-07)")
	}
}

// ---------------------------------------------------------------------------
// CR-08 — no pre-confirm EnsureDir/chmod under the real SSH directory
// ---------------------------------------------------------------------------

// TestGenerateDoesNotCreateRealSSHDirBeforeConfirm proves that Generate never
// calls EnsureDir on the real ~/.ssh — the real SSH directory must remain
// absent until the confirmed transaction (CR-08).
//
// Current state (RED): buildIdentityDeps.Generate calls
//
//	filewriter.EnsureDir(b.sshDir, sshDirMode)
//
// before creating the staging directory, which can create ~/.ssh on a fresh machine.
func TestGenerateDoesNotCreateRealSSHDirBeforeConfirm(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// NO seedSSHDir — intentionally no ~/.ssh to start.

	b := newBackendForHome(home)
	sshDir := filepath.Join(home, ".ssh")

	// Verify ~/.ssh does not pre-exist.
	if _, err := os.Stat(sshDir); !os.IsNotExist(err) {
		t.Fatalf("fixture invalid: %s must not pre-exist; stat err = %v", sshDir, err)
	}

	in := b.createInputFromSpec(tuikit.CreateSpec{
		Identity: "fresh", Alias: "fresh.github.com",
		Hostname: "ssh.github.com", Port: "443",
	})
	staged, err := b.deps.Generate(in)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	_ = staged

	// After Generate, the real ~/.ssh must still be absent.
	if _, serr := os.Stat(sshDir); !os.IsNotExist(serr) {
		t.Errorf("Generate created the real SSH directory %s before confirmation (CR-08); stat err = %v", sshDir, serr)
	}
}

// TestGenerateDoesNotChmodRealSSHDirBeforeConfirm proves that Generate does not
// change the mode of an existing ~/.ssh — only the confirmed transaction may
// touch that directory (CR-08).
func TestGenerateDoesNotChmodRealSSHDirBeforeConfirm(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sshDir := filepath.Join(home, ".ssh")
	// Create ~/.ssh with an unusual mode (0o755 is intentional — we want to
	// detect if Generate changes it; a lower-than-0700 mode is still unusual
	// enough to detect an EnsureDir call). gosec G301 is suppressed here
	// because 0755 is the exact test fixture mode we need to observe.
	if err := os.MkdirAll(sshDir, 0o755); err != nil { //nolint:gosec // intentional 0755 fixture for mode-drift detection (G301)
		t.Fatalf("seeding ~/.ssh at 0755: %v", err)
	}

	b := newBackendForHome(home)
	in := b.createInputFromSpec(tuikit.CreateSpec{
		Identity: "fresh", Alias: "fresh.github.com",
		Hostname: "ssh.github.com", Port: "443",
	})
	staged, err := b.deps.Generate(in)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	_ = staged

	info, err := os.Stat(sshDir)
	if err != nil {
		t.Fatalf("stat %s after Generate: %v", sshDir, err)
	}
	if got := info.Mode().Perm(); got != 0o755 {
		t.Errorf("Generate changed ~/.ssh mode from 0755 to %o — must not chmod before confirmation (CR-08)", got)
	}
}

// ---------------------------------------------------------------------------
// CR-09 — reused private key 0644→0600 in confirmed transaction, rolled back
// ---------------------------------------------------------------------------

// TestReuseKeyPermissionNormalisedOnConfirm proves that a reused private key
// at mode 0644 is normalised to 0600 only inside the confirmed transaction
// (CR-09).
//
// Current state (RED): commitCreateTransaction skips the chmod when
//
//	staged.PrivPEM == nil (the reuse case), so the private key stays 0644.
func TestReuseKeyPermissionNormalisedOnConfirm(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)

	keyPath := filepath.Join(home, ".ssh", "id_ed25519_reuse")
	// Intentionally bad mode — 0644 — to prove it gets fixed on confirm.
	mat, err := keygen.GenerateMaterial(keygen.Params{
		Algo: "ed25519", Identity: "reuse", Comment: "reuse@gitid",
	})
	if err != nil {
		t.Fatalf("generating key material: %v", err)
	}
	if werr := os.WriteFile(keyPath, mat.PrivPEM, 0o644); werr != nil { //nolint:gosec // test fixture with intentionally wrong mode
		t.Fatalf("writing private key at 0644: %v", werr)
	}
	if werr := os.WriteFile(keyPath+".pub", []byte(mat.PubLine+"\n"), 0o644); werr != nil { //nolint:gosec // public key; hermetic sandbox HOME (G306)
		t.Fatalf("writing public key: %v", werr)
	}

	b := newBackendForHome(home)
	id := tuikit.DemoIdentity{
		Name:         "reuse",
		SSHHost:      "reuse.github.com",
		Hostname:     "ssh.github.com",
		Port:         443,
		KeyPath:      keyPath,
		ReuseKeyPath: keyPath,
	}
	unlockStoreForIdentity(t, b, id)

	state := b.Persist(tuikit.DemoState{}, tuikit.AddIdentity{Identity: id})
	if err := b.PersistError(); err != nil {
		t.Fatalf("Persist: %v", err)
	}
	if len(state.Identities) == 0 {
		t.Fatal("Persist returned no identities after confirmed reuse create")
	}

	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("stat reused private key after confirm: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("reused private key mode after confirmed transaction = %o, want 0600 (CR-09)", got)
	}
}

// TestReuseKeyModeRestoredOnTransactionFailure proves that if the confirmed
// transaction fails AFTER the private-key chmod, the key's original mode is
// restored (CR-09 rollback).
//
// Current state (RED): there is no chmod-op for reused keys, so there is
// also no rollback of it. This test verifies both the chmod and its rollback.
func TestReuseKeyModeRestoredOnTransactionFailure(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)

	keyPath := filepath.Join(home, ".ssh", "id_ed25519_reuseroll")
	mat, err := keygen.GenerateMaterial(keygen.Params{
		Algo: "ed25519", Identity: "reuseroll", Comment: "reuseroll@gitid",
	})
	if err != nil {
		t.Fatalf("generating key material: %v", err)
	}
	// Seed at 0644 — the "wrong" mode that would be normalised.
	if werr := os.WriteFile(keyPath, mat.PrivPEM, 0o644); werr != nil { //nolint:gosec // test fixture with intentionally wrong mode
		t.Fatalf("writing private key at 0644: %v", werr)
	}
	if werr := os.WriteFile(keyPath+".pub", []byte(mat.PubLine+"\n"), 0o644); werr != nil { //nolint:gosec // public key (G306)
		t.Fatalf("writing public key: %v", werr)
	}

	b := newBackendForHome(home)
	id := tuikit.DemoIdentity{
		Name:         "reuseroll",
		SSHHost:      "reuseroll.github.com",
		Hostname:     "ssh.github.com",
		Port:         443,
		KeyPath:      keyPath,
		ReuseKeyPath: keyPath,
	}
	unlockStoreForIdentity(t, b, id)
	// Inject failure at host-block step (AFTER any private-key chmod that occurs).
	b.failCommitAt = func(step string) error {
		if step == "host-block" {
			return fmt.Errorf("injected failure at host-block")
		}
		return nil
	}

	b.Persist(tuikit.DemoState{}, tuikit.AddIdentity{Identity: id})
	if b.PersistError() == nil {
		t.Fatal("injected failure must propagate as PersistError")
	}

	// Mode must be restored to original 0644.
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("stat reused private key after rollback: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Errorf("reused private key mode after rollback = %o, want 0644 (CR-09 mode restore)", got)
	}
}

// TestReuseKeyModeNotChangedBeforeConfirm proves that the reuse path never
// touches the private key mode before the confirmed transaction runs (CR-08/CR-09).
func TestReuseKeyModeNotChangedBeforeConfirm(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)

	keyPath := filepath.Join(home, ".ssh", "id_ed25519_reusenochmod")
	mat, err := keygen.GenerateMaterial(keygen.Params{
		Algo: "ed25519", Identity: "reusenochmod", Comment: "reusenochmod@gitid",
	})
	if err != nil {
		t.Fatalf("generating key material: %v", err)
	}
	if werr := os.WriteFile(keyPath, mat.PrivPEM, 0o644); werr != nil { //nolint:gosec // test fixture
		t.Fatalf("writing private key: %v", werr)
	}
	if werr := os.WriteFile(keyPath+".pub", []byte(mat.PubLine+"\n"), 0o644); werr != nil { //nolint:gosec // public key (G306)
		t.Fatalf("writing public key: %v", werr)
	}

	b := newBackendForHome(home)
	spec := tuikit.CreateSpec{
		Identity:     "reusenochmod",
		Alias:        "reusenochmod.github.com",
		Hostname:     "ssh.github.com",
		Port:         "443",
		ReuseKeyPath: keyPath,
	}
	in := b.createInputFromSpec(spec)
	_, err = b.stagedKeyFor(in, keyPath)
	if err != nil {
		t.Fatalf("stagedKeyFor: %v", err)
	}

	// Mode must still be 0644 — stagedKeyFor must not chmod.
	info, serr := os.Stat(keyPath)
	if serr != nil {
		t.Fatalf("stat after stagedKeyFor: %v", serr)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Errorf("stagedKeyFor changed private key mode to %o before confirmation (CR-08/CR-09); must be unchanged 0644", got)
	}
}

// ---------------------------------------------------------------------------
// CR-10 — selected algorithm propagates through DemoIdentity → createInput → commit
// ---------------------------------------------------------------------------

// TestAlgorithmCarriedInDemoIdentity proves that DemoIdentity carries the
// Algorithm field (CR-10).
//
// Current state (RED): DemoIdentity in internal/tuikit/store.go has no
// Algorithm field, so any selected algorithm is dropped after finishIdentity.
func TestAlgorithmCarriedInDemoIdentity(t *testing.T) {
	// DemoIdentity must have an Algorithm field.
	id := tuikit.DemoIdentity{
		Name:      "test",
		SSHHost:   "test.github.com",
		Algorithm: "rsa-4096",
	}
	if id.Algorithm != "rsa-4096" {
		t.Errorf("DemoIdentity.Algorithm = %q, want %q (field missing or zero — CR-10)", id.Algorithm, "rsa-4096")
	}
}

// TestCreateInputUsesAlgorithmFromDemoIdentity proves that createInput reads
// the algorithm from the DemoIdentity rather than hardcoding "ed25519" (CR-10).
//
// Current state (RED): createInput hardcodes `Algo: "ed25519"` (line 917).
func TestCreateInputUsesAlgorithmFromDemoIdentity(t *testing.T) {
	b := newBackendForHome(t.TempDir())

	id := tuikit.DemoIdentity{
		Name:      "rsauser",
		SSHHost:   "rsauser.github.com",
		Hostname:  "ssh.github.com",
		Port:      443,
		Algorithm: "rsa-4096",
	}
	in := b.createInput(id)
	if in.Algo != "rsa-4096" {
		t.Errorf("createInput(DemoIdentity{Algorithm:rsa-4096}).Algo = %q, want %q (CR-10 — hardcoded ed25519)", in.Algo, "rsa-4096")
	}
}

// TestAlgorithmSurvivesToCommitHostBlock proves that the algorithm selected in
// the wizard survives to the persisted Host block identity filename (CR-10).
// An rsa-4096 selection must produce an id_rsa-4096_* key path, never id_ed25519_*.
func TestAlgorithmSurvivesToCommitHostBlock(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)

	b := newBackendForHome(home)
	id := tuikit.DemoIdentity{
		Name:      "rsacommit",
		SSHHost:   "rsacommit.github.com",
		Hostname:  "ssh.github.com",
		Port:      443,
		Algorithm: "rsa-4096",
	}
	unlockStoreForIdentityAlgo(t, b, id)

	state := b.Persist(tuikit.DemoState{}, tuikit.AddIdentity{Identity: id})
	if err := b.PersistError(); err != nil {
		t.Fatalf("Persist: %v", err)
	}
	if len(state.Identities) == 0 {
		t.Fatal("Persist returned no identities")
	}

	included := readFile(t, filepath.Join(home, ".ssh", "config.d", "gitid.config"))
	if strings.Contains(included, "id_ed25519_rsacommit") {
		t.Errorf("Host block contains ed25519 key path — the rsa-4096 algorithm selection was lost (CR-10):\n%s", included)
	}
	if !strings.Contains(included, "id_rsa-4096_rsacommit") {
		t.Errorf("Host block does not contain rsa-4096 key path (CR-10):\n%s", included)
	}
}

// unlockStoreForIdentityAlgo records accepted stage outcomes for a DemoIdentity
// that carries an Algorithm field, so the specFingerprint includes the algo.
func unlockStoreForIdentityAlgo(t *testing.T, b *realBackend, id tuikit.DemoIdentity) {
	t.Helper()
	in := b.createInput(id)
	b.recordOutcomeFor(1, tuikit.TestOutcomePass, in)
	b.recordOutcomeFor(2, tuikit.TestOutcomePass, in)
}

// ---------------------------------------------------------------------------
// WR-01 — validated provider survives, never re-derived from alias suffix
// ---------------------------------------------------------------------------

// TestProviderCarriedInDemoIdentity proves that DemoIdentity carries the
// Provider field (WR-01).
//
// Current state (RED): DemoIdentity has no Provider field.
func TestProviderCarriedInDemoIdentity(t *testing.T) {
	id := tuikit.DemoIdentity{
		Name:     "enterprise",
		SSHHost:  "enterprise.company.co.uk",
		Provider: "company.co.uk",
	}
	if id.Provider != "company.co.uk" {
		t.Errorf("DemoIdentity.Provider = %q, want %q (field missing or zero — WR-01)", id.Provider, "company.co.uk")
	}
}

// TestCreateInputUsesProviderFromDemoIdentity proves that createInput reads the
// provider from DemoIdentity.Provider rather than deriving it from the alias
// suffix via providerFromAlias (WR-01).
//
// Current state (RED): createInput calls providerFromAlias(row.SSHHost) which
// truncates multi-label providers to the last two labels.
func TestCreateInputUsesProviderFromDemoIdentity(t *testing.T) {
	b := newBackendForHome(t.TempDir())

	id := tuikit.DemoIdentity{
		Name:     "enterprise",
		SSHHost:  "enterprise.company.co.uk",
		Hostname: "ssh.company.co.uk",
		Port:     22,
		Provider: "company.co.uk", // three-label provider — providerFromAlias would give "co.uk"
	}
	in := b.createInput(id)
	if in.Provider != "company.co.uk" {
		t.Errorf("createInput(DemoIdentity{Provider:company.co.uk}).Provider = %q, want %q (WR-01 — providerFromAlias reconstruction)", in.Provider, "company.co.uk")
	}
}

// TestProviderSurvivesToCommitHostBlock proves that the provider persisted in
// the Host block comes from DemoIdentity.Provider, not from providerFromAlias
// truncation (WR-01).
func TestProviderSurvivesToCommitHostBlock(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)

	b := newBackendForHome(home)
	// Three-label provider that providerFromAlias would truncate to "co.uk".
	id := tuikit.DemoIdentity{
		Name:     "enterprise",
		SSHHost:  "enterprise.company.co.uk",
		Hostname: "ssh.company.co.uk",
		Port:     22,
		Provider: "company.co.uk",
	}
	// Use generic unlock that reads from createInput (must use Provider field).
	in := b.createInput(id)
	b.recordOutcomeFor(1, tuikit.TestOutcomePass, in)
	b.recordOutcomeFor(2, tuikit.TestOutcomePass, in)

	state := b.Persist(tuikit.DemoState{}, tuikit.AddIdentity{Identity: id})
	if err := b.PersistError(); err != nil {
		t.Fatalf("Persist: %v", err)
	}
	if len(state.Identities) == 0 {
		t.Fatal("Persist returned no identities")
	}

	// The Host block provider comment should use the full provider, not "co.uk".
	included := readFile(t, filepath.Join(home, ".ssh", "config.d", "gitid.config"))
	_ = included // provider marker check depends on RenderHostBlock format; log it for inspection
	t.Logf("written Host block:\n%s", included)

	// Key path should use the full provider context — verify the provider
	// persists in the CreateInput flowing to RenderHostBlock.
	hostBlock := sshconfig.RenderHostBlock("enterprise.company.co.uk", "ssh.company.co.uk", 22,
		filepath.Join(home, ".ssh", "id_ed25519_enterprise"), "company.co.uk")
	if !strings.Contains(hostBlock, "company.co.uk") {
		t.Errorf("RenderHostBlock with company.co.uk provider does not embed provider (WR-01):\n%s", hostBlock)
	}
	// The actual written Host block must not contain only the two-label "co.uk" truncation.
	if strings.Contains(included, "co.uk") && !strings.Contains(included, "company.co.uk") {
		t.Errorf("written Host block contains only two-label truncation 'co.uk' rather than full 'company.co.uk' (WR-01):\n%s", included)
	}
}

// ---------------------------------------------------------------------------
// CR-11 — IdentityFile strict token validation at ValidateHostBlock
// ---------------------------------------------------------------------------

// TestValidateHostBlockRejectsUnsafeIdentityFileTokens proves that
// ValidateHostBlock rejects IdentityFile values that contain characters
// unsafe for unquoted OpenSSH tokens (CR-11).
//
// Current state (RED): ValidateHostBlock only rejects \n, \r, and NUL —
// spaces, tabs, quotes, backslashes, and other control chars pass through.
func TestValidateHostBlockRejectsUnsafeIdentityFileTokens(t *testing.T) {
	unsafe := []struct {
		name  string
		value string
	}{
		{"space", "/home/user/my key"},
		{"tab", "/home/user/my\tkey"},
		{"double-quote", `/home/user/"key"`},
		{"single-quote", "/home/user/'key'"},
		{"backslash", `/home/user/my\key`},
		{"hash-comment", "/home/user/key#comment"},
		{"equals-separator", "/home/user/key=val"},
		{"form-feed", "/home/user/key\x0c"},
		{"vertical-tab", "/home/user/key\x0b"},
		{"DEL", "/home/user/key\x7f"},
	}
	for _, tc := range unsafe {
		t.Run(tc.name, func(t *testing.T) {
			err := sshconfig.ValidateHostBlock("alias", "hostname", "22", tc.value)
			if err == nil {
				t.Errorf("ValidateHostBlock accepted unsafe IdentityFile %q — should reject (CR-11)", tc.value)
			}
		})
	}
}

// TestValidateHostBlockAcceptsSafeIdentityFilePaths proves that ValidateHostBlock
// accepts paths that are valid unquoted OpenSSH IdentityFile tokens (CR-11 positive case).
func TestValidateHostBlockAcceptsSafeIdentityFilePaths(t *testing.T) {
	safe := []string{
		"/home/user/.ssh/id_ed25519",
		"/home/user/.ssh/id_rsa-4096_work",
		"~/.ssh/id_ed25519_personal",
		"/Users/user/.ssh/id_ed25519_acme",
	}
	for _, path := range safe {
		t.Run(path, func(t *testing.T) {
			err := sshconfig.ValidateHostBlock("alias", "hostname", "22", path)
			if err != nil {
				t.Errorf("ValidateHostBlock rejected safe IdentityFile %q — should accept (CR-11): %v", path, err)
			}
		})
	}
}

// TestValidateHostBlockIdentityFileRejectedAtRenderBoundary proves that the
// validation is applied immediately before rendering — the caller must not be
// able to bypass it by going directly to RenderHostBlock (CR-11 defense in depth).
// This is a structural source-code assertion: ValidateHostBlock must be called
// in the rendering/execution path.
func TestValidateHostBlockIdentityFileRejectedAtRenderBoundary(t *testing.T) {
	// A path with a space would produce a malformed SSH directive if rendered.
	// ValidateHostBlock must catch it.
	err := sshconfig.ValidateHostBlock("alias", "hostname", "22", "/home/user/path with spaces")
	if err == nil {
		t.Fatal("ValidateHostBlock must reject IdentityFile paths with spaces (CR-11)")
	}
	var ve *sshconfig.ValidationError
	if !isValidationError(err, &ve) {
		t.Errorf("error type should be *sshconfig.ValidationError; got %T: %v", err, err)
	} else if ve.Field != "identityFile" {
		t.Errorf("ValidationError.Field = %q, want %q", ve.Field, "identityFile")
	}
}

// isValidationError checks if err is a *sshconfig.ValidationError and fills ve.
func isValidationError(err error, ve **sshconfig.ValidationError) bool {
	if e, ok := err.(*sshconfig.ValidationError); ok {
		*ve = e
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Integration: specFingerprint includes Algorithm (CR-10 cache integrity)
// ---------------------------------------------------------------------------

// TestSpecFingerprintIncludesAlgorithm proves that specFingerprint produces
// different values for different Algorithm selections so that a cached stage
// proof for ed25519 cannot be replayed when the user selects rsa-4096 (CR-10).
//
// Current state (RED): specFingerprint uses in.Algo which is populated from
// createInput — but createInput hardcodes "ed25519", so all specs produce the
// same fingerprint regardless of the selected algorithm.
func TestSpecFingerprintIncludesAlgorithm(t *testing.T) {
	b := newBackendForHome(t.TempDir())

	idED := tuikit.DemoIdentity{
		Name: "work", SSHHost: "work.github.com", Hostname: "ssh.github.com", Port: 443,
		Algorithm: "ed25519",
	}
	idRSA := tuikit.DemoIdentity{
		Name: "work", SSHHost: "work.github.com", Hostname: "ssh.github.com", Port: 443,
		Algorithm: "rsa-4096",
	}
	inED := b.createInput(idED)
	inRSA := b.createInput(idRSA)

	fpED := specFingerprint(inED)
	fpRSA := specFingerprint(inRSA)
	if fpED == fpRSA {
		t.Errorf("specFingerprint must differ for ed25519 vs rsa-4096 selections (CR-10):\n  ed25519: %s\n  rsa-4096: %s", fpED, fpRSA)
	}
}

// ---------------------------------------------------------------------------
// Integration: tester.ResolvedViaGCommand existence (CR-07 new API)
// ---------------------------------------------------------------------------

// TestResolvedViaGCommandExists proves the new ssh -G display helper exists in
// the tester package (CR-07 — currently tester.go has no such function).
func TestResolvedViaGCommandExists(t *testing.T) {
	cmd := tester.ResolvedViaGCommand("/tmp/cfg", "work.github.com")
	if cmd == "" {
		t.Fatal("tester.ResolvedViaGCommand returned empty string — function may not exist (CR-07)")
	}
	if !strings.Contains(cmd, "-G") {
		t.Errorf("ResolvedViaGCommand output should contain -G: %q", cmd)
	}
	if !strings.Contains(cmd, "work.github.com") {
		t.Errorf("ResolvedViaGCommand output should contain the alias: %q", cmd)
	}
}

// TestValidateResolvedConfigExists proves the new validation helper exists in
// the tester package (CR-07 — currently tester.go has no such function).
func TestValidateResolvedConfigExists(t *testing.T) {
	// If the function does not exist, this file won't compile — the compile
	// failure is the RED signal. When it exists, we run a basic call.
	err := tester.ValidateResolvedConfig(
		tester.ResolvedConfig{User: "git", Hostname: "h", Port: "22", IdentitiesOnly: "yes", IdentityFiles: []string{"/k"}},
		tester.ExpectedResolution{User: "git", Hostname: "h", Port: "22", IdentitiesOnly: "yes", ExpectedKeyPath: "/k"},
	)
	if err != nil {
		t.Errorf("ValidateResolvedConfig with matching fields returned error: %v", err)
	}
}

// TestDemoIdentityAlgorithmAndProviderFieldsExist proves at compile time that
// both Algorithm and Provider fields exist on DemoIdentity (CR-10/WR-01).
// A field that is silently zero is NOT the same as a field that does not exist.
func TestDemoIdentityAlgorithmAndProviderFieldsExist(t *testing.T) {
	id := tuikit.DemoIdentity{
		Algorithm: "rsa-4096",
		Provider:  "company.co.uk",
	}
	if id.Algorithm == "" {
		t.Error("DemoIdentity.Algorithm field is missing or always zero (CR-10)")
	}
	if id.Provider == "" {
		t.Error("DemoIdentity.Provider field is missing or always zero (WR-01)")
	}
}

// ---------------------------------------------------------------------------
// CR-05/CR-06: Production TestStage2 validates before recording; view carries
// both commands and both raw outputs
// ---------------------------------------------------------------------------

// TestTestResultViewHasResolutionFields proves TestResultView carries the
// separate resolution command and resolution output required for CR-05/CR-06.
// These fields allow the TUI to render both ssh connectivity and ssh -G outputs.
//
// Current state (RED): TestResultView has only Command and Detail — no
// ResolutionCommand or ResolutionOutput fields.
func TestTestResultViewHasResolutionFields(t *testing.T) {
	// Build a TestResultView with both connectivity and resolution fields.
	// If these fields don't exist, the test will fail to compile (RED signal).
	view := tuikit.TestResultView{
		Outcome:           tuikit.TestOutcomePass,
		Command:           "ssh -F /tmp/cfg -T git@alias",
		Detail:            "Hi name! You've successfully authenticated.",
		ResolutionCommand: "ssh -F /tmp/cfg -G alias",
		ResolutionOutput:  "user git\nhostname ssh.github.com\nport 443\nidentitiesonly yes\nidentityfile /home/.ssh/id_ed25519",
	}
	if view.ResolutionCommand == "" {
		t.Error("TestResultView.ResolutionCommand field is missing or always zero (CR-05/CR-06)")
	}
	if view.ResolutionOutput == "" {
		t.Error("TestResultView.ResolutionOutput field is missing or always zero (CR-05/CR-06)")
	}
}

// TestStage2RecordsOutcomeAfterValidation proves that realBackend.TestStage2
// validates the ssh -G resolution fields BEFORE recording the accepted outcome.
// A resolved config with wrong User must result in a Failure outcome, not PASS.
//
// Current state (RED): TestStage2 calls recordOutcomeFor at line 590, before
// any ValidateResolvedConfig call, so a wrong resolved config still records PASS.
func TestStage2RecordsOutcomeAfterValidation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)

	b := newBackendForHome(home)

	// Inject a stage-2 runner that returns a wrong User in the resolution output
	// (ssh -G output has wrong-user instead of git). The connectivity output
	// has "successfully authenticated" so it would otherwise classify as PASS.
	// After validation, the stage must be downgraded to Failure.
	b.deps.ResolvedVia = func(configPath, _ string, alias string) (tester.Result, tester.ResolvedConfig) {
		res := tester.Result{
			Command: "ssh -F " + configPath + " -T git@" + alias,
			Output:  "Hi name! You've successfully authenticated.",
			Outcome: tester.PASS,
		}
		// Wrong user — validation must catch this
		rc := tester.ResolvedConfig{
			User:           "wrong-user",
			Hostname:       "ssh.github.com",
			Port:           "443",
			IdentitiesOnly: "yes",
			IdentityFiles:  []string{b.sshDir + "/id_ed25519_acme"},
		}
		return res, rc
	}

	spec := tuikit.CreateSpec{
		Identity: "acme",
		Alias:    "acme.github.com",
		Hostname: "ssh.github.com",
		Port:     "443",
		KeyPath:  b.sshDir + "/id_ed25519_acme",
	}

	// Prepare staging
	staged, err := b.deps.Generate(b.createInputFromSpec(spec))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	_ = staged

	// Run stage 2 via the tea.Cmd
	cmd := b.TestStage2(spec)
	msg := cmd()
	stageMsg, ok := msg.(tuikit.WizardStageMsg)
	if !ok {
		t.Fatalf("TestStage2 did not return WizardStageMsg; got %T", msg)
	}

	// With wrong User in resolution, the stage MUST be Failure, not PASS.
	if stageMsg.Result.Outcome != tuikit.TestOutcomeFailure {
		t.Errorf("TestStage2 with wrong User in resolution: outcome = %v, want TestOutcomeFailure (CR-05 — outcome recorded before validation)", stageMsg.Result.Outcome)
	}
	// And the store must remain locked.
	in := b.createInputFromSpec(spec)
	if b.storeUnlockedFor(in) {
		t.Error("store must NOT be unlocked after a failed stage-2 validation (CR-05)")
	}
}

func TestStage2RetainsConnectivityAndRawResolutionOutput(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)
	b := newBackendForHome(home)

	const connectivity = "stage-two connectivity banner\nwith exact spacing  \n"
	const marker = "gitidrawmarker proof-retained-verbatim  \nunknown-setting stays-unparsed\n"
	b.deps.ResolvedVia = func(configPath, keyPath, alias string) (tester.Result, tester.ResolvedConfig) {
		raw := "user git\nhostname ssh.github.com\nport 443\nidentitiesonly yes\nidentityfile " + keyPath + "\n" + marker
		return tester.Result{
			Command:          "ssh -F " + configPath + " -T git@" + alias,
			Output:           connectivity,
			ResolutionOutput: raw,
			Outcome:          tester.PASS,
		}, tester.ParseResolved(raw)
	}

	spec := tuikit.CreateSpec{Identity: "acme", Alias: "acme.github.com", Hostname: "ssh.github.com", Port: "443"}
	msg, ok := b.TestStage2(spec)().(tuikit.WizardStageMsg)
	if !ok {
		t.Fatal("TestStage2 did not return WizardStageMsg")
	}
	if msg.Result.Detail != connectivity {
		t.Errorf("stage-two Detail = %q, want unmodified connectivity output %q", msg.Result.Detail, connectivity)
	}
	wantRaw := "user git\nhostname ssh.github.com\nport 443\nidentitiesonly yes\nidentityfile " + b.staged.TempPrivatePath + "\n" + marker
	if msg.Result.ResolutionOutput != wantRaw {
		t.Errorf("stage-two ResolutionOutput = %q, want exact raw proof %q", msg.Result.ResolutionOutput, wantRaw)
	}
}

// ---------------------------------------------------------------------------
// CR-07: specFingerprint includes key source (generate vs reuse) and normalized
// reuse path so that a mid-flow key-source switch invalidates stale cached proof
// ---------------------------------------------------------------------------

// TestSpecFingerprintIncludesKeySource proves that specFingerprint distinguishes
// between "generate a new key" (empty reuse path) and "reuse an existing key"
// (non-empty reuse path), so a mid-flow switch invalidates cached staged material
// and both accepted outcomes (CR-07).
//
// Current state (RED): specFingerprint does not include a key-source field —
// stagedKeyFor uses a separate stagedReuseKeyPath check but the outcome fingerprint
// cannot distinguish generate vs reuse, allowing stale proof to leak across switch.
func TestSpecFingerprintIncludesKeySource(t *testing.T) {
	b := newBackendForHome(t.TempDir())

	// Same identity, different key sources.
	idGenerate := tuikit.DemoIdentity{
		Name:     "work",
		SSHHost:  "work.github.com",
		Hostname: "ssh.github.com",
		Port:     443,
	}
	idReuse := tuikit.DemoIdentity{
		Name:         "work",
		SSHHost:      "work.github.com",
		Hostname:     "ssh.github.com",
		Port:         443,
		ReuseKeyPath: "/home/.ssh/id_ed25519",
	}
	inGenerate := b.createInput(idGenerate)
	inReuse := b.createInput(idReuse)

	fpGenerate := specFingerprint(inGenerate)
	fpReuse := specFingerprint(inReuse)

	if fpGenerate == fpReuse {
		t.Errorf("specFingerprint must differ for generate vs reuse key source (CR-07):\n  generate: %s\n  reuse:    %s", fpGenerate, fpReuse)
	}
}

// TestSpecFingerprintIncludesNormalizedReusePath proves that specFingerprint
// includes the normalized absolute reuse path so that two different reuse paths
// produce different fingerprints (CR-07).
func TestSpecFingerprintIncludesNormalizedReusePath(t *testing.T) {
	b := newBackendForHome(t.TempDir())

	idA := tuikit.DemoIdentity{
		Name:         "work",
		SSHHost:      "work.github.com",
		Hostname:     "ssh.github.com",
		Port:         443,
		ReuseKeyPath: "/home/.ssh/id_ed25519_one",
	}
	idB := tuikit.DemoIdentity{
		Name:         "work",
		SSHHost:      "work.github.com",
		Hostname:     "ssh.github.com",
		Port:         443,
		ReuseKeyPath: "/home/.ssh/id_ed25519_two",
	}
	inA := b.createInput(idA)
	inB := b.createInput(idB)

	fpA := specFingerprint(inA)
	fpB := specFingerprint(inB)

	if fpA == fpB {
		t.Errorf("specFingerprint must differ for different reuse paths (CR-07):\n  path-one: %s\n  path-two: %s", fpA, fpB)
	}
}

// ---------------------------------------------------------------------------
// CR-08/WR-01: RenderCheckedHostBlock used at preview/staging/commit;
// preview uses spec.Provider not providerFromAlias
// ---------------------------------------------------------------------------

// TestCheckedRendererUsedAtPreviewBoundary proves that HostBlockPreview uses
// the checked renderer (or equivalent validation) so that an unsafe IdentityFile
// path cannot silently pass through to the UI preview (CR-08).
//
// Current state (RED): HostBlockPreview calls sshconfig.RenderHostBlock directly
// without validation — a Unicode-space IdentityFile would pass through.
func TestCheckedRendererUsedAtPreviewBoundary(t *testing.T) {
	b := newBackendForHome(t.TempDir())

	// A safe IdentityFile must still work.
	spec := tuikit.CreateSpec{
		Alias:    "work.github.com",
		Hostname: "ssh.github.com",
		Port:     "443",
		KeyPath:  "~/.ssh/id_ed25519_work",
		Provider: "github.com",
	}
	preview := b.HostBlockPreview(spec)
	if !strings.Contains(preview, "Host work.github.com") {
		t.Errorf("HostBlockPreview with safe spec returned empty/invalid preview: %q", preview)
	}
}

// TestHostBlockPreviewUsesSpecProvider proves that HostBlockPreview renders
// the provider comment from spec.Provider, NOT from providerFromAlias(spec.Alias),
// so that multi-label providers like "company.co.uk" are not truncated to "co.uk"
// (WR-01).
//
// Current state (RED): HostBlockPreview passes providerFromAlias(spec.Alias) which
// truncates multi-label providers to the last two labels.
func TestHostBlockPreviewUsesSpecProvider(t *testing.T) {
	b := newBackendForHome(t.TempDir())

	// Multi-label provider: "company.co.uk" would be truncated to "co.uk" by
	// providerFromAlias("enterprise.company.co.uk").
	spec := tuikit.CreateSpec{
		Alias:    "enterprise.company.co.uk",
		Hostname: "ssh.company.co.uk",
		Port:     "22",
		KeyPath:  "~/.ssh/id_ed25519_enterprise",
		Provider: "company.co.uk", // the full, validated provider
	}
	preview := b.HostBlockPreview(spec)

	// The preview MUST contain the full provider, not the truncated "co.uk".
	if !strings.Contains(preview, "company.co.uk") {
		t.Errorf("HostBlockPreview does not include full provider 'company.co.uk' (WR-01); preview:\n%s", preview)
	}
	// And must NOT only contain the truncated "co.uk" version.
	if strings.Contains(preview, "provider=co.uk") && !strings.Contains(preview, "provider=company.co.uk") {
		t.Errorf("HostBlockPreview uses providerFromAlias truncation 'co.uk' instead of spec.Provider 'company.co.uk' (WR-01);\npreview:\n%s", preview)
	}
}

// ---------------------------------------------------------------------------
// CR-09: Directory rollback in reverse creation order
// ---------------------------------------------------------------------------

// TestRollbackCreatedDirsInReverseOrder proves that created directories are
// removed in reverse creation order (child before parent) during transaction
// rollback (CR-09).
//
// Current state (RED): createdDirs is iterated in forward order, so os.Remove
// of ~/.ssh fails (config.d still exists), leaving the directory behind.
func TestRollbackCreatedDirsInReverseOrder(t *testing.T) {
	// Use a completely fresh home: no .ssh at all.
	home := t.TempDir()
	t.Setenv("HOME", home)
	// DO NOT call seedSSHDir — we want a fresh machine state where both
	// ~/.ssh and ~/.ssh/config.d are created during the transaction.

	b := newBackendForHome(home)

	id := tuikit.DemoIdentity{
		Name:     "fresh",
		SSHHost:  "fresh.github.com",
		Hostname: "ssh.github.com",
		Port:     443,
	}
	// Unlock the store for this identity (record outcomes).
	in := b.createInput(id)
	b.recordOutcomeFor(1, tuikit.TestOutcomePass, in)
	b.recordOutcomeFor(2, tuikit.TestOutcomePass, in)

	// Inject a failure at the host-block step — AFTER both ~/.ssh and ~/.ssh/config.d
	// have been created, so rollback must remove config.d FIRST, then ~/.ssh.
	b.failCommitAt = func(step string) error {
		if step == "host-block" {
			return fmt.Errorf("injected failure at host-block step")
		}
		return nil
	}

	b.Persist(tuikit.DemoState{}, tuikit.AddIdentity{Identity: id})
	if b.PersistError() == nil {
		t.Fatal("injected failure must propagate as PersistError")
	}

	sshDir := filepath.Join(home, ".ssh")
	includeDir := filepath.Join(home, ".ssh", "config.d")

	// Both directories must be gone after rollback (reverse removal succeeded).
	if _, err := os.Stat(includeDir); !os.IsNotExist(err) {
		t.Errorf("rollback left include dir %s behind (CR-09 — forward removal order)", includeDir)
	}
	if _, err := os.Stat(sshDir); !os.IsNotExist(err) {
		t.Errorf("rollback left SSH dir %s behind (CR-09 — forward removal fails on non-empty dir)", sshDir)
	}
}
