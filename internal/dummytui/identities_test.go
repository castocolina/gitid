package dummytui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// identitiesApp returns a fresh App (Identities tab active).
func identitiesApp() App { return NewApp() }

// pressSeq sends a sequence of keys to the app.
func pressSeq(t *testing.T, a App, keys ...string) App {
	t.Helper()
	for _, k := range keys {
		a, _ = press(t, a, k)
	}
	return a
}

// typeText types each rune of text into the app.
func typeText(t *testing.T, a App, text string) App {
	t.Helper()
	for _, r := range text {
		a, _ = press(t, a, string(r))
	}
	return a
}

// identModel extracts the identities child model from the app.
func identModel(t *testing.T, a App) identitiesModel {
	t.Helper()
	m, ok := a.screens[tabIdentities].(identitiesModel)
	if !ok {
		t.Fatalf("screens[0] is %T, want identitiesModel", a.screens[tabIdentities])
	}
	return m
}

// paneFlat extracts the detail-pane region (right of the sidebar) from the
// rendered frame and collapses whitespace, so assertions survive the
// pane's word-wrapping of long spec copy.
func paneFlat(a App) string {
	return regionFlat(a, sidebarWidth(a.width)+1, a.width)
}

// --------------------------------------------------------------------------
// Pips + live master-detail.
// --------------------------------------------------------------------------

func TestPipsMappingOverAllSeededRows(t *testing.T) {
	want := map[string][3]string{ // name → tone glyph, S pip, G pip (spec §2)
		"personal":   {"✓", "✓", "✓"},
		"work":       {"!", "✓", "–"},
		"opensource": {"!", "–", "✓"},
		"archived":   {"!", "–", "–"},
		"staging":    {"✓", "✓", "–"},
		"clientA":    {"✓", "✓", "✓"},
		"clientB":    {"✗", "✗", "–"},
		"legacy":     {"✗", "✓", "✗"},
	}
	for _, row := range Seed().Identities {
		expected, ok := want[row.Name]
		if !ok {
			t.Fatalf("unexpected seeded row %q", row.Name)
		}
		if glyph := IdentityManagerGlyphByState[row.State]; glyph != expected[0] {
			t.Errorf("%s tone glyph = %q, want %q", row.Name, glyph, expected[0])
		}
		s, g := pips(row)
		if s != expected[1] || g != expected[2] {
			t.Errorf("%s pips = S%s G%s, want S%s G%s", row.Name, s, g, expected[1], expected[2])
		}
	}
}

func TestLiveDetailArrowSelectionNoEnter(t *testing.T) {
	a := identitiesApp()
	before := appView(a)
	if !strings.Contains(before, "personal  ✓ complete") {
		t.Fatalf("initial detail header should show personal; view:\n%s", before)
	}
	// ONE ↓ — no Enter — and the SAME View() output shows the next detail.
	a, _ = press(t, a, "down")
	after := appView(a)
	if !strings.Contains(after, "work  ! incomplete") {
		t.Errorf("after ↓ the detail header must show work immediately (no Enter)")
	}
	if !strings.Contains(after, "S ssh · G git") {
		t.Error("sidebar legend line missing")
	}
}

func TestDetailShowsSSHFirstAndNeverFabricatesGit(t *testing.T) {
	a := pressSeq(t, identitiesApp(), "down") // → work (SSH only)
	pane := paneFlat(a)
	if !strings.Contains(pane, "SSH — shown first, always") {
		t.Error("SSH section heading missing")
	}
	if !strings.Contains(pane, "! Git not configured — no fabricated values shown.") {
		t.Error("SSH-only identity must show the no-fabrication warning (MGR-03)")
	}
	if !strings.Contains(pane, "Global baseline (inherited") || !strings.Contains(pane, "Edit in Global Git (3)") {
		t.Error("read-only global baseline strip missing (GITUI-01)")
	}
	if !strings.Contains(pane, "same data the Doctor shows (4)") {
		t.Error("findings sub-panel heading missing")
	}
}

// --------------------------------------------------------------------------
// Create wizard — state 1 validation.
// --------------------------------------------------------------------------

func TestWizardDuplicatePrefixBlocksNext(t *testing.T) {
	a := pressSeq(t, identitiesApp(), "n")
	view := appView(a)
	if !strings.Contains(view, "[1] SSH") || !strings.Contains(view, "New identity › SSH details") {
		t.Fatalf("wizard should open at step 1; view:\n%s", view)
	}
	// Type "personal" over the default prefix — a taken name.
	a = clearPrefixRaw(t, a)
	a = typeText(t, a, "personal")
	view = appView(a)
	if !strings.Contains(view, `"personal" already exists — pick another prefix.`) {
		t.Error("duplicate prefix must show the exact error copy")
	}
	// Next must be blocked.
	a, _ = press(t, a, "enter")
	if !strings.Contains(appView(a), "[1] SSH") {
		t.Error("Enter must not advance while the prefix duplicates an existing identity")
	}
}

// clearPrefixRaw backspaces the default prefix (4 chars "acme" + slack).
func clearPrefixRaw(t *testing.T, a App) App {
	t.Helper()
	for i := 0; i < 12; i++ {
		model, _ := a.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
		a = model.(App)
	}
	return a
}

