package main

// fix_test.go covers `gitid fix` (08-02-PLAN.md Task 3): --dry-run writes
// nothing, --yes applies the flagship fixture end-to-end, an Interactive-
// only fix is applied correctly (no nil-Fn panic, no silent skip), and the
// maxPasses backstop halts a fixture engineered to never converge.

import (
	"bytes"
	"crypto/sha256"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/tuikit"
)

func TestBaselineGitignoreFixViaCLI(t *testing.T) {
	home := t.TempDir()
	seedInstalledBaseline(t, home)
	if err := runFix(io.Discard, strings.NewReader(""), home, true, false); err != nil {
		t.Fatalf("runFix(--yes): %v", err)
	}
	// core.excludesfile is patched into the "baseline" managed block INSIDE
	// baselineFilePath, never gitconfigPath directly — `git config --file`
	// does not follow [include] directives when reading, and this is
	// exactly where CheckBaseline's gitignore-pair check reads it from
	// (state.BaselineKeys, parsed from the fragment's own block body).
	baselineFilePath := filepath.Join(home, ".gitconfig.d", "00-baseline")
	gitignorePath := filepath.Join(home, ".gitignore_global")
	value, err := gitconfig.RunGitConfigGet(baselineFilePath, "core.excludesfile")
	if err != nil {
		t.Fatalf("reading core.excludesfile: %v", err)
	}
	if value != gitignorePath {
		t.Errorf("core.excludesfile = %q, want %q", value, gitignorePath)
	}
	content := readFile(t, gitignorePath)
	if !strings.Contains(content, "# BEGIN gitid managed: gitignore") {
		t.Errorf("missing managed gitignore block:\n%s", content)
	}
}

// TestBaselineGitignoreFixPreservesOtherBaselineSettings is the regression
// test for a real bug found empirically (not assumed): the gitignore-pair
// fix originally wrote core.excludesfile via `git config --file gitconfigPath
// ...`, landing it OUTSIDE any managed block in the WRONG file — `git config
// --file` does not follow [include] directives when reading, and
// CheckBaseline reads core.excludesfile from state.BaselineKeys (parsed from
// the baseline FRAGMENT's own block body), so the fix could never converge.
// A second bug surfaced fixing the first: patching the fragment naively via
// a bare git-config-set would land the key OUTSIDE the "baseline" managed
// block's sentinels too. This test proves the corrected fix patches the
// EXISTING block body in place — preserving the user's own Tier-2 baseline
// choices (init.defaultBranch, a custom [alias] section) byte-for-byte —
// and that the check converges to zero findings afterward.
func TestBaselineGitignoreFixPreservesOtherBaselineSettings(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatalf("seeding .ssh: %v", err)
	}
	gitconfigPath := filepath.Join(home, ".gitconfig")
	baselineFilePath := filepath.Join(home, ".gitconfig.d", "00-baseline")
	if err := os.MkdirAll(filepath.Dir(baselineFilePath), 0o700); err != nil {
		t.Fatalf("seeding .gitconfig.d: %v", err)
	}
	writeFile(t, baselineFilePath, managedBlock("baseline",
		"[core]\n\tignorecase = false\n[init]\n\tdefaultBranch = my-custom-branch\n[alias]\n\tco = checkout\n"))
	if _, err := gitconfig.WriteBaselineInclude(gitconfigPath, baselineFilePath); err != nil {
		t.Fatalf("seeding baseline-include: %v", err)
	}

	if err := runFix(io.Discard, strings.NewReader(""), home, true, false); err != nil {
		t.Fatalf("runFix(--yes): %v", err)
	}

	after := readFile(t, baselineFilePath)
	if !strings.Contains(after, "defaultBranch = my-custom-branch") {
		t.Errorf("the user's init.defaultBranch override was lost:\n%s", after)
	}
	if !strings.Contains(after, "[alias]\n\tco = checkout") {
		t.Errorf("the user's custom [alias] section was lost:\n%s", after)
	}
	if !strings.Contains(after, "excludesfile = "+filepath.Join(home, ".gitignore_global")) {
		t.Errorf("core.excludesfile was not patched into the baseline block:\n%s", after)
	}

	for _, f := range doctorFindings(home) {
		if f.Family == "Baseline" {
			t.Errorf("expected the fix to converge to zero Baseline findings, got: %+v", f)
		}
	}
}

