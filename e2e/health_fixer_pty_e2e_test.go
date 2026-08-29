//go:build e2e

package e2e

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// startEphemeralSSHAgent spawns a real, throwaway ssh-agent for the duration
// of one test -- with zero identities seeded, a truly-zero-finding
// health-all-green/nothing-to-fix state requires CheckAgent's own
// "ssh-agent: not reachable" report-only warning to genuinely NOT fire,
// which only happens when a real agent is reachable (this project's own
// SSH_AUTH_SOCK= test convention otherwise makes that warning unavoidable).
// The agent is killed in t.Cleanup; nothing here touches the developer's
// real ssh-agent or real keys.
func startEphemeralSSHAgent(t *testing.T) (sockEnv string) {
	t.Helper()
	out, err := exec.Command("ssh-agent", "-s").Output() //nolint:gosec // fixed args, no shell (G204)
	if err != nil {
		t.Fatalf("starting ephemeral ssh-agent: %v", err)
	}
	sockRe := regexp.MustCompile(`SSH_AUTH_SOCK=([^;]+);`)
	pidRe := regexp.MustCompile(`SSH_AGENT_PID=([0-9]+);`)
	sockMatch := sockRe.FindSubmatch(out)
	pidMatch := pidRe.FindSubmatch(out)
	if sockMatch == nil || pidMatch == nil {
		t.Fatalf("could not parse ssh-agent -s output:\n%s", out)
	}
	sock := string(sockMatch[1])
	pid, err := strconv.Atoi(string(pidMatch[1]))
	if err != nil {
		t.Fatalf("parsing ssh-agent pid: %v", err)
	}
	t.Cleanup(func() {
		if proc, err := os.FindProcess(pid); err == nil {
			_ = proc.Kill()
		}
	})
	return "SSH_AUTH_SOCK=" + sock
}

func seedHealthFixerFlagship(t *testing.T, home string) string {
	t.Helper()
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("creating SSH directory: %v", err)
	}
	config := filepath.Join(sshDir, "config")
	writeFileT(t, config, "# hand-written header\nHost clientb.github.com\n\tHostName ssh.github.com\n\tIdentitiesOnly no # deliberately loose\n\tIdentityFile ~/.ssh/id_ed25519_clientb\n\nHost untouched.example.com\n\tHostName example.com\n")
	if err := os.Chmod(config, 0o600); err != nil {
		t.Fatalf("securing SSH config permissions: %v", err)
	}
	return config
}

func seedHealthFixerGreen(t *testing.T, home string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatalf("creating SSH directory: %v", err)
	}
	sshConfig := filepath.Join(home, ".ssh", "config")
	writeFileT(t, sshConfig, "Host *\n\tIdentitiesOnly yes\n")
	if err := os.Chmod(sshConfig, 0o600); err != nil {
		t.Fatalf("securing SSH config permissions: %v", err)
	}
	gitDir := filepath.Join(home, ".gitconfig.d")
	if err := os.MkdirAll(gitDir, 0o700); err != nil {
		t.Fatalf("creating Git config directory: %v", err)
	}
	baseline := filepath.Join(gitDir, "00-baseline")
	writeFileT(t, baseline, "# BEGIN gitid managed: baseline\n[core]\n\tignorecase = false\n\texcludesfile = "+filepath.Join(home, ".gitignore_global")+"\n# END gitid managed: baseline\n")
	writeFileT(t, filepath.Join(home, ".gitconfig"), "# BEGIN gitid managed: baseline-include\n[include]\n\tpath = "+baseline+"\n# END gitid managed: baseline-include\n")
	// Every gitconfig.DefaultGitignorePatterns() entry must be present or
	// CheckBaseline's Check 4 (curated entries) reports a warning -- a truly
	// clean all-green fixture needs the complete curated set, not a subset.
	patterns := strings.Join([]string{
		".DS_Store", "Thumbs.db", "*.log", "*.bak", "*.tmp", "*.swp", "*.swo",
		".idea/", ".vscode/", "node_modules/", "__pycache__/", "*.pyc", ".env",
	}, "\n")
	writeFileT(t, filepath.Join(home, ".gitignore_global"), "# BEGIN gitid managed: gitignore\n"+patterns+"\n# END gitid managed: gitignore\n")
}

func startHealthFixerPTY(t *testing.T, home string) *ptySession {
	t.Helper()
	return startHealthFixerPTYWithEnv(t, home)
}

func startHealthFixerPTYWithEnv(t *testing.T, home string, extraEnv ...string) *ptySession {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	s := startPTYAt(t, newPTYCmd(t, ctx, BuildBinary(t), home, extraEnv...), dummyTermWidth, dummyTermHeight)
	t.Cleanup(func() { s.close(t) })
	uiReady(t, s)
	return s
}

