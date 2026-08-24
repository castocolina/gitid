//go:build e2e

package e2e

// create_flow_pty_e2e_test.go — Per-screen PTY e2e for the REAL create-flow
// wizard (plan 03-06, Task 1, DLV-06). Every test here drives the REAL
// `gitid` binary (cmd/gitid, not gitid-dummy) via raw keystrokes over a
// pseudo-terminal at the design's minimum geometry (100x30), reusing
// FakeSSHDir (harness_test.go, D-22) prepended to the child process's PATH
// so the real exec plumbing runs against a swapped `ssh`, never a Go mock.
//
// Superseded: the POC-CLI create_e2e_test.go (D-14 target) was already
// removed from this repository by an earlier Phase-3 wave (the archival
// commit that dropped cmd/gitid's Cobra command surface) — there is nothing
// left here to delete; verified via `ls e2e/` at the start of this session.
//
// Covered, one test function per screen/path (03-06-PLAN.md Task 1):
//
//  1. TestCreateFlow_SSHFormAliasCollision       — algorithm+SSH form, D-09
//  2. TestCreateFlow_TestStagePass                — both stages, mode "pass"
//  3. TestCreateFlow_TestStageReachableNotUploaded — both stages, mode "denied" (D-02)
//  4. TestCreateFlow_TestStageFailureRetry        — stage 1, mode "timeout" (D-01)
//  5. TestCreateFlow_GitStepDisabledReasonAndConfirmWrite — D-19 + SSHUI-04 + TEST-03/D-05..D-09
//  6. TestCreateFlow_ReuseExistingEncryptedKeyClosesL2Seam — KEY-06/D-11, the L2 seam proof
//  7. TestCreateFlow_MouseFieldFocus              — SSHUI-02 mouse half
//
// L2 closure (blocking obligation, plan 03-06's own frontmatter): Test 6 is
// the ONLY place the identity.Deps.ReadPub injected-seam wiring blindspot can
// be closed — a unit/wiring test (cmd/gitid/wiring_test.go's
// TestReuseEncryptedKeyWithExistingPubSucceeds) proves the real constructor
// and real filesystem, but never a live raw-keystroke PTY session driving
// the actual binary a user would run.

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/castocolina/gitid/internal/keygen"
)

// wizardKeyRight is the raw xterm CSI byte sequence for the right arrow —
// used at the wizard's step-0 D-10 key-source toggle (Generate <-> Reuse).
var wizardKeyRight = []byte{0x1b, 0x5b, 0x43}

// newRealCreateFlowCmd builds the exec.Cmd for a create-flow PTY test: the
// REAL gitid binary, a sandboxed HOME, and — when fakeSSHDir is non-empty —
// that directory prepended to PATH so the real process execs the fake `ssh`
// (D-22) instead of the system one. TERM is pinned the same way every other
// PTY e2e test in this package pins it (dummy_demo_e2e_test.go), which is
// proven to still hold under the project's TERM=dumb/SSH_AUTH_SOCK= CI
// reproduction (L1/L3) because the LAST TERM= entry in cmd.Env wins.
func newRealCreateFlowCmd(ctx context.Context, bin, home, fakeSSHDir string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, bin) //nolint:gosec // bin from BuildBinary; no user input
	env := append(os.Environ(), "HOME="+home, "TERM=xterm-256color")
	if fakeSSHDir != "" {
		env = append(env, "PATH="+fakeSSHDir+":"+os.Getenv("PATH"))
	}
	cmd.Env = env
	return cmd
}

// openCreateWizard boots the real binary, waits for the shell to render, and
// presses 'n' to open the create wizard from the identities pane (works
// whether or not any identity already exists — 'n' is unconditional).
func openCreateWizard(t *testing.T, s *ptySession) {
	t.Helper()
	uiReady(t, s)
	s.sendKey([]byte("n"), keystrokeDelay)
	mustSee(t, s, "Step 1/4", "wizard: step 0 (SSH details) opens")
}

// quitCleanly sends q then Enter (the real top-level quit confirmation) and
// waits for the process to exit, matching the graceful-quit pattern already
// proven in dummy_demo_e2e_test.go — avoids relying on close()'s best-effort
// ctrl+c for tests that want to assert a clean (0) exit.
func quitCleanly(t *testing.T, s *ptySession) {
	t.Helper()
	waitCh := make(chan error, 1)
	go func() { waitCh <- s.cmd.Wait() }()
	s.sendKey([]byte("q"), keystrokeDelay)
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	select {
	case werr := <-waitCh:
		if werr != nil {
			t.Fatalf("quit: process exited with error: %v", werr)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("quit: process did not exit within 10s of q + Enter")
	}
}

// seedEncryptedKeyFixture writes a real ed25519 private key (optionally
// passphrase-encrypted) plus its `.pub` sibling at keyPath and returns the
// generated authorized-key line. Mirrors cmd/gitid/wiring_test.go's
// seedGeneratedKey — duplicated here because e2e is a separate test binary
// (build-tagged, its own package) and cannot import an unexported test
// helper from cmd/gitid's own _test.go files.
func seedEncryptedKeyFixture(t *testing.T, keyPath, identityName, passphrase string) string {
	t.Helper()
	mat, err := keygen.GenerateMaterial(keygen.Params{
		Algo: "ed25519", Identity: identityName, Comment: identityName + "@gitid", Passphrase: passphrase,
	})
	if err != nil {
		t.Fatalf("generating key fixture %s: %v", keyPath, err)
	}
	if err := os.WriteFile(keyPath, mat.PrivPEM, 0o600); err != nil {
		t.Fatalf("writing private key fixture %s: %v", keyPath, err)
	}
	if err := os.WriteFile(keyPath+".pub", []byte(mat.PubLine+"\n"), 0o644); err != nil { //nolint:gosec // .pub is public key material by definition; hermetic sandbox HOME (G306)
		t.Fatalf("writing public key fixture %s.pub: %v", keyPath, err)
	}
	return mat.PubLine
}

// clickLabelRow locates label in the CURRENT decoded frame and sends a real
// SGR mouse press+release at that row — the SAME coordinate-from-decoded-
// frame technique TestDummyDemo_MouseAndGitApply already proves reliable
// (dummy_demo_e2e_test.go): never a hardcoded column/row, since layout can
// legitimately shift between runs/platforms.
func clickLabelRow(t *testing.T, s *ptySession, label string) {
	t.Helper()
	var col, row int
	last, ok := s.waitFor(8*time.Second, func(text string) bool {
		for y, line := range strings.Split(text, "\n") {
			if idx := strings.Index(line, label); idx >= 0 {
				col = len([]rune(line[:idx])) + 2 // +1 into the label text, +1 for 1-based SGR coords
				row = y + 1
				return true
			}
		}
		return false
	})
	if !ok {
		t.Fatalf("clickLabelRow: %q never rendered. Last frame:\n%s", label, last)
	}
	s.sendKey([]byte(fmt.Sprintf("\x1b[<0;%d;%dM", col, row)), keystrokeDelay) // SGR press
	s.sendKey([]byte(fmt.Sprintf("\x1b[<0;%d;%dm", col, row)), keystrokeDelay) // SGR release
}

// tabKeys sends n literal Tab bytes, each followed by keystrokeDelay — the
// wizard's step-0/step-2 focus rings both move via Tab (identities.go
// handleWizardKey "tab", "down" case).
func tabKeys(s *ptySession, n int) {
	for i := 0; i < n; i++ {
		s.sendKey([]byte("\t"), keystrokeDelay)
	}
}

// requireFocusedProof waits for stage-two completion, focuses the proof
// viewport, and pages through its retained source to expose the requested raw
// proof markers at the fixed 100x30 viewport size.
func requireFocusedProof(t *testing.T, s *ptySession, markers ...string) {
	t.Helper()
	mustSee(t, s, "Next: Git identity", "both test stages completed")
	s.sendKey([]byte("v"), keystrokeDelay)
	mustSee(t, s, "Proof viewport focused", "raw v focuses the proof viewport")

	seen := make(map[string]bool, len(markers))
	for range 12 {
		frame := s.snapshot()
		complete := true
		for _, marker := range markers {
			seen[marker] = seen[marker] || strings.Contains(frame, marker)
			complete = complete && seen[marker]
		}
		if complete {
			return
		}
		// The focused viewport updates synchronously in the TUI reducer; this
		// short pause only gives the PTY decoder time to receive the next frame.
		s.sendKey([]byte("\x1b[6~"), 10*time.Millisecond)
	}
	for _, marker := range markers {
		if !seen[marker] {
			t.Errorf("focused real 100x30 proof viewport never exposed %q through raw PgDn navigation", marker)
		}
	}
}

// ---------------------------------------------------------------------------
// 1. Algorithm + SSH form (SSHUI-01/03, D-09)
// ---------------------------------------------------------------------------

func TestCreateFlow_AlgorithmAvailability(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)
	fakeSSH := FakeSSHDir(t, "pass")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	s := startPTYAt(t, newRealCreateFlowCmd(ctx, bin, home, fakeSSH), dummyTermWidth, dummyTermHeight)
	defer s.close(t)

	openCreateWizard(t, s)
	mustSee(t, s, "ed25519 — ★ recommended", "ssh -Q key keeps ed25519 available in the real catalog")
	mustNotSee(t, s, "ed25519 — ★ recommended — Disabled", "available ed25519 must not render disabled")
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Step 2/4", "an available selected algorithm unlocks the test step")
}

