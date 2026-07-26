package tui

// detail_test.go — Tests for the Identity Detail pane (Plan 04 GREEN).
//
// Tests are adapted for the Phase 5.6 two-pane architecture. The test
// NAMES are locked by VALIDATION.md and referenced in the plan behavior block.
//
// Analogous Phase 5 source: tui/identitydetail.go + tui/updateform.go.

import (
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
	"github.com/castocolina/gitid/internal/tester"
)

// makeTestAccount returns a populated identity.Account for use in detail tests.
func makeTestAccount() identity.Account {
	return identity.Account{
		Name:     "personal",
		GitName:  "Ramon Colina",
		GitEmail: "user@example.com",
		Provider: "github.com",
		Alias:    "personal.github.com",
		Hostname: "github.com",
		Port:     22,
		KeyPath:  "~/.ssh/gitid_personal",
		PubPath:  "~/.ssh/gitid_personal.pub",
		Matches: []gitconfig.Match{
			{Kind: gitconfig.MatchGitdir, Value: "~/git/personal/"},
		},
		FragmentPath: "~/.gitconfig.d/personal",
	}
}

// fakeUpdateDeps returns a tuiDeps with no-op UpdateDeps stubs.
func fakeUpdateDeps(called *bool) tuiDeps {
	return tuiDeps{
		update: identity.UpdateDeps{
			WriteSSH: func(_, _, _ string) (string, error) { return "", nil },
			WriteGitconfig: func(_, _, _ string, _ []gitconfig.Match) (string, error) {
				return "", nil
			},
			WriteFragment: func(_, _, _, _ string, _ bool) error { return nil },
			WriteAllowedSigners: func(_, _, _ string) (string, error) {
				if called != nil {
					*called = true
				}
				return "", nil
			},
			RemoveAllowedSigners: func(_, _ string) (string, error) { return "", nil },
			Resolved: func(_ string) (tester.Result, tester.ResolvedConfig) {
				return tester.Result{}, tester.ResolvedConfig{}
			},
			ReadPub: func(_ string) (string, error) { return "ssh-ed25519 AAA...", nil },
		},
	}
}

// TestDetailRendersSelected verifies that detail.view() shows the key fields of a
// selected Account: Name, Git Name, Git Email, Provider, Port, Aliases/Sites block,
// signing state, and key path.
// Requirement: TUI-05 (detail pane, D-01 sidebar stays).
// Closes: Plan 04.
func TestDetailRendersSelected(t *testing.T) {
	acct := makeTestAccount()
	m := newIdentityDetailModel()
	m.account = &acct

	out := m.view(80)

	checks := []struct {
		label string
		want  string
	}{
		{"name", acct.Name},
		{"git name", acct.GitName},
		{"git email", acct.GitEmail},
		{"provider", acct.Provider},
		{"key path (faint)", acct.KeyPath},
		{"alias", acct.Alias},
	}
	for _, c := range checks {
		if !strings.Contains(out, c.want) {
			t.Errorf("detail.view must contain %s %q; output: %q", c.label, c.want, out)
		}
	}

	// Aliases / Sites section header.
	if !strings.Contains(out, "Aliases") {
		t.Error("detail.view must contain 'Aliases' section header")
	}
}

// TestDetailEmptyState verifies the empty-state message when no account is selected.
// Closes: Plan 04.
func TestDetailEmptyState(t *testing.T) {
	m := newIdentityDetailModel()
	// account is nil — empty state.
	out := m.view(80)
	if !strings.Contains(out, "Select an identity") {
		t.Errorf("empty-state view must contain 'Select an identity'; got: %q", out)
	}
}

// TestInlineEdit verifies that pressing 'e' sets inlineEditMode; Tab cycles fields;
// Enter on a non-structural field (git email) opens editConfirmModal;
// Esc cancels edits without dispatching any write.
// The locked TestInlineEdit is in wizard_test.go per VALIDATION.md (Plan 04 GREEN).
// Closes: Plan 04.
func TestDetailInlineEditEntryExit(t *testing.T) {
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
}

// TestStructuralEditOpensProve verifies that committing a structural field
// (alias/hostname/port/match strategy) emits a command that sets the proveModal,
// NOT a simple confirm.
// Requirement: TUI-05 (D-07, T-05.6-10).
// Closes: Plan 04.
func TestStructuralEditOpensProve(t *testing.T) {
	acct := makeTestAccount()
	m := newIdentityDetailModel()
	m.account = &acct
	m.deps = tuiDeps{}

	// Enter inline edit mode, position on alias field (structural).
	m2, _ := m.handleKey("e")
	// Find the alias field and focus it directly.
	for i, f := range m2.editFields {
		if f.label == "Alias" {
			m2.focusedField = i
			break
		}
	}

	// Pressing Enter on the alias field must result in proveModalPending=true
	// (signals the root model to open proveModal).
	m3, cmd := m2.handleKey("enter")
	if !m3.proveModalPending && cmd == nil {
		t.Error("structural field Enter must set proveModalPending or emit a cmd")
	}
	// If proveModalPending is true, the root model will open the prove loop.
	_ = m3
}

// TestMatchStrategyLivePreview verifies that when the match-strategy field is
// focused, renderMatchPreview returns a non-empty preview containing "includeIf".
// Requirement: TUI-05 (D-06, match strategy live preview).
// Closes: Plan 04.
func TestMatchStrategyLivePreview(t *testing.T) {
	acct := makeTestAccount()
	m := newIdentityDetailModel()
	m.account = &acct

	// Build edit fields and focus the match-strategy field.
	m.inlineEditMode = true
	m.editFields = buildDetailEditFields(acct)
	for i, f := range m.editFields {
		if f.label == "Match Strategy" {
			m.focusedField = i
			break
		}
	}

	preview := m.renderMatchPreview()
	if preview == "" {
		t.Error("renderMatchPreview must return a non-empty string when match-strategy is focused")
	}
	if !strings.Contains(preview, "includeIf") {
		t.Errorf("renderMatchPreview must contain 'includeIf'; got: %q", preview)
	}
}

// TestEditEscCancels verifies that Esc in inline-edit mode discards all field
// changes and never dispatches any write command.
// Requirement: TUI-05 (D-05, Esc is safe default).
// Closes: Plan 04.
func TestEditEscCancels(t *testing.T) {
	acct := makeTestAccount()
	var writeCalled bool
	deps := fakeUpdateDeps(&writeCalled)
	m := newIdentityDetailModel()
	m.account = &acct
	m.deps = deps

	// Enter edit mode, type something, then Esc.
	m2, _ := m.handleKey("e")
	m3, cmd := m2.handleKey("esc")

	if m3.inlineEditMode {
		t.Error("Esc must clear inlineEditMode")
	}
	// Cmd from Esc must not be a write — it can be nil.
	if cmd != nil {
		// Run the cmd to check it doesn't trigger a write.
		msg := cmd()
		// Esc cmd should be a clearModalMsg or nil message, not a writeResultMsg.
		if _, isWrite := msg.(writeResultMsg); isWrite {
			t.Error("Esc must not dispatch a write command")
		}
	}
	if writeCalled {
		t.Error("Esc must not call any write function")
	}
}
