//go:build e2e

package e2e

// ui_pty_e2e_test.go — Autonomous PTY-driven UI e2e (Plan 08, Task 1, D-13).
//
// These tests drive the REAL gitid binary over a pseudo-terminal using
// github.com/creack/pty, feeding RAW keystroke byte sequences (not synthesised
// tea.Msg objects), and assert on vt100-decoded frames produced by
// github.com/charmbracelet/x/vt.
//
// This closes the D-13/D-16 blindspot: teatest / Update(msg) inject a tea.Msg
// and bypass the real input decoder, so the historical "couldn't type in the
// wizard" bug passed every model test and failed only on a real terminal. Here
// raw bytes go through the same tty/stdin path as a real user.
//
// Each test case:
//  1. Builds the gitid binary once (BuildBinary, cached via sync.Once).
//  2. Seeds a sandboxed HOME with fixtures appropriate to the surface under test.
//  3. Starts gitid via pty.StartWithSize at a fixed 80×24 terminal size.
//  4. Spawns a goroutine that pumps PTY output into a vt.Emulator; that
//     goroutine signals each write so callers can poll the decoded text grid
//     with a bounded WaitFor.
//  5. Writes raw keystrokes (type a name, Tab, Esc, ctrl+r, 'A', 'c') and
//     asserts on the decoded text — e.g. the typed name appears in the form
//     field (input-decoding regression test, D-13).
//  6. Snapshots each decoded frame to tmp/ui-frames/<surface>.txt (WR-12:
//     gitignored scratch space, never a tracked directory — see saveFrame)
//     so a UI critique has REAL PTY-decoded frames as primary evidence.
//
// Security notes (gosec/CLAUDE.md):
//   - The gitid binary is built from this repo, not a user-supplied string.
//   - exec.Command invocations are arg-slices (no shell expansion).
//   - No production imports of creack/pty or charmbracelet/x/vt; they appear
//     only behind the e2e build tag.

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
)

// ptyTermWidth / ptyTermHeight are the fixed terminal dimensions used for all
// PTY e2e tests. 80×24 is the POSIX default and the minimum size gitid supports.
const (
	ptyTermWidth  = 80
	ptyTermHeight = 24
)

// snapshotReq is a request/response pair for asking the event loop for the
// current emulator text. The resp channel is created per-request.
type snapshotReq struct {
	resp chan string
}

// ptySession wraps an active gitid process running under a pseudo-terminal.
// All emulator access is serialised through the event loop goroutine (started
// in startPTY) to avoid data races on the vt.Emulator (not goroutine-safe).
type ptySession struct {
	ptmx      *os.File         // master PTY file (write keystrokes here)
	cmd       *exec.Cmd        // the running gitid process
	reqCh     chan snapshotReq // callers send a request with a reply channel
	stopCh    chan struct{}    // close to stop the event loop
	done      chan struct{}    // closed when the event loop exits
	drainDone chan struct{}    // closed when the response drainer (goroutine B) exits
	emuPipe   *io.PipeWriter   // emulator's input pipe writer; closed to unblock the drainer
}

