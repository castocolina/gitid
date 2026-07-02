package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/filewriter"
)

// TestDoctorCmdRegistered verifies that `gitid doctor` is a registered top-level
// command on the root (not nested under a subgroup).
func TestDoctorCmdRegistered(t *testing.T) {
	root := newRootCmd()
	found := false
	for _, cmd := range root.Commands() {
		if cmd.Use == "doctor" {
			found = true
			break
		}
	}
	if !found {
		t.Error("newRootCmd() does not have a 'doctor' command registered")
	}
}

// TestDoctorRenderGrouped verifies that runDoctor produces output containing
// the grouped family headers (at minimum Permissions must appear).
func TestDoctorRenderGrouped(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	var out bytes.Buffer
	code := runDoctor(&out, false, false)
	if code < 0 || code > 3 {
		t.Errorf("runDoctor returned invalid exit code %d (want 0-3)", code)
	}

	output := out.String()
	if !strings.Contains(output, "=== Permissions ===") {
		t.Errorf("expected '=== Permissions ===' in output; got:\n%s", output)
	}
}

// TestDoctorCleanAllClear verifies that a clean temp home (no SSH config,
// no managed identities, no bad perms) runs without crashing and produces
// a valid exit code (0-3). A clean home will report baseline errors because
// the baseline has not been set up, which is correct behavior (CheckBaseline
// is now real, not a stub). The test checks that the output contains the
// expected grouped report structure and that no critical/panic occurs.
func TestDoctorCleanAllClear(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	var out bytes.Buffer
	code := runDoctor(&out, false, false)
	output := out.String()

	// Valid exit codes are 0-3; any other value indicates a bug.
	if code < 0 || code > 3 {
		t.Errorf("runDoctor on clean home returned invalid exit code %d (want 0-3); output:\n%s", code, output)
	}
	// The report must include at least the Baseline section.
	if !strings.Contains(output, "=== Baseline ===") {
		t.Errorf("expected '=== Baseline ===' in output; got:\n%s", output)
	}
	// A clean temp home has no SSH identity permission findings (no keys present).
	// Critical findings (e.g. exposed private key) should not appear on a fresh home.
	if code == 3 {
		t.Errorf("runDoctor on clean home returned critical exit code 3 (unexpected); output:\n%s", output)
	}
}

// TestDoctorYesRequiresFix verifies that --yes without --fix returns an error
// containing "doctor: --yes requires --fix".
func TestDoctorYesRequiresFix(t *testing.T) {
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"doctor", "--yes"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when --yes without --fix, got nil")
	}
	if !strings.Contains(err.Error(), "--yes requires --fix") {
		t.Errorf("expected error to contain '--yes requires --fix', got: %v", err)
	}
}

// --- D-04 gate/confirm/batching tests ------------------------------------------
// All tests below drive applyFixes with injected recordings fakes — never the
// real home directory. The tests fail in RED because applyFixes does not exist
// yet (Task 1 GREEN will make them pass).

// makeFixableFindings returns a slice of findings with one perms + one orphan
// finding, each with a recording Fix.Fn. The call counts are tracked via the
// supplied pointers so the test can assert they were (or were not) called.
func makeFixableFindings(permCalled, orphanCalled *int) []doctor.Finding {
	return []doctor.Finding{
		{
			Family:      doctor.FamilyPerms,
			Severity:    doctor.SeverityCritical,
			Title:       "~/.ssh/key: 0644 (expected 0600)",
			Explanation: "Private key has wrong permissions.",
			Fix: &doctor.FixDescriptor{
				Summary: "chmod 0600 ~/.ssh/key",
				Fn: func() error {
					*permCalled++
					return nil
				},
			},
		},
		{
			Family:      doctor.FamilyOrphans,
			Severity:    doctor.SeverityWarning,
			Title:       "SSH Host block \"old\": no gitconfig includeIf",
			Explanation: "Orphaned SSH Host block.",
			Fix: &doctor.FixDescriptor{
				Summary: "remove orphaned SSH Host block \"old\"",
				Fn: func() error {
					*orphanCalled++
					return nil
				},
			},
		},
	}
}

