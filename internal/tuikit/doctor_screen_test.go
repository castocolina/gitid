package tuikit

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

const (
	mergedFixableTitle    = "Private key is world-readable"
	mergedNonFixableTitle = "opensource has no dedicated SSH Host block"
	mergedCeremonyNeedle  = "Fix: Private key is world-readable"
)

// TestDoctorMergedListShowsEveryFinding is the Task 2 tracer: ONE App, ONE
// tab, all THREE merge properties in sequence. Property (a) alone is not a
// merge proof — today's Health tab already lists every finding — so the
// conjunction is the thing under test. Failure messages name which of
// (a)/(b)/(c) failed.
func TestDoctorMergedListShowsEveryFinding(t *testing.T) {
	a := doctorApp(t)
	startTab := a.tab

	ordered := orderedFindings(a.state)
	fixable := fixableFindings(ordered)
	if len(fixable) == 0 {
		t.Fatal("fixture sanity: Seed must include at least one fixable finding")
	}
	var nonFixable DemoFinding
	for _, f := range ordered {
		if !f.Fixable {
			nonFixable = f
			break
		}
	}
	if nonFixable.Title == "" {
		t.Fatal("fixture sanity: Seed must include at least one non-fixable finding")
	}
	if nonFixable.Title != mergedNonFixableTitle {
		t.Fatalf("fixture sanity: non-fixable title = %q, want %q", nonFixable.Title, mergedNonFixableTitle)
	}
	if fixable[0].Title != mergedFixableTitle {
		t.Fatalf("fixture sanity: first fixable title = %q, want %q", fixable[0].Title, mergedFixableTitle)
	}

	view := appView(a)
	wantCount := fmt.Sprintf("%d finding", len(ordered))
	// List rows truncLine the title; match a distinctive prefix of each
	// title that survives the master-list width, plus the status count.
	if !strings.Contains(view, "Private key is world-readable") ||
		!strings.Contains(view, "opensource has no dedicated SSH") ||
		!strings.Contains(view, wantCount) {
		t.Fatalf("(a) merged list must show both findings and status count %q:\n%s", wantCount, view)
	}

	a, _ = press(t, a, "f")
	if a.tab != startTab {
		t.Fatalf("(b) pressing f must stay on the same tab, got %v", a.tab)
	}
	view = appView(a)
	if !strings.Contains(view, mergedCeremonyNeedle) {
		t.Fatalf("(b) f on the fixable row must open the fix ceremony (want %q):\n%s", mergedCeremonyNeedle, view)
	}
	if !ceremonyPending(a) {
		t.Fatal("(b) f on the fixable row must set pendingFixID / fixing on the merged model")
	}

	a, _ = press(t, a, "esc")
	if ceremonyPending(a) {
		t.Fatal("setup: Esc must cancel the ceremony before (c)")
	}

	for i := 0; i < len(ordered)-1; i++ {
		a = pressSeq(t, a, "down")
	}
	sel := selectedFinding(a, ordered)
	if sel.ID != nonFixable.ID {
		t.Fatalf("setup: selected = %q, want non-fixable %q", sel.ID, nonFixable.ID)
	}

	a, _ = press(t, a, "f")
	if a.tab != startTab {
		t.Fatalf("(c) pressing f must stay on the same tab, got %v", a.tab)
	}
	if ceremonyPending(a) {
		t.Fatal("(c) f on the non-fixable row must leave browse mode — no ceremony, no pendingFixID")
	}
	view = appView(a)
	if strings.Contains(view, "Fix: "+nonFixable.Title) {
		t.Fatalf("(c) f on the non-fixable row must not open a ceremony:\n%s", view)
	}
	if !strings.Contains(view, mergedNonFixableTitle) {
		t.Fatalf("(c) non-fixable finding must still be listed after f:\n%s", view)
	}
}

func ceremonyPending(a App) bool {
	m, ok := a.screens[a.tab].(doctorModel)
	return ok && (m.fixing || m.pendingFixID != "")
}

func selectedFinding(a App, ordered []DemoFinding) DemoFinding {
	id := ""
	if m, ok := a.screens[a.tab].(doctorModel); ok {
		id = m.selectedID
	}
	sel, _, _ := selectFinding(ordered, id)
	return sel
}