func TestWizardBlankPrefixIsWYSIWYG(t *testing.T) {
	a := pressSeq(t, identitiesApp(), "n")
	a = clearPrefixRaw(t, a)
	m := identModel(t, a)
	if got := m.wizard.form.sshHost(); got != "github.com" {
		t.Errorf("blank prefix SSH Host = %q, want the provider host verbatim (WYSIWYG)", got)
	}
	if !strings.Contains(appView(a), "Blank prefix → SSH Host = the provider host itself") {
		t.Error("blank-prefix helper copy missing")
	}
}

func TestWizardManualHostEditTurnsAutoJoinOff(t *testing.T) {
	a := pressSeq(t, identitiesApp(), "n")
	if !strings.Contains(appView(a), "Auto-joined: <prefix>.<provider> — editable") {
		t.Fatal("auto-join helper missing before a manual edit")
	}
	// Tab from prefix → SSH Host, then type.
	a = pressSeq(t, a, "tab")
	a = typeText(t, a, "x")
	view := appView(a)
	if !strings.Contains(view, "Manually edited — auto-join off") {
		t.Error("manual host edit must flip the helper to auto-join off")
	}
	// Later prefix edits no longer change the host.
	m := identModel(t, a)
	hostBefore := m.wizard.form.sshHost()
	a = pressSeq(t, a, "shift+tab") // back to prefix
	a = typeText(t, a, "zz")
	m = identModel(t, a)
	if m.wizard.form.sshHost() != hostBefore {
		t.Errorf("prefix edits changed a manually-edited host: %q → %q", hostBefore, m.wizard.form.sshHost())
	}
}

func TestWizardPortAcceptsDigitsOnly(t *testing.T) {
	a := pressSeq(t, identitiesApp(), "n", "tab", "tab", "tab") // prefix → host → hostname → port
	m := identModel(t, a)
	if m.wizard.focus != sshFieldPort {
		t.Fatalf("focus = %d, want port", m.wizard.focus)
	}
	a = typeText(t, a, "a!x")
	m = identModel(t, a)
	if got := m.wizard.form.port.Value(); got != "443" {
		t.Errorf("port after typing letters = %q, want unchanged 443 (digits only)", got)
	}
	a = typeText(t, a, "22")
	m = identModel(t, a)
	if got := m.wizard.form.port.Value(); got != "44322" {
		t.Errorf("port after typing digits = %q, want 44322", got)
	}
}

func TestWizardSKAlgorithmsDisabledWithRationale(t *testing.T) {
	pane := paneFlat(pressSeq(t, identitiesApp(), "n"))
	if !strings.Contains(pane, "ed25519 — ★ recommended") {
		t.Error("ed25519 must render as the recommended default")
	}
	if !strings.Contains(pane, "Disabled: needs libfido2 + a FIDO2 security key — none detected on this machine") {
		t.Error("the -sk entries must render the libfido2 disabled rationale")
	}
	// ←/→ on the algorithm select must never land on a disabled entry.
	a := pressSeq(t, identitiesApp(), "n", "up", "up") // prefix → provider → algorithm (focus 5)
	m := identModel(t, a)
	if m.wizard.focus != 5 {
		t.Fatalf("focus = %d, want 5 (algorithm)", m.wizard.focus)
	}
	for i := 0; i < len(AlgorithmCatalog)+2; i++ {
		a, _ = press(t, a, "right")
		m = identModel(t, a)
		if algoDisabled(AlgorithmCatalog[m.wizard.algoIdx]) {
			t.Fatalf("algorithm select landed on the disabled entry %q", m.wizard.algo())
		}
	}
	if !strings.Contains(paneFlat(a), "Live Host-block preview — written exactly like this on confirm") {
		t.Error("live preview label missing")
	}
}

// --------------------------------------------------------------------------
// Create wizard — state 2 test stages.
// --------------------------------------------------------------------------

// wizardToStep2 opens the wizard with a fresh prefix and advances to the
// test stage.
func wizardToStep2(t *testing.T, a App) App {
	t.Helper()
	a = pressSeq(t, a, "n")
	a = clearPrefixRaw(t, a)
	a = typeText(t, a, "acme2")
	a, _ = press(t, a, "enter")
	if !strings.Contains(appView(a), "[2] Test") || !strings.Contains(appView(a), "New identity › Test connection") {
		t.Fatalf("wizard did not reach step 2:\n%s", appView(a))
	}
	return a
}

// completeStage completes the pending running phase via the tick message.
func completeStage(t *testing.T, a App, stage int) App {
	t.Helper()
	model, _ := a.Update(wizardStageMsg{stage: stage})
	return model.(App)
}

