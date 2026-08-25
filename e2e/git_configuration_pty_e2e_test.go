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
//  5. TestGitConfiguration_CompiledRealVsLiveDummyPTY (04-04-PLAN.md Task 2,
//     D-12) — pairs two REAL PTY sessions, one for the compiled `cmd/gitid`
//     binary (seeded via seedGitPTYIdentity/removeGitSide) and one for the
//     compiled `cmd/gitid-dummy` binary (its own frozen "personal"/"work"
//     fixture identities), and compares NORMALIZED semantic checkpoints —
//     field order, labels, controls, defaults, ceremony beats, and required
//     visible content — never raw terminal bytes and never any web/HTML/
//     MUI/Chromium/PNG artifact. Every unequal or one-sided region consumes
//     exactly one classification entry in
//     .planning/design/git-screen/visual-divergence-allowlist.txt.

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

	// CR-08/WR-29: assert the TRANSITION, not a state that also holds if the
	// click/space never did anything. seedGitPTYIdentity never writes a
	// provider-rewrite ([url "..."] insteadOf) block into ~/.gitconfig, so
	// (per CR-09) this identity's real ForceSSH state starts false — the
	// checkbox must render "☐ Force SSH" BEFORE any interaction. A bare
	// `mustSee(t, s, "☐ Force SSH", ...)` after the click+space would pass
	// whether or not the click and the space actually did anything, which is
	// exactly how the CR-08 regression (Force SSH unreachable in the pane)
	// sailed through this suite undetected. Assert the un-clicked baseline
	// first, then the flip to "☑", then flip back — proving both the click
	// (focus) and the space (toggle) are real.
	mustSee(t, s, "☐ Force SSH", "Force SSH starts unchecked — no provider-rewrite block seeded")
	clickLabelRow(t, s, "Force SSH")
	s.sendKey([]byte(" "), keystrokeDelay)
	mustSee(t, s, "☑ Force SSH", "mouse click focused the Force-SSH toggle; space toggled it on")
	clickLabelRow(t, s, "Force SSH")
	s.sendKey([]byte(" "), keystrokeDelay)
	mustSee(t, s, "☐ Force SSH", "a second click+space toggles the Force-SSH checkbox back off")

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
	mustSee(t, s, "Writing…", "mouse click on the ceremony's Write it control starts the asynchronous write")
	mustSee(t, s, `Git identity "acme" configured`, "asynchronous ceremony shows the successful write receipt")

	saveFrame(t, "git-configuration-mouse-field-focus", s)
}

// ---------------------------------------------------------------------------
// 5. Compiled real vs. live dummy PTY comparison (04-04-PLAN.md Task 2, D-12).
//
// Both cmd/gitid and cmd/gitid-dummy render Configure-Git through the SAME
// internal/tuikit identities.go code (the Phase 3 D-17 extraction): cmd/gitid
// injects a real Backend, cmd/gitid-dummy injects
// internal/dummytui.FixtureBackend. A structural divergence here therefore
// means either a genuine, classified fixture-vs-live-data difference (see
// .planning/design/git-screen/visual-divergence-allowlist.txt) or a real
// regression the gate must catch.
//
// The comparison is SEMANTIC, not byte-for-byte: each PTY frame (already
// ANSI-free plain text — github.com/charmbracelet/x/vt decodes the terminal
// and .String() returns the plain grid) is split into named regions, then
// normalized (identity token / email / gitdir path / timestamp placeholders)
// before comparing. No web/HTML/MUI/Chromium/PNG artifact participates.
// ---------------------------------------------------------------------------

// gitScreenRegion names a semantic sub-area of a Configure-Git PTY frame for
// the real-vs-dummy comparison.
type gitScreenRegion string

const (
	gitRegionSidebar      gitScreenRegion = "sidebar"
	gitRegionHeaderStatus gitScreenRegion = "header-status"
	gitRegionFormFields   gitScreenRegion = "git-form-fields"
	gitRegionStrategy     gitScreenRegion = "git-strategy"
	gitRegionPreview      gitScreenRegion = "git-preview"
	gitRegionCeremony     gitScreenRegion = "git-ceremony"
)

