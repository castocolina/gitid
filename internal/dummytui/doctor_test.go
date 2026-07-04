package dummytui

import (
	"strings"
	"testing"
)

// doctorApp returns an App on the Doctor tab with the scan completed.
func doctorApp(t *testing.T) App {
	t.Helper()
	a, cmd := press(t, NewApp(), "4")
	if cmd == nil {
		t.Fatal("first Doctor entry must schedule the auto-scan tick")
	}
	if !strings.Contains(appView(a), "running doctor scan…") {
		t.Fatal("scanning state missing")
	}
	model, _ := a.Update(doctorScanMsg{})
	return model.(App)
}

// docModel extracts the Doctor child model.
func docModel(t *testing.T, a App) doctorModel {
	t.Helper()
	m, ok := a.screens[tabDoctor].(doctorModel)
	if !ok {
		t.Fatalf("screens[3] is %T, want doctorModel", a.screens[tabDoctor])
	}
	return m
}

func TestDoctorAutoScanThenGroupedFindings(t *testing.T) {
	a := doctorApp(t)
	if !a.state.Scanned {
		t.Fatal("scan completion must dispatch MarkScanned")
	}
	view := appView(a)

	// Grouped SSH-then-Git with per-identity subgroup labels.
	for _, group := range []string{"SSH · archived", "SSH · clientB", "SSH · global", "Git · legacy", "Git · opensource"} {
		if !strings.Contains(view, group) {
			t.Errorf("group label %q missing", group)
		}
	}
	sshIdx := strings.Index(view, "SSH · archived")
	gitIdx := strings.Index(view, "Git · legacy")
	if sshIdx == -1 || gitIdx == -1 || sshIdx > gitIdx {
		t.Error("SSH groups must render before Git groups")
	}

	// Later visits are instant (no scan tick).
	a, _ = press(t, a, "1")
	a, cmd := press(t, a, "4")
	if cmd != nil {
		t.Error("revisiting the Doctor must not re-run the scan")
	}
	if !strings.Contains(appView(a), "SSH · archived") {
		t.Error("findings must render instantly on later visits")
	}
}

func TestDoctorSeverityRenderPinsLockedContract(t *testing.T) {
	view := appView(doctorApp(t))
	for _, want := range []string{"✗ critical", "✗ error", "! warning", "~ info"} {
		if !strings.Contains(view, want) {
			t.Errorf("severity render missing %q (locked glyph+word contract)", want)
		}
	}
	if strings.Contains(view, "✗ warning") {
		t.Error("NEVER ✗ for a warning")
	}
}

func TestDoctorDetailAndInfoOnlyFinding(t *testing.T) {
	a := doctorApp(t)
	// Ordered: critical(perms), error(contradiction), error(includeif),
	// warning(dup host star), info(opensource). Move to the info finding.
	a = pressSeq(t, a, "down", "down", "down", "down")
	m := docModel(t, a)
	if m.selectedID != "git-opensource-no-host-block" {
		t.Fatalf("selected = %q", m.selectedID)
	}
	view := appView(a)
	if !strings.Contains(view, "Informational only — nothing to fix.") {
		t.Error("info-only finding must render the nothing-to-fix alert")
	}
	// f on an info finding is a no-op.
	a, _ = press(t, a, "f")
	if docModel(t, a).fixing {
		t.Error("f must not open a ceremony for an unfixable finding")
	}
}

func TestDoctorFixThisRemovesFindingLive(t *testing.T) {
	a := doctorApp(t)
	countsBefore := CountFindings(a.state)

	// Selected defaults to the first ordered finding: the critical perms.
	a, _ = press(t, a, "f")
	view := appView(a)
	if !strings.Contains(view, "Fix: Private key is world-readable") {
		t.Fatalf("fix ceremony missing:\n%s", view)
	}
	a, _ = press(t, a, "enter") // confirm
	if !strings.Contains(appView(a), "chmod 0600 ~/.ssh/id_ed25519_archived applied.") {
		t.Error("fix receipt missing")
	}
	a, _ = press(t, a, "enter") // done → dispatch

	if hasFinding(a.state, "ssh-key-perms-archived") {
		t.Error("fixed finding must disappear LIVE")
	}
	counts := CountFindings(a.state)
	if counts.Errors != countsBefore.Errors-1 {
		t.Errorf("errors = %d, want %d (chip decrements)", counts.Errors, countsBefore.Errors-1)
	}
	if !strings.Contains(appView(a), "✗ 2") {
		t.Error("header chip must show the decremented count")
	}
}