func TestWizardTestStageCommandsAndFlagOrder(t *testing.T) {
	a := wizardToStep2(t, identitiesApp())
	m := identModel(t, a)

	want1 := "ssh -T -F /tmp/gitid-test-a1b2c3.config -p 443 -i ~/.ssh/id_ed25519_acme2 git@ssh.github.com"
	if got := m.wizard.stage1Cmd(); got != want1 {
		t.Errorf("stage-1 command = %q, want %q (consistent flag order)", got, want1)
	}
	// Stage-1 flag order matches data.go's pinned command shape.
	if !strings.Contains(CreateFlowTestStage1Command, "ssh -T -F ") || !strings.Contains(CreateFlowTestStage1Command, " -p 443 -i ") {
		t.Error("data.go stage-1 fixture no longer pins the flag order this test mirrors")
	}

	cmd2 := m.wizard.stage2Cmd()
	if !strings.Contains(cmd2, "-G") || !strings.Contains(cmd2, "acme2.github.com") {
		t.Errorf("stage-2 command = %q, want -G + the alias", cmd2)
	}
	if strings.Contains(cmd2, "-i") {
		t.Errorf("stage-2 command must NOT contain -i (TEST-02 by design); got %q", cmd2)
	}

	// Run stage 1, complete the tick, assert the success line + rationale.
	a, _ = press(t, a, "enter")
	a = completeStage(t, a, 1)
	pane := paneFlat(a)
	if !strings.Contains(pane, "✓ Hi acme2! You've successfully authenticated") {
		t.Errorf("stage-1 success line missing:\n%s", pane)
	}
	if !strings.Contains(pane, "No -i here on purpose: the config must supply the key; that is exactly what this stage proves.") {
		t.Error("stage-2 no `-i` rationale missing from the render")
	}
}

func TestWizardSimulateFailToggleAndRetry(t *testing.T) {
	a := wizardToStep2(t, identitiesApp())

	// Toggle the demo failure control, run stage 1 → Permission denied path.
	a, _ = press(t, a, "space")
	m := identModel(t, a)
	if !m.wizard.simulateFail {
		t.Fatal("space must toggle the simulate-failure control")
	}
	a, _ = press(t, a, "enter")
	// Toggle locked while running.
	a, _ = press(t, a, "space")
	m = identModel(t, a)
	if !m.wizard.simulateFail {
		t.Error("toggle must lock while a stage is running")
	}
	a = completeStage(t, a, 1)
	view := appView(a)
	if !strings.Contains(view, "✗ git@ssh.github.com: Permission denied (publickey).") {
		t.Errorf("failure path output missing:\n%s", view)
	}
	if !strings.Contains(view, "Copy public key") {
		t.Error("failure path must offer Copy public key")
	}

	// Retry returns to idle and clears the toggle.
	a, _ = press(t, a, "enter")
	m = identModel(t, a)
	if m.wizard.testPhase != testIdle || m.wizard.simulateFail {
		t.Errorf("retry: phase=%q simulateFail=%v, want idle/false", m.wizard.testPhase, m.wizard.simulateFail)
	}

	// Pass both stages; the toggle then locks for good.
	a, _ = press(t, a, "enter")
	a = completeStage(t, a, 1)
	a, _ = press(t, a, "enter")
	a = completeStage(t, a, 2)
	m = identModel(t, a)
	if m.wizard.testPhase != testStage2 {
		t.Fatalf("phase = %q, want stage2", m.wizard.testPhase)
	}
	a, _ = press(t, a, "space")
	m = identModel(t, a)
	if m.wizard.simulateFail {
		t.Error("toggle must be disabled once the test has passed")
	}
	if !strings.Contains(appView(a), "✓ identityfile ~/.ssh/id_ed25519_acme2") {
		t.Error("stage-2 identityfile proof missing")
	}
}

// --------------------------------------------------------------------------
// Create wizard — full flow + skip flow.
// --------------------------------------------------------------------------

// wizardThroughTest gets a fresh wizard past both test stages.
func wizardThroughTest(t *testing.T, a App) App {
	t.Helper()
	a = wizardToStep2(t, a)
	a, _ = press(t, a, "enter")
	a = completeStage(t, a, 1)
	a, _ = press(t, a, "enter")
	a = completeStage(t, a, 2)
	a, _ = press(t, a, "enter") // → step 3 Git identity
	if !strings.Contains(appView(a), "[3] Git") || !strings.Contains(appView(a), "New identity › Git identity") {
		t.Fatalf("wizard did not reach step 3:\n%s", appView(a))
	}
	return a
}

