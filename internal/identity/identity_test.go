package identity

import (
	"errors"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/tester"
)

// fakeDeps builds a Deps whose function fields record invocation counts so the
// orchestration test can assert exactly which dependencies Create called.
type callLog struct {
	generate            int
	copyPub             int
	preWrite            int
	writeSSH            int
	writeGitconfig      int
	writeFragment       int
	writeAllowedSigners int
	resolved            int
	persistKey          int
	cleanup             int
}

func newFakeDeps(log *callLog, preOutcome tester.Outcome) Deps {
	return Deps{
		Generate: func(in CreateInput) (StagedKey, error) {
			log.generate++
			return StagedKey{
				TempPrivatePath:  "/tmp/stage/key",
				FinalPrivatePath: "/tmp/.ssh/id_ed25519_" + in.Name,
				FinalPubPath:     "/tmp/.ssh/id_ed25519_" + in.Name + ".pub",
				PubLine:          "ssh-ed25519 AAAAFAKEKEY comment\n",
				PrivPEM:          []byte("FAKEPEM"),
			}, nil
		},
		PersistKey: func(s StagedKey) (KeyResult, error) {
			log.persistKey++
			if s.PrivPEM == nil {
				// Existing-key path: return current paths without writing.
				return KeyResult{
					PrivatePath: s.FinalPrivatePath,
					PubPath:     s.FinalPubPath,
					PubLine:     s.PubLine,
				}, nil
			}
			return KeyResult{
				PrivatePath: s.FinalPrivatePath,
				PubPath:     s.FinalPubPath,
				PubLine:     s.PubLine,
			}, nil
		},
		Cleanup: func(_ StagedKey) {
			log.cleanup++
		},
		CopyPub: func(_ string) error {
			log.copyPub++
			return nil
		},
		PreWrite: func(keyPath, hostname string, _ int) tester.Result {
			log.preWrite++
			return tester.Result{
				Command: "ssh -i " + keyPath + " -T git@" + hostname,
				Output:  "pre-write output",
				Outcome: preOutcome,
			}
		},
		WriteSSH: func(_, _, _ string) (string, error) {
			log.writeSSH++
			return "", nil
		},
		WriteGitconfig: func(_, _, _ string, _ []gitconfig.Match) (string, error) {
			log.writeGitconfig++
			return "", nil
		},
		WriteFragment: func(_, _, _, _ string, _ bool) error {
			log.writeFragment++
			return nil
		},
		WriteAllowedSigners: func(_, _, _ string) (string, error) {
			log.writeAllowedSigners++
			return "", nil
		},
		Resolved: func(alias string) (tester.Result, tester.ResolvedConfig) {
			log.resolved++
			return tester.Result{
					Command: "ssh -T git@" + alias,
					Output:  "Hi! You've successfully authenticated",
					Outcome: tester.PASS,
				}, tester.ResolvedConfig{
					User:           "git",
					Hostname:       "github.com",
					Port:           "443",
					IdentitiesOnly: "yes",
					IdentityFiles:  []string{"/tmp/.ssh/id_ed25519_work"},
				}
		},
	}
}

func sampleInput() CreateInput {
	return CreateInput{
		Name:               "work",
		GitName:            "Work User",
		GitEmail:           "work@example.com",
		Provider:           "github",
		Algo:               "ed25519",
		Alias:              "work.github.com",
		Hostname:           "ssh.github.com",
		Port:               443,
		Matches:            []gitconfig.Match{DefaultMatch("work")},
		FragmentPath:       "/tmp/.gitconfig.d/work",
		GitconfigPath:      "/tmp/.gitconfig",
		SSHConfigPath:      "/tmp/.ssh/config",
		AllowedSignersPath: "/tmp/.ssh/allowed_signers",
		GlobalBlock:        "Host *\n  UseKeychain yes\n",
		Confirmed:          true,
	}
}

func TestCreateAbortsOnPreWriteFailure(t *testing.T) {
	var log callLog
	deps := newFakeDeps(&log, tester.Failure)

	_, err := Create(sampleInput(), deps)
	if err == nil {
		t.Fatal("Create() expected an error when pre-write test fails, got nil")
	}
	if log.writeSSH != 0 || log.writeGitconfig != 0 || log.writeFragment != 0 || log.writeAllowedSigners != 0 {
		t.Fatalf("Create() must perform NO writes on pre-write Failure; got writes ssh=%d gitconfig=%d fragment=%d signers=%d",
			log.writeSSH, log.writeGitconfig, log.writeFragment, log.writeAllowedSigners)
	}
	if log.resolved != 0 {
		t.Fatalf("Create() must not run the resolved test on Failure; ran %d times", log.resolved)
	}
}

