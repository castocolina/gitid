package dummytui

// batch3_test.go pins review batch 3 — full click-target and focus-ring
// parity with the web demo: clickable contextual footer hints, in-pane
// buttons, checkbox/radio click semantics, the extended Tab/←→ focus
// rings, the divider right-gutter (R1), the master-list `…` clip cue (R2),
// and the honest reserved footer for every key-consuming pane state.
// Click tests locate the needle in the RENDERED frame (batch-1 style).

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// --------------------------------------------------------------------------
// 1. Clickable footer contextual actions.
// --------------------------------------------------------------------------

func TestMouseFooterContextualHintDispatchesItsKey(t *testing.T) {
	a := NewApp()
	a = clickCell(t, a, "n new", 0, a.height-2)
	if got := identModel(t, a).pane; got != paneCreate {
		t.Fatalf("pane = %v after clicking the `n new` footer hint, want paneCreate", got)
	}
}

func TestMouseFooterApplyHintOpensGlobalGitCeremony(t *testing.T) {
	a, _ := press(t, NewApp(), "3")
	a = clickCell(t, a, "a apply 10 selected", 0, a.height-2)
	if !gitModelOf(t, a).ceremonyOpen {
		t.Fatal("clicking the `a apply 10 selected` footer hint must open the apply ceremony")
	}
}

func TestMouseFooterMultiKeyHintsAndReservedRowStayInert(t *testing.T) {
	a := NewApp()
	// Combined navigation hints are not one action — inert.
	a = clickCell(t, a, "↑↓ select identity", 0, a.height-2)
	if m := identModel(t, a); m.pane != paneDetail {
		t.Error("clicking a combined ↑↓ hint must not dispatch anything")
	}
	// The reserved footer line stays keyboard-only.
	a = clickCell(t, a, "q quit", 0, a.height-1)
	if a.overlay != overlayNone {
		t.Error("clicking the reserved `q quit` must not open the quit prompt")
	}
}

// --------------------------------------------------------------------------
// 2. Clickable in-pane buttons.
// --------------------------------------------------------------------------

func TestMouseEditSSHRewriteButtonOpensCeremony(t *testing.T) {
	a := pressSeq(t, NewApp(), "e")
	a = clickCell(t, a, "Rewrite Host block", 0, frameBodyTop)
	if got := identModel(t, a).pane; got != paneEditCeremony {
		t.Fatalf("pane = %v after clicking the Rewrite button, want paneEditCeremony", got)
	}
}

func TestMouseCeremonyButtonsCancelConfirmDone(t *testing.T) {
	// Cancel returns to the form without dispatching.
	a := pressSeq(t, NewApp(), "e", "enter")
	a = clickCell(t, a, "Cancel (Esc)", 0, frameBodyTop)
	if got := identModel(t, a).pane; got != paneEditSSH {
		t.Fatalf("pane = %v after clicking Cancel, want paneEditSSH", got)
	}
	// Confirm writes (state B), Done dismisses and dispatches.
	a = pressSeq(t, a, "enter")
	a = clickCell(t, a, "Save changes (Enter)", 0, frameBodyTop)
	if !strings.Contains(appView(a), "Wrote →") {
		t.Fatal("clicking the confirm button must reach the receipt")
	}
	a = clickCell(t, a, "Done (Enter)", 0, frameBodyTop)
	if got := identModel(t, a).pane; got != paneDetail {
		t.Errorf("pane = %v after clicking Done, want paneDetail", got)
	}
	if a.note != `SSH settings of "personal" updated.` {
		t.Errorf("note = %q — Done must dispatch the EditSSH action path", a.note)
	}
}

func TestMouseWizardGitStepButtonsClick(t *testing.T) {
	a := wizardThroughTest(t, identitiesApp())
	a = clickCell(t, a, "[ Skip Git ]", 0, frameBodyTop)
	m := identModel(t, a)
	if m.wizard.step != 3 || m.wizard.configureGit {
		t.Fatalf("step = %d configureGit = %v after clicking Skip, want 3/false", m.wizard.step, m.wizard.configureGit)
	}
	// The wizard ceremony's Cancel click returns to the Git step.
	a = clickCell(t, a, "Cancel (Esc)", 0, frameBodyTop)
	if got := identModel(t, a).wizard.step; got != 2 {
		t.Fatalf("step = %d after clicking the ceremony Cancel, want 2", got)
	}
	a = clickCell(t, a, "Back (Esc)", 0, frameBodyTop)
	if got := identModel(t, a).wizard.step; got != 1 {
		t.Errorf("step = %d after clicking Back, want 1 (test step)", got)
	}
}