func openHealth(t *testing.T, s *ptySession) {
	t.Helper()
	s.sendKey([]byte("4"), keystrokeDelay)
	mustSee(t, s, "read-only diagnostics", "key 4 opens Health after its real scan")
}

func openFixer(t *testing.T, s *ptySession) {
	t.Helper()
	s.sendKey([]byte("5"), keystrokeDelay)
	mustSee(t, s, "every fix is previewed", "key 5 opens Fixer after its real scan")
}

func typeFixerConfirm(s *ptySession) {
	for _, r := range "clientb.github.com" {
		s.sendKey([]byte(string(r)), keystrokeDelay)
	}
	s.sendKey(dummyKeyEnter, keystrokeDelay)
}

func TestHealthFixer_RealPTYHealthFindingsInlineDetailAndIdentity(t *testing.T) {
	home := SandboxHome(t)
	seedHealthFixerFlagship(t, home)
	s := startHealthFixerPTY(t, home)
	openHealth(t, s)
	mustSee(t, s, "SSH", "Health keeps SSH findings in their own section")
	mustSee(t, s, "Git", "Health keeps Git findings in their own section")
	mustSee(t, s, "IdentitiesOnly no contradicts", "Health renders the real contradiction finding")
	mustSee(t, s, "Suggested fix:", "Health renders the selected finding inline detail")
	mustSee(t, s, "Switch to Fixer to apply this.", "Health remains read-only")
	s.sendKey(dummyKeyDown, keystrokeDelay)
	mustSee(t, s, "[", "Health navigation preserves an inline finding detail pane")
}

func TestHealthFixer_RealPTYHealthAllGreen(t *testing.T) {
	home := SandboxHome(t)
	seedHealthFixerGreen(t, home)
	sockEnv := startEphemeralSSHAgent(t)
	s := startHealthFixerPTYWithEnv(t, home, sockEnv)
	openHealth(t, s)
	mustSee(t, s, "SSH -- 0 fixable problems", "all-green Health reports SSH clean")
	mustSee(t, s, "Git -- 0 fixable problems", "all-green Health reports Git clean")
}

func TestHealthFixer_RealPTYPerIdentityHealth(t *testing.T) {
	home := SandboxHome(t)
	seedMinimalIdentity(t, home, "legacy")
	if err := os.Remove(filepath.Join(home, ".gitconfig.d", "legacy")); err != nil {
		t.Fatalf("removing legacy fragment: %v", err)
	}
	s := startHealthFixerPTY(t, home)
	openHealth(t, s)
	mustSee(t, s, "legacy", "global Health finding identifies the affected identity")
}

func TestHealthFixer_RealPTYParseError(t *testing.T) {
	home := SandboxHome(t)
	if err := os.MkdirAll(filepath.Join(home, ".gitconfig.d"), 0o700); err != nil {
		t.Fatalf("creating Git config directory: %v", err)
	}
	writeFileT(t, filepath.Join(home, ".gitconfig"), "[include]\n\tpath = ~/.gitconfig.d/broken\n")
	broken := filepath.Join(home, ".gitconfig.d", "broken")
	writeFileT(t, broken, "[user\n\temail = broken@example.com\n")
	if err := os.Chmod(broken, 0o600); err != nil {
		t.Fatalf("securing broken fragment permissions: %v", err)
	}
	s := startHealthFixerPTY(t, home)
	s.sendKey([]byte("4"), keystrokeDelay)
	mustSee(t, s, "configuration parse error", "Health renders the Files parse-error frame")
	mustSee(t, s, "Checks paused until this configuration parses again.", "parse error pauses only the affected checks")
	mustSee(t, s, "Raw error:", "parse error surfaces the real parser output")
}