func TestWizardFullFlowCreatesCompleteIdentity(t *testing.T) {
	a := wizardThroughTest(t, identitiesApp())
	pane := paneFlat(a)
	if !strings.Contains(pane, "Kept byte-identical to ~/.ssh/allowed_signers (GITUI-04)") {
		t.Error("email helper missing")
	}
	// The dedicated "Signing: ... a PATH, never key material" line and the
	// fragment preview's full body were both trimmed for row budget
	// (02-STYLE-SPEC.md §7, the field-contour + frozen-hint-lines cost) —
	// the fragment preview now shows its first line + the clip cue.
	if !strings.Contains(pane, "[user]") || !strings.Contains(pane, "more lines") {
		t.Error("fragment preview must still show its opening line + clip cue")
	}
	if !strings.Contains(pane, "gitdir (default) — applies inside ~/acme2/") {
		t.Error("default match-strategy copy missing")
	}
	if !strings.Contains(pane, "(fragment file — preview)") || !strings.Contains(pane, "(includeIf block — preview)") {
		t.Error("dual previews missing")
	}
	if !strings.Contains(pane, "[ Skip Git ]") {
		t.Error("skip button copy missing")
	}
	if !strings.Contains(pane, "Skip keeps this identity SSH-only and marks it incomplete.") {
		t.Error("skip hint copy missing")
	}

	idsBefore := len(a.state.Identities)
	a, _ = press(t, a, "enter") // [ Continue ] → ceremony
	view := appView(a)
	if !strings.Contains(view, `Create identity "acme2" — ed25519, test passed ✓`) {
		t.Fatalf("ceremony heading missing:\n%s", view)
	}
	if !strings.Contains(view, "~/.ssh/allowed_signers") {
		t.Error("git-configured ceremony must touch allowed_signers")
	}
	a, _ = press(t, a, "enter") // confirm → receipt
	if !strings.Contains(appView(a), "Wrote →") {
		t.Error("receipt missing Wrote → lines")
	}
	a, _ = press(t, a, "enter") // Done → dispatch

	if len(a.state.Identities) != idsBefore+1 {
		t.Fatalf("identity count = %d, want %d", len(a.state.Identities), idsBefore+1)
	}
	acme2 := findIdentity(t, a.state, "acme2")
	if acme2.State != "complete" {
		t.Errorf("full path state = %q, want complete", acme2.State)
	}
	// Sidebar gains the row; the header chip id count increments live.
	view = appView(a)
	if !strings.Contains(view, "9 ids") {
		t.Error("header chip id count must increment live")
	}
	m := identModel(t, a)
	if m.selected != "acme2" || m.pane != paneDetail {
		t.Errorf("wizard Done must select the new row in detail; selected=%q pane=%v", m.selected, m.pane)
	}
}

func TestWizardStrategySelectShowsAllThreeOptions(t *testing.T) {
	a := wizardThroughTest(t, identitiesApp())
	a = pressSeq(t, a, "tab", "tab") // name → email → strategy
	pane := paneFlat(a)
	for _, want := range []string{
		"gitdir (default) — applies inside ~/acme2/",
		"hasconfig — repos whose remote uses this alias",
		"both — either condition (two includeIf blocks = OR)",
	} {
		if !strings.Contains(pane, want) {
			t.Errorf("focused strategy select missing option copy %q", want)
		}
	}
	// ←/→ change the selection.
	a, _ = press(t, a, "right")
	m := identModel(t, a)
	if m.wizard.git.strategy() != "hasconfig" {
		t.Errorf("strategy after → = %q, want hasconfig", m.wizard.git.strategy())
	}
}

func TestWizardSkipCreatesIncompleteIdentity(t *testing.T) {
	a := wizardThroughTest(t, identitiesApp())
	// Tab past name/email/strategy/Back to the Skip button, then Enter (M2 —
	// Skip is a real focusable control, not a Ctrl+S chord).
	a = pressSeq(t, a, "tab", "tab", "tab", "tab")
	m := identModel(t, a)
	if m.wizard.gitFocus != gitFocusSkip {
		t.Fatalf("gitFocus = %d after 4 tabs, want the Skip button (%d)", m.wizard.gitFocus, gitFocusSkip)
	}
	a, _ = press(t, a, "enter") // activate [ Skip Git ]
	if !strings.Contains(appView(a), `Create identity "acme2"`) {
		t.Fatal("skip must still walk the review ceremony")
	}
	a, _ = press(t, a, "enter") // confirm
	a, _ = press(t, a, "enter") // done
	acme2 := findIdentity(t, a.state, "acme2")
	if acme2.State != "incomplete" {
		t.Errorf("skip path state = %q, want incomplete", acme2.State)
	}
	if acme2.Note != "SSH Host block present; no Git identity configured for this alias." {
		t.Errorf("skip note = %q", acme2.Note)
	}
	if a.note == "" || !strings.Contains(a.note, "SSH only (incomplete)") {
		t.Errorf("status note = %q, want the SSH-only note", a.note)
	}
}

// --------------------------------------------------------------------------
// Edit SSH — SAME form, identity fields locked.
// --------------------------------------------------------------------------

func TestEditSSHRendersSameFormWithLockedIdentityFields(t *testing.T) {
	a := pressSeq(t, identitiesApp(), "e")
	pane := paneFlat(a)
	if !strings.Contains(pane, "Edit SSH — personal") {
		t.Fatalf("edit pane missing:\n%s", pane)
	}
	for _, want := range []string{
		"Locked — the provider comes from the Host alias",
		"Locked — the identity name never changes in place; use Clone to rename",
		"SSH Host (alias)", "Real hostname", "Port",
	} {
		if !strings.Contains(pane, want) {
			t.Errorf("edit form missing %q", want)
		}
	}
	m := identModel(t, a)
	if !m.editForm.lockIdentity {
		t.Error("edit form must be the SAME component with lockIdentity=true")
	}

	// Enter opens the rewrite ceremony; confirm + done dispatch EditSSH.
	a, _ = press(t, a, "enter")
	if !strings.Contains(appView(a), `Rewrite the managed Host block for "personal"`) {
		t.Fatal("edit ceremony heading missing")
	}
	a, _ = press(t, a, "enter")
	a, _ = press(t, a, "enter")
	if a.note != `SSH settings of "personal" updated.` {
		t.Errorf("note = %q", a.note)
	}
}