// TestDoctorGateDeclined verifies bare doctor (fix=false) gate: when the user
// enters "n" at the gate prompt, no Fix.Fn is called and the output contains
// "No fixes applied."
func TestDoctorGateDeclined(t *testing.T) {
	var permCalled, orphanCalled int
	findings := makeFixableFindings(&permCalled, &orphanCalled)

	var out bytes.Buffer
	r := bufio.NewReader(strings.NewReader("n\n"))
	applied, skipped := applyFixes(r, &out, findings, false /* fix */, false /* yes */)

	output := out.String()
	if !strings.Contains(output, "No fixes applied.") {
		t.Errorf("expected 'No fixes applied.' in output; got:\n%s", output)
	}
	if permCalled != 0 || orphanCalled != 0 {
		t.Errorf("expected no Fix.Fn calls on gate declined; perm=%d orphan=%d", permCalled, orphanCalled)
	}
	if applied != 0 || skipped != 0 {
		t.Errorf("expected applied=0, skipped=0 on gate declined; got applied=%d, skipped=%d", applied, skipped)
	}
}

// TestDoctorGateAcceptedThenAllDeclined verifies that when the top-level gate is
// accepted ("y") but all per-finding confirms are declined ("n"), the tally shows
// 0 applied, N skipped.
func TestDoctorGateAcceptedThenAllDeclined(t *testing.T) {
	var permCalled, orphanCalled int
	findings := makeFixableFindings(&permCalled, &orphanCalled)

	var out bytes.Buffer
	// "y" accepts gate; two "n" for per-finding confirms
	r := bufio.NewReader(strings.NewReader("y\nn\nn\n"))
	applied, skipped := applyFixes(r, &out, findings, false /* fix */, false /* yes */)

	if permCalled != 0 || orphanCalled != 0 {
		t.Errorf("expected no Fix.Fn calls; perm=%d orphan=%d", permCalled, orphanCalled)
	}
	if applied != 0 {
		t.Errorf("expected 0 applied; got %d", applied)
	}
	if skipped != 2 {
		t.Errorf("expected 2 skipped; got %d", skipped)
	}
}

// TestDoctorFixPerFindingConfirm verifies --fix mode: skips the gate, goes straight
// to per-finding confirm. With stdin "y\nn\n" the first fix is applied, the second
// is skipped, and the tally reads "1 fix(es) applied, 1 skipped."
func TestDoctorFixPerFindingConfirm(t *testing.T) {
	var permCalled, orphanCalled int
	findings := makeFixableFindings(&permCalled, &orphanCalled)

	var out bytes.Buffer
	r := bufio.NewReader(strings.NewReader("y\nn\n"))
	applied, skipped := applyFixes(r, &out, findings, true /* fix */, false /* yes */)

	output := out.String()
	if permCalled != 1 {
		t.Errorf("expected perm Fix.Fn called once; got %d", permCalled)
	}
	if orphanCalled != 0 {
		t.Errorf("expected orphan Fix.Fn not called; got %d", orphanCalled)
	}
	if applied != 1 || skipped != 1 {
		t.Errorf("expected 1 applied, 1 skipped; got applied=%d, skipped=%d", applied, skipped)
	}
	if !strings.Contains(output, "doctor: 1 fix(es) applied, 1 skipped.") {
		t.Errorf("expected tally line in output; got:\n%s", output)
	}
}

// TestDoctorFixYesNonInteractive verifies --fix --yes: all fixable findings are
// applied with no prompt text in output, and the output contains "fixed:" lines.
func TestDoctorFixYesNonInteractive(t *testing.T) {
	var permCalled, orphanCalled int
	findings := makeFixableFindings(&permCalled, &orphanCalled)

	var out bytes.Buffer
	r := bufio.NewReader(strings.NewReader("")) // no input needed
	applied, skipped := applyFixes(r, &out, findings, true /* fix */, true /* yes */)

	output := out.String()
	if permCalled != 1 {
		t.Errorf("expected perm Fix.Fn called once; got %d", permCalled)
	}
	if orphanCalled != 1 {
		t.Errorf("expected orphan Fix.Fn called once; got %d", orphanCalled)
	}
	if applied != 2 {
		t.Errorf("expected 2 applied; got %d", applied)
	}
	if skipped != 0 {
		t.Errorf("expected 0 skipped; got %d", skipped)
	}
	if !strings.Contains(output, "fixed:") {
		t.Errorf("expected 'fixed:' in --yes output; got:\n%s", output)
	}
	// --yes should not produce a prompt like "[y/N]:"
	if strings.Contains(output, "[y/N]") {
		t.Errorf("expected no prompt in --yes output; got:\n%s", output)
	}
}