func TestHealthFixer_RealPTYFixerCeremonyWritesAndBacksUp(t *testing.T) {
	home := SandboxHome(t)
	config := seedHealthFixerFlagship(t, home)
	before := readFileE2E(t, config)
	s := startHealthFixerPTY(t, home)
	openFixer(t, s)
	mustSee(t, s, "IdentitiesOnly no contradicts", "Fixer lists the real fixable contradiction")
	s.sendKey([]byte("f"), keystrokeDelay)
	mustSee(t, s, "IdentitiesOnly no # deliberately loose", "ceremony state A previews the real before line")
	mustSee(t, s, "IdentitiesOnly yes # deliberately loose", "ceremony state A previews the real after line")
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Type the Host name \"clientb.github.com\"", "ceremony requires the exact typed host confirmation")
	typeFixerConfirm(s)
	mustSee(t, s, "IdentitiesOnly set to yes on Host clientb.github.com", "ceremony state B reports the real applied result")
	mustSee(t, s, "Backed up →", "result receipt names the real backup")
	// The receipt above is shown OPTIMISTICALLY (fixCeremonyFor's own
	// non-Async done=true-on-confirm contract, see ceremony.go's
	// commitFailed doc comment) -- the real Persist dispatch only happens on
	// this SECOND Enter, acknowledging the receipt.
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	after := readFileE2E(t, config)
	want := strings.Replace(before, "IdentitiesOnly no # deliberately loose", "IdentitiesOnly yes # deliberately loose", 1)
	if after != want {
		t.Fatalf("flagship rewrite changed bytes beyond the one directive:\nwant:\n%s\ngot:\n%s", want, after)
	}
	backups, err := filepath.Glob(config + ".bak.*")
	if err != nil || len(backups) != 1 {
		t.Fatalf("SSH config backups = %v, %v; want exactly one", backups, err)
	}
	if backup := readFileE2E(t, backups[0]); backup != before {
		t.Fatalf("backup bytes differ from pre-fix config:\nwant:\n%s\ngot:\n%s", before, backup)
	}
}

// seedHealthFixerBatch seeds two REAL, independently managed identities
// (reusing this project's own seedTwoIdentitiesSameProviderE2E building
// blocks -- NOT two seedMinimalIdentity calls, which each os.WriteFile the
// whole ~/.ssh/config and ~/.gitconfig and would silently clobber each
// other's managed blocks) whose private keys are world-readable -- two
// independent, non-destructive Permissions/Critical fixable findings
// (chmod 0600) -- so the F batch walk has a real 2-item queue to
// auto-advance through end to end against the real binary.
func seedHealthFixerBatch(t *testing.T, home string) (keyAlpha, keyBravo string) {
	t.Helper()
	first, second := "alpha", "bravo"
	writeStubKeyPair(t, home, first)
	writeStubKeyPair(t, home, second)

	sshConfig := "" +
		"# BEGIN gitid managed: _global\n" +
		"Host *\n" +
		"  IdentitiesOnly yes\n" +
		"# END gitid managed: _global\n\n" +
		sshHostBlock(first) + sshHostBlock(second)
	writeFileT(t, filepath.Join(home, ".ssh", "config"), sshConfig)

	gitconfig := "[user]\n  name = Global User\n\n" +
		gitconfigIncludeIfBlock(first) + gitconfigIncludeIfBlock(second)
	writeFileT(t, filepath.Join(home, ".gitconfig"), gitconfig)

	if err := os.MkdirAll(filepath.Join(home, ".gitconfig.d"), 0o700); err != nil {
		t.Fatalf("seedHealthFixerBatch: MkdirAll .gitconfig.d: %v", err)
	}
	writeFileT(t, filepath.Join(home, ".gitconfig.d", first), plainFragment(first))
	writeFileT(t, filepath.Join(home, ".gitconfig.d", second), plainFragment(second))

	keyAlpha = filepath.Join(home, ".ssh", "id_ed25519_"+first)
	keyBravo = filepath.Join(home, ".ssh", "id_ed25519_"+second)
	if err := os.Chmod(keyAlpha, 0o644); err != nil {
		t.Fatalf("loosening alpha key permissions: %v", err)
	}
	if err := os.Chmod(keyBravo, 0o644); err != nil {
		t.Fatalf("loosening bravo key permissions: %v", err)
	}
	return keyAlpha, keyBravo
}

// confirmNonDestructiveFix drives one non-Async, non-destructive fix
// ceremony to completion: the first Enter reaches the receipt state
// (fixCeremonyFor's own done=true-on-Enter-#1 contract, see ceremony.go's
// commitFailed doc comment), the second Enter acknowledges the receipt and
// dispatches the real Persist call -- the batch walk then auto-advances.
func confirmNonDestructiveFix(s *ptySession) {
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	s.sendKey(dummyKeyEnter, keystrokeDelay)
}

