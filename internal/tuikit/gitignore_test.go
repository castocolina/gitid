package tuikit

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func gignModel(t *testing.T, a App) gitIgnoreModel {
	t.Helper()
	m, ok := a.screens[TabGitIgnore].(gitIgnoreModel)
	if !ok {
		t.Fatalf("screens[TabGitIgnore] is %T, want gitIgnoreModel", a.screens[TabGitIgnore])
	}
	return m
}

func gignApp(t *testing.T, b stubBackend) App {
	t.Helper()
	a, _ := press(t, NewApp(b), "5")
	if a.ActiveTab() != TabGitIgnore {
		t.Fatalf("tab = %v after pressing 5, want TabGitIgnore", a.ActiveTab())
	}
	return a
}

func TestGitIgnoreActivateStoresState(t *testing.T) {
	want := GlobalGitIgnoreView{
		Path:           "~/.gitignore_global",
		Content:        ".DS_Store\n*.log",
		Managed:        true,
		DefaultContent: ".DS_Store",
		ExcludesFile:   "~/.gitignore_global",
		Wiring:         GitIgnoreWiredAtManaged,
	}
	calls := 0
	b := stubBackend{
		gignState: want,
		gignStateFn: func() (GlobalGitIgnoreView, error) {
			calls++
			return want, nil
		},
	}
	a := gignApp(t, b)
	m := gignModel(t, a)
	if calls != 1 {
		t.Fatalf("activate called GlobalGitIgnoreState %d times, want 1", calls)
	}
	if m.state.Content != want.Content || m.state.Path != want.Path || !m.state.Managed ||
		m.state.DefaultContent != want.DefaultContent || m.state.Wiring != want.Wiring {
		t.Errorf("stored view = %+v, want %+v", m.state, want)
	}
}

func TestGitIgnoreEditorSeedsAndResetsCursor(t *testing.T) {
	b := stubBackend{gignState: GlobalGitIgnoreView{Content: "one\ntwo", DefaultContent: "default"}}
	a := gignApp(t, b)
	m := gignModel(t, a)
	if got := m.editor.Value(); got != "one\ntwo" {
		t.Fatalf("editor value = %q, want seeded content", got)
	}
	if got := m.editor.Line(); got != 0 {
		t.Fatalf("editor line = %d, want 0", got)
	}
	a, _ = press(t, a, "enter")
	m = gignModel(t, a)
	if got := m.editor.Line(); got != 0 {
		t.Fatalf("focused editor line = %d, want 0", got)
	}
	a, _ = press(t, a, "esc")
	a, _ = press(t, a, "r")
	m = gignModel(t, a)
	if got := m.editor.Value(); got != "default" || m.editor.Line() != 0 {
		t.Fatalf("reset editor = %q at line %d, want default at line 0", got, m.editor.Line())
	}
}

func TestGitIgnoreEditorTypingAndBlur(t *testing.T) {
	b := stubBackend{gignState: GlobalGitIgnoreView{Content: "one", DefaultContent: "default"}}
	a := gignApp(t, b)
	a, _ = press(t, a, "enter")
	for _, key := range []string{"r", "a", "e"} {
		a, _ = press(t, a, key)
	}
	m := gignModel(t, a)
	if m.editor.Value() != "raeone" {
		t.Fatalf("editor value = %q, want typed letters", m.editor.Value())
	}
	a, _ = press(t, a, "esc")
	m = gignModel(t, a)
	if m.editing || m.editor.Focused() || m.editor.Value() != "raeone" {
		t.Fatalf("blurred editor state = editing %v focused %v value %q", m.editing, m.editor.Focused(), m.editor.Value())
	}
}

func TestGitIgnoreStateErrorFailsClosed(t *testing.T) {
	b := stubBackend{gignStateErr: fmt.Errorf("probe failed: disk unreadable")}
	a := gignApp(t, b)
	body := appView(a)
	if !strings.Contains(body, "disk unreadable") {
		t.Errorf("pane must render the state error, got:\n%s", body)
	}
	a, _ = press(t, a, "a")
	if gignModel(t, a).ceremonyOpen {
		t.Error("apply must be inert when the state probe failed")
	}
}

