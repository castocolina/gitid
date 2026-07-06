package dummytui

// reviewfix_test.go pins the combined review-findings fix pass for plan
// 02-15 (a fresh agent-ui-ux-designer parity critique + a fresh-context code
// review, both run before the 02-12 human re-presentation). This file covers
// F3 (code review 2): TUI click hijacks from whole-line substring matching —
// hitFieldRow previously used a bare strings.Contains, which let (a) a
// button row's disabled-suffix prose ("...needs user.name...") shadow the
// user.name field, (b) a disabled algorithm row ("ed25519-sk") shadow an
// enabled one ("ed25519") sharing its prefix, and (c) a bordered
// PreviewBlock/config-preview line ("Port 443", "id_ed25519_acme") shadow an
// unrelated field. anchoredLabelMatch (identities.go) fixes all three.

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// clearGitFieldRaw backspaces the wizard Git step's currently-focused text
// field enough times to empty even the longest seeded default ("Acme
// Identity", 13 chars).
func clearGitFieldRaw(t *testing.T, a App) App {
	t.Helper()
	for i := 0; i < 20; i++ {
		model, _ := a.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
		a = model.(App)
	}
	return a
}

// TestMouseWizardGitStepButtonRowIgnoresDisabledSuffixFieldText is F3(a):
// with the Git form invalid, [ Continue ]'s own row carries the
// disabled-suffix prose "— needs user.name + a valid email" — a bare
// strings.Contains match against the user.name FIELD's label used to
// hijack a click meant for Back/Skip, silently focusing user.name instead
// of activating the button.
func TestMouseWizardGitStepButtonRowIgnoresDisabledSuffixFieldText(t *testing.T) {
	a := wizardThroughTest(t, identitiesApp()) // step 2, user.name field focused, valid by default
	a = clearGitFieldRaw(t, a)                 // empties user.name — Continue is now disabled
	if identModel(t, a).wizard.git.valid() {
		t.Fatal("setup: expected the git form to be invalid after clearing user.name")
	}
	if !strings.Contains(paneFlat(a), "needs user.name") {
		t.Fatalf("setup: expected the disabled-suffix prose on screen:\n%s", paneFlat(a))
	}

	// Clicking Back must actually go back a step — not silently focus
	// user.name (whose row is untouched, so the step would stay put).
	back := clickCell(t, a, "Back (Esc)", 0, frameBodyTop)
	if got := identModel(t, back).wizard.step; got != 1 {
		t.Fatalf("step = %d after clicking Back with Continue disabled, want 1 (test step)", got)
	}

	// Clicking Skip must skip Git and advance to the review step — not
	// silently focus user.name either.
	skip := clickCell(t, a, "[ Skip Git ]", 0, frameBodyTop)
	m := identModel(t, skip)
	if m.wizard.step != 3 || m.wizard.configureGit {
		t.Fatalf("step = %d configureGit = %v after clicking Skip with Continue disabled, want 3/false",
			m.wizard.step, m.wizard.configureGit)
	}
}

// TestMouseWizardDisabledAlgorithmRowNeverResurrectsAPriorSelection is
// F3(b): "ed25519-sk" (disabled) textually contains the enabled "ed25519" —
// clicking the disabled row must never move the selection, even when the
// PRIOR selection is non-zero (the earlier regression test could not catch
// a bug that always falsely reset the index back to 0, because its own
// `before` value already was 0).
func TestMouseWizardDisabledAlgorithmRowNeverResurrectsAPriorSelection(t *testing.T) {
	a := pressSeq(t, identitiesApp(), "n") // step 0
	a = clickCell(t, a, "rsa-4096", 0, frameBodyTop)
	before := identModel(t, a).wizard.algoIdx
	if before == 0 {
		t.Fatal("setup: expected rsa-4096 to be a non-zero algorithm index")
	}
	a = clickCell(t, a, "ed25519-sk", 0, frameBodyTop)
	if got := identModel(t, a).wizard.algoIdx; got != before {
		t.Errorf("algoIdx = %d after clicking the disabled ed25519-sk row, want unchanged (%d)", got, before)
	}
}

// TestMouseWizardPreviewLineClickIsInert is F3(c): the Live Host-block
// preview renders "Port 443" inside a bordered, read-only PreviewBlock —
// clicking it must never be mistaken for the actual Port field row.
func TestMouseWizardPreviewLineClickIsInert(t *testing.T) {
	a := pressSeq(t, identitiesApp(), "n") // step 0, Alias prefix focused
	before := identModel(t, a).wizard.focus
	if !strings.Contains(paneFlat(a), "Port 443") {
		t.Fatalf("setup: expected the Live Host-block preview on screen:\n%s", paneFlat(a))
	}
	a = clickCell(t, a, "Port 443", 0, frameBodyTop)
	if got := identModel(t, a).wizard.focus; got != before {
		t.Errorf("focus = %d after clicking the preview's Port line, want unchanged (%d)", got, before)
	}
}
