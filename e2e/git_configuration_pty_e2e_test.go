//go:build e2e

package e2e

// git_configuration_pty_e2e_test.go — Real-PTY coverage for the STANDALONE
// Configure-Git pane reached from the identity detail view (04-04-PLAN.md
// Task 1, GITUI-01..05, DLV-06). Every test drives the REAL `gitid` binary
// (never gitid-dummy) via raw keystrokes/SGR mouse bytes over a pseudo-
// terminal at the design's minimum geometry (100x30). This file is the ONLY
// place the standalone "g" edit/complete flow (identities.go handleGitKey,
// paneGit -> paneGitCeremony) is exercised through the compiled binary — the
// combined create-wizard's demo'd Git step already has its own coverage in
// create_flow_pty_e2e_test.go.
//
// Covered, one test function per state group (04-04-PLAN.md Task 1):
//
//  1. TestGitConfiguration_RealPTYCompleteEditFlow — git-form-filled (D-08),
//     match-strategy-select (gitdir/hasconfig/both, D-01), review-readonly,
//     confirm-write, backup notice/receipt, result-success; proves the
//     signer principal is REPLACED (stale email removed), not appended.
//  2. TestGitConfiguration_RealPTYSSHOnlyCompletionFlow — git-form-empty
//     (invalid until email), fill, review, write, result-success; proves no
//     Git artifact exists before the confirm keystroke (GITUI-05).
//  3. TestGitConfiguration_RealPTYWriteFailureRollback — result-failure: a
//     symlinked managed fragment path (wiring.go containedRegularPath) is
//     the real, deterministic failure the compiled binary itself rejects; no
//     backdoor test hook is used. Proves the exact target/restoration copy
//     and a byte-identical prior HOME (T-04-11/T-04-12).
//  4. TestGitConfiguration_RealPTYMouseFieldFocus — raw SGR mouse clicks
//     focus name, email, the Force-SSH toggle, each strategy row, the gitdir
//     path field, and the ceremony's Write-it control.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// seedGitPTYIdentity extends seedMinimalIdentity (ui_pty_e2e_test.go) with an
// ~/.ssh/allowed_signers entry keyed to the SAME email the fragment already
// carries (<name>@example.com) — required so a later edit-to-a-new-email can
// prove the stale principal is replaced, not merely appended (GITUI-04).
func seedGitPTYIdentity(t *testing.T, home, name string) {
	t.Helper()
	seedMinimalIdentity(t, home, name)
	signersPath := filepath.Join(home, ".ssh", "allowed_signers")
	line := fmt.Sprintf(
		"# BEGIN gitid managed: %s\n%s@example.com namespaces=\"git\" ssh-ed25519 AAAAC3NzaC1lZDI1NTE5STUB %s@gitid-test\n# END gitid managed: %s\n",
		name, name, name, name,
	)
	if err := os.WriteFile(signersPath, []byte(line), 0o644); err != nil { //nolint:gosec // hermetic t.TempDir() sandbox HOME fixture (G306)
		t.Fatalf("seedGitPTYIdentity: WriteFile allowed_signers: %v", err)
	}
}

// removeGitSide strips the Git-configured side of a seedGitPTYIdentity
// fixture (gitconfig includeIf block, fragment file, allowed_signers) so the
// identity is SSH-only — openGitForm then solicits real author values into
// git-form-empty instead of prefilling from a fragment (GITUI-01).
func removeGitSide(t *testing.T, home, name string) {
	t.Helper()
	for _, path := range []string{
		filepath.Join(home, ".gitconfig"),
		filepath.Join(home, ".gitconfig.d", name),
		filepath.Join(home, ".ssh", "allowed_signers"),
	} {
		if err := os.Remove(path); err != nil {
			t.Fatalf("removeGitSide: removing %s: %v", path, err)
		}
	}
}

// openStandaloneGitForm boots the real binary against home, waits for the
// shell, and presses "g" from the identity detail pane (unconditional once
// an identity is selected — identities.go handleDetailKey case "g").
func openStandaloneGitForm(t *testing.T, s *ptySession) {
	t.Helper()
	uiReady(t, s)
	mustSee(t, s, "Identities", "sidebar renders with the seeded identity")
	s.sendKey([]byte("g"), keystrokeDelay)
}