func TestGitIgnoreNoBaselineBlockFailsClosed(t *testing.T) {
	b := stubBackend{
		gignState: GlobalGitIgnoreView{
			Path:    "~/.gitignore_global",
			Content: ".DS_Store",
			Wiring:  GitIgnoreNoBaselineBlock,
		},
	}
	a := gignApp(t, b)
	body := appView(a)
	if !strings.Contains(body, "No gitid-managed Git baseline") {
		t.Errorf("pane must render the no-baseline wiring warning, got:\n%s", body)
	}
	a, _ = press(t, a, "a")
	if gignModel(t, a).ceremonyOpen {
		t.Error("apply must be inert when no managed baseline block exists")
	}
}

func TestGitIgnoreKeyUnsetShowsTwoTargetNoteAndCeremony(t *testing.T) {
	plan := GlobalGitIgnoreApplyPlanView{
		Targets: []string{"~/.gitignore_global", "~/.gitconfig.d/00-baseline"},
		Backups: []string{
			"~/.gitignore_global.bak.<timestamp>",
			"~/.gitconfig.d/00-baseline.bak.<timestamp>",
		},
		Diff: "+ excludesfile",
	}
	b := stubBackend{
		gignState: GlobalGitIgnoreView{
			Path:    "~/.gitignore_global",
			Content: ".DS_Store",
			Managed: true,
			Wiring:  GitIgnoreKeyUnset,
		},
		gignApplyPlan: plan,
	}
	a := gignApp(t, b)
	body := appView(a)
	if !strings.Contains(body, GitIgnoreTwoTargetNote) {
		t.Errorf("pane must say the write will also set core.excludesfile, got:\n%s", body)
	}
	a, _ = press(t, a, "a")
	m := gignModel(t, a)
	if !m.ceremonyOpen {
		t.Fatal("apply must open the ceremony when the key is unset")
	}
	if got := strings.Join(m.ceremony.cfg.Targets, ","); got != strings.Join(plan.Targets, ",") {
		t.Errorf("ceremony targets = %q, want both files %q", got, plan.Targets)
	}
}

func TestGitIgnoreDifferentFileStillOffersApply(t *testing.T) {
	b := stubBackend{
		gignState: GlobalGitIgnoreView{
			Path:         "~/.gitignore_global",
			Content:      ".DS_Store",
			ExcludesFile: "~/my-own-ignore",
			Wiring:       GitIgnorePointsElsewhere,
			Managed:      true,
		},
	}
	a := gignApp(t, b)
	body := appView(a)
	if !strings.Contains(body, "~/my-own-ignore") {
		t.Errorf("pane must name the other excludesfile path, got:\n%s", body)
	}
	if !strings.Contains(body, "~/.gitignore_global") {
		t.Errorf("pane must name this file's path, got:\n%s", body)
	}
	a, _ = press(t, a, "a")
	if !gignModel(t, a).ceremonyOpen {
		t.Error("apply must still be offered when the key points at a different file")
	}
}

func TestGitIgnoreApplyOpensCeremonyWithPlan(t *testing.T) {
	content := ".DS_Store\n*.log"
	plan := GlobalGitIgnoreApplyPlanView{
		Targets:   []string{"~/.gitignore_global"},
		Backups:   []string{"~/.gitignore_global.bak.<timestamp>"},
		Diff:      "+ *.log",
		PlanToken: "token-abc",
	}
	var gotContent string
	b := stubBackend{
		gignState: GlobalGitIgnoreView{
			Path:    "~/.gitignore_global",
			Content: content,
			Managed: true,
			Wiring:  GitIgnoreWiredAtManaged,
		},
		gignApplyPlan: plan,
		gignApplyPlanFn: func(c string) (GlobalGitIgnoreApplyPlanView, error) {
			gotContent = c
			return plan, nil
		},
	}
	a := gignApp(t, b)
	a, _ = press(t, a, "a")
	m := gignModel(t, a)
	if !m.ceremonyOpen {
		t.Fatal("pressing a must open the ceremony")
	}
	if gotContent != content {
		t.Errorf("apply plan received %q, want %q", gotContent, content)
	}
	if got := strings.Join(m.ceremony.cfg.Targets, ","); got != strings.Join(plan.Targets, ",") {
		t.Errorf("ceremony targets = %q, want %q", got, plan.Targets)
	}
	if got := strings.Join(m.ceremony.cfg.Backups, ","); got != strings.Join(plan.Backups, ",") {
		t.Errorf("ceremony backups = %q, want %q", got, plan.Backups)
	}
	if m.ceremony.cfg.Preview != plan.Diff {
		t.Errorf("ceremony preview = %q, want %q", m.ceremony.cfg.Preview, plan.Diff)
	}
}