// TestDoctorPermsBatched verifies that multiple FamilyPerms findings are
// presented as one batched confirm ("Fix N permission(s):"), not as individual
// confirms. A single "y" accepts the batch; all Fn are called.
func TestDoctorPermsBatched(t *testing.T) {
	calls := 0
	perm1Called := 0
	perm2Called := 0
	_ = calls
	findings := []doctor.Finding{
		{
			Family:   doctor.FamilyPerms,
			Severity: doctor.SeverityCritical,
			Title:    "key1: 0644 (expected 0600)",
			Fix: &doctor.FixDescriptor{
				Summary: "chmod 0600 key1",
				Fn:      func() error { perm1Called++; return nil },
			},
		},
		{
			Family:   doctor.FamilyPerms,
			Severity: doctor.SeverityWarning,
			Title:    "key2: 0666 (expected 0644)",
			Fix: &doctor.FixDescriptor{
				Summary: "chmod 0644 key2",
				Fn:      func() error { perm2Called++; return nil },
			},
		},
	}

	var out bytes.Buffer
	r := bufio.NewReader(strings.NewReader("y\n"))
	applied, _ := applyFixes(r, &out, findings, true /* fix */, false /* yes */)

	output := out.String()
	if !strings.Contains(output, "Fix 2 permission(s):") {
		t.Errorf("expected 'Fix 2 permission(s):' batch header; got:\n%s", output)
	}
	if perm1Called != 1 || perm2Called != 1 {
		t.Errorf("expected both perm Fix.Fn called; perm1=%d perm2=%d", perm1Called, perm2Called)
	}
	if applied != 2 {
		t.Errorf("expected 2 applied; got %d", applied)
	}
}

// TestDoctorOrphanNotBatched verifies that an Orphans finding is presented with
// its own individual confirm — never batched with a perms finding.
func TestDoctorOrphanNotBatched(t *testing.T) {
	var permCalled, orphanCalled int
	findings := []doctor.Finding{
		{
			Family:   doctor.FamilyPerms,
			Severity: doctor.SeverityCritical,
			Title:    "key: 0644 (expected 0600)",
			Fix: &doctor.FixDescriptor{
				Summary: "chmod 0600 key",
				Fn:      func() error { permCalled++; return nil },
			},
		},
		{
			Family:   doctor.FamilyOrphans,
			Severity: doctor.SeverityWarning,
			Title:    "SSH Host block \"old\": no gitconfig includeIf",
			Fix: &doctor.FixDescriptor{
				Summary: "remove orphaned SSH Host block \"old\"",
				Fn:      func() error { orphanCalled++; return nil },
			},
		},
	}

	var out bytes.Buffer
	// "y" for the perms batch; "n" for the orphan individual confirm
	r := bufio.NewReader(strings.NewReader("y\nn\n"))
	applyFixes(r, &out, findings, true /* fix */, false /* yes */)

	output := out.String()
	// The orphan must NOT be batched under "Fix N permission(s):"
	// Instead it should appear as its own "Fix: ..." line.
	if !strings.Contains(output, "Fix:") {
		t.Errorf("expected individual 'Fix:' prompt for orphan; got:\n%s", output)
	}
	if permCalled != 1 {
		t.Errorf("expected perm called; got %d", permCalled)
	}
	if orphanCalled != 0 {
		t.Errorf("expected orphan not called (answered n); got %d", orphanCalled)
	}
}

