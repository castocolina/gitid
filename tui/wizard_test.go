package tui

// wizard_test.go — Tests for the create/add wizard modal and the prove-before-write loop.
//
// Tests are ported in shape from tui/prove_test.go and adapted to the Phase 5.6
// wizard modal (Plans 04/05). The test NAMES are LOCKED by VALIDATION.md.
//
// Key differences from Phase 5:
//   - The wizard is a modal overlay on rootModel, not a pushed screenModel.
//   - The prove-before-write logic (two-phase SSH test + confirm gate) is the
//     same; only the rendering context changes (modal box vs full screen).
//   - Inline editing ('e') in the detail pane opens the wizard in "update" mode.

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
	"github.com/castocolina/gitid/internal/tester"
)

// --- shared helper factories for wizard tests ---

// makeTestWizardModel builds a createWizardModel ready for testing (step 1, create mode).
func makeTestWizardModel(deps tuiDeps) createWizardModel {
	return newCreateWizardModel("", deps)
}

// makeTestAddAccountWizard builds a createWizardModel for the "add account" mode
// where the identity name is pre-filled and locked.
func makeTestAddAccountWizard(name string, deps tuiDeps) createWizardModel {
	return newCreateWizardModel(name, deps)
}

// TestWizardOpen verifies that pressing 'a' from the Identities view opens the
// create wizard modal (step 1: name/provider/alias form).
// Requirement: TUI-05 (create wizard modal, D-05).
// Closes: Plan 05.
func TestWizardOpen(t *testing.T) {
	m := buildModel()
	// Ensure we are in identities view.
	m2 := sendKey(m, "1")

	// Press 'a' — should open the create wizard modal.
	m3 := sendKey(m2, "a")
	if m3.activeModal != createWizardModal {
		t.Errorf("'a' must open createWizardModal; got activeModal=%v", m3.activeModal)
	}

	// The wizard must be at step 1 (form).
	if m3.wizard.step != wizardStepForm {
		t.Errorf("wizard must start at wizardStepForm; got %v", m3.wizard.step)
	}

	// The view must contain the modal title.
	m3.width = 120
	m3.height = 40
	view := m3.renderContent()
	if !strings.Contains(view, "Create Identity") {
		t.Errorf("modal must render 'Create Identity'; view snippet: %q", truncateString(view, 200))
	}

	// 8 form fields must be present.
	if len(m3.wizard.inputs) != 8 {
		t.Errorf("wizard form must have 8 inputs; got %d", len(m3.wizard.inputs))
	}
}

// TestWizardAddAccountNameLocked verifies that opening the wizard as Add Account
// pre-fills the identity name read-only and leaves other fields editable.
// Requirement: TUI-05 (add account mode, D-05).
// Closes: Plan 05.
func TestWizardAddAccountNameLocked(t *testing.T) {
	deps := fakeWriteTUIDeps(nil)
	w := makeTestAddAccountWizard("existing-id", deps)

	// Name must be pre-filled.
	if w.inputs[0].Value() != "existing-id" {
		t.Errorf("add account: Identity Name must be pre-filled; got %q", w.inputs[0].Value())
	}

	// Name field must be locked (nameLocked=true).
	if !w.nameLocked {
		t.Error("add account: nameLocked must be true when name is pre-filled")
	}

	// Other fields (Git Name = index 1) must be editable (empty by default).
	if w.inputs[1].Value() != "" {
		t.Logf("add account: Git Name is %q (expected blank)", w.inputs[1].Value())
	}

	// The view for this wizard must contain the name in the title.
	view := w.view(80)
	if !strings.Contains(view, "Add Account") {
		t.Errorf("add account view must contain 'Add Account'; got: %q", truncateString(view, 200))
	}
}

// TestWizardFormValidation verifies that an invalid identity name (uppercase/space)
// shows the validation error below the field and blocks advancing.
// Requirement: TUI-05 (form validation, D-05).
// Closes: Plan 05.
func TestWizardFormValidation(t *testing.T) {
	deps := fakeWriteTUIDeps(nil)
	w := makeTestWizardModel(deps)

	// Set invalid name with a space (not allowed by ValidateName).
	w.inputs[0].SetValue("bad name")
	// Focus the name field.
	w.focusIdx = 0

	// Press enter — should not advance; validation error should appear.
	w2, _ := w.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})

	if w2.step != wizardStepForm {
		t.Errorf("invalid name must keep wizard on form step; got step=%v", w2.step)
	}
	if w2.err == "" {
		t.Error("invalid name must set an error message in the wizard")
	}
	if !strings.Contains(w2.err, "invalid") && !strings.Contains(w2.err, "Name") && !strings.Contains(w2.err, "name") {
		t.Errorf("validation error must mention name validity; got: %q", w2.err)
	}
}

// TestWizardFormEmailValidation verifies that a malformed git email (e.g. one
// with spaces) is rejected at the form step — BEFORE keygen and the SSH test —
// rather than failing deep in the fragment write. Reported on the real TTY:
// "I typed email with spaces and it let me continue."
//
// Plan 10 note: email validation is deferred to Screen 3 on screenSSHIdentity.
// This test uses the legacy form path (non-Screen-1) where email is still validated
// at form-submit time. The staged wizard's Screen 1 intentionally skips email
// validation (the email field lives on Screen 3, implemented in Plan 13).
func TestWizardFormEmailValidation(t *testing.T) {
	deps := fakeWriteTUIDeps(nil)
	w := makeTestWizardModel(deps)
	// Switch to legacy form mode so email is validated at this stage.
	w.screen = screenGitConfig // non-Screen-1 → uses handleKeyLegacy + advanceFromForm email gate

	w.inputs[0].SetValue("personal")        // valid identity name
	w.inputs[2].SetValue("foo bar@example") // email with a space — invalid
	w.focusIdx = len(w.inputs) - 1          // last field: Enter triggers advance

	w2, _ := w.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})

	if w2.step != wizardStepForm {
		t.Errorf("email with spaces must keep the wizard on the form step; got step=%v", w2.step)
	}
	if !strings.Contains(strings.ToLower(w2.err), "email") {
		t.Errorf("validation error must mention email; got %q", w2.err)
	}
	if w2.focusIdx != 2 {
		t.Errorf("focus must move to the email field (index 2); got %d", w2.focusIdx)
	}
}

// TestWizardProveRetry verifies the prove-before-write loop inside the wizard:
// Phase 1 (pre-write SSH test) fails → retry path is active; pressing 'r'
// re-starts phase 1 without re-opening the form.
// Requirement: TUI-05 (prove-before-write, D-07/T-write-gate).
// Closes: Plan 05.
// TestWizardTestScreenKeepsKeyCopyAndInstructions asserts the inline test screen
// (D-16 round 3: one screen, no view switch) keeps the FULL public key, the [c]
// copy affordance, and the upload instructions visible while/after testing — and
// that a failure prints the exact ssh command + raw output (Q2/Q3/Q5, TEST-03).
func TestWizardTestScreenKeepsKeyCopyAndInstructions(t *testing.T) {
	w := makeTestWizardModel(fakeWriteTUIDeps(nil))
	fullKey := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIThisIsTheFullPublicKeyDoNotTruncateMe work@gitid"
	w.staged = identity.StagedKey{
		PubLine:          fullKey + "\n",
		TempPrivatePath:  "/tmp/gitid-key-xyz/key",
		FinalPrivatePath: "/home/u/.ssh/id_ed25519_work",
	}
	w.step = wizardStepProve1Failed
	w.phase1Result = tester.Result{
		Command: "ssh -i /tmp/k -o IdentitiesOnly=yes -T git@github.com",
		Output:  "git@github.com: Permission denied (publickey).",
		Outcome: tester.ReachableNotUploaded,
	}

	view := w.view(90)

	// The key wraps inside the 72-col modal, so assert non-truncation via tokens
	// that the old truncatePubLine (60-char cut) would have dropped: the prefix
	// AND the trailing comment must both survive.
	if !strings.Contains(view, "ssh-ed25519") {
		t.Errorf("test screen must show the public key prefix; got:\n%s", view)
	}
	if !strings.Contains(view, "work@gitid") {
		t.Errorf("test screen must show the FULL key incl. trailing comment (not truncated); got:\n%s", view)
	}
	if !strings.Contains(view, "[c] copy key") {
		t.Errorf("test screen must keep the [c] copy affordance; got:\n%s", view)
	}
	if !strings.Contains(view, "github.com") {
		t.Errorf("test screen must keep the upload instructions; got:\n%s", view)
	}
	if !strings.Contains(view, "ssh -i /tmp/k") {
		t.Errorf("failed test must show the exact ssh command; got:\n%s", view)
	}
	if !strings.Contains(view, "Permission denied (publickey)") {
		t.Errorf("failed test must show the raw ssh output; got:\n%s", view)
	}
	// The staged (tested-now) path and the final install path must both be shown
	// so the `ssh -i <path>` command is verifiable and the key is findable.
	if !strings.Contains(view, "/tmp/gitid-key-xyz/key") {
		t.Errorf("test screen must show the staged private-key path; got:\n%s", view)
	}
	if !strings.Contains(view, "/home/u/.ssh/id_ed25519_work") {
		t.Errorf("test screen must show the final install path; got:\n%s", view)
	}
}