// allGitScreenRegions lists every region the comparison checks on every
// checkpoint. A region absent from BOTH sides is skipped (not applicable to
// that checkpoint's pane state) rather than treated as a divergence.
func allGitScreenRegions() []gitScreenRegion {
	return []gitScreenRegion{
		gitRegionSidebar, gitRegionHeaderStatus, gitRegionFormFields,
		gitRegionStrategy, gitRegionPreview, gitRegionCeremony,
	}
}

// newDummyCmd builds the exec.Cmd for a Configure-Git PTY test against the
// LIVE cmd/gitid-dummy binary — the SAME internal/tuikit render stack the
// real binary uses, injected with dummytui.FixtureBackend instead of a real
// Backend (D-12).
func newDummyCmd(ctx context.Context, bin, home string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, bin) //nolint:gosec // bin from BuildDummyBinary; no user input
	cmd.Env = append(os.Environ(), "HOME="+home, "TERM=xterm-256color")
	return cmd
}

// openDummyGitFormEditMode opens Configure Git for the dummy's default-
// selected "personal" fixture identity (internal/dummytui/data.go
// IdentityManagerRows[0], state "complete") — the dummy-side equivalent of
// the real binary's seeded complete identity (git-form-filled, edit mode).
func openDummyGitFormEditMode(t *testing.T, s *ptySession) {
	t.Helper()
	mustSee(t, s, "[1] Identities", "dummy: launches on the Identities tab")
	mustSee(t, s, "personal", "dummy: seeded fixture sidebar row")
	s.sendKey([]byte("g"), keystrokeDelay)
}

// openDummyGitFormSSHOnly selects the dummy's "work" fixture identity (state
// "incomplete" — SSH host present, no Git side configured yet) and opens
// Configure Git — the dummy-side equivalent of the real binary's SSH-only
// completion entry (git-form-empty).
func openDummyGitFormSSHOnly(t *testing.T, s *ptySession) {
	t.Helper()
	mustSee(t, s, "[1] Identities", "dummy: launches on the Identities tab")
	s.sendKey(dummyKeyDown, keystrokeDelay) // personal -> work
	mustSee(t, s, "! incomplete", "dummy: work fixture selected")
	s.sendKey([]byte("g"), keystrokeDelay)
}

