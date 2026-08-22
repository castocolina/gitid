//go:build screenshot

package screenshot_test

// createflow_test.go — tests for CaptureCreateFlowScreens, region extraction,
// and the offline/determinism requirements (plan 03-09 Task 1).
//
// These RED tests drive the implementation in:
//   - internal/screenshot/createflow_regions.go  (ExtractRegion, RegionName constants)
//   - cmd/gitid/gate_visual_regression_test.go   (strict schema validation, used-entry check)

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/castocolina/gitid/internal/dummytui"
	"github.com/castocolina/gitid/internal/screenshot"
)

// TestCaptureCreateFlowScreens_HasAllIDs verifies every enumerated screen ID
// is present in the output map (no silent omissions).
func TestCaptureCreateFlowScreens_HasAllIDs(t *testing.T) {
	backend := dummytui.NewFixtureBackend()
	captures := screenshot.CaptureCreateFlowScreens(backend)
	for _, id := range screenshot.CreateFlowScreenIDs {
		if _, ok := captures[id]; !ok {
			t.Errorf("CaptureCreateFlowScreens: screen %q missing from output", id)
		}
	}
}

// TestCaptureCreateFlowScreens_NonEmptyContent ensures no captured screen is
// an empty string (a capture bug that silently passes a byte-exact diff).
func TestCaptureCreateFlowScreens_NonEmptyContent(t *testing.T) {
	backend := dummytui.NewFixtureBackend()
	captures := screenshot.CaptureCreateFlowScreens(backend)
	for _, id := range screenshot.CreateFlowScreenIDs {
		if strings.TrimSpace(captures[id]) == "" {
			t.Errorf("CaptureCreateFlowScreens: screen %q has empty content", id)
		}
	}
}

// TestCaptureCreateFlowScreens_Deterministic verifies that two consecutive
// captures of the same backend produce byte-identical output per screen.
// D-22/D-24 determinism contract: the gate must not rely on random ordering.
func TestCaptureCreateFlowScreens_Deterministic(t *testing.T) {
	backend := dummytui.NewFixtureBackend()
	first := screenshot.CaptureCreateFlowScreens(backend)
	second := screenshot.CaptureCreateFlowScreens(backend)
	for _, id := range screenshot.CreateFlowScreenIDs {
		if first[id] != second[id] {
			t.Errorf("CaptureCreateFlowScreens: screen %q not deterministic: run1 len=%d run2 len=%d",
				id, len(first[id]), len(second[id]))
		}
	}
}

// TestExtractRegion_Header verifies that the header region (first line,
// containing the nav tabs) is present and contains "Identities".
func TestExtractRegion_Header(t *testing.T) {
	backend := dummytui.NewFixtureBackend()
	captures := screenshot.CaptureCreateFlowScreens(backend)
	screen := captures["ssh-form-filled"]
	header := screenshot.ExtractRegion(screen, screenshot.RegionHeader)
	if strings.TrimSpace(header) == "" {
		t.Error("ExtractRegion(RegionHeader): expected non-empty header line, got empty")
	}
	if !strings.Contains(header, "Identities") {
		t.Errorf("ExtractRegion(RegionHeader): expected nav tab text; got %q", header)
	}
}

// TestExtractRegion_Breadcrumb verifies the breadcrumb region (line 2)
// contains the create-wizard navigation path.
func TestExtractRegion_Breadcrumb(t *testing.T) {
	backend := dummytui.NewFixtureBackend()
	captures := screenshot.CaptureCreateFlowScreens(backend)
	screen := captures["ssh-form-filled"]
	bc := screenshot.ExtractRegion(screen, screenshot.RegionBreadcrumb)
	if !strings.Contains(bc, "New identity") {
		t.Errorf("ExtractRegion(RegionBreadcrumb): expected 'New identity' path; got %q", bc)
	}
}

// TestExtractRegion_Keybar verifies the keybar region (last non-empty lines)
// contains keyboard affordances.
func TestExtractRegion_Keybar(t *testing.T) {
	backend := dummytui.NewFixtureBackend()
	captures := screenshot.CaptureCreateFlowScreens(backend)
	screen := captures["ssh-form-filled"]
	kb := screenshot.ExtractRegion(screen, screenshot.RegionKeybar)
	if !strings.Contains(kb, "Tab") && !strings.Contains(kb, "Enter") {
		t.Errorf("ExtractRegion(RegionKeybar): expected keyboard hint; got %q", kb)
	}
}