func TestMouseWizardTestStepButtonsClick(t *testing.T) {
	a := wizardToStep2(t, identitiesApp())
	a = clickCell(t, a, "simulate a provider failure", 0, frameBodyTop)
	if !identModel(t, a).wizard.simulateFail {
		t.Fatal("clicking the demo-failure toggle must toggle it like space")
	}
	a = clickCell(t, a, "simulate a provider failure", 0, frameBodyTop)
	a = clickCell(t, a, "Run stage 1 (Enter)", 0, frameBodyTop)
	if got := identModel(t, a).wizard.testPhase; got != testRunning1 {
		t.Errorf("testPhase = %q after clicking Run stage 1, want %q", got, testRunning1)
	}
}

func TestMouseConfigureNowAndPerFindingFixClick(t *testing.T) {
	// `[Configure now (g)]` on an identity without a Git side.
	a := NewApp()
	a = clickCell(t, a, "work", sidebarWidth(a.width), frameBodyTop)
	a = clickCell(t, a, "[Configure now (g)]", 0, frameBodyTop)
	if got := identModel(t, a).pane; got != paneGit {
		t.Fatalf("pane = %v after clicking Configure now, want paneGit", got)
	}

	// A per-finding `Fix…` opens THAT finding's fix ceremony.
	b := NewApp()
	b = clickCell(t, b, "archived", sidebarWidth(b.width), frameBodyTop)
	b = clickCell(t, b, "Fix…", 0, frameBodyTop)
	m := identModel(t, b)
	if m.pane != paneFix || m.fixFindingID != "ssh-key-perms-archived" {
		t.Fatalf("pane = %v fixFindingID = %q after clicking Fix…, want paneFix/ssh-key-perms-archived",
			m.pane, m.fixFindingID)
	}
}

func TestMouseCloneButtonClones(t *testing.T) {
	a := pressSeq(t, NewApp(), "c")
	a = clickCell(t, a, "Clone (Enter)", 0, frameBodyTop)
	if got := identModel(t, a).selected; got != "personal-clone" {
		t.Fatalf("selected = %q after clicking Clone, want personal-clone", got)
	}
	if len(a.state.Identities) != 9 {
		t.Error("clicking Clone must dispatch CloneIdentity")
	}
}

func TestMouseDeleteScopeRowChoosesThatScope(t *testing.T) {
	a := pressSeq(t, NewApp(), "d")
	a = clickCell(t, a, "Delete everything (SSH + Git + key)", 0, frameBodyTop)
	m := identModel(t, a)
	if m.pane != paneDeleteScope || m.deleteScope != "everything" {
		t.Fatalf("pane = %v scope = %q after clicking the everything row, want chooser/everything", m.pane, m.deleteScope)
	}
	a = clickCell(t, a, "Delete Git identity only", 0, frameBodyTop)
	if got := identModel(t, a).deleteScope; got != "git-only" {
		t.Errorf("scope = %q after clicking the git-only row, want git-only", got)
	}
}

func TestMouseStorageRadioAndMigrateButton(t *testing.T) {
	a := pressSeq(t, NewApp(), "2", "right") // → Storage & preview
	a = clickCell(t, a, "gitid-owned", masterListWidth(a.width), frameBodyTop)
	if got := gssModelOf(t, a).storageChoice; got != StorageInclude {
		t.Fatalf("storageChoice = %q after clicking the include radio row, want include", got)
	}
	a = clickCell(t, a, "Migrate layout… (Enter)", 0, frameBodyTop)
	if got := gssModelOf(t, a).mode; got != gssStorageCeremony {
		t.Fatalf("mode = %v after clicking Migrate, want the storage ceremony", got)
	}
	// The ceremony's confirm button clicks through too.
	a = clickCell(t, a, "Migrate (Enter)", 0, frameBodyTop)
	a = clickCell(t, a, "Done (Enter)", 0, frameBodyTop)
	if a.state.SSHStorage != StorageInclude {
		t.Error("the clicked migrate ceremony must dispatch SetSSHStorage")
	}
}

func TestMouseDoctorFixThisButtonAndCeremonyCancel(t *testing.T) {
	a := doctorApp(t)
	a = clickCell(t, a, "Fix this…", 0, frameBodyTop)
	if !docModel(t, a).fixing {
		t.Fatal("clicking `f · Fix this…` must open the fix ceremony")
	}
	a = clickCell(t, a, "Cancel (Esc)", 0, frameBodyTop)
	if docModel(t, a).fixing {
		t.Error("clicking the fix ceremony's Cancel must close it")
	}
}

