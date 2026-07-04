package dummytui

import (
	"strings"
	"testing"
)

// gssApp returns an App on the Global SSH tab.
func gssApp(t *testing.T) App {
	t.Helper()
	a, _ := press(t, NewApp(), "2")
	return a
}

// gssModel extracts the Global SSH child model.
func gssModel(t *testing.T, a App) globalSSHModel {
	t.Helper()
	m, ok := a.screens[tabGlobalSSH].(globalSSHModel)
	if !ok {
		t.Fatalf("screens[1] is %T, want globalSSHModel", a.screens[tabGlobalSSH])
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
	if !strings.Contains(regionFlat(a, 44, 100), "This is advisory, never a compliance gate.") {
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
	keys := m.applyChosen(overlaidOptions(a.state))
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

	a, _ = press(t, a, "enter") // confirm
	if !strings.Contains(appView(a), "3 of 4 recommended options applied to Host *.") {
		t.Error("result message missing")
	}
	a, _ = press(t, a, "enter") // done → dispatch
	if len(a.state.SSHApplied) != 3 {
		t.Fatalf("SSHApplied = %v", a.state.SSHApplied)
	}

	// Applied overlay: an applied key's detail renders "Applied by
	// gitid — <one-liner>" (IdentitiesOnly always shows the deep-dive, so
	// move the selection up to HashKnownHosts).
	a, _ = press(t, a, "up")
	if !strings.Contains(regionFlat(a, 44, 100), "Applied by gitid — Hashing known_hosts") {
		t.Error("applied keys must render the Applied-by-gitid overlay one-liner")
	}
	// ForwardAgent is still pending.
	if got := pendingOptions(overlaidOptions(a.state)); len(got) != 1 || got[0].Key != "ForwardAgent" {
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
	if !strings.Contains(regionFlat(a, 0, 45), "Sentinel blocks in ~/.ssh/config (default) — current") {
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

func TestGlobalSSHSpaceToggleIsCopyOnWrite(t *testing.T) {
	m := newGlobalSSHModel()
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