func TestCreateProceedsOnReachableNotUploaded(t *testing.T) {
	var log callLog
	deps := newFakeDeps(&log, tester.ReachableNotUploaded)

	res, err := Create(sampleInput(), deps)
	if err != nil {
		t.Fatalf("Create() returned unexpected error on ReachableNotUploaded: %v", err)
	}

	// All FOUR writers invoked exactly once on a confirmed write.
	if log.writeSSH != 1 {
		t.Errorf("WriteSSH called %d times, want 1", log.writeSSH)
	}
	if log.writeGitconfig != 1 {
		t.Errorf("WriteGitconfig called %d times, want 1", log.writeGitconfig)
	}
	if log.writeFragment != 1 {
		t.Errorf("WriteFragment called %d times, want 1", log.writeFragment)
	}
	if log.writeAllowedSigners != 1 {
		t.Errorf("WriteAllowedSigners called %d times, want 1 (SIGN-01 fourth writer)", log.writeAllowedSigners)
	}
	if log.resolved != 1 {
		t.Errorf("Resolved called %d times, want 1", log.resolved)
	}
	if res.Resolved.User != "git" {
		t.Errorf("Resolved config user = %q, want git", res.Resolved.User)
	}
	if res.PreWrite.Outcome != tester.ReachableNotUploaded {
		t.Errorf("PreWrite outcome = %v, want ReachableNotUploaded", res.PreWrite.Outcome)
	}
}

func TestCreateDryRunSkipsWrites(t *testing.T) {
	var log callLog
	deps := newFakeDeps(&log, tester.ReachableNotUploaded)
	in := sampleInput()
	in.Confirmed = false // dry-run / unconfirmed: preview only

	res, err := Create(in, deps)
	if err != nil {
		t.Fatalf("Create() unconfirmed returned error: %v", err)
	}
	if log.writeSSH != 0 || log.writeGitconfig != 0 || log.writeFragment != 0 || log.writeAllowedSigners != 0 {
		t.Fatalf("Create() unconfirmed must perform NO writes; got ssh=%d gitconfig=%d fragment=%d signers=%d",
			log.writeSSH, log.writeGitconfig, log.writeFragment, log.writeAllowedSigners)
	}
	if log.resolved != 0 {
		t.Errorf("Create() unconfirmed must not run resolved test; ran %d", log.resolved)
	}
	// Previews are still produced for display.
	if res.SSHPreview == "" || res.GitconfigPreview == "" || res.AllowedSignersPreview == "" {
		t.Error("Create() unconfirmed must still return artifact previews")
	}
	if !res.PreWriteOnly {
		t.Error("Create() unconfirmed must mark the result as preview-only (no write performed)")
	}
}

func TestCreatePropagatesGenerateError(t *testing.T) {
	var log callLog
	deps := newFakeDeps(&log, tester.PASS)
	deps.Generate = func(_ CreateInput) (StagedKey, error) {
		log.generate++
		return StagedKey{}, errors.New("boom")
	}
	_, err := Create(sampleInput(), deps)
	if err == nil {
		t.Fatal("Create() expected generate error to propagate")
	}
	if log.preWrite != 0 {
		t.Error("Create() must not run pre-write test after a generate failure")
	}
}

func TestDefaultAlias(t *testing.T) {
	if got := DefaultAlias("work", "github"); got != "work.github" {
		t.Errorf("DefaultAlias = %q, want work.github", got)
	}
}

func TestDefaultMatch(t *testing.T) {
	m := DefaultMatch("work")
	if m.Kind != gitconfig.MatchGitdir {
		t.Errorf("DefaultMatch kind = %v, want MatchGitdir", m.Kind)
	}
	if m.Value != "~/git/work/" {
		t.Errorf("DefaultMatch value = %q, want ~/git/work/", m.Value)
	}
}

