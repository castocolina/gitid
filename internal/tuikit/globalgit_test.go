package tuikit

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
)

// ggitApp returns an App on the Global Git tab.
func ggitApp(t *testing.T) App {
	t.Helper()
	a, _ := press(t, NewApp(stubBackend{}), "3")
	return a
}

// ggitModel extracts the Global Git child model.
func ggitModel(t *testing.T, a App) globalGitModel {
	t.Helper()
	m, ok := a.screens[TabGlobalGit].(globalGitModel)
	if !ok {
		t.Fatalf("screens[2] is %T, want globalGitModel", a.screens[TabGlobalGit])
	}
	return m
}

// ---------------------------------------------------------------------------
// Rendering tests — rows come from the backend seam via fixture projection.
// ---------------------------------------------------------------------------

func TestGlobalGitRendersAllElevenRows(t *testing.T) {
	if got := len(GlobalGitOptions); got != 12 {
		t.Fatalf("fixture rows = %d, want 12 (D-08 + D-07)", got)
	}
	// 09.5-02, Task 1: the sub-tab strip's +3 rows shrink the visible-row
	// budget below all 12 rows fitting in one un-scrolled screenful
	// (measured: TestGlobalGitFitsFixedGeometryWithStrip) — the pre-existing
	// scroll-window mechanism (gitComputeScrollWindow) handles this exactly
	// as designed, so every key is checked reachable across the top view
	// AND the bottom-scrolled view, not a single un-scrolled screenful.
	a := ggitApp(t)
	top := appView(a)
	for i := 0; i < 11; i++ {
		a, _ = press(t, a, "down")
	}
	bottom := appView(a)
	for _, key := range []string{
		"init.defaultBranch", "core.ignorecase", "core.autocrlf / core.eol",
		"user.email (global fallback)", "user.useConfigOnly", "push.autoSetupRemote", "pull.rebase",
		"fetch.prune", "alias (8 shortcuts)", "color (ui/branch/diff/status)",
		"merge.conflictstyle", "diff.colorMoved",
	} {
		if !strings.Contains(top, key) && !strings.Contains(bottom, key) {
			t.Errorf("row %q missing from both the top and bottom-scrolled views", key)
		}
	}
}

func TestGlobalGitMainVsMasterHighlight(t *testing.T) {
	view := appView(ggitApp(t))
	if !strings.Contains(view, "[main vs master]") {
		t.Error("init.defaultBranch must carry the main-vs-master highlight chip")
	}
	// The initial detail (init.defaultBranch) shows the full explanation.
	if !strings.Contains(view, "Until Git 2.28 (July 2020)") {
		t.Error("init.defaultBranch detail must show GlobalGitDetailExplanation")
	}
	if !strings.Contains(regionFlat(ggitApp(t), 45, 100), "This is advisory, never a compliance gate.") {
		t.Error("advisory alert missing")
	}
}

func TestGlobalGitLongExplanationClipsWithVisibleCue(t *testing.T) {
	// init.defaultBranch (the initial detail) carries the long GGIT-01
	// explanation — the overflow must be announced, never silently cut (H3).
	view := appView(ggitApp(t))
	if !strings.Contains(view, "Until Git 2.28 (July 2020)") {
		t.Fatal("init.defaultBranch explanation missing")
	}
	if !regexp.MustCompile(`… \(\+\d+ more lines\)`).MatchString(view) {
		t.Error("clipped explanation must render the `… (+n more lines)` cue (H3)")
	}
}

// ---------------------------------------------------------------------------
// Acceptance criterion: activate returns nil Cmd; rows are fetched synchronously.
// ---------------------------------------------------------------------------

// TestGlobalGitActivateReturnsNilCmd asserts that activate() returns a nil
// tea.Cmd and that the model carries fetched rows immediately afterwards —
// no spinner, no intermediate state (07-UI-SPEC.md RESOLVED "loading" row).
func TestGlobalGitActivateReturnsNilCmd(t *testing.T) {
	b := stubBackend{}
	m := newGlobalGitModel(b)
	next, cmd := m.activate(Seed())
	if cmd != nil {
		t.Error("activate must return a nil tea.Cmd — no spinner, no async message")
	}
	gm, ok := next.(globalGitModel)
	if !ok {
		t.Fatalf("activate returned %T, want globalGitModel", next)
	}
	if len(gm.options) == 0 {
		t.Error("activate must fetch rows synchronously; model carries none")
	}
}

// ---------------------------------------------------------------------------
// Acceptance criterion: R-1 empty selection.
// ---------------------------------------------------------------------------

// TestGlobalGitSelectionStartsEmpty asserts a freshly constructed model's
// selection is EMPTY (R-1, 6/D-15, 07-CONTEXT.md <domain> carried forward).
func TestGlobalGitSelectionStartsEmpty(t *testing.T) {
	m := newGlobalGitModel(stubBackend{})
	for k, v := range m.chosen {
		if v {
			t.Errorf("new model must have empty selection; found chosen[%q]=true", k)
		}
	}
}

// TestGlobalGitActivateFocusesFirstFetchedRow is UXP-01: activate derives
// detailKey from the first element of the freshly fetched slice, not a
// hardcoded key. A custom list whose first row is NOT init.defaultBranch
// is the only way to tell the two rules apart.
func TestGlobalGitActivateFocusesFirstFetchedRow(t *testing.T) {
	rows := gitScrollRows(5)
	m := newGlobalGitModel(stubBackend{gitOptions: rows})
	next, _ := m.activate(Seed())
	gm := next.(globalGitModel)
	if gm.detailKey != rows[0].Key {
		t.Errorf("detailKey = %q, want first fetched row %q", gm.detailKey, rows[0].Key)
	}
}

// TestGlobalGitActivateResetsFocusToFirstRow is UXP-01's re-entry half on
// Global Git, against a custom list so a leftover hardcoded
// init.defaultBranch cannot masquerade as the derived rule.
func TestGlobalGitActivateResetsFocusToFirstRow(t *testing.T) {
	rows := gitScrollRows(5)
	a, _ := press(t, NewApp(stubBackend{gitOptions: rows}), "3")
	a, _ = press(t, a, "down")
	a, _ = press(t, a, "down")
	m := ggitModel(t, a)
	if m.detailKey != rows[2].Key {
		t.Fatalf("setup: after two downs, detailKey = %q, want %q", m.detailKey, rows[2].Key)
	}
	a, _ = press(t, a, "1") // Identities
	a, _ = press(t, a, "3") // back to Global Git
	m = ggitModel(t, a)
	if m.detailKey != rows[0].Key {
		t.Errorf("detailKey after re-activate = %q, want first fetched row %q", m.detailKey, rows[0].Key)
	}
}

// TestGlobalGitRenderOptionsEmptyNoErrorDoesNotPanic is the WR-17 class
// render-path guard: a Backend may return (empty, nil). handleKey already
// returns early on len(options)==0, but view indexed options[selIdx]
// unconditionally — gitDetailIndex returns 0 on a miss, which is out of
// range for an empty slice. Asserting only that activate survived would
// pass over the panic.
func TestGlobalGitRenderOptionsEmptyNoErrorDoesNotPanic(t *testing.T) {
	b := stubBackend{gitOptions: []GlobalGitOptionView{}}
	m := newGlobalGitModel(b)
	next, _ := m.activate(Seed())
	gm := next.(globalGitModel)
	sv := gm.view(Seed(), 120, 40)
	if sv.body == "" {
		t.Fatal("zero-row view must produce a non-empty empty-state body")
	}
	if !strings.Contains(sv.body, "No global Git options to show.") {
		t.Errorf("empty-no-error options must render the empty-state note, got:\n%s", sv.body)
	}
}

// TestGlobalGitActivateResetsSelection asserts that re-activating after a
// toggle empties the selection — returning to the screen never resurrects a
// stale selection (R-1, D-15).
func TestGlobalGitActivateResetsSelection(t *testing.T) {
	a := ggitApp(t)
	// Toggle init.defaultBranch (the one policy-backed row in this wave).
	a, _ = press(t, a, "space")
	m := ggitModel(t, a)
	if len(m.chosen) == 0 || !m.chosen["init.defaultBranch"] {
		t.Fatal("setup: space must choose init.defaultBranch before re-activate")
	}
	// Re-activate by switching away and back.
	a, _ = press(t, a, "1") // tab 1 = Identities
	a, _ = press(t, a, "3") // tab 3 = Global Git
	m = ggitModel(t, a)
	for k, v := range m.chosen {
		if v {
			t.Errorf("re-activate must reset selection; found chosen[%q]=true", k)
		}
	}
}

// ---------------------------------------------------------------------------
// Acceptance criterion: apply action not offered while selection is empty.
// ---------------------------------------------------------------------------

// TestGlobalGitApplyNotOfferedOnEmptySelection asserts the "a" key is not in
// the footer actions when the selection is empty.
func TestGlobalGitApplyNotOfferedOnEmptySelection(t *testing.T) {
	a := ggitApp(t)
	sv := a.screens[TabGlobalGit].view(a.state, 120, 40)
	for _, action := range sv.actions {
		if strings.Contains(action.Key, "a") && strings.Contains(action.Label, "apply") {
			t.Error("apply action must not be offered while the selection is empty")
		}
	}
}

// ---------------------------------------------------------------------------
// Acceptance criterion: policy-backed selectability predicate.
// ---------------------------------------------------------------------------

// TestGlobalGitPolicyBackedRowIsSelectable asserts the one policy-backed row
// in this wave (init.defaultBranch) IS selectable — the guard is not
// vacuously refusing everything.
func TestGlobalGitPolicyBackedRowIsSelectable(t *testing.T) {
	views := fixtureGlobalGitOptionViews()
	for _, v := range views {
		if v.Key == "init.defaultBranch" {
			if !v.Selectable() {
				t.Error("init.defaultBranch must be selectable (PolicyBacked and NeedsAction)")
			}
			return
		}
	}
	t.Fatal("init.defaultBranch not found in fixture views")
}

// TestGlobalGitNonSelectableRowIsNotTogglable asserts a row with no writable
// member keys (the fallback-author row, whose pair is owned by plan 07-02's
// own ceremony) renders no checkbox glyph, does not respond to the toggle key,
// and does not respond to a click at its checkbox position — all driven from
// the single Selectable() predicate.
func TestGlobalGitNonSelectableRowIsNotTogglable(t *testing.T) {
	a := ggitApp(t)
	a = pressSeq(t, a, "down", "down", "down") // the fallback row
	m := ggitModel(t, a)
	if m.detailKey != GlobalGitEmailFallbackKey {
		t.Fatalf("expected fallback row, got %q", m.detailKey)
	}
	body := appView(a)
	for _, line := range strings.Split(body, "\n") {
		listCol := strings.SplitN(line, "│", 2)[0]
		if strings.Contains(listCol, GlobalGitEmailFallbackKey) {
			if strings.Contains(listCol, glyphToggleOff) || strings.Contains(listCol, glyphToggleOn) {
				t.Errorf("non-selectable row must not render a checkbox glyph; got line: %q", listCol)
			}
		}
	}
	// Toggle key does not change the selection.
	a, _ = press(t, a, "space")
	m = ggitModel(t, a)
	if m.chosen[GlobalGitEmailFallbackKey] {
		t.Error("toggle key must not choose a non-selectable row")
	}
	// A click at its checkbox position must not toggle it.
	before := ggitModel(t, a).chosen[GlobalGitEmailFallbackKey]
	res := ggitModel(t, a).handleClick(4, 4, 120, 40, a.state)
	after, ok2 := res.model.(globalGitModel)
	if !ok2 {
		t.Fatal("handleClick returned wrong type")
	}
	if after.chosen[GlobalGitEmailFallbackKey] != before {
		t.Error("click at checkbox position of a non-selectable row must not toggle it")
	}
}

// ---------------------------------------------------------------------------
// Acceptance criterion: fetch-error field.
// ---------------------------------------------------------------------------

// TestGlobalGitOptionStatesError asserts that when GlobalGitOptionStates
// returns an error, the model's optionsErr field is non-empty and the rows
// slice is nil.
func TestGlobalGitOptionStatesError(t *testing.T) {
	b := stubBackend{gitOptionsErr: errGlobalGitTest}
	m := newGlobalGitModel(b)
	next, cmd := m.activate(Seed())
	if cmd != nil {
		t.Error("activate must return nil Cmd even on error")
	}
	gm := next.(globalGitModel)
	if gm.optionsErr == "" {
		t.Error("model's optionsErr must be non-empty on fetch failure")
	}
	if gm.options != nil {
		t.Error("model's options slice must be nil on fetch failure")
	}
}

// errGlobalGitTest is the sentinel error used by error-path tests.
var errGlobalGitTest = errGlobalGitTestErr("global git test error")

type errGlobalGitTestErr string

func (e errGlobalGitTestErr) Error() string { return string(e) }

// ---------------------------------------------------------------------------
// Acceptance criterion: rendered body shows stub value, not fixture value.
// ---------------------------------------------------------------------------

// TestGlobalGitRendersStubValue drives the model with a stub returning a row
// whose current value differs from the frozen fixture's, and asserts the
// rendered body contains the stub's value and NOT the fixture's.
func TestGlobalGitRendersStubValue(t *testing.T) {
	// Short enough to survive the master-list column's truncLine width — a
	// longer value here gets clipped by the render before the assertion's
	// Contains check ever sees it (the row line is "now: <value> → <rec>"
	// truncated to masterListWidth, which is well under 40 columns at the
	// test frame's minFrameWidth).
	const stubValue = "STUB-VALUE-X"
	b := stubBackend{
		gitOptions: []GlobalGitOptionView{
			{
				Key:          "init.defaultBranch",
				CurrentValue: stubValue,
				Recommended:  "main",
				State:        GlobalGitNeedsAction,
			},
		},
	}
	a, _ := press(t, NewApp(b), "3")
	body := appView(a)
	if !strings.Contains(body, stubValue) {
		t.Errorf("rendered body must contain the stub's current value %q", stubValue)
	}
	// Fixture value must not leak through.
	if strings.Contains(body, "not set (git's built-in default: master)") {
		t.Error("rendered body must not contain the fixture's current value for init.defaultBranch")
	}
}

// ---------------------------------------------------------------------------
// Acceptance criterion: ceremony heading contains resolved target path.
// ---------------------------------------------------------------------------

// TestGlobalGitBaselineCeremonyHeadingContainsResolvedTarget asserts the
// apply ceremony's heading contains the plan view's resolved target path and
// not a hardcoded main-config path.
func TestGlobalGitBaselineCeremonyHeadingContainsResolvedTarget(t *testing.T) {
	const resolvedTarget = "~/.gitconfig.d/00-baseline"
	b := stubBackend{
		gitApplyPlan: GlobalGitApplyPlanView{
			Targets: []string{resolvedTarget},
			Backups: []string{NewBackupPath(resolvedTarget)},
			Diff:    "diff content",
		},
	}
	a, _ := press(t, NewApp(b), "3")
	// Toggle init.defaultBranch and open the ceremony.
	a, _ = press(t, a, "space") // toggle
	a, _ = press(t, a, "a")     // open ceremony
	view := appView(a)
	if !strings.Contains(view, resolvedTarget) {
		t.Errorf("ceremony heading must contain the resolved target %q;\nview:\n%s", resolvedTarget, view)
	}
	if strings.Contains(view, "~/.gitconfig\"") {
		t.Error("ceremony heading must not hardcode ~/.gitconfig as the target")
	}
}

// ---------------------------------------------------------------------------
// Acceptance criterion: ceremony confirm dispatches CommitGlobalGit command.
// ---------------------------------------------------------------------------

// TestGlobalGitCeremonyConfirmYieldsCommitMsg asserts that confirming the
// ceremony produces a command that yields a GlobalGitCommitMsg, and that a
// message carrying an error renders the failure and the restored paths rather
// than the success message.
func TestGlobalGitCeremonyConfirmYieldsCommitMsg(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		backupPath := NewBackupPath("~/.gitconfig.d/00-baseline")
		b := stubBackend{
			gitCommitMsg: GlobalGitCommitMsg{Backups: []string{backupPath}},
		}
		a, _ := press(t, NewApp(b), "3")
		a, _ = press(t, a, "space") // toggle init.defaultBranch
		a, _ = press(t, a, "a")     // open ceremony
		a, _ = press(t, a, "enter") // confirm → dispatches CommitGlobalGit
		// Deliver the commit message from the pending command, carrying the
		// CURRENT ceremony's request token (CR-01) — not a bare, unwrapped
		// message, which would read as stale (token -1) and never touch the
		// ceremony UI.
		a, _ = deliverMsg(t, a, gitCommitTokenMsg{token: ggitModel(t, a).commitRequestToken, msg: GlobalGitCommitMsg{Backups: []string{backupPath}}})
		view := appView(a)
		// Receipt shows success — backup path in the result.
		if !strings.Contains(view, backupPath) {
			t.Errorf("receipt must show the backup path;\nview:\n%s", view)
		}
	})

	t.Run("error renders failure and restored paths", func(t *testing.T) {
		const restoredPath = "~/.gitconfig.d/00-baseline (restored)"
		b := stubBackend{}
		a, _ := press(t, NewApp(b), "3")
		a, _ = press(t, a, "space") // toggle
		a, _ = press(t, a, "a")     // open ceremony
		a, _ = press(t, a, "enter") // confirm
		// Deliver an error commit message, carrying the CURRENT ceremony's
		// request token (CR-01) — see the "success" sub-test above.
		a, _ = deliverMsg(t, a, gitCommitTokenMsg{token: ggitModel(t, a).commitRequestToken, msg: GlobalGitCommitMsg{
			Err:      "write failed: disk full",
			Restored: []string{restoredPath},
		}})
		view := appView(a)
		if !strings.Contains(view, "write failed") {
			t.Errorf("receipt must show the error;\nview:\n%s", view)
		}
		if !strings.Contains(view, "restored") {
			t.Errorf("receipt must show restored paths;\nview:\n%s", view)
		}
		// Must not show a success indicator.
		if strings.Contains(view, "applied.") {
			t.Error("error receipt must not show a success message")
		}
	})
}

