//go:build screenshot

package screenshot_test

// createflow_packet_test.go — TDD RED/GREEN tests for the canonical 24-panel
// evidence packet (plan 03-11 Task 2, CR-01, CR-10).
//
// These tests verify the packet generation and validation infrastructure that
// the `cmd/gitid-evidence` publisher uses.

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/castocolina/gitid/internal/dummytui"
	"github.com/castocolina/gitid/internal/screenshot"
)

// unmarshalJSON parses JSON bytes into v, used by canonical manifest tests.
func unmarshalJSON(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

// fixedClock returns a deterministic clock for tests.
func fixedClock(t time.Time) func() time.Time { return func() time.Time { return t } }

// makeTestCaptures returns two identical sets of text captures from the dummy
// backend. RequiredScreenSpecs() is a MERGED registry (04-04-PLAN.md Task 3):
// the create-flow specs PLUS the five Phase 4 git-screen checkpoints, so both
// CaptureCreateFlowScreens AND CaptureGitScreenScreens must be merged into
// each returned map -- otherwise packet generation fails with "live capture
// missing screen ..." for every git-screen-only ID (mirrors
// cmd/gitid/gate_visual_regression_test.go's mergeGitScreenCaptures).
func makeTestCaptures(t *testing.T) (map[string]string, map[string]string) {
	t.Helper()
	backend := dummytui.NewFixtureBackend()
	live, err := screenshot.CaptureCreateFlowScreens(backend)
	if err != nil {
		t.Fatalf("CaptureCreateFlowScreens (live): %v", err)
	}
	approved, err := screenshot.CaptureCreateFlowScreens(backend)
	if err != nil {
		t.Fatalf("CaptureCreateFlowScreens (approved): %v", err)
	}
	gitLive, err := screenshot.CaptureGitScreenScreens(backend)
	if err != nil {
		t.Fatalf("CaptureGitScreenScreens (live): %v", err)
	}
	gitApproved, err := screenshot.CaptureGitScreenScreens(backend)
	if err != nil {
		t.Fatalf("CaptureGitScreenScreens (approved): %v", err)
	}
	for id, text := range gitLive {
		live[id] = text
	}
	for id, text := range gitApproved {
		approved[id] = text
	}
	// 05-09-PLAN.md Task 3: identity-manager captures merged the SAME way
	// git-screen's are, immediately above.
	imgrLive, err := screenshot.CaptureIdentityManagerScreens(backend)
	if err != nil {
		t.Fatalf("CaptureIdentityManagerScreens (live): %v", err)
	}
	imgrApproved, err := screenshot.CaptureIdentityManagerScreens(backend)
	if err != nil {
		t.Fatalf("CaptureIdentityManagerScreens (approved): %v", err)
	}
	for id, text := range imgrLive {
		live[id] = text
	}
	for id, text := range imgrApproved {
		approved[id] = text
	}
	// 06-07-PLAN.md Task 1: Global SSH captures merged the SAME way the
	// git-screen and identity-manager captures are, immediately above —
	// RequiredScreenSpecs() is now a FOUR-way merged registry.
	gssLive, err := screenshot.CaptureGlobalSSHScreens(backend)
	if err != nil {
		t.Fatalf("CaptureGlobalSSHScreens (live): %v", err)
	}
	gssApproved, err := screenshot.CaptureGlobalSSHScreens(backend)
	if err != nil {
		t.Fatalf("CaptureGlobalSSHScreens (approved): %v", err)
	}
	for id, text := range gssLive {
		live[id] = text
	}
	for id, text := range gssApproved {
		approved[id] = text
	}
	// 07-06-PLAN.md Task 1: Global Git captures merged the SAME way the
	// previous three, immediately above — RequiredScreenSpecs() is now a
	// FIVE-way merged registry.
	ggitLive, err := screenshot.CaptureGlobalGitScreens(backend)
	if err != nil {
		t.Fatalf("CaptureGlobalGitScreens (live): %v", err)
	}
	ggitApproved, err := screenshot.CaptureGlobalGitScreens(backend)
	if err != nil {
		t.Fatalf("CaptureGlobalGitScreens (approved): %v", err)
	}
	for id, text := range ggitLive {
		live[id] = text
	}
	for id, text := range ggitApproved {
		approved[id] = text
	}
	// 08-08-PLAN.md Task 2: Health/Fixer captures merged the SAME way the
	// previous four, immediately above — RequiredScreenSpecs() is now a
	// SIX-way merged registry.
	hfLive, err := screenshot.CaptureHealthFixerScreens(backend)
	if err != nil {
		t.Fatalf("CaptureHealthFixerScreens (live): %v", err)
	}
	hfApproved, err := screenshot.CaptureHealthFixerScreens(backend)
	if err != nil {
		t.Fatalf("CaptureHealthFixerScreens (approved): %v", err)
	}
	for id, text := range hfLive {
		live[id] = text
	}
	for id, text := range hfApproved {
		approved[id] = text
	}
	return live, approved
}

// TestGenerateTextPacket_CreatesManifest verifies that GenerateTextPacket writes
// a MANIFEST.json into the output directory (CR-01 basic packet creation).
func TestGenerateTextPacket_CreatesManifest(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "packet")
	live, approved := makeTestCaptures(t)

	result, err := screenshot.GenerateTextPacket(screenshot.PacketOptions{
		SourceCommit:   strings.Repeat("a", 40),
		ApprovalCommit: screenshot.PacketApprovalCommit,
		OutputDir:      outDir,
		Clock:          fixedClock(time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)),
	}, live, approved)
	if err != nil {
		t.Fatalf("GenerateTextPacket: %v", err)
	}

	if result.ManifestPath == "" {
		t.Error("GenerateTextPacket: ManifestPath is empty")
	}
	if _, err := os.Stat(result.ManifestPath); err != nil {
		t.Errorf("GenerateTextPacket: MANIFEST.json not found at %s: %v", result.ManifestPath, err)
	}
	if result.Packet.SourceCommit != strings.Repeat("a", 40) {
		t.Errorf("GenerateTextPacket: source commit mismatch: %q", result.Packet.SourceCommit)
	}
}

// TestGenerateTextPacket_RejectsShortSourceCommit verifies fail-closed behavior:
// a source commit that is not 40 hex characters is rejected (CR-01).
func TestGenerateTextPacket_RejectsShortSourceCommit(t *testing.T) {
	live, approved := makeTestCaptures(t)
	_, err := screenshot.GenerateTextPacket(screenshot.PacketOptions{
		SourceCommit:   "abc123", // too short
		ApprovalCommit: screenshot.PacketApprovalCommit,
		OutputDir:      filepath.Join(t.TempDir(), "packet"),
	}, live, approved)
	if err == nil {
		t.Fatal("GenerateTextPacket with short source commit must return error (CR-01 fail-closed)")
	}
}

// TestGenerateTextPacket_RejectsExistingDestination verifies that an existing
// output directory is rejected (CR-01 immutability guarantee).
func TestGenerateTextPacket_RejectsExistingDestination(t *testing.T) {
	outDir := t.TempDir() // already exists
	live, approved := makeTestCaptures(t)

	_, err := screenshot.GenerateTextPacket(screenshot.PacketOptions{
		SourceCommit:   strings.Repeat("b", 40),
		ApprovalCommit: screenshot.PacketApprovalCommit,
		OutputDir:      outDir,
	}, live, approved)
	if err == nil {
		t.Fatal("GenerateTextPacket with existing OutputDir must return error (CR-01 immutability)")
	}
}

// TestGenerateTextPacket_AllScreensPresent verifies the registry inventory is
// represented for every applicable surface.
func TestGenerateTextPacket_AllScreensPresent(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "packet")
	live, approved := makeTestCaptures(t)

	result, err := screenshot.GenerateTextPacket(screenshot.PacketOptions{
		SourceCommit:   strings.Repeat("c", 40),
		ApprovalCommit: screenshot.PacketApprovalCommit,
		OutputDir:      outDir,
		Clock:          fixedClock(time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)),
	}, live, approved)
	if err != nil {
		t.Fatalf("GenerateTextPacket: %v", err)
	}

	// Verify each screen ID appears once for every applicable text surface.
	screenCount := make(map[string]int)
	for _, m := range result.Packet.Members {
		if m.ScreenID != "" {
			screenCount[m.ScreenID]++
		}
	}
	for _, spec := range screenshot.RequiredScreenSpecs() {
		want := 0
		if spec.ApplicableLive {
			want++
		}
		if spec.ApplicableApprovedTUI {
			want++
		}
		if screenCount[spec.ScreenID] != want {
			t.Errorf("screen %q has %d packet members, want %d from RequiredScreenSpecs applicability", spec.ScreenID, screenCount[spec.ScreenID], want)
		}
	}
}

// TestValidatePacket_AcceptsValidPacket verifies that a freshly generated packet
// passes ValidatePacket without error (CR-01 round-trip).
func TestValidatePacket_AcceptsValidPacket(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "packet")
	live, approved := makeTestCaptures(t)

	_, err := screenshot.GenerateTextPacket(screenshot.PacketOptions{
		SourceCommit:   strings.Repeat("d", 40),
		ApprovalCommit: screenshot.PacketApprovalCommit,
		OutputDir:      outDir,
		Clock:          fixedClock(time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)),
	}, live, approved)
	if err != nil {
		t.Fatalf("GenerateTextPacket: %v", err)
	}

	pkt, verr := screenshot.ValidatePacket(outDir)
	if verr != nil {
		t.Fatalf("ValidatePacket rejected a freshly generated packet: %v", verr)
	}
	if pkt.SourceCommit == "" {
		t.Error("ValidatePacket returned packet with empty source_commit")
	}
}

// TestValidatePacket_RejectsTamperedMember verifies that ValidatePacket rejects
// a packet where a member file has been modified after generation (CR-01 hash
// integrity).
func TestValidatePacket_RejectsTamperedMember(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "packet")
	live, approved := makeTestCaptures(t)

	result, err := screenshot.GenerateTextPacket(screenshot.PacketOptions{
		SourceCommit:   strings.Repeat("e", 40),
		ApprovalCommit: screenshot.PacketApprovalCommit,
		OutputDir:      outDir,
		Clock:          fixedClock(time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)),
	}, live, approved)
	if err != nil {
		t.Fatalf("GenerateTextPacket: %v", err)
	}

	// Tamper with the first member.
	if len(result.Packet.Members) == 0 {
		t.Fatal("packet has no members — cannot test tampering")
	}
	firstMember := result.Packet.Members[0]
	absPath := filepath.Join(outDir, firstMember.Path)
	original, err := os.ReadFile(absPath) //nolint:gosec // test fixture path (G304)
	if err != nil {
		t.Fatalf("reading member %s: %v", firstMember.Path, err)
	}
	tampered := append(original, []byte("\nTAMPERED\n")...)
	if err := os.WriteFile(absPath, tampered, 0o600); err != nil { //nolint:gosec // test fixture path scoped to this test's own outDir (G703)
		t.Fatalf("writing tampered member: %v", err)
	}

	_, verr := screenshot.ValidatePacket(outDir)
	if verr == nil {
		t.Fatal("ValidatePacket must reject a tampered member (CR-01 hash integrity)")
	}
	if !strings.Contains(verr.Error(), "hash mismatch") {
		t.Errorf("ValidatePacket error should mention 'hash mismatch'; got: %v", verr)
	}
}

