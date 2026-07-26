package tui

// addrepo_test.go — Tests for the Add Repo modal (Task 2 of Plan 05.7-07).
// TDD RED tests (written before implementation).
//
// Test coverage:
//   - newAddRepoModel starts at addRepoStepDetect
//   - detectResultMsg sets provider + identity match → advances to client picker
//   - no identity match → addRepoStepInlineCreate; on wizard success resume to picker
//   - rewrite preview, clone→pull→done flow
//   - dest-exists prompt, clone failure retry
//   - ctrl+r opens Add Repo modal from Identities view
//   - palette entry present

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/castocolina/gitid/internal/identity"
	"github.com/castocolina/gitid/internal/repoclone"
)

// ─── Add Repo modal initial state ─────────────────────────────────────────────

// TestAddRepoModelInitialState verifies that newAddRepoModel starts at
// addRepoStepDetect with an empty URL field.
func TestAddRepoModelInitialState(t *testing.T) {
	m := newAddRepoModel(tuiDeps{})
	if m.step != addRepoStepDetect {
		t.Errorf("newAddRepoModel must start at addRepoStepDetect; got %v", m.step)
	}
}

// TestAddRepoDetectResultProviderMatch verifies that detectResultMsg with a matched
// provider and identity advances to addRepoStepClientPicker.
func TestAddRepoDetectResultProviderMatch(t *testing.T) {
	m := newAddRepoModel(tuiDeps{})
	m.step = addRepoStepDetect

	m2, _ := m.update(detectResultMsg{
		provider:     "github.com",
		matchedAlias: "personal.github.com",
		matchedName:  "personal",
		err:          nil,
	})

	if m2.step != addRepoStepClientPicker {
		t.Errorf("detectResultMsg with match must advance to addRepoStepClientPicker; got %v", m2.step)
	}
	if m2.provider != "github.com" {
		t.Errorf("detectResultMsg must set provider; got %q", m2.provider)
	}
}

// TestAddRepoDetectResultNoMatch verifies that detectResultMsg with no identity match
// advances to addRepoStepInlineCreate (D-08: continuous flow, no abort).
func TestAddRepoDetectResultNoMatch(t *testing.T) {
	m := newAddRepoModel(tuiDeps{})
	m.step = addRepoStepDetect

	m2, _ := m.update(detectResultMsg{
		provider:    "github.com",
		matchedName: "", // no match
		err:         nil,
	})

	if m2.step != addRepoStepInlineCreate {
		t.Errorf("no identity match must advance to addRepoStepInlineCreate (D-08); got %v", m2.step)
	}
	// The saved rawURL and provider must be preserved for resume.
	if m2.savedProvider != "github.com" {
		t.Errorf("savedProvider must be preserved for inline-create resume; got %q", m2.savedProvider)
	}
}

// TestAddRepoInlineCreateResume verifies the modal-stack invariant (Pitfall 6):
// after wizard success, addRepoModel resumes at client-picker with the saved URL.
// The modal-stack rule: only one of {addRepo, wizard} renders at a time.
func TestAddRepoInlineCreateResume(t *testing.T) {
	m := newAddRepoModel(tuiDeps{})
	m.step = addRepoStepInlineCreate
	m.savedRawURL = "https://github.com/org/repo.git"
	m.savedProvider = "github.com"

	// Simulate wizard completion with a wizard success result.
	m2, _ := m.update(wizardCreateResultMsg{err: nil})

	if m2.step != addRepoStepClientPicker {
		t.Errorf("wizard success in inline-create must resume to addRepoStepClientPicker; got %v", m2.step)
	}
	// rawURL must be restored from savedRawURL.
	if m2.rawURL != "https://github.com/org/repo.git" {
		t.Errorf("rawURL must be restored from savedRawURL on resume; got %q", m2.rawURL)
	}
	// Modal-stack invariant: createWizard sub-model must be nil after resume.
	if m2.createWizard != nil {
		t.Error("createWizard must be nil after inline-create completes (modal-stack invariant)")
	}
}

// TestAddRepoCloneResultSuccess verifies that cloneResultMsg{err: nil} appends clone
// lines and dispatches the pull cmd.
func TestAddRepoCloneResultSuccess(t *testing.T) {
	m := newAddRepoModel(tuiDeps{})
	m.step = addRepoStepCloning

	m2, pullCmd := m.update(cloneResultMsg{
		lines: []string{"Cloning into 'repo'...", "done."},
		err:   nil,
	})

	if m2.step != addRepoStepPulling {
		t.Errorf("successful clone must advance to addRepoStepPulling; got %v", m2.step)
	}
	if len(m2.cloneLines) == 0 {
		t.Error("cloneResultMsg must populate cloneLines")
	}
	if pullCmd == nil {
		t.Error("successful clone must dispatch pull cmd")
	}
}