// --------------------------------------------------------------------------
// Delete — scope chooser (safer default) + typed destructive confirm.
// --------------------------------------------------------------------------

func TestDeleteEverythingRequiresTypedNameAndRemovesFindings(t *testing.T) {
	a := identitiesApp()
	// Move to clientB (has a finding) — personal(0) → … → clientB(6).
	a = pressSeq(t, a, "down", "down", "down", "down", "down", "down")
	m := identModel(t, a)
	if m.selected != "clientB" {
		t.Fatalf("selected = %q, want clientB", m.selected)
	}
	a, _ = press(t, a, "d")
	view := appView(a)
	if !strings.Contains(view, "Delete Git identity only (safer — SSH stays)") ||
		!strings.Contains(view, "Delete everything (SSH + Git + key) — irreversible") {
		t.Fatalf("scope chooser copy missing:\n%s", view)
	}
	m = identModel(t, a)
	if m.deleteScope != "git-only" {
		t.Error("the safer scope must be default-focused")
	}
	a = pressSeq(t, a, "down", "enter") // choose everything → ceremony

	// Enter before typing the identity name is a no-op.
	a, _ = press(t, a, "enter")
	m = identModel(t, a)
	if m.pane != paneDelete || m.deleteCerem.done {
		t.Fatal("destructive delete must stay unconfirmed until the name is typed")
	}
	a = typeText(t, a, "clientB")
	a, _ = press(t, a, "enter") // confirm
	a, _ = press(t, a, "enter") // done
	if hasIdentity(a.state, "clientB") {
		t.Error("clientB should be deleted")
	}
	if hasFinding(a.state, "ssh-identitiesonly-contradiction") {
		t.Error("clientB's finding must be removed with it")
	}
	m = identModel(t, a)
	if m.selected == "clientB" {
		t.Error("delete-everything must re-select a fallback row")
	}
}

func TestDeleteGitOnlyHealsToIncomplete(t *testing.T) {
	a := pressSeq(t, identitiesApp(), "d", "enter") // personal, safer scope, ceremony
	a, _ = press(t, a, "enter")                     // confirm (no typed word needed)
	a, _ = press(t, a, "enter")                     // done
	personal := findIdentity(t, a.state, "personal")
	if personal.State != "incomplete" || personal.GitFragmentPath != "" {
		t.Errorf("git-only delete: state=%q fragment=%q, want incomplete/cleared", personal.State, personal.GitFragmentPath)
	}
	if !hasIdentity(a.state, "personal") {
		t.Error("git-only delete must keep the row")
	}
}

// --------------------------------------------------------------------------
// Clone.
// --------------------------------------------------------------------------

func TestCloneValidatesAndSelectsTheClone(t *testing.T) {
	a := pressSeq(t, identitiesApp(), "c")
	pane := paneFlat(a)
	if !strings.Contains(pane, "the Git author is copied (MGR-04)") {
		t.Error("clone explanation missing")
	}
	if !strings.Contains(pane, "Creates personal-clone.github.com + ~/.ssh/id_ed25519_personal-clone") {
		t.Error("clone helper missing")
	}
	a, _ = press(t, a, "enter")
	if !hasIdentity(a.state, "personal-clone") {
		t.Fatal("clone not created")
	}
	m := identModel(t, a)
	if m.selected != "personal-clone" || m.pane != paneDetail {
		t.Error("clone must be selected after creation")
	}
}

// --------------------------------------------------------------------------
// Per-finding fix — live healing.
// --------------------------------------------------------------------------

func TestFixLegacyFromDetailHealsAndDecrementsChip(t *testing.T) {
	a := identitiesApp()
	// legacy is the last row (index 7).
	for i := 0; i < 7; i++ {
		a, _ = press(t, a, "down")
	}
	m := identModel(t, a)
	if m.selected != "legacy" {
		t.Fatalf("selected = %q, want legacy", m.selected)
	}
	if !strings.Contains(appView(a), "✗ error") {
		t.Error("legacy detail must show its error finding with the severity word")
	}

	a, _ = press(t, a, "f")
	if !strings.Contains(appView(a), "Fix: includeIf targets a missing fragment") {
		t.Fatalf("fix ceremony missing:\n%s", appView(a))
	}
	a, _ = press(t, a, "enter") // confirm
	a, _ = press(t, a, "enter") // done

	legacy := findIdentity(t, a.state, "legacy")
	if legacy.State != "complete" {
		t.Errorf("legacy state = %q, want complete (healed live)", legacy.State)
	}
	if hasFinding(a.state, "git-includeif-missing-fragment") {
		t.Error("fixed finding must disappear")
	}
	// Header chip error count decremented 3 → 2.
	if !strings.Contains(appView(a), "✗ 2") {
		t.Error("header chip must decrement live after the fix")
	}
}

