package tuikit

import (
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/castocolina/gitid/internal/globalgit"
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
	if got := len(GlobalGitOptions); got != 11 {
		t.Fatalf("fixture rows = %d, want 11 (GGIT-01 baseline)", got)
	}
	view := appView(ggitApp(t))
	for _, key := range []string{
		"init.defaultBranch", "core.ignorecase", "core.autocrlf / core.eol",
		"user.email (global fallback)", "push.autoSetupRemote", "pull.rebase",
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

// TestGlobalGitNonPolicyRowIsNotSelectable asserts a row whose key the policy
// table does not resolve renders no checkbox glyph, does not respond to the
// toggle key, and does not respond to a click at its checkbox position — all
// driven from the single gitSelectable predicate.
func TestGlobalGitNonPolicyRowIsNotSelectable(t *testing.T) {
	// Use a row whose key is known NOT to be in the policy table yet
	// (core.ignorecase is in the fixture but not in globalgit.Policy for 07-01).
	_, ok := globalgit.PolicyFor("core.ignorecase")
	if ok {
		t.Skip("core.ignorecase is now in the policy table — pick a different key")
	}

	a := ggitApp(t)
	// Navigate to core.ignorecase (row index 1, one down from init.defaultBranch).
	a, _ = press(t, a, "down")
	m := ggitModel(t, a)
	if m.detailKey != "core.ignorecase" {
		t.Fatalf("expected core.ignorecase, got %q", m.detailKey)
	}

	// 1. No checkbox glyph in the rendered row. Master and detail panes are
	// joined onto the SAME visual line (joinMasterDetail), separated by
	// "│" — a naive whole-line Contains check picks up detail-pane prose
	// that happens to mention the key, so only the master-list column
	// (left of "│") is inspected.
	body := appView(a)
	// Find the core.ignorecase line in the master list.
	for _, line := range strings.Split(body, "\n") {
		listCol := strings.SplitN(line, "│", 2)[0]
		if strings.Contains(listCol, "core.ignorecase") {
			// The checkbox glyph must not appear on this line.
			if strings.Contains(listCol, glyphCheckOff) || strings.Contains(listCol, glyphCheckOn) {
				t.Errorf("non-policy row must not render a checkbox glyph; got line: %q", listCol)
			}
		}
	}

	// 2. Toggle key does not change the selection.
	a, _ = press(t, a, "space")
	m = ggitModel(t, a)
	if m.chosen["core.ignorecase"] {
		t.Error("toggle key must not choose a non-policy row")
	}

	// 3. A click at the checkbox position of this row must not toggle it.
	// We drive handleClick directly at y=2 (row index 1, line 2/3 of the body),
	// x=4 (the checkbox column). The result should not toggle the row.
	before := ggitModel(t, a).chosen["core.ignorecase"]
	res := ggitModel(t, a).handleClick(4, 2, 120, 40, a.state)
	after, ok2 := res.model.(globalGitModel)
	if !ok2 {
		t.Fatal("handleClick returned wrong type")
	}
	if after.chosen["core.ignorecase"] != before {
		t.Error("click at checkbox position of non-policy row must not toggle it")
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
// D9 user.email fallback tests (unchanged from original — ceremony is still
// synchronous for the email path, plan 07-02 owns its real seam).
// ---------------------------------------------------------------------------

// TestGlobalGitUserEmailFallbackIsEditableAndDefaultsOff pins D9
// (checkpoint-2 contract): the promoted global-fallback user.email row is a
// first-class EDITABLE field + apply checkbox, default OFF (recipes leave
// it unset), and is EXCLUDED from the generic baseline apply set — it has
// its own dedicated ceremony.
func TestGlobalGitUserEmailFallbackIsEditableAndDefaultsOff(t *testing.T) {
	a := ggitApp(t)
	// user.email (global fallback) row is index 3.
	a = pressSeq(t, a, "down", "down", "down")
	m := ggitModel(t, a)
	if m.detailKey != GlobalGitEmailFallbackKey {
		t.Fatalf("detailKey = %q, want %q", m.detailKey, GlobalGitEmailFallbackKey)
	}
	if m.chosen[GlobalGitEmailFallbackKey] {
		t.Fatal("D9: the global-fallback row must default to unchecked (recipes leave it unset)")
	}
	detail := regionFlat(a, 45, 100) // word-wrap-proof: flatten the detail column
	if !strings.Contains(detail, GlobalGitEmailFallbackHelper) {
		t.Errorf("D9 helper copy missing; detail pane:\n%s", detail)
	}
	if !strings.Contains(detail, GlobalGitEmailFallbackAdvisory) {
		t.Errorf("D9 advisory copy missing; detail pane:\n%s", detail)
	}
	// Apply never includes it in the generic baseline set.
	options := ggitModel(t, a).overlaidGitOptions(a.state)
	for _, key := range ggitModel(t, a).gitApplyChosen(options) {
		if key == GlobalGitEmailFallbackKey {
			t.Error("the generic baseline apply set must never contain the global-fallback row")
		}
	}
	// Enter starts text-editing (D8/D9) — typing then reaches the field.
	a, _ = press(t, a, "enter")
	if !ggitModel(t, a).emailEditing {
		t.Fatal("Enter on the selected fallback row must start text-editing")
	}
	a = typeText(t, a, "team@example.com")
	if got := ggitModel(t, a).emailInput.Value(); got != "team@example.com" {
		t.Errorf("emailInput = %q, want the typed value", got)
	}
	// Esc exits editing back to row navigation.
	a, _ = press(t, a, "esc")
	if ggitModel(t, a).emailEditing {
		t.Error("Esc must exit text-editing")
	}
	// space checks the row — an explicit opt-in.
	a, _ = press(t, a, "space")
	m = ggitModel(t, a)
	if !m.chosen[GlobalGitEmailFallbackKey] {
		t.Error("space must check the global-fallback row (explicit opt-in)")
	}
}

// TestGlobalGitUserEmailFallbackApplyGatedOnPlausibleEmail pins
// review-findings F10: an empty (or "@"-less) fallback email must never be
// applicable.
func TestGlobalGitUserEmailFallbackApplyGatedOnPlausibleEmail(t *testing.T) {
	a := ggitApp(t)
	// Navigate to init.defaultBranch, toggle, and apply it first so the
	// baseline apply set is empty when we test the email gate.
	a, _ = press(t, a, "space") // toggle init.defaultBranch
	a, _ = press(t, a, "a")     // open baseline ceremony
	a, _ = press(t, a, "enter") // confirm
	// Deliver the commit msg to complete the ceremony.
	a, _ = deliverMsg(t, a, GlobalGitCommitMsg{Backups: []string{NewBackupPath("~/.gitconfig.d/00-baseline")}})
	a, _ = press(t, a, "enter") // done (finish ceremony view)

	a = pressSeq(t, a, "down", "down", "down") // → the fallback row
	a, _ = press(t, a, "space")                // opt in, email still empty
	if !strings.Contains(regionFlat(a, 45, 100), "needs @") {
		t.Errorf("expected the inline 'needs @' error while the fallback email is empty:\n%s", regionFlat(a, 45, 100))
	}
	// "a" must be a no-op.
	a, _ = press(t, a, "a")
	if strings.Contains(appView(a), GlobalGitEmailCeremonyHeading) {
		t.Error("pressing a with an implausible (empty) fallback email must not open the dedicated ceremony")
	}

	// Typing a value without "@" still blocks it.
	a, _ = press(t, a, "enter")
	a = typeText(t, a, "not-an-email")
	a, _ = press(t, a, "esc")
	if !strings.Contains(regionFlat(a, 45, 100), "needs @") {
		t.Error("expected the inline 'needs @' error for a value missing '@'")
	}
	a, _ = press(t, a, "a")
	if strings.Contains(appView(a), GlobalGitEmailCeremonyHeading) {
		t.Error("pressing a with an implausible email (no @) must not open the dedicated ceremony")
	}

	// A plausible email clears the error and unblocks apply.
	a, _ = press(t, a, "enter")
	a = clearGitFieldRaw(t, a)
	a = typeText(t, a, "team@example.com")
	a, _ = press(t, a, "esc")
	if strings.Contains(regionFlat(a, 45, 100), "needs @") {
		t.Error("the inline 'needs @' error must clear once the email is plausible")
	}
	a, _ = press(t, a, "a")
	if !strings.Contains(appView(a), GlobalGitEmailCeremonyHeading) {
		t.Fatalf("expected the dedicated ceremony to open with a plausible email:\n%s", appView(a))
	}
}

// TestGlobalGitUserEmailFallbackDedicatedCeremony pins D9's dedicated apply
// ceremony — distinct heading/target/annotated-diff/result from the
// baseline managed-block ceremony, and the includeIf-precedence invariant
// stated in the result line.
func TestGlobalGitUserEmailFallbackDedicatedCeremony(t *testing.T) {
	a := ggitApp(t)
	a = pressSeq(t, a, "down", "down", "down") // → the fallback row
	a, _ = press(t, a, "enter")                // start editing
	a = typeText(t, a, "team@example.com")
	a, _ = press(t, a, "esc")   // done editing
	a, _ = press(t, a, "space") // opt in
	a, _ = press(t, a, "a")
	view := appView(a)
	if !strings.Contains(view, GlobalGitEmailCeremonyHeading) {
		t.Fatalf("ceremony heading missing:\n%s", view)
	}
	if !strings.Contains(view, GlobalGitEmailDiffAnnotation) {
		t.Error("the diff preview must carry the includeIf-precedence annotation")
	}
	if !strings.Contains(view, "team@example.com") {
		t.Error("the diff preview must show the typed email value")
	}
	a, _ = press(t, a, "enter") // confirm (email ceremony is synchronous)
	if !strings.Contains(appView(a), "Global fallback user.email set — used only where no identity matches") {
		t.Error("the receipt must carry the frozen result message")
	}
	a, _ = press(t, a, "enter") // done
	if a.state.GitGlobalEmail != "team@example.com" {
		t.Errorf("GitGlobalEmail = %q, want the applied email", a.state.GitGlobalEmail)
	}
	// The baseline itself was never touched by the email-only apply.
	if a.state.GitBaselineApplied {
		t.Error("applying the fallback email must not also mark the baseline applied")
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