// wave2to5FixableFindings mirrors, in shape and ID naming convention, every
// fixable (Fix != nil, per doctor.Finding) finding this phase's checks
// produce through Wave 5: the flagship D-09 IdentitiesOnly/IdentityFile
// contradiction (Wave 2), the D-05 gitignore pair fix (Wave 4), and the
// pre-existing Coherence/Permissions/Orphans fixable findings the Fixer must
// keep surfacing unchanged. Waves 3 and 5 shipped no NEW fixable findings
// (Wave 3's tolerance downgrades and parse gate, and Wave 5's
// shadowed-option/author-resolution/directive-above-block checks are all
// report-only, Fix: nil per their own SUMMARY.md) — this fixture still
// includes their family shapes as info-only rows so the filter is proven to
// EXCLUDE them, not just include the fixable ones.
//
// 08-08 code review CR-01: fixableFindings' true signal is the Fixable bit
// (set from Fix != nil at conversion, cmd/gitid/wiring.go's
// runDoctorAndConvert), NOT SuggestedFix non-emptiness — a report-only check
// can and does carry non-empty advisory SuggestedFix text ("do this by
// hand") while still being un-fixable via the Fixer's own ceremony. The
// report-only rows below now carry a non-empty SuggestedFix (matching the
// real checks' actual production shape) with Fixable left false (its zero
// value), so this fixture exercises the REAL discriminator instead of a
// shape engineered to pass.
func wave2to5FixableFindings() []DemoFinding {
	return []DemoFinding{
		{HealthFinding: HealthFinding{
			ID: "ssh-identitiesonly-contradiction", Section: "SSH", Severity: SeverityError, Family: "Coherence",
			Title:        "IdentitiesOnly no contradicts an explicit IdentityFile",
			SuggestedFix: "Set IdentitiesOnly yes on the clientb.github.com Host block -- available on the Fixer screen.",
			Fixable:      true,
		}},
		{HealthFinding: HealthFinding{
			ID: "git-baseline-gitignore-pair", Section: "Git", Severity: SeverityWarning, Family: "Baseline",
			Title:        "core.excludesfile and global gitignore are not configured",
			SuggestedFix: "run 'gitid baseline setup'",
			Fixable:      true,
		}},
		{HealthFinding: HealthFinding{
			ID: "ssh-key-perms-archived", Section: "SSH", Severity: SeverityCritical, Family: "Permissions",
			Title:        "Private key is world-readable",
			SuggestedFix: "chmod 0600 ~/.ssh/id_ed25519_archived -- available on the Fixer screen.",
			Fixable:      true,
		}},
		{HealthFinding: HealthFinding{
			ID: "git-allowed-signers-missing", Section: "SSH", Severity: SeverityError, Family: "Coherence",
			Title:        "allowed_signers: no entry for you@example.com",
			SuggestedFix: "add the line manually or re-run 'gitid identity add'",
			Fixable:      true,
		}},
		{HealthFinding: HealthFinding{
			ID: "git-allowed-signers-mismatch", Section: "SSH", Severity: SeverityError, Family: "Coherence",
			Title:        "allowed_signers: email mismatch for identity \"legacy\"",
			SuggestedFix: "correct the email in ~/.ssh/allowed_signers to exactly match 'you@example.com'",
			Fixable:      true,
		}},
		{HealthFinding: HealthFinding{
			ID: "ssh-orphan-class2", Section: "SSH", Severity: SeverityWarning, Family: "Orphans",
			Title:        "Orphaned SSH Host block for \"stale\"",
			SuggestedFix: "remove the orphaned Host block or re-run 'gitid identity add'",
			Fixable:      true,
		}},
		// Wave 3/5 report-only checks — carry real advisory SuggestedFix
		// text (matching production) but Fixable stays false: must NOT
		// appear as fixable.
		{HealthFinding: HealthFinding{
			ID: "ssh-shadowed-option", Section: "SSH", Severity: SeverityWarning, Family: "Coherence",
			Title:        "IdentitiesOnly is shadowed by an earlier directive",
			SuggestedFix: "review the earlier directive and remove the shadow manually -- advisory only; not offered as a fix.",
		}},
		{HealthFinding: HealthFinding{
			ID: "git-author-resolution", Section: "Git", Severity: SeverityError, Family: "Coherence",
			Title:        "Author resolution invariant broke for \"legacy\"",
			SuggestedFix: "repair via the Global Git screen or re-run 'gitid identity add' -- not offered as a fix.",
		}},
		{HealthFinding: HealthFinding{
			ID: "ssh-directive-above-block", Section: "SSH", Severity: SeverityWarning, Family: "Coherence",
			Title:        "User directive precedes the managed block",
			SuggestedFix: "user content above a managed block is left untouched by design -- not offered as a fix.",
		}},
		{HealthFinding: HealthFinding{
			ID: "git-set-differs", Section: "Git", Severity: SeverityInfo, Family: "Baseline",
			Title: "init.defaultBranch: trunk (differs from recommendation)",
		}},
	}
}