// TestGitIgnoreApplyCeremonyEnterConfirmsWithoutTypedWord is the 260919-jnl
// contract: Ignore apply keeps the preview ceremony, Confirm is focused, and
// Enter confirms without typing yes.
func TestGitIgnoreApplyCeremonyEnterConfirmsWithoutTypedWord(t *testing.T) {
	plan := GlobalGitIgnoreApplyPlanView{
		Targets:   []string{"~/.gitignore_global"},
		Backups:   []string{"~/.gitignore_global.bak.<timestamp>"},
		Diff:      "+ *.log",
		PlanToken: "token-enter",
	}
	b := stubBackend{
		gignState: GlobalGitIgnoreView{
			Path:    "~/.gitignore_global",
			Content: ".DS_Store",
			Managed: true,
			Wiring:  GitIgnoreWiredAtManaged,
		},
		gignApplyPlan: plan,
	}
	a := gignApp(t, b)
	a, _ = press(t, a, "a")
	m := gignModel(t, a)
	if !m.ceremonyOpen {
		t.Fatal("setup: a must open the apply ceremony")
	}
	if m.ceremony.cfg.Destructive != nil {
		t.Fatal("apply ceremony must not require typing a confirm word")
	}
	if m.ceremony.focus != ceremonyFocusConfirm {
		t.Fatalf("apply ceremony focus = %v, want Confirm so Enter confirms", m.ceremony.focus)
	}
	a, _ = press(t, a, "enter")
	m = gignModel(t, a)
	if !m.commitPending {
		t.Fatal("Enter on the apply ceremony must confirm without typing yes")
	}
}

func TestGitIgnoreApplyPlanErrorFailsClosed(t *testing.T) {
	commits := 0
	b := stubBackend{
		gignState: GlobalGitIgnoreView{
			Path:    "~/.gitignore_global",
			Content: ".DS_Store",
			Managed: true,
			Wiring:  GitIgnoreWiredAtManaged,
		},
		gignApplyPlanFn: func(string) (GlobalGitIgnoreApplyPlanView, error) {
			return GlobalGitIgnoreApplyPlanView{}, fmt.Errorf("line 3 looks like a gitid managed-block marker")
		},
		gignCommitFn: func(_, _ string) tea.Cmd {
			commits++
			return nil
		},
	}
	a := gignApp(t, b)
	a, _ = press(t, a, "a")
	m := gignModel(t, a)
	if m.ceremonyOpen {
		t.Error("apply-plan error must not open the ceremony")
	}
	if !strings.Contains(appView(a), "managed-block marker") {
		t.Errorf("pane must render the apply error, got:\n%s", appView(a))
	}
	if commits != 0 {
		t.Errorf("CommitGlobalGitIgnore called %d times, want 0", commits)
	}
}

func TestGitIgnoreEmptyBackupsStayEmpty(t *testing.T) {
	b := stubBackend{
		gignState: GlobalGitIgnoreView{
			Path:    "~/.gitignore_global",
			Content: ".DS_Store",
			Managed: true,
			Wiring:  GitIgnoreWiredAtManaged,
		},
		gignApplyPlan: GlobalGitIgnoreApplyPlanView{
			Targets: []string{"~/.gitignore_global"},
			Diff:    "+ .DS_Store",
		},
	}
	a := gignApp(t, b)
	a, _ = press(t, a, "a")
	m := gignModel(t, a)
	if len(m.ceremony.cfg.Backups) != 0 {
		t.Errorf("empty plan backups must stay empty, got %v", m.ceremony.cfg.Backups)
	}
}