func TestFixFromIdentityPaneBacksUpThePlanFile(t *testing.T) {
	a := identitiesApp()
	// legacy is the last row (index 7); its fixable finding is
	// git-includeif-missing-fragment, whose fix plan targets
	// ~/.gitconfig.d/legacy — NOT ~/.ssh/config.
	for i := 0; i < 7; i++ {
		a, _ = press(t, a, "down")
	}
	if m := identModel(t, a); m.selected != "legacy" {
		t.Fatalf("selected = %q, want legacy", m.selected)
	}
	before := len(a.state.Backups)
	a = pressSeq(t, a, "f", "enter", "enter") // open fix, confirm, done
	if len(a.state.Backups) != before+1 {
		t.Fatalf("backups = %d, want %d — the fix must record exactly one backup", len(a.state.Backups), before+1)
	}
	if got := a.state.Backups[0]; !strings.HasPrefix(got, "~/.gitconfig.d/legacy.backup.") {
		t.Errorf("backup = %q, want the finding's plan file (~/.gitconfig.d/legacy.backup.*), matching doctor.go's dispatch", got)
	}
}

// --------------------------------------------------------------------------
// Esc never destructive.
// --------------------------------------------------------------------------

func TestEscFromEveryFormPaneReturnsToDetailWithoutDispatch(t *testing.T) {
	openers := map[string]string{"n": "New identity", "e": "Edit SSH", "g": "Configure Git", "c": "Clone", "d": "Delete"}
	for key, crumb := range openers {
		a := pressSeq(t, identitiesApp(), key)
		if !strings.Contains(appView(a), crumb) {
			t.Errorf("%q should open the %s pane", key, crumb)
			continue
		}
		before := a.state
		a, _ = press(t, a, "esc")
		m := identModel(t, a)
		if m.pane != paneDetail {
			t.Errorf("Esc from %s must return to detail", crumb)
		}
		if !stateEqual(before, a.state) {
			t.Errorf("Esc from %s must not dispatch anything", crumb)
		}
	}
}

func stateEqual(a, b DemoState) bool {
	return len(a.Identities) == len(b.Identities) && len(a.Findings) == len(b.Findings) &&
		len(a.Backups) == len(b.Backups) && a.GitBaselineApplied == b.GitBaselineApplied
}

// --------------------------------------------------------------------------
// Review batch 2 — visual structure, edit preview, focusable wizard
// buttons, footer honesty.
// --------------------------------------------------------------------------

func TestMasterDetailDividerOnEveryScreen(t *testing.T) {
	// Identities, Global SSH, Global Git, and the Doctor all draw the
	// full-height master↔detail divider (H2).
	for _, tab := range []string{"1", "2", "3"} {
		a, _ := press(t, NewApp(), tab)
		if n := strings.Count(appView(a), "│"); n < 15 {
			t.Errorf("tab %s: divider glyph count = %d, want a full-height │ column", tab, n)
		}
	}
	a := doctorApp(t)
	if n := strings.Count(appView(a), "│"); n < 15 {
		t.Errorf("doctor: divider glyph count = %d, want a full-height │ column", n)
	}
}

func TestIdentityDetailSectionHeadersReadAsGroups(t *testing.T) {
	a := identitiesApp()
	raw := a.View().Content
	// Bold+underline (SGR 1;4) section headers stand in for the web's
	// outlined Paper cards (H2).
	if !strings.Contains(raw, "\x1b[1;4") {
		t.Error("detail pane must render bold+underlined section headers")
	}
	plain := appView(a)
	for _, want := range []string{"SSH — shown first, always", "Git", "Findings (0)"} {
		if !strings.Contains(plain, want) {
			t.Errorf("detail pane missing section header %q", want)
		}
	}
}

func TestEditSSHShowsLiveHostBlockPreview(t *testing.T) {
	a := pressSeq(t, identitiesApp(), "e")
	pane := paneFlat(a)
	if !strings.Contains(pane, "Live Host-block preview — written exactly like this on confirm") {
		t.Fatalf("edit-SSH pane must show the same live preview the wizard shows (M1):\n%s", pane)
	}
	if !strings.Contains(pane, "Host personal.github.com") || !strings.Contains(pane, "IdentitiesOnly yes") {
		t.Errorf("live preview must render the current Host block:\n%s", pane)
	}
	// The preview rebuilds on every keystroke: type into the Port field.
	a = pressSeq(t, a, "tab", "tab") // host → hostname → port
	a = typeText(t, a, "9")
	if !strings.Contains(paneFlat(a), "Port 4439") {
		t.Errorf("preview must rebuild live on keystrokes:\n%s", paneFlat(a))
	}
}