// --------------------------------------------------------------------------
// 3. Checkbox / radio click semantics.
// --------------------------------------------------------------------------

func TestMouseGlobalSSHCheckboxCellTogglesWithoutSelecting(t *testing.T) {
	a, _ := press(t, NewApp(), "2")
	if !gssModelOf(t, a).chosen["StrictHostKeyChecking"] {
		t.Fatal("fixture: StrictHostKeyChecking must start chosen")
	}
	// The first ☑ in the list is StrictHostKeyChecking's checkbox cell.
	a = clickCell(t, a, "☑", masterListWidth(a.width), frameBodyTop)
	m := gssModelOf(t, a)
	if m.chosen["StrictHostKeyChecking"] {
		t.Error("clicking the ☑ cell must uncheck the row (like space)")
	}
	if m.detailKey != "IdentitiesOnly" {
		t.Errorf("detailKey = %q — the checkbox click must NOT move the selection", m.detailKey)
	}
	// On a fresh screen the first ☐ is ForwardAgent's (the fixture
	// decline) — clicking the empty checkbox cell checks it.
	b, _ := press(t, NewApp(), "2")
	b = clickCell(t, b, "☐", masterListWidth(b.width), frameBodyTop)
	if !gssModelOf(t, b).chosen["ForwardAgent"] {
		t.Error("clicking the ☐ cell must check the row")
	}
}

func TestMouseGlobalGitCheckboxCellToggles(t *testing.T) {
	a, _ := press(t, NewApp(), "3")
	// Skip past init.defaultBranch (row 0) — click core.ignorecase's ☑.
	a = clickCell(t, a, "☑", masterListWidth(a.width), frameBodyTop+3)
	m := gitModelOf(t, a)
	if m.chosen["core.ignorecase"] {
		t.Error("clicking the ☑ cell must uncheck core.ignorecase")
	}
	if m.detailKey != "init.defaultBranch" {
		t.Errorf("detailKey = %q — the checkbox click must NOT move the selection", m.detailKey)
	}
}

// --------------------------------------------------------------------------
// 4. Focus rings + ←/→ button movement.
// --------------------------------------------------------------------------

func TestEditSSHFocusRingReachesRewriteButton(t *testing.T) {
	a := pressSeq(t, NewApp(), "e", "tab", "tab", "tab") // host → hostname → port → button
	m := identModel(t, a)
	if m.editFocus != editFocusButton {
		t.Fatalf("editFocus = %d after 3 tabs, want the Rewrite button (%d)", m.editFocus, editFocusButton)
	}
	raw := a.View().Content
	if !strings.Contains(raw, "\x1b[1;7m Rewrite Host block… (Enter) ") &&
		!strings.Contains(raw, "\x1b[7;1m Rewrite Host block… (Enter) ") {
		t.Error("the focused Rewrite button must render reverse-video")
	}
	a, _ = press(t, a, "enter")
	if got := identModel(t, a).pane; got != paneEditCeremony {
		t.Errorf("pane = %v after Enter on the focused button, want paneEditCeremony", got)
	}
	// The ring wraps back to the first field.
	b := pressSeq(t, NewApp(), "e", "tab", "tab", "tab", "tab")
	if got := identModel(t, b).editFocus; got != sshFieldHost {
		t.Errorf("editFocus = %d after the full ring, want host (%d)", got, sshFieldHost)
	}
}

func TestCloneFocusRingInputToButton(t *testing.T) {
	a := pressSeq(t, NewApp(), "c", "tab")
	m := identModel(t, a)
	if !m.cloneOnButton {
		t.Fatal("Tab must move the clone focus onto the Clone button")
	}
	// Typing while the button is focused must not reach the name input.
	a = typeText(t, a, "x")
	if got := identModel(t, a).cloneInput.Value(); got != "personal-clone" {
		t.Errorf("clone name = %q — typing on the button must not edit the input", got)
	}
	a, _ = press(t, a, "enter")
	if got := identModel(t, a).selected; got != "personal-clone" {
		t.Errorf("Enter on the focused Clone button must clone; selected = %q", got)
	}
}

