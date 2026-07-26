package tui

// adopt_test.go — Tests for the Adopt modal and sidebar fragment discriminator.
// Task 1 of Plan 05.7-07: TDD RED tests (written before implementation).
//
// Test coverage:
//   - buildUnmanaged populates kindFragment rows from deps.adopt.ListCandidates
//   - kindOrphanKey rows are NOT affected
//   - [Adopt] affordance shows on kindFragment rows, NOT on kindOrphanKey
//   - adoptModel state machine: method toggle, cmd dispatch, success, error
//   - Offer-remove step (migrate only): N-default never calls remove seam

import (
	"errors"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/castocolina/gitid/internal/adopter"
	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
)

// fakeAdoptDeps returns an adopter.Deps with controllable stubs for unit testing.
func fakeAdoptDeps(candidates []string) adopter.Deps {
	return adopter.Deps{
		ReadFile:        func(_ string) ([]byte, error) { return []byte{}, nil },
		WriteFile:       func(_ string, _ []byte, _ os.FileMode) (string, error) { return "", nil },
		CopyFile:        func(_, _ string) error { return nil },
		BackupAndRemove: func(_ string) (string, error) { return "", nil },
		WriteIncludeIf:  func(_, _ string, _ []gitconfig.Match) (string, error) { return "", nil },
		ReadFragment:    func(_ string) (gitconfig.FragmentInfo, error) { return gitconfig.FragmentInfo{}, nil },
		ListCandidates:  func(_ string) ([]string, error) { return candidates, nil },
	}
}

// fakeAdoptDepsFull returns a complete adopter.Deps for the adopt cmd tests.
// If removeCalled is non-nil, BackupAndRemove sets it to true when called.
func fakeAdoptDepsFull(removeCalled *bool) adopter.Deps {
	return adopter.Deps{
		ReadFile:  func(_ string) ([]byte, error) { return []byte{}, nil },
		WriteFile: func(_ string, _ []byte, _ os.FileMode) (string, error) { return "", nil },
		CopyFile:  func(_, _ string) error { return nil },
		BackupAndRemove: func(_ string) (string, error) {
			if removeCalled != nil {
				*removeCalled = true
			}
			return "/backup.bak", nil
		},
		WriteIncludeIf: func(_, _ string, _ []gitconfig.Match) (string, error) { return "", nil },
		ReadFragment:   func(_ string) (gitconfig.FragmentInfo, error) { return gitconfig.FragmentInfo{}, nil },
		ListCandidates: func(_ string) ([]string, error) { return nil, nil },
	}
}

// makeFakeAdoptTUIDeps builds a tuiDeps with a ListCandidates seam returning candidates.
func makeFakeAdoptTUIDeps(candidates []string) tuiDeps {
	return tuiDeps{
		doctor: doctor.Deps{},
		adopt:  fakeAdoptDeps(candidates),
	}
}

// ─── Sidebar fragment discriminator tests (Task 1 critical ordering) ─────────

// TestBuildUnmanagedFragmentRows verifies that buildUnmanaged populates kindFragment
// rows from deps.adopt.ListCandidates BEFORE any [Adopt] dispatch can act on them.
// Requirement: ADOPT-01 (sidebar fragment discriminator — first step).
func TestBuildUnmanagedFragmentRows(t *testing.T) {
	// The ListCandidates seam returns one fragment path.
	deps := makeFakeAdoptTUIDeps([]string{"/home/user/.gitconfig_work"})

	// Fake out the doctor side so buildUnmanaged doesn't need real key files.
	deps.doctor.AllSSHHostIdentityFiles = nil
	deps.doctor.KeyPaths = nil

	result := buildUnmanaged(deps)

	// Expect one kindFragment row.
	if len(result) == 0 {
		t.Fatal("buildUnmanaged must return at least one entry when ListCandidates returns fragments")
	}
	found := false
	for _, e := range result {
		if e.kind == kindFragment {
			found = true
			if e.fragmentPath == "" {
				t.Error("kindFragment entry must have fragmentPath set")
			}
			if e.shortName == "" {
				t.Error("kindFragment entry must have shortName set")
			}
		}
	}
	if !found {
		t.Error("buildUnmanaged must produce at least one kindFragment entry from ListCandidates")
	}
}

// TestBuildUnmanagedOrphanKeysKeepKindOrphanKey verifies that existing orphan-key
// rows from AllSSHHostIdentityFiles keep kind == kindOrphanKey after the fragment
// discriminator is added (regression guard — fragment addition must not change
// existing orphan-key logic).
func TestBuildUnmanagedOrphanKeysKeepKindOrphanKey(t *testing.T) {
	deps := tuiDeps{
		doctor: doctor.Deps{
			AllSSHHostIdentityFiles: []string{"/home/user/.ssh/gitid_work"},
			KeyPaths:                []string{"/home/user/.ssh/orphan_key"},
		},
		adopt: adopter.Deps{
			// No candidates — ListCandidates returns nil (no fragments).
			ListCandidates: func(_ string) ([]string, error) { return nil, nil },
		},
	}

	result := buildUnmanaged(deps)

	foundOrphan := false
	for _, e := range result {
		if e.kind == kindOrphanKey {
			foundOrphan = true
		}
		if e.kind == kindFragment {
			t.Error("no fragment candidates were returned; no kindFragment entries expected")
		}
	}
	if !foundOrphan {
		t.Error("buildUnmanaged must still produce kindOrphanKey rows for unreferenced key files")
	}
}

