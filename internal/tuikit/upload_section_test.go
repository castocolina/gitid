package tuikit

// upload_section_test.go — Phase 9 (UP-02/UP-03) tracer tests, kept in its
// own file so it never collides with a concurrent identities_test.go edit
// (09-02-PLAN.md Task 1's explicit instruction).

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/castocolina/gitid/internal/upload"
)

// openWizardUploadReady opens the create wizard ("n") and runs the
// resulting checkUploadEligibility tea.Cmd synchronously, mirroring the
// real Bubble Tea runtime's async dispatch, so the returned App holds a
// wizard whose eligibility answer has already arrived. stubBackend answers
// Ready for the default acme.github.com fixture (backend_stub_test.go).
func openWizardUploadReady(t *testing.T) App {
	t.Helper()
	return pressAndRun(t, identitiesApp(), "n")
}

// --------------------------------------------------------------------------
// wizardStep0FocusOrder / stepAdvance (Pitfall 2's replacement ring).
// --------------------------------------------------------------------------

// TestWizardStep0FocusOrderIncludesCheckbox asserts the exact focus order
// for every (keySource, uploadRowVisible) combination this task's action
// block enumerates.
func TestWizardStep0FocusOrderIncludesCheckbox(t *testing.T) {
	got := wizardStep0FocusOrder(keySourceGenerate, true)
	want := []int{sshFieldPrefix, sshFieldHost, sshFieldHostname, sshFieldPort, wizardFocusUploadCheckbox, wizardFocusKeySource, wizardFocusKeyBody}
	if !equalIntSlices(got, want) {
		t.Errorf("wizardStep0FocusOrder(generate, true) = %v, want %v", got, want)
	}

	got = wizardStep0FocusOrder(keySourceGenerate, false)
	want = []int{sshFieldPrefix, sshFieldHost, sshFieldHostname, sshFieldPort, wizardFocusKeySource, wizardFocusKeyBody}
	if !equalIntSlices(got, want) {
		t.Errorf("wizardStep0FocusOrder(generate, false) = %v, want %v", got, want)
	}

	got = wizardStep0FocusOrder(keySourceReuse, true)
	want = []int{sshFieldPrefix, sshFieldHost, sshFieldHostname, sshFieldPort, wizardFocusUploadCheckbox, wizardFocusKeySource, wizardFocusKeyBody, wizardFocusManualPath}
	if !equalIntSlices(got, want) {
		t.Errorf("wizardStep0FocusOrder(reuse, true) = %v, want %v", got, want)
	}
}

