package main

// doctor_realwiring_test.go: integration tests that drive the REAL buildDoctorDeps
// wiring against a temp HOME and assert on-disk effects. These tests are RED until
// Task 2 (GREEN) plumbs the real RemoveBlock/AddWiring closures through the check
// Fix.Fn fields.
//
// Contract (gap_closure_contract): each test MUST use buildDoctorDeps (or runDoctor)
// against a t.TempDir() home. No test may inject a fake RemoveBlock or AddWiring.
// Assertions are on REAL on-disk effects (os.ReadFile, os.Stat), never fake call counts.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/doctor/checks"
	"github.com/castocolina/gitid/internal/filewriter"
	"github.com/castocolina/gitid/internal/keygen"
	"github.com/castocolina/gitid/internal/sshconfig"
)

// seedOrphanSSHConfig creates a temp HOME with ~/.ssh/config containing a
// gitid-managed Host block named "orphanid" but NO matching gitconfig block,
// so buildDoctorDeps will classify it as an orphaned SSH block.
func seedOrphanSSHConfig(t *testing.T, home string) (sshConfigPath string) {
	t.Helper()
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("seedOrphanSSHConfig: MkdirAll .ssh: %v", err)
	}
	sshConfigPath = filepath.Join(sshDir, "config")

	// A gitid-managed SSH Host block with no corresponding gitconfig includeIf block.
	// buildDoctorDeps reads this via sshconfig.ParseManagedHosts → SSHManagedBlockNames,
	// but the gitconfig managed block list will be empty → orphan detected.
	managedBlock := filewriter.BeginPrefix + "orphanid\n" +
		sshconfig.RenderHostBlock("github.com", "github.com", 22, filepath.Join(home, ".ssh", "id_orphanid")) +
		filewriter.EndPrefix + "orphanid\n"
	if err := os.WriteFile(sshConfigPath, []byte(managedBlock), 0o600); err != nil {
		t.Fatalf("seedOrphanSSHConfig: WriteFile config: %v", err)
	}
	return sshConfigPath
}

// TestDoctorRealWiring1_OrphanBlockRemoval (DOC-04/DOC-06):
// Seeds ~/.ssh/config with a gitid-managed Host block "orphanid" and NO matching
// gitconfig includeIf block. Builds deps via buildDoctorDeps, locates the orphan
// finding (Family=Orphans, Fix != nil), calls Fix.Fn(), then re-reads ~/.ssh/config
// and asserts the managed block is ABSENT and a timestamped backup exists.
func TestDoctorRealWiring1_OrphanBlockRemoval(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	sshConfigPath := seedOrphanSSHConfig(t, home)
	sshBytes, err := os.ReadFile(sshConfigPath) //nolint:gosec // test-controlled path
	if err != nil {
		t.Fatalf("reading seeded ssh config: %v", err)
	}

	d := buildDoctorDeps(home, sshBytes, nil)
	findings := checks.CheckOrphans(d)

	// Locate the orphaned SSH block finding (Class 1) with Fix != nil.
	var orphanFinding *doctor.Finding
	for i, f := range findings {
		if f.Family == doctor.FamilyOrphans && f.Fix != nil && strings.Contains(f.Title, "orphanid") {
			orphanFinding = &findings[i]
			break
		}
	}
	if orphanFinding == nil {
		t.Fatalf("expected orphan finding for 'orphanid' with Fix != nil; findings: %+v", findings)
	}

	// Call the real Fix.Fn — must remove the block from disk.
	if err := orphanFinding.Fix.Fn(); err != nil {
		t.Fatalf("Fix.Fn() returned error: %v", err)
	}

	// Assert: the managed block is ABSENT from ~/.ssh/config.
	afterContent, err := os.ReadFile(sshConfigPath) //nolint:gosec // test-controlled path
	if err != nil {
		t.Fatalf("reading config after Fix.Fn: %v", err)
	}
	if strings.Contains(string(afterContent), "BEGIN gitid managed: orphanid") {
		t.Errorf("managed block 'orphanid' still present after Fix.Fn; content:\n%s", afterContent)
	}

	// Assert: a timestamped backup config.bak.* exists in ~/.ssh.
	entries, globErr := filepath.Glob(sshConfigPath + ".bak.*")
	if globErr != nil || len(entries) == 0 {
		t.Errorf("expected a timestamped backup after Fix.Fn; found none (glob: %v)", globErr)
	}
}