// TestBuildUnmanagedNilListCandidatesGuard verifies that a nil ListCandidates seam
// (test mode) does not panic — buildUnmanaged skips the fragment loop gracefully.
func TestBuildUnmanagedNilListCandidatesGuard(t *testing.T) {
	deps := tuiDeps{
		doctor: doctor.Deps{
			AllSSHHostIdentityFiles: []string{"/home/user/.ssh/known_key"},
			KeyPaths:                nil, // no orphans
		},
		adopt: adopter.Deps{
			// nil ListCandidates — guard must not panic.
			ListCandidates: nil,
		},
	}

	// Must not panic.
	result := buildUnmanaged(deps)
	// No fragments expected.
	for _, e := range result {
		if e.kind == kindFragment {
			t.Error("nil ListCandidates must yield no kindFragment entries")
		}
	}
}

// ─── Adopt affordance display tests ──────────────────────────────────────────

// TestSidebarFragmentRowShowsAdoptAffordance verifies that a focused kindFragment
// unmanaged row renders [Adopt] (StyleAccent) instead of the static ~ glyph.
func TestSidebarFragmentRowShowsAdoptAffordance(t *testing.T) {
	sb := newSidebarModel(doctor.Deps{})
	sb.unmanaged = []unmanagedEntry{
		{shortName: "gitconfig_work", kind: kindFragment, fragmentPath: "/home/user/.gitconfig_work"},
	}
	sb.selectedUnmanaged = 0

	rendered := sb.view(30, 20, true)

	if !strings.Contains(rendered, "[Adopt]") {
		t.Errorf("focused kindFragment row must render [Adopt] affordance; sidebar: %q", rendered)
	}
}

// TestSidebarOrphanKeyRowNoAdoptAffordance verifies that a kindOrphanKey row does
// NOT show [Adopt] — the adopt affordance is fragment-only (D-06).
func TestSidebarOrphanKeyRowNoAdoptAffordance(t *testing.T) {
	sb := newSidebarModel(doctor.Deps{})
	sb.unmanaged = []unmanagedEntry{
		{shortName: "orphan_key", kind: kindOrphanKey, keyPath: "/home/user/.ssh/orphan_key"},
	}
	sb.selectedUnmanaged = 0

	rendered := sb.view(30, 20, true)

	if strings.Contains(rendered, "[Adopt]") {
		t.Error("kindOrphanKey row must NOT render [Adopt] affordance")
	}
}

// ─── adoptModel state machine tests ──────────────────────────────────────────

// TestAdoptModelDefaultsToMigrate verifies that newAdoptModel initializes with
// method == AdoptMigrate (D-04 decision).
func TestAdoptModelDefaultsToMigrate(t *testing.T) {
	m := newAdoptModel("/home/user/.gitconfig_work", "work", nil, tuiDeps{})
	if m.method != adopter.AdoptMigrate {
		t.Errorf("newAdoptModel must default to AdoptMigrate (D-04); got %v", m.method)
	}
	if m.phase != adoptPhaseConfirm {
		t.Errorf("newAdoptModel must start at adoptPhaseConfirm; got %v", m.phase)
	}
}

// TestAdoptModelMethodToggle verifies that pressing 'm' sets migrate and 'r' sets
// reference-in-place.
func TestAdoptModelMethodToggle(t *testing.T) {
	m := newAdoptModel("/home/user/.gitconfig_work", "work", nil, tuiDeps{})

	// Press 'r' → reference in place.
	m2, _ := m.update(tea.KeyPressMsg{Code: 'r'})
	if m2.method != adopter.AdoptReferenceInPlace {
		t.Errorf("pressing 'r' must set AdoptReferenceInPlace; got %v", m2.method)
	}

	// Press 'm' → back to migrate.
	m3, _ := m2.update(tea.KeyPressMsg{Code: 'm'})
	if m3.method != adopter.AdoptMigrate {
		t.Errorf("pressing 'm' must set AdoptMigrate; got %v", m3.method)
	}
}

// TestAdoptModelMethodRadios verifies that view() renders [x]/[ ] radio buttons
// reflecting the current method selection.
func TestAdoptModelMethodRadios(t *testing.T) {
	m := newAdoptModel("/home/user/.gitconfig_work", "work", nil, tuiDeps{})
	// Default: migrate selected.
	v := m.view(80)
	if !strings.Contains(v, "[x]") {
		t.Error("adoptModel view must render [x] radio for the selected method")
	}
	if !strings.Contains(v, "[ ]") {
		t.Error("adoptModel view must render [ ] radio for the unselected method")
	}
}