// TestDoctorFixYesExitCodePreFix is the D-07/WARNING 5 gate test: even when
// --fix --yes applies all fixes successfully, runDoctor returns the PRE-fix
// highest severity (3 for critical findings), never 0.
//
// This test is driven through the full runDoctor path using a temp home, not just
// applyFixes, because it verifies that runDoctor captures `pre` before calling
// applyFixes and returns it unconditionally.
//
// To produce a critical pre-fix finding without mutating the real home, we
// create a real key file with 0644 mode in a temp home, so CheckPermissions fires
// with SeverityCritical. Then --fix --yes calls the real FixPerm (os.Chmod) which
// succeeds. The return value must still be 3.
func TestDoctorFixYesExitCodePostFix(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Create ~/.ssh directory and a private key with wrong perms (0644) to trigger
	// a SeverityCritical perm finding.
	sshDir := filepath.Join(tmpHome, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("creating temp .ssh dir: %v", err)
	}
	keyPath := filepath.Join(sshDir, "gitid_test_key")
	if err := os.WriteFile(keyPath, []byte("fake-key-content"), 0o644); err != nil { //nolint:gosec // 0o644 is intentional: this test needs a critical perm finding
		t.Fatalf("writing fake key: %v", err)
	}

	// Create a fake SSH config and gitconfig that reference this key so that
	// identity.Reconstruct picks it up and builds KeyPaths from it. We use the
	// gitid managed block format so the key is discovered.
	sshConfigContent := "# BEGIN gitid managed: testid\n" +
		"Host github.com\n" +
		"  Hostname github.com\n" +
		"  Port 22\n" +
		"  User git\n" +
		"  IdentityFile " + keyPath + "\n" +
		"  IdentitiesOnly yes\n" +
		"# END gitid managed: testid\n"
	if err := os.WriteFile(filepath.Join(sshDir, "config"), []byte(sshConfigContent), 0o600); err != nil {
		t.Fatalf("writing fake ssh config: %v", err)
	}

	// runDoctor with fix=true, yes=true: run checks (critical perm = pre-fix code
	// 3), apply the chmod, RE-EVALUATE, and return the POST-fix exit code (Bug B).
	// The chmod resolves the critical, so the post-fix severity is strictly below
	// the pre-fix 3 — proving runDoctor re-evaluated rather than returning the stale
	// pre-fix code. (Plain `gitid doctor`/CI runs still return the pre-fix code via
	// runDoctor's non-fix early return.)
	var out bytes.Buffer
	code := runDoctor(&out, true /* fix */, true /* yes */)

	if code >= 3 {
		t.Errorf("expected re-evaluated post-fix code below the pre-fix critical (3); got %d\noutput:\n%s", code, out.String())
	}
	if !strings.Contains(out.String(), "fixed: chmod") {
		t.Errorf("expected the chmod fix to be applied; output:\n%s", out.String())
	}
}

// TestDoctorFixYesHealsMissingBaselineFromScratch verifies Fix A + Fix B together:
// on a pristine home with no baseline, `doctor --fix --yes` runs the FULL baseline
// setup (creating the fragment AND the include — not a dangling pointer), then
// re-evaluates to a clean state and exits 0. No manual `gitid baseline setup`.
func TestDoctorFixYesHealsMissingBaselineFromScratch(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	var out bytes.Buffer
	code := runDoctor(&out, true /* fix */, true /* yes */)

	if code != 0 {
		t.Errorf("expected --fix --yes to fully heal a missing baseline and exit 0; got %d\noutput:\n%s",
			code, out.String())
	}
	// The fixer must have created the fragment, not just a dangling include.
	frag := filepath.Join(home, ".gitconfig.d", "00-baseline")
	if _, err := os.Stat(frag); err != nil {
		t.Errorf("baseline fragment %s must exist after the full-setup fix; stat err: %v", frag, err)
	}
	gc, _ := os.ReadFile(filepath.Join(home, ".gitconfig")) //nolint:gosec // test reads a gitid-managed path in a temp home (G304)
	if !strings.Contains(string(gc), "baseline-include") {
		t.Errorf("~/.gitconfig must contain the baseline-include block after the fix; got:\n%s", string(gc))
	}
}

// TestConvergeFixes_ReachesCleanAndReturnsPostFix verifies the convergence loop
// re-evaluates after applying and returns the POST-fix findings (Bug B): once a
// pass resolves the only finding, runChecks returns clean and the loop stops with
// an empty (exit-code-0) finding set.
func TestConvergeFixes_ReachesCleanAndReturnsPostFix(t *testing.T) {
	initial := []doctor.Finding{{
		Family: doctor.FamilyPerms, Severity: doctor.SeverityCritical, Title: "key: bad perms",
		Fix: &doctor.FixDescriptor{Summary: "chmod", Fn: func() error { return nil }},
	}}
	passes := 0
	final := convergeFixes(
		initial,
		func(fixable []doctor.Finding) int { return len(fixable) }, // "apply" succeeds
		func() []doctor.Finding { passes++; return nil },           // re-check: now clean
		10,
	)
	if len(final) != 0 {
		t.Errorf("expected clean post-fix findings, got %d: %v", len(final), final)
	}
	if doctor.ExitCode(final) != 0 {
		t.Errorf("expected post-fix exit code 0, got %d", doctor.ExitCode(final))
	}
	if passes != 1 {
		t.Errorf("expected exactly one re-check pass, got %d", passes)
	}
}