// gitArtifactPaths are the three files a Configure-Git write ceremony
// touches (identities.go gitCeremonyFor Targets) — used to assert the
// "nothing written before confirm" (GITUI-05) and rollback (T-04-12)
// invariants directly against the sandbox filesystem.
func gitArtifactPaths(home, name string) []string {
	return []string{
		filepath.Join(home, ".gitconfig"),
		filepath.Join(home, ".gitconfig.d", name),
		filepath.Join(home, ".ssh", "allowed_signers"),
	}
}

// snapshotGitBytes reads every path that currently exists, keyed by path —
// missing files are simply absent from the map (never an error), so a
// before/after comparison naturally covers files created or removed too.
func snapshotGitBytes(t *testing.T, paths []string) map[string][]byte {
	t.Helper()
	out := make(map[string][]byte, len(paths))
	for _, p := range paths {
		b, err := os.ReadFile(p) //nolint:gosec // fixed set of sandbox HOME paths from gitArtifactPaths, not user input (G304)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			t.Fatalf("snapshotGitBytes: reading %s: %v", p, err)
		}
		out[p] = b
	}
	return out
}

func assertGitBytesUnchanged(t *testing.T, before, after map[string][]byte) {
	t.Helper()
	if len(before) != len(after) {
		t.Fatalf("HOME artifact set changed: before had %d files, after has %d (before=%v, after=%v)", len(before), len(after), keysOf(before), keysOf(after))
	}
	for p, want := range before {
		got, ok := after[p]
		if !ok {
			t.Fatalf("HOME artifact %s existed before a failed write and is now missing", p)
		}
		if string(got) != string(want) {
			t.Fatalf("HOME artifact %s changed byte content after a failed write:\nbefore:\n%s\nafter:\n%s", p, want, got)
		}
	}
}