func equalIntSlices(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestNoExistingFocusConstantChangedValue proves wizardFocusKeySource,
// wizardFocusKeyBody, wizardFocusManualPath, and editFocusButton evaluate to
// the same integers Phase 9 found them at — the checkbox slot was appended
// AFTER wizardFocusManualPath, not inserted between Port and KeySource
// (09-RESEARCH.md Pitfall 2 avoided rather than merely survived).
func TestNoExistingFocusConstantChangedValue(t *testing.T) {
	if sshFieldPort != 3 {
		t.Fatalf("sshFieldPort = %d, want 3 (setup precondition)", sshFieldPort)
	}
	if wizardFocusKeySource != sshFieldPort+1 {
		t.Errorf("wizardFocusKeySource = %d, want %d", wizardFocusKeySource, sshFieldPort+1)
	}
	if wizardFocusKeyBody != wizardFocusKeySource+1 {
		t.Errorf("wizardFocusKeyBody = %d, want %d", wizardFocusKeyBody, wizardFocusKeySource+1)
	}
	if wizardFocusManualPath != wizardFocusKeyBody+1 {
		t.Errorf("wizardFocusManualPath = %d, want %d", wizardFocusManualPath, wizardFocusKeyBody+1)
	}
	if editFocusButton != sshFieldPort+1 {
		t.Errorf("editFocusButton = %d, want %d", editFocusButton, sshFieldPort+1)
	}
	// The checkbox slot must be the NEW, fourth, appended value.
	if wizardFocusUploadCheckbox != wizardFocusManualPath+1 {
		t.Errorf("wizardFocusUploadCheckbox = %d, want %d (appended after ManualPath)", wizardFocusUploadCheckbox, wizardFocusManualPath+1)
	}
}

// --------------------------------------------------------------------------
// Checkbox toggling: space, u, and text-field inertness.
// --------------------------------------------------------------------------

// TestUploadCheckboxTogglesOnSpaceAndU proves both toggle paths D-01
// specifies: `space` while the checkbox row itself is focused, and `u` from
// any non-text-editing slot.
func TestUploadCheckboxTogglesOnSpaceAndU(t *testing.T) {
	a := openWizardUploadReady(t)
	m := identModel(t, a)
	if !m.wizard.uploadChecked {
		t.Fatal("setup: the GitHub-ready checkbox must default to checked")
	}

	// tab from prefix -> host -> hostname -> port -> checkbox
	a = pressSeq(t, a, "tab", "tab", "tab", "tab")
	m = identModel(t, a)
	if m.wizard.focus != wizardFocusUploadCheckbox {
		t.Fatalf("focus = %d, want the checkbox slot %d", m.wizard.focus, wizardFocusUploadCheckbox)
	}

	a, _ = press(t, a, "space")
	m = identModel(t, a)
	if m.wizard.uploadChecked {
		t.Error("space on the focused checkbox must flip it off")
	}

	// u also toggles from a non-text slot (key-source, focus > sshFieldPort).
	a, _ = press(t, a, "tab") // checkbox -> key source
	m = identModel(t, a)
	if m.wizard.focus != wizardFocusKeySource {
		t.Fatalf("focus = %d, want key source %d", m.wizard.focus, wizardFocusKeySource)
	}
	a, _ = press(t, a, "u")
	m = identModel(t, a)
	if !m.wizard.uploadChecked {
		t.Error("u from the key-source slot must flip the checkbox back on")
	}
}

// TestUToggleIsInertWhileEditingATextField proves that `u` types a literal
// character into a focused text field instead of flipping the checkbox.
func TestUToggleIsInertWhileEditingATextField(t *testing.T) {
	a := openWizardUploadReady(t)
	m := identModel(t, a)
	before := m.wizard.uploadChecked
	if m.wizard.focus != sshFieldPrefix {
		t.Fatalf("setup: focus = %d, want the prefix field %d", m.wizard.focus, sshFieldPrefix)
	}

	a, _ = press(t, a, "u")
	m = identModel(t, a)
	if !strings.Contains(m.wizard.form.prefix.Value(), "u") {
		t.Errorf("prefix value = %q, want it to contain a typed 'u'", m.wizard.form.prefix.Value())
	}
	if m.wizard.uploadChecked != before {
		t.Errorf("uploadChecked changed from %v to %v while typing in a text field", before, m.wizard.uploadChecked)
	}
}

// --------------------------------------------------------------------------
// enter on step 1 / testUpload auto-advance.
// --------------------------------------------------------------------------

// openWizardAtTestStep opens the ready-checked wizard and advances to step 1
// (Test connection) via a valid enter on step 0.
func openWizardAtTestStep(t *testing.T) App {
	t.Helper()
	a := openWizardUploadReady(t)
	a, _ = press(t, a, "enter")
	m := identModel(t, a)
	if m.wizard.step != 1 {
		t.Fatalf("setup: wizard.step = %d, want 1 (Test connection)", m.wizard.step)
	}
	return a
}

// TestEnterOnTestIdleEntersUploadWhenChecked proves the checked-checkbox
// path: enter on testIdle sets testPhase = testUpload and dispatches
// RunUpload, NOT TestStage1.
func TestEnterOnTestIdleEntersUploadWhenChecked(t *testing.T) {
	a := openWizardAtTestStep(t)
	m := identModel(t, a)
	if !m.wizard.uploadChecked {
		t.Fatal("setup: checkbox must be checked")
	}
	if m.wizard.testPhase != testIdle {
		t.Fatalf("setup: testPhase = %q, want testIdle", m.wizard.testPhase)
	}

	next, cmd := press(t, a, "enter")
	m = identModel(t, next)
	if m.wizard.testPhase != testUpload {
		t.Errorf("testPhase after enter = %q, want testUpload", m.wizard.testPhase)
	}
	if cmd == nil {
		t.Fatal("enter on testIdle (checked) must dispatch a non-nil cmd")
	}
	msg := cmd()
	if _, ok := msg.(UploadRunMsg); !ok {
		t.Errorf("dispatched cmd delivered %T, want UploadRunMsg (RunUpload, not TestStage1)", msg)
	}
}

// TestEnterOnTestIdleSkipsUploadWhenUnchecked proves the unchecked path is
// UNCHANGED: straight to testRunning1 + TestStage1.
func TestEnterOnTestIdleSkipsUploadWhenUnchecked(t *testing.T) {
	a := openWizardAtTestStep(t)
	m := identModel(t, a)
	m.wizard.uploadChecked = false
	a.screens[TabIdentities] = m

	next, cmd := press(t, a, "enter")
	m = identModel(t, next)
	if m.wizard.testPhase != testRunning1 {
		t.Errorf("testPhase after enter (unchecked) = %q, want testRunning1", m.wizard.testPhase)
	}
	if cmd == nil {
		t.Fatal("enter on testIdle (unchecked) must dispatch a non-nil cmd")
	}
	msg := cmd()
	if _, ok := msg.(WizardStageMsg); !ok {
		t.Errorf("dispatched cmd delivered %T, want WizardStageMsg (TestStage1)", msg)
	}
}

// TestUploadRunMsgAutoAdvancesToStage1 proves D-02: receiving UploadRunMsg
// while testPhase == testUpload renders the result and immediately
// dispatches TestStage1 with NO simulated keystroke in between.
func TestUploadRunMsgAutoAdvancesToStage1(t *testing.T) {
	a := openWizardAtTestStep(t)
	a, cmd := press(t, a, "enter") // testIdle -> testUpload, dispatches RunUpload
	if cmd == nil {
		t.Fatal("setup: enter must dispatch RunUpload")
	}
	runMsg := cmd()
	if _, ok := runMsg.(UploadRunMsg); !ok {
		t.Fatalf("setup: dispatched cmd delivered %T, want UploadRunMsg", runMsg)
	}

	model, nextCmd := a.Update(runMsg)
	next, ok := model.(App)
	if !ok {
		t.Fatalf("Update(UploadRunMsg) returned %T, want App", model)
	}
	m := identModel(t, next)
	if m.wizard.testPhase != testRunning1 {
		t.Errorf("testPhase after UploadRunMsg = %q, want testRunning1 (D-02 auto-advance)", m.wizard.testPhase)
	}
	if nextCmd == nil {
		t.Fatal("UploadRunMsg handler must dispatch a non-nil cmd (TestStage1) with no keystroke")
	}
	stageMsg := nextCmd()
	if _, ok := stageMsg.(WizardStageMsg); !ok {
		t.Errorf("auto-advance cmd delivered %T, want WizardStageMsg", stageMsg)
	}
	if len(m.wizard.uploadRun.Rows) == 0 {
		t.Error("wizard.uploadRun.Rows must be populated by the UploadRunMsg handler")
	}
}

// --------------------------------------------------------------------------
// Mouse click-to-focus (checkpoint-2 D8).
// --------------------------------------------------------------------------

// TestUploadCheckboxRowIsClickable proves the checkbox row is reachable by
// mouse, mirroring every other form row's click-to-focus contract — and
// that the click ALSO toggles it, matching how paneDeleteScope's rows
// already behave.
func TestUploadCheckboxRowIsClickable(t *testing.T) {
	a := openWizardUploadReady(t)
	m := identModel(t, a)
	if !m.wizard.uploadChecked {
		t.Fatal("setup: the GitHub-ready checkbox must default to checked")
	}

	a = clickCell(t, a, "Register with GitHub automatically", 0, frameBodyTop)
	m = identModel(t, a)
	if m.wizard.focus != wizardFocusUploadCheckbox {
		t.Errorf("focus after clicking the checkbox row = %d, want %d", m.wizard.focus, wizardFocusUploadCheckbox)
	}
	if m.wizard.uploadChecked {
		t.Error("clicking the checked checkbox row must flip it off")
	}
}

// --------------------------------------------------------------------------
// R3 — eligibility resolution stays off the render path.
// --------------------------------------------------------------------------

// TestUploadEligibilityIsResolvedInUpdateNotView is a source-level check
// over identities.go: no render/View function body may call the
// eligibility seam (backend.UploadEligibility / checkUploadEligibility).
// Comment lines are filtered out by go/parser before the scan, so a doc
// comment describing the rule can never satisfy or trip it.
func TestUploadEligibilityIsResolvedInUpdateNotView(t *testing.T) {
	fset := token.NewFileSet()
	src, err := os.ReadFile(filepath.Join(".", "identities.go")) //nolint:gosec // package-local source file (G304)
	if err != nil {
		t.Fatalf("reading identities.go: %v", err)
	}
	file, err := parser.ParseFile(fset, "identities.go", src, 0)
	if err != nil {
		t.Fatalf("parsing identities.go: %v", err)
	}

	ast.Inspect(file, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			return true
		}
		// Scope the rule to render-path functions: any function whose name
		// contains "render" or "View" (case-sensitive per this file's own
		// naming convention: renderX / X.view).
		name := fn.Name.Name
		isRenderPath := strings.Contains(name, "render") || strings.HasSuffix(name, "view") || strings.HasSuffix(name, "View")
		if !isRenderPath {
			return true
		}
		ast.Inspect(fn.Body, func(inner ast.Node) bool {
			sel, ok := inner.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if sel.Sel.Name == "UploadEligibility" || sel.Sel.Name == "checkUploadEligibility" {
				t.Errorf("render-path function %s calls %s — eligibility must be resolved in Update, never View (R3)", name, sel.Sel.Name)
			}
			return true
		})
		return true
	})
}