func TestFixCmdDryRunWritesNothing(t *testing.T) {
	home := t.TempDir()
	configPath := seedFlagshipFixture(t, home)
	before := sha256.Sum256([]byte(readFile(t, configPath)))

	var out bytes.Buffer
	if err := runFix(&out, strings.NewReader(""), home, false, true); err != nil {
		t.Fatalf("runFix(dry-run): %v", err)
	}

	after := sha256.Sum256([]byte(readFile(t, configPath)))
	if before != after {
		t.Fatal("--dry-run must never write to disk")
	}
	if !strings.Contains(out.String(), "IdentitiesOnly no contradicts") {
		t.Errorf("dry-run output missing the fixable finding's title:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "- \tIdentitiesOnly no") {
		t.Errorf("dry-run output missing the real diff:\n%s", out.String())
	}
}

func TestFixCmdDryRunNoFixableFindings(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatalf("seeding .ssh: %v", err)
	}
	// An installed baseline removes the one fixable finding an otherwise
	// empty home always carries (CheckBaseline's Fn-only fallback is a
	// documented, genuinely non-convergent no-op — irrelevant to what this
	// test asserts).
	seedInstalledBaseline(t, home)
	baselineFilePath := filepath.Join(home, ".gitconfig.d", "00-baseline")
	if err := fixExcludesfile(baselineFilePath)(filepath.Join(home, ".gitignore_global")); err != nil {
		t.Fatalf("seeding gitignore pair: %v", err)
	}

	var out bytes.Buffer
	if err := runFix(&out, strings.NewReader(""), home, false, true); err != nil {
		t.Fatalf("runFix(dry-run): %v", err)
	}
	if !strings.Contains(out.String(), "no fixable findings") {
		t.Errorf("output = %q, want the no-fixable-findings message", out.String())
	}
}

// seedInstalledBaseline writes a fully-installed baseline (both the
// "baseline-include" block in ~/.gitconfig and the "baseline" block in
// ~/.gitconfig.d/00-baseline) so CheckBaseline's Installed check passes —
// isolating a test's fixable-findings scope from baseline's separate,
// already-known-incomplete Fn-only fallback (its Fix.Fn restores only the
// include pointer, never the fragment; ReadBaselineState requires BOTH).
func seedInstalledBaseline(t *testing.T, home string) {
	t.Helper()
	gitconfigPath := filepath.Join(home, ".gitconfig")
	baselineFilePath := filepath.Join(home, ".gitconfig.d", "00-baseline")
	if err := os.MkdirAll(filepath.Dir(baselineFilePath), 0o700); err != nil {
		t.Fatalf("seeding .gitconfig.d: %v", err)
	}
	writeFile(t, baselineFilePath, managedBlock("baseline", "[core]\n\tignorecase = false\n"))
	if _, err := gitconfig.WriteBaselineInclude(gitconfigPath, baselineFilePath); err != nil {
		t.Fatalf("seeding baseline-include: %v", err)
	}
}

func TestFixCmdYesAppliesFlagshipAndExitsZero(t *testing.T) {
	home := t.TempDir()
	configPath := seedFlagshipFixture(t, home)
	// A home with no baseline installed and loose permissions carries its
	// OWN independently-fixable findings (correct permissions converge
	// cleanly; baseline's Fn-only fallback is a documented, genuinely
	// non-convergent no-op until SetupBaseline is wired — a future addition,
	// not this test's concern). Neutralize both so the loop converges after
	// exactly the ONE flagship fix this test is about.
	if err := os.Chmod(filepath.Join(home, ".ssh"), 0o700); err != nil { //nolint:gosec // 0700 is the CORRECT restrictive mode for a directory; G302's 0600 ceiling is file-oriented (G302)
		t.Fatalf("chmod .ssh: %v", err)
	}
	if err := os.Chmod(configPath, 0o600); err != nil {
		t.Fatalf("chmod config: %v", err)
	}
	seedInstalledBaseline(t, home)

	var out bytes.Buffer
	err := runFix(&out, strings.NewReader(""), home, true, false)
	if err != nil {
		t.Fatalf("runFix(--yes): %v\noutput:\n%s", err, out.String())
	}

	after := readFile(t, configPath)
	if !strings.Contains(after, "IdentitiesOnly yes # deliberately loose") {
		t.Fatalf("--yes did not apply the flagship rewrite:\n%s", after)
	}
	if !strings.Contains(after, "Host untouched.example.com\n\tHostName example.com") {
		t.Fatalf("an unrelated Host block was altered:\n%s", after)
	}
	if !strings.Contains(out.String(), "fixed") {
		t.Errorf("output missing a 'fixed' confirmation:\n%s", out.String())
	}
}

