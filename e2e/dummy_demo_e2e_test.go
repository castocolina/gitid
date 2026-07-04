//go:build e2e

package e2e

// dummy_demo_e2e_test.go — PTY-driven e2e proof of the LIVE gitid-dummy
// design demo (02-13 Task 3, DLV-05/DLV-02).
//
// TestDummyDemo_LiveWalk drives the REAL built cmd/gitid-dummy binary over
// a pseudo-terminal at 100x30 (this demo's minimum geometry — NOT the
// 80x24 the product suite uses), feeding RAW keystroke bytes through the
// same tty/stdin path as a real user, and asserts on vt-decoded frames:
// launch, live master-detail (↓ flips the detail with no Enter), tab
// navigation, a full create-wizard walk (both test stages + ceremony +
// receipt), a doctor fix ceremony with live chip healing, the help
// overlay, and a clean q-quit (exit 0). Afterwards it walks the sandboxed
// HOME recursively and asserts ZERO files or directories were created —
// the demo is 100% in-memory (T-02-13-WRITE).
//
// The 3-goroutine emulator architecture is the same single-owner design as
// startPTY in ui_pty_e2e_test.go (see the deadlock notes there); this file
// carries a size-parameterized variant because the vt emulator dimensions
// are fixed at construction.

import (
	"context"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
)

// Dummy-demo terminal geometry: the demo's design minimum (100x30).
const (
	dummyTermWidth  = 100
	dummyTermHeight = 30
)

// startPTYAt launches cmd under a pseudo-terminal of the given size with
// the same single-owner emulator event loop as startPTY (goroutine A: PTY
// reader; B: capability-response drainer; C: event loop owning emu).
func startPTYAt(t *testing.T, cmd *exec.Cmd, cols, rows int) *ptySession {
	t.Helper()
	emu := vt.NewEmulator(cols, rows)

	ws := &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)} //nolint:gosec // small constants
	ptmx, err := pty.StartWithSize(cmd, ws)                    //nolint:gosec // arg-slice; binary path from BuildDummyBinary
	if err != nil {
		t.Fatalf("startPTYAt: pty.StartWithSize: %v", err)
	}

	emuPipe, ok := emu.InputPipe().(*io.PipeWriter)
	if !ok {
		t.Fatalf("startPTYAt: emu.InputPipe() is %T, want *io.PipeWriter", emu.InputPipe())
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

	ptyCh := make(chan []byte, 128)

	// Goroutine A: PTY reader.
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

	// Goroutine B: capability-response drainer (see startPTY notes).
	go func() {
		defer close(s.drainDone)
		resp := make([]byte, 256)
		for {
			n, rerr := emu.Read(resp)
			if n > 0 {
				_, _ = ptmx.Write(resp[:n]) //nolint:errcheck // best-effort feed-back
			}
			if rerr != nil {
				return
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
				return
			}
		}
	}()

	return s
}

// dummyKey* are the raw keystroke byte sequences the walk sends.
var (
	dummyKeyEnter     = []byte("\r")
	dummyKeyEsc       = []byte{0x1b}
	dummyKeyDown      = []byte{0x1b, 0x5b, 0x42} // ESC [ B
	dummyKeyBackspace = []byte{0x7f}
)

// mustSee polls the decoded frame for substr and fails fatally on timeout.
func mustSee(t *testing.T, s *ptySession, substr, context string) {
	t.Helper()
	last, ok := s.waitFor(8*time.Second, func(text string) bool {
		return strings.Contains(text, substr)
	})
	if !ok {
		t.Fatalf("%s: %q never appeared. Last frame:\n%s", context, substr, last)
	}
}

// mustNotSee polls until substr disappears from the decoded frame.
func mustNotSee(t *testing.T, s *ptySession, substr, context string) {
	t.Helper()
	last, ok := s.waitFor(8*time.Second, func(text string) bool {
		return !strings.Contains(text, substr)
	})
	if !ok {
		t.Fatalf("%s: %q never disappeared. Last frame:\n%s", context, substr, last)
	}
}