func TestGitIgnoreEscCancelsWithoutCommit(t *testing.T) {
	commits := 0
	b := stubBackend{
		gignState: GlobalGitIgnoreView{
			Path:    "~/.gitignore_global",
			Content: ".DS_Store",
			Managed: true,
			Wiring:  GitIgnoreWiredAtManaged,
		},
		gignApplyPlan: GlobalGitIgnoreApplyPlanView{
			Targets: []string{"~/.gitignore_global"},
			Diff:    "+ .DS_Store",
		},
		gignCommitFn: func(_, _ string) tea.Cmd {
			commits++
			return nil
		},
	}
	a := gignApp(t, b)
	a, _ = press(t, a, "a")
	a, _ = press(t, a, "esc")
	if gignModel(t, a).ceremonyOpen {
		t.Error("Esc must close the ceremony")
	}
	if commits != 0 {
		t.Errorf("Esc dispatched %d commits, want 0", commits)
	}
}

func TestGitIgnoreConfirmDispatchesExactPreviewedContent(t *testing.T) {
	content := ".DS_Store\n*.log"
	var recordedContent, recordedToken string
	calls := 0
	b := stubBackend{
		gignState: GlobalGitIgnoreView{
			Path:    "~/.gitignore_global",
			Content: content,
			Managed: true,
			Wiring:  GitIgnoreWiredAtManaged,
		},
		gignApplyPlan: GlobalGitIgnoreApplyPlanView{
			Targets:   []string{"~/.gitignore_global"},
			Diff:      "+ *.log",
			PlanToken: "tok-1",
		},
		gignCommitFn: func(c, token string) tea.Cmd {
			calls++
			recordedContent = c
			recordedToken = token
			return func() tea.Msg { return GlobalGitIgnoreCommitMsg{} }
		},
	}
	a := gignApp(t, b)
	a, _ = press(t, a, "a")
	_, cmd := press(t, a, "enter")
	if cmd == nil {
		t.Fatal("confirm must dispatch CommitGlobalGitIgnore")
	}
	_ = cmd()
	if calls != 1 {
		t.Fatalf("CommitGlobalGitIgnore called %d times, want 1", calls)
	}
	if recordedContent != content {
		t.Errorf("commit content = %q, want the exact apply-plan string %q", recordedContent, content)
	}
	if recordedToken != "tok-1" {
		t.Errorf("commit token = %q, want tok-1", recordedToken)
	}
}

func TestGitIgnoreCommitMsgRendersReceipts(t *testing.T) {
	open := func(t *testing.T) App {
		t.Helper()
		b := stubBackend{
			gignState: GlobalGitIgnoreView{
				Path:    "~/.gitignore_global",
				Content: ".DS_Store",
				Managed: true,
				Wiring:  GitIgnoreWiredAtManaged,
			},
			gignApplyPlan: GlobalGitIgnoreApplyPlanView{
				Targets: []string{"~/.gitignore_global"},
				Diff:    "+ .DS_Store",
			},
		}
		a := gignApp(t, b)
		a, _ = press(t, a, "a")
		a, _ = press(t, a, "enter")
		return a
	}

	t.Run("error", func(t *testing.T) {
		a := open(t)
		a, _ = deliverMsg(t, a, GlobalGitIgnoreCommitMsg{Err: "write failed: disk full"})
		if !strings.Contains(appView(a), "write failed") {
			t.Errorf("failure receipt missing, got:\n%s", appView(a))
		}
	})

	t.Run("backups listed", func(t *testing.T) {
		a := open(t)
		backup := "~/.gitignore_global.bak.123"
		a, _ = deliverMsg(t, a, GlobalGitIgnoreCommitMsg{Backups: []string{backup}})
		view := appView(a)
		if !strings.Contains(view, backup) {
			t.Errorf("success receipt must list backups, got:\n%s", view)
		}
		if !strings.Contains(view, GitIgnoreReceiptSingleTarget) {
			t.Errorf("wired success must use the single-target receipt, got:\n%s", view)
		}
	})

	t.Run("no backups uses no-backup wording", func(t *testing.T) {
		a := open(t)
		a, _ = deliverMsg(t, a, GlobalGitIgnoreCommitMsg{})
		view := appView(a)
		if !strings.Contains(view, GitIgnoreReceiptNoBackup) {
			t.Errorf("empty backups must use the no-backup-needed wording, got:\n%s", view)
		}
		if strings.Contains(view, ".bak.") {
			t.Error("no-backup receipt must not invent a backup path")
		}
	})
}