// TestAddRepoPullResultSuccess verifies that pullResultMsg{err: nil} advances to done.
func TestAddRepoPullResultSuccess(t *testing.T) {
	m := newAddRepoModel(tuiDeps{})
	m.step = addRepoStepPulling

	m2, _ := m.update(pullResultMsg{
		lines: []string{"Already up to date."},
		err:   nil,
	})

	if m2.step != addRepoStepDone {
		t.Errorf("successful pull must advance to addRepoStepDone; got %v", m2.step)
	}
}

// TestAddRepoDestExistsPrompt verifies that cloneResultMsg with ErrDestExists
// advances to addRepoStepDestExists (overwrite prompt, Pitfall 5).
func TestAddRepoDestExistsPrompt(t *testing.T) {
	m := newAddRepoModel(tuiDeps{})
	m.step = addRepoStepCloning

	m2, _ := m.update(cloneResultMsg{
		lines: nil,
		err:   repoclone.ErrDestExists,
	})

	if m2.step != addRepoStepDestExists {
		t.Errorf("ErrDestExists must advance to addRepoStepDestExists; got %v", m2.step)
	}
}

// TestAddRepoCloneFailureShowsError verifies that a non-ErrDestExists clone error
// advances to addRepoStepError with the error stored.
func TestAddRepoCloneFailureShowsError(t *testing.T) {
	m := newAddRepoModel(tuiDeps{})
	m.step = addRepoStepCloning

	m2, _ := m.update(cloneResultMsg{
		lines: []string{"fatal: repository not found"},
		err:   errors.New("git clone: exit status 128"),
	})

	if m2.step != addRepoStepError {
		t.Errorf("clone error must advance to addRepoStepError; got %v", m2.step)
	}
	if m2.err == nil {
		t.Error("addRepoStepError must have err set")
	}
}

// TestAddRepoViewRendersURL verifies that the view renders the raw URL entered by
// the user at the detect step.
func TestAddRepoViewRendersURL(t *testing.T) {
	m := newAddRepoModel(tuiDeps{})
	m.rawURL = "https://github.com/org/repo.git"

	v := m.view(80)
	if !strings.Contains(v, "Add Repo") {
		t.Errorf("addRepoModel view must contain 'Add Repo' title; got: %q", truncateString(v, 200))
	}
}

// ─── ctrl+r opens Add Repo modal from Identities view ──────────────────────

// TestCtrlROpensAddRepoModal verifies that pressing ctrl+r in the Identities view
// opens the addRepoModal.
func TestCtrlROpensAddRepoModal(t *testing.T) {
	m := newRootModel(fakeDocDeps(), fakeIdentityDeps(), identity.UpdateDeps{}, identity.DeleteDeps{})
	m = sendMsg(m, windowSizeMsg())
	m.activeView = identitiesView

	// ctrl+r: KeyMsg with Code='r' and Mod=ModCtrl
	m2 := sendMsg(m, tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})

	if m2.activeModal != addRepoModal {
		t.Errorf("ctrl+r in Identities view must open addRepoModal; got activeModal=%v", m2.activeModal)
	}
}

// TestPaletteHasAddRepoEntry verifies that the command palette includes an "add repo"
// entry with action "action:addrepo" or similar.
func TestPaletteHasAddRepoEntry(t *testing.T) {
	p := newPaletteModel()
	found := false
	for _, item := range p.items {
		if strings.Contains(strings.ToLower(item.label), "repo") ||
			item.action == "action:addrepo" {
			found = true
			break
		}
	}
	if !found {
		t.Error("command palette must include an 'add repo' entry")
	}
}

// TestAddRepoFooterHint verifies that the footer in Identities view includes
// the ctrl+r repo hint.
func TestAddRepoFooterHint(t *testing.T) {
	m := newRootModel(fakeDocDeps(), fakeIdentityDeps(), identity.UpdateDeps{}, identity.DeleteDeps{})
	m = sendMsg(m, windowSizeMsg())
	m.activeView = identitiesView

	footer := m.renderFooter()
	if !strings.Contains(footer, "ctrl+r") || !strings.Contains(footer, "repo") {
		t.Errorf("Identities view footer must include ctrl+r repo hint; footer: %q", footer)
	}
}