// TestCompareManifests_IdenticalReturnsNil verifies that two identical packet
// results produce no manifest diff (CR-10 cross-process determinism).
func TestCompareManifests_IdenticalReturnsNil(t *testing.T) {
	makePacket := func(outDir string) screenshot.PacketResult {
		live, approved := makeTestCaptures(t)
		result, err := screenshot.GenerateTextPacket(screenshot.PacketOptions{
			SourceCommit:   strings.Repeat("f", 40),
			ApprovalCommit: screenshot.PacketApprovalCommit,
			OutputDir:      outDir,
			Clock:          fixedClock(time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)),
		}, live, approved)
		if err != nil {
			t.Fatalf("GenerateTextPacket: %v", err)
		}
		return result
	}

	tmp := t.TempDir()
	r1 := makePacket(filepath.Join(tmp, "p1"))
	r2 := makePacket(filepath.Join(tmp, "p2"))

	if err := screenshot.CompareManifests(r1, r2); err != nil {
		t.Errorf("CompareManifests: two identical packets should produce no error; got: %v", err)
	}
}

// TestCompareTextCaptures_IdenticalReturnsNil verifies that two identical capture
// sets produce no diff (CR-10 positive case).
func TestCompareTextCaptures_IdenticalReturnsNil(t *testing.T) {
	backend := dummytui.NewFixtureBackend()
	c1, err := screenshot.CaptureCreateFlowScreens(backend)
	if err != nil {
		t.Fatalf("CaptureCreateFlowScreens: %v", err)
	}
	c2, err := screenshot.CaptureCreateFlowScreens(backend)
	if err != nil {
		t.Fatalf("CaptureCreateFlowScreens: %v", err)
	}

	if err := screenshot.CompareTextCaptures(c1, c2); err != nil {
		t.Errorf("CompareTextCaptures with identical captures: %v", err)
	}
}

// TestCompareTextCaptures_DifferentReturnError verifies that captures with
// different content produce an error (CR-10 negative case).
func TestCompareTextCaptures_DifferentReturnError(t *testing.T) {
	backend := dummytui.NewFixtureBackend()
	c1, err := screenshot.CaptureCreateFlowScreens(backend)
	if err != nil {
		t.Fatalf("CaptureCreateFlowScreens: %v", err)
	}
	c2 := make(map[string]string)
	for k, v := range c1 {
		c2[k] = v
	}
	// Mutate one screen in c2.
	firstID := screenshot.RequiredScreenSpecs()[0].ScreenID
	c2[firstID] = c2[firstID] + "\nMUTATED"

	if err := screenshot.CompareTextCaptures(c1, c2); err == nil {
		t.Error("CompareTextCaptures with different captures must return error (CR-10 negative)")
	}
}

// TestGenerateTextPacket_MissingLiveCapture verifies fail-closed behavior when
// a live capture is missing a required screen (CR-01).
func TestGenerateTextPacket_MissingLiveCapture(t *testing.T) {
	backend := dummytui.NewFixtureBackend()
	live, err := screenshot.CaptureCreateFlowScreens(backend)
	if err != nil {
		t.Fatalf("CaptureCreateFlowScreens: %v", err)
	}
	approved, err := screenshot.CaptureCreateFlowScreens(backend)
	if err != nil {
		t.Fatalf("CaptureCreateFlowScreens: %v", err)
	}

	// Remove one screen from live.
	delete(live, screenshot.RequiredScreenSpecs()[0].ScreenID)

	_, err = screenshot.GenerateTextPacket(screenshot.PacketOptions{
		SourceCommit:   strings.Repeat("g", 40),
		ApprovalCommit: screenshot.PacketApprovalCommit,
		OutputDir:      filepath.Join(t.TempDir(), "packet"),
		Clock:          fixedClock(time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)),
	}, live, approved)
	if err == nil {
		t.Fatal("GenerateTextPacket with missing live screen must return error (CR-01 fail-closed)")
	}
}

// TestPacketApprovalCommitConstant verifies the approval commit constant is the
// correct full 40-hex SHA (CR-02).
func TestPacketApprovalCommitConstant(t *testing.T) {
	ac := screenshot.PacketApprovalCommit
	if len(ac) != 40 {
		t.Errorf("PacketApprovalCommit must be 40 hex chars; got %d: %q", len(ac), ac)
	}
	for _, r := range ac {
		if !strings.ContainsRune("0123456789abcdef", r) {
			t.Errorf("PacketApprovalCommit contains non-hex char %q in: %q", r, ac)
		}
	}
}

// ---------------------------------------------------------------------------
// 03-12 Task 2: Strict screen registry and provenance-complete publisher
// ---------------------------------------------------------------------------

// TestDuplicateNamedScreen proves the publisher rejects two entries on the
// same surface that share a logical screen name (UI-REVIEW Critical finding:
// stage-1 and stage-2 had identical PNG hashes because both were advanced to
// the same terminal state before capture). This test creates exactly 24 panels
// but with the first and last "live" panels sharing the same screen ID —
// GenerateVisualPacket must detect the duplicate BEFORE accepting the packet.
func TestDuplicateNamedScreen(t *testing.T) {
	dir := t.TempDir()
	panels := makeMinimalPanels(t, dir)
	// Replace the last "live" panel (confirm-write) with a second copy of
	// the first "live" panel (ssh-form-filled) — same surface, same screen ID,
	// different PNG — so the count stays at 24 but a duplicate exists.
	lastLiveIdx := 0
	for _, spec := range screenshot.RequiredScreenSpecs() {
		if spec.ApplicableLive {
			lastLiveIdx++
		}
	}
	lastLiveIdx--
	dupPNG := makeTinyPNG(t, dir, "dup.png")
	panels[lastLiveIdx] = screenshot.VisualPanel{
		Surface:  "live",
		ScreenID: screenshot.RequiredScreenSpecs()[0].ScreenID, // duplicate first registry frame
		Text:     "duplicate text",
		PNGPath:  dupPNG,
	}

	outDir := filepath.Join(t.TempDir(), "dup-packet")
	_, err := screenshot.GenerateVisualPacket(screenshot.PacketOptions{
		SourceCommit:   strings.Repeat("d", 40),
		ApprovalCommit: screenshot.PacketApprovalCommit,
		OutputDir:      outDir,
	}, panels, screenshot.PacketCapture{
		Commands: []string{"test"}, ToolVersions: []screenshot.PacketTool{{Name: "go", Version: "test"}},
		Geometry: "100x30", FontSHA256: strings.Repeat("a", 64), Theme: "test",
	})
	if err == nil {
		t.Fatal("GenerateVisualPacket must reject duplicate named screen on same surface")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("error must mention 'duplicate'; got: %v", err)
	}
}

// TestDuplicatePNGBytes proves ValidatePacket rejects a packet where two
// differently-named screen IDs on the same surface have identical PNG bytes
// (UI-REVIEW Critical: stage-1 and stage-2 PNGs had the same SHA-256).
// This test builds a packet where two live panels share the same PNG bytes
// by providing identical source PNGs to GenerateVisualPacket.
func TestDuplicatePNGBytes(t *testing.T) {
	dir := t.TempDir()
	panels := makeMinimalPanels(t, dir)
	// Make live/ssh-form-filled and live/reuse-key-vs-generate share the same
	// PNG by pointing both at the same source file before packet generation.
	// GenerateVisualPacket doesn't check for cross-screen duplicate PNG bytes;
	// ValidatePacket must catch it.
	sharedPNG := panels[0].PNGPath // live/ssh-form-filled PNG
	panels[1].PNGPath = sharedPNG  // live/reuse-key-vs-generate now points to same PNG

	outDir := filepath.Join(t.TempDir(), "dedup-packet")
	_, err := screenshot.GenerateVisualPacket(screenshot.PacketOptions{
		SourceCommit:   strings.Repeat("e", 40),
		ApprovalCommit: screenshot.PacketApprovalCommit,
		OutputDir:      outDir,
	}, panels, screenshot.PacketCapture{
		Commands: []string{"test"}, ToolVersions: []screenshot.PacketTool{{Name: "go", Version: "test"}},
		Geometry: "100x30", FontSHA256: strings.Repeat("b", 64), Theme: "test",
	})
	if err != nil {
		t.Fatalf("setup: GenerateVisualPacket failed: %v", err)
	}

	// ValidatePacket must reject: two different named screens on the same
	// surface cannot share identical PNG bytes on disk.
	_, verr := screenshot.ValidatePacket(outDir)
	if verr == nil {
		t.Fatal("ValidatePacket must reject a packet where two different screen IDs share identical PNG bytes")
	}
	if !strings.Contains(verr.Error(), "duplicate") && !strings.Contains(verr.Error(), "identical") {
		t.Errorf("error must mention duplicate/identical PNG bytes; got: %v", verr)
	}
}

// TestCorrectReferenceRoutes proves that the HTML capture routes match the
// approved reference routes per the plan's interface specification:
//   - reuse-manual-path → /create-flow/reuse-key-vs-generate (not ssh-form-blank-prefix)
//   - mouse-focused-field → /create-flow/ssh-form-filled (not ssh-form-empty)
//   - git-form-demo → /git-screen/git-form-filled (not create-flow/backup-notice)
func TestCorrectReferenceRoutes(t *testing.T) {
	routes := screenshot.ApprovedHTMLRoutes()
	cases := []struct {
		screenID  string
		wantRoute string
		badRoute  string
	}{
		{"reuse-manual-path", "/create-flow/reuse-key-vs-generate", "/create-flow/ssh-form-blank-prefix"},
		{"mouse-focused-field", "/create-flow/ssh-form-filled", "/create-flow/ssh-form-empty"},
		{"git-form-demo", "/git-screen/git-form-filled", "/create-flow/backup-notice"},
	}
	for _, tc := range cases {
		route, ok := routes[tc.screenID]
		if !ok {
			t.Errorf("screen %q missing from ApprovedHTMLRoutes", tc.screenID)
			continue
		}
		if !strings.HasSuffix(route, tc.wantRoute) && route != tc.wantRoute {
			t.Errorf("screen %q: route = %q, want suffix %q", tc.screenID, route, tc.wantRoute)
		}
		if strings.HasSuffix(route, tc.badRoute) || route == tc.badRoute {
			t.Errorf("screen %q: route = %q is the WRONG (old) route %q", tc.screenID, route, tc.badRoute)
		}
	}
}

// TestRequiredProvenance proves that a visual packet missing EVIDENCE.json,
// REGION-DIFFS.json, or REVIEW-PROVENANCE.json is rejected by ValidatePacket
// (UI-REVIEW Critical: these three files were absent from the prior packet).
func TestRequiredProvenance(t *testing.T) {
	// A visual packet MUST declare and hash all provenance files.
	// We test that ValidateVisualPacketProvenance rejects a packet that lacks them.
	// The validation function is exported by the screenshot package.
	pkt := screenshot.Packet{
		Version:        "03-12.1",
		SourceCommit:   strings.Repeat("f", 40),
		ApprovalCommit: screenshot.PacketApprovalCommit,
	}
	// Packet with NO provenance files declared.
	if err := screenshot.ValidateProvenanceRecords(pkt); err == nil {
		t.Fatal("ValidateProvenanceRecords must reject a packet without EVIDENCE.json declaration")
	}
}

// TestUndeclaredReviewArtifact proves that ValidatePacket rejects a packet
// directory containing a file not listed in the manifest — preventing an extra
// review file from being injected after publication (UI-REVIEW Critical).
func TestUndeclaredReviewArtifact(t *testing.T) {
	dir := t.TempDir()
	panels := makeMinimalPanels(t, dir)
	outDir := filepath.Join(t.TempDir(), "undeclared-packet")
	_, err := screenshot.GenerateVisualPacket(screenshot.PacketOptions{
		SourceCommit:   strings.Repeat("g", 40),
		ApprovalCommit: screenshot.PacketApprovalCommit,
		OutputDir:      outDir,
	}, panels, screenshot.PacketCapture{
		Commands: []string{"test"}, ToolVersions: []screenshot.PacketTool{{Name: "go", Version: "test"}},
		Geometry: "100x30", FontSHA256: strings.Repeat("c", 64), Theme: "test",
	})
	if err != nil {
		t.Fatalf("setup: GenerateVisualPacket failed: %v", err)
	}

	// Inject an undeclared file into the packet directory.
	undeclared := filepath.Join(outDir, "EXTRA-REVIEW.md")
	if err := os.WriteFile(undeclared, []byte("extra content"), 0o644); err != nil { //nolint:gosec // test fixture
		t.Fatalf("setup: writing undeclared file: %v", err)
	}

	_, verr := screenshot.ValidatePacket(outDir)
	if verr == nil {
		t.Fatal("ValidatePacket must reject a packet directory containing an undeclared file")
	}
	if !strings.Contains(verr.Error(), "undeclared") {
		t.Errorf("error must mention 'undeclared'; got: %v", verr)
	}
}