var (
	gitScreenEmailPattern     = regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
	gitScreenTimestampPattern = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}[:\-]\d{2}[:\-]\d{2}Z`)
	gitScreenGitDirPattern    = regexp.MustCompile(`~/git/[A-Za-z0-9_-]+/`)
	// gitScreenIdentityTokens is the fixed, known vocabulary of identity
	// names this suite's real (seedGitPTYIdentity) and dummy
	// (internal/dummytui/data.go IdentityManagerRows) fixtures use —
	// deliberately narrow (not a generic word-matcher), since both sides
	// are controlled test fixtures, never arbitrary user data.
	gitScreenIdentityTokens = []string{"acme", "personal", "work"}
)

// normalizeGitCheckpoint replaces identity-specific, backend-specific, and
// wall-clock-specific substrings with stable placeholders so a REAL and a
// DUMMY capture of the SAME semantic state compare structurally rather than
// byte-for-byte — the checkpoint script drives different fixture identities
// on each side (D-12: field order/labels/controls/defaults are the
// comparison target, not literal fixture values).
func normalizeGitCheckpoint(s string) string {
	s = gitScreenTimestampPattern.ReplaceAllString(s, "<timestamp>")
	s = gitScreenEmailPattern.ReplaceAllString(s, "<email>")
	s = gitScreenGitDirPattern.ReplaceAllString(s, "<gitdir>")
	for _, tok := range gitScreenIdentityTokens {
		s = strings.ReplaceAll(s, tok, "<identity>")
	}
	return s
}

// gitScreenRightOfDivider returns the text to the right of the identities
// pane's master/detail "│" separator. The vt-decoded PTY frame is already
// ANSI-free plain text, so no ANSI stripping is needed here (unlike the
// in-process, ANSI-preserving captures in internal/screenshot).
func gitScreenRightOfDivider(line string) string {
	idx := strings.Index(line, "│")
	if idx < 0 {
		return line
	}
	return line[idx+len("│"):]
}

// extractGitScreenSidebar returns the master-pane identity list (everything
// left of the "│" divider on lines that carry sidebar content).
func extractGitScreenSidebar(lines []string) string {
	var out []string
	for _, line := range lines {
		idx := strings.Index(line, "│")
		if idx > 0 && strings.TrimSpace(line[:idx]) != "" {
			out = append(out, strings.TrimSpace(line[:idx]))
		}
	}
	return strings.Join(out, "\n")
}

// extractGitScreenHeaderStatus returns the header line's trailing status
// chip ("N ids · <health>"), the portion after the last nav-tab label.
func extractGitScreenHeaderStatus(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	header := lines[0]
	idx := strings.LastIndex(header, "Doctor")
	if idx < 0 {
		return ""
	}
	return strings.TrimSpace(header[idx+len("Doctor"):])
}

// extractGitScreenFormFields returns the git-form's user.name/user.email
// rows plus the compact gpg.format/signingkey/gpgsign/Force-SSH metadata
// line — from "user.name" up to (not including) the "Match strategy" row.
func extractGitScreenFormFields(lines []string) string {
	var out []string
	capturing := false
	for _, line := range lines {
		rp := gitScreenRightOfDivider(line)
		if strings.Contains(rp, "Match strategy") {
			break
		}
		if strings.Contains(rp, "user.name") {
			capturing = true
		}
		if capturing {
			out = append(out, strings.TrimRight(rp, " "))
		}
	}
	return strings.Join(out, "\n")
}

// extractGitScreenStrategy returns the match-strategy header, its three
// always-rendered option rows, and the hint line — from "Match strategy" up
// to (not including) the fragment preview block.
func extractGitScreenStrategy(lines []string) string {
	var out []string
	capturing := false
	for _, line := range lines {
		rp := gitScreenRightOfDivider(line)
		if strings.Contains(rp, "fragment file") {
			break
		}
		if strings.Contains(rp, "Match strategy") {
			capturing = true
		}
		if capturing {
			out = append(out, strings.TrimRight(rp, " "))
		}
	}
	return strings.Join(out, "\n")
}

// extractGitScreenPreview returns the fragment-file and includeIf-block
// preview boxes (and the conditional gitdir-path row, when present) — from
// the first preview title up to the "Write it" button row.
func extractGitScreenPreview(lines []string) string {
	var out []string
	capturing := false
	for _, line := range lines {
		rp := gitScreenRightOfDivider(line)
		if strings.Contains(rp, "Write it") {
			break
		}
		if strings.Contains(rp, "fragment file") || strings.Contains(rp, "includeIf block") {
			capturing = true
		}
		if capturing {
			out = append(out, strings.TrimRight(rp, " "))
		}
	}
	return strings.Join(out, "\n")
}

// extractGitScreenCeremony returns the write-ceremony pane's content — from
// its heading ("Write Git identity for …") or its receipt heading (the
// "… configured" result message) through the end of the frame. Ceremony
// checkpoints never render the git-form fields, so this never collides with
// extractGitScreenFormFields/Strategy/Preview.
func extractGitScreenCeremony(lines []string) string {
	start := -1
	for i, line := range lines {
		rp := gitScreenRightOfDivider(line)
		if strings.Contains(rp, "Write Git identity") || strings.Contains(rp, "configured") {
			start = i
			break
		}
	}
	if start < 0 {
		return ""
	}
	var out []string
	for _, line := range lines[start:] {
		out = append(out, strings.TrimRight(gitScreenRightOfDivider(line), " "))
	}
	return strings.Join(out, "\n")
}

// extractGitScreenRegion dispatches to the named region's extractor.
func extractGitScreenRegion(frame string, region gitScreenRegion) string {
	lines := strings.Split(frame, "\n")
	switch region {
	case gitRegionSidebar:
		return extractGitScreenSidebar(lines)
	case gitRegionHeaderStatus:
		return extractGitScreenHeaderStatus(lines)
	case gitRegionFormFields:
		return extractGitScreenFormFields(lines)
	case gitRegionStrategy:
		return extractGitScreenStrategy(lines)
	case gitRegionPreview:
		return extractGitScreenPreview(lines)
	case gitRegionCeremony:
		return extractGitScreenCeremony(lines)
	}
	return ""
}

// gitScreenAllowlistEntry is one parsed, strict-schema
// (checkpoint:region:predicate:decision-ref:reason) divergence
// classification — the SAME 5-field format
// cmd/gitid/gate_visual_regression_test.go's create-flow allowlist uses,
// scoped here to git-screen checkpoints and CTX-D-NN/UI-D-NN decision refs
// (04-CONTEXT.md/04-UI-SPEC.md).
type gitScreenAllowlistEntry struct {
	Checkpoint  string
	Region      gitScreenRegion
	Predicate   string
	DecisionRef string
	Reason      string
	used        bool
}

// splitGitScreenAllowlistLine splits one allowlist line into exactly 5
// fields, handling the predicate field's own embedded colon
// ("contains:<text>"/"absent:<text>") the same way
// cmd/gitid/gate_visual_regression_test.go's splitAllowlistLine does for the
// create-flow allowlist — a naive 5-way colon split misaligns fields because
// the predicate itself contains a colon.
func splitGitScreenAllowlistLine(s string) []string {
	cut := func(r string) (field, rest string, ok bool) {
		idx := strings.Index(r, ":")
		if idx < 0 {
			return "", r, false
		}
		return r[:idx], r[idx+1:], true
	}
	f0, rest, ok := cut(s)
	if !ok {
		return nil
	}
	f1, rest, ok := cut(rest)
	if !ok {
		return nil
	}
	var f2, f3, f4 string
	switch {
	case strings.HasPrefix(rest, "contains:"), strings.HasPrefix(rest, "absent:"):
		keyword := "contains:"
		if strings.HasPrefix(rest, "absent:") {
			keyword = "absent:"
		}
		inner := rest[len(keyword):]
		if strings.HasPrefix(inner, `"`) {
			closeQ := strings.Index(inner[1:], `"`)
			if closeQ < 0 {
				idx := strings.Index(inner, ":")
				if idx < 0 {
					return nil
				}
				f2 = keyword + inner[:idx]
				rest = inner[idx+1:]
			} else {
				quoted := inner[:closeQ+2]
				f2 = keyword + quoted
				rest = inner[closeQ+2:]
				rest = strings.TrimPrefix(rest, ":")
			}
		} else {
			idx := strings.Index(inner, ":")
			if idx < 0 {
				return nil
			}
			f2 = keyword + inner[:idx]
			rest = inner[idx+1:]
		}
	default:
		var ok2 bool
		f2, rest, ok2 = cut(rest)
		if !ok2 {
			return nil
		}
	}
	f3, f4, ok = cut(rest)
	if !ok {
		return nil
	}
	return []string{f0, f1, f2, f3, f4}
}