// TestFixerCompleteFixableSet proves every Wave 2-5 fixable finding
// (Fixable == true) appears in the Fixer's fixableFindings-filtered list
// and its detail pane shows the "Fix this" affordance when selected — while
// every report-only Wave 3/5 finding (Fixable == false, even though each
// carries real non-empty advisory SuggestedFix text) is excluded (08-08
// code review CR-01: SuggestedFix non-emptiness is not the fixability
// signal).
func TestFixerCompleteFixableSet(t *testing.T) {
	all := wave2to5FixableFindings()
	state := DemoState{Scanned: true, Findings: all}

	var wantFixable []string
	for _, f := range all {
		if f.Fixable {
			wantFixable = append(wantFixable, f.ID)
		}
	}
	if len(wantFixable) != 6 {
		t.Fatalf("fixture sanity: want 6 fixable findings, got %d", len(wantFixable))
	}

	fixable := fixableFindings(orderedFindings(state))
	if len(fixable) != len(wantFixable) {
		t.Fatalf("fixableFindings = %d, want %d: %+v", len(fixable), len(wantFixable), fixable)
	}
	got := make(map[string]bool, len(fixable))
	for _, f := range fixable {
		got[f.ID] = true
	}
	for _, id := range wantFixable {
		if !got[id] {
			t.Errorf("fixableFindings missing Wave 2-5 fixable finding %q", id)
		}
	}
	for _, id := range []string{"ssh-shadowed-option", "git-author-resolution", "ssh-directive-above-block", "git-set-differs"} {
		if got[id] {
			t.Errorf("fixableFindings must exclude report-only finding %q", id)
		}
	}

	m := newDoctorModel(stubBackend{})
	for _, id := range wantFixable {
		m.selectedID = id
		view := m.view(state, 100, 30)
		if !strings.Contains(stripANSI(view.body), " f · Fix this… ") {
			t.Errorf("Doctor detail pane for %q must show the Fix-this affordance:\n%s", id, view.body)
		}
	}
}

// TestFixerSuggestedFixDropsStaleFixerHandoff proves 08-08's UX review
// finding F6: Doctor's own detail pane never renders the "available
// on the Fixer screen" hand-off clause SuggestedFix carries for Health's
// benefit -- it is stale once the user is already standing on Doctor. Health's own detail pane must still render the FULL text unchanged
// (health_screen_test.go covers that side).
func TestFixerSuggestedFixDropsStaleFixerHandoff(t *testing.T) {
	finding := DemoFinding{HealthFinding: HealthFinding{
		ID: "ssh-identitiesonly-contradiction", Section: "SSH", Severity: SeverityError, Family: "Coherence",
		Title:        "IdentitiesOnly no contradicts an explicit IdentityFile",
		SuggestedFix: "Set IdentitiesOnly yes on the clientb.github.com Host block -- available on the Fixer screen.",
		Fixable:      true,
	}}
	state := DemoState{Scanned: true, Findings: []DemoFinding{finding}}
	m := newDoctorModel(stubBackend{})
	m.selectedID = finding.ID
	view := stripANSI(m.view(state, 100, 30).body)
	if strings.Contains(view, "available on the Fixer screen") {
		t.Errorf("Doctor detail pane must not render the stale Fixer hand-off clause:\n%s", view)
	}
	for _, want := range []string{"Set IdentitiesOnly yes on the", "clientb.github.com Host block"} {
		if !strings.Contains(view, want) {
			t.Errorf("Doctor detail pane must still render the rest of the suggested-fix text (missing %q):\n%s", want, view)
		}
	}
}

