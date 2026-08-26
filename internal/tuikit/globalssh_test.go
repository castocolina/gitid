package tuikit

import (
	"errors"
	"regexp"
	"strings"
	"testing"
)

// gssApp returns an App on the Global SSH tab.
func gssApp(t *testing.T) App {
	t.Helper()
	a, _ := press(t, NewApp(stubBackend{}), "2")
	return a
}

// gssModel extracts the Global SSH child model.
func gssModel(t *testing.T, a App) globalSSHModel {
	t.Helper()
	m, ok := a.screens[TabGlobalSSH].(globalSSHModel)
	if !ok {
		t.Fatalf("screens[1] is %T, want globalSSHModel", a.screens[TabGlobalSSH])
	}
	return m
}

func TestGlobalSSHArrowsSwitchSubTabs(t *testing.T) {
	a := gssApp(t)
	view := appView(a)
	if !strings.Contains(view, "Options") || !strings.Contains(view, "Storage & preview") {
		t.Fatal("sub-tab strip missing")
	}
	if !strings.Contains(view, "Global SSH › Options") {
		t.Error("breadcrumb should show the active sub-tab")
	}
	a, _ = press(t, a, "right")
	if !strings.Contains(appView(a), "Global SSH › Storage & preview") {
		t.Error("→ must switch to the Storage sub-tab")
	}
	a, _ = press(t, a, "left")
	if !strings.Contains(appView(a), "Global SSH › Options") {
		t.Error("← must switch back to Options")
	}
}

func TestGlobalSSHOptionsMasterDetail(t *testing.T) {
	a := gssApp(t)
	view := appView(a)
	// IdentitiesOnly (the initial detail) shows the full explanation.
	if !strings.Contains(view, "IdentitiesOnly") {
		t.Error("IdentitiesOnly row missing")
	}
	if !strings.Contains(view, "When IdentitiesOnly is not set") {
		t.Error("IdentitiesOnly must show the full GSSH-01 explanation")
	}
	if !strings.Contains(regionFlat(a, 45, 100), "This is advisory, never a compliance gate.") {
		t.Error("advisory note missing (never blocking)")
	}
	// Moving the selection updates the detail live.
	a, _ = press(t, a, "up") // IdentitiesOnly(3) → HashKnownHosts(2)
	if !strings.Contains(appView(a), "Hashing known_hosts hides which hosts you connect to") {
		t.Error("↑ must re-render the detail pane with the one-liner")
	}
	// SSH findings banner (seeded: 3 SSH findings).
	if !strings.Contains(appView(a), "The doctor found 3 SSH findings beyond these global options.") {
		t.Error("SSH findings banner missing")
	}
}

func TestGlobalSSHApplySubsetMarksAppliedAndShowsDeclined(t *testing.T) {
	a := gssApp(t)
	m := gssModel(t, a)
	// Initial chosen: every needs-action key EXCEPT ForwardAgent.
	keys := m.applyChosen(m.overlaidOptions(a.state))
	if len(keys) != 3 {
		t.Fatalf("initial chosen = %v, want 3 (ForwardAgent declined)", keys)
	}

	a, _ = press(t, a, "a")
	view := appView(a)
	if !strings.Contains(view, "Write Host * managed block to ~/.ssh/config") {
		t.Fatalf("apply ceremony missing:\n%s", view)
	}
	if !strings.Contains(view, "+ StrictHostKeyChecking ask") || !strings.Contains(view, "+ IdentitiesOnly yes") {
		t.Error("chosen keys must render as + diff lines")
	}
	if !strings.Contains(view, "ForwardAgent — left unchanged (declined; advisory)") {
		t.Error("declined pending key must render the left-unchanged line")
	}
	if !strings.Contains(view, "UseKeychain yes (already set)") {
		t.Error("already-set options must render as context lines")
	}

	// Confirm dispatches the async commit; NOTHING applies optimistically.
	a, cmd := press(t, a, "enter")
	if len(a.state.SSHApplied) != 0 {
		t.Fatalf("confirmation must not apply optimistically, SSHApplied = %v", a.state.SSHApplied)
	}
	if cmd == nil {
		t.Fatal("confirmation must dispatch the async commit command")
	}
	msg, ok := cmd().(GlobalSSHCommitMsg)
	if !ok {
		t.Fatalf("commit delivered %T, want GlobalSSHCommitMsg", cmd())
	}

	// The receipt appears ONLY from the commit's explicit success.
	model, _ := a.Update(msg)
	a = model.(App)
	if !strings.Contains(appView(a), "3 of 4 recommended options applied to Host *.") {
		t.Error("result message missing")
	}
	if len(a.state.SSHApplied) != 3 {
		t.Fatalf("SSHApplied = %v, want 3 only after the commit succeeded", a.state.SSHApplied)
	}

	a, _ = press(t, a, "enter") // done → back to browse
	// Applied overlay: an applied key's detail renders "Applied by
	// gitid — <one-liner>" (IdentitiesOnly always shows the deep-dive, so
	// move the selection up to HashKnownHosts).
	a, _ = press(t, a, "up")
	if !strings.Contains(regionFlat(a, 45, 100), "Applied by gitid — Hashing known_hosts") {
		t.Error("applied keys must render the Applied-by-gitid overlay one-liner")
	}
	// ForwardAgent is still pending.
	if got := pendingOptions(m.overlaidOptions(a.state)); len(got) != 1 || got[0].Key != "ForwardAgent" {
		t.Errorf("pending after apply = %v, want only ForwardAgent", got)
	}
}

