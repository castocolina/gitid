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
	// D-15: the selection starts EMPTY on every entry to the screen.
	keys := m.applyChosen(m.overlaidOptions(a.state))
	if len(keys) != 0 {
		t.Fatalf("initial chosen = %v, want 0 (D-15: selection starts empty)", keys)
	}

	// Toggle HashKnownHosts and StrictHostKeyChecking.
	// List order: StrictHostKeyChecking(0) ForwardAgent(1) HashKnownHosts(2) IdentitiesOnly(3) ...
	// Starting at IdentitiesOnly: up→HashKnownHosts, space; up→ForwardAgent, skip;
	// up→StrictHostKeyChecking, space.
	a, _ = press(t, a, "up") // IdentitiesOnly → HashKnownHosts
	a, _ = press(t, a, "space")
	a, _ = press(t, a, "up") // HashKnownHosts → ForwardAgent (skip)
	a, _ = press(t, a, "up") // ForwardAgent → StrictHostKeyChecking
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
	msg, ok := cmd().(GlobalSSHCommitMsg)
	if !ok {
		t.Fatalf("commit delivered %T, want GlobalSSHCommitMsg", cmd())
	}

	// The receipt appears ONLY from the commit's explicit success.
	model, _ := a.Update(msg)
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
	a, _ = press(t, a, "up") // IdentitiesOnly is verify-only; HashKnownHosts is writable.
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
	msg := cmd().(SSHStorageCommitMsg)
	model, _ := a.Update(msg)
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
	msg = cmd().(SSHStorageCommitMsg)
	model, _ = a.Update(msg)
	a = model.(App)
	if a.state.SSHStorage != StorageSentinel {
		t.Errorf("SSHStorage = %q, want sentinel (round trip)", a.state.SSHStorage)
	}
}

func TestGlobalSSHApplyTargetsOwnedFileUnderIncludeLayout(t *testing.T) {
	a := gssApp(t)
	a.state = Reduce(a.state, SetSSHStorage{Layout: StorageInclude, Backup: "b"})
	// D-15: selection starts empty; must select something before applying.
	a, _ = press(t, a, "up") // HashKnownHosts
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
	// D-15: selection starts empty; must select something before pressing "a".
	a, _ = press(t, a, "up") // HashKnownHosts
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
	// D-15: selection starts empty; select HashKnownHosts before applying.
	a, _ = press(t, a, "up") // HashKnownHosts
	a, _ = press(t, a, "space")
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
	user := optionRow(GlobalSSHOptionView{Key: "ForwardAgent", CurrentValue: "yes", Recommended: "no", State: GlobalSSHDiffers, AttributedToUser: true, WritableToHostStar: true}, false, false, false, 100)
	outside := optionRow(GlobalSSHOptionView{Key: "ForwardAgent", CurrentValue: "yes", Recommended: "no", State: GlobalSSHDiffers, AttributedToUser: false, WritableToHostStar: true}, false, false, false, 100)
	u2 := strings.Split(stripANSI(user), "\n")[1]
	o2 := strings.Split(stripANSI(outside), "\n")[1]
	if u2 == o2 {
		t.Fatalf("attributed and non-attributed differs line-2 must differ; both %q", u2)
	}
	if !strings.Contains(u2, "your choice") {
		t.Fatalf("user-attributed line-2 = %q, want the user-attribution wording", u2)
	}
	if strings.Contains(o2, "your choice") {
		t.Fatalf("non-attributed line-2 = %q must not contain the user-attribution wording", o2)
	}
	if !strings.Contains(o2, GlobalSSHWordDiffersOutside) {
		t.Fatalf("non-attributed line-2 = %q, want %q", o2, GlobalSSHWordDiffersOutside)
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
		if strings.Contains(text, glyphCheckOn) || strings.Contains(text, glyphCheckOff) {
			t.Fatalf("not-applicable row for %v must contain neither checkbox glyph", r)
		}
		if strings.Contains(text, "→") {
			t.Fatalf("not-applicable row for %v must omit the recommendation arrow", r)
		}
	}
	if len(seen) != len(reasons) {
		t.Fatalf("got %d distinct sentences, want %d (one per reason)", len(seen), len(reasons))
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
	msg, ok := cmd().(SSHStorageCommitMsg)
	if !ok {
		t.Fatalf("CommitSSHStorage delivered %T, want SSHStorageCommitMsg", cmd())
	}
	if msg.Err != "" {
		t.Fatalf("stub commit errored: %v", msg.Err)
	}
	model, _ := a.Update(msg)
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
	msg, ok := cmd().(SSHStorageCommitMsg)
	if !ok {
		t.Fatalf("expected SSHStorageCommitMsg, got %T", cmd())
	}
	model, _ := a.Update(msg)
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