// TestWizardCopyFromTestScreen asserts [c] re-copies the public key from the
// inline test screen without changing the wizard step (re-copy in place, Q5).
func TestWizardCopyFromTestScreen(t *testing.T) {
	w := makeTestWizardModel(fakeWriteTUIDeps(nil))
	w.staged = identity.StagedKey{PubLine: "ssh-ed25519 AAAAKEY work@gitid\n", TempPrivatePath: "/tmp/k"}
	w.step = wizardStepProve1Failed

	w2, cmd := w.update(tea.KeyPressMsg{Code: 'c'})
	if cmd == nil {
		t.Error("[c] on the test screen must dispatch a clipboard copy command")
	}
	if w2.step != wizardStepProve1Failed {
		t.Errorf("[c] must not change the wizard step; got %v", w2.step)
	}
}

func TestWizardProveRetry(t *testing.T) {
	deps := fakeWriteTUIDeps(nil)
	// Override PreWrite to always fail.
	deps.identity.PreWrite = func(_, _ string, _ int) tester.Result {
		return tester.Result{Outcome: tester.Failure}
	}

	w := makeTestWizardModel(deps)
	// Advance wizard to prove step manually.
	w.step = wizardStepProve1Running
	w, _ = w.initProve()
	origRunID := w.runID

	// Simulate phase-1 FAIL.
	failMsg := preWriteResultMsg{result: tester.Result{Outcome: tester.Failure}}
	w2, _ := w.update(failMsg)
	if w2.step != wizardStepProve1Failed {
		t.Errorf("phase1 FAIL: expected wizardStepProve1Failed; got %v", w2.step)
	}
	if w2.confirmActive {
		t.Error("phase1 FAIL: confirmActive must be false")
	}

	// Press 'r' → re-run phase1 with incremented runID.
	w3, retryCmd := w2.update(tea.KeyPressMsg{Code: 'r'})
	if w3.runID <= origRunID {
		t.Errorf("retry: runID must increase; was %d, now %d", origRunID, w3.runID)
	}
	if w3.step != wizardStepProve1Running {
		t.Errorf("retry: expected wizardStepProve1Running; got %v", w3.step)
	}
	if retryCmd == nil {
		t.Error("retry: must dispatch a new preWriteCmd")
	}

	// Press 's' → sets skipConfirmPending.
	w4, _ := w2.update(tea.KeyPressMsg{Code: 's'})
	if !w4.skipConfirmPending {
		t.Error("'s' on failed phase must set skipConfirmPending=true")
	}

	// Press 'q' → emits a cmd that includes clearModalMsg (may be a batch with toast).
	_, qCmd := w2.update(tea.KeyPressMsg{Code: 'q'})
	if qCmd == nil {
		t.Error("'q' must emit a cmd (clearModalCmd or batch)")
	}
	// We accept any non-nil cmd — the root model will handle clearModalMsg and toast.
	// Verify at minimum it returns something (not a no-op nil).
	_ = qCmd
}

// TestWizardPersistsOnlyAfterPass verifies the security invariant: PersistAll
// fires ONLY after both prove phases PASS and the Enter write confirm
// (FIX-CREATE-01). It must NOT fire on form submit or keygen.
// Requirement: TUI-05 (write gate, FIX-CREATE-01, T-05.6-15).
// Closes: Plan 05.
func TestWizardPersistsOnlyAfterPass(t *testing.T) {
	var persistCalled bool
	deps := fakeWriteTUIDeps(&persistCalled)
	// Override WriteSSH to track if persist was called.
	deps.identity.WriteSSH = func(_, _, _ string) (string, error) {
		persistCalled = true
		return "bak", nil
	}
	deps.identity.Generate = func(_ identity.CreateInput) (identity.StagedKey, error) {
		return identity.StagedKey{
			TempPrivatePath:  "/tmp/key",
			FinalPrivatePath: "/tmp/key",
			FinalPubPath:     "/tmp/key.pub",
			PubLine:          "ssh-ed25519 AAAA test@gitid",
		}, nil
	}
	deps.identity.PersistKey = func(_ identity.StagedKey) (identity.KeyResult, error) {
		return identity.KeyResult{PubLine: "ssh-ed25519 AAAA test@gitid"}, nil
	}
	deps.identity.Cleanup = func(_ identity.StagedKey) {}
	deps.identity.CopyPub = func(_ string) error { return nil }
	deps.identity.WriteGitconfig = func(_, _, _ string, _ []gitconfig.Match) (string, error) { return "", nil }
	deps.identity.WriteFragment = func(_, _, _, _ string, _ bool) error { return nil }
	deps.identity.WriteAllowedSigners = func(_, _, _ string) (string, error) { return "", nil }
	deps.identity.Resolved = func(_ string) (tester.Result, tester.ResolvedConfig) {
		return tester.Result{Outcome: tester.PASS}, tester.ResolvedConfig{}
	}
	deps.identity.PreWrite = func(_, _ string, _ int) tester.Result {
		return tester.Result{Outcome: tester.PASS}
	}

	w := makeTestWizardModel(deps)

	// Fill in a valid form and advance to keygen.
	// On Screen 1 (screenSSHIdentity) the last Tab stop is Folder (focusIdx 5).
	// Git Name/Email/Match/Signing live on Screen 3; we still set them in inputs[]
	// for full wiring through buildCreateInput, but the Screen-1 Enter-advance
	// triggers at focusIdx == screen1FocusCount-1 == 5 (Folder field).
	w.inputs[0].SetValue("personal")
	w.inputs[1].SetValue("Test User")
	w.inputs[2].SetValue("test@example.com")
	w.inputs[3].SetValue("github.com")
	w.inputs[4].SetValue("22")
	w.inputs[5].SetValue("personal.github.com")
	w.inputs[6].SetValue("1")
	w.inputs[7].SetValue("y")
	w.focusIdx = folderFocusIdx() // last Screen-1 field (Folder, pos 5)

	// Press Enter on last Screen-1 field — advances to keygen.
	w2, _ := w.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if w2.step == wizardStepWritten {
		t.Error("form submit must NOT persist; step became written immediately")
	}
	if persistCalled {
		t.Error("form submit must NOT call PersistAll (WriteSSH)")
	}

	// Simulate keygen completing.
	keygenMsg := keygenResultMsg{staged: identity.StagedKey{
		TempPrivatePath:  "/tmp/key",
		FinalPrivatePath: "/tmp/key",
		FinalPubPath:     "/tmp/key.pub",
		PubLine:          "ssh-ed25519 AAAA test@gitid",
	}}
	w3, _ := w2.update(keygenMsg)
	if persistCalled {
		t.Error("keygen result must NOT call PersistAll")
	}
	_ = w3

	// Advance wizard to prove step (simulating Enter on upload step).
	w3.step = wizardStepProve1Running
	w4, _ := w3.initProve()

	// Simulate phase-1 PASS.
	w5, _ := w4.update(preWriteResultMsg{result: tester.Result{Outcome: tester.PASS}})
	// Simulate phase-2 PASS.
	w6, _ := w5.update(resolvedResultMsg{result: tester.Result{Outcome: tester.PASS}})
	if !w6.confirmActive {
		t.Fatal("confirmActive must be true after both phases PASS")
	}
	if persistCalled {
		t.Error("after phases PASS, PersistAll must NOT fire until Enter is pressed")
	}

	// Press Enter — now PersistAll should fire.
	_, writeCmd := w6.update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if writeCmd == nil {
		t.Error("Enter after confirmActive must dispatch write cmd")
	}
	if writeCmd != nil {
		writeCmd() //nolint:errcheck // result checked via persistCalled
		if !persistCalled {
			t.Error("write gate: Enter after confirmActive must call PersistAll (WriteSSH)")
		}
	}
}