// TestAdoptModelEnterDispatchesAdoptCmd verifies that pressing Enter on the confirm
// phase dispatches a cmd (runAdoptCmd).
func TestAdoptModelEnterDispatchesAdoptCmd(t *testing.T) {
	deps := tuiDeps{adopt: fakeAdoptDeps(nil)}
	m := newAdoptModel("/home/user/.gitconfig_work", "work", nil, deps)

	_, cmd := m.update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Error("pressing Enter on adoptPhaseConfirm must dispatch runAdoptCmd (non-nil cmd)")
	}
}

// TestAdoptModelResultMsgSuccess verifies that adoptResultMsg{err: nil} transitions
// the model to adoptPhaseDone.
func TestAdoptModelResultMsgSuccess(t *testing.T) {
	m := newAdoptModel("/home/user/.gitconfig_work", "work", nil, tuiDeps{})
	m.phase = adoptPhaseRunning

	m2, _ := m.update(adoptResultMsg{result: adopter.AdoptResult{MigratedPath: "/home/user/.gitconfig.d/work"}, err: nil})
	if m2.phase != adoptPhaseDone {
		t.Errorf("adoptResultMsg with nil error must set adoptPhaseDone; got %v", m2.phase)
	}
}

// TestAdoptModelResultMsgError verifies that adoptResultMsg{err: <non-nil>} transitions
// the model to adoptPhaseError with the error text set.
func TestAdoptModelResultMsgError(t *testing.T) {
	m := newAdoptModel("/home/user/.gitconfig_work", "work", nil, tuiDeps{})
	m.phase = adoptPhaseRunning

	testErr := errors.New("copy failed: permission denied")
	m2, _ := m.update(adoptResultMsg{err: testErr})
	if m2.phase != adoptPhaseError {
		t.Errorf("adoptResultMsg with error must set adoptPhaseError; got %v", m2.phase)
	}
	if m2.errText == "" {
		t.Error("adoptPhaseError must set errText from the error message")
	}
}

// TestAdoptModelEscCancels verifies that Esc emits adoptCancelMsg (closes the modal).
func TestAdoptModelEscCancels(t *testing.T) {
	m := newAdoptModel("/home/user/.gitconfig_work", "work", nil, tuiDeps{})
	_, cmd := m.update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Error("Esc on adoptModel must dispatch a cmd (adoptCancelMsg)")
		return
	}
	msg := cmd()
	if _, ok := msg.(adoptCancelMsg); !ok {
		t.Errorf("Esc on adoptModel must emit adoptCancelMsg; got %T", msg)
	}
}

// TestAdoptModelOfferRemoveNDefault verifies the offer-remove step (migrate only):
// the default answer is N, and pressing Enter with N does NOT call the remove seam.
func TestAdoptModelOfferRemoveNDefault(t *testing.T) {
	removeCalled := false
	deps := tuiDeps{adopt: fakeAdoptDepsFull(&removeCalled)}
	m := newAdoptModel("/home/user/.gitconfig_work", "work", nil, deps)
	// Advance to offer-remove step.
	m.phase = adoptPhaseOfferRemove
	m.removeChoice = false // default: no remove

	// Pressing Enter with default N must close the modal, NOT call the remove seam.
	_, cmd := m.update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if removeCalled {
		t.Error("pressing Enter with default N (removeChoice=false) must NOT call BackupAndRemove seam (D-05)")
	}
	if cmd == nil {
		t.Error("pressing Enter at adoptPhaseOfferRemove must dispatch a cmd (clearModalCmd or toast)")
	}
}

// TestAdoptModelViewRendersSourcePath verifies that adoptModel.view() shows the
// fragment source path in faint text.
func TestAdoptModelViewRendersSourcePath(t *testing.T) {
	m := newAdoptModel("/home/user/.gitconfig_work", "work", nil, tuiDeps{})
	v := m.view(80)
	if !strings.Contains(v, ".gitconfig_work") {
		t.Errorf("adoptModel view must show the fragment source path; got: %q", truncateString(v, 300))
	}
}

// ─── Sidebar dispatch: A key on fragment row opens adopt modal ────────────────

// TestSidebarAKeyOpensAdoptModal verifies that pressing 'A' on a focused
// kindFragment unmanaged entry opens the adopt modal.
func TestSidebarAKeyOpensAdoptModal(t *testing.T) {
	m := newRootModel(fakeDocDeps(), fakeIdentityDeps(), identity.UpdateDeps{}, identity.DeleteDeps{})
	m = sendMsg(m, windowSizeMsg())
	m.activeView = identitiesView
	m.sidebar.accounts = nil
	m.sidebar.selected = -1
	m.sidebar.unmanaged = []unmanagedEntry{
		{shortName: "gitconfig_work", kind: kindFragment, fragmentPath: "/home/user/.gitconfig_work"},
	}
	m.sidebar.selectedUnmanaged = 0

	// 'A' on a fragment row must open adoptModal.
	m2 := sendKey(m, "A")
	if m2.activeModal != adoptModal {
		t.Errorf("'A' on kindFragment row must open adoptModal; got activeModal=%v", m2.activeModal)
	}
}

// windowSizeMsg returns a standard 120×40 WindowSizeMsg for tests.
func windowSizeMsg() tea.WindowSizeMsg {
	return tea.WindowSizeMsg{Width: 120, Height: 40}
}