func keysOf(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// ---------------------------------------------------------------------------
// 1. Complete edit flow: git-form-filled, match-strategy-select,
//    review-readonly, confirm-write, receipt, signer replacement.
// ---------------------------------------------------------------------------

func TestGitConfiguration_RealPTYCompleteEditFlow(t *testing.T) {
	home := SandboxHome(t)
	seedGitPTYIdentity(t, home, "acme")
	bin := BuildBinary(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	s := startPTYAt(t, newRealCreateFlowCmd(ctx, bin, home, ""), dummyTermWidth, dummyTermHeight)
	closed := false
	defer func() {
		if !closed {
			s.close(t)
		}
	}()

	openStandaloneGitForm(t, s)
	mustSee(t, s, "Git identity — acme (editing existing fragment)", "standalone edit opens the compiled real Git form: git-form-filled")
	mustSee(t, s, "acme User", "git-form-filled is prefilled from the seeded fragment")
	mustSee(t, s, "acme@example.com", "git-form-filled is prefilled from the seeded fragment")

	before := snapshotGitBytes(t, gitArtifactPaths(home, "acme"))

	// Edit the email FIRST (a single Tab from the initial name focus reaches
	// it directly) so the receipt can prove signer REPLACEMENT (GITUI-04).
	s.sendKey([]byte("\t"), keystrokeDelay) // name -> email
	for range "acme@example.com" {
		s.sendKey([]byte{0x7f}, keystrokeDelay) // backspace the seeded value
	}
	for _, r := range "acme-edited@example.com" {
		s.sendKey([]byte(string(r)), keystrokeDelay)
	}
	mustSee(t, s, "acme-edited@example.com", "edited email renders in git-form-filled")

	// match-strategy-select: cycle through all three strategies via ←/→ on
	// the strategy field (gitFieldStrategy, one more Tab from email).
	s.sendKey([]byte("\t"), keystrokeDelay) // email -> strategy
	mustSee(t, s, "● gitdir (default)", "gitdir is the seeded fixture's default strategy")
	s.sendKey([]byte{0x1b, 0x5b, 0x43}, keystrokeDelay) // right arrow
	mustSee(t, s, "● hasconfig", "match-strategy-select: right arrow advances to hasconfig")
	s.sendKey([]byte{0x1b, 0x5b, 0x43}, keystrokeDelay)
	mustSee(t, s, "● both", "match-strategy-select: right arrow advances to both")
	s.sendKey([]byte{0x1b, 0x5b, 0x44}, keystrokeDelay) // left arrow
	s.sendKey([]byte{0x1b, 0x5b, 0x44}, keystrokeDelay)
	mustSee(t, s, "● gitdir (default)", "match-strategy-select: left arrow returns to gitdir")
	mustSee(t, s, "acme-edited@example.com", "the email edit survives strategy cycling")

	// Nothing on disk yet — still just editing the form (GITUI-05).
	assertGitBytesUnchanged(t, before, snapshotGitBytes(t, gitArtifactPaths(home, "acme")))

	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, `Write Git identity for "acme"`, "review-readonly: valid form reaches the write ceremony")
	mustSee(t, s, "Nothing has changed yet", "review-readonly: the pre-confirm assurance appears before any write")
	mustSee(t, s, "Backup", "review-readonly: the backup promise/notice renders before confirm")

	assertGitBytesUnchanged(t, before, snapshotGitBytes(t, gitArtifactPaths(home, "acme")))

	s.sendKey(dummyKeyEnter, keystrokeDelay) // confirm-write
	mustSee(t, s, `Git identity "acme" configured`, "result-success: the receipt renders after confirm")
	mustSee(t, s, "Wrote →", "result-success: the receipt lists the written targets")
	mustSee(t, s, "Backed up →", "result-success: the receipt lists the backup path")

	fragment, err := os.ReadFile(filepath.Join(home, ".gitconfig.d", "acme"))
	if err != nil {
		t.Fatalf("reading updated fragment: %v", err)
	}
	if !strings.Contains(string(fragment), "acme-edited@example.com") {
		t.Fatalf("fragment missing the new email:\n%s", fragment)
	}

	signers, err := os.ReadFile(filepath.Join(home, ".ssh", "allowed_signers"))
	if err != nil {
		t.Fatalf("reading updated allowed_signers: %v", err)
	}
	if !strings.Contains(string(signers), `acme-edited@example.com namespaces="git"`) {
		t.Fatalf("allowed_signers missing the new principal:\n%s", signers)
	}
	if strings.Contains(string(signers), `acme@example.com namespaces="git"`) {
		t.Fatalf("allowed_signers retained the stale principal instead of replacing it:\n%s", signers)
	}

	saveFrame(t, "git-configuration-complete-edit-result", s)
}

// ---------------------------------------------------------------------------
// 2. SSH-only completion flow: git-form-empty, fill, review, write, success.
// ---------------------------------------------------------------------------

func TestGitConfiguration_RealPTYSSHOnlyCompletionFlow(t *testing.T) {
	home := SandboxHome(t)
	seedGitPTYIdentity(t, home, "work")
	removeGitSide(t, home, "work")
	bin := BuildBinary(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	s := startPTYAt(t, newRealCreateFlowCmd(ctx, bin, home, ""), dummyTermWidth, dummyTermHeight)
	closed := false
	defer func() {
		if !closed {
			s.close(t)
		}
	}()

	openStandaloneGitForm(t, s)
	mustSee(t, s, "Git identity — work (completes this identity)", "SSH-only identity enters the reusable empty Git form: git-form-empty")
	mustSee(t, s, "needs @", "git-form-empty is invalid until an email with @ is provided")

	before := snapshotGitBytes(t, gitArtifactPaths(home, "work"))
	for _, p := range gitArtifactPaths(home, "work") {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("Git artifact %s must not exist before the form is even filled: %v", p, err)
		}
	}

	for _, r := range "Work Identity" {
		s.sendKey([]byte(string(r)), keystrokeDelay)
	}
	s.sendKey([]byte("\t"), keystrokeDelay) // -> email
	for _, r := range "work@example.com" {
		s.sendKey([]byte(string(r)), keystrokeDelay)
	}
	mustSee(t, s, "work@example.com", "git-form-filled renders after raw keyboard entry into the empty form")
	mustNotSee(t, s, "needs @", "a valid email clears the inline validation error")

	assertGitBytesUnchanged(t, before, snapshotGitBytes(t, gitArtifactPaths(home, "work")))

	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, `Write Git identity for "work"`, "review-readonly: SSH-only completion reaches the write ceremony")

	for _, p := range gitArtifactPaths(home, "work") {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("Git artifact %s exists at review-readonly, before confirmation: %v", p, err)
		}
	}

	s.sendKey(dummyKeyEnter, keystrokeDelay) // confirm-write
	mustSee(t, s, `Git identity "work" configured`, "result-success: SSH-only completion reaches the receipt")

	for _, p := range gitArtifactPaths(home, "work") {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("Git artifact %s missing after a successful write: %v", p, err)
		}
	}
	signers, err := os.ReadFile(filepath.Join(home, ".ssh", "allowed_signers"))
	if err != nil {
		t.Fatalf("reading new allowed_signers: %v", err)
	}
	if !strings.Contains(string(signers), `work@example.com namespaces="git"`) {
		t.Fatalf("allowed_signers missing the new principal:\n%s", signers)
	}

	saveFrame(t, "git-configuration-ssh-only-completion-result", s)
}