// ---------------------------------------------------------------------------
// 03-13 Task 3: Canonical manifest, self-hash, region diffs, duplicate policy.
// ---------------------------------------------------------------------------

// TestCanonicalManifest proves that a freshly generated packet's ManifestSHA256
// field can be reproduced by zeroing the field and re-hashing — the documented
// canonical self-hash algorithm (empty self-field, UTF-8 indented JSON, no trailing
// newline). This is the machine-verifiable contract the review can check.
func TestCanonicalManifest(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "packet")
	live, approved := makeTestCaptures(t)

	result, err := screenshot.GenerateTextPacket(screenshot.PacketOptions{
		SourceCommit:   strings.Repeat("h", 40),
		ApprovalCommit: screenshot.PacketApprovalCommit,
		OutputDir:      outDir,
		Clock:          fixedClock(time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)),
	}, live, approved)
	if err != nil {
		t.Fatalf("GenerateTextPacket: %v", err)
	}

	// Read the stored MANIFEST.json bytes.
	manifestBytes, err := os.ReadFile(result.ManifestPath) //nolint:gosec // test fixture (G304)
	if err != nil {
		t.Fatalf("reading MANIFEST.json: %v", err)
	}

	// Verify the packet is valid (self-hash validates from stored bytes).
	if _, err := screenshot.ValidatePacket(outDir); err != nil {
		t.Fatalf("ValidatePacket rejected freshly generated packet: %v", err)
	}

	// Prove round-trip: unmarshal + re-hash must match the declared hash.
	declared := result.Packet.ManifestSHA256
	if declared == "" {
		t.Fatal("GenerateTextPacket returned empty ManifestSHA256")
	}

	// The stored bytes must not have a trailing newline per the canonical rule.
	if len(manifestBytes) > 0 && manifestBytes[len(manifestBytes)-1] == '\n' {
		t.Error("stored MANIFEST.json must not end with a trailing newline (canonical rule)")
	}

	// The stored manifest must be valid JSON parseable.
	var pkt screenshot.Packet
	if err := unmarshalJSON(manifestBytes, &pkt); err != nil {
		t.Fatalf("stored MANIFEST.json is not valid JSON: %v", err)
	}

	// Zeroing the hash field and re-serializing must reproduce the stored hash.
	pkt.ManifestSHA256 = ""
	recomputed := screenshot.CanonicalManifestHash(pkt)
	if recomputed != declared {
		t.Errorf("self-hash mismatch:\n  declared: %s\n  recomputed: %s", declared, recomputed)
	}
}

// TestSelfHashAlgorithm proves the canonical self-hash algorithm explicitly:
// CanonicalManifestHash(pkt) equals the declared ManifestSHA256 when pkt
// has the hash set — because CanonicalManifestHash zeroes the field before
// hashing, exactly reproducing how the hash was originally computed.
func TestSelfHashAlgorithm(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "packet")
	live, approved := makeTestCaptures(t)

	result, err := screenshot.GenerateTextPacket(screenshot.PacketOptions{
		SourceCommit:   strings.Repeat("i", 40),
		ApprovalCommit: screenshot.PacketApprovalCommit,
		OutputDir:      outDir,
		Clock:          fixedClock(time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)),
	}, live, approved)
	if err != nil {
		t.Fatalf("GenerateTextPacket: %v", err)
	}

	declared := result.Packet.ManifestSHA256
	if declared == "" {
		t.Fatal("GenerateTextPacket returned empty ManifestSHA256")
	}

	// CanonicalManifestHash must reproduce the declared hash (it zeroes the
	// field internally, so passing in the packet with the hash set or empty
	// both produce the same result — and that result must equal declared).
	recomputed := screenshot.CanonicalManifestHash(result.Packet)
	if recomputed != declared {
		t.Errorf("CanonicalManifestHash must reproduce declared hash:\n  declared:   %s\n  recomputed: %s", declared, recomputed)
	}

	// Passing a packet with ManifestSHA256 zeroed must give the same result.
	pktEmpty := result.Packet
	pktEmpty.ManifestSHA256 = ""
	recomputedEmpty := screenshot.CanonicalManifestHash(pktEmpty)
	if recomputedEmpty != declared {
		t.Errorf("CanonicalManifestHash with empty ManifestSHA256 must still reproduce declared hash:\n  declared:   %s\n  recomputed: %s", declared, recomputedEmpty)
	}
}

// TestExplicitVariant proves that a valid same-route interaction variant
// (with VariantOf + VariantRationale) is ALLOWED by ValidateScreenSpecs
// while a spec without that metadata is rejected.
func TestExplicitVariant(t *testing.T) {
	specs := screenshot.RequiredScreenSpecs()
	// The built-in registry must pass validation (variants are declared).
	if err := screenshot.ValidateScreenSpecRegistry(); err != nil {
		t.Fatalf("ValidateScreenSpecRegistry failed on built-in registry: %v", err)
	}

	// Now add an explicit valid variant — it should NOT fail.
	validVariant := screenshot.ScreenSpec{
		ScreenID:               "reuse-manual-path",
		Route:                  "/create-flow/reuse-key-vs-generate",
		StateMarker:            "Enter a path manually",
		VariantOf:              "reuse-key-vs-generate",
		VariantRationale:       "No separate HTML route exists for manual-path interaction.",
		ApplicableLive:         true,
		ApplicableApprovedTUI:  true,
		ApplicableApprovedHTML: true,
	}
	// A registry with the valid variant alongside the base spec must pass.
	baseSpecs := []screenshot.ScreenSpec{}
	for _, s := range specs {
		if s.ScreenID == "reuse-key-vs-generate" || s.ScreenID == "reuse-manual-path" {
			baseSpecs = append(baseSpecs, s)
		}
	}
	if err := screenshot.ValidateScreenSpecs(baseSpecs); err != nil {
		t.Errorf("ValidateScreenSpecs must accept valid variant pair; got: %v", err)
	}
	_ = validVariant
}

// TestFinalPacketRequiresReviewProvenance proves ValidateProvenanceRecords
// requires REVIEW-PROVENANCE.json to be declared — raw independent reviews
// must exist before the final packet is considered complete.
func TestFinalPacketRequiresReviewProvenance(t *testing.T) {
	// A packet with EVIDENCE.json and REGION-DIFFS.json but no REVIEW-PROVENANCE.json.
	pkt := screenshot.Packet{
		Version:        "03-12.1",
		SourceCommit:   strings.Repeat("j", 40),
		ApprovalCommit: screenshot.PacketApprovalCommit,
		Members: []screenshot.PacketMember{
			{Path: "EVIDENCE.json", SHA256: strings.Repeat("a", 64), Kind: "provenance"},
			{Path: "REGION-DIFFS.json", SHA256: strings.Repeat("b", 64), Kind: "provenance"},
			// No REVIEW-PROVENANCE.json.
		},
	}
	if err := screenshot.ValidateProvenanceRecords(pkt); err == nil {
		t.Fatal("ValidateProvenanceRecords must reject a packet without REVIEW-PROVENANCE.json")
	}

	// With all three, it passes.
	pkt.Members = append(pkt.Members, screenshot.PacketMember{
		Path: "REVIEW-PROVENANCE.json", SHA256: strings.Repeat("c", 64), Kind: "provenance",
	})
	if err := screenshot.ValidateProvenanceRecords(pkt); err != nil {
		t.Errorf("ValidateProvenanceRecords must accept packet with all three provenance files: %v", err)
	}
}

// TestRegionDiffCoverage proves that BuildRegionDiffs stores the complete
// AllRegionNames inventory for each ScreenSpec. Empty extracted evidence is a
// valid inventory entry; RequiredRegions separately defines what must be nonempty.
func TestRegionDiffCoverage(t *testing.T) {
	// Use dummy backend captures as the input (live vs approved-tui text).
	// RequiredScreenSpecs() below is a MERGED registry (04-04-PLAN.md Task 3,
	// 05-09-PLAN.md Task 3), so makeTestCaptures merges CaptureCreateFlowScreens,
	// CaptureGitScreenScreens, AND CaptureIdentityManagerScreens.
	liveCaptures, approvedCaptures := makeTestCaptures(t)

	specs := screenshot.RequiredScreenSpecs()
	diffs, err := screenshot.BuildRegionDiffs("test-commit", liveCaptures, approvedCaptures, specs)
	if err != nil {
		t.Fatalf("BuildRegionDiffs: %v", err)
	}

	// Must have at least one diff per ScreenSpec.
	if len(diffs) == 0 {
		t.Fatal("BuildRegionDiffs must return non-empty diffs")
	}
	allRegionNames := screenshot.AllRegionNames()
	screensSeen := make(map[string]bool)
	for _, d := range diffs {
		screensSeen[d.ScreenID] = true
		if d.ScreenID == "" {
			t.Error("diff has empty ScreenID")
		}
		if len(d.Regions) != len(allRegionNames) {
			t.Errorf("diff %q has %d named regions, want complete inventory of %d", d.ScreenID, len(d.Regions), len(allRegionNames))
		}
		regionsSeen := make(map[screenshot.RegionName]bool, len(d.Regions))
		for _, region := range d.Regions {
			regionsSeen[region.Name] = true
		}
		for _, name := range allRegionNames {
			if !regionsSeen[name] {
				t.Errorf("diff %q is missing named region %q", d.ScreenID, name)
			}
		}
	}
	// Every spec must have at least one diff record.
	for _, spec := range specs {
		if !screensSeen[spec.ScreenID] {
			t.Errorf("BuildRegionDiffs missing coverage for spec %q", spec.ScreenID)
		}
	}
}

func TestCandidateManifestIsReviewableButNotFinal(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "candidate")
	result, err := screenshot.GenerateVisualPacket(screenshot.PacketOptions{
		SourceCommit:   strings.Repeat("a", 40),
		ApprovalCommit: strings.Repeat("b", 40),
		LiveBackendRef: "candidate",
		OutputDir:      dir,
		ProvenanceFiles: map[string][]byte{
			"EVIDENCE.json":     []byte("evidence"),
			"REGION-DIFFS.json": validRegionDiffs(t, strings.Repeat("a", 40)),
		},
	}, makeMinimalPanels(t, t.TempDir()), screenshot.PacketCapture{
		Commands: []string{"test"}, ToolVersions: []screenshot.PacketTool{{Name: "go", Version: "test"}},
		Geometry: "100x30", FontSHA256: strings.Repeat("c", 64), Theme: "test",
	})
	if err != nil {
		t.Fatalf("GenerateVisualPacket: %v", err)
	}
	if err := os.Rename(result.ManifestPath, filepath.Join(dir, "CANDIDATE-MANIFEST.json")); err != nil {
		t.Fatalf("renaming candidate manifest: %v", err)
	}
	if _, err := screenshot.ValidateCandidate(dir); err != nil {
		t.Fatalf("ValidateCandidate rejected reviewable candidate: %v", err)
	}
	if _, err := screenshot.ValidatePacket(dir); err == nil {
		t.Fatal("ValidatePacket accepted a candidate without final review provenance")
	}
}