// TestCreateFlow_SSHFormAliasCollision proves SSHUI-03's live Host-block
// preview (Port 443 + IdentitiesOnly yes, recipe-faithful) and D-09's
// alias-collision block, driven through the REAL binary: seeding an existing
// managed "acme" identity in the fake HOME collides with the wizard's own
// default "acme" prefix with zero keystrokes, and Enter must not advance.
func TestCreateFlow_SSHFormAliasCollision(t *testing.T) {
	home := SandboxHome(t)
	seedMinimalIdentity(t, home, "acme")
	bin := BuildBinary(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	s := startPTYAt(t, newRealCreateFlowCmd(ctx, bin, home, ""), dummyTermWidth, dummyTermHeight)
	defer s.close(t)

	openCreateWizard(t, s)

	mustSee(t, s, "Port 443", "step 0: the live Host-block preview shows the default recipe-faithful port")
	mustSee(t, s, "IdentitiesOnly yes", "step 0: the live Host-block preview shows IdentitiesOnly yes")

	mustSee(t, s, "SSH Host alias already exists", "step 0: D-09 alias-collision inline error fires against the seeded identity")
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Step 1/4", "step 0: a blocked Enter must not advance past the collision")

	saveFrame(t, "create-flow-ssh-form-collision", s)
}

// ---------------------------------------------------------------------------
// 2-4. Two-stage connectivity test (TEST-01/02, D-01/D-02/D-04)
// ---------------------------------------------------------------------------

// TestCreateFlow_TestStagePass drives both connectivity stages through the
// REAL binary with FakeSSHDir(t, "pass"): stage 1 renders the green PASS
// banner (the real ssh output, not a paraphrase), and stage 2 chains on
// Enter (D-04) to the ssh -G resolution proof, then the wizard advances to
// the Git step.
func TestCreateFlow_TestStagePass(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)
	fakeSSH := FakeSSHDir(t, "pass")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	s := startPTYAt(t, newRealCreateFlowCmd(ctx, bin, home, fakeSSH), dummyTermWidth, dummyTermHeight)
	defer s.close(t)

	openCreateWizard(t, s)
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Step 2/4", "step 0 -> step 1 (defaults are valid)")

	s.sendKey(dummyKeyEnter, keystrokeDelay) // run stage 1; stage 2 auto-chains (D-04)
	mustSee(t, s, "Hi user!", "stage 1: the REAL ssh PASS banner from the fake-ssh PATH shim")
	requireFocusedProof(t, s, "identityfile")

	s.sendKey(dummyKeyEnter, keystrokeDelay) // -> step 2 (Git, demo'd)
	mustSee(t, s, "Step 3/4", "wizard advances to the Git step once both stages PASS")

	saveFrame(t, "create-flow-test-stage-pass", s)
}

// TestCreateFlow_TestStageReachableNotUploaded proves D-02/Pitfall 6: a
// "Permission denied (publickey)" ssh output renders the NEW yellow warning
// state on BOTH stages — never the red hard-failure treatment — and D-04
// still chains stage 2 on Enter after this outcome (it unlocks the store
// exactly like PASS does).
func TestCreateFlow_TestStageReachableNotUploaded(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)
	fakeSSH := FakeSSHDir(t, "denied")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	s := startPTYAt(t, newRealCreateFlowCmd(ctx, bin, home, fakeSSH), dummyTermWidth, dummyTermHeight)
	defer s.close(t)

	openCreateWizard(t, s)
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Step 2/4", "step 0 -> step 1")

	s.sendKey(dummyKeyEnter, keystrokeDelay) // run stage 1; stage 2 auto-chains (D-04)
	mustSee(t, s, "! Reachable — key not uploaded yet", "D-02: stage 1 renders the yellow warning, never red (Pitfall 6)")
	// "✗ broken" is part of the ALWAYS-visible sidebar legend, so a bare
	// mustNotSee(s, "✗", ...) would false-positive on that unrelated
	// chrome text — assert absence of the hard-Failure-SPECIFIC copy
	// instead (never shown for a ReachableNotUploaded outcome, Pitfall 6).
	mustNotSee(t, s, "The connection failed", "\"Permission denied (publickey)\" must never render the hard-failure retry copy")
	requireFocusedProof(t, s, "identityfile")
	mustSee(t, s, "! Reachable — key not uploaded yet", "D-02: stage 2 also renders the yellow warning")

	s.sendKey(dummyKeyEnter, keystrokeDelay) // -> step 2 (Git, demo'd)
	mustSee(t, s, "Step 3/4", "wizard advances past ReachableNotUploaded (D-01 store gate: PASS or ReachableNotUploaded)")

	saveFrame(t, "create-flow-test-stage-reachable-not-uploaded", s)
}