func TestGlobalSSHSpaceTogglesChoice(t *testing.T) {
	a := gssApp(t) // detail starts at IdentitiesOnly (pending, chosen)
	a, _ = press(t, a, "space")
	m := gssModel(t, a)
	if m.chosen["IdentitiesOnly"] {
		t.Error("space must uncheck the selected pending option")
	}
	a, _ = press(t, a, "space")
	m = gssModel(t, a)
	if !m.chosen["IdentitiesOnly"] {
		t.Error("space must re-check the selected pending option")
	}
}

func TestGlobalSSHStoragePreviewsSwitchAndMigrateRoundTrips(t *testing.T) {
	a := gssApp(t)
	a, _ = press(t, a, "right") // → Storage & preview
	view := appView(a)
	if !strings.Contains(regionFlat(a, 0, 44), "Sentinel blocks in ~/.ssh/config (default) — current") {
		t.Error("sentinel radio with current marker missing")
	}
	if !strings.Contains(view, "sentinel blocks in place") && !strings.Contains(view, "sentinel-delimited") {
		t.Error("sentinel resulting-config preview missing")
	}
	if strings.Contains(view, "Migrate layout…") {
		t.Error("Migrate must NOT appear while the choice matches the current layout")
	}

	// Choose include: previews switch, Migrate appears.
	a, _ = press(t, a, "down")
	view = appView(a)
	if !strings.Contains(view, "Include ~/.ssh/config.d/gitid.config") {
		t.Error("include-layout preview missing")
	}
	if !strings.Contains(view, "# ~/.ssh/config.d/gitid.config (gitid-owned file)") {
		t.Error("owned-file preview missing")
	}
	if !strings.Contains(view, "Migrate layout… (Enter)") {
		t.Error("Migrate must appear when the choice differs from current")
	}

	// Walk the migration ceremony.
	a, _ = press(t, a, "enter")
	if !strings.Contains(appView(a), "Migrate SSH storage layout → Include’d gitid.config") {
		t.Fatalf("migration ceremony missing:\n%s", appView(a))
	}
	a, _ = press(t, a, "enter")
	a, _ = press(t, a, "enter")
	if a.state.SSHStorage != StorageInclude {
		t.Fatalf("SSHStorage = %q, want include", a.state.SSHStorage)
	}

	// Reversible (STORE-03): migrate back.
	a, _ = press(t, a, "down") // toggle radio back to sentinel
	a, _ = press(t, a, "enter")
	a, _ = press(t, a, "enter")
	a, _ = press(t, a, "enter")
	if a.state.SSHStorage != StorageSentinel {
		t.Errorf("SSHStorage = %q, want sentinel (round trip)", a.state.SSHStorage)
	}
}

func TestGlobalSSHApplyTargetsOwnedFileUnderIncludeLayout(t *testing.T) {
	a := gssApp(t)
	a.state = Reduce(a.state, SetSSHStorage{Layout: StorageInclude, Backup: "b"})
	a, _ = press(t, a, "a")
	if !strings.Contains(appView(a), "Touches ~/.ssh/config.d/gitid.config") {
		t.Error("apply ceremony must target the owned file under the include layout")
	}
}