func TestDeleteScopeRingTabAndArrows(t *testing.T) {
	a := pressSeq(t, NewApp(), "d", "tab")
	if got := identModel(t, a).deleteScope; got != "everything" {
		t.Fatalf("scope = %q after Tab, want everything (the 2-option ring)", got)
	}
	a, _ = press(t, a, "left")
	if got := identModel(t, a).deleteScope; got != "git-only" {
		t.Errorf("scope = %q after ←, want git-only", got)
	}
	// The chosen scope renders reverse-video (visible focus).
	raw := a.View().Content
	if !strings.Contains(raw, "\x1b[1;7m"+IdentityManagerDeleteChoiceGitOnly) {
		t.Error("the focused scope option must render reverse-video")
	}
}

func TestCeremonyTabRingAndEnterActivatesFocused(t *testing.T) {
	a := pressSeq(t, NewApp(), "e", "enter") // edit-SSH ceremony (non-destructive)
	c := identModel(t, a).editCeremony
	if c.focus != ceremonyFocusPrimary {
		t.Fatal("a fresh ceremony must start in the primary focus state")
	}
	a, _ = press(t, a, "tab")
	if got := identModel(t, a).editCeremony.focus; got != ceremonyFocusConfirm {
		t.Fatalf("focus = %v after Tab, want the affirmative", got)
	}
	a, _ = press(t, a, "tab")
	if got := identModel(t, a).editCeremony.focus; got != ceremonyFocusCancel {
		t.Fatalf("focus = %v after Tab Tab, want Cancel", got)
	}
	// Enter on the explicitly focused Cancel cancels (never writes).
	before := len(a.state.Backups)
	a, _ = press(t, a, "enter")
	m := identModel(t, a)
	if m.pane != paneEditSSH {
		t.Errorf("pane = %v after Enter on focused Cancel, want paneEditSSH", m.pane)
	}
	if len(a.state.Backups) != before {
		t.Error("Enter on Cancel must not dispatch")
	}
}

func TestCeremonyArrowsMoveButtonFocusNonDestructive(t *testing.T) {
	a := pressSeq(t, NewApp(), "e", "enter", "right")
	if got := identModel(t, a).editCeremony.focus; got != ceremonyFocusConfirm {
		t.Errorf("focus = %v after →, want the affirmative", got)
	}
}

func TestCeremonyDestructiveArrowsStayOnTypedInput(t *testing.T) {
	a := pressSeq(t, NewApp(), "d", "tab", "enter") // everything → destructive ceremony
	if got := identModel(t, a).pane; got != paneDelete {
		t.Fatalf("pane = %v, want paneDelete", got)
	}
	a, _ = press(t, a, "left") // cursor movement inside the typed-confirm input
	if got := identModel(t, a).deleteCerem.focus; got != ceremonyFocusPrimary {
		t.Errorf("focus = %v after ← in the primary state, want primary (input keeps ←/→)", got)
	}
	a, _ = press(t, a, "tab")
	if got := identModel(t, a).deleteCerem.focus; got != ceremonyFocusConfirm {
		t.Errorf("focus = %v after Tab, want the affirmative slot", got)
	}
}

// TestWizardGitButtonsArrowNavigatesWizardSteps pins the round-3/02-STYLE-
// SPEC.md arrow-key precedence rule: a button slot (Back/Skip/Continue) is a
// NON-EDITING focus region, so <-/-> now perform WIZARD-STEP navigation —
// this REPLACES the old button-ring-arrow-movement behavior (the ring
// itself moves via Tab/Shift+Tab/Up/Down only, still true at
// TestWizardGitStepButtonsAreFocusable).
func TestWizardGitButtonsArrowNavigatesWizardSteps(t *testing.T) {
	a := wizardThroughTest(t, identitiesApp())
	a = pressSeq(t, a, "tab", "tab", "tab") // → Back button (non-editing focus)
	if got := identModel(t, a).wizard.gitFocus; got != gitFocusBack {
		t.Fatalf("gitFocus = %d after 3 tabs, want Back (%d)", got, gitFocusBack)
	}
	// Forward is validity-gated (the default git form values are valid) —
	// never a validity override, even from the Back button's focus.
	a, _ = press(t, a, "right")
	if got := identModel(t, a).wizard.step; got != 3 {
		t.Fatalf("step = %d after → from the Back button, want 3 (validity-gated forward)", got)
	}
	// Cancel the ceremony back to step 2 — gitFocus is untouched by the
	// cancel (still the Back button) — then ← always goes back to step 1.
	a, _ = press(t, a, "esc")
	m := identModel(t, a)
	if m.wizard.step != 2 {
		t.Fatalf("esc from the ceremony must return to step 2, got %d", m.wizard.step)
	}
	if m.wizard.gitFocus != gitFocusBack {
		t.Fatalf("gitFocus = %d after cancelling the ceremony, want it untouched at Back (%d)", m.wizard.gitFocus, gitFocusBack)
	}
	a, _ = press(t, a, "left")
	if got := identModel(t, a).wizard.step; got != 1 {
		t.Errorf("step = %d after ← from the Back button, want 1 (back always allowed)", got)
	}
}