// TestDoctorRealWiring2_AllowedSignersModeAfterReAdd (DOC-06 + WR-02):
// Seeds an identity whose allowed_signers entry is missing, drives the
// allowed_signers re-add Fix.Fn, then asserts the entry line now exists in
// ~/.ssh/allowed_signers AND that the file mode is 0644 (NOT 0600).
func TestDoctorRealWiring2_AllowedSignersModeAfterReAdd(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("MkdirAll .ssh: %v", err)
	}
	gitconfigDDir := filepath.Join(home, ".gitconfig.d")
	if err := os.MkdirAll(gitconfigDDir, 0o700); err != nil { //nolint:gosec // test-controlled temp dir; 0700 matches real ~/.gitconfig.d
		t.Fatalf("MkdirAll .gitconfig.d: %v", err)
	}

	// Generate a real Ed25519 key pair so that DerivePublicKey works on it.
	keyPath := filepath.Join(sshDir, "id_ed25519_sigtest")
	pubPath := keyPath + ".pub"
	pubLine, privBytes, err := generateTestKeyPair(t)
	if err != nil {
		t.Fatalf("generating test key: %v", err)
	}
	if err := os.WriteFile(keyPath, privBytes, 0o600); err != nil {
		t.Fatalf("writing private key: %v", err)
	}
	if err := os.WriteFile(pubPath, []byte(pubLine), 0o644); err != nil { //nolint:gosec // test public key
		t.Fatalf("writing public key: %v", err)
	}

	const identityName = "sigtest"
	const email = "sigtest@example.com"

	// Seed ~/.ssh/config with a gitid-managed Host block for identityName.
	sshConfigContent := filewriter.BeginPrefix + identityName + "\n" +
		sshconfig.RenderHostBlock("github.com", "github.com", 22, keyPath) +
		filewriter.EndPrefix + identityName + "\n"
	sshConfigPath := filepath.Join(sshDir, "config")
	if err := os.WriteFile(sshConfigPath, []byte(sshConfigContent), 0o600); err != nil {
		t.Fatalf("writing ssh config: %v", err)
	}

	// Seed ~/.gitconfig with a gitid-managed includeIf block for identityName.
	fragmentPath := filepath.Join(gitconfigDDir, identityName)
	gitconfigContent := fmt.Sprintf(
		"# BEGIN gitid managed: %s\n[includeIf \"gitdir:~/git/%s/\"]\n\tpath = %s\n# END gitid managed: %s\n",
		identityName, identityName, fragmentPath, identityName,
	)
	gitconfigPath := filepath.Join(home, ".gitconfig")
	if err := os.WriteFile(gitconfigPath, []byte(gitconfigContent), 0o644); err != nil { //nolint:gosec // test gitconfig
		t.Fatalf("writing gitconfig: %v", err)
	}

	// Seed the fragment with gpg.format=ssh and user.email so CheckCoherence can read it.
	fragmentContent := fmt.Sprintf(
		"[user]\n\tname = Sig Test\n\temail = %s\n\tsigningkey = %s\n[gpg]\n\tformat = ssh\n[commit]\n\tgpgsign = true\n",
		email, pubPath,
	)
	if err := os.WriteFile(fragmentPath, []byte(fragmentContent), 0o644); err != nil { //nolint:gosec // test fragment
		t.Fatalf("writing fragment: %v", err)
	}

	sshBytes, err := os.ReadFile(sshConfigPath) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("reading ssh config bytes: %v", err)
	}
	gcBytes, err := os.ReadFile(gitconfigPath) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("reading gitconfig bytes: %v", err)
	}

	d := buildDoctorDeps(home, sshBytes, gcBytes)
	findings := checks.CheckCoherence(d)

	// Locate the allowed_signers-missing finding (Fix != nil) for this identity.
	var missingFinding *doctor.Finding
	for i, f := range findings {
		if f.Family == doctor.FamilyCoherence && f.Fix != nil &&
			strings.Contains(f.Title, email) {
			missingFinding = &findings[i]
			break
		}
	}
	if missingFinding == nil {
		t.Fatalf("expected allowed_signers-missing finding for email %q with Fix != nil; findings: %+v", email, findings)
	}

	// Call the real Fix.Fn — must re-add the entry to ~/.ssh/allowed_signers.
	if err := missingFinding.Fix.Fn(); err != nil {
		t.Fatalf("allowed_signers Fix.Fn() returned error: %v", err)
	}

	allowedSignersPath := filepath.Join(sshDir, "allowed_signers")

	// Assert: the entry line now exists in ~/.ssh/allowed_signers.
	signerContent, err := os.ReadFile(allowedSignersPath) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("reading allowed_signers after Fix.Fn: %v", err)
	}
	if !strings.Contains(string(signerContent), email) {
		t.Errorf("allowed_signers does not contain email %q after Fix.Fn; content:\n%s", email, signerContent)
	}
	if !strings.Contains(string(signerContent), `namespaces="git"`) {
		t.Errorf("allowed_signers missing namespaces=\"git\" after Fix.Fn; content:\n%s", signerContent)
	}

	// Assert: the file mode is 0644 (WR-02 — NOT 0600).
	fi, err := os.Stat(allowedSignersPath)
	if err != nil {
		t.Fatalf("stat allowed_signers after Fix.Fn: %v", err)
	}
	if fi.Mode().Perm() != 0o644 {
		t.Errorf("allowed_signers mode = %04o; want 0644 (WR-02)", fi.Mode().Perm())
	}
}