// loadGitScreenAllowlist parses
// .planning/design/git-screen/visual-divergence-allowlist.txt.
func loadGitScreenAllowlist(t *testing.T) []*gitScreenAllowlistEntry {
	t.Helper()
	path := filepath.Join(repoRoot(t), ".planning", "design", "git-screen", "visual-divergence-allowlist.txt")
	data, err := os.ReadFile(path) //nolint:gosec // fixed repo-relative path (G304)
	if err != nil {
		t.Fatalf("loadGitScreenAllowlist: reading %s: %v", path, err)
	}
	validCheckpoints := map[string]bool{
		"git-form-filled": true, "git-form-empty": true, "match-strategy-select": true,
		"review-readonly": true, "result-success": true,
	}
	validRegions := make(map[gitScreenRegion]bool, len(allGitScreenRegions()))
	for _, r := range allGitScreenRegions() {
		validRegions[r] = true
	}
	seen := make(map[string]bool)
	var entries []*gitScreenAllowlistEntry
	for lineNum, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := splitGitScreenAllowlistLine(line)
		if len(parts) != 5 {
			t.Fatalf("git-screen allowlist line %d: expected 5 colon-separated fields (checkpoint:region:predicate:decision-ref:reason), got %d in: %q", lineNum+1, len(parts), line)
		}
		checkpoint := strings.TrimSpace(parts[0])
		region := gitScreenRegion(strings.TrimSpace(parts[1]))
		predicate := strings.TrimSpace(parts[2])
		decisionRef := strings.TrimSpace(parts[3])
		reason := strings.TrimSpace(parts[4])
		if !validCheckpoints[checkpoint] {
			t.Fatalf("git-screen allowlist line %d: unknown checkpoint %q", lineNum+1, checkpoint)
		}
		if !validRegions[region] {
			t.Fatalf("git-screen allowlist line %d: unknown region %q", lineNum+1, region)
		}
		if predicate == "differs" {
			t.Fatalf("git-screen allowlist line %d: forbidden predicate %q — use contains:<text> or absent:<text> (CR-04 precedent)", lineNum+1, predicate)
		}
		if !strings.HasPrefix(predicate, "contains:") && !strings.HasPrefix(predicate, "absent:") {
			t.Fatalf("git-screen allowlist line %d: invalid predicate %q (must be 'contains:<text>' or 'absent:<text>')", lineNum+1, predicate)
		}
		if !strings.HasPrefix(decisionRef, "CTX-D-") && !strings.HasPrefix(decisionRef, "UI-D-") {
			t.Fatalf("git-screen allowlist line %d: decision-ref %q must be a scoped CTX-D-NN or UI-D-NN identifier", lineNum+1, decisionRef)
		}
		if reason == "" {
			t.Fatalf("git-screen allowlist line %d: blank reason", lineNum+1)
		}
		key := checkpoint + ":" + string(region)
		if seen[key] {
			t.Fatalf("git-screen allowlist line %d: duplicate entry for checkpoint %q region %q", lineNum+1, checkpoint, region)
		}
		seen[key] = true
		entries = append(entries, &gitScreenAllowlistEntry{
			Checkpoint: checkpoint, Region: region, Predicate: predicate, DecisionRef: decisionRef, Reason: reason,
		})
	}
	return entries
}