// TestExtractRegion_WizardStepper verifies the wizard stepper region contains
// the "Step 1/4" indicator and SSH details label.
func TestExtractRegion_WizardStepper(t *testing.T) {
	backend := dummytui.NewFixtureBackend()
	captures := screenshot.CaptureCreateFlowScreens(backend)
	screen := captures["ssh-form-filled"]
	stepper := screenshot.ExtractRegion(screen, screenshot.RegionWizardStepper)
	if !strings.Contains(stepper, "Step 1/4") {
		t.Errorf("ExtractRegion(RegionWizardStepper): expected 'Step 1/4'; got %q", stepper)
	}
}

// TestExtractRegion_FormFields verifies the form-fields region contains all
// four approved SSH form fields per SSHUI-01/FIELDS.md.
func TestExtractRegion_FormFields(t *testing.T) {
	backend := dummytui.NewFixtureBackend()
	captures := screenshot.CaptureCreateFlowScreens(backend)
	screen := captures["ssh-form-filled"]
	fields := screenshot.ExtractRegion(screen, screenshot.RegionFormFields)
	for _, label := range []string{"Alias prefix", "SSH Host", "Real hostname", "Port"} {
		if !strings.Contains(fields, label) {
			t.Errorf("ExtractRegion(RegionFormFields): expected field label %q; got:\n%s", label, fields)
		}
	}
}

// TestExtractRegion_HostPreview verifies the host-block preview region
// contains the recipe-mandatory directives (SSHUI-03, recipes/ssh-config.recipe).
func TestExtractRegion_HostPreview(t *testing.T) {
	backend := dummytui.NewFixtureBackend()
	captures := screenshot.CaptureCreateFlowScreens(backend)
	screen := captures["ssh-form-filled"]
	preview := screenshot.ExtractRegion(screen, screenshot.RegionHostPreview)
	for _, directive := range []string{"Host", "Hostname", "Port", "User git", "IdentityFile", "IdentitiesOnly yes"} {
		if !strings.Contains(preview, directive) {
			t.Errorf("ExtractRegion(RegionHostPreview): expected %q directive; got:\n%s", directive, preview)
		}
	}
}

func TestExtractRegion_ConnectivityOutputIncludesHardFailure(t *testing.T) {
	screen := strings.Join([]string{
		"header",
		"breadcrumb",
		"│ ✗ The connection failed",
		"│ connect to host ssh.github.com port 443: Operation timed out",
		"│ Retry (Enter)",
		"│ Esc returns",
	}, "\n")

	region := screenshot.ExtractRegion(screen, screenshot.RegionConnectivityOutput)
	for _, marker := range []string{"The connection failed", "connect to host", "Retry (Enter)"} {
		if !strings.Contains(region, marker) {
			t.Errorf("hard-failure connectivity region must contain %q; got:\n%s", marker, region)
		}
	}
}

func TestExtractRegion_PreservesUTF8AfterPaneSeparator(t *testing.T) {
	screen := strings.Join([]string{
		"header",
		"breadcrumb",
		"│ Shift+→",
		"│ Alias prefix [acme]",
		"│ Key Generate",
	}, "\n")

	region := screenshot.ExtractRegion(screen, screenshot.RegionFormFields)
	if !utf8.ValidString(region) {
		t.Fatalf("ExtractRegion must preserve valid UTF-8 after the pane separator: %q", region)
	}
}

func TestExtractRegion_ConfirmationPreviewRequiresCeremony(t *testing.T) {
	withoutCeremony := strings.Join([]string{
		"header",
		"│ # BEGIN gitid managed: acme",
	}, "\n")
	if region := screenshot.ExtractRegion(withoutCeremony, screenshot.RegionConfirmationPreview); region != "" {
		t.Fatalf("confirmation preview must be empty outside the ceremony; got %q", region)
	}

	withCeremony := strings.Join([]string{
		"header",
		"│ Exact change: PgUp/PgDn scroll",
		"│ # BEGIN gitid managed: acme",
		"│ # END gitid managed: acme",
	}, "\n")
	region := screenshot.ExtractRegion(withCeremony, screenshot.RegionConfirmationPreview)
	for _, marker := range []string{"Exact change", "# BEGIN gitid managed:", "# END gitid managed:"} {
		if !strings.Contains(region, marker) {
			t.Errorf("confirmation preview must contain %q; got %q", marker, region)
		}
	}
}