func TestGitIgnoreReceiptWiringVariants(t *testing.T) {
	confirm := func(t *testing.T, wiring GlobalGitIgnoreWiring, excludes string) string {
		t.Helper()
		b := stubBackend{
			gignState: GlobalGitIgnoreView{
				Path:         "~/.gitignore_global",
				Content:      ".DS_Store",
				Managed:      true,
				Wiring:       wiring,
				ExcludesFile: excludes,
			},
			gignApplyPlan: GlobalGitIgnoreApplyPlanView{
				Targets: []string{"~/.gitignore_global"},
				Diff:    "+ .DS_Store",
			},
		}
		a := gignApp(t, b)
		a, _ = press(t, a, "a")
		a, _ = press(t, a, "enter")
		a, _ = deliverMsg(t, a, GlobalGitIgnoreCommitMsg{Backups: []string{"~/.gitignore_global.bak.1"}})
		return appView(a)
	}

	t.Run("points elsewhere names the other path", func(t *testing.T) {
		view := confirm(t, GitIgnorePointsElsewhere, "~/my-own-ignore")
		if !strings.Contains(view, "~/my-own-ignore") {
			t.Errorf("wrong-target receipt must name the other path, got:\n%s", view)
		}
		if strings.Contains(view, GitIgnoreReceiptSingleTarget) && !strings.Contains(view, "still points") {
			t.Errorf("wrong-target receipt must not be the plain single-target line, got:\n%s", view)
		}
		if !strings.Contains(view, "not reading") && !strings.Contains(view, "still points") {
			t.Errorf("wrong-target receipt must say Git is not reading this file, got:\n%s", view)
		}
	})

	t.Run("wired uses the plain single-target line", func(t *testing.T) {
		view := confirm(t, GitIgnoreWiredAtManaged, "~/.gitignore_global")
		if !strings.Contains(view, GitIgnoreReceiptSingleTarget) {
			t.Errorf("wired receipt must use the plain single-target line, got:\n%s", view)
		}
		if strings.Contains(view, "my-own-ignore") {
			t.Errorf("wired receipt must not mention another path, got:\n%s", view)
		}
	})
}

func TestGitIgnoreChangedSincePreview(t *testing.T) {
	b := stubBackend{
		gignState: GlobalGitIgnoreView{
			Path:    "~/.gitignore_global",
			Content: ".DS_Store",
			Managed: true,
			Wiring:  GitIgnoreWiredAtManaged,
		},
		gignApplyPlan: GlobalGitIgnoreApplyPlanView{
			Targets: []string{"~/.gitignore_global"},
			Diff:    "+ .DS_Store",
		},
	}
	a := gignApp(t, b)
	a, _ = press(t, a, "a")
	a, _ = press(t, a, "enter")
	a, _ = deliverMsg(t, a, GlobalGitIgnoreCommitMsg{
		Err:                 "file changed since preview",
		ChangedSincePreview: true,
	})
	view := appView(a)
	if !strings.Contains(view, "changed since you last reviewed") {
		t.Errorf("changed-since-preview wording missing, got:\n%s", view)
	}
}

func TestGitIgnoreStaleCommitMsgIsIgnored(t *testing.T) {
	b := stubBackend{
		gignState: GlobalGitIgnoreView{
			Path:    "~/.gitignore_global",
			Content: ".DS_Store",
			Managed: true,
			Wiring:  GitIgnoreWiredAtManaged,
		},
	}
	a := gignApp(t, b)
	before := appView(a)
	a, _ = deliverMsg(t, a, GlobalGitIgnoreCommitMsg{Backups: []string{"invented.bak"}})
	if appView(a) != before {
		t.Error("a commit message arriving with no pending commit must not flip the receipt")
	}
	if strings.Contains(appView(a), "invented.bak") {
		t.Error("stale commit must not render a backup path")
	}
}