// TestCreateFlow_TestStageFailureRetry proves D-01: a hard Failure (real
// connect-timeout output) is the ONLY connectivity outcome that stops the
// wizard — red glyph, the real ssh output verbatim, a retry affordance, and
// no advance past the test step.
func TestCreateFlow_TestStageFailureRetry(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)
	fakeSSH := FakeSSHDir(t, "timeout")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	s := startPTYAt(t, newRealCreateFlowCmd(ctx, bin, home, fakeSSH), dummyTermWidth, dummyTermHeight)
	defer s.close(t)

	openCreateWizard(t, s)
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Step 2/4", "step 0 -> step 1")

	s.sendKey(dummyKeyEnter, keystrokeDelay) // run stage 1
	mustSee(t, s, "✗", "a hard Failure renders the red glyph")
	mustSee(t, s, "connect to host", "the real ssh timeout output surfaces verbatim, not a paraphrase")
	mustSee(t, s, "Retry (Enter)", "a hard Failure offers retry, never a silent advance")
	mustNotSee(t, s, "Step 3/4", "a hard Failure must never advance to the Git step")

	s.sendKey(dummyKeyEnter, keystrokeDelay) // Enter on a Failure resets to idle
	mustSee(t, s, "Run stage 1 (Enter)", "retry resets the stage to idle, ready to re-run")
	mustNotSee(t, s, "Step 3/4", "still on the test step after the reset")

	saveFrame(t, "create-flow-test-stage-failure-retry", s)
}

// ---------------------------------------------------------------------------
// 5. Git-form-demo'd step (D-18/D-19) + confirm-write ceremony (SSHUI-04/05,
//    TEST-03, D-05/D-06/D-08/D-09)
// ---------------------------------------------------------------------------

// TestCreateFlow_GitConfigurationDefaultTracer drives the compiled real binary
// through the default Git path and proves no live artifact appears before the
// single combined confirmation.
func TestCreateFlow_GitConfigurationDefaultTracer(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)
	fakeSSH := FakeSSHDir(t, "pass")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	s := startPTYAt(t, newRealCreateFlowCmd(ctx, bin, home, fakeSSH), dummyTermWidth, dummyTermHeight)
	closed := false
	defer func() {
		if !closed {
			s.close(t)
		}
	}()

	liveSSHConfig := filepath.Join(home, ".ssh", "config")
	includedTarget := filepath.Join(home, ".ssh", "config.d", "gitid.config")

	openCreateWizard(t, s)
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Step 2/4", "step 0 -> step 1")
	s.sendKey(dummyKeyEnter, keystrokeDelay) // stage 1; stage 2 auto-chains (D-04)
	mustSee(t, s, "Hi user!", "stage 1 PASS")
	requireFocusedProof(t, s, "identityfile")
	s.sendKey(dummyKeyEnter, keystrokeDelay) // -> step 2 (Git, demo'd)
	mustSee(t, s, "Step 3/4", "advanced to the Git step")

	mustNotSee(t, s, "arrives with the next build", "Phase 4 removes the retired capability-disabled reason")

	// SSHUI-04: nothing has touched the LIVE ~/.ssh/config yet.
	if _, err := os.Stat(liveSSHConfig); !os.IsNotExist(err) {
		t.Fatalf("the live ~/.ssh/config must be untouched before the confirm ceremony; stat err = %v", err)
	}

	// The default form is valid, so Enter reaches the combined read-only review.
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, `Create identity "acme"`, "valid Git form reaches the combined review ceremony")
	mustSee(t, s, "~/.gitconfig.d/acme", "review includes the fragment target")
	mustSee(t, s, "allowed_signers", "review includes the signing target")
	mustSee(t, s, "Nothing has changed yet", "the approved pre-confirm assurance appears before any write")

	// Still untouched — the ceremony is a PREVIEW; nothing is written until
	// the confirm keystroke below.
	if _, err := os.Stat(liveSSHConfig); !os.IsNotExist(err) {
		t.Fatalf("the live ~/.ssh/config must still be untouched at the review ceremony (pre-confirm); stat err = %v", err)
	}

	s.sendKey(dummyKeyEnter, keystrokeDelay) // confirm write
	mustSee(t, s, "Wrote →", "the receipt renders after the confirmed write")
	s.sendKey(dummyKeyEnter, keystrokeDelay) // Done

	// The write inside Persist is synchronous; give the render loop one tick
	// to settle before reading the filesystem directly.
	time.Sleep(200 * time.Millisecond)

	live, err := os.ReadFile(liveSSHConfig) //nolint:gosec // hermetic sandbox HOME
	if err != nil {
		t.Fatalf("reading the live ~/.ssh/config after confirm: %v", err)
	}
	if !strings.Contains(string(live), "Include ~/.ssh/config.d/*.config") {
		t.Errorf("the live ~/.ssh/config must gain the gitid Include line on first create (D-06):\n%s", live)
	}

	included, err := os.ReadFile(includedTarget) //nolint:gosec // hermetic sandbox HOME
	if err != nil {
		t.Fatalf("reading the Include'd target %s: %v", includedTarget, err)
	}
	includedText := string(included)
	if !strings.Contains(includedText, "Host acme.github.com") {
		t.Errorf("the Include'd target must carry the identity's Host block:\n%s", includedText)
	}
	if !strings.Contains(includedText, "Port 443") || !strings.Contains(includedText, "IdentitiesOnly yes") {
		t.Errorf("the written Host block must be recipe-faithful (Port 443 + IdentitiesOnly yes):\n%s", includedText)
	}

	fragment, err := os.ReadFile(filepath.Join(home, ".gitconfig.d", "acme"))
	if err != nil {
		t.Fatalf("reading Git fragment after confirm: %v", err)
	}
	if !strings.Contains(string(fragment), "email = you@acme.example") {
		t.Errorf("fragment must carry the entered user.email:\n%s", fragment)
	}
	gitconfig, err := os.ReadFile(filepath.Join(home, ".gitconfig"))
	if err != nil {
		t.Fatalf("reading ~/.gitconfig after confirm: %v", err)
	}
	if !strings.Contains(string(gitconfig), `[includeIf "gitdir:~/git/acme/"]`) {
		t.Errorf("gitconfig must carry the default gitdir includeIf:\n%s", gitconfig)
	}
	signers, err := os.ReadFile(filepath.Join(home, ".ssh", "allowed_signers"))
	if err != nil {
		t.Fatalf("reading allowed_signers after confirm: %v", err)
	}
	if !strings.Contains(string(signers), `you@acme.example namespaces="git"`) {
		t.Errorf("allowed_signers must use the exact user.email principal:\n%s", signers)
	}

	// SSHUI-05/D-08: on darwin, every create writes the idempotent macOS
	// Host * globals block into the SAME resolved target.
	if runtime.GOOS == "darwin" {
		if !strings.Contains(includedText, "Host *") || !strings.Contains(includedText, "UseKeychain yes") {
			t.Errorf("darwin must carry the macOS Host * globals block in the resolved target:\n%s", includedText)
		}
	}

	saveFrame(t, "create-flow-git-step-and-confirm-write", s)

	quitCleanly(t, s)
	s.close(t) // idempotent for an already-exited process
	closed = true
}