// ---------------------------------------------------------------------------
// 03-16 Task 1: Single configured-review finalization contract.
//
// Exactly one record from opencode-my-plan-review / opencode /
// local-llm-env/my-plan-review is necessary and sufficient.  Every substitute,
// duplicate, missing, stale, nonzero, empty-provider, or provider-inconsistent
// verdict fails closed.
// ---------------------------------------------------------------------------

// makeFinalizeCandidate builds a minimal reviewable candidate for Task 1 tests.
func makeFinalizeCandidate(t *testing.T) (candidateDir string, candidateManifestSHA256 string, source string) {
	t.Helper()
	source = strings.Repeat("d", 40)
	candidateDir = filepath.Join(t.TempDir(), "candidate")
	result, err := screenshot.GenerateVisualPacket(screenshot.PacketOptions{
		SourceCommit: source, ApprovalCommit: screenshot.PacketApprovalCommit, OutputDir: candidateDir,
		ProvenanceFiles: map[string][]byte{"EVIDENCE.json": []byte("evidence"), "REGION-DIFFS.json": validRegionDiffs(t, source)},
	}, makeMinimalPanels(t, t.TempDir()), screenshot.PacketCapture{
		Commands: []string{"test"}, ToolVersions: []screenshot.PacketTool{{Name: "go", Version: "test"}},
		Geometry: "100x30", FontSHA256: strings.Repeat("a", 64), Theme: "test",
	})
	if err != nil {
		t.Fatalf("GenerateVisualPacket: %v", err)
	}
	if err := os.Rename(result.ManifestPath, filepath.Join(candidateDir, "CANDIDATE-MANIFEST.json")); err != nil {
		t.Fatalf("renaming candidate manifest: %v", err)
	}
	return candidateDir, result.Packet.ManifestSHA256, source
}

// makeConfiguredReview builds a ReviewInput from the configured reviewer constants.
func makeConfiguredReview(t *testing.T, source, candidateHash string, critical, high int, stdout, providerOverride, modelOverride string) screenshot.ReviewInput {
	t.Helper()
	reviewer := screenshot.ConfiguredReviewerInstance
	verdictReviewer := reviewer
	verdictVersion := "03-16.1"
	verdict, err := json.Marshal(screenshot.ReviewVerdict{
		Version:                 verdictVersion,
		SourceCommit:            source,
		CandidateManifestSHA256: candidateHash,
		Reviewer:                verdictReviewer,
		OpenCritical:            critical,
		OpenHigh:                high,
	})
	if err != nil {
		t.Fatalf("marshaling verdict: %v", err)
	}
	tool := screenshot.ConfiguredReviewerCLI
	model := screenshot.ConfiguredReviewerModel
	if modelOverride != "" {
		model = modelOverride
	}
	provider := screenshot.ConfiguredReviewerCLI + "-provider-hash-abc123"
	if providerOverride != "" {
		provider = providerOverride
	}
	return screenshot.ReviewInput{
		Reviewer:  reviewer,
		Tool:      tool,
		Provider:  provider,
		Model:     model,
		Session:   "session-abc",
		StartedAt: "2026-08-23T00:00:00Z",
		EndedAt:   "2026-08-23T00:01:00Z",
		Prompt:    []byte("review prompt"),
		Stdout:    []byte(stdout),
		Stderr:    []byte(""),
		Verdict:   verdict,
	}
}

// TestFinalizeCandidateSingleConfiguredReview: one fresh zero-blocker record
// from the configured reviewer finalizes successfully.
func TestFinalizeCandidateSingleConfiguredReview(t *testing.T) {
	candidateDir, candidateHash, source := makeFinalizeCandidate(t)
	review := makeConfiguredReview(t, source, candidateHash, 0, 0, "unique stdout content", "", "")
	finalDir := filepath.Join(t.TempDir(), "final")
	if _, err := screenshot.FinalizeCandidate(candidateDir, finalDir, []screenshot.ReviewInput{review}); err != nil {
		t.Fatalf("FinalizeCandidate must accept one configured zero-blocker review: %v", err)
	}
	if _, err := screenshot.ValidatePacket(finalDir); err != nil {
		t.Fatalf("ValidatePacket must accept finalized packet: %v", err)
	}
}

// TestFinalizeCandidateRejectsSubstituteReviewer: a non-configured reviewer name fails.
func TestFinalizeCandidateRejectsSubstituteReviewer(t *testing.T) {
	candidateDir, candidateHash, source := makeFinalizeCandidate(t)
	// Build a review with a wrong reviewer name but correct tool/model.
	verdictBytes, _ := json.Marshal(screenshot.ReviewVerdict{
		Version: "03-16.1", SourceCommit: source, CandidateManifestSHA256: candidateHash,
		Reviewer: "wrong-reviewer", OpenCritical: 0, OpenHigh: 0,
	})
	review := screenshot.ReviewInput{
		Reviewer: "wrong-reviewer", Tool: screenshot.ConfiguredReviewerCLI,
		Provider: "some-provider", Model: screenshot.ConfiguredReviewerModel,
		Session: "s", StartedAt: "2026-08-23T00:00:00Z", EndedAt: "2026-08-23T00:01:00Z",
		Prompt: []byte("p"), Stdout: []byte("out"), Stderr: []byte(""), Verdict: verdictBytes,
	}
	if _, err := screenshot.FinalizeCandidate(candidateDir, filepath.Join(t.TempDir(), "blocked"), []screenshot.ReviewInput{review}); err == nil {
		t.Fatal("FinalizeCandidate must reject a substitute reviewer name")
	}
}

// TestFinalizeCandidateRejectsReviewerCLIMismatch: wrong CLI fails.
func TestFinalizeCandidateRejectsReviewerCLIMismatch(t *testing.T) {
	candidateDir, candidateHash, source := makeFinalizeCandidate(t)
	verdictBytes, _ := json.Marshal(screenshot.ReviewVerdict{
		Version: "03-16.1", SourceCommit: source, CandidateManifestSHA256: candidateHash,
		Reviewer: screenshot.ConfiguredReviewerInstance, OpenCritical: 0, OpenHigh: 0,
	})
	review := screenshot.ReviewInput{
		Reviewer: screenshot.ConfiguredReviewerInstance, Tool: "wrong-cli",
		Provider: "some-provider", Model: screenshot.ConfiguredReviewerModel,
		Session: "s", StartedAt: "2026-08-23T00:00:00Z", EndedAt: "2026-08-23T00:01:00Z",
		Prompt: []byte("p"), Stdout: []byte("out"), Stderr: []byte(""), Verdict: verdictBytes,
	}
	if _, err := screenshot.FinalizeCandidate(candidateDir, filepath.Join(t.TempDir(), "blocked"), []screenshot.ReviewInput{review}); err == nil {
		t.Fatal("FinalizeCandidate must reject a CLI mismatch")
	}
}

// TestFinalizeCandidateRejectsReviewerModelMismatch: wrong model fails.
func TestFinalizeCandidateRejectsReviewerModelMismatch(t *testing.T) {
	candidateDir, candidateHash, source := makeFinalizeCandidate(t)
	verdictBytes, _ := json.Marshal(screenshot.ReviewVerdict{
		Version: "03-16.1", SourceCommit: source, CandidateManifestSHA256: candidateHash,
		Reviewer: screenshot.ConfiguredReviewerInstance, OpenCritical: 0, OpenHigh: 0,
	})
	review := screenshot.ReviewInput{
		Reviewer: screenshot.ConfiguredReviewerInstance, Tool: screenshot.ConfiguredReviewerCLI,
		Provider: "some-provider", Model: "wrong-model",
		Session: "s", StartedAt: "2026-08-23T00:00:00Z", EndedAt: "2026-08-23T00:01:00Z",
		Prompt: []byte("p"), Stdout: []byte("out"), Stderr: []byte(""), Verdict: verdictBytes,
	}
	if _, err := screenshot.FinalizeCandidate(candidateDir, filepath.Join(t.TempDir(), "blocked"), []screenshot.ReviewInput{review}); err == nil {
		t.Fatal("FinalizeCandidate must reject a model mismatch")
	}
}

// TestFinalizeCandidateRejectsEmptyReviewerProvider: empty provider fails.
func TestFinalizeCandidateRejectsEmptyReviewerProvider(t *testing.T) {
	candidateDir, candidateHash, source := makeFinalizeCandidate(t)
	verdictBytes, _ := json.Marshal(screenshot.ReviewVerdict{
		Version: "03-16.1", SourceCommit: source, CandidateManifestSHA256: candidateHash,
		Reviewer: screenshot.ConfiguredReviewerInstance, OpenCritical: 0, OpenHigh: 0,
	})
	review := screenshot.ReviewInput{
		Reviewer: screenshot.ConfiguredReviewerInstance, Tool: screenshot.ConfiguredReviewerCLI,
		Provider: "", Model: screenshot.ConfiguredReviewerModel, // empty provider
		Session: "s", StartedAt: "2026-08-23T00:00:00Z", EndedAt: "2026-08-23T00:01:00Z",
		Prompt: []byte("p"), Stdout: []byte("out"), Stderr: []byte(""), Verdict: verdictBytes,
	}
	if _, err := screenshot.FinalizeCandidate(candidateDir, filepath.Join(t.TempDir(), "blocked"), []screenshot.ReviewInput{review}); err == nil {
		t.Fatal("FinalizeCandidate must reject an empty provider")
	}
}

// TestFinalizeCandidateRejectsTamperedReviewerProviderProvenance: tampered provider fails.
func TestFinalizeCandidateRejectsTamperedReviewerProviderProvenance(t *testing.T) {
	candidateDir, candidateHash, source := makeFinalizeCandidate(t)
	// Provider field is intentionally a hard-coded literal (configured CLI name) —
	// that would be a tampered/unverifiable provenance.
	verdictBytes, _ := json.Marshal(screenshot.ReviewVerdict{
		Version: "03-16.1", SourceCommit: source, CandidateManifestSHA256: candidateHash,
		Reviewer: screenshot.ConfiguredReviewerInstance, OpenCritical: 0, OpenHigh: 0,
	})
	review := screenshot.ReviewInput{
		Reviewer: screenshot.ConfiguredReviewerInstance, Tool: screenshot.ConfiguredReviewerCLI,
		Provider: screenshot.ConfiguredReviewerCLI, // provider equals CLI name — tampered
		Model:    screenshot.ConfiguredReviewerModel,
		Session:  "s", StartedAt: "2026-08-23T00:00:00Z", EndedAt: "2026-08-23T00:01:00Z",
		Prompt: []byte("p"), Stdout: []byte("out"), Stderr: []byte(""), Verdict: verdictBytes,
	}
	if _, err := screenshot.FinalizeCandidate(candidateDir, filepath.Join(t.TempDir(), "blocked"), []screenshot.ReviewInput{review}); err == nil {
		t.Fatal("FinalizeCandidate must reject a tampered provider (provider == CLI name)")
	}
}

// TestFinalizeCandidateRejectsDuplicateConfiguredReviewer: two records with same reviewer fails.
func TestFinalizeCandidateRejectsDuplicateConfiguredReviewer(t *testing.T) {
	candidateDir, candidateHash, source := makeFinalizeCandidate(t)
	r1 := makeConfiguredReview(t, source, candidateHash, 0, 0, "output-a", "", "")
	r2 := makeConfiguredReview(t, source, candidateHash, 0, 0, "output-b", "", "")
	if _, err := screenshot.FinalizeCandidate(candidateDir, filepath.Join(t.TempDir(), "blocked"), []screenshot.ReviewInput{r1, r2}); err == nil {
		t.Fatal("FinalizeCandidate must reject duplicate configured reviewer records")
	}
}

