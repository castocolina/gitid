package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFrame writes a source frame file into dir for name.
func writeFrame(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name+".txt"), []byte(content), 0o600); err != nil {
		t.Fatalf("writeFrame(%s): %v", name, err)
	}
}

// TestPromoteFramesWritesNothingWhenAnyCaptureIsMissing is the WR-13
// regression: the old promotion loop wrote each tracked baseline as it
// went, only checking for missing captures AFTER the loop finished — a
// partial capture run left some baselines rewritten to the new run's
// content and others frozen at their previous commit's content, with the
// provenance table never regenerated at all. A partial capture set must
// leave dstDir completely untouched.
func TestPromoteFramesWritesNothingWhenAnyCaptureIsMissing(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	frames := []promotionEntry{
		{stateID: "a", frame: "frame-a", test: "TestA", shimMode: "ok", geometry: "100x30"},
		{stateID: "b", frame: "frame-b", test: "TestB", shimMode: "ok", geometry: "100x30"},
	}
	// Only "frame-a" exists; "frame-b" is missing (the partial-run shape).
	writeFrame(t, srcDir, "frame-a", "old baseline content for a\n")
	// Seed dstDir with a PRE-EXISTING baseline for "a" from a prior,
	// already-approved run, so we can prove it survives untouched.
	priorContent := "PRIOR approved content for a — must survive untouched\n"
	if err := os.WriteFile(filepath.Join(dstDir, "frame-a.txt"), []byte(priorContent), 0o600); err != nil {
		t.Fatalf("seeding dstDir: %v", err)
	}

	rows, err := promoteFrames(srcDir, dstDir, frames, "deadbeef", nil)
	if err == nil {
		t.Fatal("promoteFrames must fail when a capture is missing")
	}
	if rows != nil {
		t.Errorf("promoteFrames returned rows=%v on a failed run, want nil", rows)
	}

	got, rerr := os.ReadFile(filepath.Join(dstDir, "frame-a.txt")) //nolint:gosec // test-controlled path
	if rerr != nil {
		t.Fatalf("reading dstDir/frame-a.txt: %v", rerr)
	}
	if string(got) != priorContent {
		t.Errorf("frame-a.txt was rewritten despite frame-b being missing — dstDir must stay untouched on partial capture: got %q, want the prior content %q", got, priorContent)
	}
	if _, statErr := os.Stat(filepath.Join(dstDir, "frame-b.txt")); statErr == nil {
		t.Error("frame-b.txt should not exist — it was never a real capture, and nothing should have been written for it")
	}
}

// TestPromoteFramesWritesEveryFrameWhenAllCapturesArePresent is the
// positive control: a complete capture set promotes every frame and
// returns one provenance row per frame, sorted by stateID.
func TestPromoteFramesWritesEveryFrameWhenAllCapturesArePresent(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	frames := []promotionEntry{
		{stateID: "z-second", frame: "frame-z", test: "TestZ", shimMode: "ok", geometry: "100x30"},
		{stateID: "a-first", frame: "frame-a", test: "TestA", shimMode: "ok", geometry: "100x30"},
	}
	writeFrame(t, srcDir, "frame-a", "content a\n")
	writeFrame(t, srcDir, "frame-z", "content z\n")

	rows, err := promoteFrames(srcDir, dstDir, frames, "deadbeef", nil)
	if err != nil {
		t.Fatalf("promoteFrames: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %v, want 2", rows)
	}
	// Sorted by stateID: "a-first" before "z-second".
	if !strings.Contains(rows[0], "a-first") || !strings.Contains(rows[1], "z-second") {
		t.Errorf("rows not sorted by stateID: %v", rows)
	}
	for _, name := range []string{"frame-a", "frame-z"} {
		if _, statErr := os.Stat(filepath.Join(dstDir, name+".txt")); statErr != nil {
			t.Errorf("%s.txt was not written: %v", name, statErr)
		}
	}
}

// ---------------------------------------------------------------------------
// 09.5-05-PLAN.md Task 2 (D-K): phase parameterisation. Every test below
// drives resolvePhase/promoteForPhase — the orchestration layer ABOVE
// promoteFrames — so promoteFrames's own signature and the two pre-existing
// tests above stay byte-for-byte unmodified.
// ---------------------------------------------------------------------------

// seedPhaseFrames writes every entry's frame file into srcDir with
// placeholder content unique per frame (so a cross-frame mixup would be
// detectable), for the given entries.
func seedPhaseFrames(t *testing.T, srcDir string, entries []promotionEntry) {
	t.Helper()
	for _, e := range entries {
		writeFrame(t, srcDir, e.frame, "placeholder content for "+e.frame+"\n")
	}
}