// threeBatchFindings returns 3 fixable findings in strictly descending
// severity (Critical, Error, Warning) so orderedFindings' stable severity
// sort makes their walk order deterministic: fix-1, fix-2, fix-3.
func threeBatchFindings() []DemoFinding {
	return []DemoFinding{
		{HealthFinding: HealthFinding{
			ID: "fix-1", Section: "SSH", Severity: SeverityCritical, Family: "Permissions",
			Title: "Fix One", SuggestedFix: "chmod 0600 ~/.ssh/id_ed25519_one -- available on the Fixer screen.",
			Fixable: true,
		}},
		{HealthFinding: HealthFinding{
			ID: "fix-2", Section: "SSH", Severity: SeverityError, Family: "Coherence",
			Title: "Fix Two", SuggestedFix: "repair Fix Two -- available on the Fixer screen.",
			Fixable: true,
		}},
		{HealthFinding: HealthFinding{
			ID: "fix-3", Section: "Git", Severity: SeverityWarning, Family: "Orphans",
			Title: "Fix Three", SuggestedFix: "repair Fix Three -- available on the Fixer screen.",
			Fixable: true,
		}},
	}
}

// confirmFix drives one fix's ceremony to completion: two Enters (the first
// reaches the receipt, non-async fixCeremonyFor sets done=true directly;
// the second acknowledges the receipt and dispatches the FixFinding
// action — ceremony.go's own documented two-Enter contract for a
// non-Async ceremony).
func confirmFix(t *testing.T, a App) App {
	t.Helper()
	a, _ = press(t, a, "enter")
	a, _ = press(t, a, "enter")
	return a
}

// TestBatchWalkHalt proves 08-06-PLAN.md Task 3 (D-16): in a 3-fix batch
// walk where the SECOND fix's Persist fails, fix 1 stands (already
// succeeded, listed), fix 2's own ceremony shows the retryable failure
// state and the batch halt message names it, and fix 3 is never attempted
// (the batch's queue is cleared, not merely paused).
func TestBatchWalkHalt(t *testing.T) {
	backend := stubBackend{
		fixPersistErr:  errors.New("simulated write failure"),
		fixFailID:      "fix-2",
		lastPersistErr: new(error),
	}
	a := NewApp(backend)
	a.state.Scanned = true
	a.state.Findings = threeBatchFindings()
	a, _ = a.setTab(TabDoctor)
	fx, ok := a.screens[TabDoctor].(doctorModel)
	if !ok {
		t.Fatalf("screens[TabDoctor] is %T, want doctorModel", a.screens[TabDoctor])
	}
	fx.scanning = false
	a.screens[TabDoctor] = fx

	a, _ = press(t, a, "F")
	fx, _ = a.screens[TabDoctor].(doctorModel)
	if fx.batch == nil || fx.batch.queue[0] != "fix-1" {
		t.Fatalf("fixture sanity: batch walk order = %+v, want fix-1 first", fx.batch)
	}

	// Fix 1: succeeds (not fix-2, the configured failure).
	a = confirmFix(t, a)
	fx, _ = a.screens[TabDoctor].(doctorModel)
	if fx.batchHalt != "" {
		t.Fatalf("fix 1 must not halt the batch: %q", fx.batchHalt)
	}
	if len(fx.batchSucceeded) != 1 || fx.batchSucceeded[0] != "Fix One" {
		t.Fatalf("batchSucceeded after fix 1 = %v, want [Fix One]", fx.batchSucceeded)
	}
	if fx.selectedID != "fix-2" {
		t.Fatalf("selectedID after fix 1 = %q, want fix-2 (the walk must advance)", fx.selectedID)
	}

	// Fix 2: fails.
	a = confirmFix(t, a)
	fx, _ = a.screens[TabDoctor].(doctorModel)
	if fx.batch != nil {
		t.Error("batch must be nil once halted — fix 3 must never be attempted")
	}
	if fx.batchFailedName != "Fix Two" {
		t.Errorf("batchFailedName = %q, want %q", fx.batchFailedName, "Fix Two")
	}
	wantHalt := "Fix 2 of 3 failed and was rolled back from its own backup -- the first 1 fixes already applied stand. Nothing else in this batch was attempted."
	if fx.batchHalt != wantHalt {
		t.Errorf("batchHalt = %q, want %q", fx.batchHalt, wantHalt)
	}
	if fx.ceremony.commitErr == "" {
		t.Error("fix 2's own ceremony must show the retryable failure state (commitErr set)")
	}
	view := stripANSI(fx.view(a.state, 100, 30).body)
	for _, want := range []string{"Fix 2 of 3 failed and was rolled back", "the first 1 fixes already applied stand", "Nothing else in this batch was attempted"} {
		if !strings.Contains(view, want) {
			t.Errorf("Doctor view must render the halt message segment %q:\n%s", want, view)
		}
	}
	if !strings.Contains(view, "simulated write failure") {
		t.Errorf("Doctor view must render fix 2's own ceremony failure:\n%s", view)
	}

	// Findings state proves fix 1 genuinely applied (Reduce ran for it) and
	// fix 2/3 were never touched by Reduce (fix 2's Persist returned state
	// unchanged; fix 3 was never dispatched at all).
	stillPresent := map[string]bool{}
	for _, f := range a.state.Findings {
		stillPresent[f.ID] = true
	}
	if stillPresent["fix-1"] {
		t.Error("fix-1 must have been removed by a real Persist (it succeeded)")
	}
	if !stillPresent["fix-2"] {
		t.Error("fix-2 must still be present (its Persist failed and left state unchanged)")
	}
	if !stillPresent["fix-3"] {
		t.Error("fix-3 must still be present (it was never attempted)")
	}
}