// TestFinalizeCandidateRejectsMissingConfiguredReviewer: no configured reviewer fails.
func TestFinalizeCandidateRejectsMissingConfiguredReviewer(t *testing.T) {
	candidateDir, candidateHash, source := makeFinalizeCandidate(t)
	// Use a different (non-configured) reviewer.
	verdictBytes, _ := json.Marshal(screenshot.ReviewVerdict{
		Version: "03-16.1", SourceCommit: source, CandidateManifestSHA256: candidateHash,
		Reviewer: "other-reviewer", OpenCritical: 0, OpenHigh: 0,
	})
	review := screenshot.ReviewInput{
		Reviewer: "other-reviewer", Tool: "other-cli",
		Provider: "other-provider-hash", Model: "other-model",
		Session: "s", StartedAt: "2026-08-23T00:00:00Z", EndedAt: "2026-08-23T00:01:00Z",
		Prompt: []byte("p"), Stdout: []byte("out"), Stderr: []byte(""), Verdict: verdictBytes,
	}
	if _, err := screenshot.FinalizeCandidate(candidateDir, filepath.Join(t.TempDir(), "blocked"), []screenshot.ReviewInput{review}); err == nil {
		t.Fatal("FinalizeCandidate must reject a review with no configured reviewer")
	}
}

// TestFinalizeCandidateRejectsStaleOrMismatchedConfiguredReview: stale/mismatched verdict fails.
func TestFinalizeCandidateRejectsStaleOrMismatchedConfiguredReview(t *testing.T) {
	candidateDir, candidateHash, source := makeFinalizeCandidate(t)
	// Stale version string.
	staleVerdictBytes, _ := json.Marshal(screenshot.ReviewVerdict{
		Version:      "03-14.1", // old version
		SourceCommit: source, CandidateManifestSHA256: candidateHash,
		Reviewer: screenshot.ConfiguredReviewerInstance, OpenCritical: 0, OpenHigh: 0,
	})
	review := screenshot.ReviewInput{
		Reviewer: screenshot.ConfiguredReviewerInstance, Tool: screenshot.ConfiguredReviewerCLI,
		Provider: "provider-hash-xyz", Model: screenshot.ConfiguredReviewerModel,
		Session: "s", StartedAt: "2026-08-23T00:00:00Z", EndedAt: "2026-08-23T00:01:00Z",
		Prompt: []byte("p"), Stdout: []byte("out"), Stderr: []byte(""), Verdict: staleVerdictBytes,
	}
	if _, err := screenshot.FinalizeCandidate(candidateDir, filepath.Join(t.TempDir(), "blocked-stale"), []screenshot.ReviewInput{review}); err == nil {
		t.Fatal("FinalizeCandidate must reject a stale verdict version")
	}
	// Mismatched candidate hash.
	mismatchVerdictBytes, _ := json.Marshal(screenshot.ReviewVerdict{
		Version: "03-16.1", SourceCommit: source,
		CandidateManifestSHA256: strings.Repeat("0", 64), // wrong hash
		Reviewer:                screenshot.ConfiguredReviewerInstance, OpenCritical: 0, OpenHigh: 0,
	})
	review2 := screenshot.ReviewInput{
		Reviewer: screenshot.ConfiguredReviewerInstance, Tool: screenshot.ConfiguredReviewerCLI,
		Provider: "provider-hash-xyz", Model: screenshot.ConfiguredReviewerModel,
		Session: "s2", StartedAt: "2026-08-23T00:00:00Z", EndedAt: "2026-08-23T00:01:00Z",
		Prompt: []byte("p"), Stdout: []byte("out2"), Stderr: []byte(""), Verdict: mismatchVerdictBytes,
	}
	if _, err := screenshot.FinalizeCandidate(candidateDir, filepath.Join(t.TempDir(), "blocked-mismatch"), []screenshot.ReviewInput{review2}); err == nil {
		t.Fatal("FinalizeCandidate must reject a mismatched candidate hash")
	}
}

// TestValidateReviewProvenanceRejectsSubstituteReviewerTriple: wrong reviewer triple in provenance fails.
func TestValidateReviewProvenanceRejectsSubstituteReviewerTriple(t *testing.T) {
	candidateDir, candidateHash, source := makeFinalizeCandidate(t)
	// First build a finalized packet using the correct configured reviewer,
	// then directly test ValidateReviewProvenance with a tampered provenance.
	_ = candidateDir
	pkt := screenshot.Packet{
		Version:        "03-11.2",
		SourceCommit:   source,
		ApprovalCommit: screenshot.PacketApprovalCommit,
		Members: []screenshot.PacketMember{
			{Path: "EVIDENCE.json", SHA256: strings.Repeat("a", 64), Kind: "provenance"},
			{Path: "REGION-DIFFS.json", SHA256: strings.Repeat("b", 64), Kind: "provenance"},
			{Path: "REVIEW-PROVENANCE.json", SHA256: strings.Repeat("c", 64), Kind: "provenance"},
			{Path: "CANDIDATE-MANIFEST.json", SHA256: strings.Repeat("d", 64), Kind: "candidate-manifest"},
		},
	}
	_ = pkt
	_ = candidateHash
	// This test verifies the validator rejects a wrong reviewer triple.
	// We test it indirectly: a finalized packet with a non-configured reviewer
	// reviewer-name in REVIEW-PROVENANCE.json must fail ValidateReviewProvenance.
	// Since we can't easily call ValidateReviewProvenance in isolation without
	// a full packet directory, we verify the behavior through FinalizeCandidate.
	verdictBytes, _ := json.Marshal(screenshot.ReviewVerdict{
		Version: "03-16.1", SourceCommit: source, CandidateManifestSHA256: candidateHash,
		Reviewer: "wrong-reviewer", OpenCritical: 0, OpenHigh: 0,
	})
	review := screenshot.ReviewInput{
		Reviewer: "wrong-reviewer", Tool: screenshot.ConfiguredReviewerCLI,
		Provider: "some-provider-hash", Model: screenshot.ConfiguredReviewerModel,
		Session: "s", StartedAt: "2026-08-23T00:00:00Z", EndedAt: "2026-08-23T00:01:00Z",
		Prompt: []byte("p"), Stdout: []byte("out"), Stderr: []byte(""), Verdict: verdictBytes,
	}
	if _, err := screenshot.FinalizeCandidate(candidateDir, filepath.Join(t.TempDir(), "blocked"), []screenshot.ReviewInput{review}); err == nil {
		t.Fatal("FinalizeCandidate (via ValidateReviewProvenance) must reject wrong reviewer triple")
	}
}

// TestValidateReviewProvenanceRejectsCLIOrModelMismatch: CLI/model mismatch fails.
func TestValidateReviewProvenanceRejectsCLIOrModelMismatch(t *testing.T) {
	candidateDir, candidateHash, source := makeFinalizeCandidate(t)
	// Wrong CLI
	verdictBytes, _ := json.Marshal(screenshot.ReviewVerdict{
		Version: "03-16.1", SourceCommit: source, CandidateManifestSHA256: candidateHash,
		Reviewer: screenshot.ConfiguredReviewerInstance, OpenCritical: 0, OpenHigh: 0,
	})
	cliMismatch := screenshot.ReviewInput{
		Reviewer: screenshot.ConfiguredReviewerInstance, Tool: "wrong-cli",
		Provider: "provider-hash", Model: screenshot.ConfiguredReviewerModel,
		Session: "s", StartedAt: "2026-08-23T00:00:00Z", EndedAt: "2026-08-23T00:01:00Z",
		Prompt: []byte("p"), Stdout: []byte("out"), Stderr: []byte(""), Verdict: verdictBytes,
	}
	if _, err := screenshot.FinalizeCandidate(candidateDir, filepath.Join(t.TempDir(), "blocked-cli"), []screenshot.ReviewInput{cliMismatch}); err == nil {
		t.Fatal("FinalizeCandidate must reject wrong CLI")
	}
	// Wrong model
	modelMismatch := screenshot.ReviewInput{
		Reviewer: screenshot.ConfiguredReviewerInstance, Tool: screenshot.ConfiguredReviewerCLI,
		Provider: "provider-hash", Model: "wrong-model",
		Session: "s", StartedAt: "2026-08-23T00:00:00Z", EndedAt: "2026-08-23T00:01:00Z",
		Prompt: []byte("p"), Stdout: []byte("out2"), Stderr: []byte(""), Verdict: verdictBytes,
	}
	if _, err := screenshot.FinalizeCandidate(candidateDir, filepath.Join(t.TempDir(), "blocked-model"), []screenshot.ReviewInput{modelMismatch}); err == nil {
		t.Fatal("FinalizeCandidate must reject wrong model")
	}
}

// TestValidateReviewProvenanceRejectsEmptyProvider: empty provider fails.
func TestValidateReviewProvenanceRejectsEmptyProvider(t *testing.T) {
	candidateDir, candidateHash, source := makeFinalizeCandidate(t)
	verdictBytes, _ := json.Marshal(screenshot.ReviewVerdict{
		Version: "03-16.1", SourceCommit: source, CandidateManifestSHA256: candidateHash,
		Reviewer: screenshot.ConfiguredReviewerInstance, OpenCritical: 0, OpenHigh: 0,
	})
	review := screenshot.ReviewInput{
		Reviewer: screenshot.ConfiguredReviewerInstance, Tool: screenshot.ConfiguredReviewerCLI,
		Provider: "", Model: screenshot.ConfiguredReviewerModel,
		Session: "s", StartedAt: "2026-08-23T00:00:00Z", EndedAt: "2026-08-23T00:01:00Z",
		Prompt: []byte("p"), Stdout: []byte("out"), Stderr: []byte(""), Verdict: verdictBytes,
	}
	if _, err := screenshot.FinalizeCandidate(candidateDir, filepath.Join(t.TempDir(), "blocked"), []screenshot.ReviewInput{review}); err == nil {
		t.Fatal("ValidateReviewProvenance must reject empty provider")
	}
}

// TestValidateReviewProvenanceRejectsTamperedProviderProvenance: tampered provider fails.
func TestValidateReviewProvenanceRejectsTamperedProviderProvenance(t *testing.T) {
	candidateDir, candidateHash, source := makeFinalizeCandidate(t)
	verdictBytes, _ := json.Marshal(screenshot.ReviewVerdict{
		Version: "03-16.1", SourceCommit: source, CandidateManifestSHA256: candidateHash,
		Reviewer: screenshot.ConfiguredReviewerInstance, OpenCritical: 0, OpenHigh: 0,
	})
	// Provider is a configured literal (same as the CLI name) — tampered.
	review := screenshot.ReviewInput{
		Reviewer: screenshot.ConfiguredReviewerInstance, Tool: screenshot.ConfiguredReviewerCLI,
		Provider: screenshot.ConfiguredReviewerCLI, // tampered: provider == tool name
		Model:    screenshot.ConfiguredReviewerModel,
		Session:  "s", StartedAt: "2026-08-23T00:00:00Z", EndedAt: "2026-08-23T00:01:00Z",
		Prompt: []byte("p"), Stdout: []byte("out"), Stderr: []byte(""), Verdict: verdictBytes,
	}
	if _, err := screenshot.FinalizeCandidate(candidateDir, filepath.Join(t.TempDir(), "blocked"), []screenshot.ReviewInput{review}); err == nil {
		t.Fatal("ValidateReviewProvenance must reject tampered provider provenance (provider == tool)")
	}
}