// TestHealthFixer_RealPTYFixerBatchWalk proves the F batch walk auto-chains
// real fix ceremonies against the real binary: it deliberately does NOT
// assert the fixture reaches a global "nothing to fix" state, because
// seedMinimalIdentity's two identities carry their own unrelated fixable
// findings (baseline wiring, allowed_signers, agent-not-loaded) that this
// test's two Permissions/Critical findings (the highest severity in the
// queue, so guaranteed to walk first) do not touch -- the real permission
// byte check below is the authoritative proof both real fixes applied.
func TestHealthFixer_RealPTYFixerBatchWalk(t *testing.T) {
	home := SandboxHome(t)
	keyAlpha, keyBravo := seedHealthFixerBatch(t, home)
	s := startHealthFixerPTY(t, home)
	openFixer(t, s)
	s.sendKey([]byte("F"), keystrokeDelay)
	mustSee(t, s, "Fix all", "F starts the real batch walk over the fixable queue")
	confirmNonDestructiveFix(s)
	mustSee(t, s, "1 / ", "batch progress line advances after the real first fix applies")
	confirmNonDestructiveFix(s)
	mustSee(t, s, "2 / ", "batch progress line advances after the real second fix applies")

	for _, key := range []string{keyAlpha, keyBravo} {
		info, err := os.Stat(key)
		if err != nil {
			t.Fatalf("stat %s: %v", key, err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Errorf("%s permissions = %04o, want 0600 (batch walk must have really applied both fixes)", key, got)
		}
	}
}

// makeImmutable applies macOS/BSD's chflags uchg (the user-immutable flag)
// to path, which blocks chmod(2) with EPERM even for the file's owner --
// a real, deterministic, OS-level failure for a Permissions Fix.Fn
// (deps.FixPerm wraps os.Chmod, cmd/gitid/wiring.go), reused here to force
// a genuine D-16 mid-batch Persist failure through the real binary rather
// than a stubbed one (08-08-VERIFICATION.md's DLV-06 gap: the halt-on-
// failure path was previously proven only at the unit level,
// TestBatchWalkHalt, never through a real compiled-binary PTY session).
// t.Cleanup lifts the flag so t.TempDir()'s own removal doesn't fail.
func makeImmutable(t *testing.T, path string) {
	t.Helper()
	if err := exec.Command("chflags", "uchg", path).Run(); err != nil { //nolint:gosec // fixed args, test-only, no shell (G204)
		t.Fatalf("chflags uchg %s: %v", path, err)
	}
	t.Cleanup(func() {
		_ = exec.Command("chflags", "nouchg", path).Run() //nolint:gosec // fixed args, test-only, no shell (G204)
	})
}

// TestHealthFixer_RealPTYFixerBatchWalkHalt proves the D-16 batch-walk
// halt-on-failure path against the REAL compiled binary: fix-1 (alpha's
// key) succeeds normally; fix-2 (bravo's key, made immutable via chflags
// uchg so its real chmod(2) genuinely fails with EPERM) halts the batch,
// naming the failure and the already-applied count, and the queue is
// cleared -- proven by returning to the fixer list (not silently paused)
// and by bravo's permission bits being UNCHANGED (the failed chmod never
// took effect).
func TestHealthFixer_RealPTYFixerBatchWalkHalt(t *testing.T) {
	home := SandboxHome(t)
	keyAlpha, keyBravo := seedHealthFixerBatch(t, home)
	makeImmutable(t, keyBravo)
	s := startHealthFixerPTY(t, home)
	openFixer(t, s)
	s.sendKey([]byte("F"), keystrokeDelay)
	mustSee(t, s, "Fix all", "F starts the real batch walk over the fixable queue")
	confirmNonDestructiveFix(s)
	mustSee(t, s, "1 / ", "batch progress line advances after the real first fix applies")
	confirmNonDestructiveFix(s)
	mustSee(t, s, "Fix 2 of", "the real chmod failure halts the batch with the D-16 message")
	mustSee(t, s, "failed and was rolled back", "the real chmod failure halts the batch with the D-16 message")
	mustSee(t, s, "Nothing else in this batch was attempted", "the halt message states the queue stopped")
	mustSee(t, s, "operation not permitted", "the ceremony's own failure state names the real OS error")

	alphaInfo, err := os.Stat(keyAlpha)
	if err != nil {
		t.Fatalf("stat alpha key: %v", err)
	}
	if got := alphaInfo.Mode().Perm(); got != 0o600 {
		t.Errorf("alpha key permissions = %04o, want 0600 (fix-1 must have really applied and stand)", got)
	}
	bravoInfo, err := os.Stat(keyBravo)
	if err != nil {
		t.Fatalf("stat bravo key: %v", err)
	}
	if got := bravoInfo.Mode().Perm(); got != 0o644 {
		t.Errorf("bravo key permissions = %04o, want 0644 unchanged (the failed chmod must never have taken effect)", got)
	}
}

func TestHealthFixer_RealPTYFixerNothingToFix(t *testing.T) {
	home := SandboxHome(t)
	seedHealthFixerGreen(t, home)
	sockEnv := startEphemeralSSHAgent(t)
	s := startHealthFixerPTYWithEnv(t, home, sockEnv)
	openFixer(t, s)
	mustSee(t, s, "SSH -- 0 fixable problems", "nothing-to-fix reports SSH clean")
	mustSee(t, s, "Git -- 0 fixable problems", "nothing-to-fix reports Git clean")
}