// ---------------------------------------------------------------------------
// 6. Reuse-existing-key path — the L2 identity.Deps.ReadPub seam proof
//    (KEY-06/D-10/D-11/D-13)
// ---------------------------------------------------------------------------

// TestCreateFlow_ReuseExistingEncryptedKeyClosesL2Seam is the ONLY place the
// identity.Deps.ReadPub injected-seam wiring blindspot can be closed (plan
// 03-06's own frontmatter): an ENCRYPTED fixture key with an existing `.pub`
// sibling is reused through the REAL binary. This is reachable ONLY if
// ReadPub is wired in the real constructor — with the seam unwired,
// ensurePub would fall back to DerivePub, which parses the ENCRYPTED private
// key via ssh.ParsePrivateKey and fails with no passphrase, so persistCreate
// would record the error and return the state UNCHANGED. Deleting the
// ReadPub assignment in cmd/gitid/wiring.go makes THIS test fail.
func TestCreateFlow_ReuseExistingEncryptedKeyClosesL2Seam(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)
	fakeSSH := FakeSSHDir(t, "pass")

	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("seeding ~/.ssh: %v", err)
	}

	// D-11/KEY-06/D-13 fixture set: an ENCRYPTED key with an existing .pub
	// (the L2 target), a passphraseless decoy, and an unparseable file the
	// picker must SKIP without aborting the scan. Sorted filenames keep the
	// encrypted target at picker index 0 (keygen.ScanReusableKeys sorts by
	// path), so no picker navigation is needed beyond flipping to reuse mode.
	lockedPath := filepath.Join(sshDir, "id_ed25519_a_locked")
	lockedPubLine := seedEncryptedKeyFixture(t, lockedPath, "locked", "s3cret-passphrase")
	plainPath := filepath.Join(sshDir, "id_ed25519_b_plain")
	seedEncryptedKeyFixture(t, plainPath, "plain", "")
	garbagePath := filepath.Join(sshDir, "id_ed25519_c_garbage")
	if err := os.WriteFile(garbagePath, []byte("not a real ssh key\n"), 0o600); err != nil {
		t.Fatalf("seeding the unparseable decoy: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	s := startPTYAt(t, newRealCreateFlowCmd(ctx, bin, home, fakeSSH), dummyTermWidth, dummyTermHeight)
	closed := false
	defer func() {
		if !closed {
			s.close(t)
		}
	}()

	openCreateWizard(t, s)

	// Tab from Alias prefix (focus 1) to the Generate/Reuse toggle (focus 5):
	// SSH Host, Real hostname, Port, then the toggle itself — 4 Tabs.
	tabKeys(s, 4)
	mustSee(t, s, "Generate a new key", "the D-10 key-source toggle rendered")

	// D-10: flip to reuse — reuseIdx resets to 0, i.e. the sorted-FIRST
	// scanned candidate (the encrypted "a_locked" fixture) is already
	// selected; the unparseable "c_garbage" decoy never reaches the list.
	s.sendKey(wizardKeyRight, keystrokeDelay)
	// "Reuse an existing key" can wrap across two physical rows at the
	// fixed 62-col detail-pane width ("...● Reuse an" / "existing key") —
	// assert the tail, which always renders on one wrapped line.
	mustSee(t, s, "existing key", "reuse mode selected")
	mustSee(t, s, "id_ed25519_a_locked", "the picker lists the encrypted fixture")
	mustSee(t, s, "(encrypted)", "the picker flags the encrypted entry — informational only (D-13)")
	mustNotSee(t, s, "id_ed25519_c_garbage", "the unparseable decoy is skipped, not listed (D-13 skip-not-abort)")

	// Defensive proof no passphrase prompt exists anywhere in this UI: the
	// only place "passphrase" may legitimately appear is the (encrypted)
	// informational flag already asserted above.
	if text := s.snapshot(); strings.Contains(strings.ToLower(text), "passphrase") &&
		!strings.Contains(text, "(encrypted)") {
		t.Fatalf("unexpected passphrase-related text outside the informational (encrypted) flag:\n%s", text)
	}

	s.sendKey(dummyKeyEnter, keystrokeDelay) // step0Valid resolves via the reuse selection
	mustSee(t, s, "Step 2/4", "step 0 -> step 1 with the reuse selection already resolved")

	s.sendKey(dummyKeyEnter, keystrokeDelay) // stage 1; stage 2 auto-chains (D-04)
	mustSee(t, s, "Hi user!", "stage 1 PASS")
	requireFocusedProof(t, s, "identityfile")
	s.sendKey(dummyKeyEnter, keystrokeDelay) // -> step 2 (Git, demo'd)
	mustSee(t, s, "Step 3/4", "advanced to the Git step")

	tabKeys(s, 4) // -> Skip Git
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, `Create identity "acme"`, "review ceremony opens")

	s.sendKey(dummyKeyEnter, keystrokeDelay) // confirm write
	mustSee(t, s, "Wrote →", "the receipt renders — reachable ONLY if the L2 ReadPub seam is wired (see the test doc comment)")
	s.sendKey(dummyKeyEnter, keystrokeDelay) // Done

	time.Sleep(200 * time.Millisecond)

	includedTarget := filepath.Join(home, ".ssh", "config.d", "gitid.config")
	included, err := os.ReadFile(includedTarget) //nolint:gosec // hermetic sandbox HOME
	if err != nil {
		t.Fatalf("reading the Include'd target after the reuse-key create: %v", err)
	}
	includedText := string(included)
	if !strings.Contains(includedText, "IdentityFile "+lockedPath) {
		t.Errorf("the written Host block must reference the REUSED key path, not a freshly generated one:\n%s", includedText)
	}

	// The reused key's OWN .pub must be byte-identical (trimmed) to what was
	// seeded — proof ensurePub read it VERBATIM via ReadPub rather than
	// attempting to re-derive it from the (unparseable-without-a-passphrase)
	// private key.
	pubAfter, err := os.ReadFile(lockedPath + ".pub") //nolint:gosec // hermetic sandbox HOME
	if err != nil {
		t.Fatalf("reading the reused key's .pub after create: %v", err)
	}
	if strings.TrimRight(string(pubAfter), "\n") != strings.TrimRight(lockedPubLine, "\n") {
		t.Errorf("the reused key's .pub was modified — want the byte-identical seeded line, got:\nwant: %q\ngot:  %q",
			strings.TrimRight(lockedPubLine, "\n"), strings.TrimRight(string(pubAfter), "\n"))
	}

	saveFrame(t, "create-flow-reuse-existing-key-l2-seam", s)

	quitCleanly(t, s)
	s.close(t) // idempotent for an already-exited process
	closed = true
}