// TestNonDivergentRegionsBetweenTwoDummyCaptures verifies that structural
// regions (header, breadcrumb, wizard stepper, keybar) are byte-identical
// between two separate dummy captures — proving the extraction is stable
// and not affected by capture order.
func TestNonDivergentRegionsBetweenTwoDummyCaptures(t *testing.T) {
	cap1 := screenshot.CaptureCreateFlowScreens(dummytui.NewFixtureBackend())
	cap2 := screenshot.CaptureCreateFlowScreens(dummytui.NewFixtureBackend())

	for _, id := range []string{"ssh-form-filled", "git-form-demo"} {
		for _, region := range []screenshot.RegionName{
			screenshot.RegionHeader,
			screenshot.RegionBreadcrumb,
			screenshot.RegionWizardStepper,
			screenshot.RegionKeybar,
		} {
			r1 := screenshot.ExtractRegion(cap1[id], region)
			r2 := screenshot.ExtractRegion(cap2[id], region)
			if r1 != r2 {
				t.Errorf("screen %q region %q not identical between captures:\nfirst:  %q\nsecond: %q",
					id, region, r1, r2)
			}
		}
	}
}

// TestNegativeControl_UnallowlistedRegionDifferenceFails verifies that a
// difference in a non-allowlisted region (the breadcrumb) causes the
// region-scoped comparison to detect a mismatch.
//
// This is the CR-10 negative-control proof: the strict gate MUST detect
// unrelated drift, not exempt it silently. The check is structural — we
// verify that ExtractRegion returns different text when the test deliberately
// creates two different captures (the same backend, but with the region
// text artificially noted as different).
func TestNegativeControl_UnallowlistedRegionDifferenceFails(t *testing.T) {
	backend := dummytui.NewFixtureBackend()
	captures := screenshot.CaptureCreateFlowScreens(backend)
	screen := captures["ssh-form-filled"]

	// The breadcrumb should contain "New identity" — extract it
	bc := screenshot.ExtractRegion(screen, screenshot.RegionBreadcrumb)
	if !strings.Contains(bc, "New identity") {
		t.Fatalf("TestNegativeControl: breadcrumb does not contain 'New identity'; got %q", bc)
	}

	// Verify that a modified version (simulating an unallowlisted drift)
	// produces a different region extraction — proving the gate would catch it.
	modifiedScreen := strings.ReplaceAll(screen, "New identity", "Changed")
	modifiedBC := screenshot.ExtractRegion(modifiedScreen, screenshot.RegionBreadcrumb)
	if bc == modifiedBC {
		t.Error("TestNegativeControl: ExtractRegion returned identical text after modification — the gate cannot detect unallowlisted drift")
	}
	if strings.Contains(modifiedBC, "New identity") {
		t.Error("TestNegativeControl: modified breadcrumb still contains original text — region extraction is not sensitive to changes")
	}
	t.Logf("TestNegativeControl: confirmed — breadcrumb diff detected: %q vs %q", bc, modifiedBC)
}

// TestOfflineGuard_CaptureDoesNotBlock verifies that CaptureCreateFlowScreens
// completes without blocking when given the offline FixtureBackend.
// D-22: capture must never wait for a real SSH connection. A blocked capture
// (hung goroutine) indicates an unexpected external-binary invocation.
func TestOfflineGuard_CaptureDoesNotBlock(t *testing.T) {
	backend := dummytui.NewFixtureBackend()
	done := make(chan struct{})
	go func() {
		screenshot.CaptureCreateFlowScreens(backend)
		close(done)
	}()
	select {
	case <-done:
		// returned without blocking — structural offline proof
	case <-time.After(10 * time.Second):
		t.Fatal("CaptureCreateFlowScreens blocked (>10s): indicates an unexpected external-binary/network wait; capture must be offline (D-22)")
	}
}