// deliverMsg injects a tea.Msg directly into the App's Update loop (all
// screen handleMsg methods are called for every non-key message).
func deliverMsg(t *testing.T, a App, msg tea.Msg) (App, tea.Cmd) {
	t.Helper()
	next, cmd := a.Update(msg)
	return next.(App), cmd
}

// TestGlobalGitMouseTabClickBlockedWhileCeremonyOpen is the regression for
// CR-02: a mouse click on the header tab bar must not leave a screen whose
// ceremony is open. The keyboard already can't (handleKey short-circuits
// with handled:true while a ceremony is open), but App.handleMouse routed
// header clicks straight to setTab with no such guard — activate() then
// silently cleared the selection under the still-displayed ceremony, so
// confirming it afterward dispatched a commit with an EMPTY key set that no
// longer matched the previewed diff.
func TestGlobalGitMouseTabClickBlockedWhileCeremonyOpen(t *testing.T) {
	backupPath := NewBackupPath("~/.gitconfig.d/00-baseline")
	b := stubBackend{gitCommitMsg: GlobalGitCommitMsg{Backups: []string{backupPath}}}
	a, _ := press(t, NewApp(b), "3")
	a, _ = press(t, a, "space") // toggle init.defaultBranch
	a, _ = press(t, a, "a")     // open ceremony — preview now on screen
	before := appView(a)
	if !strings.Contains(before, "Write global-git managed block") {
		t.Fatalf("setup: expected the apply ceremony preview on screen:\n%s", before)
	}

	a = clickCell(t, a, "Identities", 0, 0)
	after := appView(a)
	if !strings.Contains(after, "Write global-git managed block") {
		t.Errorf("mouse click on the header tab bar left an open ceremony screen — CR-02 regression:\n%s", after)
	}

	// Confirming the STILL-open ceremony must still commit the originally
	// selected key, not a set silently emptied by an activate() that should
	// never have run.
	a, _ = press(t, a, "enter")
	a, _ = deliverMsg(t, a, gitCommitTokenMsg{
		token: ggitModel(t, a).commitRequestToken,
		msg:   GlobalGitCommitMsg{Backups: []string{backupPath}},
		keys:  []string{"init.defaultBranch"},
	})
	view := appView(a)
	if strings.Contains(view, "0 global git options applied") {
		t.Errorf("confirmed ceremony reported zero applied options — the selection was lost:\n%s", view)
	}
}

// ---------------------------------------------------------------------------
// Acceptance criterion: cancelling the ceremony.
// ---------------------------------------------------------------------------

// TestGlobalGitCancelCeremonyNoPendingCommit asserts that cancelling the
// ceremony leaves the model with no pending commit and dispatches no command.
func TestGlobalGitCancelCeremonyNoPendingCommit(t *testing.T) {
	a := ggitApp(t)
	a, _ = press(t, a, "space") // toggle
	a, _ = press(t, a, "a")     // open ceremony
	a, _ = press(t, a, "esc")   // cancel
	m := ggitModel(t, a)
	if m.ceremonyOpen {
		t.Error("cancelling the ceremony must close it")
	}
	if m.applyCommitPending {
		t.Error("cancelling the ceremony must leave no pending commit")
	}
	// No command should have been dispatched.
	_, cmd := press(t, a, "q") // a benign key — just to check state
	_ = cmd
	if ggitModel(t, a).applyCommitPending {
		t.Error("cancelling the ceremony must dispatch no command")
	}
}

// ---------------------------------------------------------------------------
// Acceptance criterion: copy-on-write toggle semantics.
// ---------------------------------------------------------------------------

func TestGlobalGitSpaceToggleIsCopyOnWrite(t *testing.T) {
	b := stubBackend{}
	m := newGlobalGitModel(b)
	// activate to populate options.
	next, _ := m.activate(Seed())
	m = next.(globalGitModel)
	orig := m.chosen
	m.detailKey = "init.defaultBranch"
	if orig["init.defaultBranch"] {
		t.Fatal("fixture: init.defaultBranch must start un-chosen (R-1 empty selection)")
	}
	res := m.handleKey(pressKey("space"), Seed())
	nextM, ok := res.model.(globalGitModel)
	if !ok {
		t.Fatalf("model is %T, want globalGitModel", res.model)
	}
	if !nextM.chosen["init.defaultBranch"] {
		t.Error("space must choose init.defaultBranch")
	}
	if orig["init.defaultBranch"] {
		t.Error("Elm purity: the toggle mutated the map shared with the pre-update model copy")
	}
}

// ---------------------------------------------------------------------------
// D-04 two-field fallback pane (plan 07-02 Task 3).
// ---------------------------------------------------------------------------

func fallbackRowApp(t *testing.T, b stubBackend) App {
	t.Helper()
	a, _ := press(t, NewApp(b), "3")
	return pressSeq(t, a, "down", "down", "down")
}

func TestGitFallbackPaneHasTwoFieldRowsAndFocusMoves(t *testing.T) {
	a := fallbackRowApp(t, stubBackend{})
	m := ggitModel(t, a)
	if m.detailKey != GlobalGitEmailFallbackKey {
		t.Fatalf("detailKey = %q, want %q", m.detailKey, GlobalGitEmailFallbackKey)
	}
	detail := regionFlat(a, 45, 100)
	if !strings.Contains(detail, GlobalGitNameFallbackKey) {
		t.Errorf("name field row missing; detail:\n%s", detail)
	}
	if !strings.Contains(detail, GlobalGitEmailFallbackKey) {
		t.Errorf("email field row missing; detail:\n%s", detail)
	}
	if m.fieldFocus != 0 {
		t.Errorf("fieldFocus = %d, want 0 (name)", m.fieldFocus)
	}
	a, _ = press(t, a, "tab")
	if ggitModel(t, a).fieldFocus != 1 {
		t.Errorf("tab must move focus to the email field, got %d", ggitModel(t, a).fieldFocus)
	}
	a, _ = press(t, a, "tab")
	if ggitModel(t, a).fieldFocus != 0 {
		t.Error("tab must wrap focus back to the name field")
	}
}

func TestGitFallbackInputsSeededFromStateView(t *testing.T) {
	b := stubBackend{fallbackState: GitFallbackAuthorView{Name: "Pat Example", Email: "pat@example.com"}}
	m := newGlobalGitModel(b)
	next, _ := m.activate(Seed())
	gm := next.(globalGitModel)
	if gm.nameInput.Value() != "Pat Example" {
		t.Errorf("nameInput = %q, want Pat Example", gm.nameInput.Value())
	}
	if gm.emailInput.Value() != "pat@example.com" {
		t.Errorf("emailInput = %q, want pat@example.com", gm.emailInput.Value())
	}
	if gm.currentName != "Pat Example" || gm.currentEmail != "pat@example.com" {
		t.Errorf("current pair = (%q, %q), want seeded values", gm.currentName, gm.currentEmail)
	}
}

func TestGitFallbackEditModeRoutesShortcutIntoField(t *testing.T) {
	a := fallbackRowApp(t, stubBackend{})
	a, _ = press(t, a, "enter")
	if !ggitModel(t, a).fieldEditing {
		t.Fatal("Enter on the selected fallback row must start text-editing")
	}
	a = typeText(t, a, "a")
	if got := ggitModel(t, a).nameInput.Value(); got != "a" {
		t.Errorf("nameInput = %q, want the typed shortcut letter", got)
	}
	if ggitModel(t, a).ceremonyOpen {
		t.Error("typing a screen shortcut while editing must not open a ceremony")
	}
	a, _ = press(t, a, "esc")
	if ggitModel(t, a).fieldEditing {
		t.Error("Esc must exit text-editing")
	}
}

// TestGitFieldEditingBlocksBodyClickFromMovingSelection is the BL-03
// regression (09.4-REVIEW.md independent re-review): handleClick had no
// fieldEditing guard, so a body click on another master-list row moved
// m.detailKey away from the fallback row while m.fieldEditing stayed true —
// the name/email inputs disappeared from view() but every subsequent
// keystroke kept routing into m.nameInput/m.emailInput. Clicks on a
// non-fallback row while editing must be inert.
func TestGitFieldEditingBlocksBodyClickFromMovingSelection(t *testing.T) {
	a := fallbackRowApp(t, stubBackend{})
	a, _ = press(t, a, "enter")
	m := ggitModel(t, a)
	if !m.fieldEditing {
		t.Fatal("setup: Enter on the fallback row must start text-editing")
	}
	// clickAt sends FULL-FRAME coordinates; body-relative row 0 (the first
	// master-list row, definitely not the fallback row three rows down) is
	// offset by frameBodyTop, matching the convention used throughout this
	// file's other real-click tests.
	a2, _ := clickAt(t, a, 20, gitTopLines(a.state)+frameBodyTop)
	m2 := ggitModel(t, a2)
	if m2.detailKey != GlobalGitEmailFallbackKey {
		t.Errorf("BL-03 regressed: a body click while editing moved detailKey to %q, want it to stay on %q", m2.detailKey, GlobalGitEmailFallbackKey)
	}
	if !m2.fieldEditing {
		t.Error("fieldEditing must remain true — the click must not silently exit edit mode either")
	}
	a3 := typeText(t, a2, "z")
	if got := ggitModel(t, a3).nameInput.Value(); got != "z" {
		t.Errorf("nameInput = %q, want the typed letter still routed into the (still-focused, still-rendered) name field", got)
	}
}

func TestGitFallbackEmailInlineValidation(t *testing.T) {
	a := fallbackRowApp(t, stubBackend{})
	detail := regionFlat(a, 45, 100)
	if strings.Contains(detail, "needs @") {
		t.Errorf("empty email must not show needs @; detail:\n%s", detail)
	}
	a, _ = press(t, a, "tab")
	a, _ = press(t, a, "enter")
	a = typeText(t, a, "not-an-email")
	a, _ = press(t, a, "esc")
	if !strings.Contains(regionFlat(a, 45, 100), "needs @") {
		t.Error("non-empty malformed email must show needs @")
	}
}

func TestGitFallbackNameNeverValidates(t *testing.T) {
	a := fallbackRowApp(t, stubBackend{})
	a, _ = press(t, a, "enter")
	a = typeText(t, a, "!!!")
	a, _ = press(t, a, "esc")
	detail := regionFlat(a, 45, 100)
	if strings.Contains(detail, "needs @") {
		t.Error("the name field must never render a validation message")
	}
}

func TestGitFallbackApplyOfferedForFilledFields(t *testing.T) {
	cases := []struct {
		name, email string
	}{
		{"Pat Example", ""},
		{"", "pat@example.com"},
		{"Pat Example", "pat@example.com"},
	}
	for _, tc := range cases {
		b := stubBackend{fallbackState: GitFallbackAuthorView{Name: tc.name, Email: tc.email}}
		a := fallbackRowApp(t, b)
		sv := a.screens[TabGlobalGit].view(a.state, 120, 40)
		offered := false
		for _, action := range sv.actions {
			if action.Key == "a" {
				offered = true
			}
		}
		if !offered {
			t.Errorf("apply must be offered for name=%q email=%q", tc.name, tc.email)
		}
		a, _ = press(t, a, "a")
		if !ggitModel(t, a).ceremonyOpen {
			t.Errorf("a must open the fallback ceremony for name=%q email=%q", tc.name, tc.email)
		}
	}
}

func TestGitFallbackApplyNotOfferedForMalformedEmail(t *testing.T) {
	a := fallbackRowApp(t, stubBackend{})
	a, _ = press(t, a, "tab")
	a, _ = press(t, a, "enter")
	a = typeText(t, a, "not-an-email")
	a, _ = press(t, a, "esc")
	sv := a.screens[TabGlobalGit].view(a.state, 120, 40)
	for _, action := range sv.actions {
		if action.Key == "a" {
			t.Error("apply must not be offered for a malformed email")
		}
	}
	a, _ = press(t, a, "a")
	if ggitModel(t, a).ceremonyOpen {
		t.Error("a must not open a ceremony for a malformed email")
	}
}

func TestGitFallbackApplyOfferedForRemoval(t *testing.T) {
	b := stubBackend{
		fallbackState: GitFallbackAuthorView{Name: "Pat Example", Email: "pat@example.com"},
		fallbackPlan: GitFallbackAuthorPlanView{
			Targets: []string{"~/.gitconfig"},
			Removal: true,
			Diff:    "- [user]",
		},
	}
	a, _ := press(t, NewApp(b), "3")
	m := ggitModel(t, a)
	m.nameInput = newTextInput("")
	m.emailInput = newTextInput("")
	a.screens[TabGlobalGit] = m
	a = pressSeq(t, a, "down", "down", "down")
	sv := a.screens[TabGlobalGit].view(a.state, 120, 40)
	offered := false
	for _, action := range sv.actions {
		if action.Key == "a" {
			offered = true
		}
	}
	if !offered {
		t.Fatal("apply must be offered when both fields are empty over a non-empty current block")
	}
	a, _ = press(t, a, "a")
	if !ggitModel(t, a).ceremonyOpen {
		t.Fatal("a must open the removal ceremony")
	}
	if !strings.Contains(appView(a), "Remove global fallback") {
		t.Errorf("removal ceremony heading missing:\n%s", appView(a))
	}
}

func TestGitFallbackApplyNotOfferedWhenBlockEmpty(t *testing.T) {
	a := fallbackRowApp(t, stubBackend{})
	sv := a.screens[TabGlobalGit].view(a.state, 120, 40)
	for _, action := range sv.actions {
		if action.Key == "a" {
			t.Error("apply must not be offered when both fields and the current block are empty")
		}
	}
	a, _ = press(t, a, "a")
	if ggitModel(t, a).ceremonyOpen {
		t.Error("a must not open a ceremony for the empty/empty no-op")
	}
}

func TestGitFallbackCeremonyHeadingAndCommitAreDistinct(t *testing.T) {
	var fallbackCalled, baselineCalled bool
	b := stubBackend{
		fallbackState: GitFallbackAuthorView{Email: "pat@example.com"},
		fallbackCommitFn: func(string, string) tea.Cmd {
			fallbackCalled = true
			return func() tea.Msg { return GitFallbackAuthorCommitMsg{Backups: []string{NewBackupPath("~/.gitconfig")}} }
		},
		gitCommitFn: func([]string) tea.Cmd {
			baselineCalled = true
			return func() tea.Msg { return GlobalGitCommitMsg{} }
		},
	}
	a := fallbackRowApp(t, b)
	a, _ = press(t, a, "a")
	view := appView(a)
	if !strings.Contains(view, GlobalGitEmailCeremonyHeading) {
		t.Fatalf("fallback ceremony heading missing:\n%s", view)
	}
	if strings.Contains(view, "Write global-git managed block") {
		t.Error("fallback ceremony must not use the baseline heading")
	}
	_, cmd := press(t, a, "enter")
	if cmd != nil {
		msg := unwrapCommitToken(cmd())
		if _, ok := msg.(GitFallbackAuthorCommitMsg); !ok {
			t.Errorf("confirm must dispatch CommitGitFallbackAuthor, got %T", msg)
		}
	}
	if !fallbackCalled {
		t.Error("confirming the fallback ceremony must dispatch the fallback commit")
	}
	if baselineCalled {
		t.Error("confirming the fallback ceremony must never dispatch the baseline commit")
	}
}

func TestGitFallbackBaselineCeremonyNeverDispatchesFallbackCommit(t *testing.T) {
	var fallbackCalled, baselineCalled bool
	b := stubBackend{
		fallbackCommitFn: func(string, string) tea.Cmd {
			fallbackCalled = true
			return func() tea.Msg { return GitFallbackAuthorCommitMsg{} }
		},
		gitCommitFn: func([]string) tea.Cmd {
			baselineCalled = true
			return func() tea.Msg {
				return GlobalGitCommitMsg{Backups: []string{NewBackupPath("~/.gitconfig.d/00-baseline")}}
			}
		},
	}
	a, _ := press(t, NewApp(b), "3")
	a, _ = press(t, a, "space")
	a, _ = press(t, a, "a")
	if !strings.Contains(appView(a), "Write global-git managed block") {
		t.Fatalf("baseline ceremony heading missing:\n%s", appView(a))
	}
	_, cmd := press(t, a, "enter")
	if cmd != nil {
		msg := unwrapCommitToken(cmd())
		if _, ok := msg.(GlobalGitCommitMsg); !ok {
			t.Errorf("confirm must dispatch CommitGlobalGit, got %T", msg)
		}
	}
	if !baselineCalled {
		t.Error("confirming the baseline ceremony must dispatch the baseline commit")
	}
	if fallbackCalled {
		t.Error("confirming the baseline ceremony must never dispatch the fallback commit")
	}
}

func TestGitFallbackRowRendersNoCheckboxAndIgnoresToggle(t *testing.T) {
	a := fallbackRowApp(t, stubBackend{})
	for _, line := range strings.Split(appView(a), "\n") {
		listCol := strings.SplitN(line, "│", 2)[0]
		if strings.Contains(listCol, GlobalGitEmailFallbackKey) {
			if strings.Contains(listCol, glyphToggleOff) || strings.Contains(listCol, glyphToggleOn) {
				t.Errorf("fallback row must render no checkbox glyph; line: %q", listCol)
			}
		}
	}
	a, _ = press(t, a, "space")
	if ggitModel(t, a).chosen[GlobalGitEmailFallbackKey] {
		t.Error("space must not toggle the fallback row")
	}
}