func TestGitIgnoreCapturesKeys(t *testing.T) {
	b := stubBackend{
		gignState: GlobalGitIgnoreView{
			Path:    "~/.gitignore_global",
			Content: ".DS_Store",
			Managed: true,
			Wiring:  GitIgnoreWiredAtManaged,
		},
		gignApplyPlan: GlobalGitIgnoreApplyPlanView{
			Targets: []string{"~/.gitignore_global"},
			Diff:    "+ .DS_Store",
		},
	}
	a := gignApp(t, b)
	if gignModel(t, a).view(Seed(), minFrameWidth, minFrameHeight).capturesKeys {
		t.Error("browse state must report capturesKeys false")
	}
	a, _ = press(t, a, "a")
	if !gignModel(t, a).view(Seed(), minFrameWidth, minFrameHeight).capturesKeys {
		t.Error("open ceremony must report capturesKeys true")
	}
}

// TestGitIgnoreViewCrumbsAreAlwaysEmpty pins 09.2-UI-REVIEW.md finding 1's
// fix (09.2-REVIEW.md WR-10 point 5): view() must return crumbs: []string{}
// on BOTH branches (browse/editor and the ceremony) so RenderFrame's
// breadcrumb reads "Global Git Ignore" exactly once — passing the tab's own
// label as an extra crumbs[] segment previously triple-duplicated the
// heading. Guarded only by re-promotable PTY frames before this unit
// assertion existed.
func TestGitIgnoreViewCrumbsAreAlwaysEmpty(t *testing.T) {
	b := stubBackend{
		gignState: GlobalGitIgnoreView{
			Path:    "~/.gitignore_global",
			Content: ".DS_Store",
			Managed: true,
			Wiring:  GitIgnoreWiredAtManaged,
		},
		gignApplyPlan: GlobalGitIgnoreApplyPlanView{
			Targets: []string{"~/.gitignore_global"},
			Diff:    "+ .DS_Store",
		},
	}
	a := gignApp(t, b)
	if v := gignModel(t, a).view(Seed(), minFrameWidth, minFrameHeight); len(v.crumbs) != 0 {
		t.Errorf("browse/editor crumbs = %#v, want an empty slice", v.crumbs)
	}
	a, _ = press(t, a, "a")
	if v := gignModel(t, a).view(Seed(), minFrameWidth, minFrameHeight); len(v.crumbs) != 0 {
		t.Errorf("ceremony crumbs = %#v, want an empty slice", v.crumbs)
	}
}

func TestGitIgnoreBrowseFooterUsesEditorCopy(t *testing.T) {
	b := stubBackend{gignState: GlobalGitIgnoreView{Content: "one", DefaultContent: "default"}}
	a := gignApp(t, b)
	v := gignModel(t, a).view(Seed(), minFrameWidth, minFrameHeight)
	if len(v.actions) != 3 || v.actions[0] != (FooterAction{Key: "Enter", Label: GitIgnoreEditLabel}) ||
		v.actions[1] != (FooterAction{Key: "r", Label: GitIgnoreResetLabel}) ||
		v.actions[2] != (FooterAction{Key: "a", Label: GitIgnoreApplyLabel}) {
		t.Fatalf("browse actions = %#v", v.actions)
	}
	if strings.Contains(v.status, "written") {
		t.Fatalf("browse status must warn about discarded edits, got %q", v.status)
	}
	a, _ = press(t, a, "enter")
	v = gignModel(t, a).view(Seed(), minFrameWidth, minFrameHeight)
	if len(v.actions) != 1 || v.actions[0] != (FooterAction{Key: "Esc", Label: GitIgnoreDoneEditingLabel}) {
		t.Fatalf("editing actions = %#v", v.actions)
	}
}

// TestGitIgnoreWindowResizePersistsEditorGeometry is the 09.2-REVIEW.md WR-03
// regression: before the fix, m.editor.SetWidth/SetHeight only ran inside
// view()'s value-receiver copy, so the model actually stored in
// App.screens[TabGitIgnore] (the one handleKey/handleMsg feed keystrokes to)
// never left textarea.New()'s 40x6 defaults. A real tea.WindowSizeMsg must
// now resize the PERSISTED model, not just whatever the next view() call
// renders.
func TestGitIgnoreWindowResizePersistsEditorGeometry(t *testing.T) {
	b := stubBackend{gignState: GlobalGitIgnoreView{Content: "one", DefaultContent: "default"}}
	a := gignApp(t, b)
	before := gignModel(t, a).editor
	if before.Width() >= 90 {
		t.Fatalf("setup: editor already wide before any resize (%d) — test no longer proves anything", before.Width())
	}
	model, _ := a.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	a, ok := model.(App)
	if !ok {
		t.Fatalf("Update(WindowSizeMsg) returned %T, want App", model)
	}
	after := gignModel(t, a).editor
	if after.Width() < 90 {
		t.Errorf("persisted editor width = %d after a 100-wide resize, want it to track the real terminal width (WR-03 regressed)", after.Width())
	}
	if after.Height() <= 1 {
		t.Errorf("persisted editor height = %d after a 30-row resize, want a real budget, not the 6-row textarea default", after.Height())
	}
}