func TestWizardGitStepButtonsAreFocusable(t *testing.T) {
	a := wizardThroughTest(t, identitiesApp())
	pane := paneFlat(a)
	for _, want := range []string{"Back (Esc)", "[ Skip Git ]", "[ Continue ]"} {
		if !strings.Contains(pane, want) {
			t.Errorf("Git step missing button %q (M2)", want)
		}
	}
	for _, want := range []string{"Skip keeps this identity SSH-only and marks it incomplete.", "Continue reviews the Git fragment, includeIf, and allowed_signers entries before writing."} {
		if !strings.Contains(pane, want) {
			t.Errorf("Git step missing the frozen adjacent hint %q", want)
		}
	}
	if strings.Contains(appView(a), "Ctrl+S") {
		t.Error("Ctrl+S must be gone — it is an XOFF hazard on IXON terminals")
	}

	// Tab ring: name → email → strategy → Back → Skip → Continue → name.
	a = pressSeq(t, a, "tab", "tab", "tab")
	m := identModel(t, a)
	if m.wizard.gitFocus != gitFocusBack {
		t.Fatalf("gitFocus = %d after 3 tabs, want Back (%d)", m.wizard.gitFocus, gitFocusBack)
	}
	// The focused button renders reverse-video like the ceremony buttons.
	raw := a.View().Content
	if !strings.Contains(raw, "\x1b[1;7m Back (Esc) ") && !strings.Contains(raw, "\x1b[7;1m Back (Esc) ") {
		t.Error("focused Back button must render reverse-video")
	}
	a = pressSeq(t, a, "tab", "tab", "tab")
	if got := identModel(t, a).wizard.gitFocus; got != gitFieldName {
		t.Errorf("gitFocus = %d after the full ring, want name (%d)", got, gitFieldName)
	}

	// Enter on Back returns to the test step; ctrl+s does nothing.
	a = pressSeq(t, a, "tab", "tab", "tab") // → Back
	a, _ = press(t, a, "enter")
	if got := identModel(t, a).wizard.step; got != 1 {
		t.Fatalf("Enter on Back: step = %d, want 1", got)
	}
	model, _ := a.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	a = model.(App)
	if got := identModel(t, a).wizard.step; got != 1 {
		t.Errorf("ctrl+s must be inert; step = %d", got)
	}
}

func TestWizardGitStepEnterOnFieldStillContinues(t *testing.T) {
	a := wizardThroughTest(t, identitiesApp()) // focus: name field
	a, _ = press(t, a, "enter")                // Enter on a field = Continue (web parity)
	if !strings.Contains(appView(a), `Create identity "acme2"`) {
		t.Error("Enter on a field must fall through to [ Continue ]")
	}
}

func TestReservedFooterHonestWhileInputFocused(t *testing.T) {
	// Detail mode: full reserved footer.
	a := identitiesApp()
	if view := appView(a); !strings.Contains(view, "q quit") || !strings.Contains(view, "? help") {
		t.Fatal("detail mode must keep the full reserved footer")
	}
	// A form pane with a focused text input: the honest variant only (L1).
	a = pressSeq(t, a, "c") // clone — its name input is always focused
	view := appView(a)
	for _, forbidden := range []string{"q quit", "? help"} {
		if strings.Contains(view, forbidden) {
			t.Errorf("footer must not advertise %q while an input swallows it", forbidden)
		}
	}
	for _, want := range []string{"Esc back", "Ctrl+P palette"} {
		if !strings.Contains(view, want) {
			t.Errorf("input-focused footer missing %q", want)
		}
	}
}

func TestWizardGitStepButtonsSurviveExpandedStrategySelect(t *testing.T) {
	// The expanded strategy select adds two rows; the fragment preview
	// yields two rows back so the button row never clips out of the frame.
	a := wizardThroughTest(t, identitiesApp())
	a = pressSeq(t, a, "tab", "tab") // → strategy focused (3 radio rows)
	view := appView(a)
	for _, want := range []string{"both — either condition", "[ Continue ]"} {
		if !strings.Contains(view, want) {
			t.Errorf("expanded-strategy Git step must keep %q on screen", want)
		}
	}
}

// --------------------------------------------------------------------------
// 02-14 round-2/round-3 checkpoint-feedback polish: first-class stepper,
// arrow-key precedence, field contours, hint persistence.
// --------------------------------------------------------------------------

func TestRenderStepperActiveSegmentIsBoldAccentNotFaint(t *testing.T) {
	active := renderStepper(1)
	plain := stripANSI(active)
	for _, want := range []string{"[1] SSH", "[2] Test", "[3] Git", "[4] Review"} {
		if !strings.Contains(plain, want) {
			t.Errorf("stepper must render all four segments; missing %q in %q", want, plain)
		}
	}
	if strings.Contains(active, styleFaint.Render("[2] Test")) {
		t.Error("the ACTIVE segment must NOT render through styleFaint (the old `Step n/4` line read dimmer than body text)")
	}
	if !strings.Contains(active, styleStepperActive.Render("[2] Test")) {
		t.Error("the active segment must render bold + accent (styleStepperActive)")
	}
	if !strings.Contains(plain, "✓") {
		t.Error("a completed segment (step 0, before the active step 1) must carry a ✓ glyph")
	}
}