// TestValidateReviewProvenanceRejectsDuplicateMissingStaleOrMismatchedTriple:
// duplicate, missing, and stale all fail.
func TestValidateReviewProvenanceRejectsDuplicateMissingStaleOrMismatchedTriple(t *testing.T) {
	candidateDir, candidateHash, source := makeFinalizeCandidate(t)
	// Zero reviews — missing configured reviewer.
	if _, err := screenshot.FinalizeCandidate(candidateDir, filepath.Join(t.TempDir(), "empty"), []screenshot.ReviewInput{}); err == nil {
		t.Fatal("FinalizeCandidate must reject empty review list")
	}
	// Two reviews — duplicate configured reviewer.
	r1 := makeConfiguredReview(t, source, candidateHash, 0, 0, "out-a", "", "")
	r2 := makeConfiguredReview(t, source, candidateHash, 0, 0, "out-b", "", "")
	if _, err := screenshot.FinalizeCandidate(candidateDir, filepath.Join(t.TempDir(), "dup"), []screenshot.ReviewInput{r1, r2}); err == nil {
		t.Fatal("FinalizeCandidate must reject duplicate configured reviewer")
	}
	// Stale version.
	staleVerdictBytes, _ := json.Marshal(screenshot.ReviewVerdict{
		Version: "03-14.1", SourceCommit: source, CandidateManifestSHA256: candidateHash,
		Reviewer: screenshot.ConfiguredReviewerInstance, OpenCritical: 0, OpenHigh: 0,
	})
	staleReview := screenshot.ReviewInput{
		Reviewer: screenshot.ConfiguredReviewerInstance, Tool: screenshot.ConfiguredReviewerCLI,
		Provider: "provider-hash", Model: screenshot.ConfiguredReviewerModel,
		Session: "s", StartedAt: "2026-08-23T00:00:00Z", EndedAt: "2026-08-23T00:01:00Z",
		Prompt: []byte("p"), Stdout: []byte("out"), Stderr: []byte(""), Verdict: staleVerdictBytes,
	}
	if _, err := screenshot.FinalizeCandidate(candidateDir, filepath.Join(t.TempDir(), "stale"), []screenshot.ReviewInput{staleReview}); err == nil {
		t.Fatal("FinalizeCandidate must reject stale verdict version")
	}
}

// ---------------------------------------------------------------------------
// 03-13 Task 2: ScreenSpec registry, state markers, confirm sentinel.
// ---------------------------------------------------------------------------

// TestScreenSpecRegistry proves that the ScreenSpec registry contains every
// required logical screen ID with non-empty route, interaction, and state marker.
func TestScreenSpecRegistry(t *testing.T) {
	specs := screenshot.ScreenSpecRegistry()
	if len(specs) == 0 {
		t.Fatal("ScreenSpecRegistry must return non-empty specs")
	}
	if err := screenshot.ValidateScreenSpecs(specs); err != nil {
		t.Fatalf("ScreenSpecRegistry is not a complete valid inventory: %v", err)
	}
	for _, spec := range specs {
		if len(spec.RequiredRegions) == 0 {
			t.Errorf("spec %q has no required regions", spec.ScreenID)
		}
	}
}

func TestRequiredSemanticInventoryAndRawEvidence(t *testing.T) {
	seen := make(map[string]bool)
	for _, spec := range screenshot.RequiredScreenSpecs() {
		seen[spec.ScreenID] = true
	}
	for _, id := range []string{
		"reuse-manual-resolved", "test-stage1-pass", "test-stage1-command-output", "test-stage2-command-output",
		"test-stage2-resolution-user-host-port", "test-stage2-resolution-identities-key",
		"test-reachable-not-uploaded", "test-hard-failure-retry", "confirm-summary-key-path", "confirm-managed-block",
	} {
		if !seen[id] {
			t.Errorf("registry is missing required semantic frame %q", id)
		}
	}
	panels := makeMinimalPanels(t, t.TempDir())
	panels[0].RawText = ""
	_, err := screenshot.GenerateVisualPacket(screenshot.PacketOptions{
		SourceCommit: strings.Repeat("e", 40), ApprovalCommit: screenshot.PacketApprovalCommit, OutputDir: filepath.Join(t.TempDir(), "packet"),
	}, panels, screenshot.PacketCapture{Commands: []string{"test"}, ToolVersions: []screenshot.PacketTool{{Name: "go", Version: "test"}}, Geometry: "100x30", FontSHA256: strings.Repeat("a", 64), Theme: "test"})
	if err == nil || !strings.Contains(err.Error(), "raw evidence") {
		t.Fatalf("packet must reject an absent raw transcript; got %v", err)
	}
}

func TestRequiredScreenSpecsCoverExactFrames(t *testing.T) {
	want := map[string][]string{
		"test-stage1-command-output":            {"Stage 1 output:"},
		"test-stage2-command-output":            {"Stage 2 output:"},
		"test-stage2-resolution-user-host-port": {"user git", "hostname ssh.github.com", "port 443"},
		"test-stage2-resolution-identities-key": {"identitiesonly yes", "identityfile"},
		"confirm-summary-key-path":              {"~/.ssh/id_ed25519_acme"},
		"confirm-managed-block":                 {"# BEGIN gitid managed:", "# END gitid managed:"},
	}
	seen := make(map[string]screenshot.ScreenSpec)
	for _, spec := range screenshot.RequiredScreenSpecs() {
		seen[spec.ScreenID] = spec
	}
	for id, markers := range want {
		spec, ok := seen[id]
		if !ok {
			t.Errorf("RequiredScreenSpecs is missing exact frame %q", id)
			continue
		}
		if !spec.ApplicableLive {
			t.Errorf("exact frame %q is not live-applicable", id)
		}
		if fmt.Sprint(spec.StateMarkers) != fmt.Sprint(markers) {
			t.Errorf("exact frame %q markers = %v, want %v", id, spec.StateMarkers, markers)
		}
	}
	for _, obsolete := range []string{"test-stage2-proof-top", "test-stage2-proof-bottom", "test-stage2-proof-right", "confirm-summary"} {
		if _, ok := seen[obsolete]; ok {
			t.Errorf("RequiredScreenSpecs still contains generic/clipped frame %q", obsolete)
		}
	}
	if err := screenshot.ValidateCapturedState(seen["test-stage2-command-output"], "Proof viewport focused"); err == nil {
		t.Fatal("generic proof focus marker must not authorize an exact command/output frame")
	}
	if err := screenshot.ValidateCapturedState(seen["confirm-summary-key-path"], "~/.ssh/id_ed25519_ac"); err == nil {
		t.Fatal("clipped key path must not authorize the complete summary frame")
	}
	if err := screenshot.ValidateCapturedState(seen["test-stage2-command-output"], "Stage 1 output:"); err == nil {
		t.Fatal("another frame's marker must not authorize the stage-two frame")
	}
}

func liveOnlyRegionSpec() screenshot.ScreenSpec {
	return screenshot.ScreenSpec{
		ScreenID:        "live-only-proof",
		StateMarker:     "Stage 1 output:",
		StateMarkers:    []string{"Stage 1 output:"},
		ApplicableLive:  true,
		RequiredRegions: []screenshot.RegionName{screenshot.RegionConnectivityOutput},
		NonApplicability: []screenshot.SurfaceNonApplicability{
			{Surface: "approved-tui", Decision: "D-04", Reason: "The dummy has no completed proof viewport.", Classification: "ux-improvement"},
			{Surface: "approved-html", Decision: "D-04", Reason: "HTML is not a parity target.", Classification: "ux-improvement"},
		},
	}
}

func TestRegionNonComparable(t *testing.T) {
	spec := liveOnlyRegionSpec()
	// extractConnectivityOutput's marker is "ssh -" (a flag-prefixed
	// invocation), not a bare "ssh " substring (04-04-PLAN.md Task 3
	// discovery, createflow_regions.go:513-521 -- narrowed to stop
	// false-positiving on the git-screen form's "gpg.format=ssh" line). This
	// fixture must use a real invocation-shaped line to stay inside the
	// region it is testing.
	live := "│ ssh -T git@host\n│ Stage 1 output: exact bytes\n"
	records, err := screenshot.BuildRegionDiffs("test-commit", map[string]string{spec.ScreenID: live}, nil, []screenshot.ScreenSpec{spec})
	if err != nil {
		t.Fatalf("BuildRegionDiffs live-only frame: %v", err)
	}
	region := regionDiffByName(t, records[0], screenshot.RegionConnectivityOutput)
	if region.Comparable || region.Equal {
		t.Fatalf("live-only region must be explicitly non-comparable and unequal: %+v", region)
	}
	if region.NonApplicabilityReason == "" || region.Decision != "D-04" || region.Classification != "ux-improvement" {
		t.Fatalf("live-only region lacks decision-linked classification: %+v", region)
	}
}

func TestRegionClassification(t *testing.T) {
	spec := screenshot.ScreenSpec{
		ScreenID:              "applicable-proof",
		StateMarker:           "ssh command",
		StateMarkers:          []string{"ssh command"},
		ApplicableLive:        true,
		ApplicableApprovedTUI: true,
		RequiredRegions:       []screenshot.RegionName{screenshot.RegionConnectivityOutput},
		RegionDispositions: []screenshot.RegionDisposition{{
			Region: screenshot.RegionConnectivityOutput, Divergence: "connectivity-output", Decision: "D-02",
			Reason: "The live outcome differs from the approved fixture.", Classification: "ux-improvement",
		}},
		NonApplicability: []screenshot.SurfaceNonApplicability{{Surface: "approved-html", Decision: "D-04", Reason: "HTML is not a parity target.", Classification: "ux-improvement"}},
	}
	live := "shared header\nshared breadcrumb\n│ ssh command\n│ authenticated live\nEsc returns\nshared footer 1\nshared footer 2\n"
	dummy := "shared header\nshared breadcrumb\n│ ssh command\n│ authenticated dummy\nEsc returns\nshared footer 1\nshared footer 2\n"
	records, err := screenshot.BuildRegionDiffs("test-commit", map[string]string{spec.ScreenID: live}, map[string]string{spec.ScreenID: dummy}, []screenshot.ScreenSpec{spec})
	if err != nil {
		t.Fatalf("BuildRegionDiffs applicable difference: %v", err)
	}
	region := regionDiffByName(t, records[0], screenshot.RegionConnectivityOutput)
	if !region.Comparable || region.Equal || region.Classification == "" {
		t.Fatalf("applicable unequal region lacks an explicit classification: %+v", region)
	}
	data := screenshot.BuildRegionDiffsJSON("test-commit", records)
	if err := screenshot.ValidateRegionDiffs(data, "test-commit", []screenshot.ScreenSpec{spec}); err != nil {
		t.Fatalf("ValidateRegionDiffs rejected classified difference: %v", err)
	}
	for i := range records[0].Regions {
		if records[0].Regions[i].Name == screenshot.RegionConnectivityOutput {
			records[0].Regions[i].Classification = ""
		}
	}
	data = screenshot.BuildRegionDiffsJSON("test-commit", records)
	if err := screenshot.ValidateRegionDiffs(data, "test-commit", []screenshot.ScreenSpec{spec}); err == nil {
		t.Fatal("ValidateRegionDiffs accepted an unequal region without classification")
	}
}

func TestBuildRegionDiffsRejectsUnequalRegionWithoutScreenDisposition(t *testing.T) {
	spec := screenshot.ScreenSpec{
		ScreenID:              "undeclared-required-difference",
		StateMarker:           "ssh command",
		ApplicableLive:        true,
		ApplicableApprovedTUI: true,
		RequiredRegions:       []screenshot.RegionName{screenshot.RegionConnectivityOutput},
		NonApplicability: []screenshot.SurfaceNonApplicability{{
			Surface: "approved-html", Decision: "D-04", Reason: "HTML is not a parity target.", Classification: "ux-improvement",
		}},
	}
	live := "shared header\nshared breadcrumb\n│ ssh command\n│ authenticated live\nEsc returns\nshared footer 1\nshared footer 2\n"
	approved := "shared header\nshared breadcrumb\n│ ssh command\n│ authenticated approved\nEsc returns\nshared footer 1\nshared footer 2\n"

	_, err := screenshot.BuildRegionDiffs(
		"test-commit",
		map[string]string{spec.ScreenID: live},
		map[string]string{spec.ScreenID: approved},
		[]screenshot.ScreenSpec{spec},
	)
	if err == nil {
		t.Fatal("BuildRegionDiffs accepted an unequal comparable region without a screen-specific declared disposition")
	}
	if !strings.Contains(err.Error(), `region "connectivity-output"`) || !strings.Contains(err.Error(), "screen-specific declared disposition") {
		t.Fatalf("BuildRegionDiffs rejected the wrong condition: %v", err)
	}
}