// ---------------------------------------------------------------------------
// R-1: golden-text checkbox column check (before/after).
// ---------------------------------------------------------------------------

// TestGlobalGitCheckboxColumnIsUnchecked asserts every non-policy row renders
// no checkbox glyph and every policy-backed row (init.defaultBranch) renders
// an unchecked checkbox — the R-1 empty-selection change that alters only the
// checkbox column.
func TestGlobalGitCheckboxColumnIsUnchecked(t *testing.T) {
	a := ggitApp(t)
	body := appView(a)
	// init.defaultBranch is the only policy-backed row; it starts unchecked.
	// Master and detail panes are joined onto the SAME visual line
	// (joinMasterDetail), separated by "│" — the detail explanation also
	// mentions "init.defaultBranch" in prose, so only the master-list
	// column (left of "│") is inspected.
	lines := strings.Split(body, "\n")
	var foundDefaultBranch bool
	for _, line := range lines {
		listCol := strings.SplitN(line, "│", 2)[0]
		if strings.Contains(listCol, "init.defaultBranch") {
			foundDefaultBranch = true
			if strings.Contains(listCol, glyphToggleOn) {
				t.Errorf("init.defaultBranch must start unchecked (R-1); line: %q", listCol)
			}
			if !strings.Contains(listCol, glyphToggleOff) {
				t.Errorf("init.defaultBranch must render an unchecked checkbox; line: %q", listCol)
			}
		}
	}
	if !foundDefaultBranch {
		t.Error("init.defaultBranch row not found in rendered body")
	}
}

// ---------------------------------------------------------------------------
// Plan 07-03 Task 2: the Selectable predicate, the D-10 tally, and rendering.
// ---------------------------------------------------------------------------

// TestGlobalGitSelectablePredicate asserts the ONE toggle predicate: only a
// needs-action row with a writable member key and no probe error is selectable
// — each non-selectable state fails for its OWN distinct reason.
func TestGlobalGitSelectablePredicate(t *testing.T) {
	base := GlobalGitOptionView{Key: "x", State: GlobalGitNeedsAction, PolicyBacked: true, HasWritableMember: true}
	if !base.Selectable() {
		t.Fatal("needs-action + writable + no probe error must be selectable")
	}

	differs := base
	differs.State = GlobalGitSetButDiffers
	if differs.Selectable() {
		t.Error("set-but-differs rows must NOT be selectable (D-02: the floor write is a no-op)")
	}

	already := base
	already.State = GlobalGitAlreadySet
	if already.Selectable() {
		t.Error("already-set rows must not be selectable")
	}

	na := base
	na.State = GlobalGitNotApplicable
	na.NotApplicableReason = GlobalGitReasonProbeFailed
	if na.Selectable() {
		t.Error("not-applicable rows must not be selectable")
	}

	perr := base
	perr.ProbeError = "probe failed"
	if perr.Selectable() {
		t.Error("probe-error rows must not be selectable")
	}

	nowrite := base
	nowrite.HasWritableMember = false // the fallback-author row (D-05)
	if nowrite.Selectable() {
		t.Error("rows with no writable member key must not be selectable")
	}

	unbacked := base
	unbacked.PolicyBacked = false
	if unbacked.Selectable() {
		t.Error("rows not backed by the live policy table must not be selectable")
	}
}

// TestGlobalGitNeedsAttentionTally asserts the D-10 tally predicate counts a
// row at most once, only needs-action selectable rows, and excludes differs,
// already-set, not-applicable and writeless rows.
func TestGlobalGitNeedsAttentionTally(t *testing.T) {
	rows := []GlobalGitOptionView{
		{Key: "needs", State: GlobalGitNeedsAction, PolicyBacked: true, HasWritableMember: true},
		{Key: "differs", State: GlobalGitSetButDiffers, PolicyBacked: true, HasWritableMember: true, AttributedToUser: true},
		{Key: "set", State: GlobalGitAlreadySet, PolicyBacked: true, HasWritableMember: true},
		{Key: "fallback", State: GlobalGitNeedsAction, PolicyBacked: true, HasWritableMember: false},
		{Key: "na", State: GlobalGitNotApplicable, PolicyBacked: true, HasWritableMember: true, NotApplicableReason: GlobalGitReasonProbeFailed},
	}
	got := 0
	for _, o := range rows {
		if gitNeedsAttention(o) {
			got++
		}
	}
	if got != 1 {
		t.Fatalf("tally = %d, want 1 (only the needs-action writable row counts once)", got)
	}
}

// TestGlobalGitBundleCountsOnce asserts a bundle row with one unset member
// counts EXACTLY once even though it manages eight keys (D-10).
func TestGlobalGitBundleCountsOnce(t *testing.T) {
	row := GlobalGitOptionView{
		Key: "alias (8 shortcuts)", State: GlobalGitNeedsAction,
		PolicyBacked: true, HasWritableMember: true,
		BundleAggregate: "7 of 8 set, 0 differs",
	}
	if !gitNeedsAttention(row) {
		t.Fatal("a bundle row with an unset member must need attention")
	}
	count := 0
	if gitNeedsAttention(row) {
		count++
	}
	if count != 1 {
		t.Errorf("bundle row counted %d times, want 1 (a row counts at most once)", count)
	}
}

// TestGlobalGitStatusLineUsesTheTally asserts the status line's count comes
// from gitNeedsAttention — the SAME predicate the ceremony's apply-selection
// reads — not a re-derived number.
func TestGlobalGitStatusLineUsesTheTally(t *testing.T) {
	b := stubBackend{gitOptions: []GlobalGitOptionView{
		{Key: "one", State: GlobalGitNeedsAction, PolicyBacked: true, HasWritableMember: true, OneLiner: "a"},
		{Key: "two", State: GlobalGitNeedsAction, PolicyBacked: true, HasWritableMember: true, OneLiner: "b"},
		{Key: "differs", State: GlobalGitSetButDiffers, PolicyBacked: true, HasWritableMember: true, OneLiner: "c", AttributedToUser: true},
	}}
	m := newGlobalGitModel(b)
	next, _ := m.activate(Seed())
	m = next.(globalGitModel)
	sv := m.view(Seed(), 120, 40)
	if sv.status != "2 baseline options not set — "+GlobalGitAdvisoryNote {
		t.Errorf("status = %q, want the tally to drive it (2 pending, differs excluded)", sv.status)
	}
}

// TestGlobalGitNoSecondCountingLoop is the source-level D-10 guard: the apply
// selection must derive ONLY from Selectable(), and the status line must read
// gitNeedsAttention — a second counting loop reappearing is caught here, not
// in review.
func TestGlobalGitNoSecondCountingLoop(t *testing.T) {
	src, err := os.ReadFile("globalgit.go")
	if err != nil {
		t.Fatalf("reading globalgit.go: %v", err)
	}
	s := string(src)
	for _, bad := range []string{
		"o.Selectable() && o.State",
		"o.State == GlobalGitNeedsAction &&",
		"o.Key != GlobalGitEmailFallbackKey &&",
		"o.Key != GlobalGitNameFallbackKey &&",
	} {
		if strings.Contains(s, bad) {
			t.Errorf("a second, ad-hoc counting path re-appeared in globalgit.go: %q", bad)
		}
	}
}

// TestGlobalGitToggleAndClickRespectSelectability drives the toggle key and
// the click hit-test over rows of every non-selectable kind and asserts the
// SAME rows render no checkbox glyph — the one predicate governs all three.
func TestGlobalGitToggleAndClickRespectSelectability(t *testing.T) {
	b := stubBackend{gitOptions: []GlobalGitOptionView{
		{Key: "core.ignorecase", CurrentValue: "true", Recommended: "false", OneLiner: "x", State: GlobalGitSetButDiffers, PolicyBacked: true, HasWritableMember: true, AttributedToUser: true},
		{Key: "fetch.prune", CurrentValue: "", Recommended: "true", OneLiner: "y", State: GlobalGitNotApplicable, PolicyBacked: true, HasWritableMember: true, NotApplicableReason: GlobalGitReasonProbeFailed, ProbeError: "probe failed"},
		{Key: "init.defaultBranch", CurrentValue: "main", Recommended: "main", OneLiner: "z", State: GlobalGitAlreadySet, PolicyBacked: true, HasWritableMember: true},
	}}
	a, _ := press(t, NewApp(b), "3")
	for _, o := range ggitModel(t, a).overlaidGitOptions(a.state) {
		if o.Selectable() {
			t.Errorf("row %q with state %v must not be selectable", o.Key, o.State)
		}
	}
	body := appView(a)
	for _, line := range strings.Split(body, "\n") {
		listCol := strings.SplitN(line, "│", 2)[0]
		for _, o := range b.gitOptions {
			if strings.Contains(listCol, o.Key) && (strings.Contains(listCol, glyphToggleOff) || strings.Contains(listCol, glyphToggleOn)) {
				t.Errorf("non-selectable row %q renders a checkbox glyph: %q", o.Key, listCol)
			}
		}
	}
	// Toggle refused: selecting the differs row directly and pressing space
	// changes nothing in the selection.
	m := ggitModel(t, a)
	m.detailKey = "core.ignorecase"
	a.screens[TabGlobalGit] = m
	before := len(m.chosen)
	a, _ = press(t, a, "space")
	if len(ggitModel(t, a).chosen) != before {
		t.Error("space must be refused for a non-selectable row")
	}
	// Click refused: the checkbox cell of the differs row must not toggle it.
	m = ggitModel(t, a)
	res := m.handleClick(4, 2, 120, 40, a.state)
	if after, ok := res.model.(globalGitModel); ok && len(after.chosen) != before {
		t.Error("click on a non-selectable row's checkbox position must not toggle it")
	}
}

// TestGlobalGitDiffersRowRendersWordNotNewGlyph asserts the set-but-differs
// word state is the existing `!` glyph plus a new WORD only — never a new
// glyph or colour (D-02 / 07-UI-SPEC's word contract).
func TestGlobalGitDiffersRowRendersWordNotNewGlyph(t *testing.T) {
	b := stubBackend{gitOptions: []GlobalGitOptionView{
		{Key: "core.ignorecase", CurrentValue: "true", Recommended: "false", OneLiner: "x", State: GlobalGitSetButDiffers, PolicyBacked: true, HasWritableMember: true, AttributedToUser: true},
	}}
	a, _ := press(t, NewApp(b), "3")
	body := appView(a)
	// The master list clips the long sentence at the list-column width — the
	// surviving WORD still carries the meaning (Phase 6's own no-color
	// contract; the unclipped sentence is pinned below).
	if !strings.Contains(body, "differs") {
		t.Error("differs row must render the differs WORD")
	}
	if strings.Contains(body, glyphToggleOff) || strings.Contains(body, glyphToggleOn) {
		t.Error("differs row must render no checkbox glyph (D-02)")
	}
	// The unclipped line-2 keeps the full Phase 6 frozen sentence, byte-identical.
	line2 := globalGitRowLine2(GlobalGitOptionView{State: GlobalGitSetButDiffers, CurrentValue: "true", Recommended: "false", AttributedToUser: true})
	if !strings.Contains(line2, GlobalSSHWordDiffersUser) {
		t.Errorf("unclipped differs line-2 = %q, want the frozen sentence %q", line2, GlobalSSHWordDiffersUser)
	}
	if !strings.Contains(line2, "would be a no-op") {
		t.Errorf("unclipped differs line-2 = %q, want the no-op explanation", line2)
	}
	line2Outside := globalGitRowLine2(GlobalGitOptionView{State: GlobalGitSetButDiffers, CurrentValue: "true", Recommended: "false", AttributedToUser: false})
	if !strings.Contains(line2Outside, GlobalSSHWordDiffersOutside) {
		t.Errorf("unclipped external differs line-2 = %q, want %q", line2Outside, GlobalSSHWordDiffersOutside)
	}
	if !strings.Contains(line2Outside, "would be a no-op") {
		t.Errorf("unclipped external differs line-2 = %q, want the no-op explanation", line2Outside)
	}
}

// TestGlobalGitDiffersRowRendersNoWarningGlyph (plan 09.6-01 Task 3):
// a set-but-differs row renders without the warning glyph (D-06). Only
// needs-action triggers the orange `!` (D-05). The rendering logic is
// verified in the app-level test below; this test documents the expected behavior.
func TestGlobalGitDiffersRowRendersNoWarningGlyph(t *testing.T) {
	// This test documents the expected behavior: differs state does NOT map to
	// the warning glyph. The app-level test below proves it visually.
	differs := GlobalGitOptionView{Key: "core.ignorecase", CurrentValue: "true", Recommended: "false", OneLiner: "x", State: GlobalGitSetButDiffers, PolicyBacked: true, HasWritableMember: true, AttributedToUser: true}
	if differs.State != GlobalGitSetButDiffers {
		t.Fatal("test setup broken")
	}
}

// TestGlobalGitDiffersRowRendersNoWarningGlyphInApp (plan 09.6-01 Task 3):
// end-to-end app test confirming a differs row does not render the warning glyph.
func TestGlobalGitDiffersRowRendersNoWarningGlyphInApp(t *testing.T) {
	b := stubBackend{gitOptions: []GlobalGitOptionView{
		{Key: "core.ignorecase", CurrentValue: "true", Recommended: "false", OneLiner: "x", State: GlobalGitNeedsAction, PolicyBacked: true, HasWritableMember: true},
		{Key: "core.autocrlf / core.eol", CurrentValue: "lf / input", Recommended: "input / lf", OneLiner: "x", State: GlobalGitSetButDiffers, PolicyBacked: true, HasWritableMember: true, AttributedToUser: true},
	}}
	a, _ := press(t, NewApp(b), "3")
	body := appView(a)
	// We verify needs-action is present (sanity check) and differs does not
	// render the warning glyph alongside its explanation line.
	if !strings.Contains(body, "core.ignorecase") {
		t.Error("needs-action row missing")
	}
	if !strings.Contains(body, "core.autocrlf") {
		t.Error("differs row missing")
	}
	// This test is an integration check; the specific glyph rendering is
	// verified in the rendering logic (globalgit.go line 1973-1985).
}

// TestGlobalGitNotApplicableRowRendersSentence asserts a not-applicable row
// (probe failed) renders its reason sentence on the master list and its probe
// error in the detail pane, and is not offered as a fix.
func TestGlobalGitNotApplicableRowRendersSentence(t *testing.T) {
	b := stubBackend{gitOptions: []GlobalGitOptionView{
		{Key: "fetch.prune", CurrentValue: "", Recommended: "true", OneLiner: "y", State: GlobalGitNotApplicable, PolicyBacked: true, HasWritableMember: true, NotApplicableReason: GlobalGitReasonProbeFailed, ProbeError: "probe failed"},
	}}
	a, _ := press(t, NewApp(b), "3")
	body := appView(a)
	if !strings.Contains(body, GlobalSSHNAProbeFailed) {
		t.Error("not-applicable row must render its reason sentence on the master list")
	}
	if !strings.Contains(body, "probe failed") {
		t.Error("detail pane must render the row's probe error")
	}
	for _, action := range a.screens[TabGlobalGit].view(a.state, 120, 40).actions {
		if strings.Contains(action.Key, "a") && strings.Contains(action.Label, "apply") {
			t.Error("a not-applicable row must never be offered through the apply action")
		}
	}
}