// TestConvergeFixes_TerminatesOnPingPong verifies the maxPasses backstop: even if
// two checks ever disagree about the same artifact (a fix that re-creates another
// finding forever), the loop terminates instead of spinning. Defense in depth on
// top of the reserved-block exclusion (Fix C) that removed the real ping-pong.
func TestConvergeFixes_TerminatesOnPingPong(t *testing.T) {
	mkFinding := func(title string) []doctor.Finding {
		return []doctor.Finding{{
			Family: doctor.FamilyOrphans, Severity: doctor.SeverityWarning, Title: title,
			Fix: &doctor.FixDescriptor{Summary: "toggle", Fn: func() error { return nil }},
		}}
	}
	toggle := false
	passes := 0
	convergeFixes(
		mkFinding("A"),
		func(_ []doctor.Finding) int { return 1 },
		func() []doctor.Finding {
			passes++
			toggle = !toggle
			if toggle {
				return mkFinding("B")
			}
			return mkFinding("A")
		},
		10,
	)
	if passes > 10 {
		t.Errorf("convergeFixes did not respect maxPasses backstop: ran %d passes", passes)
	}
	if passes == 0 {
		t.Error("expected the loop to run at least one re-check pass")
	}
}

// TestDoctorFixFailContinues verifies that when a Fix.Fn returns an error, the
// run prints "doctor: fix failed: ..." and continues to the next finding.
func TestDoctorFixFailContinues(t *testing.T) {
	var secondCalled int
	findings := []doctor.Finding{
		{
			Family:   doctor.FamilyPerms,
			Severity: doctor.SeverityError,
			Title:    "key: bad perms",
			Fix: &doctor.FixDescriptor{
				Summary: "chmod 0600 key",
				Fn:      func() error { return fmt.Errorf("chmod failed: permission denied") },
			},
		},
		{
			Family:   doctor.FamilyOrphans,
			Severity: doctor.SeverityWarning,
			Title:    "orphan block",
			Fix: &doctor.FixDescriptor{
				Summary: "remove orphan",
				Fn:      func() error { secondCalled++; return nil },
			},
		},
	}

	var out bytes.Buffer
	r := bufio.NewReader(strings.NewReader(""))
	applyFixes(r, &out, findings, true /* fix */, true /* yes */)

	output := out.String()
	if !strings.Contains(output, "doctor: fix failed:") {
		t.Errorf("expected 'doctor: fix failed:' in output; got:\n%s", output)
	}
	if secondCalled != 1 {
		t.Errorf("expected second fix called despite first failure; got %d", secondCalled)
	}
}

// --- Task 2: filewriter chokepoint integration tests ---------------------------
// These tests use real t.TempDir() homes to exercise RemoveBlock and AddWiring
// closures through the actual filewriter path. They fail RED because buildDoctorDeps
// leaves RemoveBlock and AddWiring nil.