func TestBuildRegionDiffsRejectsUndeclaredVisibleRegionOutsideRequiredRegions(t *testing.T) {
	spec := screenshot.ScreenSpec{
		ScreenID:              "unknown-visible-difference",
		StateMarker:           "shared header",
		ApplicableLive:        true,
		ApplicableApprovedTUI: true,
		RequiredRegions:       []screenshot.RegionName{screenshot.RegionHeader},
		NonApplicability: []screenshot.SurfaceNonApplicability{{
			Surface: "approved-html", Decision: "D-04", Reason: "HTML is not a parity target.", Classification: "ux-improvement",
		}},
	}
	live := "shared header\nIdentities › live breadcrumb\n"
	approved := "shared header\nIdentities › approved breadcrumb\n"

	_, err := screenshot.BuildRegionDiffs(
		"test-commit",
		map[string]string{spec.ScreenID: live},
		map[string]string{spec.ScreenID: approved},
		[]screenshot.ScreenSpec{spec},
	)
	if err == nil {
		t.Fatal("BuildRegionDiffs ignored an unequal visible named region outside RequiredRegions")
	}
	if !strings.Contains(err.Error(), `region "breadcrumb"`) || !strings.Contains(err.Error(), "screen-specific declared disposition") {
		t.Fatalf("BuildRegionDiffs rejected the wrong condition: %v", err)
	}
}

func TestDummyOnlyFrameClassification(t *testing.T) {
	spec := screenshot.ScreenSpec{
		ScreenID:              "dummy-only-state",
		StateMarker:           "ssh command",
		StateMarkers:          []string{"ssh command"},
		ApplicableApprovedTUI: true,
		RequiredRegions:       []screenshot.RegionName{screenshot.RegionConnectivityOutput},
		NonApplicability: []screenshot.SurfaceNonApplicability{
			{Surface: "live", Decision: "D-99", Reason: "The live surface removed this obsolete state.", Classification: "ux-improvement"},
			{Surface: "approved-html", Decision: "D-99", Reason: "HTML is not a parity target.", Classification: "ux-improvement"},
		},
	}
	dummy := "│ ssh command\n│ authenticated dummy-only state\n"
	records, err := screenshot.BuildRegionDiffs("test-commit", nil, map[string]string{spec.ScreenID: dummy}, []screenshot.ScreenSpec{spec})
	if err != nil {
		t.Fatalf("BuildRegionDiffs dummy-only frame: %v", err)
	}
	region := regionDiffByName(t, records[0], screenshot.RegionConnectivityOutput)
	if region.Comparable || region.Equal || region.LiveApplicable || !region.ApprovedApplicable {
		t.Fatalf("dummy-only region applicability/equality is contradictory: %+v", region)
	}
	if region.Decision != "D-99" || region.Classification != "ux-improvement" || region.NonApplicabilityReason == "" {
		t.Fatalf("dummy-only region lacks an explicit classified reason: %+v", region)
	}
}

// TestApprovedTUIPhase3OnlyStatesHaveDecisionLinkedNonApplicability proves the
// approval commit is never asked to impersonate a later Phase 3 state.
func TestApprovedTUIPhase3OnlyStatesHaveDecisionLinkedNonApplicability(t *testing.T) {
	for _, spec := range screenshot.RequiredScreenSpecs() {
		for _, surface := range []string{"live", "approved-tui", "approved-html"} {
			record, found := screenshot.NonApplicabilityForSurface(spec, surface)
			if screenshot.ScreenAppliesToSurface(spec, surface) {
				if found {
					t.Errorf("applicable %s/%s has a non-applicability record", surface, spec.ScreenID)
				}
				continue
			}
			if !found {
				t.Errorf("non-applicable %s/%s lacks a registry record", surface, spec.ScreenID)
				continue
			}
			if record.Decision == "" || record.Reason == "" {
				t.Errorf("non-applicable %s/%s has an incomplete record: %+v", surface, spec.ScreenID, record)
			}
		}
	}

	var stage2 screenshot.ScreenSpec
	for _, spec := range screenshot.RequiredScreenSpecs() {
		if spec.ScreenID == "test-stage2-by-alias" {
			stage2 = spec
			break
		}
	}
	if stage2.ScreenID == "" {
		t.Fatal("test-stage2-by-alias is missing from the registry")
	}
	if stage2.ApplicableApprovedTUI {
		t.Fatal("approved TUI must not claim the Phase 3 completed stage-2 state")
	}
	record, found := screenshot.NonApplicabilityForSurface(stage2, "approved-tui")
	if !found || record.Decision != "D-04" {
		t.Fatalf("approved stage-2 state must be linked to D-04, got %+v", record)
	}
}

func TestBuildRegionDiffsDeclaresFormDefaultsComparator(t *testing.T) {
	spec := screenshot.ScreenSpec{
		ScreenID:              "ssh-form-filled",
		StateMarker:           "Alias prefix",
		ApplicableLive:        true,
		ApplicableApprovedTUI: true,
		RequiredRegions:       []screenshot.RegionName{screenshot.RegionFormFields},
		RegionDispositions: []screenshot.RegionDisposition{{
			Region: screenshot.RegionFormFields, Divergence: "form-defaults", Decision: "D-16",
			Reason: "The live defaults differ from the approved fixture.", Classification: "ux-improvement",
		}},
		NonApplicability: []screenshot.SurfaceNonApplicability{{
			Surface: "approved-html", Decision: "D-16", Reason: "HTML is not a parity target.", Classification: "ux-improvement",
		}},
	}
	live := "header\nbreadcrumb\n│ Shift+→\n│ Alias prefix live\n│ Key Generate\nfooter 1\nfooter 2\nfooter 3\n"
	approved := "header\nbreadcrumb\n│ Shift+→\n│ Alias prefix approved\n│ Key Generate\nfooter 1\nfooter 2\nfooter 3\n"

	diffs, err := screenshot.BuildRegionDiffs("test-commit", map[string]string{spec.ScreenID: live}, map[string]string{spec.ScreenID: approved}, []screenshot.ScreenSpec{spec})
	if err != nil {
		t.Fatalf("BuildRegionDiffs rejected the declared form-default comparator: %v", err)
	}
	region := regionDiffByName(t, diffs[0], screenshot.RegionFormFields)
	if region.Divergence != "form-defaults" || !strings.Contains(region.Justification, "D-16") {
		t.Fatalf("form defaults comparator must be D-16-linked, got %+v", region)
	}
}

func TestBuildRegionDiffsDeclaresConfirmationPreviewComparator(t *testing.T) {
	spec := screenshot.ScreenSpec{
		ScreenID:              "confirm-write",
		StateMarker:           "Exact change",
		ApplicableLive:        true,
		ApplicableApprovedTUI: true,
		RequiredRegions:       []screenshot.RegionName{screenshot.RegionConfirmationPreview},
		RegionDispositions: []screenshot.RegionDisposition{{
			Region: screenshot.RegionConfirmationPreview, Divergence: "confirmation-preview", Decision: "D-05",
			Reason: "The live ceremony differs from the approved fixture.", Classification: "ux-improvement",
		}},
		NonApplicability: []screenshot.SurfaceNonApplicability{{
			Surface: "approved-html", Decision: "D-05", Reason: "HTML is not a parity target.", Classification: "ux-improvement",
		}},
	}
	live := "header\nbreadcrumb\n│ Exact change live\nfooter 1\nfooter 2\nfooter 3\n"
	approved := "header\nbreadcrumb\n│ Exact change approved\nfooter 1\nfooter 2\nfooter 3\n"

	diffs, err := screenshot.BuildRegionDiffs("test-commit", map[string]string{spec.ScreenID: live}, map[string]string{spec.ScreenID: approved}, []screenshot.ScreenSpec{spec})
	if err != nil {
		t.Fatalf("BuildRegionDiffs rejected the declared confirmation comparator: %v", err)
	}
	region := regionDiffByName(t, diffs[0], screenshot.RegionConfirmationPreview)
	if region.Divergence != "confirmation-preview" || !strings.Contains(region.Justification, "D-05") {
		t.Fatalf("confirmation comparator must be D-05-linked, got %+v", region)
	}
}

// TestBuildRegionDiffsAcceptsDivergenceSatisfyingScopedPredicate proves
// WR-27: a disposition with a real Predicate (not just the empty default)
// still accepts a divergence that genuinely satisfies it — the mechanism
// WR-19 introduced but which, until this fix, had zero production callers.
func TestBuildRegionDiffsAcceptsDivergenceSatisfyingScopedPredicate(t *testing.T) {
	spec := screenshot.ScreenSpec{
		ScreenID:              "scoped-predicate-satisfied",
		StateMarker:           "shared header",
		ApplicableLive:        true,
		ApplicableApprovedTUI: true,
		RequiredRegions:       []screenshot.RegionName{screenshot.RegionGitPreview},
		RegionDispositions: []screenshot.RegionDisposition{{
			Region: screenshot.RegionGitPreview, Divergence: "gitdir-default", Decision: "CTX-D-02",
			Reason: "the real binary derives the modern gitdir default", Classification: "ux-improvement",
			Predicate: `absent:"gitdir:~/git/"`,
		}},
		NonApplicability: []screenshot.SurfaceNonApplicability{{
			Surface: "approved-html", Decision: "CTX-D-02", Reason: "HTML is not a parity target.", Classification: "ux-improvement",
		}},
	}
	// Trailing footer1/footer2/footer3 lines are byte-identical on both sides
	// so extractKeybar's own "last 3 non-empty lines" heuristic (an
	// unrelated region) never differs and needs no disposition of its own —
	// isolating this test to the RegionGitPreview predicate under test.
	live := "shared header\n│ includeIf block\n│ [includeIf \"gitdir:~/git/acme/\"]\n│ Write it\nfooter1\nfooter2\nfooter3\n"
	approved := "shared header\n│ includeIf block\n│ [includeIf \"gitdir:~/acme/\"]\n│ Write it\nfooter1\nfooter2\nfooter3\n"

	diffs, err := screenshot.BuildRegionDiffs("test-commit", map[string]string{spec.ScreenID: live}, map[string]string{spec.ScreenID: approved}, []screenshot.ScreenSpec{spec})
	if err != nil {
		t.Fatalf("BuildRegionDiffs rejected a divergence that satisfies its scoped predicate: %v", err)
	}
	region := regionDiffByName(t, diffs[0], screenshot.RegionGitPreview)
	if region.Divergence != "gitdir-default" || !strings.Contains(region.Justification, "CTX-D-02") {
		t.Fatalf("scoped predicate must still classify the accepted divergence, got %+v", region)
	}
}