// TestSingleFixFailureNoNonsensicalBatchMessage proves 08-08 code review
// WR-01: a single `f` fix (never part of an `F` batch walk) that fails
// must NOT render the D-16 batch-shaped "Fix N of M failed..." banner --
// haltBatch previously built that message unconditionally, producing the
// nonsensical "Fix 1 of 0 failed... this batch..." for a fix that was
// never in a batch. The ceremony's own retryable failure state (commitErr,
// "Retry"/"Cancel") must still render — the fix must not fail silently.
func TestSingleFixFailureNoNonsensicalBatchMessage(t *testing.T) {
	backend := stubBackend{
		fixPersistErr:  errors.New("simulated single-fix write failure"),
		fixFailID:      "fix-1",
		lastPersistErr: new(error),
	}
	a := NewApp(backend)
	a.state.Scanned = true
	a.state.Findings = threeBatchFindings()
	a, _ = a.setTab(TabDoctor)
	fx, ok := a.screens[TabDoctor].(doctorModel)
	if !ok {
		t.Fatalf("screens[TabDoctor] is %T, want doctorModel", a.screens[TabDoctor])
	}
	fx.scanning = false
	a.screens[TabDoctor] = fx

	// "f" on the single highest-severity finding — NOT "F" (no batch walk).
	a, _ = press(t, a, "f")
	fx, _ = a.screens[TabDoctor].(doctorModel)
	if fx.batch != nil {
		t.Fatalf("fixture sanity: single 'f' fix must never start a batch, got %+v", fx.batch)
	}
	a = confirmFix(t, a)
	fx, _ = a.screens[TabDoctor].(doctorModel)

	if fx.batchHalt != "" {
		t.Errorf("a single, non-batch fix failure must not set the batch-shaped halt message, got %q", fx.batchHalt)
	}
	if fx.ceremony.commitErr == "" {
		t.Error("the failed single fix's own ceremony must still show the retryable failure state (commitErr set)")
	}
	view := stripANSI(fx.view(a.state, 100, 30).body)
	if strings.Contains(view, "of 0 failed") || strings.Contains(view, "Fix 1 of 0") {
		t.Errorf("Doctor view must never render the nonsensical batch-shaped message for a single fix:\n%s", view)
	}
	if !strings.Contains(view, "simulated single-fix write failure") {
		t.Errorf("Doctor view must still render the single fix's own ceremony failure:\n%s", view)
	}
}

func TestParseErrorScreenRequiresFilesFamily(t *testing.T) {
	files := DemoFinding{HealthFinding: HealthFinding{Family: "Files", Severity: SeverityCritical, Section: "Git", Title: "Git configuration cannot be parsed", Explanation: "bad config"}}
	if _, ok := parseErrorFinding([]DemoFinding{files}); !ok {
		t.Fatal("Files-critical finding must select the parse-error frame")
	}
	perms := DemoFinding{HealthFinding: HealthFinding{Family: "Permissions", Severity: SeverityCritical, Section: "SSH", Title: "private key exposed"}}
	if _, ok := parseErrorFinding([]DemoFinding{perms}); ok {
		t.Fatal("Permissions-critical finding must not select the parse-error frame")
	}
}

