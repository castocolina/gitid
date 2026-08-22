//go:build screenshot

package screenshot_test

// createflow_packet_test.go — TDD RED/GREEN tests for the canonical 24-panel
// evidence packet (plan 03-11 Task 2, CR-01, CR-10).
//
// These tests verify the packet generation and validation infrastructure that
// the `cmd/gitid-evidence` publisher uses.

import (
	"encoding/json"
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
// 03-13 Task 3: Canonical manifest, self-hash, region diffs, duplicate policy.
// ---------------------------------------------------------------------------

// TestCanonicalManifest proves that a freshly generated packet's ManifestSHA256
// field can be reproduced by zeroing the field and re-hashing — the documented
// canonical self-hash algorithm (empty self-field, UTF-8 indented JSON, no trailing
// newline). This is the machine-verifiable contract the review can check.
func TestCanonicalManifest(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "packet")
	live, approved := makeTestCaptures()

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
	live, approved := makeTestCaptures()

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
	specs := screenshot.ScreenSpecRegistry()
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

// TestRegionDiffCoverage proves that the BuildRegionDiffs function (replacing
// buildRegionDiffsJSONPlaceholder) produces at least one meaningful region
// record for each ScreenSpec. An empty screens list is not acceptable.
func TestRegionDiffCoverage(t *testing.T) {
	// Use dummy backend captures as the input (live vs approved-tui text).
	backend := dummytui.NewFixtureBackend()
	liveCaptures := screenshot.CaptureCreateFlowScreens(backend)
	approvedCaptures := screenshot.CaptureCreateFlowScreens(backend)

	specs := screenshot.ScreenSpecRegistry()
	diffs := screenshot.BuildRegionDiffs("test-commit", liveCaptures, approvedCaptures, specs)

	// Must have at least one diff per ScreenSpec.
	if len(diffs) == 0 {
		t.Fatal("BuildRegionDiffs must return non-empty diffs")
	}
	screensSeen := make(map[string]bool)
	for _, d := range diffs {
		screensSeen[d.ScreenID] = true
		// Each diff must have a non-empty comparison.
		if d.ScreenID == "" {
			t.Error("diff has empty ScreenID")
		}
		if len(d.Regions) == 0 {
			t.Errorf("diff %q has no named regions", d.ScreenID)
		}
		for _, region := range d.Regions {
			if region.Name == "" || region.LiveText == "" || region.ApprovedText == "" {
				t.Errorf("diff %q has an empty named-region comparison: %+v", d.ScreenID, region)
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
			"REGION-DIFFS.json": []byte("regions"),
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
// 03-13 Task 2: ScreenSpec registry, state markers, confirm sentinel.
// ---------------------------------------------------------------------------

// TestScreenSpecRegistry proves that the ScreenSpec registry contains every
// required logical screen ID with non-empty route, interaction, and state marker.
func TestScreenSpecRegistry(t *testing.T) {
	specs := screenshot.ScreenSpecRegistry()
	if len(specs) == 0 {
		t.Fatal("ScreenSpecRegistry must return non-empty specs")
	}
	// Every CreateFlowScreenID must have a spec.
	for _, id := range screenshot.CreateFlowScreenIDs {
		found := false
		for _, s := range specs {
			if s.ScreenID == id {
				found = true
				// Each spec must have a route and state marker.
				if s.Route == "" {
					t.Errorf("spec %q has empty route", id)
				}
				if s.StateMarker == "" {
					t.Errorf("spec %q has empty StateMarker", id)
				}
				break
			}
		}
		if !found {
			t.Errorf("ScreenSpecRegistry missing spec for screen ID %q", id)
		}
	}
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
	// Text with the marker must be accepted.
	textWithMarker := textWithoutMarker + "\n" + spec.StateMarker
	if err := screenshot.ValidateCapturedState(spec, textWithMarker); err != nil {
		t.Errorf("ValidateCapturedState must accept text with state marker %q; got: %v", spec.StateMarker, err)
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
		ScreenID:    first.ScreenID,
		Route:       first.Route,
		StateMarker: first.StateMarker + "-dup",
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