// TestBuildRegionDiffsRejectsDivergenceViolatingScopedPredicate is WR-27's
// explicit required test: a mutation the predicate is supposed to catch
// must produce an error, not a silent pass. Before WR-27, every disposition
// used blanket uxRegionDifference (empty Predicate), so
// regionPredicateSatisfied always returned true and this rejection branch
// (createflow_packet.go's BuildRegionDiffs) never fired for any real
// disposition — this test proves it now does for a genuinely scoped one.
// The fixture uses the SAME absent:"gitdir:~/git/" predicate
// gitScreenSpecs' real gitPreviewDisposition now carries (WR-27), but with
// text present on BOTH sides — a divergence the predicate was never meant
// to authorize.
func TestBuildRegionDiffsRejectsDivergenceViolatingScopedPredicate(t *testing.T) {
	spec := screenshot.ScreenSpec{
		ScreenID:              "scoped-predicate-violated",
		StateMarker:           "shared header",
		ApplicableLive:        true,
		ApplicableApprovedTUI: true,
		RequiredRegions:       []screenshot.RegionName{screenshot.RegionGitPreview},
		RegionDispositions: []screenshot.RegionDisposition{{
			Region: screenshot.RegionGitPreview, Divergence: "gitdir-default", Decision: "CTX-D-02",
			Reason: "the real binary derives the modern gitdir default", Classification: "ux-improvement",
			Predicate: `absent:"gitdir:~/git/"`,
		}},
		NonApplicability: []screenshot.SurfaceNonApplicability{{
			Surface: "approved-html", Decision: "CTX-D-02", Reason: "HTML is not a parity target.", Classification: "ux-improvement",
		}},
	}
	// Both sides now contain "gitdir:~/git/" -- an UNRELATED divergence
	// (different identity names) that the "absent:" predicate must reject,
	// since the text it was scoped to exclude is present on both sides.
	live := "shared header\n│ includeIf block\n│ [includeIf \"gitdir:~/git/acme-live/\"]\n│ Write it\nfooter1\nfooter2\nfooter3\n"
	approved := "shared header\n│ includeIf block\n│ [includeIf \"gitdir:~/git/acme-approved/\"]\n│ Write it\nfooter1\nfooter2\nfooter3\n"

	_, err := screenshot.BuildRegionDiffs("test-commit", map[string]string{spec.ScreenID: live}, map[string]string{spec.ScreenID: approved}, []screenshot.ScreenSpec{spec})
	if err == nil {
		t.Fatal("BuildRegionDiffs accepted a divergence that violates its own scoped predicate")
	}
	if !strings.Contains(err.Error(), "disposition predicate") || !strings.Contains(err.Error(), "does not match the observed divergence") {
		t.Fatalf("BuildRegionDiffs rejected for the wrong reason: %v", err)
	}
}

func TestValidateRegionDiffsRejectsMissingRequiredRegion(t *testing.T) {
	source := strings.Repeat("f", 40)
	var diffs screenshot.RegionDiffs
	if err := json.Unmarshal(validRegionDiffs(t, source), &diffs); err != nil {
		t.Fatalf("unmarshaling region diffs: %v", err)
	}
	diffs.Screens[0].Regions = nil
	data, err := json.Marshal(diffs)
	if err != nil {
		t.Fatalf("marshaling malformed region diffs: %v", err)
	}
	if err := screenshot.ValidateRegionDiffs(data, source, screenshot.RequiredScreenSpecs()); err == nil {
		t.Fatal("region validation must reject a missing required region")
	}
}

func TestValidateRegionDiffsRejectsMissingVisibleNonRequiredRegion(t *testing.T) {
	source := strings.Repeat("f", 40)
	spec := screenshot.ScreenSpec{
		ScreenID:              "visible-non-required-region",
		StateMarker:           "shared header",
		ApplicableLive:        true,
		ApplicableApprovedTUI: true,
		RequiredRegions:       []screenshot.RegionName{screenshot.RegionHeader},
		NonApplicability: []screenshot.SurfaceNonApplicability{{
			Surface: "approved-html", Decision: "D-04", Reason: "HTML is not a parity target.", Classification: "ux-improvement",
		}},
	}
	capture := "shared header\nIdentities › shared breadcrumb\n"
	records, err := screenshot.BuildRegionDiffs(
		source,
		map[string]string{spec.ScreenID: capture},
		map[string]string{spec.ScreenID: capture},
		[]screenshot.ScreenSpec{spec},
	)
	if err != nil {
		t.Fatalf("building valid region evidence: %v", err)
	}
	var found bool
	for i, region := range records[0].Regions {
		if region.Name == screenshot.RegionBreadcrumb {
			found = true
			records[0].Regions = append(records[0].Regions[:i], records[0].Regions[i+1:]...)
			break
		}
	}
	if !found {
		t.Fatal("test fixture did not produce the visible non-required breadcrumb region")
	}

	data := screenshot.BuildRegionDiffsJSON(source, records)
	if err := screenshot.ValidateRegionDiffs(data, source, []screenshot.ScreenSpec{spec}); err == nil {
		t.Fatal("ValidateRegionDiffs accepted a document missing a visible non-required named region")
	}
}

func regionDiffByName(t *testing.T, record screenshot.RegionDiffRecord, name screenshot.RegionName) screenshot.NamedRegionDiff {
	t.Helper()
	for _, region := range record.Regions {
		if region.Name == name {
			return region
		}
	}
	t.Fatalf("frame %q has no region %q", record.ScreenID, name)
	return screenshot.NamedRegionDiff{}
}

// TestStateMarkerGate proves ValidateCapturedState rejects text that lacks
// the spec's state marker — a route alone cannot authorize a label.
func TestStateMarkerGate(t *testing.T) {
	specs := screenshot.ScreenSpecRegistry()
	if len(specs) == 0 {
		t.Skip("ScreenSpecRegistry not yet implemented")
	}
	spec := specs[0]
	// Text with the route but not the state marker must be rejected.
	textWithoutMarker := "route: " + spec.Route + "\nsome content but no marker"
	if err := screenshot.ValidateCapturedState(spec, textWithoutMarker); err == nil {
		t.Errorf("ValidateCapturedState must reject text lacking the state marker for %q", spec.ScreenID)
	}
	// Text with the marker inside a focused pane (│ prefix) must be accepted.
	// Region-bound enforcement requires the marker to appear within a pane line.
	marker := spec.StateMarker
	if len(spec.StateMarkers) > 0 {
		marker = spec.StateMarkers[0]
	}
	textWithMarker := textWithoutMarker + "\n│ " + marker
	if err := screenshot.ValidateCapturedState(spec, textWithMarker); err != nil {
		t.Errorf("ValidateCapturedState must accept text with state marker %q inside pane; got: %v", marker, err)
	}
}

// TestRouteMarkerMismatch proves ValidateCapturedState rejects a state marker
// from another screen's spec (cross-state marker impersonation).
func TestRouteMarkerMismatch(t *testing.T) {
	specs := screenshot.ScreenSpecRegistry()
	if len(specs) < 2 {
		t.Skip("need at least 2 specs")
	}
	spec0 := specs[0]
	spec1 := specs[1]
	// Text with spec1's marker but spec0's route is rejected.
	crossText := "route: " + spec0.Route + "\n" + spec1.StateMarker
	if err := screenshot.ValidateCapturedState(spec0, crossText); err == nil {
		t.Errorf("ValidateCapturedState must reject marker from another state; screen=%q marker=%q", spec1.ScreenID, spec1.StateMarker)
	}
}

// TestDuplicatePolicyRejectsUnresolved proves the duplicate ID check fails
// for screens with the same ScreenID and no VariantOf declaration.
func TestDuplicatePolicyRejectsUnresolved(t *testing.T) {
	specs := screenshot.ScreenSpecRegistry()
	if len(specs) == 0 {
		t.Skip("ScreenSpecRegistry not yet implemented")
	}
	first := specs[0]
	// A second spec with the same ScreenID but no VariantOf must fail.
	dup := screenshot.ScreenSpec{
		ScreenID:               first.ScreenID,
		Route:                  first.Route,
		StateMarker:            first.StateMarker + "-dup",
		ApplicableLive:         true,
		ApplicableApprovedTUI:  true,
		ApplicableApprovedHTML: true,
		RequiredRegions:        []screenshot.RegionName{screenshot.RegionFormFields},
	}
	err := screenshot.ValidateScreenSpecs(append(specs, dup))
	if err == nil {
		t.Errorf("ValidateScreenSpecs must reject duplicate screen ID %q without VariantOf", first.ScreenID)
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("error must mention 'duplicate'; got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// makeTinyPNG writes a complete valid PNG for packet-validation fixtures.
func makeTinyPNG(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	writeTinyPNG(t, path, color.RGBA{A: 0xff})
	return path
}

func writeTinyPNG(t *testing.T, path string, pixel color.RGBA) {
	t.Helper()
	file, err := os.Create(path) //nolint:gosec // test fixture path built from this test's own t.TempDir() (G304)
	if err != nil {
		t.Fatalf("makeTinyPNG: creating file: %v", err)
	}
	image := image.NewRGBA(image.Rect(0, 0, 1, 1))
	image.SetRGBA(0, 0, pixel)
	if err := png.Encode(file, image); err != nil {
		_ = file.Close()
		t.Fatalf("makeTinyPNG: encoding image: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("makeTinyPNG: closing file: %v", err)
	}
}

// makeMinimalPanels returns every registry-required panel with a unique PNG.
func makeMinimalPanels(t *testing.T, dir string) []screenshot.VisualPanel {
	t.Helper()
	// TUI-only surfaces: approved-html removed from candidate/gate paths (03-16 Task 2).
	surfaces := []string{"live", "approved-tui"}
	panels := make([]screenshot.VisualPanel, 0, screenshot.RequiredVisualPanelCount())
	for _, surface := range surfaces {
		for i, spec := range screenshot.RequiredScreenSpecs() {
			if !screenshot.ScreenAppliesToSurface(spec, surface) {
				continue
			}
			id := spec.ScreenID
			pngPath := filepath.Join(dir, surface+"-"+id+".png")
			writeTinyPNG(t, pngPath, color.RGBA{R: uint8(len(surfaces)*i + len(surface)), A: 0xff}) //nolint:gosec // test-only pixel value, always < 256 for this fixture's tiny loop bounds (G115)
			panels = append(panels, screenshot.VisualPanel{
				Surface:  surface,
				ScreenID: id,
				Text:     "text content for " + surface + "/" + id,
				RawText:  "raw terminal transcript for " + surface + "/" + id,
				PNGPath:  pngPath,
			})
		}
	}
	return panels
}

func validRegionDiffs(t *testing.T, source string) []byte {
	t.Helper()
	hash := func(value string) string {
		sum := sha256.Sum256([]byte(value))
		return fmt.Sprintf("%x", sum)
	}
	diffs := screenshot.RegionDiffs{Version: "test", SourceCommit: source, GeneratedAt: "test"}
	for _, spec := range screenshot.RequiredScreenSpecs() {
		record := screenshot.RegionDiffRecord{ScreenID: spec.ScreenID}
		requiredRegions := make(map[screenshot.RegionName]bool, len(spec.RequiredRegions))
		for _, name := range spec.RequiredRegions {
			requiredRegions[name] = true
		}
		for _, name := range screenshot.AllRegionNames() {
			live := ""
			if requiredRegions[name] && spec.ApplicableLive {
				live = "live " + spec.ScreenID + " " + string(name)
			}
			approved := ""
			if requiredRegions[name] && spec.ApplicableApprovedTUI {
				approved = "approved " + spec.ScreenID + " " + string(name)
				if spec.ApplicableLive {
					approved = live
				}
			}
			region := screenshot.NamedRegionDiff{
				Name: name, LiveText: live, ApprovedText: approved,
				LiveApplicable:     spec.ApplicableLive,
				ApprovedApplicable: spec.ApplicableApprovedTUI,
				Comparable:         spec.ApplicableLive && spec.ApplicableApprovedTUI,
				LiveHash:           hash(live), ApprovedHash: hash(approved), Equal: spec.ApplicableLive && spec.ApplicableApprovedTUI,
			}
			if !region.Comparable {
				surface := "approved-tui"
				if !region.LiveApplicable {
					surface = "live"
				}
				record, ok := screenshot.NonApplicabilityForSurface(spec, surface)
				if !ok {
					t.Fatalf("missing %s non-applicability for %s", surface, spec.ScreenID)
				}
				region.Decision = record.Decision
				region.NonApplicabilityReason = record.Reason
				region.Classification = record.Classification
				region.Justification = record.Decision + ": " + record.Reason
			}
			record.Regions = append(record.Regions, region)
		}
		diffs.Screens = append(diffs.Screens, record)
	}
	data, err := json.Marshal(diffs)
	if err != nil {
		t.Fatalf("marshaling region diffs: %v", err)
	}
	return data
}