func TestDoctorFixAllWalksEveryFixableWithCounter(t *testing.T) {
	a := doctorApp(t)
	fixable := fixableFindings(orderedFindings(a.state))
	if len(fixable) != 4 {
		t.Fatalf("fixable = %d, want 4", len(fixable))
	}
	if !strings.Contains(appView(a), "fix all (4)") {
		t.Error("footer must offer fix all (4)")
	}

	a, _ = press(t, a, "F")
	if !strings.Contains(regionFlat(a, 45, 100), "Fix all — 0 / 4 fixed; each change still previews its own diff and backup before writing.") {
		t.Fatalf("batch banner missing:\n%s", appView(a))
	}

	// Fix 1: critical perms (plain).
	a, _ = press(t, a, "enter")
	a, _ = press(t, a, "enter")
	pane := regionFlat(a, 45, 100)
	if !strings.Contains(pane, "Fix all — 1 / 4 fixed") {
		t.Errorf("counter must advance to 1 / 4:\n%s", pane)
	}

	// Fix 2: the IdentitiesOnly contradiction — DESTRUCTIVE, its own
	// ceremony with the typed Host name (never a silent batch).
	if !strings.Contains(pane, "Fix: IdentitiesOnly no contradicts an explicit IdentityFile") {
		t.Fatal("second ceremony must render for the next finding")
	}
	a, _ = press(t, a, "enter") // no-op until the word is typed
	if !docModel(t, a).fixing {
		t.Fatal("destructive batch step must stay gated")
	}
	a = typeText(t, a, "clientb.github.com")
	a, _ = press(t, a, "enter")
	a, _ = press(t, a, "enter")
	if !strings.Contains(appView(a), "Fix all — 2 / 4 fixed") {
		t.Error("counter must advance to 2 / 4")
	}

	// Fix 3: the legacy includeIf — heals "legacy" to complete.
	a, _ = press(t, a, "enter")
	a, _ = press(t, a, "enter")
	legacy := findIdentity(t, a.state, "legacy")
	if legacy.State != "complete" {
		t.Errorf("legacy state = %q, want complete (healed by the batch fix)", legacy.State)
	}

	// Fix 4: the duplicate Host *.
	a, _ = press(t, a, "enter")
	a, _ = press(t, a, "enter")

	m := docModel(t, a)
	if m.fixing || m.batch != nil {
		t.Error("batch must end after the last fixable finding")
	}
	counts := CountFindings(a.state)
	if counts.Warnings != 0 || counts.Errors != 0 {
		t.Errorf("counts after fix-all = %+v, want zero", counts)
	}
	// Only the info finding remains.
	if len(a.state.Findings) != 1 || a.state.Findings[0].ID != "git-opensource-no-host-block" {
		t.Errorf("remaining findings = %v", a.state.Findings)
	}
	if !strings.Contains(appView(a), "✓ ok") {
		t.Error("header chip must flip to ✓ ok once warnings/errors are gone")
	}
}

func TestDoctorEscCancelsBatchRemainder(t *testing.T) {
	a := doctorApp(t)
	a, _ = press(t, a, "F")
	a, _ = press(t, a, "enter") // confirm fix 1
	a, _ = press(t, a, "enter") // done fix 1 → fix 2 ceremony renders
	a, _ = press(t, a, "esc")   // cancel the remainder
	m := docModel(t, a)
	if m.fixing || m.batch != nil {
		t.Error("Esc must cancel the remainder of the batch")
	}
	if len(a.state.Findings) != 4 {
		t.Errorf("findings = %d, want 4 (only the first fix applied)", len(a.state.Findings))
	}
}

func TestDoctorAllGreenRendersBothSummaries(t *testing.T) {
	a := doctorApp(t)
	clean := a.state
	clean.Findings = nil
	a.state = clean
	view := appView(a)
	if !strings.Contains(view, FixerNothingToFixSSH) {
		t.Error("all-green SSH summary missing")
	}
	if !strings.Contains(view, FixerNothingToFixGit) {
		t.Error("all-green Git summary missing")
	}
}

func TestDoctorListDimsDuringFixCeremony(t *testing.T) {
	// The findings list dims while the fix ceremony owns the pane (L3 —
	// web: opacity 0.75), the same treatment as the Identities sidebar.
	a := doctorApp(t)
	a, _ = press(t, a, "f")
	if !docModel(t, a).fixing {
		t.Fatal("f must open the fix ceremony")
	}
	raw := a.View().Content
	// Every list row renders faint (SGR 2) with its own styling stripped.
	if !strings.Contains(raw, "\x1b[2m") {
		t.Fatal("dimmed list missing faint rendering")
	}
	for _, line := range strings.Split(raw, "\n") {
		if strings.Contains(stripANSI(line), "SSH · archived") && !strings.Contains(line, "\x1b[2m") {
			t.Error("group label row must render faint during the ceremony")
		}
	}
}