// withFixScanOverride sets fixScanOverride for the duration of the test and
// restores it afterward — the shared setup every synthetic-scan test below
// uses.
func withFixScanOverride(t *testing.T, fn func(home string) ([]doctor.Finding, []tuikit.DemoFinding)) {
	t.Helper()
	old := fixScanOverride
	t.Cleanup(func() { fixScanOverride = old })
	fixScanOverride = fn
}

// syntheticFinding builds a minimal converged (raw, converted) pair for a
// single finding — the shape scanForFix returns, index-aligned by
// construction like runDoctorAndConvert's real output.
func syntheticFinding(f doctor.Finding) ([]doctor.Finding, []tuikit.DemoFinding) {
	return []doctor.Finding{f}, []tuikit.DemoFinding{{
		HealthFinding: tuikit.HealthFinding{
			ID:           string(f.Family) + "|" + f.Title,
			Section:      "Git",
			Family:       string(f.Family),
			Title:        f.Title,
			SuggestedFix: f.Fix.Summary,
			Severity:     tuikit.SeverityError,
		},
	}}
}

// TestFixCmdInteractiveFixAppliedNoNilFnPanic proves a finding whose Fix
// sets ONLY Interactive (Fn nil) is applied correctly by `gitid fix --yes`
// through the REAL runFix apply-gate switch: no nil-Fn panic, the finding
// is not silently skipped, and assumeYes is threaded from --yes. Wired via
// fixScanOverride since no real check currently produces an
// Interactive-without-Fn finding (SetupBaseline is nil in the real wiring,
// 08-01-SUMMARY.md).
func TestFixExitCode(t *testing.T) {
	findings := []doctor.Finding{{Family: doctor.FamilyCoherence, Severity: doctor.SeverityError, Title: "report-only"}}
	if got := exitStatusOf(healthFinish(doctor.ExitCode(findings))); got != 2 {
		t.Errorf("fix exit code = %d, want 2", got)
	}
}

func TestFixCmdInteractiveFixAppliedNoNilFnPanic(t *testing.T) {
	var interactiveCalled bool
	var gotAssumeYes bool
	served := false
	withFixScanOverride(t, func(string) ([]doctor.Finding, []tuikit.DemoFinding) {
		if served {
			// Converge after one apply — an Interactive fix that "worked"
			// must not be offered again this pass.
			return nil, nil
		}
		served = true
		return syntheticFinding(doctor.Finding{
			Family:   doctor.FamilyBaseline,
			Severity: doctor.SeverityError,
			Title:    "interactive-only fixable finding",
			Fix: &doctor.FixDescriptor{
				Summary: "run the interactive fix",
				Interactive: func(_ io.Reader, _ io.Writer, assumeYes bool) error {
					interactiveCalled = true
					gotAssumeYes = assumeYes
					return nil
				},
			},
		})
	})

	home := t.TempDir()
	var out bytes.Buffer
	if err := runFix(&out, strings.NewReader(""), home, true, false); err != nil {
		t.Fatalf("runFix(--yes) with an Interactive-only fix: %v", err)
	}
	if !interactiveCalled {
		t.Fatal("Fix.Interactive was never invoked — Fn nil must not silently skip the finding")
	}
	if !gotAssumeYes {
		t.Error("assumeYes must be true when --yes was passed")
	}
}

// TestFixCmdMaxPassesHaltsOnNonConvergence proves the batch loop halts via
// maxPasses on a fixture engineered to never converge — a Fn that reports
// success but changes nothing, so the SAME finding reappears every pass —
// rather than looping forever.
func TestFixCmdMaxPassesHaltsOnNonConvergence(t *testing.T) {
	var applyCount int
	withFixScanOverride(t, func(string) ([]doctor.Finding, []tuikit.DemoFinding) {
		return syntheticFinding(doctor.Finding{
			Family:   doctor.FamilyCoherence,
			Severity: doctor.SeverityError,
			Title:    "never converges",
			Fix: &doctor.FixDescriptor{
				Summary: "no-op",
				Fn: func() error {
					applyCount++
					return nil // reports success but changes nothing
				},
			},
		})
	})

	home := t.TempDir()
	var out bytes.Buffer
	err := runFix(&out, strings.NewReader(""), home, true, false)
	if err == nil {
		t.Fatal("want an error when the batch never converges")
	}
	if !strings.Contains(err.Error(), "did not converge") {
		t.Errorf("error = %v, want it to name non-convergence", err)
	}
	if applyCount != fixMaxPasses {
		t.Errorf("apply count = %d, want exactly %d (the maxPasses backstop)", applyCount, fixMaxPasses)
	}
}