// TestDummyDemo_LiveWalk drives the real gitid-dummy binary end to end.
func TestDummyDemo_LiveWalk(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildDummyBinary(t)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin) //nolint:gosec // bin from BuildDummyBinary; no user input
	cmd.Env = append(os.Environ(), "HOME="+home, "TERM=xterm-256color")

	s := startPTYAt(t, cmd, dummyTermWidth, dummyTermHeight)
	closed := false
	defer func() {
		if !closed {
			s.close(t)
		}
	}()

	// ---- launch: header tabs + breadcrumb + sidebar + legend ----
	mustSee(t, s, "1 Identities", "launch: header nav tabs")
	mustSee(t, s, "4 Doctor", "launch: header nav tabs")
	mustSee(t, s, "personal", "launch: seeded sidebar row")
	mustSee(t, s, "S ssh · G git", "launch: sidebar legend line")
	mustSee(t, s, "8 ids", "launch: live health chip")

	// ---- live master-detail: ↓ flips the detail with NO Enter ----
	s.sendKey(dummyKeyDown, keystrokeDelay)
	mustSee(t, s, "! incomplete", "live detail: ↓ selects work")
	mustSee(t, s, "SSH — shown first, always", "live detail: SSH section first")

	// ---- tab navigation: 2 / 3 / 4 with per-screen signatures ----
	s.sendKey([]byte("2"), keystrokeDelay)
	mustSee(t, s, "Global SSH › Options", "tab 2: breadcrumb")
	mustSee(t, s, "Storage & preview", "tab 2: STORE-01 sub-tab label")

	s.sendKey([]byte("3"), keystrokeDelay)
	mustSee(t, s, "Global Git › Options", "tab 3: breadcrumb")
	mustSee(t, s, "main vs master", "tab 3: highlight chip")

	s.sendKey([]byte("4"), keystrokeDelay)
	// Doctor-BODY-specific proof (the "Doctor" header tab label is always
	// present, so it can never fail): the post-scan status line.
	mustSee(t, s, "Health only diagnoses", "tab 4: doctor status line after auto-scan")
	// Auto-scan runs on first entry, then findings render grouped.
	mustSee(t, s, "Private key is world-readable", "tab 4: finding title after auto-scan")

	// ---- create wizard: full walk ----
	s.sendKey([]byte("1"), keystrokeDelay)
	s.sendKey([]byte("n"), keystrokeDelay)
	mustSee(t, s, "Step 1/4", "wizard: state 1 opens")

	// Replace the default "acme" prefix with a fresh one, proving raw
	// keystrokes reach the focused input (the D-13 class of regression).
	for i := 0; i < 6; i++ {
		s.sendKey(dummyKeyBackspace, keystrokeDelay)
	}
	for _, ch := range "e2e" {
		s.sendKey([]byte(string(ch)), keystrokeDelay)
	}
	mustSee(t, s, "e2e.github.com", "wizard: auto-joined alias from the typed prefix")

	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Step 2/4", "wizard: state 2 (test)")

	s.sendKey(dummyKeyEnter, keystrokeDelay) // run stage 1
	mustSee(t, s, "Hi e2e!", "wizard: stage-1 success banner")

	s.sendKey(dummyKeyEnter, keystrokeDelay) // run stage 2
	mustSee(t, s, "identityfile", "wizard: stage-2 ssh -G proof")

	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Step 3/4", "wizard: state 3 (Git identity)")

	s.sendKey(dummyKeyEnter, keystrokeDelay) // Continue: review & write
	mustSee(t, s, `Create identity "e2e"`, "wizard: ceremony heading")

	s.sendKey(dummyKeyEnter, keystrokeDelay) // confirm write
	mustSee(t, s, "Wrote →", "wizard: receipt")

	s.sendKey(dummyKeyEnter, keystrokeDelay) // Done
	mustSee(t, s, "9 ids", "wizard: header chip id count incremented live")

	// ---- doctor: fix the selected finding through the ceremony ----
	s.sendKey([]byte("4"), keystrokeDelay)
	mustSee(t, s, "Private key is world-readable", "doctor: findings render instantly on revisit")
	s.sendKey([]byte("f"), keystrokeDelay)
	mustSee(t, s, "Fix: Private key is world-readable", "doctor: fix ceremony opens")
	s.sendKey(dummyKeyEnter, keystrokeDelay) // confirm fix
	mustSee(t, s, "Backed up →", "doctor: fix receipt")
	s.sendKey(dummyKeyEnter, keystrokeDelay) // done
	mustNotSee(t, s, "Private key is world-readable", "doctor: fixed finding disappears live")
	mustSee(t, s, "✗ 2", "doctor: header chip error count decremented live")

	// ---- help overlay ----
	s.sendKey([]byte("?"), keystrokeDelay)
	mustSee(t, s, "key-used-ssh-only", "help: full 8-state legend row")
	s.sendKey(dummyKeyEsc, keystrokeDelay*3)
	mustNotSee(t, s, "key-used-ssh-only", "help: Esc closes the overlay")

	// ---- quit: q then Esc stays; q then Enter really exits (code 0) ----
	s.sendKey([]byte("q"), keystrokeDelay)
	mustSee(t, s, "Quit gitid?", "quit: prompt opens")
	s.sendKey(dummyKeyEsc, keystrokeDelay*3)
	mustNotSee(t, s, "Quit gitid?", "quit: Esc stays")

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
	s.close(t) // idempotent for an already-exited process
	closed = true

	// ---- DLV-05 zero-writes: the sandbox HOME must be untouched ----
	var created []string
	walkErr := filepath.WalkDir(home, func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path != home {
			created = append(created, path)
		}
		return nil
	})
	if walkErr != nil {
		t.Fatalf("zero-writes walk: %v", walkErr)
	}
	if len(created) > 0 {
		t.Fatalf("DLV-05 violated — the demo created files under sandbox HOME:\n%s", strings.Join(created, "\n"))
	}
}