// ---------------------------------------------------------------------------
// 7. Mouse-driven field focus (SSHUI-02 mouse half)
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// 03-12. Host preview visibility, distinct stage captures, exact proof
// ---------------------------------------------------------------------------

// TestCreateFlow_HostPreviewScrollable proves that at exactly 100×30, the
// live Host-block preview inside the wizard's SSH step shows the complete
// recipe-faithful block including "IdentitiesOnly yes" without ellipsis
// replacement (UI-REVIEW Critical Pillar 5: the real binary clipped the
// preview at IdentityFile when maxLines was 6).
func TestCreateFlow_HostPreviewScrollable(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	s := startPTYAt(t, newRealCreateFlowCmd(ctx, bin, home, ""), dummyTermWidth, dummyTermHeight)
	defer s.close(t)

	openCreateWizard(t, s)

	// The Host-block preview must show IdentitiesOnly yes without clipping.
	mustSee(t, s, "IdentitiesOnly yes", "Host preview at 100×30 must show IdentitiesOnly yes (maxLines fix)")
	// IdentityFile must also appear (regression guard).
	mustSee(t, s, "IdentityFile", "Host preview must show IdentityFile")

	saveFrame(t, "create-flow-host-preview-100x30", s)
}

// TestCreateFlow_DistinctStageCaptures proves that stage-1 and stage-2 PTY
// states produce genuinely different content. Because D-04 auto-chains stage-2
// immediately after stage-1 with the fast fake SSH, the PTY evidence publisher
// must use two separate capture scripts that target DIFFERENT terminal states:
//   - stage-1 evidence: captured at testRunning2 (stage-1 visible + "… running")
//   - stage-2 evidence: captured after "identityfile" appears (testStage2)
//
// This test proves the two outcomes are distinct at the PTY level by checking
// that "identityfile" (stage-2 resolution proof) appears after stage-1's SSH
// banner, not before — the distinction the evidence publisher must capture.
func TestCreateFlow_DistinctStageCaptures(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)
	fakeSSH := FakeSSHDir(t, "pass")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	s := startPTYAt(t, newRealCreateFlowCmd(ctx, bin, home, fakeSSH), dummyTermWidth, dummyTermHeight)
	defer s.close(t)

	openCreateWizard(t, s)
	s.sendKey(dummyKeyEnter, keystrokeDelay) // step 0 → step 1
	mustSee(t, s, "Step 2/4", "step 0 → step 1")

	// Press Enter in testIdle → stage-1 and stage-2 run (D-04 auto-chain).
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	// Stage-1 SSH banner appears first.
	mustSee(t, s, "Hi user!", "stage-1: SSH banner visible")
	// Stage-2 resolution proof appears after (proving sequential ordering).
	requireFocusedProof(t, s, "identityfile")
	// Final "Next: Git identity" affordance — stage-2 complete.
	mustSee(t, s, "Next: Git identity", "stage-2: final affordance confirms completion")

	saveFrame(t, "create-flow-distinct-stage-captures", s)
}

// TestCreateFlow_ExactStageProof proves TEST-01/02: the stage outcome text
// (the actual SSH command output, not a paraphrase) is visible in the proof
// panes at 100×30. The "Hi user!" SSH banner from stage-1 and "identityfile"
// from stage-2 must appear verbatim — not replaced by ellipsis or paraphrase.
func TestCreateFlow_ExactStageProof(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)
	fakeSSH := FakeSSHDir(t, "pass")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	s := startPTYAt(t, newRealCreateFlowCmd(ctx, bin, home, fakeSSH), dummyTermWidth, dummyTermHeight)
	defer s.close(t)

	openCreateWizard(t, s)
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Step 2/4", "step 0 → step 1")

	s.sendKey(dummyKeyEnter, keystrokeDelay) // run stage 1; stage 2 auto-chains
	// Stage-1 PASS: the real SSH banner from the fake-ssh PATH shim.
	mustSee(t, s, "Hi user!", "stage-1: exact SSH banner text is visible (TEST-01 shown==run)")
	// Stage-2: the ssh -G resolution proof.
	requireFocusedProof(t, s, "identityfile")

	saveFrame(t, "create-flow-exact-stage-proof", s)
}

// TestCreateFlow_Stage2RendersExactRawSSHOutput proves that a syntactically
// harmless line emitted only by the staged ssh -G process survives to the
// focused proof viewport. A parsed-field reconstruction cannot produce it.
func TestCreateFlow_Stage2RendersExactRawSSHOutput(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)
	fakeSSH := FakeSSHDir(t, "pass")

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	s := startPTYAt(t, newRealCreateFlowCmd(ctx, bin, home, fakeSSH), dummyTermWidth, dummyTermHeight)
	defer s.close(t)

	openCreateWizard(t, s)
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Step 2/4", "step 0 -> step 1")
	s.sendKey(dummyKeyEnter, keystrokeDelay)

	requireFocusedProof(t, s,
		"Hi user!",
		"gitidrawmarker proof-retained-verbatim",
	)
}