// TestGlobalSSHApplyCeremonyNamesStorageTarget pins D-07's confirm-screen
// copy: state A names the file GlobalSSHApplyPlanView.Targets supplied, not
// a hardcoded ~/.ssh/config. The path is a scoped divergence from the frozen
// heading (Phase-3 D-05 precedent).
func TestGlobalSSHApplyCeremonyNamesStorageTarget(t *testing.T) {
	const sentinel = "/tmp/gitid-sentinel-resolved-target.config"
	b := &stubBackend{sshApplyPlan: GlobalSSHApplyPlanView{Targets: []string{sentinel}}}
	a := NewApp(b)
	a, _ = press(t, a, "2")
	a, _ = press(t, a, "a")
	view := appView(a)
	want := "Write Host * managed block to " + sentinel
	if !strings.Contains(view, want) {
		t.Fatalf("ceremony state A must name the plan target; want %q in:\n%s", want, view)
	}
	if strings.Contains(view, "Write Host * managed block to ~/.ssh/config\n") ||
		strings.Contains(view, "Write Host * managed block to ~/.ssh/config ") {
		t.Error("ceremony must not fall back to a hardcoded path when the plan supplied a target")
	}
}

func TestGlobalSSHSpaceToggleIsCopyOnWrite(t *testing.T) {
	m := newGlobalSSHModel(stubBackend{})
	activated, _ := m.activate(Seed())
	m = activated.(globalSSHModel)
	orig := m.chosen
	m.detailKey = "HashKnownHosts"
	if !orig["HashKnownHosts"] {
		t.Fatal("fixture: HashKnownHosts must start pre-chosen")
	}
	res := m.handleKey(pressKey("space"), Seed())
	next, ok := res.model.(globalSSHModel)
	if !ok {
		t.Fatalf("model is %T, want globalSSHModel", res.model)
	}
	if next.chosen["HashKnownHosts"] {
		t.Error("space must un-choose the selected option")
	}
	if !orig["HashKnownHosts"] {
		t.Error("Elm purity: the toggle mutated the map shared with the pre-update model copy")
	}
}

func TestGlobalSSHLongExplanationClipsWithVisibleCue(t *testing.T) {
	// IdentitiesOnly (the initial detail) carries the long GSSH-01
	// explanation — at 100x30 it cannot fully fit, and the overflow must be
	// announced, never silently cut mid-sentence (H3).
	a := gssApp(t)
	view := appView(a)
	if !strings.Contains(view, "When IdentitiesOnly is not set") {
		t.Fatal("IdentitiesOnly explanation missing")
	}
	if !regexp.MustCompile(`… \(\+\d+ more lines\)`).MatchString(view) {
		t.Error("clipped explanation must render the `… (+n more lines)` cue (H3)")
	}
	// A short one-liner detail shows no cue.
	a, _ = press(t, a, "up") // → HashKnownHosts
	if regexp.MustCompile(`… \(\+\d+ more lines\)`).MatchString(appView(a)) {
		t.Error("short detail must not render a clip cue")
	}
}

func TestGlobalSSHOptionsRenderBackendViewSentinels(t *testing.T) {
	// The real tracer proof at the render layer: the Options sub-tab body must
	// render the CURRENT VALUE and PROVENANCE the backend returned — never the
	// static fixture strings for that key.
	b := &stubBackend{sshOptions: []GlobalSSHOptionView{
		{Key: "HashKnownHosts", CurrentValue: "SENTINEL-CURRENT", Provenance: "SENTINEL-PROVENANCE", Recommended: "yes", Risk: "Low", State: GlobalSSHNeedsAction},
		{Key: "StrictHostKeyChecking", CurrentValue: "ask", Provenance: "set by you at ~/.ssh/config line 1", Recommended: "accept-new", Risk: "Medium", State: GlobalSSHNeedsAction},
		{Key: "UseKeychain", CurrentValue: "yes (macOS only)", Provenance: "fixture value", Recommended: "yes", Risk: "Low", State: GlobalSSHAlreadySet},
	}}
	a := NewApp(b)
	a, _ = press(t, a, "2")
	view := appView(a)
	if !strings.Contains(view, "SENTINEL-CURRENT") {
		t.Error("the backend's CurrentValue must render in the row (not the fixture's flat string)")
	}
	if !strings.Contains(view, "SENTINEL-PROVENANCE") {
		t.Error("the backend's Provenance must render in the detail pane")
	}
}