// --------------------------------------------------------------------------
// Stale-guard: a reply for a host the wizard is no longer probing.
// --------------------------------------------------------------------------

// TestStaleUploadEligibilityMsgIsDiscarded delivers a message whose
// Hostname does not match the wizard's current probe host and asserts the
// cached view is left unchanged.
func TestUploadCheckboxRendersAllFourStates(t *testing.T) {
	tests := []struct {
		name string
		view UploadEligibilityView
		want string
	}{
		{"ready", UploadEligibilityView{State: UploadEligibilityReady, ProviderName: "GitHub"}, "Register with GitHub automatically"},
		{"unauth", UploadEligibilityView{State: UploadEligibilityUnauth, ProviderName: "GitLab", ToolName: "glab", Hostname: "gitlab.com"}, "not logged in"},
		{"disabled", UploadEligibilityView{State: UploadEligibilityDisabled, ProviderName: "GitHub"}, "Auto-registration unavailable"},
		{"omitted", UploadEligibilityView{State: UploadEligibilityOmitted}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderUploadCheckboxRow(tt.view, false, false, 200)
			if tt.want == "" && got != "" {
				t.Errorf("renderUploadCheckboxRow() = %q, want empty", got)
			}
			if tt.want != "" && !strings.Contains(got, tt.want) {
				t.Errorf("renderUploadCheckboxRow() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestUploadCheckboxIsLegibleWithoutColor proves each visible state's
// distinguishing words survive with SGR color stripped (frame_test.go's
// stripANSI, the project's existing no-color assertion helper) — the
// UI-SPEC's non-negotiable glyph-plus-word rule, not a color-only signal.
func TestUploadCheckboxIsLegibleWithoutColor(t *testing.T) {
	tests := []struct {
		name string
		view UploadEligibilityView
		want string
	}{
		{"ready", UploadEligibilityView{State: UploadEligibilityReady, ProviderName: "GitHub"}, "Register with GitHub automatically"},
		{"unauth", UploadEligibilityView{State: UploadEligibilityUnauth, ProviderName: "GitLab", ToolName: "glab", Hostname: "gitlab.com"}, "not logged in"},
		{"disabled", UploadEligibilityView{State: UploadEligibilityDisabled, ProviderName: "GitHub"}, "Auto-registration unavailable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripANSI(renderUploadCheckboxRow(tt.view, false, false, 200))
			if !strings.Contains(got, tt.want) {
				t.Errorf("stripANSI(renderUploadCheckboxRow()) = %q, want it to contain %q", got, tt.want)
			}
		})
	}
}

func TestUploadCheckboxRowIsExactlyOnePhysicalLine(t *testing.T) {
	for _, view := range []UploadEligibilityView{
		{State: UploadEligibilityReady, ProviderName: strings.Repeat("GitHub", 20)},
		{State: UploadEligibilityUnauth, ProviderName: strings.Repeat("GitLab", 20), ToolName: strings.Repeat("glab", 20), Hostname: strings.Repeat("gitlab.", 20)},
		{State: UploadEligibilityDisabled, ProviderName: strings.Repeat("GitHub", 20)},
	} {
		got := renderUploadCheckboxRow(view, false, false, 60)
		if strings.Contains(got, "\n") || len([]rune(ansi.Strip(got))) > 60 {
			t.Errorf("row = %q, want one line no wider than 60", got)
		}
	}
}

func TestDisabledUploadCheckboxCannotBeToggled(t *testing.T) {
	w := wizardModel{uploadEligibility: UploadEligibilityView{State: UploadEligibilityDisabled}}
	for range []string{"space", "u", "right", "click"} {
		w = w.toggleUploadCheckbox()
		if w.uploadChecked {
			t.Fatal("disabled checkbox became checked")
		}
	}
}

func TestOmittedUploadStateRemovesTheFocusSlot(t *testing.T) {
	order := wizardStep0FocusOrder(keySourceGenerate, false)
	for _, focus := range order {
		if focus == wizardFocusUploadCheckbox {
			t.Fatal("omitted state includes upload focus slot")
		}
	}
	if got := stepAdvance(order, sshFieldPort, 1); got != wizardFocusKeySource {
		t.Errorf("Tab from Port = %d, want KeySource %d", got, wizardFocusKeySource)
	}
}

// --------------------------------------------------------------------------
// 09-04-PLAN.md Task 2 — result rows, manual fallback, row budget, auto-advance.
// --------------------------------------------------------------------------

// TestUploadResultRowsRenderGlyphAndWord asserts each of the three outcomes
// carries its glyph AND its word — the project's NO_COLOR-legible contract.
func TestUploadResultRowsRenderGlyphAndWord(t *testing.T) {
	run := UploadRunView{Rows: []UploadResultRow{
		{Label: UploadRegistrationLabelAuth, Outcome: UploadRowUploaded},
		{Label: UploadRegistrationLabelSigning, Outcome: UploadRowAlreadyPresent},
		{Label: UploadRegistrationLabelCombined, Outcome: UploadRowFailed, Reason: "insufficient scope"},
	}}
	got := stripANSI(renderUploadRun(run, "GitHub", 100))
	for _, want := range []string{
		"✓ " + UploadRegistrationLabelAuth + " key registered",
		"✓ " + UploadRegistrationLabelSigning + " key already registered (skipped)",
		"✗ " + UploadRegistrationLabelCombined + " key registration failed: insufficient scope",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("renderUploadRun output = %q, want it to contain %q", got, want)
		}
	}
}

// TestManualFallbackIsByteIdenticalToInstructions compares the rendered
// manual-fallback block against upload.Instructions output, asserting exact
// equality after stripping the heading — UP-02/UP-03's shown==run contract.
func TestManualFallbackIsByteIdenticalToInstructions(t *testing.T) {
	for _, provider := range []string{"github.com", "gitlab.com"} {
		instructions := upload.Instructions(provider)
		run := UploadRunView{ManualFallback: instructions}
		got := stripANSI(renderUploadRun(run, provider, 100))
		if !strings.HasPrefix(got, " "+UploadManualHeading+"\n") {
			t.Fatalf("rendered fallback = %q, want it to start with the frozen heading", got)
		}
		body := strings.TrimPrefix(got, " "+UploadManualHeading+"\n")
		if body != instructions {
			t.Errorf("fallback body = %q, want byte-identical to upload.Instructions(%q) = %q", body, provider, instructions)
		}
	}
}

// TestUploadSectionFitsTheFrameInTheWorstCase asserts the worst realistic
// case (two announce lines + two result rows + the GitHub instructions
// block) fits frameBodyRows(30), and separately exercises the overflow
// branch with a deliberately oversized fallback block.
func TestUploadSectionFitsTheFrameInTheWorstCase(t *testing.T) {
	run := UploadRunView{
		Rows: []UploadResultRow{
			{Label: UploadRegistrationLabelAuth, Command: "gh ssh-key add ~/.ssh/id_ed25519_acme.pub --title t --type authentication", Outcome: UploadRowUploaded},
			{Label: UploadRegistrationLabelSigning, Command: "gh ssh-key add ~/.ssh/id_ed25519_acme.pub --title t --type signing", Outcome: UploadRowFailed, Reason: "insufficient scope"},
		},
		ManualFallback: upload.Instructions("github.com"),
	}
	rendered := renderUploadRun(run, "GitHub", 100)
	lines := strings.Count(rendered, "\n")
	if lines > frameBodyRows(minFrameHeight) {
		t.Errorf("rendered upload section = %d lines, want at most %d (frameBodyRows(30))", lines, frameBodyRows(minFrameHeight))
	}

	// Overflow branch: an artificially oversized fallback must still stay
	// within the same budget by routing through the bounded viewport.
	oversizedRun := UploadRunView{ManualFallback: strings.Repeat("a very long manual instruction line\n", 40)}
	oversized := renderUploadRun(oversizedRun, "GitHub", 100)
	if got := strings.Count(oversized, "\n"); got > frameBodyRows(minFrameHeight) {
		t.Errorf("oversized fallback rendered %d lines, want the viewport to cap it at %d", got, frameBodyRows(minFrameHeight))
	}
}

// TestEveryUploadTerminalStateAutoAdvances covers success, partial failure,
// total failure, degraded, already-complete, and skipped: every one yields
// testRunning1 plus a non-nil command once UploadRunMsg arrives.
func TestEveryUploadTerminalStateAutoAdvances(t *testing.T) {
	cases := []struct {
		name string
		view UploadRunView
	}{
		{"success", UploadRunView{Rows: []UploadResultRow{{Outcome: UploadRowUploaded}}}},
		{"partial failure", UploadRunView{Rows: []UploadResultRow{{Outcome: UploadRowUploaded}, {Outcome: UploadRowFailed}}}},
		{"total failure", UploadRunView{Rows: []UploadResultRow{{Outcome: UploadRowFailed}}, ManualFallback: "x"}},
		{"degraded", UploadRunView{Rows: []UploadResultRow{{Outcome: UploadRowUploaded}}, InventoryDegraded: true}},
		{"already-complete", UploadRunView{AlreadyComplete: true}},
		{"skipped", UploadRunView{Skipped: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := openWizardAtTestStep(t)
			a, cmd := press(t, a, "enter") // testIdle -> testUpload, dispatches RunUpload
			if cmd == nil {
				t.Fatal("setup: enter must dispatch a non-nil cmd")
			}
			cmd() // drain the (possibly UploadStartedMsg-shaped) setup dispatch

			model, nextCmd := a.Update(UploadRunMsg{View: tc.view})
			next, ok := model.(App)
			if !ok {
				t.Fatalf("Update(UploadRunMsg) returned %T, want App", model)
			}
			m := identModel(t, next)
			if m.wizard.testPhase != testRunning1 {
				t.Errorf("testPhase = %q, want testRunning1", m.wizard.testPhase)
			}
			if nextCmd == nil {
				t.Fatal("UploadRunMsg handler must dispatch a non-nil cmd (TestStage1)")
			}
		})
	}
}

func TestStaleUploadEligibilityMsgIsDiscarded(t *testing.T) {
	a := openWizardUploadReady(t)
	m := identModel(t, a)
	before := m.wizard.uploadEligibility
	if m.wizard.uploadEligibilityHost == "" {
		t.Fatal("setup: wizard must already hold a probe host")
	}

	stale := UploadEligibilityMsg{
		Hostname: "some-other-host-never-probed.example.com",
		View:     UploadEligibilityView{State: UploadEligibilityDisabled, ProviderName: "Nope"},
	}
	model, _ := a.Update(stale)
	next, ok := model.(App)
	if !ok {
		t.Fatalf("Update(stale UploadEligibilityMsg) returned %T, want App", model)
	}
	m = identModel(t, next)
	if m.wizard.uploadEligibility != before {
		t.Errorf("stale message mutated the cached view: got %+v, want unchanged %+v", m.wizard.uploadEligibility, before)
	}
}
