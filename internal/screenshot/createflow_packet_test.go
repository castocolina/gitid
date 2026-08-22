//go:build screenshot

package screenshot_test

// createflow_packet_test.go — TDD RED/GREEN tests for the canonical 24-panel
// evidence packet (plan 03-11 Task 2, CR-01, CR-10).
//
// These tests verify the packet generation and validation infrastructure that
// the `cmd/gitid-evidence` publisher uses.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/castocolina/gitid/internal/dummytui"
	"github.com/castocolina/gitid/internal/screenshot"
)

// fixedClock returns a deterministic clock for tests.
func fixedClock(t time.Time) func() time.Time { return func() time.Time { return t } }

// makeTestCaptures returns two identical sets of text captures from the dummy backend.
func makeTestCaptures() (map[string]string, map[string]string) {
	backend := dummytui.NewFixtureBackend()
	live := screenshot.CaptureCreateFlowScreens(backend)
	approved := screenshot.CaptureCreateFlowScreens(backend)
	return live, approved
}

// TestGenerateTextPacket_CreatesManifest verifies that GenerateTextPacket writes
// a MANIFEST.json into the output directory (CR-01 basic packet creation).
func TestGenerateTextPacket_CreatesManifest(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "packet")
	live, approved := makeTestCaptures()

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
	live, approved := makeTestCaptures()
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
	live, approved := makeTestCaptures()

	_, err := screenshot.GenerateTextPacket(screenshot.PacketOptions{
		SourceCommit:   strings.Repeat("b", 40),
		ApprovalCommit: screenshot.PacketApprovalCommit,
		OutputDir:      outDir,
	}, live, approved)
	if err == nil {
		t.Fatal("GenerateTextPacket with existing OutputDir must return error (CR-01 immutability)")
	}
}

// TestGenerateTextPacket_AllScreensPresent verifies all 8 CreateFlowScreenIDs
// are represented in the packet members (required for completeness).
func TestGenerateTextPacket_AllScreensPresent(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "packet")
	live, approved := makeTestCaptures()

	result, err := screenshot.GenerateTextPacket(screenshot.PacketOptions{
		SourceCommit:   strings.Repeat("c", 40),
		ApprovalCommit: screenshot.PacketApprovalCommit,
		OutputDir:      outDir,
		Clock:          fixedClock(time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)),
	}, live, approved)
	if err != nil {
		t.Fatalf("GenerateTextPacket: %v", err)
	}

	// Verify each screen ID appears in the members (as both live and approved-tui).
	screenCount := make(map[string]int)
	for _, m := range result.Packet.Members {
		if m.ScreenID != "" {
			screenCount[m.ScreenID]++
		}
	}
	for _, id := range screenshot.CreateFlowScreenIDs {
		if screenCount[id] < 2 {
			t.Errorf("screen %q missing from packet members (need ≥2 — live + approved-tui); got %d", id, screenCount[id])
		}
	}
}

// TestValidatePacket_AcceptsValidPacket verifies that a freshly generated packet
// passes ValidatePacket without error (CR-01 round-trip).
func TestValidatePacket_AcceptsValidPacket(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "packet")
	live, approved := makeTestCaptures()

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
	live, approved := makeTestCaptures()

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
	if err := os.WriteFile(absPath, tampered, 0o600); err != nil {
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
		live, approved := makeTestCaptures()
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
	c1 := screenshot.CaptureCreateFlowScreens(backend)
	c2 := screenshot.CaptureCreateFlowScreens(backend)

	if err := screenshot.CompareTextCaptures(c1, c2); err != nil {
		t.Errorf("CompareTextCaptures with identical captures: %v", err)
	}
}

// TestCompareTextCaptures_DifferentReturnError verifies that captures with
// different content produce an error (CR-10 negative case).
func TestCompareTextCaptures_DifferentReturnError(t *testing.T) {
	backend := dummytui.NewFixtureBackend()
	c1 := screenshot.CaptureCreateFlowScreens(backend)
	c2 := make(map[string]string)
	for k, v := range c1 {
		c2[k] = v
	}
	// Mutate one screen in c2.
	firstID := screenshot.CreateFlowScreenIDs[0]
	c2[firstID] = c2[firstID] + "\nMUTATED"

	if err := screenshot.CompareTextCaptures(c1, c2); err == nil {
		t.Error("CompareTextCaptures with different captures must return error (CR-10 negative)")
	}
}

// TestGenerateTextPacket_MissingLiveCapture verifies fail-closed behavior when
// a live capture is missing a required screen (CR-01).
func TestGenerateTextPacket_MissingLiveCapture(t *testing.T) {
	backend := dummytui.NewFixtureBackend()
	live := screenshot.CaptureCreateFlowScreens(backend)
	approved := screenshot.CaptureCreateFlowScreens(backend)

	// Remove one screen from live.
	delete(live, screenshot.CreateFlowScreenIDs[0])

	_, err := screenshot.GenerateTextPacket(screenshot.PacketOptions{
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
	lastLiveIdx := len(screenshot.CreateFlowScreenIDs) - 1 // confirm-write
	dupPNG := makeTinyPNG(t, dir, "dup.png")
	panels[lastLiveIdx] = screenshot.VisualPanel{
		Surface:  "live",
		ScreenID: screenshot.CreateFlowScreenIDs[0], // ssh-form-filled — duplicate
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
// Test helpers
// ---------------------------------------------------------------------------

// makeTinyPNG writes a minimal 1-byte file that won't fail the "non-empty PNG"
// check (GenerateVisualPacket only checks len(png) > 0, not PNG magic bytes).
func makeTinyPNG(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x01}, 0o644); err != nil { //nolint:gosec // test fixture
		t.Fatalf("makeTinyPNG: %v", err)
	}
	return path
}

// makeMinimalPanels returns the exact 24-panel set (one per surface×screen)
// required by GenerateVisualPacket, each with a unique tiny PNG.
func makeMinimalPanels(t *testing.T, dir string) []screenshot.VisualPanel {
	t.Helper()
	surfaces := []string{"live", "approved-tui", "approved-html"}
	panels := make([]screenshot.VisualPanel, 0, 24)
	for _, surface := range surfaces {
		for i, id := range screenshot.CreateFlowScreenIDs {
			// Each panel gets a unique PNG (different byte at index 8).
			pngBytes := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, byte(len(surfaces)*i + len(surface))}
			pngPath := filepath.Join(dir, surface+"-"+id+".png")
			if err := os.WriteFile(pngPath, pngBytes, 0o644); err != nil { //nolint:gosec // test fixture
				t.Fatalf("makeMinimalPanels: writing PNG %s: %v", pngPath, err)
			}
			panels = append(panels, screenshot.VisualPanel{
				Surface:  surface,
				ScreenID: id,
				Text:     "text content for " + surface + "/" + id,
				PNGPath:  pngPath,
			})
		}
	}
	return panels
}
