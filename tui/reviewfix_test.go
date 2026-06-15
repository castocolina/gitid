package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
	"github.com/castocolina/gitid/internal/tester"
)

// --- FIX-1: TUI update must PRESERVE existing signing state, not infer it from
// the presence of an email. ---

// writeFragment writes a real per-identity fragment via `git config --file` so
// gitconfig.ReadFragment observes it exactly as production does. signing=true
// sets gpg.format=ssh (the marker ReadFragment uses for "signing on").
func writeFragment(t *testing.T, fragPath, name, email string, signing bool) {
	t.Helper()
	set := func(key, val string) {
		cmd := exec.Command("git", "config", "--file", fragPath, key, val) //nolint:gosec // test-only; fragPath/key/val are fixed in-test values, no shell (G204)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git config --file %s %s %s: %v\n%s", fragPath, key, val, err, out)
		}
	}
	if name != "" {
		set("user.name", name)
	}
	if email != "" {
		set("user.email", email)
	}
	if signing {
		set("gpg.format", "ssh")
		set("user.signingkey", fragPath+".pub")
		set("commit.gpgsign", "true")
	}
}

// updateCapture records the signing decision and whether a structural re-test
// ran during a TUI update write, observed via the update deps that runWriteCmd
// ultimately invokes on identity.Update.
type updateCapture struct {
	ran        bool
	signing    bool
	structural bool
}

func captureUpdateDeps(capt *updateCapture) tuiDeps {
	d := recordingTUIDeps(&capt.ran)
	// WriteFragment receives the final signing bool — the single most direct
	// observation of the signing decision that flows into identity.Update.
	d.update.WriteFragment = func(_, _, _, _ string, signing bool) error {
		capt.signing = signing
		return nil
	}
	// Resolved only runs when identity.Update detects a structural change
	// (alias/hostname/port differ between existing and edited), so observing it
	// proves a DISTINCT original vs edited was threaded through (FIX-2).
	d.update.Resolved = func(_ string) (tester.Result, tester.ResolvedConfig) {
		capt.structural = true
		return tester.Result{Outcome: tester.PASS}, tester.ResolvedConfig{}
	}
	return d
}

// TestTUIUpdatePreservesSigningOff asserts a TUI update of an identity whose
// fragment has signing OFF does NOT enable signing even when GitEmail is
// non-empty (the old email-presence heuristic would have turned it ON).
func TestTUIUpdatePreservesSigningOff(t *testing.T) {
	home := t.TempDir()
	fragDir := filepath.Join(home, ".gitconfig.d")
	if err := os.MkdirAll(fragDir, 0o750); err != nil {
		t.Fatal(err)
	}
	fragPath := filepath.Join(fragDir, "work")
	writeFragment(t, fragPath, "Work Dev", "work@example.com", false) // signing OFF

	acct := identity.Account{
		Name:         "work",
		GitName:      "Work Dev",
		GitEmail:     "work@example.com", // non-empty: old heuristic would flip signing ON
		Provider:     "github.com",
		Alias:        "work.github.com",
		Hostname:     "github.com",
		Port:         22,
		KeyPath:      "~/.ssh/gitid_work",
		PubPath:      "~/.ssh/gitid_work.pub",
		FragmentPath: fragPath,
	}

	var capt updateCapture
	deps := captureUpdateDeps(&capt)
	form := newUpdateFormModel(acct, deps)

	// Submit unchanged (no structural edit): the prove screen is pushed with the
	// preserved signing state.
	form.focusIdx = len(form.inputs) - 1
	screen, cmd := form.update(tea.KeyPressMsg{Code: tea.KeyEnter})
	_ = screen
	prove := mustPushTarget(t, cmd).(proveModel)

	// Drive to confirm and execute the write.
	driveProveToWrite(t, prove)

	if capt.signing {
		t.Fatal("FIX-1: TUI update must PRESERVE signing OFF; got signing enabled (email-inference regression)")
	}
}

// TestTUIUpdatePreservesSigningOn asserts a TUI update of an identity whose
// fragment has signing ON keeps signing ON.
func TestTUIUpdatePreservesSigningOn(t *testing.T) {
	home := t.TempDir()
	fragDir := filepath.Join(home, ".gitconfig.d")
	if err := os.MkdirAll(fragDir, 0o750); err != nil {
		t.Fatal(err)
	}
	fragPath := filepath.Join(fragDir, "work")
	writeFragment(t, fragPath, "Work Dev", "work@example.com", true) // signing ON

	acct := identity.Account{
		Name:         "work",
		GitName:      "Work Dev",
		GitEmail:     "work@example.com",
		Provider:     "github.com",
		Alias:        "work.github.com",
		Hostname:     "github.com",
		Port:         22,
		KeyPath:      "~/.ssh/gitid_work",
		PubPath:      "~/.ssh/gitid_work.pub",
		FragmentPath: fragPath,
	}

	var capt updateCapture
	deps := captureUpdateDeps(&capt)
	form := newUpdateFormModel(acct, deps)
	form.focusIdx = len(form.inputs) - 1
	_, cmd := form.update(tea.KeyPressMsg{Code: tea.KeyEnter})
	prove := mustPushTarget(t, cmd).(proveModel)
	driveProveToWrite(t, prove)

	if !capt.signing {
		t.Fatal("FIX-1: TUI update must PRESERVE signing ON; got signing disabled")
	}
}

// --- FIX-2: a structural edit (alias/hostname/port) must trigger the resolved
// re-test, which requires a DISTINCT original vs edited threaded to
// identity.Update. ---