func TestGlobalSSHOptionsErrorRendersNoteInsteadOfBlankPane(t *testing.T) {
	b := &stubBackend{sshOptionsErr: errors.New("probe exploded")}
	a := NewApp(b)
	a, _ = press(t, a, "2")
	view := appView(a)
	if !strings.Contains(view, "probe exploded") {
		t.Errorf("the option-states error note must render instead of a blank pane:\n%s", view)
	}
	if strings.Contains(view, "now: ") {
		t.Errorf("no fixture option rows may render when the backend could not answer:\n%s", view)
	}
}

// TestGlobalSSHApplyConfirmationIsInFlightNoApplyAction drives the ceremony
// model directly (mirroring the standalone Git ceremony tests): confirming the
// apply dispatches the async commit and leaves the ceremony IN FLIGHT with no
// apply action emitted; the ApplySSH action arrives only from the commit's
// explicit success.
func TestGlobalSSHApplyConfirmationIsInFlightNoApplyAction(t *testing.T) {
	b := &stubBackend{sshCommitMsg: GlobalSSHCommitMsg{Backups: []string{"~/.ssh/config.bak.1"}}}
	m := newGlobalSSHModel(b)
	state := Seed()
	activated, _ := m.activate(state)
	m = activated.(globalSSHModel)
	opened := m.handleKey(pressKey("a"), state)
	m = opened.model.(globalSSHModel)
	confirmed := m.handleKey(pressKey("enter"), state)
	if len(confirmed.actions) != 0 {
		t.Fatalf("confirmation emitted %d actions, want 0 while the ceremony is in flight", len(confirmed.actions))
	}
	if confirmed.cmd == nil {
		t.Fatal("confirmation must dispatch the async CommitGlobalSSH command")
	}
	if m := confirmed.model.(globalSSHModel); !m.ceremony.pending || m.ceremony.done {
		t.Fatalf("ceremony after confirm = pending:%t done:%t, want in-flight async state", m.ceremony.pending, m.ceremony.done)
	}
	// Keys are inert while pending.
	idle := confirmed.model.(globalSSHModel).handleKey(pressKey("enter"), state)
	if len(idle.actions) != 0 {
		t.Error("a plain key while pending must not emit an apply action")
	}
	// The commit's explicit success is what reaches the receipt + applies.
	msg := confirmed.cmd().(GlobalSSHCommitMsg)
	success := confirmed.model.(globalSSHModel).handleMsg(msg, state)
	if len(success.actions) != 1 {
		t.Fatalf("successful commit delivered %d actions, want one ApplySSH", len(success.actions))
	}
	if _, isApply := success.actions[0].(ApplySSH); !isApply {
		t.Fatalf("action = %T, want ApplySSH", success.actions[0])
	}
	if m := success.model.(globalSSHModel); !m.ceremony.done {
		t.Error("the ceremony must reach the receipt after an explicit success")
	}
}

// TestGlobalSSHApplyFailureRendersRetryNoReceiptNoApply pins the failed-commit
// path: the error renders with Retry/Cancel, the receipt is NOT shown, and no
// ApplySSH action was emitted.
func TestGlobalSSHApplyFailureRendersRetryNoReceiptNoApply(t *testing.T) {
	b := &stubBackend{sshCommitMsg: GlobalSSHCommitMsg{Err: "disk on fire"}}
	a := NewApp(b)
	a, _ = press(t, a, "2")
	a, _ = press(t, a, "a")
	a, cmd := press(t, a, "enter") // confirm
	msg := cmd().(GlobalSSHCommitMsg)
	model, _ := a.Update(msg)
	a = model.(App)

	view := appView(a)
	if !strings.Contains(view, "disk on fire") {
		t.Errorf("the concrete commit error must render, got:\n%s", view)
	}
	if !strings.Contains(view, "Retry (Enter)") || !strings.Contains(view, "Cancel (Esc)") {
		t.Errorf("the failed ceremony must offer retry and cancel:\n%s", view)
	}
	if strings.Contains(view, "recommended options applied") {
		t.Error("the receipt must NOT render after a failed commit")
	}
	if len(a.state.SSHApplied) != 0 {
		t.Error("no ApplySSH action may be emitted after a failed commit")
	}
}