// startPTY launches cmd under a pseudo-terminal of fixed size ptyTermWidth×ptyTermHeight.
// A single event-loop goroutine serialises all emulator access:
//
//  1. Goroutine A (PTY reader): reads raw bytes from the PTY master and forwards
//     them on ptyCh (buffered) to the event loop.
//
//  2. Goroutine B (response drainer): drains emu.Read() — the emulator's internal
//     io.PipeReader — and writes the terminal-capability responses back to ptmx.
//     Without this drainer, emu.Write() blocks when Bubble Tea issues DA/DECRQM
//     queries whose responses must flow back to the app.
//
//  3. Goroutine C (event loop): receives PTY chunks and feeds them to emu.Write(),
//     and answers snapshot requests from the test via reqCh.
//
// emu is owned exclusively by goroutine C. No other goroutine calls emu.Write(),
// emu.String(), etc. This single-owner design is the only reliable way to prevent
// the deadlock: (goroutine A holds mutex + blocks in emu.Write → io.PipeWriter,
// goroutine B blocks in mutex.Lock called by snapshot()).
func startPTY(t *testing.T, cmd *exec.Cmd) *ptySession {
	t.Helper()
	emu := vt.NewEmulator(ptyTermWidth, ptyTermHeight)

	ws := &pty.Winsize{Rows: ptyTermHeight, Cols: ptyTermWidth}
	ptmx, err := pty.StartWithSize(cmd, ws) //nolint:gosec // arg-slice; binary path from BuildBinary
	if err != nil {
		t.Fatalf("startPTY: pty.StartWithSize: %v", err)
	}

	// emu.InputPipe() returns the emulator's internal *io.PipeWriter. Closing it
	// (CloseWithError) is the io.Pipe-safe way to unblock the drainer's blocking
	// emu.Read() at shutdown WITHOUT calling emu.Close() — the latter writes the
	// unsynchronised emu.closed bool that races with the drainer's Read (the pipe
	// itself is concurrency-safe; the bool is not).
	emuPipe, ok := emu.InputPipe().(*io.PipeWriter)
	if !ok {
		t.Fatalf("startPTY: emu.InputPipe() is %T, want *io.PipeWriter", emu.InputPipe())
	}

	s := &ptySession{
		ptmx:      ptmx,
		cmd:       cmd,
		reqCh:     make(chan snapshotReq, 4),
		stopCh:    make(chan struct{}),
		done:      make(chan struct{}),
		drainDone: make(chan struct{}),
		emuPipe:   emuPipe,
	}

	ptyCh := make(chan []byte, 128) // buffered to decouple blocking Read from event loop

	// Goroutine A: PTY reader — reads raw PTY output and forwards to event loop.
	go func() {
		buf := make([]byte, 4096)
		for {
			n, rerr := ptmx.Read(buf)
			if n > 0 {
				cp := make([]byte, n)
				copy(cp, buf[:n])
				select {
				case ptyCh <- cp:
				case <-s.stopCh:
					return
				}
			}
			if rerr != nil {
				return
			}
		}
	}()

	// Goroutine B: response drainer — drains emu.Read() (the emulator's internal
	// PipeReader) so emu.Write() never blocks on a full pipe. Responses are fed
	// back to the PTY master so the Bubble Tea app receives its terminal-capability
	// replies (DA, DECRQM, etc.).
	go func() {
		defer close(s.drainDone)
		resp := make([]byte, 256)
		for {
			n, rerr := emu.Read(resp)
			if n > 0 {
				_, _ = ptmx.Write(resp[:n]) //nolint:errcheck // best-effort feed-back
			}
			if rerr != nil {
				return // io.EOF once emuPipe is CloseWithError'd in close()
			}
		}
	}()

	// Goroutine C: event loop — sole owner of emu.
	go func() {
		defer close(s.done)
		for {
			select {
			case chunk := <-ptyCh:
				_, _ = emu.Write(chunk) //nolint:errcheck // vt.Emulator.Write always returns nil err
			case req := <-s.reqCh:
				req.resp <- emu.String()
			case <-s.stopCh:
				// Do NOT call emu.Close() here: it writes the unsynchronised
				// emu.closed bool concurrently with the drainer's emu.Read().
				// close() unblocks the drainer via emuPipe.CloseWithError instead,
				// after this event loop (the sole emu.Write caller) has exited.
				return
			}
		}
	}()

	return s
}

// close terminates the session: sends ctrl+c to the process, stops the event
// loop, closes the PTY master, and waits for all goroutines to drain.
func (s *ptySession) close(t *testing.T) {
	t.Helper()
	// Send ctrl+c to terminate the TUI gracefully (best-effort).
	_, _ = s.ptmx.Write([]byte{0x03}) //nolint:errcheck
	// Give the process a moment to handle the signal — bounded, so a
	// process that (for whatever reason: a sandboxed PTY not delivering
	// the interrupt byte's signal semantics, a hung render loop, …) never
	// reacts to ctrl+c cannot hang test cleanup forever. Escalates to a
	// hard kill only after the grace period; a healthy process that
	// handles ctrl+c exits well within it and this never fires.
	waitDone := make(chan struct{})
	go func() {
		_ = s.cmd.Wait() //nolint:errcheck // best-effort cleanup; the test's own assertions already ran
		close(waitDone)
	}()
	select {
	case <-waitDone:
	case <-time.After(5 * time.Second):
		if s.cmd.Process != nil {
			_ = s.cmd.Process.Kill() //nolint:errcheck // best-effort — Wait() below still reaps it
		}
		<-waitDone
	}
	// Stop the event loop (sole emu.Write caller) first.
	close(s.stopCh)
	<-s.done // wait for event loop to exit — no more emu.Write after this
	// Now unblock the drainer's blocking emu.Read() via the io.Pipe-safe path.
	// This avoids emu.Close()'s unsynchronised closed-bool write racing the Read.
	_ = s.emuPipe.CloseWithError(io.EOF)
	<-s.drainDone // wait for the drainer to exit
	s.ptmx.Close()
}