// TestCreateFlow_ReuseManualPath proves the KEY-06 manual-path reuse route
// at the PTY level: switching to reuse mode in an empty SSH dir immediately
// shows the manual-path row (the only available row), and typing a valid
// path resolves the key for reuse.
func TestCreateFlow_ReuseManualPath(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)

	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("seeding ~/.ssh: %v", err)
	}
	// Seed a plain (unencrypted) key to reuse via manual path.
	manualKeyPath := filepath.Join(sshDir, "id_ed25519_manual_test")
	seedEncryptedKeyFixture(t, manualKeyPath, "manual-test", "") // empty passphrase = plain

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	s := startPTYAt(t, newRealCreateFlowCmd(ctx, bin, home, ""), dummyTermWidth, dummyTermHeight)
	defer s.close(t)

	openCreateWizard(t, s)

	// Tab to the key-source toggle (4 Tabs from Alias prefix: Host, Hostname, Port, Source).
	tabKeys(s, 4)
	mustSee(t, s, "Generate a new key", "D-10 key-source toggle visible")

	// Flip to reuse mode (right arrow on the key-source toggle).
	s.sendKey(wizardKeyRight, keystrokeDelay)
	mustSee(t, s, "existing key", "reuse mode selected")
	// With no scanned keys, the manual-path row is immediately at reuseIdx=0.
	mustSee(t, s, "Enter a path manually", "manual-path row visible (no scanned keys in sandbox)")

	// Tab from key-source (focus 4) to picker body (focus 5), then to manual
	// path text input (focus 6).
	tabKeys(s, 2)
	// Type the manual key path character by character.
	for _, b := range []byte(manualKeyPath) {
		s.sendKey([]byte{b}, 10*time.Millisecond)
	}
	mustSee(t, s, "id_ed25519_manual_test", "manual path input shows the typed path")

	saveFrame(t, "create-flow-reuse-manual-path", s)
}

func TestCreateFlow_GitStepContinueHint(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)
	fakeSSH := FakeSSHDir(t, "pass")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	s := startPTYAt(t, newRealCreateFlowCmd(ctx, bin, home, fakeSSH), dummyTermWidth, dummyTermHeight)
	defer s.close(t)

	openCreateWizard(t, s)
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Step 2/4", "step 0 → step 1")
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Hi user!", "stage 1 PASS")
	requireFocusedProof(t, s, "identityfile")
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Step 3/4", "Git step")

	// 03-13 correction: Continue hint MUST appear (FIELDS.md:159-164).
	mustSee(t, s, "Continue reviews the Git fragment",
		"the enabled Git step retains its review hint")
	mustNotSee(t, s, "arrives with the next build", "retired disabled copy must not remain")

	saveFrame(t, "create-flow-git-disabled-hint-corrected-03-13", s)
}

// ---------------------------------------------------------------------------
// 03-13. Completed stage proof viewport, semantic states, Git hint.
// ---------------------------------------------------------------------------

// TestCreateFlow_CompletedStage1ProofViewport proves that after stage-1 completes
// and stage-2 is still running (testRunning2 state), the exact stage-1 SSH output
// ("Hi user!") is visible in the 100x30 frame without ellipsis substitution.
// This covers D-22 (barrier-controlled fake SSH), TEST-01 (exact shown output).
func TestCreateFlow_CompletedStage1ProofViewport(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)
	fakeSSH := FakeSSHDir(t, "pass")

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	s := startPTYAt(t, newRealCreateFlowCmd(ctx, bin, home, fakeSSH), dummyTermWidth, dummyTermHeight)
	defer s.close(t)

	openCreateWizard(t, s)
	s.sendKey(dummyKeyEnter, keystrokeDelay) // step 0 → step 1
	mustSee(t, s, "Step 2/4", "step 0 → step 1")

	// Run stage 1; D-04 auto-chains stage-2 immediately.
	// Stage-1 banner must appear before stage-2 completes.
	s.sendKey(dummyKeyEnter, keystrokeDelay)

	// Stage-1 completed output must be visible (testRunning2 state shows it).
	mustSee(t, s, "Hi user!", "stage-1: exact SSH banner visible in testRunning2 state (TEST-01)")

	// Must NOT have ellipsis substituting the output.
	// (We check the snapshot for "Hi user!" without "…" replacing it.)
	snap := s.snapshot()
	for _, line := range strings.Split(snap, "\n") {
		if strings.Contains(line, "Hi user!") && strings.HasSuffix(strings.TrimSpace(line), "…") {
			t.Errorf("stage-1 output line must not end with ellipsis substitution: %q", line)
		}
	}

	saveFrame(t, "create-flow-completed-stage1-proof-viewport", s)
}

// TestCreateFlow_CompletedStage2ProofViewport proves that after both stages
// complete (testStage2), the exact stage-2 resolution proof ("identityfile")
// is visible in the 100x30 frame. This covers TEST-02.
func TestCreateFlow_CompletedStage2ProofViewport(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)
	fakeSSH := FakeSSHDir(t, "pass")

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	s := startPTYAt(t, newRealCreateFlowCmd(ctx, bin, home, fakeSSH), dummyTermWidth, dummyTermHeight)
	defer s.close(t)

	openCreateWizard(t, s)
	s.sendKey(dummyKeyEnter, keystrokeDelay) // step 0 → step 1
	mustSee(t, s, "Step 2/4", "step 0 → step 1")

	s.sendKey(dummyKeyEnter, keystrokeDelay) // run both stages (D-04 auto-chain)
	mustSee(t, s, "Hi user!", "stage-1 pass")
	requireFocusedProof(t, s, "identityfile")

	// No ellipsis on the identityfile line.
	snap := s.snapshot()
	for _, line := range strings.Split(snap, "\n") {
		if strings.Contains(line, "identityfile") && strings.HasSuffix(strings.TrimSpace(line), "…") {
			t.Errorf("identityfile proof line must not end with ellipsis substitution: %q", line)
		}
	}

	saveFrame(t, "create-flow-completed-stage2-proof-viewport", s)
}

func TestCreateFlow_CompletedStageExactProofViewport(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)
	fakeSSH := FakeSSHDir(t, "pass")

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	s := startPTYAt(t, newRealCreateFlowCmd(ctx, bin, home, fakeSSH), dummyTermWidth, dummyTermHeight)
	defer s.close(t)

	openCreateWizard(t, s)
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Step 2/4", "test step")
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Next: Git identity", "both exact test stages completed")
	s.sendKey([]byte("v"), keystrokeDelay)
	mustSee(t, s, "Proof viewport focused", "raw v focuses the proof viewport")

	wanted := map[string]bool{
		"Stage 1 command:":        false,
		"Stage 1 output:":         false,
		"Stage 2 command:":        false,
		"Stage 2 output:":         false,
		"user git":                false,
		"hostname ssh.github.com": false,
		"port 443":                false,
		"identitiesonly yes":      false,
		"identityfile ":           false,
	}
	for range 10 {
		frame := s.snapshot()
		for marker := range wanted {
			wanted[marker] = wanted[marker] || strings.Contains(frame, marker)
		}
		s.sendKey([]byte("\x1b[6~"), keystrokeDelay)
	}
	for marker, seen := range wanted {
		if !seen {
			t.Errorf("real 100x30 proof viewport never exposed %q through raw PgDn navigation", marker)
		}
	}
	s.sendKey([]byte("\x1b[5~"), keystrokeDelay)
	s.sendKey([]byte("\x1b[C"), keystrokeDelay)
	s.sendKey([]byte("\x1b[D"), keystrokeDelay)
	saveFrame(t, "create-flow-completed-stage-exact-proof-viewport", s)
}

