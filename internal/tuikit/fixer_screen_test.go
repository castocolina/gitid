package tuikit

import (
	"errors"
	"strings"
	"testing"
)

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

	m := newFixerModel(stubBackend{})
	for _, id := range wantFixable {
		m.selectedID = id
		view := m.view(state, 100, 30)
		if !strings.Contains(stripANSI(view.body), " f · Fix this… ") {
			t.Errorf("Fixer detail pane for %q must show the Fix-this affordance:\n%s", id, view.body)
		}
	}
}

// TestFixerSuggestedFixDropsStaleFixerHandoff proves 08-08's UX review
// finding F6: the Fixer tab's own detail pane never renders the "available
// on the Fixer screen" hand-off clause SuggestedFix carries for Health's
// benefit -- it is stale once the user is already standing on the Fixer
// tab. Health's own detail pane must still render the FULL text unchanged
// (health_screen_test.go covers that side).
func TestFixerSuggestedFixDropsStaleFixerHandoff(t *testing.T) {
	finding := DemoFinding{HealthFinding: HealthFinding{
		ID: "ssh-identitiesonly-contradiction", Section: "SSH", Severity: SeverityError, Family: "Coherence",
		Title:        "IdentitiesOnly no contradicts an explicit IdentityFile",
		SuggestedFix: "Set IdentitiesOnly yes on the clientb.github.com Host block -- available on the Fixer screen.",
		Fixable:      true,
	}}
	state := DemoState{Scanned: true, Findings: []DemoFinding{finding}}
	m := newFixerModel(stubBackend{})
	m.selectedID = finding.ID
	view := stripANSI(m.view(state, 100, 30).body)
	if strings.Contains(view, "available on the Fixer screen") {
		t.Errorf("Fixer detail pane must not render the stale Fixer hand-off clause:\n%s", view)
	}
	for _, want := range []string{"Set IdentitiesOnly yes on the", "clientb.github.com Host block"} {
		if !strings.Contains(view, want) {
			t.Errorf("Fixer detail pane must still render the rest of the suggested-fix text (missing %q):\n%s", want, view)
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
	a, _ = a.setTab(TabFixer)
	fx, ok := a.screens[TabFixer].(fixerModel)
	if !ok {
		t.Fatalf("screens[TabFixer] is %T, want fixerModel", a.screens[TabFixer])
	}
	fx.scanning = false
	a.screens[TabFixer] = fx

	a, _ = press(t, a, "F")
	fx, _ = a.screens[TabFixer].(fixerModel)
	if fx.batch == nil || fx.batch.queue[0] != "fix-1" {
		t.Fatalf("fixture sanity: batch walk order = %+v, want fix-1 first", fx.batch)
	}

	// Fix 1: succeeds (not fix-2, the configured failure).
	a = confirmFix(t, a)
	fx, _ = a.screens[TabFixer].(fixerModel)
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
	fx, _ = a.screens[TabFixer].(fixerModel)
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
	view := stripANSI(fx.view(fixableState(a.state), 100, 30).body)
	for _, want := range []string{"Fix 2 of 3 failed and was rolled back", "the first 1 fixes already applied stand", "Nothing else in this batch was attempted"} {
		if !strings.Contains(view, want) {
			t.Errorf("Fixer view must render the halt message segment %q:\n%s", want, view)
		}
	}
	if !strings.Contains(view, "simulated write failure") {
		t.Errorf("Fixer view must render fix 2's own ceremony failure:\n%s", view)
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
	a, _ = a.setTab(TabFixer)
	fx, ok := a.screens[TabFixer].(fixerModel)
	if !ok {
		t.Fatalf("screens[TabFixer] is %T, want fixerModel", a.screens[TabFixer])
	}
	fx.scanning = false
	a.screens[TabFixer] = fx

	// "f" on the single highest-severity finding — NOT "F" (no batch walk).
	a, _ = press(t, a, "f")
	fx, _ = a.screens[TabFixer].(fixerModel)
	if fx.batch != nil {
		t.Fatalf("fixture sanity: single 'f' fix must never start a batch, got %+v", fx.batch)
	}
	a = confirmFix(t, a)
	fx, _ = a.screens[TabFixer].(fixerModel)

	if fx.batchHalt != "" {
		t.Errorf("a single, non-batch fix failure must not set the batch-shaped halt message, got %q", fx.batchHalt)
	}
	if fx.ceremony.commitErr == "" {
		t.Error("the failed single fix's own ceremony must still show the retryable failure state (commitErr set)")
	}
	view := stripANSI(fx.view(fixableState(a.state), 100, 30).body)
	if strings.Contains(view, "of 0 failed") || strings.Contains(view, "Fix 1 of 0") {
		t.Errorf("Fixer view must never render the nonsensical batch-shaped message for a single fix:\n%s", view)
	}
	if !strings.Contains(view, "simulated single-fix write failure") {
		t.Errorf("Fixer view must still render the single fix's own ceremony failure:\n%s", view)
	}
}