func TestWizardArrowKeyPrecedenceStep0(t *testing.T) {
	// Clause 1: the algorithm select owns plain arrows (cycles the
	// catalog), never changing the wizard step.
	a := pressSeq(t, identitiesApp(), "n", "up", "up") // prefix → provider → algorithm (focus 5)
	if identModel(t, a).wizard.focus != 5 {
		t.Fatal("setup: expected algorithm focus")
	}
	a, _ = press(t, a, "right")
	if identModel(t, a).wizard.step != 0 {
		t.Error("clause 1: the algorithm select must own plain arrows, not change the wizard step")
	}

	// Clause 2: a focused text field keeps its cursor keys — plain arrows
	// never change the wizard step from a field focus.
	a2 := pressSeq(t, identitiesApp(), "n") // focus: Alias prefix (a text field)
	a2, _ = press(t, a2, "right")
	if identModel(t, a2).wizard.step != 0 {
		t.Error("clause 2: a focused text field must not change the wizard step on plain arrows")
	}

	// Clause 5: Shift+Right is a FOCUS-OVERRIDE reaching step-nav even from
	// inside the prefix field, still validity-gated forward (advances here
	// because the seeded defaults pass step0Valid).
	model, _ := a2.Update(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModShift})
	a2 = model.(App)
	if identModel(t, a2).wizard.step != 1 {
		t.Error("clause 5: shift+right must reach step-nav from inside a focused field and advance when valid")
	}
	// Shift+Left is always allowed back.
	model, _ = a2.Update(tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModShift})
	a2 = model.(App)
	if identModel(t, a2).wizard.step != 0 {
		t.Error("clause 5: shift+left must always go back")
	}
}

func TestWizardArrowKeyPrecedenceStep1(t *testing.T) {
	a := wizardToStep2(t, identitiesApp()) // wizard.step == 1 (test connection)

	// Forward is validity-gated on the two-stage test having passed —
	// blocked for BOTH plain and Shift+Right (never a validity override).
	a, _ = press(t, a, "right")
	if identModel(t, a).wizard.step != 1 {
		t.Error("right must be blocked at the test step until stage 2 passes")
	}
	model, _ := a.Update(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModShift})
	a = model.(App)
	if identModel(t, a).wizard.step != 1 {
		t.Error("shift+right must ALSO be blocked until stage 2 passes")
	}

	// Left always goes back.
	a, _ = press(t, a, "left")
	if identModel(t, a).wizard.step != 0 {
		t.Error("left must always go back from the test step")
	}
}

func TestWizardArrowKeyPrecedenceStep2(t *testing.T) {
	// Clause 2: a focused Git field (name) keeps its cursor keys.
	a := wizardThroughTest(t, identitiesApp()) // focus: git name field
	a, _ = press(t, a, "right")
	if identModel(t, a).wizard.step != 2 {
		t.Error("clause 2: a focused Git field must not change the wizard step on plain arrows")
	}

	// Clause 1: the strategy select owns plain arrows (cycles the option).
	a = pressSeq(t, a, "tab", "tab") // name → email → strategy
	a, _ = press(t, a, "right")
	m := identModel(t, a)
	if m.wizard.step != 2 {
		t.Error("clause 1: the strategy select must own plain arrows, not change the wizard step")
	}
	if m.wizard.git.strategy() != "hasconfig" {
		t.Error("clause 1: plain right on the strategy select must cycle the option")
	}

	// Clause 3: a button-slot focus (non-editing region) now performs
	// wizard-step navigation — replacing the old button-ring-arrow
	// behavior.
	a = pressSeq(t, a, "tab") // strategy → Back
	a, _ = press(t, a, "right")
	if got := identModel(t, a).wizard.step; got != 3 {
		t.Errorf("clause 3: right from a button slot must advance (validity-gated); step=%d", got)
	}
}

func TestWizardFocusedFieldRoundedContourVsBlurredDimBracket(t *testing.T) {
	a := pressSeq(t, identitiesApp(), "n") // step 0, Alias prefix focused
	raw := a.View().Content
	plain := paneFlat(a)
	if !strings.Contains(raw, "\x1b[34m") {
		t.Error("the focused field must carry the accent (blue, SGR 34) contour")
	}
	if !strings.Contains(plain, "╭─") {
		t.Error("the focused field must render a ROUNDED solid contour (╭─), distinct from a preview's dashed ╭╌ border")
	}
	// The Provider field (blurred at wizard open) renders a single-row dim
	// bracket contour — never a border on every field (row-budget trap).
	if !strings.Contains(plain, "[github.com]") {
		t.Errorf("blurred fields must render a dim bracket contour; pane:\n%s", plain)
	}
}

func TestGitFormMatchStrategyHintPersistsWhenExpanded(t *testing.T) {
	a := wizardThroughTest(t, identitiesApp())
	collapsed := paneFlat(a)
	if !strings.Contains(collapsed, "Determines which repos this Git identity applies to.") {
		t.Fatal("the reserved match-strategy hint must be present when collapsed")
	}
	a = pressSeq(t, a, "tab", "tab") // name → email → strategy (expanded)
	expanded := paneFlat(a)
	if !strings.Contains(expanded, "Determines which repos this Git identity applies to.") {
		t.Error("the reserved hint row must remain present when the strategy select expands (pushed, not replaced)")
	}
	if !strings.Contains(expanded, "hasconfig — repos whose remote uses this alias") {
		t.Error("expanding the select must show its option rows")
	}
}