func TestDoctorParseErrorFrameSuppressesOrdinaryFindings(t *testing.T) {
	m := newDoctorModel(stubBackend{})
	state := DemoState{Scanned: true, Findings: []DemoFinding{
		{HealthFinding: HealthFinding{Family: "Files", Severity: SeverityCritical, Section: "Git", Title: "Git configuration cannot be parsed", Explanation: "bad config"}},
		{HealthFinding: HealthFinding{Family: "Coherence", Severity: SeverityError, Section: "Git", Title: "misleading derived finding"}},
	}}
	view := m.view(state, 100, 30)
	if !strings.Contains(view.body, "Checks paused") || strings.Contains(view.body, "misleading derived finding") {
		t.Fatalf("parse-error frame = %q", view.body)
	}
}

func TestDoctorBrowseDoesNotShowWriteCeremonyMarkers(t *testing.T) {
	withFindings := Seed()
	withFindings.Scanned = true
	perIdentity := withFindings
	parseError := DemoState{Scanned: true, Findings: []DemoFinding{{
		HealthFinding: HealthFinding{Family: "Files", Severity: SeverityCritical, Section: "Git", Title: "Git configuration cannot be parsed", Explanation: "bad config"},
		Identity:      "legacy",
	}}}
	for name, tc := range map[string]struct {
		model doctorModel
		state DemoState
	}{
		"with-findings": {model: newDoctorModel(stubBackend{}), state: withFindings},
		"all-green":     {model: newDoctorModel(stubBackend{}), state: DemoState{Scanned: true}},
		"per-identity":  {model: doctorModel{identityName: "legacy"}, state: perIdentity},
		"parse-error":   {model: newDoctorModel(stubBackend{}), state: parseError},
	} {
		t.Run(name, func(t *testing.T) {
			view := stripANSI(tc.model.view(tc.state, 100, 30).body)
			for _, forbidden := range []string{"Apply fix", "Confirm write", "Backed up ->", "Wrote ->"} {
				if strings.Contains(view, forbidden) {
					t.Errorf("Doctor browse rendered forbidden write marker %q:\n%s", forbidden, view)
				}
			}
		})
	}
}

func TestDoctorNewScreensIsFiveAndDoctorTyped(t *testing.T) {
	screens := newScreens(stubBackend{}, DemoState{})
	if len(screens) != 5 {
		t.Fatalf("newScreens returned %d screens, want 5", len(screens))
	}
	if _, ok := screens[TabDoctor].(doctorModel); !ok {
		t.Fatalf("screens[TabDoctor] is %T, want doctorModel", screens[TabDoctor])
	}
}

func TestDoctorIdentityFilterResetsOnOrdinaryReentry(t *testing.T) {
	a := NewApp(stubBackend{})
	for identModel(t, a).selected != "clientB" {
		a = pressSeq(t, a, "down")
	}
	a, _ = press(t, a, "h")
	if a.tab != TabDoctor {
		t.Fatalf("tab after h = %v, want TabDoctor", a.tab)
	}
	if docModel(t, a).identityName != "clientB" {
		t.Fatalf("identityName after h = %q, want clientB", docModel(t, a).identityName)
	}
	model, _ := a.Update(doctorScanMsg{})
	a = model.(App)
	view := appView(a)
	if strings.Contains(view, "Private key is world-readable") {
		t.Fatalf("deep-link must stay filtered to clientB:\n%s", view)
	}
	a, _ = press(t, a, "1")
	if a.tab != TabIdentities {
		t.Fatalf("tab after 1 = %v, want TabIdentities", a.tab)
	}
	a, _ = press(t, a, "4")
	if a.tab != TabDoctor {
		t.Fatalf("tab after 4 = %v, want TabDoctor", a.tab)
	}
	if docModel(t, a).identityName != "" {
		t.Fatalf("ordinary re-entry must clear identityName, got %q", docModel(t, a).identityName)
	}
	view = appView(a)
	if !strings.Contains(view, "Private key is world-readable") {
		t.Fatalf("ordinary re-entry must show every identity's findings:\n%s", view)
	}
	if !strings.Contains(view, "IdentitiesOnly no contradicts") {
		t.Fatalf("ordinary re-entry must still include clientB's finding:\n%s", view)
	}
}

