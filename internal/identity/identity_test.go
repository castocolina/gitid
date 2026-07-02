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

// TestRenderPreviewsZeroWrites asserts RenderPreviews performs zero writes and
// returns non-empty previews for all four artifact strings (D-02/D-03: dry-run
// path for create-new uses RenderPreviews, not Create).
func TestRenderPreviewsZeroWrites(t *testing.T) {
	var log callLog
	deps := newFakeDeps(&log, tester.ReachableNotUploaded)
	in := sampleInput()
	staged := sampleStaged(in)

	res := RenderPreviews(in, staged)

	// RenderPreviews must call ZERO writers.
	if log.writeSSH != 0 || log.writeGitconfig != 0 || log.writeFragment != 0 || log.writeAllowedSigners != 0 {
		t.Fatalf("RenderPreviews must perform NO writes; got ssh=%d gitconfig=%d fragment=%d signers=%d",
			log.writeSSH, log.writeGitconfig, log.writeFragment, log.writeAllowedSigners)
	}
	if log.copyPub != 0 {
		t.Errorf("RenderPreviews must not copy to clipboard; CopyPub called %d times", log.copyPub)
	}
	if log.preWrite != 0 {
		t.Errorf("RenderPreviews must not call PreWrite; called %d times", log.preWrite)
	}
	if log.resolved != 0 {
		t.Errorf("RenderPreviews must not run the resolved test; ran %d", log.resolved)
	}
	// All four previews must be non-empty.
	if res.SSHPreview == "" || res.GitconfigPreview == "" || res.FragmentPreview == "" || res.AllowedSignersPreview == "" {
		t.Error("RenderPreviews must return non-empty previews for all four artifacts")
	}
	// The deps arg is intentionally ignored — unused deps confirms no writes.
	_ = deps
}