// ---------------------------------------------------------------------------
// 3. Write failure + rollback: a real, deterministic rejection the compiled
//    binary itself enforces (wiring.go containedRegularPath), no test hook.
// ---------------------------------------------------------------------------

// TestGitConfiguration_RealPTYWriteFailureRollback seeds the managed fragment
// PATH ITSELF as a symlink before the binary starts. wiring.go's
// containedRegularPath (called from mutationJournal.watchFile) refuses any
// symlinked component of a managed target — this is the same safety check a
// real attacker-controlled or misconfigured HOME would trip, so it is a
// genuine end-to-end failure rather than a manufactured test-only hook
// (T-04-11/T-04-12).
func TestGitConfiguration_RealPTYWriteFailureRollback(t *testing.T) {
	home := SandboxHome(t)
	seedGitPTYIdentity(t, home, "acme")

	fragmentPath := filepath.Join(home, ".gitconfig.d", "acme")
	decoyTarget := filepath.Join(home, ".gitconfig.d", "acme-decoy-target")
	if err := os.WriteFile(decoyTarget, []byte("[user]\n\tname = Decoy\n"), 0o644); err != nil { //nolint:gosec // hermetic t.TempDir() sandbox HOME fixture (G306)
		t.Fatalf("seeding symlink decoy target: %v", err)
	}
	if err := os.Remove(fragmentPath); err != nil {
		t.Fatalf("removing seeded fragment before symlinking: %v", err)
	}
	if err := os.Symlink(decoyTarget, fragmentPath); err != nil {
		t.Fatalf("symlinking managed fragment path: %v", err)
	}

	bin := BuildBinary(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	s := startPTYAt(t, newRealCreateFlowCmd(ctx, bin, home, ""), dummyTermWidth, dummyTermHeight)
	closed := false
	defer func() {
		if !closed {
			s.close(t)
		}
	}()

	watched := []string{fragmentPath, decoyTarget, filepath.Join(home, ".gitconfig"), filepath.Join(home, ".ssh", "allowed_signers")}
	before := snapshotGitBytes(t, watched)

	openStandaloneGitForm(t, s)
	mustSee(t, s, "Git identity — acme (editing existing fragment)", "edit flow still opens; the symlink only blocks the WRITE, not the read-through preview")

	// The read-through preview follows the symlink to the decoy content
	// (name = "Decoy", no email) — fill a valid email so the form itself is
	// valid and Enter can reach the ceremony; the WRITE is what must fail.
	s.sendKey([]byte("\t"), keystrokeDelay) // name -> email
	for _, r := range "acme@example.com" {
		s.sendKey([]byte(string(r)), keystrokeDelay)
	}
	mustNotSee(t, s, "needs @", "a valid email clears the inline validation error")

	s.sendKey(dummyKeyEnter, keystrokeDelay) // -> review-readonly
	mustSee(t, s, `Write Git identity for "acme"`, "review-readonly reached before the write is attempted")

	// The symlink must still be in place right before confirm — the write
	// attempt itself is what wiring.go's containedRegularPath must reject.
	if info, err := os.Lstat(fragmentPath); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("symlink fixture lost before confirm (test setup bug, not the product): err=%v mode=%v", err, info)
	}

	s.sendKey(dummyKeyEnter, keystrokeDelay) // confirm-write -> attempts the real transaction
	mustSee(t, s, "✗", "result-failure: the ceremony renders the red failure glyph")
	mustSee(t, s, "refusing symlinked managed path", "result-failure: the exact rejection reason from wiring.go surfaces verbatim")
	mustSee(t, s, "Retry (Enter)", "result-failure: a failed write offers retry, never a silent dead end")

	assertGitBytesUnchanged(t, before, snapshotGitBytes(t, watched))
	if info, err := os.Lstat(fragmentPath); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("the symlink itself must survive the rejected transaction untouched: %v, mode=%v", err, info)
	}

	saveFrame(t, "git-configuration-write-failure-rollback", s)
}

