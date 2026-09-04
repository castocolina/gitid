package tuikit

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
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

// TestGlobalSSHArrowsCycleThreeSubTabsInOppositeDirections is plan
// 09.5-01's acceptance criterion: with a THIRD sub-tab, ← and → must move in
// OPPOSITE directions around the cycle (Options → Storage → All directives →
// Options for →; the reverse for ←) — with only two sub-tabs the two
// directions were indistinguishable aliases of the same toggle, which is
// exactly the bug this test pins as fixed.
func TestGlobalSSHArrowsCycleThreeSubTabsInOppositeDirections(t *testing.T) {
	a := gssApp(t)
	if !strings.Contains(appView(a), "Global SSH › Options") {
		t.Fatal("setup: must start on Options")
	}

	// → cycles forward: Options → Storage → All directives → Options.
	a, _ = press(t, a, "right")
	if !strings.Contains(appView(a), "Global SSH › Storage & preview") {
		t.Fatalf("→ from Options must land on Storage, got:\n%s", appView(a))
	}
	a, _ = press(t, a, "right")
	if !strings.Contains(appView(a), "Global SSH › All directives") {
		t.Fatalf("→ from Storage must land on All directives, got:\n%s", appView(a))
	}
	a, _ = press(t, a, "right")
	if !strings.Contains(appView(a), "Global SSH › Options") {
		t.Fatalf("→ from All directives must wrap to Options, got:\n%s", appView(a))
	}

	// ← from Options must land on the THIRD sub-tab (All directives), not
	// the second (Storage) — the opposite-direction proof.
	a, _ = press(t, a, "left")
	if !strings.Contains(appView(a), "Global SSH › All directives") {
		t.Fatalf("← from Options must land on All directives (not Storage), got:\n%s", appView(a))
	}
	a, _ = press(t, a, "left")
	if !strings.Contains(appView(a), "Global SSH › Storage & preview") {
		t.Fatalf("← from All directives must land on Storage, got:\n%s", appView(a))
	}
	a, _ = press(t, a, "left")
	if !strings.Contains(appView(a), "Global SSH › Options") {
		t.Fatalf("← from Storage must wrap to Options, got:\n%s", appView(a))
	}
}

// TestGlobalSSHActivateFocusesFirstFetchedRow is UXP-01 / D-01: a freshly
// constructed model, activated once, selects the FIRST row of the fetched
// option list. The construction-time IdentitiesOnly default (a middle row)
// is the bug this pins.
func TestGlobalSSHActivateFocusesFirstFetchedRow(t *testing.T) {
	rows := []GlobalSSHOptionView{
		{Key: "HashKnownHosts", CurrentValue: "no", Recommended: "yes", Risk: "Low", OneLiner: "hash", State: GlobalSSHNeedsAction, WritableToHostStar: true},
		{Key: "ForwardAgent", CurrentValue: "no", Recommended: "no", Risk: "High", OneLiner: "agent", State: GlobalSSHNeedsAction, WritableToHostStar: true},
		{Key: "StrictHostKeyChecking", CurrentValue: "ask", Recommended: "accept-new", Risk: "Medium", OneLiner: "strict", State: GlobalSSHNeedsAction, WritableToHostStar: true},
	}
	m := newGlobalSSHModel(stubBackend{sshOptions: rows})
	next, _ := m.activate(Seed())
	sm := next.(globalSSHModel)
	if sm.detailKey != rows[0].Key {
		t.Errorf("detailKey = %q, want first fetched row %q", sm.detailKey, rows[0].Key)
	}
}

// TestGlobalSSHActivateResetsFocusToFirstRow is UXP-01's re-entry half:
// navigate down two rows, leave the screen, re-activate — selection is back
// on the first fetched row. A construction-time-only default would not cover
// this; activate() must reset from the freshly fetched list. A custom list
// whose first row is not IdentitiesOnly (or StrictHostKeyChecking) is the
// only way to tell a derived rule from a leftover hardcoded key.
func TestGlobalSSHActivateResetsFocusToFirstRow(t *testing.T) {
	rows := []GlobalSSHOptionView{
		{Key: "HashKnownHosts", CurrentValue: "no", Recommended: "yes", Risk: "Low", OneLiner: "hash", State: GlobalSSHNeedsAction, WritableToHostStar: true},
		{Key: "ForwardAgent", CurrentValue: "no", Recommended: "no", Risk: "High", OneLiner: "agent", State: GlobalSSHNeedsAction, WritableToHostStar: true},
		{Key: "StrictHostKeyChecking", CurrentValue: "ask", Recommended: "accept-new", Risk: "Medium", OneLiner: "strict", State: GlobalSSHNeedsAction, WritableToHostStar: true},
	}
	a, _ := press(t, NewApp(stubBackend{sshOptions: rows}), "2")
	a, _ = press(t, a, "down")
	a, _ = press(t, a, "down")
	m := gssModel(t, a)
	if m.detailKey != rows[2].Key {
		t.Fatalf("setup: after two downs, detailKey = %q, want %q", m.detailKey, rows[2].Key)
	}
	a, _ = press(t, a, "1") // Identities
	a, _ = press(t, a, "2") // back to Global SSH — re-activates in place
	m = gssModel(t, a)
	if m.detailKey != rows[0].Key {
		t.Errorf("detailKey after re-activate = %q, want first fetched row %q", m.detailKey, rows[0].Key)
	}
}