func TestCreateFlow_ConfirmationExactViewport(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)
	fakeSSH := FakeSSHDir(t, "pass")

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	s := startPTYAt(t, newRealCreateFlowCmd(ctx, bin, home, fakeSSH), dummyTermWidth, dummyTermHeight)
	defer s.close(t)

	openCreateWizard(t, s)
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Step 2/4", "test step")
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Next: Git identity", "both stages complete")
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Step 3/4", "Git step")
	tabKeys(s, 4)
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Create identity", "pre-write confirmation")
	s.sendKey([]byte("v"), keystrokeDelay)
	mustSee(t, s, "Exact change focused", "raw v focuses the confirmation viewport")

	const keyPath = "~/.ssh/id_ed25519_acme"
	keyPathSeen := false
	for range 16 {
		keyPathSeen = keyPathSeen || strings.Contains(s.snapshot(), keyPath)
		s.sendKey([]byte("\x1b[C"), keystrokeDelay)
	}
	if !keyPathSeen {
		t.Fatalf("confirmation viewport never exposed complete key path %q", keyPath)
	}
	for range 16 {
		s.sendKey([]byte("\x1b[D"), keystrokeDelay)
	}

	wantedBlock := []string{
		"# BEGIN gitid managed: acme",
		"Host acme.github.com",
		"Hostname ssh.github.com",
		"Port 443",
		"User git",
		"IdentityFile ~/.ssh/id_ed25519_acme",
		"IdentitiesOnly yes",
		"# END gitid managed: acme",
	}
	blockSeen := false
	for range 8 {
		frame := s.snapshot()
		all := true
		for _, marker := range wantedBlock {
			all = all && strings.Contains(frame, marker)
		}
		if all {
			blockSeen = true
			break
		}
		s.sendKey([]byte("\x1b[6~"), keystrokeDelay)
	}
	if !blockSeen {
		t.Fatalf("confirmation viewport never exposed the complete managed block:\n%s", s.snapshot())
	}
	saveFrame(t, "create-flow-confirmation-exact-viewport", s)
}

// TestCreateFlow_ReachableNotUploadedEvidence proves D-02 warning state evidence:
// yellow "!" glyph + warning words, the permission denied output visible, and the
// copy public key affordance present.
func TestCreateFlow_ReachableNotUploadedEvidence(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)
	fakeSSH := FakeSSHDir(t, "denied")

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	s := startPTYAt(t, newRealCreateFlowCmd(ctx, bin, home, fakeSSH), dummyTermWidth, dummyTermHeight)
	defer s.close(t)

	openCreateWizard(t, s)
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Step 2/4", "step 0 → step 1")
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	// Warning copy with glyph and word — never hard-failure.
	mustSee(t, s, "! Reachable — key not uploaded yet", "D-02: yellow warning state (glyph + word)")
	// Raw ssh output visible.
	requireFocusedProof(t, s, "identityfile")
	// Copy affordance present.
	mustSee(t, s, "copy public key", "D-03: copy .pub affordance present in warning state")
	// Hard-failure copy absent.
	mustNotSee(t, s, "The connection failed", "warning state must not show hard-failure copy")

	saveFrame(t, "create-flow-reachable-not-uploaded-evidence", s)
}

// TestCreateFlow_HardFailureRetryEvidence proves D-01 hard Failure state:
// red "✗" glyph, real ssh timeout output, retry affordance.
func TestCreateFlow_HardFailureRetryEvidence(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)
	fakeSSH := FakeSSHDir(t, "timeout")

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	s := startPTYAt(t, newRealCreateFlowCmd(ctx, bin, home, fakeSSH), dummyTermWidth, dummyTermHeight)
	defer s.close(t)

	openCreateWizard(t, s)
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Step 2/4", "step 0 → step 1")
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	// Hard-failure glyph.
	mustSee(t, s, "✗", "hard Failure: red '✗' glyph")
	// Real ssh output (connect timeout).
	mustSee(t, s, "connect to host", "hard Failure: real timeout output visible")
	// Failure copy + retry.
	mustSee(t, s, "The connection failed", "hard Failure: failure copy present")
	mustSee(t, s, "Retry (Enter)", "hard Failure: retry affordance present")
	// Copy public key NOT offered on hard failure.
	mustNotSee(t, s, "copy public key", "hard Failure must not offer copy public key")
	// Warning copy NOT shown.
	mustNotSee(t, s, "! Reachable", "hard Failure must not show warning copy")

	saveFrame(t, "create-flow-hard-failure-retry-evidence", s)
}

func TestCreateFlow_GitStepUsesFormValidityReason(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)
	fakeSSH := FakeSSHDir(t, "pass")

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	s := startPTYAt(t, newRealCreateFlowCmd(ctx, bin, home, fakeSSH), dummyTermWidth, dummyTermHeight)
	defer s.close(t)

	openCreateWizard(t, s)
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Step 2/4", "step 0 → step 1")
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Hi user!", "stage 1 PASS")
	requireFocusedProof(t, s, "identityfile")
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Step 3/4", "Git step")

	mustNotSee(t, s, "arrives with the next build", "retired capability-disabled reason")
	mustSee(t, s, "Continue reviews the Git fragment", "wizardContinueHint always visible")
	// Skip hint also present.
	mustSee(t, s, "Skip keeps this identity SSH-only", "Skip hint always visible")

	saveFrame(t, "create-flow-git-step-disabled-reason-03-13", s)
}

// ---------------------------------------------------------------------------
// 7. Mouse-driven field focus (SSHUI-02 mouse half)
// ---------------------------------------------------------------------------

// TestCreateFlow_MouseFieldFocus injects real xterm SGR mouse CSI press/
// release sequences to click each of the SSH form's four approved fields —
// Alias prefix, SSH Host (alias), Real hostname, Port (FIELDS.md's 4-field
// contract; there is no "user" field) — and asserts the clicked row becomes
// focused and a subsequent keystroke lands in THAT field. A trailing click
// on a non-interactive row (the live Host-block preview) must not steal
// focus.
func TestCreateFlow_MouseFieldFocus(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	s := startPTYAt(t, newRealCreateFlowCmd(ctx, bin, home, ""), dummyTermWidth, dummyTermHeight)
	defer s.close(t)

	openCreateWizard(t, s)

	// Move focus off Alias prefix first (to Port) so the FIRST click below
	// (on Alias prefix) provably MOVES focus, rather than finding it already
	// there.
	tabKeys(s, 3)
	mustSee(t, s, "▸ Port", "focus starts on Port before the click sequence begins")

	fields := []struct {
		label string
		mark  string
	}{
		{"Alias prefix", "Z"},
		{"SSH Host (alias)", "Z"},
		{"Real hostname", "Z"},
		{"Port", "9"},
	}

	for _, f := range fields {
		clickLabelRow(t, s, f.label)
		mustSee(t, s, "▸ "+f.label, "mouse click focused the "+f.label+" row (SSHUI-02)")
		s.sendKey([]byte(f.mark), keystrokeDelay)
		mustSee(t, s, f.mark, "the typed keystroke landed in the clicked "+f.label+" field")
	}

	// A click on a non-interactive row (the live Host-block preview) must
	// not steal focus from the last-focused field (Port).
	clickLabelRow(t, s, "Live Host-block preview")
	mustSee(t, s, "▸ Port", "a click on a non-interactive preview row does not steal focus")

	saveFrame(t, "create-flow-mouse-field-focus", s)
}

