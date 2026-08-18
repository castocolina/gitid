package tuikit

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// identitiesApp returns a fresh App (Identities tab active).
func identitiesApp() App { return NewApp(stubBackend{}) }

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
	m, ok := a.screens[TabIdentities].(identitiesModel)
	if !ok {
		t.Fatalf("screens[0] is %T, want identitiesModel", a.screens[TabIdentities])
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
	if !strings.Contains(view, "Step 1/4") || !strings.Contains(view, "New identity › SSH details") {
		t.Fatalf("wizard should open at step 1; view:\n%s", view)
	}
	// Type "personal" over the default prefix — a taken name.
	a = clearPrefixRaw(t, a)
	a = typeText(t, a, "personal")
	view = appView(a)
	if !strings.Contains(view, "SSH Host alias already exists") {
		t.Error("duplicate alias must show the exact error copy")
	}
	// Next must be blocked.
	a, _ = press(t, a, "enter")
	if !strings.Contains(appView(a), "Step 1/4") {
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
	if !strings.Contains(pane, "Disabled: needs libfido2 + FIDO2 key") {
		t.Error("the -sk entries must render the libfido2 disabled rationale")
	}
	// ←/→ on the algorithm select must never land on a disabled entry.
	a := pressSeq(t, identitiesApp(), "n", "up") // prefix → algorithm
	m := identModel(t, a)
	if m.wizard.focus != wizardFocusKeyBody {
		t.Fatalf("focus = %d, want %d (algorithm)", m.wizard.focus, wizardFocusKeyBody)
	}
	for i := 0; i < len(AlgorithmCatalog)+2; i++ {
		a, _ = press(t, a, "right")
		m = identModel(t, a)
		if algoDisabled(AlgorithmCatalog[m.wizard.algoIdx]) {
			t.Fatalf("algorithm select landed on the disabled entry %q", m.wizard.algo())
		}
	}
	if !strings.Contains(paneFlat(a), "Live Host-block preview (written on confirm)") {
		t.Error("live preview label missing")
	}
}

// --------------------------------------------------------------------------
// D-20 / D-21 — the provider is READ OFF the SSH Host suffix and drives the
// Real hostname + Port autofill through the Backend's known-provider table.
// --------------------------------------------------------------------------

// clearFieldRaw backspaces n times over the focused input.
func clearFieldRaw(t *testing.T, a App, n int) App {
	t.Helper()
	for i := 0; i < n; i++ {
		model, _ := a.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
		a = model.(App)
	}
	return a
}

// wizardOnHostField opens the wizard on a backend answering the FULL provider
// table and leaves the SSH Host field focused and empty.
func wizardOnHostField(t *testing.T, b Backend) App {
	t.Helper()
	a := pressSeq(t, NewApp(b), "n", "tab") // wizard → SSH Host
	if identModel(t, a).wizard.focus != sshFieldHost {
		t.Fatalf("setup: focus = %d, want the SSH Host field", identModel(t, a).wizard.focus)
	}
	return clearFieldRaw(t, a, 40)
}

// TestSSHHostSuffixDrivesEndpointAutofill is D-20: editing the SSH Host suffix
// consults the known-provider table and autofills Real hostname + Port — the
// recipe alt-SSH pairing for a known provider, the host itself on 22 for an
// unknown one (D-21). No new field is involved: the provider is inferred from
// the alias the user is already typing.
func TestSSHHostSuffixDrivesEndpointAutofill(t *testing.T) {
	cases := []struct{ host, hostname, port string }{
		{"personal.github.com", "ssh.github.com", "443"},
		{"work.gitlab.com", "altssh.gitlab.com", "443"},
		{"side.bitbucket.org", "altssh.bitbucket.org", "443"},
		{"git.internal.example", "internal.example", "22"},
	}
	for _, tc := range cases {
		t.Run(tc.host, func(t *testing.T) {
			a := typeText(t, wizardOnHostField(t, tableBackend{}), tc.host)
			form := identModel(t, a).wizard.form
			if got := form.hostname.Value(); got != tc.hostname {
				t.Errorf("Real hostname = %q, want %q", got, tc.hostname)
			}
			if got := form.port.Value(); got != tc.port {
				t.Errorf("Port = %q, want %q", got, tc.port)
			}
		})
	}
}

// TestSSHHostSuffixAutofillStaysEditable proves the autofilled values remain
// the user's: once Real hostname or Port is hand-edited, endpointTouched
// suppresses every later re-autofill (the approved behavior, preserved).
func TestSSHHostSuffixAutofillStaysEditable(t *testing.T) {
	a := typeText(t, wizardOnHostField(t, tableBackend{}), "work.gitlab.com")
	// Hand-edit the Real hostname.
	a = pressSeq(t, a, "tab")
	a = clearFieldRaw(t, a, 40)
	a = typeText(t, a, "ssh.corp.example")
	// Back to the alias, retype a KNOWN provider suffix.
	a = pressSeq(t, a, "shift+tab")
	a = clearFieldRaw(t, a, 40)
	a = typeText(t, a, "personal.github.com")

	form := identModel(t, a).wizard.form
	if got := form.hostname.Value(); got != "ssh.corp.example" {
		t.Errorf("a hand-edited Real hostname was clobbered: %q", got)
	}
	if !form.endpointTouched {
		t.Error("endpointTouched must stay set after a manual endpoint edit")
	}
}

// TestUnknownProviderShowsAltSSHHint is D-21: an unknown/custom host defaults
// to port 22 and says WHY on the existing Port row — the warning glyph plus the
// word, never color alone, and never a new row.
func TestUnknownProviderShowsAltSSHHint(t *testing.T) {
	known := typeText(t, wizardOnHostField(t, tableBackend{}), "personal.github.com")
	if strings.Contains(paneFlat(known), altSSHHint) {
		t.Error("a KNOWN provider must not carry the unknown-provider hint")
	}
	unknown := typeText(t, wizardOnHostField(t, tableBackend{}), "git.internal.example")
	if !strings.Contains(paneFlat(unknown), altSSHHint) {
		t.Errorf("an unknown provider must explain the port-22 default (D-21):\n%s", paneFlat(unknown))
	}
	if !strings.HasPrefix(altSSHHint, "! ") {
		t.Errorf("the hint must pair its warning color with the `!` glyph and a word; got %q", altSSHHint)
	}
}

// TestLivePreviewIsRecipeFaithful is SSHUI-03: the live Host-block preview is
// the block that will be written, matching recipes/ssh-config.recipe's shape —
// alias, alt-SSH Hostname, Port 443, User git, IdentityFile, IdentitiesOnly yes.
func TestLivePreviewIsRecipeFaithful(t *testing.T) {
	a := typeText(t, wizardOnHostField(t, tableBackend{}), "work.gitlab.com")
	pane := paneFlat(a)
	for _, want := range []string{
		"Host work.gitlab.com",
		"Hostname altssh.gitlab.com",
		"Port 443",
		"User git",
		"IdentityFile ~/.ssh/id_ed25519_acme",
		"IdentitiesOnly yes",
	} {
		if !strings.Contains(pane, want) {
			t.Errorf("live preview missing the recipe-canonical %q:\n%s", want, pane)
		}
	}
}

// TestBlankPrefixPreviewIsWYSIWYG proves the blank-prefix rule survives all the
// way into the preview: SSH Host is the provider host VERBATIM, with no
// invented `.`-joined suffix, and the previewed block says exactly that.
func TestBlankPrefixPreviewIsWYSIWYG(t *testing.T) {
	a := pressSeq(t, NewApp(tableBackend{}), "n")
	a = clearPrefixRaw(t, a)
	pane := paneFlat(a)
	if !strings.Contains(pane, "Host github.com") {
		t.Errorf("blank prefix must preview `Host github.com` verbatim:\n%s", pane)
	}
	if strings.Contains(pane, "Host .github.com") {
		t.Error("blank prefix must never invent a leading-dot alias")
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
	if !strings.Contains(appView(a), "Step 2/4") || !strings.Contains(appView(a), "New identity › Test connection") {
		t.Fatalf("wizard did not reach step 2:\n%s", appView(a))
	}
	return a
}

// completeStage completes the pending running phase by delivering the
// WizardStageMsg the injected Backend's TestStage1/TestStage2 command would
// deliver for the wizard's CURRENT spec — same result, without waiting out
// the demo's running tick. Building it from the Backend (rather than a
// hand-made message) keeps the outcome — including the simulate-failure
// path — decided by the seam, exactly as it is at runtime.
//
// Stage-1 success auto-chains into stage 2 (D-04), so completing stage 1 also
// drains the returned stage-2 command and feeds its result back in, leaving
// the wizard in the final answered state.
func completeStage(t *testing.T, a App, stage int) App {
	t.Helper()
	spec := identModel(t, a).wizard.spec()
	b := stubBackend{}
	result := b.stage1Result(spec)
	if stage == 2 {
		result = b.stage2Result(spec)
	}
	model, cmd := a.Update(WizardStageMsg{Stage: stage, Result: result})
	a = model.(App)
	if stage == 1 && cmd != nil {
		if next, ok := cmd().(WizardStageMsg); ok && next.Stage == 2 {
			return completeStage(t, a, 2)
		}
	}
	return a
}

// completeCommit confirms an async create ceremony and drains the resulting
// backend commit command, returning the model with the receipt showing.
func completeCommit(t *testing.T, a App) App {
	t.Helper()
	a, cmd := press(t, a, "enter") // confirm
	if cmd == nil {
		return a
	}
	msg, ok := cmd().(WizardCommitMsg)
	if !ok {
		return a
	}
	model, _ := a.Update(msg)
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

	// TEST-01 shown==run: once stage 1 has answered, the rendered command
	// line is the CAPTURED TestResultView.Command — never a freshly
	// recomputed string that could drift from what actually ran.
	m = identModel(t, a)
	if m.wizard.stage1Cmd() != m.wizard.stage1.Command {
		t.Errorf("stage1Cmd() = %q, want the captured TestResultView.Command %q", m.wizard.stage1Cmd(), m.wizard.stage1.Command)
	}

	// D-03's copy-.pub action is ABSENT at a plain PASS — it is
	// warning-only, never offered when the key already authenticated.
	if strings.Contains(appView(a), "copy public key") {
		t.Error("a PASS outcome must never offer the copy-public-key footer action (D-03 is warning-only)")
	}
}

// recordingCopyBackend wraps stubBackend to capture the exact path
// CopyPublicKey was called with — proving D-03 never leaks private key
// material, only the .pub line's path, across the clipboard seam.
type recordingCopyBackend struct {
	stubBackend
	copiedPath string
}

func (b *recordingCopyBackend) CopyPublicKey(path string) (string, error) {
	b.copiedPath = path
	return "Public key copied to clipboard (demo).", nil
}

// TestCopyPubSeamReceivesOnlyThePubPath proves D-03/T-03-17: pressing "c" at
// the ReachableNotUploaded warning state calls the Backend's clipboard seam
// with exactly the .pub path — never the private key path or its material.
func TestCopyPubSeamReceivesOnlyThePubPath(t *testing.T) {
	rec := &recordingCopyBackend{}
	a := wizardToStep2(t, NewApp(rec))
	a, _ = press(t, a, "space") // preview the D-02 warning path
	a, _ = press(t, a, "enter")
	a = completeStage(t, a, 1)

	_, _ = press(t, a, "c")
	want := "~/.ssh/id_ed25519_acme2.pub"
	if rec.copiedPath != want {
		t.Errorf("CopyPublicKey called with %q, want %q (the .pub path, never the private key)", rec.copiedPath, want)
	}
}

// TestFrozenReachableWarningAndKeyUnusedCopy pins the two D-02/D-01 frozen
// strings this plan registers with the §6 copy-freeze mechanism BYTE-EXACT —
// this is the Go-test half of the copy-freeze gate, alongside the Makefile
// grep target. If either string drifts, `make test` goes red.
func TestFrozenReachableWarningAndKeyUnusedCopy(t *testing.T) {
	if stageWarningLine != "! Reachable — key not uploaded yet" {
		t.Errorf("stageWarningLine = %q, want the frozen D-02 draft copy", stageWarningLine)
	}
	if keyUnusedResultMessage != "Stored — key not uploaded yet; this identity is not proven for Git yet" {
		t.Errorf("keyUnusedResultMessage = %q, want the frozen D-01 store-copy", keyUnusedResultMessage)
	}

	// Both strings are actually rendered, not just declared: the render for
	// TestOutcomeReachableNotUploaded (renderStageOutcome) and the
	// key-unused store ceremony (reviewCeremony) both use them, byte-exact.
	rendered := renderStageOutcome(TestResultView{Outcome: TestOutcomeReachableNotUploaded}, "github.com", true, 62)
	if !strings.Contains(rendered, stageWarningLine) {
		t.Errorf("renderStageOutcome does not render stageWarningLine:\n%s", rendered)
	}
}

// TestRenderStageOutcomeShowHint pins design-review finding F2 (post-03-05
// retroactive DLV-02 critique): the D-03 instruction line is rendered once,
// controlled by the showHint parameter, not unconditionally repeated on
// every ReachableNotUploaded stage — the common case is both stages
// resolving to ReachableNotUploaded for the same key, and repeating the
// identical instruction twice reads as an error loop rather than one
// coherent state.
func TestRenderStageOutcomeShowHint(t *testing.T) {
	r := TestResultView{Outcome: TestOutcomeReachableNotUploaded, Detail: "git@ssh.github.com: Permission denied (publickey)."}

	withHint := renderStageOutcome(r, "github.com", true, 62)
	if !strings.Contains(withHint, "Press c to copy the .pub") {
		t.Errorf("renderStageOutcome(showHint=true) missing the D-03 instruction:\n%s", withHint)
	}
	if !strings.Contains(withHint, r.Detail) {
		t.Errorf("renderStageOutcome(showHint=true) does not surface the real ssh output (F1):\n%s", withHint)
	}

	withoutHint := renderStageOutcome(r, "github.com", false, 62)
	if strings.Contains(withoutHint, "Press c to copy the .pub") {
		t.Errorf("renderStageOutcome(showHint=false) still rendered the instruction line:\n%s", withoutHint)
	}
	if !strings.Contains(withoutHint, stageWarningLine) {
		t.Errorf("renderStageOutcome(showHint=false) dropped the frozen warning line:\n%s", withoutHint)
	}
}

// TestReachableHintNamesKeystrokeAndURL pins design-review finding F3: the
// D-03 hint names the copy keystroke and a directly navigable provider URL
// (not just a label), and falls back to the existing label for an unknown
// provider (no URL can be guessed for it).
func TestReachableHintNamesKeystrokeAndURL(t *testing.T) {
	cases := map[string]string{
		"github.com":    "github.com/settings/keys",
		"gitlab.com":    "gitlab.com/-/user_settings/ssh_keys",
		"bitbucket.org": "bitbucket.org/account/settings/ssh-keys/",
		"example.com":   "your provider's key settings",
	}
	for provider, want := range cases {
		hint := reachableHint(provider)
		if !strings.HasPrefix(hint, "Press c to copy the .pub, then add it at ") {
			t.Errorf("reachableHint(%q) = %q, want the keystroke+destination framing", provider, hint)
		}
		if !strings.Contains(hint, want) {
			t.Errorf("reachableHint(%q) = %q, want it to contain %q", provider, hint, want)
		}
	}
}

// TestIdentityNameFollowsEditedHost pins design-review finding D-20/DLV-04.2:
// with a blank Alias prefix, editing the SSH Host field to a DIFFERENT provider
// must rename the identity after the EDITED provider suffix, not the original
// default provider — identityName() previously read a separate Provider field
// that the Host-edit path never updated, silently producing the WRONG persisted
// identity name (e.g. still "github" after editing SSH Host to gitlab.com).
func TestIdentityNameFollowsEditedHost(t *testing.T) {
	f := newSSHForm(tableBackend{}, "", "", "ssh.github.com", "443", false)
	if got := f.identityName(); got != "github" {
		t.Fatalf("identityName() before any edit = %q, want %q", got, "github")
	}

	f = f.setFocus(sshFieldHost)
	for _, r := range "gitlab.com" {
		f = f.handleEdit(pressKey(string(r)), sshFieldHost)
	}

	if got := f.identityName(); got != "gitlab" {
		t.Errorf("identityName() after editing SSH Host to gitlab.com = %q, want %q (D-20 edited-alias-wins)", got, "gitlab")
	}
}

// TestReviewCeremonyKeyUnusedResultHint pins design-review finding F4.2: the
// create-flow's receipt (state B) for a key-unused (ReachableNotUploaded)
// store repeats the same keystroke+URL instruction the test-stage warning
// showed — the receipt is the last screen before the user leaves gitid to
// open a browser, so the instruction must survive past the wizard closing.
// A PASS ceremony carries no ResultHint (nothing left to finish).
func TestReviewCeremonyKeyUnusedResultHint(t *testing.T) {
	w := newWizard(stubBackend{})
	w.stage1 = TestResultView{Outcome: TestOutcomeReachableNotUploaded, Detail: "git@ssh.github.com: Permission denied (publickey)."}
	w.stage2 = w.stage1

	c := w.reviewCeremony()
	if c.cfg.ResultHint == "" {
		t.Fatal("reviewCeremony().ResultHint is empty for a key-unused store; want the F4.2 follow-on instruction")
	}
	if !strings.Contains(c.cfg.ResultHint, "Press c to copy the .pub") {
		t.Errorf("ResultHint = %q, want the D-03 keystroke+URL instruction", c.cfg.ResultHint)
	}

	w.stage1 = TestResultView{Outcome: TestOutcomePass, Detail: "git@ssh.github.com: Hi acme! You've successfully authenticated."}
	w.stage2 = w.stage1
	passC := w.reviewCeremony()
	if passC.cfg.ResultHint != "" {
		t.Errorf("ResultHint = %q, want empty on a full PASS store", passC.cfg.ResultHint)
	}
}

// TestWizardSimulateFailToggleAndRetry proves D-02/D-04 (Pitfall 6): the
// dummy's demo control previews "Permission denied (publickey)", which is
// the ReachableNotUploaded WARNING state — never a hard failure. It must
// render the yellow `!` (never red `✗`), offer the D-03 copy-.pub action,
// and STILL chain straight into stage 2 on Enter (D-04), because the D-01
// store gate already unlocked.
func TestWizardSimulateFailToggleAndRetry(t *testing.T) {
	a := wizardToStep2(t, identitiesApp())

	// Toggle the demo failure control, run stage 1 → the D-02 warning path.
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
	a = completeStage(t, a, 1) // stage 1 auto-chains into stage 2 (D-04)
	m = identModel(t, a)
	if m.wizard.testPhase != testStage2 {
		t.Fatalf("phase = %q, want stage2 (stage 1 auto-chained into stage 2)", m.wizard.testPhase)
	}
	if m.wizard.stage1.Outcome != TestOutcomeReachableNotUploaded {
		t.Fatalf("stage1 outcome = %v, want TestOutcomeReachableNotUploaded", m.wizard.stage1.Outcome)
	}
	pane := paneFlat(a)
	if !strings.Contains(pane, "! Reachable — key not uploaded yet") {
		t.Errorf("D-02 warning line missing:\n%s", pane)
	}
	// The wizard pane itself must never show the red failure line for this
	// outcome (Pitfall 6). The app-wide header's identity-count chip
	// legitimately contains a "✗" glyph for unrelated seeded rows, so this
	// checks the SPECIFIC failure-styled line, not a blanket glyph absence.
	if strings.Contains(pane, "✗ git@") || strings.Contains(pane, "connection failed") {
		t.Error("ReachableNotUploaded must NEVER render the red ✗ failure line (Pitfall 6)")
	}
	// The copy action is a FOOTER/keybar affordance (never a second inline
	// body row, D-03), so it is checked against the raw multi-line view —
	// paneFlat's column slice only covers the two-pane split above the
	// footer, not the full-width footer row below it.
	if !strings.Contains(appView(a), "copy public key") {
		t.Error("warning path must offer the copy-public-key footer action (D-03)")
	}

	// "c" copies the public key at the warning state.
	a2, _ := press(t, a, "c")
	if !strings.Contains(a2.note, "Public key copied to clipboard") {
		t.Errorf("copy note = %q, want a clipboard receipt", a2.note)
	}

	if !strings.Contains(paneFlat(a), "✓ identityfile ~/.ssh/id_ed25519_acme2") {
		t.Errorf("stage-2 identityfile proof missing:\n%s", paneFlat(a))
	}
	// Once past, the simulate-failure toggle is LOCKED — space is a no-op —
	// even though its underlying value (set by the earlier toggle press,
	// never reset by this success chain) stays true; the render says so.
	a, _ = press(t, a, "space")
	if !strings.Contains(paneFlat(a), "Demo failure control — locked") {
		t.Error("toggle must render as locked once the test has passed")
	}
}

// TestWizardHardFailureRendersRedAndRetriesNoCopy proves the OTHER side of
// D-02/Pitfall 6: a genuine TestOutcomeFailure (connection refused, DNS,
// timeout — never simulated by the dummy's own demo control) renders red
// `✗` with a retry affordance, and NEVER offers the D-03 copy-.pub action
// (that is warning-only).
func TestWizardHardFailureRendersRedAndRetriesNoCopy(t *testing.T) {
	a := wizardToStep2(t, identitiesApp())
	a, _ = press(t, a, "enter") // → testRunning1
	fail := TestResultView{
		Outcome: TestOutcomeFailure,
		Command: identModel(t, a).wizard.stage1Cmd(),
		Detail:  "ssh: connect to host ssh.github.com port 443: Connection refused",
	}
	model, _ := a.Update(WizardStageMsg{Stage: 1, Result: fail})
	a = model.(App)
	m := identModel(t, a)
	if m.wizard.testPhase != testFailed {
		t.Fatalf("phase = %q, want failed", m.wizard.testPhase)
	}
	pane := paneFlat(a)
	if !strings.Contains(pane, "✗ ssh: connect to host ssh.github.com port 443: Connection refused") {
		t.Errorf("hard-failure line missing the real ssh output:\n%s", pane)
	}
	if strings.Contains(appView(a), "copy public key") {
		t.Error("a hard Failure must never offer the copy-public-key footer action (D-03 is warning-only)")
	}
	if !strings.Contains(pane, "Retry (Enter)") {
		t.Error("hard-failure retry affordance missing")
	}

	// "c" is a no-op at a hard failure.
	a2, _ := press(t, a, "c")
	if a2.note != "" {
		t.Errorf("c must be a no-op at a hard failure; note = %q", a2.note)
	}

	// Enter retries: back to idle, toggle cleared.
	a, _ = press(t, a, "enter")
	m = identModel(t, a)
	if m.wizard.testPhase != testIdle {
		t.Errorf("retry: phase = %q, want idle", m.wizard.testPhase)
	}
}

// TestReachableNotUploadedStoresKeyUnusedCopy proves the D-01/D-02 "no
// silent success" contract at the ceremony level: once EITHER test stage
// answered ReachableNotUploaded, the confirm-write heading and the receipt
// both say so plainly, byte-exact — the frozen key-unused copy this plan
// registers with the §6 copy-freeze mechanism.
func TestReachableNotUploadedStoresKeyUnusedCopy(t *testing.T) {
	a := wizardToStep2(t, identitiesApp())
	a, _ = press(t, a, "space") // preview the D-02 warning path
	a, _ = press(t, a, "enter")
	a = completeStage(t, a, 1)                     // stage-1 warning auto-chains into stage 2 (D-04)
	a, _ = press(t, a, "enter")                    // → step 2 Git identity
	a = pressSeq(t, a, "tab", "tab", "tab", "tab") // → Skip button
	a, _ = press(t, a, "enter")                    // activate [ Skip Git ] → ceremony

	pane := paneFlat(a)
	if !strings.Contains(pane, `Create identity "acme2" — ed25519, reachable — key not uploaded yet`) {
		t.Fatalf("ceremony heading must name the warning outcome, never a false test-passed framing:\n%s", pane)
	}
	if strings.Contains(pane, "test passed ✓") {
		t.Error("a ReachableNotUploaded store must never claim \"test passed\"")
	}

	a = completeCommit(t, a) // confirm → async commit → receipt
	receipt := paneFlat(a)
	if !strings.Contains(receipt, "Stored — key not uploaded yet; this identity is not proven for Git yet") {
		t.Errorf("receipt missing the frozen key-unused copy:\n%s", receipt)
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
	a = completeStage(t, a, 1)  // stage-1 success auto-chains into stage 2 (D-04)
	a, _ = press(t, a, "enter") // → step 2 Git identity
	if !strings.Contains(appView(a), "Step 3/4") || !strings.Contains(appView(a), "New identity › Git identity") {
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
	a = completeCommit(t, a) // confirm → async commit → receipt
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
	a = completeCommit(t, a)    // confirm → async commit → receipt
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

// TestWizardGitContinueForcedDisabledByBackend proves D-19's render side: a
// Backend that reports alwaysDisabled=true (the REAL binary's shape) forces
// the create wizard's Git-step [ Continue ] disabled with the Backend's OWN
// reason string, regardless of the form's own validity — the dummy-style
// validity gate (gitFormDisabledSuffix) never applies here.
func TestWizardGitContinueForcedDisabledByBackend(t *testing.T) {
	const realReason = "— Git configuration arrives with the next build"
	b := stubBackend{gitStepAlwaysDisabled: true, gitStepReason: realReason}
	// newWizard's own defaults ("Acme Identity" / "you@acme.example") are
	// already a FULLY VALID Git form — Continue must still stay disabled.
	a := wizardThroughTest(t, NewApp(b))

	pane := paneFlat(a)
	if !strings.Contains(pane, realReason) {
		t.Errorf("real-binary disabled reason missing from the render:\n%s", pane)
	}
	if strings.Contains(pane, gitFormDisabledSuffix) {
		t.Error("the real binary must never show the dummy's validity-based reason (D-19)")
	}

	// Continue is unreachable — Enter on a filled-valid form does NOT advance.
	before := identModel(t, a).wizard.step
	a, _ = press(t, a, "enter")
	if identModel(t, a).wizard.step != before {
		t.Error("Continue must never advance the wizard when the Backend forces it disabled (D-19)")
	}

	// Skip Git remains the ONLY functional path forward (D-18).
	a = pressSeq(t, a, "tab", "tab", "tab", "tab") // name → email → strategy → Back → Skip
	if identModel(t, a).wizard.gitFocus != gitFocusSkip {
		t.Fatalf("gitFocus = %d, want Skip", identModel(t, a).wizard.gitFocus)
	}
	a, _ = press(t, a, "enter")
	if !strings.Contains(appView(a), `Create identity "acme2"`) {
		t.Error("Skip must still walk the review ceremony even with Continue force-disabled")
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
		a, _ := press(t, NewApp(stubBackend{}), tab)
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
	if !strings.Contains(pane, "Live Host-block preview (written on confirm)") {
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

// TestRenderStepperRevertsToStepCounterWithLongLabel pins D5 (checkpoint-2
// contract): the stepper REVERTS from 02-14's bracketed short-segment
// stepper (bracket-number + short word per step, joined by middots) to
// `Step n/4 · <label> ● ○ ○ ○` using the LONG wizardSteps labels — the
// bracket format moves onto the MAIN NAV instead (D4).
func TestRenderStepperRevertsToStepCounterWithLongLabel(t *testing.T) {
	active := renderStepper(1)
	plain := stripANSI(active)
	if !strings.Contains(plain, "Step 2/4") {
		t.Fatalf("stepper must render the Step n/4 counter; got %q", plain)
	}
	if !strings.Contains(plain, "Test connection") {
		t.Errorf("stepper must render the LONG active label (wizardSteps), not a short segment; got %q", plain)
	}
	// Built from parts (never a contiguous literal in this source file) so
	// this negative assertion itself never trips the repo-wide extended
	// copy-freeze grep (02-STYLE-SPEC.md §6) that forbids these strings.
	for i, short := range []string{"SSH", "Test", "Git", "Review"} {
		forbidden := fmt.Sprintf("[%d]", i+1) + " " + short
		if strings.Contains(plain, forbidden) {
			t.Errorf("the superseded bracketed short-segment stepper must be GONE (D5); found %q in %q", forbidden, plain)
		}
	}
	if strings.Contains(active, styleFaint.Render("Test connection")) {
		t.Error("the ACTIVE label must NOT render through styleFaint (the old `Step n/4` line read dimmer than body text)")
	}
	if !strings.Contains(active, styleStepperActive.Render("Test connection")) {
		t.Error("the active label must render bold + accent (styleStepperActive)")
	}
	if !strings.Contains(plain, "●") || !strings.Contains(plain, "○") {
		t.Error("the stepper must render both a passed/active accent dot (●) and an upcoming faint dot (○)")
	}
}

// TestWizardChordHintIsStepConditionalAndAlwaysVisible pins D5/D7: the
// always-visible faint line directly under the stepper carries the frozen,
// step-conditional Shift-chord hint.
func TestWizardChordHintIsStepConditionalAndAlwaysVisible(t *testing.T) {
	cases := []struct {
		step int
		want string
	}{
		{0, "Shift+→ next section · Shift+← exits the wizard"},
		{1, "Shift+←/→ jump sections · forward needs a valid step"},
		{2, "Shift+←/→ jump sections · forward needs a valid step"},
		{3, "Shift+← back to Git · Enter writes"},
	}
	for _, tc := range cases {
		if got := wizardChordHint(tc.step); got != tc.want {
			t.Errorf("wizardChordHint(%d) = %q, want %q", tc.step, got, tc.want)
		}
	}
}

func TestWizardArrowKeyPrecedenceStep0(t *testing.T) {
	// Clause 1: the algorithm select owns plain arrows (cycles the
	// catalog), never changing the wizard step.
	a := pressSeq(t, identitiesApp(), "n", "up") // prefix → algorithm
	if identModel(t, a).wizard.focus != wizardFocusKeyBody {
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

// TestWizardHoistedShiftGateReachesEveryStepIncludingTheReviewCeremony pins
// D7 (checkpoint-2 contract): ONE hoisted Shift+←/→ gate works at EVERY
// step, including the previously-DEAD review ceremony (step 3) where
// ceremony.handleKey's own default: case used to swallow the chord. A
// table-driven walk asserts Shift+← always goes back (uniform stepBack)
// and Shift+→ is blocked at each unpassed step with the frozen status note.
func TestWizardHoistedShiftGateReachesEveryStepIncludingTheReviewCeremony(t *testing.T) {
	shiftLeft := tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModShift}
	shiftRight := tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModShift}

	// Step 0 → exits the wizard on Shift+Left (Esc parity).
	a0 := pressSeq(t, identitiesApp(), "n")
	model, _ := a0.Update(shiftLeft)
	a0 = model.(App)
	if got := identModel(t, a0).pane; got != paneDetail {
		t.Errorf("step 0 shift+left: pane = %v, want paneDetail (exits the wizard)", got)
	}

	// Step 0 blocked forward: clear the required hostname so step0Valid
	// fails, then assert Shift+Right is blocked with the frozen note.
	a0b := pressSeq(t, identitiesApp(), "n", "tab", "tab") // prefix → host → hostname
	for i := 0; i < 20; i++ {
		model, _ := a0b.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
		a0b = model.(App)
	}
	model, _ = a0b.Update(shiftRight)
	a0b = model.(App)
	if got := identModel(t, a0b).wizard.step; got != 0 {
		t.Fatalf("step 0 shift+right with an invalid form: step = %d, want blocked at 0", got)
	}
	if a0b.note != "Can't continue yet — check the alias prefix, hostname, and port." {
		t.Errorf("blocked-forward note = %q, want the frozen D7 status note", a0b.note)
	}

	// Steps 1→0, 2→1, and 3→2 (the previously-DEAD review step) via the
	// SAME uniform stepBack(), reached from wherever focus happens to be.
	a3 := wizardThroughTest(t, identitiesApp())
	a3, _ = press(t, a3, "enter") // [ Continue ] → step 3 review ceremony
	if got := identModel(t, a3).wizard.step; got != 3 {
		t.Fatalf("setup: wizard did not reach the step-3 review ceremony; step=%d", got)
	}
	model, _ = a3.Update(shiftLeft)
	a3 = model.(App)
	if got := identModel(t, a3).wizard.step; got != 2 {
		t.Errorf("step 3 (review ceremony) shift+left: step = %d, want 2 — the D7 fix for the previously-dead review step", got)
	}

	// Step 2 blocked forward: blank the Git form's user.name so
	// gitForm.valid() fails, then assert Shift+Right is blocked with the
	// frozen note (mirrors TestWizardShiftArrowIsFocusOverrideNotValidityOverride
	// but via the note assertion, table-style).
	a2 := wizardThroughTest(t, identitiesApp())
	for i := 0; i < 20; i++ {
		model, _ := a2.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
		a2 = model.(App)
	}
	model, _ = a2.Update(shiftRight)
	a2 = model.(App)
	if got := identModel(t, a2).wizard.step; got != 2 {
		t.Fatalf("step 2 shift+right with an invalid Git form: step = %d, want blocked at 2", got)
	}
	if a2.note != "Can't continue yet — add user.name and a valid email." {
		t.Errorf("blocked-forward note = %q, want the frozen D7 status note", a2.note)
	}
}

// TestWizardFocusedFieldIsSingleRowColorOnlyNoBox pins D1 (checkpoint-2
// contract): every field is ONE constant-height row in every state —
// focus is signalled by accent color + the `▸` marker, NEVER a reflowing
// box. This SUPERSEDES 02-14's renderFocusedFieldBox 3-row rounded contour.
func TestWizardFocusedFieldIsSingleRowColorOnlyNoBox(t *testing.T) {
	a := pressSeq(t, identitiesApp(), "n") // step 0, Alias prefix focused
	raw := a.View().Content
	plain := paneFlat(a)
	if !strings.Contains(raw, "\x1b[1;34m") {
		t.Error("the focused field must carry the accent (blue, bold) SGR")
	}
	if strings.Contains(plain, "╭─") {
		t.Error("D1: no field may ever render a SOLID rounded box border (renderFocusedFieldBox is deleted) — only dashed PreviewBlock borders (╭╌) may remain on the pane")
	}
	if !strings.Contains(plain, "▸") {
		t.Error("the focused field must carry the redundant non-color `▸` marker cue")
	}
	// A blurred field at wizard open (e.g. Real hostname) renders a single-row
	// dim bracket contour — brackets present in EVERY state (D1).
	if !strings.Contains(plain, "[ssh.github.com]") {
		t.Errorf("blurred fields must render a dim bracket contour; pane:\n%s", plain)
	}
}

// TestGitFormStrategyAlwaysExpandedWithHeaderHint pins D2 (checkpoint-2
// contract): the match-strategy group renders ALL options at constant
// height in BOTH focus states — no expand/collapse branch — and the
// `(←/→ change)` hint sits on the group's HEADER line, visible in BOTH
// states (it used to show only while blurred — backwards).
func TestGitFormStrategyAlwaysExpandedWithHeaderHint(t *testing.T) {
	a := wizardThroughTest(t, identitiesApp())
	blurred := paneFlat(a)
	if !strings.Contains(blurred, "Determines which repos this Git identity applies to.") {
		t.Fatal("the reserved match-strategy hint must be present while blurred")
	}
	if !strings.Contains(blurred, "(←/→ change)") {
		t.Error("D2: the (←/→ change) hint must be on the header line even while blurred")
	}
	for _, want := range []string{
		"gitdir (default) — applies inside ~/acme2/",
		"hasconfig — repos whose remote uses this alias",
		"both — either condition (two includeIf blocks = OR)",
	} {
		if !strings.Contains(blurred, want) {
			t.Errorf("D2: all three strategy options must render even while blurred; missing %q", want)
		}
	}

	a = pressSeq(t, a, "tab", "tab") // name → email → strategy (focused)
	focused := paneFlat(a)
	if !strings.Contains(focused, "Determines which repos this Git identity applies to.") {
		t.Error("the reserved hint row must remain present while focused")
	}
	if !strings.Contains(focused, "(←/→ change)") {
		t.Error("D2: the (←/→ change) hint must still be on the header line while focused")
	}
	if !strings.Contains(focused, "hasconfig — repos whose remote uses this alias") {
		t.Error("all three option rows must remain present while focused")
	}
}

func TestWizardAlgorithmSelectionMirrorsStrategyFocusAccent(t *testing.T) {
	// Checkpoint micro-fix: the Key-algorithm radio (wizard step 0) must
	// carry the SAME selected-option treatment as the match-strategy radio
	// (D2 one-radio-treatment): FieldFocused accent while the group owns
	// focus, plain-bold when blurred — never a plain unstyled label.
	a := pressSeq(t, identitiesApp(), "n")
	selected := "ed25519 — ★ recommended"
	focusedWant := DefaultTheme.FieldFocused.Render(selected)
	boldWant := styleBold.Render(selected)

	blurred := a.View().Content
	if !strings.Contains(blurred, boldWant) {
		t.Error("blurred algorithm radio must render the selected label plain-bold, mirroring the strategy radio")
	}
	if strings.Contains(blurred, focusedWant) {
		t.Error("the FieldFocused accent must not apply while the algorithm group is blurred")
	}

	a = pressSeq(t, a, "tab", "tab", "tab", "tab", "tab") // prefix(1) → host → hostname → port → key source → algorithm
	focused := a.View().Content
	if !strings.Contains(focused, focusedWant) {
		t.Error("focused algorithm radio must render the selected label through Theme.FieldFocused (accent), mirroring the strategy radio")
	}
}

// --------------------------------------------------------------------------
// D-09/D-10/D-12/D-13 — reuse-existing-key picker (KEY-06).
// --------------------------------------------------------------------------

// openReusePicker opens the create wizard, toggles to "Reuse an existing
// key" (D-10), and leaves focus on the picker body with the default
// selection (reuseIdx 0 — the first scanned key, "personal").
func openReusePicker(t *testing.T) App {
	t.Helper()
	a := pressSeq(t, identitiesApp(), "n", "tab", "tab", "tab", "tab") // prefix→host→hostname→port→key source
	if identModel(t, a).wizard.focus != wizardFocusKeySource {
		t.Fatalf("setup: focus = %d, want the key-source toggle", identModel(t, a).wizard.focus)
	}
	a, _ = press(t, a, "right") // generate → reuse
	if identModel(t, a).wizard.keySource != keySourceReuse {
		t.Fatal("setup: expected keySource == reuse after toggling")
	}
	a, _ = press(t, a, "tab") // key source → picker body
	if identModel(t, a).wizard.focus != wizardFocusKeyBody {
		t.Fatalf("setup: focus = %d, want the picker body", identModel(t, a).wizard.focus)
	}
	return a
}

// TestReusePickerListsFilenameAlgorithmFingerprintAndInUseBy proves the D-10
// picker renders every candidate's filename + algorithm + fingerprint, and
// the D-12 "in use by: <identity> (<provider>)" label for a referenced key.
func TestReusePickerListsFilenameAlgorithmFingerprintAndInUseBy(t *testing.T) {
	pane := paneFlat(openReusePicker(t))
	for _, want := range []string{"id_ed25519_personal", "ssh-ed25519", "SHA256:stub-personal", "in use by: personal (github.com)"} {
		if !strings.Contains(pane, want) {
			t.Errorf("picker pane missing %q:\n%s", want, pane)
		}
	}
}

// TestReusePickerHasManualPathRow proves the D-10 manual-path row is always
// offered alongside the scanned candidates.
func TestReusePickerHasManualPathRow(t *testing.T) {
	pane := paneFlat(openReusePicker(t))
	if !strings.Contains(pane, "Enter a path manually…") {
		t.Errorf("picker pane missing the manual-path row:\n%s", pane)
	}
}

// TestReusePickerSameProviderWarningIsAdvisoryNotBlocking proves D-12: a
// same-provider reuse selection warns but never blocks advance.
func TestReusePickerSameProviderWarningIsAdvisoryNotBlocking(t *testing.T) {
	a := openReusePicker(t) // reuseIdx defaults to 0 = "personal", same provider (github.com)
	m := identModel(t, a)
	if !m.wizard.sameProviderReuse() {
		t.Fatal("setup: expected the default selection to trigger the same-provider warning")
	}
	if !strings.Contains(paneFlat(a), "Same provider as an existing identity") {
		t.Error("same-provider warning must render (D-12, advisory)")
	}
	valid, _ := m.wizard.step0Valid(m.backend.InitialState())
	if !valid {
		t.Error("a same-provider reuse selection must NOT block advance (D-12: warn, allow)")
	}
}

// TestReusePickerNonCatalogAlgorithmShowsInfoNoteNotBlocking proves D-13: a
// legacy/foreign algorithm shows an informational note but is never blocked.
func TestReusePickerNonCatalogAlgorithmShowsInfoNoteNotBlocking(t *testing.T) {
	a := openReusePicker(t)
	// Cycle to clientA (index 4): personal(0) work(1) archived(2) staging(3) clientA(4).
	for i := 0; i < 4; i++ {
		a, _ = press(t, a, "right")
	}
	m := identModel(t, a)
	if m.wizard.reuseIdx != 4 {
		t.Fatalf("setup: reuseIdx = %d, want 4 (clientA)", m.wizard.reuseIdx)
	}
	if !strings.Contains(paneFlat(a), "Not one of gitid's generate algorithms") {
		t.Error("a non-catalog algorithm must show the D-13 informational note")
	}
	valid, _ := m.wizard.step0Valid(m.backend.InitialState())
	if !valid {
		t.Error("a non-catalog algorithm must NOT block advance (D-13)")
	}
}

// TestReusePickerEncryptedWithPubKeyIsSelectableAndNotBlocking proves
// D-11/KEY-06: an encrypted key with an existing .pub is a normal, selectable,
// non-blocking picker entry.
func TestReusePickerEncryptedWithPubKeyIsSelectableAndNotBlocking(t *testing.T) {
	a := openReusePicker(t)
	// Cycle to staging (index 3): personal(0) work(1) archived(2) staging(3).
	for i := 0; i < 3; i++ {
		a, _ = press(t, a, "right")
	}
	m := identModel(t, a)
	if m.wizard.reuseIdx != 3 {
		t.Fatalf("setup: reuseIdx = %d, want 3 (staging)", m.wizard.reuseIdx)
	}
	view, ok := m.wizard.selectedReuseView()
	if !ok || !view.Encrypted {
		t.Fatalf("setup: expected the encrypted staging key selected, got %+v (ok=%v)", view, ok)
	}
	valid, _ := m.wizard.step0Valid(m.backend.InitialState())
	if !valid {
		t.Error("an encrypted-with-.pub key selection must NOT block advance (D-11/KEY-06)")
	}
	if got := m.wizard.reuseKeyPath(); got != "~/.ssh/id_ed25519_staging" {
		t.Errorf("reuseKeyPath() = %q, want the selected key's path", got)
	}
}

// TestReuseSelectionRoutesCreateThroughNoGenerate proves selecting a reuse
// candidate routes the create through the reuse path — CreateSpec.KeyPath
// AND ReuseKeyPath both name the EXISTING key, never a generated one.
func TestReuseSelectionRoutesCreateThroughNoGenerate(t *testing.T) {
	a := openReusePicker(t) // defaults to personal (index 0)
	m := identModel(t, a)
	want := "~/.ssh/id_ed25519_personal"
	if got := m.wizard.keyPath(); got != want {
		t.Errorf("keyPath() = %q, want the reused key's path %q (no generate)", got, want)
	}
	if got := m.wizard.spec().ReuseKeyPath; got != want {
		t.Errorf("spec().ReuseKeyPath = %q, want %q", got, want)
	}
}

// TestReusePickerManualPathRow proves the D-10 manual-path row: empty/invalid
// blocks advance, a resolved candidate unblocks it and supplies the reuse
// path.
func TestReusePickerManualPathRow(t *testing.T) {
	a := openReusePicker(t)
	// Cycle past every scanned key to the manual row (6 fixture rows carry a
	// KeyPath, so the manual row sits at index 6).
	for i := 0; i < 6; i++ {
		a, _ = press(t, a, "right")
	}
	m := identModel(t, a)
	if m.wizard.reuseIdx != 6 {
		t.Fatalf("setup: reuseIdx = %d, want 6 (manual row)", m.wizard.reuseIdx)
	}
	valid, _ := m.wizard.step0Valid(m.backend.InitialState())
	if valid {
		t.Error("an empty manual-path row must block advance (D-10)")
	}

	a, _ = press(t, a, "tab") // picker body → manual path text input
	a = typeText(t, a, stubManualReusePath)
	m = identModel(t, a)
	if m.wizard.manualErr != "" {
		t.Fatalf("manualErr = %q, want no error for the fixture manual path", m.wizard.manualErr)
	}
	if got := m.wizard.reuseKeyPath(); got != stubManualReusePath {
		t.Errorf("reuseKeyPath() = %q, want the resolved manual path %q", got, stubManualReusePath)
	}
	valid, _ = m.wizard.step0Valid(m.backend.InitialState())
	if !valid {
		t.Error("a resolved manual-path candidate must unblock advance")
	}
}

// TestReusePickerManualPathRejectsInvalidCandidate proves an unrecognized
// manual path shows its rejection inline and blocks advance — the picker's
// half of the T-03-13 symlink-rejection contract the Backend enforces.
func TestReusePickerManualPathRejectsInvalidCandidate(t *testing.T) {
	a := openReusePicker(t)
	for i := 0; i < 6; i++ {
		a, _ = press(t, a, "right")
	}
	a, _ = press(t, a, "tab")
	a = typeText(t, a, "/no/such/key")
	m := identModel(t, a)
	if m.wizard.manualErr == "" {
		t.Fatal("an unrecognized manual path must record an inline error")
	}
	if !strings.Contains(paneFlat(a), m.wizard.manualErr) {
		t.Error("the manual-path error must render inline in the picker")
	}
	valid, _ := m.wizard.step0Valid(m.backend.InitialState())
	if valid {
		t.Error("an unresolved manual-path candidate must block advance")
	}
}