// TestPromoteFramesTargetsTheRequestedPhaseDirectory proves that promoting
// phase "09.5" writes ONLY into that phase's ui-frames directory under the
// given planning-phases root, and that Phase 9's directory is never created
// as a side effect.
func TestPromoteFramesTargetsTheRequestedPhaseDirectory(t *testing.T) {
	srcDir := t.TempDir()
	phasesRoot := t.TempDir()
	seedPhaseFrames(t, srcDir, phase95Frames)

	dstDir, rows, err := promoteForPhase(srcDir, phasesRoot, "09.5", "deadbeef", nil)
	if err != nil {
		t.Fatalf("promoteForPhase(09.5): %v", err)
	}
	wantDst := filepath.Join(phasesRoot, "09.5-full-ssh-git-properties-browser", "ui-frames")
	if dstDir != wantDst {
		t.Errorf("dstDir = %q, want %q", dstDir, wantDst)
	}
	if len(rows) != len(phase95Frames) {
		t.Fatalf("rows = %d, want %d", len(rows), len(phase95Frames))
	}
	for _, e := range phase95Frames {
		if _, statErr := os.Stat(filepath.Join(dstDir, e.frame+".txt")); statErr != nil {
			t.Errorf("%s.txt was not written under the phase 09.5 directory: %v", e.frame, statErr)
		}
	}
	// Nowhere else: Phase 9's directory must not have been created.
	phase9Dir := filepath.Join(phasesRoot, "09-upload-credentials-assist")
	if _, statErr := os.Stat(phase9Dir); statErr == nil {
		t.Errorf("phase 9 directory %q must not exist — promoting phase 09.5 must write nowhere else", phase9Dir)
	}
}

// TestPromoteFramesPhase9DefaultUnchanged proves that the default phase
// ("09", what main() resolves to with no flag override) still resolves to
// today's exact Phase 9 destination directory and today's exact phase9Frames
// list — the parameterisation changed nothing about the existing behavior.
func TestPromoteFramesPhase9DefaultUnchanged(t *testing.T) {
	dir, frames, err := resolvePhase(defaultPhase)
	if err != nil {
		t.Fatalf("resolvePhase(defaultPhase): %v", err)
	}
	if defaultPhase != "09" {
		t.Fatalf("defaultPhase = %q, want \"09\" — the no-argument invocation must still mean Phase 9", defaultPhase)
	}
	wantDir := "09-upload-credentials-assist"
	if dir != wantDir {
		t.Errorf("resolvePhase(%q) dir = %q, want %q", defaultPhase, dir, wantDir)
	}
	if len(frames) != len(phase9Frames) {
		t.Fatalf("resolvePhase(%q) returned %d frames, want %d (phase9Frames unchanged)", defaultPhase, len(frames), len(phase9Frames))
	}
	for i, e := range frames {
		if e != phase9Frames[i] {
			t.Errorf("resolvePhase(%q) frame[%d] = %+v, want %+v (phase9Frames unchanged)", defaultPhase, i, e, phase9Frames[i])
		}
	}
}

// TestPromoteFramesUnknownPhaseFailsClosed proves that an unknown phase
// identifier is refused by name and nothing is written anywhere — the tool
// never writes to a directory it was not asked for.
func TestPromoteFramesUnknownPhaseFailsClosed(t *testing.T) {
	if _, _, err := resolvePhase("99-does-not-exist"); err == nil {
		t.Fatal("resolvePhase must refuse an unknown phase identifier")
	}

	srcDir := t.TempDir()
	phasesRoot := t.TempDir()
	dstDir, rows, err := promoteForPhase(srcDir, phasesRoot, "99-does-not-exist", "deadbeef", nil)
	if err == nil {
		t.Fatal("promoteForPhase must fail for an unknown phase")
	}
	if dstDir != "" {
		t.Errorf("promoteForPhase returned dstDir=%q on failure, want empty", dstDir)
	}
	if rows != nil {
		t.Errorf("promoteForPhase returned rows=%v on failure, want nil", rows)
	}
	entries, readErr := os.ReadDir(phasesRoot)
	if readErr != nil {
		t.Fatalf("reading phasesRoot: %v", readErr)
	}
	if len(entries) != 0 {
		t.Errorf("phasesRoot must stay empty on an unknown-phase refusal, found: %v", entries)
	}
}