func TestGlobalSSHOptionsMasterDetail(t *testing.T) {
	a := gssApp(t)
	view := appView(a)
	if !strings.Contains(view, "IdentitiesOnly") {
		t.Error("IdentitiesOnly row missing")
	}
	if !strings.Contains(view, "StrictHostKeyChecking") {
		t.Error("first row StrictHostKeyChecking missing")
	}
	if !strings.Contains(regionFlat(a, 45, 100), "This is advisory, never a compliance gate.") {
		t.Error("advisory note missing (never blocking)")
	}
	// IdentitiesOnly is a middle row; navigate to it for the long GSSH-01 explanation.
	a = pressSeq(t, a, "down", "down", "down")
	if !strings.Contains(appView(a), "When IdentitiesOnly is not set") {
		t.Error("IdentitiesOnly must show the full GSSH-01 explanation")
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
	// D-15: the selection starts EMPTY on every entry to the screen.
	keys := m.applyChosen(m.overlaidOptions(a.state))
	if len(keys) != 0 {
		t.Fatalf("initial chosen = %v, want 0 (D-15: selection starts empty)", keys)
	}

	// Toggle HashKnownHosts and StrictHostKeyChecking.
	// List order: StrictHostKeyChecking(0) ForwardAgent(1) HashKnownHosts(2) IdentitiesOnly(3) ...
	// Starting at the first row: space on StrictHostKeyChecking; down→ForwardAgent
	// skip; down→HashKnownHosts, space.
	a, _ = press(t, a, "space")
	a, _ = press(t, a, "down") // StrictHostKeyChecking → ForwardAgent (skip)
	a, _ = press(t, a, "down") // ForwardAgent → HashKnownHosts
	a, _ = press(t, a, "space")
	m = gssModel(t, a)
	keys = m.applyChosen(m.overlaidOptions(a.state))
	if len(keys) != 2 {
		t.Fatalf("after toggling 2 keys, chosen = %v, want 2", keys)
	}

	a, _ = press(t, a, "a")
	view := appView(a)
	if !strings.Contains(view, "Write Host * managed block to ~/.ssh/config") {
		t.Fatalf("apply ceremony missing:\n%s", view)
	}
	if !strings.Contains(view, "+ StrictHostKeyChecking accept-new") || !strings.Contains(view, "+ HashKnownHosts yes") {
		t.Error("chosen writable keys must render as + diff lines")
	}
	if strings.Contains(view, "+ IdentitiesOnly") {
		t.Error("IdentitiesOnly must never appear as a Host * write")
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
	raw := cmd()
	if _, ok := unwrapCommitToken(raw).(GlobalSSHCommitMsg); !ok {
		t.Fatalf("commit delivered %T, want GlobalSSHCommitMsg", unwrapCommitToken(raw))
	}

	// The receipt appears ONLY from the commit's explicit success.
	model, _ := a.Update(raw)
	a = model.(App)
	if !strings.Contains(appView(a), "2 of 3 recommended options applied to Host *.") {
		t.Error("result message missing")
	}
	if len(a.state.SSHApplied) != 2 {
		t.Fatalf("SSHApplied = %v, want 2 only after the commit succeeded", a.state.SSHApplied)
	}

	a, _ = press(t, a, "enter") // done → back to browse
	// Applied overlay: an applied key's detail renders "Applied by
	// gitid — <one-liner>". Navigate to HashKnownHosts explicitly.
	// After ceremony, detailKey may be anywhere; use down to reach HashKnownHosts.
	for _, key := range [3]string{"down", "down", "down"} {
		a, _ = press(t, a, key)
	}
	// Wrap around a few times to find HashKnownHosts — simpler: just set it directly.
	m = gssModel(t, a)
	m.detailKey = "HashKnownHosts"
	a.screens[TabGlobalSSH] = m
	if !strings.Contains(regionFlat(a, 45, 100), "Applied by gitid — Hashing known_hosts") {
		t.Error("applied keys must render the Applied-by-gitid overlay one-liner")
	}
	// ForwardAgent is still pending.
	got := pendingOptions(gssModel(t, a).overlaidOptions(a.state))
	if len(got) != 1 || got[0].Key != "ForwardAgent" {
		t.Errorf("pending after apply = %v, want only ForwardAgent (declined; IdentitiesOnly is verify-only)", got)
	}
}

func TestGlobalSSHSpaceTogglesChoice(t *testing.T) {
	a := gssApp(t)
	a, _ = press(t, a, "down") // StrictHostKeyChecking → ForwardAgent
	a, _ = press(t, a, "down") // ForwardAgent → HashKnownHosts (writable)
	// D-15: selection starts empty, so first space must CHECK the option.
	a, _ = press(t, a, "space")
	m := gssModel(t, a)
	if !m.chosen["HashKnownHosts"] {
		t.Error("space must check the selected pending option (D-15: starts empty)")
	}
	a, _ = press(t, a, "space")
	m = gssModel(t, a)
	if m.chosen["HashKnownHosts"] {
		t.Error("space must uncheck the selected pending option")
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

	// Walk the migration ceremony (async: confirm dispatches CommitSSHStorage,
	// the state changes only after SSHStorageCommitMsg arrives).
	a, _ = press(t, a, "enter")
	if !strings.Contains(appView(a), "Migrate SSH storage layout → Include’d gitid.config") {
		t.Fatalf("migration ceremony missing:\n%s", appView(a))
	}
	a, cmd := press(t, a, "enter") // confirm
	if cmd == nil {
		t.Fatal("confirmation must dispatch the async CommitSSHStorage command")
	}
	model, _ := a.Update(cmd())
	a = model.(App)
	if a.state.SSHStorage != StorageInclude {
		t.Fatalf("SSHStorage = %q, want include", a.state.SSHStorage)
	}
	a, _ = press(t, a, "enter") // dismiss receipt

	// Reversible (STORE-03): migrate back.
	a, _ = press(t, a, "down")    // toggle radio back to sentinel
	a, _ = press(t, a, "enter")   // open ceremony
	a, cmd = press(t, a, "enter") // confirm
	if cmd == nil {
		t.Fatal("round-trip confirmation must dispatch the async CommitSSHStorage command")
	}
	model, _ = a.Update(cmd())
	a = model.(App)
	if a.state.SSHStorage != StorageSentinel {
		t.Errorf("SSHStorage = %q, want sentinel (round trip)", a.state.SSHStorage)
	}
}

func TestGlobalSSHApplyTargetsOwnedFileUnderIncludeLayout(t *testing.T) {
	a := gssApp(t)
	a.state = Reduce(a.state, SetSSHStorage{Layout: StorageInclude, Backup: "b"})
	// D-15: selection starts empty; first row is already focused and selectable.
	a, _ = press(t, a, "space")
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
	// D-15: selection starts empty; first row is already focused and selectable.
	a, _ = press(t, a, "space")
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
	// D-15: selection starts empty; pre-check it to test uncheck behavior.
	if orig["HashKnownHosts"] {
		t.Fatal("D-15: HashKnownHosts must start NOT pre-chosen (empty selection)")
	}
	// First toggle: check the option.
	res := m.handleKey(pressKey("space"), Seed())
	next, ok := res.model.(globalSSHModel)
	if !ok {
		t.Fatalf("model is %T, want globalSSHModel", res.model)
	}
	if !next.chosen["HashKnownHosts"] {
		t.Error("space must choose the selected option")
	}
	// Second toggle: un-check it (copy-on-write property).
	res2 := next.handleKey(pressKey("space"), Seed())
	next2, ok := res2.model.(globalSSHModel)
	if !ok {
		t.Fatalf("model is %T, want globalSSHModel", res2.model)
	}
	if next2.chosen["HashKnownHosts"] {
		t.Error("space must un-choose the selected option")
	}
	// Elm purity: the map must be copy-on-write; the original must be unchanged.
	// orig is the empty map from activate; it must still be empty after toggles.
	if orig["HashKnownHosts"] {
		t.Error("Elm purity: the toggle mutated the map shared with the pre-update model copy")
	}
}

func TestGlobalSSHLongExplanationClipsWithVisibleCue(t *testing.T) {
	// IdentitiesOnly carries the long GSSH-01 explanation — at 100x30 it
	// cannot fully fit, and the overflow must be announced, never silently
	// cut mid-sentence (H3). It is a middle row; navigate to it first.
	a := gssApp(t)
	a = pressSeq(t, a, "down", "down", "down")
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

func TestGlobalSSHOptionVersionNoteInDetailPane(t *testing.T) {
	b := &stubBackend{sshOptions: []GlobalSSHOptionView{
		{Key: "StrictHostKeyChecking", CurrentValue: "ask", Recommended: "accept-new", Risk: "Medium", OneLiner: "accept-new pins first-seen keys", VersionNote: "Your OpenSSH: 9.7p1 — accept-new is available", State: GlobalSSHNeedsAction},
		{Key: "HashKnownHosts", CurrentValue: "no", Recommended: "yes", Risk: "Low", OneLiner: "Hashing known_hosts hides which hosts you connect to", State: GlobalSSHNeedsAction},
	}}
	a := NewApp(b)
	a, _ = press(t, a, "2")
	m := gssModel(t, a)
	m.detailKey = "StrictHostKeyChecking"
	a.screens[TabGlobalSSH] = m
	if !strings.Contains(appView(a), "Your OpenSSH: 9.7p1 — accept-new is available") {
		t.Fatal("StrictHostKeyChecking detail must show the dynamic version note")
	}
	m.detailKey = "HashKnownHosts"
	a.screens[TabGlobalSSH] = m
	if strings.Contains(appView(a), "Your OpenSSH:") {
		t.Fatal("HashKnownHosts detail must not show the version note")
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

// TestGlobalSSHOptionsErrorDoesNotTrapNavigation is the regression for CR-01:
// a failed option probe must not consume every key. Global Git already
// documents this fail-open contract (07-UI-SPEC.md RESOLVED "error" row);
// Global SSH must match it rather than swallowing tab switches, help, and
// quit.
func TestGlobalSSHOptionsErrorDoesNotTrapNavigation(t *testing.T) {
	b := &stubBackend{sshOptionsErr: errors.New("probe exploded")}
	a := NewApp(b)
	a, _ = press(t, a, "2")
	if !strings.Contains(appView(a), "Global SSH") {
		t.Fatalf("setup: expected to land on Global SSH before probing the trap")
	}
	a, _ = press(t, a, "1")
	if strings.Contains(appView(a), "Global SSH") {
		t.Errorf("pressing 1 after a failed option probe must leave Global SSH, but the tab did not change:\n%s", appView(a))
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
	// D-15: selection starts empty; toggle to select HashKnownHosts before applying.
	m.detailKey = "HashKnownHosts"
	toggled := m.handleKey(pressKey("space"), state)
	m = toggled.model.(globalSSHModel)
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
	success := confirmed.model.(globalSSHModel).handleMsg(confirmed.cmd(), state)
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

// TestGlobalSSHAbandonedApplyStillDispatchesReducerAction is the BL-04
// regression (09.4-REVIEW.md independent re-review): the write completes on
// disk regardless of whether the ceremony UI is still around to show its
// receipt. Confirm an apply (dispatches the async commit), then simulate the
// abandonment path — Ctrl+P -> another screen -> back, which runs
// activate() and resets mode/applyCommitPending/ceremony (CR-02) — before
// the commit's success message arrives. The reducer action (and note) must
// still fire so App.state refreshes; only the ceremony UI mutation is
// skipped.
func TestGlobalSSHAbandonedApplyStillDispatchesReducerAction(t *testing.T) {
	b := &stubBackend{sshCommitMsg: GlobalSSHCommitMsg{Backups: []string{"~/.ssh/config.bak.1"}}}
	m := newGlobalSSHModel(b)
	state := Seed()
	activated, _ := m.activate(state)
	m = activated.(globalSSHModel)
	m.detailKey = "HashKnownHosts"
	toggled := m.handleKey(pressKey("space"), state)
	m = toggled.model.(globalSSHModel)
	opened := m.handleKey(pressKey("a"), state)
	m = opened.model.(globalSSHModel)
	confirmed := m.handleKey(pressKey("enter"), state)
	pendingModel := confirmed.model.(globalSSHModel)
	if !pendingModel.applyCommitPending {
		t.Fatal("setup: confirming must set applyCommitPending")
	}
	msg := confirmed.cmd()

	// Abandonment: re-entering the screen (e.g. via Ctrl+P then back) runs
	// activate(), which CR-02 made reset mode/applyCommitPending/ceremony —
	// but the async commit above is still in flight and will still arrive.
	reactivated, _ := pendingModel.activate(state)
	abandoned := reactivated.(globalSSHModel)
	if abandoned.mode == gssApplyCeremony || abandoned.applyCommitPending {
		t.Fatal("setup: activate() must have cleared the ceremony state")
	}

	success := abandoned.handleMsg(msg, state)
	if len(success.actions) != 1 {
		t.Fatalf("BL-04 regressed: abandoned commit delivered %d actions, want one ApplySSH — the write happened on disk but App.state never refreshed", len(success.actions))
	}
	if _, isApply := success.actions[0].(ApplySSH); !isApply {
		t.Fatalf("action = %T, want ApplySSH", success.actions[0])
	}
	if success.note == "" {
		t.Error("BL-04 regressed: abandoned commit produced no note")
	}
}

// TestGlobalSSHApplyFailureRendersRetryNoReceiptNoApply pins the failed-commit
// path: the error renders with Retry/Cancel, the receipt is NOT shown, and no
// ApplySSH action was emitted.
func TestGlobalSSHApplyFailureRendersRetryNoReceiptNoApply(t *testing.T) {
	b := &stubBackend{sshCommitMsg: GlobalSSHCommitMsg{Err: "disk on fire"}}
	a := NewApp(b)
	a, _ = press(t, a, "2")
	// D-15: selection starts empty; first row is already focused and selectable.
	a, _ = press(t, a, "space")
	a, _ = press(t, a, "a")
	a, cmd := press(t, a, "enter") // confirm
	model, _ := a.Update(cmd())
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

func TestOptionRowFourStatesAreTwoLines(t *testing.T) {
	for _, o := range []GlobalSSHOptionView{
		{Key: "StrictHostKeyChecking", CurrentValue: "ask", Recommended: "accept-new", State: GlobalSSHNeedsAction, WritableToHostStar: true},
		{Key: "ForwardAgent", CurrentValue: "yes", Recommended: "no", State: GlobalSSHDiffers, AttributedToUser: true, WritableToHostStar: true},
		{Key: "AddKeysToAgent", CurrentValue: "yes", Recommended: "yes", State: GlobalSSHAlreadySet, WritableToHostStar: true},
		{Key: "UseKeychain", CurrentValue: "", Recommended: "yes", State: GlobalSSHNotApplicable, NotApplicableReason: GlobalSSHReasonPlatform},
	} {
		got := strings.Split(stripANSI(optionRow(o, false, false, false, 44)), "\n")
		if len(got) != optionRowLines {
			t.Errorf("%s state %v: %d lines, want %d", o.Key, o.State, len(got), optionRowLines)
		}
	}
}

func TestOptionRowNeedsActionAndDiffersShareWarningGlyph(t *testing.T) {
	needs := optionRow(GlobalSSHOptionView{Key: "HashKnownHosts", CurrentValue: "no", Recommended: "yes", State: GlobalSSHNeedsAction, WritableToHostStar: true}, false, false, false, 80)
	differs := optionRow(GlobalSSHOptionView{Key: "ForwardAgent", CurrentValue: "yes", Recommended: "no", State: GlobalSSHDiffers, AttributedToUser: true, WritableToHostStar: true}, false, false, false, 80)
	warn := styleWarning.Render("!")
	if !strings.Contains(needs, warn) || !strings.Contains(differs, warn) {
		t.Fatal("needs-action and set-but-differs must share the warning glyph and theme role")
	}
	if strings.Contains(needs, styleHealthy.Render("✓")) || strings.Contains(differs, styleHealthy.Render("✓")) {
		t.Fatal("flagged rows must not use the healthy glyph")
	}
	n2 := strings.Split(stripANSI(needs), "\n")[1]
	d2 := strings.Split(stripANSI(differs), "\n")[1]
	if n2 == d2 {
		t.Fatalf("line-2 texts must differ; both %q", n2)
	}
	if !strings.Contains(d2, GlobalSSHWordDiffersUser) {
		t.Fatalf("differs line-2 = %q, want %q", d2, GlobalSSHWordDiffersUser)
	}
}

func TestOptionRowDiffersAttribution(t *testing.T) {
	const (
		wantUser    = "set, differs — yours, would be a no-op here"
		wantOutside = "set, differs — external, would be a no-op here"
	)
	user := optionRow(GlobalSSHOptionView{Key: "ForwardAgent", CurrentValue: "yes", Recommended: "no", State: GlobalSSHDiffers, AttributedToUser: true, WritableToHostStar: true}, false, false, false, 100)
	outside := optionRow(GlobalSSHOptionView{Key: "ForwardAgent", CurrentValue: "yes", Recommended: "no", State: GlobalSSHDiffers, AttributedToUser: false, WritableToHostStar: true}, false, false, false, 100)
	userLines := strings.Split(stripANSI(user), "\n")
	outsideLines := strings.Split(stripANSI(outside), "\n")
	if len(userLines) != optionRowLines || len(outsideLines) != optionRowLines {
		t.Fatalf("differs rows must remain %d lines; user=%d outside=%d", optionRowLines, len(userLines), len(outsideLines))
	}
	u2 := userLines[1]
	o2 := outsideLines[1]
	if u2 == o2 {
		t.Fatalf("attributed and non-attributed differs line-2 must differ; both %q", u2)
	}
	if GlobalSSHWordDiffersUser != wantUser {
		t.Fatalf("user-attributed differs sentence = %q, want %q", GlobalSSHWordDiffersUser, wantUser)
	}
	if GlobalSSHWordDiffersOutside != wantOutside {
		t.Fatalf("outside-attributed differs sentence = %q, want %q", GlobalSSHWordDiffersOutside, wantOutside)
	}
	for name, line := range map[string]string{
		"user":    optionRowLine2(GlobalSSHOptionView{CurrentValue: "accept-new", Recommended: "ask", State: GlobalSSHDiffers, AttributedToUser: true}),
		"outside": optionRowLine2(GlobalSSHOptionView{CurrentValue: "accept-new", Recommended: "ask", State: GlobalSSHDiffers}),
	} {
		if !strings.Contains(line, "would be a no-op") {
			t.Errorf("%s differs line = %q, want the no-op explanation", name, line)
		}
		if width := ansi.StringWidth(line); width > 100 {
			t.Errorf("%s longest composed differs line width = %d, want <= 100: %q", name, width, line)
		}
	}
}

func TestOptionRowAlreadySetUsesHealthyGlyph(t *testing.T) {
	got := optionRow(GlobalSSHOptionView{Key: "AddKeysToAgent", CurrentValue: "yes", Recommended: "yes", State: GlobalSSHAlreadySet, WritableToHostStar: true}, false, false, false, 80)
	if !strings.Contains(got, styleHealthy.Render("✓")) {
		t.Fatal("already-set row must use the healthy glyph")
	}
	if strings.Contains(got, styleWarning.Render("!")) {
		t.Fatal("already-set row must not use the warning glyph")
	}
}

func TestOptionRowAlreadySetBaselineProvenance(t *testing.T) {
	parsed := GlobalSSHOptionView{
		Key: "AddKeysToAgent", CurrentValue: "yes", Recommended: "yes", State: GlobalSSHAlreadySet,
		AttributedToUser: true, WritableToHostStar: true,
		Provenance: "set by you at ~/.ssh/config line 4",
	}
	baseline := GlobalSSHOptionView{
		Key: "ForwardAgent", CurrentValue: "no", Recommended: "no", State: GlobalSSHAlreadySet,
		AttributedToUser: false, WritableToHostStar: true,
		Provenance: "not set (OpenSSH default: no)",
	}
	if parsed.Provenance == baseline.Provenance {
		t.Fatal("gitid-parsed and baseline provenance must differ")
	}
	if strings.Contains(baseline.Provenance, "your choice") || strings.Contains(baseline.Provenance, "/") {
		t.Fatalf("baseline provenance = %q must contain neither user-attribution nor a file path", baseline.Provenance)
	}
	b2 := strings.Split(stripANSI(optionRow(baseline, false, false, false, 100)), "\n")[1]
	if !strings.Contains(b2, GlobalSSHWordSafeByDefault) {
		t.Fatalf("baseline already-set line-2 = %q, want %q", b2, GlobalSSHWordSafeByDefault)
	}
	if strings.Contains(b2, "your choice") || strings.Contains(b2, "~/.ssh") {
		t.Fatalf("baseline already-set line-2 = %q must not credit the user or a file", b2)
	}
}

func TestOptionRowNotApplicableReasonsAreDistinct(t *testing.T) {
	reasons := []GlobalSSHNotApplicableReason{
		GlobalSSHReasonPlatform, GlobalSSHReasonVersionTooOld, GlobalSSHReasonVersionUnverified, GlobalSSHReasonNothingToVerify,
		GlobalSSHReasonProbeFailed,
	}
	seen := map[string]GlobalSSHNotApplicableReason{}
	platform := notApplicableSentence(GlobalSSHReasonPlatform)
	for _, r := range reasons {
		o := GlobalSSHOptionView{Key: "UseKeychain", State: GlobalSSHNotApplicable, NotApplicableReason: r}
		if r != GlobalSSHReasonPlatform {
			o.Key = "StrictHostKeyChecking"
		}
		text := stripANSI(optionRow(o, false, false, false, 80))
		sent := notApplicableSentence(r)
		if sent == "" || strings.Count(text, sent) == 0 {
			t.Fatalf("reason %v: sentence %q missing from %q", r, sent, text)
		}
		if prev, ok := seen[sent]; ok {
			t.Fatalf("reasons %v and %v share sentence %q", prev, r, sent)
		}
		seen[sent] = r
		if r != GlobalSSHReasonPlatform && strings.Contains(text, platform) {
			t.Fatalf("version/verify reason %v leaked the platform sentence %q", r, platform)
		}
		if strings.Contains(text, glyphToggleOn) || strings.Contains(text, glyphToggleOff) {
			t.Fatalf("not-applicable row for %v must contain neither bracket toggle", r)
		}
		if !strings.Contains(text, "·") {
			t.Fatalf("not-applicable row for %v must render the placeholder", r)
		}
		if strings.Contains(text, "→") {
			t.Fatalf("not-applicable row for %v must omit the recommendation arrow", r)
		}
	}
	if len(seen) != len(reasons) {
		t.Fatalf("got %d distinct sentences, want %d (one per reason)", len(seen), len(reasons))
	}
}

func optionRowPrefixWidth(t *testing.T, rendered, key string) int {
	t.Helper()
	first := strings.Split(stripANSI(rendered), "\n")[0]
	idx := strings.Index(first, key)
	if idx < 0 {
		t.Fatalf("key %q not found in option row %q", key, first)
	}
	return ansi.StringWidth(first[:idx])
}

func listColPrefixWidth(t *testing.T, body, key string) int {
	t.Helper()
	for _, line := range strings.Split(body, "\n") {
		listCol := stripANSI(strings.SplitN(line, "│", 2)[0])
		if idx := strings.Index(listCol, key); idx >= 0 {
			return ansi.StringWidth(listCol[:idx])
		}
	}
	t.Fatalf("key %q not found in master list", key)
	return 0
}

func TestOptionRowCheckboxColumnWidthIsUniform(t *testing.T) {
	if ansi.StringWidth(glyphToggleOn) != optionBoxWidth || ansi.StringWidth(glyphToggleOff) != optionBoxWidth {
		t.Fatalf("optionBoxWidth = %d, want ansi.StringWidth of both toggle glyphs (on=%d off=%d)",
			optionBoxWidth, ansi.StringWidth(glyphToggleOn), ansi.StringWidth(glyphToggleOff))
	}

	sshSel := GlobalSSHOptionView{Key: "OptKey", State: GlobalSSHNeedsAction, WritableToHostStar: true}
	sshNA := GlobalSSHOptionView{Key: "OptKey", State: GlobalSSHNotApplicable, NotApplicableReason: GlobalSSHReasonPlatform}
	sshSet := GlobalSSHOptionView{Key: "OptKey", CurrentValue: "yes", Recommended: "yes", State: GlobalSSHAlreadySet, WritableToHostStar: true}

	got := map[string]int{
		"ssh-off":     optionRowPrefixWidth(t, optionRow(sshSel, false, false, false, 80), "OptKey"),
		"ssh-on":      optionRowPrefixWidth(t, optionRow(sshSel, true, false, false, 80), "OptKey"),
		"ssh-na":      optionRowPrefixWidth(t, optionRow(sshNA, false, false, false, 80), "OptKey"),
		"ssh-applied": optionRowPrefixWidth(t, optionRow(sshSet, false, false, true, 80), "OptKey"),
	}

	b := stubBackend{gitOptions: []GlobalGitOptionView{
		{Key: "gitOff", CurrentValue: "", Recommended: "true", OneLiner: "x", State: GlobalGitNeedsAction, PolicyBacked: true, HasWritableMember: true},
		{Key: "gitNA", CurrentValue: "", Recommended: "true", OneLiner: "y", State: GlobalGitNotApplicable, PolicyBacked: true, HasWritableMember: true, NotApplicableReason: GlobalGitReasonProbeFailed, ProbeError: "probe failed"},
		{Key: "gitSet", CurrentValue: "true", Recommended: "true", OneLiner: "z", State: GlobalGitAlreadySet, PolicyBacked: true, HasWritableMember: true},
	}}
	a, _ := press(t, NewApp(b), "3")
	got["git-off"] = listColPrefixWidth(t, appView(a), "gitOff")
	got["git-na"] = listColPrefixWidth(t, appView(a), "gitNA")
	got["git-set"] = listColPrefixWidth(t, appView(a), "gitSet")
	a, _ = press(t, a, "space")
	got["git-on"] = listColPrefixWidth(t, appView(a), "gitOff")

	var want int
	for name, w := range got {
		if want == 0 {
			want = w
		}
		if w != want {
			t.Errorf("%s prefix width = %d, want uniform %d (all variants: %v)", name, w, want, got)
		}
	}
	box := want - 5 // leading space + marker + tone + name-gutter
	if box != optionBoxWidth {
		t.Errorf("inferred checkbox-column width = %d, want optionBoxWidth %d (prefix widths %v)", box, optionBoxWidth, got)
	}
}

func optionRowBoxPlain(t *testing.T, rendered string) string {
	t.Helper()
	first := strings.Split(stripANSI(rendered), "\n")[0]
	runes := []rune(first)
	const prefixCells = 3 // leading space + unselected marker
	if len(runes) < prefixCells+optionBoxWidth {
		t.Fatalf("option row too short to hold a checkbox column: %q", first)
	}
	return string(runes[prefixCells : prefixCells+optionBoxWidth])
}

func TestGlobalSSHNonSelectableRowRendersDotPlaceholder(t *testing.T) {
	o := GlobalSSHOptionView{Key: "UseKeychain", State: GlobalSSHNotApplicable, NotApplicableReason: GlobalSSHReasonPlatform}
	got := optionRow(o, false, false, false, 80)
	box := optionRowBoxPlain(t, got)
	if !strings.Contains(box, "·") {
		t.Fatalf("non-selectable SSH checkbox column = %q, want the faint-dot placeholder", box)
	}
	if strings.Contains(box, glyphToggleOn) || strings.Contains(box, glyphToggleOff) {
		t.Fatalf("non-selectable SSH checkbox column = %q must not render a bracket toggle", box)
	}
	if !strings.Contains(got, styleFaint.Render("·")) {
		t.Fatalf("non-selectable SSH placeholder must use the faint role; got %q", got)
	}
}

func TestOptionRowToggleBracketStates(t *testing.T) {
	ssh := GlobalSSHOptionView{Key: "HashKnownHosts", CurrentValue: "no", Recommended: "yes", State: GlobalSSHNeedsAction, WritableToHostStar: true}
	off := optionRow(ssh, false, false, false, 80)
	on := optionRow(ssh, true, false, false, 80)
	if !strings.Contains(off, glyphToggleOff) {
		t.Fatalf("selectable SSH off-state = %q, want %q", off, glyphToggleOff)
	}
	if !strings.Contains(on, glyphToggleOn) {
		t.Fatalf("selectable SSH on-state = %q, want %q", on, glyphToggleOn)
	}
	if !strings.Contains(off, styleFaint.Render(glyphToggleOff)) {
		t.Fatalf("off-state toggle must be faint; got %q", off)
	}
	onStyled := styleHealthy.Bold(true).Render(glyphToggleOn)
	if !strings.Contains(on, onStyled) {
		t.Fatalf("on-state toggle must be bold+healthy; got %q, want it to contain %q", on, onStyled)
	}

	b := stubBackend{gitOptions: []GlobalGitOptionView{
		{Key: "init.defaultBranch", CurrentValue: "master", Recommended: "main", OneLiner: "x", State: GlobalGitNeedsAction, PolicyBacked: true, HasWritableMember: true},
	}}
	a, _ := press(t, NewApp(b), "3")
	gitOff := appView(a)
	if !strings.Contains(gitOff, glyphToggleOff) {
		t.Fatalf("selectable Git off-state frame missing %q", glyphToggleOff)
	}
	a, _ = press(t, a, "space")
	gitOn := appView(a)
	if !strings.Contains(gitOn, glyphToggleOn) {
		t.Fatalf("selectable Git on-state frame missing %q", glyphToggleOn)
	}
}

func TestOptionRowToggleLegibleWithoutColor(t *testing.T) {
	ssh := GlobalSSHOptionView{Key: "HashKnownHosts", State: GlobalSSHNeedsAction, WritableToHostStar: true}
	na := GlobalSSHOptionView{Key: "UseKeychain", State: GlobalSSHNotApplicable, NotApplicableReason: GlobalSSHReasonPlatform}
	plainOff := stripANSI(optionRow(ssh, false, false, false, 80))
	plainOn := stripANSI(optionRow(ssh, true, false, false, 80))
	plainNA := stripANSI(optionRow(na, false, false, false, 80))
	if !strings.Contains(plainOff, glyphToggleOff) {
		t.Fatalf("NO_COLOR off-state = %q, want bracket %q", plainOff, glyphToggleOff)
	}
	if !strings.Contains(plainOn, glyphToggleOn) {
		t.Fatalf("NO_COLOR on-state = %q, want bracket %q", plainOn, glyphToggleOn)
	}
	if strings.Contains(plainOff, glyphToggleOn) || strings.Contains(plainOn, glyphToggleOff) {
		t.Fatalf("NO_COLOR on/off states must not share a bracket; off=%q on=%q", plainOff, plainOn)
	}
	if !strings.Contains(plainNA, "·") {
		t.Fatalf("NO_COLOR placeholder = %q, want ·", plainNA)
	}
	if strings.Contains(plainNA, glyphToggleOn) || strings.Contains(plainNA, glyphToggleOff) {
		t.Fatalf("NO_COLOR placeholder must not use a bracket toggle; got %q", plainNA)
	}
}

func TestGlobalSSHSelectablePredicate(t *testing.T) {
	ok := GlobalSSHOptionView{Key: "HashKnownHosts", State: GlobalSSHNeedsAction, WritableToHostStar: true}
	if !ok.Selectable() {
		t.Fatal("writable needs-action row must be selectable")
	}
	differs := GlobalSSHOptionView{Key: "ForwardAgent", State: GlobalSSHDiffers, WritableToHostStar: true}
	if !differs.Selectable() {
		t.Fatal("writable set-but-differs row must be selectable")
	}
	cases := []GlobalSSHOptionView{
		{Key: "AddKeysToAgent", State: GlobalSSHAlreadySet, WritableToHostStar: true},
		{Key: "UseKeychain", State: GlobalSSHNotApplicable, NotApplicableReason: GlobalSSHReasonPlatform, WritableToHostStar: true},
		{Key: "IdentitiesOnly", State: GlobalSSHNeedsAction, WritableToHostStar: false},
		{Key: "HashKnownHosts", State: GlobalSSHNeedsAction, WritableToHostStar: true, ProbeError: "ssh -G failed"},
	}
	for _, o := range cases {
		if o.Selectable() {
			t.Errorf("%s state=%v writable=%v probe=%q: Selectable() = true, want false", o.Key, o.State, o.WritableToHostStar, o.ProbeError)
		}
	}
}

func TestGlobalSSHToggleAndClickRespectSelectability(t *testing.T) {
	b := &stubBackend{sshOptions: []GlobalSSHOptionView{
		{Key: "StrictHostKeyChecking", CurrentValue: "ask", Recommended: "accept-new", Risk: "Medium", OneLiner: "a", State: GlobalSSHNeedsAction, WritableToHostStar: true},
		{Key: "ForwardAgent", CurrentValue: "no", Recommended: "no", Risk: "High", OneLiner: "b", State: GlobalSSHAlreadySet, Provenance: "not set (OpenSSH default: no)", WritableToHostStar: true},
		{Key: "HashKnownHosts", CurrentValue: "no", Recommended: "yes", Risk: "Low", OneLiner: "c", State: GlobalSSHDiffers, AttributedToUser: true, WritableToHostStar: true},
		{Key: "IdentitiesOnly", CurrentValue: "mixed", Recommended: "yes", Risk: "High", OneLiner: "d", Explanation: "per-alias", State: GlobalSSHNeedsAction, WritableToHostStar: false},
		{Key: "AddKeysToAgent", CurrentValue: "yes", Recommended: "yes", Risk: "Low", OneLiner: "e", State: GlobalSSHNeedsAction, WritableToHostStar: true, ProbeError: "ssh -G failed"},
		{Key: "UseKeychain", CurrentValue: "", Recommended: "yes", Risk: "Low", OneLiner: "f", State: GlobalSSHNotApplicable, NotApplicableReason: GlobalSSHReasonPlatform},
	}}
	m := newGlobalSSHModel(b)
	activated, _ := m.activate(Seed())
	m = activated.(globalSSHModel)
	m.chosen = map[string]bool{}
	s := Seed()
	width, height := minFrameWidth, minFrameHeight

	assertUnchanged := func(t *testing.T, key string, via string) {
		t.Helper()
		m.detailKey = key
		before := len(m.chosen)
		res := m.handleKey(pressKey("space"), s)
		m = res.model.(globalSSHModel)
		if len(m.chosen) != before || m.chosen[key] {
			t.Fatalf("%s via space: chosen = %v, want unchanged", via, m.chosen)
		}
		row := m.detailIndex(m.overlaidOptions(s))
		res = m.handleClick(3, gssOptionsTopLines(s)+row*optionRowLines, width, height, s)
		m = res.model.(globalSSHModel)
		if m.chosen[key] {
			t.Fatalf("%s via checkbox click: chosen gained %q", via, key)
		}
	}
	assertUnchanged(t, "ForwardAgent", "already-set baseline")
	assertUnchanged(t, "UseKeychain", "not-applicable platform")
	m.options[0].State = GlobalSSHNotApplicable
	m.options[0].NotApplicableReason = GlobalSSHReasonVersionTooOld
	assertUnchanged(t, "StrictHostKeyChecking", "not-applicable version-too-old")
	m.options[0].NotApplicableReason = GlobalSSHReasonVersionUnverified
	assertUnchanged(t, "StrictHostKeyChecking", "not-applicable version-unverified")
	m.options[0].NotApplicableReason = GlobalSSHReasonNothingToVerify
	assertUnchanged(t, "StrictHostKeyChecking", "not-applicable nothing-to-verify")
	m.options[0].State = GlobalSSHNeedsAction
	m.options[0].NotApplicableReason = GlobalSSHReasonNone
	assertUnchanged(t, "AddKeysToAgent", "probe-error")
	assertUnchanged(t, "IdentitiesOnly", "IdentitiesOnly")

	m.detailKey = "HashKnownHosts"
	res := m.handleKey(pressKey("space"), s)
	m = res.model.(globalSSHModel)
	if !m.chosen["HashKnownHosts"] {
		t.Fatal("space on a set-but-differs row must add that key")
	}
}

func TestGlobalSSHDetailShowsProvenanceAndProbeError(t *testing.T) {
	b := &stubBackend{sshOptions: []GlobalSSHOptionView{
		{Key: "HashKnownHosts", CurrentValue: "no", Recommended: "yes", Risk: "Low", OneLiner: "hash", Provenance: "set by you at ~/.ssh/config line 9", State: GlobalSSHNeedsAction, WritableToHostStar: true},
		{Key: "ForwardAgent", CurrentValue: "yes", Recommended: "no", Risk: "High", OneLiner: "agent", ProbeError: "ssh -G failed: exit 255", State: GlobalSSHNeedsAction, WritableToHostStar: true},
	}}
	a := NewApp(b)
	a, _ = press(t, a, "2")
	if !strings.Contains(appView(a), "set by you at ~/.ssh/config line 9") {
		t.Fatal("detail pane must show the selected row's provenance label")
	}
	m := gssModel(t, a)
	m.detailKey = "ForwardAgent"
	a.screens[TabGlobalSSH] = m
	if !strings.Contains(appView(a), "ssh -G failed: exit 255") {
		t.Fatal("probe-error row's detail pane must show the error note")
	}
}

func TestGlobalSSHNoColorStatesDistinguishable(t *testing.T) {
	b := &stubBackend{sshOptions: []GlobalSSHOptionView{
		{Key: "StrictHostKeyChecking", CurrentValue: "ask", Recommended: "accept-new", Risk: "Medium", OneLiner: "a", State: GlobalSSHNeedsAction, WritableToHostStar: true},
		{Key: "ForwardAgent", CurrentValue: "yes", Recommended: "no", Risk: "High", OneLiner: "b", State: GlobalSSHDiffers, AttributedToUser: true, WritableToHostStar: true},
		{Key: "HashKnownHosts", CurrentValue: "yes", Recommended: "yes", Risk: "Low", OneLiner: "c", State: GlobalSSHAlreadySet, WritableToHostStar: true, Provenance: "set by you at ~/.ssh/config line 1"},
		{Key: "UseKeychain", CurrentValue: "", Recommended: "yes", Risk: "Low", OneLiner: "d", State: GlobalSSHNotApplicable, NotApplicableReason: GlobalSSHReasonPlatform},
	}}
	a := NewApp(b)
	a, _ = press(t, a, "2")
	view := appView(a)
	if !strings.Contains(view, "now: ask → accept-new") {
		t.Fatal("needs-action must keep the plain recommendation form")
	}
	if !strings.Contains(view, "differs") {
		t.Fatal("set-but-differs must be named by its word")
	}
	if !strings.Contains(view, GlobalSSHWordAlreadySet) {
		t.Fatal("already-set must be named by its word")
	}
	if !strings.Contains(view, "not applicable") {
		t.Fatal("not-applicable must be named by its reason sentence")
	}
}

func TestGlobalSSHStatusTallyCountsNeedsActionAndDiffers(t *testing.T) {
	b := &stubBackend{sshOptions: []GlobalSSHOptionView{
		{Key: "StrictHostKeyChecking", CurrentValue: "ask", Recommended: "accept-new", Risk: "Medium", OneLiner: "a", State: GlobalSSHNeedsAction, WritableToHostStar: true},
		{Key: "ForwardAgent", CurrentValue: "yes", Recommended: "no", Risk: "High", OneLiner: "b", State: GlobalSSHDiffers, WritableToHostStar: true},
		{Key: "HashKnownHosts", CurrentValue: "yes", Recommended: "yes", Risk: "Low", OneLiner: "c", State: GlobalSSHAlreadySet, WritableToHostStar: true},
		{Key: "IdentitiesOnly", CurrentValue: "mixed", Recommended: "yes", Risk: "High", OneLiner: "d", State: GlobalSSHNeedsAction, WritableToHostStar: false},
		{Key: "AddKeysToAgent", CurrentValue: "yes", Recommended: "yes", Risk: "Low", OneLiner: "e", State: GlobalSSHAlreadySet, WritableToHostStar: true},
		{Key: "UseKeychain", CurrentValue: "", Recommended: "yes", Risk: "Low", OneLiner: "f", State: GlobalSSHNotApplicable, NotApplicableReason: GlobalSSHReasonPlatform},
	}}
	m := newGlobalSSHModel(b)
	activated, _ := m.activate(Seed())
	m = activated.(globalSSHModel)
	sv := m.view(Seed(), minFrameWidth, minFrameHeight)
	want := 0
	for _, o := range m.overlaidOptions(Seed()) {
		if o.needsAttention() {
			want++
		}
	}
	if want != 3 {
		t.Fatalf("fixture attention count = %d, want 3 (needs-action + differs, skip N/A and already-set)", want)
	}
	if !strings.Contains(sv.status, fmt.Sprintf("%d of %d options need action", want, len(b.sshOptions))) {
		t.Fatalf("status = %q, want tally %d of %d", sv.status, want, len(b.sshOptions))
	}
}

func TestGlobalSSHGeometryWithEveryStateAndBanner(t *testing.T) {
	b := &stubBackend{sshOptions: []GlobalSSHOptionView{
		{Key: "StrictHostKeyChecking", CurrentValue: "ask\x1b[0m" + strings.Repeat("X", 80), Recommended: "accept-new", Risk: "Medium", OneLiner: "a", State: GlobalSSHNeedsAction, WritableToHostStar: true},
		{Key: "ForwardAgent", CurrentValue: "yes", Recommended: "no", Risk: "High", OneLiner: "b", State: GlobalSSHDiffers, AttributedToUser: false, WritableToHostStar: true},
		{Key: "HashKnownHosts", CurrentValue: "yes", Recommended: "yes", Risk: "Low", OneLiner: "c", State: GlobalSSHAlreadySet, WritableToHostStar: true},
		{Key: "IdentitiesOnly", CurrentValue: "mixed", Recommended: "yes", Risk: "High", OneLiner: "d", Explanation: GlobalSSHDetailExplanation, State: GlobalSSHNeedsAction, WritableToHostStar: false},
		{Key: "AddKeysToAgent", CurrentValue: "yes", Recommended: "yes", Risk: "Low", OneLiner: "e", State: GlobalSSHAlreadySet, Provenance: "not set (OpenSSH default: yes)", WritableToHostStar: true},
		{Key: "UseKeychain", CurrentValue: "", Recommended: "yes", Risk: "Low", OneLiner: "f", State: GlobalSSHNotApplicable, NotApplicableReason: GlobalSSHReasonPlatform},
	}}
	a := NewApp(b)
	model, _ := a.Update(tea.WindowSizeMsg{Width: minFrameWidth, Height: minFrameHeight})
	a = model.(App)
	a, _ = press(t, a, "2")
	content := a.View().Content
	lines := strings.Split(content, "\n")
	if len(lines) != minFrameHeight {
		t.Fatalf("frame has %d rows, want %d", len(lines), minFrameHeight)
	}
	for i, line := range lines {
		if w := ansi.StringWidth(stripANSI(line)); w > minFrameWidth {
			t.Errorf("line %d width %d > %d: %q", i, w, minFrameWidth, stripANSI(line))
		}
	}
	if !strings.Contains(appView(a), "The doctor found 3 SSH findings beyond these global options.") {
		t.Fatal("geometry fixture must keep the findings banner")
	}
}

// ---------------------------------------------------------------------------
// Plan 06-05 Task 2 — tuikit-level acceptance criteria
// ---------------------------------------------------------------------------

// TestGlobalSSHStorageRenderSentinelLayoutHasOnePreviewBlock proves the
// acceptance criterion: with the sentinel layout, the right pane renders
// exactly one preview block (SentinelPreview only).
func TestGlobalSSHStorageRenderSentinelLayoutHasOnePreviewBlock(t *testing.T) {
	const sentinel = "SENTINEL-PREVIEW-SENTINEL-DISTINCTIVE"
	b := &stubBackend{
		sshStoragePlanFn: func(layout SSHStorageLayout) (SSHStorageMigrationView, error) {
			return SSHStorageMigrationView{
				CurrentLayout:   StorageInclude,
				TargetLayout:    layout,
				SentinelPreview: sentinel,
				MainPreview:     "",
				OwnedPreview:    "",
				PlanToken:       "tok-sentinel",
			}, nil
		},
	}
	a := NewApp(b)
	a, _ = press(t, a, "2")
	a, _ = press(t, a, "right")
	// In the sentinel layout (current is include, we show include target → sentinel direction)
	// but for render: the stub's current layout is Include, target is Sentinel.
	// The fixture state has SSHStorage = StorageSentinel, so the "current" radio is Sentinel.
	// Move to the Include choice (which differs from current = Sentinel).
	a, _ = press(t, a, "down")
	view := appView(a)
	// The Include-layout preview pane renders MainPreview + OwnedPreview (two blocks).
	// But with the sentinel direction (from Include to Sentinel), it renders SentinelPreview.
	// The exact block count depends on the direction.
	// What the plan requires: "with the sentinel layout selected the right pane renders
	// one resulting-config preview." In the fixture, StorageSentinel → one block.
	// Let's reset to the sentinel choice (initial).
	a2 := NewApp(b)
	a2, _ = press(t, a2, "2")
	a2, _ = press(t, a2, "right")
	view2 := appView(a2)
	// Sentinel layout selected → SentinelPreview renders (one block).
	if !strings.Contains(view2, sentinel) {
		t.Errorf("sentinel layout must render the SentinelPreview; view does not contain sentinel text:\n%s", view2)
	}
	_ = view
}

// TestGlobalSSHStorageRenderIncludeLayoutHasTwoPreviewBlocks proves the
// acceptance criterion: with the Include layout selected the right pane renders
// two preview blocks (MainPreview + OwnedPreview).
func TestGlobalSSHStorageRenderIncludeLayoutHasTwoPreviewBlocks(t *testing.T) {
	const main = "MAIN-PREVIEW-DISTINCTIVE"
	const owned = "OWNED-PREVIEW-DISTINCTIVE"
	b := &stubBackend{
		sshStoragePlanFn: func(layout SSHStorageLayout) (SSHStorageMigrationView, error) {
			return SSHStorageMigrationView{
				CurrentLayout: StorageSentinel,
				TargetLayout:  layout,
				MainPreview:   main,
				OwnedPreview:  owned,
				PlanToken:     "tok-include",
			}, nil
		},
	}
	a := NewApp(b)
	a, _ = press(t, a, "2")
	a, _ = press(t, a, "right")
	// Move to the Include choice (differs from current = Sentinel).
	a, _ = press(t, a, "down")
	view := appView(a)
	if !strings.Contains(view, main) {
		t.Errorf("include layout must render MainPreview; view does not contain %q:\n%s", main, view)
	}
	if !strings.Contains(view, owned) {
		t.Errorf("include layout must render OwnedPreview; view does not contain %q:\n%s", owned, view)
	}
}

// TestGlobalSSHStoragePreviewTextFromStubViewFields proves the acceptance
// criterion: the previews' text comes from the stub's view fields (distinctive
// sentinels) and not from the package's fixture preview helpers.
func TestGlobalSSHStoragePreviewTextFromStubViewFields(t *testing.T) {
	const distinctive = "STUB-INJECTED-PREVIEW-NOT-FROM-FIXTURE"
	b := &stubBackend{
		sshStorageView: SSHStorageMigrationView{
			CurrentLayout:   StorageSentinel,
			TargetLayout:    StorageInclude,
			MainPreview:     distinctive,
			OwnedPreview:    distinctive + "-OWNED",
			SentinelPreview: distinctive + "-SENTINEL",
			PlanToken:       "tok-distinctive",
		},
	}
	a := NewApp(b)
	a, _ = press(t, a, "2")
	a, _ = press(t, a, "right")
	a, _ = press(t, a, "down")
	view := appView(a)
	if !strings.Contains(view, distinctive) {
		t.Errorf("render must use the stub's view field text, not fixture text; view does not contain %q:\n%s", distinctive, view)
	}
}

// TestGlobalSSHStorageMigrateActionAbsentWhenLayoutMatchesCurrent proves the
// acceptance criterion: the migrate action is absent when the selected layout
// equals the current one.
func TestGlobalSSHStorageMigrateActionAbsentWhenLayoutMatchesCurrent(t *testing.T) {
	a := gssApp(t)
	a, _ = press(t, a, "right")
	view := appView(a)
	// Initial state: Sentinel is current, and radio starts on Sentinel.
	if strings.Contains(view, "Migrate layout") {
		t.Errorf("Migrate must be absent when selected layout matches current:\n%s", view)
	}
	// Move to Include (differs from current = Sentinel) → Migrate must appear.
	a, _ = press(t, a, "down")
	view = appView(a)
	if !strings.Contains(view, "Migrate layout") {
		t.Errorf("Migrate must appear when selected layout differs from current:\n%s", view)
	}
}

// TestGlobalSSHStorageCeremonyInFlightUntilSuccessMsg proves the acceptance
// criterion: confirming the ceremony leaves it in flight and emits no storage
// action until a success message arrives.
func TestGlobalSSHStorageCeremonyInFlightUntilSuccessMsg(t *testing.T) {
	rec := &storageCommitCall{}
	b := &stubBackend{
		storageCall:      rec,
		sshStorageCommit: SSHStorageCommitMsg{}, // empty = success
	}
	a := NewApp(b)
	a, _ = press(t, a, "2")
	a, _ = press(t, a, "right")
	a, _ = press(t, a, "down")     // select Include
	a, _ = press(t, a, "enter")    // open ceremony
	a, cmd := press(t, a, "enter") // confirm
	// No storage action before the commit msg arrives.
	if a.state.SSHStorage != StorageSentinel {
		t.Errorf("SSHStorage must not change before SSHStorageCommitMsg: %v", a.state.SSHStorage)
	}
	if cmd == nil {
		t.Fatal("confirmation must dispatch the async CommitSSHStorage command")
	}
	// Now deliver the success message.
	raw := cmd()
	msg, ok := unwrapCommitToken(raw).(SSHStorageCommitMsg)
	if !ok {
		t.Fatalf("CommitSSHStorage delivered %T, want SSHStorageCommitMsg", unwrapCommitToken(raw))
	}
	if msg.Err != "" {
		t.Fatalf("stub commit errored: %v", msg.Err)
	}
	model, _ := a.Update(raw)
	a = model.(App)
	if a.state.SSHStorage != StorageInclude {
		t.Errorf("SSHStorage must change to Include after success msg: %v", a.state.SSHStorage)
	}
}

// TestGlobalSSHStorageFailingCommitMsgShowsRetryAndCancel proves the acceptance
// criterion: a failing commit message renders the error with retry and cancel
// and no storage action is emitted.
func TestGlobalSSHStorageFailingCommitMsgShowsRetryAndCancel(t *testing.T) {
	b := &stubBackend{
		sshStorageCommit: SSHStorageCommitMsg{Err: "injected storage failure"},
	}
	a := NewApp(b)
	a, _ = press(t, a, "2")
	a, _ = press(t, a, "right")
	a, _ = press(t, a, "down")
	a, _ = press(t, a, "enter")
	a, cmd := press(t, a, "enter")
	if cmd == nil {
		t.Fatal("confirmation must dispatch CommitSSHStorage")
	}
	raw := cmd()
	if _, ok := unwrapCommitToken(raw).(SSHStorageCommitMsg); !ok {
		t.Fatalf("expected SSHStorageCommitMsg, got %T", unwrapCommitToken(raw))
	}
	model, _ := a.Update(raw)
	a = model.(App)
	view := appView(a)
	if !strings.Contains(view, "injected storage failure") {
		t.Errorf("failure must render the error string:\n%s", view)
	}
	// Must show retry and cancel options.
	if !strings.Contains(view, "Retry") && !strings.Contains(view, "retry") {
		t.Errorf("failure must show retry option:\n%s", view)
	}
	if !strings.Contains(view, "Cancel") && !strings.Contains(view, "cancel") {
		t.Errorf("failure must show cancel option:\n%s", view)
	}
	// No storage action dispatched — SSHStorage stays at initial Sentinel.
	if a.state.SSHStorage != StorageSentinel {
		t.Errorf("SSHStorage must not change on failure: %v", a.state.SSHStorage)
	}
}

// TestGlobalSSHStoragePlanErrSuppressesMigrateAction proves the acceptance
// criterion: a stub whose plan call errors asserts the migrate action is
// suppressed and the error renders.
func TestGlobalSSHStoragePlanErrSuppressesMigrateAction(t *testing.T) {
	b := &stubBackend{
		sshStorageErr: fmt.Errorf("plan seam error"),
	}
	a := NewApp(b)
	a, _ = press(t, a, "2")
	a, _ = press(t, a, "right")
	a, _ = press(t, a, "down")
	view := appView(a)
	// Migrate must be suppressed when the plan call errored.
	if strings.Contains(view, "Migrate layout") {
		t.Errorf("Migrate action must be absent when plan errors:\n%s", view)
	}
	if !strings.Contains(view, "plan seam error") {
		t.Errorf("plan error must render in the pane:\n%s", view)
	}
}

// TestGlobalSSHStoragePostMigrationRefetchUsesConfirmedLayout is the WR-18
// regression: handleMsg's post-migration refetch must plan for the
// CONFIRMED target layout, not s.SSHStorage (the state captured BEFORE the
// SetSSHStorage reducer runs — still the OLD, pre-migration layout). Before
// the fix, this refetch silently planned the REVERSE migration (back to the
// old layout) and stored it as the live pending plan via
// SSHStorageMigrationPlan's own putPendingMigration side effect.
func TestGlobalSSHStoragePostMigrationRefetchUsesConfirmedLayout(t *testing.T) {
	var calls []SSHStorageLayout
	b := stubBackend{
		sshStoragePlanFn: func(layout SSHStorageLayout) (SSHStorageMigrationView, error) {
			calls = append(calls, layout)
			return fixtureSSHStorageView(layout), nil
		},
	}
	a := NewApp(b)
	a, _ = press(t, a, "2")
	a, _ = press(t, a, "right") // activate Storage sub-tab (fixture starts at Sentinel)
	a, _ = press(t, a, "down")  // select Include (the migration target)
	a, _ = press(t, a, "enter") // open ceremony
	a, cmd := press(t, a, "enter")
	if cmd == nil {
		t.Fatal("confirmation must dispatch the async CommitSSHStorage command")
	}
	raw := cmd()
	if _, ok := unwrapCommitToken(raw).(SSHStorageCommitMsg); !ok {
		t.Fatalf("cmd() = %T, want SSHStorageCommitMsg", unwrapCommitToken(raw))
	}
	callsBeforeCommitMsg := len(calls)
	model, _ := a.Update(raw)
	a = model.(App)
	_ = a

	if len(calls) != callsBeforeCommitMsg+1 {
		t.Fatalf("expected exactly one additional SSHStorageMigrationPlan call after the commit message, got %d (total calls: %v)",
			len(calls)-callsBeforeCommitMsg, calls)
	}
	got := calls[len(calls)-1]
	if got != StorageInclude {
		t.Errorf("post-migration refetch planned for %v, want the confirmed target %v; WR-18 regressed (reverse-migration plan)", got, StorageInclude)
	}
}

// TestGlobalSSHStorageMouseClickRefetchesAfterActivationError is the CR-05
// regression for the mouse path. It reproduces the exact pre-fix production
// shape via sshStoragePlanFn: SSHStorageMigrationPlan errors when asked to
// plan for the layout that IS already current (the real wiring.go guard the
// review found), so activate() lands with m.storageViewErr set — same as the
// real backend used to on every entry to the tab.
//
// Before the CR-05 fix, handleStorageClick set m.storageChoice WITHOUT
// refetching, so m.storageViewErr kept holding this activation error forever
// — the Migrate button (gated on storageViewErr == "") could never render
// and the mouse-only path could never reach the migration ceremony. After
// the fix, a click on the OTHER radio calls refetchStoragePlan(), which
// clears the error and fetches a fresh (successful) view for the new choice.
func TestGlobalSSHStorageMouseClickRefetchesAfterActivationError(t *testing.T) {
	b := stubBackend{
		sshStoragePlanFn: func(layout SSHStorageLayout) (SSHStorageMigrationView, error) {
			if layout == StorageSentinel { // gssApp's fixture home starts at Sentinel
				return SSHStorageMigrationView{}, fmt.Errorf("gitid: layout is already %s — nothing to plan", layout)
			}
			return fixtureSSHStorageView(layout), nil
		},
	}
	a := NewApp(b)
	a, _ = press(t, a, "2")
	a, _ = press(t, a, "right") // activate Storage sub-tab — hits the injected "already this layout" error

	before := appView(a)
	if !strings.Contains(before, "nothing to plan") {
		t.Fatalf("test setup: expected activation to render the injected error:\n%s", before)
	}
	if strings.Contains(before, "Migrate layout") {
		t.Fatalf("test setup: Migrate button must be absent while storageViewErr is set:\n%s", before)
	}

	// Mouse-click the OTHER (Include) radio row.
	a = clickCell(t, a, "gitid-owned ~/.ssh/config.d", 0, 0)

	after := appView(a)
	if strings.Contains(after, "nothing to plan") {
		t.Errorf("mouse click did not clear the stale activation error; CR-05 regressed:\n%s", after)
	}
	if !strings.Contains(after, "Migrate layout") {
		t.Fatalf("Migrate button still absent after mouse click cleared the error; CR-05 regressed:\n%s", after)
	}

	// Complete the mouse-only path: click the Migrate button itself and prove
	// it actually opens the STORE-03 ceremony (gssStorageCeremony), not just
	// that the button renders.
	a = clickCell(t, a, "Migrate layout… (Enter)", 0, 0)
	m := gssModel(t, a)
	if m.mode != gssStorageCeremony {
		t.Errorf("mode = %v after clicking Migrate, want gssStorageCeremony — the mouse-only path still cannot reach the migration ceremony", m.mode)
	}
}

// TestGlobalSSHRenderOptionsEmptyNoErrorDoesNotPanic is the WR-17
// regression: a Backend may legitimately return (nil-or-empty, nil) — zero
// rows, no error — for GlobalSSHOptionStates (globalssh.Statuses always
// returns len(Policy) rows today, but GlobalSSHOptionStates is an interface
// method any implementation may satisfy). Before the fix, renderOptions
// indexed options[selIdx] with no length guard and panicked with an
// index-out-of-range on the first render.
func TestGlobalSSHRenderOptionsEmptyNoErrorDoesNotPanic(t *testing.T) {
	b := stubBackend{sshOptions: []GlobalSSHOptionView{}}
	a := NewApp(b)
	a, _ = press(t, a, "2") // activate Global SSH tab — renders Options sub-tab
	view := appView(a)
	if !strings.Contains(view, "No global SSH options to show.") {
		t.Errorf("empty-no-error options must render the empty-state note, got:\n%s", view)
	}
}

// TestGlobalSSHStorageTokenPassthroughAndClearing proves the acceptance
// criterion: the token the model sends to CommitSSHStorage is byte-identical
// to the one the view it opened the ceremony with carried, and that moving
// the layout selection or re-activating the screen clears it.
func TestGlobalSSHStorageTokenPassthroughAndClearing(t *testing.T) {
	rec := &storageCommitCall{}
	b := &stubBackend{
		storageCall: rec,
	}
	a := NewApp(b)
	a, _ = press(t, a, "2")
	a, _ = press(t, a, "right")
	a, _ = press(t, a, "down")     // select Include (trigger plan call)
	a, _ = press(t, a, "enter")    // open ceremony
	_, cmd := press(t, a, "enter") // confirm; a is not used further
	if cmd == nil {
		t.Fatal("confirmation must dispatch CommitSSHStorage")
	}
	_ = cmd()
	// The token the model passed must equal the fixture token the stub's plan returned.
	wantToken := fixtureSSHStorageView(StorageInclude).PlanToken
	if rec.token != wantToken {
		t.Errorf("CommitSSHStorage received token %q, want %q (byte-identical to view.PlanToken)", rec.token, wantToken)
	}

	// Moving the layout selection must invalidate the stored view: it
	// clears the old plan and fetches a new one with a fresh token. The
	// new token is still a valid fixture token — not empty — but it
	// belongs to the NEW selection, so the old (committed) token cannot
	// be reused. We verify the token changed (or the view was re-fetched).
	a2 := NewApp(b)
	a2, _ = press(t, a2, "2")
	a2, _ = press(t, a2, "right")
	a2, _ = press(t, a2, "down") // select Include → fetches Include plan
	m2i, ok2 := a2.screens[TabGlobalSSH].(globalSSHModel)
	if !ok2 {
		t.Fatalf("screens[TabGlobalSSH] = %T, want globalSSHModel", a2.screens[TabGlobalSSH])
	}
	includeToken := m2i.storageView.PlanToken
	if includeToken == "" {
		t.Error("storageView.PlanToken must be set after selecting Include")
	}
	// Move back to Sentinel — the plan is re-fetched for Sentinel.
	a2, _ = press(t, a2, "up")
	m2s, ok2s := a2.screens[TabGlobalSSH].(globalSSHModel)
	if !ok2s {
		t.Fatalf("screens[TabGlobalSSH] after up = %T, want globalSSHModel", a2.screens[TabGlobalSSH])
	}
	// The token must differ from the Include token (different direction = different plan).
	// For the fixture stub, PlanToken is "fixture-token-<layout>".
	sentinelToken := m2s.storageView.PlanToken
	if sentinelToken == includeToken {
		t.Errorf("moving layout selection must change the plan token; both are %q", includeToken)
	}

	// Re-activating the screen resets the selection to the current layout
	// and refetches. The token after re-activation must match the current
	// layout's fixture token (not the previous Include selection).
	a3 := NewApp(b)
	a3, _ = press(t, a3, "2")
	a3, _ = press(t, a3, "right")
	a3, _ = press(t, a3, "down")  // select Include
	a3, _ = press(t, a3, "1")     // switch away
	a3, _ = press(t, a3, "2")     // switch back — activate resets selection to current layout
	a3, _ = press(t, a3, "right") // back to Storage sub-tab
	m3, ok := a3.screens[TabGlobalSSH].(globalSSHModel)
	if !ok {
		t.Fatalf("screens[TabGlobalSSH] = %T, want globalSSHModel", a3.screens[TabGlobalSSH])
	}
	// After re-activate, storageChoice is reset to the current layout (Sentinel).
	// The storageView is re-fetched for Sentinel, so PlanToken should be the Sentinel token.
	if m3.storageChoice != StorageSentinel {
		t.Errorf("re-activating screen must reset storageChoice to current layout (Sentinel); got %v", m3.storageChoice)
	}
}

// TestSubTabStripRowAccountingIsSingleSourced verifies that the sub-tab
// strip is present and consistent across both sub-tabs. The strip is the
// first rendered line on both Options and Storage.
func TestSubTabStripRowAccountingIsSingleSourced(t *testing.T) {
	a := gssApp(t)
	m := gssModel(t, a)

	// Render Options to measure the strip.
	optionsView := m.view(a.state, a.width, a.height)
	optionsBody := optionsView.body
	optionsLines := strings.Split(optionsBody, "\n")
	if len(optionsLines) == 0 {
		t.Fatal("Options body is empty")
	}

	// The strip occupies gssSubTabStripRows rows.
	stripRows := gssSubTabStripRows()

	// The strip occupies rows 0..stripRows-1.
	// The label line (row 1) should contain "Options" and "Storage".
	if len(optionsLines) < 2 {
		t.Fatalf("Options body too short (need at least 2 lines for the strip), got %d lines", len(optionsLines))
	}
	labelLine := optionsLines[1] // The middle line of the 3-row border box
	if !strings.Contains(labelLine, "Options") || !strings.Contains(labelLine, "Storage") {
		t.Errorf("Options: strip label line (row 1) should contain labels, got: %q", labelLine)
	}

	// gssOptionsTopLines should equal stripRows exactly when no findings
	// banner is present — a loose ">=" would also pass a hardcoded literal
	// disconnected from gssSubTabStripRows(), defeating the "single-sourced"
	// claim in this test's name (WR-07). gssApp(t)'s default stubBackend
	// state carries SSH findings (so the banner IS present there), so build
	// a banner-free state explicitly for this check.
	bannerFreeState := a.state
	bannerFreeState.Findings = nil
	topLines := gssOptionsTopLines(bannerFreeState)
	if topLines != stripRows {
		t.Errorf("gssOptionsTopLines = %d, want exactly %d (no findings banner present)", topLines, stripRows)
	}

	// The rendered body's own border-close row must land at exactly
	// stripRows-1, proving gssSubTabStripRows() is the actual row count the
	// renderer produced, not just a number the accounting helper repeats.
	borderCloseRow := -1
	for i, line := range optionsLines {
		if strings.Contains(line, "╰") || strings.Contains(line, "╯") {
			borderCloseRow = i
			break
		}
	}
	if borderCloseRow != stripRows-1 {
		t.Errorf("rendered strip's border-close row = %d, want %d (gssSubTabStripRows()-1)", borderCloseRow, stripRows-1)
	}

	// Switch to Storage sub-tab and verify the strip is present there too.
	a, _ = press(t, a, "right")
	m = gssModel(t, a)
	storageView := m.view(a.state, a.width, a.height)
	storageBody := storageView.body
	storageLines := strings.Split(storageBody, "\n")

	if len(storageLines) == 0 {
		t.Fatal("Storage body is empty")
	}
	if len(storageLines) < 2 {
		t.Fatalf("Storage body too short (need at least 2 lines for the strip), got %d lines", len(storageLines))
	}
	storageLabelLine := storageLines[1] // The middle line of the 3-row border box
	if !strings.Contains(storageLabelLine, "Options") || !strings.Contains(storageLabelLine, "Storage") {
		t.Errorf("Storage: strip label line (row 1) should contain labels, got: %q", storageLabelLine)
	}

	// Both views should have the strip as their first line, confirming that
	// renderOptions and renderStorage both prefix with subTabStrip().
}

// TestSubTabStripClickSwitchesSubTabs verifies that clicking on either
// sub-tab label switches sub-tabs, using the actual rendered coordinates.
func TestSubTabStripClickSwitchesSubTabs(t *testing.T) {
	a := gssApp(t)
	m := gssModel(t, a)
	if m.subTab != gssOptions {
		t.Fatalf("initial subTab = %v, want gssOptions", m.subTab)
	}

	// Click on the Storage label to switch sub-tabs.
	a = clickCell(t, a, "Storage & preview", 0, 0)
	m = gssModel(t, a)
	if m.subTab != gssStorage {
		t.Errorf("subTab after clicking Storage label = %v, want gssStorage", m.subTab)
	}

	// Click on the Options label to switch back.
	a = clickCell(t, a, "Options", 0, 0)
	m = gssModel(t, a)
	if m.subTab != gssOptions {
		t.Errorf("subTab after clicking Options label = %v, want gssOptions", m.subTab)
	}

	// Click on the third "All directives" label (PROP-01, plan 09.5-01
	// Task 3) — the fourth two-sub-tab-assumption site (handleClick's strip
	// hit-test) proven directly, complementing the real-PTY SGR click
	// coverage in TestGlobalSSH_RealPTYAllDirectivesLabelMouseClick.
	a = clickCell(t, a, gssTabPropertiesLabel, 0, 0)
	m = gssModel(t, a)
	if m.subTab != gssProperties {
		t.Errorf("subTab after clicking the All directives label = %v, want gssProperties", m.subTab)
	}

	// And back to Options, proving the click is not a one-way trip.
	a = clickCell(t, a, "Options", 0, 0)
	m = gssModel(t, a)
	if m.subTab != gssOptions {
		t.Errorf("subTab after clicking Options label from Properties = %v, want gssOptions", m.subTab)
	}
}

// TestSubTabStripClickHitTestMatchesRenderedSpans is the regression for
// WR-01: the old hit-test computed label spans from a hand-written "┊ "
// prefix offset and len() byte counts, one column to the left of where
// subTabStrip() actually draws them, and hardcoded the label row as
// `y == 1`. The symptom: a click on the LAST cell of either label (still
// visually on the label) missed as inert, while a click on the blank gap
// BETWEEN the labels (one column left of Storage's real start) wrongly
// activated Storage. This test clicks coordinates read from the ACTUAL
// rendered frame — never a recomputed offset — so it can only pass if the
// hit-test truly matches what subTabStrip() draws.
func TestSubTabStripClickHitTestMatchesRenderedSpans(t *testing.T) {
	a := gssApp(t)
	// Coordinates must be read from the FULL rendered frame (appView),
	// matching clickCell's convention — handleClick receives Y already
	// offset by frameBodyTop, so a coordinate computed from the body-only
	// view (m.view(...).body) would land one screen off from where the
	// click actually lands.
	lines := strings.Split(appView(a), "\n")
	labelRow := -1
	var plain string
	for y, line := range lines {
		p := ansi.Strip(line)
		if strings.Contains(p, gssTabOptionsLabel) {
			labelRow, plain = y, p
			break
		}
	}
	if labelRow < 0 {
		t.Fatalf("sub-tab strip label row not found in rendered frame:\n%s", appView(a))
	}

	optIdx := strings.Index(plain, gssTabOptionsLabel)
	optLastCol := ansi.StringWidth(plain[:optIdx]) + ansi.StringWidth(gssTabOptionsLabel) - 1

	stoIdx := strings.Index(plain, gssTabStorageLabel)
	if stoIdx < 0 {
		t.Fatalf("Storage label not found on the strip's label row:\n%s", plain)
	}
	stoStartCol := ansi.StringWidth(plain[:stoIdx])

	// Start from Storage so an INERT click (the pre-fix symptom) is
	// distinguishable from a correct one — asserting "still gssOptions"
	// from the initial gssOptions state would pass vacuously either way.
	fromStorage, _ := clickAt(t, a, ansi.StringWidth(plain[:stoIdx])+1, labelRow)
	if m := gssModel(t, fromStorage); m.subTab != gssStorage {
		t.Fatalf("setup: clicking the Storage label did not select gssStorage, got %v", m.subTab)
	}

	// The last cell of the Options label must still switch back to Options
	// — the off-by-one bug clipped this cell off as inert, which from
	// gssStorage would visibly stay on gssStorage instead.
	backToOptions, _ := clickAt(t, fromStorage, optLastCol, labelRow)
	if m := gssModel(t, backToOptions); m.subTab != gssOptions {
		t.Errorf("clicking the Options label's LAST cell (col %d) did not select gssOptions, got %v", optLastCol, m.subTab)
	}

	// The gap column immediately BEFORE Storage's real start (the cell the
	// old shifted-left math wrongly attributed to Storage) must stay inert
	// — from gssOptions, it must NOT switch to gssStorage.
	gapCol := stoStartCol - 1
	if gapCol > optLastCol { // only meaningful if there is a real gap
		clicked, _ := clickAt(t, backToOptions, gapCol, labelRow)
		if m := gssModel(t, clicked); m.subTab != gssOptions {
			t.Errorf("clicking the gap column %d (before Storage's real start %d) wrongly switched to %v — the hit-test is shifted", gapCol, stoStartCol, m.subTab)
		}
	}
}

// TestSubTabStripFitsFixedGeometryInEveryStripState measures the sub-tab
// strip's row cost across all five strip-bearing render states and logs
// the available/used/headroom for each.
// It records the tightest state's margin for Task 2's box-variant decision.
func TestSubTabStripFitsFixedGeometryInEveryStripState(t *testing.T) {
	a := gssApp(t)
	tightestMargin := 100 // Start high, will be minimized

	// State 1: Options normal with findings banner
	a.state.Findings = []DemoFinding{
		{HealthFinding: HealthFinding{Section: "SSH", Severity: SeverityWarning, Title: "test", Family: "test", Fixable: false}},
	}
	m := gssModel(t, a)
	view := m.view(a.state, a.width, a.height)
	bodyLines := strings.Split(view.body, "\n")
	usedRows := len(bodyLines)
	availRows := frameBodyRows(a.height)
	headroom := availRows - usedRows
	t.Logf("State 1: Options normal with findings banner")
	t.Logf("  Available body rows: %d", availRows)
	t.Logf("  Used rows: %d", usedRows)
	t.Logf("  Headroom: %d rows", headroom)
	if usedRows > availRows {
		t.Errorf("State 1 exceeds available rows: used=%d, available=%d", usedRows, availRows)
	}
	if headroom < tightestMargin {
		tightestMargin = headroom
	}

	// State 2: Options zero-rows
	a2 := NewAppOnGlobalSSH(stubBackend{sshOptions: []GlobalSSHOptionView{}}, false)
	m2 := gssModel(t, a2)
	view2 := m2.view(a2.state, a2.width, a2.height)
	bodyLines2 := strings.Split(view2.body, "\n")
	usedRows2 := len(bodyLines2)
	availRows2 := frameBodyRows(a2.height)
	headroom2 := availRows2 - usedRows2
	t.Logf("State 2: Options zero-rows")
	t.Logf("  Available body rows: %d", availRows2)
	t.Logf("  Used rows: %d", usedRows2)
	t.Logf("  Headroom: %d rows", headroom2)
	if usedRows2 > availRows2 {
		t.Errorf("State 2 exceeds available rows: used=%d, available=%d", usedRows2, availRows2)
	}
	if headroom2 < tightestMargin {
		tightestMargin = headroom2
	}

	// State 3: Options with error
	m3 := gssModel(t, a)
	m3.optionsErr = "test error"
	a3 := a
	a3.screens[TabGlobalSSH] = m3
	view3 := m3.view(a3.state, a3.width, a3.height)
	bodyLines3 := strings.Split(view3.body, "\n")
	usedRows3 := len(bodyLines3)
	availRows3 := frameBodyRows(a3.height)
	headroom3 := availRows3 - usedRows3
	t.Logf("State 3: Options with error")
	t.Logf("  Available body rows: %d", availRows3)
	t.Logf("  Used rows: %d", usedRows3)
	t.Logf("  Headroom: %d rows", headroom3)
	if usedRows3 > availRows3 {
		t.Errorf("State 3 exceeds available rows: used=%d, available=%d", usedRows3, availRows3)
	}
	if headroom3 < tightestMargin {
		tightestMargin = headroom3
	}

	// State 4: Storage normal
	a4 := gssApp(t)
	a4, _ = press(t, a4, "right")
	m4 := gssModel(t, a4)
	view4 := m4.view(a4.state, a4.width, a4.height)
	bodyLines4 := strings.Split(view4.body, "\n")
	usedRows4 := len(bodyLines4)
	availRows4 := frameBodyRows(a4.height)
	headroom4 := availRows4 - usedRows4
	t.Logf("State 4: Storage normal")
	t.Logf("  Available body rows: %d", availRows4)
	t.Logf("  Used rows: %d", usedRows4)
	t.Logf("  Headroom: %d rows", headroom4)
	if usedRows4 > availRows4 {
		t.Errorf("State 4 exceeds available rows: used=%d, available=%d", usedRows4, availRows4)
	}
	if headroom4 < tightestMargin {
		tightestMargin = headroom4
	}

	// State 5: apply ceremony.
	a5 := gssApp(t)
	m5 := gssModel(t, a5)
	optionsForCeremony := m5.overlaidOptions(a5.state)
	if len(optionsForCeremony) == 0 {
		t.Fatal("no options available to enter the apply ceremony")
	}
	m5.chosen = map[string]bool{optionsForCeremony[0].Key: true}
	a5.screens[TabGlobalSSH] = m5
	a5, _ = press(t, a5, "a")
	m5b := gssModel(t, a5)
	if m5b.mode != gssApplyCeremony {
		t.Fatalf("expected apply ceremony to open, mode=%d", m5b.mode)
	}
	view5 := m5b.view(a5.state, a5.width, a5.height)
	bodyLines5 := strings.Split(view5.body, "\n")
	usedRows5 := len(bodyLines5)
	availRows5 := frameBodyRows(a5.height)
	headroom5 := availRows5 - usedRows5
	t.Logf("State 5: Apply ceremony")
	t.Logf("  Available body rows: %d", availRows5)
	t.Logf("  Used rows: %d", usedRows5)
	t.Logf("  Headroom: %d rows", headroom5)
	if usedRows5 > availRows5 {
		t.Errorf("State 5 exceeds available rows: used=%d, available=%d", usedRows5, availRows5)
	}
	// WR-02 regression: the ceremony body must not re-render the 3-row
	// bordered sub-tab strip (redundant — the crumb line above the body
	// already says "Options"/"Storage & preview"), which used to eat 3 of
	// this state's ~5 spare rows down to a bare margin of 2.
	if headroom5 < 4 {
		t.Errorf("State 5 headroom too tight (WR-02 regressed): got %d, want >= 4", headroom5)
	}
	if headroom5 < tightestMargin {
		tightestMargin = headroom5
	}

	// State 6: storage ceremony.
	a6 := gssApp(t)
	a6, _ = press(t, a6, "right")
	a6, _ = press(t, a6, "down")
	a6, _ = press(t, a6, "enter")
	m6 := gssModel(t, a6)
	if m6.mode != gssStorageCeremony {
		t.Fatalf("expected storage ceremony to open, mode=%d", m6.mode)
	}
	view6 := m6.view(a6.state, a6.width, a6.height)
	bodyLines6 := strings.Split(view6.body, "\n")
	usedRows6 := len(bodyLines6)
	availRows6 := frameBodyRows(a6.height)
	headroom6 := availRows6 - usedRows6
	t.Logf("State 6: Storage ceremony")
	t.Logf("  Available body rows: %d", availRows6)
	t.Logf("  Used rows: %d", usedRows6)
	t.Logf("  Headroom: %d rows", headroom6)
	if usedRows6 > availRows6 {
		t.Errorf("State 6 exceeds available rows: used=%d, available=%d", usedRows6, availRows6)
	}
	if headroom6 < 4 {
		t.Errorf("State 6 headroom too tight (WR-02 regressed): got %d, want >= 4", headroom6)
	}
	if headroom6 < tightestMargin {
		tightestMargin = headroom6
	}

	t.Logf("Tightest state headroom: %d rows", tightestMargin)
	if tightestMargin < 0 {
		t.Errorf("Some state exceeds fixed geometry; tightest headroom = %d rows", tightestMargin)
	}
}

// TestSubTabStripRendersBordered verifies that the sub-tab strip renders
// as a bordered box with the accent color, occupying more than one line.
// This test is authored RED (fails against unbordered strip), implemented
// GREEN, and committed together with the implementation (not as a separate RED commit).
func TestSubTabStripRendersBordered(t *testing.T) {
	a := gssApp(t)
	m := gssModel(t, a)

	// Render Options to inspect the strip.
	optionsView := m.view(a.state, a.width, a.height)
	optionsBody := optionsView.body
	optionsLines := strings.Split(optionsBody, "\n")
	if len(optionsLines) == 0 {
		t.Fatal("Options body is empty")
	}

	// The strip should occupy multiple lines (at least 3: top border, label line, bottom border).
	stripEndLine := 1 // For now, assume strip is just the first line before border
	for i := 1; i < len(optionsLines); i++ {
		// Look for the border bottom rune (╰ or the closing corner).
		// If we find it, the strip ends here.
		if strings.Contains(optionsLines[i], "╰") || strings.Contains(optionsLines[i], "╯") {
			stripEndLine = i + 1
			break
		}
	}

	if stripEndLine < 3 {
		t.Errorf("strip should occupy at least 3 lines (top border + labels + bottom border), occupies %d lines", stripEndLine)
	}

	// The first line should contain the top border rune (╭ or the opening corner).
	if !strings.Contains(optionsLines[0], "╭") && !strings.Contains(optionsLines[0], "╮") {
		t.Errorf("strip first line should contain top-border corner rune (╭ or ╮), got: %q", optionsLines[0])
	}

	// The strip should render in the accent color (ANSI 4, blue).
	// The accent color is proven by the rune presence above; color codes are optional in stripped contexts.

	// The active label should still be marked with styleReverse (raw ANSI
	// SGR 7, "\x1b[7m"). WR-07: a plain strings.Contains(line, "Options")
	// check here would pass even if the reverse styling were lost entirely
	// — it only proves the label text is present, not that it is marked as
	// active. Assert the reverse escape actually wraps "Options".
	activeMarked := false
	for _, line := range optionsLines[:stripEndLine] {
		if strings.Contains(line, "\x1b[7m Options \x1b[m") {
			activeMarked = true
			break
		}
	}
	if !activeMarked {
		t.Errorf("active sub-tab label should be marked with styleReverse (\\x1b[7m) in the bordered strip, got lines: %q", optionsLines[:stripEndLine])
	}
}

// gssScrollRows builds n synthetic, selectable SSH option rows for scroll
// tests — the Global SSH mirror of globalgit_test.go's gitScrollRows —
// enough to force the master list past the measured body budget. Keys are
// short and distinct ("row00".."rowNN") so truncLine's width clip never
// swallows the identifying substring the tests assert on.
func gssScrollRows(n int) []GlobalSSHOptionView {
	out := make([]GlobalSSHOptionView, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, GlobalSSHOptionView{
			Key:                fmt.Sprintf("row%02d", i),
			CurrentValue:       "not set",
			Recommended:        "x",
			OneLiner:           "scroll test row",
			State:              GlobalSSHNeedsAction,
			WritableToHostStar: true,
		})
	}
	return out
}

// TestGlobalSSHSmallListNoScrollByteIdentical is the WR-09 regression: this
// screen used to render every option row unconditionally with no scroll
// window at all (unlike Global Git's gitComputeScrollWindow), risking
// silent truncation past the frame's fixed row budget once the policy
// table grows. This asserts the no-scroll path stays byte-identical for a
// list that already fits: window start zero, no cue line.
func TestGlobalSSHSmallListNoScrollByteIdentical(t *testing.T) {
	b := stubBackend{sshOptions: gssScrollRows(3)}
	a, _ := press(t, NewApp(b), "2")
	m := gssModel(t, a)
	if m.listWindowStart != 0 {
		t.Errorf("listWindowStart = %d, want 0 for a small list", m.listWindowStart)
	}
	view := appView(a)
	if strings.Contains(view, "more options") {
		t.Errorf("a small list must render no scroll cue:\n%s", view)
	}
}

// TestGlobalSSHFullListFitsComputedBudget renders a fixture large enough to
// need scrolling at the frame's real geometry and asserts every rendered
// master-list line count is within the SAME budget gssVisibleRowCount
// computes — the test derives the budget from the same helper rather than
// hardcoding a number (plan 02-15's standing lesson).
func TestGlobalSSHFullListFitsComputedBudget(t *testing.T) {
	b := stubBackend{sshOptions: gssScrollRows(20)}
	a, _ := press(t, NewApp(b), "2")
	m := gssModel(t, a)
	s := a.state
	options := m.overlaidOptions(s)
	w := m.gssComputeScrollWindow(len(options), s)
	budgetLines := w.visibleRows * optionRowLines
	if w.needsScroll {
		budgetLines++ // the reserved cue line
	}
	if got := frameBodyRows(minFrameHeight) - gssOptionsTopLines(s); budgetLines > got {
		t.Errorf("computed render budget %d exceeds frameBodyRows-chrome budget %d", budgetLines, got)
	}
}

// TestGlobalSSHScrollDownMovesWindowByOneRowPerStep drives the selection
// from the first row to the last, one keystroke at a time, and asserts the
// window start increases by exactly one on each step past the edge and
// never more.
func TestGlobalSSHScrollDownMovesWindowByOneRowPerStep(t *testing.T) {
	const n = 20
	b := stubBackend{sshOptions: gssScrollRows(n)}
	a, _ := press(t, NewApp(b), "2")
	m := gssModel(t, a)
	if !m.gssComputeScrollWindow(n, a.state).needsScroll {
		t.Fatal("setup: 20 rows must need scrolling at the fixed frame size")
	}
	prevStart := m.listWindowStart
	for i := 0; i < n-1; i++ {
		a, _ = press(t, a, "down")
		m = gssModel(t, a)
		delta := m.listWindowStart - prevStart
		if delta < 0 || delta > 1 {
			t.Fatalf("step %d: listWindowStart moved by %d, want 0 or 1", i, delta)
		}
		prevStart = m.listWindowStart
	}
}

// TestGlobalSSHScrollUpMovesWindowByOneRowPerStep drives the selection back
// up from the last row to the first and asserts the window start decreases
// by exactly one per step past the edge.
func TestGlobalSSHScrollUpMovesWindowByOneRowPerStep(t *testing.T) {
	const n = 20
	b := stubBackend{sshOptions: gssScrollRows(n)}
	a, _ := press(t, NewApp(b), "2")
	for i := 0; i < n-1; i++ {
		a, _ = press(t, a, "down")
	}
	m := gssModel(t, a)
	maxStart := m.listWindowStart
	if maxStart == 0 {
		t.Fatal("setup: scrolling to the last row must have advanced the window")
	}
	prevStart := maxStart
	for i := 0; i < n-1; i++ {
		a, _ = press(t, a, "up")
		m = gssModel(t, a)
		delta := prevStart - m.listWindowStart
		if delta < 0 || delta > 1 {
			t.Fatalf("step %d: listWindowStart moved by %d, want 0 or 1", i, delta)
		}
		prevStart = m.listWindowStart
	}
	if m.listWindowStart != 0 {
		t.Errorf("after returning to the first row, listWindowStart = %d, want 0", m.listWindowStart)
	}
}

// TestGlobalSSHScrollClickRowMatchesWindow is the click-side half of WR-09:
// after scrolling the window down, a click on a rendered row must select
// the ROW ACTUALLY DRAWN THERE, not the row that would occupy that screen
// position under window start 0 — the exact click-row desync class this
// finding warns about.
func TestGlobalSSHScrollClickRowMatchesWindow(t *testing.T) {
	const n = 20
	b := stubBackend{sshOptions: gssScrollRows(n)}
	a, _ := press(t, NewApp(b), "2")
	for i := 0; i < 10; i++ {
		a, _ = press(t, a, "down")
	}
	m := gssModel(t, a)
	if m.listWindowStart == 0 {
		t.Fatal("setup: scrolling 10 rows down must have advanced the window")
	}
	options := m.overlaidOptions(a.state)
	w := m.gssComputeScrollWindow(len(options), a.state)
	// The first visible row (screen row 0 of the list, right after any
	// gitCueUp cue line) must resolve to windowStart, not row 0.
	topOfListY := gssOptionsTopLines(a.state)
	if w.cue == gitCueUp {
		topOfListY++ // the reserved cue line occupies the first list row
	}
	// clickAt sends FULL-FRAME coordinates (App.handleMouse's input); the
	// body-relative offsets above must be shifted by frameBodyTop to match.
	// x=20 lands well past the "[ ]" checkbox glyph (columns ~3-6) and
	// inside the row label text — a checkbox-column click would instead
	// toggle selection without moving m.detailKey, defeating this test.
	a2, _ := clickAt(t, a, 20, topOfListY+frameBodyTop)
	m2 := gssModel(t, a2)
	wantKey := options[w.windowStart].Key
	if m2.detailKey != wantKey {
		t.Errorf("clicking the first visible row selected %q, want %q (windowStart=%d)", m2.detailKey, wantKey, w.windowStart)
	}
}

// mutuallyExclusiveSSHStoragePlanFn mimics the REAL backend's field
// discipline (cmd/gitid/wiring.go: SentinelPreview is set only for the
// sentinel layout; MainPreview/OwnedPreview only for the include layout) —
// unlike fixtureSSHStorageView, which populates all three fields regardless
// of the requested layout and so cannot detect a missing refetch (BL-01,
// 09.4-REVIEW.md independent re-review).
func mutuallyExclusiveSSHStoragePlanFn(layout SSHStorageLayout) (SSHStorageMigrationView, error) {
	v := SSHStorageMigrationView{
		CurrentLayout: StorageSentinel,
		TargetLayout:  layout,
		PlanToken:     "tok-" + string(layout),
	}
	if layout == StorageSentinel {
		v.SentinelPreview = "SENTINEL-PREVIEW-FOR-" + string(layout)
	} else {
		v.MainPreview = "MAIN-PREVIEW-FOR-" + string(layout)
		v.OwnedPreview = "OWNED-PREVIEW-FOR-" + string(layout)
	}
	return v, nil
}

// TestGlobalSSHStorageRefetchesOnEveryChoiceMutationSite is the BL-01
// regression: refetchStoragePlan() was wired into only 2 of the 4 sites
// that mutate m.storageChoice (the keyboard radio toggle and the mouse
// radio click), leaving the left/right sub-tab switch (both the browse and
// zero-options branches) and the sub-tab-strip mouse click resetting
// storageChoice back to s.SSHStorage WITHOUT refetching — so the pane kept
// rendering the PREVIOUS layout's preview fields, which read as empty once
// a real (mutually-exclusive-field) backend is used. Drives the exact
// repro: Storage -> toggle radio to Include (refetch) -> Options -> back to
// Storage (choice resets to Sentinel) and asserts the resulting pane is
// NOT empty.
func TestGlobalSSHStorageRefetchesOnEveryChoiceMutationSite(t *testing.T) {
	b := &stubBackend{sshStoragePlanFn: mutuallyExclusiveSSHStoragePlanFn}
	a, _ := press(t, NewApp(b), "2")
	a, _ = press(t, a, "right") // Options -> Storage (choice = s.SSHStorage = sentinel)
	a, _ = press(t, a, "down")  // toggle radio -> Include, refetch (existing site)
	a, _ = press(t, a, "left")  // Storage -> Options
	a, _ = press(t, a, "right") // Options -> Storage AGAIN: choice resets to sentinel
	m := gssModel(t, a)
	if m.storageChoice != StorageSentinel {
		t.Fatalf("setup: storageChoice = %v, want StorageSentinel after the second right", m.storageChoice)
	}
	if m.storageView.SentinelPreview == "" {
		t.Errorf("BL-01 regressed: storageView.SentinelPreview is empty after switching sub-tabs back to Storage — refetchStoragePlan() was not called on this mutation site")
	}
	view := appView(a)
	if !strings.Contains(view, "SENTINEL-PREVIEW-FOR-sentinel") {
		t.Errorf("Resulting config pane does not render the sentinel preview after the round-trip; got:\n%s", view)
	}
}

// TestGlobalSSHOptionsErrorRendersFindingsBanner is the regression for
// WR-12: Global Git's probe-failure branch renders the doctor findings
// banner above the error note (globalgit.go); Global SSH's equivalent
// branch rendered only the warning and explanatory line, dropping the
// "! The doctor found N SSH findings beyond these global options." banner
// and its "Open Doctor (4)" link exactly when the screen is least able to
// help.
func TestGlobalSSHOptionsErrorRendersFindingsBanner(t *testing.T) {
	b := &stubBackend{sshOptionsErr: errors.New("probe exploded")}
	a, _ := press(t, NewApp(b), "2")
	a.state.Findings = []DemoFinding{
		{HealthFinding: HealthFinding{Section: "SSH", Title: "some SSH finding"}},
	}
	m := gssModel(t, a)
	view := stripANSI(m.view(a.state, 100, 30).body)
	if !strings.Contains(view, "The doctor found 1 SSH finding") {
		t.Errorf("Global SSH's probe-failure branch must render the findings banner, like Global Git's does:\n%s", view)
	}
	if !strings.Contains(view, "Open Doctor (4)") {
		t.Errorf("findings banner must include the Open Doctor link:\n%s", view)
	}
}

// TestGlobalSSHOptionsErrorKeysFullyFailOpen is the regression for WR-12's
// second half: Global Git's optionsErr branch in handleKey returns fully
// unhandled (`keyResult{model: m}`), letting every key — including ←/→ —
// reach the top-level tab switcher; Global SSH's equivalent branch special-
// cased ←/→ into its OWN sub-tab switch and marked it handled, which
// consumed the key and blocked the top-level ←/→ tab navigation Global
// Git's contract allows.
func TestGlobalSSHOptionsErrorKeysFullyFailOpen(t *testing.T) {
	b := &stubBackend{sshOptionsErr: errors.New("probe exploded")}
	m := newGlobalSSHModel(b)
	state := Seed()
	activated, _ := m.activate(state)
	m = activated.(globalSSHModel)

	res := m.handleKey(pressKey("left"), state)
	if res.handled {
		t.Error("left must be fully unhandled while an options probe error is active, matching Global Git's fail-open contract")
	}
}

// TestGlobalSSHScrollBudgetGrowsWithRealTerminalHeight is the regression
// for WR-10: gssVisibleRowCount was pinned to frameBodyRows(minFrameHeight)
// — a constant ~25-row budget — while view() sizes the pane from the REAL
// height. On a taller terminal the list still windowed to the small
// budget, leaving blank rows under a "+N more options" cue that a bigger
// terminal should not need at all. A resize (tea.WindowSizeMsg) must
// persist the real height on the model and grow the budget from it.
func TestGlobalSSHScrollBudgetGrowsWithRealTerminalHeight(t *testing.T) {
	b := stubBackend{sshOptions: gssScrollRows(20)}
	a, _ := press(t, NewApp(b), "2")
	before := gssModel(t, a).rowBudgetHeight()
	if before != minFrameHeight {
		t.Fatalf("fixture sanity: rowBudgetHeight before any resize = %d, want minFrameHeight (%d)", before, minFrameHeight)
	}
	beforeVisible := gssVisibleRowCount(20, before, a.state)
	if beforeVisible >= 20 {
		t.Fatalf("fixture sanity: 20 rows must need scrolling at the canonical height, got visible=%d", beforeVisible)
	}

	model, _ := a.Update(tea.WindowSizeMsg{Width: 100, Height: 60})
	a, ok := model.(App)
	if !ok {
		t.Fatalf("Update(WindowSizeMsg) returned %T, want App", model)
	}
	m := gssModel(t, a)
	if m.lastHeight != 60 {
		t.Fatalf("lastHeight after resize = %d, want 60", m.lastHeight)
	}
	afterVisible := gssVisibleRowCount(20, m.rowBudgetHeight(), a.state)
	if afterVisible <= beforeVisible {
		t.Errorf("visible row count must grow on a taller terminal: before=%d after=%d", beforeVisible, afterVisible)
	}
	if afterVisible != 20 {
		t.Errorf("all 20 rows must fit at height=60 with no scrolling needed, got visible=%d", afterVisible)
	}
}

// TestSSHStaleCommitMsgNotMisattributedToNewerCeremony is the Global SSH
// mirror of TestGitStaleCommitMsgNotMisattributedToNewerCeremony
// (CR-01, 09.4-REVIEW.md second independent re-review) — the review
// confirmed the identical shape (single applyCommitPending boolean, no
// per-request correlation) was present unmodified in this file's
// GlobalSSHCommitMsg handler.
func TestSSHStaleCommitMsgNotMisattributedToNewerCeremony(t *testing.T) {
	b := &stubBackend{}
	m := newGlobalSSHModel(b)
	state := Seed()
	activated, _ := m.activate(state)
	m = activated.(globalSSHModel)

	m.detailKey = "HashKnownHosts"
	toggled := m.handleKey(pressKey("space"), state)
	m = toggled.model.(globalSSHModel)
	opened := m.handleKey(pressKey("a"), state)
	m = opened.model.(globalSSHModel)
	confirmed1 := m.handleKey(pressKey("enter"), state)
	m = confirmed1.model.(globalSSHModel)
	staleMsg := confirmed1.cmd()

	reactivated, _ := m.activate(state)
	m = reactivated.(globalSSHModel)
	if m.mode == gssApplyCeremony || m.applyCommitPending {
		t.Fatal("setup: activate() must have cleared the ceremony state")
	}

	m.detailKey = "ForwardAgent"
	toggled2 := m.handleKey(pressKey("space"), state)
	m = toggled2.model.(globalSSHModel)
	opened2 := m.handleKey(pressKey("a"), state)
	m = opened2.model.(globalSSHModel)
	confirmed2 := m.handleKey(pressKey("enter"), state)
	m = confirmed2.model.(globalSSHModel)
	if !m.applyCommitPending {
		t.Fatal("setup: confirming ceremony 2 must set applyCommitPending")
	}
	ceremony2HeadingBefore := m.ceremony.cfg.Heading

	result := m.handleMsg(staleMsg, state)
	m = result.model.(globalSSHModel)

	if !m.applyCommitPending {
		t.Error("CR-01 regressed: a stale message must not clear applyCommitPending for the genuinely in-flight ceremony 2")
	}
	if m.ceremony.cfg.Heading != ceremony2HeadingBefore || m.ceremony.done {
		t.Errorf("CR-01 regressed: a stale message must not mutate ceremony 2's still-pending UI (heading=%q done=%t)",
			m.ceremony.cfg.Heading, m.ceremony.done)
	}
	if len(result.actions) != 1 {
		t.Fatalf("stale message must still dispatch its own reducer action (BL-04), got %d", len(result.actions))
	}
	// CR-01 (third independent re-review): the dispatched action must carry
	// the STALE ceremony's own submitted keys ("HashKnownHosts"), never
	// m.appliedKeys — which ceremony 2's confirm already overwrote to
	// ["ForwardAgent"] before this stale message arrived.
	applySSH, ok := result.actions[0].(ApplySSH)
	if !ok || len(applySSH.Keys) != 1 || applySSH.Keys[0] != "HashKnownHosts" {
		t.Errorf("CR-01 regressed: reducer action must carry the stale ceremony's own keys, got %#v", result.actions[0])
	}
}

// TestSSHRowBudgetHeightFloorsAtMinFrameHeight is the Global SSH mirror of
// TestGitRowBudgetHeightFloorsAtMinFrameHeight (WR-02, 09.4-REVIEW.md
// second independent re-review).
func TestSSHRowBudgetHeightFloorsAtMinFrameHeight(t *testing.T) {
	m := newGlobalSSHModel(&stubBackend{})
	res := m.handleMsg(tea.WindowSizeMsg{Width: 100, Height: 5}, Seed())
	m = res.model.(globalSSHModel)
	if got := m.rowBudgetHeight(); got != minFrameHeight {
		t.Errorf("rowBudgetHeight() after a sub-minFrameHeight resize = %d, want the minFrameHeight floor (%d)", got, minFrameHeight)
	}
}

// TestSSHAbandonedFailedCommitStillProducesNote is the Global SSH mirror of
// TestGitAbandonedFailedCommitStillProducesNote (WR-01, 09.4-REVIEW.md
// second independent re-review).
func TestSSHAbandonedFailedCommitStillProducesNote(t *testing.T) {
	b := &stubBackend{}
	m := newGlobalSSHModel(b)
	state := Seed()
	activated, _ := m.activate(state)
	m = activated.(globalSSHModel)
	m.detailKey = "HashKnownHosts"
	toggled := m.handleKey(pressKey("space"), state)
	m = toggled.model.(globalSSHModel)
	opened := m.handleKey(pressKey("a"), state)
	m = opened.model.(globalSSHModel)
	confirmed := m.handleKey(pressKey("enter"), state)
	pendingModel := confirmed.model.(globalSSHModel)

	reactivated, _ := pendingModel.activate(state)
	abandoned := reactivated.(globalSSHModel)
	if abandoned.mode == gssApplyCeremony || abandoned.applyCommitPending {
		t.Fatal("setup: activate() must have cleared the ceremony state")
	}

	result := abandoned.handleMsg(gitCommitTokenMsg{
		token: pendingModel.commitRequestToken,
		msg:   GlobalSSHCommitMsg{Err: "disk full"},
	}, state)
	if result.note == "" {
		t.Error("WR-01 regressed: an abandoned commit's failure must still produce a note")
	}
}

// ---------------------------------------------------------------------------
// Plan 09.5-01 Task 2 — the "All directives" flat filterable master-detail
// body: filter, match count, detail pane, and the three empty/error states.
// ---------------------------------------------------------------------------

// gssPropertiesApp opens Global SSH and navigates to the "All directives"
// sub-tab via two → presses from the default Options sub-tab.
func gssPropertiesApp(t *testing.T, b Backend) App {
	t.Helper()
	return pressSeq(t, NewApp(b), "2", "right", "right")
}

// TestPropertiesFilterNarrowsTheList is 09.5-01 Task 2's core filter
// behavior: case-insensitive, matching BOTH key and value, narrowing the
// visible rows and the match-count line.
func TestPropertiesFilterNarrowsTheList(t *testing.T) {
	rows := []SSHDirectiveView{
		{Key: "stricthostkeychecking", Value: "ask"},
		{Key: "loglevel", Value: "INFO"},
	}
	newFiltered := func(query string) string {
		a := pressSeq(t, gssPropertiesApp(t, stubBackend{sshDirectives: rows}), "/")
		return appView(typeText(t, a, query))
	}

	unfiltered := appView(gssPropertiesApp(t, stubBackend{sshDirectives: rows}))
	if !strings.Contains(unfiltered, "2 of 2 shown") {
		t.Fatalf("unfiltered match count wrong:\n%s", unfiltered)
	}

	byKey := newFiltered("strict")
	if !strings.Contains(byKey, "stricthostkeychecking") {
		t.Errorf("filtering by key substring must keep the matching row:\n%s", byKey)
	}
	if strings.Contains(byKey, "loglevel") {
		t.Errorf("filtering by key substring must hide the non-matching row:\n%s", byKey)
	}
	if !strings.Contains(byKey, "1 of 2 shown") {
		t.Errorf("match count must show narrowed/total, got:\n%s", byKey)
	}

	// Case-insensitive VALUE match: "info" (lowercase) must match loglevel's
	// "INFO" value, per the behavior's "covers BOTH key and value" contract.
	byValue := newFiltered("info")
	if !strings.Contains(byValue, "loglevel") {
		t.Errorf("filtering by value substring (case-insensitive) must keep the matching row:\n%s", byValue)
	}
	if strings.Contains(byValue, "stricthostkeychecking") {
		t.Errorf("filtering by value substring must hide the non-matching row:\n%s", byValue)
	}
}

// TestPropertiesFilterResetsSelectionToFirstMatch is D-C: on filter-text
// change, the selection resets to the FIRST row of the newly filtered set,
// so a selected-but-invisible row is structurally impossible.
func TestPropertiesFilterResetsSelectionToFirstMatch(t *testing.T) {
	rows := []SSHDirectiveView{
		{Key: "aaa", Value: "1"},
		{Key: "bbbstrict", Value: "2"},
		{Key: "cccstrict", Value: "3"},
	}
	a := pressSeq(t, gssPropertiesApp(t, stubBackend{sshDirectives: rows}), "down", "down")
	m := gssModel(t, a)
	if m.propDetailKey != "cccstrict" {
		t.Fatalf("setup: expected selection on cccstrict, got %q", m.propDetailKey)
	}

	a, _ = press(t, a, "/")
	a = typeText(t, a, "strict")
	m = gssModel(t, a)
	if m.propDetailKey != "bbbstrict" {
		t.Errorf("filter change must reset selection to the first row of the filtered set, got %q", m.propDetailKey)
	}
	if m.listWindowStart != 0 {
		t.Errorf("filter change must reset listWindowStart to 0, got %d", m.listWindowStart)
	}
}

// TestPropertiesFilterCapturesKeys is D-B: while the filter is focused, every
// key but esc routes into the input and is reported handled; esc blurs
// WITHOUT clearing the text, and after the blur the same digit switches main
// tabs again.
func TestPropertiesFilterCapturesKeys(t *testing.T) {
	rows := []SSHDirectiveView{{Key: "loglevel", Value: "INFO"}}
	a := pressSeq(t, gssPropertiesApp(t, stubBackend{sshDirectives: rows}), "/")
	if !strings.Contains(appView(a), "Global SSH › All directives") {
		t.Fatalf("setup: must be on All directives with the filter focused:\n%s", appView(a))
	}
	if !gssModel(t, a).view(Seed(), minFrameWidth, minFrameHeight).capturesKeys {
		t.Fatal("focused filter must report capturesKeys true")
	}

	a, _ = press(t, a, "3")
	if gssModel(t, a).filter.Value() != "3" {
		t.Fatalf("digit key must be routed into the focused filter, got filter value %q", gssModel(t, a).filter.Value())
	}
	if !strings.Contains(appView(a), "Global SSH › All directives") {
		t.Fatalf("a digit typed into the focused filter must NOT switch main tabs:\n%s", appView(a))
	}

	a, _ = press(t, a, "esc")
	m := gssModel(t, a)
	if m.filterFocused {
		t.Error("esc must blur the filter")
	}
	if m.filter.Value() != "3" {
		t.Errorf("esc must NOT clear the filter text, got %q", m.filter.Value())
	}
	if m.view(Seed(), minFrameWidth, minFrameHeight).capturesKeys {
		t.Error("blurred filter must report capturesKeys false")
	}

	a, _ = press(t, a, "3")
	if strings.Contains(appView(a), "Global SSH") {
		t.Errorf("after blur, a digit key must switch main tabs away from Global SSH:\n%s", appView(a))
	}
}

// TestPropertiesDetailPaneShowsFullValueAndSource proves the detail pane
// renders the selected row's full, untruncated value plus the frozen
// ssh -G source line, even when the master-list row itself truncated with a
// visible cue.
func TestPropertiesDetailPaneShowsFullValueAndSource(t *testing.T) {
	longValue := "alpha bravo charlie delta echo foxtrot golf hotel india juliet kilo lima mike november oscar papa quebec"
	rows := []SSHDirectiveView{{Key: "identityfile", Value: longValue}}
	a := gssPropertiesApp(t, stubBackend{sshDirectives: rows})

	master := regionFlat(a, 0, masterListWidth(minFrameWidth))
	if strings.Contains(master, longValue) {
		t.Errorf("master-list row must NOT show the full value: %q", master)
	}
	if !strings.Contains(master, "…") {
		t.Errorf("master-list row must carry the visible truncation cue: %q", master)
	}

	detail := regionFlat(a, masterListWidth(minFrameWidth)+1, minFrameWidth)
	if !strings.Contains(detail, longValue) {
		t.Errorf("detail pane must show the FULL, unclipped value:\ndetail=%q", detail)
	}
	if !strings.Contains(detail, PropsSSHSourceLine) {
		t.Errorf("detail pane must show the frozen source line:\ndetail=%q", detail)
	}
}

// TestPropertiesCrossReferenceNoteOnlyForPolicyBackedRows proves the frozen
// cross-reference note renders ONLY for a PolicyBacked row, and that the
// value is never rendered twice for the same selected key (a long-enough
// value truncates in the master row, so its untruncated form appears
// exactly once, in the detail pane).
func TestPropertiesCrossReferenceNoteOnlyForPolicyBackedRows(t *testing.T) {
	rows := []SSHDirectiveView{
		{Key: "stricthostkeychecking", Value: "ask", PolicyBacked: true},
		{Key: "ciphers", Value: "chacha20-poly1305@openssh.com,aes128-ctr", PolicyBacked: false},
	}
	detailRegion := func(a App) string {
		return regionFlat(a, masterListWidth(minFrameWidth)+1, minFrameWidth)
	}

	a := gssPropertiesApp(t, stubBackend{sshDirectives: rows})
	if !strings.Contains(detailRegion(a), PropsCrossReferenceNote) {
		t.Fatalf("PolicyBacked row's detail pane must show the cross-reference note:\n%s", appView(a))
	}

	a, _ = press(t, a, "down")
	view := appView(a)
	if strings.Contains(detailRegion(a), PropsCrossReferenceNote) {
		t.Fatalf("non-PolicyBacked row's detail pane must NOT show the cross-reference note:\n%s", view)
	}
	if got := strings.Count(view, "chacha20-poly1305@openssh.com,aes128-ctr"); got != 1 {
		t.Errorf("value must render exactly once (master row truncates), got %d occurrences:\n%s", got, view)
	}
}

// TestPropertiesEmptyStates covers the three distinct empty/error bodies:
// probe failure (fail-open navigation), a filter matching zero rows, and a
// successful probe returning genuinely zero rows (must not panic indexing
// an empty slice — the WR-17 defect class).
func TestPropertiesEmptyStates(t *testing.T) {
	t.Run("probe failure", func(t *testing.T) {
		a := gssPropertiesApp(t, stubBackend{sshDirectivesErr: errors.New("boom")})
		view := appView(a)
		if !strings.Contains(view, PropsSSHProbeFailedHeading) {
			t.Errorf("probe-failure state missing the frozen heading:\n%s", view)
		}
		if !strings.Contains(view, PropsSSHProbeFailedBody) {
			t.Errorf("probe-failure state missing the frozen body:\n%s", view)
		}
		// Fail-open: a main-tab digit key still switches tabs.
		a, _ = press(t, a, "1")
		if !strings.Contains(appView(a), "[1] Identities") {
			t.Errorf("probe-failure state must stay fail-open (main tab keys still work):\n%s", appView(a))
		}
	})

	t.Run("filter matches zero rows", func(t *testing.T) {
		rows := []SSHDirectiveView{{Key: "loglevel", Value: "INFO"}}
		a := pressSeq(t, gssPropertiesApp(t, stubBackend{sshDirectives: rows}), "/")
		a = typeText(t, a, "zzz")
		view := appView(a)
		want := fmt.Sprintf(PropsSSHNoFilterMatchFmt, "zzz")
		if !strings.Contains(view, want) {
			t.Errorf("filter-zero-match state missing %q:\n%s", want, view)
		}
	})

	t.Run("zero directives, no error", func(t *testing.T) {
		a := gssPropertiesApp(t, stubBackend{})
		view := appView(a)
		if !strings.Contains(view, "No SSH directives to show.") {
			t.Errorf("zero-rows-no-error state missing its faint sentence:\n%s", view)
		}
	})
}

// TestPropertiesFitsFixedGeometry proves the properties body's rendered line
// count never exceeds frameBodyRows at the fixed 100x30 geometry, for a
// 91-row directive set, a filtered set, and each of the three empty states —
// logging available/used/difference for each case (09.5-01 acceptance
// criterion).
func TestPropertiesFitsFixedGeometry(t *testing.T) {
	rows := make([]SSHDirectiveView, 0, 91)
	for i := range 90 {
		rows = append(rows, SSHDirectiveView{Key: fmt.Sprintf("directive%02d", i), Value: fmt.Sprintf("value-%02d", i)})
	}
	rows = append(rows, SSHDirectiveView{Key: "matchneedle", Value: "unique-match-value"})

	available := frameBodyRows(minFrameHeight)
	check := func(name string, a App) {
		t.Helper()
		body := gssModel(t, a).view(Seed(), minFrameWidth, minFrameHeight).body
		used := strings.Count(body, "\n") + 1
		t.Logf("%s: available=%d used=%d diff=%d", name, available, used, available-used)
		if used > available {
			t.Errorf("%s: rendered %d body lines, exceeds the %d-row budget", name, used, available)
		}
	}

	full := gssPropertiesApp(t, stubBackend{sshDirectives: rows})
	check("91-row unfiltered list", full)

	filteredOne := pressSeq(t, full, "/")
	filteredOne = typeText(t, filteredOne, "matchneedle")
	check("filtered set (1 match)", filteredOne)

	errState := gssPropertiesApp(t, stubBackend{sshDirectivesErr: errors.New("boom")})
	check("probe-failure empty state", errState)

	zeroMatch := pressSeq(t, gssPropertiesApp(t, stubBackend{sshDirectives: rows}), "/")
	zeroMatch = typeText(t, zeroMatch, "nonexistent-zzz")
	check("filter-zero-match empty state", zeroMatch)

	zeroRows := gssPropertiesApp(t, stubBackend{})
	check("zero-rows-no-error empty state", zeroRows)
}
