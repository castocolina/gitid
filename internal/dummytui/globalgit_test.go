package dummytui

import (
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
		"user.email (global)", "push.autoSetupRemote", "pull.rebase",
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
	if !strings.Contains(regionFlat(ggitApp(t), 44, 100), "This is advisory, never a compliance gate.") {
		t.Error("advisory alert missing")
	}
}

func TestGlobalGitUserEmailIsAwarenessOnly(t *testing.T) {
	a := ggitApp(t)
	// user.email row is index 3; it must never be checkable nor applied.
	a = pressSeq(t, a, "down", "down", "down")
	m := ggitModel(t, a)
	if m.detailKey != "user.email (global)" {
		t.Fatalf("detailKey = %q, want user.email (global)", m.detailKey)
	}
	a, _ = press(t, a, "space")
	m = ggitModel(t, a)
	if m.chosen["user.email (global)"] {
		t.Error("the user.email awareness row must never be checkable")
	}
	if !strings.Contains(appView(a), "gitid never writes a global [user] section") {
		t.Error("user.email one-liner missing")
	}
	// Apply never includes it.
	for _, key := range m.gitApplyChosen(overlaidGitOptions(a.state)) {
		if key == "user.email (global)" {
			t.Error("apply set must never contain the user.email row")
		}
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
	if !strings.Contains(regionFlat(a, 44, 100), "Applied by gitid — Keeps file-name case always significant") {
		t.Error("applied rows must render the overlay one-liner")
	}
	if !strings.Contains(appView(a), "Baseline applied. user.email stays untouched — identities own their author.") {
		t.Error("post-apply status missing")
	}
}