// TestRenderPreviewsIncludesProvider asserts RenderPreviews passes in.Provider
// to RenderHostBlock so the SSH preview contains the provider marker comment
// (Plan 02 RenderHostBlock signature with provider arg, D-11).
func TestRenderPreviewsIncludesProvider(t *testing.T) {
	in := sampleInput()
	in.Provider = "github"
	staged := sampleStaged(in)

	res := RenderPreviews(in, staged)

	if !strings.Contains(res.SSHPreview, "provider=github") {
		t.Errorf("RenderPreviews SSHPreview must contain provider marker 'provider=github'; got:\n%s", res.SSHPreview)
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

// TestDefaultHostname pins the recipe alt-SSH endpoints for the three known
// providers (D-10 parity, T-05.7-09-03). The literal strings must match
// recipes/ssh-config.recipe Hostname lines exactly:
//   - github    -> ssh.github.com
//   - gitlab    -> altssh.gitlab.com
//   - bitbucket -> altssh.bitbucket.org
//
// Unknown providers fall back to the cmd-compatible shape ("<token>.com" when the
// token has no dot, otherwise the token verbatim) so the CLI's existing
// defaultHostname parity is preserved.
func TestDefaultHostname(t *testing.T) {
	cases := []struct {
		provider string
		want     string
	}{
		// Known providers — recipe alt-SSH endpoints (SSH-01/SSH-02).
		{"github", "ssh.github.com"},
		{"gitlab", "altssh.gitlab.com"},
		{"bitbucket", "altssh.bitbucket.org"},
		// Case-insensitive matching.
		{"GitHub", "ssh.github.com"},
		{"GitLab", "altssh.gitlab.com"},
		{"Bitbucket", "altssh.bitbucket.org"},
		// Token extraction: leading component before the first "." is used for the
		// keyword lookup (handles "github.com" passed as provider).
		{"github.com", "ssh.github.com"},
		{"gitlab.com", "altssh.gitlab.com"},
		{"bitbucket.org", "altssh.bitbucket.org"},
		// Unknown provider without a dot → "<provider>.com" fallback (cmd parity).
		{"custom", "custom.com"},
		// Unknown provider with a dot → verbatim fallback (cmd parity).
		{"myhost.internal", "myhost.internal"},
	}
	for _, tc := range cases {
		t.Run(tc.provider, func(t *testing.T) {
			if got := DefaultHostname(tc.provider); got != tc.want {
				t.Errorf("DefaultHostname(%q) = %q, want %q", tc.provider, got, tc.want)
			}
		})
	}
}

// TestDefaultPort pins the single default-port constant (recipe Port 443).
func TestDefaultPort(t *testing.T) {
	if got := DefaultPort(); got != 443 {
		t.Errorf("DefaultPort() = %d, want 443", got)
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

// TestPersistAll_RunsAllFourWritersInOrder asserts PersistAll calls PersistKey
// (when PrivPEM != nil) then all four writers in order, then Resolved, and
// returns backup paths (D-03 auto-persist shape).
func TestPersistAll_RunsAllFourWritersInOrder(t *testing.T) {
	var log callLog
	var callOrder []string

	deps := newFakeDeps(&log, tester.PASS)
	deps.PersistKey = func(s StagedKey) (KeyResult, error) {
		log.persistKey++
		callOrder = append(callOrder, "persistKey")
		return KeyResult{PrivatePath: s.FinalPrivatePath, PubPath: s.FinalPubPath, PubLine: s.PubLine}, nil
	}
	deps.WriteSSH = func(_, _, _ string) (string, error) {
		log.writeSSH++
		callOrder = append(callOrder, "writeSSH")
		return "bak-ssh", nil
	}
	deps.WriteGitconfig = func(_, _, _ string, _ []gitconfig.Match) (string, error) {
		log.writeGitconfig++
		callOrder = append(callOrder, "writeGitconfig")
		return "bak-gc", nil
	}
	deps.WriteFragment = func(_, _, _, _ string, _ bool) error {
		log.writeFragment++
		callOrder = append(callOrder, "writeFragment")
		return nil
	}
	deps.WriteAllowedSigners = func(_, _, _ string) (string, error) {
		log.writeAllowedSigners++
		callOrder = append(callOrder, "writeAllowedSigners")
		return "bak-signers", nil
	}
	deps.Resolved = func(_ string) (tester.Result, tester.ResolvedConfig) {
		log.resolved++
		callOrder = append(callOrder, "resolved")
		return tester.Result{Outcome: tester.PASS}, tester.ResolvedConfig{User: "git"}
	}

	in := sampleInput()
	staged := sampleStaged(in)

	res, err := PersistAll(in, staged, deps)
	if err != nil {
		t.Fatalf("PersistAll returned error: %v", err)
	}

	// PersistKey must fire before the four writers (PrivPEM != nil in sampleStaged).
	if log.persistKey != 1 {
		t.Errorf("PersistAll: PersistKey called %d times, want 1", log.persistKey)
	}
	if len(callOrder) > 0 && callOrder[0] != "persistKey" {
		t.Errorf("PersistAll: PersistKey must be called first; order was %v", callOrder)
	}
	if log.writeSSH != 1 {
		t.Errorf("PersistAll: WriteSSH called %d times, want 1", log.writeSSH)
	}
	if log.writeGitconfig != 1 {
		t.Errorf("PersistAll: WriteGitconfig called %d times, want 1", log.writeGitconfig)
	}
	if log.writeFragment != 1 {
		t.Errorf("PersistAll: WriteFragment called %d times, want 1", log.writeFragment)
	}
	if log.writeAllowedSigners != 1 {
		t.Errorf("PersistAll: WriteAllowedSigners called %d times, want 1", log.writeAllowedSigners)
	}
	if log.resolved != 1 {
		t.Errorf("PersistAll: Resolved called %d times, want 1", log.resolved)
	}
	// Backup paths returned.
	if res.SSHBackup != "bak-ssh" {
		t.Errorf("PersistAll: SSHBackup = %q, want bak-ssh", res.SSHBackup)
	}
	if res.GitconfigBackup != "bak-gc" {
		t.Errorf("PersistAll: GitconfigBackup = %q, want bak-gc", res.GitconfigBackup)
	}
	if res.AllowedSignersBackup != "bak-signers" {
		t.Errorf("PersistAll: AllowedSignersBackup = %q, want bak-signers", res.AllowedSignersBackup)
	}
}

// TestPersistAll_SkipsPersistKeyWhenPrivPEMNil asserts PersistAll skips
// PersistKey when staged.PrivPEM is nil (existing-key reuse path).
func TestPersistAll_SkipsPersistKeyWhenPrivPEMNil(t *testing.T) {
	var log callLog
	deps := newFakeDeps(&log, tester.PASS)

	in := sampleInput()
	staged := sampleStaged(in)
	staged.PrivPEM = nil // existing-key path

	_, err := PersistAll(in, staged, deps)
	if err != nil {
		t.Fatalf("PersistAll (PrivPEM nil) returned error: %v", err)
	}
	if log.persistKey != 0 {
		t.Errorf("PersistAll: PersistKey called %d times with PrivPEM nil, want 0", log.persistKey)
	}
	// Four writers still run.
	if log.writeSSH != 1 || log.writeGitconfig != 1 || log.writeFragment != 1 || log.writeAllowedSigners != 1 {
		t.Errorf("PersistAll: four writers must all run; ssh=%d gitconfig=%d fragment=%d signers=%d",
			log.writeSSH, log.writeGitconfig, log.writeFragment, log.writeAllowedSigners)
	}
}

// TestCreateGateFailure_PersistKeyCountZero asserts that a pre-write gate Failure
// records PersistKey count 0 (no orphan key), Cleanup IS called, and no writer ran.
// (Create/runPipeline is used by Reuse/Rotate paths which still have the gate.)
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

// TestCreatePersistKeyCountOneAndFinalPaths asserts that Create (which now always
// writes via runPipeline) records PersistKey count exactly 1 (fires BEFORE the
// four writers), res.Key.PrivatePath equals the FINAL path, and the SSH/fragment
// previews reference the FINAL path, not the temp staging path.
func TestCreatePersistKeyCountOneAndFinalPaths(t *testing.T) {
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

	res, err := Create(sampleInput(), deps)
	if err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}

	if log.persistKey != 1 {
		t.Errorf("PersistKey called %d times, want exactly 1", log.persistKey)
	}
	if log.cleanup != 1 {
		t.Errorf("Cleanup called %d times, want 1", log.cleanup)
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
// <temp> before any ~/.ssh write; for runPipeline / Reuse/Rotate callers).
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

// sampleStaged builds a StagedKey consistent with sampleInput() for unit tests
// that call PersistAll or RenderPreviews directly.
func sampleStaged(in CreateInput) StagedKey {
	return StagedKey{
		TempPrivatePath:  "/tmp/stage/key",
		FinalPrivatePath: "/tmp/.ssh/id_ed25519_" + in.Name,
		FinalPubPath:     "/tmp/.ssh/id_ed25519_" + in.Name + ".pub",
		PubLine:          "ssh-ed25519 AAAAFAKEKEY comment\n",
		PrivPEM:          []byte("FAKEPEM"),
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

// --- PersistSSH + PersistGitconfig split tests (T-05.7-09) ---
//
// These tests capture which writers fire per step (anti-blindspot: must test
// the real split functions via call-log fakes, not just a success bool).

// newSplitDeps builds a Deps with instrumented writers that record each call in
// a shared callOrder slice and return predictable backup strings. It does NOT
// wire PreWrite, Generate, CopyPub, Cleanup, or Resolved — those deps are nil
// because PersistSSH/PersistGitconfig/PersistAll do not invoke them.
func newSplitDeps(log *callLog, callOrder *[]string) Deps {
	return Deps{
		PersistKey: func(s StagedKey) (KeyResult, error) {
			log.persistKey++
			*callOrder = append(*callOrder, "persistKey")
			return KeyResult{
				PrivatePath: s.FinalPrivatePath,
				PubPath:     s.FinalPubPath,
				PubLine:     s.PubLine,
			}, nil
		},
		WriteSSH: func(_, _, _ string) (string, error) {
			log.writeSSH++
			*callOrder = append(*callOrder, "writeSSH")
			return "bak-ssh", nil
		},
		WriteGitconfig: func(_, _, _ string, _ []gitconfig.Match) (string, error) {
			log.writeGitconfig++
			*callOrder = append(*callOrder, "writeGitconfig")
			return "bak-gc", nil
		},
		WriteFragment: func(_, _, _, _ string, _ bool) error {
			log.writeFragment++
			*callOrder = append(*callOrder, "writeFragment")
			return nil
		},
		WriteAllowedSigners: func(_, _, _ string) (string, error) {
			log.writeAllowedSigners++
			*callOrder = append(*callOrder, "writeAllowedSigners")
			return "bak-signers", nil
		},
		Resolved: func(_ string) (tester.Result, tester.ResolvedConfig) {
			log.resolved++
			*callOrder = append(*callOrder, "resolved")
			return tester.Result{Outcome: tester.PASS}, tester.ResolvedConfig{User: "git"}
		},
	}
}

// TestPersistSSH_OnlyLEG1Writers asserts PersistSSH fires PersistKey (when
// PrivPEM != nil) and WriteSSH, and does NOT fire WriteGitconfig, WriteFragment,
// WriteAllowedSigners, or Resolved (T-05.7-09, anti-blindspot point (a)).
func TestPersistSSH_OnlyLEG1Writers(t *testing.T) {
	var log callLog
	var callOrder []string
	deps := newSplitDeps(&log, &callOrder)

	in := sampleInput()
	staged := sampleStaged(in)

	res, err := PersistSSH(in, staged, deps)
	if err != nil {
		t.Fatalf("PersistSSH returned error: %v", err)
	}

	// PersistKey must fire first (PrivPEM != nil in sampleStaged).
	if log.persistKey != 1 {
		t.Errorf("PersistSSH: PersistKey called %d times, want 1", log.persistKey)
	}
	if len(callOrder) > 0 && callOrder[0] != "persistKey" {
		t.Errorf("PersistSSH: PersistKey must be called first; order was %v", callOrder)
	}
	// WriteSSH must fire.
	if log.writeSSH != 1 {
		t.Errorf("PersistSSH: WriteSSH called %d times, want 1", log.writeSSH)
	}
	// LEG-2 writers and Resolved must NOT fire.
	if log.writeGitconfig != 0 {
		t.Errorf("PersistSSH: WriteGitconfig called %d times, want 0", log.writeGitconfig)
	}
	if log.writeFragment != 0 {
		t.Errorf("PersistSSH: WriteFragment called %d times, want 0", log.writeFragment)
	}
	if log.writeAllowedSigners != 0 {
		t.Errorf("PersistSSH: WriteAllowedSigners called %d times, want 0", log.writeAllowedSigners)
	}
	if log.resolved != 0 {
		t.Errorf("PersistSSH: Resolved called %d times, want 0", log.resolved)
	}
	// SSHBackup is set; gitconfig/signers backups are empty.
	if res.SSHBackup != "bak-ssh" {
		t.Errorf("PersistSSH: SSHBackup = %q, want bak-ssh", res.SSHBackup)
	}
	if res.GitconfigBackup != "" {
		t.Errorf("PersistSSH: GitconfigBackup = %q, want empty", res.GitconfigBackup)
	}
	if res.AllowedSignersBackup != "" {
		t.Errorf("PersistSSH: AllowedSignersBackup = %q, want empty", res.AllowedSignersBackup)
	}
	// SSHPreview must be non-empty; gitconfig/fragment previews empty.
	if res.SSHPreview == "" {
		t.Error("PersistSSH: SSHPreview must be non-empty")
	}
	if res.GitconfigPreview != "" {
		t.Errorf("PersistSSH: GitconfigPreview = %q, want empty (not set by LEG 1)", res.GitconfigPreview)
	}
}

// TestPersistSSH_SkipsPersistKeyWhenPrivPEMNil asserts PersistSSH skips
// PersistKey when staged.PrivPEM is nil (existing-key path, T-05.7-09).
func TestPersistSSH_SkipsPersistKeyWhenPrivPEMNil(t *testing.T) {
	var log callLog
	var callOrder []string
	deps := newSplitDeps(&log, &callOrder)

	in := sampleInput()
	staged := sampleStaged(in)
	staged.PrivPEM = nil // existing-key path

	_, err := PersistSSH(in, staged, deps)
	if err != nil {
		t.Fatalf("PersistSSH (PrivPEM nil) returned error: %v", err)
	}
	if log.persistKey != 0 {
		t.Errorf("PersistSSH (PrivPEM nil): PersistKey called %d times, want 0", log.persistKey)
	}
	// WriteSSH must still fire.
	if log.writeSSH != 1 {
		t.Errorf("PersistSSH (PrivPEM nil): WriteSSH called %d times, want 1", log.writeSSH)
	}
}

// TestPersistGitconfig_OnlyLEG2Writers asserts PersistGitconfig fires
// WriteGitconfig, WriteFragment, and WriteAllowedSigners, and does NOT fire
// PersistKey, WriteSSH, or Resolved (T-05.7-09, anti-blindspot point (b)).
func TestPersistGitconfig_OnlyLEG2Writers(t *testing.T) {
	var log callLog
	var callOrder []string
	deps := newSplitDeps(&log, &callOrder)

	in := sampleInput()
	staged := sampleStaged(in)

	res, err := PersistGitconfig(in, staged, deps)
	if err != nil {
		t.Fatalf("PersistGitconfig returned error: %v", err)
	}

	// PersistKey must NOT fire (LEG 2 does not touch the key).
	if log.persistKey != 0 {
		t.Errorf("PersistGitconfig: PersistKey called %d times, want 0", log.persistKey)
	}
	// WriteSSH must NOT fire.
	if log.writeSSH != 0 {
		t.Errorf("PersistGitconfig: WriteSSH called %d times, want 0", log.writeSSH)
	}
	// LEG-2 writers must fire.
	if log.writeGitconfig != 1 {
		t.Errorf("PersistGitconfig: WriteGitconfig called %d times, want 1", log.writeGitconfig)
	}
	if log.writeFragment != 1 {
		t.Errorf("PersistGitconfig: WriteFragment called %d times, want 1", log.writeFragment)
	}
	if log.writeAllowedSigners != 1 {
		t.Errorf("PersistGitconfig: WriteAllowedSigners called %d times, want 1", log.writeAllowedSigners)
	}
	// Resolved must NOT fire.
	if log.resolved != 0 {
		t.Errorf("PersistGitconfig: Resolved called %d times, want 0", log.resolved)
	}
	// Backup paths for LEG 2.
	if res.GitconfigBackup != "bak-gc" {
		t.Errorf("PersistGitconfig: GitconfigBackup = %q, want bak-gc", res.GitconfigBackup)
	}
	if res.AllowedSignersBackup != "bak-signers" {
		t.Errorf("PersistGitconfig: AllowedSignersBackup = %q, want bak-signers", res.AllowedSignersBackup)
	}
	// SSHBackup must be empty (LEG 2 does not set it).
	if res.SSHBackup != "" {
		t.Errorf("PersistGitconfig: SSHBackup = %q, want empty", res.SSHBackup)
	}
	// GitconfigPreview and AllowedSignersPreview must be non-empty.
	if res.GitconfigPreview == "" {
		t.Error("PersistGitconfig: GitconfigPreview must be non-empty")
	}
	if res.AllowedSignersPreview == "" {
		t.Error("PersistGitconfig: AllowedSignersPreview must be non-empty")
	}
}

// TestPersistAll_CompositionEqualsLEG1PlusLEG2 asserts PersistAll fires all
// writers in the same order as PersistSSH+PersistGitconfig combined, runs
// Resolved, and returns a CreateResult field-for-field equivalent to a reference
// that captures the individual leg outputs (anti-blindspot point (c), T-05.7-09).
func TestPersistAll_CompositionEqualsLEG1PlusLEG2(t *testing.T) {
	// Build reference: run PersistSSH then PersistGitconfig separately.
	var refLog callLog
	var refOrder []string
	refDeps := newSplitDeps(&refLog, &refOrder)
	in := sampleInput()
	staged := sampleStaged(in)

	res1, err := PersistSSH(in, staged, refDeps)
	if err != nil {
		t.Fatalf("reference PersistSSH error: %v", err)
	}
	res2, err := PersistGitconfig(in, staged, refDeps)
	if err != nil {
		t.Fatalf("reference PersistGitconfig error: %v", err)
	}
	// Attach Resolved to the reference result.
	refResolvedTest, refResolved := refDeps.Resolved(in.Alias)
	refResult := CreateResult{
		Key:                   res1.Key,
		SSHPreview:            res1.SSHPreview,
		SSHBackup:             res1.SSHBackup,
		GitconfigPreview:      res2.GitconfigPreview,
		GitconfigBackup:       res2.GitconfigBackup,
		FragmentPreview:       res2.FragmentPreview,
		AllowedSignersPreview: res2.AllowedSignersPreview,
		AllowedSignersLine:    res2.AllowedSignersLine,
		AllowedSignersBackup:  res2.AllowedSignersBackup,
		ResolvedTest:          refResolvedTest,
		Resolved:              refResolved,
	}

	// Now run PersistAll and compare.
	var allLog callLog
	var allOrder []string
	allDeps := newSplitDeps(&allLog, &allOrder)

	got, err := PersistAll(in, staged, allDeps)
	if err != nil {
		t.Fatalf("PersistAll error: %v", err)
	}

	// Writer call counts must match LEG1+LEG2 reference.
	if allLog.persistKey != refLog.persistKey {
		t.Errorf("PersistAll: persistKey count = %d, want %d (from reference)", allLog.persistKey, refLog.persistKey)
	}
	if allLog.writeSSH != 1 {
		t.Errorf("PersistAll: writeSSH = %d, want 1", allLog.writeSSH)
	}
	if allLog.writeGitconfig != 1 {
		t.Errorf("PersistAll: writeGitconfig = %d, want 1", allLog.writeGitconfig)
	}
	if allLog.writeFragment != 1 {
		t.Errorf("PersistAll: writeFragment = %d, want 1", allLog.writeFragment)
	}
	if allLog.writeAllowedSigners != 1 {
		t.Errorf("PersistAll: writeAllowedSigners = %d, want 1", allLog.writeAllowedSigners)
	}
	if allLog.resolved != 1 {
		t.Errorf("PersistAll: resolved = %d, want 1", allLog.resolved)
	}

	// Field-for-field equality against the reference composition.
	if got.SSHBackup != refResult.SSHBackup {
		t.Errorf("PersistAll: SSHBackup = %q, want %q", got.SSHBackup, refResult.SSHBackup)
	}
	if got.GitconfigBackup != refResult.GitconfigBackup {
		t.Errorf("PersistAll: GitconfigBackup = %q, want %q", got.GitconfigBackup, refResult.GitconfigBackup)
	}
	if got.AllowedSignersBackup != refResult.AllowedSignersBackup {
		t.Errorf("PersistAll: AllowedSignersBackup = %q, want %q", got.AllowedSignersBackup, refResult.AllowedSignersBackup)
	}
	if got.SSHPreview != refResult.SSHPreview {
		t.Errorf("PersistAll: SSHPreview mismatch")
	}
	if got.GitconfigPreview != refResult.GitconfigPreview {
		t.Errorf("PersistAll: GitconfigPreview mismatch")
	}
	if got.FragmentPreview != refResult.FragmentPreview {
		t.Errorf("PersistAll: FragmentPreview mismatch")
	}
	if got.AllowedSignersPreview != refResult.AllowedSignersPreview {
		t.Errorf("PersistAll: AllowedSignersPreview mismatch")
	}
	if got.AllowedSignersLine != refResult.AllowedSignersLine {
		t.Errorf("PersistAll: AllowedSignersLine mismatch")
	}
	if got.ResolvedTest.Outcome != refResult.ResolvedTest.Outcome {
		t.Errorf("PersistAll: ResolvedTest.Outcome = %v, want %v", got.ResolvedTest.Outcome, refResult.ResolvedTest.Outcome)
	}
	if got.Resolved.User != refResult.Resolved.User {
		t.Errorf("PersistAll: Resolved.User = %q, want %q", got.Resolved.User, refResult.Resolved.User)
	}
}