// gitScreenPredicateSatisfied reports whether entry's predicate is satisfied
// by the pair (real, dummy) — the SAME contains:/absent: schema
// cmd/gitid/gate_visual_regression_test.go's create-flow gate uses (CR-04:
// "differs" is never a valid predicate; every divergence must name specific
// text).
//
// CR-10 (iteration 4): fixed the identical vacuous-accept bug
// internal/screenshot/createflow.go's regionPredicateSatisfied had — the
// prior "hold on EITHER side" grammar (`!strings.Contains(real, needle) ||
// !strings.Contains(dummy, needle)`) was permanently true whenever one side
// structurally never carries the needle, regardless of what the OTHER
// (real) side rendered. contains:X now requires the marker on BOTH sides;
// absent:X now requires the presence/absence ASYMMETRY itself (exactly one
// side carries X) — see the sibling function's comment for the full
// rationale. Keep both in sync.
func gitScreenPredicateSatisfied(entry *gitScreenAllowlistEntry, real, dummy string) bool {
	switch {
	case strings.HasPrefix(entry.Predicate, "contains:"):
		needle := strings.Trim(strings.TrimPrefix(entry.Predicate, "contains:"), `"`)
		return strings.Contains(real, needle) && strings.Contains(dummy, needle)
	case strings.HasPrefix(entry.Predicate, "absent:"):
		needle := strings.Trim(strings.TrimPrefix(entry.Predicate, "absent:"), `"`)
		return strings.Contains(real, needle) != strings.Contains(dummy, needle)
	}
	return false
}

// compareGitScreenCheckpoint compares every region of one semantic
// checkpoint between a real-binary frame and a live-dummy-binary frame.
// Regions that are structurally identical after normalizeGitCheckpoint
// require no allowlist entry (D-12: field order/labels/controls/defaults
// match). Every remaining unequal or one-sided region must consume EXACTLY
// ONE allowlist entry; an unmatched divergence, or an entry whose predicate
// does not actually hold, both fail.
func compareGitScreenCheckpoint(t *testing.T, checkpoint string, realFrame, dummyFrame string, allowlist []*gitScreenAllowlistEntry) {
	t.Helper()
	comparable := 0
	for _, region := range allGitScreenRegions() {
		realRegion := extractGitScreenRegion(realFrame, region)
		dummyRegion := extractGitScreenRegion(dummyFrame, region)
		if strings.TrimSpace(realRegion) == "" && strings.TrimSpace(dummyRegion) == "" {
			continue // region not applicable to this checkpoint's pane on either side
		}
		comparable++
		if normalizeGitCheckpoint(realRegion) == normalizeGitCheckpoint(dummyRegion) {
			continue // structurally identical — no divergence to classify
		}
		var matched *gitScreenAllowlistEntry
		for _, entry := range allowlist {
			if entry.Checkpoint == checkpoint && entry.Region == region {
				matched = entry
				break
			}
		}
		if matched == nil {
			t.Errorf("git-screen semantic gate: %s/%s diverges with NO allowlist classification (D-12 requires ux-improvement or defect for every difference)\n--- real ---\n%s\n--- dummy ---\n%s",
				checkpoint, region, realRegion, dummyRegion)
			continue
		}
		if !gitScreenPredicateSatisfied(matched, realRegion, dummyRegion) {
			t.Errorf("git-screen semantic gate: %s/%s allowlist entry %q does not match the observed divergence\n--- real ---\n%s\n--- dummy ---\n%s",
				checkpoint, region, matched.Predicate, realRegion, dummyRegion)
			continue
		}
		matched.used = true
	}
	if comparable == 0 {
		t.Fatalf("git-screen semantic gate: checkpoint %q produced NO comparable region on either side — checkpoint script bug, not a real absence", checkpoint)
	}
}