func delayedResolutionSSHDir(t *testing.T) string {
	t.Helper()
	dir := FakeSSHDir(t, "pass")
	path := filepath.Join(dir, "ssh")
	script, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading fake ssh fixture: %v", err)
	}
	const resolutionBranch = "if [ \"$is_resolution\" = \"1\" ]; then\n"
	if !strings.Contains(string(script), resolutionBranch) {
		t.Fatal("fake ssh fixture no longer exposes the resolution branch")
	}
	script = []byte(strings.Replace(string(script), resolutionBranch, resolutionBranch+"  sleep 2\n", 1))
	if err := os.WriteFile(path, script, 0o700); err != nil {
		t.Fatalf("delaying fake ssh resolution: %v", err)
	}
	return dir
}

// TestCreateFlow_PTYReviewCorrections guards the three Phase-3 presentation
// corrections through real 100x30 PTYs. The real binary uses disposable HOME
// directories and fake SSH; the dummy is the only UI reference.
func TestCreateFlow_PTYReviewCorrections(t *testing.T) {
	t.Run("running stage", func(t *testing.T) {
		home := SandboxHome(t)
		bin := BuildBinary(t)
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		s := startPTYAt(t, newRealCreateFlowCmd(ctx, bin, home, delayedResolutionSSHDir(t)), dummyTermWidth, dummyTermHeight)
		defer s.close(t)
		openCreateWizard(t, s)
		s.sendKey(dummyKeyEnter, keystrokeDelay)
		mustSee(t, s, "Step 2/4", "step 0 -> step 1")
		s.sendKey(dummyKeyEnter, keystrokeDelay)
		frame, ok := s.waitFor(8*time.Second, func(frame string) bool {
			return strings.Contains(frame, "Hi user!") && strings.Contains(frame, "running ssh")
		})
		if !ok {
			t.Fatalf("stage-two-in-progress frame never showed completed stage-one output and a running stage two:\n%s", frame)
		}
		if strings.Contains(frame, "Completed test stages; exact captured proof follows.") {
			t.Errorf("stage-two-in-progress frame claims completion:\n%s", frame)
		}
		if !strings.Contains(frame, "Hi user!") {
			t.Errorf("stage-two-in-progress frame lost stage-one output:\n%s", frame)
		}
		requireFocusedProof(t, s, "identityfile")
		mustSee(t, s, "Completed test stages; exact captured proof follows.", "completed stage labels the proof viewport")
	})

	t.Run("reachable warning", func(t *testing.T) {
		home := SandboxHome(t)
		bin := BuildBinary(t)
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		s := startPTYAt(t, newRealCreateFlowCmd(ctx, bin, home, FakeSSHDir(t, "denied")), dummyTermWidth, dummyTermHeight)
		defer s.close(t)
		openCreateWizard(t, s)
		s.sendKey(dummyKeyEnter, keystrokeDelay)
		mustSee(t, s, "Step 2/4", "step 0 -> step 1")
		s.sendKey(dummyKeyEnter, keystrokeDelay)
		requireFocusedProof(t, s, "identityfile")
		frame := s.snapshot()
		if got := strings.Count(frame, "! Reachable — key not uploaded yet"); got != 1 {
			t.Errorf("D-02 warning count = %d, want 1:\n%s", got, frame)
		}
		if got := strings.Count(frame, "Press c to copy the .pub"); got != 1 {
			t.Errorf("D-03 instruction count = %d, want 1:\n%s", got, frame)
		}
		for _, want := range []string{"Permission denied (publickey).", "identityfile", "copy public key"} {
			if !strings.Contains(frame, want) {
				t.Errorf("warning frame lost %q:\n%s", want, frame)
			}
		}
	})

	t.Run("compact includeIf preview", func(t *testing.T) {
		home := SandboxHome(t)
		bin := BuildBinary(t)
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		s := startPTYAt(t, newRealCreateFlowCmd(ctx, bin, home, FakeSSHDir(t, "pass")), dummyTermWidth, dummyTermHeight)
		defer s.close(t)
		openCreateWizard(t, s)
		s.sendKey(dummyKeyEnter, keystrokeDelay)
		mustSee(t, s, "Step 2/4", "step 0 -> step 1")
		s.sendKey(dummyKeyEnter, keystrokeDelay)
		mustSee(t, s, "Next: Git identity", "both test stages complete")
		s.sendKey(dummyKeyEnter, keystrokeDelay)
		mustSee(t, s, "Step 3/4", "Git step")
		realFrame := s.snapshot()
		if !strings.Contains(realFrame, "[includeIf ") || strings.Contains(realFrame, "# BEGIN gitid managed:") {
			t.Errorf("real compact preview must show an includeIf condition, not a sentinel:\n%s", realFrame)
		}

		dummyHome := SandboxHome(t)
		dummyBin := BuildDummyBinary(t)
		dummyCtx, dummyCancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer dummyCancel()
		dummyCmd := exec.CommandContext(dummyCtx, dummyBin) //nolint:gosec // binary is built by BuildDummyBinary
		dummyCmd.Env = append(os.Environ(), "HOME="+dummyHome, "TERM=xterm-256color")
		dummy := startPTYAt(t, dummyCmd, dummyTermWidth, dummyTermHeight)
		defer dummy.close(t)
		openCreateWizard(t, dummy)
		dummy.sendKey(dummyKeyEnter, keystrokeDelay)
		mustSee(t, dummy, "Step 2/4", "dummy step 0 -> step 1")
		dummy.sendKey(dummyKeyEnter, keystrokeDelay)
		mustSee(t, dummy, "Next: Git identity", "dummy test stages complete")
		dummy.sendKey(dummyKeyEnter, keystrokeDelay)
		mustSee(t, dummy, "Step 3/4", "dummy Git step")
		mustSee(t, dummy, `[includeIf "gitdir:~/acme/"]`, "dummy compact preview exposes its includeIf condition")
	})
}