// TestCreatePassesHostnameNotAlias asserts that runPipeline (called from Create)
// dials in.Hostname + in.Port through PreWrite, NOT the SSH alias (in.Alias).
// BUG-1: prior to the fix, the call site used in.Alias ("work.github.com") which
// is unresolvable before the SSH config is written.
func TestCreatePassesHostnameNotAlias(t *testing.T) {
	in := sampleInput() // Alias="work.github.com", Hostname="ssh.github.com", Port=443

	var capturedHostname string
	var capturedPort int
	var capturedKeyPath string

	var log callLog
	deps := newFakeDeps(&log, tester.ReachableNotUploaded)
	// Override the PreWrite fake to capture the args for inspection.
	deps.PreWrite = func(keyPath, hostname string, port int) tester.Result {
		log.preWrite++
		capturedKeyPath = keyPath
		capturedHostname = hostname
		capturedPort = port
		return tester.Result{
			Command: "ssh -i " + keyPath + " -T git@" + hostname,
			Output:  "git@ssh.github.com: Permission denied (publickey).",
			Outcome: tester.ReachableNotUploaded,
		}
	}

	if _, err := Create(in, deps); err != nil {
		t.Fatalf("Create() returned unexpected error: %v", err)
	}

	// BUG-1: must dial in.Hostname, not in.Alias.
	if capturedHostname != in.Hostname {
		t.Errorf("PreWrite called with hostname=%q, want in.Hostname=%q (must NOT use alias %q)",
			capturedHostname, in.Hostname, in.Alias)
	}
	// BUG-2: must pass in.Port so port-443 endpoints are reachable.
	if capturedPort != in.Port {
		t.Errorf("PreWrite called with port=%d, want in.Port=%d", capturedPort, in.Port)
	}
	// Sanity: the key path from Generate is passed through.
	if capturedKeyPath == "" {
		t.Error("PreWrite called with empty keyPath")
	}
}

// --- New behavioral tests (BUG-4 temp-then-promote) ---

// TestCreateDryRun_PersistKeyCountZero asserts that a dry-run (Confirmed=false)
// Create records PersistKey count 0, Cleanup IS called, and the four artifact
// previews are all non-empty. PreWriteOnly must be true.
func TestCreateDryRun_PersistKeyCountZero(t *testing.T) {
	var log callLog
	deps := newFakeDeps(&log, tester.ReachableNotUploaded)
	in := sampleInput()
	in.Confirmed = false

	res, err := Create(in, deps)
	if err != nil {
		t.Fatalf("Create() dry-run returned error: %v", err)
	}
	if log.persistKey != 0 {
		t.Errorf("dry-run: PersistKey called %d times, want 0 (must not persist before confirm)", log.persistKey)
	}
	if log.cleanup != 1 {
		t.Errorf("dry-run: Cleanup called %d times, want 1 (defer must always fire)", log.cleanup)
	}
	if res.SSHPreview == "" || res.GitconfigPreview == "" || res.FragmentPreview == "" || res.AllowedSignersPreview == "" {
		t.Error("dry-run: all four artifact previews must be non-empty")
	}
	if !res.PreWriteOnly {
		t.Error("dry-run: PreWriteOnly must be true")
	}
}

// TestCreateGateFailure_PersistKeyCountZero asserts that a pre-write gate Failure
// records PersistKey count 0 (no orphan key), Cleanup IS called, and no writer ran.
func TestCreateGateFailure_PersistKeyCountZero(t *testing.T) {
	var log callLog
	deps := newFakeDeps(&log, tester.Failure)

	_, err := Create(sampleInput(), deps)
	if err == nil {
		t.Fatal("Create() gate-Failure must return an error")
	}
	if log.persistKey != 0 {
		t.Errorf("gate-Failure: PersistKey called %d times, want 0 (no orphan key)", log.persistKey)
	}
	if log.cleanup != 1 {
		t.Errorf("gate-Failure: Cleanup called %d times, want 1 (defer must always fire)", log.cleanup)
	}
	if log.writeSSH != 0 || log.writeGitconfig != 0 || log.writeFragment != 0 || log.writeAllowedSigners != 0 {
		t.Errorf("gate-Failure: no writer must run; got ssh=%d gitconfig=%d fragment=%d signers=%d",
			log.writeSSH, log.writeGitconfig, log.writeFragment, log.writeAllowedSigners)
	}
}