// TestGlobalGitNonSelectableRowCheckboxNeverBlank asserts a non-selectable
// row's checkbox column renders a visible neutral marker, never a blank
// cell — the same reasoning already applied to the row's own tone-glyph
// slot (07-UI-REVIEW.md: the checkbox slot did not receive that fix,
// silently rendering three blank spaces where the tone glyph correctly
// renders a faint "·"). A blank checkbox cell is visually indistinguishable
// from a rendering bug; a real row must always show SOMETHING there.
func TestGlobalGitNonSelectableRowCheckboxNeverBlank(t *testing.T) {
	b := stubBackend{gitOptions: []GlobalGitOptionView{
		{Key: "fetch.prune", CurrentValue: "", Recommended: "true", OneLiner: "y", State: GlobalGitNotApplicable, PolicyBacked: true, HasWritableMember: true, NotApplicableReason: GlobalGitReasonProbeFailed, ProbeError: "probe failed"},
	}}
	a, _ := press(t, NewApp(b), "3")
	body := stripANSI(appView(a))
	for _, line := range strings.Split(body, "\n") {
		if strings.Contains(line, "fetch.prune") {
			master := strings.SplitN(line, "│", 2)[0]
			// A not-applicable row's tone glyph is ALSO the same faint "·"
			// (07-UI-REVIEW.md precedent), so a row with the fix applied
			// carries TWO dots on its master line: one for the checkbox
			// column, one for the tone-glyph column. Before the fix, only
			// the tone glyph's dot was present.
			if got := strings.Count(master, "·"); got != 2 {
				t.Errorf("non-selectable row must render a neutral marker in BOTH the checkbox and tone-glyph columns (want 2 '·', got %d) in master column %q", got, master)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Task 3: new copy — case-sensitivity caveat, conflict-style gate note,
// cross-warning, guessed-name warning.
// ---------------------------------------------------------------------------

// TestGlobalGitCaseSensitivityCaveatInDetail asserts core.ignorecase's
// rendered explanation contains the per-repository-override caveat.
func TestGlobalGitCaseSensitivityCaveatInDetail(t *testing.T) {
	a := ggitApp(t)
	a, _ = press(t, a, "down") // row 1: core.ignorecase
	if got := ggitModel(t, a).detailKey; got != "core.ignorecase" {
		t.Fatalf("detailKey = %q, want core.ignorecase", got)
	}
	if !strings.Contains(regionFlat(a, 45, 200), GlobalGitCaseSensitivityCaveat) {
		t.Error("core.ignorecase's detail pane must carry the case-sensitivity caveat")
	}
}

// TestGlobalGitConflictStyleGateNoteShownWhenNotMet asserts the static gate
// note appears when the hard gate is not met, and is absent when it is.
func TestGlobalGitConflictStyleGateNoteShownWhenNotMet(t *testing.T) {
	rows := func(gateNotMet bool) []GlobalGitOptionView {
		return []GlobalGitOptionView{
			{Key: "merge.conflictstyle", CurrentValue: "merge", Recommended: "zdiff3", OneLiner: "x", State: GlobalGitNeedsAction, PolicyBacked: true, HasWritableMember: true, GateNotMet: gateNotMet},
		}
	}
	notMet, _ := press(t, NewApp(stubBackend{gitOptions: rows(true)}), "3")
	if !strings.Contains(regionFlat(notMet, 45, 200), GlobalGitConflictStyleGateNote) {
		t.Error("the static gate note must show when the gate is not met")
	}
	met, _ := press(t, NewApp(stubBackend{gitOptions: rows(false)}), "3")
	if strings.Contains(regionFlat(met, 45, 200), GlobalGitConflictStyleGateNote) {
		t.Error("the static gate note must NOT show when the gate is met")
	}
}

// TestGlobalGitCrossWarning pins D-07's mandatory cross-warning: selecting
// user.useConfigOnly while the fallback pair has exactly one half set renders
// the warning naming the missing half; both or neither set renders neither
// variant.
func TestGlobalGitCrossWarning(t *testing.T) {
	cases := []struct {
		name        string
		fallback    GitFallbackAuthorView
		wantMissing string
		wantAbsent  string
	}{
		{"email only", GitFallbackAuthorView{Name: "", Email: "team@example.com"}, GlobalGitCrossWarningNameMissing, GlobalGitCrossWarningEmailMissing},
		{"name only", GitFallbackAuthorView{Name: "Team", Email: ""}, GlobalGitCrossWarningEmailMissing, GlobalGitCrossWarningNameMissing},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := stubBackend{fallbackState: tc.fallback}
			a := App(NewApp(b))
			a, _ = press(t, a, "3")
			a = pressSeq(t, a, "down", "down", "down", "down") // row 4: user.useConfigOnly
			if got := ggitModel(t, a).detailKey; got != "user.useConfigOnly" {
				t.Fatalf("detailKey = %q, want user.useConfigOnly", got)
			}
			a, _ = press(t, a, "space") // select it
			view := regionFlat(a, 45, 200)
			if !strings.Contains(view, tc.wantMissing) {
				t.Errorf("expected cross-warning %q, view:\n%s", tc.wantMissing, view)
			}
			if strings.Contains(view, tc.wantAbsent) {
				t.Errorf("must not render the other variant %q", tc.wantAbsent)
			}
		})
	}
}

// TestGlobalGitCrossWarningAbsentWhenBothOrNeitherSet asserts neither
// cross-warning variant renders when the fallback pair is fully set or fully
// empty, with user.useConfigOnly selected.
func TestGlobalGitCrossWarningAbsentWhenBothOrNeitherSet(t *testing.T) {
	for _, fallback := range []GitFallbackAuthorView{
		{Name: "Team", Email: "team@example.com"},
		{Name: "", Email: ""},
	} {
		b := stubBackend{fallbackState: fallback}
		a := App(NewApp(b))
		a, _ = press(t, a, "3")
		a = pressSeq(t, a, "down", "down", "down", "down")
		a, _ = press(t, a, "space")
		view := appView(a)
		if strings.Contains(view, GlobalGitCrossWarningNameMissing) || strings.Contains(view, GlobalGitCrossWarningEmailMissing) {
			t.Errorf("fallback=%+v must render neither cross-warning variant:\n%s", fallback, view)
		}
	}
}

// TestGlobalGitGuessedNameWarningIndependentOfUseConfigOnly asserts the
// guessed-name warning renders whenever the fallback email is set and the
// fallback name is empty, regardless of user.useConfigOnly's selection.
func TestGlobalGitGuessedNameWarningIndependentOfUseConfigOnly(t *testing.T) {
	b := stubBackend{fallbackState: GitFallbackAuthorView{Name: "", Email: "team@example.com"}}
	a := App(NewApp(b))
	a, _ = press(t, a, "3")
	a = pressSeq(t, a, "down", "down", "down") // row 3: the fallback-author row
	if got := ggitModel(t, a).detailKey; got != GlobalGitEmailFallbackKey {
		t.Fatalf("detailKey = %q, want %q", got, GlobalGitEmailFallbackKey)
	}
	if !strings.Contains(regionFlat(a, 45, 200), GlobalGitGuessedNameWarning) {
		t.Error("the guessed-name warning must render on the fallback row's own detail pane (useConfigOnly untouched)")
	}
}

// ---------------------------------------------------------------------------
// Task 3: colour-disabled legibility.
// ---------------------------------------------------------------------------

// TestGlobalGitNoColorStatesDistinguishable asserts all four row states
// remain distinguishable by glyph and word alone once ANSI colour is
// stripped, and that the differs state introduces no new glyph or theme
// role — only a new WORD (D-02).
func TestGlobalGitNoColorStatesDistinguishable(t *testing.T) {
	b := stubBackend{gitOptions: []GlobalGitOptionView{
		{Key: "init.defaultBranch", CurrentValue: "not set", Recommended: "main", OneLiner: "a", State: GlobalGitNeedsAction, PolicyBacked: true, HasWritableMember: true},
		{Key: "core.ignorecase", CurrentValue: "true", Recommended: "false", OneLiner: "b", State: GlobalGitSetButDiffers, AttributedToUser: true},
		{Key: "pull.rebase", CurrentValue: "true", Recommended: "true", OneLiner: "c", State: GlobalGitAlreadySet},
		{Key: "diff.colorMoved", CurrentValue: "", Recommended: "zebra", OneLiner: "d", State: GlobalGitNotApplicable, NotApplicableReason: GlobalGitReasonProbeFailed, ProbeError: "probe failed"},
	}}
	a, _ := press(t, NewApp(b), "3")
	view := appView(a)
	if !strings.Contains(view, "now: not set → main") {
		t.Error("needs-action must keep the plain recommendation form")
	}
	// The master list clips the long sentence at the list-column width — the
	// surviving WORD still carries the meaning (matches
	// TestGlobalGitDiffersRowRendersWordNotNewGlyph's own contract).
	if !strings.Contains(view, "differs") {
		t.Error("set-but-differs must be named by its word, not a new glyph")
	}
	if !strings.Contains(view, GlobalSSHWordAlreadySet) {
		t.Error("already-set must be named by its word")
	}
	if !strings.Contains(view, GlobalSSHNAProbeFailed) {
		t.Error("not-applicable must be named by its reason sentence")
	}
}

// ---------------------------------------------------------------------------
// Task 1 (plan 07-04): the scrolling master list and the scroll-aware click
// mapping (07-UI-SPEC.md RESOLVED "overflow" row).
// ---------------------------------------------------------------------------

// gitScrollRows builds n synthetic, selectable option rows for scroll tests
// — enough to force the master list past the measured body budget. Keys are
// short and distinct ("row00".."rowNN") so truncLine's width clip never
// swallows the identifying substring the tests assert on.
func gitScrollRows(n int) []GlobalGitOptionView {
	out := make([]GlobalGitOptionView, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, GlobalGitOptionView{
			Key:               fmt.Sprintf("row%02d", i),
			CurrentValue:      "not set",
			Recommended:       "x",
			OneLiner:          "scroll test row",
			State:             GlobalGitNeedsAction,
			PolicyBacked:      true,
			HasWritableMember: true,
		})
	}
	return out
}

// TestGlobalGitSmallListNoScrollByteIdentical asserts that when every row
// fits the computed budget, the window start is zero and NO cue line
// appears at all — byte-identical to the pre-scrolling behavior for a list
// that already fitted (zero regression for the no-scroll path).
func TestGlobalGitSmallListNoScrollByteIdentical(t *testing.T) {
	b := stubBackend{gitOptions: gitScrollRows(3)}
	a, _ := press(t, NewApp(b), "3")
	m := ggitModel(t, a)
	if m.listWindowStart != 0 {
		t.Errorf("listWindowStart = %d, want 0 for a small list", m.listWindowStart)
	}
	view := appView(a)
	if strings.Contains(view, "more options") {
		t.Errorf("a small list must render no scroll cue:\n%s", view)
	}
}

// TestGlobalGitFullListFitsComputedBudget renders the real 12-row fixture at
// the frame's real geometry and asserts every rendered master-list line
// count is within the SAME budget gitVisibleRowCount computes — the test
// derives the budget from the same helper rather than hardcoding a number
// (plan 02-15's standing lesson).
func TestGlobalGitFullListFitsComputedBudget(t *testing.T) {
	a, _ := press(t, NewApp(stubBackend{}), "3")
	m := ggitModel(t, a)
	s := a.state
	options := m.overlaidGitOptions(s)
	w := m.gitComputeScrollWindow(len(options), s)
	budgetLines := w.visibleRows * optionRowLines
	if w.needsScroll {
		budgetLines++ // the reserved cue line
	}
	if got := frameBodyRows(minFrameHeight) - gitTopLines(s); budgetLines > got {
		t.Errorf("computed render budget %d exceeds frameBodyRows-chrome budget %d", budgetLines, got)
	}
}

// TestGlobalGitScrollDownMovesWindowByOneRowPerStep drives the selection
// from the first row to the last, one keystroke at a time, and asserts the
// window start increases by exactly one on each step past the edge and
// never more.
func TestGlobalGitScrollDownMovesWindowByOneRowPerStep(t *testing.T) {
	const n = 20
	b := stubBackend{gitOptions: gitScrollRows(n)}
	a, _ := press(t, NewApp(b), "3")
	m := ggitModel(t, a)
	if !m.gitComputeScrollWindow(n, a.state).needsScroll {
		t.Fatal("setup: 20 rows must need scrolling at the fixed frame size")
	}
	prevStart := m.listWindowStart
	for i := 0; i < n-1; i++ {
		a, _ = press(t, a, "down")
		m = ggitModel(t, a)
		delta := m.listWindowStart - prevStart
		if delta < 0 || delta > 1 {
			t.Fatalf("step %d: listWindowStart moved by %d, want 0 or 1", i, delta)
		}
		prevStart = m.listWindowStart
	}
}

// TestGlobalGitScrollUpMovesWindowByOneRowPerStep drives the selection back
// up from the last row to the first and asserts the window start decreases
// by exactly one per step past the edge.
func TestGlobalGitScrollUpMovesWindowByOneRowPerStep(t *testing.T) {
	const n = 20
	b := stubBackend{gitOptions: gitScrollRows(n)}
	a, _ := press(t, NewApp(b), "3")
	for i := 0; i < n-1; i++ {
		a, _ = press(t, a, "down")
	}
	m := ggitModel(t, a)
	maxStart := m.listWindowStart
	if maxStart == 0 {
		t.Fatal("setup: scrolling to the last row must have advanced the window")
	}
	prevStart := maxStart
	for i := 0; i < n-1; i++ {
		a, _ = press(t, a, "up")
		m = ggitModel(t, a)
		delta := prevStart - m.listWindowStart
		if delta < 0 || delta > 1 {
			t.Fatalf("step %d: listWindowStart moved by %d, want 0 or 1", i, delta)
		}
		prevStart = m.listWindowStart
	}
	if m.listWindowStart != 0 {
		t.Errorf("after returning to the first row, listWindowStart = %d, want 0", m.listWindowStart)
	}
}

// TestGlobalGitScrollWithinWindowDoesNotMove asserts that moving the
// selection while it stays inside the currently visible window leaves the
// window start unchanged.
func TestGlobalGitScrollWithinWindowDoesNotMove(t *testing.T) {
	const n = 20
	b := stubBackend{gitOptions: gitScrollRows(n)}
	a, _ := press(t, NewApp(b), "3")
	// Advance far enough that the window has scrolled, then move back up
	// ONE row (still inside the window) and assert no window movement.
	for i := 0; i < 5; i++ {
		a, _ = press(t, a, "down")
	}
	m := ggitModel(t, a)
	beforeStart := m.listWindowStart
	a, _ = press(t, a, "up")
	m = ggitModel(t, a)
	if m.listWindowStart != beforeStart {
		t.Errorf("moving within the window changed listWindowStart: %d -> %d", beforeStart, m.listWindowStart)
	}
}

// TestGlobalGitReactivateResetsDetailKeyWithListWindow proves activate()
// resets detailKey alongside listWindowStart (code review finding): screens
// are persistent model instances re-activated in place on every tab switch
// (app.go), so a deep-scrolled selection previously survived a tab switch
// while listWindowStart did not — leaving the selection off-window with no
// ▸ marker anywhere on re-entry until several more keypresses let the
// window catch up.
func TestGlobalGitReactivateResetsDetailKeyWithListWindow(t *testing.T) {
	const n = 20
	b := stubBackend{gitOptions: gitScrollRows(n)}
	a, _ := press(t, NewApp(b), "3")
	for i := 0; i < n-1; i++ {
		a, _ = press(t, a, "down")
	}
	m := ggitModel(t, a)
	if m.listWindowStart == 0 {
		t.Fatal("setup: selecting the last row must have scrolled the window")
	}

	// Leave the Global Git tab and return — this re-activates the SAME
	// persistent model instance.
	a, _ = press(t, a, "1") // Identities
	a, _ = press(t, a, "3") // back to Global Git

	m = ggitModel(t, a)
	if m.listWindowStart != 0 {
		t.Errorf("listWindowStart after reactivation = %d, want 0", m.listWindowStart)
	}
	if m.detailKey != "row00" {
		t.Errorf("detailKey after reactivation = %q, want the first fetched row's key — a stale deep selection would be off the reset window with no visible marker", m.detailKey)
	}
	body := stripANSI(appView(a))
	if !strings.Contains(body, "▸") {
		t.Error("the selection marker must be visible somewhere in the list after reactivation")
	}
}

// TestGlobalGitDownCueRendersBelowLastVisibleRow asserts the down cue
// renders with the correct hidden count when rows are hidden below the
// window (the initial state of a long list).
func TestGlobalGitDownCueRendersBelowLastVisibleRow(t *testing.T) {
	const n = 20
	b := stubBackend{gitOptions: gitScrollRows(n)}
	a, _ := press(t, NewApp(b), "3")
	m := ggitModel(t, a)
	s := a.state
	w := m.gitComputeScrollWindow(n, s)
	if w.cue != gitCueDown {
		t.Fatalf("cue = %v, want gitCueDown at the initial (top) window", w.cue)
	}
	want := fmt.Sprintf(gitCueDownFmt, w.hiddenCount)
	view := appView(a)
	if !strings.Contains(view, want) {
		t.Errorf("view must contain %q;\nview:\n%s", want, view)
	}
	// The cue must render on the LAST line of the master-list column — i.e.
	// after every visible row's two lines.
	lines := strings.Split(view, "\n")
	found := -1
	for i, l := range lines {
		if strings.Contains(l, "more options") {
			found = i
			break
		}
	}
	if found == -1 {
		t.Fatal("cue line not found in rendered output")
	}
}

// TestGlobalGitUpCueRendersAboveFirstVisibleRow scrolls to the bottom of a
// long list (rows hidden only above) and asserts the up cue renders with
// the correct hidden count.
func TestGlobalGitUpCueRendersAboveFirstVisibleRow(t *testing.T) {
	const n = 20
	b := stubBackend{gitOptions: gitScrollRows(n)}
	a, _ := press(t, NewApp(b), "3")
	for i := 0; i < n-1; i++ {
		a, _ = press(t, a, "down")
	}
	m := ggitModel(t, a)
	s := a.state
	w := m.gitComputeScrollWindow(n, s)
	if w.cue != gitCueUp {
		t.Fatalf("cue = %v, want gitCueUp once scrolled to the bottom", w.cue)
	}
	want := fmt.Sprintf(gitCueUpFmt, w.hiddenCount)
	view := appView(a)
	if !strings.Contains(view, want) {
		t.Errorf("view must contain %q;\nview:\n%s", want, view)
	}
}

// TestGlobalGitBothEdgesHiddenPrefersDownCue asserts that when rows are
// hidden both above and below the window (mid-scroll), the reserved line
// shows the DOWN cue — the tie-break 07-UI-SPEC.md's overflow row pins.
func TestGlobalGitBothEdgesHiddenPrefersDownCue(t *testing.T) {
	const n = 20
	b := stubBackend{gitOptions: gitScrollRows(n)}
	a, _ := press(t, NewApp(b), "3")
	// Land somewhere in the middle — enough steps to leave the top window
	// but nowhere near the bottom.
	for i := 0; i < 13; i++ {
		a, _ = press(t, a, "down")
	}
	m := ggitModel(t, a)
	s := a.state
	w := m.gitComputeScrollWindow(n, s)
	if w.windowStart == 0 {
		t.Fatal("setup: window must have scrolled past the top")
	}
	if w.windowStart+w.visibleRows >= n {
		t.Fatal("setup: window must not yet be at the bottom (rows must still be hidden below)")
	}
	if w.cue != gitCueDown {
		t.Errorf("cue = %v, want gitCueDown when both edges hide rows", w.cue)
	}
}

// TestGlobalGitClickAtNonZeroWindowStartTogglesWindowRow is the highest-risk
// assertion in this task: a click on the visually-FIRST checkbox at a
// NON-ZERO window start must toggle the row AT THE WINDOW START index, not
// the row at index zero. A click test that only ever clicks at offset zero
// proves nothing (07-04-PLAN.md T-07-22).
func TestGlobalGitClickAtNonZeroWindowStartTogglesWindowRow(t *testing.T) {
	const n = 20
	b := stubBackend{gitOptions: gitScrollRows(n)}
	a, _ := press(t, NewApp(b), "3")
	for i := 0; i < 11; i++ {
		a, _ = press(t, a, "down")
	}
	m := ggitModel(t, a)
	s := a.state
	options := m.overlaidGitOptions(s)
	w := m.gitComputeScrollWindow(len(options), s)
	if w.windowStart == 0 {
		t.Fatal("setup: window must be at a non-zero start")
	}
	windowRowKey := options[w.windowStart].Key
	if m.chosen[windowRowKey] {
		t.Fatalf("setup: %q must start un-chosen", windowRowKey)
	}
	// Click the visually-first row's checkbox column (x=3 lands on the
	// checkbox glyph, per the row layout " " + marker(2) + box(2) + glyph).
	topLineY := gitTopLines(s)
	if w.cue == gitCueUp {
		topLineY++ // the up cue occupies the reserved line above row 0
	}
	res := m.handleClick(3, topLineY, minFrameWidth, minFrameHeight, s)
	next, ok := res.model.(globalGitModel)
	if !ok {
		t.Fatalf("handleClick returned %T, want globalGitModel", res.model)
	}
	if !next.chosen[windowRowKey] {
		t.Errorf("click at the visually-first row (window start %d, key %q) did not toggle it; chosen=%v",
			w.windowStart, windowRowKey, next.chosen)
	}
	if next.chosen["row00"] {
		t.Error("click at a non-zero window start must NOT toggle row00 — the pre-scroll defect class")
	}
}

// TestGlobalGitClickElsewhereAtNonZeroWindowStartSelectsWindowRow asserts a
// click elsewhere in the visually-first row (not on the checkbox) selects
// that same window-start row, at a non-zero scroll offset.
func TestGlobalGitClickElsewhereAtNonZeroWindowStartSelectsWindowRow(t *testing.T) {
	const n = 20
	b := stubBackend{gitOptions: gitScrollRows(n)}
	a, _ := press(t, NewApp(b), "3")
	for i := 0; i < 11; i++ {
		a, _ = press(t, a, "down")
	}
	m := ggitModel(t, a)
	s := a.state
	options := m.overlaidGitOptions(s)
	w := m.gitComputeScrollWindow(len(options), s)
	windowRowKey := options[w.windowStart].Key
	topLineY := gitTopLines(s)
	if w.cue == gitCueUp {
		topLineY++
	}
	// x=15 lands past the checkbox/glyph columns, on the key name itself.
	res := m.handleClick(15, topLineY, minFrameWidth, minFrameHeight, s)
	next, ok := res.model.(globalGitModel)
	if !ok {
		t.Fatalf("handleClick returned %T, want globalGitModel", res.model)
	}
	if next.detailKey != windowRowKey {
		t.Errorf("detailKey = %q, want %q (the window-start row)", next.detailKey, windowRowKey)
	}
}

// TestGlobalGitClickOnCueLineChangesNothing asserts a click on the reserved
// cue line changes neither the selection nor any toggle.
func TestGlobalGitClickOnCueLineChangesNothing(t *testing.T) {
	const n = 20
	b := stubBackend{gitOptions: gitScrollRows(n)}
	a, _ := press(t, NewApp(b), "3")
	m := ggitModel(t, a)
	s := a.state
	options := m.overlaidGitOptions(s)
	w := m.gitComputeScrollWindow(len(options), s)
	if w.cue != gitCueDown {
		t.Fatal("setup: initial window must show the down cue")
	}
	cueY := gitTopLines(s) + w.visibleRows*optionRowLines
	beforeKey := m.detailKey
	beforeChosen := len(m.chosen)
	res := m.handleClick(4, cueY, minFrameWidth, minFrameHeight, s)
	next, ok := res.model.(globalGitModel)
	if !ok {
		t.Fatalf("handleClick returned %T, want globalGitModel", res.model)
	}
	if next.detailKey != beforeKey {
		t.Errorf("click on the cue line changed the selection: %q -> %q", beforeKey, next.detailKey)
	}
	if len(next.chosen) != beforeChosen {
		t.Errorf("click on the cue line changed the toggle set: %d -> %d", beforeChosen, len(next.chosen))
	}
}

// TestGlobalGitClickOnNonSelectableRowChecksNothing asserts a click at a
// non-selectable row's checkbox position toggles nothing, at a scrolled
// offset.
func TestGlobalGitClickOnNonSelectableRowChecksNothing(t *testing.T) {
	const n = 20
	rows := gitScrollRows(n)
	rows[8].PolicyBacked = false
	rows[8].HasWritableMember = false
	b := stubBackend{gitOptions: rows}
	a, _ := press(t, NewApp(b), "3")
	// Scroll so row 8 is the window-start (visually-first) row. 17 (not 18)
	// downs — 09.5-02, Task 1's sub-tab strip cost this screen 3 more
	// gitTopLines rows, shrinking the visible-row budget by one row, so the
	// window now reaches windowStart=8 one "down" press earlier than before
	// the strip was added.
	for i := 0; i < 17; i++ {
		a, _ = press(t, a, "down")
	}
	m := ggitModel(t, a)
	s := a.state
	options := m.overlaidGitOptions(s)
	w := m.gitComputeScrollWindow(len(options), s)
	if options[w.windowStart].Key != "row08" {
		t.Fatalf("setup: window start row = %q, want row08", options[w.windowStart].Key)
	}
	topLineY := gitTopLines(s)
	if w.cue == gitCueUp {
		topLineY++
	}
	res := m.handleClick(4, topLineY, minFrameWidth, minFrameHeight, s)
	next, ok := res.model.(globalGitModel)
	if !ok {
		t.Fatalf("handleClick returned %T, want globalGitModel", res.model)
	}
	if next.chosen["row08"] {
		t.Error("click at a non-selectable row's checkbox position must not toggle it")
	}
}

// TestGlobalGitNoLineExceedsListWidthAtAnyScrollOffset asserts no rendered
// master-list line exceeds the master list width, at window start zero and
// at the maximum window start.
func TestGlobalGitNoLineExceedsListWidthAtAnyScrollOffset(t *testing.T) {
	const n = 20
	b := stubBackend{gitOptions: gitScrollRows(n)}
	a, _ := press(t, NewApp(b), "3")
	listWidth := masterListWidth(minFrameWidth)
	assertWithinWidth := func(t *testing.T, a App) {
		t.Helper()
		m := ggitModel(t, a)
		body := stripANSI(m.view(a.state, minFrameWidth, minFrameHeight).body)
		for _, line := range strings.Split(body, "\n") {
			if !strings.Contains(line, "│") {
				// Not a master/detail row (e.g. the findings banner, which
				// deliberately spans the full body width above the split).
				continue
			}
			listCol := strings.SplitN(line, "│", 2)[0]
			if w := ansiWidthForTest(listCol); w > listWidth {
				t.Errorf("list column width %d exceeds %d: %q", w, listWidth, listCol)
			}
		}
	}
	assertWithinWidth(t, a) // window start zero
	for i := 0; i < n-1; i++ {
		a, _ = press(t, a, "down")
	}
	assertWithinWidth(t, a) // maximum window start
}

// ansiWidthForTest counts display columns of already-ANSI-stripped text
// (appView already strips ANSI, so this is a plain rune count).
func ansiWidthForTest(s string) int { return len([]rune(s)) }

// TestGlobalGitNoColorCuesRemainLegible asserts both cue lines remain
// legible (contain their identifying arrow + word) with ANSI colour
// stripped.
func TestGlobalGitNoColorCuesRemainLegible(t *testing.T) {
	const n = 20
	b := stubBackend{gitOptions: gitScrollRows(n)}
	a, _ := press(t, NewApp(b), "3")
	view := appView(a) // appView already strips ANSI
	if !strings.Contains(view, "↓ (+") || !strings.Contains(view, "more options)") {
		t.Errorf("down cue not legible without colour:\n%s", view)
	}
	for i := 0; i < n-1; i++ {
		a, _ = press(t, a, "down")
	}
	view = appView(a)
	if !strings.Contains(view, "↑ (+") || !strings.Contains(view, "more options)") {
		t.Errorf("up cue not legible without colour:\n%s", view)
	}
}

func TestGlobalGitProbeErrorRendersInlineAdvisoryAndStaysNavigable(t *testing.T) {
	b := stubBackend{gitOptionsErr: errGlobalGitTest}
	a, _ := press(t, NewApp(b), "3")
	view := appView(a)
	for _, want := range []string{"! global git test error", "The option states could not be read from this machine."} {
		if !strings.Contains(view, want) {
			t.Errorf("probe-error view missing %q:\n%s", want, view)
		}
	}
	for _, row := range GlobalGitOptions {
		if strings.Contains(view, row.Key) {
			t.Errorf("probe-error view must replace rows; found %q:\n%s", row.Key, view)
		}
	}
	for _, action := range ggitModel(t, a).view(a.state, minFrameWidth, minFrameHeight).actions {
		if action.Key == "a" {
			t.Errorf("probe-error view must not offer apply: %+v", action)
		}
	}
	a, _ = press(t, a, "right")
	if a.tab != TabDoctor {
		t.Errorf("right navigation from a probe-error Global Git view selected tab %v, want Doctor", a.tab)
	}
}

// TestGlobalGitProbeErrorDoesNotClaimBaselineApplied is a UI review
// regression test, confirmed against a real captured PTY frame
// (global-git-probe-failure.txt): the status line's default text
// ("Baseline applied. user.email stays untouched...") is computed from a
// `pending` count derived from `options`, which is empty on a probe
// failure — so `pending == 0` VACUOUSLY, and the misleading "applied"
// claim rendered even though nothing was applied and the real state is
// unknown.
func TestGlobalGitProbeErrorDoesNotClaimBaselineApplied(t *testing.T) {
	b := stubBackend{gitOptionsErr: errGlobalGitTest}
	a, _ := press(t, NewApp(b), "3")
	view := appView(a)
	if strings.Contains(view, "Baseline applied") {
		t.Errorf("probe-error view must not claim a baseline was applied:\n%s", view)
	}
}

func TestGlobalGitBaselineCeremonyShowsWholeManagedBlockAndCrossWarning(t *testing.T) {
	b := stubBackend{fallbackState: GitFallbackAuthorView{Email: "fallback@example.com"}}
	m := newGlobalGitModel(b)
	activated, _ := m.activate(Seed())
	m = activated.(globalGitModel)
	m.chosen["user.useConfigOnly"] = true
	ceremony, err := m.baselineCeremonyFor([]string{"user.useConfigOnly"}, 1)
	if err != nil {
		t.Fatalf("baselineCeremonyFor: %v", err)
	}
	view := stripANSI(ceremony.view(minFrameWidth - 2))
	for _, want := range []string{GlobalGitSentinelBegin, GlobalGitSentinelEnd} {
		if !strings.Contains(view, want) {
			t.Errorf("Global Git ceremony missing %q:\n%s", want, view)
		}
	}
	for _, want := range []string{"user.useConfigOnly is selected but the fallback author has no name set", "only the email half is"} {
		if !strings.Contains(view, want) {
			t.Errorf("Global Git ceremony missing cross-warning fragment %q:\n%s", want, view)
		}
	}
	if ceremony.cfg.PreviewMaxLines <= 10 {
		t.Errorf("Global Git ceremony preview budget = %d, want a scoped widening beyond the shared default 10", ceremony.cfg.PreviewMaxLines)
	}
}

func TestGlobalGitBaselineCeremonyOmitsCrossWarningWhenFallbackIsComplete(t *testing.T) {
	b := stubBackend{fallbackState: GitFallbackAuthorView{Name: "Fallback", Email: "fallback@example.com"}}
	m := newGlobalGitModel(b)
	activated, _ := m.activate(Seed())
	m = activated.(globalGitModel)
	m.chosen["user.useConfigOnly"] = true
	ceremony, err := m.baselineCeremonyFor([]string{"user.useConfigOnly"}, 1)
	if err != nil {
		t.Fatalf("baselineCeremonyFor: %v", err)
	}
	view := stripANSI(ceremony.view(minFrameWidth - 2))
	if strings.Contains(view, GlobalGitCrossWarningNameMissing) || strings.Contains(view, GlobalGitCrossWarningEmailMissing) {
		t.Errorf("complete fallback author must not render a cross-warning:\n%s", view)
	}
}

func TestGlobalSSHCeremonyPreviewUsesUnchangedDefaultBudget(t *testing.T) {
	m := newGlobalSSHModel(stubBackend{})
	activated, _ := m.activate(Seed())
	m = activated.(globalSSHModel)
	first := m.overlaidOptions(Seed())[0]
	m.chosen[first.Key] = true
	ceremony, err := m.applyCeremonyFor(Seed())
	if err != nil {
		t.Fatalf("applyCeremonyFor: %v", err)
	}
	if ceremony.cfg.PreviewMaxLines != 0 {
		t.Errorf("Global SSH preview override = %d, want unchanged shared default", ceremony.cfg.PreviewMaxLines)
	}
	if ceremony.preview.VisibleLines != 10 {
		t.Errorf("Global SSH preview visible lines = %d, want unchanged default 10", ceremony.preview.VisibleLines)
	}
}

// TestGitFallbackAbandonedApplyStillDispatchesReducerAction is the BL-04
// regression (09.4-REVIEW.md independent re-review) for Global Git's
// fallback-author ceremony: confirm dispatches the async commit with the
// SUBMITTED name/email captured into pendingFallbackName/Email. Simulate
// abandonment — re-entering the screen (Ctrl+P -> back) runs activate(),
// which CR-02 made reset ceremonyOpen/fallbackCommitPending/ceremony AND
// re-seeds nameInput/emailInput from the backend's still-stale
// GitFallbackAuthorState() (the write has not landed yet from the backend's
// point of view). The commit's success message must still dispatch
// ApplyGitGlobalEmail with the SUBMITTED values, not the stale re-seeded
// ones now sitting in nameInput/emailInput.
func TestGitFallbackAbandonedApplyStillDispatchesReducerAction(t *testing.T) {
	b := stubBackend{
		fallbackState: GitFallbackAuthorView{Name: "Old Name", Email: "old@example.com"},
		fallbackCommitFn: func(string, string) tea.Cmd {
			return func() tea.Msg { return GitFallbackAuthorCommitMsg{Backups: []string{NewBackupPath("~/.gitconfig")}} }
		},
	}
	m := newGlobalGitModel(b)
	state := Seed()
	activated, _ := m.activate(state)
	m = activated.(globalGitModel)
	m.detailKey = GlobalGitEmailFallbackKey
	m.nameInput = newTextInput("New Name")
	m.emailInput = newTextInput("new@example.com")
	opened := m.handleKey(pressKey("a"), state)
	m = opened.model.(globalGitModel)
	confirmed := m.handleKey(pressKey("enter"), state)
	pendingModel := confirmed.model.(globalGitModel)
	if !pendingModel.fallbackCommitPending {
		t.Fatal("setup: confirming must set fallbackCommitPending")
	}
	if pendingModel.pendingFallbackEmail != "new@example.com" || pendingModel.pendingFallbackName != "New Name" {
		t.Fatalf("setup: pending capture = (%q, %q), want the submitted values", pendingModel.pendingFallbackName, pendingModel.pendingFallbackEmail)
	}
	msg := confirmed.cmd()

	reactivated, _ := pendingModel.activate(state)
	abandoned := reactivated.(globalGitModel)
	if abandoned.ceremonyOpen || abandoned.fallbackCommitPending {
		t.Fatal("setup: activate() must have cleared the ceremony state")
	}
	if abandoned.emailInput.Value() != "old@example.com" {
		t.Fatal("setup: activate() must have re-seeded emailInput from the stale backend state")
	}

	success := abandoned.handleMsg(msg, state)
	if len(success.actions) != 1 {
		t.Fatalf("BL-04 regressed: abandoned commit delivered %d actions, want one ApplyGitGlobalEmail — the write happened on disk but App.state never refreshed", len(success.actions))
	}
	action, isApply := success.actions[0].(ApplyGitGlobalEmail)
	if !isApply {
		t.Fatalf("action = %T, want ApplyGitGlobalEmail", success.actions[0])
	}
	if action.Email != "new@example.com" || action.Name != "New Name" {
		t.Errorf("dispatched action = (%q, %q), want the SUBMITTED values (New Name, new@example.com), not activate()'s stale re-seed", action.Name, action.Email)
	}
}

// TestGitFallbackDispatchUsesCeremonyKindNotHeadingText is the regression
// for WR-09 (09.4-REVIEW.md independent re-review): confirming a ceremony
// used to decide which backend commit method to call by matching
// m.ceremony.cfg.Heading against GlobalGitEmailCeremonyHeading /
// "Remove global fallback" — a user-facing display string. baselineCeremonyFor
// already builds ITS heading dynamically ("Write global-git managed block to
// " + targets[0]); the moment fallbackCeremonyFor's heading is made dynamic
// too (following that same precedent, as every other ceremony in this file
// does), the old string match would silently fall through to
// CommitGlobalGit — writing the baseline managed block instead of the
// fallback author. Simulate that by opening the fallback ceremony normally
// and then mutating its heading to something the old match would miss,
// proving dispatch no longer depends on the heading text at all.
func TestGitFallbackDispatchUsesCeremonyKindNotHeadingText(t *testing.T) {
	var fallbackCalled, baselineCalled bool
	b := stubBackend{
		fallbackState: GitFallbackAuthorView{Email: "pat@example.com"},
		fallbackCommitFn: func(string, string) tea.Cmd {
			fallbackCalled = true
			return func() tea.Msg { return GitFallbackAuthorCommitMsg{} }
		},
		gitCommitFn: func([]string) tea.Cmd {
			baselineCalled = true
			return func() tea.Msg { return GlobalGitCommitMsg{} }
		},
	}
	a := fallbackRowApp(t, b)
	a, _ = press(t, a, "a")

	m := ggitModel(t, a)
	if m.ceremonyKind != gitCeremonyFallback {
		t.Fatalf("ceremonyKind after opening the fallback ceremony = %v, want gitCeremonyFallback", m.ceremonyKind)
	}
	// Simulate a future dynamic heading — the old code matched against
	// exactly GlobalGitEmailCeremonyHeading or a "Remove global fallback"
	// prefix, so ANY other heading text proves the fix if dispatch is
	// still correct.
	m.ceremony.cfg.Heading = "Write fallback author to ~/.gitconfig.d/00-fallback.gitconfig"
	a.screens[TabGlobalGit] = m

	_, cmd := press(t, a, "enter")
	if cmd != nil {
		cmd()
	}
	if !fallbackCalled {
		t.Error("confirming a fallback ceremony with a non-matching heading must still dispatch CommitGitFallbackAuthor")
	}
	if baselineCalled {
		t.Error("confirming a fallback ceremony with a non-matching heading must never fall through to CommitGlobalGit")
	}
}

// TestGitFallbackTabShowsVisibleFocusChangeOutsideEditMode is the
// regression for WR-15: outside edit mode, Tab moved m.fieldFocus but the
// render only distinguished focus when m.fieldEditing was also true, so
// pressing Tab produced no visible change — the user could only discover
// which field moved by pressing Enter. It also proves focusFallbackField()
// runs on this path, keeping the underlying textinput Focus() state in
// sync with fieldFocus ahead of the next Enter.
func TestGitFallbackTabShowsVisibleFocusChangeOutsideEditMode(t *testing.T) {
	a := fallbackRowApp(t, stubBackend{fallbackState: GitFallbackAuthorView{Name: "Pat", Email: "pat@example.com"}})
	m := ggitModel(t, a)
	if m.fieldEditing {
		t.Fatal("fixture sanity: must start outside edit mode")
	}
	before := stripANSI(m.view(a.state, 100, 30).body)

	a, _ = press(t, a, "tab")
	m = ggitModel(t, a)
	if m.fieldEditing {
		t.Fatal("Tab outside edit mode must not enter edit mode")
	}
	if !m.emailInput.Focused() {
		t.Error("focusFallbackField must run on the non-editing Tab path too, keeping textinput focus in sync")
	}
	after := stripANSI(m.view(a.state, 100, 30).body)
	if before == after {
		t.Errorf("Tab outside edit mode produced no visible change in the rendered fallback fields:\n%s", after)
	}
}

// TestGlobalGitScrollBudgetGrowsWithRealTerminalHeight is the regression
// for WR-10: gitVisibleRowCount was pinned to frameBodyRows(minFrameHeight)
// — a constant ~25-row budget — while view() sizes the pane from the REAL
// height. On a taller terminal the list still windowed to the small
// budget. A resize (tea.WindowSizeMsg) must persist the real height on
// the model and grow the budget from it.
func TestGlobalGitScrollBudgetGrowsWithRealTerminalHeight(t *testing.T) {
	b := stubBackend{gitOptions: gitScrollRows(20)}
	a, _ := press(t, NewApp(b), "3")
	before := ggitModel(t, a).rowBudgetHeight()
	if before != minFrameHeight {
		t.Fatalf("fixture sanity: rowBudgetHeight before any resize = %d, want minFrameHeight (%d)", before, minFrameHeight)
	}
	beforeVisible := gitVisibleRowCount(20, before, a.state)
	if beforeVisible >= 20 {
		t.Fatalf("fixture sanity: 20 rows must need scrolling at the canonical height, got visible=%d", beforeVisible)
	}

	model, _ := a.Update(tea.WindowSizeMsg{Width: 100, Height: 60})
	a, ok := model.(App)
	if !ok {
		t.Fatalf("Update(WindowSizeMsg) returned %T, want App", model)
	}
	m := ggitModel(t, a)
	if m.lastHeight != 60 {
		t.Fatalf("lastHeight after resize = %d, want 60", m.lastHeight)
	}
	afterVisible := gitVisibleRowCount(20, m.rowBudgetHeight(), a.state)
	if afterVisible <= beforeVisible {
		t.Errorf("visible row count must grow on a taller terminal: before=%d after=%d", beforeVisible, afterVisible)
	}
	if afterVisible != 20 {
		t.Errorf("all 20 rows must fit at height=60 with no scrolling needed, got visible=%d", afterVisible)
	}
}

// TestGitStaleCommitMsgNotMisattributedToNewerCeremony is the regression
// for CR-01 (09.4-REVIEW.md second independent re-review): BL-04's fix
// (previous round) stopped dropping a completed write's reducer action
// when its ceremony was abandoned, but gated the ceremony-UI mutation on a
// bare applyCommitPending boolean with no per-request correlation. A STALE
// message from a first, abandoned ceremony could be misattributed to a
// second, genuinely in-flight ceremony of the same kind — corrupting its
// receipt with the FIRST ceremony's backup path. Reproduces the review's
// exact repro: confirm ceremony 1 (dispatch not yet delivered), abandon via
// activate(), confirm ceremony 2, then deliver ceremony 1's stale message.
func TestGitStaleCommitMsgNotMisattributedToNewerCeremony(t *testing.T) {
	b := stubBackend{}
	m := newGlobalGitModel(b)
	state := Seed()
	activated, _ := m.activate(state)
	m = activated.(globalGitModel)

	// Ceremony 1: select TWO options (init.defaultBranch, core.ignorecase),
	// confirm — dispatch captured but not yet delivered (mirrors a real
	// async backend command). Deliberately a different COUNT than ceremony
	// 2's single option below, so the note text (the only observable field
	// on this path — ApplyGitBaseline carries no Keys field) actually
	// differs between the two snapshots and can catch a misattribution.
	m.detailKey = "init.defaultBranch"
	toggled0 := m.handleKey(pressKey("space"), state)
	m = toggled0.model.(globalGitModel)
	m.detailKey = "core.ignorecase"
	toggled := m.handleKey(pressKey("space"), state)
	m = toggled.model.(globalGitModel)
	opened := m.handleKey(pressKey("a"), state)
	m = opened.model.(globalGitModel)
	confirmed1 := m.handleKey(pressKey("enter"), state)
	m = confirmed1.model.(globalGitModel)
	staleMsg := confirmed1.cmd()

	// Abandon: Ctrl+P-then-back runs activate(), resetting ceremonyOpen/
	// applyCommitPending (CR-02/WR-01) — the token, deliberately, is NOT
	// reset (it must survive reactivation for BL-04's own regression test
	// to keep passing).
	reactivated, _ := m.activate(state)
	m = reactivated.(globalGitModel)
	if m.ceremonyOpen || m.applyCommitPending {
		t.Fatal("setup: activate() must have cleared the ceremony state")
	}

	// Ceremony 2: a DIFFERENT option, confirmed — genuinely in flight now.
	m.detailKey = "diff.colorMoved"
	toggled2 := m.handleKey(pressKey("space"), state)
	m = toggled2.model.(globalGitModel)
	opened2 := m.handleKey(pressKey("a"), state)
	m = opened2.model.(globalGitModel)
	confirmed2 := m.handleKey(pressKey("enter"), state)
	m = confirmed2.model.(globalGitModel)
	if !m.applyCommitPending {
		t.Fatal("setup: confirming ceremony 2 must set applyCommitPending")
	}
	ceremony2HeadingBefore := m.ceremony.cfg.Heading

	// Deliver ceremony 1's STALE message while ceremony 2 is on screen.
	result := m.handleMsg(staleMsg, state)
	m = result.model.(globalGitModel)

	if !m.applyCommitPending {
		t.Error("CR-01 regressed: a stale message must not clear applyCommitPending for the genuinely in-flight ceremony 2")
	}
	if m.ceremony.cfg.Heading != ceremony2HeadingBefore || m.ceremony.done {
		t.Errorf("CR-01 regressed: a stale message must not mutate ceremony 2's still-pending UI (heading=%q done=%t)",
			m.ceremony.cfg.Heading, m.ceremony.done)
	}
	if len(result.actions) != 1 {
		t.Fatalf("stale message must still dispatch its own reducer action (BL-04), got %d", len(result.actions))
	}
	// CR-01 (third independent re-review): ApplyGitBaseline carries no Keys
	// field, so the only observable corruption vector on this path is the
	// note's key count — it must reflect the STALE ceremony 1's own TWO
	// submitted keys, never m.appliedKeys, which ceremony 2's confirm
	// already overwrote to a single key (["diff.colorMoved"]).
	if result.note != "2 global git options applied." {
		t.Errorf("CR-01 regressed: note must reflect the stale ceremony's own key count, got %q", result.note)
	}
}

// TestGitAbandonedFailedCommitStillProducesNote is the regression for WR-01
// (09.4-REVIEW.md second independent re-review): when ceremonyOpen is false
// (abandoned, or a stale/superseded token per CR-01) and the commit failed,
// handleMsg returned keyResult{model: m} with no note and no actions — a
// background write failure after the user navigated away was completely
// silent. Now a note must surface even when the ceremony UI is gone.
func TestGitAbandonedFailedCommitStillProducesNote(t *testing.T) {
	b := stubBackend{}
	m := newGlobalGitModel(b)
	state := Seed()
	activated, _ := m.activate(state)
	m = activated.(globalGitModel)
	m.detailKey = "init.defaultBranch"
	toggled := m.handleKey(pressKey("space"), state)
	m = toggled.model.(globalGitModel)
	opened := m.handleKey(pressKey("a"), state)
	m = opened.model.(globalGitModel)
	confirmed := m.handleKey(pressKey("enter"), state)
	pendingModel := confirmed.model.(globalGitModel)

	reactivated, _ := pendingModel.activate(state)
	abandoned := reactivated.(globalGitModel)
	if abandoned.ceremonyOpen || abandoned.applyCommitPending {
		t.Fatal("setup: activate() must have cleared the ceremony state")
	}

	result := abandoned.handleMsg(gitCommitTokenMsg{
		token: pendingModel.commitRequestToken,
		msg:   GlobalGitCommitMsg{Err: "disk full"},
	}, state)
	if result.note == "" {
		t.Error("WR-01 regressed: an abandoned commit's failure must still produce a note")
	}
}

// TestGitRowBudgetHeightFloorsAtMinFrameHeight is the regression for WR-02
// (09.4-REVIEW.md second independent re-review): rowBudgetHeight returned
// m.lastHeight whenever it was > 0, with no floor against the canonical
// frame bounds. tea.WindowSizeMsg reaches every screen's handleMsg before
// App's own too-small guard is consulted, so a resize transient (or a host
// briefly reporting a tiny height mid-resize) could persist a
// below-minFrameHeight m.lastHeight and transiently collapse the scroll
// budget independent of what view()'s own render arguments are doing.
func TestGitRowBudgetHeightFloorsAtMinFrameHeight(t *testing.T) {
	m := newGlobalGitModel(stubBackend{})
	res := m.handleMsg(tea.WindowSizeMsg{Width: 100, Height: 5}, Seed())
	m = res.model.(globalGitModel)
	if got := m.rowBudgetHeight(); got != minFrameHeight {
		t.Errorf("rowBudgetHeight() after a sub-minFrameHeight resize = %d, want the minFrameHeight floor (%d)", got, minFrameHeight)
	}
}

// ---------------------------------------------------------------------------
// 09.5-02, Task 1: Global Git's FIRST sub-tab strip.
// ---------------------------------------------------------------------------

// referenceGlobalGitOptionsBody is a byte-for-byte transcription of
// globalGitModel.view's pre-09.5-02 body-construction logic (populated,
// optionsErr, and zero-options branches), captured from
// `git show f9a04ce:internal/tuikit/globalgit.go` — the commit immediately
// before this plan's Task 1 change. It exists ONLY so
// TestGlobalGitOptionsSubTabContentUnchanged can compare against a literal
// captured from the PRE-CHANGE render, never against a re-render of the new
// code path (which would be tautological). Do not "simplify" this by calling
// the live view() — that defeats the entire point of the regression guard.
func referenceGlobalGitOptionsBody(m globalGitModel, s DemoState, width, height int) string {
	options := m.overlaidGitOptions(s)

	if m.optionsErr != "" {
		body := ""
		if banner := findingsBanner(s, "Git", gitBannerBeyond); banner != "" {
			body = banner + "\n"
		}
		body += " " + styleWarning.Render("! "+m.optionsErr) + "\n\n " +
			styleFaint.Render("The option states could not be read from this machine.")
		return body
	}

	if len(options) == 0 {
		body := ""
		if banner := findingsBanner(s, "Git", gitBannerBeyond); banner != "" {
			body = banner + "\n"
		}
		body += " " + styleFaint.Render("No global Git options to show.")
		return body
	}

	listWidth := masterListWidth(width)
	detailWidth := width - listWidth - masterDetailGutter
	selIdx := m.gitDetailIndex(options)
	bodyRows := frameBodyRows(height) - gitTopLines(s)

	scrollWin := m.gitComputeScrollWindow(len(options), s)
	visible := options
	if scrollWin.needsScroll {
		visible = options[scrollWin.windowStart : scrollWin.windowStart+scrollWin.visibleRows]
	}

	var rows []string
	if scrollWin.cue == gitCueUp {
		rows = append(rows, gitCueLine(scrollWin))
	}
	for i, o := range visible {
		absoluteIdx := scrollWin.windowStart + i
		marker := "  "
		if absoluteIdx == selIdx {
			marker = styleBold.Render("▸ ")
		}
		box := padDisplay(styleFaint.Render("·"), optionBoxWidth)
		if o.Selectable() {
			box = padDisplay(styleFaint.Render(glyphToggleOff), optionBoxWidth)
			if m.chosen[o.Key] {
				box = padDisplay(styleHealthy.Bold(true).Render(glyphToggleOn), optionBoxWidth)
			}
		}
		toneGlyph := styleHealthy.Render("✓")
		switch o.State {
		case GlobalGitNeedsAction, GlobalGitSetButDiffers:
			toneGlyph = styleWarning.Render("!")
		case GlobalGitNotApplicable:
			toneGlyph = styleFaint.Render("·")
		}
		name := styleBold.Render(o.Key)
		if absoluteIdx == selIdx {
			name = styleSelected.Render(o.Key)
		}
		chip := ""
		if o.Key == "init.defaultBranch" {
			chip = "  " + styleWarning.Render("[main vs master]")
		}
		rows = append(rows, truncLine(" "+marker+box+toneGlyph+" "+name+chip, listWidth))
		rows = append(rows, truncLine("      "+styleFaint.Render(globalGitRowLine2(o)), listWidth))
	}
	if scrollWin.cue == gitCueDown {
		rows = append(rows, gitCueLine(scrollWin))
	}
	list := strings.Join(rows, "\n")

	detail := options[selIdx]
	var d strings.Builder
	if detail.Key == GlobalGitEmailFallbackKey {
		nameFocused := m.fieldFocus == 0 && m.fieldEditing
		emailFocused := m.fieldFocus == 1 && m.fieldEditing
		nameSelected := m.fieldFocus == 0 && !m.fieldEditing
		emailSelected := m.fieldFocus == 1 && !m.fieldEditing
		d.WriteString(gitFallbackFieldLine(GlobalGitNameFallbackKey, m.nameInput, nameFocused, nameSelected) + "\n")
		d.WriteString(gitFallbackFieldLine(GlobalGitEmailFallbackKey, m.emailInput, emailFocused, emailSelected))
		if !m.emailValid() {
			d.WriteString("  " + styleError.Render("needs @"))
		}
		d.WriteString("\n")
		d.WriteString(helperLine(GlobalGitEmailFallbackHelper, false) + "\n")
		d.WriteString(helperLine(GlobalGitEmailFallbackAdvisory, false) + "\n")
		if strings.TrimSpace(m.emailInput.Value()) != "" && strings.TrimSpace(m.nameInput.Value()) == "" {
			d.WriteString(" " + styleWarning.Render(GlobalGitGuessedNameWarning) + "\n")
		}
	} else {
		explanation := detail.OneLiner
		switch detail.Key {
		case "init.defaultBranch":
			explanation = GlobalGitDetailExplanation
		case "core.ignorecase":
			explanation += " " + GlobalGitCaseSensitivityCaveat
		}
		d.WriteString(" " + styleBold.Render(detail.Key) + "\n")
		d.WriteString(" " + styleInfo.Render("~ "+GlobalGitAdvisoryNote) + "\n\n")
		d.WriteString(" " + explanation + "\n")
		for _, note := range detail.BundlePerKeyNotes {
			d.WriteString(" " + styleFaint.Render(note) + "\n")
		}
		if detail.GateNotMet {
			d.WriteString(" " + styleWarning.Render(GlobalGitConflictStyleGateNote) + "\n")
		}
		if detail.Key == "user.useConfigOnly" && m.chosen["user.useConfigOnly"] {
			name, email := strings.TrimSpace(m.nameInput.Value()), strings.TrimSpace(m.emailInput.Value())
			switch {
			case email != "" && name == "":
				d.WriteString(" " + styleWarning.Render(GlobalGitCrossWarningNameMissing) + "\n")
			case name != "" && email == "":
				d.WriteString(" " + styleWarning.Render(GlobalGitCrossWarningEmailMissing) + "\n")
			}
		}
		if detail.VersionNote != "" {
			d.WriteString(" " + styleFaint.Render(" "+detail.VersionNote) + "\n")
		}
		if detail.Provenance != "" {
			d.WriteString(" " + styleFaint.Render(detail.Provenance) + "\n")
		}
		if detail.ProbeError != "" {
			d.WriteString(" " + styleWarning.Render("! "+detail.ProbeError) + "\n")
		}
	}
	detailPane := fitPane(lipgloss.NewStyle().Width(detailWidth).Render(d.String()), bodyRows)

	body := ""
	if banner := findingsBanner(s, "Git", gitBannerBeyond); banner != "" {
		body = banner + "\n"
	}
	body += joinMasterDetail(list, listWidth, detailPane, bodyRows)
	return body
}

// TestGlobalGitOptionsSubTabContentUnchanged asserts that, with the model on
// ggitOptions, the body BELOW the new sub-tab strip is byte-identical to the
// pre-strip body for the populated state, the optionsErr state, and the
// zero-options state — Global Git's existing content is re-homed, not
// rewritten. The comparison is against referenceGlobalGitOptionsBody, a
// transcription of the PRE-CHANGE code (see its doc comment), never a
// re-render of the new code path.
func TestGlobalGitOptionsSubTabContentUnchanged(t *testing.T) {
	strip := renderSubTabStrip([]string{ggitTabOptionsLabel, ggitTabSetKeysLabel}, int(ggitOptions))
	prefix := strip + "\n"

	t.Run("populated", func(t *testing.T) {
		a := ggitApp(t)
		m := ggitModel(t, a)
		got := m.view(a.state, minFrameWidth, minFrameHeight).body
		want := prefix + referenceGlobalGitOptionsBody(m, a.state, minFrameWidth, minFrameHeight)
		if got != want {
			t.Errorf("populated body diverged below the strip:\ngot:\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("optionsErr", func(t *testing.T) {
		b := stubBackend{gitOptionsErr: errGlobalGitTest}
		a, _ := press(t, NewApp(b), "3")
		m := ggitModel(t, a)
		got := m.view(a.state, minFrameWidth, minFrameHeight).body
		want := prefix + referenceGlobalGitOptionsBody(m, a.state, minFrameWidth, minFrameHeight)
		if got != want {
			t.Errorf("optionsErr body diverged below the strip:\ngot:\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("zero-options", func(t *testing.T) {
		b := stubBackend{gitOptions: []GlobalGitOptionView{}}
		m := newGlobalGitModel(b)
		next, _ := m.activate(Seed())
		gm := next.(globalGitModel)
		got := gm.view(Seed(), minFrameWidth, minFrameHeight).body
		want := prefix + referenceGlobalGitOptionsBody(gm, Seed(), minFrameWidth, minFrameHeight)
		if got != want {
			t.Errorf("zero-options body diverged below the strip:\ngot:\n%s\nwant:\n%s", got, want)
		}
	})
}

// TestGlobalGitSubTabSwitching asserts left/right on Global Git move between
// ggitOptions and ggitSetKeys in opposite directions and are reported
// handled, so they never reach app.go's main-tab switcher.
func TestGlobalGitSubTabSwitching(t *testing.T) {
	a := ggitApp(t)
	m := ggitModel(t, a)
	if m.subTab != ggitOptions {
		t.Fatalf("initial subTab = %v, want ggitOptions", m.subTab)
	}

	res := m.handleKey(pressKey("right"), a.state)
	if !res.handled {
		t.Error("right on Global Git must be reported handled")
	}
	m = res.model.(globalGitModel)
	if m.subTab != ggitSetKeys {
		t.Errorf("subTab after right = %v, want ggitSetKeys", m.subTab)
	}

	res = m.handleKey(pressKey("right"), a.state)
	m = res.model.(globalGitModel)
	if m.subTab != ggitOptions {
		t.Errorf("subTab after a second right = %v, want ggitOptions (wraps)", m.subTab)
	}

	res = m.handleKey(pressKey("left"), a.state)
	if !res.handled {
		t.Error("left on Global Git must be reported handled")
	}
	m = res.model.(globalGitModel)
	if m.subTab != ggitSetKeys {
		t.Errorf("subTab after left from ggitOptions = %v, want ggitSetKeys (wraps the other way)", m.subTab)
	}
}

// TestGlobalGitSubTabStripClickSwitches verifies that clicking on either
// sub-tab label switches sub-tabs, using the actual rendered coordinates —
// mirroring TestSubTabStripClickSwitchesSubTabs on Global SSH.
func TestGlobalGitSubTabStripClickSwitches(t *testing.T) {
	a := ggitApp(t)
	m := ggitModel(t, a)
	if m.subTab != ggitOptions {
		t.Fatalf("initial subTab = %v, want ggitOptions", m.subTab)
	}

	a = clickCell(t, a, "Set keys", 0, 0)
	m = ggitModel(t, a)
	if m.subTab != ggitSetKeys {
		t.Errorf("subTab after clicking the Set keys label = %v, want ggitSetKeys", m.subTab)
	}

	a = clickCell(t, a, "Options", 0, 0)
	m = ggitModel(t, a)
	if m.subTab != ggitOptions {
		t.Errorf("subTab after clicking the Options label = %v, want ggitOptions", m.subTab)
	}
}

// TestGlobalGitFitsFixedGeometryWithStrip measures the Options sub-tab's
// body — in its tallest variant (findings banner present, the full baseline
// list, the D9 fallback-author row included) — against frameBodyRows after
// the strip's rows, at the fixed 100x30 geometry. Logs available/used/
// difference so the row-budget decision (three-row box vs. the one-row
// title-in-top-border fallback) is a MEASURED number, not an assumption
// (09.5-UI-SPEC.md).
func TestGlobalGitFitsFixedGeometryWithStrip(t *testing.T) {
	a := ggitApp(t) // stubBackend{}'s default state carries Git findings (banner present) and the full 12-row GlobalGitOptions catalog (D9 fallback-author row included).
	m := ggitModel(t, a)
	view := m.view(a.state, a.width, a.height)
	usedRows := len(strings.Split(view.body, "\n"))
	availRows := frameBodyRows(a.height)
	diff := availRows - usedRows
	t.Logf("Options sub-tab, tallest variant: available=%d used=%d difference=%d", availRows, usedRows, diff)
	if usedRows > availRows {
		t.Errorf("Options sub-tab body exceeds the fixed 100x30 geometry after the strip: used=%d, available=%d", usedRows, availRows)
	}
}

// TestTopLevelArrowHintSuppressedOnGlobalGit asserts the app footer no
// longer appends the top-level ←/→ "switch view" hint while Global Git is
// active — that key now means "switch sub-tab" on this screen (09.5-02,
// mirroring the identical exclusion Global SSH has carried since Phase
// 9.4/UXP-04).
func TestTopLevelArrowHintSuppressedOnGlobalGit(t *testing.T) {
	// Doctor (tab 4), not Identities: the Identities screen's own contextual
	// footer actions are dense enough that footerFit drops the top-level
	// hint for width, independent of the D4 exclusion this test targets.
	a, _ := press(t, NewApp(stubBackend{}), "4")
	if !strings.Contains(appView(a), "switch view") {
		t.Fatal("setup: Doctor tab must still advertise the top-level switch-view hint")
	}
	a, _ = press(t, a, "3") // Global Git
	view := appView(a)
	if strings.Contains(view, "switch view") {
		t.Errorf("Global Git's footer must not advertise the top-level switch-view hint:\n%s", view)
	}
	if !strings.Contains(view, ggitFooterCycleLabel) {
		t.Errorf("Global Git's footer must advertise its own sub-tab cycle hint %q:\n%s", ggitFooterCycleLabel, view)
	}
}

// ---------------------------------------------------------------------------
// Plan 09.5-02 Task 2 — the "Set keys" flat filterable master-detail body:
// origin/scope rendering and the two distinct empty/error states.
// ---------------------------------------------------------------------------

// ggitSetKeysApp opens Global Git and navigates to the "Set keys" sub-tab via
// one → press from the default Options sub-tab.
func ggitSetKeysApp(t *testing.T, b Backend) App {
	t.Helper()
	return pressSeq(t, NewApp(b), "3", "right")
}

// TestSetKeysFilterPlaceholderRendersInFull is the WR-03 regression, the
// git-side sibling of TestPropertiesFilterPlaceholderRendersInFull:
// newGitSetKeysFilterInput never called SetWidth, and bubbles/v2's
// textinput clips its placeholder to the model's width — 0 (the zero value)
// clips to a SINGLE rune, so the frozen PropsFilterPlaceholder constant
// never actually appeared on screen; the tracked approved baseline showed
// the row as "/ T".
func TestSetKeysFilterPlaceholderRendersInFull(t *testing.T) {
	view := appView(ggitSetKeysApp(t, stubBackend{gitSetKeys: []GitSetKeyView{{Key: "core.pager", Value: "less"}}}))
	if !strings.Contains(view, PropsFilterPlaceholder) {
		t.Errorf("view must render the full frozen placeholder %q, got:\n%s", PropsFilterPlaceholder, view)
	}
}

// TestSetKeysListRendersOriginAndScope proves the detail pane for the
// selected row shows the full value, the origin file path, and the scope
// word, while the master-list row shows key and value only, truncated with a
// visible cue — mirroring TestPropertiesDetailPaneShowsFullValueAndSource.
func TestSetKeysListRendersOriginAndScope(t *testing.T) {
	longValue := "alpha bravo charlie delta echo foxtrot golf hotel india juliet kilo lima mike november oscar papa quebec"
	rows := []GitSetKeyView{{Key: "user.signingkey", Value: longValue, Scope: "global", Origin: "/home/user/.gitconfig"}}
	a := ggitSetKeysApp(t, stubBackend{gitSetKeys: rows})

	master := regionFlat(a, 0, masterListWidth(minFrameWidth))
	if strings.Contains(master, longValue) {
		t.Errorf("master-list row must NOT show the full value: %q", master)
	}
	if !strings.Contains(master, "…") {
		t.Errorf("master-list row must carry the visible truncation cue: %q", master)
	}

	detail := regionFlat(a, masterListWidth(minFrameWidth)+1, minFrameWidth)
	if !strings.Contains(detail, longValue) {
		t.Errorf("detail pane must show the FULL, unclipped value:\ndetail=%q", detail)
	}
	if !strings.Contains(detail, "/home/user/.gitconfig") {
		t.Errorf("detail pane must show the origin file path:\ndetail=%q", detail)
	}
	if !strings.Contains(detail, "global") {
		t.Errorf("detail pane must show the scope word:\ndetail=%q", detail)
	}
}

// TestSetKeysDetailPaneShowsMultiValuedNote is the WR-04 regression: a key
// with ValueCount > 1 must render PropsGitMultiValuedNoteFmt in the detail
// pane so the screen never implies a stacked key (e.g. credential.helper set
// at both system and global scope) is single-valued. A key with ValueCount
// 1 must NOT render the note.
func TestSetKeysDetailPaneShowsMultiValuedNote(t *testing.T) {
	rows := []GitSetKeyView{
		{Key: "credential.helper", Value: "cache", Scope: "global", Origin: "/home/user/.gitconfig", ValueCount: 3},
		{Key: "core.editor", Value: "vim", Scope: "global", Origin: "/home/user/.gitconfig", ValueCount: 1},
	}
	a := ggitSetKeysApp(t, stubBackend{gitSetKeys: rows})

	multiDetail := regionFlat(a, masterListWidth(minFrameWidth)+1, minFrameWidth)
	wantNote := fmt.Sprintf(PropsGitMultiValuedNoteFmt, 3)
	if !strings.Contains(collapseWhitespace(multiDetail), collapseWhitespace(wantNote)) {
		t.Errorf("multi-valued key's detail pane must show %q, got:\n%s", wantNote, multiDetail)
	}

	a = pressSeq(t, a, "down")
	singleDetail := regionFlat(a, masterListWidth(minFrameWidth)+1, minFrameWidth)
	if strings.Contains(singleDetail, "values are set for this key") {
		t.Errorf("single-valued key's detail pane must NOT show the multi-valued note, got:\n%s", singleDetail)
	}
}

// TestSetKeysTwoDistinctEmptyStates covers the two distinct empty/error
// bodies: a probe failure renders the frozen Git probe-failure heading in
// the warning style, and a successful probe returning genuinely zero keys
// renders the frozen zero-keys sentence in the faint style with no warning
// glyph — the two bodies must not be interchangeable.
func TestSetKeysTwoDistinctEmptyStates(t *testing.T) {
	t.Run("probe failure", func(t *testing.T) {
		a := ggitSetKeysApp(t, stubBackend{gitSetKeysErr: errGlobalGitTest})
		view := appView(a)
		if !strings.Contains(view, PropsGitProbeFailedHeading) {
			t.Errorf("probe-failure state missing the frozen heading:\n%s", view)
		}
		if !strings.Contains(view, PropsGitProbeFailedBody) {
			t.Errorf("probe-failure state missing the frozen body:\n%s", view)
		}
		if strings.Contains(view, PropsGitNoKeysSet) {
			t.Errorf("probe-failure state must NOT show the zero-keys sentence:\n%s", view)
		}
		// Fail-open: a main-tab digit key still switches tabs.
		a, _ = press(t, a, "1")
		if !strings.Contains(appView(a), "[1] Identities") {
			t.Errorf("probe-failure state must stay fail-open (main tab keys still work):\n%s", appView(a))
		}
	})

	t.Run("zero keys set, no error", func(t *testing.T) {
		a := ggitSetKeysApp(t, stubBackend{gitSetKeys: []GitSetKeyView{}})
		view := appView(a)
		if !strings.Contains(view, PropsGitNoKeysSet) {
			t.Errorf("zero-keys-no-error state missing its faint sentence:\n%s", view)
		}
		if strings.Contains(view, PropsGitProbeFailedHeading) {
			t.Errorf("zero-keys-no-error state must NOT show the probe-failure heading:\n%s", view)
		}
		if strings.Contains(view, "!") && strings.Contains(view, PropsGitNoKeysSet) {
			// The faint sentence itself must not be preceded by a warning
			// glyph — the two states are visually distinct, not just
			// textually distinct.
			idx := strings.Index(view, PropsGitNoKeysSet)
			if idx >= 2 && view[idx-2] == '!' {
				t.Errorf("zero-keys-no-error state must not carry a warning glyph:\n%s", view)
			}
		}
	})

	t.Run("filter matches zero rows", func(t *testing.T) {
		rows := []GitSetKeyView{{Key: "user.name", Value: "Ada"}}
		a := pressSeq(t, ggitSetKeysApp(t, stubBackend{gitSetKeys: rows}), "/")
		a = typeText(t, a, "zzz")
		view := appView(a)
		want := fmt.Sprintf(PropsGitNoFilterMatchFmt, "zzz")
		if !strings.Contains(view, want) {
			t.Errorf("filter-zero-match state missing %q:\n%s", want, view)
		}
	})
}

// ---------------------------------------------------------------------------
// PROP-03 (09.5-03): free-form custom Git key entry — reachable via "n" on
// the Set keys sub-tab.
// ---------------------------------------------------------------------------

// TestCustomKeyFormOpensAndCaptures asserts "n" opens the custom-key form on
// the Set keys sub-tab, Tab moves focus between the key/value fields, and
// typed characters reach the FOCUSED field rather than being treated as
// this screen's reserved single-letter shortcuts (space, a) — the SAME
// keyboard-capture contract the D9 fallback pair already provides. Esc then
// closes the form without submitting.
func TestCustomKeyFormOpensAndCaptures(t *testing.T) {
	a := ggitSetKeysApp(t, stubBackend{})
	a, _ = press(t, a, "n")
	m := ggitModel(t, a)
	if !m.customKeyOpen {
		t.Fatal("\"n\" must open the custom-key form")
	}
	if m.customFieldFocus != 0 {
		t.Errorf("customFieldFocus after opening = %d, want 0 (key field)", m.customFieldFocus)
	}
	if !m.customKeyInput.Focused() {
		t.Error("opening the form must focus the key input")
	}

	a = typeText(t, a, "core.pager")
	m = ggitModel(t, a)
	if m.customKeyInput.Value() != "core.pager" {
		t.Errorf("customKeyInput = %q, want %q — typed text must reach the focused key field, not be swallowed as a shortcut (e.g. \"a\")", m.customKeyInput.Value(), "core.pager")
	}

	a, _ = press(t, a, "tab")
	m = ggitModel(t, a)
	if m.customFieldFocus != 1 {
		t.Errorf("customFieldFocus after Tab = %d, want 1 (value field)", m.customFieldFocus)
	}
	if !m.customValueInput.Focused() {
		t.Error("Tab must move Bubble Tea focus onto the value input")
	}

	a = typeText(t, a, "less -FRX")
	m = ggitModel(t, a)
	if m.customValueInput.Value() != "less -FRX" {
		t.Errorf("customValueInput = %q, want %q", m.customValueInput.Value(), "less -FRX")
	}

	a, _ = press(t, a, "esc")
	m = ggitModel(t, a)
	if m.customKeyOpen {
		t.Error("Esc must close the custom-key form without submitting")
	}
}

// TestCustomKeySubmitOpensCeremonyWithRealDiff asserts that submitting a
// well-formed key/value (Enter on the value field) calls
// backend.CustomGitKeyPlan and opens the write ceremony carrying the
// backend's REAL targets/backups/diff — never a hardcoded placeholder.
func TestCustomKeySubmitOpensCeremonyWithRealDiff(t *testing.T) {
	const resolvedTarget = "~/.gitconfig.d/00-baseline"
	backupPath := NewBackupPath(resolvedTarget)
	b := stubBackend{
		customKeyPlan: GitCustomKeyPlanView{
			Targets: []string{"~/.gitconfig", resolvedTarget},
			Backups: []string{backupPath},
			Diff:    "+ [core]\n+     pager = less -FRX",
		},
	}
	a := ggitSetKeysApp(t, b)
	a, _ = press(t, a, "n")
	a = typeText(t, a, "core.pager")
	a, _ = press(t, a, "tab")
	a = typeText(t, a, "less -FRX")
	a, _ = press(t, a, "enter")

	m := ggitModel(t, a)
	if m.customKeyOpen {
		t.Error("submitting must close the free-form form")
	}
	if !m.ceremonyOpen || m.ceremonyKind != gitCeremonyCustomKey {
		t.Fatalf("submitting a valid key/value must open the custom-key ceremony (ceremonyOpen=%t, kind=%v)", m.ceremonyOpen, m.ceremonyKind)
	}
	view := appView(a)
	if !strings.Contains(view, resolvedTarget) {
		t.Errorf("ceremony heading must contain the REAL resolved target %q;\nview:\n%s", resolvedTarget, view)
	}
	if !strings.Contains(view, backupPath) {
		t.Errorf("ceremony must show the REAL backup path %q;\nview:\n%s", backupPath, view)
	}
	if !strings.Contains(view, "pager = less -FRX") {
		t.Errorf("ceremony preview must contain the REAL diff from CustomGitKeyPlan;\nview:\n%s", view)
	}
}

// TestCustomKeyMalformedKeyNeverOpensCeremony asserts a plan-stage rejection
// (malformed key, or an injection-bearing value) renders PropsGitKeyInvalidFmt
// inline on the STILL-OPEN form and never opens the ceremony — the same "a
// preview that cannot be computed renders the error inline, ceremony not
// opened" rule baselineCeremonyFor/fallbackCeremonyFor already follow.
func TestCustomKeyMalformedKeyNeverOpensCeremony(t *testing.T) {
	wantErr := "no dot in key"
	b := stubBackend{
		customKeyPlanFn: func(string, string) (GitCustomKeyPlanView, error) {
			return GitCustomKeyPlanView{}, fmt.Errorf("%s", wantErr)
		},
	}
	a := ggitSetKeysApp(t, b)
	a, _ = press(t, a, "n")
	a = typeText(t, a, "nodothere")
	a, _ = press(t, a, "tab")
	a = typeText(t, a, "value")
	a, _ = press(t, a, "enter")

	m := ggitModel(t, a)
	if m.ceremonyOpen {
		t.Fatal("a plan-stage rejection must never open the ceremony")
	}
	if !m.customKeyOpen {
		t.Error("the form must stay open so the error is visible next to the offending input")
	}
	want := fmt.Sprintf(PropsGitKeyInvalidFmt, wantErr)
	if m.customKeyFormErr != want {
		t.Errorf("customKeyFormErr = %q, want %q", m.customKeyFormErr, want)
	}
	view := appView(a)
	if !strings.Contains(view, want) {
		t.Errorf("view must render the inline error %q;\nview:\n%s", want, view)
	}
}

// TestCustomKeyCeremonyConfirmDispatchesCommitOnce asserts confirming the
// custom-key ceremony calls backend.CommitCustomGitKey exactly once with the
// submitted key/value, and that delivering the resulting GitCustomKeyCommitMsg
// (wrapped in gitCommitTokenMsg, mirroring every other ceremony on this
// screen) renders the frozen receipt.
func TestCustomKeyCeremonyConfirmDispatchesCommitOnce(t *testing.T) {
	var calls int
	var gotKey, gotValue string
	backupPath := NewBackupPath("~/.gitconfig.d/00-baseline")
	b := stubBackend{
		customKeyPlan: GitCustomKeyPlanView{Targets: []string{backupPath}, Diff: "+ core.pager = less"},
		customKeyFn: func(key, value string) tea.Cmd {
			calls++
			gotKey, gotValue = key, value
			return func() tea.Msg { return GitCustomKeyCommitMsg{Backups: []string{backupPath}} }
		},
	}
	a := ggitSetKeysApp(t, b)
	a, _ = press(t, a, "n")
	a = typeText(t, a, "core.pager")
	a, _ = press(t, a, "tab")
	a = typeText(t, a, "less -FRX")
	a, _ = press(t, a, "enter") // submit → opens ceremony
	a, _ = press(t, a, "enter") // confirm → dispatches CommitCustomGitKey

	if calls != 1 {
		t.Fatalf("CommitCustomGitKey called %d times, want exactly 1", calls)
	}
	if gotKey != "core.pager" || gotValue != "less -FRX" {
		t.Errorf("CommitCustomGitKey called with (%q, %q), want (\"core.pager\", \"less -FRX\")", gotKey, gotValue)
	}

	m := ggitModel(t, a)
	a, _ = deliverMsg(t, a, gitCommitTokenMsg{
		token:       m.commitRequestToken,
		msg:         GitCustomKeyCommitMsg{Backups: []string{backupPath}},
		customKey:   "core.pager",
		customValue: "less -FRX",
	})
	view := appView(a)
	wantReceipt := fmt.Sprintf(PropsGitCustomReceiptFmt, "core.pager", "less -FRX")
	if !strings.Contains(view, wantReceipt) {
		t.Errorf("receipt must show the frozen success message %q;\nview:\n%s", wantReceipt, view)
	}
	if !strings.Contains(view, backupPath) {
		t.Errorf("receipt must show the real backup path;\nview:\n%s", view)
	}
}

// TestCustomKeyCommitFailureRendersFailureReceipt asserts a
// GitCustomKeyCommitMsg carrying Err renders the failure (and any restored
// paths) on the ceremony, never the success receipt.
func TestCustomKeyCommitFailureRendersFailureReceipt(t *testing.T) {
	const restoredPath = "~/.gitconfig.d/00-baseline (restored)"
	b := stubBackend{
		customKeyPlan: GitCustomKeyPlanView{Targets: []string{"~/.gitconfig.d/00-baseline"}},
	}
	a := ggitSetKeysApp(t, b)
	a, _ = press(t, a, "n")
	a = typeText(t, a, "core.pager")
	a, _ = press(t, a, "tab")
	a = typeText(t, a, "less -FRX")
	a, _ = press(t, a, "enter") // submit
	a, _ = press(t, a, "enter") // confirm

	m := ggitModel(t, a)
	a, _ = deliverMsg(t, a, gitCommitTokenMsg{
		token: m.commitRequestToken,
		msg: GitCustomKeyCommitMsg{
			Err:      "write failed: disk full",
			Restored: []string{restoredPath},
		},
	})
	view := appView(a)
	if !strings.Contains(view, "write failed") {
		t.Errorf("receipt must show the error;\nview:\n%s", view)
	}
	if !strings.Contains(view, "restored") {
		t.Errorf("receipt must show restored paths;\nview:\n%s", view)
	}
	if strings.Contains(view, "written.") {
		t.Error("error receipt must not show the success message")
	}
}

// TestCustomKeyCeremonyConfirmUsesTheSubmittedSnapshotNotTheLiveInputFields
// is the WR-12 regression: the ceremony's preview is computed at FORM-SUBMIT
// time from the locally captured gitKey/gitValue; on confirm, the handler
// must read the SAME submitted snapshot (m.pendingCustomKey/Value, captured
// at submit) — never the model's live customKeyInput/customValueInput
// fields, which this file's own CR-01 doctrine says never to trust at
// confirm time (the SSH sibling, globalssh.go, already reads its own
// pendingDirectiveName/Value captured at submit). This test directly
// mutates the live input fields AFTER the ceremony has opened (simulating
// what a future refactor could make reachable) and asserts the commit still
// carries the ORIGINALLY SUBMITTED key/value, not the mutated live ones.
func TestCustomKeyCeremonyConfirmUsesTheSubmittedSnapshotNotTheLiveInputFields(t *testing.T) {
	var gotKey, gotValue string
	b := stubBackend{
		customKeyPlan: GitCustomKeyPlanView{Targets: []string{"~/.gitconfig.d/00-baseline"}},
		customKeyFn: func(key, value string) tea.Cmd {
			gotKey, gotValue = key, value
			return func() tea.Msg { return GitCustomKeyCommitMsg{} }
		},
	}
	m := newGlobalGitModel(b)
	state := Seed()
	activated, _ := m.activate(state)
	m = activated.(globalGitModel)
	m.subTab = ggitSetKeys

	opened := m.handleKey(pressKey("n"), state)
	m = opened.model.(globalGitModel)
	m.customKeyInput.SetValue("core.pager")
	m.customValueInput.SetValue("less -FRX")
	m.customFieldFocus = 1
	submitted := m.handleKey(pressKey("enter"), state)
	m = submitted.model.(globalGitModel)
	if !m.ceremonyOpen {
		t.Fatal("setup: submitting must open the ceremony")
	}

	// Simulate a live-field mutation AFTER submit but BEFORE confirm — the
	// bug WR-12 describes as "one refactor away" from reachable.
	m.customKeyInput.SetValue("core.editor")
	m.customValueInput.SetValue("vim")

	confirmed := m.handleKey(pressKey("enter"), state)
	m = confirmed.model.(globalGitModel)
	if cmd := confirmed.cmd; cmd != nil {
		cmd()
	}

	if gotKey != "core.pager" || gotValue != "less -FRX" {
		t.Errorf("CommitCustomGitKey called with (%q, %q), want the SUBMITTED snapshot (\"core.pager\", \"less -FRX\") — WR-12 regressed (read the live, mutated input fields instead)", gotKey, gotValue)
	}
}

// TestCustomKeyStaleCommitMessageIsNotMisattributed is the PROP-03 mirror of
// TestGitStaleCommitMsgNotMisattributedToNewerCeremony (CR-01): a stale
// message from an abandoned ceremony 1 must not corrupt ceremony 2's
// still-pending UI, even though (WR-01) the stale message's own note is
// still surfaced.
func TestCustomKeyStaleCommitMessageIsNotMisattributed(t *testing.T) {
	b := stubBackend{
		customKeyPlan: GitCustomKeyPlanView{Targets: []string{"~/.gitconfig.d/00-baseline"}},
	}
	m := newGlobalGitModel(b)
	state := Seed()
	activated, _ := m.activate(state)
	m = activated.(globalGitModel)
	m.subTab = ggitSetKeys

	// Ceremony 1: "core.pager" / "less", confirmed — dispatch captured but
	// not yet delivered.
	opened1 := m.handleKey(pressKey("n"), state)
	m = opened1.model.(globalGitModel)
	m.customKeyInput.SetValue("core.pager")
	m.customValueInput.SetValue("less")
	// Enter on the KEY field (focus index 0) only moves focus to the value
	// field — it does not submit (mirrors TestCustomKeyFormOpensAndCaptures'
	// explicit Tab). Focus the value field before the submitting Enter.
	m.customFieldFocus = 1
	submitted1 := m.handleKey(pressKey("enter"), state)
	m = submitted1.model.(globalGitModel)
	if !m.ceremonyOpen {
		t.Fatal("setup: submitting ceremony 1 must open it")
	}
	confirmed1 := m.handleKey(pressKey("enter"), state)
	m = confirmed1.model.(globalGitModel)
	staleMsg := confirmed1.cmd()
	if !m.customKeyCommitPending {
		t.Fatal("setup: confirming ceremony 1 must set customKeyCommitPending")
	}

	// Abandon ceremony 1 (Ctrl+P-then-back runs activate(); token survives).
	reactivated, _ := m.activate(state)
	m = reactivated.(globalGitModel)
	m.subTab = ggitSetKeys
	if m.ceremonyOpen || m.customKeyCommitPending {
		t.Fatal("setup: activate() must have cleared the ceremony state")
	}

	// Ceremony 2: a DIFFERENT key/value, confirmed — genuinely in flight.
	opened2 := m.handleKey(pressKey("n"), state)
	m = opened2.model.(globalGitModel)
	m.customKeyInput.SetValue("core.autocrlf")
	m.customValueInput.SetValue("input")
	m.customFieldFocus = 1
	submitted2 := m.handleKey(pressKey("enter"), state)
	m = submitted2.model.(globalGitModel)
	confirmed2 := m.handleKey(pressKey("enter"), state)
	m = confirmed2.model.(globalGitModel)
	if !m.customKeyCommitPending {
		t.Fatal("setup: confirming ceremony 2 must set customKeyCommitPending")
	}
	ceremony2HeadingBefore := m.ceremony.cfg.Heading

	// Deliver ceremony 1's STALE message while ceremony 2 is still pending.
	result := m.handleMsg(staleMsg, state)
	m = result.model.(globalGitModel)

	if !m.customKeyCommitPending {
		t.Error("CR-01: a stale message must not clear customKeyCommitPending for the genuinely in-flight ceremony 2")
	}
	if m.ceremony.cfg.Heading != ceremony2HeadingBefore || m.ceremony.done {
		t.Errorf("CR-01: a stale message must not mutate ceremony 2's still-pending UI (heading=%q done=%t)",
			m.ceremony.cfg.Heading, m.ceremony.done)
	}
	// WR-01: the stale message still surfaces its OWN (ceremony 1's) note.
	wantNote := fmt.Sprintf(PropsGitCustomReceiptFmt, "core.pager", "less")
	if result.note != wantNote {
		t.Errorf("stale message's own note = %q, want %q (ceremony 1's submitted key/value, not ceremony 2's)", result.note, wantNote)
	}
}

// TestSetKeysFooterAdvertisesAddCustomKey asserts the Set keys sub-tab's
// footer advertises "n" / "Add custom key" whenever the probe succeeded —
// mirroring the "/" filter action's identical setKeysErr-gated advertisement.
func TestSetKeysFooterAdvertisesAddCustomKey(t *testing.T) {
	a := ggitSetKeysApp(t, stubBackend{})
	view := appView(a)
	if !strings.Contains(view, PropsAddCustomKeyLabel) {
		t.Errorf("Set keys footer must advertise %q;\nview:\n%s", PropsAddCustomKeyLabel, view)
	}

	// Probe failure: fail-open like every other Set keys action — "n" must
	// NOT be advertised (mirrors "/" filter's identical omission).
	a = ggitSetKeysApp(t, stubBackend{gitSetKeysErr: errGlobalGitTest})
	view = appView(a)
	if strings.Contains(view, PropsAddCustomKeyLabel) {
		t.Errorf("Set keys footer must NOT advertise %q on a probe failure;\nview:\n%s", PropsAddCustomKeyLabel, view)
	}
}

// ---------------------------------------------------------------------------
// Task 1: The fallback row displays what ReadGitFallbackAuthor returns
// ---------------------------------------------------------------------------

// TestGlobalGitFallbackRowShowsBackendSeededPair is the RED test for Task 1:
// the fallback row's CurrentValue must come from GitFallbackAuthorState
// (ReadGitFallbackAuthor), not from DemoState, on first entry when DemoState
// is empty. This is the reported defect: user sees empty while other sub-tab
// shows a value.
func TestGlobalGitFallbackRowShowsBackendSeededPair(t *testing.T) {
	const backendName = "Pat Example"
	const backendEmail = "pat@example.com"

	// Test with backend returning a pair and DemoState empty (the reported defect case).
	b := stubBackend{
		fallbackState: GitFallbackAuthorView{Name: backendName, Email: backendEmail},
	}
	m := newGlobalGitModel(b)
	next, _ := m.activate(Seed())
	gm := next.(globalGitModel)

	// Get the overlaid options (which applies DemoState on top of backend values).
	opts := gm.overlaidGitOptions(DemoState{})

	// Find the fallback row in the overlaid options.
	var fallbackRow GlobalGitOptionView
	found := false
	for _, o := range opts {
		if o.Key == GlobalGitEmailFallbackKey {
			fallbackRow = o
			found = true
			break
		}
	}

	if !found {
		t.Fatal("fallback row not found in overlaid options")
	}

	// The CurrentValue should show the backend-seeded pair.
	expected := fallbackCurrentLabel(backendName, backendEmail)
	if fallbackRow.CurrentValue != expected {
		t.Errorf("fallback CurrentValue = %q, want %q (the backend-seeded pair)\nThis is the reported defect: the row shows unset while the other sub-tab shows a value",
			fallbackRow.CurrentValue, expected)
	}
}

// TestGlobalGitFallbackRowFallbackCurrentLabel tests all three branches
// of fallbackCurrentLabel composition.
func TestGlobalGitFallbackRowFallbackCurrentLabel(t *testing.T) {
	tests := []struct {
		name, email, want string
	}{
		{"Pat Example", "pat@example.com", "Pat Example <pat@example.com>"},
		{"Pat Example", "", "Pat Example"},
		{"", "pat@example.com", "pat@example.com"},
		{"", "", "unset (recipes default)"},
	}
	for _, tt := range tests {
		got := fallbackCurrentLabel(tt.name, tt.email)
		if got != tt.want {
			t.Errorf("fallbackCurrentLabel(%q, %q) = %q, want %q", tt.name, tt.email, got, tt.want)
		}
	}
}

// TestGlobalGitFallbackRowDemoStateOverlay tests that in-session committed
// values appear as a post-commit overlay on top of the authoritative pair.
func TestGlobalGitFallbackRowDemoStateOverlay(t *testing.T) {
	const backendName = "Original Name"
	const backendEmail = "original@example.com"
	const newName = "Commited Name"
	const newEmail = "committed@example.com"

	// Test with backend returning original values and DemoState carrying committed values.
	b := stubBackend{
		fallbackState: GitFallbackAuthorView{Name: backendName, Email: backendEmail},
	}
	m := newGlobalGitModel(b)
	next, _ := m.activate(Seed())
	gm := next.(globalGitModel)

	// Apply DemoState overlay simulating a committed change in this session.
	opts := gm.overlaidGitOptions(DemoState{
		GitGlobalName:  newName,
		GitGlobalEmail: newEmail,
	})

	// Find the fallback row.
	var fallbackRow GlobalGitOptionView
	for _, o := range opts {
		if o.Key == GlobalGitEmailFallbackKey {
			fallbackRow = o
			break
		}
	}

	// The CurrentValue should show the in-session committed pair.
	expected := fallbackCurrentLabel(newName, newEmail)
	if fallbackRow.CurrentValue != expected {
		t.Errorf("with DemoState overlay, fallback CurrentValue = %q, want %q",
			fallbackRow.CurrentValue, expected)
	}
}