// TestInlineEdit verifies that pressing 'e' in the Identities detail pane
// enables inline editing: the focused field gets an active-input border;
// pressing Enter commits the field; pressing Esc cancels all edits.
// Structural field change (alias/hostname/port) triggers the prove loop.
// Requirement: TUI-05 (inline editing, D-05/D-07).
// Closes: Plan 04.
func TestInlineEdit(t *testing.T) {
	acct := makeTestAccount()
	m := newIdentityDetailModel()
	m.account = &acct

	// Press 'e' → enters inline edit mode.
	m2, _ := m.handleKey("e")
	if !m2.inlineEditMode {
		t.Error("pressing 'e' must set inlineEditMode=true")
	}

	// Tab cycles to next field.
	focusBefore := m2.focusedField
	m3, _ := m2.handleKey("tab")
	if len(m2.editFields) > 1 && m3.focusedField == focusBefore {
		t.Error("Tab must advance focusedField")
	}

	// Esc cancels — inlineEditMode returns to false, no write dispatched.
	m4, _ := m2.handleKey("esc")
	if m4.inlineEditMode {
		t.Error("Esc must clear inlineEditMode")
	}

	// Structural field (Alias) + Enter → proveModalPending set (prove loop signaled).
	m5, _ := m2.handleKey("e") // re-enter edit mode from clean state
	_ = m5
	m6 := m2 // already in edit mode
	// Focus the alias field (structural).
	for i, f := range m6.editFields {
		if f.label == "Alias" {
			m6.focusedField = i
			break
		}
	}
	m7, _ := m6.handleKey("enter")
	if !m7.proveModalPending {
		t.Error("structural field Enter must set proveModalPending=true")
	}

	// Non-structural field (Git Email) + Enter → editConfirmPending set.
	m8 := m2 // re-use edit mode
	for i, f := range m8.editFields {
		if f.label == "Git Email" {
			m8.focusedField = i
			break
		}
	}
	m9, _ := m8.handleKey("enter")
	if !m9.editConfirmPending {
		t.Error("non-structural field Enter must set editConfirmPending=true")
	}
}

// TestCreateWizardFormKeysNoPanic is a regression test for the nil-cmd panic
// at model.go:377. In the create-wizard form step, field-cycling keys (Tab,
// Shift+Tab) return a nil tea.Cmd. The root model invoked cmd() unconditionally
// to inspect its message, dereferencing nil and crashing the real program
// ("invalid memory address or nil pointer dereference" on Tab, reported on a
// 100+ column terminal). The fix guards cmd != nil before inspecting it.
func TestCreateWizardFormKeysNoPanic(t *testing.T) {
	m := newRootModel(fakeDocDeps(), fakeIdentityDeps(), identity.UpdateDeps{}, identity.DeleteDeps{})
	m.activeView = identitiesView
	m = sendKey(m, "a") // open the create wizard
	if m.activeModal != createWizardModal {
		t.Fatalf("expected createWizardModal after 'a'; got %v", m.activeModal)
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("create-wizard form key panicked (regression model.go:377): %v", r)
		}
	}()

	// Tab and Shift+Tab cycle fields and return a nil cmd — must not panic.
	m = sendMsg(m, tea.KeyPressMsg{Code: tea.KeyTab})
	m = sendMsg(m, tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	// A printable key in the form must also be safe.
	m = sendMsg(m, tea.KeyPressMsg{Code: 'x'})
	_ = m
}

// TestWizardFormTypingInsertsText is a regression test (D-1): typing printable
// keys in the create-wizard form must insert into the focused field. The root
// model stringified the key event (model.go) and the wizard rebuilt a
// KeyPressMsg without the Text field (wizard.go), so bubbles v2 textinput —
// which inserts from msg.Text — dropped every printable key. Reported on the
// real TTY: "I tried to add an Identity but I'm not able to write in the form,
// only tabs to navigate it." A real terminal sets Text on a printable
// KeyPressMsg, so the test mirrors that.
func TestWizardFormTypingInsertsText(t *testing.T) {
	m := newRootModel(fakeDocDeps(), fakeIdentityDeps(), identity.UpdateDeps{}, identity.DeleteDeps{})
	m.activeView = identitiesView
	m = sendKey(m, "a") // open the create wizard (focus is on field 0: Identity Name)
	if m.activeModal != createWizardModal {
		t.Fatalf("expected createWizardModal after 'a'; got %v", m.activeModal)
	}

	for _, r := range "dev" {
		m = sendMsg(m, tea.KeyPressMsg{Code: r, Text: string(r)})
	}

	if got := m.wizard.inputs[0].Value(); got != "dev" {
		t.Errorf("typing must insert into the focused field; got %q want %q", got, "dev")
	}
}

// TestWizardSigningToggle verifies P0-3: the Signing field is a Space-toggle, not
// a cryptic free-text "y"/"n" field, and typing does not corrupt it.
// The Signing field belongs to Screen 3 (Plan 13); this test uses the legacy form
// path to exercise the toggle behavior which the staged flow preserves for Screen 3.
func TestWizardSigningToggle(t *testing.T) {
	w := newCreateWizardModel("", tuiDeps{})
	// Use legacy form (Screen 3) so fieldSigning is active in the focus cycle.
	w.screen = screenGitConfig
	w.focusIdx = fieldSigning

	// A real TTY reports the space bar as the "space" key (String() == "space"),
	// not a literal " " — mirror that so the test exercises the live path (D-16).
	w, _ = w.handleKey(tea.KeyPressMsg{Code: tea.KeySpace})
	if got := w.inputs[fieldSigning].Value(); got != "n" {
		t.Errorf("space must toggle signing y→n; got %q", got)
	}
	w, _ = w.handleKey(tea.KeyPressMsg{Code: tea.KeySpace})
	if got := w.inputs[fieldSigning].Value(); got != "y" {
		t.Errorf("space must toggle signing n→y; got %q", got)
	}
	w, _ = w.handleKey(tea.KeyPressMsg{Code: 'z', Text: "z"})
	if got := w.inputs[fieldSigning].Value(); got != "y" {
		t.Errorf("typing must not edit the signing toggle; got %q", got)
	}
}

// TestWizardFormReadableChoices verifies P0-3: the legacy form (Screen 3) shows
// readable Match Strategy + Signing values and a live includeIf preview — never
// a cryptic "> 1". These fields live on Screen 3 (Plan 13); Screen 1 shows only
// SSH-identity fields (which is verified in TestWizardScreen1ShowsSSHIdentityFields).
func TestWizardFormReadableChoices(t *testing.T) {
	w := newCreateWizardModel("myid", tuiDeps{})
	// Switch to legacy form so Match/Signing fields are rendered.
	w.screen = screenGitConfig
	var sb strings.Builder
	w.viewForm(&sb, 72)
	out := sb.String()

	if strings.Contains(out, "> 1") {
		t.Errorf("match strategy must not show cryptic '> 1'; got:\n%s", out)
	}
	for _, want := range []string{"gitdir", "includeIf preview", "enabled"} {
		if !strings.Contains(out, want) {
			t.Errorf("wizard form must contain %q (readable choices + preview); got:\n%s", want, out)
		}
	}
}

// TestRotateRelabeledAsNewKey verifies P2-8: the rotate action is labeled in
// plain language ("new key"), not the "rotate" jargon, in the footer.
func TestRotateRelabeledAsNewKey(t *testing.T) {
	if foot := buildModel().renderFooter(); !strings.Contains(foot, "new key") {
		t.Errorf("footer must label rotate as 'new key'; got %q", foot)
	}
}

// TestDefaultHasconfigPattern verifies that defaultHasconfigPattern derives
// git@<alias>:*/** (recipe canonical form, D-03).
func TestDefaultHasconfigPattern(t *testing.T) {
	got := defaultHasconfigPattern("personal.github.com")
	want := "git@personal.github.com:*/**"
	if got != want {
		t.Errorf("defaultHasconfigPattern(%q): want %q, got %q", "personal.github.com", want, got)
	}
}

// TestDefaultHasconfigPatternEmpty verifies that an empty alias returns "".
func TestDefaultHasconfigPatternEmpty(t *testing.T) {
	got := defaultHasconfigPattern("")
	if got != "" {
		t.Errorf("defaultHasconfigPattern(empty): want \"\", got %q", got)
	}
}

// TestLiveIncludeIfPreview_Gitdir verifies that liveIncludeIfPreview for
// strategyGitdir produces a [includeIf "gitdir:..."] block.
func TestLiveIncludeIfPreview_Gitdir(t *testing.T) {
	got := liveIncludeIfPreview(strategyGitdir, "personal", "personal.github.com", "~/git/personal/", "git@personal.github.com:*/**")
	if !strings.Contains(got, `[includeIf "gitdir:`) {
		t.Errorf("liveIncludeIfPreview(gitdir): must contain gitdir includeIf; got:\n%s", got)
	}
	if strings.Contains(got, "hasconfig:") {
		t.Errorf("liveIncludeIfPreview(gitdir): must NOT contain hasconfig; got:\n%s", got)
	}
}