// TestCreateConfirmed_PersistKeyCountOneAndFinalPaths asserts that a confirmed
// create-new records PersistKey count exactly 1 (fires BEFORE the four writers),
// res.Key.PrivatePath equals the FINAL path, and the SSH/fragment previews
// reference the FINAL path, not the temp staging path.
func TestCreateConfirmed_PersistKeyCountOneAndFinalPaths(t *testing.T) {
	var log callLog
	// Track write order to confirm PersistKey fires before the four writers.
	var callOrder []string
	deps := newFakeDeps(&log, tester.ReachableNotUploaded)

	const tempPath = "/tmp/stage/key"
	const finalPath = "/tmp/.ssh/id_ed25519_work"

	deps.PersistKey = func(s StagedKey) (KeyResult, error) {
		log.persistKey++
		callOrder = append(callOrder, "persistKey")
		return KeyResult{PrivatePath: s.FinalPrivatePath, PubPath: s.FinalPubPath, PubLine: s.PubLine}, nil
	}
	deps.WriteSSH = func(_, _, _ string) (string, error) {
		log.writeSSH++
		callOrder = append(callOrder, "writeSSH")
		return "", nil
	}

	in := sampleInput()
	in.Confirmed = true

	res, err := Create(in, deps)
	if err != nil {
		t.Fatalf("Create() confirmed returned error: %v", err)
	}

	if log.persistKey != 1 {
		t.Errorf("confirmed: PersistKey called %d times, want exactly 1", log.persistKey)
	}
	if log.cleanup != 1 {
		t.Errorf("confirmed: Cleanup called %d times, want 1", log.cleanup)
	}

	// PersistKey must fire before WriteSSH.
	if len(callOrder) >= 2 && callOrder[0] != "persistKey" {
		t.Errorf("PersistKey must be called before WriteSSH; order was %v", callOrder)
	}

	// res.Key.PrivatePath must be the FINAL path.
	if res.Key.PrivatePath != finalPath {
		t.Errorf("res.Key.PrivatePath = %q, want FINAL path %q", res.Key.PrivatePath, finalPath)
	}

	// SSHPreview and FragmentPreview must reference FINAL path, never temp.
	if !strings.Contains(res.SSHPreview, finalPath) {
		t.Errorf("SSHPreview does not contain FINAL path %q:\n%s", finalPath, res.SSHPreview)
	}
	if strings.Contains(res.SSHPreview, tempPath) {
		t.Errorf("SSHPreview must not contain temp path %q:\n%s", tempPath, res.SSHPreview)
	}
	if strings.Contains(res.FragmentPreview, tempPath) {
		t.Errorf("FragmentPreview must not contain temp path %q:\n%s", tempPath, res.FragmentPreview)
	}
}

// TestCreateGate_UsesTempPath asserts that PreWrite is invoked with the
// StagedKey.TempPrivatePath, not the final path (BUG-4: gate must run ssh -i
// <temp> before any ~/.ssh write).
func TestCreateGate_UsesTempPath(t *testing.T) {
	var log callLog
	var capturedKeyPath string
	deps := newFakeDeps(&log, tester.ReachableNotUploaded)

	const tempPath = "/tmp/stage/key"
	const finalPath = "/tmp/.ssh/id_ed25519_work"

	deps.PreWrite = func(keyPath, _ string, _ int) tester.Result {
		log.preWrite++
		capturedKeyPath = keyPath
		return tester.Result{
			Command: "ssh -i " + keyPath,
			Output:  "pre-write output",
			Outcome: tester.ReachableNotUploaded,
		}
	}

	if _, err := Create(sampleInput(), deps); err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}

	if capturedKeyPath != tempPath {
		t.Errorf("PreWrite called with keyPath=%q, want TempPrivatePath=%q (not final %q)",
			capturedKeyPath, tempPath, finalPath)
	}
}

// TestReuseNoPersistKey asserts that Reuse records PersistKey count 0 (existing
// key, PrivPEM nil, so the persist call is skipped entirely).
func TestReuseNoPersistKey(t *testing.T) {
	var log callLog
	log2 := modeLog{callLog: log}
	log2.pubExistsRet = true
	deps := newFakeModeDeps(&log2, tester.ReachableNotUploaded)

	existingKey := "/tmp/.ssh/id_ed25519_existing"
	if _, err := Reuse(reuseInput(), existingKey, deps); err != nil {
		t.Fatalf("Reuse returned error: %v", err)
	}
	if log2.persistKey != 0 {
		t.Errorf("Reuse: PersistKey called %d times, want 0 (existing key, PrivPEM nil)", log2.persistKey)
	}
}

// TestAddAccountNoPersistKey asserts that AddAccount records PersistKey count 0
// (existing key, PrivPEM nil, so the persist call is skipped entirely).
func TestAddAccountNoPersistKey(t *testing.T) {
	var log modeLog
	log.pubExistsRet = true
	deps := newFakeModeDeps(&log, tester.ReachableNotUploaded)

	existing := Account{
		Name:     "work",
		GitName:  "Work User",
		GitEmail: "work@example.com",
		Provider: "github",
		Alias:    "work.github.com",
		Hostname: "ssh.github.com",
		Port:     443,
		KeyPath:  "/tmp/.ssh/id_ed25519_work",
		PubPath:  "/tmp/.ssh/id_ed25519_work.pub",
		Matches:  []gitconfig.Match{DefaultMatch("work")},
	}

	if _, err := AddAccount(existing, "gitlab", "work.gitlab.com", deps); err != nil {
		t.Fatalf("AddAccount returned error: %v", err)
	}
	if log.persistKey != 0 {
		t.Errorf("AddAccount: PersistKey called %d times, want 0 (existing key, PrivPEM nil)", log.persistKey)
	}
}