// TestGitConfiguration_CompiledRealVsLiveDummyPTY drives the SAME
// Configure-Git checkpoints through two real PTY sessions — the compiled
// cmd/gitid binary and the compiled cmd/gitid-dummy binary — and compares
// NORMALIZED semantic checkpoints (field order, labels, controls, defaults,
// navigation/state order, ceremony beats, required visible content), never
// raw terminal bytes and never any web/HTML/MUI/Chromium/PNG artifact
// (D-12). Every unequal or one-sided region on every checkpoint consumes
// EXACTLY ONE classification entry in
// .planning/design/git-screen/visual-divergence-allowlist.txt, citing a
// scoped CTX-D-NN (04-CONTEXT.md) or UI-D-NN (04-UI-SPEC.md) decision.
func TestGitConfiguration_CompiledRealVsLiveDummyPTY(t *testing.T) {
	allowlist := loadGitScreenAllowlist(t)
	realBin := BuildBinary(t)
	dummyBin := BuildDummyBinary(t)

	t.Run("git-form-filled", func(t *testing.T) {
		realHome := SandboxHome(t)
		seedGitPTYIdentity(t, realHome, "acme")
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		real := startPTYAt(t, newRealCreateFlowCmd(ctx, realBin, realHome, ""), dummyTermWidth, dummyTermHeight)
		defer real.close(t)
		openStandaloneGitForm(t, real)
		mustSee(t, real, "editing existing fragment", "real: edit-mode Configure-Git opens")

		dummyHome := SandboxHome(t)
		dctx, dcancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer dcancel()
		dummy := startPTYAt(t, newDummyCmd(dctx, dummyBin, dummyHome), dummyTermWidth, dummyTermHeight)
		defer dummy.close(t)
		openDummyGitFormEditMode(t, dummy)
		mustSee(t, dummy, "editing existing fragment", "dummy: edit-mode Configure-Git opens")

		compareGitScreenCheckpoint(t, "git-form-filled", real.snapshot(), dummy.snapshot(), allowlist)
	})

	t.Run("git-form-empty", func(t *testing.T) {
		realHome := SandboxHome(t)
		seedGitPTYIdentity(t, realHome, "work")
		removeGitSide(t, realHome, "work")
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		real := startPTYAt(t, newRealCreateFlowCmd(ctx, realBin, realHome, ""), dummyTermWidth, dummyTermHeight)
		defer real.close(t)
		openStandaloneGitForm(t, real)
		mustSee(t, real, "completes this identity", "real: SSH-only Configure-Git opens")

		dummyHome := SandboxHome(t)
		dctx, dcancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer dcancel()
		dummy := startPTYAt(t, newDummyCmd(dctx, dummyBin, dummyHome), dummyTermWidth, dummyTermHeight)
		defer dummy.close(t)
		openDummyGitFormSSHOnly(t, dummy)
		mustSee(t, dummy, "completes this identity", "dummy: SSH-only Configure-Git opens")

		compareGitScreenCheckpoint(t, "git-form-empty", real.snapshot(), dummy.snapshot(), allowlist)
	})

	t.Run("match-strategy-select", func(t *testing.T) {
		realHome := SandboxHome(t)
		seedGitPTYIdentity(t, realHome, "acme")
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		real := startPTYAt(t, newRealCreateFlowCmd(ctx, realBin, realHome, ""), dummyTermWidth, dummyTermHeight)
		defer real.close(t)
		openStandaloneGitForm(t, real)
		mustSee(t, real, "editing existing fragment", "real: Configure-Git opens")
		real.sendKey([]byte("\t"), keystrokeDelay) // name -> email
		real.sendKey([]byte("\t"), keystrokeDelay) // email -> strategy
		mustSee(t, real, "● gitdir (default)", "real: match strategy focused at its default")

		dummyHome := SandboxHome(t)
		dctx, dcancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer dcancel()
		dummy := startPTYAt(t, newDummyCmd(dctx, dummyBin, dummyHome), dummyTermWidth, dummyTermHeight)
		defer dummy.close(t)
		openDummyGitFormEditMode(t, dummy)
		mustSee(t, dummy, "editing existing fragment", "dummy: Configure-Git opens")
		dummy.sendKey([]byte("\t"), keystrokeDelay)
		dummy.sendKey([]byte("\t"), keystrokeDelay)
		mustSee(t, dummy, "● gitdir (default)", "dummy: match strategy focused at its default")

		compareGitScreenCheckpoint(t, "match-strategy-select", real.snapshot(), dummy.snapshot(), allowlist)
	})

	t.Run("review-readonly", func(t *testing.T) {
		realHome := SandboxHome(t)
		seedGitPTYIdentity(t, realHome, "acme")
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		real := startPTYAt(t, newRealCreateFlowCmd(ctx, realBin, realHome, ""), dummyTermWidth, dummyTermHeight)
		defer real.close(t)
		openStandaloneGitForm(t, real)
		mustSee(t, real, "editing existing fragment", "real: Configure-Git opens")
		real.sendKey(dummyKeyEnter, keystrokeDelay)
		mustSee(t, real, `Write Git identity for "acme"`, "real: review-readonly reached")

		dummyHome := SandboxHome(t)
		dctx, dcancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer dcancel()
		dummy := startPTYAt(t, newDummyCmd(dctx, dummyBin, dummyHome), dummyTermWidth, dummyTermHeight)
		defer dummy.close(t)
		openDummyGitFormEditMode(t, dummy)
		mustSee(t, dummy, "editing existing fragment", "dummy: Configure-Git opens")
		dummy.sendKey(dummyKeyEnter, keystrokeDelay)
		mustSee(t, dummy, `Write Git identity for "personal"`, "dummy: review-readonly reached")

		compareGitScreenCheckpoint(t, "review-readonly", real.snapshot(), dummy.snapshot(), allowlist)
	})

	t.Run("result-success", func(t *testing.T) {
		realHome := SandboxHome(t)
		seedGitPTYIdentity(t, realHome, "acme")
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		real := startPTYAt(t, newRealCreateFlowCmd(ctx, realBin, realHome, ""), dummyTermWidth, dummyTermHeight)
		defer real.close(t)
		openStandaloneGitForm(t, real)
		mustSee(t, real, "editing existing fragment", "real: Configure-Git opens")
		real.sendKey(dummyKeyEnter, keystrokeDelay)
		mustSee(t, real, `Write Git identity for "acme"`, "real: review-readonly reached")
		real.sendKey(dummyKeyEnter, keystrokeDelay)
		mustSee(t, real, `Git identity "acme" configured`, "real: result-success reached")

		dummyHome := SandboxHome(t)
		dctx, dcancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer dcancel()
		dummy := startPTYAt(t, newDummyCmd(dctx, dummyBin, dummyHome), dummyTermWidth, dummyTermHeight)
		defer dummy.close(t)
		openDummyGitFormEditMode(t, dummy)
		mustSee(t, dummy, "editing existing fragment", "dummy: Configure-Git opens")
		dummy.sendKey(dummyKeyEnter, keystrokeDelay)
		mustSee(t, dummy, `Write Git identity for "personal"`, "dummy: review-readonly reached")
		dummy.sendKey(dummyKeyEnter, keystrokeDelay)
		mustSee(t, dummy, `Git identity "personal" configured`, "dummy: result-success reached")

		compareGitScreenCheckpoint(t, "result-success", real.snapshot(), dummy.snapshot(), allowlist)
	})

	for _, entry := range allowlist {
		if !entry.used {
			t.Errorf("git-screen allowlist entry %s/%s (%s) was never triggered by any checkpoint comparison — remove the stale entry", entry.Checkpoint, entry.Region, entry.DecisionRef)
		}
	}
}