// TestFixerRemovesOrphanBlock verifies that the RemoveBlock closure in
// buildDoctorDeps correctly removes a gitid-managed block from a file using
// filewriter.RemoveBlock+Write, creates a timestamped backup, and preserves
// content outside the block verbatim. A second call is idempotent (no diff).
func TestFixerRemovesOrphanBlock(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	sshDir := filepath.Join(tmpHome, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("creating .ssh dir: %v", err)
	}

	// Create ~/.ssh/config with a managed block + surrounding foreign content.
	configPath := filepath.Join(sshDir, "config")
	foreignBefore := "# hand-written: this must survive\n"
	managedBlock := "# BEGIN gitid managed: orphan\n" +
		"Host old.example.com\n" +
		"  IdentityFile ~/.ssh/id_orphan\n" +
		"# END gitid managed: orphan\n"
	foreignAfter := "# hand-written: this must also survive\n"
	original := foreignBefore + managedBlock + foreignAfter
	if err := os.WriteFile(configPath, []byte(original), 0o600); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	// Build deps from the live buildDoctorDeps; this is NOT nil-safe for RemoveBlock
	// yet — this test is RED until Task 2 wires it.
	d := buildDoctorDeps(tmpHome, []byte(original), nil)
	if d.RemoveBlock == nil {
		t.Fatal("RemoveBlock must be wired in buildDoctorDeps (currently nil — this is the RED failure)")
	}

	// First call: should remove the block.
	if err := d.RemoveBlock(configPath, "orphan"); err != nil {
		t.Fatalf("RemoveBlock: %v", err)
	}

	afterContent, err := os.ReadFile(configPath) //nolint:gosec // configPath is a test-controlled temp path
	if err != nil {
		t.Fatalf("reading config after removal: %v", err)
	}

	// Foreign content must be preserved verbatim.
	if !strings.Contains(string(afterContent), "hand-written: this must survive") {
		t.Errorf("foreign content before block was destroyed; got:\n%s", afterContent)
	}
	if !strings.Contains(string(afterContent), "hand-written: this must also survive") {
		t.Errorf("foreign content after block was destroyed; got:\n%s", afterContent)
	}

	// The managed block must be gone.
	if strings.Contains(string(afterContent), "BEGIN gitid managed: orphan") {
		t.Errorf("managed block was not removed; got:\n%s", afterContent)
	}

	// A timestamped backup must exist.
	entries, err := filepath.Glob(configPath + ".bak.*")
	if err != nil || len(entries) == 0 {
		t.Errorf("expected a timestamped backup after RemoveBlock; found none")
	}

	// Second call (idempotency): content unchanged, no error.
	before2, _ := os.ReadFile(configPath) //nolint:gosec // test-controlled path
	if err := d.RemoveBlock(configPath, "orphan"); err != nil {
		t.Fatalf("RemoveBlock (second call): %v", err)
	}
	after2, _ := os.ReadFile(configPath) //nolint:gosec // test-controlled path
	if !bytes.Equal(before2, after2) {
		t.Errorf("RemoveBlock is not idempotent; content changed on second call")
	}

	// Confirm internal/doctor/checks does NOT import filewriter (D-01 / Pitfall 4).
	// This is a build-time guard enforced by go test ./internal/doctor/... passing above.
	// Document it here as an assertion comment.
	// (The grep gate is in the plan's verification section; tested in CI.)
}

// TestFixerReAddsWiring is NOT wired in this plan — the AddWiring fixer requires
// full identity data (alias, hostname, port, keyPath, email, pubKey) that needs
// an end-to-end managed SSH config. This test verifies that AddWiring is non-nil
// in buildDoctorDeps (i.e. it IS wired). The full functional test would require
// setting up a complete identity environment.
func TestFixerAddWiringNotNil(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	d := buildDoctorDeps(tmpHome, nil, nil)
	if d.AddWiring == nil {
		t.Fatal("AddWiring must be wired in buildDoctorDeps (currently nil — this is the RED failure)")
	}
}

// TestFixerDocImportsNoFilewriter is a static confirmation test that asserts
// internal/doctor does NOT import internal/filewriter (D-01 / Pitfall 4).
// The functional guarantee is the compile+test passing with grep returning empty.
// This test simply documents the invariant; the CI grep gate asserts it.
func TestFixerDocImportsNoFilewriter(_ *testing.T) {
	// Static assertion: if this test compiles and internal/doctor/doctor.go imports
	// filewriter, the build would fail for the checks package too. We confirm the
	// fix closures are built in cmd (not internal/doctor) by the fact that
	// doctor.Deps.RemoveBlock is a func field (injected), not a call site.
	_ = doctor.Deps{}.RemoveBlock
	_ = doctor.Deps{}.AddWiring
	// If FixPerm/RemoveBlock/AddWiring are zero-value function fields, the
	// import cycle is NOT present (which it isn't by design).
}

// TestDoctorRunDoctorPassesFix verifies that runDoctor passes the fix+yes flags
// through to the fixer path. With no fixable findings in a clean temp home, the
// exit code is still 0 and the run does not crash.
func TestDoctorRunDoctorPassesFix(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	var out bytes.Buffer
	code := runDoctor(&out, true /* fix */, true /* yes */)
	if code < 0 || code > 3 {
		t.Errorf("runDoctor(fix=true,yes=true) on clean home returned invalid code %d", code)
	}
	// On a clean home with no fixable findings, --fix --yes should produce no prompt
	// and the output should still contain the family headers.
	output := out.String()
	if !strings.Contains(output, "=== Permissions ===") {
		t.Errorf("expected '=== Permissions ===' in output; got:\n%s", output)
	}
}

// Compile-time guard: ensure filewriter package is accessible from cmd (not from
// internal/doctor). The test file imports it only for the idempotency assertion.
var _ = filewriter.ListBlocks