// TestTUIUpdateStructuralChangeTriggersRetest changes the SSH alias in the
// update form and asserts identity.Update detects a structural change (runs the
// resolved re-test). With the old code (existing == edited) structural is always
// false and the re-test is dead.
func TestTUIUpdateStructuralChangeTriggersRetest(t *testing.T) {
	home := t.TempDir()
	fragDir := filepath.Join(home, ".gitconfig.d")
	if err := os.MkdirAll(fragDir, 0o750); err != nil {
		t.Fatal(err)
	}
	fragPath := filepath.Join(fragDir, "work")
	writeFragment(t, fragPath, "Work Dev", "work@example.com", false)

	acct := identity.Account{
		Name:         "work",
		GitName:      "Work Dev",
		GitEmail:     "work@example.com",
		Provider:     "github.com",
		Alias:        "work.github.com",
		Hostname:     "github.com",
		Port:         22,
		KeyPath:      "~/.ssh/gitid_work",
		PubPath:      "~/.ssh/gitid_work.pub",
		FragmentPath: fragPath,
	}

	var capt updateCapture
	deps := captureUpdateDeps(&capt)
	form := newUpdateFormModel(acct, deps)

	// Change the SSH Alias field (index 4) — a structural field.
	form.inputs[4].SetValue("work-renamed.github.com")
	form.focusIdx = len(form.inputs) - 1
	_, cmd := form.update(tea.KeyPressMsg{Code: tea.KeyEnter})
	prove := mustPushTarget(t, cmd).(proveModel)

	// The prove screen must carry an original distinct from the edited account.
	if prove.original.Alias == prove.account.Alias {
		t.Fatalf("FIX-2: prove screen must carry DISTINCT original vs edited alias; both are %q", prove.account.Alias)
	}

	driveProveToWrite(t, prove)

	if !capt.structural {
		t.Fatal("FIX-2: a changed alias must trigger identity.Update's structural re-test (Resolved); it never ran")
	}
}

// driveProveToWrite advances a freshly-pushed prove screen through phase 1 and
// phase 2 (both passing) and executes the confirmed write cmd.
func driveProveToWrite(t *testing.T, prove proveModel) {
	t.Helper()
	updated, _ := prove.update(preWriteResultMsg{result: tester.Result{Outcome: tester.PASS}})
	pm := updated.(proveModel)
	updated2, _ := pm.update(resolvedResultMsg{result: tester.Result{Outcome: tester.PASS}, resolved: tester.ResolvedConfig{}})
	pm2 := updated2.(proveModel)
	if !pm2.confirmActive {
		t.Fatal("prove screen must reach confirm-active after both phases pass")
	}
	_, writeCmd := pm2.update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if writeCmd == nil {
		t.Fatal("confirm must dispatch a write cmd")
	}
	if _, ok := writeCmd().(writeResultMsg); !ok {
		t.Fatal("write cmd must produce a writeResultMsg")
	}
}

// --- FIX-3: create-mode phase-1 status must NOT claim authentication of a
// not-yet-generated key. ---

// drivePhase1Pass returns a prove screen for the given action after phase 1 has
// passed, so the phase-1 status line is rendered.
func provePhase1PassedView(t *testing.T, action string) string {
	t.Helper()
	deps := fakeWriteTUIDeps(nil)
	in := makeTestInput()
	m := newProveScreen(action, in, identity.Account{}, "~/.ssh/gitid_personal", deps)
	updated, _ := m.update(preWriteResultMsg{result: makePassResult()})
	pm := updated.(proveModel)
	return pm.view()
}

// TestCreatePhase1StatusDoesNotClaimAuthenticated asserts the create-mode
// phase-1 status line does NOT assert "authenticated" (the key does not exist
// yet) and instead mentions the key is generated on confirm.
func TestCreatePhase1StatusDoesNotClaimAuthenticated(t *testing.T) {
	view := provePhase1PassedView(t, "create")
	if containsCI(view, "✓ authenticated") {
		t.Errorf("FIX-3: create-mode phase-1 status must NOT claim '✓ authenticated' (key not generated yet); got:\n%s", view)
	}
	if !containsCI(view, "generated on confirm") {
		t.Errorf("FIX-3: create-mode phase-1 status should mention the key is generated on confirm; got:\n%s", view)
	}
}

// TestUpdatePhase1StatusStillAuthenticated asserts update/add-account modes,
// where the key already exists, still show the authenticated status.
func TestUpdatePhase1StatusStillAuthenticated(t *testing.T) {
	for _, action := range []string{"update", "add-account"} {
		view := provePhase1PassedView(t, action)
		if !containsCI(view, "✓ authenticated") {
			t.Errorf("FIX-3: %s-mode phase-1 status must still show '✓ authenticated'; got:\n%s", action, view)
		}
	}
}

// --- guard: ReadFragment seam is wired (compile-time guard against dropping it) ---

func TestReadFragmentSeamWired(t *testing.T) {
	// newRootModel wires the live tuiDeps, including the readFragment seam used
	// by the update form to preserve signing state (FIX-1).
	deps := newRootModel(fakeDocDeps(), identity.Deps{}, identity.UpdateDeps{}).deps
	if deps.readFragment == nil {
		t.Fatal("tuiDeps.readFragment seam must be wired by newRootModel")
	}
	// Ensure the real seam matches gitconfig.ReadFragment's contract on a missing path.
	info, err := deps.readFragment(filepath.Join(t.TempDir(), "nope"))
	if err != nil {
		t.Fatalf("readFragment on missing path returned error: %v", err)
	}
	if !info.Missing {
		t.Fatal("readFragment on a missing fragment must report Missing=true")
	}
	_ = gitconfig.FragmentInfo{} // anchor the import
}