// TestGitIgnoreEditorHeightAccountsForHeaderRows is the 09.2-REVIEW.md WR-04
// regression: the editor's height budget must shrink when the header above
// it grows (e.g. a stateErr advisory line appears), not stay pinned to a
// fixed guess that overflows the pane and gets silently truncated by
// fitPane.
func TestGitIgnoreEditorHeightAccountsForHeaderRows(t *testing.T) {
	// Managed: true so the "No managed block found yet" advisory (gated on
	// !m.state.Managed && m.stateErr == "") does not confound the
	// comparison — it would otherwise DISAPPEAR when stateErr is set,
	// masking the row stateErr itself adds.
	plain := gitIgnoreModel{state: GlobalGitIgnoreView{Path: "~/.gitignore_global", Managed: true, Wiring: GitIgnoreWiredAtManaged}}
	withAdvisory := plain
	withAdvisory.stateErr = "line 3: orphan BEGIN sentinel — repair the file by hand before gitid will touch it"

	const bodyBudget, editorWidth = 20, 96
	plainHeight := plain.editorHeight(bodyBudget, editorWidth, plain.headerText())
	advisoryHeight := withAdvisory.editorHeight(bodyBudget, editorWidth, withAdvisory.headerText())
	if advisoryHeight >= plainHeight {
		t.Errorf("editorHeight with a stateErr advisory = %d, want less than the no-advisory height %d (the advisory adds a header row)", advisoryHeight, plainHeight)
	}
	if plainHeight+advisoryHeight <= 0 {
		t.Fatalf("editorHeight must always return at least 1, got plain=%d advisory=%d", plainHeight, advisoryHeight)
	}
}

func TestGitIgnoreKeyFiveFromInitialView(t *testing.T) {
	a, _ := press(t, NewApp(stubBackend{}), "5")
	if a.ActiveTab() != TabGitIgnore {
		t.Errorf("key 5 → tab %v, want TabGitIgnore", a.ActiveTab())
	}
}

func TestGitIgnoreRightArrowFromDoctor(t *testing.T) {
	a, _ := press(t, NewApp(stubBackend{}), "4")
	if a.ActiveTab() != TabDoctor {
		t.Fatalf("setup: key 4 → %v, want TabDoctor", a.ActiveTab())
	}
	a, _ = press(t, a, "right")
	if a.ActiveTab() != TabGitIgnore {
		t.Errorf("right from Doctor → %v, want TabGitIgnore", a.ActiveTab())
	}
	a, _ = press(t, a, "right")
	if a.ActiveTab() != TabGitIgnore {
		t.Errorf("right from Git Ignore must clamp, got %v", a.ActiveTab())
	}
}

func TestHelpRowFitsFiveViews(t *testing.T) {
	a, _ := press(t, NewApp(stubBackend{}), "?")
	view := appView(a)
	if !strings.Contains(view, "1-5") {
		t.Errorf("help row must use the 1-5 range form, got:\n%s", view)
	}
	if strings.Contains(view, "1 · 2 · 3 · 4 · 5") {
		t.Error("help key column must not use the dotted form")
	}
	row := ""
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "1-5") {
			row = line
			break
		}
	}
	if row == "" {
		t.Fatal("help overlay missing the five-view row")
	}
	if !strings.Contains(row, "Ignore") {
		t.Errorf("help description must name Ignore, got %q", row)
	}
	if w := len([]rune(strings.TrimRight(row, " "))); w > minFrameWidth {
		t.Errorf("help row width = %d, want <= %d", w, minFrameWidth)
	}
	if !strings.HasPrefix(strings.TrimSpace(row), "1-5") {
		t.Errorf("key column must start with 1-5 untruncated, got %q", row)
	}
}