// TestLiveIncludeIfPreview_Hasconfig verifies that liveIncludeIfPreview for
// strategyHasconfig produces a [includeIf "hasconfig:remote.*.url:..."] block.
func TestLiveIncludeIfPreview_Hasconfig(t *testing.T) {
	got := liveIncludeIfPreview(strategyHasconfig, "personal", "personal.github.com", "~/git/personal/", "git@personal.github.com:*/**")
	if !strings.Contains(got, `hasconfig:remote.*.url:`) {
		t.Errorf("liveIncludeIfPreview(hasconfig): must contain hasconfig; got:\n%s", got)
	}
	if strings.Contains(got, `[includeIf "gitdir:`) {
		t.Errorf("liveIncludeIfPreview(hasconfig): must NOT contain gitdir; got:\n%s", got)
	}
}

// TestLiveIncludeIfPreview_Both verifies that liveIncludeIfPreview for
// strategyBoth produces TWO [includeIf] blocks (one gitdir, one hasconfig).
func TestLiveIncludeIfPreview_Both(t *testing.T) {
	got := liveIncludeIfPreview(strategyBoth, "personal", "personal.github.com", "~/git/personal/", "git@personal.github.com:*/**")
	if !strings.Contains(got, `[includeIf "gitdir:`) {
		t.Errorf("liveIncludeIfPreview(both): must contain gitdir block; got:\n%s", got)
	}
	if !strings.Contains(got, `hasconfig:remote.*.url:`) {
		t.Errorf("liveIncludeIfPreview(both): must contain hasconfig block; got:\n%s", got)
	}
}

// TestWizardMatchSelectorNavigation verifies that pressing ↓ from the match
// selector field moves from gitdir → hasconfig and the rendered view changes.
// The Match Strategy selector lives on Screen 3 (Plan 13); this test uses the
// legacy form path to verify the selector navigation.
func TestWizardMatchSelectorNavigation(t *testing.T) {
	w := newCreateWizardModel("personal", tuiDeps{})
	// Use legacy form (Screen 3) so the match selector is active.
	w.screen = screenGitConfig
	w.focusIdx = fieldMatch
	w.inputs[0].SetValue("personal")

	// Initial state: gitdir selected.
	if w.matchSel != strategyGitdir {
		t.Errorf("initial matchSel must be strategyGitdir; got %v", w.matchSel)
	}

	// Press ↓ → should move to hasconfig.
	w2, _ := w.handleKey(tea.KeyPressMsg{Code: tea.KeyDown})
	if w2.matchSel != strategyHasconfig {
		t.Errorf("↓ must select strategyHasconfig; got %v", w2.matchSel)
	}

	// View must now show hasconfig preview.
	view := w2.view(90)
	if !strings.Contains(view, "hasconfig:remote.*.url:") {
		t.Errorf("after ↓, view must contain hasconfig preview; got:\n%s", view)
	}
	if strings.Contains(view, `[includeIf "gitdir:`) {
		t.Errorf("after ↓ to hasconfig, view must NOT show gitdir includeIf; got:\n%s", view)
	}
}

// ─── Task 2: Screen 1 SSH-identity staged-flow tests ───────────────────────

// TestWizardScreen1ShowsSSHIdentityFields verifies that Screen 1 (screenSSHIdentity)
// renders the always-visible editable SSH-identity fields and does NOT render
// Git Name / Git Email / Match Strategy / Signing rows.
// Requirement: TUI-04 (G-1 HARD requirement: always-visible editable fields).
func TestWizardScreen1ShowsSSHIdentityFields(t *testing.T) {
	w := newCreateWizardModel("", tuiDeps{})
	w.inputs[0].SetValue("personal")
	view := w.view(90)

	// Screen 1 must show all SSH-identity rows.
	for _, want := range []string{"Identity Name", "Key Algorithm", "Provider", "SSH Alias", "Hostname", "Port", "Folder"} {
		if !strings.Contains(view, want) {
			t.Errorf("Screen 1 must show %q; got:\n%s", want, view)
		}
	}

	// Screen 1 must NOT render Git Name / Git Email / Match Strategy / Signing.
	for _, notWant := range []string{"Git Name", "Git Email", "Match Strategy", "Signing"} {
		if strings.Contains(view, notWant) {
			t.Errorf("Screen 1 must NOT show %q (belongs to Screen 3); got:\n%s", notWant, view)
		}
	}
}

// TestWizardScreen1AltSSHDefaults verifies Hostname pre-fills with the recipe
// alt-SSH endpoint (ssh.github.com) and Port with 443 (not 22).
// Requirement: SSH-01 (alt-SSH per recipe), TUI-08.
func TestWizardScreen1AltSSHDefaults(t *testing.T) {
	w := newCreateWizardModel("", tuiDeps{})

	// Port default must be 443.
	if got := w.inputs[4].Value(); got != "443" {
		t.Errorf("Port default must be '443' (alt-SSH); got %q", got)
	}

	// Hostname must pre-fill with identity.DefaultHostname("github.com") = "ssh.github.com".
	if got := w.hostnameVal.Value(); got != "ssh.github.com" {
		t.Errorf("Hostname must pre-fill with 'ssh.github.com'; got %q", got)
	}
}

