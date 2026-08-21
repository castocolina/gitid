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