// TestDoctorBatchHaltSurvivesModelRename is T-09.4-01: checkFixBatchHalt's
// two type assertions must stay on doctorModel. A stale-but-compiling
// assertion (ok == false) silently turns the D-16 rollback into a no-op,
// so this guard asserts the halt MESSAGE TEXT, never merely "no panic".
func TestDoctorBatchHaltSurvivesModelRename(t *testing.T) {
	backend := stubBackend{
		fixPersistErr:  errors.New("simulated write failure"),
		fixFailID:      "fix-2",
		lastPersistErr: new(error),
	}
	a := NewApp(backend)
	a.state.Scanned = true
	a.state.Findings = threeBatchFindings()
	a, _ = a.setTab(TabDoctor)
	fx := docModel(t, a)
	fx.scanning = false
	a.screens[TabDoctor] = fx

	a, _ = press(t, a, "F")
	a = confirmFix(t, a)
	a = confirmFix(t, a)

	view := appView(a)
	for _, want := range []string{
		"Fix 2 of 3 failed and was rolled back",
		"the first 1 fixes already applied stand",
		"Nothing else in this batch was attempted",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("T-09.4-01: rendered pane must contain the D-16 halt segment %q:\n%s", want, view)
		}
	}
}

// TestDoctorFixGateRespectsFixableBit is T-09.4-02: the merged list widened
// to every finding, but f/F still gate on the Fixable bit. A non-fixable
// row stays listed; neither key opens a ceremony against it.
func TestDoctorFixGateRespectsFixableBit(t *testing.T) {
	a := doctorApp(t)
	a = pressSeq(t, a, "down", "down", "down", "down")
	if docModel(t, a).selectedID != "git-opensource-no-host-block" {
		t.Fatalf("selected = %q, want git-opensource-no-host-block", docModel(t, a).selectedID)
	}
	a, _ = press(t, a, "f")
	m := docModel(t, a)
	if m.fixing {
		t.Fatal("f on a non-fixable finding must leave fixing false")
	}
	if m.pendingFixID != "" {
		t.Fatalf("f on a non-fixable finding must leave pendingFixID empty, got %q", m.pendingFixID)
	}
	view := appView(a)
	if !strings.Contains(view, "opensource has no dedicated SSH") {
		t.Fatalf("list must still contain the non-fixable finding after f:\n%s", view)
	}

	a = NewApp(stubBackend{})
	a.state.Scanned = true
	a.state.Findings = []DemoFinding{{
		HealthFinding: HealthFinding{
			ID: "info-only", Section: "Git", Severity: SeverityInfo, Family: "Orphans",
			Title: "opensource has no dedicated SSH Host block",
		},
	}}
	a, _ = a.setTab(TabDoctor)
	fx := docModel(t, a)
	fx.scanning = false
	a.screens[TabDoctor] = fx
	a, _ = press(t, a, "F")
	fx = docModel(t, a)
	if fx.batch != nil || fx.fixing {
		t.Fatalf("F on an all-non-fixable fixture must start no batch, got batch=%+v fixing=%v", fx.batch, fx.fixing)
	}
	view = appView(a)
	if !strings.Contains(view, "opensource has no dedicated SSH") {
		t.Fatalf("all-non-fixable list must still contain the finding:\n%s", view)
	}
}

// TestDoctorCountIsTheOnlyFindingCount is UXP-05: the number Doctor's
// status line reports equals len(orderedFindings(state)) and is strictly
// greater than len(fixableFindings(...)) when the fixture has an info-only
// finding — the two numbers that used to disagree across two tabs now
// come from one source, and one is the superset.
func TestDoctorCountIsTheOnlyFindingCount(t *testing.T) {
	a := doctorApp(t)
	ordered := orderedFindings(a.state)
	fixable := fixableFindings(ordered)
	if len(fixable) >= len(ordered) {
		t.Fatalf("fixture sanity: need at least one info-only finding, ordered=%d fixable=%d", len(ordered), len(fixable))
	}
	view := appView(a)
	want := fmt.Sprintf("%d finding%s — every fix is previewed", len(ordered), pluralS(len(ordered)))
	if !strings.Contains(view, want) {
		t.Fatalf("UXP-05: Doctor status must report the unfiltered count %q:\n%s", want, view)
	}
	wrong := fmt.Sprintf("%d finding%s — every fix is previewed", len(fixable), pluralS(len(fixable)))
	if strings.Contains(view, wrong) {
		t.Fatalf("UXP-05: Doctor status must not report the fixable-only count %q:\n%s", wrong, view)
	}
}