// ---------------------------------------------------------------------------
// 4. Mouse field focus: raw SGR clicks on every Configure-Git control.
// ---------------------------------------------------------------------------

func TestGitConfiguration_RealPTYMouseFieldFocus(t *testing.T) {
	home := SandboxHome(t)
	seedGitPTYIdentity(t, home, "acme")
	bin := BuildBinary(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	s := startPTYAt(t, newRealCreateFlowCmd(ctx, bin, home, ""), dummyTermWidth, dummyTermHeight)
	defer s.close(t)

	openStandaloneGitForm(t, s)
	mustSee(t, s, "Git identity — acme (editing existing fragment)", "compiled real Git form open for mouse coverage")

	clickLabelRow(t, s, "user.name")
	for range "acme User" {
		s.sendKey([]byte{0x7f}, keystrokeDelay)
	}
	for _, r := range "Acme Clicked" {
		s.sendKey([]byte(string(r)), keystrokeDelay)
	}
	mustSee(t, s, "Acme Clicked", "mouse click focused user.name for raw keyboard entry")

	clickLabelRow(t, s, "user.email")
	for range "acme@example.com" {
		s.sendKey([]byte{0x7f}, keystrokeDelay)
	}
	for _, r := range "acme-clicked@example.com" {
		s.sendKey([]byte(string(r)), keystrokeDelay)
	}
	mustSee(t, s, "acme-clicked@example.com", "mouse click focused user.email for raw keyboard entry")

	clickLabelRow(t, s, "Force SSH")
	s.sendKey([]byte(" "), keystrokeDelay)
	mustSee(t, s, "☐ Force SSH", "mouse click focused the Force-SSH toggle; space toggled it off")

	clickLabelRow(t, s, "hasconfig — repos whose remote uses this alias")
	mustSee(t, s, "● hasconfig", "mouse click on a strategy row selects that strategy")

	clickLabelRow(t, s, "gitdir (default)")
	mustSee(t, s, "● gitdir (default)", "mouse click on the gitdir strategy row re-selects it")

	clickLabelRow(t, s, "gitdir path")
	for _, r := range "extra" {
		s.sendKey([]byte(string(r)), keystrokeDelay)
	}
	mustSee(t, s, "extra", "mouse click focused the gitdir path field for raw keyboard entry")

	clickLabelRow(t, s, "Write it")
	mustSee(t, s, `Write Git identity for "acme"`, "mouse click on Write it… reaches the write ceremony")

	clickLabelRow(t, s, "Write it")
	mustSee(t, s, `Git identity "acme" configured`, "mouse click on the ceremony's Write it (Enter) control confirms the write")

	saveFrame(t, "git-configuration-mouse-field-focus", s)
}
