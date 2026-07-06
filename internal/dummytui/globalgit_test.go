package dummytui

import (
	"regexp"
	"strings"
	"testing"
)

// ggitApp returns an App on the Global Git tab.
func ggitApp(t *testing.T) App {
	t.Helper()
	a, _ := press(t, NewApp(), "3")
	return a
}

// ggitModel extracts the Global Git child model.
func ggitModel(t *testing.T, a App) globalGitModel {
	t.Helper()
	m, ok := a.screens[tabGlobalGit].(globalGitModel)
	if !ok {
		t.Fatalf("screens[2] is %T, want globalGitModel", a.screens[tabGlobalGit])
	}
	return m
}

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
	for _, key := range m.gitApplyChosen(overlaidGitOptions(a.state)) {
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
	a, _ = press(t, a, "enter") // confirm
	// The receipt is NOT wrapped (matching the existing 02-14 pattern for
	// long ceremony result messages — see TestGlobalGitBaselineCeremonyAndResult)
	// so it hard-clips at the frame width; assert the frozen PREFIX instead
	// of the full string.
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

func TestGlobalGitBaselineCeremonyAndResult(t *testing.T) {
	a := ggitApp(t)
	m := ggitModel(t, a)
	if got := len(m.gitApplyChosen(overlaidGitOptions(a.state))); got != 10 {
		t.Fatalf("initial chosen = %d, want 10 (all needs-action rows)", got)
	}

	a, _ = press(t, a, "a")
	view := appView(a)
	if !strings.Contains(view, "Write baseline managed block to ~/.gitconfig") {
		t.Fatalf("ceremony missing:\n%s", view)
	}
	// The preview shows the sentinel-delimited managed block.
	if !strings.Contains(view, "# BEGIN gitid managed: global-git") {
		t.Error("preview must show the BEGIN sentinel")
	}
	if !strings.Contains(view, "defaultBranch = main") {
		t.Error("preview must show the recommended key=value lines")
	}

	a, _ = press(t, a, "enter") // confirm → receipt with GlobalGitResultMessage
	if !strings.Contains(appView(a), "10 of 10 baseline options applied to ~/.gitconfig.") {
		t.Error("GlobalGitResultMessage missing from the receipt")
	}
	a, _ = press(t, a, "enter") // done
	if !a.state.GitBaselineApplied {
		t.Fatal("ApplyGitBaseline not dispatched")
	}
	if a.note != "Global git baseline applied — user.email untouched." {
		t.Errorf("note = %q", a.note)
	}

	// Applied overlay: move off init.defaultBranch (which always shows the
	// deep-dive) — core.ignorecase now renders the overlay one-liner. The
	// keypress also clears the transient note, revealing the new status.
	a, _ = press(t, a, "down")
	if !strings.Contains(regionFlat(a, 45, 100), "Applied by gitid — Keeps file-name case always significant") {
		t.Error("applied rows must render the overlay one-liner")
	}
	if !strings.Contains(appView(a), "Baseline applied. user.email stays untouched — identities own their author.") {
		t.Error("post-apply status missing")
	}
}

func TestGlobalGitSpaceToggleIsCopyOnWrite(t *testing.T) {
	m := newGlobalGitModel()
	orig := m.chosen
	m.detailKey = "pull.rebase"
	if !orig["pull.rebase"] {
		t.Fatal("fixture: pull.rebase must start pre-chosen")
	}
	res := m.handleKey(pressKey("space"), Seed())
	next, ok := res.model.(globalGitModel)
	if !ok {
		t.Fatalf("model is %T, want globalGitModel", res.model)
	}
	if next.chosen["pull.rebase"] {
		t.Error("space must un-choose the selected option")
	}
	if !orig["pull.rebase"] {
		t.Error("Elm purity: the toggle mutated the map shared with the pre-update model copy")
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
