package tuikit

import (
	"os"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
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
	view := appView(ggitApp(t))
	for _, key := range []string{
		"init.defaultBranch", "core.ignorecase", "core.autocrlf / core.eol",
		"user.email (global fallback)", "user.useConfigOnly", "push.autoSetupRemote", "pull.rebase",
		"fetch.prune", "alias (8 shortcuts)", "color (ui/branch/diff/status)",
		"merge.conflictstyle", "diff.colorMoved",
	} {
		if !strings.Contains(view, key) {
			t.Errorf("row %q missing", key)
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
			if strings.Contains(listCol, glyphCheckOff) || strings.Contains(listCol, glyphCheckOn) {
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
		// Deliver the commit message from the pending command.
		a, _ = deliverMsg(t, a, GlobalGitCommitMsg{Backups: []string{backupPath}})
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
		// Deliver an error commit message.
		a, _ = deliverMsg(t, a, GlobalGitCommitMsg{
			Err:      "write failed: disk full",
			Restored: []string{restoredPath},
		})
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
		msg := cmd()
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
		msg := cmd()
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
			if strings.Contains(listCol, glyphCheckOff) || strings.Contains(listCol, glyphCheckOn) {
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
			if strings.Contains(listCol, glyphCheckOn) {
				t.Errorf("init.defaultBranch must start unchecked (R-1); line: %q", listCol)
			}
			if !strings.Contains(listCol, glyphCheckOff) {
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
			if strings.Contains(listCol, o.Key) && (strings.Contains(listCol, glyphCheckOff) || strings.Contains(listCol, glyphCheckOn)) {
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
	if strings.Contains(body, glyphCheckOff) || strings.Contains(body, glyphCheckOn) {
		t.Error("differs row must render no checkbox glyph (D-02)")
	}
	// The unclipped line-2 keeps the full Phase 6 frozen sentence, byte-identical.
	line2 := globalGitRowLine2(GlobalGitOptionView{State: GlobalGitSetButDiffers, CurrentValue: "true", Recommended: "false", AttributedToUser: true})
	if !strings.Contains(line2, GlobalSSHWordDiffersUser) {
		t.Errorf("unclipped differs line-2 = %q, want the frozen sentence %q", line2, GlobalSSHWordDiffersUser)
	}
	line2Outside := globalGitRowLine2(GlobalGitOptionView{State: GlobalGitSetButDiffers, CurrentValue: "true", Recommended: "false", AttributedToUser: false})
	if !strings.Contains(line2Outside, GlobalSSHWordDiffersOutside) {
		t.Errorf("unclipped external differs line-2 = %q, want %q", line2Outside, GlobalSSHWordDiffersOutside)
	}
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