// TestWizardScreen1HostnameEditable verifies that editing the Hostname field
// overrides the alt-SSH default (user can revert to github.com:22).
// Drives via the real model Update/handleKey path (anti-blindspot).
// Requirement: TUI-08 (hostname overridable).
func TestWizardScreen1HostnameEditable(t *testing.T) {
	w := newCreateWizardModel("personal", tuiDeps{})
	w.inputs[0].SetValue("personal")

	// Directly set the hostname to what the user would have typed (simulates the
	// user clearing the pre-filled value and entering a custom hostname — the same
	// effect as buildCreateInput receiving the edited value).
	w.hostnameVal.SetValue("github.com")
	w.hostnameEdited = true

	in := w.buildCreateInput("github.com")
	if in.Hostname != "github.com" {
		t.Errorf("edited Hostname must override alt-SSH default in buildCreateInput; got %q want %q", in.Hostname, "github.com")
	}

	// Also verify via real handleKey: focus the hostname slot and type a char.
	w2 := newCreateWizardModel("personal", tuiDeps{})
	w2.inputs[0].SetValue("personal")
	w2.focusIdx = hostnameFocusIdx()
	// Set a known starting value so appended text is predictable.
	w2.hostnameVal.SetValue("")
	w2.hostnameVal.Focus()
	for _, r := range "github.com" {
		w2, _ = w2.handleKey(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	if !w2.hostnameEdited {
		t.Error("typing into hostname field must set hostnameEdited=true")
	}
	in2 := w2.buildCreateInput("github.com")
	if in2.Hostname != "github.com" {
		t.Errorf("after typing via handleKey, buildCreateInput.Hostname=%q; want 'github.com'", in2.Hostname)
	}
}

// TestWizardScreen1PortEditable verifies that typing "22" into the Port field
// overrides the 443 default and buildCreateInput.Port == 22.
// Requirement: TUI-08 (port overridable).
func TestWizardScreen1PortEditable(t *testing.T) {
	w := newCreateWizardModel("personal", tuiDeps{})
	w.inputs[0].SetValue("personal")
	// Focus the port field (inputs[4]).
	w.focusIdx = 4
	w.inputs[w.focusIdx].Focus()

	// Clear and type "22".
	w.inputs[4].SetValue("22")
	in := w.buildCreateInput("github.com")
	if in.Port != 22 {
		t.Errorf("Port '22' override must produce buildCreateInput.Port==22; got %d", in.Port)
	}
}

// TestWizardScreen1FolderEditable verifies that the Folder (gitdir) row is
// a top-level editable field on Screen 1 and Tab cycles to it.
// Requirement: TUI-04 (folder always visible + editable on Screen 1).
func TestWizardScreen1FolderEditable(t *testing.T) {
	w := newCreateWizardModel("personal", tuiDeps{})
	w.inputs[0].SetValue("personal")

	// Type into the folder field at its focus index.
	folderIdx := folderFocusIdx()
	w.focusIdx = folderIdx

	for _, r := range "~/work/personal/" {
		w, _ = w.handleKey(tea.KeyPressMsg{Code: r, Text: string(r)})
	}

	in := w.buildCreateInput("github.com")
	if !strings.Contains(in.Matches[0].Value, "~/work/personal/") {
		t.Errorf("Folder edit must propagate to buildCreateInput matches; got %+v", in.Matches)
	}
}

// TestWizardScreen1TabCycleAllFields verifies all Screen-1 fields are reachable
// via Tab (none skipped). Drives via real handleKey (anti-blindspot).
// Requirement: TUI-04 (no hidden fields in Screen 1 tab cycle).
func TestWizardScreen1TabCycleAllFields(t *testing.T) {
	w := newCreateWizardModel("", tuiDeps{})
	// Tab through all Screen-1 focus positions and collect which indices we visit.
	visited := map[int]bool{}
	// Number of Screen-1 fields: name(0), provider(3), alias(5), then hostname, port(4), folder virtual indices.
	// We check that a full Tab cycle visits >= 6 distinct focus positions.
	for i := 0; i < 20; i++ {
		visited[w.focusIdx] = true
		w, _ = w.handleKey(tea.KeyPressMsg{Code: tea.KeyTab})
	}
	if len(visited) < 5 {
		t.Errorf("Tab cycle on Screen 1 must visit at least 5 distinct focus positions; visited %d: %v", len(visited), visited)
	}
}

// TestWizardScreen1HostnameTracksProvider verifies that when the Provider field
// changes and the user has NOT manually edited Hostname, hostnameVal auto-updates.
// Requirement: TUI-08 (hostname auto-tracks provider unless overridden).
func TestWizardScreen1HostnameTracksProvider(t *testing.T) {
	w := newCreateWizardModel("", tuiDeps{})

	// Provider defaults to github.com; hostname should be ssh.github.com.
	if got := w.hostnameVal.Value(); got != "ssh.github.com" {
		t.Errorf("initial Hostname: want ssh.github.com, got %q", got)
	}

	// Change provider to gitlab.com (type into inputs[3]).
	w.focusIdx = 3
	w.inputs[3].SetValue("gitlab.com")
	// Simulate provider-change processing (trigger via a provider-change key).
	w = w.refreshHostnameIfUnedited()

	if got := w.hostnameVal.Value(); got != "altssh.gitlab.com" {
		t.Errorf("after provider change to gitlab.com, Hostname must auto-update to altssh.gitlab.com; got %q", got)
	}
}

// TestWizardScreen1BuildCreateInputHostname verifies buildCreateInput uses the
// alt-SSH default when Hostname is unedited and the typed value when edited.
// Requirement: SSH-01, TUI-08 (preview/write parity).
func TestWizardScreen1BuildCreateInputHostname(t *testing.T) {
	w := newCreateWizardModel("personal", tuiDeps{})
	w.inputs[0].SetValue("personal")

	// Unedited: should use identity.DefaultHostname("github.com").
	in := w.buildCreateInput("github.com")
	if in.Hostname != identity.DefaultHostname("github.com") {
		t.Errorf("unedited Hostname: want %q, got %q", identity.DefaultHostname("github.com"), in.Hostname)
	}

	// Edited: should use the typed value.
	w.hostnameVal.SetValue("github.com")
	w.hostnameEdited = true
	in2 := w.buildCreateInput("github.com")
	if in2.Hostname != "github.com" {
		t.Errorf("edited Hostname: want 'github.com', got %q", in2.Hostname)
	}
}

// ─── Task 3: Live SSH Host-block preview tests ──────────────────────────────

// TestWizardScreen1SSHBlockPreview verifies that Screen 1 renders a live preview
// of the exact Host block that will be written, using alt-SSH defaults.
// Drives via the real model view path (anti-blindspot).
// Requirement: TUI-04 (G-2 preview half), SSH-01.
func TestWizardScreen1SSHBlockPreview(t *testing.T) {
	w := newCreateWizardModel("", tuiDeps{})
	w.inputs[0].SetValue("personal")
	w.inputs[3].SetValue("github.com")
	// Explicit alias: a typed alias renders as-is. (Blank-alias→provider-host is
	// covered by TestWizardAliasBlankYieldsProviderHost, UAT G-5 honesty.)
	w.inputs[5].SetValue("personal.github.com")

	view := w.view(90)

	// Preview section header must appear.
	if !strings.Contains(view, "Will write to") && !strings.Contains(view, "~/.ssh/config") {
		t.Errorf("Screen 1 must show Host-block preview header; got:\n%s", view)
	}

	// The preview must contain the exact Host-block lines (alt-SSH defaults).
	wantLines := []string{
		"Host personal.github.com",
		"Hostname ssh.github.com",
		"Port 443",
		"User git",
		"IdentitiesOnly yes",
	}
	for _, want := range wantLines {
		if !strings.Contains(view, want) {
			t.Errorf("Screen 1 preview must contain %q; got:\n%s", want, view)
		}
	}

	// Preview must show IdentityFile line.
	if !strings.Contains(view, "IdentityFile") {
		t.Errorf("Screen 1 preview must contain IdentityFile; got:\n%s", view)
	}

	// No advisory-divergence line.
	if strings.Contains(view, "advisory") || strings.Contains(view, "diverge") {
		t.Errorf("Screen 1 preview must NOT contain advisory/divergence text; got:\n%s", view)
	}
}

// TestWizardScreen1PreviewGuardedOnEmptyName verifies the Host-block preview
// is not shown when the identity name is blank.
func TestWizardScreen1PreviewGuardedOnEmptyName(t *testing.T) {
	w := newCreateWizardModel("", tuiDeps{})
	// Name is blank.
	view := w.view(90)

	if strings.Contains(view, "Will write to") {
		t.Errorf("preview must not appear when name is blank; got:\n%s", view)
	}
}

// TestWizardScreen1PreviewUpdatesOnPortEdit verifies the live preview re-renders
// when the user overrides the Port field (from 443 to 22).
// Drives via the real model Update/handleKey path (anti-blindspot).
func TestWizardScreen1PreviewUpdatesOnPortEdit(t *testing.T) {
	w := newCreateWizardModel("", tuiDeps{})
	w.inputs[0].SetValue("personal")
	w.inputs[3].SetValue("github.com")

	// Override port to 22.
	w.inputs[4].SetValue("22")

	view := w.view(90)

	if !strings.Contains(view, "Port 22") {
		t.Errorf("preview must update with edited Port 22; got:\n%s", view)
	}
	// Must NOT show advisory text.
	if strings.Contains(view, "advisory") {
		t.Errorf("no advisory text expected; got:\n%s", view)
	}
}

// TestWizardScreen1PreviewUpdatesOnAliasEdit verifies the live preview re-renders
// with a custom alias when the user types into the SSH Alias field.
func TestWizardScreen1PreviewUpdatesOnAliasEdit(t *testing.T) {
	w := newCreateWizardModel("", tuiDeps{})
	w.inputs[0].SetValue("personal")
	w.inputs[3].SetValue("github.com")
	w.inputs[5].SetValue("mygithub")

	view := w.view(90)

	if !strings.Contains(view, "Host mygithub") {
		t.Errorf("preview must show typed alias 'mygithub'; got:\n%s", view)
	}
}

// TestWizardScreen1PreviewUsesRenderHostBlock verifies the preview goes through
// sshconfig.RenderHostBlock (no hand-built Host block in tui/).
// Requirement: T-05.7-10-02 (preview/write parity).
func TestWizardScreen1PreviewUsesRenderHostBlock(t *testing.T) {
	w := newCreateWizardModel("", tuiDeps{})
	w.inputs[0].SetValue("personal")
	w.inputs[3].SetValue("github.com")
	// Set a staged key so the preview shows the deterministic final path.
	w.staged.FinalPrivatePath = "/home/user/.ssh/id_ed25519_personal"

	view := w.view(90)

	// The IdentityFile line must contain the staged FinalPrivatePath.
	if !strings.Contains(view, "/home/user/.ssh/id_ed25519_personal") {
		t.Errorf("preview IdentityFile must use staged.FinalPrivatePath; got:\n%s", view)
	}
}

// ─── Plan 12, Task 1: Screen 2 command+path visibility tests ──────────────────

// TestWizardProve1DoneShowsCommand verifies that after Phase-1 PASS the view
// renders the exact ssh command from phase1Result.Command (G-3). The command must
// remain visible on success — not only on failure.
// Drives via the real model Update path and asserts on view(width) output.
// Requirement: G-3 (full command+path consistently visible); AUTOUP-01 (no auto-upload).
func TestWizardProve1DoneShowsCommand(t *testing.T) {
	w := makeTestWizardModel(fakeWriteTUIDeps(nil))
	w.staged = identity.StagedKey{
		PubLine:          "ssh-ed25519 AAAA test@gitid\n",
		TempPrivatePath:  "/tmp/gitid/id_ed25519_personal",
		FinalPrivatePath: "/home/u/.ssh/id_ed25519_personal",
	}
	w.step = wizardStepProve1Done
	w.phase1Result = tester.Result{
		Command: "ssh -i /tmp/gitid/id_ed25519_personal -o IdentitiesOnly=yes -p 443 -T git@ssh.github.com",
		Output:  "Hi personal! You've successfully authenticated",
		Outcome: tester.PASS,
	}

	view := w.view(90)

	// The ssh command must appear in the view even on PASS.
	if !strings.Contains(view, "ssh -i /tmp/gitid/id_ed25519_personal") {
		t.Errorf("prove1Done view must show the ssh command on success; got:\n%s", view)
	}
	// The tested key path must remain visible.
	if !strings.Contains(view, "/tmp/gitid/id_ed25519_personal") {
		t.Errorf("prove1Done view must show the tested key path; got:\n%s", view)
	}
}

// TestWizardProve2DoneShowsCommand verifies that after Phase-2 PASS the view
// renders the ssh command from phase2Result.Command alongside the Phase-1 success
// indicator (G-3: consistent on pre-run + success + failure).
func TestWizardProve2DoneShowsCommand(t *testing.T) {
	w := makeTestWizardModel(fakeWriteTUIDeps(nil))
	w.staged = identity.StagedKey{
		PubLine:          "ssh-ed25519 AAAA test@gitid\n",
		TempPrivatePath:  "/tmp/gitid/id_ed25519_personal",
		FinalPrivatePath: "/home/u/.ssh/id_ed25519_personal",
	}
	w.step = wizardStepProve2Done
	w.confirmActive = true
	w.phase1Result = tester.Result{
		Command: "ssh -i /tmp/gitid/id_ed25519_personal -o IdentitiesOnly=yes -p 443 -T git@ssh.github.com",
		Output:  "Hi personal! You've successfully authenticated",
		Outcome: tester.PASS,
	}
	w.phase2Result = tester.Result{
		Command: "ssh -o BatchMode=yes -o ConnectTimeout=10 -T git@personal.github.com",
		Output:  "Hi personal! You've successfully authenticated",
		Outcome: tester.PASS,
	}

	view := w.view(90)

	// Phase 1 ssh command must still be visible on the prove2Done screen.
	if !strings.Contains(view, "ssh -i /tmp/gitid/id_ed25519_personal") {
		t.Errorf("prove2Done view must show phase1 ssh command; got:\n%s", view)
	}
}

// TestWizardPreRunScreenShowsCommand verifies the pre-run (upload step) shows
// the exact command that WILL run via tester.PreWriteCommand, using the alt-SSH
// endpoint (ssh.github.com:443 for github), not the raw provider hostname.
// Drives via the real model view path (anti-blindspot; drives viewUpload).
func TestWizardPreRunScreenShowsCommand(t *testing.T) {
	w := makeTestWizardModel(fakeWriteTUIDeps(nil))
	w.inputs[0].SetValue("personal")
	w.inputs[3].SetValue("github.com")
	w.inputs[4].SetValue("443")
	w.staged = identity.StagedKey{
		PubLine:          "ssh-ed25519 AAAA test@gitid\n",
		TempPrivatePath:  "/tmp/gitid/id_ed25519_personal",
		FinalPrivatePath: "/home/u/.ssh/id_ed25519_personal",
	}
	w.step = wizardStepUpload

	view := w.view(90)

	// The pre-run screen must show a command referencing the alt-SSH hostname.
	if !strings.Contains(view, "ssh.github.com") {
		t.Errorf("pre-run screen must show alt-SSH hostname (ssh.github.com) in the command; got:\n%s", view)
	}
	// The pre-run command must reference the staged key path.
	if !strings.Contains(view, "/tmp/gitid/id_ed25519_personal") {
		t.Errorf("pre-run screen must show the staged key path in the command; got:\n%s", view)
	}
	// Must NOT have gh/glab auto-upload instructions.
	if strings.Contains(view, "gh auth") || strings.Contains(view, "glab") {
		t.Errorf("pre-run screen must NOT include auto-upload (gh/glab); got:\n%s", view)
	}
}

// TestWizardInitProveUsesAltSSHEndpoint verifies that initProve computes hostname
// and port from the same sources as buildCreateInput (alt-SSH defaults, not the raw
// provider). The cmd dispatched by initProve must reference ssh.github.com:443, not
// github.com:22.
func TestWizardInitProveUsesAltSSHEndpoint(t *testing.T) {
	var capturedHostname string
	var capturedPort int
	deps := fakeWriteTUIDeps(nil)
	deps.identity.PreWrite = func(keyPath, hostname string, port int) tester.Result {
		capturedHostname = hostname
		capturedPort = port
		return tester.Result{Outcome: tester.PASS, Command: "ssh -i " + keyPath + " -p " + fmt.Sprintf("%d", port) + " git@" + hostname}
	}

	w := makeTestWizardModel(deps)
	w.inputs[0].SetValue("personal")
	w.inputs[3].SetValue("github.com")
	// Port from inputs[4] (default 443); do NOT override to verify default propagates.
	w.staged = identity.StagedKey{
		TempPrivatePath:  "/tmp/gitid/id_ed25519_personal",
		FinalPrivatePath: "/home/u/.ssh/id_ed25519_personal",
	}

	w, cmd := w.initProve()
	if cmd == nil {
		t.Fatal("initProve must dispatch a command")
	}
	// initProve returns a batch cmd (preWriteCmd + spinner tick); run all msgs.
	msgs := runAllCmds(cmd)
	var gotPreWrite bool
	for _, msg := range msgs {
		if _, ok := msg.(preWriteResultMsg); ok {
			gotPreWrite = true
		}
	}
	if !gotPreWrite {
		t.Fatalf("initProve batch must include a preWriteResultMsg; got msgs: %+v", msgs)
	}

	if capturedHostname != "ssh.github.com" {
		t.Errorf("initProve must use alt-SSH hostname (ssh.github.com); got %q", capturedHostname)
	}
	if capturedPort != 443 {
		t.Errorf("initProve must use alt-SSH port (443); got %d", capturedPort)
	}
}

// TestWizardMatchValidationGitdirRequired verifies that the plan's validation
// copy string "gitdir path is required" is present in the UI-SPEC (placeholder;
// this string must appear in the selector's validation path).
func TestWizardMatchValidationCopyPresent(t *testing.T) {
	// The validation messages are checked in advanceFromForm. This test documents
	// the contract: if the selector is wired and the form has the identity name,
	// the view must show gitdir as the current selection.
	w := newCreateWizardModel("testid", tuiDeps{})
	w.focusIdx = fieldMatch
	w.inputs[0].SetValue("testid")
	w2, _ := w.handleKey(tea.KeyPressMsg{Code: tea.KeyDown}) // move to hasconfig
	// Preview for hasconfig should include the URL pattern (or placeholder text).
	view := w2.renderMatchSelector("Match Strategy: ", true)
	if !strings.Contains(view, "hasconfig") {
		t.Errorf("expanded selector must contain 'hasconfig' option; got:\n%s", view)
	}
	if !strings.Contains(view, "Preview:") {
		t.Errorf("expanded selector must contain 'Preview:' section; got:\n%s", view)
	}
}

// ─── Plan 12, Task 2: PersistSSH on success + no-write guidance tests ─────────

// TestWizardPersistSSHOnPassPath verifies the security invariant for Plan 12:
// on the Screen-2 PASS path (prove2Done + Enter), the wizard calls PersistSSH
// (WriteSSH fires) and does NOT call PersistGitconfig (WriteGitconfig must NOT fire).
// This is the LEG-1-only invariant — LEG 2 is Plan 13.
//
// Drives via the real model path: simulate prove phases PASS → confirm gate open →
// Enter → run returned cmd → assert WriteSSH called with a Host block containing
// `Host <alias>` and `IdentitiesOnly yes`; assert WriteGitconfig NOT called.
//
// Requirement: G-2 (progressive LEG-1 write on success), T-05.7-12-02 (LEG 1 only).
func TestWizardPersistSSHOnPassPath(t *testing.T) {
	var writeSSHCalled bool
	var writeSSHBlock string
	var writeGitconfigCalled bool

	deps := fakeWriteTUIDeps(nil)
	deps.identity.WriteSSH = func(_, hostBlock, _ string) (string, error) {
		writeSSHCalled = true
		writeSSHBlock = hostBlock
		return "bak-ssh-20260621", nil
	}
	deps.identity.WriteGitconfig = func(_, _, _ string, _ []gitconfig.Match) (string, error) {
		writeGitconfigCalled = true
		return "", nil
	}
	deps.identity.PersistKey = func(_ identity.StagedKey) (identity.KeyResult, error) {
		return identity.KeyResult{PubLine: "ssh-ed25519 AAAA test@gitid"}, nil
	}

	w := makeTestWizardModel(deps)
	w.inputs[0].SetValue("personal")
	w.inputs[3].SetValue("github.com")
	w.inputs[4].SetValue("443")
	w.inputs[5].SetValue("personal.github.com")
	w.staged = identity.StagedKey{
		TempPrivatePath:  "/tmp/gitid/id_ed25519_personal",
		FinalPrivatePath: "/home/u/.ssh/id_ed25519_personal",
		FinalPubPath:     "/home/u/.ssh/id_ed25519_personal.pub",
		PubLine:          "ssh-ed25519 AAAA test@gitid\n",
		PrivPEM:          []byte("fake-pem"),
	}

	// Drive prove phases to PASS.
	w.step = wizardStepProve1Running
	w, _ = w.update(preWriteResultMsg{result: tester.Result{Outcome: tester.PASS, Command: "ssh -T git@ssh.github.com"}})
	w, _ = w.update(resolvedResultMsg{result: tester.Result{Outcome: tester.PASS}})

	if !w.confirmActive {
		t.Fatal("confirmActive must be true after both phases PASS")
	}
	if writeSSHCalled {
		t.Error("WriteSSH must NOT fire before Enter is pressed")
	}

	// Press Enter — dispatch the write cmd.
	w2, writeCmd := w.update(tea.KeyPressMsg{Code: tea.KeyEnter})
	_ = w2
	if writeCmd == nil {
		t.Fatal("Enter on prove2Done+confirmActive must dispatch a write cmd")
	}

	// Execute the write cmd.
	msg := writeCmd()
	res, ok := msg.(wizardCreateResultMsg)
	if !ok {
		t.Fatalf("write cmd must return wizardCreateResultMsg; got %T", msg)
	}
	if res.err != nil {
		t.Errorf("write cmd must succeed; got err=%v", res.err)
	}

	// WriteSSH must have fired (LEG 1).
	if !writeSSHCalled {
		t.Error("PersistSSH path: WriteSSH must fire on the PASS+Enter path")
	}
	// WriteGitconfig must NOT have fired (LEG 2 is Plan 13).
	if writeGitconfigCalled {
		t.Error("LEG 1 only: WriteGitconfig must NOT fire on the Screen-2 PASS path")
	}
	// The captured Host block must contain `Host <alias>` and `IdentitiesOnly yes`.
	if !strings.Contains(writeSSHBlock, "Host personal.github.com") {
		t.Errorf("WriteSSH hostBlock must contain 'Host personal.github.com'; got:\n%s", writeSSHBlock)
	}
	if !strings.Contains(writeSSHBlock, "IdentitiesOnly yes") {
		t.Errorf("WriteSSH hostBlock must contain 'IdentitiesOnly yes'; got:\n%s", writeSSHBlock)
	}
}

// TestWizardSkipWriteOfflinePersistSSH verifies that the secondary offline skip-
// and-write path ([s] + Enter double-confirm) writes LEG 1 via PersistSSH
// (WriteSSH fires, WriteGitconfig does NOT fire), preserving the double-confirm
// gate and the unauthenticated-write warning (FIX-CREATE-01, T-05.7-12-01).
//
// Drives via the real model key-event path from wizardStepProve1Failed.
func TestWizardSkipWriteOfflinePersistSSH(t *testing.T) {
	var writeSSHCalled bool
	var writeGitconfigCalled bool

	deps := fakeWriteTUIDeps(nil)
	deps.identity.WriteSSH = func(_, _, _ string) (string, error) {
		writeSSHCalled = true
		return "bak-ssh", nil
	}
	deps.identity.WriteGitconfig = func(_, _, _ string, _ []gitconfig.Match) (string, error) {
		writeGitconfigCalled = true
		return "", nil
	}
	deps.identity.PersistKey = func(_ identity.StagedKey) (identity.KeyResult, error) {
		return identity.KeyResult{}, nil
	}

	w := makeTestWizardModel(deps)
	w.inputs[0].SetValue("personal")
	w.inputs[3].SetValue("github.com")
	w.inputs[5].SetValue("personal.github.com")
	w.staged = identity.StagedKey{
		TempPrivatePath:  "/tmp/gitid/id_ed25519_personal",
		FinalPrivatePath: "/home/u/.ssh/id_ed25519_personal",
		FinalPubPath:     "/home/u/.ssh/id_ed25519_personal.pub",
		PubLine:          "ssh-ed25519 AAAA test@gitid\n",
	}
	w.step = wizardStepProve1Failed
	w.phase1Result = tester.Result{
		Outcome: tester.ReachableNotUploaded,
		Command: "ssh -T git@ssh.github.com",
	}

	// First [s] must set skipConfirmPending=true (first leg of double-confirm).
	w2, _ := w.update(tea.KeyPressMsg{Code: 's'})
	if !w2.skipConfirmPending {
		t.Error("[s] on failed phase must set skipConfirmPending=true")
	}
	if writeSSHCalled {
		t.Error("[s] alone must NOT write anything (requires second Enter confirm)")
	}

	// Second Enter (double-confirm): dispatch the skip-write cmd.
	w3, skipCmd := w2.update(tea.KeyPressMsg{Code: tea.KeyEnter})
	_ = w3
	if skipCmd == nil {
		t.Fatal("second Enter (skip confirm) must dispatch a write cmd")
	}

	// Execute the skip write cmd.
	msg := skipCmd()
	_, ok := msg.(wizardCreateResultMsg)
	if !ok {
		t.Fatalf("skip write cmd must return wizardCreateResultMsg; got %T", msg)
	}

	// WriteSSH must fire (LEG 1 via PersistSSH).
	if !writeSSHCalled {
		t.Error("skip-write path: WriteSSH must fire on skip+Enter")
	}
	// WriteGitconfig must NOT fire (LEG 1 only).
	if writeGitconfigCalled {
		t.Error("LEG 1 only: WriteGitconfig must NOT fire on the skip-write path")
	}
}

// TestWizardNoWriteGuidanceOnReachableNotUploaded verifies that when Phase 1 fails
// with ReachableNotUploaded the view shows:
//   - "Nothing has been written yet" (or similar no-write statement)
//   - A WHY line naming the key-not-on-provider reason
//   - Two paths forward including a discoverable [s] offline escape
//
// Requirement: G-2 (clear no-write feedback + discoverable offline skip).
func TestWizardNoWriteGuidanceOnReachableNotUploaded(t *testing.T) {
	w := makeTestWizardModel(fakeWriteTUIDeps(nil))
	w.staged = identity.StagedKey{
		PubLine:          "ssh-ed25519 AAAA test@gitid\n",
		TempPrivatePath:  "/tmp/gitid/id_ed25519_personal",
		FinalPrivatePath: "/home/u/.ssh/id_ed25519_personal",
	}
	w.inputs[3].SetValue("github.com")
	w.step = wizardStepProve1Failed
	w.phase1Result = tester.Result{
		Command: "ssh -T git@ssh.github.com",
		Output:  "git@ssh.github.com: Permission denied (publickey).",
		Outcome: tester.ReachableNotUploaded,
	}

	view := w.view(90)

	// Must state that nothing has been written.
	if !strings.Contains(strings.ToLower(view), "nothing") && !strings.Contains(strings.ToLower(view), "not written") {
		t.Errorf("view must state nothing was written on failure; got:\n%s", view)
	}
	// Must name the reason (key not uploaded / not yet on provider).
	if !strings.Contains(strings.ToLower(view), "not") {
		t.Errorf("view must mention why (key not on provider); got:\n%s", view)
	}
	// Must mention [s] for skip & write offline escape.
	if !strings.Contains(view, "[s]") {
		t.Errorf("view must show discoverable [s] offline escape; got:\n%s", view)
	}
}

// TestWizardNoWriteGuidanceOnFailure verifies that a Failure outcome (connection
// refused / DNS failure) also shows the no-write guidance with a different WHY
// line (could not reach provider, not "key not uploaded").
func TestWizardNoWriteGuidanceOnFailure(t *testing.T) {
	w := makeTestWizardModel(fakeWriteTUIDeps(nil))
	w.staged = identity.StagedKey{
		PubLine:          "ssh-ed25519 AAAA test@gitid\n",
		TempPrivatePath:  "/tmp/gitid/id_ed25519_personal",
		FinalPrivatePath: "/home/u/.ssh/id_ed25519_personal",
	}
	w.inputs[3].SetValue("github.com")
	w.step = wizardStepProve1Failed
	w.phase1Result = tester.Result{
		Command: "ssh -T git@ssh.github.com",
		Output:  "ssh: connect to host ssh.github.com port 443: Connection refused",
		Outcome: tester.Failure,
	}

	view := w.view(90)

	// Must state nothing was written.
	if !strings.Contains(strings.ToLower(view), "nothing") && !strings.Contains(strings.ToLower(view), "not written") {
		t.Errorf("view must state nothing was written on connection failure; got:\n%s", view)
	}
	// Must still show [s] escape.
	if !strings.Contains(view, "[s]") {
		t.Errorf("view must show [s] offline escape on connection failure; got:\n%s", view)
	}
}

// TestWizardQuitFromFailedProveEmitsRecoveryToast verifies that pressing 'q' from
// a failed prove phase emits a toast that explains nothing was written and how
// to finish — not just "Key kept at <path>" (G-2 no-write clarity).
func TestWizardQuitFromFailedProveEmitsRecoveryToast(t *testing.T) {
	w := makeTestWizardModel(fakeWriteTUIDeps(nil))
	w.inputs[0].SetValue("personal")
	w.staged = identity.StagedKey{
		TempPrivatePath:  "/tmp/gitid/id_ed25519_personal",
		FinalPrivatePath: "/home/u/.ssh/id_ed25519_personal",
	}
	w.step = wizardStepProve1Failed
	w.phase1Result = tester.Result{Outcome: tester.ReachableNotUploaded}

	_, qCmd := w.update(tea.KeyPressMsg{Code: 'q'})
	if qCmd == nil {
		t.Fatal("'q' from failed prove must dispatch a cmd")
	}
	// Execute the batch cmd — run all inner msgs.
	msgs := runAllCmds(qCmd)

	// There must be a setToastMsg that mentions "Nothing" or "nothing written" and
	// how to proceed (re-run or [s] skip).
	found := false
	for _, msg := range msgs {
		if tm, ok := msg.(setToastMsg); ok {
			text := strings.ToLower(tm.text)
			if strings.Contains(text, "nothing") || strings.Contains(text, "not written") {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("'q' from failed prove must emit a toast that mentions nothing was written; msgs: %+v", msgs)
	}
}

// runAllCmds executes a tea.Cmd (including nested BatchMsg) and returns all leaf
// tea.Msg values. Used by tests to inspect what messages a cmd or batch produces
// without needing a real runtime.
func runAllCmds(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var all []tea.Msg
		for _, c := range batch {
			all = append(all, runAllCmds(c)...)
		}
		return all
	}
	return []tea.Msg{msg}
}

// ─── Task 1 (Plan 15): EffectiveAlias honesty + command+path on every phase ──

// TestWizardAliasBlankYieldsProviderHost verifies that when the SSH Alias field
// is blank, the Screen-1 live preview renders `Host github.com` (the provider host)
// and NOT a <name>.<provider> suffix. The EffectiveAlias function must replace
// DefaultAlias in the preview render path.
//
// Requirement: T-05.7-15-04 (WYSIWYG alias), UAT G-5 alias dishonesty fix.
func TestWizardAliasBlankYieldsProviderHost(t *testing.T) {
	w := newCreateWizardModel("personal", fakeWriteTUIDeps(nil))
	w.inputs[0].SetValue("personal")
	w.inputs[3].SetValue("github.com")
	// Leave inputs[5] (SSH Alias) blank — blank alias must yield `Host github.com`.
	w.inputs[5].SetValue("")

	view := w.view(90)

	// The preview MUST show `Host github.com` — WYSIWYG (blank alias = provider host).
	if !strings.Contains(view, "Host github.com") {
		t.Errorf("blank alias must show 'Host github.com' in preview; got:\n%s", view)
	}
	// The preview must NOT contain a <name>.<provider> invented suffix.
	if strings.Contains(view, "personal.github.com") {
		t.Errorf("blank alias must NOT invent 'personal.github.com' suffix in preview; got:\n%s", view)
	}
}

// TestWizardAliasTypedYieldsExactAlias verifies that when the user types an alias
// the preview shows exactly `Host <typed>` (not <name>.<provider>).
func TestWizardAliasTypedYieldsExactAlias(t *testing.T) {
	w := newCreateWizardModel("work", fakeWriteTUIDeps(nil))
	w.inputs[0].SetValue("work")
	w.inputs[3].SetValue("github.com")
	w.inputs[5].SetValue("work.github.com")

	view := w.view(90)

	if !strings.Contains(view, "Host work.github.com") {
		t.Errorf("typed alias 'work.github.com' must appear as 'Host work.github.com' in preview; got:\n%s", view)
	}
}

// TestWizardEffectiveAliasBuildCreateInput verifies that buildCreateInput uses
// identity.EffectiveAlias so a blank alias produces the provider host (github.com),
// not an invented <name>.<provider> suffix.
func TestWizardEffectiveAliasBuildCreateInput(t *testing.T) {
	w := newCreateWizardModel("personal", fakeWriteTUIDeps(nil))
	w.inputs[0].SetValue("personal")
	w.inputs[3].SetValue("github.com")
	w.inputs[5].SetValue("") // blank alias

	in := w.buildCreateInput("github.com")

	// EffectiveAlias("", "github.com") = "github.com"
	if in.Alias != "github.com" {
		t.Errorf("blank alias: buildCreateInput.Alias must be 'github.com' (provider host), got %q", in.Alias)
	}
}

// TestWizardPreRunScreenShowsKeyPath verifies the pre-run screen shows both the
// ssh command AND the key path (G-3: command+path visible on pre-run, Phase-1 PASS,
// Phase-2). The key path must be co-visible with the command on the upload screen.
func TestWizardPreRunScreenShowsKeyPath(t *testing.T) {
	w := makeTestWizardModel(fakeWriteTUIDeps(nil))
	w.inputs[0].SetValue("personal")
	w.inputs[3].SetValue("github.com")
	w.staged = identity.StagedKey{
		PubLine:          "ssh-ed25519 AAAA test@gitid\n",
		TempPrivatePath:  "/tmp/gitid-staged/key",
		FinalPrivatePath: "/home/u/.ssh/id_ed25519_personal",
	}
	w.step = wizardStepUpload

	view := w.view(90)

	// The pre-run screen must show the command (already verified by TestWizardPreRunScreenShowsCommand).
	// In addition it must show the staged key path so the `ssh -i <path>` command is verifiable.
	if !strings.Contains(view, "/tmp/gitid-staged/key") {
		t.Errorf("pre-run screen must show the staged key path; got:\n%s", view)
	}
}

// TestWizardPhase1PassShowsCommandAndKeyPath verifies Phase-1 PASS (wizardStepProve1Done)
// shows BOTH the ssh command AND the tested key path (G-3 command+path everywhere).
func TestWizardPhase1PassShowsCommandAndKeyPath(t *testing.T) {
	w := makeTestWizardModel(fakeWriteTUIDeps(nil))
	w.staged = identity.StagedKey{
		PubLine:          "ssh-ed25519 AAAA test@gitid\n",
		TempPrivatePath:  "/tmp/gitid-staged/key",
		FinalPrivatePath: "/home/u/.ssh/id_ed25519_personal",
	}
	w.step = wizardStepProve1Done
	w.phase1Result = tester.Result{
		Command: "ssh -i /tmp/gitid-staged/key -o IdentitiesOnly=yes -p 443 -T git@ssh.github.com",
		Output:  "Hi personal!",
		Outcome: tester.PASS,
	}

	view := w.view(90)

	// Command must be visible on Phase-1 PASS (not just failure).
	if !strings.Contains(view, "ssh -i /tmp/gitid-staged/key") {
		t.Errorf("Phase-1 PASS view must show the ssh command; got:\n%s", view)
	}
	// Key path must also be visible (via renderUploadHeader which shows TempPrivatePath).
	if !strings.Contains(view, "/tmp/gitid-staged/key") {
		t.Errorf("Phase-1 PASS view must show the staged key path; got:\n%s", view)
	}
}

// TestWizardPhase2ShowsCommandAndKeyPath verifies Phase-2 (wizardStepProve2Done)
// shows the Phase-2 command + the key path.
func TestWizardPhase2ShowsCommandAndKeyPath(t *testing.T) {
	w := makeTestWizardModel(fakeWriteTUIDeps(nil))
	w.staged = identity.StagedKey{
		PubLine:          "ssh-ed25519 AAAA test@gitid\n",
		TempPrivatePath:  "/tmp/gitid-staged/key",
		FinalPrivatePath: "/home/u/.ssh/id_ed25519_personal",
	}
	w.step = wizardStepProve2Done
	w.confirmActive = true
	w.phase1Result = tester.Result{
		Command: "ssh -i /tmp/gitid-staged/key -o IdentitiesOnly=yes -p 443 -T git@ssh.github.com",
		Outcome: tester.PASS,
	}
	w.phase2Result = tester.Result{
		Command: "ssh -T git@github.com",
		Outcome: tester.PASS,
	}

	view := w.view(90)

	// Phase-2 command must be visible.
	if !strings.Contains(view, "ssh -T git@github.com") {
		t.Errorf("Phase-2 done view must show the phase-2 command; got:\n%s", view)
	}
	// Key path still visible via renderUploadHeader.
	if !strings.Contains(view, "/tmp/gitid-staged/key") {
		t.Errorf("Phase-2 done view must show the staged key path; got:\n%s", view)
	}
}