// sendKey writes raw bytes to the PTY master, followed by a brief sleep to let
// the TUI process and render the input before the next keystroke.
// raw MUST be a static literal — never derived from user input (gosec G204 clean).
func (s *ptySession) sendKey(raw []byte, delay time.Duration) {
	_, _ = s.ptmx.Write(raw) //nolint:errcheck // best-effort
	time.Sleep(delay)
}

// snapshot returns the current decoded text of the emulator (no ANSI codes).
// It sends a request to the event loop goroutine (sole owner of emu) and waits
// for the response. This is safe to call from the test goroutine at any time.
func (s *ptySession) snapshot() string {
	req := snapshotReq{resp: make(chan string, 1)}
	select {
	case s.reqCh <- req:
	case <-s.done:
		return ""
	}
	select {
	case text := <-req.resp:
		return text
	case <-s.done:
		return ""
	}
}

// waitFor polls snapshot() up to timeout, returning true when predicate(text)
// returns true. On failure it returns the LAST snapshot the predicate was
// actually evaluated against — never a fresh, unchecked one — so a failure
// message's "Last frame" is trustworthy diagnostic evidence, not a
// misleading coincidence of exactly when the extra call happened to land.
// (v0.1.0-rc.5's investigation found the prior version took one final,
// unvalidated snapshot after the deadline for the failure message, which
// could show content that would have passed had it been the one checked —
// making several rc.3-rc.5 "never appeared" diagnoses unreliable.)
//
// timeout is scaled by ciTimeoutMultiplier: v0.1.0-rc.1 and rc.2 both failed
// make test-e2e on GitHub Actions with a shifting, non-reproducing set of
// PTY content-assertion misses across all 3 matrix OSes and across dozens
// of distinct callers of this one function (never the same test twice) —
// the signature of a shared runner slower/noisier than local dev hardware,
// not a product defect. Scaling centrally here, instead of at each of the
// ~30 call sites across this package, is what actually closes every one of
// them in a single change.
func (s *ptySession) waitFor(timeout time.Duration, predicate func(string) bool) (last string, ok bool) {
	timeout *= ciTimeoutMultiplier()
	deadline := time.Now().Add(timeout)
	for {
		last = s.snapshot()
		if predicate(last) {
			return last, true
		}
		if !time.Now().Before(deadline) {
			return last, false
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// saveFrame writes the current emulator snapshot to a scratch ui-frames
// directory, creating it if necessary. Non-fatal: frame saving is evidence
// collection only.
//
// WR-12: this used to write into .planning/phases/05.7-.../ui-frames/, a
// TRACKED directory — every run embeds absolute sandbox paths
// (t.TempDir()) and nanosecond backup suffixes, so every run on every
// machine produced a different file and dirtied the working tree (the same
// failure mode TestGateVisualRegressionReadOnly exists to prevent for the
// sibling screenshot gate). /tmp/ at the repo root is ALREADY gitignored
// for exactly this purpose (".gitignore: Local scratch directory
// (screenshots, notes — not part of the repo)"), and — unlike t.TempDir(),
// which Go removes at test end — it survives the run for manual
// inspection.
func saveFrame(t *testing.T, name string, s *ptySession) {
	t.Helper()
	saveFrameContent(t, name, s.snapshot())
}

// saveFrameContent is saveFrame's underlying write, taking the content
// directly rather than re-snapshotting — shared with captureGlobalGitFrame
// (global_git_pty_e2e_test.go), which saves a NORMALIZED copy of the
// snapshot rather than the raw one (09.5-05-PLAN.md Task 2: the Set-keys
// detail pane's absolute ShortSandboxHome path is not a stable promoted
// baseline — see normalizeShortSandboxHomePath's doc comment).
func saveFrameContent(t *testing.T, name, content string) {
	t.Helper()
	root := repoRoot(t)
	dir := filepath.Join(root, "tmp", "ui-frames")
	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // test-only dir (G306)
		t.Logf("saveFrame: MkdirAll: %v (non-fatal)", err)
		return
	}
	path := filepath.Join(dir, name+".txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil { //nolint:gosec // test-only snapshot (G306)
		t.Logf("saveFrame: WriteFile: %v (non-fatal)", err)
		return
	}
	t.Logf("saved PTY frame: %s", path)
}

// newPTYCmd constructs an exec.Cmd for the gitid binary (no-args = TUI mode)
// with the given sandboxed HOME and any extra environment entries.
// The binary path comes from BuildBinary — never from user input (G204 clean).
func newPTYCmd(t *testing.T, ctx context.Context, bin, home string, extraEnv ...string) *exec.Cmd {
	t.Helper()
	cmd := exec.CommandContext(ctx, bin) //nolint:gosec // bin from BuildBinary; no user input
	env, _ := e2eEnv(t, home)
	cmd.Env = append(append(env, "TERM=xterm-256color"), extraEnv...)
	return cmd
}

// seedMinimalIdentity writes enough fixtures to HOME so the TUI sidebar has at
// least one managed identity (needed for the Copy and Add Repo surface tests).
func seedMinimalIdentity(t *testing.T, home, name string) {
	t.Helper()
	sshDir := filepath.Join(home, ".ssh")
	gitconfigD := filepath.Join(home, ".gitconfig.d")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("seedMinimalIdentity: MkdirAll .ssh: %v", err)
	}
	if err := os.MkdirAll(gitconfigD, 0o755); err != nil {
		t.Fatalf("seedMinimalIdentity: MkdirAll .gitconfig.d: %v", err)
	}

	// Minimal SSH private/public key stubs (not real crypto — tests never do SSH).
	privKey := filepath.Join(sshDir, "id_ed25519_"+name)
	pubKey := privKey + ".pub"
	if err := os.WriteFile(privKey, []byte("-----BEGIN OPENSSH PRIVATE KEY-----\nSTUB\n-----END OPENSSH PRIVATE KEY-----\n"), 0o600); err != nil {
		t.Fatalf("seedMinimalIdentity: WriteFile privKey: %v", err)
	}
	pubContent := fmt.Sprintf("ssh-ed25519 AAAAC3NzaC1lZDI1NTE5STUB %s@gitid-test\n", name)
	if err := os.WriteFile(pubKey, []byte(pubContent), 0o644); err != nil {
		t.Fatalf("seedMinimalIdentity: WriteFile pubKey: %v", err)
	}

	// Minimal ~/.ssh/config with a gitid-managed block.
	sshConfig := filepath.Join(sshDir, "config")
	sshConfigContent := fmt.Sprintf(
		"# BEGIN gitid managed: %s\nHost %s.github.com\n  HostName github.com\n  User git\n  IdentityFile ~/.ssh/id_ed25519_%s\n  IdentitiesOnly yes\n# END gitid managed: %s\n\nHost *\n  IgnoreUnknown UseKeychain\n  AddKeysToAgent yes\n",
		name, name, name, name,
	)
	if err := os.WriteFile(sshConfig, []byte(sshConfigContent), 0o600); err != nil {
		t.Fatalf("seedMinimalIdentity: WriteFile ssh/config: %v", err)
	}

	// Minimal ~/.gitconfig with includeIf block.
	gitconfigPath := filepath.Join(home, ".gitconfig")
	gitconfigContent := fmt.Sprintf(
		"[user]\n  name = Test User\n  email = test@example.com\n\n# BEGIN gitid managed: %s\n[includeIf \"gitdir:~/git/%s/\"]\n  path = ~/.gitconfig.d/%s\n# END gitid managed: %s\n",
		name, name, name, name,
	)
	if err := os.WriteFile(gitconfigPath, []byte(gitconfigContent), 0o644); err != nil {
		t.Fatalf("seedMinimalIdentity: WriteFile .gitconfig: %v", err)
	}

	// Minimal ~/.gitconfig.d/<name> fragment.
	fragment := filepath.Join(gitconfigD, name)
	fragmentContent := fmt.Sprintf("[user]\n  name = %s User\n  email = %s@example.com\n  signingkey = ~/.ssh/id_ed25519_%s.pub\n", name, name, name)
	if err := os.WriteFile(fragment, []byte(fragmentContent), 0o644); err != nil {
		t.Fatalf("seedMinimalIdentity: WriteFile fragment: %v", err)
	}
}

// seedFragmentCandidate writes an unmanaged gitconfig fragment that the TUI
// surfaces in the "Unmanaged" sidebar section as a kindFragment adopt candidate.
func seedFragmentCandidate(t *testing.T, home, name string) {
	t.Helper()
	fragPath := filepath.Join(home, ".gitconfig_"+name)
	content := fmt.Sprintf("[user]\n  name = %s User\n  email = %s@example.com\n", name, name)
	if err := os.WriteFile(fragPath, []byte(content), 0o644); err != nil {
		t.Fatalf("seedFragmentCandidate: WriteFile: %v", err)
	}
}

// keystrokeDelay is the inter-keystroke pause. Bubble Tea needs a render cycle
// between keystrokes, and the vt emulator needs time to receive and process the
// PTY output. 80ms was "conservative enough" on local dev hardware but a flat
// value can't be: v0.1.0-rc.1 through rc.4 kept failing make test-e2e on
// GitHub Actions with tests stuck mid-flow (never advancing past a step,
// not slow to render one) even after every wait/context budget in the
// package was scaled — a symptom of dropped or coalesced input under
// -race's CPU overhead on a shared runner, not a rendering delay. Scaled by
// the same ciTimeoutMultiplier as every other CI-only budget in this
// package; a var (not a func) so the ~390 existing call sites need no change.
var keystrokeDelay = 80 * time.Millisecond * ciTimeoutMultiplier()

// uiReady waits for the TUI to render its initial frame (the sidebar header).
func uiReady(t *testing.T, s *ptySession) {
	t.Helper()
	last, ok := s.waitFor(8*time.Second, func(text string) bool {
		// The TUI renders "gitid" in its header and "Identities" in the sidebar.
		return strings.Contains(text, "gitid") || strings.Contains(text, "Identities")
	})
	if !ok {
		t.Fatalf("uiReady: TUI did not render within 8 seconds. Last frame:\n%s", last)
	}
}

// TestUIPTY_RealShellBoots is the D-15 smoke proof: a bare `gitid` invocation
// on a real TTY launches the REAL approved app shell (the tuikit chrome with
// its five numbered nav tabs), driven by the real Backend composition root in
// cmd/gitid/wiring.go — not the retired 0.0.1 POC tui/ package.
//
// It is deliberately minimal. The per-screen create-flow PTY suite (form,
// both connectivity stages via the fake-ssh PATH shim, confirm-write, and the
// reuse-existing-key/ReadPub proof) is plan 03-06's job; this test only proves
// the entry point is rewired and the shell boots and keeps accepting keys.
func TestUIPTY_RealShellBoots(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second*ciTimeoutMultiplier())
	defer cancel()

	// 100x30 is the approved design's minimum geometry (D-04 capture geometry):
	// the tuikit shell refuses to render smaller and prints a resize hint
	// instead, so the smoke test must open the PTY at the real size.
	s := startPTYAt(t, newPTYCmd(t, ctx, bin, home), dummyTermWidth, dummyTermHeight)
	defer s.close(t)

	uiReady(t, s)

	// The approved chrome renders all five numbered nav tabs (SHELL-01).
	// Phase 09.4 reversed 08-01-PLAN.md Task 1's TabID split: Health and
	// Fixer collapse back into one Doctor tab (keys 1..5; key 6 is inert).
	for _, tab := range []string{"Identities", "SSH", "Git", "Doctor", "Ignore"} {
		last, ok := s.waitFor(8*time.Second, func(text string) bool {
			return strings.Contains(text, tab)
		})
		if !ok {
			t.Fatalf("real app shell never rendered the %q nav tab. Last frame:\n%s", tab, last)
		}
	}

	// The shell keeps decoding raw keystrokes: '2' selects the Global SSH tab.
	s.sendKey([]byte("2"), keystrokeDelay*2)
	last, ok := s.waitFor(8*time.Second, func(text string) bool {
		return strings.Contains(text, "Global SSH")
	})
	if !ok {
		t.Fatalf("pressing '2' did not keep the shell responsive. Last frame:\n%s", last)
	}

	saveFrame(t, "real-shell-boot", s)
}