// TestDoctorRealWiring3_ReportOnlyStaysUnfixable (D-03):
// Seeds a configuration that produces an unused-key finding (a managed key on disk
// referenced by no Host block) and a missing-IdentityFile coherence finding.
// Asserts BOTH have Fix == nil so buildDoctorDeps/applyFixes can never claim a
// phantom repair for them.
func TestDoctorRealWiring3_ReportOnlyStaysUnfixable(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("MkdirAll .ssh: %v", err)
	}
	gitconfigDDir := filepath.Join(home, ".gitconfig.d")
	if err := os.MkdirAll(gitconfigDDir, 0o700); err != nil { //nolint:gosec // test-controlled temp dir
		t.Fatalf("MkdirAll .gitconfig.d: %v", err)
	}

	const identityName = "report_only_test"

	// Create a key file that exists on disk but is not referenced in any SSH Host block.
	unusedKeyPath := filepath.Join(sshDir, "id_ed25519_unused")
	if err := os.WriteFile(unusedKeyPath, []byte("fake-private-key\n"), 0o600); err != nil {
		t.Fatalf("writing unused key: %v", err)
	}

	// Seed ~/.ssh/config with a managed Host block that references a DIFFERENT
	// (non-existent) key path, so the unused key above is never referenced.
	missingKeyPath := filepath.Join(sshDir, "id_ed25519_missing")
	sshConfigContent := filewriter.BeginPrefix + identityName + "\n" +
		sshconfig.RenderHostBlock("github.com", "github.com", 22, missingKeyPath) +
		filewriter.EndPrefix + identityName + "\n"
	sshConfigPath := filepath.Join(sshDir, "config")
	if err := os.WriteFile(sshConfigPath, []byte(sshConfigContent), 0o600); err != nil {
		t.Fatalf("writing ssh config: %v", err)
	}

	// Seed ~/.gitconfig with a gitid-managed includeIf block for identityName.
	fragmentPath := filepath.Join(gitconfigDDir, identityName)
	gitconfigContent := fmt.Sprintf(
		"# BEGIN gitid managed: %s\n[includeIf \"gitdir:~/git/%s/\"]\n\tpath = %s\n# END gitid managed: %s\n",
		identityName, identityName, fragmentPath, identityName,
	)
	gitconfigPath := filepath.Join(home, ".gitconfig")
	if err := os.WriteFile(gitconfigPath, []byte(gitconfigContent), 0o644); err != nil { //nolint:gosec // test gitconfig
		t.Fatalf("writing gitconfig: %v", err)
	}

	// Seed a fragment pointing to the non-existent key, so Reconstruct sees a real account.
	fragmentContent := fmt.Sprintf(
		"[user]\n\tname = Report Only\n\temail = report_only@example.com\n\tsigningkey = %s.pub\n[gpg]\n\tformat = ssh\n[commit]\n\tgpgsign = true\n",
		missingKeyPath,
	)
	if err := os.WriteFile(fragmentPath, []byte(fragmentContent), 0o644); err != nil { //nolint:gosec // test fragment
		t.Fatalf("writing fragment: %v", err)
	}

	// The identity's KeyPath reconstructs to missingKeyPath (does not exist on disk).
	// The unused key unusedKeyPath IS on disk but is NOT referenced in any Host block.
	// We need buildDoctorDeps to see unusedKeyPath as a KeyPath.
	// Reconstruct only includes keys from the SSHHostInfo. The unused key is not in
	// any Host block, so we must add it to KeyPaths manually by seeding a second identity.
	// Actually, KeyPaths comes from Reconstruct, which only includes keys referenced
	// by managed Host blocks. Since unusedKeyPath is never in a Host block, Reconstruct
	// won't include it in KeyPaths directly.
	// HOWEVER: CheckOrphans uses deps.KeyPaths for Class 3 (unused key). The key must
	// be in KeyPaths to be checked. Since our test seeds are from managed blocks, the
	// unused key won't appear in KeyPaths from Reconstruct.
	// To test the unused-key report-only behavior, we need a key that IS in KeyPaths
	// (i.e. referenced by an account) but NOT in AllSSHHostIdentityFiles.
	// Actually, for Class 3: the key IS in deps.KeyPaths but NOT in AllSSHHostIdentityFiles.
	// missingKeyPath IS in KeyPaths (via the managed Host block) but doesn't exist on disk,
	// so CheckOrphans' Stat check skips it (os.IsNotExist → continue).
	// Instead, let's check:
	// - IdentityFile missing → coherence finding, Fix == nil (Check 1 in coherence.go).
	// - We can skip unused-key for this test since the account using the unused key
	//   would need a different setup.

	sshBytes, err := os.ReadFile(sshConfigPath) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("reading ssh config bytes: %v", err)
	}
	gcBytes, err := os.ReadFile(gitconfigPath) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("reading gitconfig bytes: %v", err)
	}

	d := buildDoctorDeps(home, sshBytes, gcBytes)

	// The IdentityFile-missing coherence finding must have Fix == nil (D-03).
	coherenceFindings := checks.CheckCoherence(d)
	var missingIDFileFinding *doctor.Finding
	for i, f := range coherenceFindings {
		if f.Family == doctor.FamilyCoherence && f.Fix == nil &&
			strings.Contains(f.Title, "IdentityFile") {
			missingIDFileFinding = &coherenceFindings[i]
			break
		}
	}
	if missingIDFileFinding == nil {
		t.Errorf("expected a report-only IdentityFile-missing coherence finding (Fix==nil); coherence findings: %+v", coherenceFindings)
	}

	// Verify the unused-key finding is also Fix==nil.
	// For this we need a key in KeyPaths that exists on disk but is not in AllSSHHostIdentityFiles.
	// We can set up the deps directly from the built deps for clarity.
	// unusedKeyPath is NOT in d.KeyPaths (Reconstruct only adds keys from managed Host blocks).
	// The Class 3 check uses d.KeyPaths. Let's confirm that adding the unusedKeyPath to KeyPaths
	// and running CheckOrphans produces a Fix==nil finding.
	dWithUnused := d
	dWithUnused.KeyPaths = append(dWithUnused.KeyPaths, unusedKeyPath)
	orphanFindings := checks.CheckOrphans(dWithUnused)
	var unusedKeyFinding *doctor.Finding
	for i, f := range orphanFindings {
		if f.Family == doctor.FamilyOrphans && strings.Contains(f.Title, unusedKeyPath) {
			unusedKeyFinding = &orphanFindings[i]
			break
		}
	}
	if unusedKeyFinding == nil {
		t.Errorf("expected unused-key finding for %q; orphan findings: %+v", unusedKeyPath, orphanFindings)
	} else if unusedKeyFinding.Fix != nil {
		t.Errorf("unused-key finding must have Fix==nil (D-03/D-13); got non-nil Fix: %+v", unusedKeyFinding.Fix)
	}
}