// ---------------------------------------------------------------------------
// CR-01 — Determinism: two runs must produce byte-identical text/PNG hashes
// ---------------------------------------------------------------------------

// TestCaptureCreateFlowScreens_DeterministicTwoRuns verifies that two
// consecutive CaptureCreateFlowScreens calls with fixed inputs (deterministic
// backend, normalized paths, fixed spec) produce byte-identical per-screen
// text content (CR-01).
//
// Current failure: the confirm-write capture embeds a wall-clock backup path
// through fixture state, making runs nondeterministic across second boundaries.
func TestCaptureCreateFlowScreens_DeterministicTwoRuns(t *testing.T) {
	backend := dummytui.NewFixtureBackend()
	first := screenshot.CaptureCreateFlowScreens(backend)
	second := screenshot.CaptureCreateFlowScreens(backend)
	for _, id := range screenshot.CreateFlowScreenIDs {
		if first[id] != second[id] {
			t.Errorf("CaptureCreateFlowScreens: screen %q not byte-identical across two runs (CR-01 nondeterminism):\n  run1 len=%d run2 len=%d",
				id, len(first[id]), len(second[id]))
		}
	}
}

// ---------------------------------------------------------------------------
// CR-02 — Stage captures must show distinct content from valid running states
// ---------------------------------------------------------------------------

// TestCaptureCreateFlowScreens_StagesDiffer proves that the stage-1 and
// stage-2 captures are byte-DIFFERENT (CR-02). If both show the idle screen,
// the stage injection was silently dropped (wrong testPhase state).
func TestCaptureCreateFlowScreens_StagesDiffer(t *testing.T) {
	backend := dummytui.NewFixtureBackend()
	captures := screenshot.CaptureCreateFlowScreens(backend)
	s1 := captures["test-stage1-direct"]
	s2 := captures["test-stage2-by-alias"]
	if s1 == "" || s2 == "" {
		t.Fatal("test-stage1-direct or test-stage2-by-alias screen is empty")
	}
	if s1 == s2 {
		t.Error("test-stage1-direct and test-stage2-by-alias captures are IDENTICAL — both show the same (likely idle) state; stage injection is not reaching valid running phases (CR-02)")
	}
}

// TestCaptureCreateFlowScreens_Stage1ContainsStageOutput proves stage-1 capture
// contains stage-specific content (connectivity command or result), not just the
// idle "Run stage 1 (Enter)" prompt (CR-02 content assertion).
func TestCaptureCreateFlowScreens_Stage1ContainsStageOutput(t *testing.T) {
	backend := dummytui.NewFixtureBackend()
	captures := screenshot.CaptureCreateFlowScreens(backend)
	s1 := captures["test-stage1-direct"]
	stripped := screenshot.StripANSIExported(s1)
	// The stage-1 screen must show a stage outcome, not just the idle prompt.
	// "Run stage 1 (Enter)" means the model is still in testIdle state.
	if strings.Contains(stripped, "Run stage 1") && !strings.Contains(stripped, "ssh ") &&
		!strings.Contains(stripped, "Permission denied") && !strings.Contains(stripped, "authenticated") &&
		!strings.Contains(stripped, "Reachable") {
		t.Errorf("test-stage1-direct still shows idle 'Run stage 1' with no stage outcome — stage injection failed (CR-02):\n%s", stripped)
	}
}

// TestCaptureCreateFlowScreens_Stage2ContainsResolutionProof proves stage-2
// capture contains the alias resolution proof (identityfile line), which
// distinguishes it from stage-1 (CR-02 content assertion).
func TestCaptureCreateFlowScreens_Stage2ContainsResolutionProof(t *testing.T) {
	backend := dummytui.NewFixtureBackend()
	captures := screenshot.CaptureCreateFlowScreens(backend)
	s2 := captures["test-stage2-by-alias"]
	stripped := screenshot.StripANSIExported(s2)
	if !strings.Contains(stripped, "identityfile") && !strings.Contains(stripped, "IdentityFile") {
		t.Errorf("test-stage2-by-alias does not contain resolution proof (identityfile) — stage-2 injection may have failed (CR-02):\n%s", stripped)
	}
}