// TestWizardShiftArrowIsFocusOverrideNotValidityOverride pins clause 5: the
// Shift+<-/-> chord reaches wizard step-nav even while focus is on a TEXT
// FIELD (name), but forward STILL respects the step-2 validity gate.
func TestWizardShiftArrowIsFocusOverrideNotValidityOverride(t *testing.T) {
	a := wizardThroughTest(t, identitiesApp()) // focus: git name field
	// Make the Git form invalid (blank the name) — Shift+Right must NOT
	// bypass the validity gate.
	for i := 0; i < 20; i++ {
		model, _ := a.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
		a = model.(App)
	}
	model, _ := a.Update(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModShift})
	a = model.(App)
	if got := identModel(t, a).wizard.step; got != 2 {
		t.Fatalf("step = %d after shift+right with an invalid Git form (focus on a field), want 2 (blocked)", got)
	}
	// Shift+Left is a focus override reaching step-nav from the SAME field
	// focus, and back is always allowed.
	model, _ = a.Update(tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModShift})
	a = model.(App)
	if got := identModel(t, a).wizard.step; got != 1 {
		t.Errorf("step = %d after shift+left from a focused field, want 1 (focus-override back)", got)
	}
}

// --------------------------------------------------------------------------
// 8. Honest reserved footer for every key-consuming pane state.
// --------------------------------------------------------------------------

func TestReservedFooterHonestInKeyConsumingStates(t *testing.T) {
	cases := []struct {
		name string
		app  func(t *testing.T) App
	}{
		{"wizard test step", func(t *testing.T) App { return wizardToStep2(t, identitiesApp()) }},
		{"wizard algorithm select", func(t *testing.T) App {
			return pressSeq(t, identitiesApp(), "n", "tab", "tab", "tab", "tab") // prefix → … → algorithm
		}},
		{"wizard git buttons", func(t *testing.T) App {
			return pressSeq(t, wizardThroughTest(t, identitiesApp()), "tab", "tab", "tab")
		}},
		{"delete-scope chooser", func(t *testing.T) App { return pressSeq(t, identitiesApp(), "d") }},
		{"global ssh apply ceremony", func(t *testing.T) App { return pressSeq(t, NewApp(), "2", "a") }},
		{"global git apply ceremony", func(t *testing.T) App { return pressSeq(t, NewApp(), "3", "a") }},
		{"doctor fix ceremony", func(t *testing.T) App { return pressSeq(t, doctorApp(t), "f") }},
	}
	for _, tc := range cases {
		view := appView(tc.app(t))
		for _, forbidden := range []string{"q quit", "? help"} {
			if strings.Contains(view, forbidden) {
				t.Errorf("%s: footer must not advertise %q — the pane consumes plain keys", tc.name, forbidden)
			}
		}
		if !strings.Contains(view, "Esc back") || !strings.Contains(view, "Ctrl+P palette") {
			t.Errorf("%s: honest reserved footer missing Esc/Ctrl+P", tc.name)
		}
	}
}

// --------------------------------------------------------------------------
// R2 — master-list `now:` values clip with a visible ellipsis.
// --------------------------------------------------------------------------

func TestOptionRowNowValueClipsWithEllipsis(t *testing.T) {
	long := optionRow("StrictHostKeyChecking",
		"not set (OpenSSH default: ask)", "ask", "Medium", true, true, false, false, 44)
	lines := strings.Split(stripANSI(long), "\n")
	if len(lines) != optionRowLines {
		t.Fatalf("option row = %d lines, want %d", len(lines), optionRowLines)
	}
	if !strings.HasSuffix(lines[1], "…") {
		t.Errorf("a clipped `now:` line must end with …; got %q", lines[1])
	}
	short := stripANSI(optionRow("K", "a", "b", "", true, false, false, false, 44))
	if strings.Contains(short, "…") {
		t.Errorf("an unclipped row must not carry the … cue; got %q", short)
	}
}