// TestDoctorRealWiring4_WR01EmailMismatchFalsePositive (WR-01):
// Seeds ~/.ssh/allowed_signers with a case-differing line for an email FOLLOWED
// BY a byte-exact line for the same email (both with namespaces="git"). Builds
// deps, runs CheckCoherence via doctor.Run, and asserts NO "email mismatch"
// finding is produced for that identity (the exact match later in the file must win).
func TestDoctorRealWiring4_WR01EmailMismatchFalsePositive(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("MkdirAll .ssh: %v", err)
	}
	gitconfigDDir := filepath.Join(home, ".gitconfig.d")
	if err := os.MkdirAll(gitconfigDDir, 0o700); err != nil { //nolint:gosec // test-controlled temp dir
		t.Fatalf("MkdirAll .gitconfig.d: %v", err)
	}

	// Generate a test key pair.
	pubLine, privBytes, err := generateTestKeyPair(t)
	if err != nil {
		t.Fatalf("generating test key: %v", err)
	}
	keyPath := filepath.Join(sshDir, "id_ed25519_wr01")
	pubPath := keyPath + ".pub"
	if err := os.WriteFile(keyPath, privBytes, 0o600); err != nil {
		t.Fatalf("writing private key: %v", err)
	}
	if err := os.WriteFile(pubPath, []byte(pubLine), 0o644); err != nil { //nolint:gosec // test public key
		t.Fatalf("writing public key: %v", err)
	}

	const identityName = "wr01test"
	const email = "WR01Test@Example.Com" // byte-exact identity email (note mixed case)

	// Seed ~/.ssh/config with a managed Host block.
	sshConfigContent := filewriter.BeginPrefix + identityName + "\n" +
		sshconfig.RenderHostBlock("github.com", "github.com", 22, keyPath) +
		filewriter.EndPrefix + identityName + "\n"
	sshConfigPath := filepath.Join(sshDir, "config")
	if err := os.WriteFile(sshConfigPath, []byte(sshConfigContent), 0o600); err != nil {
		t.Fatalf("writing ssh config: %v", err)
	}

	// Seed ~/.gitconfig with includeIf block.
	fragmentPath := filepath.Join(gitconfigDDir, identityName)
	gitconfigContent := fmt.Sprintf(
		"# BEGIN gitid managed: %s\n[includeIf \"gitdir:~/git/%s/\"]\n\tpath = %s\n# END gitid managed: %s\n",
		identityName, identityName, fragmentPath, identityName,
	)
	gitconfigPath := filepath.Join(home, ".gitconfig")
	if err := os.WriteFile(gitconfigPath, []byte(gitconfigContent), 0o644); err != nil { //nolint:gosec // test gitconfig
		t.Fatalf("writing gitconfig: %v", err)
	}

	// Seed fragment with gpg.format=ssh and user.email == email (byte-exact).
	fragmentContent := fmt.Sprintf(
		"[user]\n\tname = WR01 Test\n\temail = %s\n\tsigningkey = %s\n[gpg]\n\tformat = ssh\n[commit]\n\tgpgsign = true\n",
		email, pubPath,
	)
	if err := os.WriteFile(fragmentPath, []byte(fragmentContent), 0o644); err != nil { //nolint:gosec // test fragment
		t.Fatalf("writing fragment: %v", err)
	}

	// Seed ~/.ssh/allowed_signers with TWO lines for this identity:
	//   Line 1: a CASE-DIFFERING version of the email (lowercase).
	//   Line 2: the BYTE-EXACT email (mixed case).
	// The current (buggy) findSignerLine returns on first case-fold match → reports mismatch.
	// The fixed findSignerLine continues scanning and returns the exact match → no mismatch.
	caseDiffLine := keygen.AllowedSignersLine(strings.ToLower(email), strings.TrimRight(pubLine, "\n"))
	exactLine := keygen.AllowedSignersLine(email, strings.TrimRight(pubLine, "\n"))
	allowedSignersContent := caseDiffLine + exactLine
	allowedSignersPath := filepath.Join(sshDir, "allowed_signers")
	if err := os.WriteFile(allowedSignersPath, []byte(allowedSignersContent), 0o644); err != nil { //nolint:gosec // test allowed_signers
		t.Fatalf("writing allowed_signers: %v", err)
	}

	sshBytes, err := os.ReadFile(sshConfigPath) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("reading ssh config bytes: %v", err)
	}
	gcBytes, err := os.ReadFile(gitconfigPath) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("reading gitconfig bytes: %v", err)
	}

	d := buildDoctorDeps(home, sshBytes, gcBytes)
	findings := checks.CheckCoherence(d)

	// Assert: NO "email mismatch" finding for this identity.
	// The exact match later in the file should win over the case-differing line.
	for _, f := range findings {
		if f.Family == doctor.FamilyCoherence && strings.Contains(f.Title, "mismatch") &&
			strings.Contains(f.Title, identityName) {
			t.Errorf("WR-01: spurious 'email mismatch' finding for identity %q; "+
				"an exact signer line exists after the case-differing line. finding: %+v", identityName, f)
		}
	}
}

// generateTestKeyPair generates an Ed25519 key pair in memory using
// keygen.GenerateMaterial and returns (pubLine, privBytes, error).
// It writes nothing to disk — the caller places the bytes where needed.
func generateTestKeyPair(t *testing.T) (pubLine string, privBytes []byte, err error) {
	t.Helper()
	m, genErr := keygen.GenerateMaterial(keygen.Params{Algo: "ed25519", Identity: "test", Comment: "test@gitid"})
	if genErr != nil {
		return "", nil, fmt.Errorf("keygen.GenerateMaterial: %w", genErr)
	}
	return m.PubLine, m.PrivPEM, nil
}
